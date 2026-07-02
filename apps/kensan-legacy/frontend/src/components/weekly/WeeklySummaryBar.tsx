import { useMemo } from 'react'
import { Card, CardContent } from '@/components/ui/card'
import { formatDurationShort } from '@/lib/dateFormat'
import { calculateMinutesFromDatetimes } from '@/lib/taskUtils'
import type { TimeBlock, TimeEntry } from '@/types'

interface WeeklySummaryBarProps {
  blocksByDate: Record<string, TimeBlock[]>
  entriesByDate: Record<string, TimeEntry[]>
  weekDates: string[]
}

interface GoalTime {
  goalName: string
  goalColor: string
  minutes: number
}

export function WeeklySummaryBar({ blocksByDate, entriesByDate, weekDates }: WeeklySummaryBarProps) {
  const { totalPlanned, totalActual, goalTimes } = useMemo(() => {
    let planned = 0
    let actual = 0
    const goalMap = new Map<string, GoalTime>()

    for (const dateStr of weekDates) {
      const blocks = blocksByDate[dateStr] || []
      const entries = entriesByDate[dateStr] || []

      // goalIdがあるブロック/実績のみを集計対象とする
      const blocksWithGoal = blocks.filter(b => b.goalId)
      const entriesWithGoal = entries.filter(e => e.goalId)

      planned += calculateMinutesFromDatetimes(blocksWithGoal)
      actual += calculateMinutesFromDatetimes(entriesWithGoal)

      for (const block of blocksWithGoal) {
        const goalId = block.goalId!
        const existing = goalMap.get(goalId)
        const blockMinutes = (new Date(block.endDatetime).getTime() - new Date(block.startDatetime).getTime()) / 60000
        if (existing) {
          existing.minutes += blockMinutes
        } else {
          goalMap.set(goalId, {
            goalName: block.goalName || '',
            goalColor: block.goalColor || '#94a3b8',
            minutes: blockMinutes,
          })
        }
      }
    }

    return {
      totalPlanned: planned,
      totalActual: actual,
      goalTimes: Array.from(goalMap.values()).sort((a, b) => b.minutes - a.minutes),
    }
  }, [blocksByDate, entriesByDate, weekDates])

  const totalGoalMinutes = goalTimes.reduce((sum, g) => sum + g.minutes, 0)

  return (
    <Card>
      <CardContent className="p-4">
        <div className="flex items-center justify-between mb-3">
          <div className="text-sm font-medium">週間サマリー</div>
          <div className="text-sm text-muted-foreground">
            <span className="font-medium text-emerald-600 dark:text-emerald-400">{formatDurationShort(totalActual)}</span>
            {' / '}
            <span className="font-medium text-foreground">{formatDurationShort(totalPlanned)}</span>
            {totalPlanned > 0 && (
              <span className={`ml-1.5 font-medium ${
                totalActual / totalPlanned >= 1
                  ? 'text-emerald-600 dark:text-emerald-400'
                  : totalActual / totalPlanned >= 0.7
                    ? 'text-foreground'
                    : 'text-muted-foreground'
              }`}>
                ({Math.round((totalActual / totalPlanned) * 100)}%)
              </span>
            )}
          </div>
        </div>

        {/* Goal color bar */}
        {totalGoalMinutes > 0 && (
          <div className="space-y-2">
            <div className="flex h-3 rounded-full overflow-hidden bg-muted">
              {goalTimes.map((goal) => (
                <div
                  key={goal.goalName}
                  className="h-full transition-all"
                  style={{
                    backgroundColor: goal.goalColor,
                    width: `${(goal.minutes / totalGoalMinutes) * 100}%`,
                  }}
                  title={`${goal.goalName}: ${formatDurationShort(goal.minutes)}`}
                />
              ))}
            </div>
            <div className="flex flex-wrap gap-x-4 gap-y-1">
              {goalTimes.map((goal) => (
                <div key={goal.goalName} className="flex items-center gap-1.5 text-xs">
                  <div
                    className="w-2.5 h-2.5 rounded-full"
                    style={{ backgroundColor: goal.goalColor }}
                  />
                  <span className="text-muted-foreground">{goal.goalName}</span>
                  <span className="font-medium">{formatDurationShort(goal.minutes)}</span>
                </div>
              ))}
            </div>
          </div>
        )}

        {totalGoalMinutes === 0 && (
          <div className="text-xs text-muted-foreground/50 text-center py-2">
            この週にはまだタイムブロックがありません
          </div>
        )}
      </CardContent>
    </Card>
  )
}
