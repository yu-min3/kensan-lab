// Timer MSW handlers
import { http, HttpResponse } from 'msw'
import { generateId, goals, milestones } from '../data'

const BASE_URL = 'http://localhost:8084/api/v1'

interface RunningTimer {
  id: string
  taskId?: string
  taskName: string
  milestoneId?: string
  milestoneName?: string
  goalId?: string
  goalName?: string
  goalColor?: string
  tagIds?: string[]
  startedAt: string
}

interface TimeEntry {
  id: string
  date: string
  startTime: string
  endTime: string
  taskId?: string
  taskName: string
  milestoneId?: string
  milestoneName?: string
  goalId?: string
  goalName?: string
  goalColor?: string
  tagIds?: string[]
  description?: string
}

// In-memory timer state (only one timer can run at a time)
let currentTimer: RunningTimer | null = null

export const timerHandlers = [
  // GET /timer/current - Get the current running timer
  http.get(`${BASE_URL}/timer/current`, () => {
    return HttpResponse.json(currentTimer)
  }),

  // POST /timer/start - Start a new timer
  http.post(`${BASE_URL}/timer/start`, async ({ request }) => {
    const body = await request.json() as {
      taskId?: string
      taskName: string
      milestoneId?: string
      milestoneName?: string
      goalId?: string
      goalName?: string
      goalColor?: string
      tagIds?: string[]
    }

    // Stop any existing timer (implicitly)
    currentTimer = null

    // Find milestone and goal info if milestoneId is provided
    let milestoneName = body.milestoneName
    let goalId = body.goalId
    let goalName = body.goalName
    let goalColor = body.goalColor

    if (body.milestoneId) {
      const milestone = milestones.find(m => m.id === body.milestoneId)
      if (milestone) {
        milestoneName = milestone.name
        const goal = goals.find(g => g.id === milestone.goalId)
        if (goal) {
          goalId = goal.id
          goalName = goal.name
          goalColor = goal.color
        }
      }
    }

    // Create new timer
    currentTimer = {
      id: generateId('timer'),
      taskId: body.taskId,
      taskName: body.taskName,
      milestoneId: body.milestoneId,
      milestoneName,
      goalId,
      goalName,
      goalColor,
      tagIds: body.tagIds || [],
      startedAt: new Date().toISOString(),
    }

    return HttpResponse.json(currentTimer, { status: 201 })
  }),

  // POST /timer/stop - Stop the current timer
  http.post(`${BASE_URL}/timer/stop`, () => {
    if (!currentTimer) {
      return HttpResponse.json(
        { error: { code: 'NO_RUNNING_TIMER', message: 'No timer is currently running' } },
        { status: 400 }
      )
    }

    const now = new Date()
    const startedAt = new Date(currentTimer.startedAt)

    // Create time entry from timer (stored in UTC)
    const timeEntry: TimeEntry = {
      id: generateId('entry'),
      date: startedAt.toISOString().split('T')[0],
      startTime: startedAt.toISOString().split('T')[1].slice(0, 5),
      endTime: now.toISOString().split('T')[1].slice(0, 5),
      taskId: currentTimer.taskId,
      taskName: currentTimer.taskName,
      milestoneId: currentTimer.milestoneId,
      milestoneName: currentTimer.milestoneName,
      goalId: currentTimer.goalId,
      goalName: currentTimer.goalName,
      goalColor: currentTimer.goalColor,
      tagIds: currentTimer.tagIds,
    }

    // Calculate duration in seconds
    const duration = Math.floor((now.getTime() - startedAt.getTime()) / 1000)

    // Clear the timer
    currentTimer = null

    return HttpResponse.json({
      timeEntry,
      duration,
    })
  }),
]
