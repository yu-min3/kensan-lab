/**
 * Timezone utilities for converting between local dates/times and UTC ISO datetimes
 *
 * The database stores all timestamps as TIMESTAMPTZ (effectively UTC).
 * The API returns UTC ISO 8601 strings (e.g., "2026-01-20T15:00:00Z").
 * Frontend is responsible for all timezone conversion.
 *
 * All conversions use Intl.DateTimeFormat to resolve timezone offsets dynamically,
 * which correctly handles DST transitions for any IANA timezone.
 *
 * Example: JST 2026-01-21 (Asia/Tokyo, UTC+9)
 *   - Start: 2026-01-21 00:00:00 JST = 2026-01-20 15:00:00 UTC
 *   - End:   2026-01-21 23:59:59 JST = 2026-01-21 14:59:59 UTC
 */

/**
 * Get the UTC offset in minutes for a given timezone at a specific point in time.
 * Uses Intl.DateTimeFormat to correctly handle DST.
 *
 * @param timezone - IANA timezone string (e.g., 'Asia/Tokyo', 'America/New_York')
 * @param atDate - The Date at which to evaluate the offset (defaults to now)
 * @returns Offset in minutes (positive for east of UTC, e.g., +540 for JST)
 */
export function getTimezoneOffsetMinutes(timezone: string, atDate: Date = new Date()): number {
  // Format the date parts in the target timezone
  const formatter = new Intl.DateTimeFormat('en-US', {
    timeZone: timezone,
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false,
  })

  const parts = formatter.formatToParts(atDate)
  const get = (type: Intl.DateTimeFormatPartTypes): number => {
    const val = parts.find((p) => p.type === type)?.value ?? '0'
    return parseInt(val, 10)
  }

  const year = get('year')
  const month = get('month')
  const day = get('day')
  let hour = get('hour')
  const minute = get('minute')
  const second = get('second')

  // Intl may return hour=24 for midnight; normalize to 0
  if (hour === 24) hour = 0

  // Build a UTC timestamp that represents the same wall-clock reading
  const localAsUtcMs = Date.UTC(year, month - 1, day, hour, minute, second)

  // The offset is the difference between "wall clock interpreted as UTC" and actual UTC
  const diffMs = localAsUtcMs - atDate.getTime()

  // Round to nearest minute to avoid sub-second drift from formatting
  return Math.round(diffMs / 60000)
}

/**
 * Get the UTC offset in hours for a timezone (backward-compatible helper).
 * For DST-observing timezones, the offset depends on the date.
 *
 * @param timezone - IANA timezone string
 * @param atDate - The Date at which to evaluate the offset (defaults to now)
 */
export function getTimezoneOffset(timezone: string, atDate?: Date): number {
  return getTimezoneOffsetMinutes(timezone, atDate) / 60
}

/**
 * Convert a local date string to UTC date range
 *
 * @param localDate - Date string in YYYY-MM-DD format (in user's timezone)
 * @param timezone - User's timezone (e.g., 'Asia/Tokyo')
 * @returns Object with startUtc and endUtc in ISO format
 */
export function localDateToUtcRange(
  localDate: string,
  timezone: string
): { startUtc: string; endUtc: string } {
  // Parse the local date
  const [year, month, day] = localDate.split('-').map(Number)

  // Build a rough UTC estimate for start-of-day, then compute the real offset at that moment
  const estimateMs = Date.UTC(year, month - 1, day, 0, 0, 0, 0)
  const estimateDate = new Date(estimateMs)
  const offsetMinutes = getTimezoneOffsetMinutes(timezone, estimateDate)

  // Create date at start of day in local timezone (00:00:00)
  // Then convert to UTC by subtracting the offset
  const localStartMs = Date.UTC(year, month - 1, day, 0, 0, 0, 0)
  const utcStartMs = localStartMs - offsetMinutes * 60 * 1000

  // End of day is start of next day - compute offset for end of day too
  // (in case of DST transition within the day)
  const localEndMs = Date.UTC(year, month - 1, day + 1, 0, 0, 0, 0)
  const endEstimate = new Date(localEndMs)
  const endOffsetMinutes = getTimezoneOffsetMinutes(timezone, endEstimate)
  const utcEndMs = localEndMs - endOffsetMinutes * 60 * 1000

  const startUtc = new Date(utcStartMs).toISOString()
  const endUtc = new Date(utcEndMs).toISOString()

  return { startUtc, endUtc }
}

