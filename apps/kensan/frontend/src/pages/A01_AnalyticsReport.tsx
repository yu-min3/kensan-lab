import { useState, useEffect, useMemo } from 'react'
import { Link } from 'react-router-dom'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Progress } from '@/components/ui/progress'
import { GoalBadge } from '@/components/common/GoalBadge'
import { formatDurationShort, formatMonthDay } from '@/lib/dateFormat'
import { useAnalyticsStore } from '@/stores/useAnalyticsStore'
import { useNoteStore } from '@/stores/useNoteStore'
import { WidgetError } from '@/components/common/WidgetError'
import {
  BarChart3,
  Clock,
  Target,
  TrendingUp,
  Loader2,
  BookOpen,
  ArrowRight,
} from 'lucide-react'
import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  Tooltip,
  ResponsiveContainer,
  PieChart,
  Pie,
  Cell,
  CartesianGrid,
  Legend,
} from 'recharts'
import type { ContentType } from 'recharts/types/component/Tooltip'
import { AIReviewSection } from '@/components/analytics/AIReviewSection'
import {
  AnalyticsPeriodSelector,
  formatLocalDate,
  formatDateRange,
  getDateRangeForPeriod,
} from '@/components/analytics/AnalyticsPeriodSelector'
import type { PeriodType } from '@/components/analytics/AnalyticsPeriodSelector'
import { cn } from '@/lib/utils'
import type { DateRange } from 'react-day-picker'
import { Calendar as CalendarIcon } from 'lucide-react'
import { PageGuide } from '@/components/guide/PageGuide'

// Custom tooltip for stacked bar chart
function createStackedBarTooltip(
  goalList: { id: string; name: string; color: string }[]
): ContentType<number, string> {
  return ({ active, payload, label }) => {
    if (!active || !payload?.length) return null

    const nonZeroItems = payload.filter((p) => p.value && (p.value as number) > 0)
    if (nonZeroItems.length === 0) return null

    const total = nonZeroItems.reduce((sum, p) => sum + ((p.value as number) ?? 0), 0)

    return (
      <div className="rounded-lg border bg-popover px-3 py-2 text-sm text-popover-foreground shadow-md">
        <p className="font-medium mb-1">
          {label}
          {payload[0]?.payload?.day && label !== payload[0].payload.day && (
            <span className="text-muted-foreground ml-1">({payload[0].payload.day})</span>
          )}
        </p>
        {nonZeroItems.map((item) => {
          const goal = goalList.find((g) => g.id === item.dataKey)
          return (
            <div key={item.dataKey} className="flex items-center gap-2 py-0.5">
              <div
                className="w-2.5 h-2.5 rounded-sm shrink-0"
                style={{ backgroundColor: item.color as string }}
              />
              <span className="text-muted-foreground">{goal?.name ?? item.dataKey}</span>
              <span className="ml-auto font-medium">{item.value}h</span>
            </div>
          )
        })}
        {nonZeroItems.length > 1 && (
          <div className="border-t mt-1 pt-1 flex justify-between text-muted-foreground">
            <span>合計</span>
            <span className="font-medium text-foreground">{Math.round(total * 10) / 10}h</span>
          </div>
        )}
      </div>
    )
  }
}

