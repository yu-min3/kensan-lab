#!/usr/bin/env bash
# markport の版と sha256 を values-markport.yaml に書き込む。
# 使い方: scripts/markport-bump.sh [vX.Y.Z]   （省略時は最新リリース）
#
# linux の 2 バイナリを実際に取得して sha256 を計算し、上流の checksums ファイルと
# 一致したときだけ書き換える。書き換えたら stdout の最終行に新しい版を出す。
# 既に同じ版なら何も変えず、何も出さない。gh・curl・yq が要る。
set -euo pipefail

repo=yomon8/markport
values=kubernetes/apps/app-kensan/values-markport.yaml

env_value() {
  yq -r ".initContainers[] | select(.name == \"fetch-markport\") | .env[] | select(.name == \"$1\") | .value" "$values"
}

sha256() {
  if command -v sha256sum >/dev/null; then sha256sum "$1"; else shasum -a 256 "$1"; fi | cut -d' ' -f1
}

want=${1:-$(gh release view -R "$repo" --json tagName --jq .tagName)}
[[ $want =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo "unexpected version: $want" >&2; exit 1; }

cur=$(env_value MARKPORT_VERSION)
cur_amd64=$(env_value MARKPORT_SHA256_AMD64)
cur_arm64=$(env_value MARKPORT_SHA256_ARM64)
[[ -n $cur && -n $cur_amd64 && -n $cur_arm64 ]] || { echo "markport env not found in $values" >&2; exit 1; }
if [[ $cur == "$want" ]]; then
  echo "markport is already $want" >&2
  exit 0
fi

dir=$(mktemp -d)
trap 'rm -rf "$dir"' EXIT
gh release download "$want" -R "$repo" -D "$dir" \
  -p "checksums_${want}.txt" -p "markport_${want}_linux_amd64" -p "markport_${want}_linux_arm64"

# 取得したバイナリの sha256 を、上流の checksums と一致したときだけ返す
verified_sum() {
  local file="markport_${want}_linux_$1" got listed
  got=$(sha256 "$dir/$file")
  listed=$(awk -v f="$file" '$2 == f {print $1}' "$dir/checksums_${want}.txt")
  if [[ $got != "$listed" ]]; then
    echo "sha256 mismatch for $file: downloaded $got, checksums file ${listed:-<missing>}" >&2
    return 1
  fi
  echo "$got"
}
sum_amd64=$(verified_sum amd64)
sum_arm64=$(verified_sum arm64)

# 値は一意な文字列なので、コメントと書式を保つため行単位で置き換える
sed -i.bak \
  -e "s/value: ${cur}\$/value: ${want}/" \
  -e "s/${cur_amd64}/${sum_amd64}/" \
  -e "s/${cur_arm64}/${sum_arm64}/" \
  "$values"
rm -f "$values.bak"

[[ $(env_value MARKPORT_VERSION) == "$want" &&
   $(env_value MARKPORT_SHA256_AMD64) == "${sum_amd64}" &&
   $(env_value MARKPORT_SHA256_ARM64) == "${sum_arm64}" ]] || { echo "failed to rewrite $values" >&2; exit 1; }

echo "markport $cur -> $want" >&2
echo "$want"
