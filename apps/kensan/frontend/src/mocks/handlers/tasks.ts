// Goals, Milestones, Tags, Tasks MSW handlers
import { http, HttpResponse } from 'msw'
import { goals, milestones, tags, noteTags, tasks } from '../data'
import { createMockCrudHandlers, createToggleHandler } from '../createMockCrudHandlers'

const BASE_URL = 'http://localhost:8082/api/v1'

// Transform to API response format
const toGoalResponse = (g: (typeof goals)[0]) => ({
  id: g.id,
  name: g.name,
  description: g.description,
  color: g.color,
  status: g.status,
  sortOrder: g.sortOrder,
  createdAt: g.createdAt.toISOString(),
  updatedAt: g.updatedAt.toISOString(),
})

const toMilestoneResponse = (m: (typeof milestones)[0]) => ({
  id: m.id,
  goalId: m.goalId,
  name: m.name,
  description: m.description,
  targetDate: m.targetDate,
  status: m.status,
  createdAt: m.createdAt.toISOString(),
  updatedAt: m.updatedAt.toISOString(),
})

const toTagResponse = (t: (typeof tags)[0]) => ({
  id: t.id,
  name: t.name,
  color: t.color,
  type: t.type,
  pinned: t.pinned,
  usageCount: t.usageCount,
  createdAt: t.createdAt.toISOString(),
  updatedAt: t.updatedAt.toISOString(),
})

const toTaskResponse = (t: (typeof tasks)[0], index?: number) => ({
  id: t.id,
  name: t.name,
  milestoneId: t.milestoneId,
  parentTaskId: t.parentTaskId,
  tagIds: t.tagIds,
  estimatedMinutes: t.estimatedMinutes,
  completed: t.completed,
  dueDate: t.dueDate,
  frequency: t.frequency,
  daysOfWeek: t.daysOfWeek,
  sortOrder: t.sortOrder ?? index ?? 0,
  createdAt: t.createdAt.toISOString(),
  updatedAt: t.updatedAt.toISOString(),
})

// Goal CRUD handlers
const goalHandlers = createMockCrudHandlers(
  {
    baseUrl: BASE_URL,
    resourcePath: '/goals',
    transform: toGoalResponse,
    data: goals,
    getId: (g) => g.id,
    idPrefix: 'goal-',
    resourceName: 'Goal',
    prependOnAdd: false,
  },
  {
    filters: [
      { paramName: 'status', fieldName: 'status', type: 'equals' },
    ],
  }
)

// Milestone CRUD handlers
const milestoneHandlers = createMockCrudHandlers(
  {
    baseUrl: BASE_URL,
    resourcePath: '/milestones',
    transform: toMilestoneResponse,
    data: milestones,
    getId: (m) => m.id,
    idPrefix: 'ms-',
    resourceName: 'Milestone',
    prependOnAdd: false,
  },
  {
    filters: [
      { paramName: 'goal_id', fieldName: 'goalId', type: 'equals' },
      { paramName: 'status', fieldName: 'status', type: 'equals' },
    ],
  }
)

// Tag CRUD handlers
const tagHandlers = createMockCrudHandlers(
  {
    baseUrl: BASE_URL,
    resourcePath: '/tags',
    transform: toTagResponse,
    data: tags,
    getId: (t) => t.id,
    idPrefix: 'tag-',
    resourceName: 'Tag',
    prependOnAdd: false,
  },
  {}
)

// Note Tag handlers (separate from task tags)
const noteTagHandlers = createMockCrudHandlers(
  {
    baseUrl: BASE_URL,
    resourcePath: '/note-tags',
    transform: toTagResponse,
    data: noteTags,
    getId: (t) => t.id,
    idPrefix: 'ntag-',
    resourceName: 'NoteTag',
    prependOnAdd: false,
  },
  {}
)

// Task CRUD handlers
const taskCrudHandlers = createMockCrudHandlers(
  {
    baseUrl: BASE_URL,
    resourcePath: '/tasks',
    transform: toTaskResponse,
    data: tasks,
    getId: (t) => t.id,
    idPrefix: 't',
    resourceName: 'Task',
    prependOnAdd: false,
  },
  {
    filters: [
      { paramName: 'milestone_id', fieldName: 'milestoneId', type: 'equals' },
      { paramName: 'completed', fieldName: 'completed', type: 'boolean' },
      { paramName: 'parent_id', fieldName: 'parentTaskId', type: 'equals' },
    ],
  }
)

// Task toggle complete handler
const taskToggleHandler = createToggleHandler({
  baseUrl: BASE_URL,
  resourcePath: '/tasks',
  data: tasks,
  getId: (t) => t.id,
  transform: toTaskResponse,
  resourceName: 'Task',
  toggleField: 'completed',
  toggleEndpoint: 'complete',
})

// Task reorder handler
const taskReorderHandler = http.post(`${BASE_URL}/tasks/reorder`, async ({ request }) => {
  const { taskIds } = (await request.json()) as { taskIds: string[] }

  // Update sortOrder for each task in the provided order
  const updatedTasks: ReturnType<typeof toTaskResponse>[] = []
  taskIds.forEach((taskId, index) => {
    const taskIndex = tasks.findIndex((t) => t.id === taskId)
    if (taskIndex !== -1) {
      tasks[taskIndex].sortOrder = index
      tasks[taskIndex].updatedAt = new Date()
      updatedTasks.push(toTaskResponse(tasks[taskIndex]))
    }
  })

  return HttpResponse.json(updatedTasks)
})

// Task bulk delete handler
const taskBulkDeleteHandler = http.post(`${BASE_URL}/tasks/bulk-delete`, async ({ request }) => {
  const { taskIds } = (await request.json()) as { taskIds: string[] }

  // Remove tasks and their children
  const idsToDelete = new Set(taskIds)

  // Also find child tasks
  tasks.forEach((t) => {
    if (t.parentTaskId && idsToDelete.has(t.parentTaskId)) {
      idsToDelete.add(t.id)
    }
  })

  // Filter out deleted tasks
  const remainingTasks = tasks.filter((t) => !idsToDelete.has(t.id))
  tasks.length = 0
  tasks.push(...remainingTasks)

  return HttpResponse.json({ deleted: Array.from(idsToDelete) })
})

// Task bulk complete handler
const taskBulkCompleteHandler = http.post(`${BASE_URL}/tasks/bulk-complete`, async ({ request }) => {
  const { taskIds, completed } = (await request.json()) as { taskIds: string[]; completed: boolean }

  const updatedTasks: ReturnType<typeof toTaskResponse>[] = []
  taskIds.forEach((taskId) => {
    const taskIndex = tasks.findIndex((t) => t.id === taskId)
    if (taskIndex !== -1) {
      tasks[taskIndex].completed = completed
      tasks[taskIndex].updatedAt = new Date()
      updatedTasks.push(toTaskResponse(tasks[taskIndex]))
    }
  })

  return HttpResponse.json(updatedTasks)
})

export const taskHandlers = [
  ...goalHandlers,
  ...milestoneHandlers,
  ...tagHandlers,
  ...noteTagHandlers,
  ...taskCrudHandlers,
  taskToggleHandler,
  taskReorderHandler,
  taskBulkDeleteHandler,
  taskBulkCompleteHandler,
]
