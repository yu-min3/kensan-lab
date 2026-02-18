import { useState } from 'react'
import { Button } from '@/components/ui/button'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Calendar } from '@/components/ui/calendar'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { Calendar as CalendarIcon, ArrowRight } from 'lucide-react'
import { cn } from '@/lib/utils'
import type { DateRange } from 'react-day-picker'

export type PeriodType = 'today' | 'week' | 'month' | 'custom'

// Format Date as yyyy-MM-dd in local timezone (avoids UTC shift from toISOString)
export function formatLocalDate(date: Date): string {
  const y = date.getFullYear()
  const m = String(date.getMonth() + 1).padStart(2, '0')
  const d = String(date.getDate()).padStart(2, '0')
  return `${y}-${m}-${d}`
}

// Helper to get date range for each period
export function getDateRangeForPeriod(period: PeriodType, customRange?: DateRange): { start: Date; end: Date } {
  const today = new Date()
  today.setHours(0, 0, 0, 0)

  switch (period) {
    case 'today':
      return { start: today, end: today }
    case 'week': {
      const dayOfWeek = today.getDay()
      const mondayOffset = dayOfWeek === 0 ? -6 : 1 - dayOfWeek
      const monday = new Date(today)
      monday.setDate(today.getDate() + mondayOffset)
      const sunday = new Date(monday)
      sunday.setDate(monday.getDate() + 6)
      return { start: monday, end: sunday }
    }
    case 'month': {
      const firstDay = new Date(today.getFullYear(), today.getMonth(), 1)
      const lastDay = new Date(today.getFullYear(), today.getMonth() + 1, 0)
      return { start: firstDay, end: lastDay }
    }
    case 'custom':
      if (customRange?.from && customRange?.to) {
        return { start: customRange.from, end: customRange.to }
      }
      return { start: today, end: today }
  }
}

// Format date range for display
export function formatDateRange(start: Date, end: Date): string {
  const startStr = `${start.getMonth() + 1}/${start.getDate()}`
  const endStr = `${end.getMonth() + 1}/${end.getDate()}`
  if (startStr === endStr) return startStr
  return `${startStr} - ${endStr}`
}

function formatDisplayDate(date: Date): string {
  return `${date.getMonth() + 1}/${date.getDate()}`
}

// Preset period definitions
const presets = [
  { label: '先週', getDates: () => {
    const today = new Date(); today.setHours(0, 0, 0, 0)
    const dayOfWeek = today.getDay()
    const mondayOffset = dayOfWeek === 0 ? -6 : 1 - dayOfWeek
    const thisMonday = new Date(today); thisMonday.setDate(today.getDate() + mondayOffset)
    const lastMonday = new Date(thisMonday); lastMonday.setDate(thisMonday.getDate() - 7)
    const lastSunday = new Date(lastMonday); lastSunday.setDate(lastMonday.getDate() + 6)
    return { from: lastMonday, to: lastSunday }
  }},
  { label: '先月', getDates: () => {
    const today = new Date(); today.setHours(0, 0, 0, 0)
    const firstDay = new Date(today.getFullYear(), today.getMonth() - 1, 1)
    const lastDay = new Date(today.getFullYear(), today.getMonth(), 0)
    return { from: firstDay, to: lastDay }
  }},
  { label: '過去7日', getDates: () => {
    const today = new Date(); today.setHours(0, 0, 0, 0)
    const start = new Date(today); start.setDate(today.getDate() - 6)
    return { from: start, to: today }
  }},
  { label: '過去30日', getDates: () => {
    const today = new Date(); today.setHours(0, 0, 0, 0)
    const start = new Date(today); start.setDate(today.getDate() - 29)
    return { from: start, to: today }
  }},
] as const

type ActiveField = 'from' | 'to'

interface AnalyticsPeriodSelectorProps {
  period: PeriodType
  onPeriodChange: (period: PeriodType) => void
  customRange: DateRange | undefined
  onCustomRangeChange: (range: DateRange | undefined) => void
}

