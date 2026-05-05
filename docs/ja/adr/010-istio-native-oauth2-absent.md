# ADR-010: Istio native oauth2 provider 不在の発見と Path 再選定

## ステータス

**決定済み: Path A — oauth2-proxy + `envoy_ext_authz_http`** (2026-05-05)。

本 ADR は ADR-005 の**アーキテクチャ前提**を置換した。ADR-005 が想定した「Istio 1.27 native `oauth2` extension provider」は実在しない。段階的アプローチと二段階認可モデル自体は ADR-005 のまま有効、Gateway 層 OIDC のメカニズムのみ再選定した。

### 決定根拠（2026-05-05 追加検討）

初版起票時には Path D (OIDC native アプリは per-service OIDC、Hubble/Prometheus/Longhorn/Alertmanager 等の OIDC 非対応アプリのみ oauth2-proxy で守る hybrid) も併記していたが、SSO 体験 / single-logout / 監査一元化 / 新アプリ追加時の運用コストを比較した結果、**failure-mode resilience を除く全軸で Path A (oauth2-proxy 一本) が優位**と判断。failure resilience は `replicaCount: 2` + `PodDisruptionBudget` で軽減可能。

CLI 経路 (`vault login -method=oidc` / `argocd login --sso`) は oauth2-proxy を通れないため per-service OIDC client を維持する。oauth2-proxy の `--skip-jwt-bearer-tokens` で事前認証済み Bearer token は素通り扱いになるため、browser 経路と CLI 経路が Path A 配下で共存する。

Path B (`EnvoyFilter` で `envoy.filters.http.oauth2`) は Envoy filter が公式に "currently under active development" と明記されており、Enterprise Platform Engineering reference を標榜する homelab には不適と判断。

Path C (per-service OIDC のみ) は OIDC 非対応アプリ (Hubble/Prometheus/Longhorn/Alertmanager) を LAN-only か ad-hoc basic auth で守ることになり、SSO 統合の看板に反するため不採用。

## 日付

2026-05-05

## コンテキスト

ADR-005（2026-05-03）は Gateway 層 OIDC を「Istio native `oauth2` extension provider」で実装すると決定した。OAuth2 Proxy 案を退けた理由として「Istio 1.27 が stable な native `oauth2` ext provider を供給」「Pod 追加なし」を挙げていた。

### 検証（2026-05-05）

実装着手にあたり Istio API repo の proto 定義を直接確認した:

```bash
curl -s https://raw.githubusercontent.com/istio/api/release-1.27/mesh/v1alpha1/config.proto \
  | grep -E "Provider [a-z]+ = [0-9]+;"
```

`MeshConfig.ExtensionProvider` の `oneof provider` で利用可能な type:

| field | 用途 |
|---|---|
| `envoy_ext_authz_http` | HTTP service による外部認可 (oauth2-proxy 等) |
| `envoy_ext_authz_grpc` | gRPC service による外部認可 |
| `zipkin` / `datadog` / `stackdriver` / `skywalking` / `opentelemetry` | tracing |
| `prometheus` | metrics |
| `envoy_file_access_log` / `envoy_http_als` / `envoy_tcp_als` | access log |
| `sds` | Secret Discovery Service |

**`oauth2` / `oidc` / `envoy_oauth2` 相当の type は存在しない**。`master`、`release-1.28`、`release-1.26` ブランチも同様。Istio 公式 reference docs (`istio.io/latest/docs/reference/config/istio.mesh.v1alpha1/`) も同じ集合。

### 「native oauth2」と取り違えられた可能性

おそらく以下のいずれかと混同:

1. **Envoy の `envoy.filters.http.oauth2` HTTP filter** — Envoy 本体に存在、`EnvoyFilter` CRD 経由で Istio に挿入可能。`MeshConfig.ExtensionProvider` ではない。Envoy docs では「currently under active development」と明記、OIDC discovery URL 非対応（auth/token endpoint を hardcode 必要）。
2. **`RequestAuthentication` + `AuthorizationPolicy`** — GA かつ stable だが、JWT *検証* と認可のみ。OIDC redirect / cookie flow を駆動する機能はない。JWT を持たない user を Keycloak に redirect することはこれらだけではできない。

いずれも OAuth2 Proxy の drop-in 代替にならない。

## 検討した Path

### Path A: oauth2-proxy + `envoy_ext_authz_http` （推奨）

