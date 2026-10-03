#!/usr/bin/env bash
# Run baseline or candidate tests without credentials, network or source writes.
set -euo pipefail
rootfs=${1:?absolute Python verifier rootfs required}
checkout=${2:?absolute checkout required}
state=${3:?absolute controller state to hide required}
for path in "$rootfs" "$checkout" "$state"; do
  [[ $path == /* && $path != / && -d $path ]] || exit 1
done
host_net_ns=$(readlink /proc/self/ns/net)
systemd-run --quiet --wait --pipe --collect --uid=kensan-dev \
  --property=NoNewPrivileges=yes --property=MemoryMax=1G \
  --property=TasksMax=64 --property=CPUQuota=100% \
  --property=RuntimeMaxSec=120 --property=LimitCORE=0 \
  /usr/bin/bwrap --unshare-all --die-with-parent --new-session --clearenv \
  --setenv HOME /tmp --setenv PATH /usr/local/bin:/usr/bin:/bin \
  --setenv PYTHONDONTWRITEBYTECODE 1 \
  --ro-bind "$rootfs" / --proc /proc --dev /dev --tmpfs /tmp \
  --ro-bind "$checkout" /workspace --chdir /workspace -- \
  /usr/bin/python3 -c '
import subprocess, sys
subprocess.run(["/usr/local/bin/sense-dev-worker", "-preflight-verifier",
                "-hidden-path", sys.argv[1], "-host-net-ns", sys.argv[2]], check=True)
for args in (["--collect-only", "-q", "tests/test_main.py"],
             ["-q", "tests/test_main.py"]):
    subprocess.run([sys.executable, "-m", "pytest", "-p", "no:cacheprovider", *args], check=True)
' "$state" "$host_net_ns"
