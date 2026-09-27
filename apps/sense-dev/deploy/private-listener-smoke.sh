#!/usr/bin/env bash
set -euo pipefail

# Read-only smoke check on sense after first install. This is not a public-route audit.
# Usage: sudo bash private-listener-smoke.sh <reviewed-unit-sha256> <reviewed-binary-sha256>

[[ $# -eq 2 && $1 =~ ^[0-9a-f]{64}$ && $2 =~ ^[0-9a-f]{64}$ ]] || { printf 'expected reviewed unit and binary SHA-256\n' >&2; exit 2; }
[[ $EUID -eq 0 ]] || { printf 'root is required to verify socket ownership; no changes are made\n' >&2; exit 1; }
[[ $(hostname -s) == sense ]] || { printf 'refusing non-sense host\n' >&2; exit 1; }

service=kensan-dev-controller.service
unit=/etc/systemd/system/$service
binary=/opt/kensan-dev/bin/sense-dev
expected_argv="$binary -listen 127.0.0.1:8787 -data /var/lib/kensan-dev -admin-token-file /var/lib/kensan-dev/admin-token -tokens-css /opt/kensan-dev/source/packages/design-tokens/tokens.css -mock-worker"

active=$(systemctl is-active "$service")
[[ $active == active ]] || { printf 'service inactive\n' >&2; exit 1; }
enabled=$(systemctl is-enabled "$service")
[[ $enabled == enabled ]] || { printf 'service disabled\n' >&2; exit 1; }
fragment=$(systemctl show "$service" -p FragmentPath --value)
[[ $fragment == "$unit" ]] || { printf 'unexpected unit fragment\n' >&2; exit 1; }
dropins=$(systemctl show "$service" -p DropInPaths --value)
[[ -z $dropins ]] || { printf 'unexpected unit drop-in\n' >&2; exit 1; }
[[ $(sha256sum "$unit" | cut -d ' ' -f 1) == "$1" ]] || { printf 'unit SHA-256 mismatch\n' >&2; exit 1; }
user=$(systemctl show "$service" -p User --value)
[[ $user == kensan-dev ]] || { printf 'unexpected service user\n' >&2; exit 1; }
group=$(systemctl show "$service" -p Group --value)
[[ $group == kensan-dev ]] || { printf 'unexpected service group\n' >&2; exit 1; }
environment=$(systemctl show "$service" -p Environment --value)
[[ -z $environment ]] || { printf 'unexpected service environment\n' >&2; exit 1; }
exec_start=$(systemctl show "$service" -p ExecStart --value)
[[ $exec_start == *"path=$binary ; argv[]=$expected_argv ; ignore_errors="* ]] || { printf 'unexpected effective ExecStart\n' >&2; exit 1; }
main_pid=$(systemctl show "$service" -p MainPID --value)
[[ $main_pid =~ ^[1-9][0-9]*$ ]] || { printf 'missing service MainPID\n' >&2; exit 1; }
running_binary=$(readlink "/proc/$main_pid/exe")
[[ $running_binary == "$binary" ]] || { printf 'unexpected running binary\n' >&2; exit 1; }
[[ $(sha256sum "/proc/$main_pid/exe" | cut -d ' ' -f 1) == "$2" ]] || { printf 'running binary SHA-256 mismatch\n' >&2; exit 1; }
[[ $(sha256sum "$binary" | cut -d ' ' -f 1) == "$2" ]] || { printf 'installed binary SHA-256 mismatch\n' >&2; exit 1; }

listeners=$(ss -H -ltnp '( sport = :8787 )')
[[ -n $listeners ]] || { printf 'no listener on port 8787\n' >&2; exit 1; }
count=0
while IFS= read -r line; do
  [[ -n $line ]] || continue
  local_address=$(awk '{print $4}' <<< "$line")
  [[ $local_address == '127.0.0.1:8787' ]] || { printf 'non-loopback or unexpected listener: %s\n' "$local_address" >&2; exit 1; }
  [[ $line == *"pid=$main_pid,"* ]] || { printf 'listener not owned by reviewed service MainPID\n' >&2; exit 1; }
  count=$((count + 1))
done <<< "$listeners"
[[ $count -eq 1 ]] || { printf 'unexpected listener count: %s\n' "$count" >&2; exit 1; }

http_code=$(curl -q --noproxy '*' --silent --show-error --max-time 3 --output /dev/null --write-out '%{http_code}' 'http://127.0.0.1:8787/login')
[[ $http_code == 200 ]] || { printf 'loopback login returned HTTP %s\n' "$http_code" >&2; exit 1; }
after_pid=$(systemctl show "$service" -p MainPID --value)
[[ $after_pid == "$main_pid" ]] || { printf 'service restarted during smoke check\n' >&2; exit 1; }

printf 'SERVICE=active\nUNIT_HASH=matched\nBINARY_HASH=matched\nLISTENER=127.0.0.1:8787\nLOGIN_HTTP=200\n'
printf 'EXTERNAL_ROUTE=unverified; this check alone is not private-ready evidence\n'
