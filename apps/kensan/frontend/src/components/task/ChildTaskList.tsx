import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { ConfirmPopover } from '@/components/common/ConfirmPopover'
import { EntityMemoPopover } from '@/components/common/EntityMemoPopover'
import { Edit, Trash2, Calendar } from 'lucide-react'
import { cn } from '@/lib/utils'
import type { Task } from '@/types'

interface ChildTaskListProps {
  childTasks: Task[]
  isSelectionMode: boolean
  selectedTaskIds: Set<string>
  recentlyCompleted: Set<string>
  onSelect: (taskId: string, checked: boolean) => void
  onToggleComplete: (taskId: string) => void
  onEdit: (task: Task) => void
  onDelete: (taskId: string) => void
}

export function ChildTaskList({
  childTasks,
  isSelectionMode,
  selectedTaskIds,
  recentlyCompleted,
  onSelect,
  onToggleComplete,
  onEdit,
  onDelete,
}: ChildTaskListProps) {
  return (
    <div className="ml-8 border-l pl-2 space-y-1">
      {childTasks.map(childTask => (
        <div
          key={childTask.id}
          className={cn(
            'flex items-center gap-2 p-2 rounded-lg transition-colors group',
            'hover:bg-muted/50',
            recentlyCompleted.has(childTask.id) && 'bg-green-100 dark:bg-green-900/30',
            selectedTaskIds.has(childTask.id) && 'bg-primary/10 border border-primary/30'
          )}
        >
          {isSelectionMode ? (
            <Checkbox
              checked={selectedTaskIds.has(childTask.id)}
              onCheckedChange={(checked) => onSelect(childTask.id, checked === true)}
            />
          ) : (
            <div className="w-6" />
          )}
          <Checkbox
            checked={childTask.completed}
            onCheckedChange={() => onToggleComplete(childTask.id)}
          />
          <span
            className={cn(
              'flex-1 text-sm cursor-pointer hover:text-primary',
              childTask.completed && 'line-through text-muted-foreground'
            )}
            onClick={() => onEdit(childTask)}
          >
            {childTask.name}
          </span>
          <EntityMemoPopover entityType="task" entityId={childTask.id} />
          {childTask.dueDate && (() => {
            const today = new Date().toISOString().slice(0, 10)
            const isOverdue = !childTask.completed && childTask.dueDate < today
            const isToday = childTask.dueDate === today
            const [, m, d] = childTask.dueDate.split('-')
            return (
              <span className={cn(
                'flex items-center gap-0.5 text-xs flex-shrink-0',
                isOverdue ? 'text-red-500 font-medium' : isToday ? 'text-amber-500' : 'text-muted-foreground'
              )}>
                <Calendar className="h-3 w-3" />
                {`${Number(m)}/${Number(d)}`}
              </span>
            )
          })()}
          <div className="opacity-0 group-hover:opacity-100 flex gap-1">
            <Button
              variant="ghost"
              size="sm"
              className="h-6 w-6 p-0"
              onClick={() => onEdit(childTask)}
            >
              <Edit className="h-3 w-3" />
            </Button>
            <ConfirmPopover
              message="このタスクを削除しますか？"
              confirmLabel="削除"
              onConfirm={() => onDelete(childTask.id)}
              variant="destructive"
            >
              <Button
                variant="ghost"
                size="sm"
                className="h-6 w-6 p-0"
              >
                <Trash2 className="h-3 w-3" />
              </Button>
            </ConfirmPopover>
          </div>
        </div>
      ))}
    </div>
  )
}
