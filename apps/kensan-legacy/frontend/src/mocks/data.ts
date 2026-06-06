// Mock data for MSW handlers
// Persona: 田中翔太（30歳・Go+GCPバックエンドエンジニア）
// This data is mutable to allow CRUD operations in development

import type {
  Goal,
  Milestone,
  Tag,
  Task,
  TimeBlock,
  TimeEntry,
  Note,
  UserSettings,
  AIReviewReport,
  WeeklySummary,
} from '@/types'
import { format, subDays, addDays } from 'date-fns'

// Helper functions for date calculations
export const getToday = () => format(new Date(), 'yyyy-MM-dd')
export const getYesterday = () => format(subDays(new Date(), 1), 'yyyy-MM-dd')
export const getTomorrow = () => format(addDays(new Date(), 1), 'yyyy-MM-dd')

const today = getToday()
const yesterday = getYesterday()
const tomorrow = getTomorrow()

// Helper to create UTC ISO datetime from local date (YYYY-MM-DD) and time (HH:mm) in Asia/Tokyo
// Uses localToUtcDatetime for proper Intl-based timezone handling
import { localToUtcDatetime } from '@/lib/timezone'

function toUtcIso(localDate: string, localTime: string): string {
  return localToUtcDatetime(localDate, localTime, 'Asia/Tokyo')
}

// ============================================
// Goals (目標)
// ============================================
export let goals: Goal[] = [
  {
    id: 'goal-cert',
    name: 'GCPスキルアップ',
    description: 'GCP認定資格を取得し、クラウドアーキテクチャの知識を深める',
    color: '#0EA5E9',
    status: 'active',
    sortOrder: 0,
    createdAt: subDays(new Date(), 90),
    updatedAt: subDays(new Date(), 3),
  },
  {
    id: 'goal-output',
    name: '技術アウトプット',
    description: '技術ブログとLT登壇でアウトプットを習慣化する',
    color: '#F59E0B',
    status: 'active',
    sortOrder: 1,
    createdAt: subDays(new Date(), 60),
    updatedAt: subDays(new Date(), 7),
  },
  {
    id: 'goal-product',
    name: '個人開発プロダクト',
    description: '個人開発SaaSをMVPリリースしてポートフォリオを強化する',
    color: '#10B981',
    status: 'active',
    sortOrder: 2,
    createdAt: subDays(new Date(), 45),
    updatedAt: subDays(new Date(), 1),
  },
]

// ============================================
// Milestones (マイルストーン)
// ============================================
export let milestones: Milestone[] = [
  {
    id: 'ms-ace',
    goalId: 'goal-cert',
    name: 'ACE合格',
    description: 'Associate Cloud Engineer 認定取得',
    targetDate: '2026-04-30',
    status: 'active',
    createdAt: subDays(new Date(), 90),
    updatedAt: subDays(new Date(), 3),
  },
  {
    id: 'ms-pcd',
    goalId: 'goal-cert',
    name: 'PCD合格',
    description: 'Professional Cloud Developer 認定取得',
    targetDate: '2026-07-31',
    status: 'active',
    createdAt: subDays(new Date(), 60),
    updatedAt: subDays(new Date(), 14),
  },
  {
    id: 'ms-blog',
    goalId: 'goal-output',
    name: '技術ブログ月4本',
    description: '毎月4本の技術ブログ記事を投稿する',
    status: 'active',
    createdAt: subDays(new Date(), 30),
    updatedAt: subDays(new Date(), 7),
  },
  {
    id: 'ms-lt',
    goalId: 'goal-output',
    name: 'LT登壇 年3回',
    description: '社外勉強会でLT登壇する',
    targetDate: '2026-12-31',
    status: 'active',
    createdAt: subDays(new Date(), 30),
    updatedAt: subDays(new Date(), 14),
  },
  {
    id: 'ms-mvp',
    goalId: 'goal-product',
    name: 'SaaS MVPリリース',
    description: '個人開発SaaSの最小機能をリリースする',
    targetDate: '2026-06-30',
    status: 'active',
    createdAt: subDays(new Date(), 45),
    updatedAt: subDays(new Date(), 1),
  },
]

// ============================================
// Tags (タグ) - タスク用
// ============================================
export let tags: Tag[] = [
  { id: 'tag-dev', name: '開発', color: '#8B5CF6', type: 'task', category: 'tech', pinned: true, usageCount: 45, createdAt: subDays(new Date(), 90), updatedAt: subDays(new Date(), 1) },
  { id: 'tag-input', name: 'Input', color: '#06B6D4', type: 'task', category: 'general', pinned: true, usageCount: 32, createdAt: subDays(new Date(), 90), updatedAt: subDays(new Date(), 2) },
  { id: 'tag-exercise', name: '運動', color: '#EF4444', type: 'task', category: 'general', pinned: false, usageCount: 18, createdAt: subDays(new Date(), 90), updatedAt: subDays(new Date(), 5) },
  { id: 'tag-reading', name: '読書', color: '#84CC16', type: 'task', category: 'general', pinned: false, usageCount: 12, createdAt: subDays(new Date(), 90), updatedAt: subDays(new Date(), 3) },
  { id: 'tag-review', name: '振り返り', color: '#EC4899', type: 'task', category: 'trait', pinned: true, usageCount: 8, createdAt: subDays(new Date(), 90), updatedAt: subDays(new Date(), 7) },
  { id: 'tag-gcp', name: 'GCP', color: '#4285F4', type: 'task', category: 'tech', pinned: true, usageCount: 28, createdAt: subDays(new Date(), 60), updatedAt: subDays(new Date(), 1) },
  { id: 'tag-writing', name: '執筆', color: '#F59E0B', type: 'task', category: 'trait', pinned: false, usageCount: 15, createdAt: subDays(new Date(), 45), updatedAt: subDays(new Date(), 3) },
]