oauth2-proxy を `auth-system` ns に置き、Istio `MeshConfig.ExtensionProvider` の `envoy_ext_authz_http` で接続。`AuthorizationPolicy` の `action: CUSTOM` で gateway-platform にバインド。OIDC redirect / cookie / refresh は oauth2-proxy が処理、Istio はゲートのみ。

- ✅ Istio docs が公式に文書化したパターン (External Authorization task)
- ✅ oauth2-proxy は K8s ecosystem 標準 (homelab references も大半が採用)
- ✅ OIDC discovery URL 対応、refresh token 安定、CSRF 対策内蔵
- ✅ `envoy_ext_authz_http` は GA proto
- ✅ "Enterprise Platform Engineering reference" の文脈では production-realistic な oauth2-proxy が合う
- ⚠️ Pod 1 個追加 (replica 2 + PDB で軽減)
- ⚠️ "Istio 内で auth 完結" 思想を裏切る

### Path B: `EnvoyFilter` で `envoy.filters.http.oauth2`

Istio の escape hatch (`EnvoyFilter`) で gateway-platform の listener に Envoy native oauth2 filter を挿入。Pod 追加なし。

- ✅ ADR-005 の「Istio 内で auth 完結」に近い
- ✅ refresh token / single logout 対応
- ❌ Envoy oauth2 filter が "currently under active development" と明記
- ❌ OIDC discovery 非対応 (Keycloak hostname 変更で drift)
- ❌ EnvoyFilter は Istio 最も brittle な API（minor upgrade で割れる事例多数）
- ❌ CSRF 対策別途必要、HTTPS 必須

### Path C: per-service OIDC

Gateway 層 OIDC 諦め。各 service の native OIDC（ArgoCD / Grafana / Backstage / Vault）に分散。Hubble / Prometheus / Longhorn UI は basic auth or LAN-only NetworkPolicy。

- ✅ 実装ゼロ、現状維持
- ✅ ADR-002 Phase 1 のまま
- ❌ SSO 体験が分散 (re-login per service)
- ❌ default-deny の fail-secure 利益なし
- ❌ 「Istio + Keycloak で SSO 完結」という homelab 差別化が消える

## 決定（保留）

**推奨: Path A**。理由:
- Path B の Envoy filter stability disclaimer (`under active development`) は production-aligned setup には実害リスク
- Path A の "Pod 追加" コストは Helm chart 1 個 + ns 1 個。homelab 文脈では小さい
- Path C は ADR-002/005 の SSO 統合方針を放棄
- ADR-005 が "Pod 追加なし" を要件ではなく結果（positive consequence）として書いていた点を考慮すると、根本要件（per-service OIDC を毎回実装しない統合 SSO）は Path A で完全に満たせる

**Yu の決定が、本 PR の path 別 YAML を Gateway にバインドする前に必要**。

## 本 PR の実装範囲

ADR-010 採択後の本 PR (Path A: oauth2-proxy via envoy_ext_authz_http) は以下を含む:

1. `bootstrap/keycloak/setup.sh` を OIDC client 関数化 + `istio-gateway-platform` client 作成を Path A 仕様で activate (`redirectUris=[https://oauth2-proxy.platform.yu-min3.com/oauth2/callback]`)。Vault KV 投入も同 script に bundle。
2. `infrastructure/network/istio/istiod/values.yaml` の `meshConfig.extensionProviders` に oauth2-proxy entry を登録。
3. `infrastructure/security/oauth2-proxy/` に Helm multi-source app (replicaCount 2 + PDB、ExternalSecret 経由 Vault KV)。
4. ADR-010 (本書) status を Decided: Path A に更新、ADR-005 を Re-evaluation Required に更新。

`RequestAuthentication` / `AuthorizationPolicy` (deny-all + ALLOW + CUSTOM) は後続 PR で投入。本 PR では provider 登録 + deploy のみで、AuthZ binding がない以上ランタイム挙動の変更はゼロ。

## 参照

- ADR-002, ADR-005
- [Istio API repo (release-1.27): config.proto](https://github.com/istio/api/blob/release-1.27/mesh/v1alpha1/config.proto)
- [Envoy: HTTP OAuth2 filter](https://www.envoyproxy.io/docs/envoy/latest/configuration/http/http_filters/oauth2_filter)
- [Istio: External Authorization](https://istio.io/latest/docs/tasks/security/authorization/authz-custom/)
- [oauth2-proxy: Istio integration](https://oauth2-proxy.github.io/oauth2-proxy/configuration/integration#configuring-for-use-with-the-istio-ingress-gateway)
