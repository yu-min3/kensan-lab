import { useState, useEffect } from 'react'
import { Play, Square } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { GoalBadge } from '@/components/common/GoalBadge'
import { StartTimerDialog } from '@/components/common/StartTimerDialog'
import { useTimerStore } from '@/stores/useTimerStore'
import { cn } from '@/lib/utils'

// Format elapsed seconds to HH:MM:SS
function formatElapsedTime(seconds: number): string {
  const hours = Math.floor(seconds / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)
  const secs = seconds % 60

  return [hours, minutes, secs]
    .map(v => v.toString().padStart(2, '0'))
    .join(':')
}

export function TimerWidget() {
  const {
    currentTimer,
    isLoading,
    elapsedSeconds,
    stopTimer,
    fetchCurrentTimer,
  } = useTimerStore()

  const [dialogOpen, setDialogOpen] = useState(false)

  // Fetch current timer on mount
  useEffect(() => {
    fetchCurrentTimer()
  }, [fetchCurrentTimer])

  const handleStopTimer = async () => {
    await stopTimer()
  }

  // Timer is running
  if (currentTimer) {
    return (
      <div className="flex items-center gap-2">
        <div
          className={cn(
            'flex items-center gap-2 px-3 py-1.5 rounded-md',
            'bg-primary/10 border border-primary/20',
            'animate-pulse'
          )}
        >
          {currentTimer.goalName && currentTimer.goalColor && (
            <GoalBadge
              name={currentTimer.goalName}
              color={currentTimer.goalColor}
              size="sm"
            />
          )}
          <span className="text-sm font-medium max-w-32 truncate">
            {currentTimer.taskName}
          </span>
          <span className="text-sm font-mono font-semibold text-primary">
            {formatElapsedTime(elapsedSeconds)}
          </span>
        </div>
        <Button
          variant="destructive"
          size="sm"
          onClick={handleStopTimer}
          disabled={isLoading}
        >
          <Square className="h-4 w-4" />
          <span className="sr-only">Stop</span>
        </Button>
      </div>
    )
  }

  // No timer running
  return (
    <>
      <Button
        size="sm"
        onClick={() => setDialogOpen(true)}
        disabled={isLoading}
        className="gap-2"
      >
        <Play className="h-4 w-4" />
        <span className="hidden sm:inline">タイマー開始</span>
      </Button>
      <StartTimerDialog open={dialogOpen} onOpenChange={setDialogOpen} />
    </>
  )
}
