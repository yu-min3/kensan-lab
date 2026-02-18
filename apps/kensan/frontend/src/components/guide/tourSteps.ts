export interface TourStep {
  target: string        // data-guide attribute value
  title: string
  description: string
  placement: 'top' | 'bottom' | 'left' | 'right'
}

export interface TourDefinition {
  pageId: string
  steps: TourStep[]
}

export const tourDefinitions: TourDefinition[] = [
  {
    pageId: 'daily',
    steps: [
      {
        target: 'daily-header',
        title: 'ヘッダーとサマリー',
        description: '今日の日付と活動サマリーが表示されます。過去の日付もURLパラメータで閲覧できます。',
        placement: 'bottom',
      },
      {
        target: 'daily-timeblocks',
        title: 'タイムブロック',
        description: '1日のスケジュールをタイムライン上で管理します。ブロックをクリックして編集、ドラッグでリサイズできます。',
        placement: 'right',
      },
      {
        target: 'daily-tasks',
        title: 'タスクリスト',
        description: '未完了のタスクが表示されます。タスクをタイムラインにドラッグ＆ドロップしてブロックを作成できます。',
        placement: 'left',
      },
      {
        target: 'daily-ai',
        title: 'AIアドバイス',
        description: 'AIがタイムブロックとタスクの状況を分析し、今日の計画についてアドバイスします。',
        placement: 'top',
      },
    ],
  },
  {
    pageId: 'tasks',
    steps: [
      {
        target: 'task-goals',
        title: '目標カラム',
        description: '大きな目標をここで管理します。目標を選択するとマイルストーンが表示されます。ドラッグで並び替えも可能です。',
        placement: 'right',
      },
      {
        target: 'task-milestones',
        title: 'マイルストーン',
        description: '選択した目標のマイルストーン（中間目標）を表示します。進捗バーで達成度を確認できます。',
        placement: 'right',
      },
      {
        target: 'task-tasks',
        title: 'タスク一覧',
        description: '具体的なタスクを管理します。完了チェック、編集、サブタスク追加ができます。',
        placement: 'left',
      },
      {
        target: 'task-search',
        title: '検索とフィルタ',
        description: 'キーワード検索や完了タスクの表示/非表示を切り替えられます。',
        placement: 'bottom',
      },
    ],
  },
  {
    pageId: 'weekly',
    steps: [
      {
        target: 'weekly-header',
        title: '週ナビゲーション',
        description: '矢印ボタンで前後の週に移動します。「今週」ボタンで現在の週に戻れます。',
        placement: 'bottom',
      },
      {
        target: 'weekly-calendar',
        title: 'カレンダーグリッド',
        description: '7日分のタイムブロックを時間帯ごとに表示します。セルをクリックしてブロックを追加、既存ブロックのドラッグでリサイズも可能です。',
        placement: 'top',
      },
      {
        target: 'weekly-tasks',
        title: 'タスクカード',
        description: '未配置のタスクが表示されます。カレンダーにドラッグ＆ドロップしてタイムブロックを作成できます。',
        placement: 'top',
      },
    ],
  },
  {
    pageId: 'prompts',
    steps: [
      {
        target: 'prompt-sidebar',
        title: 'コンテキスト一覧',
        description: 'AIの各機能のコンテキストを選択します。',
        placement: 'right',
      },
      {
        target: 'prompt-editor',
        title: 'プロンプト編集',
        description: 'プロンプトを直接編集。変数も使えます。',
        placement: 'left',
      },
      {
        target: 'prompt-tabs',
        title: 'プロンプト編集 & 最適化',
        description: '「プロンプト編集」タブで直接編集、「最適化」タブでAI提案のレビュー・A/Bテスト・バージョン管理が可能。',
        placement: 'bottom',
      },
    ],
  },
  {
    pageId: 'interactions',
    steps: [
      {
        target: 'explorer-header',
        title: 'フィルターと時間範囲',
        description: 'アウトカムや時間範囲で絞り込みます。',
        placement: 'bottom',
      },
      {
        target: 'explorer-stats',
        title: '統計サマリー',
        description: 'インタラクション数、成功率、平均トークン数。',
        placement: 'bottom',
      },
      {
        target: 'explorer-table',
        title: 'インタラクション一覧',
        description: '各対話の詳細を確認できます。',
        placement: 'top',
      },
    ],
  },
]

