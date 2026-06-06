# ADR-0005: AIエージェントのツール実行戦略

**Status**: Accepted (Phase 2 は直接SQL、将来的にHTTP API経由へ移行)
**Date**: 2026-01-31

---

## Context

AI Agent Cockpit（Phase 2）で kensan-ai に統一エージェントを実装するにあたり、ツール（タスクCRUD、タイムブロック操作等）の実行方法を決める必要がある。

現在、同じPostgreSQLに対して2つのアクセスパスが存在する：

- **Go microservices**: REST API + バリデーション + ビジネスロジック経由でDBにアクセス
- **kensan-ai**: ツール実行時に直接SQLでDBにアクセス（`db/queries/*.py`）

これにより以下の問題がある：

1. **バリデーションの二重管理**: Go側のビジネスロジックをPython側でも再実装する必要がある
2. **スキーマ変更時の二重修正**: テーブル変更時に Go の repository と Python の queries を両方修正
3. **Go APIのバイパス**: microservicesアーキテクチャの意義が薄れる

## 検討した選択肢

### A. 直接SQL（現状）

```
kensan-ai → tools → PostgreSQL 直接
```

- 利点: レイテンシが低い、既存実装を活用できる、追加の依存なし
- 欠点: 二重管理、Goサービスのバリデーションをバイパス

### B. Go microservices の HTTP API 経由

```
kensan-ai → tools → Go microservices (HTTP) → PostgreSQL
```

- 利点: 単一のデータアクセスパス、バリデーション再利用、ツール実装が簡素化
- 欠点: HTTPラウンドトリップのレイテンシ（数ms/回、asyncio.gatherで並列化可能）

### C. MCP Server

```
kensan-ai → Claude API → MCP → Go microservices → PostgreSQL
```

- 利点: 標準プロトコル、他のAIクライアントからも利用可能
- 欠点: Go MCP SDK の成熟度、各サービスへの実装コスト、現時点でオーバーエンジニアリング

## Decision

**Phase 2 は A（直接SQL）で実装し、将来的に B（HTTP API経由）へ移行する。**

### Phase 2（現在）: 直接SQL

- 既存の `db/queries/*.py` パターンを活用して速やかに実装する
- kensan-ai 固有のデータ（`user_memory`, `user_facts`, `ai_contexts`, `ai_interactions`）は直接SQLのまま維持

### 将来の移行: HTTP API 経由

移行のトリガー：
- Go 側のビジネスロジックが複雑化し、kensan-ai での再実装コストが目立ってきた時
- スキーマ変更で二重修正の手間が繰り返し発生した時

移行時の方針：
- ツール実行ロジックを `db/queries/*.py` の直接SQL から `httpx.AsyncClient` による Go API 呼び出しに差し替える
- ツール定義（name, description, input_schema）は kensan-ai に残す（Claude API に渡すAI固有の情報のため）
- kensan-ai 固有テーブル（user_memory, user_facts 等）は引き続き直接SQLでアクセス
- Go 側に不足しているエンドポイント（analytics summary, daily summary 等）は必要に応じて追加
- 読み取りツールの並列呼び出しは `asyncio.gather` で対応

### MCP について

現時点では採用しない。Go MCP SDK の成熟度と、自アプリ内でのみ使用する状況を考慮すると、HTTP API で十分。エコシステムが成熟し、外部AIクライアントからの利用ニーズが出てきた場合に再検討する。

## Consequences

- Phase 2 の実装速度を優先できる
- 二重管理の技術的負債は認識した上で許容する
- 将来の移行パスが明確であり、ツール定義と実行ロジックが分離されているため移行コストは限定的
