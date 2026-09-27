#!/usr/bin/env bash
set -euo pipefail

# First-install bootstrap for sense only. It never stops k3s or changes routes.
# Usage: sudo bash private-host-service.sh install <linux-amd64-binary> <tokens.css> <unit-file> <binary-sha256> <tokens-sha256> <unit-sha256>
#        sudo bash private-host-service.sh status
#        sudo bash private-host-service.sh rollback

service=kensan-dev-controller.service
unit=/etc/systemd/system/$service
root=/opt/kensan-dev
state=/var/lib/kensan-dev
stage=
service_armed=0

cleanup() {
  local rc=$?
  if [[ $service_armed == 1 && $rc -ne 0 ]]; then
    systemctl disable --now "$service" || true
  fi
  if [[ -n $stage ]]; then
    rm -f "$stage/binary" "$stage/tokens.css" "$stage/unit"
    rmdir "$stage" || true
  fi
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

usage() {
  printf 'usage: %s install <binary> <tokens.css> <unit-file> <binary-sha256> <tokens-sha256> <unit-sha256> | status | rollback\n' "$0" >&2
  exit 2
}

[[ ${EUID} -eq 0 ]] || { printf 'run as root\n' >&2; exit 1; }
[[ $(hostname -s) == sense ]] || { printf 'refusing non-sense host\n' >&2; exit 1; }

case ${1:-} in
  install)
    [[ $# -eq 7 ]] || usage
    binary=$2
    tokens=$3
    unit_source=$4
    [[ -f $binary && -f $tokens && -f $unit_source ]] || { printf 'missing input file\n' >&2; exit 1; }
    exec 9>/run/kensan-dev-private-install.lock
    flock -n 9 || { printf 'another install is running\n' >&2; exit 1; }
    [[ ! -e $root && ! -L $root && ! -e $state && ! -L $state ]] || { printf 'dedicated path already exists; first-install only\n' >&2; exit 1; }
    for expected in "$5" "$6" "$7"; do
      [[ $expected =~ ^[0-9a-f]{64}$ ]] || { printf 'invalid expected SHA-256\n' >&2; exit 1; }
    done
    [[ ! -e $unit && ! -L $unit ]] || { printf 'unit already exists; first-install only\n' >&2; exit 1; }
    [[ ! -e ${unit}.d && ! -L ${unit}.d ]] || { printf 'unit drop-in already exists; first-install only\n' >&2; exit 1; }
    [[ $(systemctl show "$service" -p LoadState --value) == not-found ]] || { printf 'systemd already knows this service\n' >&2; exit 1; }
    if getent group kensan-dev >/dev/null || id -u kensan-dev >/dev/null 2>&1; then
      printf 'dedicated identity already exists; first-install only\n' >&2
      exit 1
    fi
    [[ $(uname -m) == x86_64 ]] || { printf 'expected x86_64 host\n' >&2; exit 1; }
    listeners=$(ss -H -ltn '( sport = :8787 )')
    if [[ -n $listeners ]]; then
      printf 'port 8787 is occupied\n' >&2
      exit 1
    fi
    stage=$(mktemp -d /run/kensan-dev-stage.XXXXXX)
    install -o root -g root -m 0700 "$binary" "$stage/binary"
    install -o root -g root -m 0600 "$tokens" "$stage/tokens.css"
    install -o root -g root -m 0600 "$unit_source" "$stage/unit"
    [[ $(sha256sum "$stage/binary" | cut -d ' ' -f 1) == "$5" ]] || { printf 'binary SHA-256 mismatch in protected staging\n' >&2; exit 1; }
    [[ $(sha256sum "$stage/tokens.css" | cut -d ' ' -f 1) == "$6" ]] || { printf 'tokens SHA-256 mismatch in protected staging\n' >&2; exit 1; }
    [[ $(sha256sum "$stage/unit" | cut -d ' ' -f 1) == "$7" ]] || { printf 'unit SHA-256 mismatch in protected staging\n' >&2; exit 1; }
    file -b "$stage/binary" | grep -Eq '^ELF 64-bit.*x86-64' || { printf 'expected linux-amd64 ELF\n' >&2; exit 1; }
    grep -Fq 'ExecStart=/opt/kensan-dev/bin/sense-dev -listen 127.0.0.1:8787 ' "$stage/unit" || { printf 'unit is not loopback-only\n' >&2; exit 1; }
    grep -Fq -- '-mock-worker' "$stage/unit" || { printf 'unit must use simulation-only worker\n' >&2; exit 1; }
    groupadd --system kensan-dev
    useradd --system --gid kensan-dev --home-dir "$state" --shell /usr/sbin/nologin kensan-dev
    install -d -o root -g root -m 0755 "$root/bin" "$root/source/packages/design-tokens"
    install -d -o kensan-dev -g kensan-dev -m 0700 "$state"
    install -o root -g root -m 0755 "$stage/binary" "$root/bin/sense-dev"
    install -o root -g root -m 0644 "$stage/tokens.css" "$root/source/packages/design-tokens/tokens.css"
    install -o root -g root -m 0644 "$stage/unit" "$unit"
    [[ $(sha256sum "$root/bin/sense-dev" | cut -d ' ' -f 1) == "$5" ]] || { printf 'binary SHA-256 mismatch after root copy\n' >&2; exit 1; }
    [[ $(sha256sum "$root/source/packages/design-tokens/tokens.css" | cut -d ' ' -f 1) == "$6" ]] || { printf 'tokens SHA-256 mismatch after root copy\n' >&2; exit 1; }
    [[ $(sha256sum "$unit" | cut -d ' ' -f 1) == "$7" ]] || { printf 'unit SHA-256 mismatch after root copy\n' >&2; exit 1; }
    token_tmp=$(mktemp "$state/.admin-token.XXXXXX")
    openssl rand -hex 32 > "$token_tmp"
    chown kensan-dev:kensan-dev "$token_tmp"
    chmod 0600 "$token_tmp"
    mv "$token_tmp" "$state/admin-token"
    systemctl daemon-reload
    [[ $(systemctl show "$service" -p LoadState --value) == loaded ]] || { printf 'systemd did not load reviewed unit\n' >&2; exit 1; }
    [[ $(systemctl show "$service" -p FragmentPath --value) == "$unit" ]] || { printf 'unexpected unit fragment\n' >&2; exit 1; }
    [[ -z $(systemctl show "$service" -p DropInPaths --value) ]] || { printf 'unexpected unit drop-in\n' >&2; exit 1; }
    [[ $(systemctl show "$service" -p User --value) == kensan-dev ]] || { printf 'unexpected unit user\n' >&2; exit 1; }
    [[ $(systemctl show "$service" -p Group --value) == kensan-dev ]] || { printf 'unexpected unit group\n' >&2; exit 1; }
    exec_start=$(systemctl show "$service" -p ExecStart --value)
    [[ $exec_start == *'path=/opt/kensan-dev/bin/sense-dev'* && $exec_start == *'argv[]=/opt/kensan-dev/bin/sense-dev -listen 127.0.0.1:8787 '* && $exec_start == *'-mock-worker'* ]] || { printf 'unexpected effective ExecStart\n' >&2; exit 1; }
    [[ -z $(systemctl show "$service" -p Environment --value) ]] || { printf 'unexpected unit environment\n' >&2; exit 1; }
    service_armed=1
    systemctl enable "$service"
    if ! systemctl start "$service" || ! systemctl is-active --quiet "$service"; then
      systemctl disable --now "$service" || true
      printf 'service start failed; disabled, data retained\n' >&2
      exit 1
    fi
    printf 'private service active; admin token retained at %s/admin-token (not printed)\n' "$state"
    sha256sum "$root/bin/sense-dev" "$root/source/packages/design-tokens/tokens.css" "$unit"
    ;;
  status)
    [[ $# -eq 1 ]] || usage
    systemctl --no-pager --full status "$service"
    ss -H -ltn '( sport = :8787 )'
    ;;
  rollback)
    [[ $# -eq 1 ]] || usage
    systemctl disable --now "$service"
    printf 'service stopped and disabled; state, token, binary, and unit retained\n'
    ;;
  *) usage ;;
esac
