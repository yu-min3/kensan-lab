# ArgoCD ↔ Keycloak OIDC integration (Path B)

ArgoCD は Keycloak realm `kensan` を IdP として、Dex を経由しない直接 OIDC 統合を採用している。Vault / Grafana と同じ "app native auth" pattern。

## 構成

| 要素 | 場所 | 補足 |
|---|---|---|
| Keycloak client `argocd` | `bootstrap/keycloak/setup.sh` で作成 | redirect URI: `https://argocd.platform.yu-min3.com/auth/callback` |
| client_secret | Vault KV `secret/security/argocd/oidc` | bootstrap TF で投入 |
| Kubernetes Secret `argocd-oidc-secret` | ESO (ExternalSecret) で生成 | namespace: `argocd` |
| `argocd-cm` の `oidc.config` | `infrastructure/gitops/argocd/values.yaml` | `$argocd-oidc-secret:client-secret` で参照 |

## values.yaml の主要部

```yaml
configs:
  cm:
    url: "https://argocd.platform.yu-min3.com"
    oidc.config: |
      name: Keycloak
      issuer: https://auth.platform.yu-min3.com/realms/kensan
      clientID: argocd
      clientSecret: $argocd-oidc-secret:client-secret
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

## scope と claim の取り扱い

`requestedScopes` に `groups` を**含めない**。Keycloak で `groups` は scope ではなく claim 名なので、scope に入れると "Invalid scopes: groups" で reject される。

`argocd` client に直付けした `oidc-group-membership-mapper` (`bootstrap/keycloak/setup.sh` で作成) が id_token に `groups` claim を乗せる。ArgoCD は `requestedIDTokenClaims` で `groups: { essential: true }` を指定し受け取る。

## RBAC mapping

| Keycloak group | ArgoCD role |
|---|---|
| `platform-admin` | `role:admin` |
| `platform-dev` | `role:readonly` |

既存 admin user (`admin.enabled: true` 既定) は break-glass として残す。

## なぜ Path B (直接統合) か

Dex 経由 (Path A) と比べた採択理由:

- Vault / Grafana が同じ "Keycloak 直接統合" pattern を採用済み。ArgoCD だけ Dex を挟むと統一感がない
- Dex 経由は config が二段になる ((argocd → dex → keycloak) のため debug が複雑
- ArgoCD は Dex を内部依存しているが、外部 IdP との統合では `oidc.config` で直接 Keycloak を向ければ Dex を起動しなくて済む

## 関連

- [Gateway OIDC](./gateway-oidc.md): Istio Gateway 側の OIDC 認証 (oauth2-proxy)
- [oauth2-proxy 構成](./oauth2-proxy.md)
- ADR-002: Authentication and Authorization Architecture
