// ============================================
// Goal (目標)
// ============================================
export type GoalStatus = 'active' | 'completed' | 'archived'

export interface Goal {
  id: string
  name: string
  description?: string
  color: string // Hex color, e.g., "#0EA5E9"
  status: GoalStatus
  sortOrder: number
  createdAt: Date
  updatedAt: Date
}

// ============================================
// Milestone (マイルストーン) - 旧Project
// ============================================
export type MilestoneStatus = 'active' | 'completed' | 'archived'

export interface Milestone {
  id: string
  goalId: string // 親Goal
  name: string
  description?: string
  startDate?: string // YYYY-MM-DD, 開始日
  targetDate?: string // YYYY-MM-DD, 期限
  status: MilestoneStatus
  createdAt: Date
  updatedAt: Date
}

// ============================================
// Tag (タグ) - 集計用の自由タグ
// ============================================
export type TagType = 'task' | 'note'
export type TagCategory = 'general' | 'trait' | 'tech' | 'project'

export interface Tag {
  id: string
  name: string
  color: string // Hex color
  type: TagType
  category: TagCategory
  pinned: boolean
  usageCount: number
  createdAt: Date
  updatedAt: Date
}

// ============================================
// Task (タスク)
// ============================================
export type TaskFrequency = 'daily' | 'weekly' | 'custom'

export interface Task {
  id: string
  name: string
  milestoneId?: string // 任意（目標なしタスクも可）
  parentTaskId?: string
  tagIds?: string[] // 複数タグ
  estimatedMinutes?: number
  completed: boolean
  dueDate?: string // YYYY-MM-DD
  sortOrder: number // 並び順（小さいほど上）
  // 定期タスク設定
  frequency?: TaskFrequency // undefined = 単発タスク
  daysOfWeek?: number[] // 0=日, 1=月, ..., 6=土 (frequencyがcustomの時)
  createdAt: Date
  updatedAt: Date
}

// ============================================
// TimeBlock (タイムブロック - 計画)
// ============================================
export interface TimeBlock {
  id: string
  startDatetime: string // ISO 8601 UTC (e.g., "2026-01-20T15:00:00Z")
  endDatetime: string // ISO 8601 UTC
  taskId?: string
  taskName: string
  // 非正規化フィールド（表示用）
  milestoneId?: string
  milestoneName?: string
  goalId?: string
  goalName?: string
  goalColor?: string
  tagIds?: string[]
}

// ============================================
// TimeEntry (時間記録 - 実績)
// ============================================
export interface TimeEntry {
  id: string
  startDatetime: string // ISO 8601 UTC (e.g., "2026-01-20T15:00:00Z")
  endDatetime: string // ISO 8601 UTC
  taskId?: string
  taskName: string
  // 非正規化フィールド（表示用）
  milestoneId?: string
  milestoneName?: string
  goalId?: string
  goalName?: string
  goalColor?: string
  tagIds?: string[]
  description?: string
}

// ============================================
// Todo (単発タスク + 繰り返しタスク統合)
// ============================================
export type TodoFrequency = 'daily' | 'weekly' | 'monthly' | 'custom'

export interface Todo {
  id: string
  userId: string
  name: string
  frequency?: TodoFrequency // undefined = 単発タスク
  daysOfWeek?: number[] // 0=日, 1=月, ..., 6=土
  dueDate?: string // YYYY-MM-DD, 単発タスクの期日
  estimatedMinutes?: number
  tagIds?: string[]
  enabled: boolean
  createdAt: Date
  updatedAt: Date
}

// 特定日付の完了状態付きTodo
export interface TodoWithStatus extends Todo {
  completedToday: boolean
  completedAt?: Date
  isOverdue: boolean // 単発タスクで期日超過
}

// 完了記録
export interface TodoCompletion {
  id: string
  todoId: string
  completedDate: string // YYYY-MM-DD
  completedAt: Date
}