// ============================================
// Note Tags (ノートタグ) - ノート専用
// ============================================
export let noteTags: Tag[] = [
  // trait
  { id: 'ntag-reflection', name: '振り返り', color: '#EC4899', type: 'note', category: 'trait', pinned: true, usageCount: 15, createdAt: subDays(new Date(), 300), updatedAt: subDays(new Date(), 1) },
  // general
  { id: 'ntag-idea', name: 'アイデア', color: '#F59E0B', type: 'note', category: 'general', pinned: false, usageCount: 8, createdAt: subDays(new Date(), 300), updatedAt: subDays(new Date(), 5) },
  { id: 'ntag-memo', name: 'メモ', color: '#84CC16', type: 'note', category: 'general', pinned: false, usageCount: 10, createdAt: subDays(new Date(), 300), updatedAt: subDays(new Date(), 3) },
  // tech
  { id: 'ntag-go', name: 'Go', color: '#00ADD8', type: 'note', category: 'tech', pinned: true, usageCount: 14, createdAt: subDays(new Date(), 300), updatedAt: subDays(new Date(), 2) },
  { id: 'ntag-gcp', name: 'GCP', color: '#4285F4', type: 'note', category: 'tech', pinned: true, usageCount: 11, createdAt: subDays(new Date(), 300), updatedAt: subDays(new Date(), 1) },
  { id: 'ntag-aws', name: 'AWS', color: '#FF9900', type: 'note', category: 'tech', pinned: false, usageCount: 3, createdAt: subDays(new Date(), 300), updatedAt: subDays(new Date(), 150) },
  { id: 'ntag-nextjs', name: 'Next.js', color: '#171717', type: 'note', category: 'tech', pinned: false, usageCount: 2, createdAt: subDays(new Date(), 90), updatedAt: subDays(new Date(), 2) },
  { id: 'ntag-k8s', name: 'Kubernetes', color: '#326CE5', type: 'note', category: 'tech', pinned: false, usageCount: 3, createdAt: subDays(new Date(), 300), updatedAt: subDays(new Date(), 4) },
  { id: 'ntag-linux', name: 'Linux', color: '#E8B30B', type: 'note', category: 'tech', pinned: false, usageCount: 2, createdAt: subDays(new Date(), 300), updatedAt: subDays(new Date(), 100) },
  { id: 'ntag-genai', name: '生成AI', color: '#A855F7', type: 'note', category: 'tech', pinned: false, usageCount: 2, createdAt: subDays(new Date(), 250), updatedAt: subDays(new Date(), 120) },
  { id: 'ntag-ml', name: 'ML', color: '#F97316', type: 'note', category: 'tech', pinned: false, usageCount: 1, createdAt: subDays(new Date(), 300), updatedAt: subDays(new Date(), 180) },
  { id: 'ntag-docker', name: 'Docker', color: '#2496ED', type: 'note', category: 'tech', pinned: false, usageCount: 3, createdAt: subDays(new Date(), 300), updatedAt: subDays(new Date(), 90) },
  { id: 'ntag-api', name: 'API設計', color: '#14B8A6', type: 'note', category: 'tech', pinned: true, usageCount: 6, createdAt: subDays(new Date(), 300), updatedAt: subDays(new Date(), 3) },
  { id: 'ntag-db', name: 'データベース', color: '#6366F1', type: 'note', category: 'tech', pinned: false, usageCount: 3, createdAt: subDays(new Date(), 300), updatedAt: subDays(new Date(), 160) },
  { id: 'ntag-security', name: 'セキュリティ', color: '#EF4444', type: 'note', category: 'tech', pinned: false, usageCount: 2, createdAt: subDays(new Date(), 300), updatedAt: subDays(new Date(), 1) },
  { id: 'ntag-terraform', name: 'Terraform', color: '#7B42BC', type: 'note', category: 'tech', pinned: false, usageCount: 1, createdAt: subDays(new Date(), 300), updatedAt: subDays(new Date(), 210) },
  { id: 'ntag-cicd', name: 'CI/CD', color: '#2088FF', type: 'note', category: 'tech', pinned: false, usageCount: 2, createdAt: subDays(new Date(), 300), updatedAt: subDays(new Date(), 180) },
  // project
  { id: 'ntag-saas', name: '個人開発SaaS', color: '#10B981', type: 'note', category: 'project', pinned: false, usageCount: 3, createdAt: subDays(new Date(), 90), updatedAt: subDays(new Date(), 2) },
]

// ============================================
// Tasks (タスク)
// ============================================
export let tasks: Task[] = [
  // ACE関連タスク
  {
    id: 't-ace',
    name: 'ACE試験対策',
    milestoneId: 'ms-ace',
    tagIds: ['tag-input', 'tag-gcp'],
    completed: false,
    sortOrder: 0,
    createdAt: subDays(new Date(), 30),
    updatedAt: subDays(new Date(), 1),
  },
  {
    id: 't-ace-1',
    name: 'GCP基礎・Compute/GKE復習',
    milestoneId: 'ms-ace',
    parentTaskId: 't-ace',
    tagIds: ['tag-input', 'tag-gcp'],
    completed: true,
    sortOrder: 0,
    createdAt: subDays(new Date(), 30),
    updatedAt: subDays(new Date(), 5),
  },
  {
    id: 't-ace-2',
    name: 'IAM・セキュリティ',
    milestoneId: 'ms-ace',
    parentTaskId: 't-ace',
    tagIds: ['tag-input', 'tag-gcp'],
    completed: false,
    sortOrder: 1,
    createdAt: subDays(new Date(), 14),
    updatedAt: subDays(new Date(), 1),
  },
  {
    id: 't-ace-3',
    name: '模擬試験',
    milestoneId: 'ms-ace',
    parentTaskId: 't-ace',
    tagIds: ['tag-input', 'tag-gcp'],
    completed: false,
    sortOrder: 2,
    createdAt: subDays(new Date(), 14),
    updatedAt: subDays(new Date(), 7),
  },
  // ブログ関連
  {
    id: 't-blog-mw',
    name: '「GoのHTTPミドルウェア設計パターン」執筆',
    milestoneId: 'ms-blog',
    tagIds: ['tag-dev', 'tag-writing'],
    completed: false,
    dueDate: format(addDays(new Date(), 5), 'yyyy-MM-dd'),
    sortOrder: 0,
    createdAt: subDays(new Date(), 7),
    updatedAt: subDays(new Date(), 2),
  },
  {
    id: 't-blog-cicd',
    name: '「GCPで始めるCI/CD」記事',
    milestoneId: 'ms-blog',
    tagIds: ['tag-gcp', 'tag-writing'],
    completed: false,
    sortOrder: 1,
    createdAt: subDays(new Date(), 3),
    updatedAt: subDays(new Date(), 3),
  },
  // 個人開発
  {
    id: 't-mvp',
    name: 'SaaS MVP開発',
    milestoneId: 'ms-mvp',
    tagIds: ['tag-dev'],
    completed: false,
    sortOrder: 0,
    createdAt: subDays(new Date(), 21),
    updatedAt: subDays(new Date(), 1),
  },
  {
    id: 't-mvp-db',
    name: 'DB設計',
    milestoneId: 'ms-mvp',
    parentTaskId: 't-mvp',
    tagIds: ['tag-dev'],
    completed: true,
    sortOrder: 0,
    createdAt: subDays(new Date(), 21),
    updatedAt: subDays(new Date(), 10),
  },
  {
    id: 't-mvp-api',
    name: 'API実装',
    milestoneId: 'ms-mvp',
    parentTaskId: 't-mvp',
    tagIds: ['tag-dev'],
    completed: false,
    sortOrder: 1,
    createdAt: subDays(new Date(), 14),
    updatedAt: subDays(new Date(), 1),
  },
  {
    id: 't-mvp-front',
    name: 'フロントエンド（Next.js）',
    milestoneId: 'ms-mvp',
    parentTaskId: 't-mvp',
    tagIds: ['tag-dev'],
    completed: false,
    sortOrder: 2,
    createdAt: subDays(new Date(), 7),
    updatedAt: subDays(new Date(), 3),
  },
  // 目標なしタスク
  {
    id: 't-gym',
    name: 'ジム（筋トレ）',
    tagIds: ['tag-exercise'],
    completed: false,
    sortOrder: 0,
    createdAt: subDays(new Date(), 30),
    updatedAt: subDays(new Date(), 1),
  },
  {
    id: 't-reading',
    name: '技術書読書',
    tagIds: ['tag-reading', 'tag-input'],
    completed: false,
    sortOrder: 1,
    createdAt: subDays(new Date(), 30),
    updatedAt: subDays(new Date(), 3),
  },
]

