# Gateway OIDC (Path A) 運用ガイド

`*.platform.yu-min3.com` 配下の認証/認可を Istio gateway-platform で集中制御する仕組みのオペレーションドキュメント。

設計判断の根拠と path 選定の議論は ADR-005 / ADR-010 を参照。本書は「**今動いてる構成の解説と、新 host を追加するときの作業手順**」に絞る。

## TL;DR

| 認証方式 | どの host が該当 | gateway 層での扱い |
|---|---|---|
| **app-native auth** (app が自前で OIDC や独自認証を持つ) | Vault, ArgoCD, Keycloak, oauth2-proxy 自身 | **bypass** (gateway は素通し) |
| **gateway-enforced OIDC** (app は認証を持たない or proxy header を信頼) | Backstage, Grafana, Prometheus, Hubble, Longhorn | **CUSTOM (oauth2-proxy ext_authz) + ALLOW (group claim 検査)** |

判定軸は「**アプリが自前で auth を持ってるか**」だけ。CLI/UI の区別ではない。Vault は CLI も UI も同じ host を共有しており、Vault 自身が両方を認証する。

## アーキテクチャ概要

```
                    Internet
                       │
                       ▼
              gateway-platform (Istio)
                       │
            ┌──────────┴──────────┐
            ▼                     ▼
     [bypass hosts]         [protected hosts]
     vault / argocd /        backstage / grafana /
     auth / oauth2-proxy     prometheus / hubble /
            │               longhorn
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
            │              (Bearer JWT 検証)
            │                     │
            │                     ▼
            │             ALLOW (groups claim)
            │                     │
            ▼                     ▼
       upstream app          upstream app
       (自前 auth)          (header trust)
```

## Host × Group マトリクス

`infrastructure/network/istio/authorizationpolicy-gateway-platform-allow.yaml` の rule に対応:

| Host | Category | 許可 group | 認証方式 |
|---|---|---|---|
| `auth.platform.yu-min3.com` | 1: bypass | (claim 検査なし) | Keycloak 自身 |
| `oauth2-proxy.platform.yu-min3.com` | 1: bypass | (claim 検査なし) | callback 専用、保護対象外 |
| `vault.platform.yu-min3.com` | 1: bypass | (claim 検査なし) | Vault native OIDC + Vault token |
| `argocd.platform.yu-min3.com` | 1: bypass | (claim 検査なし) | ArgoCD native OIDC + 自前 JWT |
| `backstage.platform.yu-min3.com` | 2: admin + dev | `platform-admin`, `platform-dev` | oauth2-proxy 強制 |
| `grafana.platform.yu-min3.com` | 2: admin + dev | `platform-admin`, `platform-dev` | oauth2-proxy 強制 |
| `prometheus.platform.yu-min3.com` | 2: admin + dev | `platform-admin`, `platform-dev` | oauth2-proxy 強制 |
| `hubble.platform.yu-min3.com` | 3: admin only | `platform-admin` | oauth2-proxy 強制 |
| `longhorn.platform.yu-min3.com` | 3: admin only | `platform-admin` | oauth2-proxy 強制 |

## リソースレイアウト

| ファイル | 役割 |
|---|---|
| `infrastructure/network/istio/requestauthentication-gateway-platform.yaml` | Authorization header の Bearer JWT を Keycloak で検証、`request.auth.claims` を populate |
| `infrastructure/network/istio/authorizationpolicy-gateway-platform-allow.yaml` | 許可ルール本体。3 rule で全 host を網羅。ここに無い host は暗黙 deny |
| `infrastructure/network/istio/authorizationpolicy-gateway-platform-oauth2.yaml` | Category 2/3 host にだけ oauth2-proxy ext_authz を強制 (CUSTOM action) |
| `infrastructure/security/oauth2-proxy/values.yaml` | oauth2-proxy 自身の Helm values。cookie domain / Keycloak client id 等 |
| `infrastructure/network/istio/istiod/values.yaml` | `meshConfig.extensionProviders[oauth2-proxy]` で oauth2-proxy を ext_authz target として登録 |

## 新 host を追加するときのチェックリスト

### Case A: 新 UI app (自前 auth なし、oauth2-proxy で守りたい)

例: `awx.platform.yu-min3.com` を追加、admin + dev に開放したい。

1. `authorizationpolicy-gateway-platform-allow.yaml` の Category 2 rule の `hosts:` に追記
2. `authorizationpolicy-gateway-platform-oauth2.yaml` の `hosts:` にも追記 (CUSTOM の対象に入れる)
3. HTTPRoute をアプリ側に追加 (`gateway-platform` を parentRef)
4. アプリの認証は **header trust モード** に設定 (oauth2-proxy が `X-Auth-Request-User` 等を付ける)。Grafana なら `auth.proxy`、Backstage なら proxy auth provider 等。

### Case B: 新自前-auth app (Vault/ArgoCD と同類)

例: `nexus.platform.yu-min3.com` を追加、Nexus 自身に OIDC 設定済み。

