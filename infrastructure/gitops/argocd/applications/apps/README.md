# Argo CD Application マニフェスト管理

このディレクトリには、Backstageテンプレートから作成された全アプリケーションのArgo CD Application CRが格納されます。

## ⚠️ 重要: リポジトリ命名規則

**全てのアプリケーションリポジトリは `kensan-lab-apps-` プレフィックスを持つ必要があります。**

- **正しい例**: `kensan-lab-apps-my-app`, `kensan-lab-apps-api-server`
- **誤った例**: `my-app`, `api-server`, `app-my-app`

このプレフィックスは：
- Backstageテンプレートで自動的に付与されます
- **変更しないでください** - Argo CDの追跡に使用されます
- 将来的にApplicationSetでの自動検出に対応予定

## ディレクトリ構造

各アプリケーションは専用のサブディレクトリを持ち、Dev/Prod両環境のApplication CRを含みます：

```
apps/
├── my-app/
│   └── argocd-apps.yaml    # app-dev-my-app と app-prod-my-app を含む
│                           # リポジトリ: kensan-lab-apps-my-app
├── api-server/
│   └── argocd-apps.yaml    # app-dev-api-server と app-prod-api-server を含む
│                           # リポジトリ: kensan-lab-apps-api-server
└── README.md
```

## アプリケーション追加フロー

1. **開発者**がBackstageのSoftware Templatesから新規アプリケーションを作成
   - アプリケーション名入力: `my-app`
2. **Backstage**が自動的に以下を生成：
   - アプリケーションリポジトリ: `github.com/yu-min3/kensan-lab-apps-my-app`
   - このリポジトリ（platform-config）へのPull Request（Application CRを含む）
3. **Platform Engineer**がPRをレビューしてマージ
4. **Argo CD**が自動的に新しいApplicationを検出してsync

## Application CR フォーマット

各`argocd-apps.yaml`ファイルには、`---`で区切られた2つのApplicationリソースが含まれます：

```yaml
---
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: app-dev-<app-name>
  namespace: argocd
spec:
  project: app-project-dev
  source:
    repoURL: https://github.com/yu-min3/kensan-lab-apps-<app-name>.git
    targetRevision: main
    path: overlays/dev
  destination:
    namespace: app-dev
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
---
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: app-prod-<app-name>
  namespace: argocd
spec:
  project: app-project-prod
  source:
    repoURL: https://github.com/yu-min3/kensan-lab-apps-<app-name>.git
    targetRevision: main
    path: overlays/prod
  destination:
    namespace: app-prod
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
```

## この構造のメリット

✅ **ファイル名の衝突なし**: 各アプリが専用ディレクトリを持つ
✅ **簡単な追跡**: 1ディレクトリ = 1アプリケーション
✅ **シンプルな管理**: ディレクトリ削除でDev/Prod両方を削除
✅ **明確な整理**: 数百のアプリケーションでもスケール可能
