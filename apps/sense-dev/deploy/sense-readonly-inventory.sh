#!/usr/bin/env bash
# Read-only host inventory for G0/G4a. Run as the normal operator account on
# sense; it never invokes sudo, reads Secret values, or changes services.
set -uo pipefail

section() { printf '\n### %s\n' "$1"; }
capture() {
  if ! "$@" 2>/dev/null; then
    printf 'UNAVAILABLE: %s\n' "$1"
  fi
}

printf 'KENSAN_SENSE_INVENTORY_VERSION=1\n'
printf 'UTC='; date -u '+%Y-%m-%dT%H:%M:%SZ'
printf 'HOST='; hostname

section 'Host and capacity'
capture uname -srmo
capture uptime
capture nproc
capture free -h
capture df -hT
capture lsblk -o NAME,SIZE,FSTYPE,MOUNTPOINTS
capture findmnt -rn -o TARGET,SOURCE,FSTYPE

section 'Network and listeners (no process command lines)'
capture ip -brief address
capture ip -4 route show
capture ip -6 route show
capture ss -H -lntup

section 'Relevant service names and states (no unit environment or ExecStart)'
if command -v systemctl >/dev/null 2>&1; then
  systemctl list-unit-files --type=service --no-legend --no-pager 2>/dev/null | \
    awk '$1 ~ /(k3s|docker|containerd|cloudflared|tailscale|nginx|traefik|caddy|syncthing|argo|kensan)/ {print $1, $2}'
  systemctl list-units --type=service --all --no-legend --no-pager 2>/dev/null | \
    awk '$1 ~ /(k3s|docker|containerd|cloudflared|tailscale|nginx|traefik|caddy|syncthing|argo|kensan)/ {print $1, $3, $4}'
fi

section 'Container workload names (no environment or logs)'
if command -v docker >/dev/null 2>&1; then
  capture docker ps --format '{{.Names}}\t{{.Status}}\t{{.Image}}'
fi

section 'Kubernetes topology, volumes, and existing routes (no Secrets/ConfigMaps)'
if command -v kubectl >/dev/null 2>&1; then
  kubectl_safe=(kubectl)
elif command -v k3s >/dev/null 2>&1; then
  kubectl_safe=(k3s kubectl)
else
  kubectl_safe=()
fi
if ((${#kubectl_safe[@]})); then
  capture "${kubectl_safe[@]}" get nodes -o wide
  capture "${kubectl_safe[@]}" get pods -A -o wide
  capture "${kubectl_safe[@]}" get svc,ingress -A -o wide
  capture "${kubectl_safe[@]}" get pv,pvc -A -o wide
  capture "${kubectl_safe[@]}" get storageclass
else
  printf 'UNAVAILABLE: kubectl or k3s CLI\n'
fi

section 'Potential public-route configuration file paths only (not contents)'
for config_dir in /etc/cloudflared /etc/nginx /etc/caddy /etc/traefik /etc/rancher/k3s /etc/systemd/system; do
  if [[ -d "$config_dir" ]]; then
    find "$config_dir" -maxdepth 2 -type f -print 2>/dev/null | sort
  fi
done

section 'Inventory limits'
printf 'This output does not prove Internet inaccessibility, cluster independence, or safe shutdown.\n'
printf 'Inspect proxy/tunnel routes, router/firewall, CI previews, and dependencies separately before a private-ready decision.\n'
