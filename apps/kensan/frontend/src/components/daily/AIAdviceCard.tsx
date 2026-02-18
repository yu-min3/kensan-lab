import { useCallback, useMemo, useRef, useState } from 'react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { useTimeBlockStore } from '@/stores/useTimeBlockStore'
import { useTimerStore } from '@/stores/useTimerStore'
import { useSettingsStore } from '@/stores/useSettingsStore'
import { streamAgentChat } from '@/api/services/agent'
import { getLocalDate, getLocalTime } from '@/lib/timezone'
import { formatDurationShort } from '@/lib/dateFormat'
import { calculateMinutesFromDatetimes } from '@/lib/taskUtils'
import type { AIPlanningResult, ProposedBlock } from '@/types'
import {
  Sparkles,
  Bot,
  Loader2,
  RefreshCw,
  Zap,
  Target,
  Calendar,
  AlertTriangle,
  CheckCircle2,
  BookOpen,
} from 'lucide-react'

interface AIAdviceCardProps {
  selectedDate: string // YYYY-MM-DD
}

function parsePlanningFromStream(text: string): AIPlanningResult | null {
  const jsonMatch = text.match(/```json\s*([\s\S]*?)```/) || text.match(/\{[\s\S]*\}/)
  if (!jsonMatch) return null

  try {
    const jsonStr = jsonMatch[1] ?? jsonMatch[0]
    const parsed = JSON.parse(jsonStr) as Record<string, unknown>

    return {
      message: (parsed.message as string) || undefined,
      yesterdayReview: (parsed.yesterdayReview as AIPlanningResult['yesterdayReview']) || undefined,
      insights: (parsed.insights as AIPlanningResult['insights']) || [],
      proposedBlocks: (parsed.proposedBlocks as AIPlanningResult['proposedBlocks']) || [],
      taskPriorities: (parsed.taskPriorities as AIPlanningResult['taskPriorities']) || [],
      alerts: (parsed.alerts as AIPlanningResult['alerts']) || [],
    }
  } catch {
    return null
  }
}

const CATEGORY_ICONS = {
  productivity: Zap,
  goal: Target,
  planning: Calendar,
  alert: AlertTriangle,
} as const

const ALERT_STYLES = {
  goal_stalled: 'border-amber-400 bg-amber-50 dark:bg-amber-950/20 text-amber-800 dark:text-amber-200',
  overdue: 'border-red-400 bg-red-50 dark:bg-red-950/20 text-red-800 dark:text-red-200',
  overcommit: 'border-orange-400 bg-orange-50 dark:bg-orange-950/20 text-orange-800 dark:text-orange-200',
} as const

const ACTION_BADGE_STYLES = {
  today: 'bg-emerald-100 text-emerald-800 dark:bg-emerald-900/30 dark:text-emerald-300',
  defer: 'bg-gray-100 text-gray-800 dark:bg-gray-800 dark:text-gray-300',
  split: 'bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-300',
} as const

const ACTION_LABELS = {
  today: '今日やる',
  defer: '延期',
  split: '分割',
} as const

