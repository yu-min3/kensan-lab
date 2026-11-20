#!/usr/bin/env bash
# Configure the OSS Vault OIDC login after the Keycloak realm and VCO role exist.
set +x
set -euo pipefail
: "${VAULT_ADDR:?Set VAULT_ADDR and sign in with an authorized Vault account}"
vault token lookup >/dev/null
script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
# Complete the issuer lookup before writing. A failed lookup must never replace
# the OIDC configuration with an empty secret.
client_secret=$("$script_dir/../keycloak/read-vault-client-secret.sh")
[[ -n "$client_secret" ]] || { echo "Vault OIDC client secret missing" >&2; exit 1; }
printf '%s' "$client_secret" | vault write auth/oidc/config \
  oidc_discovery_url="https://auth.yu-mins.com/realms/kensan" \
  oidc_client_id=vault oidc_client_secret=- default_role=default >/dev/null
unset client_secret
vault read -field=default_role auth/oidc/config | grep -qx default
echo 'Vault OIDC configured. Sign in at https://vault.platform.yu-min3.com/ui/vault/auth?with=oidc/'
