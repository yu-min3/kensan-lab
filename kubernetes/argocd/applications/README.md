# Argo CD Applications ディレクトリ

このディレクトリには Argo CD の Application CR と ApplicationSet CR が配置されています。
`platform-root-app` がこのディレクトリを再帰スキャンし、全てのリソースをデプロイします。

## パターン使い分け

| カテゴリ | パターン | 理由 |
|---------|---------|------|
| **observability/** | ApplicationSet | Helm multi-source で構造が均一。config.json でパラメータ化 |
| **environments/** | ApplicationSet | single source (Git directory) で構造が均一。namespace 追加が容易 |
| **network/** | 個別 Application | sync-wave 依存関係、ignoreDifferences が各アプリ固有 |
| **auth/** | 個別 Application | Keycloak / oauth2-proxy / vault-oidc-auth で構成が異なる |
| **secrets/** | 個別 Application + 一部 ApplicationSet (per-DB instance) | sync-wave あり。vault-database-engine の instance は ApplicationSet で量産 |
| **storage/** | 個別 Application | Longhorn は Prune=false など固有設定多数 |
| **gitops/** | 個別 Application | Argo CD 自己管理、複雑な ignoreDifferences |
| **backstage/** | 個別 Application | 単独 Kustomize アプリ |
| **apps/** | 個別 Application | Backstage が自動コミット、イメージタグ個別更新 |

詳細な設計判断は [ADR-003](../../../docs/adr/003-applicationset-migration-strategy.md) を参照してください。

## 新規コンポーネント追加

### Observability に追加する場合

1. `kubernetes/observability/<name>/values.yaml` を作成
2. `kubernetes/observability/<name>/config.json` を作成（テンプレートは既存を参照）
3. 必要に応じて `kubernetes/observability/<name>/resources/` に追加マニフェストを配置
4. commit & push → ApplicationSet が自動で Application を生成

### Environments に追加する場合

1. `kubernetes/environments/<name>/` に namespace 等のマニフェストを配置
2. `kubernetes/environments/<name>/config.json` を作成
3. commit & push → ApplicationSet が自動で Application を生成
