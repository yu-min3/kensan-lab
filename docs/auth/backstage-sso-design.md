# Backstage SSO implementation design

> **Superseded (2026-08-11):** This document records the original oauth2-proxy
> implementation. [ADR-022](../adr/022-backstage-native-oidc.md) replaces it
> with Backstage native OIDC after production validation exposed unnecessary
> coupling between Gateway identity headers and Backstage application tokens.

## 結論

Backstage 専用の OIDC client は作らず、既存の **Istio Gateway + oauth2-proxy + Keycloak** を認証入口として再利用し、Backstage の `oauth2Proxy` auth provider へ検証済み identity header を渡す。

今回の実装ゴールは「Backstage が `guest` ではなく `user:default/yu` として利用者を識別できること」と「production の危険な auth bypass を廃止すること」。Permission Framework による操作別 RBAC は identity 導入後の別フェーズとし、今回同時には有効化しない。

| 判定 | 方式 | 理由 |
|---|---|---|
| **採用** | oauth2-proxy header trust + Backstage proxy provider | 既存 SSO session を再利用でき、新しい client secret が不要。ADR-002 / ADR-010 と現行 Gateway 設計に一致する |
| **却下** | Backstage 専用 Keycloak OIDC client | 二重の OAuth callback・session・secret 運用を増やし、Gateway 集中認証の判断を覆す |
| **却下** | guest を維持し Gateway 認証だけ使う | 到達制御はできても Backstage 内の user identity、監査、将来の権限制御が成立しない |
| **後続** | Permission Framework + group-based RBAC | identity 導入と認可変更を同時に行うと障害時の切り分けが難しい。SSO 安定後に別変更で行う |

## 現状と問題

Gateway ではすでに Backstage の LAN / external 両 host に oauth2-proxy を強制し、Keycloak の `platform-admin` または `platform-dev` group のみ通している。認証成功時には `X-Auth-Request-User`、`X-Auth-Request-Email`、`X-Auth-Request-Groups` が upstream に渡る。

一方、Backstage 内部は次の暫定状態にある。

| 重要度 | 問題 | 根拠 | 影響 |
|---|---|---|---|
| 🔴 **Critical** | production で guest provider を許可 | `backstage/app/app-config.kubernetes.yaml` | 全利用者が同一 identity になり監査・個別認可ができない |
| 🔴 **Critical** | default backend auth policy を全体で無効化 | 同ファイルの `dangerouslyDisableDefaultAuthPolicy` | Gateway 到達制御と Backstage plugin API の認証境界が分離していない |
| 🟠 **High** | frontend が guest provider 固定 | `backstage/app/packages/app/src/App.tsx` | oauth2-proxy の identity を Backstage session に変換できない |
| 🟠 **High** | Catalog に実利用者がいない | `backstage/app/catalog/organizations/teams.yaml` | email resolver が `user:default/yu` を解決できない |
| 🟡 **Medium** | Permission Framework 無効 | production config | identity 導入後も操作別 RBAC は全員同一。今回の非目標として明示する |

## 目標アーキテクチャ

```text
Browser
  │  shared SSO cookie
  ▼
Istio gateway-platform
  │  ext_authz check
  ▼
oauth2-proxy ───────────────► Keycloak realm: kensan
  │  X-Auth-Request-Email: ymisaki00@gmail.com
  │  X-Auth-Request-Groups: platform-admin
  ▼
Backstage oauth2Proxy provider
  │  emailMatchingUserEntityProfileEmail
  ▼
Catalog User: user:default/yu
  │
  ▼
Backstage token / plugin API identity
```

信頼境界は二層に分ける。

1. Gateway は「この利用者が Backstage へ到達してよいか」を Keycloak group で判定する。
2. Backstage は検証済み email header を Catalog User に解決し、「誰が操作しているか」を表現する。

`X-Auth-Request-*` は署名付き credential ではないため、Backstage Service を直接公開しないこと、Gateway 以外から任意 header を注入できないことが前提になる。現在の ClusterIP、Gateway route、NetworkPolicy、Istio sidecar を境界として維持し、受入試験で直接到達経路がないことを確認する。

## Identity mapping

初期実装は email を安定キーとする。

