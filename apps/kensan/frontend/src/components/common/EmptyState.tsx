import type { LucideIcon } from 'lucide-react'
import { Button } from '@/components/ui/button'

interface EmptyStateProps {
  icon: LucideIcon
  message: string
  actionLabel?: string
  onAction?: () => void
}

export function EmptyState({ icon: Icon, message, actionLabel, onAction }: EmptyStateProps) {
  return (
    <div className="text-center py-8 text-muted-foreground text-sm">
      <Icon className="h-8 w-8 mx-auto mb-2 opacity-50" />
      {message}
      {actionLabel && onAction && (
        <Button variant="link" size="sm" className="block mx-auto mt-2" onClick={onAction}>
          {actionLabel}
        </Button>
      )}
    </div>
  )
}
