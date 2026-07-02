import { Button } from '@/components/ui/button'
import { CalendarDays, ChevronLeft, ChevronRight } from 'lucide-react'
import { formatDateRange, formatDateIso } from '@/lib/dateFormat'

interface WeeklyHeaderProps {
  selectedWeekStart: Date
  onWeekChange: (monday: Date) => void
}

function getMonday(date: Date): Date {
  const d = new Date(date)
  const dayOfWeek = d.getDay()
  const mondayOffset = dayOfWeek === 0 ? -6 : 1 - dayOfWeek
  d.setDate(d.getDate() + mondayOffset)
  d.setHours(0, 0, 0, 0)
  return d
}

export function WeeklyHeader({ selectedWeekStart, onWeekChange }: WeeklyHeaderProps) {
  const weekEnd = new Date(selectedWeekStart)
  weekEnd.setDate(selectedWeekStart.getDate() + 6)

  const currentMonday = getMonday(new Date())
  const isCurrentWeek = formatDateIso(selectedWeekStart) === formatDateIso(currentMonday)

  const goToPreviousWeek = () => {
    const prev = new Date(selectedWeekStart)
    prev.setDate(prev.getDate() - 7)
    onWeekChange(prev)
  }

  const goToNextWeek = () => {
    const next = new Date(selectedWeekStart)
    next.setDate(next.getDate() + 7)
    onWeekChange(next)
  }

  const goToCurrentWeek = () => {
    onWeekChange(currentMonday)
  }

  return (
    <div className="flex items-center gap-3">
      <CalendarDays className="h-8 w-8 text-slate-500" />
      <h1 className="text-2xl font-bold">週間プラン</h1>
      <div className="flex items-center gap-1">
        <Button variant="ghost" size="icon" onClick={goToPreviousWeek}>
          <ChevronLeft className="h-4 w-4" />
        </Button>
        <span className="text-base font-semibold min-w-[160px] text-center bg-muted/50 px-3 py-1 rounded-md">
          {formatDateRange(selectedWeekStart, weekEnd)}
        </span>
        <Button variant="ghost" size="icon" onClick={goToNextWeek}>
          <ChevronRight className="h-4 w-4" />
        </Button>
      </div>
      {!isCurrentWeek && (
        <Button variant="outline" size="sm" onClick={goToCurrentWeek}>
          今週へ
        </Button>
      )}
    </div>
  )
}
