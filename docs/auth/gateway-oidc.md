# Gateway OIDC (Path A) operations guide

The operational documentation for centrally controlling auth/authz under `*.platform.yu-min3.com` at the Istio gateway-platform.

For the design rationale and the path-selection discussion, see ADR-005 / ADR-010. This document focuses on **explaining what's currently running, and the steps for adding a new host**.

## TL;DR

| Auth method | Which hosts | Treatment at the gateway |
|---|---|---|
| **App-native auth** (the app has its own OIDC or auth) | Vault, ArgoCD, Keycloak, oauth2-proxy itself, **Grafana** | **Bypass** (gateway passes through) |
| **Gateway-enforced OIDC** (the app has no auth of its own, or trusts proxy headers) | Backstage, Prometheus, Hubble, Longhorn | **CUSTOM (oauth2-proxy ext_authz) + ALLOW (group-claim check)** |

> **Why Grafana bypasses**: Grafana misreads the `Authorization: Bearer ...` header that oauth2-proxy injects as its own API key and returns 403, so the gateway lets it through untouched and Grafana's own Keycloak OIDC (`auth.generic_oauth`) handles authentication. Implemented as Category 1 in `authorizationpolicy-gateway-platform-allow.yaml`.

The deciding axis is only **whether the app has its own auth** — not CLI vs UI. Vault shares the same host between its CLI and UI, and Vault itself authenticates both.

## Architecture overview

```
                    Internet
                       │
                       ▼
              gateway-platform (Istio)
                       │
            ┌──────────┴──────────┐
            ▼                     ▼
     [bypass hosts]         [protected hosts]
     vault / argocd /        backstage / prometheus /
     auth / oauth2-proxy /   hubble / longhorn
     grafana                       │
            │                     │
            │                     ▼
            │             ext_authz CUSTOM
            │                     │
            │                     ▼
            │             oauth2-proxy
            │              (cookie / Keycloak redirect)
            │                     │
            │                     ▼
            │             RequestAuthentication
            │              (Bearer JWT verification)
            │                     │
            │                     ▼
            │             ALLOW (groups claim)
            │                     │
            ▼                     ▼
       upstream app          upstream app
       (native auth)          (header trust)
```

## Host × group matrix

Corresponds to the rules in `kubernetes/network/istio/authorizationpolicy-gateway-platform-allow.yaml`:

| Host | Category | Allowed groups | Auth method |
|---|---|---|---|
| `auth.platform.yu-min3.com` | 1: bypass | (no claim check) | Keycloak itself |
| `oauth2-proxy.platform.yu-min3.com` | 1: bypass | (no claim check) | Callback-only, not protected |
| `vault.platform.yu-min3.com` | 1: bypass | (no claim check) | Vault native OIDC + Vault token |
| `argocd.platform.yu-min3.com` | 1: bypass | (no claim check) | ArgoCD native OIDC + its own JWT |
| `grafana.platform.yu-min3.com` | 1: bypass | (no claim check) | Grafana native OIDC (`auth.generic_oauth`). Bypasses to avoid the Bearer-header misread |
| `backstage.platform.yu-min3.com` | 2: admin + dev | `platform-admin`, `platform-dev` | oauth2-proxy enforced |
| `prometheus.platform.yu-min3.com` | 2: admin + dev | `platform-admin`, `platform-dev` | oauth2-proxy enforced |
| `hubble.platform.yu-min3.com` | 3: admin only | `platform-admin` | oauth2-proxy enforced |
| `longhorn.platform.yu-min3.com` | 3: admin only | `platform-admin` | oauth2-proxy enforced |

## Resource layout

| File | Role |
|---|---|
| `kubernetes/network/istio/requestauthentication-gateway-platform.yaml` | Verifies the Authorization header's Bearer JWT against Keycloak, populates `request.auth.claims` |
| `kubernetes/network/istio/authorizationpolicy-gateway-platform-allow.yaml` | The main allow rules. 3 rules cover every host; anything not listed here is implicitly denied |
| `kubernetes/network/istio/authorizationpolicy-gateway-platform-oauth2.yaml` | Enforces oauth2-proxy ext_authz (CUSTOM action) only for Category 2/3 hosts |
| `kubernetes/auth/oauth2-proxy/values.yaml` | oauth2-proxy's own Helm values — cookie domain, Keycloak client id, etc. |
| `kubernetes/network/istio/istiod/values.yaml` | Registers oauth2-proxy as an ext_authz target via `meshConfig.extensionProviders[oauth2-proxy]` |

## Checklist for adding a new host

### Case A: a new UI app (no native auth, want oauth2-proxy to protect it)

Example: adding `awx.platform.yu-min3.com`, opening it to admin + dev.

1. Add it to `hosts:` in the Category 2 rule of `authorizationpolicy-gateway-platform-allow.yaml`
2. Also add it to `hosts:` in `authorizationpolicy-gateway-platform-oauth2.yaml` (bring it under CUSTOM enforcement)
3. Add an HTTPRoute on the app side (parentRef `gateway-platform`)
4. Configure the app's auth in **header-trust mode** (trusting oauth2-proxy's `X-Auth-Request-User` etc.). For Backstage, this means the proxy auth provider, etc. (Note: Grafana bypasses instead, due to the Bearer misread — see Case B.)

