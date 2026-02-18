# ADR-0001: マイクロサービス間での共有データベース採用

**ステータス**: 承認済み
**日付**: 2026-01-12
**決定者**: Yu

---

## コンテキスト

Kensanはマイクロサービスアーキテクチャを採用しており、8つの独立したサービスで構成されている。

- user-service (Go, :8081)
- task-service (Go, :8082)
- timeblock-service (Go, :8084)
- routine-service (Go, :8085)
- analytics-service (Go, :8088)
- memo-service (Go, :8090)
- note-service (Go, :8091)
- kensan-ai (Python, :8089)

> **Note**: 本ADR作成時（2026-01-12）は diary-service, record-service, sync-service, ai-service が存在していたが、その後 note-service に統合（diary/record）、sync-service は廃止（Clockify連携廃止）、ai-service は kensan-ai として Python で再実装された。

マイクロサービスのベストプラクティスでは「Database per Service」パターンが推奨されるが、現段階でこのパターンを採用するかどうかを決定する必要がある。

## 決定

**全サービスで単一のPostgreSQLインスタンスを共有する。**

サービス間のデータ連携はHTTP APIやイベント駆動ではなく、共有データベースへの直接アクセスで行う。

```
┌─────────────┐  ┌─────────────┐  ┌─────────────┐
│user-service │  │task-service │  │  他サービス  │
└──────┬──────┘  └──────┬──────┘  └──────┬──────┘
       │                │                │
       └────────────────┼────────────────┘
                        │
                ┌───────▼───────┐
                │  PostgreSQL   │
                │   (共有DB)    │
                └───────────────┘
```

## 理由

### 1. プロジェクトの規模と要件

- 個人向けアプリケーションであり、大規模なスケーリングは想定していない
- 単一ユーザーまたは少数ユーザーでの利用を前提とする
- MVPフェーズでは開発速度を優先する

### 2. 開発・運用コストの削減

- 複数DBの管理が不要
- バックアップ・リストアが単純
- ローカル開発環境の構築が容易

### 3. データ整合性の確保

- トランザクションによる強整合性が保てる
- 複雑な結果整合性（Eventual Consistency）の実装が不要
- JOINによる効率的なクエリが可能（例: analytics-serviceの集計）

### 4. 段階的な移行の可能性

- プロセスは分離されているため、将来的なDB分離への移行パスは確保されている
- サービスごとにrepositoryレイヤーが分離されており、DB接続先の変更は局所的

## 結果

### メリット

- 開発・運用がシンプル
- トランザクション整合性が自然に保たれる
- インフラコストが低い
- ローカル開発が容易（docker-compose一発）

### デメリット

- サービス間の暗黙的な依存（スキーマ変更時に複数サービスへ影響）
- 特定サービスだけのスケールアウトが困難
- DB技術の選択肢が制限される（全サービスPostgreSQL）

### 将来の検討事項

以下の状況が発生した場合、DB分離を再検討する:

1. **パフォーマンス要件**: 特定サービスの負荷が他に影響を与える場合
2. **チーム分割**: 複数チームでの開発でスキーマ変更の調整コストが高くなる場合
3. **技術要件**: 検索（Elasticsearch）やキャッシュ（Redis）など、異なるデータストアが必要になる場合
4. **マルチテナント化**: SaaS化など、テナント分離が必要になる場合

## 参考

- [Microservices Patterns - Database per Service](https://microservices.io/patterns/data/database-per-service.html)
- [Shared Database Pattern](https://microservices.io/patterns/data/shared-database.html)
