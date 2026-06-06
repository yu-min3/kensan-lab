import { useMemo, useEffect } from 'react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Progress } from '@/components/ui/progress'
import { cn } from '@/lib/utils'
import {
  getTaskFrequencyLabel,
  getPlannedCountThisWeek,
  getWeekRange,
  calculateRate,
} from '@/lib/taskUtils'
import type { Goal, Milestone, Task } from '@/types'
import { useTimeBlockStore } from '@/stores/useTimeBlockStore'
import { RefreshCw, CheckCircle2, Circle } from 'lucide-react'

interface RecurringTaskWidgetProps {
  goals: Goal[]
  milestones: Milestone[]
  tasks: Task[]
  className?: string
}

interface RecurringTaskStats {
  task: Task
  goal?: Goal
  milestone?: Milestone
  plannedThisWeek: number
  actualThisWeek: number
  rate: number
  frequencyLabel: string
}


export function RecurringTaskWidget({ goals, milestones, tasks, className }: RecurringTaskWidgetProps) {
  const { timeEntries, fetchTimeEntriesRange } = useTimeBlockStore()

  const weekRange = useMemo(() => getWeekRange(), [])

  // 今週のTimeEntryを取得
  useEffect(() => {
    fetchTimeEntriesRange(weekRange.startStr, weekRange.endStr)
  }, [weekRange.startStr, weekRange.endStr, fetchTimeEntriesRange])

  // 定期タスクの統計を計算
  const recurringTaskStats = useMemo(() => {
    const recurringTasks = tasks.filter(t => t.frequency && !t.completed)

    return recurringTasks.map(task => {
      const milestone = task.milestoneId
        ? milestones.find(m => m.id === task.milestoneId)
        : undefined
      const goal = milestone
        ? goals.find(g => g.id === milestone.goalId)
        : undefined

      const plannedThisWeek = getPlannedCountThisWeek(task.frequency, task.daysOfWeek)

      // TimeEntryでtaskIdが一致するものをカウント（今週分）
      const actualThisWeek = timeEntries.filter(entry => {
        if (entry.taskId !== task.id) return false
        const entryDate = new Date(entry.startDatetime)
        return entryDate >= weekRange.start && entryDate <= weekRange.end
      }).length

      const rate = calculateRate(actualThisWeek, plannedThisWeek)

      return {
        task,
        goal,
        milestone,
        plannedThisWeek,
        actualThisWeek,
        rate,
        frequencyLabel: getTaskFrequencyLabel(task) ?? '',
      } as RecurringTaskStats
    }).sort((a, b) => {
      // 達成率が低い順（要注意を上に）
      return a.rate - b.rate
    })
  }, [tasks, milestones, goals, timeEntries, weekRange])

  // 全体の達成率
  const overallStats = useMemo(() => {
    const totalPlanned = recurringTaskStats.reduce((sum, s) => sum + s.plannedThisWeek, 0)
    const totalActual = recurringTaskStats.reduce((sum, s) => sum + s.actualThisWeek, 0)
    const rate = calculateRate(totalActual, totalPlanned)
    return { totalPlanned, totalActual, rate }
  }, [recurringTaskStats])

  return (
    <Card className={className}>
      <CardHeader className="pb-2">
        <CardTitle className="text-sm font-medium flex items-center gap-2">
          <RefreshCw className="h-4 w-4" />
          今週の定期タスク
          <span className="text-xs text-muted-foreground ml-auto">
            {weekRange.startStr.slice(5).replace('-', '/')} - {weekRange.endStr.slice(5).replace('-', '/')}
          </span>
        </CardTitle>
      </CardHeader>
      <CardContent className="pt-0">
        {recurringTaskStats.length === 0 ? (
          <div className="text-center py-6 text-muted-foreground text-sm">
            <RefreshCw className="h-8 w-8 mx-auto mb-2 opacity-50" />
            定期タスクがありません
          </div>
        ) : (
          <div className="space-y-3">
            {/* 全体の達成率 */}
            <div className="flex items-center gap-2 p-2 bg-muted/30 rounded-lg">
              <span className="text-sm font-medium">全体</span>
              <Progress value={overallStats.rate} className="flex-1 h-2" />
              <span className="text-sm tabular-nums w-20 text-right">
                {overallStats.totalActual}/{overallStats.totalPlanned} ({overallStats.rate}%)
              </span>
            </div>

            {/* 個別タスク */}
            <div className="space-y-2">
              {recurringTaskStats.map(stats => (
                <div key={stats.task.id} className="flex items-center gap-2">
                  {/* 達成率に応じたアイコン */}
                  {stats.rate >= 100 ? (
                    <CheckCircle2 className="h-4 w-4 text-green-500 flex-shrink-0" />
                  ) : (
                    <Circle className={cn(
                      "h-4 w-4 flex-shrink-0",
                      stats.rate >= 50 ? "text-amber-500" : "text-muted-foreground"
                    )} />
                  )}

                  {/* タスク名 */}
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-1.5">
                      {stats.goal && (
                        <div
                          className="w-2 h-2 rounded-full flex-shrink-0"
                          style={{ backgroundColor: stats.goal.color }}
                        />
                      )}
                      <span className="text-sm truncate">{stats.task.name}</span>
                      <span className="text-[10px] text-muted-foreground flex-shrink-0">
                        ({stats.frequencyLabel})
                      </span>
                    </div>
                  </div>

                  {/* 進捗 */}
                  <div className="flex items-center gap-2 flex-shrink-0">
                    <Progress
                      value={stats.rate}
                      className={cn(
                        "w-16 h-1.5",
                        stats.rate >= 100 && "[&>div]:bg-green-500"
                      )}
                    />
                    <span className={cn(
                      "text-xs tabular-nums w-12 text-right",
                      stats.rate >= 100
                        ? "text-green-600"
                        : stats.rate >= 50
                          ? "text-amber-600"
                          : "text-muted-foreground"
                    )}>
                      {stats.actualThisWeek}/{stats.plannedThisWeek}
                    </span>
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}
      </CardContent>
    </Card>
  )
}
