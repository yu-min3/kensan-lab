/**
 * DateTimeRangePicker - Hybrid datetime range selector
 * Combines hour/minute selects with duration presets for intuitive time entry.
 */
import { useMemo } from 'react'
import { Button } from '@/components/ui/button'
import { Calendar } from '@/components/ui/calendar'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { CalendarIcon } from 'lucide-react'
import { cn } from '@/lib/utils'
import { format, parse } from 'date-fns'
import { ja } from 'date-fns/locale'

interface DateTimeRangePickerProps {
  /** Start datetime in YYYY-MM-DDTHH:mm format (local) */
  startDatetime: string
  /** End datetime in YYYY-MM-DDTHH:mm format (local) */
  endDatetime: string
  /** Called when start datetime changes */
  onStartChange: (value: string) => void
  /** Called when end datetime changes */
  onEndChange: (value: string) => void
  /** Show duration preset buttons (default: true) */
  showDurationPresets?: boolean
}

const HOURS = Array.from({ length: 24 }, (_, i) => i)
const MINUTES = [0, 5, 10, 15, 20, 25, 30, 35, 40, 45, 50, 55]

const DURATION_PRESETS = [
  { label: '30分', value: 30 },
  { label: '1h', value: 60 },
  { label: '1.5h', value: 90 },
  { label: '2h', value: 120 },
  { label: '3h', value: 180 },
]

function parseDatetime(datetime: string) {
  if (!datetime) {
    const now = new Date()
    return {
      date: now,
      hour: now.getHours(),
      minute: Math.floor(now.getMinutes() / 5) * 5,
    }
  }
  const dt = parse(datetime, "yyyy-MM-dd'T'HH:mm", new Date())
  return {
    date: dt,
    hour: dt.getHours(),
    minute: dt.getMinutes(),
  }
}

function buildDatetime(date: Date, hour: number, minute: number): string {
  const d = new Date(date)
  d.setHours(hour, minute, 0, 0)
  return format(d, "yyyy-MM-dd'T'HH:mm")
}