// ============================================
// Note (統合ノート - 日記・学習記録・一般・読書レビュー等)
// ============================================
export type NoteType = string
export type NoteFormat = 'markdown' | 'drawio'

// ============================================
// NoteTypeConfig (ノートタイプ設定 - DB駆動)
// ============================================
export interface TypeConstraints {
  dateRequired: boolean
  titleRequired: boolean
  contentRequired: boolean
  dailyUnique: boolean
}

export interface FieldSchema {
  key: string
  label: string
  labelEn?: string
  type: 'string' | 'integer' | 'float' | 'boolean' | 'enum' | 'date' | 'url'
  required: boolean
  constraints?: Record<string, unknown>
}

export interface NoteTypeConfig {
  id: string
  slug: string
  displayName: string
  displayNameEn?: string
  description?: string
  icon: string
  color: string
  constraints: TypeConstraints
  metadataSchema: FieldSchema[]
  sortOrder: number
  isSystem: boolean
  isActive: boolean
}

// ============================================
// NoteMetadata (ノートメタデータ)
// ============================================
export interface NoteMetadataItem {
  id: string
  noteId: string
  key: string
  value?: string
  createdAt: Date
  updatedAt: Date
}

// コンテンツタイプ（複数コンテンツ対応）
export type ContentType = 'markdown' | 'drawio' | 'image' | 'pdf' | 'code' | 'mindmap'
export type StorageProvider = 'minio' | 'r2' | 's3' | 'local'
export type IndexStatus = 'pending' | 'processing' | 'indexed' | 'failed'

// NoteContent - ノート内の個別コンテンツ
export interface NoteContent {
  id: string
  noteId: string
  contentType: ContentType
  content?: string // インラインコンテンツ（小さい場合）
  storageProvider?: StorageProvider // ストレージ種別
  storageKey?: string // ストレージ内のキー
  fileName?: string
  mimeType?: string
  fileSizeBytes?: number
  checksum?: string
  thumbnailBase64?: string // 画像のサムネイル
  sortOrder: number
  metadata?: Record<string, unknown>
  createdAt: Date
  updatedAt: Date
}

export interface Note {
  id: string
  userId: string
  type: NoteType
  title?: string
  content: string // 後方互換性のため残す
  format: NoteFormat // 後方互換性のため残す
  contents?: NoteContent[] // 複数コンテンツ
  date?: string // YYYY-MM-DD, diary/learningでは必須
  taskId?: string // 関連タスク
  milestoneId?: string
  milestoneName?: string
  goalId?: string
  goalName?: string
  goalColor?: string
  tagIds?: string[] // note_tagsテーブル経由
  metadata?: NoteMetadataItem[] // note_metadataテーブル経由
  relatedTimeEntryIds?: string[]
  fileUrl?: string // drawioの場合のファイルURL（後方互換）
  indexStatus?: IndexStatus
  indexedAt?: Date
  archived: boolean
  createdAt: Date
  updatedAt: Date
}

// Noteの一覧表示用（contentなし）
export interface NoteListItem {
  id: string
  userId: string
  type: NoteType
  title?: string
  format: NoteFormat
  date?: string
  taskId?: string
  milestoneId?: string
  milestoneName?: string
  goalId?: string
  goalName?: string
  goalColor?: string
  tagIds?: string[]
  relatedTimeEntryIds?: string[]
  fileUrl?: string
  archived: boolean
  createdAt: Date
  updatedAt: Date
}

// 検索結果
export interface NoteSearchResult {
  note: NoteListItem
  score: number
}

// ============================================
// UserSettings (ユーザー設定)
// ============================================
export type Theme = 'light' | 'dark' | 'system'

export interface UserSettings {
  timezone: string
  theme: Theme
  isConfigured: boolean
  userName: string
}

// ============================================
// DailyPlan (日次計画)
// ============================================
export interface DailyPlan {
  date: string // YYYY-MM-DD
  timeBlocks: TimeBlock[]
  createdAt: Date
  updatedAt: Date
}

