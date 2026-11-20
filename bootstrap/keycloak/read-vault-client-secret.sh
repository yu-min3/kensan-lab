#!/usr/bin/env bash
# Shared admin session for setup.sh and the Vault OIDC client lookup.
# Each invocation owns a private kcadm config in one Keycloak Pod.
keycloak_exec() {
  kubectl --context "$KC_CONTEXT" -n platform-auth-prod exec -i "$KC_POD" -c keycloak -- "$@"
}

keycloak_close_session() {
  if [[ -n ${KC_CONFIG_DIR:-} ]]; then
    keycloak_exec rm -rf -- "$KC_CONFIG_DIR" </dev/null >/dev/null || true
  fi
}

keycloak_open_session() {
  : "${KC_CONTEXT:?Select a Kubernetes context first}"
  KC_POD=$(kubectl --context "$KC_CONTEXT" -n platform-auth-prod get pods -l app=keycloak -o json |
    jq -er 'first(.items[] | select(.metadata.deletionTimestamp == null)
      | select(any(.status.conditions[]?; .type == "Ready" and .status == "True"))
      | "pod/" + .metadata.name)')
  KC_CONFIG_DIR=$(keycloak_exec mktemp -d /tmp/kensan-kcadm.XXXXXXXX </dev/null)
  [[ "$KC_CONFIG_DIR" =~ ^/tmp/kensan-kcadm\.[[:alnum:]]+$ ]] || {
    echo 'Unable to allocate a private Keycloak admin session' >&2; return 1;
  }
  trap keycloak_close_session EXIT
  # KEYCLOAK_ADMIN_PASSWORD is already supplied to Keycloak via envFrom.
  keycloak_exec sh -ec '
    printf "%s\n" "${KEYCLOAK_ADMIN_PASSWORD:?}" |
      /opt/keycloak/bin/kcadm.sh config credentials --config "$1/kcadm.config" \
        --server http://localhost:8080 --realm master --user admin
  ' sh "$KC_CONFIG_DIR" </dev/null >/dev/null
}

kcadm() {
  keycloak_exec /opt/keycloak/bin/kcadm.sh "$@" --config "$KC_CONFIG_DIR/kcadm.config"
}

keycloak_client_secret() {
  kcadm get "clients/$1/client-secret" -r kensan |
    jq -er '.value | select(type == "string" and length > 0)'
}

if [[ ${BASH_SOURCE[0]} == "$0" ]]; then
  set +x
  set -euo pipefail
  if [[ -t 1 ]]; then
    echo 'Pipe this lookup to its consumer. The output is a credential.' >&2
    exit 1
  fi
  KC_CONTEXT=$(kubectl config current-context)
  keycloak_open_session
  client_uuid=$(kcadm get clients -r kensan --query clientId=vault --fields id,clientId |
    jq -er 'map(select(.clientId == "vault")) | select(length == 1) | .[0].id | select(type == "string" and length > 0)')
  keycloak_client_secret "$client_uuid"
fi