export function getTourSteps(pageId: string): TourStep[] | undefined {
  return tourDefinitions.find((t) => t.pageId === pageId)?.steps
}

// --- Demo Tour (cross-page) ---

export interface DemoTourStep {
  page: string
  target?: string         // undefined = center card, no spotlight
  title: string
  description: string
  placement: 'top' | 'bottom' | 'left' | 'right' | 'center'
  action?: 'open-chat' | 'close-chat' | 'switch-optimize-tab'
  customCard?: string     // special card renderer identifier
  sectionLabel?: string   // section badge (e.g. "AI") for visual continuity
}

export const demoTourSteps: DemoTourStep[] = [
  {
    page: '/',
    target: 'sidebar-daily',
    title: 'デイリー',
    description: '1日のタイムブロックとタスクを一画面で管理するページです。予定を立てて、実績を記録していきます。',
    placement: 'right',
  },
  {
    page: '/',
    target: 'daily-timeblocks',
    title: 'タイムブロック',
    description: '予定と実績の両方をタイムライン上で記録します。クリックで編集、ドラッグでリサイズ。',
    placement: 'right',
  },
  {
    page: '/',
    target: 'daily-tasks',
    title: 'タスクカード',
    description: '未配置のタスクが一覧で表示されます。ここからタイムラインにドラッグ&ドロップでタイムブロックを作成できます。',
    placement: 'left',
  },
  {
    page: '/',
    target: 'header-timer',
    title: 'タイマー',
    description: 'ヘッダーのタイマーでタスクの作業時間を計測できます。停止するとタイムブロックとして自動記録されます。',
    placement: 'bottom',
  },
  {
    page: '/tasks',
    target: 'sidebar-tasks',
    title: 'タスク管理',
    description: '目標の下にマイルストーン、その下にタスクという3階層で管理します。ここで作成したタスクがデイリーに表示されます。',
    placement: 'right',
  },
  {
    page: '/analytics',
    target: 'sidebar-analytics',
    title: '分析・レポート',
    description: '活動データを分析し、学習時間や達成率を期間別に可視化します。AIによる週次レビューも確認できます。',
    placement: 'right',
  },
  {
    page: '/analytics',
    title: 'あなた専用のAI',
    description: '会話とフィードバックをもとに自律的に学習し、使うほどあなたに最適化されていきます。',
    placement: 'center',
    customCard: 'ai-intro',
    sectionLabel: 'AI',
  },
  {
    page: '/analytics',
    target: 'header-ai-button',
    title: 'AIと対話する',
    description: 'あなたの傾向を学んだAIが、計画の提案から振り返りまでサポート。可視化ツールやデータ分析で、対話を超えた深い洞察が得られます。',
    placement: 'bottom',
    action: 'open-chat',
    sectionLabel: 'AI',
  },
  {
    page: '/prompts',
    target: 'prompt-sidebar',
    title: 'AIを育てる',
    description: 'コンテキストごとにプロンプトを編集して、AIの応答スタイルや分析の視点をカスタマイズ。編集はすべて自動でバージョン管理され、履歴の確認やロールバックも可能です。',
    placement: 'right',
    action: 'close-chat',
    sectionLabel: 'AI',
  },
  {
    page: '/prompts',
    target: 'prompt-tab-optimize',
    title: 'AI最適化とA/Bテスト',
    description: '「最適化」タブでAIが自動生成した改善候補をレビュー。採用・却下の判断や、A/Bテストで応答品質を比較できます。',
    placement: 'bottom',
    action: 'switch-optimize-tab',
    sectionLabel: 'AI',
  },
  {
    page: '/prompts',
    target: 'version-detail',
    title: 'バージョン詳細とA/Bテスト',
    description: 'バージョンを選択すると詳細を表示。AI候補は評価サマリー付きで採用/却下でき、A/Bテストで現行と候補の応答を並べて比較できます。',
    placement: 'left',
    sectionLabel: 'AI',
  },
  {
    page: '/interactions',
    target: 'explorer-stats',
    title: '成長を実感する',
    description: '5分ごとにインタラクションログを自動収集。成功率・トークン数・ターン数からAIとの対話の質と変化を分析できます。',
    placement: 'bottom',
    sectionLabel: 'AI',
  },
]