// ============================================
// Time blocks (計画)
// ============================================
export let timeBlocks: TimeBlock[] = [
  {
    id: 'tb1',
    startDatetime: toUtcIso(today, '19:00'),
    endDatetime: toUtcIso(today, '20:00'),
    taskId: 't-ace-2',
    taskName: 'ACE試験対策 - IAM・セキュリティ',
    milestoneId: 'ms-ace',
    milestoneName: 'ACE合格',
    goalId: 'goal-cert',
    goalName: 'GCPスキルアップ',
    goalColor: '#0EA5E9',
    tagIds: ['tag-input', 'tag-gcp'],
  },
  {
    id: 'tb2',
    startDatetime: toUtcIso(today, '20:00'),
    endDatetime: toUtcIso(today, '20:30'),
    taskId: 't-blog-mw',
    taskName: '「GoのHTTPミドルウェア設計パターン」執筆',
    milestoneId: 'ms-blog',
    milestoneName: '技術ブログ月4本',
    goalId: 'goal-output',
    goalName: '技術アウトプット',
    goalColor: '#F59E0B',
    tagIds: ['tag-dev', 'tag-writing'],
  },
  {
    id: 'tb3',
    startDatetime: toUtcIso(today, '20:30'),
    endDatetime: toUtcIso(today, '21:30'),
    taskId: 't-mvp-api',
    taskName: 'SaaS MVP - API実装',
    milestoneId: 'ms-mvp',
    milestoneName: 'SaaS MVPリリース',
    goalId: 'goal-product',
    goalName: '個人開発プロダクト',
    goalColor: '#10B981',
    tagIds: ['tag-dev'],
  },
  // Tomorrow
  {
    id: 'tb-tm1',
    startDatetime: toUtcIso(tomorrow, '19:00'),
    endDatetime: toUtcIso(tomorrow, '20:30'),
    taskId: 't-ace-2',
    taskName: 'ACE試験対策 - IAM・セキュリティ',
    milestoneId: 'ms-ace',
    milestoneName: 'ACE合格',
    goalId: 'goal-cert',
    goalName: 'GCPスキルアップ',
    goalColor: '#0EA5E9',
    tagIds: ['tag-input', 'tag-gcp'],
  },
]

// ============================================
// Time entries (実績)
// ============================================
export let timeEntries: TimeEntry[] = [
  // Yesterday
  {
    id: 'te1',
    startDatetime: toUtcIso(yesterday, '19:00'),
    endDatetime: toUtcIso(yesterday, '20:00'),
    taskId: 't-ace-2',
    taskName: 'ACE試験対策 - IAM・セキュリティ',
    milestoneId: 'ms-ace',
    milestoneName: 'ACE合格',
    goalId: 'goal-cert',
    goalName: 'GCPスキルアップ',
    goalColor: '#0EA5E9',
    tagIds: ['tag-input', 'tag-gcp'],
  },
  {
    id: 'te2',
    startDatetime: toUtcIso(yesterday, '20:00'),
    endDatetime: toUtcIso(yesterday, '20:30'),
    taskId: 't-blog-mw',
    taskName: '「GoのHTTPミドルウェア設計パターン」執筆',
    milestoneId: 'ms-blog',
    milestoneName: '技術ブログ月4本',
    goalId: 'goal-output',
    goalName: '技術アウトプット',
    goalColor: '#F59E0B',
    tagIds: ['tag-dev', 'tag-writing'],
  },
  {
    id: 'te3',
    startDatetime: toUtcIso(yesterday, '20:30'),
    endDatetime: toUtcIso(yesterday, '21:30'),
    taskId: 't-mvp-api',
    taskName: 'SaaS MVP - API実装',
    milestoneId: 'ms-mvp',
    milestoneName: 'SaaS MVPリリース',
    goalId: 'goal-product',
    goalName: '個人開発プロダクト',
    goalColor: '#10B981',
    tagIds: ['tag-dev'],
  },
  {
    id: 'te4',
    startDatetime: toUtcIso(yesterday, '08:00'),
    endDatetime: toUtcIso(yesterday, '08:15'),
    taskId: 'r1',
    taskName: '技術ニュースチェック',
    tagIds: ['tag-input'],
  },
  {
    id: 'te5',
    startDatetime: toUtcIso(yesterday, '18:00'),
    endDatetime: toUtcIso(yesterday, '18:30'),
    taskName: '設計レビュー（業務）',
    description: 'チームメンバーのAPI設計レビュー',
  },
  // Today
  {
    id: 'te-today-1',
    startDatetime: toUtcIso(today, '08:00'),
    endDatetime: toUtcIso(today, '08:15'),
    taskId: 'r1',
    taskName: '技術ニュースチェック',
    tagIds: ['tag-input'],
  },
  {
    id: 'te-today-2',
    startDatetime: toUtcIso(today, '19:00'),
    endDatetime: toUtcIso(today, '20:00'),
    taskId: 't-ace-2',
    taskName: 'ACE試験対策 - IAM・セキュリティ',
    milestoneId: 'ms-ace',
    milestoneName: 'ACE合格',
    goalId: 'goal-cert',
    goalName: 'GCPスキルアップ',
    goalColor: '#0EA5E9',
    tagIds: ['tag-input', 'tag-gcp'],
  },
]