// ============================================
// WeeklySummary (週次サマリー)
// ============================================
export interface GoalSummary {
  id: string
  name: string
  color: string
  minutes: number
}

export interface TagSummary {
  id: string
  name: string
  color: string
  minutes: number
}

export interface MilestoneSummary {
  id: string
  name: string
  goalId: string
  minutes: number
}

export interface WeeklySummary {
  weekStart: string // YYYY-MM-DD
  weekEnd: string // YYYY-MM-DD
  totalMinutes: number
  byGoal: GoalSummary[]
  byTag: TagSummary[]
  byMilestone: MilestoneSummary[]
  completedTasks: number
  plannedVsActual: {
    planned: number
    actual: number
  }
}

// ============================================
// AIReviewReport (AI振り返り)
// ============================================
export type TaskEvaluationStatus = 'achieved' | 'good' | 'partial' | 'missed'

export interface TaskEvaluation {
  taskName: string
  status: TaskEvaluationStatus
  comment: string
}

export interface TimeEvaluation {
  goalName: string
  goalColor?: string
  actualMinutes: number
  targetMinutes?: number
  comment: string
}

export interface LearningTopic {
  topic: string
  goalName?: string
  goalColor?: string
  depth: 'deep' | 'moderate' | 'light'
  insight: string
}

export interface LearningSummaryData {
  overview: string
  topics: LearningTopic[]
  weeklyPattern?: string
  goalConnection?: string
}

export interface SuggestedAction {
  label: string
  description: string
  type: 'chat'
  prompt: string
}

export interface AIReviewReport {
  id: string
  periodStart: string
  periodEnd: string
  // 4セクション構造
  taskEvaluations: TaskEvaluation[]
  taskSummary?: string
  timeEvaluations: TimeEvaluation[]
  learningSummary?: string
  learningSummaryData?: LearningSummaryData
  goodPoints: string[]
  improvementPoints: string[]
  advice: string[]
  // おまけ: 日記へのひとこと
  diaryFeedback?: string
  // アクション提案
  suggestedActions?: SuggestedAction[]
  // 後方互換
  summary: string
  createdAt: Date
}

// ============================================
// EntityMemo (エンティティメモ)
// ============================================
export type EntityType = 'goal' | 'milestone' | 'task'

export interface EntityMemo {
  id: string
  userId: string
  entityType: EntityType
  entityId: string
  content: string
  pinned: boolean
  createdAt: Date
  updatedAt: Date
}

// ============================================
// AI Planning (AI計画提案)
// ============================================
export interface PlanningInsight {
  category: 'productivity' | 'goal' | 'planning' | 'alert'
  title: string
  description: string
}

export interface ProposedBlock {
  taskId: string | null
  taskName: string
  goalId: string | null
  goalName: string
  goalColor: string
  startTime: string // HH:mm
  endTime: string // HH:mm
  reason: string
}

export interface TaskPrioritySuggestion {
  taskId: string
  taskName: string
  suggestedAction: 'today' | 'defer' | 'split'
  reason: string
}

export interface PlanningAlert {
  type: 'goal_stalled' | 'overdue' | 'overcommit'
  message: string
}

export interface YesterdayReview {
  summary: string
  highlights: string[]
  learningConnections: string[]
}

export interface AIPlanningResult {
  message?: string
  yesterdayReview?: YesterdayReview
  insights: PlanningInsight[]
  proposedBlocks: ProposedBlock[]
  taskPriorities: TaskPrioritySuggestion[]
  alerts: PlanningAlert[]
}

// ============================================
// デフォルトカラーパレット
// ============================================
export const DEFAULT_COLORS = [
  '#0EA5E9', // Sky blue (Brand)
  '#10B981', // Emerald
  '#F59E0B', // Amber
  '#EF4444', // Red
  '#8B5CF6', // Violet
  '#EC4899', // Pink
  '#06B6D4', // Cyan
  '#84CC16', // Lime
] as const

