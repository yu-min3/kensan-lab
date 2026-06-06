import { useEffect } from 'react'
import { useTimerStore } from '@/stores/useTimerStore'

const DEFAULT_TITLE = 'Kensan'

// Format elapsed seconds to HH:MM:SS
function formatElapsedTime(seconds: number): string {
  const hours = Math.floor(seconds / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)
  const secs = seconds % 60

  return [hours, minutes, secs]
    .map(v => v.toString().padStart(2, '0'))
    .join(':')
}

/**
 * Updates document title based on timer state
 * Shows elapsed time when timer is running
 */
export function useDocumentTitle() {
  const { currentTimer, elapsedSeconds } = useTimerStore()

  useEffect(() => {
    if (currentTimer) {
      const timeStr = formatElapsedTime(elapsedSeconds)
      document.title = `⏱ ${timeStr} - ${currentTimer.taskName} | Kensan`
    } else {
      document.title = DEFAULT_TITLE
    }

    // Cleanup: reset title when component unmounts
    return () => {
      document.title = DEFAULT_TITLE
    }
  }, [currentTimer, elapsedSeconds])
}