1. `authorizationpolicy-gateway-platform-allow.yaml` の Category 1 rule の `hosts:` に追記
2. Keycloak に nexus 用 OIDC client を作る (bootstrap/keycloak/setup.sh と同様の手順)
3. アプリ側で Keycloak realm `kensan` を SSO source に設定
4. CUSTOM policy には**触らない** (bypass のため)

### Case C: 既存 bypass host を gateway-enforced に切り替えたい

非推奨。Vault/ArgoCD は CLI が壊れる ([ADR-010](../adr/010-istio-native-oauth2-absent.md) 参照)。やるなら host 分離 (Path B) を別途設計。

## SSO の挙動

### Single Sign-On (login 1 回で全部)

朝 Yu が初めて何らかの UI を開く:

1. `grafana.platform.yu-min3.com` (例) → oauth2-proxy が cookie なし検出 → Keycloak へ 302
2. Keycloak login (Yu / password)
3. oauth2-proxy が cookie set (cookie_domain `.platform.yu-min3.com`)
4. その後、`backstage.platform...` `prometheus.platform...` `hubble.platform...` 全部 cookie 1 個で透過

`vault.platform...` を開くと:
1. Vault UI が「Login with OIDC」に飛ばす
2. Vault が Keycloak へ redirect → **Keycloak の session が活きてる** ので login 画面スキップ → token 発行
3. Vault UI 入れる

つまり Yu の体感では **朝 1 回 Keycloak login すれば全 UI に通る**。

### Single Logout (の限界)

oauth2-proxy `/sign_out` を叩くと cookie が消えて Category 2/3 の UI から落ちる。**ただし** Vault session / ArgoCD session は **個別に kill しないと残る** (それぞれ独自の token surface を持つため)。

完全に切るには:
- Keycloak admin console で realm session kill (全アプリの次回 refresh で失効)
- または各アプリで個別 logout

homelab 規模では「PC 落とす」「次回 cookie 期限切れ」を待つ運用で十分。

## トラブルシュート

### 症状: Category 2/3 の UI で `403 RBAC: access denied`

考えられる原因:
1. Keycloak group に user が入ってない
   - `kcadm.sh get groups -r kensan` で確認
2. Keycloak の group claim mapper が ID token に乗ってない
   - `bootstrap/keycloak/setup.sh` の `ensure_oidc_client` 関数で付与してるはず
   - 検査: oauth2-proxy ログで実際の id_token の claim を見る
3. ALLOW policy の rule が間違ってる
   - `kubectl -n istio-system get authorizationpolicy gateway-platform-allow -o yaml`
4. RequestAuthentication が JWKS を取れてない
   - istiod ログ: `kubectl -n istio-system logs deploy/istiod | grep -i jwks`

### 症状: bypass host で 403

bypass host が ALLOW policy の Category 1 に入ってない可能性。

```bash
kubectl -n istio-system get authorizationpolicy gateway-platform-allow \
  -o jsonpath='{.spec.rules[0].to[0].operation.hosts}' | jq
```

### 症状: oauth2-proxy へ届かず "no healthy upstream"

oauth2-proxy が落ちてる、または auth-system ns の Service / Endpoints が壊れてる。

```bash
kubectl -n auth-system get pod,svc,endpoints
kubectl -n auth-system logs -l app.kubernetes.io/name=oauth2-proxy -c oauth2-proxy --tail=50
```

`failOpen: false` の設計なので oauth2-proxy 落下 → Category 2/3 の UI は全部 503。bypass host (Vault/ArgoCD/Keycloak) は影響なし。

### 症状: `vault login -method=oidc` が失敗するようになった

bypass policy が壊れてる可能性。Phase 2 PR が ALLOW policy で vault.platform を Category 1 に入れ忘れた / 削除されたなど。

```bash
# Vault host が ALLOW されてるか確認
kubectl -n istio-system get authorizationpolicy gateway-platform-allow \
  -o yaml | grep -A 5 vault
```

緊急 revert: `kubectl -n istio-system delete authorizationpolicy gateway-platform-oauth2-authz` (CUSTOM を消すと oauth2-proxy 強制が外れて全 host pass through に戻る)。

## ロールバック手順

すべて取り消したい場合:

```bash
kubectl -n istio-system delete \
  authorizationpolicy/gateway-platform-allow \
  authorizationpolicy/gateway-platform-oauth2-authz \
  requestauthentication/gateway-platform-keycloak-jwt
```

ArgoCD selfHeal が ON なら数十秒で復活する。完全に消すには Git から 3 つの YAML を削除して push。

## 関連 ADR

- [ADR-002](../adr/002-authentication-authorization-architecture.md): platform 全体の認証方式
- [ADR-005](../adr/005-istio-native-oauth2.md): Istio native OAuth2 (status: Re-evaluation Required)
- [ADR-010](../adr/010-istio-native-oauth2-absent.md): Path A (oauth2-proxy ext_authz) 採択の根拠
