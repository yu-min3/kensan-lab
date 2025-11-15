# Namespace ラベル設計

このドキュメントでは、プラットフォーム全体で統一されたNamespaceラベリング戦略を定義します。

## 設計原則

1. **一貫性**: すべてのNamespaceで同じラベルキーと値のパターンを使用
2. **検索性**: `kubectl get ns -l <label>=<value>`で簡単にフィルタリング可能
3. **セキュリティ**: NetworkPolicy、RBAC等でラベルセレクターを活用
4. **標準準拠**: Kubernetesの推奨ラベル（`app.kubernetes.io/*`）に準拠

## ラベルスキーマ

### 必須ラベル（全Namespaceに適用）

| ラベルキー | 値 | 説明 |
|-----------|-----|------|
| `app.kubernetes.io/managed-by` | `argocd` | GitOpsツールの識別（Kubernetes推奨ラベル） |
| `goldship.platform/environment` | `production` \| `development` \| `infrastructure` | 環境タイプ |
| `goldship.platform/tier` | `platform` \| `application` | 責務レイヤー（PE管理 vs AD管理） |

### オプショナルラベル（用途に応じて適用）

| ラベルキー | 値（例） | 説明 | 適用対象 |
|-----------|---------|------|----------|
| `goldship.platform/component` | `keycloak`, `backstage`, `monitoring`, `service-mesh`, `core` | コンポーネント識別 | プラットフォーム層のみ |
| `istio-injection` | `enabled` | Istio自動サイドカーインジェクション | サービスメッシュ対象Namespace |

## ラベル値の定義

### `goldship.platform/environment`

| 値 | 説明 | 対象Namespace例 |
|----|------|-----------------|
| `infrastructure` | クラスター基盤インフラ | `kube-system`, `istio-system`, `monitoring`, `argocd` |
| `production` | 本番環境 | `backstage`, `platform-auth-prod`, `app-prod` |
| `development` | 開発環境 | `platform-auth-dev`, `app-dev` |

### `goldship.platform/tier`

| 値 | 説明 | 管理者 | 対象Namespace例 |
|----|------|--------|-----------------|
| `platform` | プラットフォームインフラ層 | Platform Engineer (PE) | `istio-system`, `backstage`, `platform-auth-prod` |
| `application` | アプリケーション層 | Application Developer (AD) | `app-prod`, `app-dev`, `app-prod-<name>` |

### `goldship.platform/component`

| 値 | 説明 | 対象Namespace |
|----|------|--------------|
| `core` | Kubernetes基盤システム | `kube-system` |
| `service-mesh` | サービスメッシュ制御プレーン | `istio-system` |
| `gitops` | GitOpsツール | `argocd` |
| `observability` | モニタリング・ログ管理 | `monitoring` |
| `keycloak` | 認証基盤 | `platform-auth-prod`, `platform-auth-dev` |
| `developer-portal` | 開発者ポータル | `backstage` |

## Namespace定義テンプレート

### プラットフォームインフラ層（環境非依存）

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: <namespace-name>
  labels:
    app.kubernetes.io/managed-by: argocd
    goldship.platform/environment: infrastructure
    goldship.platform/tier: platform
    goldship.platform/component: <component-name>
    # オプション: Istio使用時
    # istio-injection: enabled
```

### プラットフォーム層（環境別）

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: <namespace-name>
  labels:
    app.kubernetes.io/managed-by: argocd
    goldship.platform/environment: production|development
    goldship.platform/tier: platform
    goldship.platform/component: <component-name>
    istio-injection: enabled
```


### アプリケーション層

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: <namespace-name>
  labels:
    app.kubernetes.io/managed-by: argocd
    goldship.platform/environment: production|development
    goldship.platform/tier: application
    istio-injection: enabled
```

## ラベルの活用例

### 2. NetworkPolicy での活用

```yaml
# プラットフォーム層間の通信を許可
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-platform-tier
  namespace: backstage
