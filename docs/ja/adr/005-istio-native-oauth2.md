# ADR-005: Istio native oauth2 + Keycloak による Phase 1 認証実装

## ステータス

採用済み (Accepted)

ADR-002 で Phase 2 の Gateway 一括認証として想定されていた OAuth2 Proxy 方式を、本 ADR で Istio native `oauth2` extension provider に置き換える。ADR-002 の段階的アプローチ（Phase 1〜3）と Phase 3 の多層認可は引き続き有効。

## 日付

2026-05-03

## コンテキスト

ADR-002 では Phase 2 の Gateway 一括認証として OAuth2 Proxy を ExtAuthz Provider に据える前提で設計したが、当該マニフェストはまだ作成されていない（実装は Phase 1 = 無認証のまま）。

その後、Istio 1.27 から Gateway API + native `oauth2` extension provider が安定供給されたため、別 Pod として OAuth2 Proxy を運用しなくても Istio 単体で OIDC + JWT cookie が完結するようになった。

### 環境確認結果（2026-05-03）

- **Istio**: 1.27.3（ArgoCD Application `targetRevision: 1.27.3`）
- **Gateway API モード**: 有効（`PILOT_ENABLE_GATEWAY_API: "true"`）
- **既存 Gateway**: `gateway-platform` / `gateway-prod` / `gateway-dev` が Gateway API リソースで定義済み
- → native `oauth2` ext provider が確実に使える環境

### 要件

1. **追加コンポーネント最小化**: homelab 1 人運用のため、運用対象を増やさない
2. **Keycloak を唯一の IdP に**: Vault / ArgoCD / Backstage / Grafana と統一した SSO 体験
3. **二段階認可**: Gateway-level の coarse 認可（このアプリに到達してよいか）+ アプリ内 fine 認可（中で何が出来るか）
4. **fail-secure**: 認可漏れがあれば「アクセス不可」になる構造（default-deny base）
5. **JWT のアプリ転送**: アプリ側で claims を直接読めるようにする

### 検討したパターン

#### パターン A: OAuth2 Proxy ExtAuthz（ADR-002 当初想定）

OAuth2 Proxy Pod を `auth-system` namespace に配置し、Istio から ExtAuthz 経由で全リクエストを通す。

**メリット:**
- 実例が多く、AWS ALB + Cognito 風のパターンとして広く知られている
- 各サービス側は「ヘッダ読み取りだけ」で利用できる

**デメリット:**
- OAuth2 Proxy Pod 自身が SPOF。冗長化のために replicas + PodDisruptionBudget が必要
- `auth-system` namespace と OAuth2 Proxy Helm chart の運用が増える
- Istio + Keycloak だけでは完結せず、もう 1 ホップ Pod を経由する

#### パターン B: Istio native `oauth2` extension provider（採用）

`meshConfig.extensionProviders` に native `oauth2` プロバイダを定義し、`AuthorizationPolicy` の `action: CUSTOM` で Gateway に紐付ける。OIDC コードフロー / JWT cookie / token refresh を Istio が直接処理する。

**メリット:**
- 別 Pod を立てない。Istio 単体で完結
- OIDC client secret は Istio 側 Secret に置くのみ
- 「Istio + Keycloak で認証完結」という記述上のキャッチが綺麗
- 既存 Gateway リソース（Gateway API `Gateway`）にそのまま attach 可能

**デメリット:**
- 機能が新しく、運用知見・トラブルシュート事例が OAuth2 Proxy より少ない
- Istio version への依存が強く、downgrade した際の挙動を意識する必要がある

#### パターン C: 各サービス個別 OIDC

Vault / ArgoCD / Grafana / Backstage / アプリ全部に個別 OIDC client を切る。

**メリット:**
- 各サービスのネイティブ認可機能（ArgoCD RBAC、Grafana role mapping 等）を最大限活かせる

**デメリット:**
- アプリ側 OIDC 設定の運用負荷が線形に増える
- アプリチームが OIDC 実装を毎回書く必要があり、AD（Application Developer）の責務が広がる

## 決定

**パターン B（Istio native `oauth2` extension provider）を採用する。**

加えて、以下を確定する。

### 1. 二段階認可アーキテクチャ

- **Gateway-level（coarse 認可）**: `istio-system` の `AuthorizationPolicy` で host × group claim を判定。default-deny を base に、明示 ALLOW のみ列挙
- **アプリ内（fine 認可）**: `forwardOriginalToken: true` で JWT をアプリへ転送し、アプリ側 OIDC ライブラリで claims をデコードして詳細認可（管理者ボタンの可否等）を実装

