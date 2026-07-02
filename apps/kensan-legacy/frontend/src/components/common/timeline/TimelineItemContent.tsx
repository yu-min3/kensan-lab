import { Edit, Trash2, Play } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { GoalBadge } from '@/components/common/GoalBadge'
import { ConfirmPopover } from '@/components/common/ConfirmPopover'

export interface ActionButton {
  type: 'edit' | 'delete' | 'timer'
  onClick: () => void
  label?: string
  confirmMessage?: string
}

export interface TimelineItemContentProps {
  taskName: string
  goalId?: string
  goalName?: string
  goalColor?: string
  milestoneName?: string
  startTimeLabel: string
  endTimeLabel: string
  actions?: ActionButton[]
  /** Label shown instead of goal badge when no goal (e.g. "作業中" for running timer) */
  noGoalLabel?: string
  /** Extra element shown after task name (e.g. "REC" badge) */
  trailingBadge?: React.ReactNode
  /** When true, prefix end time with "翌" to indicate midnight crossover */
  crossesMidnight?: boolean
}

/**
 * Shared content layout for timeline items (time blocks, entries, running timer).
 * Line 1: Task name + action buttons
 * Line 2: Goal badge + milestone + time range
 */
export function TimelineItemContent({
  taskName,
  goalId,
  goalName,
  goalColor,
  milestoneName,
  startTimeLabel,
  endTimeLabel,
  actions,
  noGoalLabel = '目標なし',
  trailingBadge,
  crossesMidnight,
}: TimelineItemContentProps) {
  const hasGoal = !!(goalId && goalColor)

  return (
    <>
      <div className="flex items-center gap-1">
        <span className="truncate font-medium flex-1">{taskName}</span>
        {trailingBadge}
        {actions && actions.length > 0 && (
          <div className="opacity-0 group-hover:opacity-100 flex gap-0.5 ml-1">
            {actions.map((action) => {
              if (action.type === 'timer') {
                return (
                  <Button
                    key="timer"
                    variant="ghost"
                    size="sm"
                    className="h-5 w-5 p-0 text-primary hover:text-primary"
                    onClick={(e) => {
                      e.stopPropagation()
                      action.onClick()
                    }}
                    onMouseDown={(e) => e.stopPropagation()}
                    title="タイマー開始"
                  >
                    <Play className="h-3 w-3" />
                  </Button>
                )
              }
              if (action.type === 'edit') {
                return (
                  <Button
                    key="edit"
                    variant="ghost"
                    size="sm"
                    className="h-5 w-5 p-0"
                    onClick={(e) => {
                      e.stopPropagation()
                      action.onClick()
                    }}
                    onMouseDown={(e) => e.stopPropagation()}
                    title="編集"
                  >
                    <Edit className="h-3 w-3" />
                  </Button>
                )
              }
              if (action.type === 'delete') {
                return (
                  <ConfirmPopover
                    key="delete"
                    message={action.confirmMessage ?? '削除しますか？'}
                    confirmLabel="削除"
                    onConfirm={action.onClick}
                    variant="destructive"
                  >
                    <Button
                      variant="ghost"
                      size="sm"
                      className="h-5 w-5 p-0 text-destructive hover:text-destructive"
                      onMouseDown={(e) => e.stopPropagation()}
                      title="削除"
                    >
                      <Trash2 className="h-3 w-3" />
                    </Button>
                  </ConfirmPopover>
                )
              }
              return null
            })}
          </div>
        )}
      </div>
      <div className="flex items-center gap-1 text-muted-foreground">
        {hasGoal ? (
          <GoalBadge name={goalName!} color={goalColor!} size="sm" />
        ) : (
          <span className="text-[10px] px-1 py-0.5 rounded bg-muted-foreground/10 shrink-0">
            {noGoalLabel}
          </span>
        )}
        {milestoneName && (
          <span
            className="text-[10px] truncate max-w-[80px]"
            title={milestoneName}
          >
            {milestoneName}
          </span>
        )}
        <span className="text-[10px] shrink-0 ml-auto">
          {startTimeLabel} - {crossesMidnight && <span className="text-[9px] text-muted-foreground/70">翌</span>}{endTimeLabel}
        </span>
      </div>
    </>
  )
}
