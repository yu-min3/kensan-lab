# Break-glass drill

Verifies that every non-SSO escape hatch still works. Break-glass accounts exist for the moment Keycloak (or its credential chain) is down — and the [2026-06-06 incident](../incidents/2026-06-06-vault-oidc-credential-drift.md) proved both halves of the lesson: the emergency account saved the recovery, and **rarely-used paths rot silently**. This drill exercises them before they're needed.

The account inventory itself lives in the [Identity & RBAC model](../auth/identity-model.md#break-glass-accounts-outside-sso).

## Cadence

- **Quarterly**, and
- after any Keycloak realm rebuild, bootstrap re-run, or password rotation touching these accounts

## Drill steps

Read-only: each step verifies login + effective permission, changes nothing.

### 1. Vault `emergency-admin` (userpass)

```bash
vault login -method=userpass username=emergency-admin
# password: password manager item kensan-lab/vault/emergency-admin

vault token lookup | grep policies      # expect: [admin]
vault token revoke -self                # clean up the drill token
```

### 2. ArgoCD local `admin`

```bash
argocd login argocd.platform.yu-min3.com --username admin --grpc-web
# password: password manager (ArgoCD local admin)

argocd account get-user-info            # expect: loggedIn: true, username: admin
```

### 3. Grafana local `grafana-admin`

`auto_login` normally skips the local form — open it explicitly:

```
https://grafana.platform.yu-min3.com/login?disableAutoLogin=true
```

Log in with the `grafana-admin` credentials (delivered by ESO from Vault `secret/monitoring/grafana-admin`) and confirm the account has the Admin role.

### 4. Keycloak master-realm `admin`

Credentials never leave the pod (uses the pod's own env):

```bash
kubectl -n platform-auth-prod exec -i deploy/keycloak -- bash -c \
  '/opt/keycloak/bin/kcadm.sh config credentials --server http://localhost:8080 \
     --realm master --user "${KEYCLOAK_ADMIN:-admin}" --password "$KEYCLOAK_ADMIN_PASSWORD" \
   && /opt/keycloak/bin/kcadm.sh get realms/kensan --fields realm,enabled'
# expect: {"realm":"kensan","enabled":true}
```

## After the drill

1. **Cross-check the password manager**: for each account, confirm the stored value is the one that just worked (this is exactly what drifted in the 6/6 incident)
2. Record the drill date and result below

## Drill log

| Date | Result | Notes |
|---|---|---|
| _(add a row per drill)_ | | |

## If a check fails

- Login rejected → the live credential and the password manager have drifted. Recover the live value (or reset it) via the strongest still-working path, then **update the password manager in the same sitting**
- Vault userpass missing entirely → re-create via `bootstrap/vault/` (see [init.sh](https://github.com/yu-min3/kensan-lab/blob/main/bootstrap/vault/init.sh)); if Vault state itself was lost, see the [VCO drift runbook](vco-vault-state-drift.md)
- Treat any failure as an incident-worthy finding: the path was already broken and nobody knew
