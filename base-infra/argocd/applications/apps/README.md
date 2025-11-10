# Argo CD Application マニフェスト管理

このディレクトリには、Backstageテンプレートから作成された全アプリケーションのArgo CD Application CRが格納されます。

## ディレクトリ構造

各アプリケーションは専用のサブディレクトリを持ち、Dev/Prod両環境のApplication CRを含みます：

```
apps/
├── test-app/
│   └── argocd-apps.yaml    # app-dev-test-app と app-prod-test-app を含む
├── another-app/
│   └── argocd-apps.yaml    # app-dev-another-app と app-prod-another-app を含む
└── README.md
```

## アプリケーション追加フロー

1. **開発者**がBackstageのSoftware Templatesから新規アプリケーションを作成
2. **Backstage**が自動的に以下を生成：
   - アプリケーションリポジトリ（例: `github.com/yu-min3/test-app`）
   - このリポジトリへのPull Request（Application CRを含む）
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
    repoURL: https://github.com/yu-min3/<app-name>.git
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
    repoURL: https://github.com/yu-min3/<app-name>.git
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
