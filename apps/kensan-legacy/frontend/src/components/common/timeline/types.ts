import type { TimeEntry } from '@/types'

/**
 * Shared types for timeline components
 */

// Running timer data
export interface RunningTimerData {
  taskName: string
  startedAt: string // ISO timestamp
  goalId?: string
  goalName?: string
  goalColor?: string
  milestoneName?: string
}

// Resize edge type
export type ResizeEdge = 'top' | 'bottom'

// Resize state for drag/resize operations
export interface ResizeState {
  blockId: string
  edge: ResizeEdge
  initialY: number
  initialStartTime: string
  initialEndTime: string
}

// Drag state for moving blocks
export interface DragState {
  blockId: string
  initialY: number
  initialStartTime: string
  initialEndTime: string
  duration: number // in minutes
}

// Preview time during drag/resize
export interface PreviewTime {
  startTime: string
  endTime: string
}

// Time block display times (can be preview or actual)
export interface DisplayTimes {
  startTime: string
  endTime: string
}

// Props for TimeEntryItem component
export interface TimeEntryItemProps {
  entry: TimeEntry
  onEntryClick?: (entry: TimeEntry) => void
  onEntryDelete?: (entryId: string) => void
  getTopPosition: (time: string) => number
  getHeight: (start: string, end: string) => number
}

// Props for running timer display
export interface RunningTimerItemProps {
  runningTimer: RunningTimerData
  startTime: string
  endTime: string
  getTopPosition: (time: string) => number
  getHeight: (start: string, end: string) => number
}

// Grid props
export interface TimeBlockTimelineGridProps {
  hours: number[]
  hourHeight: number
}

// Generic drag/resize state (for Weekly calendar etc.)
export interface GenericDragResizeState {
  blockId: string
  type: 'drag' | 'resize'
  edge?: ResizeEdge
  initialY: number
  initialStartTime: string
  initialEndTime: string
  duration: number // in minutes
  // Optional: date context for multi-day views
  dateStr?: string
}
