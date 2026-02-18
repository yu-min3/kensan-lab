# ADR-0008: 統一ベクトル検索アーキテクチャ

**Status**: Accepted
**Date**: 2026-02-01
**Related**: ADR-0004 (データモデル再設計)

---

## Context

従来の検索アーキテクチャでは、2つの独立した検索対象が存在していた：

1. **`documents` テーブル**: kensan-ai が管理するドキュメントストア。R2 にファイルを保存し、全文 + ベクトル検索を提供
2. **`notes` テーブル**: note-service が管理するノート。後から `embedding` カラムと `note_content_chunks` テーブルを追加

この二重構造には以下の問題があった：

- **データの重複**: ノートの内容が notes と documents の両方に存在する可能性
- **検索結果の不統一**: semantic_search は documents を、search_notes は notes を検索するため、ユーザーが両方を意識する必要
- **R2 の不要性**: note-service が MinIO を使用しているのに対し、kensan-ai は R2 を使用。ストレージが分散
- **チャンクレベル検索の欠如**: documents テーブルはドキュメント全体を1ベクトルで表現。長文の検索精度が低い

## Decision

**`documents` テーブルと R2 ストレージを廃止し、`note_content_chunks` テーブルに検索を統一する。**

### 検索の2段構え

| 層 | テーブル | 用途 |
|----|---------|------|
| 粗い検索 | `notes` (embedding カラム) | ノート全体の概要ベクトル検索 |
| 精密検索 | `note_content_chunks` (embedding カラム) | チャンクレベルのベクトル + 全文検索 |

### ストレージ統一

- kensan-ai は MinIO から読み取り専用でノートコンテンツを取得（boto3 直接アクセス）
- note-service が MinIO への書き込みを担当（既存）
- R2 への依存を完全に排除

### チャンク戦略

| content_type | 戦略 | 備考 |
|-------------|------|------|
| markdown | 見出し (h1-h3) で分割、500 トークン超は段落で追加分割 | 構造を尊重 |
| code | 固定長 500 トークン、50 トークン overlap | コンテキスト維持 |
| drawio | XML から value 属性を抽出、ラベル結合で 1 チャンク | 図の要約 |
| image, pdf | スキップ (空リスト) | 将来の OCR/Vision 対応用 |

### インデックスパイプライン

polling ベース: note-service が `index_status = 'pending'` を設定済み。kensan-ai の `reindex_pending_notes()` が pending ノートを処理。

```
notes.index_status: pending → processing → indexed (or failed)
```

## Consequences

### メリット

- **検索の統一**: 全てのテキスト検索が `note_content_chunks` を対象。ユーザーは検索先を意識不要
- **チャンク検索**: 長文ノートでも関連部分のみがヒット。検索精度向上
- **ストレージ簡素化**: MinIO のみ。R2 の設定・認証情報が不要
- **JOIN 削減**: `note_content_chunks` に `user_id` を非正規化。検索クエリで notes への JOIN が不要

### デメリット

- **非正規化コスト**: `user_id` の重複保存（ただし UUID なので容量影響は軽微）
- **リインデックス必要**: 既存ノートは `reindex_notes` ツールで明示的にインデックス化が必要
- **画像/PDF 未対応**: 現時点ではテキスト系コンテンツのみ。将来の拡張が必要

## Migration

- `backend/migrations/037_unified_search.sql`:
  - `note_content_chunks` に `user_id`, `content_type` カラム追加
  - 既存データの `user_id` 埋め戻し
  - `documents` テーブル削除