// ============================================
// Notes (統合ノート - 日記・学習記録)
// ============================================
export let notes: Note[] = [
  // Diary: 5日分
  {
    id: 'note-d1',
    userId: 'user-1',
    type: 'diary',
    title: '業務後のGCP勉強が軌道に乗ってきた',
    content: `# 業務後のGCP勉強が軌道に乗ってきた

## 今日やったこと
- IAMのロールとポリシーの仕組みを整理した
- カスタムロールの作成を実際にやってみた
- Cloud Shellでの操作に少しずつ慣れてきた

## 気づき
業務でGoのAPI書いてるとGCPのサービスアカウント周りが実感を持って理解できる。やっぱり手を動かすのが一番。

## 明日
- サービスアカウントキーの管理ベストプラクティスを調べる
- 模擬試験の問題を10問解く

今日は集中できた。19時から始めるルーティンが定着してきた感じがする。`,
    format: 'markdown',
    date: today,
    goalId: 'goal-cert',
    goalName: 'GCPスキルアップ',
    goalColor: '#0EA5E9',
    tagIds: ['ntag-reflection'],
    archived: false,
    createdAt: new Date(today),
    updatedAt: new Date(today),
  },
  {
    id: 'note-d2',
    userId: 'user-1',
    type: 'diary',
    title: 'ブログ下書きが思ったより進んだ',
    content: `# ブログ下書きが思ったより進んだ

GoのHTTPミドルウェアの記事、構成が固まって一気に書けた。
業務で実際に使ってるパターンを整理するだけで記事になるのは発見。

ただ、コード例を載せるときに業務コードそのままにならないよう注意が必要。
汎用的なサンプルに書き直す作業が意外と時間かかる。

あと3セクション書けば公開できそう。明日30分確保したい。`,
    format: 'markdown',
    date: yesterday,
    tagIds: ['ntag-reflection'],
    archived: false,
    createdAt: new Date(yesterday),
    updatedAt: new Date(yesterday),
  },
  {
    id: 'note-d3',
    userId: 'user-1',
    type: 'diary',
    title: 'Go Conference miniのLTに応募した',
    content: `# Go Conference miniのLTに応募した

思い切ってCFPを出した！テーマは「実践Goミドルウェア：認証からレート制限まで」。
正直通るか不安だけど、出さなきゃ始まらない。

準備のためにブログ記事を先に完成させておきたい。スライドのベースにもなるし。

今日は個人開発のAPI実装も少し進めた。GoでCRUD書くのは手慣れてるから早い。
問題はフロントだ…Next.jsのApp Routerがまだ掴めてない。`,
    format: 'markdown',
    date: format(subDays(new Date(), 2), 'yyyy-MM-dd'),
    goalId: 'goal-output',
    goalName: '技術アウトプット',
    goalColor: '#F59E0B',
    tagIds: ['ntag-reflection'],
    archived: false,
    createdAt: subDays(new Date(), 2),
    updatedAt: subDays(new Date(), 2),
  },
  {
    id: 'note-d4',
    userId: 'user-1',
    type: 'diary',
    title: '設計レビューで学びが多かった',
    content: `# 設計レビューで学びが多かった

後輩のAPI設計をレビューした。RESTのリソース設計で自分の中の基準を言語化するいい機会になった。

「なぜこの設計がいいのか」を説明するのって難しい。
感覚的にわかっていることを論理的に伝えるスキルが必要。
これもアウトプット力の一つだなと思った。

シニアエンジニアって技術力だけじゃなくて、こういう力も求められるんだよな。`,
    format: 'markdown',
    date: format(subDays(new Date(), 3), 'yyyy-MM-dd'),
    archived: false,
    createdAt: subDays(new Date(), 3),
    updatedAt: subDays(new Date(), 3),
  },
  {
    id: 'note-d5',
    userId: 'user-1',
    type: 'diary',
    title: '週振り返り：学習時間は確保できたがアウトプットが足りない',
    content: `# 週振り返り

## 今週のハイライト
- GCP IAMセクションの学習完了
- ブログ記事の構成が固まった
- 個人開発のDB設計が完了

## 反省
- ブログの公開まで至らなかった（下書きで止まっている）
- 英語学習を3日サボってしまった
- 木曜は定例が多くて学習時間ゼロだった

## 来週の方針
- ブログ記事を1本は公開する
- ACE模擬試験に着手
- 英語は最低15分/日をキープ

全体的に悪くないが、アウトプットの「公開」まで持っていく力が課題。`,
    format: 'markdown',
    date: format(subDays(new Date(), 5), 'yyyy-MM-dd'),
    tagIds: ['ntag-reflection'],
    archived: false,
    createdAt: subDays(new Date(), 5),
    updatedAt: subDays(new Date(), 5),
  },
  // Learning notes: 最近の4件
  {
    id: 'note-lr1',
    userId: 'user-1',
    type: 'learning',
    title: 'GCP IAMまとめ',
    content: `# GCP IAM まとめ

## 基本概念

IAM (Identity and Access Management) は「誰が」「何に対して」「何をできるか」を制御する。

### メンバータイプ
- Google アカウント（個人ユーザー）
- サービスアカウント（アプリケーション/VM）
- Google グループ
- Cloud Identity ドメイン

### ロールの種類
1. **基本ロール**: Owner, Editor, Viewer（粗い制御、本番非推奨）
2. **事前定義ロール**: \`roles/compute.instanceAdmin\` など（推奨）
3. **カスタムロール**: 必要な権限だけ組み合わせ

## ベストプラクティス

\`\`\`
# 最小権限の原則
- 基本ロールは開発環境のみ
- サービスアカウントキーは極力使わない → Workload Identity
- 定期的なIAM監査
\`\`\`

## 業務での気づき
Goのサービスからサービスアカウントの認証を使うとき、ADCが効く場面と効かない場面の整理が重要。`,
    format: 'markdown',
    date: yesterday,
    taskId: 't-ace-2',
    milestoneId: 'ms-ace',
    milestoneName: 'ACE合格',
    goalId: 'goal-cert',
    goalName: 'GCPスキルアップ',
    goalColor: '#0EA5E9',
    tagIds: ['ntag-gcp', 'ntag-security'],
    relatedTimeEntryIds: ['te1'],
    archived: false,
    createdAt: new Date(yesterday),
    updatedAt: new Date(yesterday),
  },
  {
    id: 'note-lr2',
    userId: 'user-1',
    type: 'learning',
    title: 'GoのHTTPミドルウェアパターン',
    content: `# Go HTTPミドルウェアパターン

## 基本構造

\`\`\`go
type Middleware func(http.Handler) http.Handler

func Logging(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        next.ServeHTTP(w, r)
        slog.Info("request", "method", r.Method, "path", r.URL.Path, "duration", time.Since(start))
    })
}
\`\`\`

## よく使うパターン

### 1. 認証ミドルウェア
JWTトークン検証 → context にユーザー情報を注入

### 2. リクエストID付与
\`X-Request-ID\` ヘッダーまたはUUID生成 → context + レスポンスヘッダーに設定

### 3. レート制限
golang.org/x/time/rate を使ったトークンバケット

### 4. リカバリー
panicをキャッチして500を返す。スタックトレースをログに出力。

## chi での組み合わせ
\`\`\`go
r := chi.NewRouter()
r.Use(middleware.RequestID)
r.Use(middleware.RealIP)
r.Use(Logging)
r.Use(middleware.Recoverer)
\`\`\`

業務でもこの順番で使ってる。ブログ記事にまとめる。`,
    format: 'markdown',
    date: format(subDays(new Date(), 3), 'yyyy-MM-dd'),
    milestoneId: 'ms-blog',
    milestoneName: '技術ブログ月4本',
    goalId: 'goal-output',
    goalName: '技術アウトプット',
    goalColor: '#F59E0B',
    tagIds: ['ntag-go', 'ntag-api'],
    archived: false,
    createdAt: subDays(new Date(), 3),
    updatedAt: subDays(new Date(), 2),
  },
  {
    id: 'note-lr3',
    userId: 'user-1',
    type: 'learning',
    title: 'Next.js App Router入門',
    content: `# Next.js App Router 入門メモ

## Pages Router との違い
- ファイルベースルーティングは同じだが、app/ ディレクトリを使う
- Server Components がデフォルト
- layout.tsx でネストレイアウト
- loading.tsx で Suspense が自動

## つまずいたポイント
- \`"use client"\` をつけ忘れて useState が使えなかった
- Server Component から Client Component へのprops は serializable でないといけない
- fetch のキャッシュ挙動が複雑

## 感想
Go のバックエンドに比べると概念が多い。
でもフルスタックでやるならここは避けて通れない。
まずは個人開発のフロントをNext.jsで書いてみて慣れる。`,
    format: 'markdown',
    date: format(subDays(new Date(), 2), 'yyyy-MM-dd'),
    milestoneId: 'ms-mvp',
    milestoneName: 'SaaS MVPリリース',
    goalId: 'goal-product',
    goalName: '個人開発プロダクト',
    goalColor: '#10B981',
    tagIds: ['ntag-nextjs', 'ntag-saas'],
    archived: false,
    createdAt: subDays(new Date(), 2),
    updatedAt: subDays(new Date(), 2),
  },
  {
    id: 'note-lr4',
    userId: 'user-1',
    type: 'learning',
    title: 'Cloud Run vs GKE 比較メモ',
    content: `# Cloud Run vs GKE

## Cloud Run
- コンテナをサーバーレスで実行
- リクエストベースの課金（0スケール可能）
- 設定がシンプル、運用コスト低い
- 制限: 長時間実行に不向き、VPCアクセスに設定が必要

## GKE
- フルKubernetes
- 複雑なワークロードに対応
- 運用コスト高い（ノード管理、アップグレード）
- GKE Autopilot で運用負荷軽減

## 使い分けの判断基準
| 観点 | Cloud Run | GKE |
|------|-----------|-----|
| スケール | リクエスト単位 | Pod単位 |
| 常時起動 | min-instances設定 | デフォルト |
| 運用負荷 | 低 | 中〜高 |
| カスタマイズ | 限定的 | 自由 |

## 業務での所感
うちのAPIはCloud Runで十分。ただしバッチ処理やWebSocket要件があるとGKEが必要。
ACE試験ではこの使い分けが頻出らしい。`,
    format: 'markdown',
    date: format(subDays(new Date(), 4), 'yyyy-MM-dd'),
    milestoneId: 'ms-ace',
    milestoneName: 'ACE合格',
    goalId: 'goal-cert',
    goalName: 'GCPスキルアップ',
    goalColor: '#0EA5E9',
    tagIds: ['ntag-gcp', 'ntag-k8s'],
    archived: false,
    createdAt: subDays(new Date(), 4),
    updatedAt: subDays(new Date(), 4),
  },
  // 2024年以前の学習記録（当時は一般ノートとして保存されていた）
  {
    id: 'note-lr-old1',
    userId: 'user-1',
    type: 'general',
    title: 'Linuxカーネルパラメータとコンテナチューニング',
    content: '# Linuxカーネルパラメータとコンテナチューニング\n\nsysctl設定、cgroup v2、ファイルディスクリプタ上限など本番コンテナ運用に必要な設定をまとめた。',
    format: 'markdown',
    date: '2024-11-15',
    tagIds: ['ntag-linux', 'ntag-docker'],
    archived: false,
    createdAt: new Date('2024-11-15'),
    updatedAt: new Date('2024-11-15'),
  },
  {
    id: 'note-lr-old2',
    userId: 'user-1',
    type: 'general',
    title: 'Claude APIでコードレビュー自動化ツール作成',
    content: '# Claude APIでコードレビュー自動化\n\nAnthropic APIを使ってGitHub PRに自動コメントするGoツールを試作。プロンプト設計とトークン制御がポイント。',
    format: 'markdown',
    date: '2024-10-20',
    tagIds: ['ntag-genai', 'ntag-go'],
    archived: false,
    createdAt: new Date('2024-10-20'),
    updatedAt: new Date('2024-10-20'),
  },
  {
    id: 'note-lr-old3',
    userId: 'user-1',
    type: 'general',
    title: 'AWS LambdaとGCP Cloud Functionsの比較',
    content: '# AWS Lambda vs GCP Cloud Functions\n\nコールドスタート、ランタイムサポート、料金モデル、VPC統合の観点で比較。GCPのCloud Run Functionsへの統合も整理。',
    format: 'markdown',
    date: '2024-09-10',
    tagIds: ['ntag-aws', 'ntag-gcp'],
    archived: false,
    createdAt: new Date('2024-09-10'),
    updatedAt: new Date('2024-09-10'),
  },
  {
    id: 'note-lr-old4',
    userId: 'user-1',
    type: 'general',
    title: 'scikit-learnで異常検知: API応答時間の分析',
    content: '# scikit-learnで異常検知\n\nIsolation ForestとLocal Outlier Factorを使ってAPI応答時間の異常を検知する仕組みを試作。Goからpythonスクリプトを呼ぶ構成。',
    format: 'markdown',
    date: '2024-08-05',
    tagIds: ['ntag-ml', 'ntag-api'],
    archived: false,
    createdAt: new Date('2024-08-05'),
    updatedAt: new Date('2024-08-05'),
  },
  {
    id: 'note-lr-old5',
    userId: 'user-1',
    type: 'general',
    title: 'Dockerマルチステージビルドの最適化テクニック',
    content: '# Dockerマルチステージビルド最適化\n\nGoバイナリのイメージサイズ削減。scratch vs distroless、レイヤーキャッシュ戦略、.dockerignoreの重要性。',
    format: 'markdown',
    date: '2024-07-20',
    tagIds: ['ntag-docker', 'ntag-go'],
    archived: false,
    createdAt: new Date('2024-07-20'),
    updatedAt: new Date('2024-07-20'),
  },
  {
    id: 'note-lr-old6',
    userId: 'user-1',
    type: 'general',
    title: 'PostgreSQLインデックス設計とクエリ最適化',
    content: '# PostgreSQLインデックス設計\n\nB-tree, GIN, GiSTの使い分け。EXPLAIN ANALYZEの読み方。複合インデックスのカラム順序。パーティショニングとの併用。',
    format: 'markdown',
    date: '2024-06-15',
    tagIds: ['ntag-db'],
    archived: false,
    createdAt: new Date('2024-06-15'),
    updatedAt: new Date('2024-06-15'),
  },
  {
    id: 'note-lr-old7',
    userId: 'user-1',
    type: 'general',
    title: 'Goのgenerics実践パターン',
    content: '# Go generics 実践パターン\n\nGo 1.18+のジェネリクス。リポジトリパターンの汎用化、スライスユーティリティ、型制約の設計。過度な抽象化の罠についても整理。',
    format: 'markdown',
    date: '2024-05-10',
    tagIds: ['ntag-go'],
    archived: false,
    createdAt: new Date('2024-05-10'),
    updatedAt: new Date('2024-05-10'),
  },
  {
    id: 'note-lr-old8',
    userId: 'user-1',
    type: 'learning',
    title: 'TerraformでGCPインフラをIaC管理',
    content: '# Terraform + GCP IaC\n\nCloud Run, Cloud SQL, VPCをTerraformで管理。state管理はGCSバケット。モジュール分割とworkspaceの運用方針。',
    format: 'markdown',
    date: '2024-04-05',
    tagIds: ['ntag-terraform', 'ntag-gcp'],
    archived: false,
    createdAt: new Date('2024-04-05'),
    updatedAt: new Date('2024-04-05'),
  },
  {
    id: 'note-lr-old9',
    userId: 'user-1',
    type: 'learning',
    title: 'GitHub Actions CI/CDパイプライン構築',
    content: '# GitHub Actions CI/CD\n\nGoのテスト・lint・ビルド・デプロイをGitHub Actionsで自動化。マトリクスビルド、キャッシュ戦略、Cloud Runへの自動デプロイ。',
    format: 'markdown',
    date: '2024-03-18',
    tagIds: ['ntag-cicd', 'ntag-go'],
    archived: false,
    createdAt: new Date('2024-03-18'),
    updatedAt: new Date('2024-03-18'),
  },
  {
    id: 'note-lr-old10',
    userId: 'user-1',
    type: 'learning',
    title: 'OpenAI Embeddings APIでセマンティック検索',
    content: '# OpenAI Embeddings APIでセマンティック検索\n\ntext-embedding-3-smallでドキュメントをベクトル化。pgvectorでPostgreSQLに格納。コサイン類似度で検索。精度と速度のトレードオフ。',
    format: 'markdown',
    date: '2024-02-25',
    tagIds: ['ntag-genai', 'ntag-db'],
    archived: false,
    createdAt: new Date('2024-02-25'),
    updatedAt: new Date('2024-02-25'),
  },
  {
    id: 'note-lr-old11',
    userId: 'user-1',
    type: 'learning',
    title: 'GCP Cloud Runのオートスケーリング設定',
    content: '# Cloud Run オートスケーリング\n\nmin/max instances、concurrency設定、CPU allocation（request時 vs always）。コールドスタート対策とコストの関係。',
    format: 'markdown',
    date: '2024-01-20',
    tagIds: ['ntag-gcp'],
    archived: false,
    createdAt: new Date('2024-01-20'),
    updatedAt: new Date('2024-01-20'),
  },
  {
    id: 'note-lr-old12',
    userId: 'user-1',
    type: 'learning',
    title: 'Goのエラーハンドリングベストプラクティス',
    content: '# Goのエラーハンドリング\n\nerrors.Is/As、sentinel error、カスタムエラー型、errors.Joinの使い方。pkg/errorsからの移行。業務コードでの使い分けガイドライン。',
    format: 'markdown',
    date: '2023-12-10',
    tagIds: ['ntag-go'],
    archived: false,
    createdAt: new Date('2023-12-10'),
    updatedAt: new Date('2023-12-10'),
  },
  {
    id: 'note-lr-old13',
    userId: 'user-1',
    type: 'learning',
    title: 'Kubernetesのリソース管理とHPA設定',
    content: '# Kubernetes リソース管理\n\nrequests/limitsの設計、VPA vs HPA、カスタムメトリクスでのスケーリング。GKEでの実運用の知見。',
    format: 'markdown',
    date: '2023-11-05',
    tagIds: ['ntag-k8s', 'ntag-gcp'],
    archived: false,
    createdAt: new Date('2023-11-05'),
    updatedAt: new Date('2023-11-05'),
  },
  {
    id: 'note-lr-old14',
    userId: 'user-1',
    type: 'learning',
    title: 'Linux パフォーマンス分析ツールまとめ',
    content: '# Linuxパフォーマンス分析\n\ntop/htop, vmstat, iostat, sar, perf, strace。USE Method（Utilization, Saturation, Errors）に基づくトラブルシューティングフロー。',
    format: 'markdown',
    date: '2023-10-15',
    tagIds: ['ntag-linux'],
    archived: false,
    createdAt: new Date('2023-10-15'),
    updatedAt: new Date('2023-10-15'),
  },
  {
    id: 'note-lr-old15',
    userId: 'user-1',
    type: 'learning',
    title: 'AWS S3互換ストレージとGoクライアント実装',
    content: '# AWS S3互換ストレージ\n\nMinIO/Cloud StorageのS3互換API。Go aws-sdk-v2でのクライアント実装。署名付きURL生成。マルチパートアップロード。',
    format: 'markdown',
    date: '2023-09-20',
    tagIds: ['ntag-aws', 'ntag-go'],
    archived: false,
    createdAt: new Date('2023-09-20'),
    updatedAt: new Date('2023-09-20'),
  },
]

