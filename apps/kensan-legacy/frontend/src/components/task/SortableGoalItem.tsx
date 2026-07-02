import { Button } from '@/components/ui/button'
import { Progress } from '@/components/ui/progress'
import { ConfirmPopover } from '@/components/common/ConfirmPopover'
import { useSortable } from '@dnd-kit/sortable'
import { CSS } from '@dnd-kit/utilities'
import { GripVertical, Edit, Trash2, CheckCircle2 } from 'lucide-react'
import { cn } from '@/lib/utils'
import type { Goal } from '@/types'

export interface SortableGoalItemProps {
  goal: Goal
  isSelected: boolean
  progress: { completed: number; total: number; percentage: number }
  milestoneCount: number
  onSelect: (id: string) => void
  onEdit: (goal: Goal) => void
  onDelete: (id: string) => void
  onComplete: (id: string) => void
}

export function SortableGoalItem({
  goal,
  isSelected,
  progress,
  milestoneCount,
  onSelect,
  onEdit,
  onDelete,
  onComplete,
}: SortableGoalItemProps) {
  const {
    attributes,
    listeners,
    setNodeRef,
    transform,
    transition,
    isDragging,
  } = useSortable({ id: goal.id })

  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
  }

  const isCompleted = goal.status === 'completed'

  return (
    <div ref={setNodeRef} style={style}>
      <div
        className={cn(
          'p-3 rounded-lg cursor-pointer transition-colors group',
          isSelected ? 'bg-primary/10 border border-primary/30' : 'hover:bg-muted/50',
          isDragging && 'opacity-50 bg-muted',
          isCompleted && 'opacity-60'
        )}
        onClick={() => onSelect(goal.id)}
      >
        <div className="flex items-center gap-2">
          <div
            {...attributes}
            {...listeners}
            className="cursor-grab active:cursor-grabbing opacity-0 group-hover:opacity-100 transition-opacity"
            onClick={e => e.stopPropagation()}
          >
            <GripVertical className="h-4 w-4 text-muted-foreground" />
          </div>
          {isCompleted ? (
            <CheckCircle2 className="h-4 w-4 text-green-500 flex-shrink-0" />
          ) : (
            <div className="w-3 h-3 rounded-full" style={{ backgroundColor: goal.color }} />
          )}
          <span className={cn(
            'font-medium text-sm flex-1',
            isSelected && 'text-primary',
            isCompleted && 'line-through'
          )}>
            {goal.name}
          </span>
          <div className="opacity-0 group-hover:opacity-100 flex gap-1">
            {!isCompleted && (
              <ConfirmPopover
                message="この目標を完了しますか？"
                confirmLabel="完了"
                onConfirm={() => onComplete(goal.id)}
              >
                <Button
                  variant="ghost"
                  size="sm"
                  className="h-6 w-6 p-0 text-green-600"
                  onClick={e => e.stopPropagation()}
                  title="完了"
                >
                  <CheckCircle2 className="h-3 w-3" />
                </Button>
              </ConfirmPopover>
            )}
            <Button
              variant="ghost"
              size="sm"
              className="h-6 w-6 p-0"
              onClick={e => {
                e.stopPropagation()
                onEdit(goal)
              }}
            >
              <Edit className="h-3 w-3" />
            </Button>
            <ConfirmPopover
              message="この目標と配下のマイルストーン・タスクを削除しますか？"
              confirmLabel="削除"
              onConfirm={() => onDelete(goal.id)}
              variant="destructive"
            >
              <Button
                variant="ghost"
                size="sm"
                className="h-6 w-6 p-0"
                onClick={e => e.stopPropagation()}
              >
                <Trash2 className="h-3 w-3" />
              </Button>
            </ConfirmPopover>
          </div>
        </div>
        <div className="mt-2 flex items-center gap-2">
          <Progress value={progress.percentage} className="h-1.5 flex-1" />
          <span className="text-xs text-muted-foreground w-16 text-right">
            {progress.completed}/{progress.total}
          </span>
        </div>
        <div className="mt-1 text-xs text-muted-foreground">{milestoneCount} マイルストーン</div>
      </div>
    </div>
  )
}
