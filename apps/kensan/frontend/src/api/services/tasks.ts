// Goals, Milestones, Tags, Tasks, Todos API Service
import { API_CONFIG } from '../config'
import { httpClient } from '../client'
import { createApiService, extendApiService } from '../createApiService'
import type { Goal, GoalStatus, Milestone, MilestoneStatus, Tag, TagCategory, Task, EntityMemo, EntityType, Todo, TodoWithStatus, TodoFrequency } from '@/types'

// ============================================
// Goal API
// ============================================
interface GoalResponse {
  id: string
  name: string
  description?: string
  color: string
  status: GoalStatus
  sortOrder: number
  createdAt: string
  updatedAt: string
}

const transformGoal = (g: GoalResponse): Goal => ({
  id: g.id,
  name: g.name,
  description: g.description,
  color: g.color,
  status: g.status,
  sortOrder: g.sortOrder,
  createdAt: new Date(g.createdAt),
  updatedAt: new Date(g.updatedAt),
})

export interface CreateGoalInput {
  name: string
  description?: string
  color: string
}

export interface UpdateGoalInput {
  name?: string
  description?: string
  color?: string
  status?: GoalStatus
}

export interface GoalFilter {
  status?: GoalStatus
}

const baseGoalsApi = createApiService<
  GoalResponse,
  Goal,
  CreateGoalInput,
  UpdateGoalInput,
  GoalFilter
>({
  baseUrl: API_CONFIG.baseUrls.task,
  resourcePath: '/goals',
  transform: transformGoal,
})

export const goalsApi = extendApiService(baseGoalsApi, () => ({
  async reorder(goalIds: string[]): Promise<Goal[]> {
    const response = await httpClient.post<GoalResponse[]>(
      API_CONFIG.baseUrls.task,
      '/goals/reorder',
      { goalIds }
    )
    return response.map(transformGoal)
  },
}))

// ============================================
// Milestone API
// ============================================
interface MilestoneResponse {
  id: string
  goalId: string
  name: string
  description?: string
  startDate?: string
  targetDate?: string
  status: MilestoneStatus
  createdAt: string
  updatedAt: string
}

const transformMilestone = (m: MilestoneResponse): Milestone => ({
  id: m.id,
  goalId: m.goalId,
  name: m.name,
  description: m.description,
  startDate: m.startDate,
  targetDate: m.targetDate,
  status: m.status,
  createdAt: new Date(m.createdAt),
  updatedAt: new Date(m.updatedAt),
})

export interface CreateMilestoneInput {
  goalId: string
  name: string
  description?: string
  startDate?: string
  targetDate?: string
}

export interface UpdateMilestoneInput {
  name?: string
  description?: string
  startDate?: string
  targetDate?: string
  status?: MilestoneStatus
}

export interface MilestoneFilter {
  goalId?: string
  status?: MilestoneStatus
}

export const milestonesApi = createApiService<
  MilestoneResponse,
  Milestone,
  CreateMilestoneInput,
  UpdateMilestoneInput,
  MilestoneFilter
>(
  {
    baseUrl: API_CONFIG.baseUrls.task,
    resourcePath: '/milestones',
    transform: transformMilestone,
  },
  {
    filterMappings: {
      goalId: 'goal_id',
    },
  }
)

// ============================================
// Tag API
// ============================================
interface TagResponse {
  id: string
  name: string
  color: string
  type: 'task' | 'note'
  category: 'general' | 'trait' | 'tech' | 'project'
  pinned: boolean
  usageCount: number
  createdAt: string
  updatedAt: string
}

const transformTag = (t: TagResponse): Tag => ({
  id: t.id,
  name: t.name,
  color: t.color,
  type: t.type,
  category: t.category || 'general',
  pinned: t.pinned,
  usageCount: t.usageCount,
  createdAt: new Date(t.createdAt),
  updatedAt: new Date(t.updatedAt),
})

export interface CreateTagInput {
  name: string
  color: string
  category?: TagCategory
  pinned?: boolean
}

