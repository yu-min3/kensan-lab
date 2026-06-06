---
description: Database schema conventions and migration rules
globs: backend/migrations/**
---

# Database Conventions

## Core Principles (厳守)

1. **Multi-tenancy**: 全テーブルに `user_id UUID NOT NULL` カラム。全クエリに `WHERE user_id = $1`。
2. **UUID Primary Key**: `uuid-ossp` extension の `uuid_generate_v4()`。
3. **Timestamps**: `created_at TIMESTAMPTZ DEFAULT NOW()`, `updated_at TIMESTAMPTZ DEFAULT NOW()`。
4. **Auto-update trigger**: 全テーブルに `update_updated_at()` トリガー設置。

## Migration File Naming

```
backend/migrations/NNN_description.sql
```

NNN は連番（3桁、既存の最大値 + 1）。

## Denormalization Pattern

TimeBlock, TimeEntry, Note に goal/milestone 情報を複製:
- `goal_id`, `goal_name`, `goal_color`
- `milestone_id`, `milestone_name`

理由: 一覧クエリで JOIN 回避。同期トリガーが自動更新。

## Index Strategy

- 複合 index: `(user_id, date)`, `(user_id, status)`
- GIN index: 配列カラム (`tag_ids`)
- Full-text search: `to_tsvector('simple', title || ' ' || content)`
- Foreign key: `ON DELETE CASCADE`

## Note Types (Data-driven)

ノートタイプは `note_types` テーブルで管理。ハードコードしない。
新タイプ追加はマイグレーションで `note_types` にシード。
