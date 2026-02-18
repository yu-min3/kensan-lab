import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { ConfirmPopover } from '@/components/common/ConfirmPopover'
import { EntityMemoPopover } from '@/components/common/EntityMemoPopover'
import { TagBadge } from '@/components/common/TagBadge'
import { useSortable } from '@dnd-kit/sortable'
import { CSS } from '@dnd-kit/utilities'
import { GripVertical, Plus, Edit, Trash2, ChevronRight, ChevronDown, Calendar } from 'lucide-react'
import { cn } from '@/lib/utils'
import type { Task, Tag } from '@/types'

export interface SortableTaskItemProps {
  task: Task
  isSelected: boolean
  isSelectionMode: boolean
  onSelect: (taskId: string, checked: boolean) => void
  onToggleComplete: (taskId: string) => void
  onEdit: (task: Task) => void
  onDelete: (taskId: string) => void
  onAddSubtask: (milestoneId?: string, parentId?: string) => void
  getTagsByIds: (ids: string[]) => Tag[]
  hasChildren: boolean
  isExpanded: boolean
  onToggleExpand: (taskId: string) => void
  recentlyCompleted: boolean
  children?: React.ReactNode
}

export function SortableTaskItem({
  task,
  isSelected,
  isSelectionMode,
  onSelect,
  onToggleComplete,
  onEdit,
  onDelete,
  onAddSubtask,
  getTagsByIds,
  hasChildren,
  isExpanded,
  onToggleExpand,
  recentlyCompleted,
  children,
}: SortableTaskItemProps) {
  const {
    attributes,
    listeners,
    setNodeRef,
    transform,
    transition,
    isDragging,
  } = useSortable({ id: task.id })

  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
  }

  return (
    <div ref={setNodeRef} style={style}>
      <div
        className={cn(
          'flex items-center gap-2 p-2 rounded-lg transition-colors group',
          'hover:bg-muted/50',
          recentlyCompleted && 'bg-green-100 dark:bg-green-900/30',
          isDragging && 'opacity-50 bg-muted',
          isSelected && 'bg-primary/10 border border-primary/30'
        )}
      >
        {/* Drag handle */}
        <button
          {...attributes}
          {...listeners}
          className="cursor-grab active:cursor-grabbing p-1 hover:bg-muted rounded opacity-50 hover:opacity-100 touch-none"
        >
          <GripVertical className="h-3 w-3" />
        </button>

        {/* Selection checkbox (shown in selection mode) or expand button */}
        {isSelectionMode ? (
          <Checkbox
            checked={isSelected}
            onCheckedChange={(checked) => onSelect(task.id, checked === true)}
          />
        ) : hasChildren ? (
          <button onClick={() => onToggleExpand(task.id)} className="p-1 hover:bg-muted rounded">
            {isExpanded ? <ChevronDown className="h-3 w-3" /> : <ChevronRight className="h-3 w-3" />}
          </button>
        ) : (
          <div className="w-6" />
        )}

        {/* Complete checkbox */}
        <Checkbox checked={task.completed} onCheckedChange={() => onToggleComplete(task.id)} />

        {/* Task name */}
        <span
          className={cn(
            'flex-1 text-sm cursor-pointer hover:text-primary',
            task.completed && 'line-through text-muted-foreground'
          )}
          onClick={() => isSelectionMode ? onSelect(task.id, !isSelected) : onEdit(task)}
        >
          {task.name}
        </span>
        <EntityMemoPopover entityType="task" entityId={task.id} />

        {/* Due date */}
        {task.dueDate && (() => {
          const today = new Date().toISOString().slice(0, 10)
          const isOverdue = !task.completed && task.dueDate < today
          const isToday = task.dueDate === today
          const [, m, d] = task.dueDate.split('-')
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

        {/* Tags */}
        {task.tagIds && task.tagIds.length > 0 && (
          <div className="flex gap-1 flex-shrink-0">
            {getTagsByIds(task.tagIds).map(tag => (
              <TagBadge key={tag.id} name={tag.name} color={tag.color} size="sm" />
            ))}
          </div>
        )}

        {/* Action buttons */}
        <div className="opacity-0 group-hover:opacity-100 flex gap-1 flex-shrink-0">
          <Button
            variant="ghost"
            size="sm"
            className="h-6 w-6 p-0"
            onClick={() => onAddSubtask(task.milestoneId, task.id)}
            title="サブタスク追加"
          >
            <Plus className="h-3 w-3" />
          </Button>
          <Button
            variant="ghost"
            size="sm"
            className="h-6 w-6 p-0"
            onClick={() => onEdit(task)}
          >
            <Edit className="h-3 w-3" />
          </Button>
          <ConfirmPopover
            message="このタスクを削除しますか？"
            confirmLabel="削除"
            onConfirm={() => onDelete(task.id)}
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
      {children}
    </div>
  )
}
