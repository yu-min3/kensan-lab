import { useCallback, useRef } from 'react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { useAnalyticsStore } from '@/stores/useAnalyticsStore'
import { streamAgentChat } from '@/api/services/agent'
import type { AIReviewReport, SuggestedAction, TaskEvaluation, TimeEvaluation } from '@/types'
import { AIReviewContent } from './AIReviewContent'
import { Sparkles, Bot, Loader2, RefreshCw } from 'lucide-react'

interface AIReviewSectionProps {
  startDate: string
  endDate: string
}

/**
 * Parse the streamed review text into a structured AIReviewReport.
 * Expects JSON output from the AI agent.
 */
function parseReviewFromStream(text: string): AIReviewReport | null {
  // Try to extract JSON from the stream text
  // The agent may wrap it in markdown code blocks
  const jsonMatch = text.match(/```json\s*([\s\S]*?)```/) || text.match(/\{[\s\S]*\}/)
  if (!jsonMatch) return null

  try {
    const jsonStr = jsonMatch[1] ?? jsonMatch[0]
    const parsed = JSON.parse(jsonStr) as Record<string, unknown>

    return {
      id: crypto.randomUUID(),
      periodStart: (parsed.periodStart as string) || '',
      periodEnd: (parsed.periodEnd as string) || '',
      taskEvaluations: (parsed.taskEvaluations as TaskEvaluation[]) || [],
      taskSummary: (parsed.taskSummary as string) || undefined,
      timeEvaluations: (parsed.timeEvaluations as TimeEvaluation[]) || [],
      learningSummary: (parsed.learningSummary as string) || undefined,
      learningSummaryData: undefined,
      goodPoints: (parsed.goodPoints as string[]) || [],
      improvementPoints: (parsed.improvementPoints as string[]) || [],
      advice: (parsed.advice as string[]) || [],
      diaryFeedback: (parsed.diaryFeedback as string) || undefined,
      suggestedActions: (parsed.suggestedActions as SuggestedAction[]) || undefined,
      summary: (parsed.summary as string) || '',
      createdAt: new Date(),
    }
  } catch {
    return null
  }
}

function buildReviewContext(store: {
  weeklySummary: import('@/types').WeeklySummary | null
  dailyStudyHours: import('@/stores/useAnalyticsStore').DailyStudyHour[]
}): Record<string, string> | undefined {
  const ctx: Record<string, string> = {}

  if (store.weeklySummary) {
    const ws = store.weeklySummary
    const goalLines = ws.byGoal
      .map((g) => `- ${g.name} (color:${g.color}): ${Math.floor(g.minutes / 60)}h${g.minutes % 60}m`)
      .join('\n')
    ctx['週間サマリー'] = [
      `期間: ${ws.weekStart} 〜 ${ws.weekEnd}`,
      `総稼働: ${Math.floor(ws.totalMinutes / 60)}h${ws.totalMinutes % 60}m`,
      `完了タスク: ${ws.completedTasks}件`,
      `計画vs実績: 計画${ws.plannedVsActual.planned}分 / 実績${ws.plannedVsActual.actual}分`,
      `目標別:\n${goalLines}`,
    ].join('\n')
  }

  if (store.dailyStudyHours.length > 0) {
    const lines = store.dailyStudyHours.map((d) => {
      const byGoal = d.byGoal?.map((g) => `${g.name}:${g.minutes}m`).join(', ') ?? ''
      return `- ${d.date}(${d.day}): ${d.hours.toFixed(1)}h${byGoal ? ` [${byGoal}]` : ''}`
    })
    ctx['日別稼働'] = lines.join('\n')
  }

  return Object.keys(ctx).length > 0 ? ctx : undefined
}

