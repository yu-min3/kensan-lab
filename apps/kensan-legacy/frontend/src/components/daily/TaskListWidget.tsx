/**
 * タスクリストウィジェット（カード形式）
 * - 各タスクをコンパクトなカードで表示
 * - カード内に: タスク名、目標バッジ、期限表示
 * - 期限が近いものは背景色で強調
 * - 今日やるべき定期タスクを強調表示
 */
import { useMemo } from 'react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { ScrollArea } from '@/components/ui/scroll-area'
import { GoalBadge } from '@/components/common/GoalBadge'
import { WidgetError } from '@/components/common/WidgetError'
import { EntityMemoPopover } from '@/components/common/EntityMemoPopover'
import { useTaskManagerStore } from '@/stores/useTaskManagerStore'
import { useDraggable } from '@dnd-kit/core'
import { cn } from '@/lib/utils'
import {
  getDaysUntil,
  isScheduledForToday,
  getTaskFrequencyLabel,
  getUrgencyLevel,
  formatDaysUntil,
  type UrgencyLevel,
} from '@/lib/taskUtils'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Target,
  GripVertical,
  Flame,
  RefreshCw,
} from 'lucide-react'
import type { Task, Goal, Milestone } from '@/types'

// ドラッグデータの型（エクスポート - 他コンポーネントで使用）
export interface TaskDragData {
  type: 'task'
  taskId: string
  taskName: string
  estimatedMinutes?: number
  milestoneId?: string
  milestoneName?: string
  goalId?: string
  goalName?: string
  goalColor?: string
}


// カードの背景色
function getCardBgColor(level: UrgencyLevel): string {
  switch (level) {
    case 'danger':
      return 'bg-red-500/5 border-red-500/30'
    case 'warning':
      return 'bg-yellow-500/5 border-yellow-500/30'
    case 'normal':
      return 'bg-green-500/5 border-green-500/30'
    default:
      return 'bg-muted/30 border-border'
  }
}

// ドラッグ可能なタスクカード
interface DraggableTaskCardProps {
  task: Task
  goal?: Goal
  milestone?: Milestone
  daysUntil: number | null
  isScheduledToday: boolean
  frequencyLabel: string | null
  onTaskClick?: (taskId: string) => void
  onToggleComplete?: (taskId: string) => void
}

function DraggableTaskCard({ task, goal, milestone, daysUntil, isScheduledToday, frequencyLabel, onTaskClick, onToggleComplete }: DraggableTaskCardProps) {
  const dragData: TaskDragData = {
    type: 'task',
    taskId: task.id,
    taskName: task.name,
    estimatedMinutes: task.estimatedMinutes,
    milestoneId: milestone?.id,
    milestoneName: milestone?.name,
    goalId: goal?.id,
    goalName: goal?.name,
    goalColor: goal?.color,
  }

  const { attributes, listeners, setNodeRef, isDragging, transform } = useDraggable({
    id: `draggable-task-${task.id}`,
    data: dragData,
  })

  const style = transform
    ? { transform: `translate3d(${transform.x}px, ${transform.y}px, 0)` }
    : undefined

  const level = getUrgencyLevel(daysUntil)
  const bgColor = getCardBgColor(level)

  return (
    <div
      ref={setNodeRef}
      style={style}
      className={cn(
        'group rounded-lg border p-2 transition-all cursor-grab active:cursor-grabbing',
        // 今日やるべきタスクは青枠で強調
        isScheduledToday ? 'border-blue-500 border-2 bg-blue-500/5' : bgColor,
        'hover:shadow-sm',
        isDragging && 'opacity-50 shadow-lg z-50'
      )}
      {...listeners}
      {...attributes}
      role="listitem"
    >
      {/* 1行目: チェックボックス + タスク名 + 定期バッジ */}
      <div className="flex items-start gap-2">
        <Checkbox
          checked={false}
          onCheckedChange={() => onToggleComplete?.(task.id)}
          onClick={e => e.stopPropagation()}
          onPointerDown={e => e.stopPropagation()}
          className="flex-shrink-0 mt-0.5"
          aria-label={`Complete ${task.name}`}
        />
        <span
          className="text-sm font-medium flex-1 leading-tight cursor-pointer hover:text-primary"
          onClick={e => {
            e.stopPropagation()
            onTaskClick?.(task.id)
          }}
        >
          {task.name}
        </span>
        <EntityMemoPopover entityType="task" entityId={task.id} />
        {frequencyLabel && (
          <span className={cn(
            "text-[10px] px-1.5 py-0.5 rounded flex-shrink-0",
            isScheduledToday
              ? "bg-blue-500/20 text-blue-600"
              : "bg-muted text-muted-foreground"
          )}>
            {frequencyLabel}
          </span>
        )}
        <GripVertical className="h-3 w-3 text-muted-foreground/30 flex-shrink-0 opacity-0 group-hover:opacity-100 transition-opacity" />
      </div>

      {/* 2行目: 目標 + マイルストーン + 期限 */}
      <div className="flex items-center gap-2 mt-1.5 ml-6">
        {goal && <GoalBadge name={goal.name} color={goal.color} size="sm" />}
        {milestone && (
          <span className="text-[10px] text-muted-foreground truncate max-w-[100px]">
            {milestone.name}
          </span>
        )}
        {daysUntil !== null && (
          <span
            className={cn(
              'ml-auto text-[10px] font-medium flex items-center gap-0.5',
              level === 'danger' && 'text-red-500',
              level === 'warning' && 'text-yellow-600',
              level === 'normal' && 'text-green-600',
              level === 'no-deadline' && 'text-muted-foreground'
            )}
          >
            {level === 'danger' && <Flame className="h-3 w-3" />}
            {formatDaysUntil(daysUntil)}
          </span>
        )}
      </div>
    </div>
  )
}