export interface UpdateTagInput {
  name?: string
  color?: string
  category?: TagCategory
  pinned?: boolean
}

export const tagsApi = createApiService<
  TagResponse,
  Tag,
  CreateTagInput,
  UpdateTagInput,
  Record<string, never>
>({
  baseUrl: API_CONFIG.baseUrls.task,
  resourcePath: '/tags',
  transform: transformTag,
})

// ============================================
// Task API
// ============================================
interface TaskResponse {
  id: string
  name: string
  milestoneId?: string
  parentTaskId?: string
  tagIds?: string[]
  estimatedMinutes?: number
  completed: boolean
  dueDate?: string
  frequency?: 'daily' | 'weekly' | 'custom'
  daysOfWeek?: number[]
  sortOrder: number
  createdAt: string
  updatedAt: string
}

const transformTask = (t: TaskResponse): Task => ({
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
  sortOrder: t.sortOrder,
  createdAt: new Date(t.createdAt),
  updatedAt: new Date(t.updatedAt),
})

export interface CreateTaskInput {
  name: string
  milestoneId?: string
  parentTaskId?: string
  tagIds?: string[]
  estimatedMinutes?: number
  dueDate?: string
  frequency?: 'daily' | 'weekly' | 'custom'
  daysOfWeek?: number[]
}

export interface UpdateTaskInput {
  name?: string
  milestoneId?: string | null
  parentTaskId?: string | null
  tagIds?: string[]
  estimatedMinutes?: number
  dueDate?: string | null
  completed?: boolean
  frequency?: 'daily' | 'weekly' | 'custom'
  daysOfWeek?: number[]
}

export interface TaskFilter {
  milestoneId?: string
  completed?: boolean
  parentId?: string
}

const baseTasksApi = createApiService<
  TaskResponse,
  Task,
  CreateTaskInput,
  UpdateTaskInput,
  TaskFilter
>(
  {
    baseUrl: API_CONFIG.baseUrls.task,
    resourcePath: '/tasks',
    transform: transformTask,
  },
  {
    filterMappings: {
      milestoneId: 'milestone_id',
      parentId: 'parent_id',
    },
  }
)

export const tasksApi = extendApiService(baseTasksApi, () => ({
  async toggleComplete(id: string): Promise<Task> {
    const response = await httpClient.patch<TaskResponse>(
      API_CONFIG.baseUrls.task,
      `/tasks/${id}/complete`
    )
    return transformTask(response)
  },

  async reorder(taskIds: string[]): Promise<Task[]> {
    const response = await httpClient.post<TaskResponse[]>(
      API_CONFIG.baseUrls.task,
      '/tasks/reorder',
      { taskIds }
    )
    return response.map(transformTask)
  },

  async bulkDelete(taskIds: string[]): Promise<void> {
    await httpClient.post(
      API_CONFIG.baseUrls.task,
      '/tasks/bulk-delete',
      { taskIds }
    )
  },

  async bulkComplete(taskIds: string[], completed: boolean): Promise<Task[]> {
    const response = await httpClient.post<TaskResponse[]>(
      API_CONFIG.baseUrls.task,
      '/tasks/bulk-complete',
      { taskIds, completed }
    )
    return response.map(transformTask)
  },
}))

// ============================================
// EntityMemo API
// ============================================
interface EntityMemoResponse {
  id: string
  userId: string
  entityType: EntityType
  entityId: string
  content: string
  pinned: boolean
  createdAt: string
  updatedAt: string
}

const transformEntityMemo = (m: EntityMemoResponse): EntityMemo => ({
  id: m.id,
  userId: m.userId,
  entityType: m.entityType,
  entityId: m.entityId,
  content: m.content,
  pinned: m.pinned,
  createdAt: new Date(m.createdAt),
  updatedAt: new Date(m.updatedAt),
})

export interface CreateEntityMemoInput {
  entityType: EntityType
  entityId: string
  content: string
  pinned?: boolean
}

export interface UpdateEntityMemoInput {
  content?: string
  pinned?: boolean
}