export function A01AnalyticsReport() {
  const [period, setPeriod] = useState<PeriodType>('week')
  const [customRange, setCustomRange] = useState<DateRange | undefined>()

  const { weeklySummary, dailyStudyHours, isLoading, error, fetchDashboardData } = useAnalyticsStore()
  const { getByType } = useNoteStore()
  const allLearningRecords = getByType('learning')

  const dateRange = useMemo(() => {
    return getDateRangeForPeriod(period, customRange)
  }, [period, customRange])

  // Filter learning records by selected date range
  const learningRecords = useMemo(() => {
    const { start, end } = dateRange
    const startStr = formatLocalDate(start)
    const endStr = formatLocalDate(end)
    return allLearningRecords.filter((record) => {
      const recordDate = formatLocalDate(new Date(record.createdAt))
      return recordDate >= startStr && recordDate <= endStr
    })
  }, [allLearningRecords, dateRange])

  useEffect(() => {
    const { start, end } = dateRange
    const startStr = formatLocalDate(start)
    const endStr = formatLocalDate(end)
    fetchDashboardData(startStr, endStr)
  }, [fetchDashboardData, dateRange])

  // Build goal list and stacked chart data from daily study hours
  // Note: dailyStudyHours is already range-filtered by the API, no client-side filter needed
  const { goalList, stackedChartData } = useMemo(() => {
    const goals = new Map<string, { id: string; name: string; color: string }>()
    const hasByGoal = dailyStudyHours.some((d) => d.byGoal && d.byGoal.length > 0)

    // Compute goal ratios from summary for proportional fallback
    const summaryGoalRatios: { id: string; name: string; color: string; ratio: number }[] = []
    if (!hasByGoal && weeklySummary) {
      const totalSummaryMinutes = weeklySummary.byGoal.reduce((sum, g) => sum + g.minutes, 0)
      for (const g of weeklySummary.byGoal) {
        summaryGoalRatios.push({
          id: g.id,
          name: g.name,
          color: g.color,
          ratio: totalSummaryMinutes > 0 ? g.minutes / totalSummaryMinutes : 0,
        })
      }
    }

    const data = dailyStudyHours.map((d) => {
      const entry: Record<string, string | number> = { date: d.date, day: d.day }
      if (hasByGoal && d.byGoal) {
        for (const g of d.byGoal) {
          if (!goals.has(g.id)) {
            goals.set(g.id, { id: g.id, name: g.name, color: g.color })
          }
          entry[g.id] = Math.round((g.minutes / 60) * 10) / 10
        }
      } else {
        // Fallback: distribute total hours proportionally by summary goal ratios
        for (const g of summaryGoalRatios) {
          if (!goals.has(g.id)) {
            goals.set(g.id, { id: g.id, name: g.name, color: g.color })
          }
          entry[g.id] = Math.round(d.hours * g.ratio * 10) / 10
        }
      }
      return entry
    })
    return { goalList: Array.from(goals.values()), stackedChartData: data }
  }, [dailyStudyHours, weeklySummary])

  const pieData = useMemo(() => {
    if (!weeklySummary) return []
    return weeklySummary.byGoal
      .map((goal) => ({
        name: goal.name,
        value: goal.minutes,
        color: goal.color,
      }))
      .filter((d) => d.value > 0)
  }, [weeklySummary])

  const totalHours = weeklySummary ? Math.floor(weeklySummary.totalMinutes / 60) : 0
  const totalMinutes = weeklySummary ? weeklySummary.totalMinutes % 60 : 0

  // Daily average from API-returned data (already range-filtered)
  const dailyAverage = useMemo(() => {
    if (!dailyStudyHours.length) return '0h'
    const total = dailyStudyHours.reduce((sum, d) => sum + d.hours, 0)
    const avg = Math.round((total / dailyStudyHours.length) * 10) / 10
    return `${avg}h`
  }, [dailyStudyHours])

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
      </div>
    )
  }

  if (error || !weeklySummary) {
    return (
      <div className="flex items-center justify-center h-64">
        <WidgetError
          message={error || 'データを取得できませんでした'}
          onRetry={() => {
            const { start, end } = dateRange
            fetchDashboardData(formatLocalDate(start), formatLocalDate(end))
          }}
        />
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <PageGuide pageId="analytics" />

      {/* Header */}
      <div className="flex items-center justify-between flex-wrap gap-4">
        <div className="flex items-center gap-3">
          <BarChart3 className="h-8 w-8 text-slate-500" />
          <h1 className="text-2xl font-bold">分析・レポート</h1>
        </div>

        <AnalyticsPeriodSelector
          period={period}
          onPeriodChange={setPeriod}
          customRange={customRange}
          onCustomRangeChange={setCustomRange}
        />
      </div>

      {/* Period indicator */}
      <div className="text-sm text-muted-foreground">
        {formatDateRange(dateRange.start, dateRange.end)} の集計
      </div>

      {/* Summary Cards */}
      <div data-guide="analytics-summary" className="grid gap-4 md:grid-cols-4">
        <Card>
          <CardContent className="pt-6">
            <div className="flex items-center gap-3">
              <Clock className="h-5 w-5 text-muted-foreground" />
              <div>
                <p className="text-sm text-muted-foreground">総学習時間</p>
                <p className="text-2xl font-bold">
                  {totalHours}h {totalMinutes}m
                </p>
              </div>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardContent className="pt-6">
            <div className="flex items-center gap-3">
              <Target className="h-5 w-5 text-muted-foreground" />
              <div>
                <p className="text-sm text-muted-foreground">完了タスク</p>
                <p className="text-2xl font-bold">{weeklySummary.completedTasks}</p>
              </div>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardContent className="pt-6">
            <div className="flex items-center gap-3">
              <TrendingUp className="h-5 w-5 text-muted-foreground" />
              <div>
                <p className="text-sm text-muted-foreground">計画達成率</p>
                <p className="text-2xl font-bold">
                  {weeklySummary.plannedVsActual.planned > 0
                    ? Math.round(
                        (weeklySummary.plannedVsActual.actual /
                          weeklySummary.plannedVsActual.planned) *
                          100
                      )
                    : 0}
                  %
                </p>
              </div>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardContent className="pt-6">
            <div className="flex items-center gap-3">
              <CalendarIcon className="h-5 w-5 text-muted-foreground" />
              <div>
                <p className="text-sm text-muted-foreground">日平均</p>
                <p className="text-2xl font-bold">{dailyAverage}</p>
              </div>
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Goal Distribution + Goal Progress */}
      <div className="grid gap-6 md:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Target className="h-5 w-5" />
              目標別時間配分
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="flex items-center gap-8">
              <ResponsiveContainer width="50%" height={200}>
                <PieChart>
                  <Pie
                    data={pieData}
                    cx="50%"
                    cy="50%"
                    innerRadius={50}
                    outerRadius={80}
                    dataKey="value"
                  >
                    {pieData.map((entry, index) => (
                      <Cell key={`cell-${index}`} fill={entry.color} />
                    ))}
                  </Pie>
                  <Tooltip
                    content={({ active, payload }) => {
                      if (!active || !payload?.length) return null
                      const item = payload[0]
                      return (
                        <div className="rounded-lg border bg-popover px-3 py-2 text-sm text-popover-foreground shadow-md">
                          <div className="flex items-center gap-2">
                            <div
                              className="w-2.5 h-2.5 rounded-sm"
                              style={{ backgroundColor: item.payload?.color }}
                            />
                            <span>{item.name}</span>
                            <span className="ml-2 font-medium">
                              {formatDurationShort(item.value as number)}
                            </span>
                          </div>
                        </div>
                      )
                    }}
                  />
                </PieChart>
              </ResponsiveContainer>
              <div className="space-y-3 flex-1">
                {pieData.map((item) => (
                  <div key={item.name} className="flex items-center justify-between">
                    <div className="flex items-center gap-2">
                      <div
                        className="w-3 h-3 rounded"
                        style={{ backgroundColor: item.color }}
                      />
                      <span className="text-sm">{item.name}</span>
                    </div>
                    <span className="text-sm font-medium">
                      {formatDurationShort(item.value)}
                    </span>
                  </div>
                ))}
              </div>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>目標達成度</CardTitle>
          </CardHeader>
          <CardContent className="space-y-6">
            {weeklySummary.byGoal.map((goal) => {
              const progressPercent = Math.round((goal.minutes / (20 * 60)) * 100)
              return (
                <div key={goal.id}>
                  <div className="flex items-center justify-between mb-2">
                    <div className="flex items-center gap-2">
                      <GoalBadge name={goal.name} color={goal.color} size="sm" />
                    </div>
                    <div className="flex items-center gap-2">
                      <span className="text-sm font-medium">{progressPercent}%</span>
                      <span className="text-sm text-muted-foreground">
                        ({Math.floor(goal.minutes / 60)}h)
                      </span>
                    </div>
                  </div>
                  <Progress value={progressPercent} className="h-3" />
                </div>
              )
            })}
          </CardContent>
        </Card>
      </div>

      {/* Daily Study Hours - Stacked Bar Chart */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <TrendingUp className="h-5 w-5" />
            日別学習時間（目標別）
          </CardTitle>
        </CardHeader>
        <CardContent>
          <ResponsiveContainer width="100%" height={300}>
            <BarChart data={stackedChartData}>
              <CartesianGrid strokeDasharray="3 3" />
              <XAxis
                dataKey={period === 'month' || period === 'custom' ? 'date' : 'day'}
                fontSize={12}
                interval={period === 'month' ? 2 : 0}
              />
              <YAxis domain={[0, 'auto']} allowDataOverflow fontSize={12} unit="h" />
              <Tooltip
                content={createStackedBarTooltip(goalList)}
                cursor={{ fill: 'hsl(var(--muted))', opacity: 0.5 }}
              />
              <Legend
                formatter={(value: string) => {
                  const goal = goalList.find((g) => g.id === value)
                  return goal?.name ?? value
                }}
              />
              {goalList.map((goal) => (
                <Bar
                  key={goal.id}
                  dataKey={goal.id}
                  stackId="goal"
                  fill={goal.color}
                  radius={[0, 0, 0, 0]}
                />
              ))}
            </BarChart>
          </ResponsiveContainer>
        </CardContent>
      </Card>

      {/* Learning Records Section */}
      <Card>
        <CardHeader className="flex flex-row items-center justify-between">
          <CardTitle className="flex items-center gap-2">
            <BookOpen className="h-5 w-5" />
            学習記録
          </CardTitle>
          <Link to="/notes?type=learning">
            <Button variant="ghost" size="sm" className="gap-1">
              すべて見る <ArrowRight className="h-4 w-4" />
            </Button>
          </Link>
        </CardHeader>
        <CardContent>
          <div className="grid gap-3 md:grid-cols-2 lg:grid-cols-3">
            {learningRecords.slice(0, 6).map((record) => (
              <Link
                key={record.id}
                to={`/notes/${record.id}`}
                className="block p-3 rounded-lg border bg-card hover:bg-muted/50 transition-colors"
              >
                <div className="flex items-center gap-2 mb-1">
                  <span
                    className={cn(
                      'text-xs px-1.5 py-0.5 rounded',
                      record.format === 'markdown'
                        ? 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400'
                        : 'bg-orange-100 text-orange-700 dark:bg-orange-900/30 dark:text-orange-400'
                    )}
                  >
                    {record.format === 'markdown' ? '.md' : '.dio'}
                  </span>
                  <span className="text-sm font-medium truncate">{record.title}</span>
                </div>
                <div className="flex items-center gap-2 text-xs text-muted-foreground">
                  {record.goalName && record.goalColor && (
                    <GoalBadge name={record.goalName} color={record.goalColor} size="sm" />
                  )}
                  {record.milestoneName && <span>{record.milestoneName}</span>}
                  <span>•</span>
                  <span>{formatMonthDay(record.createdAt)}</span>
                </div>
              </Link>
            ))}
          </div>
          {learningRecords.length === 0 && (
            <div className="text-center py-8 text-muted-foreground">
              <BookOpen className="h-12 w-12 mx-auto mb-3 opacity-50" />
              <p>この期間の学習記録はありません</p>
              <Link to="/notes/new?type=learning">
                <Button variant="link" className="mt-2">
                  記録を作成する
                </Button>
              </Link>
            </div>
          )}
        </CardContent>
      </Card>

      {/* AI Review Section */}
      <div data-guide="analytics-ai-review">
      <AIReviewSection
        startDate={formatLocalDate(dateRange.start)}
        endDate={formatLocalDate(dateRange.end)}
      />
      </div>
    </div>
  )
}