| Source | 値 | Backstage |
|---|---|---|
| Keycloak `email` claim | `ymisaki00@gmail.com` | `spec.profile.email` と照合 |
| Keycloak username | `yu` | 表示・診断用。解決キーには使わない |
| Keycloak group | `platform-admin` / `platform-dev` | Gateway 到達制御に使用。今回 Backstage group へ自動同期しない |
| Catalog User | `user:default/yu` | Backstage identity の主体 |

`emailMatchingUserEntityProfileEmail` resolver を使い、Catalog に email が一致する実 User entity を静的に追加する。demo User 群は別変更で整理できるが、SSO cutover の必須条件ではない。

固定 user 名を header から直接発行する resolver や「Catalog entity がなくても sign-in を許す」方式は採用しない。Catalog が identity inventory の SoT となり、誤った email や未登録利用者は fail closed で sign-in 失敗になる。

## 変更設計

### Backstage application

| 対象 | 変更 |
|---|---|
| `packages/backend/package.json` | `@backstage/plugin-auth-backend-module-oauth2-proxy-provider` を同一 Backstage release line で追加。guest module は local development 専用として残す |
| `packages/backend/src/index.ts` | oauth2-proxy provider を登録。Istio ext_authz の `X-Auth-Request-*` を読む profile transform を追加 |
| `packages/app/src/App.tsx` | `guest` の自動 sign-in を `oauth2Proxy` に置換。Gateway login 済みなら追加 UI なしで Backstage session を確立 |
| `app-config.kubernetes.yaml` | `auth.providers.oauth2Proxy` と email resolver を設定し、production guest を削除 |
| `app-config.kubernetes.yaml` | `dangerouslyDisableDefaultAuthPolicy` を削除し、Backstage の既定 plugin auth policy を復元 |
| `catalog/organizations/teams.yaml` | `user:default/yu` を実 email と `platform-engineering` membership で追加 |

local development は Gateway header が存在しないため、`app-config.development.yaml` だけで guest provider を構成する。production image はこの config を読み込まず、frontend も production host では `ProxiedSignInPage` を使う。したがって guest module のコードが bundle に含まれても production の guest provider endpoint は作られない。

oauth2-proxy を Istio ext_authz の `/oauth2/auth` として使う場合、認証結果は `X-Auth-Request-Email` 等の**レスポンス**headerで返る。Backstage公式providerの既定profile transformはreverse proxy方式の `X-Forwarded-Email` を読むため、kensan-labでは公式authenticatorを再利用しつつ、profile transformだけを `X-Auth-Request-*` 用に差し替える。

### Platform manifests

Backstage の plugin API は sign-in 後に Backstage 自身が発行した Bearer token を使う。
共通 ext_authz provider が Keycloak token を `Authorization` に設定すると、この token を
上書きしてしまう。そのためKeycloak tokenは専用headerでGateway検証し、`Authorization`は
全hostで元のapplication tokenを保持する。

| 対象 | 判断 |
|---|---|
| oauth2-proxy Keycloak client / Secret | **変更なし**。既存 `istio-gateway-platform` client を共有 |
| Istio `headersToUpstreamOnAllow` | **identity headerのみ転送**。`Authorization`は元のapplication tokenを保持 |
| Istio `includeRequestHeadersInCheck` | **cookieでsession検証**。application `Authorization`はoauth2-proxyへ渡さない |
| Gateway JWT検証 | `X-Auth-Request-Access-Token` をJWKS検証し、既存のgroups許可を維持 |
| Backstage利用者 | Gatewayのadmin/dev許可に加え、Catalog User resolverでもallowlist |
| workload `RequestAuthentication` | **削除**。Keycloak tokenはGateway専用headerで検証し、workloadではBackstage tokenをbackendへ渡す |
| Backstage ExternalSecret | **変更なし**。専用 client secret は不要 |
| Backstage image | application build 後に新 tag へ更新。`latest` は使わない |

## リクエストフロー

1. Browser が Backstage を開く。
2. Istio が oauth2-proxy の `/oauth2/auth` へ ext_authz check を行う。
3. session がなければ oauth2-proxy が Keycloak へ redirect し、認証後に共有 cookie を設定する。
4. 共通 ext_authz providerがsessionを検証し、identity headerを上書きする。
5. Gatewayがaccess-token headerのgroupsを検証し、browserの`Authorization`は上書きしない。
6. Backstage oauth2Proxy provider が email header を Catalog User に照合し、Backstage token を発行する。
7. frontend と backend plugin は Backstage token で user identity を共有する。

