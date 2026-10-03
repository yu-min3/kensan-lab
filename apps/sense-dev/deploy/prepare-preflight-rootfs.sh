#!/usr/bin/env bash
# Minimal credentialless boundary probe. This is not a model runtime.
set -euo pipefail
rootfs=${1:?absolute new rootfs directory required}
worker=${2:?absolute static Linux sense-dev-worker binary required}
if [[ $rootfs != /* || $rootfs == / || -e $rootfs || $worker != /* || ! -f $worker ]]; then
  echo 'requires a new absolute rootfs and existing static worker' >&2
  exit 1
fi
mkdir -p "$rootfs"/{workspace,agent-auth,proc,dev,tmp,usr/local/bin,usr/bin}
install -m 0755 "$worker" "$rootfs/usr/local/bin/sense-dev-worker"
cp --parents -L /usr/bin/git "$rootfs"
while IFS= read -r dependency; do
  cp --parents -L "$dependency" "$rootfs"
done < <(ldd /usr/bin/git | awk '{for(i=1;i<=NF;i++) if($i ~ /^\//) print $i}')
chmod 0755 "$rootfs" "$rootfs"/{workspace,agent-auth,proc,dev,tmp}
sha256sum "$rootfs/usr/local/bin/sense-dev-worker" "$rootfs/usr/bin/git"