// ============================================
// User settings (ユーザー設定)
// ============================================
export let userSettings: UserSettings = {
  timezone: 'Asia/Tokyo',
  theme: 'system',
  isConfigured: true,
  userName: '田中翔太',
}

// ============================================
// AI review reports (AI振り返り)
// ============================================
export let aiReviewReports: AIReviewReport[] = [
  {
    id: 'ai1',
    periodStart: format(subDays(new Date(), 7), 'yyyy-MM-dd'),
    periodEnd: format(subDays(new Date(), 1), 'yyyy-MM-dd'),
    taskEvaluations: [
      { taskName: 'ACE試験対策', status: 'good', comment: 'IAM・セキュリティセクションを順調に消化。手を動かしながら学ぶスタイルが活きている。' },
      { taskName: 'ブログ記事執筆', status: 'partial', comment: '下書きは進んだが公開に至らず。あと一歩の仕上げに時間を確保したい。' },
      { taskName: 'SaaS MVP開発', status: 'good', comment: 'DB設計完了、API実装に着手。Goの経験が活きて速い。' },
    ],
    timeEvaluations: [
      { goalName: 'GCPスキルアップ', goalColor: '#0EA5E9', actualMinutes: 8 * 60, targetMinutes: 10 * 60, comment: '目標の80%。平日の学習時間は安定しているが、木曜が空白。' },
      { goalName: '技術アウトプット', goalColor: '#F59E0B', actualMinutes: 3 * 60, targetMinutes: 5 * 60, comment: '目標の60%。執筆時間の確保が課題。' },
      { goalName: '個人開発プロダクト', goalColor: '#10B981', actualMinutes: 6 * 60, targetMinutes: 7 * 60, comment: '概ね目標達成。週末の集中タイムが効いた。' },
    ],
    learningSummary:
      '今週は合計17時間の学習を実施。GCP IAMの理解が深まり、業務のサービスアカウント設計にも好影響。個人開発はGoのAPI実装フェーズに入り、得意分野でスムーズに進行。一方、ブログ公開が未達。',
    summary:
      '全体として計画の81%を達成。GCP学習と個人開発は順調だが、アウトプットの「公開」まで持っていく仕上げ力が今週の課題。',
    goodPoints: [
      '総学習時間17時間を確保（平日平均1.5h）',
      'GCP IAMセクション完了、理解度が業務にも反映',
      '個人開発のDB設計→API実装への移行がスムーズ',
    ],
    improvementPoints: [
      'ブログ記事が下書き止まり（公開0本）',
      '木曜の定例会議が多く学習時間ゼロだった',
      '英語学習を3日間サボった',
    ],
    advice: [
      '田中さんは手を動かして学ぶスタイルが最も効果的です。ブログ執筆も「書きながら調べる」アプローチが合いそうです',
      '木曜の学習時間確保が難しい場合、朝15分だけでも確保すると週の途切れがなくなります',
      'ミドルウェア記事はあと3セクション。1日30分×3日で完成できるペースです',
    ],
    diaryFeedback: '日記の内容から、田中さんが設計レビューを通じてシニアエンジニアに必要なスキルを意識し始めているのが印象的です。技術力＋言語化力の両方を磨く姿勢が素晴らしい。ブログ執筆はまさにその訓練になりますよ。',
    createdAt: subDays(new Date(), 1),
  },
]