export function AnalyticsPeriodSelector({
  period,
  onPeriodChange,
  customRange,
  onCustomRangeChange,
}: AnalyticsPeriodSelectorProps) {
  const [calendarOpen, setCalendarOpen] = useState(false)
  const [activeField, setActiveField] = useState<ActiveField>('from')

  const handlePeriodChange = (value: string) => {
    const newPeriod = value as PeriodType
    onPeriodChange(newPeriod)
    if (newPeriod === 'custom') {
      setActiveField('from')
      setCalendarOpen(true)
    }
  }

  const handlePresetSelect = (preset: typeof presets[number]) => {
    const dates = preset.getDates()
    onCustomRangeChange(dates)
    setCalendarOpen(false)
  }

  const handleDaySelect = (day: Date | undefined) => {
    if (!day) return

    if (activeField === 'from') {
      // If selecting a start date after the current end date, reset end
      const newTo = customRange?.to && day > customRange.to ? undefined : customRange?.to
      onCustomRangeChange({ from: day, to: newTo })
      setActiveField('to')
    } else {
      // If selecting an end date before the current start date, swap
      if (customRange?.from && day < customRange.from) {
        onCustomRangeChange({ from: day, to: customRange.from })
      } else {
        onCustomRangeChange({ from: customRange?.from, to: day })
      }
      // Both selected → close
      if (customRange?.from) {
        setCalendarOpen(false)
      }
    }
  }

  // For highlighting the range on the calendar visually
  const calendarMonth = activeField === 'to' && customRange?.from
    ? customRange.from
    : new Date()

  return (
    <div className="flex items-center gap-2">
      <Tabs value={period} onValueChange={handlePeriodChange}>
        <TabsList>
          <TabsTrigger value="today">今日</TabsTrigger>
          <TabsTrigger value="week">今週</TabsTrigger>
          <TabsTrigger value="month">今月</TabsTrigger>
          <TabsTrigger value="custom">カスタム</TabsTrigger>
        </TabsList>
      </Tabs>

      {period === 'custom' && (
        <Popover open={calendarOpen} onOpenChange={setCalendarOpen}>
          <PopoverTrigger asChild>
            <Button variant="outline" size="sm" className="gap-2">
              <CalendarIcon className="h-4 w-4" />
              {customRange?.from && customRange?.to
                ? formatDateRange(customRange.from, customRange.to)
                : '期間を選択'}
            </Button>
          </PopoverTrigger>
          <PopoverContent className="w-auto p-0" align="end">
            <div className="p-4 space-y-3">
              {/* Presets */}
              <div className="flex gap-2">
                {presets.map((preset) => (
                  <Button
                    key={preset.label}
                    variant="outline"
                    size="sm"
                    className="text-xs h-7"
                    onClick={() => handlePresetSelect(preset)}
                  >
                    {preset.label}
                  </Button>
                ))}
              </div>

              <div className="border-t" />

              {/* Date fields */}
              <div className="flex items-center gap-2">
                <button
                  type="button"
                  onClick={() => setActiveField('from')}
                  className={cn(
                    'flex-1 rounded-lg border-2 px-3 py-2.5 text-sm transition-all text-center',
                    activeField === 'from'
                      ? 'border-primary bg-primary/5 shadow-sm'
                      : 'border-transparent bg-muted/60 hover:bg-muted'
                  )}
                >
                  <span className={cn(
                    'block text-[10px] font-semibold uppercase tracking-widest mb-1',
                    activeField === 'from' ? 'text-primary' : 'text-muted-foreground'
                  )}>
                    開始日
                  </span>
                  <span className={cn(
                    'block text-base font-bold tabular-nums',
                    customRange?.from ? 'text-foreground' : 'text-muted-foreground/50'
                  )}>
                    {customRange?.from ? formatDisplayDate(customRange.from) : '--/--'}
                  </span>
                </button>

                <ArrowRight className={cn(
                  'h-4 w-4 shrink-0 transition-colors',
                  customRange?.from ? 'text-primary' : 'text-muted-foreground/40'
                )} />

                <button
                  type="button"
                  onClick={() => setActiveField('to')}
                  className={cn(
                    'flex-1 rounded-lg border-2 px-3 py-2.5 text-sm transition-all text-center',
                    activeField === 'to'
                      ? 'border-primary bg-primary/5 shadow-sm'
                      : 'border-transparent bg-muted/60 hover:bg-muted'
                  )}
                >
                  <span className={cn(
                    'block text-[10px] font-semibold uppercase tracking-widest mb-1',
                    activeField === 'to' ? 'text-primary' : 'text-muted-foreground'
                  )}>
                    終了日
                  </span>
                  <span className={cn(
                    'block text-base font-bold tabular-nums',
                    customRange?.to ? 'text-foreground' : 'text-muted-foreground/50'
                  )}>
                    {customRange?.to ? formatDisplayDate(customRange.to) : '--/--'}
                  </span>
                </button>
              </div>
            </div>

            {/* Single-select calendar — no confusing range double-click */}
            <Calendar
              mode="single"
              selected={activeField === 'from' ? customRange?.from : customRange?.to}
              onSelect={handleDaySelect}
              numberOfMonths={2}
              defaultMonth={calendarMonth}
              modifiers={{
                range_start: customRange?.from ? [customRange.from] : [],
                range_end: customRange?.to ? [customRange.to] : [],
                range_middle: customRange?.from && customRange?.to
                  ? [{ after: customRange.from, before: customRange.to }]
                  : [],
              }}
              modifiersClassNames={{
                range_start: 'bg-primary text-primary-foreground rounded-l-md',
                range_end: 'bg-primary text-primary-foreground rounded-r-md',
                range_middle: 'bg-accent text-accent-foreground rounded-none',
              }}
            />
          </PopoverContent>
        </Popover>
      )}
    </div>
  )
}
