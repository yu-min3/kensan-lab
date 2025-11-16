# ADR-002: 認証認可アーキテクチャの段階的実装

## ステータス

採用済み (Accepted)

## 日付

2025-11-08

## コンテキスト

プラットフォームインフラサービス（Backstage、Argo CD、Grafana、Prometheus、Hubble UI、Keycloakなど）へのアクセス制御を実装する必要がある。

### 要件

1. **段階的な導入**: 開発初期は無認証で、徐々に認証・認可を強化
2. **統合認証基盤**: Keycloakを中心とした統一的な認証体験
3. **サービス特性への対応**: SPA、API、UIのみなど、各サービスの特性に応じた認証方式
4. **将来的な拡張性**: 細かい権限制御（RBAC）への対応
5. **運用性**: 新しいサービス追加時の認証設定が容易

### プラットフォームサービス一覧

| サービス | URL | サービス特性 | 認可の必要性 |
|---------|-----|------------|------------|
| Keycloak (Prod) | keycloak.platform.your-org.com | 認証サーバー | なし（自己認証） |
| Keycloak (Dev) | keycloak-dev.platform.your-org.com | 認証サーバー | なし（自己認証） |
| Backstage | backstage.platform.your-org.com | SPA + API | 高（Permission Framework） |
| Argo CD | argocd.platform.your-org.com | UI + API | 高（RBAC） |
| Grafana | grafana.platform.your-org.com | UI + API | 中（Role Mapping） |
| Prometheus | prometheus.platform.your-org.com | UI + API | 低（全員同じ権限でOK） |
| Hubble | hubble.platform.your-org.com | UI | 低（全員同じ権限でOK） |

### 検討した認証パターン

#### パターンA: サービス個別認証

各サービスが独自にOIDC/OAuth2を実装

```
ブラウザ → Backstage (OIDC) → Keycloak
ブラウザ → Argo CD (OIDC) → Keycloak
ブラウザ → Grafana (OAuth2) → Keycloak
ブラウザ → Prometheus (認証なし)
```

**メリット:**
- 各サービスのネイティブ機能を最大限活用
- サービス単位で障害が分離

**デメリット:**
- 各サービスでOIDC設定が必要（運用負荷）
- Prometheus/Hubbleなど、OIDC非対応サービスは保護できない
- 新サービス追加時に都度設定が必要

#### パターンB: Gateway一括認証（OAuth2 Proxy）

Istio GatewayでOAuth2 Proxyを使った一括認証（AWS ALB + Cognito風）

```
ブラウザ → Istio Gateway → OAuth2 Proxy → Keycloak
                              ↓ 認証済み
                         すべてのサービス
```

**メリット:**
- 一箇所で認証管理（運用が容易）
- すべてのサービスが自動的に保護される
- 新サービス追加時も設定不要
- Prometheusなど非OIDC対応サービスも保護可能
- 1回のログインで全サービスにアクセス可能（Cookie共有）

**デメリット:**
- 各サービスのネイティブ認証機能は使えない
- 細かい認可制御はアプリ側で実装が必要
- OAuth2 Proxyが単一障害点（冗長化必要）

#### パターンC: ハイブリッド（Gateway + サービス個別）

Gateway認証をベースに、必要なサービスのみ追加の認証・認可を実装

```
ブラウザ → Istio Gateway → OAuth2 Proxy（認証Layer 1）
                              ↓ JWT付与
                         ┌────┴────┐
                    Backstage    Prometheus
                         ↓            ↓
                    OIDC検証     そのまま信頼
                  Permission      （認証済み）
                  Framework
                  （認可Layer 2）
```

**メリット:**
- パターンBのメリットを全て享受
- 必要なサービスのみ細かい認可制御を追加可能
- 段階的な導入が容易（Phase 2 → Phase 3）
- OAuth2 ProxyがJWTを転送するため、サービス側で再利用可能

**デメリット:**
- 最も複雑（ただし段階的に構築可能）

## 決定事項

**パターンC: ハイブリッド認証（Gateway + サービス個別）を採用する**

### 段階的実装フェーズ

#### Phase 1: 無認証（現状）

```yaml
状態: すべてのサービスが無認証でアクセス可能
期間: 開発初期
用途: プラットフォーム構築と動作確認
```

#### Phase 2: Gateway一括認証（OAuth2 Proxy）

