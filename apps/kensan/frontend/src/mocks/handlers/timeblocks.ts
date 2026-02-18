// Timeblocks & TimeEntries MSW handlers
import { http, HttpResponse } from 'msw'
import { timeBlocks, timeEntries } from '../data'
import { createMockCrudHandlers } from '../createMockCrudHandlers'
import type { TimeBlock, TimeEntry } from '@/types'

const BASE_URL = 'http://localhost:8084/api/v1'

// Transform to API response format
const toTimeBlockResponse = (tb: TimeBlock) => ({
  id: tb.id,
  startDatetime: tb.startDatetime,
  endDatetime: tb.endDatetime,
  taskId: tb.taskId,
  taskName: tb.taskName,
  milestoneId: tb.milestoneId,
  milestoneName: tb.milestoneName,
  goalId: tb.goalId,
  goalName: tb.goalName,
  goalColor: tb.goalColor,
  tagIds: tb.tagIds,
})

const toTimeEntryResponse = (te: TimeEntry) => ({
  id: te.id,
  startDatetime: te.startDatetime,
  endDatetime: te.endDatetime,
  taskId: te.taskId,
  taskName: te.taskName,
  milestoneId: te.milestoneId,
  milestoneName: te.milestoneName,
  goalId: te.goalId,
  goalName: te.goalName,
  goalColor: te.goalColor,
  tagIds: te.tagIds,
  description: te.description,
})

// Base CRUD handlers for TimeBlocks
const timeBlockCrudHandlers = createMockCrudHandlers(
  {
    baseUrl: BASE_URL,
    resourcePath: '/timeblocks',
    transform: toTimeBlockResponse,
    data: timeBlocks,
    getId: (tb) => tb.id,
    idPrefix: 'tb',
    resourceName: 'TimeBlock',
  },
  {
    filters: [],
  }
)

// Base CRUD handlers for TimeEntries
const timeEntryCrudHandlers = createMockCrudHandlers(
  {
    baseUrl: BASE_URL,
    resourcePath: '/time-entries',
    transform: toTimeEntryResponse,
    data: timeEntries,
    getId: (te) => te.id,
    idPrefix: 'te',
    resourceName: 'TimeEntry',
  },
  {
    filters: [],
  }
)

// Custom handlers for datetime range filtering
const customTimeBlockHandlers = [
  // GET /timeblocks - Handle start_datetime/end_datetime range filters
  http.get(`${BASE_URL}/timeblocks`, ({ request }) => {
    const url = new URL(request.url)
    const startDatetime = url.searchParams.get('start_datetime')
    const endDatetime = url.searchParams.get('end_datetime')

    let result = [...timeBlocks]

    if (startDatetime && endDatetime) {
      // UTC datetime range filter
      result = result.filter(tb =>
        tb.startDatetime >= startDatetime && tb.startDatetime < endDatetime
      )
    }

    return HttpResponse.json(result.map(toTimeBlockResponse))
  }),
]

const customTimeEntryHandlers = [
  // GET /time-entries - Handle start_datetime/end_datetime range filters
  http.get(`${BASE_URL}/time-entries`, ({ request }) => {
    const url = new URL(request.url)
    const startDatetime = url.searchParams.get('start_datetime')
    const endDatetime = url.searchParams.get('end_datetime')

    let result = [...timeEntries]

    if (startDatetime && endDatetime) {
      // UTC datetime range filter
      result = result.filter(te =>
        te.startDatetime >= startDatetime && te.startDatetime < endDatetime
      )
    }

    return HttpResponse.json(result.map(toTimeEntryResponse))
  }),
]

// Export: custom handlers first (override CRUD list), then CRUD handlers (excluding list)
export const timeblockHandlers = [
  ...customTimeBlockHandlers,
  ...timeBlockCrudHandlers.slice(1), // Skip generated list handler
  ...customTimeEntryHandlers,
  ...timeEntryCrudHandlers.slice(1), // Skip generated list handler
]
