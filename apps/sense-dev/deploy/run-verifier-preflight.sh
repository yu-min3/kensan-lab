#!/usr/bin/env bash
set -euo pipefail
rootfs=${1:?absolute preflight rootfs required}
checkout=${2:?absolute read-only fixture directory required}
state=${3:?absolute controller state to hide required}
for path in "$rootfs" "$checkout" "$state"; do
  [[ $path == /* && $path != / && -d $path ]] || exit 1
done
host_net_ns=$(readlink /proc/self/ns/net)
systemd-run --quiet --wait --pipe --collect --uid=kensan-dev \
  --property=NoNewPrivileges=yes \
  /usr/bin/bwrap --unshare-all --die-with-parent --new-session --clearenv \
  --setenv HOME /tmp --setenv PATH /usr/local/bin:/usr/bin:/bin \
  --ro-bind "$rootfs" / --proc /proc --dev /dev --tmpfs /tmp \
  --ro-bind "$checkout" /workspace --chdir /workspace -- \
  /usr/local/bin/sense-dev-worker -preflight-verifier \
  -hidden-path "$state" -host-net-ns "$host_net_ns"
