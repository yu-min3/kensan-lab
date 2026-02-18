/**
 * Task-related utility functions extracted from components.
 * Consolidates duplicated business logic for frequency, scheduling, and urgency.
 */
import type { Task, TaskFrequency } from '@/types'

// ============================================
// Frequency utilities
// ============================================

/**
 * Get Japanese display label for a task's frequency setting.
 * Shared by TaskListWidget and RecurringTaskWidget.
 */
export function getFrequencyLabel(frequency?: TaskFrequency, daysOfWeek?: number[]): string | null {
  if (!frequency) return null

  switch (frequency) {
    case 'daily':
      return '毎日'
    case 'weekly':
      return '平日'
    case 'custom': {
      const days = ['日', '月', '火', '水', '木', '金', '土']
      const selectedDays = (daysOfWeek ?? []).map(d => days[d]).join('')
      return selectedDays || null
    }
    default:
      return null
  }
}

/**
 * Get task frequency label from a Task object.
 */
export function getTaskFrequencyLabel(task: Task): string | null {
  return getFrequencyLabel(task.frequency, task.daysOfWeek)
}

/**
 * Calculate the planned count for the current week based on frequency.
 */
export function getPlannedCountThisWeek(frequency?: TaskFrequency, daysOfWeek?: number[]): number {
  if (!frequency) return 0

  switch (frequency) {
    case 'daily':
      return 7
    case 'weekly':
      return 5
    case 'custom':
      return daysOfWeek?.length ?? 0
    default:
      return 0
  }
}

// ============================================
// Scheduling utilities
// ============================================

/**
 * Determine if a task is scheduled for today based on its frequency setting.
 */
export function isScheduledForToday(task: Task): boolean {
  if (!task.frequency) return false

  const today = new Date()
  const dayOfWeek = today.getDay() // 0=Sun, 1=Mon, ..., 6=Sat

  switch (task.frequency) {
    case 'daily':
      return true
    case 'weekly':
      return dayOfWeek >= 1 && dayOfWeek <= 5
    case 'custom':
      return task.daysOfWeek?.includes(dayOfWeek) ?? false
    default:
      return false
  }
}

/**
 * Get the current week's Monday-Sunday range.
 */
export function getWeekRange(): { start: Date; end: Date; startStr: string; endStr: string } {
  const today = new Date()
  const dayOfWeek = today.getDay()
  const mondayOffset = dayOfWeek === 0 ? -6 : 1 - dayOfWeek

  const start = new Date(today)
  start.setDate(today.getDate() + mondayOffset)
  start.setHours(0, 0, 0, 0)

  const end = new Date(start)
  end.setDate(start.getDate() + 6)
  end.setHours(23, 59, 59, 999)

  const formatDate = (d: Date) => d.toISOString().split('T')[0]

  return {
    start,
    end,
    startStr: formatDate(start),
    endStr: formatDate(end),
  }
}

// ============================================
// Deadline / Urgency utilities
// ============================================

export type UrgencyLevel = 'danger' | 'warning' | 'normal' | 'no-deadline'

/**
 * Calculate days remaining until a target date.
 */
export function getDaysUntil(targetDate: string | undefined): number | null {
  if (!targetDate) return null
  const today = new Date()
  today.setHours(0, 0, 0, 0)
  const target = new Date(targetDate)
  target.setHours(0, 0, 0, 0)
  const diffTime = target.getTime() - today.getTime()
  return Math.ceil(diffTime / (1000 * 60 * 60 * 24))
}

/**
 * Determine urgency level from days remaining.
 */
export function getUrgencyLevel(daysUntil: number | null): UrgencyLevel {
  if (daysUntil === null) return 'no-deadline'
  if (daysUntil <= 3) return 'danger'
  if (daysUntil <= 7) return 'warning'
  return 'normal'
}

/**
 * Format days remaining as a Japanese display string.
 */
export function formatDaysUntil(days: number | null): string {
  if (days === null) return '期限なし'
  if (days < 0) return `${Math.abs(days)}日超過`
  if (days === 0) return '今日'
  if (days === 1) return '明日'
  return `残り${days}日`
}

// ============================================
// Duration calculation utilities
// ============================================

/**
 * Calculate total minutes from an array of items with start/end datetimes.
 */
export function calculateMinutesFromDatetimes(items: { startDatetime: string; endDatetime: string }[]): number {
  return items.reduce((acc, item) => {
    const startMs = new Date(item.startDatetime).getTime()
    const endMs = new Date(item.endDatetime).getTime()
    return acc + (endMs - startMs) / 60000
  }, 0)
}

/**
 * Calculate completion rate as a rounded percentage.
 */
export function calculateRate(actual: number, planned: number): number {
  return planned > 0 ? Math.round((actual / planned) * 100) : 0
}