### 2. JWT 転送設定

```yaml
apiVersion: security.istio.io/v1
kind: RequestAuthentication
metadata:
  name: keycloak-jwt
  namespace: istio-system
spec:
  targetRef:
    group: gateway.networking.k8s.io
    kind: Gateway
    name: gateway-prod
  jwtRules:
  - issuer: "https://auth.platform.yu-min3.com/realms/kensan"
    jwksUri: "https://auth.platform.yu-min3.com/realms/kensan/protocol/openid-connect/certs"
    forwardOriginalToken: true
```

### 3. default-deny + 明示 ALLOW の Gateway-level AuthorizationPolicy

```yaml
# default-deny base（rules 無し = deny all）
apiVersion: security.istio.io/v1
kind: AuthorizationPolicy
metadata:
  name: gateway-prod-deny-all
  namespace: istio-system
spec:
  targetRef:
    group: gateway.networking.k8s.io
    kind: Gateway
    name: gateway-prod
---
# host × groups で明示 ALLOW
apiVersion: security.istio.io/v1
kind: AuthorizationPolicy
metadata:
  name: gateway-prod-allow
  namespace: istio-system
spec:
  targetRef:
    group: gateway.networking.k8s.io
    kind: Gateway
    name: gateway-prod
  action: ALLOW
  rules:
  - to:
    - operation:
        hosts: ["streamlit.app.yu-min3.com"]
    when:
    - key: request.auth.claims[groups]
      values: ["app-team-a", "platform-admin"]
```

### 4. Workload-level AuthorizationPolicy は Phase 2 以降

各アプリ namespace 単位の `AuthorizationPolicy` は Phase 1 では必須としない。Phase 2 以降、defense-in-depth が要る高セキュリティアプリのみ追加する。

### 5. ADR-002 との関係

- ADR-002 の **Phase 2「OAuth2 Proxy による Gateway 一括認証」部分は本 ADR で置換**する。OAuth2 Proxy の Helm chart デプロイ・`auth-system` namespace は作成しない
- ADR-002 の Phase 3「サービス個別の細かい認可」は引き続き有効。Istio native `oauth2` から `forwardOriginalToken: true` で JWT を転送するため、Phase 3 の Backstage / ArgoCD / Grafana 個別認可はそのまま乗る
- ADR-002 全体を **Superseded にはしない**（Phase 2 の実装手段が変わるだけで、段階的アプローチと Phase 3 多層認可の方針は維持）

## 結果

### 良い結果

- 別 Pod（OAuth2 Proxy）の運用が不要。Istio config と Keycloak realm 設定のみで認証完結
- default-deny base + 明示 ALLOW により、新規アプリで認可ルール書き忘れた場合は「アクセス不可」で気付ける（fail-secure）
- アプリ側は `Authorization: Bearer <JWT>` を受け取るだけで済み、OIDC 実装を持たない
- Vault / ArgoCD / Backstage / Grafana / 一般アプリの 5 系統が Keycloak 経由で SSO 統一される

### トレードオフ

- Istio native `oauth2` provider はコミュニティ事例が OAuth2 Proxy より少ない。トラブル時の調査コストが増えうる
- Istio バージョンアップ時に extension provider 設定の互換性確認が必要（OAuth2 Proxy なら Istio に依存しない切り離しが可能だった）
- Keycloak 全停止時は 5 系統すべてアクセス不能になる。break-glass 経路（Vault userpass / ArgoCD 内蔵 admin / Grafana local admin / Backstage local user）を別途維持する必要がある

## 関連

- ADR-002: 認証認可アーキテクチャの段階的実装（Phase 2 の実装手段を本 ADR で置換）
- ADR-006: Application namespace 命名規約（`kensan-lab.platform/team` label を AuthorizationPolicy で参照）
- ADR-008: Keycloak の DB 認証情報を Vault に寄せない（Keycloak 自体の boring-keeping 方針）
- 設計ソース: `kensan-workspace/projects/kensan-lab/secrets-phase1-design.md` § アプリ認証/認可フロー（Istio + Keycloak）
- [Istio: External Authorization](https://istio.io/latest/docs/tasks/security/authorization/authz-custom/)
- [Istio 1.27 Release Notes](https://istio.io/latest/news/releases/1.27.x/)
