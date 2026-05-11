# auth

人間 (Yu) / service の認証認可。

## コンポーネント

| dir | 役割 | 配置 ns |
|---|---|---|
| `keycloak/` | OIDC IdP。realm / user / client を提供 | `platform-auth-prod` |
| `oauth2-proxy/` | Istio Gateway の ext_authz。Cookie / JWT を Keycloak で検証 | `auth-system` |
| `vault-oidc-auth/` | Vault に Keycloak OIDC mount を生成 (VCO 経由)。Vault CLI / UI の SSO 経路 | `vault` |

## SSO フロー (Browser → 内部 platform UI)

```mermaid
sequenceDiagram
    autonumber
    participant U as Browser
    participant GW as Istio Gateway<br/>(gateway-platform)
    participant O2P as oauth2-proxy<br/>(ext_authz)
    participant KC as Keycloak
    participant APP as Backstage / Grafana<br/>/ ArgoCD UI etc.

    U->>GW: HTTPS /
    GW->>O2P: ext_authz check
    alt Cookie 無
        O2P-->>U: 302 → Keycloak /auth
        U->>KC: login (username/password or pkce)
        KC-->>U: 302 → /oauth2/callback?code=...
        U->>O2P: GET /oauth2/callback?code=...
        O2P->>KC: code → token 交換
        O2P-->>U: Set-Cookie + 302 → 元 URL
    end
    U->>GW: 元 URL (Cookie あり)
    GW->>O2P: ext_authz check (Cookie OK)
    GW->>APP: forward + X-Auth-Request-* header
    APP-->>U: 200
```

## CLI / UI 別経路 (Vault)

Vault は `platform-auth-prod` の Keycloak に対し独自 OIDC client (`vault`) を持ち、`oauth2-proxy` を経由しない:

```
yu (local) → vault login -method=oidc role=admin
            └─ browser 開く → Keycloak で認証 → callback で Vault token 返却
```

## 関連

- ADR-002 (auth architecture), ADR-005 (Istio native OAuth2)
- 「Istio Native ext_authz じゃなく oauth2-proxy を選んだ理由」: ADR-010
