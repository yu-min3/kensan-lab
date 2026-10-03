#!/usr/bin/env bash
# Build separately from model runtimes. Dependency installation is host-only;
# the resulting verifier runs offline without provider credentials.
set -euo pipefail
rootfs=${1:?new absolute rootfs required}
base=${2:?credentialless preflight rootfs required}
requirements=${3:?hash-pinned requirements exported from the task lockfile required}
wheelhouse=${4:-}
if [[ $rootfs != /* || $rootfs == / || -e $rootfs || $base != /* || ! -d $base || ! -f $requirements ]]; then
  echo 'requires a new rootfs, existing credentialless base and hash-pinned requirements' >&2
  exit 1
fi
python=/usr/bin/python3.12
[[ -x $python && -d /usr/lib/python3.12 ]] || exit 1
stage=$(mktemp -d "${rootfs}.build.XXXXXX")
# Keep the build directory as evidence, including wheels and install log.
"$python" -m venv "$stage/installer"
download_source=()
if [[ -n $wheelhouse ]]; then
  [[ $wheelhouse == /* && -d $wheelhouse ]] || exit 1
  download_source=(--no-index --find-links "$wheelhouse")
fi
"$stage/installer/bin/python" -m pip download --disable-pip-version-check \
  --require-hashes --no-deps --only-binary=:all: \
  "${download_source[@]}" --dest "$stage/wheels" -r "$requirements" >"$stage/download.log" 2>&1
cp -a "$base" "$rootfs"
mkdir -p "$rootfs/usr/lib/python3" "$rootfs/opt/python" "$rootfs/opt/verifier-evidence"
ln -s /opt/python "$rootfs/usr/lib/python3/dist-packages"
cp --parents -L "$python" "$rootfs"
ln -s python3.12 "$rootfs/usr/bin/python3"
cp -a /usr/lib/python3.12 "$rootfs/usr/lib/"
while IFS= read -r binary; do
  while IFS= read -r dependency; do
    cp --parents -L "$dependency" "$rootfs"
  done < <(ldd "$binary" 2>/dev/null | awk '{for(i=1;i<=NF;i++) if($i ~ /^\//) print $i}')
done < <(printf '%s\n' "$python"; find /usr/lib/python3.12/lib-dynload -type f -name '*.so')
"$stage/installer/bin/python" -m pip install --disable-pip-version-check \
  --require-hashes --no-index --no-deps --no-compile \
  --find-links "$stage/wheels" --target "$rootfs/opt/python" \
  -r "$requirements" >"$stage/install.log" 2>&1
while IFS= read -r binary; do
  while IFS= read -r dependency; do
    cp --parents -L "$dependency" "$rootfs"
  done < <(ldd "$binary" 2>/dev/null | awk '{for(i=1;i<=NF;i++) if($i ~ /^\//) print $i}')
done < <(find "$rootfs/opt/python" -type f -name '*.so')
cp "$requirements" "$rootfs/opt/verifier-evidence/requirements.txt"
# Remove inherited host ownership; runtime is a read-only mount.
chown -R root:root "$rootfs"
chmod -R go-w "$rootfs"
find "$rootfs" -type f -print0 | sort -z | xargs -0 sha256sum >"$stage/rootfs-files.sha256"
sha256sum "$requirements" "$stage/rootfs-files.sha256"
printf 'build_evidence=%s\n' "$stage"
