# oauth2-proxy

ADR-010 で採択した Path A の実装。Istio Gateway (`gateway-platform`) が `*.platform.yu-min3.com` 配下の保護対象 host を `envoy_ext_authz_http` で oauth2-proxy に委譲し、OIDC redirect / cookie / refresh をすべて oauth2-proxy が担当する。

詳細な採択経緯と Path B/C の検討は [ADR-010](../adr/010-istio-native-oauth2-absent.md) を参照。

## 構成

| 項目 | 値 | 補足 |
|---|---|---|
| Chart | `oauth2-proxy/oauth2-proxy 10.4.3` (appVersion 7.15.2) | |
| Namespace | `auth-system` | |
| replica | 2 | failOpen=false 時の高可用性 |
| nodeSelector | `kubernetes.io/arch: amd64` | m4neo に固定。Pi5 群は OIDC redirect の負荷分散に不向き |
| Provider | `keycloak-oidc` | realm `kensan` |
| Cookie domain | `.platform.yu-min3.com` | 全 platform host で 1 session 共有 (SSO の核) |

## Architecture

```
[browser]
    ↓ HTTPS
[Istio gateway-platform]
    ↓ ext_authz (HTTP)
[oauth2-proxy] ←→ [Keycloak realm "kensan"]
    ↓ allow (200) or deny (302 to login)
[Istio backend route]
```

- Istio `meshConfig.extensionProviders[oauth2-proxy]` で `envoy_ext_authz_http` として登録 (`infrastructure/network/istio/istiod/values.yaml`)
- 認証成功時 oauth2-proxy が 200 OK を返してそのまま終了させる必要があるため `upstreams = ["static://200"]` を設定。Istio ext_authz は HTTP 200 のみ「allow」と認識する仕様 (202 だと deny 扱い)

## 失敗時の挙動

`failOpen: false` (Istio 側設定) なので、oauth2-proxy が落ちると保護対象 host (Hubble / Prometheus / Longhorn / Grafana 等) は 503 を返す。

- replica 2 + PodDisruptionBudget(`minAvailable: 1`) で軽減
- Pi 5 ノードへの分散は意図的に避けている: redirect / cookie 検証は AMD64 性能が活きる + Pi 5 のネットワーク不安定性 ([cilium-wifi-stability](../runbooks/cilium-wifi-stability.md)) を被ると認証全体が落ちる

## scope と claim

`scope = "openid email profile"`。`groups` は scope ではなく claim 名なので scope に含めない (Keycloak が `Invalid scopes: groups` で reject する)。

groups claim は `istio-gateway-platform` client に直付けした `oidc-group-membership-mapper` が `id.token.claim=true` で id_token に自動で乗せる。

## CLI 共存 (skip_jwt_bearer_tokens)

Bearer token を持ち込んだリクエスト (`vault login -method=oidc`、`argocd login --sso` 後の API call 等) は `skip_jwt_bearer_tokens = true` で素通り。oauth2-proxy の cookie session を二重に貼らない。

`extra_jwt_issuers = ["https://auth.platform.yu-min3.com/realms/kensan=istio-gateway-platform"]` で issuer を許可リストに登録。

## chart の罠 (実装メモ)

- chart は `image.registry` (default `quay.io`) を repository に prepend する。`image.repository` に `quay.io/...` と書くと double prefix になる。`oauth2-proxy/oauth2-proxy` だけ書くのが正解
- chart の top-level `securityContext:` は container 側にマップされる (chart values 仕様)。`fsGroup` は pod-level 専用なので `podSecurityContext` に置く必要がある
- `configFile` を渡すと `emailDomains` chart value は無視される (configFile 優先)。両方書く必要あり

## 関連

- [ADR-010](../adr/010-istio-native-oauth2-absent.md): Istio native OAuth2 不在の発見と Path A 採択
- [Gateway OIDC](./gateway-oidc.md): 全体運用ガイド