```
ブラウザ → Istio Gateway (gateway-platform)
              ↓
         OAuth2 Proxy (ExtAuthz) ←→ Keycloak
              ↓ 認証済み（JWT付与）
         ┌────┴────┬─────────┬──────────┐
    Backstage  Prometheus  Argo CD  Grafana
         ↓          ↓         ↓        ↓
    ヘッダー    ヘッダー   ヘッダー  ヘッダー
    読み取り    読み取り   読み取り  読み取り
```

**実装内容:**
- OAuth2 Proxyデプロイ（auth-system namespace）
- Istio ExtAuthz設定（EnvoyFilter）
- Keycloak Realm `platform` 作成
- OAuth2 Proxy用Client作成
- 基本的なRole/Group設定（platform-admin）

**効果:**
- ✅ すべてのサービスが自動的に保護される
- ✅ 1回のログインで全サービスアクセス可能
- ✅ 運用負荷が最小

**各サービスの対応:**
- OAuth2 Proxyから `Authorization: Bearer <JWT>` ヘッダーを受け取る
- `X-Auth-Request-User`, `X-Auth-Request-Email` ヘッダーを信頼
- 認可は行わない（全員が全機能にアクセス可能）

#### Phase 3: 多層認証（Gateway + サービス個別認可）

```
ブラウザ → Istio Gateway → OAuth2 Proxy（認証Layer 1）
                              ↓ JWT付与
         ┌────────────────────┴─────────────────┐
    Backstage                            Prometheus
         ↓                                    ↓
    Proxy Provider                       変更なし
    （JWT検証）                          （Phase 2のまま）
         ↓
    Permission Framework
    （認可Layer 2）
         ↓
    細かい権限制御
    - カタログ読み取り: 全員
    - テンプレート実行: Developers
    - カタログ削除: Platform Engineers
```

**実装内容（サービス毎に段階的に）:**

1. **Backstage**:
   ```typescript
   // auth-backend-module-proxy-provider 追加
   // Permission Framework 有効化
   // Keycloak groupsクレームに基づくRBAC
   ```

2. **Argo CD**:
   ```yaml
   # OIDC設定追加
   # JWT検証
   # policy.csv でロールマッピング
   ```

3. **Grafana**:
   ```yaml
   # Generic OAuth設定
   # role_attribute_path でロールマッピング
   ```

4. **Prometheus/Hubble**:
   - 変更なし（Phase 2のOAuth2 Proxyのみで十分）

**効果:**
- ✅ OAuth2 Proxyの設定は変更不要
- ✅ 必要なサービスのみ細かい認可を追加
- ✅ JWT検証が2回行われる（多層防御）
- ✅ サービス毎に段階的に移行可能

### 認証方式の使い分け

| サービス | Phase 2 | Phase 3 | 理由 |
|---------|---------|---------|------|
| **Backstage** | OAuth2 Proxy | OAuth2 Proxy + Proxy Provider + Permission Framework | SPA、細かい権限必要 |
| **Argo CD** | OAuth2 Proxy | OAuth2 Proxy + OIDC + RBAC | ネイティブRBACサポート良好 |
| **Grafana** | OAuth2 Proxy | OAuth2 Proxy + Generic OAuth + Role Mapping | ロールマッピング必要 |
| **Prometheus** | OAuth2 Proxy | **変更なし** | OIDC非対応、全員同じ権限でOK |
| **Hubble** | OAuth2 Proxy | **変更なし** | OIDC非対応、全員同じ権限でOK |

## 理由

### ハイブリッド方式を選択した理由

#### 1. 段階的導入が容易

**Phase 1 → Phase 2:**
```bash
# OAuth2 Proxyをデプロイするだけ
kubectl apply -f base-infra/oauth2-proxy/
kubectl apply -f base-infra/istio/oauth2-proxy-extauthz.yaml

# すべてのサービスが自動的に保護される
# 各サービスの設定変更は不要
```

**Phase 2 → Phase 3:**
```bash
# サービス毎に段階的に移行
# 1. Backstageだけ先に移行
kubectl apply -f base-infra/backstage/
# OAuth2 Proxyは変更不要

# 2. 次にArgo CD
kubectl apply -f base-infra/argocd/
# OAuth2 Proxyは変更不要

# Prometheus/Hubbleは移行不要（Phase 2のまま）
```