### Case B: a new app with its own auth (same class as Vault/ArgoCD)

Example: adding `nexus.platform.yu-min3.com`, where Nexus already has OIDC configured.

1. Add it to `hosts:` in the Category 1 rule of `authorizationpolicy-gateway-platform-allow.yaml`
2. Create an OIDC client for Nexus in Keycloak (same procedure as `bootstrap/keycloak/setup.sh`)
3. Set the Keycloak realm `kensan` as the SSO source on the app side
4. **Don't touch** the CUSTOM policy (it's bypassing it)

### Case C: switching an existing bypass host to gateway-enforced

Not recommended. This breaks the CLI for Vault/ArgoCD (see [ADR-010](../adr/010-istio-native-oauth2-absent.md)). If this is ever needed, design host separation (Path B) as a separate effort.

## SSO behavior

### Single Sign-On (one login covers everything)

When Yu opens some UI for the first time in the morning:

1. `backstage.platform.yu-min3.com` (say) → oauth2-proxy detects no cookie → 302 to Keycloak
2. Keycloak login (Yu / password)
3. oauth2-proxy sets a cookie (cookie_domain `.platform.yu-min3.com`)
4. From then on, `prometheus.platform...`, `hubble.platform...`, `longhorn.platform...` all pass through on that one cookie (Grafana goes through its own native OIDC separately, but shares the same Keycloak session)

Opening `vault.platform...`:
1. The Vault UI sends the browser to "Login with OIDC"
2. Vault redirects to Keycloak → **the Keycloak session is still alive**, so the login screen is skipped → a token is issued
3. Into the Vault UI

In practice, this means Yu logs into Keycloak once each morning and every UI is accessible from there.

### Single Logout (and its limits)

Hitting oauth2-proxy's `/sign_out` clears the cookie and logs out of Category 2/3 UIs. **However**, Vault sessions and ArgoCD sessions **survive unless killed individually** (each holds its own token surface).

To log out completely:

- Kill the realm session from the Keycloak admin console (invalidates it for every app on its next refresh)
- Or log out of each app individually

At homelab scale, just closing the laptop or letting cookies expire naturally is operationally sufficient.

## Troubleshooting

### Symptom: `403 RBAC: access denied` on a Category 2/3 UI

Likely causes:
1. The user isn't in the right Keycloak group
    - Check with `kcadm.sh get groups -r kensan`
2. The Keycloak group-claim mapper isn't attaching to the ID token
    - Should be set up by the `ensure_oidc_client` function in `bootstrap/keycloak/setup.sh`
    - Inspect: check the actual id_token claims in the oauth2-proxy logs
3. A rule in the ALLOW policy is wrong
    - `kubectl -n istio-system get authorizationpolicy gateway-platform-allow -o yaml`
4. RequestAuthentication can't fetch JWKS
    - Check the istiod logs: `kubectl -n istio-system logs deploy/istiod | grep -i jwks`

### Symptom: 403 on a bypass host

The bypass host is probably missing from Category 1 of the ALLOW policy.

```bash
kubectl -n istio-system get authorizationpolicy gateway-platform-allow \
  -o jsonpath='{.spec.rules[0].to[0].operation.hosts}' | jq
```

### Symptom: can't reach oauth2-proxy, "no healthy upstream"

oauth2-proxy is down, or the Service / Endpoints in the `auth-system` namespace are broken.

```bash
kubectl -n auth-system get pod,svc,endpoints
kubectl -n auth-system logs -l app.kubernetes.io/name=oauth2-proxy -c oauth2-proxy --tail=50
```

Since the design uses `failOpen: false`, an oauth2-proxy outage means every Category 2/3 UI returns 503. Bypass hosts (Vault/ArgoCD/Keycloak/Grafana) are unaffected.

### Symptom: `vault login -method=oidc` suddenly stopped working

Possibly a broken bypass policy — e.g. a Phase 2 PR forgot to add `vault.platform` to Category 1 in the ALLOW policy, or accidentally removed it.

```bash
# Confirm the Vault host is allowed
kubectl -n istio-system get authorizationpolicy gateway-platform-allow \
  -o yaml | grep -A 5 vault
```

Emergency revert: `kubectl -n istio-system delete authorizationpolicy gateway-platform-oauth2-authz` (deleting the CUSTOM policy lifts oauth2-proxy enforcement and reverts every host to pass-through).

## Rollback procedure

To undo everything:

```bash
kubectl -n istio-system delete \
  authorizationpolicy/gateway-platform-allow \
  authorizationpolicy/gateway-platform-oauth2-authz \
  requestauthentication/gateway-platform-keycloak-jwt
```

If Argo CD selfHeal is ON, this comes back within seconds. To remove it permanently, delete the 3 YAML files from Git and push.

## Related ADRs

- [ADR-002](../adr/002-authentication-authorization-architecture.md): the platform's overall authentication approach
- [ADR-005](../adr/005-istio-native-oauth2.md): Istio native OAuth2 (status: Re-evaluation Required)
- [ADR-010](../adr/010-istio-native-oauth2-absent.md): the rationale for adopting Path A (oauth2-proxy ext_authz)