export interface EntityMemoFilter {
  entityType?: EntityType
  entityId?: string
  pinned?: boolean
}

export const entityMemosApi = createApiService<
  EntityMemoResponse,
  EntityMemo,
  CreateEntityMemoInput,
  UpdateEntityMemoInput,
  EntityMemoFilter
>(
  {
    baseUrl: API_CONFIG.baseUrls.task,
    resourcePath: '/entity-memos',
    transform: transformEntityMemo,
  },
  {
    filterMappings: {
      entityType: 'entity_type',
      entityId: 'entity_id',
    },
  }
)

// ============================================
// Todo API
// ============================================
interface TodoResponse {
  id: string
  userId: string
  name: string
  frequency?: TodoFrequency
  daysOfWeek?: number[]
  dueDate?: string
  estimatedMinutes?: number
  tagIds?: string[]
  enabled: boolean
  createdAt: string
  updatedAt: string
}

interface TodoWithStatusResponse extends TodoResponse {
  completedToday: boolean
  completedAt?: string
  isOverdue: boolean
}

const transformTodo = (t: TodoResponse): Todo => ({
  id: t.id,
  userId: t.userId,
  name: t.name,
  frequency: t.frequency,
  daysOfWeek: t.daysOfWeek,
  dueDate: t.dueDate,
  estimatedMinutes: t.estimatedMinutes,
  tagIds: t.tagIds,
  enabled: t.enabled,
  createdAt: new Date(t.createdAt),
  updatedAt: new Date(t.updatedAt),
})

const transformTodoWithStatus = (t: TodoWithStatusResponse): TodoWithStatus => ({
  ...transformTodo(t),
  completedToday: t.completedToday,
  completedAt: t.completedAt ? new Date(t.completedAt) : undefined,
  isOverdue: t.isOverdue,
})

export interface CreateTodoInput {
  name: string
  frequency?: TodoFrequency
  daysOfWeek?: number[]
  dueDate?: string
  estimatedMinutes?: number
  tagIds?: string[]
}

export interface UpdateTodoInput {
  name?: string
  frequency?: TodoFrequency
  daysOfWeek?: number[]
  dueDate?: string
  estimatedMinutes?: number
  tagIds?: string[]
  enabled?: boolean
}

export interface TodoFilter {
  enabled?: boolean
  isRecurring?: boolean
}

const baseTodosApi = createApiService<
  TodoResponse,
  Todo,
  CreateTodoInput,
  UpdateTodoInput,
  TodoFilter
>(
  {
    baseUrl: API_CONFIG.baseUrls.task,
    resourcePath: '/todos',
    transform: transformTodo,
  },
  {
    filterMappings: {
      isRecurring: 'is_recurring',
    },
  }
)

export const todosApi = extendApiService(baseTodosApi, () => ({
  // Get todos with completion status for a specific date
  async listWithStatus(date: string): Promise<TodoWithStatus[]> {
    const response = await httpClient.get<TodoWithStatusResponse[]>(
      API_CONFIG.baseUrls.task,
      `/todos?date=${date}`
    )
    return response.map(transformTodoWithStatus)
  },

  // Toggle completion status for a todo on a specific date
  async toggleComplete(todoId: string, date: string): Promise<TodoWithStatus> {
    const response = await httpClient.patch<TodoWithStatusResponse>(
      API_CONFIG.baseUrls.task,
      `/todos/${todoId}/complete?date=${date}`
    )
    return transformTodoWithStatus(response)
  },
}))

// ============================================
// 後方互換性のための型エイリアス (移行期間中)
// ============================================
/** @deprecated Use goalsApi instead */
export const projectsApi = {
  list: goalsApi.list,
  get: goalsApi.get,
  create: async (input: { name: string; goalTag?: string; color?: string }) => {
    return goalsApi.create({
      name: input.name,
      color: input.color || '#0EA5E9',
    })
  },
  update: goalsApi.update,
  delete: goalsApi.delete,
}
