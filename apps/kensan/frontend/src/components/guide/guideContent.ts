export interface GuideTip {
  icon: string
  text: string
}

export interface GuidePageContent {
  pageId: string
  title: string
  tips: GuideTip[]
  hasTour: boolean
}

export const guideContent: GuidePageContent[] = [
  {
    pageId: 'daily',
    title: 'Daily - 今日の計画と実行',
    tips: [
      { icon: '📅', text: 'タイムブロックで1日の予定を視覚的に管理' },
      { icon: '🖱️', text: 'タスクをタイムラインにドラッグ＆ドロップで配置' },
      { icon: '✨', text: 'AIアドバイスで今日の計画を最適化' },
      { icon: '📝', text: '学習記録や日記をワンクリックで作成' },
    ],
    hasTour: true,
  },
  {
    pageId: 'weekly',
    title: 'Weekly - 週間カレンダー',
    tips: [
      { icon: '📊', text: '週間ビューで7日分のタイムブロックを俯瞰' },
      { icon: '⬅️', text: '矢印ボタンで前後の週に切り替え' },
      { icon: '🖱️', text: 'タスクカードをカレンダーにドラッグして配置' },
      { icon: '➕', text: 'セルをクリックして新しいブロックを追加' },
    ],
    hasTour: true,
  },
  {
    pageId: 'tasks',
    title: 'タスク管理 - 3カラム構成',
    tips: [
      { icon: '🎯', text: '目標（Goal）で大きな方向性を設定' },
      { icon: '🚩', text: 'マイルストーンで中間目標を管理' },
      { icon: '✅', text: 'タスクを細分化してトラッキング' },
      { icon: '🏷️', text: 'タグで横断的にタスクを分類' },
    ],
    hasTour: true,
  },
  {
    pageId: 'notes',
    title: 'ノート - タイプ別に整理',
    tips: [
      { icon: '📂', text: 'ノートタイプ（学習記録・日記・メモ等）で分類' },
      { icon: '🔍', text: 'キーワード検索でノートを素早く見つける' },
      { icon: '🏷️', text: 'タグで絞り込んで関連ノートを一覧' },
      { icon: '📦', text: 'アーカイブ機能で整理整頓' },
    ],
    hasTour: false,
  },
  {
    pageId: 'note-edit',
    title: 'ノート編集 - マルチフォーマット',
    tips: [
      { icon: '✍️', text: 'マークダウンでリッチなドキュメント作成' },
      { icon: '🗺️', text: 'Draw.io やマインドマップで図解' },
      { icon: '📋', text: 'メタデータ（目標・タグ）で紐付け管理' },
    ],
    hasTour: false,
  },
  {
    pageId: 'analytics',
    title: '分析・レポート - 学習を可視化',
    tips: [
      { icon: '📈', text: '期間を選んで活動サマリーを分析' },
      { icon: '🎯', text: '目標別の時間配分をグラフで確認' },
      { icon: '📊', text: '日別の推移チャートで傾向を把握' },
      { icon: '🤖', text: 'AIレビューで振り返りのヒントを取得' },
    ],
    hasTour: false,
  },
  {
    pageId: 'prompts',
    title: 'プロンプト管理 - AIの振る舞いを制御',
    tips: [
      { icon: '📄', text: 'コンテキストを選んでプロンプトを表示' },
      { icon: '✏️', text: 'プロンプトを編集してAIの応答をカスタマイズ' },
      { icon: '🔄', text: 'バージョン履歴で変更を比較・ロールバック' },
    ],
    hasTour: true,
  },
  {
    pageId: 'interactions',
    title: 'AI Explorer - インタラクション分析',
    tips: [
      { icon: '💬', text: 'AIとの対話履歴を一覧で確認' },
      { icon: '🔍', text: 'アウトカムや種類でフィルタリング' },
      { icon: '⏰', text: '時間範囲を指定して絞り込み' },
    ],
    hasTour: true,
  },
  {
    pageId: 'settings',
    title: '設定',
    tips: [
      { icon: '🌏', text: 'タイムゾーンを設定して正しい時刻表示' },
      { icon: '🎨', text: 'ライト/ダーク/システム連動のテーマ切替' },
    ],
    hasTour: false,
  },
]

export function getGuideContent(pageId: string): GuidePageContent | undefined {
  return guideContent.find((c) => c.pageId === pageId)
}
