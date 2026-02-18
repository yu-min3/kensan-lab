import { Checkbox } from '@/components/ui/checkbox'
import { GoalBadge } from './GoalBadge'
import type { Task } from '@/types'
import { cn } from '@/lib/utils'

interface TaskCardProps {
  task: Task
  milestoneName?: string
  goalName?: string
  goalColor?: string
  estimatedMinutes?: number
  onToggle?: (id: string) => void
  onClick?: () => void
  className?: string
  draggable?: boolean
}

export function TaskCard({
  task,
  milestoneName,
  goalName,
  goalColor,
  estimatedMinutes,
  onToggle,
  onClick,
  className,
  draggable = false,
}: TaskCardProps) {
  return (
    <div
      className={cn(
        'flex items-center gap-3 p-3 rounded-lg border bg-card hover:bg-accent/50 transition-colors',
        draggable && 'cursor-grab active:cursor-grabbing',
        className
      )}
      onClick={onClick}
      draggable={draggable}
    >
      <Checkbox
        checked={task.completed}
        onCheckedChange={() => onToggle?.(task.id)}
        onClick={(e) => e.stopPropagation()}
      />

      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2">
          <span
            className={cn(
              'text-sm font-medium truncate',
              task.completed && 'line-through text-muted-foreground'
            )}
          >
            {task.name}
          </span>
          {goalName && goalColor && (
            <GoalBadge name={goalName} color={goalColor} size="sm" />
          )}
        </div>
        {milestoneName && (
          <p className="text-xs text-muted-foreground truncate">
            {milestoneName}
          </p>
        )}
      </div>

      {estimatedMinutes && (
        <span className="text-xs text-muted-foreground whitespace-nowrap">
          {estimatedMinutes >= 60
            ? `${Math.floor(estimatedMinutes / 60)}h${estimatedMinutes % 60 > 0 ? ` ${estimatedMinutes % 60}m` : ''}`
            : `${estimatedMinutes}m`}
        </span>
      )}
    </div>
  )
}
