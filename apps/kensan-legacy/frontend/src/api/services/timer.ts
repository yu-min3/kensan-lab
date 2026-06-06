import { API_CONFIG } from '../config'
import { httpClient } from '../client'
import type { TimeEntry } from '@/types'

export interface RunningTimer {
  id: string
  taskId?: string
  taskName: string
  milestoneId?: string
  milestoneName?: string
  goalId?: string
  goalName?: string
  goalColor?: string
  tagIds?: string[]
  startedAt: string // ISO timestamp
}

export interface StartTimerInput {
  taskId?: string
  taskName: string
  milestoneId?: string
  milestoneName?: string
  goalId?: string
  goalName?: string
  goalColor?: string
  tagIds?: string[]
}

export interface StopTimerResult {
  timeEntry: TimeEntry
  duration: number // seconds
}

export const timerApi = {
  async getCurrent(): Promise<RunningTimer | null> {
    return httpClient.get<RunningTimer | null>(
      API_CONFIG.baseUrls.timeblock,
      '/timer/current'
    )
  },

  async start(input: StartTimerInput): Promise<RunningTimer> {
    return httpClient.post<RunningTimer>(
      API_CONFIG.baseUrls.timeblock,
      '/timer/start',
      input
    )
  },

  async stop(): Promise<StopTimerResult> {
    return httpClient.post<StopTimerResult>(
      API_CONFIG.baseUrls.timeblock,
      '/timer/stop'
    )
  },
}
