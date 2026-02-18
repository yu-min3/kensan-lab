# ADR-0004: データモデル再設計

**Status**: Accepted
**Date**: 2026-01-22
**Related**: ADR-0007 (非正規化戦略)

---

## Context

現在のデータモデルには以下の問題がある：

1. **GoalTag が曖昧** - 「目標」と「タグ」の両方の役割を持っている
2. **Project の位置づけが不明確** - Goal配下なのか独立なのか
3. **柔軟性の欠如** - GoalTag が固定4種（GK, OSS, Output, Other）で拡張不可
4. **色の設定不可** - プロジェクトやタグに色を設定できない

## Decision

以下の新しいデータモデルを採用する。

### 新構造

```
Goal (目標) [色付き、任意]
    └── Milestone (マイルストーン) [任意、期限付き]
          └── Task (タスク)
                └── TimeBlock / TimeEntry

Task (目標なしも可)
    └── Tag (自由タグ) [色付き、複数可]
```

### エンティティ定義

#### Goal (目標)
- ユーザーが自由に作成
- 達成したい大きな状態を表す
- 色を設定可能
- 例: "Golden Kubestronaut取得", "英語力向上"

#### Milestone (マイルストーン)
- Goal配下に作成（旧Projectを統合）
- 期限（targetDate）を持つ
- 達成状況を管理
- 例: "ICA合格 (2026/03)", "TOEIC 800点 (2026/06)"

#### Task (タスク)
- 実際の作業単位
- Milestone配下 または 独立（目標なし）で作成可能
- 例: "Traffic Management 学習", "ジム", "技術記事読む"

#### Tag (タグ)
- ユーザーが自由に作成・管理
- Task / TimeEntry に複数付与可能
- 色を設定可能
- 集計の軸として使用
- 例: "開発", "Input", "運動", "読書", "OSS"

### 変更点まとめ

| Before | After | 備考 |
|--------|-------|------|
| GoalTag (固定4種) | Goal (自由作成) | 目標として独立 |
| Project | Milestone | 期限付きマイルストーンに統合 |
| - | Tag | 新規。集計用の自由タグ |

## New Type Definitions

```typescript
// 目標
interface Goal {
  id: string
  name: string
  description?: string
  color: string           // Hex color, e.g., "#0EA5E9"
  isArchived: boolean
  createdAt: Date
  updatedAt: Date
}

// マイルストーン (旧Project)
interface Milestone {
  id: string
  goalId: string          // 親Goal
  name: string
  description?: string
  targetDate?: string     // YYYY-MM-DD, 期限
  status: 'active' | 'completed' | 'archived'
  createdAt: Date
  updatedAt: Date
}

// タスク
interface Task {
  id: string
  name: string
  milestoneId?: string    // 任意（目標なしタスクも可）
  estimatedMinutes?: number
  completed: boolean
  dueDate?: string        // YYYY-MM-DD
  createdAt: Date
  updatedAt: Date
}

// タグ
interface Tag {
  id: string
  name: string
  color: string           // Hex color
  createdAt: Date
}

// タスク-タグ関連 (多対多)
interface TaskTag {
  taskId: string
  tagId: string
}

// タイムブロック（計画）
interface TimeBlock {
  id: string
  date: string            // YYYY-MM-DD
  startTime: string       // HH:mm
  endTime: string         // HH:mm
  taskId?: string
  taskName: string        // 非正規化（表示用）
  milestoneId?: string    // 非正規化
  milestoneName?: string  // 非正規化
  goalId?: string         // 非正規化
  goalName?: string       // 非正規化
  goalColor?: string      // 非正規化
  isRoutine: boolean
  routineTaskId?: string
}

// 時間記録（実績）
interface TimeEntry {
  id: string
  date: string            // YYYY-MM-DD
  startTime: string       // HH:mm
  endTime: string         // HH:mm
  taskId?: string
  taskName: string
  milestoneId?: string
  milestoneName?: string
  goalId?: string
  goalName?: string
  goalColor?: string
  tagIds?: string[]       // 集計用
  description?: string
}

// 週次サマリー
interface WeeklySummary {
  weekStart: string
  weekEnd: string
  totalMinutes: number
  byGoal: Record<string, { name: string; color: string; minutes: number }>
  byTag: Record<string, { name: string; color: string; minutes: number }>
  byMilestone: Record<string, { name: string; minutes: number }>
  completedTasks: number
  plannedVsActual: {
    planned: number
    actual: number
  }
}
```

## UI Changes

### 今日の予定 / タイムライン
- GoalTag バッジ → **Goal名** + **Goal色** で表示
- 目標なしタスクはグレー表示

### 集計・分析画面
- **Goal別** の時間集計（色付きバー）
- **Tag別** の時間集計（色付きバー）
- 切り替え可能なビュー

### 設定画面
- Goal 管理（CRUD、色設定）
- Tag 管理（CRUD、色設定）
- Milestone 管理（Goal選択後に作成）

## Migration

### データ移行
1. 既存の GoalTag → Goal に変換
   - GK → Goal "Golden Kubestronaut" (色: #0EA5E9)
   - OSS → Goal "OSS活動" (色: #10B981)
   - Output → Goal "アウトプット" (色: #F59E0B)
   - Other → Tag "その他" (色: #6B7280)

2. 既存の Project → Milestone に変換
   - Project.goalTag から親Goal を設定

3. Tag のデフォルト作成
   - "開発", "Input", "運動", "読書" 等

## Consequences

### Positive
- ユーザーが自由に目標・タグを設定できる
- 色による視覚的な識別が可能
- 柔軟な集計が可能（Goal別、Tag別）
- 目標なしの気軽なタスクも管理可能

### Negative
- データモデルの大幅変更が必要
- 既存データの移行が必要
- UI全体の修正が必要

### Neutral
- 学習コストは低い（直感的な構造）
