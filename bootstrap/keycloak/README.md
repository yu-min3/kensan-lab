# Keycloak bootstrap

Run `bootstrap/keycloak/setup.sh` from a trusted terminal to reconcile the
`kensan` realm, its groups, platform user and the five OIDC clients. The script
displays the active Kubernetes context for confirmation. It needs `kubectl`,
`jq`, `openssl`, and a ready Keycloak pod in `platform-auth-prod`. It uses the
admin credential already supplied to the pod. A temporary private `kcadm`
session is removed when the script exits.

## Human login

For a new realm, first create a Login named `keycloak-user-yu` with a generated
password in the dedicated 1Password vault, then export `KENSAN_OP_VAULT` to
that vault's name or ID. The script uses `op read` to obtain the password while
creating the user and its group membership. An existing user's password and
memberships are preserved. If the current password is still in the previous
password manager, transfer that login to 1Password before retiring the old copy.

## Where client credentials come from

Keycloak issues and retains each client secret. A rerun reconciles the callback
URLs and uses the issuer's current credential. For the four runtime consumers,
set `VAULT_ADDR` and sign in with an authorized Vault session before running
the script. Without `VAULT_ADDR`, it completes Keycloak setup and explicitly
reports the pending KV sync. Run it again after Vault becomes available.

| Client | Consumer |
| --- | --- |
| `vault` | `bootstrap/vault/post-bootstrap-oidc-ux.sh` reads the current secret directly from Keycloak and configures Vault OIDC login. |
| `istio-gateway-platform` | Vault KV `secret/platform-auth/istio-gateway/platform` |
| `grafana` | Vault KV `secret/observability/grafana/oidc` |
| `argocd` | Vault KV `secret/security/argocd/oidc` |
| `backstage` | Vault KV `secret/backstage/oidc` |

KV synchronization preserves the existing gateway cookie and Backstage session
keys. A brand-new KV path gets a fresh key. A failed read, deleted version or
concurrent edit stops the update. Inspect the affected path and restore the
expected data before retrying. After rebuilding the realm, run the OIDC
configuration script with an existing Vault admin session or emergency-admin
access to enable SSO with the new issuer credential.

## Rebuild and restore

The current Keycloak and PostgreSQL deployments receive their credentials
through Vault KV and External Secrets. A fresh cluster needs those foundational
values before Keycloak can start. For a restored PostgreSQL data volume, its
credentials must match the restored database. Preserve database backups to
restore realm identities, user credentials, MFA enrollments and client
registrations. See the [Vault bootstrap and recovery procedure](https://github.com/yu-min3/kensan-lab/blob/main/bootstrap/vault/README.md).