// ============================================
// Weekly summary (週次サマリー)
// ============================================
export const weeklySummary: WeeklySummary = {
  weekStart: format(subDays(new Date(), 6), 'yyyy-MM-dd'),
  weekEnd: today,
  totalMinutes: 17 * 60,
  byGoal: [
    { id: 'goal-cert', name: 'GCPスキルアップ', color: '#0EA5E9', minutes: 8 * 60 },
    { id: 'goal-product', name: '個人開発プロダクト', color: '#10B981', minutes: 6 * 60 },
    { id: 'goal-output', name: '技術アウトプット', color: '#F59E0B', minutes: 3 * 60 },
  ],
  byTag: [
    { id: 'tag-input', name: 'Input', color: '#06B6D4', minutes: 9 * 60 },
    { id: 'tag-dev', name: '開発', color: '#8B5CF6', minutes: 7 * 60 },
    { id: 'tag-gcp', name: 'GCP', color: '#4285F4', minutes: 8 * 60 },
    { id: 'tag-writing', name: '執筆', color: '#F59E0B', minutes: 3 * 60 },
  ],
  byMilestone: [
    { id: 'ms-ace', name: 'ACE合格', goalId: 'goal-cert', minutes: 7 * 60 },
    { id: 'ms-mvp', name: 'SaaS MVPリリース', goalId: 'goal-product', minutes: 6 * 60 },
    { id: 'ms-blog', name: '技術ブログ月4本', goalId: 'goal-output', minutes: 3 * 60 },
    { id: 'ms-pcd', name: 'PCD合格', goalId: 'goal-cert', minutes: 1 * 60 },
  ],
  completedTasks: 8,
  plannedVsActual: {
    planned: 21 * 60,
    actual: 17 * 60,
  },
}

