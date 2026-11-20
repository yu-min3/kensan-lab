#!/usr/bin/env bash
# Initialize new Raft storage and escrow its recovery shares. No Keycloak dependency.
set -euo pipefail
set +x
umask 077

bootstrap_error() { printf 'ERROR: %s\n' "$*" >&2; exit 1; }

select_context() {
  CONTEXT=$(kubectl config current-context)
  KCTL=(kubectl --context "$CONTEXT")
  local uid base
  uid=$("${KCTL[@]}" get namespace kube-system -o jsonpath='{.metadata.uid}')
  [[ "$uid" =~ ^[a-zA-Z0-9-]+$ ]] || bootstrap_error "Cannot identify the Kubernetes cluster"
  base="${XDG_STATE_HOME:-$HOME/.local/state}/kensan-lab/vault-bootstrap"
  [[ "$base" = /* ]] || bootstrap_error "The bootstrap state path must be absolute"
  mkdir -p "$base"
  STATE="$base/$uid"
  # Serialize init, resume and finish on this Mac. Never guess stale locks.
  LOCK="$STATE.lock"
  mkdir "$LOCK" || bootstrap_error "Bootstrap is locked: $LOCK. Check for another session first."
  trap 'rmdir "$LOCK"' EXIT
}

pod_vault() { "${KCTL[@]}" -n vault exec vault-0 -c vault -- vault "$@"; }

cluster_id() {
  local id
  id=$(pod_vault status -format=json | jq -er '.cluster_id | strings') || return 1
  [[ "$id" =~ ^[[:alnum:]-]+$ ]] || return 1
  printf '%s\n' "$id"
}

select_recovery_vault() {
  : "${KENSAN_OP_VAULT:?Set KENSAN_OP_VAULT to the human recovery vault name or ID}"
  OP_VAULT_ID=$(op vault get "$KENSAN_OP_VAULT" --format json | jq -er '.id | strings | select(length > 0)')
}

init_main() {
  [[ $# = 0 ]] || bootstrap_error "Usage: init.sh"
  select_context
  [[ ! -e "$STATE/finished" ]] || bootstrap_error "Bootstrap completed already; use the recovery runbook"
  local status=0
  pod_vault operator init -status >/dev/null || status=$?
  case "$status" in
    0) [[ -s "$STATE/init.json" || -f "$STATE/custody-ok" ]] ||
         bootstrap_error "Existing initialized Vault, no matching local init result. Use the restore/resume runbook." ;;
    2) [[ ! -e "$STATE" ]] || bootstrap_error "New storage and previous bootstrap state coexist: $STATE. Investigate before any init." ;;
    *) bootstrap_error "Vault initialization status is unknown (exit $status)" ;;
  esac

  select_recovery_vault
  if [[ $status = 2 ]]; then
    # Check read access to the required break-glass login before creating new Vault state.
    op read --no-newline "op://$OP_VAULT_ID/vault-emergency-admin/password" | wc -c | awk '$1 > 0 {found=1} END {exit !found}' ||
      bootstrap_error "Create a Login named vault-emergency-admin with a nonempty password in the selected vault"
    printf 'Initialize new Vault storage in Kubernetes context %s? Type init: ' "$CONTEXT"
    read -r answer
    [[ "$answer" = init ]] || bootstrap_error "Initialization cancelled"
    mkdir "$STATE"
    printf '%s\n' "$OP_VAULT_ID" > "$STATE/op-vault-id"
    # From this point on, even a lost CLI connection may mean the server committed.
    # Keep the response on errors/signals, and never repeat init automatically.
    printf 'Private bootstrap working directory: %s\n' "$STATE"
    pod_vault operator init -recovery-shares=5 -recovery-threshold=3 -format=json > "$STATE/init.json" ||
      bootstrap_error "Init response may be incomplete; inspect storage status and preserve $STATE"
  fi

  [[ "$(cat "$STATE/op-vault-id")" = "$OP_VAULT_ID" ]] || bootstrap_error "Select the same 1Password vault used at initialization"
  local id title document matches checksum remote_checksum
  id=$(cluster_id) || bootstrap_error "Cannot identify the unsealed Vault. Check AWS KMS/access, then rerun init.sh."
  if [[ -f "$STATE/cluster-id" && "$(cat "$STATE/cluster-id")" != "$id" ]]; then
    bootstrap_error "Raft cluster ID differs from this bootstrap state"
  fi
  if [[ ! -f "$STATE/custody-ok" ]]; then
    jq -e '
      (.root_token | type == "string" and length > 0) and
      (.recovery_keys_b64 | type == "array" and length == 5 and (unique | length == 5)
        and all(.[]; type == "string" and length > 0)) and
      .recovery_keys_threshold == 3
    ' "$STATE/init.json" >/dev/null || bootstrap_error "Incomplete init response; preserve $STATE"
    jq -er '.root_token' "$STATE/init.json" > "$STATE/root-token"
    # Bind a resumed init response to this Raft cluster before escrowing its keys.
    # Stdin keeps the token out of the kubectl command/Pod exec arguments.
    "${KCTL[@]}" -n vault exec -i vault-0 -c vault -- sh -ec \
      'IFS= read -r VAULT_TOKEN; export VAULT_TOKEN; vault token lookup -format=json' < "$STATE/root-token" |
      jq -e '.data.policies | index("root") != null' >/dev/null ||
      bootstrap_error "Staged initial root token does not authenticate to this Vault"
    printf '%s\n' "$id" > "$STATE/cluster-id"
    title="vault-recovery-$id"
    # Only recovery material enters 1Password. The temporary root never leaves the operator's bootstrap session.
    jq -c --arg id "$id" '{cluster_id:$id, recovery_threshold:.recovery_keys_threshold, recovery_keys_b64}' "$STATE/init.json" > "$STATE/recovery.json"
    matches=$(op document list --vault "$OP_VAULT_ID" --format json |
      jq -cer --arg title "$title" '[.[] | select(.title == $title) | .id]')
    case "$(jq length <<<"$matches")" in
      0)
        [[ ! -e "$STATE/upload-attempted" ]] || bootstrap_error "Previous upload outcome uncertain. Verify $title in 1Password; rerun when visible or remove $STATE/upload-attempted only after confirming absence."
        touch "$STATE/upload-attempted"
        document=$(op document create "$STATE/recovery.json" --vault "$OP_VAULT_ID" --title "$title" --format json |
          jq -er '.id | strings | select(length > 0)') ;;
      1) document=$(jq -er '.[0]' <<<"$matches") ;;
      *) bootstrap_error "Ambiguous recovery document title: $title" ;;
    esac
    checksum=$(shasum -a 256 < "$STATE/recovery.json")
    remote_checksum=$(op document get "$document" --vault "$OP_VAULT_ID" | shasum -a 256)
    [[ "$checksum" = "$remote_checksum" ]] || bootstrap_error "Recovery document verification failed; retained $STATE"
    touch "$STATE/custody-ok"
  fi
  [[ -s "$STATE/root-token" ]] || bootstrap_error "Local bootstrap token is missing; use recovery procedures"
  rm -f "$STATE/init.json" "$STATE/recovery.json"
  printf 'Recovery shares verified (vault-recovery-%s). Next: port-forward Vault and run bootstrap-tf.sh core.\n' "$id"
}

if [[ "${BASH_SOURCE[0]}" = "$0" ]]; then
  init_main "$@"
fi