/**
 * Convert a local date to the possible UTC dates it could span
 *
 * For JST, a single day can span 2 UTC dates:
 * - JST 2026-01-21 00:00 = UTC 2026-01-20 15:00
 * - JST 2026-01-21 23:59 = UTC 2026-01-21 14:59
 *
 * @param localDate - Date string in YYYY-MM-DD format
 * @param timezone - User's timezone
 * @returns Array of UTC date strings that the local date spans
 */
export function localDateToUtcDates(
  localDate: string,
  timezone: string
): string[] {
  const { startUtc, endUtc } = localDateToUtcRange(localDate, timezone)

  const startDate = startUtc.split('T')[0]
  const endDate = endUtc.split('T')[0]

  if (startDate === endDate) {
    return [startDate]
  }

  return [startDate, endDate]
}

/**
 * Format a Date object to YYYY-MM-DD in a specific timezone.
 * Uses Intl.DateTimeFormat for correct DST handling.
 */
export function formatDateInTimezone(date: Date, timezone: string): string {
  const formatter = new Intl.DateTimeFormat('en-CA', {
    timeZone: timezone,
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
  })

  // en-CA locale produces YYYY-MM-DD format
  return formatter.format(date)
}

/**
 * Get today's date in a specific timezone
 */
export function getTodayInTimezone(timezone: string): string {
  return formatDateInTimezone(new Date(), timezone)
}

/**
 * Get the local date (YYYY-MM-DD) from an ISO 8601 UTC datetime string
 *
 * @param isoDatetime - ISO 8601 UTC string (e.g., "2026-01-20T15:00:00Z")
 * @param timezone - User's timezone (e.g., 'Asia/Tokyo')
 * @returns Local date string in YYYY-MM-DD format
 */
export function getLocalDate(isoDatetime: string, timezone: string): string {
  const date = new Date(isoDatetime)
  return formatDateInTimezone(date, timezone)
}

/**
 * Get the local time (HH:mm) from an ISO 8601 UTC datetime string.
 * Uses Intl.DateTimeFormat for correct DST handling.
 *
 * @param isoDatetime - ISO 8601 UTC string (e.g., "2026-01-20T15:00:00Z")
 * @param timezone - User's timezone (e.g., 'Asia/Tokyo')
 * @returns Local time string in HH:mm format
 */
export function getLocalTime(isoDatetime: string, timezone: string): string {
  const date = new Date(isoDatetime)

  const formatter = new Intl.DateTimeFormat('en-GB', {
    timeZone: timezone,
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  })

  // en-GB with hour12:false produces HH:mm format
  return formatter.format(date)
}

/**
 * Convert a local date and time to a UTC ISO 8601 datetime string
 *
 * @param localDate - Date string in YYYY-MM-DD format (user's local timezone)
 * @param localTime - Time string in HH:mm format (user's local timezone)
 * @param timezone - User's timezone (e.g., 'Asia/Tokyo')
 * @returns ISO 8601 UTC datetime string (e.g., "2026-01-20T15:00:00.000Z")
 */
export function localToUtcDatetime(
  localDate: string,
  localTime: string,
  timezone: string
): string {
  // Parse local date and time
  const [year, month, day] = localDate.split('-').map(Number)
  const timeParts = localTime.split(':').map(Number)
  const hours = timeParts[0]
  const minutes = timeParts[1]

  // Build a rough estimate to determine the offset at this local datetime
  const estimateMs = Date.UTC(year, month - 1, day, hours, minutes, 0, 0)
  const offsetMinutes = getTimezoneOffsetMinutes(timezone, new Date(estimateMs))

  // Create timestamp at local time, then subtract offset to get UTC
  const localMs = Date.UTC(year, month - 1, day, hours, minutes, 0, 0)
  const utcMs = localMs - offsetMinutes * 60 * 1000

  return new Date(utcMs).toISOString()
}