// ============================================
// Daily study hours (チャート用)
// ============================================
const dayNames = ['日', '月', '火', '水', '木', '金', '土']
const getDayName = (d: Date) => dayNames[d.getDay()]

export const dailyStudyHours = [
  {
    date: format(subDays(new Date(), 6), 'yyyy-MM-dd'),
    hours: 2,
    day: getDayName(subDays(new Date(), 6)),
    byGoal: [
      { id: 'goal-cert', name: 'GCPスキルアップ', color: '#0EA5E9', minutes: 60 },
      { id: 'goal-product', name: '個人開発プロダクト', color: '#10B981', minutes: 45 },
      { id: 'goal-output', name: '技術アウトプット', color: '#F59E0B', minutes: 15 },
    ],
  },
  {
    date: format(subDays(new Date(), 5), 'yyyy-MM-dd'),
    hours: 2.5,
    day: getDayName(subDays(new Date(), 5)),
    byGoal: [
      { id: 'goal-cert', name: 'GCPスキルアップ', color: '#0EA5E9', minutes: 60 },
      { id: 'goal-product', name: '個人開発プロダクト', color: '#10B981', minutes: 60 },
      { id: 'goal-output', name: '技術アウトプット', color: '#F59E0B', minutes: 30 },
    ],
  },
  {
    date: format(subDays(new Date(), 4), 'yyyy-MM-dd'),
    hours: 3,
    day: getDayName(subDays(new Date(), 4)),
    byGoal: [
      { id: 'goal-cert', name: 'GCPスキルアップ', color: '#0EA5E9', minutes: 90 },
      { id: 'goal-product', name: '個人開発プロダクト', color: '#10B981', minutes: 60 },
      { id: 'goal-output', name: '技術アウトプット', color: '#F59E0B', minutes: 30 },
    ],
  },
  {
    date: format(subDays(new Date(), 3), 'yyyy-MM-dd'),
    hours: 0,
    day: getDayName(subDays(new Date(), 3)),
    byGoal: [],
  },
  {
    date: format(subDays(new Date(), 2), 'yyyy-MM-dd'),
    hours: 3.5,
    day: getDayName(subDays(new Date(), 2)),
    byGoal: [
      { id: 'goal-cert', name: 'GCPスキルアップ', color: '#0EA5E9', minutes: 60 },
      { id: 'goal-product', name: '個人開発プロダクト', color: '#10B981', minutes: 90 },
      { id: 'goal-output', name: '技術アウトプット', color: '#F59E0B', minutes: 60 },
    ],
  },
  {
    date: format(subDays(new Date(), 1), 'yyyy-MM-dd'),
    hours: 2.5,
    day: getDayName(subDays(new Date(), 1)),
    byGoal: [
      { id: 'goal-cert', name: 'GCPスキルアップ', color: '#0EA5E9', minutes: 60 },
      { id: 'goal-product', name: '個人開発プロダクト', color: '#10B981', minutes: 60 },
      { id: 'goal-output', name: '技術アウトプット', color: '#F59E0B', minutes: 30 },
    ],
  },
  {
    date: format(new Date(), 'yyyy-MM-dd'),
    hours: 1.5,
    day: getDayName(new Date()),
    byGoal: [
      { id: 'goal-cert', name: 'GCPスキルアップ', color: '#0EA5E9', minutes: 60 },
      { id: 'goal-output', name: '技術アウトプット', color: '#F59E0B', minutes: 30 },
    ],
  },
]

// ============================================
// Memos (メモ)
// ============================================
export interface MockMemo {
  id: string
  userId: string
  content: string
  archived: boolean
  createdAt: Date
  updatedAt: Date
}

