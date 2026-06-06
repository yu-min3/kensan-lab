import { Badge } from '@/components/ui/badge'
import { cn } from '@/lib/utils'

interface GoalBadgeProps {
  name: string
  color: string
  size?: 'sm' | 'default'
}

/**
 * Goal を表示するバッジコンポーネント
 * 色付き背景でGoal名を表示する
 */
export function GoalBadge({ name, color, size = 'default' }: GoalBadgeProps) {
  return (
    <Badge
      className={cn(
        'text-white border-none',
        size === 'sm' && 'text-[10px] px-1.5 py-0'
      )}
      style={{ backgroundColor: color }}
    >
      {name}
    </Badge>
  )
}
