# ADR-0007: 非正規化戦略（Goal/Milestone/Task の表示データ）

**Status**: Accepted
**Date**: 2026-02-01
**Related**: ADR-0004 (データモデル再設計)

---

## Context

ADR-0004 で導入した Goal / Milestone / Task 階層において、TimeBlock・TimeEntry・RunningTimer・Note の一覧表示時に Goal 名や色などの情報が必要になる。

取得方法として以下の 2 つの選択肢がある：

1. **JOIN**: 正規化を維持し、クエリ時に goals / milestones / tasks テーブルを結合
2. **非正規化**: 表示用フィールド（goal_name, goal_color, milestone_name, task_name）を各テーブルに直接保存

## Decision

**非正規化を採用する。** 各テーブルに表示用フィールドを保持し、DB トリガーで同期する。

### 対象テーブルと非正規化フィールド

| テーブル | 非正規化フィールド |
|----------|---------------------|
| `time_blocks` | `goal_id`, `goal_name`, `goal_color`, `milestone_id`, `milestone_name`, `task_id`, `task_name` |
| `time_entries` | 同上 |
| `running_timers` | 同上 |
| `notes` | `goal_id`, `goal_name`, `goal_color`, `milestone_id`, `milestone_name` |

### 同期メカニズム

`backend/migrations/033_denormalized_field_sync_triggers.sql` に AFTER UPDATE トリガーを定義：

- `sync_goal_denormalized_fields()` — Goal の name/color 変更時に全対象テーブルを更新
- `sync_milestone_denormalized_fields()` — Milestone の name 変更時に同上
- `sync_task_denormalized_fields()` — Task の name 変更時に同上

`IS DISTINCT FROM` で実際に値が変わった場合のみ UPDATE を発行。

## Rationale

### 非正規化を選んだ理由

1. **読み取り頻度 >> 書き込み頻度**: タイムライン表示や一覧取得は頻繁に発生するが、Goal 名や色の変更はまれ。読み取り最適化が妥当
2. **マイクロサービス構成との相性**: 現在は共有 DB だが、将来サービスごとに DB を分離する場合、JOIN ではサービス間通信が必要になる。非正規化なら各サービスが自テーブルだけで完結する
3. **履歴保全**: Goal や Milestone が削除されても、過去の TimeEntry に表示データが残る（ON DELETE SET NULL で ID は消えるが名前・色は保持）
4. **クエリの単純さ**: 一覧取得が単純な SELECT で済み、複数テーブル JOIN によるクエリ複雑化を回避

### JOIN 方式との比較

| 観点 | 非正規化 | JOIN |
|------|----------|------|
| 読み取り性能 | 高い（JOIN 不要） | JOIN コスト発生 |
| 書き込み性能 | トリガーによる追加 UPDATE | シンプル（ID のみ保存） |
| データ整合性 | トリガーで担保（DB レベル） | 常に最新（Single Source of Truth） |
| クエリ複雑度 | 低い | 高い（特にタイムライン表示） |
| メンテナンスコスト | テーブル追加時にトリガー更新が必要 | 低い |
| サービス分離対応 | 容易（自テーブルで完結） | DB 共有または API 呼び出しが必要 |
| 履歴保全 | 自然に残る | 削除時にデータ消失のリスク |

## Consequences

### Positive

- 一覧・タイムライン表示のクエリがシンプルかつ高速
- 将来の DB 分離に対する先行投資
- Go サービス・AI サービスどちらの書き込みでも DB トリガーで整合性を保証

### Negative

- 新しい非正規化対象テーブルを追加する際、トリガーの更新が必要
- Goal/Milestone/Task の名前変更時にトリガーが複数テーブルを UPDATE するコスト（ただし発生頻度は低い）
- ストレージ使用量が微増

### Risks

- トリガー更新の漏れによるデータ不整合。対策: テーブル追加時のチェックリストに含める
