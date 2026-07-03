# Argo CD Application マニフェスト（apps カテゴリ）

このディレクトリには、プラットフォーム上で動く **application** の Argo CD `Application` CR を置く。
各アプリは専用サブディレクトリを持ち、その中の `app.yaml` が単一の `Application` を定義する。

## ディレクトリ構造

```
apps/
├── app-kensan/
│   └── app.yaml    # 現行 kensan（in-repo chart + per-app ns、3-source）
└── README.md
```

> ⚠️ ファイル名は `app.yaml`（過去ドキュメントの `argocd-apps.yaml` は実体と異なる）。

## 実在パターン: in-repo app（app-kensan）

現行の app は外部 repo を持たず、この monorepo 内で完結する。`app-kensan/app.yaml` は
PE 提供の汎用 chart `charts/app-base` を **multi-source** で参照する 3-source 構成:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: app-kensan
  namespace: argocd
  annotations:
    argocd.argoproj.io/sync-options: Prune=false   # Application CR の prune 防止
spec:
  project: app-project
  sources:
    # 1. PE 所有の汎用 chart
    - repoURL: https://github.com/yu-min3/kensan-lab
      targetRevision: main
      path: charts/app-base
      helm:
        releaseName: app-kensan
        valueFiles:
          - $values/kubernetes/apps/app-kensan/values.yaml
    # 2. values 参照用 ref
    - repoURL: https://github.com/yu-min3/kensan-lab
      targetRevision: main
      ref: values
    # 3. 生マニフェスト（namespace, PVC, syncthing 等）
    - repoURL: https://github.com/yu-min3/kensan-lab
      targetRevision: main
      path: kubernetes/apps/app-kensan/resources
      directory:
        recurse: true
  destination:
    server: https://kubernetes.default.svc
    namespace: app-kensan           # per-app ns（ADR-006）
  syncPolicy:
    automated: { prune: true, selfHeal: true }
    syncOptions:
      - CreateNamespace=false       # namespace.yaml（labels 付き）で管理
      - ServerSideApply=true
```

`charts/app-base` の利用方法は [charts/app-base/README.md](../../../../charts/app-base/README.md) を参照。
app 固有の値は `kubernetes/apps/app-<name>/values.yaml` に置く。

旧 kensan の `kensan/app.yaml` は Phase 7 cutover（PR #394）で撤去済み。ソースは tag `kensan-legacy-final` にアーカイブ（ADR-017）。

## 将来フロー（予定）: Backstage scaffolded app

外部 app repo を Backstage Software Template で量産する将来フローでは、テンプレートが
app repo（命名規則 `kensan-lab-apps-<name>`）と、この repo への PR（`Application` CR を含む）を生成し、
PE がレビュー & マージ → Argo CD が自動 sync、という流れを想定している。
この自動化はまだ実装段階であり、現状で稼働しているのは上記の in-repo パターンのみ。
