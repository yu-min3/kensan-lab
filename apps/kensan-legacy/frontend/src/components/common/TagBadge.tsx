import { X } from 'lucide-react'
import { cn } from '@/lib/utils'

interface TagBadgeProps {
  name: string
  color: string
  size?: 'sm' | 'md'
  onRemove?: () => void
}

/**
 * Tag を表示するバッジコンポーネント
 * 色付き背景でTag名を表示する（集計用の自由タグ）
 */
export function TagBadge({ name, color, size = 'sm', onRemove }: TagBadgeProps) {
  return (
    <span
      className={cn(
        'inline-flex items-center gap-1 rounded-full font-medium',
        size === 'sm' ? 'px-2 py-0.5 text-xs' : 'px-2.5 py-1 text-sm'
      )}
      style={{
        backgroundColor: `color-mix(in srgb, ${color} 12%, transparent)`,
        color: color,
        border: `1px solid color-mix(in srgb, ${color} 25%, transparent)`,
      }}
    >
      <span
        className={cn('rounded-full flex-shrink-0', size === 'sm' ? 'w-1.5 h-1.5' : 'w-2 h-2')}
        style={{ backgroundColor: color }}
      />
      <span className="truncate max-w-[100px]">{name}</span>
      {onRemove && (
        <button
          type="button"
          onClick={(e) => {
            e.stopPropagation()
            onRemove()
          }}
          className="flex-shrink-0 hover:opacity-70 -mr-0.5"
        >
          <X className={cn(size === 'sm' ? 'h-3 w-3' : 'h-3.5 w-3.5')} />
        </button>
      )}
    </span>
  )
}