export function AIAdviceCard({ selectedDate }: AIAdviceCardProps) {
  const { timeBlocks, timeEntries, addTimeBlock } = useTimeBlockStore()
  const { currentTimer } = useTimerStore()
  const timezone = useSettingsStore((s) => s.timezone) || 'Asia/Tokyo'

  const [planningResult, setPlanningResult] = useState<AIPlanningResult | null>(null)
  const [isGenerating, setIsGenerating] = useState(false)
  const [streamText, setStreamText] = useState('')
  const [selectedBlockIndices, setSelectedBlockIndices] = useState<Set<number>>(new Set())
  const [isApplying, setIsApplying] = useState(false)
  const [appliedCount, setAppliedCount] = useState(0)
  const abortRef = useRef<AbortController | null>(null)

  // Build context hints for the AI
  const adviceContext = useMemo(() => {
    const now = new Date()
    const todayBlocks = timeBlocks.filter(
      (b) => getLocalDate(b.startDatetime, timezone) === selectedDate,
    )
    const todayEntries = timeEntries.filter(
      (e) => getLocalDate(e.startDatetime, timezone) === selectedDate,
    )
    const completedMinutes = calculateMinutesFromDatetimes(todayEntries)

    const blockDetails = todayBlocks.map((b) => {
      const start = getLocalTime(b.startDatetime, timezone)
      const end = getLocalTime(b.endDatetime, timezone)
      return `${start}〜${end}: ${b.taskName}`
    }).join(', ')

    return {
      currentHour: String(now.getHours()),
      plannedBlocks: `${todayBlocks.length}件${todayBlocks.length > 0 ? ` (${blockDetails})` : ''}`,
      completedEntries: `${todayEntries.length}件 (${formatDurationShort(completedMinutes)})`,
      timerActive: String(currentTimer !== null),
    }
  }, [timeBlocks, timeEntries, currentTimer, selectedDate, timezone])

  const handleGenerate = useCallback(async () => {
    setIsGenerating(true)
    setStreamText('')
    setPlanningResult(null)
    setSelectedBlockIndices(new Set())
    setAppliedCount(0)

    abortRef.current = new AbortController()

    try {
      const stream = streamAgentChat(
        {
          message: '今日のアドバイスをお願いします',
          situation: 'daily_advice',
          context: adviceContext,
        },
        abortRef.current.signal,
      )

      let fullText = ''

      for await (const event of stream) {
        if (event.type === 'text') {
          const chunk = event.data.content as string
          fullText += chunk
          setStreamText((prev) => prev + chunk)
        }
      }

      const parsed = parsePlanningFromStream(fullText)
      if (parsed) {
        setPlanningResult(parsed)
        setStreamText('')
        // Select all blocks by default
        setSelectedBlockIndices(new Set(parsed.proposedBlocks.map((_, i) => i)))
      }
    } catch (err) {
      if ((err as Error).name !== 'AbortError') {
        setStreamText((prev) => prev + '\n\nエラーが発生しました。もう一度お試しください。')
      }
    } finally {
      setIsGenerating(false)
    }
  }, [adviceContext])

  const handleToggleBlock = useCallback((index: number) => {
    setSelectedBlockIndices((prev) => {
      const next = new Set(prev)
      if (next.has(index)) {
        next.delete(index)
      } else {
        next.add(index)
      }
      return next
    })
  }, [])

  const handleApplyBlocks = useCallback(async () => {
    if (!planningResult) return
    const blocks = planningResult.proposedBlocks.filter((_, i) => selectedBlockIndices.has(i))
    if (blocks.length === 0) return

    setIsApplying(true)
    let count = 0
    for (const block of blocks) {
      await addTimeBlock(selectedDate, block.startTime, selectedDate, block.endTime, {
        taskId: block.taskId || undefined,
        taskName: block.taskName,
        goalId: block.goalId || undefined,
        goalName: block.goalName || undefined,
        goalColor: block.goalColor || undefined,
      })
      count++
    }
    setAppliedCount(count)
    setIsApplying(false)
  }, [planningResult, selectedBlockIndices, selectedDate, addTimeBlock])

  return (
    <Card className="border-brand/30 bg-gradient-to-br from-brand/[0.03] to-transparent">
      <CardHeader className="flex flex-row items-center justify-between">
        <CardTitle className="flex items-center gap-2 text-base text-brand">
          <Sparkles className="h-5 w-5" />
          AIアドバイス
          <Badge variant="secondary" className="text-[10px] font-normal">
            AI生成
          </Badge>
        </CardTitle>
        {!isGenerating && (
          <Button
            variant={planningResult ? 'outline' : 'default'}
            size="sm"
            className={planningResult ? 'gap-1.5' : 'gap-1.5 bg-brand text-brand-foreground hover:bg-brand/90'}
            onClick={handleGenerate}
          >
            {planningResult ? (
              <>
                <RefreshCw className="h-3.5 w-3.5" />
                再生成
              </>
            ) : (
              <>
                <Sparkles className="h-3.5 w-3.5" />
                アドバイスを取得
              </>
            )}
          </Button>
        )}
      </CardHeader>
      <CardContent>
        {/* Generating */}
        {isGenerating && (
          <div className="space-y-4">
            <div className="rounded-lg border-l-4 border-brand bg-brand/5 p-4">
              <div className="flex items-center gap-2 text-sm">
                <Loader2 className="h-4 w-4 animate-spin text-brand" />
                <span className="font-medium text-brand">
                  アドバイスを生成中...
                </span>
              </div>
              <p className="text-xs text-brand/60 mt-1">
                状況を分析し、最適なアドバイスを作成しています
              </p>
            </div>
            {streamText && (
              <div className="text-sm text-muted-foreground whitespace-pre-wrap leading-relaxed max-h-64 overflow-y-auto">
                {streamText}
              </div>
            )}
          </div>
        )}

        {/* Result */}
        {planningResult && !isGenerating && (
          <div className="space-y-5">
            {/* Warm Message */}
            {planningResult.message && (
              <div className="rounded-lg bg-brand/5 border border-brand/20 p-4">
                <p className="text-sm leading-relaxed text-foreground">
                  {planningResult.message}
                </p>
              </div>
            )}

            {/* Yesterday Review */}
            {planningResult.yesterdayReview && (
              <div className="space-y-2">
                <h4 className="text-xs font-medium text-muted-foreground uppercase tracking-wide">
                  昨日の振り返り
                </h4>
                <div className="rounded-md border p-3 space-y-2">
                  <p className="text-sm text-foreground">{planningResult.yesterdayReview.summary}</p>
                  {planningResult.yesterdayReview.highlights.length > 0 && (
                    <div className="space-y-1">
                      {planningResult.yesterdayReview.highlights.map((h, i) => (
                        <div key={i} className="flex items-start gap-2 text-xs text-foreground/70">
                          <CheckCircle2 className="h-3.5 w-3.5 mt-0.5 flex-shrink-0 text-emerald-500" />
                          <span>{h}</span>
                        </div>
                      ))}
                    </div>
                  )}
                  {planningResult.yesterdayReview.learningConnections.length > 0 && (
                    <div className="space-y-1 border-t pt-2">
                      {planningResult.yesterdayReview.learningConnections.map((lc, i) => (
                        <div key={i} className="flex items-start gap-2 text-xs text-foreground/70">
                          <BookOpen className="h-3.5 w-3.5 mt-0.5 flex-shrink-0 text-blue-500" />
                          <span>{lc}</span>
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              </div>
            )}

            {/* Alerts */}
            {planningResult.alerts.length > 0 && (
              <div className="space-y-2">
                {planningResult.alerts.map((alert, i) => (
                  <div
                    key={i}
                    className={`rounded-lg border-l-4 p-3 text-sm ${ALERT_STYLES[alert.type] || ALERT_STYLES.overcommit}`}
                  >
                    <div className="flex items-center gap-2">
                      <AlertTriangle className="h-4 w-4 flex-shrink-0" />
                      {alert.message}
                    </div>
                  </div>
                ))}
              </div>
            )}

            {/* Insights */}
            {planningResult.insights.length > 0 && (
              <div className="space-y-1.5">
                <h4 className="text-xs font-medium text-muted-foreground uppercase tracking-wide">
                  インサイト
                </h4>
                {planningResult.insights.map((insight, i) => {
                  const Icon = CATEGORY_ICONS[insight.category] || Zap
                  return (
                    <div
                      key={i}
                      className="flex items-center gap-2 py-1"
                    >
                      <Icon className="h-4 w-4 flex-shrink-0 text-foreground/50" />
                      <span className="text-sm">{insight.title}</span>
                    </div>
                  )
                })}
              </div>
            )}

            {/* Proposed Blocks */}
            {planningResult.proposedBlocks.length > 0 && (
              <div className="space-y-2">
                <div className="flex items-center justify-between">
                  <h4 className="text-xs font-medium text-muted-foreground uppercase tracking-wide">
                    タイムブロック提案
                  </h4>
                  {appliedCount > 0 ? (
                    <span className="flex items-center gap-1 text-xs text-emerald-600 dark:text-emerald-400">
                      <CheckCircle2 className="h-3.5 w-3.5" />
                      {appliedCount}件適用済み
                    </span>
                  ) : (
                    <Button
                      size="sm"
                      variant="default"
                      className="gap-1.5"
                      disabled={selectedBlockIndices.size === 0 || isApplying}
                      onClick={handleApplyBlocks}
                    >
                      {isApplying ? (
                        <Loader2 className="h-3.5 w-3.5 animate-spin" />
                      ) : (
                        <Calendar className="h-3.5 w-3.5" />
                      )}
                      まとめて適用（{selectedBlockIndices.size}件）
                    </Button>
                  )}
                </div>
                <div className="space-y-1.5">
                  {planningResult.proposedBlocks.map((block, i) => (
                    <ProposedBlockRow
                      key={i}
                      block={block}
                      checked={selectedBlockIndices.has(i)}
                      disabled={appliedCount > 0}
                      onToggle={() => handleToggleBlock(i)}
                    />
                  ))}
                </div>
              </div>
            )}

            {/* Task Priorities */}
            {planningResult.taskPriorities.length > 0 && (
              <div className="space-y-1.5">
                <h4 className="text-xs font-medium text-muted-foreground uppercase tracking-wide">
                  タスク優先度
                </h4>
                <div className="rounded-lg border p-3 space-y-1">
                  {planningResult.taskPriorities.map((tp, i) => (
                    <div
                      key={i}
                      className="flex items-center justify-between py-1"
                    >
                      <span className="text-sm truncate mr-2">{tp.taskName}</span>
                      <span
                        className={`shrink-0 text-[11px] px-1.5 py-0.5 rounded font-medium ${ACTION_BADGE_STYLES[tp.suggestedAction]}`}
                      >
                        {ACTION_LABELS[tp.suggestedAction]}
                      </span>
                    </div>
                  ))}
                </div>
              </div>
            )}
          </div>
        )}

        {/* Fallback: stream text couldn't be parsed */}
        {!planningResult && !isGenerating && streamText && (
          <div>
            <div className="text-sm whitespace-pre-wrap leading-relaxed">{streamText}</div>
            <p className="text-xs text-muted-foreground mt-4">
              構造化データへの変換に失敗しました。テキストとして表示しています。
            </p>
          </div>
        )}

        {/* Empty state */}
        {!planningResult && !isGenerating && !streamText && (
          <div className="text-center py-8">
            <Bot className="h-10 w-10 mx-auto mb-3 text-brand/30" />
            <p className="text-sm text-muted-foreground mb-1">
              状況に応じたアドバイスを提案します
            </p>
            <p className="text-xs text-muted-foreground">
              時間帯・進捗に合わせて計画提案・進捗チェック・振り返りを自動で切り替えます
            </p>
          </div>
        )}
      </CardContent>
    </Card>
  )
}

function ProposedBlockRow({
  block,
  checked,
  disabled,
  onToggle,
}: {
  block: ProposedBlock
  checked: boolean
  disabled: boolean
  onToggle: () => void
}) {
  return (
    <label className="flex items-start gap-2.5 rounded-md border p-2.5 cursor-pointer hover:bg-accent/50 transition-colors">
      <Checkbox
        checked={checked}
        disabled={disabled}
        onCheckedChange={onToggle}
        className="mt-0.5"
      />
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2">
          {block.goalColor && (
            <div
              className="w-2.5 h-2.5 rounded-full flex-shrink-0"
              style={{ backgroundColor: block.goalColor }}
            />
          )}
          <span className="text-sm font-medium truncate">{block.taskName}</span>
          <span className="text-xs text-muted-foreground shrink-0">
            {block.startTime}〜{block.endTime}
          </span>
        </div>
        {block.goalName && (
          <p className="text-xs text-muted-foreground mt-0.5">{block.goalName}</p>
        )}
        <p className="text-xs text-foreground/60 mt-0.5">{block.reason}</p>
      </div>
    </label>
  )
}
