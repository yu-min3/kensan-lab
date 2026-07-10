# Gateway OIDC (Path A) operations guide

Operations document for the mechanism that centralizes authentication/authorization for everything under `*.platform.yu-min3.com` at the Istio gateway-platform.

For the rationale behind the design and the path-selection debate, see ADR-005 / ADR-010. This document sticks to "**how the running configuration works, and the procedure for adding a new host**".

## TL;DR

| Auth style | Hosts | Treatment at the gateway |
|---|---|---|
| **app-native auth** (the app has its own OIDC or custom auth) | Vault, ArgoCD, Keycloak, oauth2-proxy itself, **Grafana** | **bypass** (the gateway passes traffic straight through) |
| **gateway-enforced OIDC** (the app has no auth, or trusts proxy headers) | Backstage, Prometheus, Hubble, Longhorn | **CUSTOM (oauth2-proxy ext_authz) + ALLOW (group claim check)** |

> **Why Grafana is a bypass**: Grafana misinterprets the `Authorization: Bearer ...` header added by oauth2-proxy as one of its own API keys and returns 403. So the gateway passes it through and authentication is delegated to Grafana's own Keycloak OIDC (`auth.generic_oauth`). Implemented as Category 1 in `authorizationpolicy-gateway-platform-allow.yaml`.

The only deciding axis is "**does the app bring its own auth?**" — not CLI vs UI. Vault serves CLI and UI from the same host, and Vault itself authenticates both.

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
            │              (Bearer JWT validation)
            │                     │
            │                     ▼
            │             ALLOW (groups claim)
            │                     │
            ▼                     ▼
       upstream app          upstream app
       (own auth)            (header trust)
```

## Host × group matrix

Corresponds to the rules in `kubernetes/network/istio/authorizationpolicy-gateway-platform-allow.yaml`:

| Host | Category | Allowed groups | Auth style |
|---|---|---|---|
| `auth.platform.yu-min3.com` | 1: bypass | (no claim check) | Keycloak itself |
| `oauth2-proxy.platform.yu-min3.com` | 1: bypass | (no claim check) | callback only, not a protected target |
| `vault.platform.yu-min3.com` | 1: bypass | (no claim check) | Vault native OIDC + Vault tokens |
| `argocd.platform.yu-min3.com` | 1: bypass | (no claim check) | ArgoCD native OIDC + its own JWTs |
| `grafana.platform.yu-min3.com` | 1: bypass | (no claim check) | Grafana native OIDC (`auth.generic_oauth`); bypass to avoid the Bearer misinterpretation |
| `backstage.platform.yu-min3.com` | 2: admin + dev | `platform-admin`, `platform-dev` | oauth2-proxy enforced |
| `prometheus.platform.yu-min3.com` | 2: admin + dev | `platform-admin`, `platform-dev` | oauth2-proxy enforced |
| `hubble.platform.yu-min3.com` | 3: admin only | `platform-admin` | oauth2-proxy enforced |
| `longhorn.platform.yu-min3.com` | 3: admin only | `platform-admin` | oauth2-proxy enforced |

## Resource layout

| File | Role |
|---|---|
| `kubernetes/network/istio/requestauthentication-gateway-platform.yaml` | Validates Bearer JWTs in the Authorization header against Keycloak, populates `request.auth.claims` |
| `kubernetes/network/istio/authorizationpolicy-gateway-platform-allow.yaml` | The allow rules. 3 rules cover every host; a host absent from here is implicitly denied |
| `kubernetes/network/istio/authorizationpolicy-gateway-platform-oauth2.yaml` | Enforces oauth2-proxy ext_authz (CUSTOM action) on Category 2/3 hosts only |
| `kubernetes/auth/oauth2-proxy/values.yaml` | oauth2-proxy's own Helm values — cookie domain, Keycloak client id, etc. |
| `kubernetes/network/istio/istiod/values.yaml` | Registers oauth2-proxy as an ext_authz target via `meshConfig.extensionProviders[oauth2-proxy]` |

## Checklist for adding a new host

### Case A: new UI app (no auth of its own; protect with oauth2-proxy)

Example: add `awx.platform.yu-min3.com`, open to admin + dev.

1. Add the host to the Category 2 rule's `hosts:` in `authorizationpolicy-gateway-platform-allow.yaml`
2. Also add it to `hosts:` in `authorizationpolicy-gateway-platform-oauth2.yaml` (bring it under CUSTOM)
3. Add an HTTPRoute on the app side (`gateway-platform` as parentRef)
4. Configure the app for **header-trust mode** (oauth2-proxy adds `X-Auth-Request-User` etc.). For Backstage that's the proxy auth provider. (Note: Grafana is bypass + native OIDC because of the Bearer misinterpretation — see Case B)

### Case B: new own-auth app (same family as Vault/ArgoCD)

Example: add `nexus.platform.yu-min3.com`, with OIDC configured in Nexus itself.

1. Add the host to the Category 1 rule's `hosts:` in `authorizationpolicy-gateway-platform-allow.yaml`
2. Create an OIDC client for nexus in Keycloak (same procedure as `bootstrap/keycloak/setup.sh`)
3. Point the app at Keycloak realm `kensan` as its SSO source
4. **Do not touch** the CUSTOM policy (it's a bypass)

### Case C: converting an existing bypass host to gateway-enforced

Not recommended. Vault/ArgoCD would break their CLIs (see [ADR-010](../adr/010-istio-native-oauth2-absent.md)). If truly needed, design host separation (Path B) separately.

## SSO behavior

### Single Sign-On (one login for everything)

Opening any UI for the first time in the morning:

1. `backstage.platform.yu-min3.com` (say) → oauth2-proxy sees no cookie → 302 to Keycloak
2. Keycloak login (username / password)
3. oauth2-proxy sets its cookie (cookie_domain `.platform.yu-min3.com`)
4. After that, `prometheus.platform...` `hubble.platform...` `longhorn.platform...` all pass with the single cookie (Grafana uses its own native OIDC but shares the same Keycloak session)

Opening `vault.platform...`:

1. Vault UI presents "Login with OIDC"
2. Vault redirects to Keycloak → **the Keycloak session is still alive**, so the login screen is skipped → token issued
3. You're in the Vault UI

In practice: **one Keycloak login in the morning opens every UI**.

### Single Logout (and its limits)

Hitting oauth2-proxy's `/sign_out` clears the cookie and logs you out of the Category 2/3 UIs. **However**, Vault and ArgoCD sessions **survive until killed individually** (each has its own token surface).

To cut everything:

- Kill the realm session in the Keycloak admin console (every app expires on its next refresh)
- Or log out of each app individually

At homelab scale, "shut the PC" or waiting for cookie expiry is a perfectly fine operational answer.

## Troubleshooting

### Symptom: `403 RBAC: access denied` on a Category 2/3 UI

Possible causes:

1. The user isn't in the Keycloak group
   - Check with `kcadm.sh get groups -r kensan`
2. The Keycloak group claim mapper isn't on the ID token
   - The `ensure_oidc_client` function in `bootstrap/keycloak/setup.sh` should have added it
   - Inspect: look at the actual id_token claims in the oauth2-proxy log
3. The ALLOW policy rule is wrong
   - `kubectl -n istio-system get authorizationpolicy gateway-platform-allow -o yaml`
4. RequestAuthentication can't fetch JWKS
   - istiod log: `kubectl -n istio-system logs deploy/istiod | grep -i jwks`

### Symptom: 403 on a bypass host

The bypass host may be missing from Category 1 of the ALLOW policy.

```bash
kubectl -n istio-system get authorizationpolicy gateway-platform-allow \
  -o jsonpath='{.spec.rules[0].to[0].operation.hosts}' | jq