interface TaskWithMeta {
  task: Task
  goal?: Goal
  milestone?: Milestone
  daysUntil: number | null
  isScheduledToday: boolean
  frequencyLabel: string | null
}

interface TaskListWidgetProps {
  onTaskClick?: (taskId: string) => void
}

export function TaskListWidget({ onTaskClick }: TaskListWidgetProps = {}) {
  const { goals, tasks, milestones, error, fetchAll, getTasksByMilestone, getMilestonesByGoal, getStandaloneTasks, toggleTaskComplete } = useTaskManagerStore()

  // タスクデータを整理（今日やるべきタスク優先、期限順にソート）
  const taskData = useMemo(() => {
    const data: TaskWithMeta[] = []

    // マイルストーンに属するタスク
    for (const goal of goals) {
      const goalMilestones = getMilestonesByGoal(goal.id)
      for (const milestone of goalMilestones) {
        if (milestone.status === 'archived' || milestone.status === 'completed') continue
        const milestoneTasks = getTasksByMilestone(milestone.id).filter(
          (t) => !t.parentTaskId && !t.completed
        )

        for (const task of milestoneTasks) {
          // タスク自身の期限を優先、なければマイルストーンの期限にフォールバック
          const daysUntil = getDaysUntil(task.dueDate) ?? getDaysUntil(milestone.targetDate)
          data.push({
            task,
            goal,
            milestone,
            daysUntil,
            isScheduledToday: isScheduledForToday(task),
            frequencyLabel: getTaskFrequencyLabel(task),
          })
        }
      }
    }

    // 目標なしタスク（期限はタスク自体の dueDate を使用）
    const standalone = getStandaloneTasks().filter(t => !t.parentTaskId && !t.completed)
    for (const task of standalone) {
      data.push({
        task,
        goal: undefined,
        milestone: undefined,
        daysUntil: getDaysUntil(task.dueDate),
        isScheduledToday: isScheduledForToday(task),
        frequencyLabel: getTaskFrequencyLabel(task),
      })
    }

    // ソート:
    // 1. 今日やるべきタスク → 上
    // 2. 期限あり → 近い順
    // 3. 期限なし → 最後
    data.sort((a, b) => {
      // 今日やるべきタスクを優先
      if (a.isScheduledToday !== b.isScheduledToday) {
        return a.isScheduledToday ? -1 : 1
      }

      // 期限でソート
      if (a.daysUntil === null && b.daysUntil === null) return 0
      if (a.daysUntil === null) return 1
      if (b.daysUntil === null) return -1
      return a.daysUntil - b.daysUntil
    })

    return data
  }, [goals, tasks, milestones, getMilestonesByGoal, getTasksByMilestone])

  return (
    <Card className="flex flex-col h-full">
      <CardHeader className="py-3 px-4 border-b flex-shrink-0">
        <CardTitle className="flex items-center gap-2 text-base">
          <Target className="h-4 w-4" />
          タスク
          <span className="text-xs ml-auto flex items-center gap-2">
            {taskData.filter(t => t.isScheduledToday).length > 0 && (
              <span className="text-blue-600 flex items-center gap-1">
                <RefreshCw className="h-3 w-3" />
                今日{taskData.filter(t => t.isScheduledToday).length}件
              </span>
            )}
            <span className="text-muted-foreground">
              全{taskData.length}件
            </span>
          </span>
        </CardTitle>
      </CardHeader>
      <CardContent className="flex-1 p-0 overflow-hidden">
        {error ? (
          <div className="p-4">
            <WidgetError message={error} onRetry={fetchAll} />
          </div>
        ) : (
        <ScrollArea className="h-full">
          <div role="list" className="p-2 space-y-2">
            {taskData.length === 0 ? (
              <div className="text-center py-8 text-muted-foreground text-sm">
                <Target className="h-8 w-8 mx-auto mb-2 opacity-50" />
                未完了のタスクがありません
              </div>
            ) : (
              taskData.map(({ task, goal, milestone, daysUntil, isScheduledToday, frequencyLabel }) => (
                <DraggableTaskCard
                  key={task.id}
                  task={task}
                  goal={goal}
                  milestone={milestone}
                  daysUntil={daysUntil}
                  isScheduledToday={isScheduledToday}
                  frequencyLabel={frequencyLabel}
                  onTaskClick={onTaskClick}
                  onToggleComplete={toggleTaskComplete}
                />
              ))
            )}
          </div>
        </ScrollArea>
        )}
      </CardContent>
    </Card>
  )
}
