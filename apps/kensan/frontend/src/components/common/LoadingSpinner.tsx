import { Loader2 } from 'lucide-react'

interface LoadingSpinnerProps {
  message?: string
}

export function LoadingSpinner({ message = '読み込み中...' }: LoadingSpinnerProps) {
  return (
    <div className="flex flex-col items-center justify-center py-8 text-muted-foreground">
      <Loader2 className="h-8 w-8 animate-spin mb-2" />
      <span className="text-sm">{message}</span>
    </div>
  )
}
