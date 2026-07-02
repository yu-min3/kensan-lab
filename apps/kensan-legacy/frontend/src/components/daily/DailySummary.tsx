import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Progress } from '@/components/ui/progress'
import { useTimeBlockStore } from '@/stores/useTimeBlockStore'
import { useTimerStore } from '@/stores/useTimerStore'
import { useSettingsStore } from '@/stores/useSettingsStore'
import { getLocalDate } from '@/lib/timezone'
import { formatDurationShort, formatDateIso } from '@/lib/dateFormat'
import { calculateMinutesFromDatetimes, calculateRate } from '@/lib/taskUtils'
import { TrendingUp } from 'lucide-react'
import {
  PieChart,
  Pie,
  Cell,
  ResponsiveContainer,
  Tooltip,
} from 'recharts'

interface DailySummaryProps {
  mode: 'compact' | 'detailed'
  selectedDate?: string
}

export function DailySummary({ mode, selectedDate }: DailySummaryProps) {
  const { timeBlocks, timeEntries } = useTimeBlockStore()
  const { currentTimer, elapsedSeconds } = useTimerStore()
  const timezone = useSettingsStore((s) => s.timezone) || 'Asia/Tokyo'

  const targetDateIso = selectedDate || formatDateIso(new Date())

  // Filter data for the target date, excluding items without goals
  const todayBlocks = timeBlocks.filter((b) => getLocalDate(b.startDatetime, timezone) === targetDateIso && b.goalId)
  const todayEntries = timeEntries.filter((e) => getLocalDate(e.startDatetime, timezone) === targetDateIso && e.goalId)

  // 進行中タイマーが今日＆目標ありなら経過時間を加算
  const runningMinutes =
    currentTimer?.goalId && currentTimer.startedAt && getLocalDate(currentTimer.startedAt, timezone) === targetDateIso
      ? elapsedSeconds / 60
      : 0

  // 目標ありのみを達成率計算の対象とする
  const plannedMinutes = calculateMinutesFromDatetimes(todayBlocks)
  const actualMinutes = calculateMinutesFromDatetimes(todayEntries) + runningMinutes
  const difference = actualMinutes - plannedMinutes
  const completionRate = calculateRate(actualMinutes, plannedMinutes)

  // Goal-based time distribution (for detailed mode)
  const timeByGoalMap = todayEntries.reduce(
    (acc, entry) => {
      const goalId = entry.goalId!
      const goalName = entry.goalName || 'Unknown'
      const goalColor = entry.goalColor || '#6b7280'
      const startMs = new Date(entry.startDatetime).getTime()
      const endMs = new Date(entry.endDatetime).getTime()
      const minutes = (endMs - startMs) / 60000
      if (!acc[goalId]) {
        acc[goalId] = { name: goalName, color: goalColor, value: 0 }
      }
      acc[goalId].value += minutes
      return acc
    },
    {} as Record<string, { name: string; color: string; value: number }>
  )

  // 進行中タイマーの時間を円グラフにも反映
  if (runningMinutes > 0 && currentTimer?.goalId) {
    const goalId = currentTimer.goalId
    if (timeByGoalMap[goalId]) {
      timeByGoalMap[goalId].value += runningMinutes
    } else {
      timeByGoalMap[goalId] = {
        name: currentTimer.goalName || 'Unknown',
        color: currentTimer.goalColor || '#6b7280',
        value: runningMinutes,
      }
    }
  }

  const pieData = Object.values(timeByGoalMap).filter((d) => d.value > 0)

  if (mode === 'compact') {
    return (
      <div className="flex items-center gap-3 px-3 py-2 rounded-lg bg-muted/50">
        <div className="flex items-center gap-2">
          <TrendingUp className="h-4 w-4 text-muted-foreground" />
          <span className="text-sm text-muted-foreground">達成率</span>
          <span className="text-lg font-bold">{completionRate}%</span>
        </div>
        <div className="w-24">
          <Progress value={Math.min(completionRate, 100)} className="h-1.5" />
        </div>
        <span className="text-xs text-muted-foreground">
          {formatDurationShort(actualMinutes)} / {formatDurationShort(plannedMinutes)}
        </span>
      </div>
    )
  }

  // Detailed mode
  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <TrendingUp className="h-5 w-5" />
          {targetDateIso === formatDateIso(new Date()) ? '今日' : targetDateIso}のサマリー
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-6">
        {/* Completion rate */}
        <div className="flex items-center gap-6">
          <div className="text-center">
            <p className="text-4xl font-bold">{completionRate}%</p>
            <p className="text-sm text-muted-foreground">達成率</p>
          </div>
          <div className="flex-1">
            <Progress value={Math.min(completionRate, 100)} className="h-3" />
            <p className="text-sm text-muted-foreground mt-2">
              {formatDurationShort(actualMinutes)} / {formatDurationShort(plannedMinutes)}
            </p>
          </div>
        </div>

        {/* Time summary */}
        <div className="grid grid-cols-3 gap-4 pt-4 border-t">
          <div className="text-center">
            <p className="text-sm text-muted-foreground">計画</p>
            <p className="text-xl font-semibold">{formatDurationShort(plannedMinutes)}</p>
          </div>
          <div className="text-center">
            <p className="text-sm text-muted-foreground">実績</p>
            <p className="text-xl font-semibold text-brand">{formatDurationShort(actualMinutes)}</p>
          </div>
          <div className="text-center">
            <p className="text-sm text-muted-foreground">差分</p>
            <p
              className={`text-xl font-semibold ${difference >= 0 ? 'text-brand' : 'text-destructive'}`}
            >
              {difference >= 0 ? '+' : ''}
              {formatDurationShort(difference)}
            </p>
          </div>
        </div>

        {/* Goal-based time distribution */}
        {pieData.length > 0 && (
          <div className="pt-4 border-t">
            <p className="text-sm font-medium mb-3">目標別の時間配分</p>
            <div className="flex items-center gap-6">
              <ResponsiveContainer width={120} height={120}>
                <PieChart>
                  <Pie
                    data={pieData}
                    cx="50%"
                    cy="50%"
                    innerRadius={30}
                    outerRadius={50}
                    dataKey="value"
                  >
                    {pieData.map((entry, index) => (
                      <Cell key={`cell-${index}`} fill={entry.color} />
                    ))}
                  </Pie>
                  <Tooltip formatter={(value) => [formatDurationShort(value as number), '']} />
                </PieChart>
              </ResponsiveContainer>
              <div className="flex-1 space-y-2">
                {pieData.map((item) => (
                  <div key={item.name} className="flex items-center justify-between">
                    <div className="flex items-center gap-2">
                      <div
                        className="w-3 h-3 rounded"
                        style={{ backgroundColor: item.color }}
                      />
                      <span className="text-sm">{item.name}</span>
                    </div>
                    <span className="text-sm text-muted-foreground">
                      {formatDurationShort(item.value)}
                    </span>
                  </div>
                ))}
              </div>
            </div>
          </div>
        )}
      </CardContent>
    </Card>
  )
}