## 段階導入

| Phase | 変更 | Gate | Rollback |
|---|---|---|---|
| 0 | Catalog に実 User entity を追加 | Catalog API で entity と email を確認 | entity 追加を revert |
| 1 | proxy provider を追加、production guest を置換 | `/api/auth/oauth2Proxy/refresh` が identity を返す | 直前 image tag + guest config に revert |
| 2 | default backend auth policy を復元 | Catalog / Search / Scaffolder / TechDocs の主要 API が成功 | 一時的に bypass 設定を戻すが、恒久運用しない |
| 3 | E2E と運用確認後に旧 guest dependency を除去 | 24 時間の通常利用で auth error なし | Phase 1 image へ戻す |

GitOps のため runtime 変更は Git commit と Argo CD sync を経由する。Application 名は変更せず、PostgreSQL/PVC に触れない。

## 受入基準

### 機能

- Keycloak session がある利用者は追加の login form なしで Backstage を開ける。
- User Settings と Backstage identity API が `user:default/yu` を返す。
- `ownershipEntityRefs` に `group:default/platform-engineering` が含まれる。
- Catalog、Search、Scaffolder、TechDocs、Notifications の代表操作が成功する。
- LAN host と Cloudflare Tunnel host の両方で同じ identity になる。

### セキュリティ

- 未認証 browser は Keycloak へ redirect される。
- Catalog Userに登録されていないemailはBackstage sign-in resolverで拒否される。
- production の guest endpoint で sign-in できない。
- email header がない、または Catalog に一致しない場合は sign-in が fail closed になる。
- 外部から `X-Auth-Request-Email` を偽装しても Gateway / oauth2-proxy が上書きまたは拒否し、別 user になれない。
- `dangerouslyDisableDefaultAuthPolicy` が production config に残っていない。
- Git に token、client secret、cookie secret を追加していない。

### 可用性・回帰

- oauth2-proxy outage 時は現在どおり fail closed で 503 になる。
- Backstage の health probe と内部 plugin-to-plugin 呼び出しが default auth policy 復元後も成功する。
- frontend の Backstage Bearer token が Gateway で上書きされず、plugin API に到達する。
- workload sidecarがBackstage tokenを未知のissuerとして拒否しない。
- restart と oauth2-proxy cookie refresh 後も Backstage session を再確立できる。

## Observability

認証失敗を次の境界で切り分ける。

| 症状 | 境界 | 見るもの |
|---|---|---|
| 302 loop / 503 | Gateway → oauth2-proxy | oauth2-proxy log、ext_authz metrics、cookie domain |
| 403 Gateway | Gateway AuthorizationPolicy | CUSTOM/ALLOW policy、host category |
| sign-in resolver error | Backstage auth backend | email header の有無、Catalog User email |
| plugin API 401 | Backstage backend auth | Backstage token、service-to-service auth、workload JWT policyの有無 |

認証 header の値や token 本文を通常ログへ出さない。診断時も email は最小限にし、access token / cookie / authorization header は記録しない。

## 非目標

- Keycloak group と Backstage Group entity の自動同期
- Permission Framework の RBAC policy 実装
- Backstage 専用 OIDC client / secret の新設
- Keycloak realm session policy の変更
- oauth2-proxy / Gateway 全体の認証方式変更

## Yu が決めるべき未決事項

現時点で実装を止める未決事項はない。初期 identity は既存 Keycloak user `yu` と email `ymisaki00@gmail.com` を `user:default/yu` に対応させる前提で進められる。

後続 RBAC 着手時には、`platform-admin` / `platform-dev` を Backstage Catalog group に同期する方式（静的管理、Keycloak catalog provider、独自同期）の選択が必要になる。

## 参照

- [Backstage: OAuth2 Proxy provider](https://backstage.io/docs/auth/oauth2-proxy/provider/)
- [Backstage: Sign-in identities and resolvers](https://backstage.io/docs/auth/identity-resolver/)
- [Backstage: Default auth policy](https://backstage.io/docs/auth/service-to-service-auth/)
- [ADR-002: Authentication and Authorization Architecture](../adr/002-authentication-authorization-architecture.md)
- [ADR-010: oauth2-proxy ext_authz](../adr/010-istio-native-oauth2-absent.md)
- [Gateway OIDC operation guide](gateway-oidc.md)
