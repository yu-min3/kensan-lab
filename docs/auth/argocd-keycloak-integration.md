# ArgoCD ↔ Keycloak OIDC integration (Path B)

Argo CD uses the Keycloak realm `kensan` as its IdP via direct OIDC integration, bypassing Dex. Same "app-native auth" pattern as Vault / Grafana.

## Components

| Element | Location | Notes |
|---|---|---|
| Keycloak client `argocd` | Created by `bootstrap/keycloak/setup.sh` | redirect URI: `https://argocd.platform.yu-min3.com/auth/callback` |
| client_secret | Vault KV `secret/security/argocd/oidc` | Seeded by bootstrap Terraform |
| Kubernetes Secret `argocd-oidc-secret` | Generated via ESO (ExternalSecret) | namespace: `argocd` |
| `argocd-cm`'s `oidc.config` | `kubernetes/argocd/values.yaml` | Referenced via `$argocd-oidc-secret:client-secret` |

## Key parts of values.yaml

```yaml
configs:
  cm:
    url: "https://argocd.platform.yu-min3.com"
    oidc.config: |
      name: Keycloak
      issuer: https://auth.yu-mins.com/realms/kensan
      clientID: argocd
      clientSecret: $argocd-oidc-secret:client-secret
      refreshTokenThreshold: 2m
      requestedScopes:
        - openid
        - profile
        - email
      requestedIDTokenClaims:
        groups:
          essential: true
  rbac:
    policy.csv: |
      g, platform-admin, role:admin
      g, platform-dev, role:readonly
    scopes: '[groups]'
```

## Handling scopes and claims

`requestedScopes` deliberately **does not include** `groups`. In Keycloak, `groups` is a claim name, not a scope — including it in scopes gets rejected with "Invalid scopes: groups".

An `oidc-group-membership-mapper` attached directly to the `argocd` client (created by `bootstrap/keycloak/setup.sh`) puts the `groups` claim onto the id_token. Argo CD picks it up by declaring `requestedIDTokenClaims` with `groups: { essential: true }`.

`refreshTokenThreshold: 2m` renews the token before Keycloak's five-minute
access/ID token lifetime ends. It does not make the bearer token itself
long-lived: the refresh token remains bounded by the realm's three-day idle and
30-day maximum SSO session. Without the threshold, Argo CD logs
`invalid session: token is expired` after the five-minute token expires and
starts another OIDC round trip.

## RBAC mapping

| Keycloak group | Argo CD role |
|---|---|
| `platform-admin` | `role:admin` |
| `platform-dev` | `role:readonly` |

The built-in admin user (`admin.enabled: true` by default) is kept as a break-glass account.

## Why Path B (direct integration)

Reasons for choosing this over going through Dex (Path A):

- Vault / Grafana already use the same "direct Keycloak integration" pattern. Putting Dex in front of Argo CD alone would break that consistency
- Going through Dex adds a second config hop (argocd → dex → keycloak), complicating debugging
- Argo CD depends on Dex internally, but for external IdP integration, pointing `oidc.config` directly at Keycloak means Dex never needs to run at all

## Related

- [Gateway OIDC](./gateway-oidc.md): OIDC authentication at the Istio Gateway (oauth2-proxy)
- [oauth2-proxy configuration](./oauth2-proxy.md)
- ADR-002: Authentication and Authorization Architecture