export function DateTimeRangePicker({
  startDatetime,
  endDatetime,
  onStartChange,
  onEndChange,
  showDurationPresets = true,
}: DateTimeRangePickerProps) {
  const start = useMemo(() => parseDatetime(startDatetime), [startDatetime])
  const end = useMemo(() => parseDatetime(endDatetime), [endDatetime])

  // Calculate current duration in minutes
  const currentDuration = useMemo(() => {
    const startTotal = start.hour * 60 + start.minute
    const endTotal = end.hour * 60 + end.minute
    return endTotal - startTotal
  }, [start, end])

  // Update start datetime
  const updateStart = (date: Date, hour: number, minute: number) => {
    onStartChange(buildDatetime(date, hour, minute))
  }

  // Update end datetime
  const updateEnd = (date: Date, hour: number, minute: number) => {
    onEndChange(buildDatetime(date, hour, minute))
  }

  // Handle start time change - auto-adjust end to maintain duration
  const handleStartHourChange = (value: string) => {
    const newHour = parseInt(value)
    const duration = Math.max(currentDuration, 30) // min 30 min
    const startTotal = newHour * 60 + start.minute
    const endTotal = Math.min(startTotal + duration, 23 * 60 + 55)
    const endHour = Math.floor(endTotal / 60)
    const endMinute = endTotal % 60

    updateStart(start.date, newHour, start.minute)
    updateEnd(end.date, endHour, endMinute)
  }

  const handleStartMinuteChange = (value: string) => {
    const newMinute = parseInt(value)
    const duration = Math.max(currentDuration, 30)
    const startTotal = start.hour * 60 + newMinute
    const endTotal = Math.min(startTotal + duration, 23 * 60 + 55)
    const endHour = Math.floor(endTotal / 60)
    const endMinute = endTotal % 60

    updateStart(start.date, start.hour, newMinute)
    updateEnd(end.date, endHour, endMinute)
  }

  const handleDateChange = (date: Date | undefined) => {
    if (!date) return
    updateStart(date, start.hour, start.minute)
    updateEnd(date, end.hour, end.minute)
  }

  // Apply duration preset
  const applyDuration = (durationMinutes: number) => {
    const startTotal = start.hour * 60 + start.minute
    const endTotal = Math.min(startTotal + durationMinutes, 23 * 60 + 55)
    const endHour = Math.floor(endTotal / 60)
    const endMinute = endTotal % 60
    updateEnd(end.date, endHour, endMinute)
  }

  const formatDuration = (mins: number) => {
    if (mins < 60) return `${mins}分`
    const h = Math.floor(mins / 60)
    const m = mins % 60
    return m > 0 ? `${h}時間${m}分` : `${h}時間`
  }

  const matchedDuration = DURATION_PRESETS.find((d) => d.value === currentDuration)

  return (
    <div className="space-y-3">
      {/* Row 1: Date + Start time ~ End time */}
      <div className="flex gap-2 items-center flex-wrap">
        {/* Date picker */}
        <Popover>
          <PopoverTrigger asChild>
            <Button
              variant="outline"
              className={cn(
                'w-[120px] justify-start text-left font-normal h-9',
                !start.date && 'text-muted-foreground'
              )}
            >
              <CalendarIcon className="mr-2 h-4 w-4" />
              {format(start.date, 'MM/dd (E)', { locale: ja })}
            </Button>
          </PopoverTrigger>
          <PopoverContent className="w-auto p-0" align="start">
            <Calendar
              mode="single"
              selected={start.date}
              onSelect={handleDateChange}
              initialFocus
            />
          </PopoverContent>
        </Popover>

        {/* Start time */}
        <div className="flex items-center gap-0.5">
          <Select
            value={start.hour.toString()}
            onValueChange={handleStartHourChange}
            scrollToSelected
          >
            <SelectTrigger className="w-[52px] h-9 px-2">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {HOURS.map((h) => (
                <SelectItem key={h} value={h.toString()}>
                  {h.toString().padStart(2, '0')}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <span className="text-muted-foreground">:</span>
          <Select
            value={start.minute.toString()}
            onValueChange={handleStartMinuteChange}
          >
            <SelectTrigger className="w-[52px] h-9 px-2">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {MINUTES.map((m) => (
                <SelectItem key={m} value={m.toString()}>
                  {m.toString().padStart(2, '0')}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>

        <span className="text-muted-foreground">〜</span>

        {/* End time */}
        <div className="flex items-center gap-0.5">
          <Select
            value={end.hour.toString()}
            onValueChange={(v) => updateEnd(end.date, parseInt(v), end.minute)}
            scrollToSelected
          >
            <SelectTrigger className="w-[52px] h-9 px-2">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {HOURS.map((h) => (
                <SelectItem key={h} value={h.toString()}>
                  {h.toString().padStart(2, '0')}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <span className="text-muted-foreground">:</span>
          <Select
            value={end.minute.toString()}
            onValueChange={(v) => updateEnd(end.date, end.hour, parseInt(v))}
          >
            <SelectTrigger className="w-[52px] h-9 px-2">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {MINUTES.map((m) => (
                <SelectItem key={m} value={m.toString()}>
                  {m.toString().padStart(2, '0')}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      </div>

      {/* Row 2: Duration presets + current duration */}
      {showDurationPresets && (
        <div className="flex items-center gap-2">
          <div className="flex gap-1">
            {DURATION_PRESETS.map((d) => (
              <Button
                key={d.value}
                type="button"
                variant={currentDuration === d.value ? 'default' : 'outline'}
                size="sm"
                className="h-7 px-2 text-xs"
                onClick={() => applyDuration(d.value)}
              >
                {d.label}
              </Button>
            ))}
          </div>
          {!matchedDuration && currentDuration > 0 && (
            <span className="text-xs text-muted-foreground">
              ({formatDuration(currentDuration)})
            </span>
          )}
          {currentDuration <= 0 && (
            <span className="text-xs text-destructive">
              終了は開始より後に
            </span>
          )}
        </div>
      )}
    </div>
  )
}
