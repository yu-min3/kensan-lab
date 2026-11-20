#!/usr/bin/env bash
# Run both bootstrap phases against the same private local Terraform state.
set -euo pipefail
set +x
umask 077
SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
source "$SCRIPT_DIR/init.sh"
case "${1:-}" in core|oidc|finish) phase=$1 ;; *) bootstrap_error "Usage: bootstrap-tf.sh core|oidc|finish" ;; esac
[[ $# = 1 ]] || bootstrap_error "Specify exactly one phase"
select_context
[[ -f "$STATE/custody-ok" && -s "$STATE/cluster-id" && ! -e "$STATE/finished" ]] ||
  bootstrap_error "No active bootstrap with verified recovery custody for this context"
expected_id=$(cat "$STATE/cluster-id")
# Only send credentials to the tunnel after comparing its unauthenticated cluster ID.
export VAULT_ADDR=http://127.0.0.1:8200
unset VAULT_AGENT_ADDR VAULT_NAMESPACE TF_LOG TF_LOG_PATH TF_LOG_CORE TF_LOG_PROVIDER
live_id=$(vault status -format=json | jq -er '.cluster_id | strings')
[[ "$live_id" = "$expected_id" && "$(cluster_id)" = "$expected_id" ]] ||
  bootstrap_error "Port-forward, Kubernetes context and stored Raft cluster ID must agree"
tfdir="$STATE/terraform"

if [[ "$phase" = finish ]]; then
  [[ -f "$STATE/oidc-applied" ]] || bootstrap_error "Complete bootstrap-tf.sh oidc first"
  if [[ ! -f "$STATE/revoked" ]]; then
    accessor=$(cat "$STATE/root-accessor")
    # A separate admin login remains usable even when resuming after root revocation.
    vault token lookup -format=json | jq -e --arg old "$accessor" '
      .data | .accessor != $old and ((.policies + (.identity_policies // [])) | index("admin") != null)' >/dev/null ||
      bootstrap_error "Log in with the OIDC or emergency admin before finishing"
    printf 'Revoke initial root token and discard private bootstrap state for %s? Type finish: ' "$CONTEXT"
    read -r answer
    [[ "$answer" = finish ]] || bootstrap_error "Cleanup cancelled"
    accessors=$(vault list -format=json auth/token/accessors)
    jq -e 'type == "array" and all(.[]; type == "string")' <<<"$accessors" >/dev/null
    if jq -e --arg id "$accessor" 'index($id) != null' <<<"$accessors" >/dev/null; then
      vault token revoke -accessor "$accessor" >/dev/null
    fi
    touch "$STATE/revoked"
  fi
  rm -rf "$tfdir"
  rm -f "$STATE/root-token" "$STATE/root-accessor" "$STATE/root-accessor.new" "$STATE/init.json" "$STATE/recovery.json"
  touch "$STATE/finished"
  printf 'Initial root revoked; local bootstrap credentials and Terraform state removed.\n'
  exit 0
fi

[[ ! -f "$STATE/oidc-applied" || "$phase" = oidc ]] || bootstrap_error "OIDC phase already completed"
[[ "$phase" = core || -f "$STATE/core-applied" ]] || bootstrap_error "Complete bootstrap-tf.sh core first"
select_recovery_vault
[[ "$OP_VAULT_ID" = "$(cat "$STATE/op-vault-id")" ]] || bootstrap_error "Incorrect recovery vault"
export TF_VAR_vault_address="$VAULT_ADDR"
TF_VAR_vault_token=$(cat "$STATE/root-token")
export TF_VAR_vault_token
VAULT_TOKEN="$TF_VAR_vault_token" vault token lookup -format=json |
  jq -er '.data | select(.policies | index("root") != null) | .accessor | strings | select(length > 0)' > "$STATE/root-accessor.new"
mv "$STATE/root-accessor.new" "$STATE/root-accessor"
TF_VAR_emergency_admin_password=$(op read --no-newline "op://$OP_VAULT_ID/vault-emergency-admin/password")
[[ -n "$TF_VAR_emergency_admin_password" ]] || bootstrap_error "Empty break-glass password"
export TF_VAR_emergency_admin_password
targets=()
if [[ "$phase" = core ]]; then
  export TF_VAR_keycloak_oidc_client_secret=''
  for address in vault_auth_backend.kubernetes vault_kubernetes_auth_backend_config.this vault_auth_backend.userpass \
      vault_generic_endpoint.emergency_admin vault_policy.platform_dev vault_kubernetes_auth_backend_role.vault_config_operator \
      vault_kubernetes_auth_backend_role.external_secrets vault_mount.kv_v2 vault_audit.file vault_audit.stdout; do
    targets+=("-target=$address")
  done
else
  TF_VAR_keycloak_oidc_client_secret=$("$SCRIPT_DIR/../keycloak/read-vault-client-secret.sh")
  [[ -n "$TF_VAR_keycloak_oidc_client_secret" ]] || bootstrap_error "Keycloak returned an empty OIDC secret"
  export TF_VAR_keycloak_oidc_client_secret
fi

mkdir -p "$tfdir"
for name in auth engines main policies variables versions; do cp "$SCRIPT_DIR/$name.tf" "$tfdir/"; done
# Private configuration directory excludes old plaintext .tfvars and prior repository states.
unset TF_CLI_ARGS TF_CLI_ARGS_apply TF_CLI_ARGS_init TF_DATA_DIR TF_WORKSPACE
terraform -chdir="$tfdir" init -input=false
terraform -chdir="$tfdir" apply -input=true ${targets[@]+"${targets[@]}"}
touch "$STATE/$phase-applied"
printf 'Phase %s applied. Keep the private bootstrap directory until finish.\n' "$phase"