#### 2. 運用負荷の最小化

- **Phase 2**: 一箇所（OAuth2 Proxy）で認証管理
- **Phase 3**: 必要なサービスのみ追加設定
- 新サービス追加時は自動的にOAuth2 Proxyで保護される

#### 3. 多層防御の実現

```
Layer 1（Gateway）: OAuth2 Proxy
  - 無効なJWTを早期に排除
  - すべてのサービスに対する基本的な保護

Layer 2（Application）: サービス個別認可
  - JWTを再検証
  - 細かいリソースレベルの権限制御
  - Backstage Permission Framework
  - Argo CD RBAC
```

#### 4. JWTの再利用

OAuth2 Proxyは以下の設定でJWTを下流に転送:

```yaml
args:
- --pass-access-token=true
- --pass-authorization-header=true
- --set-authorization-header=true
```

各サービスは `Authorization: Bearer <JWT>` ヘッダーを受け取り:
- Phase 2: ヘッダーを読み取るだけ（検証しない）
- Phase 3: JWTを検証 + groupsクレームで認可

**OAuth2 Proxyは邪魔にならない** → JWTをパスするだけ

#### 5. サービス特性への対応

| サービス特性 | 対応方法 |
|------------|----------|
| **OIDC対応（Backstage/Argo CD/Grafana）** | Phase 3で個別OIDC実装 |
| **OIDC非対応（Prometheus/Hubble）** | Phase 2のOAuth2 Proxyのみ |
| **細かい認可必要（Backstage/Argo CD）** | Phase 3でPermission Framework/RBAC |
| **認可不要（Prometheus/Hubble）** | Phase 2のまま |

### パターンA（サービス個別認証）を見送った理由

1. **Prometheus/Hubble が保護できない**
   - OIDC実装がない
   - Basic認証では不十分

2. **運用負荷が高い**
   - 各サービスでOIDC設定が必要
   - 新サービス追加時に都度設定

3. **ユーザー体験が悪い**
   - サービス毎にリダイレクトの可能性
   - Cookie/Sessionが共有されない

### パターンB（Gateway認証のみ）を見送った理由

Phase 2ではパターンBを採用するが、将来的に以下の課題がある:

1. **細かい認可ができない**
   - Backstageで「テンプレート実行は開発者のみ」などの制御が不可能
   - Argo CDで「本番デプロイはPlatform Engineersのみ」などの制御が不可能

2. **各サービスの機能を活かせない**
   - Backstage Permission Frameworkが使えない
   - Argo CD RBACが使えない

→ **Phase 3でハイブリッド化することで解決**

## 実装詳細

### Keycloak Realm設計

```yaml
Realm: platform

Clients:
  - oauth2-proxy-client (Confidential, Standard Flow)
    # OAuth2 Proxy用
    # Redirect URI: https://auth.platform.your-org.com/oauth2/callback

  - backstage-client (Confidential, Standard Flow) # Phase 3で使用
  - argocd-client (Confidential, Standard Flow)    # Phase 3で使用
  - grafana-client (Confidential, Standard Flow)   # Phase 3で使用

Roles:
  - platform-admin     # すべてのサービスにフルアクセス
  - platform-developer # 限定的なアクセス
  - platform-viewer    # 読み取りのみ

Groups:
  - platform-engineers
    roles: [platform-admin]

  - app-developers
    roles: [platform-developer]

  - viewers
    roles: [platform-viewer]

Users:
  - admin@example.com
    groups: [platform-engineers]
```

### OAuth2 Proxy設定（Phase 2）

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: oauth2-proxy
  namespace: auth-system
spec:
  containers:
  - name: oauth2-proxy
    image: quay.io/oauth2-proxy/oauth2-proxy:v7.5.1
    args:
    # OIDC Provider
    - --provider=oidc
    - --oidc-issuer-url=https://keycloak.platform.your-org.com/realms/platform
    - --client-id=oauth2-proxy-client
    - --client-secret=$(CLIENT_SECRET)

    # Cookie設定（全サブドメインで共有）
    - --cookie-name=_oauth2_proxy
    - --cookie-secure=true
    - --cookie-domain=.platform.your-org.com

    # JWT転送（Phase 3で使用）
    - --pass-access-token=true
    - --pass-authorization-header=true
    - --set-authorization-header=true

    # 認証設定
    - --email-domain=*
    - --skip-provider-button=true
