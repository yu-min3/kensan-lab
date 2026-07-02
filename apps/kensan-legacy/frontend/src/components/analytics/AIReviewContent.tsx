import type { AIReviewReport, SuggestedAction, TaskEvaluationStatus } from '@/types'
import { useChatStore } from '@/stores/useChatStore'
import { Target, CheckCircle2, AlertTriangle, Lightbulb, ArrowRight, Bot } from 'lucide-react'
import { cn } from '@/lib/utils'

const statusConfig: Record<TaskEvaluationStatus, { label: string; className: string }> = {
  achieved: {
    label: '達成',
    className: 'bg-emerald-100 text-emerald-800 dark:bg-emerald-900/30 dark:text-emerald-400',
  },
  good: {
    label: '良好',
    className: 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/20 dark:text-emerald-300',
  },
  partial: {
    label: '一部未達',
    className: 'bg-amber-100 text-amber-800 dark:bg-amber-900/30 dark:text-amber-400',
  },
  missed: {
    label: '未達',
    className: 'bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-400',
  },
}

function SuggestedActionButton({ action }: { action: SuggestedAction }) {
  const sendPrefilled = useChatStore((s) => s.sendPrefilled)

  return (
    <button
      onClick={() => sendPrefilled(action.prompt, 'chat')}
      className="flex items-start gap-3 w-full px-3 py-3 text-left rounded-lg border border-brand/20 bg-brand/5 hover:bg-brand/10 transition-colors group"
    >
      <Bot className="h-4 w-4 text-brand shrink-0 mt-0.5" />
      <div className="flex-1 min-w-0">
        <span className="text-sm font-medium">{action.label}</span>
        {action.description && (
          <p className="text-xs text-muted-foreground mt-0.5 leading-relaxed">
            {action.description}
          </p>
        )}
      </div>
      <ArrowRight className="h-4 w-4 text-muted-foreground group-hover:text-brand transition-colors shrink-0 mt-0.5" />
    </button>
  )
}

interface AIReviewContentProps {
  review: AIReviewReport
}

export function AIReviewContent({ review }: AIReviewContentProps) {
  return (
    <div className="space-y-6">
      {/* 総括 (Summary) */}
      {review.summary && (
        <div className="rounded-lg bg-brand/5 border border-brand/20 p-4">
          <p className="text-sm leading-relaxed text-foreground whitespace-pre-wrap">
            {review.summary}
          </p>
        </div>
      )}

      {/* タスク評価 + 今週のポイント (2列) */}
      <div className="grid gap-6 md:grid-cols-2">
        {/* タスク評価 (コンパクト) */}
        <div className="rounded-lg border p-4">
          <div className="flex items-center gap-2 mb-3">
            <Target className="h-4 w-4 text-muted-foreground" />
            <h4 className="font-medium text-sm">タスク評価</h4>
          </div>
          <div className="space-y-1.5">
            {review.taskEvaluations.map((evaluation, i) => (
              <div key={i} className="flex items-center justify-between py-1">
                <span className="text-sm truncate mr-2">{evaluation.taskName}</span>
                <span
                  className={cn(
                    'text-[11px] px-1.5 py-0.5 rounded font-medium shrink-0',
                    statusConfig[evaluation.status].className
                  )}
                >
                  {statusConfig[evaluation.status].label}
                </span>
              </div>
            ))}
            {review.taskEvaluations.length === 0 && (
              <p className="text-xs text-muted-foreground">タスク評価データなし</p>
            )}
          </div>
          {review.taskSummary && (
            <p className="text-xs text-muted-foreground mt-2 pt-2 border-t">
              {review.taskSummary}
            </p>
          )}
        </div>

        {/* 今週のポイント (統合) */}
        <div className="rounded-lg bg-muted/50 p-4">
          <h4 className="font-medium text-sm mb-3">今週のポイント</h4>
          <div className="space-y-1.5">
            {review.goodPoints.map((point, i) => (
              <div key={`g-${i}`} className="flex items-start gap-2 text-sm">
                <CheckCircle2 className="h-4 w-4 text-emerald-500 shrink-0 mt-0.5" />
                <span>{point}</span>
              </div>
            ))}
            {review.improvementPoints.map((point, i) => (
              <div key={`i-${i}`} className="flex items-start gap-2 text-sm">
                <AlertTriangle className="h-4 w-4 text-amber-500 shrink-0 mt-0.5" />
                <span>{point}</span>
              </div>
            ))}
            {review.advice.map((item, i) => (
              <div key={`a-${i}`} className="flex items-start gap-2 text-sm">
                <Lightbulb className="h-4 w-4 text-blue-500 shrink-0 mt-0.5" />
                <span>{item}</span>
              </div>
            ))}
          </div>
        </div>
      </div>

      {/* Next Actions */}
      {review.suggestedActions && review.suggestedActions.length > 0 && (
        <div>
          <h4 className="text-xs font-semibold text-muted-foreground uppercase tracking-wider mb-2">
            Next Actions
          </h4>
          <div className="space-y-1.5">
            {review.suggestedActions.map((action, i) => (
              <SuggestedActionButton key={i} action={action} />
            ))}
          </div>
        </div>
      )}

      {/* Disclaimer */}
      <p className="text-xs text-muted-foreground">
        AIレビューはClaude APIで生成されています。参考情報としてご活用ください。
      </p>
    </div>
  )
}