spec:
  podSelector: {}
  policyTypes:
  - Ingress
  ingress:
  - from:
    - namespaceSelector:
        matchLabels:
          goldship.platform/tier: platform
```

```yaml
# 本番環境のアプリケーションからKeycloakへのアクセス許可
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-prod-apps-to-keycloak
  namespace: platform-auth-prod
spec:
  podSelector:
    matchLabels:
      app: keycloak
  policyTypes:
  - Ingress
  ingress:
  - from:
    - namespaceSelector:
        matchLabels:
          goldship.platform/environment: production
          goldship.platform/tier: application
```

### 3. RBAC での活用

```yaml
# Application Developerに開発環境Namespaceへのアクセス権を付与
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: app-developer
rules:
- apiGroups: ["*"]
  resources: ["*"]
  verbs: ["*"]
  # 開発環境のアプリケーション層のみアクセス可能
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: app-developers-binding
  namespace: app-dev
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: app-developer
subjects:
- kind: Group
  name: application-developers
  apiGroup: rbac.authorization.k8s.io
```

### 4. Prometheus ServiceMonitor セレクター

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: platform-services
  namespace: monitoring
spec:
  namespaceSelector:
    matchLabels:
      goldship.platform/tier: platform
  selector:
    matchLabels:
      monitoring: enabled
```

### 5. Istio Sidecar 自動インジェクション確認

```bash
# Istio有効なNamespaceを確認
kubectl get ns -l istio-injection=enabled

# 新しいNamespaceにIstio自動インジェクション設定
kubectl label namespace my-new-namespace istio-injection=enabled
```

## 移行ガイド

既存のNamespaceラベルを統一設計に移行する手順：

### Phase 1: 新規Namespaceから適用（優先度: 高）

今後作成するすべてのNamespaceは、このラベル設計に準拠させる。

### Phase 2: アプリケーション層の更新（優先度: 中）

```bash
# app-dev/namespace.yaml
# 変更前:
labels:
  environment: development

# 変更後:
labels:
  app.kubernetes.io/managed-by: argocd
  goldship.platform/environment: development
  goldship.platform/tier: application
  istio-injection: enabled
```

### Phase 3: プラットフォーム層の更新（優先度: 低）

段階的に既存Namespaceを更新。クラスター運用に影響がないことを確認しながら進める。

**注意事項:**
- ラベル変更はNamespace自体には影響しないが、ラベルセレクターを使用している既存のリソース（NetworkPolicy、ServiceMonitor等）を事前に確認
- Argo CD経由で更新（GitOps原則を維持）

## まとめ

### ラベルマトリックス

| Namespace | environment | tier | component | istio-injection |
|-----------|-------------|------|-----------|-----------------|
| kube-system | infrastructure | platform | core | ❌ |
| istio-system | infrastructure | platform | service-mesh | ❌ |
| argocd | infrastructure | platform | gitops | ❌ |
| monitoring | infrastructure | platform | observability | ❌ |
| backstage | production | platform | developer-portal | ✅ |
| platform-auth-prod | production | platform | keycloak | ✅ |
| platform-auth-dev | development | platform | keycloak | ✅ |
| app-prod | production | application | - | ✅ |
| app-dev | development | application | - | ✅ |

### ベストプラクティス

1. **必須ラベルは必ず設定**: `managed-by`, `environment`, `tier`
2. **componentラベルはプラットフォーム層のみ**: アプリケーション層には不要
3. **istio-injectionは明示的に設定**: サービスメッシュ対象のNamespaceのみ
4. **カスタムラベルは最小限に**: 必要な場合は`goldship.platform/`プレフィックスを使用
5. **ラベル変更時はNetworkPolicyを確認**: セレクターが影響を受ける可能性

このラベル設計により、プラットフォーム全体の一貫性、検索性、セキュリティが向上します。
