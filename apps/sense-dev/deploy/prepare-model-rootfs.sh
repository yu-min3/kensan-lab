#!/usr/bin/env bash
# Prepare a credential-free CLI runtime; never enables the controller.
set -euo pipefail
rootfs=${1:?new absolute rootfs required}
worker=${2:?trusted static worker required}
claude=${3:?verified Claude binary required}
codex=${4:?verified Codex binary required}
claude_sha=${5:?Claude manifest SHA-256 required}
codex_sha=${6:?Codex binary SHA-256 required}
codex_package=${7:?verified full Codex package archive required}
package_sha=${8:?Codex package SHA-256 required}
[[ $EUID == 0 && $rootfs == /* && $rootfs != / && ! -e $rootfs ]] || exit 1
for binary in "$worker" "$claude" "$codex"; do
  [[ $binary == /* && -f $binary && ! -L $binary ]] || exit 1
done
[[ $codex_package == /* && -f $codex_package && ! -L $codex_package ]] || exit 1
[[ $package_sha =~ ^[a-f0-9]{64}$ ]] || exit 1
printf '%s  %s\n' "$package_sha" "$codex_package" | sha256sum -c -
[[ $claude_sha =~ ^[a-f0-9]{64}$ && $codex_sha =~ ^[a-f0-9]{64}$ ]] || exit 1
printf '%s  %s\n%s  %s\n' "$claude_sha" "$claude" "$codex_sha" "$codex" | sha256sum -c -
mkdir -p "$rootfs"/{workspace,agent-auth,proc,dev,tmp,etc/ssl/certs,usr/local/bin,usr/bin,bin}
copy_dependencies() {
  local binary=$1 output
  if output=$(ldd "$binary" 2>&1); then
    while IFS= read -r dependency; do
      cp --parents -L "$dependency" "$rootfs"
    done < <(printf '%s\n' "$output" | awk '{for(i=1;i<=NF;i++) if($i ~ /^\//) print $i}')
  elif [[ $output != *'not a dynamic executable'* && $output != *'statically linked'* ]]; then
    printf 'cannot determine libraries for %s\n' "$binary" >&2
    exit 1
  fi
}
for name in bash sh git env cat ls mkdir cp mv rm touch head tail sed awk find sort cut tr wc grep uname date; do
  binary=$(command -v "$name")
  install -m 0755 "$binary" "$rootfs/usr/bin/$name"
  copy_dependencies "$binary"
done
ln -s /usr/bin/sh "$rootfs/bin/sh"
ln -s /usr/bin/bash "$rootfs/bin/bash"
for name in worker claude codex; do
  binary=${!name}
  destination=$name
  [[ $name != worker ]] || destination=sense-dev-worker
  install -m 0755 "$binary" "$rootfs/usr/local/bin/$destination"
  copy_dependencies "$binary"
done
# Preserve the official package layout: Codex discovers sibling resources and
# sandbox helpers relative to its executable. A thin binary is insufficient.
mkdir -p "$rootfs/opt/codex"
tar --extract --gzip --file "$codex_package" --directory "$rootfs/opt/codex" --no-same-owner
printf '%s  %s\n' "$codex_sha" "$rootfs/opt/codex/bin/codex" | sha256sum -c -
[[ -x $rootfs/opt/codex/codex-resources/bwrap && -x $rootfs/opt/codex/codex-path/rg ]] || exit 1
ln -sf ../../../opt/codex/bin/codex "$rootfs/usr/local/bin/codex"
# Git helpers and CA certificates contain no user configuration or auth.
git_exec=$(git --exec-path)
cp --parents -r -L "$git_exec" "$rootfs"
while IFS= read -r -d '' binary; do
  [[ ! -x $binary ]] || copy_dependencies "$binary"
done < <(find "$git_exec" -type f -print0)
install -m 0644 /etc/ssl/certs/ca-certificates.crt "$rootfs/etc/ssl/certs/ca-certificates.crt"
cp -L /etc/resolv.conf "$rootfs/etc/resolv.conf"
printf 'hosts: files dns\n' > "$rootfs/etc/nsswitch.conf"
printf '127.0.0.1 localhost\n::1 localhost\n' > "$rootfs/etc/hosts"
printf 'kensan-dev:x:%s:%s::/agent-auth:/bin/bash\n' "$(id -u kensan-dev)" "$(id -g kensan-dev)" > "$rootfs/etc/passwd"
find "$rootfs" -type f -exec chmod a-w {} +
sha256sum "$rootfs/usr/local/bin/"*
