import { http, HttpResponse } from 'msw'
import { tasks, goals, milestones, timeBlocks, timeEntries, weeklySummary, userMemory, userPatterns, generateId, getToday, getYesterday } from '../data'
import { getLocalDate, getLocalTime } from '@/lib/timezone'

const BASE_URL = 'http://localhost:8089/api/v1'

function sseEvent(event: string, data: Record<string, unknown>): string {
  return `event: ${event}\ndata: ${JSON.stringify(data)}\n\n`
}

function delay(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

// Mock conversation history data
const mockConversations = [
  {
    id: 'conv-001',
    lastMessage: '今日の予定を立てて',
    lastMessageAt: '2026-02-05T10:00:00.000Z',
    messageCount: 4,
  },
  {
    id: 'conv-002',
    lastMessage: 'Go Conference miniのLT、受かった！',
    lastMessageAt: '2026-02-04T19:30:00.000Z',
    messageCount: 6,
  },
  {
    id: 'conv-003',
    lastMessage: 'GCPの学習、次何やればいい？',
    lastMessageAt: '2026-02-03T19:00:00.000Z',
    messageCount: 4,
  },
  {
    id: 'conv-004',
    lastMessage: '最近生産性落ちてる気がする…',
    lastMessageAt: '2026-02-02T20:00:00.000Z',
    messageCount: 4,
  },
  {
    id: 'conv-005',
    lastMessage: '週次振り返りレビューを生成して',
    lastMessageAt: '2026-02-01T18:00:00.000Z',
    messageCount: 6,
  },
]

const mockConversationMessages: Record<string, Array<{ id: string; role: string; content: string; situation: string; toolCalls: unknown[]; createdAt: string }>> = {
  // 1. パターン分析を活用した今日の計画提案
  'conv-001': [
    { id: 'msg-1-user', role: 'user', content: '今日の予定を立てて', situation: 'chat', toolCalls: [], createdAt: '2026-02-05T10:00:00.000Z' },
    { id: 'msg-1-a1', role: 'assistant', content: '', situation: 'chat', toolCalls: [
      { id: 'tc_1', name: 'get_tasks', input: { completed: false } },
      { id: 'tc_2', name: 'get_user_patterns', input: {} },
      { id: 'tc_3', name: 'get_user_memory', input: {} },
    ], createdAt: '2026-02-05T10:00:03.000Z' },
    { id: 'msg-1-a2', role: 'assistant', content: '田中さんの行動パターンを分析しました。**19時台が最も集中できる時間帯**ですね。\n\n今日の提案です：\n\n- **19:00-20:00** ACE試験対策（IAM・セキュリティ）\n  - 期限まで残り82日。手を動かす学習が効果的なので、Cloud Shellで実践\n- **20:00-20:30** ブログ記事「GoのHTTPミドルウェア」執筆\n  - あと3セクション。今日30分で1セクション仕上げましょう\n- **20:30-21:30** SaaS MVP API実装\n  - Goの経験が活きるフェーズ。CRUD実装を進める\n\n技術アウトプット目標が**下降傾向**です。ブログの30分を確保するだけで改善できますよ。', situation: 'chat', toolCalls: [], createdAt: '2026-02-05T10:00:08.000Z' },
    { id: 'msg-2-user', role: 'user', content: 'ありがとう、この計画で実行して', situation: 'chat', toolCalls: [], createdAt: '2026-02-05T10:01:00.000Z' },
    { id: 'msg-2-a1', role: 'assistant', content: '3件のタイムブロックを作成しました。今日も頑張りましょう！', situation: 'chat', toolCalls: [], createdAt: '2026-02-05T10:01:05.000Z' },
  ],
  // 2. ファクト学習デモ: LT登壇が決まった→AIが記憶
  'conv-002': [
    { id: 'msg-3-user', role: 'user', content: 'Go Conference miniのLT、受かった！', situation: 'chat', toolCalls: [], createdAt: '2026-02-04T19:30:00.000Z' },
    { id: 'msg-3-a1', role: 'assistant', content: '', situation: 'chat', toolCalls: [
      { id: 'tc_4', name: 'add_user_fact', input: { fact_type: 'context', content: 'Go Conference mini 2026のLT登壇が決定。テーマ: 実践Goミドルウェア' } },
      { id: 'tc_5', name: 'add_user_fact', input: { fact_type: 'schedule', content: 'Go Conference mini登壇準備が必要。ブログ記事をスライドのベースにする予定' } },
      { id: 'tc_6', name: 'create_task', input: { name: 'LT登壇スライド作成', milestone_id: 'ms-lt' } },
    ], createdAt: '2026-02-04T19:30:05.000Z' },
    { id: 'msg-3-a2', role: 'assistant', content: 'おめでとうございます、田中さん！社外初のLT登壇ですね！\n\n記憶しました：\n- Go Conference mini 2026のLT登壇が決定\n- テーマ: 実践Goミドルウェア\n\n「LT登壇スライド作成」タスクをLT登壇マイルストーンに追加しました。\n\nブログ記事「GoのHTTPミドルウェア設計パターン」をベースにスライドを作る計画でしたよね。先にブログを仕上げると効率的です。', situation: 'chat', toolCalls: [], createdAt: '2026-02-04T19:30:10.000Z' },
    { id: 'msg-4-user', role: 'user', content: '登壇日は3月15日だよ', situation: 'chat', toolCalls: [], createdAt: '2026-02-04T19:31:00.000Z' },
    { id: 'msg-4-a1', role: 'assistant', content: '', situation: 'chat', toolCalls: [
      { id: 'tc_7', name: 'add_user_fact', input: { fact_type: 'schedule', content: 'Go Conference mini登壇日: 2026年3月15日' } },
    ], createdAt: '2026-02-04T19:31:03.000Z' },
    { id: 'msg-4-a2', role: 'assistant', content: '3月15日ですね、記憶しました。\n\n逆算すると：\n- 2月中: ブログ記事完成 → スライド骨子作成\n- 3月第1週: スライド仕上げ → リハーサル\n- 3月第2週: 最終調整\n\n約5週間あります。ブログ記事を今週中に仕上げるのが最優先ですね。', situation: 'chat', toolCalls: [], createdAt: '2026-02-04T19:31:08.000Z' },
  ],
  // 3. 興味プロファイル活用の学習アドバイス
  'conv-003': [
    { id: 'msg-5-user', role: 'user', content: 'GCPの学習、次何やればいい？', situation: 'chat', toolCalls: [], createdAt: '2026-02-03T19:00:00.000Z' },
    { id: 'msg-5-a1', role: 'assistant', content: '', situation: 'chat', toolCalls: [
      { id: 'tc_8', name: 'get_user_facts', input: {} },
      { id: 'tc_9', name: 'get_goals_and_milestones', input: {} },
    ], createdAt: '2026-02-03T19:00:03.000Z' },
    { id: 'msg-5-a2', role: 'assistant', content: '田中さんの学習状況を確認しました。\n\nACE試験（4/30期限）の進捗：\n- GCP基礎・Compute/GKE ✅ 完了\n- IAM・セキュリティ 🔄 進行中\n- 模擬試験 ⬜ 未着手\n\n**次のステップ提案：**\n1. IAMセクションの残り（サービスアカウントキー管理）を今週完了\n2. 来週から模擬試験に着手\n\n田中さんは**手を動かす学習**が効果的なので、模擬試験は問題を解くだけでなく、間違えた分野をCloud Shellで実際に触ることをおすすめします。\n\nGo歴5年・GCP歴3年の経験があるので、Compute系は感覚で解けるはず。IAM/ネットワーク/料金見積もり系に集中すると効率的です。', situation: 'chat', toolCalls: [], createdAt: '2026-02-03T19:00:10.000Z' },
    { id: 'msg-6-user', role: 'user', content: 'ありがとう、模擬試験はどのサービス使えばいい？', situation: 'chat', toolCalls: [], createdAt: '2026-02-03T19:01:00.000Z' },
    { id: 'msg-6-a1', role: 'assistant', content: 'ACE模擬試験でよく使われるサービス：\n\n1. **Google公式模擬試験**（無料）: まず最初にこれ\n2. **Udemy - GCP ACE Practice Tests**: 実践的な問題が多い\n3. **Whizlabs**: 解説が丁寧\n\nまず公式模擬試験で現在の実力を測って、弱点分野を特定するのがおすすめです。', situation: 'chat', toolCalls: [], createdAt: '2026-02-03T19:01:05.000Z' },
  ],
  // 4. パターン分析フィードバック: 生産性
  'conv-004': [
    { id: 'msg-7-user', role: 'user', content: '最近生産性落ちてる気がする…', situation: 'chat', toolCalls: [], createdAt: '2026-02-02T20:00:00.000Z' },
    { id: 'msg-7-a1', role: 'assistant', content: '', situation: 'chat', toolCalls: [
      { id: 'tc_10', name: 'get_user_patterns', input: {} },
      { id: 'tc_11', name: 'get_analytics_summary', input: { period: 'weekly' } },
    ], createdAt: '2026-02-02T20:00:03.000Z' },
    { id: 'msg-7-a2', role: 'assistant', content: 'パターンデータを確認しました。全体的には**悪くない**です。\n\n**数字で見ると：**\n- 計画達成率: 81%（先月比-3%）\n- 連続学習日数: 12日\n- 平均学習時間: 2.4h/日\n\n**気になるポイント：**\n- 木曜の学習時間がほぼゼロ（定例会議の影響）\n- 技術アウトプット目標が**下降傾向** 📉\n- ブログ公開が止まっている\n\n田中さん、生産性が落ちたというより**アウトプットが停滞**している感覚かもしれません。インプットは安定しているので、ブログ1本公開するだけで体感が変わると思います。\n\n「下書き→公開」の最後の一歩にハードルを感じていませんか？', situation: 'chat', toolCalls: [], createdAt: '2026-02-02T20:00:10.000Z' },
    { id: 'msg-8-user', role: 'user', content: 'たしかに。公開するのが怖いんだよな…', situation: 'chat', toolCalls: [], createdAt: '2026-02-02T20:01:00.000Z' },
    { id: 'msg-8-a1', role: 'assistant', content: 'その気持ち、よくわかります。Go歴5年の実務経験に裏打ちされた記事なので、内容には自信を持っていいと思いますよ。\n\n一つ提案です：\n**「完璧な記事」ではなく「学習ノートの公開版」として出してみませんか？**\n\n田中さんの学習ノート（GoのHTTPミドルウェアパターン）はすでに構造的で分かりやすいです。あれを少し肉付けするだけで十分な記事になります。\n\nまず1本出してみて、反応を見てから改善する。Go/API設計の実践知識は需要が高いので、きっと良い反応がもらえますよ。', situation: 'chat', toolCalls: [], createdAt: '2026-02-02T20:01:08.000Z' },
  ],
  // 5. 週次レビュー生成
  'conv-005': [
    { id: 'msg-9-user', role: 'user', content: '週次振り返りレビューを生成して', situation: 'review', toolCalls: [], createdAt: '2026-02-01T18:00:00.000Z' },
    { id: 'msg-9-a1', role: 'assistant', content: '', situation: 'review', toolCalls: [
      { id: 'tc_12', name: 'get_time_entries', input: { period: 'weekly' } },
      { id: 'tc_13', name: 'get_notes', input: { period: 'weekly' } },
      { id: 'tc_14', name: 'get_user_memory', input: {} },
    ], createdAt: '2026-02-01T18:00:05.000Z' },
    { id: 'msg-9-a2', role: 'assistant', content: '振り返りレビューを生成します。データを分析中...\n\n田中さんの今週の振り返りレビューを作成しました。', situation: 'review', toolCalls: [], createdAt: '2026-02-01T18:00:15.000Z' },
  ],
}

export const agentHandlers = [
  // POST /agent/stream - SSE streaming endpoint
  http.post(`${BASE_URL}/agent/stream`, async ({ request }) => {
    const body = (await request.json()) as {
      message: string
      conversation_id?: string
      situation?: string
    }
    const message = body.message.toLowerCase()
    const conversationId = body.conversation_id || generateId('conv')

    const encoder = new TextEncoder()
    const stream = new ReadableStream({
      async start(controller) {
        // ==========================================
        // Situation-specific handlers
        // ==========================================
        if (body.situation === 'daily_advice') {
          // AI Planning - generate structured plan from mock data
          controller.enqueue(encoder.encode(sseEvent('text', { content: '田中さんの状況を分析しています...\n\n' })))
          await delay(500)

          const tz = 'Asia/Tokyo'
          const today = getToday()

          // Get actionable (leaf-level) incomplete tasks
          const leafTasks = tasks.filter(t =>
            !t.completed && !tasks.some(child => child.parentTaskId === t.id)
          )

          // Get today's existing blocks to avoid overlap
          const todayBlocks = timeBlocks.filter(
            (b) => getLocalDate(b.startDatetime, tz) === today,
          )
          const occupiedSlots = todayBlocks.map((b) => ({
            start: getLocalTime(b.startDatetime, tz),
            end: getLocalTime(b.endDatetime, tz),
          }))

          // Available slots that don't overlap with existing blocks
          const candidateSlots = [
            { start: '18:00', end: '18:30' },
            { start: '21:30', end: '22:00' },
            { start: '22:00', end: '22:30' },
          ].filter((slot) =>
            !occupiedSlots.some((occ) => slot.start < occ.end && slot.end > occ.start)
          )

          const proposedBlocks = leafTasks.slice(0, candidateSlots.length).map((task, i) => {
            const ms = task.milestoneId ? milestones.find(m => m.id === task.milestoneId) : undefined
            const goal = ms ? goals.find(g => g.id === ms.goalId) : undefined
            return {
              taskId: task.id,
              taskName: task.name,
              goalId: goal?.id || null,
              goalName: goal?.name || '',
              goalColor: goal?.color || '',
              startTime: candidateSlots[i].start,
              endTime: candidateSlots[i].end,
              reason: ms ? `空き時間帯に配置。${ms.name}（期限: ${ms.targetDate || '未設定'}）` : '空き時間帯を活用',
            }
          })

          // Build task priorities
          const taskPriorities = leafTasks.slice(0, 5).map((task, i) => {
            const ms = task.milestoneId ? milestones.find(m => m.id === task.milestoneId) : undefined
            let reason = '優先度中'
            if (ms?.targetDate) {
              const daysLeft = Math.ceil((new Date(ms.targetDate).getTime() - Date.now()) / (1000 * 60 * 60 * 24))
              reason = `${ms.name}の期限まで残り${daysLeft}日`
            }
            return {
              taskId: task.id,
              taskName: task.name,
              suggestedAction: (i < 3 ? 'today' : 'defer') as 'today' | 'defer' | 'split',
              reason,
            }
          })

          // Build yesterday review from yesterday's entries
          const yesterday = getYesterday()
          const yesterdayEntries = timeEntries.filter(
            (e) => getLocalDate(e.startDatetime, tz) === yesterday,
          )
          const yesterdayMinutes = yesterdayEntries.reduce((sum, e) => {
            const start = new Date(e.startDatetime).getTime()
            const end = new Date(e.endDatetime).getTime()
            return sum + (end - start) / 60000
          }, 0)
          const yesterdayHours = (yesterdayMinutes / 60).toFixed(1)

          const yesterdayReview = yesterdayEntries.length > 0 ? {
            summary: `昨日は${yesterdayHours}時間（${yesterdayEntries.length}セッション）の学習を行いました。`,
            highlights: [
              `連続${userPatterns.streakDays}日の学習を継続中`,
              ...yesterdayEntries.map((e) => {
                const goal = e.goalName ? ` [${e.goalName}]` : ''
                return `${e.taskName}${goal}に取り組みました`
              }),
            ],
            learningConnections: [
              '昨日のIAM学習が今日のセキュリティ演習に直結します',
              'ブログ執筆の継続がLT登壇準備の土台になっています',
            ],
          } : undefined

          const planningJson = {
            message: `田中さん、おはようございます！連続${userPatterns.streakDays}日の学習、素晴らしいですね。昨日も${yesterdayHours}時間しっかり取り組んでいました。今日も一緒に頑張りましょう。`,
            ...(yesterdayReview ? { yesterdayReview } : {}),
            insights: [
              {
                category: 'productivity',
                title: '19時台が集中のピーク',
                description: `田中さんの過去データから、19:00-20:00が最も生産性が高い時間帯です。既存のスケジュールを優先し、追加の空き時間に提案しました。（連続${userPatterns.streakDays}日学習中）`,
              },
              {
                category: 'goal',
                title: 'アウトプット目標が下降傾向',
                description: `技術アウトプット目標の学習時間が減少中です。ブログ記事「GoのHTTPミドルウェア設計パターン」の仕上げを推奨します。`,
              },
            ],
            proposedBlocks,
            taskPriorities,
            alerts: [
              {
                type: 'goal_stalled',
                message: '技術アウトプット目標: 今月のブログ公開が0本です。ミドルウェア記事はあと3セクションで完成。',
              },
            ],
          }

          controller.enqueue(
            encoder.encode(
              sseEvent('text', {
                content: '```json\n' + JSON.stringify(planningJson, null, 2) + '\n```',
              })
            )
          )
        } else if (
          message.includes('タスク') &&
          (message.includes('見せて') || message.includes('確認') || message.includes('一覧'))
        ) {
          // Read-only: show tasks
          controller.enqueue(encoder.encode(sseEvent('text', { content: 'タスクを確認しますね。' })))
          await delay(300)

          const incompleteTasks = tasks.filter((t) => !t.completed).slice(0, 5)
          controller.enqueue(
            encoder.encode(
              sseEvent('tool_call', {
                id: 'tc_1',
                name: 'get_tasks',
                input: { completed: false },
              })
            )
          )
          await delay(500)

          controller.enqueue(
            encoder.encode(
              sseEvent('tool_result', {
                id: 'tc_1',
                name: 'get_tasks',
                result: incompleteTasks.map((t) => ({
                  id: t.id,
                  name: t.name,
                })),
              })
            )
          )
          await delay(300)

          const taskList = incompleteTasks.map((t) => `- ${t.name}`).join('\n')
          controller.enqueue(
            encoder.encode(
              sseEvent('text', {
                content: `田中さんの未完了タスクが${incompleteTasks.length}件あります：\n\n${taskList}`,
              })
            )
          )
        } else if (
          message.includes('予定') &&
          (message.includes('立てて') || message.includes('作って') || message.includes('計画'))
        ) {
          // Write: propose time blocks
          controller.enqueue(
            encoder.encode(
              sseEvent('text', { content: 'タスクとパターンを確認して、今日のスケジュールを提案しますね。' })
            )
          )
          await delay(300)

          controller.enqueue(
            encoder.encode(
              sseEvent('tool_call', {
                id: 'tc_1',
                name: 'get_tasks',
                input: { completed: false },
              })
            )
          )
          await delay(300)
          controller.enqueue(
            encoder.encode(
              sseEvent('tool_call', {
                id: 'tc_2',
                name: 'get_user_patterns',
                input: {},
              })
            )
          )
          await delay(500)

          const incompleteTasks = tasks.filter((t) => !t.completed && !tasks.some(child => child.parentTaskId === t.id)).slice(0, 3)
          controller.enqueue(
            encoder.encode(
              sseEvent('tool_result', {
                id: 'tc_1',
                name: 'get_tasks',
                result: incompleteTasks.map((t) => ({ id: t.id, name: t.name })),
              })
            )
          )
          await delay(200)
          controller.enqueue(
            encoder.encode(
              sseEvent('tool_result', {
                id: 'tc_2',
                name: 'get_user_patterns',
                result: { peakHour: 19, planAccuracy: userPatterns.planAccuracy },
              })
            )
          )
          await delay(300)

          controller.enqueue(
            encoder.encode(sseEvent('text', { content: '田中さんの行動パターンに基づいて、以下のスケジュールを提案します：' }))
          )
          await delay(200)

          const slots = ['19:00', '20:00', '20:30']
          const endSlots = ['20:00', '20:30', '21:30']
          const actions = incompleteTasks.map((t, i) => ({
            id: `a${i + 1}`,
            type: 'create_time_block',
            description: `${slots[i]}-${endSlots[i]} ${t.name}`,
            input: {
              date: new Date().toISOString().split('T')[0],
              startTime: slots[i],
              endTime: endSlots[i],
              taskId: t.id,
              title: t.name,
            },
          }))

          controller.enqueue(encoder.encode(sseEvent('action_proposal', { actions })))
        } else if (
          message.includes('レビュー') &&
          (message.includes('生成') || message.includes('振り返り'))
        ) {
          // Weekly review generation
          controller.enqueue(
            encoder.encode(sseEvent('text', { content: '田中さんの振り返りレビューを生成します。データを分析中...\n\n' }))
          )
          await delay(300)

          controller.enqueue(
            encoder.encode(
              sseEvent('tool_call', {
                id: 'tc_1',
                name: 'get_time_entries',
                input: { period: 'weekly' },
              })
            )
          )
          await delay(200)
          controller.enqueue(
            encoder.encode(
              sseEvent('tool_call', {
                id: 'tc_2',
                name: 'get_user_memory',
                input: {},
              })
            )
          )
          await delay(500)

          controller.enqueue(
            encoder.encode(
              sseEvent('tool_result', {
                id: 'tc_1',
                name: 'get_time_entries',
                result: { count: timeEntries.length },
              })
            )
          )
          await delay(200)
          controller.enqueue(
            encoder.encode(
              sseEvent('tool_result', {
                id: 'tc_2',
                name: 'get_user_memory',
                result: { strengths: userMemory.strengths, growthAreas: userMemory.growthAreas },
              })
            )
          )
          await delay(300)

          // Build task evaluations from worked tasks
          const workedTaskMap = new Map<string, string>()
          for (const te of timeEntries) {
            if (!te.taskId) continue
            const task = tasks.find(t => t.id === te.taskId)
            if (!task) continue
            const parentId = task.parentTaskId || task.id
            const parent = task.parentTaskId ? tasks.find(t => t.id === task.parentTaskId) : task
            if (parent && !workedTaskMap.has(parentId)) {
              workedTaskMap.set(parentId, parent.name)
            }
          }

          const taskEvaluations = [...workedTaskMap.entries()].map(([id, name]) => {
            const subtasks = tasks.filter(t => t.parentTaskId === id)
            const completed = subtasks.filter(t => t.completed).length
            const total = subtasks.length
            if (total > 0) {
              const ratio = completed / total
              return {
                taskName: name,
                status: ratio >= 0.5 ? 'good' : 'partial',
                comment: `サブタスク${completed}/${total}完了。${ratio >= 0.5 ? '順調に進行中。' : '進捗の加速が必要。'}`,
              }
            }
            return { taskName: name, status: 'partial', comment: '今週取り組んだ。引き続き進行中。' }
          })

          const timeEvaluations = weeklySummary.byGoal.map(g => {
            const targetMinutes = Math.round(g.minutes * 1.2)
            return {
              goalName: g.name,
              goalColor: g.color,
              actualMinutes: g.minutes,
              targetMinutes,
              comment: g.minutes >= targetMinutes * 0.9
                ? '目標を概ね達成。良いペース。'
                : `目標の${Math.round((g.minutes / targetMinutes) * 100)}%。改善の余地あり。`,
            }
          })

          const totalWeekHours = Math.floor(weeklySummary.totalMinutes / 60)
          const achieveRate = Math.round(
            (weeklySummary.plannedVsActual.actual / weeklySummary.plannedVsActual.planned) * 100
          )

          const reviewJson = {
            periodStart: weeklySummary.weekStart,
            periodEnd: weeklySummary.weekEnd,
            taskEvaluations,
            timeEvaluations,
            learningSummary: `今週は合計${totalWeekHours}時間の学習を実施。GCP IAMの理解が深まり、個人開発のAPI実装にも着手。田中さんの強みである${userMemory.strengths[0]}が活きた一週間。`,
            goodPoints: [
              `総学習時間${totalWeekHours}時間を確保`,
              `${weeklySummary.completedTasks}件のタスクを完了`,
              `連続${userPatterns.streakDays}日の学習継続`,
            ],
            improvementPoints: [
              weeklySummary.plannedVsActual.actual < weeklySummary.plannedVsActual.planned
                ? `計画${Math.floor(weeklySummary.plannedVsActual.planned / 60)}hに対し実績${Math.floor(weeklySummary.plannedVsActual.actual / 60)}h（達成率${achieveRate}%）`
                : null,
              'ブログ記事の公開まで至らなかった',
              '木曜の学習時間がゼロ',
            ].filter(Boolean) as string[],
            advice: [
              '田中さんは手を動かして学ぶスタイルが最も効果的です。ブログ執筆も「書きながら調べる」アプローチが合いそうです',
              '木曜の学習時間確保が難しい場合、朝15分だけでも確保すると週の途切れがなくなります',
              'ミドルウェア記事の完成を最優先に。LT登壇準備にもつながります',
            ],
            diaryFeedback: '日記から、設計レビューを通じてシニアエンジニアに必要なスキルを意識し始めているのが読み取れます。技術力＋言語化力の両方を磨く姿勢が素晴らしい。',
            summary: `全体として計画の${achieveRate}%を達成。GCP学習と個人開発は順調だが、アウトプットの「公開」が課題。`,
          }

          controller.enqueue(
            encoder.encode(
              sseEvent('text', {
                content: '```json\n' + JSON.stringify(reviewJson, null, 2) + '\n```',
              })
            )
          )
        } else {
          // General chat - personalized with persona name
          controller.enqueue(
            encoder.encode(
              sseEvent('text', {
                content: `田中さん、了解しました。「${body.message}」についてお手伝いします。\n\n現在の状況を確認しました。何か具体的に操作したいことがあれば教えてください。`,
              })
            )
          )
        }

        await delay(200)
        controller.enqueue(
          encoder.encode(
            sseEvent('done', {
              conversation_id: conversationId,
              tokens: { input: 500, output: 200 },
            })
          )
        )

        controller.close()
      },
    })

    return new HttpResponse(stream, {
      headers: { 'Content-Type': 'text/event-stream', 'Cache-Control': 'no-cache' },
    })
  }),

  // POST /agent/approve - Execute approved actions
  http.post(`${BASE_URL}/agent/approve`, async ({ request }) => {
    const body = (await request.json()) as {
      conversation_id: string
      action_ids: string[]
    }

    const encoder = new TextEncoder()
    const stream = new ReadableStream({
      async start(controller) {
        for (const actionId of body.action_ids) {
          controller.enqueue(
            encoder.encode(
              sseEvent('tool_call', {
                id: `tc_${actionId}`,
                name: 'create_time_block',
                input: {},
              })
            )
          )
          await delay(300)

          const newBlock = {
            id: generateId('tb'),
            title: `提案されたブロック ${actionId}`,
          }
          controller.enqueue(
            encoder.encode(
              sseEvent('tool_result', {
                id: `tc_${actionId}`,
                name: 'create_time_block',
                result: newBlock,
              })
            )
          )
          await delay(200)
        }

        controller.enqueue(
          encoder.encode(
            sseEvent('text', {
              content: `${body.action_ids.length}件のタイムブロックを作成しました。`,
            })
          )
        )
        controller.enqueue(
          encoder.encode(
            sseEvent('done', {
              conversation_id: body.conversation_id,
              tokens: { input: 100, output: 50 },
            })
          )
        )
        controller.close()
      },
    })

    return new HttpResponse(stream, {
      headers: { 'Content-Type': 'text/event-stream', 'Cache-Control': 'no-cache' },
    })
  }),

  // GET /conversations - List past conversations
  http.get(`${BASE_URL}/conversations`, () => {
    return HttpResponse.json({ conversations: mockConversations })
  }),

  // GET /conversations/:id - Get conversation messages
  http.get(`${BASE_URL}/conversations/:id`, ({ params }) => {
    const id = params.id as string
    const messages = mockConversationMessages[id]
    if (!messages) {
      return new HttpResponse(null, { status: 404 })
    }
    return HttpResponse.json({ messages })
  }),
]
