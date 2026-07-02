import { Button } from '@/components/ui/button'
import { AlertCircle, RefreshCw } from 'lucide-react'

interface WidgetErrorProps {
  message?: string
  onRetry?: () => void
  compact?: boolean
}

export function WidgetError({ message, onRetry, compact = false }: WidgetErrorProps) {
  if (compact) {
    return (
      <div className="flex items-center gap-2 text-sm text-muted-foreground p-2">
        <AlertCircle className="h-4 w-4 text-destructive shrink-0" />
        <span className="truncate">{message || 'データの取得に失敗しました'}</span>
        {onRetry && (
          <Button variant="ghost" size="sm" className="h-6 px-2 ml-auto" onClick={onRetry}>
            <RefreshCw className="h-3 w-3" />
          </Button>
        )}
      </div>
    )
  }

  return (
    <div className="flex flex-col items-center justify-center p-6 border border-destructive/30 bg-destructive/5 rounded-lg">
      <AlertCircle className="h-8 w-8 text-destructive mb-3" />
      <p className="text-sm text-muted-foreground text-center mb-4">
        {message || 'データの取得に失敗しました'}
      </p>
      {onRetry && (
        <Button variant="outline" size="sm" onClick={onRetry} className="gap-2">
          <RefreshCw className="h-4 w-4" />
          再試行
        </Button>
      )}
    </div>
  )
}
