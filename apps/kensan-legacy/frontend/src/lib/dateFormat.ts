// Date/Time formatting utilities
import { format } from 'date-fns'
import { ja } from 'date-fns/locale'

/**
 * Format duration in minutes to human-readable Japanese string.
 * Examples:
 *   30 → "30分"
 *   60 → "1時間"
 *   90 → "1時間30分"
 *   120 → "2時間"
 */
export function formatMinutes(minutes: number): string {
  if (minutes >= 60) {
    const h = Math.floor(minutes / 60)
    const m = Math.round(minutes % 60)
    return m > 0 ? `${h}時間${m}分` : `${h}時間`
  }
  return `${Math.round(minutes)}分`
}

/**
 * Format duration in minutes to short English string.
 * Examples:
 *   30 → "30m"
 *   60 → "1h"
 *   90 → "1h 30m"
 *   120 → "2h"
 */
export function formatDurationShort(minutes: number): string {
  const h = Math.floor(minutes / 60)
  const m = Math.round(minutes % 60)
  if (h > 0 && m > 0) return `${h}h ${m}m`
  if (h > 0) return `${h}h`
  return `${m}m`
}

/**
 * Format date to Japanese month string.
 * Example: 2026-01-15 → "2026年1月"
 */
export function formatMonth(date: Date | string): string {
  const d = typeof date === 'string' ? new Date(date) : date
  return format(d, 'yyyy年M月', { locale: ja })
}

/**
 * Format date to Japanese date string with day of week.
 * Example: 2026-01-15 → "2026年1月15日（水）"
 */
export function formatDateJa(date: Date | string): string {
  const d = typeof date === 'string' ? new Date(date) : date
  return format(d, 'yyyy年M月d日（E）', { locale: ja })
}

/**
 * Format date to short Japanese date string with day of week.
 * Example: 2026-01-15 → "1月15日（水）"
 */
export function formatDateShortJa(date: Date | string): string {
  const d = typeof date === 'string' ? new Date(date) : date
  return format(d, 'M月d日（E）', { locale: ja })
}

/**
 * Format date to ISO date string (YYYY-MM-DD).
 * Example: 2026-01-15 → "2026-01-15"
 */
export function formatDateIso(date: Date | string): string {
  const d = typeof date === 'string' ? new Date(date) : date
  return format(d, 'yyyy-MM-dd')
}

/**
 * Format date to time string (HH:mm).
 * Example: 2026-01-15T09:30:00 → "09:30"
 */
export function formatTime(date: Date | string): string {
  const d = typeof date === 'string' ? new Date(date) : date
  return format(d, 'HH:mm')
}

/**
 * Format date range to Japanese string.
 * Example: (Jan 8, Jan 14) → "1月8日 - 1月14日"
 */
export function formatDateRange(startDate: Date | string, endDate: Date | string): string {
  const start = typeof startDate === 'string' ? new Date(startDate) : startDate
  const end = typeof endDate === 'string' ? new Date(endDate) : endDate
  return `${format(start, 'M月d日', { locale: ja })} - ${format(end, 'M月d日', { locale: ja })}`
}

/**
 * Get day of month as string.
 * Example: 2026-01-15 → "15"
 */
export function formatDayOfMonth(date: Date | string): string {
  const d = typeof date === 'string' ? new Date(date) : date
  return format(d, 'd')
}

/**
 * Get day of week abbreviation in Japanese.
 * Example: 2026-01-15 → "水"
 */
export function formatDayOfWeekJa(date: Date | string): string {
  const d = typeof date === 'string' ? new Date(date) : date
  return format(d, 'E', { locale: ja })
}

/**
 * Format date to short month/day string.
 * Example: 2026-01-15 → "1/15"
 */
export function formatMonthDay(date: Date | string): string {
  const d = typeof date === 'string' ? new Date(date) : date
  return format(d, 'M/d')
}
