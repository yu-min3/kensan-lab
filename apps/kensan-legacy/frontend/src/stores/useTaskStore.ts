import { create } from 'zustand'
import type { Task, TaskFrequency } from '@/types'
import { tasksApi } from '@/api/services/tasks'

// ============================================
// Task-only store (new single responsibility)
// ============================================
interface TaskOnlyState {
  tasks: Task[]
  isLoading: boolean
  error: string | null

  // データ取得
  fetchTasks: () => Promise<void>

  // Task操作
  addTask: (task: {
    name: string
    milestoneId?: string
    parentTaskId?: string
    tagIds?: string[]
    estimatedMinutes?: number
    dueDate?: string
    frequency?: TaskFrequency
    daysOfWeek?: number[]
  }) => Promise<Task | undefined>
  updateTask: (id: string, updates: Partial<Task>) => Promise<void>
  deleteTask: (id: string) => Promise<void>
  toggleTaskComplete: (id: string) => Promise<void>
  reorderTasks: (taskIds: string[]) => Promise<void>
  bulkDeleteTasks: (taskIds: string[]) => Promise<void>
  bulkCompleteTasks: (taskIds: string[], completed: boolean) => Promise<void>

  // 取得（同期）
  getTaskById: (id: string) => Task | undefined
  getTasksByMilestone: (milestoneId: string) => Task[]
  getChildTasks: (parentTaskId: string) => Task[]
  getStandaloneTasks: () => Task[]

  // ユーティリティ
  clearError: () => void
}

export const useTaskOnlyStore = create<TaskOnlyState>((set, get) => ({
  tasks: [],
  isLoading: false,
  error: null,

  fetchTasks: async () => {
    set({ isLoading: true, error: null })
    try {
      const tasks = await tasksApi.list()
      set({ tasks, isLoading: false })
    } catch (error) {
      set({ error: (error as Error).message, isLoading: false })
    }
  },

  addTask: async (task) => {
    try {
      const newTask = await tasksApi.create(task)
      set((state) => ({ tasks: [...state.tasks, newTask] }))
      return newTask
    } catch (error) {
      set({ error: (error as Error).message })
      return undefined
    }
  },

  updateTask: async (id, updates) => {
    try {
      const updatedTask = await tasksApi.update(id, updates)
      set((state) => ({
        tasks: state.tasks.map((t) => (t.id === id ? updatedTask : t)),
      }))
    } catch (error) {
      set({ error: (error as Error).message })
    }
  },

  deleteTask: async (id) => {
    try {
      await tasksApi.delete(id)
      set((state) => ({
        tasks: state.tasks.filter((t) => t.id !== id && t.parentTaskId !== id),
      }))
    } catch (error) {
      set({ error: (error as Error).message })
    }
  },

  toggleTaskComplete: async (id) => {
    try {
      const updatedTask = await tasksApi.toggleComplete(id)
      set((state) => ({
        tasks: state.tasks.map((t) => (t.id === id ? updatedTask : t)),
      }))
    } catch (error) {
      set({ error: (error as Error).message })
    }
  },

  reorderTasks: async (taskIds) => {
    try {
      const updatedTasks = await tasksApi.reorder(taskIds)
      set((state) => ({
        tasks: state.tasks.map((t) => {
          const updated = updatedTasks.find((u) => u.id === t.id)
          return updated || t
        }),
      }))
    } catch (error) {
      set({ error: (error as Error).message })
    }
  },

  bulkDeleteTasks: async (taskIds) => {
    try {
      await tasksApi.bulkDelete(taskIds)
      const idsToDelete = new Set(taskIds)
      set((state) => ({
        tasks: state.tasks.filter((t) => !idsToDelete.has(t.id) && !idsToDelete.has(t.parentTaskId || '')),
      }))
    } catch (error) {
      set({ error: (error as Error).message })
    }
  },

  bulkCompleteTasks: async (taskIds, completed) => {
    try {
      const updatedTasks = await tasksApi.bulkComplete(taskIds, completed)
      set((state) => ({
        tasks: state.tasks.map((t) => {
          const updated = updatedTasks.find((u) => u.id === t.id)
          return updated || t
        }),
      }))
    } catch (error) {
      set({ error: (error as Error).message })
    }
  },

  getTaskById: (id) => get().tasks.find((t) => t.id === id),
  getTasksByMilestone: (milestoneId) =>
    get().tasks.filter((t) => t.milestoneId === milestoneId && !t.parentTaskId),
  getChildTasks: (parentTaskId) => get().tasks.filter((t) => t.parentTaskId === parentTaskId),
  getStandaloneTasks: () => get().tasks.filter((t) => !t.milestoneId && !t.parentTaskId),
  clearError: () => set({ error: null }),
}))
