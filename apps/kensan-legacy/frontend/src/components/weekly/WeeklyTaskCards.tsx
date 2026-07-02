/**
 * 週間タスクカード（目標→マイルストーンでグループ化）
 * - 目標ごとにカードを分け、マイルストーンでグループ化
 * - ドラッグ&ドロップでカレンダーにTimeBlock作成
 * - TaskListWidgetと同じTaskDragData型を使用
 */
import { useMemo } from 'react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { ScrollArea } from '@/components/ui/scroll-area'
import { useTaskManagerStore } from '@/stores/useTaskManagerStore'
import { useDraggable } from '@dnd-kit/core'
import { getDaysUntil, formatDaysUntil, getUrgencyLevel } from '@/lib/taskUtils'
import { formatDurationShort } from '@/lib/dateFormat'
import { cn } from '@/lib/utils'
import { Clock, LayoutGrid, Target, Flag, GripVertical } from 'lucide-react'
import type { Task, Goal, Milestone } from '@/types'
import type { TaskDragData } from '@/components/daily/TaskListWidget'

interface WeeklyTaskCardsProps {
  onTaskClick: (taskId: string) => void
}

export function WeeklyTaskCards({ onTaskClick }: WeeklyTaskCardsProps) {
  const { tasks, goals, milestones } = useTaskManagerStore()

  const goalGroups = useMemo(() => {
    const incompleteTasks = tasks.filter(t => !t.completed && !t.parentTaskId)
    const activeGoals = goals.filter(g => g.status === 'active')

    return activeGoals.map(goal => {
      const goalMilestones = milestones.filter(m => m.goalId === goal.id && m.status === 'active')
      const milestonesWithTasks = goalMilestones.map(ms => {
        const msTasks = incompleteTasks.filter(t => t.milestoneId === ms.id)
        return { milestone: ms, tasks: msTasks }
      }).filter(m => m.tasks.length > 0)

      const totalTasks = milestonesWithTasks.reduce((sum, m) => sum + m.tasks.length, 0)
      return { goal, milestones: milestonesWithTasks, totalTasks }
    }).filter(g => g.totalTasks > 0)
  }, [tasks, goals, milestones])

  if (goalGroups.length === 0) return null

  return (
    <Card>
      <CardHeader className="pb-2">
        <CardTitle className="text-sm font-medium flex items-center gap-2">
          <LayoutGrid className="h-4 w-4" />
          タスク
          <span className="text-xs text-muted-foreground ml-auto">
            ドラッグしてカレンダーに配置
          </span>
        </CardTitle>
      </CardHeader>
      <CardContent className="pt-0">
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3">
          {goalGroups.map(({ goal, milestones: gMilestones, totalTasks }) => (
            <div
              key={goal.id}
              className="border rounded-lg overflow-hidden"
              style={{ borderTopWidth: 4, borderTopColor: goal.color }}
            >
              {/* Goal header */}
              <div className="bg-muted/30 px-3 py-2 border-b">
                <div className="flex items-center gap-2">
                  <Target className="h-4 w-4 text-muted-foreground" />
                  <span className="font-medium text-sm flex-1 truncate">{goal.name}</span>
                  <span className="text-xs text-muted-foreground">{totalTasks}</span>
                </div>
              </div>

              {/* Milestones */}
              <ScrollArea className="max-h-64">
                <div className="p-2 space-y-2">
                  {gMilestones.map(({ milestone, tasks: msTasks }) => (
                    <div key={milestone.id} className="space-y-1">
                      <div className="flex items-center gap-1.5 px-1">
                        <Flag className="h-3 w-3 text-muted-foreground" />
                        <span className="text-xs font-medium text-muted-foreground truncate">
                          {milestone.name}
                        </span>
                      </div>
                      {msTasks.map(task => (
                        <DraggableTaskRow
                          key={task.id}
                          task={task}
                          goal={goal}
                          milestone={milestone}
                          onTaskClick={onTaskClick}
                        />
                      ))}
                    </div>
                  ))}
                </div>
              </ScrollArea>
            </div>
          ))}
        </div>
      </CardContent>
    </Card>
  )
}

function DraggableTaskRow({
  task,
  goal,
  milestone,
  onTaskClick,
}: {
  task: Task
  goal: Goal
  milestone: Milestone
  onTaskClick: (taskId: string) => void
}) {
  const daysUntil = getDaysUntil(task.dueDate)
  const urgency = getUrgencyLevel(daysUntil)

  // ドラッグデータ（TaskListWidgetと同じ形式）
  const dragData: TaskDragData = {
    type: 'task',
    taskId: task.id,
    taskName: task.name,
    milestoneId: milestone.id,
    milestoneName: milestone.name,
    goalId: goal.id,
    goalName: goal.name,
    goalColor: goal.color,
  }

  const { attributes, listeners, setNodeRef, isDragging, transform } = useDraggable({
    id: `weekly-draggable-task-${task.id}`,
    data: dragData,
  })

  const transformStyle = transform
    ? { transform: `translate3d(${transform.x}px, ${transform.y}px, 0)` }
    : {}

  return (
    <div
      ref={setNodeRef}
      style={{ ...transformStyle, borderLeftColor: goal.color, borderLeftWidth: 3 }}
      className={cn(
        'group flex items-center gap-1 py-1.5 px-2 rounded bg-background border cursor-grab active:cursor-grabbing transition-all',
        'hover:shadow-sm hover:border-primary/30',
        isDragging && 'opacity-50 shadow-lg z-50'
      )}
      {...listeners}
      {...attributes}
    >
      <button
        onClick={(e) => {
          e.stopPropagation()
          onTaskClick(task.id)
        }}
        className="flex-1 min-w-0 text-left text-xs"
      >
        <span className="truncate block hover:text-primary">{task.name}</span>
      </button>

      {daysUntil !== null && (
        <span className={cn(
          'text-[10px] flex-shrink-0',
          urgency === 'danger' && 'text-destructive',
          urgency === 'warning' && 'text-amber-500',
          urgency === 'normal' && 'text-muted-foreground',
        )}>
          {formatDaysUntil(daysUntil)}
        </span>
      )}

      {task.estimatedMinutes && (
        <span className="text-[10px] text-muted-foreground/50 flex-shrink-0 flex items-center gap-0.5">
          <Clock className="h-2.5 w-2.5" />
          {formatDurationShort(task.estimatedMinutes)}
        </span>
      )}

      <GripVertical className="h-3 w-3 text-muted-foreground/30 flex-shrink-0 opacity-0 group-hover:opacity-100 transition-opacity" />
    </div>
  )
}