export let memos: MockMemo[] = [
  {
    id: 'memo-1',
    userId: 'user-1',
    content: 'ACE試験のIAMセクション、サービスアカウントキーの管理方法を重点復習',
    archived: false,
    createdAt: subDays(new Date(), 2),
    updatedAt: subDays(new Date(), 2),
  },
  {
    id: 'memo-2',
    userId: 'user-1',
    content: 'ブログ記事: ミドルウェアのコード例を業務コードから汎用化する',
    archived: false,
    createdAt: subDays(new Date(), 1),
    updatedAt: subDays(new Date(), 1),
  },
  {
    id: 'memo-3',
    userId: 'user-1',
    content: 'Go Conference mini のCFP結果を待つ。落ちても社内LTで発表する',
    archived: false,
    createdAt: new Date(),
    updatedAt: new Date(),
  },
  {
    id: 'memo-4',
    userId: 'user-1',
    content: '個人開発DBスキーマ: usersテーブルにOAuth対応カラム追加検討',
    archived: true,
    createdAt: subDays(new Date(), 10),
    updatedAt: subDays(new Date(), 5),
  },
]

// ============================================
// User Memory (AIパーソナライズ用)
// ============================================
export const userMemory = {
  profileSummary: '30歳バックエンドエンジニア。Go+GCPで5年。BtoB SaaS企業でAPI・マイクロサービスを担当。GCP認定資格に挑戦中。ブログ・LTでアウトプット習慣化が目標。個人開発SaaS並行。',
  preferences: {
    learningStyle: 'hands-on',
    preferredTime: 'evening',
    communicationStyle: 'concise',
  },
  strengths: ['Go/API設計力', '計画的な学習', '継続的な時間管理'],
  growthAreas: ['アウトプット継続', 'フロントエンド', '英語'],
}

// ============================================
// User Facts (AIが学習した事実)
// ============================================
export const userFacts = [
  { id: 'uf-1', factType: 'schedule', content: '業務後19:00-20:30が学習のゴールデンタイム', source: 'conversation', confidence: 0.95 },
  { id: 'uf-2', factType: 'context', content: 'Go歴5年、GCP歴3年', source: 'conversation', confidence: 1.0 },
  { id: 'uf-3', factType: 'goal', content: 'ブログ月2本ペース（目標4本）、公開までの仕上げが課題', source: 'analysis', confidence: 0.9 },
  { id: 'uf-4', factType: 'schedule', content: '朝型に切り替え中（3週間目）。朝は技術ニュースチェック', source: 'conversation', confidence: 0.85 },
  { id: 'uf-5', factType: 'context', content: 'Next.js初挑戦。Server Componentsの概念に苦戦中', source: 'conversation', confidence: 0.9 },
  { id: 'uf-6', factType: 'context', content: 'LT登壇は過去2回（社内勉強会）。社外は初', source: 'conversation', confidence: 1.0 },
  { id: 'uf-7', factType: 'schedule', content: '電車通勤30分で技術書を読む習慣あり', source: 'conversation', confidence: 0.9 },
  { id: 'uf-8', factType: 'schedule', content: '木曜は定例会議が多く学習時間が取りづらい', source: 'analysis', confidence: 0.95 },
  { id: 'uf-9', factType: 'preference', content: '手を動かして学ぶスタイル（座学より実践派）', source: 'analysis', confidence: 0.9 },
  { id: 'uf-10', factType: 'goal', content: 'シニアエンジニア昇格を目指している', source: 'conversation', confidence: 1.0 },
  { id: 'uf-11', factType: 'challenge', content: '英語に苦手意識。リスニングから始めている', source: 'conversation', confidence: 0.85 },
  { id: 'uf-12', factType: 'context', content: 'Go Conference mini にCFP提出済み。テーマはミドルウェアパターン', source: 'conversation', confidence: 1.0 },
]

// ============================================
// User Patterns (行動パターン分析)
// ============================================
export const userPatterns = {
  planAccuracy: 0.81,
  peakProductivityHour: 19,
  averageDailyMinutes: 146,
  streakDays: 12,
  goalTrends: [
    { goalId: 'goal-cert', goalName: 'GCPスキルアップ', trend: 'stable' as const, weeklyAvgMinutes: 480 },
    { goalId: 'goal-product', goalName: '個人開発プロダクト', trend: 'growing' as const, weeklyAvgMinutes: 360 },
    { goalId: 'goal-output', goalName: '技術アウトプット', trend: 'declining' as const, weeklyAvgMinutes: 180 },
  ],
  overdueItems: [
    { type: 'task' as const, name: '「GoのHTTPミドルウェア設計パターン」執筆', daysPastDue: 0 },
  ],
  weekdayPattern: [
    { day: '月', avgMinutes: 90 },
    { day: '火', avgMinutes: 100 },
    { day: '水', avgMinutes: 90 },
    { day: '木', avgMinutes: 20 },
    { day: '金', avgMinutes: 80 },
    { day: '土', avgMinutes: 180 },
    { day: '日', avgMinutes: 150 },
  ],
}

// ============================================
// Interest Profile (興味プロファイル)
// ============================================
export const interestProfile = {
  topTags: [
    { name: 'Go', trend: 'stable' as const, score: 0.95 },
    { name: 'GCP', trend: 'growing' as const, score: 0.85 },
    { name: 'API設計', trend: 'stable' as const, score: 0.80 },
    { name: 'Next.js', trend: 'growing' as const, score: 0.40 },
    { name: 'テスト', trend: 'stable' as const, score: 0.60 },
  ],
  clusters: [
    { name: 'バックエンド', tags: ['Go', 'API設計', 'テスト', 'マイクロサービス'] },
    { name: 'クラウド', tags: ['GCP', 'Cloud Run', 'IAM', 'GKE'] },
    { name: 'フロントエンド', tags: ['Next.js', 'React', 'TypeScript'] },
  ],
}

// ============================================
// Trait Profile (特性プロファイル)
// ============================================
export const traitProfile = {
  workStyle: 'methodical',
  learningStyle: 'hands-on',
  outputPreference: 'structured',
  strengths: ['計画的なタスク管理', 'Go/API設計の深い知識', '継続的な学習習慣'],
  challenges: ['アウトプットの仕上げ・公開', 'フロントエンド技術', '英語'],
  communicationStyle: '簡潔で構造的。コードで示すことを好む。',
}

// ============================================
// Helper functions
// ============================================
export const generateId = (prefix: string) => `${prefix}${Date.now()}`

// Goal/Milestone/Tag lookup helpers
export const findGoalById = (id: string) => goals.find(g => g.id === id)
export const findMilestoneById = (id: string) => milestones.find(m => m.id === id)
export const findTagById = (id: string) => tags.find(t => t.id === id)
export const findTagsByIds = (ids: string[]) => tags.filter(t => ids.includes(t.id))
