import { useState, useMemo, useEffect } from 'react'
import { Check, X, Clock, ListTodo } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/ui/tabs'
import { cn } from '@/lib/utils'
import type { ActionItem } from '@/stores/useChatStore'
import { formatActionDescription } from '@/lib/actionFormatter'
import { timeblocksApi } from '@/api/services/timeblocks'
import { useSettingsStore } from '@/stores/useSettingsStore'
import { getLocalDate } from '@/lib/timezone'
import { ProposalTimeline } from './ProposalTimeline'
import type { TimeBlock } from '@/types'

interface ActionProposalProps {
  actions: ActionItem[]
  onApprove?: (actionIds: string[]) => void
  onReject?: () => void
  disabled?: boolean
  readOnly?: boolean
}

interface TimeBlockAction {
  action: ActionItem
  date: string
  startTime: string
  endTime: string
  taskName: string
  goalColor?: string
  goalName?: string
}

function formatDate(dateStr: string): string {
  try {
    const d = new Date(dateStr + 'T00:00:00')
    if (isNaN(d.getTime())) return dateStr
    const days = ['日', '月', '火', '水', '木', '金', '土']
    return `${d.getMonth() + 1}/${d.getDate()}（${days[d.getDay()]}）`
  } catch {
    return dateStr
  }
}

/** Get the Mon-Sun week range that contains the given date. */
function getWeekRange(date: Date): { start: Date; end: Date } {
  const day = date.getDay()
  const daysToMon = day === 0 ? 6 : day - 1
  const start = new Date(date)
  start.setDate(start.getDate() - daysToMon)
  const end = new Date(start)
  end.setDate(end.getDate() + 6)
  return { start, end }
}

/** Expand proposal dates to a full Mon-Sun week range, capped to current week if proposals are within it. */
function expandToFullWeek(proposalDates: string[]): string[] {
  if (proposalDates.length === 0) return []

  // Current week boundaries (Mon-Sun containing today)
  const today = new Date()
  today.setHours(0, 0, 0, 0)
  const currentWeek = getWeekRange(today)

  const first = new Date(proposalDates[0] + 'T00:00:00')
  const last = new Date(proposalDates[proposalDates.length - 1] + 'T00:00:00')

  // If most proposals fall within the current week, clamp to current week
  const inCurrentWeek = proposalDates.filter((d) => {
    const dt = new Date(d + 'T00:00:00')
    return dt >= currentWeek.start && dt <= currentWeek.end
  }).length
  const useCurrentWeek = inCurrentWeek >= proposalDates.length * 0.5

  let start: Date
  let end: Date
  if (useCurrentWeek) {
    start = currentWeek.start
    end = currentWeek.end
  } else {
    // Fall back to covering all proposals
    const firstWeek = getWeekRange(first)
    const lastWeek = getWeekRange(last)
    start = firstWeek.start
    end = lastWeek.end
  }

  const result: string[] = []
  const cursor = new Date(start)
  while (cursor <= end) {
    const yyyy = cursor.getFullYear()
    const mm = String(cursor.getMonth() + 1).padStart(2, '0')
    const dd = String(cursor.getDate()).padStart(2, '0')
    result.push(`${yyyy}-${mm}-${dd}`)
    cursor.setDate(cursor.getDate() + 1)
  }
  return result
}

