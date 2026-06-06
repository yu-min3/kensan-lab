// Analytics MSW handlers
import { http, HttpResponse } from 'msw'
import { weeklySummary, dailyStudyHours, goals } from '../data'
import { format, eachDayOfInterval, parseISO } from 'date-fns'

const BASE_URL = 'http://localhost:8088/api/v1'

const dayNames = ['日', '月', '火', '水', '木', '金', '土']

// Generate daily study hours for a given date range
function generateDailyHoursForRange(startDate: string, endDate: string) {
  const start = parseISO(startDate)
  const end = parseISO(endDate)
  const days = eachDayOfInterval({ start, end })

  return days.map((day) => {
    const dateStr = format(day, 'yyyy-MM-dd')
    // Check if we have existing mock data for this date
    const existing = dailyStudyHours.find((d) => d.date === dateStr)
    if (existing) return existing

    // Generate plausible data for dates outside the default range
    const baseHours = 3 + Math.floor(Math.random() * 4)
    const gkMinutes = Math.floor(baseHours * 60 * 0.5)
    const ossMinutes = Math.floor(baseHours * 60 * 0.3)
    const outputMinutes = baseHours * 60 - gkMinutes - ossMinutes
    return {
      date: dateStr,
      hours: baseHours,
      day: dayNames[day.getDay()],
      byGoal: [
        { id: 'goal-cert', name: 'GCPスキルアップ', color: '#0EA5E9', minutes: gkMinutes },
        { id: 'goal-product', name: '個人開発プロダクト', color: '#10B981', minutes: ossMinutes },
        { id: 'goal-output', name: '技術アウトプット', color: '#F59E0B', minutes: outputMinutes },
      ],
    }
  })
}

// Generate summary for a given date range
function generateSummaryForRange(startDate: string, endDate: string) {
  const dailyData = generateDailyHoursForRange(startDate, endDate)
  const totalMinutes = dailyData.reduce((sum, d) => sum + d.hours * 60, 0)

  // Aggregate byGoal
  const goalMap = new Map<string, { id: string; name: string; color: string; minutes: number }>()
  for (const day of dailyData) {
    if (day.byGoal) {
      for (const g of day.byGoal) {
        const existing = goalMap.get(g.id)
        if (existing) {
          existing.minutes += g.minutes
        } else {
          goalMap.set(g.id, { ...g })
        }
      }
    }
  }

  return {
    weekStart: startDate,
    weekEnd: endDate,
    totalMinutes,
    byGoal: Array.from(goalMap.values()),
    byTag: weeklySummary.byTag,
    byMilestone: weeklySummary.byMilestone,
    completedTasks: Math.max(1, dailyData.length * 2),
    plannedVsActual: {
      planned: Math.round(totalMinutes * 1.1),
      actual: totalMinutes,
    },
  }
}

export const analyticsHandlers = [
  // GET /analytics/summary/weekly
  http.get(`${BASE_URL}/analytics/summary/weekly`, () => {
    return HttpResponse.json(weeklySummary)
  }),

  // GET /analytics/summary - range-based summary
  http.get(`${BASE_URL}/analytics/summary`, ({ request }) => {
    const url = new URL(request.url)
    const startDate = url.searchParams.get('start_date')
    const endDate = url.searchParams.get('end_date')

    if (startDate && endDate) {
      return HttpResponse.json(generateSummaryForRange(startDate, endDate))
    }
    return HttpResponse.json(weeklySummary)
  }),

  // GET /analytics/daily-study-hours
  http.get(`${BASE_URL}/analytics/daily-study-hours`, ({ request }) => {
    const url = new URL(request.url)
    const startDate = url.searchParams.get('start_date')
    const endDate = url.searchParams.get('end_date')

    if (startDate && endDate) {
      return HttpResponse.json(generateDailyHoursForRange(startDate, endDate))
    }

    const days = parseInt(url.searchParams.get('days') || '7', 10)
    const data = dailyStudyHours.slice(-days)
    return HttpResponse.json(data)
  }),

  // GET /analytics/summary/monthly
  http.get(`${BASE_URL}/analytics/summary/monthly`, ({ request }) => {
    const url = new URL(request.url)
    const year = url.searchParams.get('year')
    const month = url.searchParams.get('month')

    return HttpResponse.json({
      year: year ? parseInt(year) : new Date().getFullYear(),
      month: month ? parseInt(month) : new Date().getMonth() + 1,
      totalMinutes: weeklySummary.totalMinutes * 4,
      byGoal: weeklySummary.byGoal,
      byTag: weeklySummary.byTag,
      byMilestone: weeklySummary.byMilestone,
      completedTasks: weeklySummary.completedTasks * 4,
      weeklyBreakdown: [weeklySummary],
    })
  }),

  // GET /analytics/trends
  http.get(`${BASE_URL}/analytics/trends`, ({ request }) => {
    const url = new URL(request.url)
    const period = url.searchParams.get('period') || 'week'

    return HttpResponse.json({
      period,
      data: dailyStudyHours.map(d => ({
        period: d.date,
        totalMinutes: d.hours * 60,
        byGoal: goals.map(g => ({
          id: g.id,
          name: g.name,
          color: g.color,
          minutes: Math.floor(d.hours * 60 * Math.random()),
        })),
        completedTasks: Math.floor(d.hours),
      })),
    })
  }),

  // GET /analytics/goal-progress
  http.get(`${BASE_URL}/analytics/goal-progress`, ({ request }) => {
    const url = new URL(request.url)
    const goalId = url.searchParams.get('goal_id')

    const items = goals.map(g => ({
      goalId: g.id,
      goalName: g.name,
      goalColor: g.color,
      targetMinutes: 1200,
      actualMinutes: Math.floor(Math.random() * 1000) + 200,
      progressPercent: Math.floor(Math.random() * 40) + 60,
      trend: ['up', 'down', 'stable'][Math.floor(Math.random() * 3)] as 'up' | 'down' | 'stable',
    }))

    return HttpResponse.json({
      items: goalId ? items.filter(i => i.goalId === goalId) : items,
      overallProgress: 75,
    })
  }),
]