```

### Istio ExtAuthz設定（Phase 2）

```yaml
# Istio ConfigMap
extensionProviders:
- name: oauth2-proxy
  envoyExtAuthzHttp:
    service: oauth2-proxy.auth-system.svc.cluster.local
    port: 4180
    pathPrefix: /oauth2/auth
    headersToUpstreamOnAllow:
    - authorization
    - x-auth-request-user
    - x-auth-request-email
    - x-auth-request-access-token

---
# AuthorizationPolicy（すべてのリクエストを認証）
apiVersion: security.istio.io/v1
kind: AuthorizationPolicy
metadata:
  name: gateway-platform-oauth2
  namespace: istio-system
spec:
  selector:
    matchLabels:
      gateway.networking.k8s.io/gateway-name: gateway-platform
  action: CUSTOM
  provider:
    name: oauth2-proxy
  rules:
  - {}  # すべてのリクエスト

---
# 例外パス（認証スキップ）
apiVersion: security.istio.io/v1
kind: AuthorizationPolicy
metadata:
  name: gateway-platform-skip-auth
  namespace: istio-system
spec:
  action: ALLOW
  rules:
  # OAuth2 Proxyコールバック
  - to:
    - operation:
        hosts: ["auth.platform.your-org.com"]
        paths: ["/oauth2/*"]

  # Keycloak（認証サーバー自体）
  - to:
    - operation:
        hosts:
        - "keycloak.platform.your-org.com"
        - "keycloak-dev.platform.your-org.com"
```

### Backstage設定（Phase 3）

```yaml
# app-config.kubernetes.yaml
auth:
  environment: production
  providers:
    # OAuth2 Proxyからのヘッダーを信頼
    proxy:
      signIn:
        resolvers:
        - resolver: forwardedUserMatchingUserEntityEmail

permission:
  enabled: true  # Phase 3で有効化
```

```typescript
// packages/backend/src/plugins/permission.ts
export class PlatformPermissionPolicy implements PermissionPolicy {
  async handle(request, user): Promise<PolicyDecision> {
    const groups = user?.identity.ownershipEntityRefs || [];

    // Platform Engineers: すべて許可
    if (groups.includes('group:default/platform-engineers')) {
      return { result: AuthorizeResult.ALLOW };
    }

    // App Developers: 制限付きアクセス
    if (groups.includes('group:default/app-developers')) {
      if (
        request.permission === catalogEntityReadPermission ||
        request.permission.id === 'scaffolder.template.execute'
      ) {
        return { result: AuthorizeResult.ALLOW };
      }
      return { result: AuthorizeResult.DENY };
    }

    return { result: AuthorizeResult.DENY };
  }
}
```

## 結果

### 認証フロー（Phase 2）

```
1. ユーザー → https://backstage.platform.your-org.com

2. Istio Gateway → OAuth2 Proxyに認証確認
   GET /oauth2/auth
   Headers: Cookie, X-Forwarded-*

3-A. Cookie有効（認証済み）:
   OAuth2 Proxy → 200 OK
   Headers:
     Authorization: Bearer <JWT>
     X-Auth-Request-User: admin@example.com
     X-Auth-Request-Email: admin@example.com

   → Backstageにリクエスト転送
   → Backstage: ヘッダー読み取り（検証なし）
   → レスポンス返却

3-B. Cookie無効（未認証）:
   OAuth2 Proxy → 302 Redirect
   Location: https://keycloak.platform.your-org.com/realms/platform/...

   → Keycloakでログイン
   → OAuth2 Proxyがトークン取得
   → Cookie設定
   → 元のURLにリダイレクト
   → 3-Aのフローへ
```

### 認証フロー（Phase 3 - Backstage例）

```
1. ユーザー → https://backstage.platform.your-org.com

2. OAuth2 Proxy（Layer 1）:
   - Cookie検証
   - JWT付与: Authorization: Bearer <JWT>

3. Backstage（Layer 2）:
   - Authorizationヘッダー受信
   - JWTデコード: { email: "admin@example.com", groups: ["platform-engineers"] }
   - Keycloak公開鍵でJWT検証
   - Permission Framework: groupsに基づいて認可判定

   例: catalogEntityDeletePermission
   → groups.includes('platform-engineers') → ALLOW
   → groups.includes('app-developers') → DENY