```

### Symptom: "no healthy upstream" before reaching oauth2-proxy

oauth2-proxy is down, or the Service / Endpoints in the auth-system ns are broken.

```bash
kubectl -n auth-system get pod,svc,endpoints
kubectl -n auth-system logs -l app.kubernetes.io/name=oauth2-proxy -c oauth2-proxy --tail=50
```

The design is `failOpen: false`, so an oauth2-proxy outage → every Category 2/3 UI returns 503. Bypass hosts (Vault/ArgoCD/Keycloak/Grafana) are unaffected.

### Symptom: `vault login -method=oidc` suddenly fails

The bypass policy may be broken — e.g. a PR forgot to keep vault.platform in Category 1 of the ALLOW policy, or deleted it.

```bash
# Is the Vault host still allowed?
kubectl -n istio-system get authorizationpolicy gateway-platform-allow \
  -o yaml | grep -A 5 vault
```

Emergency revert: `kubectl -n istio-system delete authorizationpolicy gateway-platform-oauth2-authz` (removing CUSTOM lifts the oauth2-proxy enforcement and every host passes through again).

## Rollback

To undo everything:

```bash
kubectl -n istio-system delete \
  authorizationpolicy/gateway-platform-allow \
  authorizationpolicy/gateway-platform-oauth2-authz \
  requestauthentication/gateway-platform-keycloak-jwt
```

With ArgoCD selfHeal ON they come back within tens of seconds. To remove permanently, delete the 3 YAMLs from Git and push.

## Related ADRs

- [ADR-002](../adr/002-authentication-authorization-architecture.md): platform-wide authentication model
- [ADR-005](../adr/005-istio-native-oauth2.md): Istio native OAuth2 (status: Re-evaluation Required)
- [ADR-010](../adr/010-istio-native-oauth2-absent.md): rationale for adopting Path A (oauth2-proxy ext_authz)
