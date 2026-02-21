# ADR-003: ApplicationSet 移行戦略 — App of Apps との混在設計

## ステータス

採用済み (Accepted)

## 日付

2026-02-20

## コンテキスト

プラットフォームの Argo CD 管理は App of Apps パターンで運用されてきた。`platform-root-app` が `applications/` ディレクトリを再帰スキャンし、各カテゴリの Application CR をデプロイする構成である。

### 課題

運用が進むにつれ、以下の問題が顕在化した：

1. **ボイラープレートの重複**: 同カテゴリ内のアプリは構造が類似しているが、個別の `app.yaml` に設定が散在
2. **設定の不統一**: finalizer、retry、ServerSideApply の有無がアプリごとにバラバラ
   - 例: grafana は SSA なし・retry あり、loki は SSA あり・retry あり、env-kensan-* は finalizer なし
3. **新規追加の手間**: 新コンポーネント追加時に `app.yaml` を丸ごとコピー＆編集が必要

### 検討した選択肢

| 選択肢 | 概要 |
|--------|------|
| A. 全て App of Apps のまま維持 | 現状維持。シンプルだがボイラープレート問題は解消されない |
| B. 全て ApplicationSet に移行 | 一貫性は最大化されるが、固有設定のある app で templatePatch が複雑化 |
| C. カテゴリごとに最適なパターンを選択（混在） | 均一なカテゴリは ApplicationSet、固有設定が多いカテゴリは個別 Application |

## 決定

**選択肢 C: カテゴリごとに最適なパターンを選択する混在構成を採用する。**

### 判定基準

<!-- TODO(human): 各カテゴリを ApplicationSet vs 個別 Application に振り分けた判定基準を記述してください -->

### 判定結果

| カテゴリ | アプリ数 | パターン | 主な理由 |
|---------|---------|---------|---------|
| Observability | 5 | ApplicationSet | Helm multi-source で構造が均一、今後も追加の可能性 |
| Environments | 5 | ApplicationSet | single source で構造が均一、namespace 追加が頻繁 |
| Network | 5 | 個別 Application | sync-wave 依存、ignoreDifferences が固有 |
| Security | 3 | 個別 Application | Helm/Kustomize 混在、sync-wave あり |
| GitOps | 1 | 個別 Application | 自己管理、複雑な ignoreDifferences |
| Backstage | 1 | 個別 Application | 単独 Kustomize アプリ |
| Apps | 3 | 個別 Application | Backstage 自動コミット、イメージタグ個別更新 |

### ApplicationSet 設計

#### Observability ApplicationSet
- **Generator**: Git File Generator (`infrastructure/observability/*/config.json`)
- **テンプレート**: Helm multi-source（chart source + values ref）
- **templatePatch**: `hasResources: "true"` の場合に 3rd source（resources/）を追加
- **統一ポリシー**: finalizer, ServerSideApply, retry を全アプリに適用

#### Environments ApplicationSet
- **Generator**: Git File Generator (`infrastructure/environments/*/config.json`)
- **テンプレート**: single source（Git directory）
- **統一ポリシー**: finalizer を全アプリに適用

### 移行の安全性設計

2-commit 戦略で無停止移行を実現：

1. **Commit 1**: 対象 Application CR から `resources-finalizer.argocd.argoproj.io` を除去
   - Argo CD sync 後、finalizer が外れる（管理下リソースには影響なし）
2. **Commit 2**: ApplicationSet CR を追加し、旧 Application CR を削除
   - root-app が旧 Application を prune（finalizer なし → cascade 削除なし）
   - ApplicationSet controller が同名の Application CR を再生成
   - クラスタ上のワークロードは中断なし

## 結果

### メリット

- **設定の一元管理**: テンプレートにより finalizer / SSA / retry が自動的に統一
- **新規追加の簡素化**: `config.json` + `values.yaml` を置くだけで Application が自動生成
- **固有設定の保全**: 個別 Application を残すことで、sync-wave や ignoreDifferences の複雑性を維持

### デメリット

- **認知モデルが 2 つ必要**: 「この app はどこを見ればいい？」がカテゴリにより異なる
  - 対策: `applications/README.md` にパターン使い分けテーブルを配置
- **root-app が ApplicationSet と Application の両方をデプロイ**: Argo CD コミュニティでは認知されたパターンだが、純粋な単一パターンではない

### 今後の検討事項

- Network カテゴリのアプリが増えた場合、ApplicationSet + merge generator で sync-wave を吸収できるか検証
- Backstage テンプレートが生成する Application CR の形式が統一されれば、Apps カテゴリも ApplicationSet 化を検討