export function ActionProposal({ actions, onApprove, onReject, disabled, readOnly }: ActionProposalProps) {
  const [selected, setSelected] = useState<Set<string>>(new Set(actions.map((a) => a.id)))
  const timezone = useSettingsStore((s) => s.timezone)
  const [existingBlocks, setExistingBlocks] = useState<TimeBlock[]>([])

  const toggleAction = (id: string) => {
    setSelected((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  // Separate time block actions from others
  const { timeBlocks, otherActions, dateGroups, sortedDates } = useMemo(() => {
    const tbs: TimeBlockAction[] = []
    const others: ActionItem[] = []

    for (const action of actions) {
      if (action.type === 'create_time_block') {
        const input = action.input
        const date = (input.date as string) || ''
        const startTime = (input.start_time as string) || (input.start_timestamp as string) || ''
        const endTime = (input.end_time as string) || (input.end_timestamp as string) || ''
        const taskName = (input.task_name as string) || (input.name as string) || ''
        const goalColor = (input.goal_color as string) || undefined
        const goalName = (input.goal_name as string) || undefined

        if (date && startTime && endTime) {
          tbs.push({ action, date, startTime, endTime, taskName, goalColor, goalName })
          continue
        }
      }
      others.push(action)
    }

    // Group by date, sorted
    const groups = new Map<string, TimeBlockAction[]>()
    tbs.sort((a, b) => a.date.localeCompare(b.date) || a.startTime.localeCompare(b.startTime))
    for (const tb of tbs) {
      const existing = groups.get(tb.date)
      if (existing) existing.push(tb)
      else groups.set(tb.date, [tb])
    }

    // Expand to full Mon-Sun week range covering all proposed dates
    const proposalDates = Array.from(groups.keys())
    const dates = expandToFullWeek(proposalDates)

    return { timeBlocks: tbs, otherActions: others, dateGroups: groups, sortedDates: dates }
  }, [actions])

  // Fetch existing time blocks for the full date range (including expanded weekdays)
  useEffect(() => {
    if (sortedDates.length === 0 || !timezone) return

    timeblocksApi
      .listByDateRange(sortedDates[0], sortedDates[sortedDates.length - 1], timezone)
      .then(setExistingBlocks)
      .catch(() => {
        // Silent fail: existing blocks are contextual, not required
      })
  }, [sortedDates, timezone])

  // Group existing blocks by local date
  const existingByDate = useMemo(() => {
    const map = new Map<string, TimeBlock[]>()
    if (!timezone) return map
    for (const tb of existingBlocks) {
      const date = getLocalDate(tb.startDatetime, timezone)
      const existing = map.get(date)
      if (existing) existing.push(tb)
      else map.set(date, [tb])
    }
    return map
  }, [existingBlocks, timezone])

  const hasTimeBlocks = timeBlocks.length > 0
  const [activeDate, setActiveDate] = useState<string>('')

  // Set initial active date when data is ready
  useEffect(() => {
    if (sortedDates.length > 0 && !sortedDates.includes(activeDate)) {
      setActiveDate(sortedDates[0])
    }
  }, [sortedDates, activeDate])

  return (
    <div className="mx-4 my-2 rounded-lg border bg-card p-3">
      {/* Timeline view for time blocks */}
      {hasTimeBlocks && (
        <>
          <div className="flex items-center gap-1.5 mb-2">
            <Clock className="h-3.5 w-3.5 text-muted-foreground" />
            <p className="text-xs font-medium text-muted-foreground">スケジュール案</p>
          </div>

          <Tabs value={activeDate} onValueChange={setActiveDate}>
            <TabsList className="h-auto mb-2 overflow-x-auto gap-0.5 p-1">
              {sortedDates.map((date) => {
                const proposalCount = dateGroups.get(date)?.length ?? 0
                const hasProposals = proposalCount > 0
                const today = getLocalDate(new Date().toISOString(), timezone)
                const isToday = date === today
                return (
                  <TabsTrigger
                    key={date}
                    value={date}
                    className={cn(
                      'text-xs px-2 py-1 h-auto flex flex-col items-center gap-0.5 relative',
                      !hasProposals && 'opacity-40',
                      isToday && 'font-bold',
                    )}
                  >
                    <span>{formatDate(date)}</span>
                    {hasProposals && (
                      <span className="flex items-center gap-0.5">
                        <span className="h-1.5 w-1.5 rounded-full bg-brand" />
                        <span className="text-[9px] text-brand font-medium">{proposalCount}</span>
                      </span>
                    )}
                    {isToday && !hasProposals && (
                      <span className="h-1 w-1 rounded-full bg-muted-foreground/50" />
                    )}
                  </TabsTrigger>
                )
              })}
            </TabsList>

            {sortedDates.map((date) => (
              <TabsContent key={date} value={date} className="mt-0">
                <ProposalTimeline
                  blocks={dateGroups.get(date) || []}
                  existingBlocks={existingByDate.get(date) || []}
                  selected={readOnly ? new Set(actions.map(a => a.id)) : selected}
                  onToggle={readOnly ? () => {} : toggleAction}
                  disabled={disabled || readOnly}
                  timezone={timezone}
                />
              </TabsContent>
            ))}
          </Tabs>
        </>
      )}

      {/* Other actions (create_task, update_task, etc.) */}
      {otherActions.length > 0 && (
        <>
          {hasTimeBlocks && (
            <div className="flex items-center gap-1.5 mb-2 mt-3">
              <ListTodo className="h-3.5 w-3.5 text-muted-foreground" />
              <p className="text-xs font-medium text-muted-foreground">その他のアクション</p>
            </div>
          )}
          {!hasTimeBlocks && (
            <p className="text-xs font-medium text-muted-foreground mb-2">提案されたアクション</p>
          )}
          <div className="space-y-1.5">
            {otherActions.map((action) => (
              readOnly ? (
                <div key={action.id} className="text-sm px-1.5 py-1">
                  {formatActionDescription(action.type, action.input)}
                </div>
              ) : (
                <label
                  key={action.id}
                  className="flex items-center gap-2 text-sm cursor-pointer hover:bg-muted/50 rounded px-1.5 py-1"
                >
                  <input
                    type="checkbox"
                    checked={selected.has(action.id)}
                    onChange={() => toggleAction(action.id)}
                    disabled={disabled}
                    className="rounded border-input"
                  />
                  <span className={cn(!selected.has(action.id) && 'opacity-40 line-through')}>
                    {formatActionDescription(action.type, action.input)}
                  </span>
                </label>
              )
            ))}
          </div>
        </>
      )}

      {/* Approve / Reject buttons */}
      {!readOnly && (
        <div className="flex gap-2 mt-3">
          <Button
            size="sm"
            onClick={() => onApprove?.(Array.from(selected))}
            disabled={disabled || selected.size === 0}
            className="flex-1"
          >
            <Check className="h-3.5 w-3.5 mr-1" />
            承認 ({selected.size})
          </Button>
          <Button size="sm" variant="outline" onClick={onReject} disabled={disabled}>
            <X className="h-3.5 w-3.5 mr-1" />
            却下
          </Button>
        </div>
      )}
    </div>
  )
}
