# Argo CD Applications ディレクトリ

このディレクトリには Argo CD の Application CR と ApplicationSet CR が配置されています。
`platform-root-app` がこのディレクトリを再帰スキャンし、全てのリソースをデプロイします。

## パターン使い分け

実装上 ApplicationSet を使っているのは **observability** と **vault-{database,transit}-engine** の 3 つだけ。
残りは全て個別 Application（environments を含む）。詳細は [ADR-003](../../../docs/adr/003-applicationset-migration-strategy.md) と
その Addendum (2026-06-07) を参照。

| カテゴリ | パターン | 理由 |
|---------|---------|------|
| **observability/** | ApplicationSet | Helm multi-source で構造が均一。Git File Generator (`kubernetes/observability/*/config.json`) でパラメータ化。リファレンス実装 |
| **secrets/vault-database-engine/**, **vault-transit-engine/** | ApplicationSet (per-instance) | 自作 chart + per-instance `platform-values/` を Git File Generator で量産。共有部分は `app-shared.yaml`（個別 Application） |
| **environments/** | 個別 Application | dev/prod 廃止後、namespace ライフサイクル専用 app に変化。専用 ns マニフェスト・`directory.include` フィルタ等で source 形状が不均一なため ApplicationSet 化せず（ADR-003 Addendum）。`config.json` は存在しない |
| **network/** | 個別 Application | sync-wave 依存関係、ignoreDifferences が各アプリ固有 |
| **auth/** | 個別 Application | Keycloak / oauth2-proxy / vault-oidc-auth で構成が異なる |
| **secrets/**（その他） | 個別 Application | sync-wave あり。Vault / external-secrets / cert-manager 等 |
| **policy/** | 個別 Application | Kyverno 本体 + kyverno-policies |
| **storage/** | 個別 Application | Longhorn は Prune=false など固有設定多数 |
| **gitops/** | 個別 Application | Argo CD 自己管理、複雑な ignoreDifferences |
| **backstage/** | 個別 Application | 単独アプリ |
| **apps/** | 個別 Application | app ごとに `app.yaml`。image タグ個別更新（in-repo の app-kensan が実例） |

## 新規コンポーネント追加

### Observability に追加する場合（ApplicationSet）

1. `kubernetes/observability/<name>/values.yaml` を作成
2. `kubernetes/observability/<name>/config.json` を作成（テンプレートは既存を参照）
3. 必要に応じて `kubernetes/observability/<name>/resources/` に追加マニフェストを配置
4. commit & push → ApplicationSet が自動で Application を生成

### Environments に namespace を追加する場合（個別 Application）

ApplicationSet ではないため `config.json` は不要。

1. `kubernetes/environments/<name>/` に namespace 等のマニフェストを配置
2. `kubernetes/argocd/applications/environments/<name>/app.yaml` を作成（既存の ns-lifecycle app を雛形にする）
3. commit & push → root-app が再帰スキャンして Application を sync
