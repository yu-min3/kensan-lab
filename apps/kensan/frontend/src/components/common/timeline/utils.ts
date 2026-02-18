/**
 * Utility functions for timeline calculations
 */

/**
 * Format time string (HH:mm:ss -> HH:mm)
 */
export function formatTime(time: string): string {
  return time.slice(0, 5)
}

/**
 * Get total minutes from time string (HH:mm or HH:mm:ss)
 */
export function getMinutesFromTime(time: string): number {
  const [hours, minutes] = time.split(':').map(Number)
  return hours * 60 + minutes
}

/**
 * Get duration in minutes between two times
 */
export function getDurationMinutes(start: string, end: string): number {
  return getMinutesFromTime(end) - getMinutesFromTime(start)
}

/**
 * Convert total minutes to time string (HH:mm)
 */
export function minutesToTimeString(totalMinutes: number): string {
  const hours = Math.floor(totalMinutes / 60)
  const minutes = totalMinutes % 60
  return `${hours.toString().padStart(2, '0')}:${minutes.toString().padStart(2, '0')}`
}

/**
 * Snap minutes to interval (default 15 minutes)
 */
export function snapToInterval(minutes: number, interval: number = 15): number {
  return Math.round(minutes / interval) * interval
}

/**
 * Calculate top position percentage for a time
 */
export function calculateTopPosition(
  time: string,
  startHour: number,
  totalMinutes: number
): number {
  const minutes = getMinutesFromTime(time) - startHour * 60
  return (minutes / totalMinutes) * 100
}

/**
 * Calculate height percentage for a time range
 */
export function calculateHeight(
  start: string,
  end: string,
  totalMinutes: number
): number {
  const duration = getDurationMinutes(start, end)
  return (duration / totalMinutes) * 100
}

/**
 * Overlap layout info for a time block
 */
export interface OverlapLayout {
  column: number
  totalColumns: number
}

/**
 * Calculate overlap layout for time blocks.
 * Returns a map from block ID to its column position and total columns in its group.
 * Blocks that overlap are placed side-by-side like Google Calendar.
 */
export function calculateOverlapLayout(
  blocks: Array<{ id: string; startTime: string; endTime: string }>
): Map<string, OverlapLayout> {
  if (blocks.length === 0) return new Map()

  // Sort by start time, then by duration (longer first)
  const sorted = [...blocks].sort((a, b) => {
    const startDiff = getMinutesFromTime(a.startTime) - getMinutesFromTime(b.startTime)
    if (startDiff !== 0) return startDiff
    // Longer blocks first so they get earlier columns
    const aDuration = getMinutesFromTime(a.endTime) - getMinutesFromTime(a.startTime)
    const bDuration = getMinutesFromTime(b.endTime) - getMinutesFromTime(b.startTime)
    return bDuration - aDuration
  })

  // For each block, find which column it can go in
  const columnEnds: number[] = [] // end time (in minutes) of the last block in each column
  const blockColumns = new Map<string, number>()

  for (const block of sorted) {
    const blockStart = getMinutesFromTime(block.startTime)
    // Find the first column where this block fits (no overlap)
    let placed = false
    for (let col = 0; col < columnEnds.length; col++) {
      if (columnEnds[col] <= blockStart) {
        columnEnds[col] = getMinutesFromTime(block.endTime)
        blockColumns.set(block.id, col)
        placed = true
        break
      }
    }
    if (!placed) {
      blockColumns.set(block.id, columnEnds.length)
      columnEnds.push(getMinutesFromTime(block.endTime))
    }
  }

  // Now determine how many columns each block's group actually has.
  // A block's totalColumns = max columns among all blocks that overlap with it (including itself).
  const result = new Map<string, OverlapLayout>()

  for (const block of sorted) {
    const blockStart = getMinutesFromTime(block.startTime)
    const blockEnd = getMinutesFromTime(block.endTime)
    let maxCol = blockColumns.get(block.id)!

    for (const other of sorted) {
      if (other.id === block.id) continue
      const otherStart = getMinutesFromTime(other.startTime)
      const otherEnd = getMinutesFromTime(other.endTime)
      // Check overlap
      if (otherStart < blockEnd && otherEnd > blockStart) {
        maxCol = Math.max(maxCol, blockColumns.get(other.id)!)
      }
    }

    result.set(block.id, {
      column: blockColumns.get(block.id)!,
      totalColumns: maxCol + 1,
    })
  }

  return result
}

/**
 * Calculate time from Y position (default 1 hour duration)
 */
export function calculateTimeFromY(
  clientY: number,
  containerRect: DOMRect,
  startHour: number,
  endHour: number,
  interval: number = 15
): { startTime: string; endTime: string } {
  return calculateTimeFromYWithDuration(clientY, containerRect, startHour, endHour, 60, interval)
}

/**
 * Calculate time from Y position with custom duration
 */
export function calculateTimeFromYWithDuration(
  clientY: number,
  containerRect: DOMRect,
  startHour: number,
  endHour: number,
  durationMinutes: number,
  interval: number = 15
): { startTime: string; endTime: string } {
  const totalMinutes = (endHour - startHour) * 60
  const relativeY = Math.max(0, Math.min(clientY - containerRect.top, containerRect.height))
  const percentage = relativeY / containerRect.height
  const rawMinutes = percentage * totalMinutes + startHour * 60
  const snappedMinutes = Math.round(rawMinutes / interval) * interval
  const clampedStartMinutes = Math.max(startHour * 60, Math.min(endHour * 60 - durationMinutes, snappedMinutes))

  const endMinutes = Math.min(clampedStartMinutes + durationMinutes, endHour * 60)

  return {
    startTime: minutesToTimeString(clampedStartMinutes),
    endTime: minutesToTimeString(endMinutes),
  }
}

/**
 * Convert Y position to minutes (for drag/resize operations)
 */
export function yToMinutes(
  clientY: number,
  containerRect: DOMRect,
  startHour: number,
  endHour: number,
  interval: number = 15
): number {
  const totalMinutes = (endHour - startHour) * 60
  const relativeY = clientY - containerRect.top
  const percentage = relativeY / containerRect.height
  const minutes = percentage * totalMinutes + startHour * 60
  return snapToInterval(Math.max(startHour * 60, Math.min(endHour * 60, minutes)), interval)
}

/**
 * Calculate top position in pixels (for fixed pixel-based layouts like Weekly)
 */
export function calculateTopPx(time: string, startHour: number, hourHeight: number): number {
  const [h, m] = time.split(':').map(Number)
  const hours = h + m / 60
  const clamped = Math.max(startHour, hours)
  return (clamped - startHour) * hourHeight
}

/**
 * Calculate height in pixels (for fixed pixel-based layouts like Weekly)
 */
export function calculateHeightPx(
  startTime: string,
  endTime: string,
  startHour: number,
  endHour: number,
  hourHeight: number
): number {
  const [sh, sm] = startTime.split(':').map(Number)
  const [eh, em] = endTime.split(':').map(Number)
  const startHours = Math.max(startHour, sh + sm / 60)
  const endHours = Math.min(endHour, eh + em / 60)
  return Math.max(hourHeight / 2, (endHours - startHours) * hourHeight)
}