```

### メリット

1. **段階的導入**
   - Phase 1: 無認証（開発）
   - Phase 2: Gateway認証（一括保護）
   - Phase 3: 多層認証（細かい権限）

2. **運用負荷の最小化**
   - Phase 2: 一箇所で認証管理
   - Phase 3: 必要なサービスのみ追加設定
   - OAuth2 Proxyの設定変更不要

3. **柔軟性**
   - サービス特性に応じた認証方式
   - Prometheus/Hubbleは Phase 2のまま
   - Backstage/Argo CDは Phase 3で細かい認可

4. **多層防御**
   - Gateway: 無効なJWT早期排除
   - Application: リソースレベル認可

5. **ユーザー体験**
   - 1回のログインで全サービスアクセス
   - Cookie共有（.platform.your-org.com）

### デメリットと対策

1. **複雑性の増加**
   - 対策: 段階的に構築（Phase 1 → 2 → 3）
   - Phase 2までは非常にシンプル

2. **OAuth2 Proxyが単一障害点**
   - 対策: レプリカ数2で冗長化
   - PodDisruptionBudgetで可用性確保

3. **パフォーマンスオーバーヘッド**
   - ExtAuthz呼び出しのレイテンシ
   - 対策: OAuth2 Proxyをistio-system namespaceに配置（近接性）
   - 対策: Cookieキャッシュにより認証済みリクエストは高速

4. **デバッグの難しさ**
   - 対策: 各レイヤーでログ出力
   - OAuth2 Proxy: `--auth-logging=true`
   - Backstage: Permission Frameworkログ

### トレードオフ

**複雑性 vs 柔軟性**

- ✅ 採用: Phase 2でシンプルに開始、Phase 3で柔軟性を追加
- ❌ 不採用: 最初から全サービスOIDC実装（運用負荷大）
- ❌ 不採用: Gateway認証のみ（細かい認可不可）

**多層防御 vs パフォーマンス**

- ✅ 採用: 2回のJWT検証（Gateway + Application）
- ❌ 不採用: 1回のみの検証（セキュリティ低下）

本環境では、以下の理由によりハイブリッド方式が適切:
- プラットフォームサービスは管理者・開発者が使用（セキュリティ重要）
- 段階的導入により開発初期の負荷を軽減
- 将来的な細かい権限制御に対応

## 実装ロードマップ

### Phase 1: 現状維持（完了）
- すべて無認証
- プラットフォーム構築に集中

### Phase 2: Gateway認証（2週間後）

**Week 1:**
1. Keycloak Realm `platform` 作成
2. OAuth2 Proxy用Client作成
3. Admin user/group作成
4. Client Secretsの取得

**Week 2:**
1. OAuth2 Proxy Deployment/Service作成
2. Sealed Secretsで秘密情報管理
3. HTTPRoute（auth.platform.your-org.com）作成
4. Istio ExtAuthz設定
5. 動作確認

**成果:**
- すべてのサービスが保護される
- 1回のログインで全サービスアクセス可能

### Phase 3: 多層認証（1ヶ月後以降、段階的に）

**Month 1: Backstage**
1. Proxy Providerモジュール追加
2. Permission Framework有効化
3. カスタムポリシー実装
4. groupsクレームベースのRBAC

**Month 2: Argo CD**
1. OIDC設定追加
2. policy.csvでロールマッピング
3. 動作確認

**Month 3: Grafana**
1. Generic OAuth設定
2. role_attribute_pathでロールマッピング
3. 動作確認

**Prometheus/Hubble:**
- 変更なし（Phase 2のまま）

## 参考資料

- [OAuth2 Proxy Documentation](https://oauth2-proxy.github.io/oauth2-proxy/)
- [Istio External Authorization](https://istio.io/latest/docs/tasks/security/authorization/authz-custom/)
- [Backstage Auth Proxy Provider](https://backstage.io/docs/auth/proxy/)
- [Backstage Permission Framework](https://backstage.io/docs/permissions/overview)
- [Argo CD OIDC Configuration](https://argo-cd.readthedocs.io/en/stable/operator-manual/user-management/#oidc)
- [Keycloak Authorization Services](https://www.keycloak.org/docs/latest/authorization_services/)
- [AWS ALB + Cognito Pattern](https://docs.aws.amazon.com/elasticloadbalancing/latest/application/listener-authenticate-users.html)