export function AIReviewSection({ startDate, endDate }: AIReviewSectionProps) {
  const {
    currentReview,
    isGeneratingReview,
    reviewStreamText,
    weeklySummary,
    dailyStudyHours,
    setCurrentReview,
    setGeneratingReview,
    setReviewStreamText,
    appendReviewStreamText,
  } = useAnalyticsStore()

  const abortRef = useRef<AbortController | null>(null)

  const handleGenerate = useCallback(async () => {
    const MAX_RETRIES = 3

    const reviewMessage = `${startDate}〜${endDate}の振り返りレビューを生成してください。
まず get_notes(type="diary", start_date="${startDate}", end_date="${endDate}") と get_notes(type="learning", start_date="${startDate}", end_date="${endDate}") でこの期間の日記・学習記録を取得し、内容を踏まえてレビューしてください。
以下のJSON形式で出力してください:
\`\`\`json
{
  "periodStart": "${startDate}",
  "periodEnd": "${endDate}",
  "summary": "今週全体の総括（3-5文。最も重要なフィールド。成果・課題・来週への展望を簡潔に。学習記録や日記があればその内容にも自然に触れる。改行で段落分けしてよい）",
  "taskEvaluations": [{"taskName": "タスク名", "status": "achieved|good|partial|missed", "comment": "1文以内の補足（省略可）"}],
  "taskSummary": "上記以外のタスク状況をまとめた1文（例: '他8件は順調に完了。ストレッチ・ジムは週5/7回達成。'）",
  "timeEvaluations": [{"goalName": "目標名", "goalColor": "#色コード", "actualMinutes": 数値, "comment": "簡潔な定性評価"}],
  "goodPoints": ["具体的な洞察（1-2項目、各1文）"],
  "improvementPoints": ["改善点（1-2項目、各1文）"],
  "advice": ["アドバイス（1項目、1文）"],
  "suggestedActions": [
    {"label": "短い行動名", "description": "振り返りを踏まえた提案理由（1-2文）", "type": "chat", "prompt": "レビュー内容を踏まえた具体的な相談プロンプト"}
  ]
}
\`\`\`

重要なルール:
- summaryは最重要フィールド。読者がこれだけで週の振り返りを把握できるように書くこと
- summaryには学習記録（type=learning）や日記（type=diary）の内容も自然に織り込むこと。例:「レイクハウス関連の学習を深め…」「喫茶店で気分転換もできたようで」等
- get_notes呼び出し時は必ずstart_dateとend_dateを指定。期間外のノートは絶対に言及しない

taskEvaluations（厳選ルール）:
- **最大5件**に絞る。全タスクを列挙しない
- 優先的に含めるもの: achieved（完了した重要タスク）、partial（遅れ気味）、missed（期限超過）
- 「良好」で特筆事項のないタスクは省略してよい
- 定期タスク（ストレッチ、ジム、ルーティン等）は個別に列挙せず、taskSummaryにまとめる
- status: achieved=完了済み、good=今週作業して順調、partial=作業したが遅れ気味、missed=期限超過
- comment: 1文以内。missedには祝福表現を使わない

taskSummary:
- taskEvaluationsに含めなかったタスクの状況を1文でまとめる
- 完了タスク数、定期タスクの実施回数などを含める
- 例: "他8件は順調に完了。ストレッチ・ジムは週5/7回達成。"

- timeEvaluations の goalColor と actualMinutes は「提供済みデータ」の週間サマリーの値をそのまま使うこと
- timeEvaluations の comment: 数値の繰り返しではなく質的な分析を簡潔に
- goodPoints/improvementPoints/advice: 各1-2項目、1文ずつ。一般論ではなくユーザー特性に基づく内容にすること

suggestedActions:
- 2〜3個生成。typeは常に"chat"
- **1つ目は必ず「来週のスケジュール相談」にすること**（label例: "来週の予定を組む"）。promptには今週の振り返りを踏まえた来週の時間配分の相談文を含め、「タイムブロックを作成して」「スケジュールを組んで」等タイムブロック作成を明示する表現を使うこと。「タスクの細分化」だけで終わる表現にしないこと
- 2つ目以降はレビュー内容に基づく提案（期限見直し、目標調整等）
- label: 短い行動名
- description: 提案理由を1-2文で。振り返りの具体的内容に基づくこと
- prompt: AIエージェントへの相談文。具体的内容（タスク名、目標名、時間データ等）を盛り込み、エージェントがツール実行できる依頼文にすること。「相談したい」「見直したい」「調整したい」「計画を立てて」等の表現を含めること`

    const reviewContext = buildReviewContext({ weeklySummary, dailyStudyHours })

    setGeneratingReview(true)
    setReviewStreamText('')
    setCurrentReview(null)

    for (let attempt = 1; attempt <= MAX_RETRIES; attempt++) {
      abortRef.current = new AbortController()

      try {
        const stream = streamAgentChat(
          { message: reviewMessage, situation: 'review', context: reviewContext },
          abortRef.current.signal
        )

        let fullText = ''

        for await (const event of stream) {
          if (event.type === 'text') {
            const chunk = event.data.content as string
            fullText += chunk
            appendReviewStreamText(chunk)
          }
        }

        // Try to parse the completed stream into structured data
        const parsed = parseReviewFromStream(fullText)
        if (parsed) {
          setCurrentReview(parsed)
          setReviewStreamText('')
          break // Success
        }

        // Parse failed — retry if attempts remain
        if (attempt < MAX_RETRIES) {
          setReviewStreamText('')
          continue
        }
        // Final attempt also failed to parse — leave raw text visible
      } catch (err) {
        if ((err as Error).name === 'AbortError') break

        if (attempt < MAX_RETRIES) {
          setReviewStreamText('')
          continue
        }
        appendReviewStreamText('\n\nエラーが発生しました。もう一度お試しください。')
      }
    }

    setGeneratingReview(false)
  }, [
    startDate,
    endDate,
    weeklySummary,
    dailyStudyHours,
    setGeneratingReview,
    setReviewStreamText,
    setCurrentReview,
    appendReviewStreamText,
  ])

  const formatCreatedAt = (date: Date) => {
    const d = new Date(date)
    const month = d.getMonth() + 1
    const day = d.getDate()
    const hours = String(d.getHours()).padStart(2, '0')
    const minutes = String(d.getMinutes()).padStart(2, '0')
    return `${month}/${day} ${hours}:${minutes}`
  }

  return (
    <Card className="border-brand/30 bg-gradient-to-br from-brand/[0.03] to-transparent">
      <CardHeader className="flex flex-row items-center justify-between">
        <CardTitle className="flex items-center gap-2 text-brand">
          <Sparkles className="h-5 w-5" />
          AI振り返りレビュー
          <Badge variant="secondary" className="text-[10px] font-normal">
            AI生成
          </Badge>
        </CardTitle>
        {!isGeneratingReview && (
          <Button
            variant={currentReview ? 'outline' : 'default'}
            size="sm"
            className={currentReview ? 'gap-1.5' : 'gap-1.5 bg-brand text-brand-foreground hover:bg-brand/90'}
            onClick={handleGenerate}
          >
            {currentReview ? (
              <>
                <RefreshCw className="h-3.5 w-3.5" />
                再生成
              </>
            ) : (
              <>
                <Sparkles className="h-3.5 w-3.5" />
                レビューを生成
              </>
            )}
          </Button>
        )}
      </CardHeader>
      <CardContent>
        {/* 生成中 */}
        {isGeneratingReview && (
          <div className="space-y-4">
            <div className="rounded-lg border-l-4 border-brand bg-brand/5 p-4">
              <div className="flex items-center gap-2 text-sm">
                <Loader2 className="h-4 w-4 animate-spin text-brand" />
                <span className="font-medium text-brand">
                  レビューを生成中...
                </span>
              </div>
              <p className="text-xs text-brand/60 mt-1">
                データを分析しています
              </p>
            </div>
            {reviewStreamText && (
              <div className="text-sm text-muted-foreground whitespace-pre-wrap leading-relaxed max-h-64 overflow-y-auto">
                {reviewStreamText}
              </div>
            )}
          </div>
        )}

        {/* 生成済み */}
        {currentReview && !isGeneratingReview && (
          <div>
            <p className="text-xs text-muted-foreground mb-4">
              生成日: {formatCreatedAt(currentReview.createdAt)}
            </p>
            <AIReviewContent review={currentReview} />
          </div>
        )}

        {/* ストリーム完了したがパースできなかった場合（フォールバック表示） */}
        {!currentReview && !isGeneratingReview && reviewStreamText && (
          <div>
            <div className="text-sm whitespace-pre-wrap leading-relaxed">
              {reviewStreamText}
            </div>
            <p className="text-xs text-muted-foreground mt-4">
              AIレビューはClaude APIで生成されています。参考情報としてご活用ください。
            </p>
          </div>
        )}

        {/* 未生成 (Empty State) */}
        {!currentReview && !isGeneratingReview && !reviewStreamText && (
          <div className="text-center py-12">
            <Bot className="h-12 w-12 mx-auto mb-4 text-brand/30" />
            <p className="text-sm text-muted-foreground mb-1">
              この期間のAIレビューはまだ生成されていません
            </p>
            <p className="text-xs text-muted-foreground">
              右上のボタンからデータを分析し、タスク評価・時間分析・振り返りを生成できます
            </p>
          </div>
        )}
      </CardContent>
    </Card>
  )
}
