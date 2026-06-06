import { create } from 'zustand'
import type { TimeBlock, TimeEntry } from '@/types'
import { timeblocksApi, timeentriesApi } from '@/api/services/timeblocks'
import { useSettingsStore } from './useSettingsStore'
import { getLocalDate } from '@/lib/timezone'

interface TimeBlockState {
  timeBlocks: TimeBlock[]
  timeEntries: TimeEntry[]
  isLoading: boolean
  error: string | null

  // データ取得 (timezone-aware)
  fetchTimeBlocksForLocalDate: (localDate: string, timezone: string) => Promise<void>
  fetchTimeEntriesForLocalDate: (localDate: string, timezone: string) => Promise<void>
  fetchTimeBlocksRange: (startDate: string, endDate: string) => Promise<void>
  fetchTimeEntriesRange: (startDate: string, endDate: string) => Promise<void>

  // タイムブロック操作 (local date/time → UTC conversion)
  addTimeBlock: (localStartDate: string, localStartTime: string, localEndDate: string, localEndTime: string, data: {
    taskId?: string
    taskName: string
    milestoneId?: string
    milestoneName?: string
    goalId?: string
    goalName?: string
    goalColor?: string
    tagIds?: string[]
  }) => Promise<void>
  updateTimeBlock: (id: string, localStartDate: string, localStartTime?: string, localEndDate?: string, localEndTime?: string, data?: {
    taskId?: string
    taskName?: string
    milestoneId?: string
    milestoneName?: string
    goalId?: string
    goalName?: string
    goalColor?: string
  }) => Promise<void>
  deleteTimeBlock: (id: string) => Promise<void>

  // 時間記録操作（local date/time → UTC conversion）
  addTimeEntry: (localStartDate: string, localStartTime: string, localEndDate: string, localEndTime: string, data: {
    taskId?: string
    taskName: string
    milestoneId?: string
    milestoneName?: string
    goalId?: string
    goalName?: string
    goalColor?: string
    tagIds?: string[]
    description?: string
  }) => Promise<void>
  updateTimeEntry: (id: string, localStartDate: string, localStartTime?: string, localEndDate?: string, localEndTime?: string, data?: {
    taskId?: string
    taskName?: string
    milestoneId?: string
    milestoneName?: string
    goalId?: string
    goalName?: string
    goalColor?: string
    description?: string
  }) => Promise<void>
  deleteTimeEntry: (id: string) => Promise<void>

  // 取得 (filter by local date using timezone conversion)
  getTimeBlocksByDate: (localDate: string) => TimeBlock[]
  getTimeEntriesByDate: (localDate: string) => TimeEntry[]
  getTodayTimeBlocks: () => TimeBlock[]
  getTodayTimeEntries: () => TimeEntry[]

  // ローカルにエントリを追加（API呼び出しなし、タイマー停止時に使用）
  insertTimeEntryLocal: (entry: TimeEntry) => void
}

// Helper to get current timezone from settings store
const getTimezone = (): string => {
  return useSettingsStore.getState().timezone || 'Asia/Tokyo'
}

export const useTimeBlockStore = create<TimeBlockState>((set, get) => ({
  timeBlocks: [],
  timeEntries: [],
  isLoading: false,
  error: null,

  // Timezone-aware fetch: converts local date to UTC range for proper querying
  fetchTimeBlocksForLocalDate: async (localDate, timezone) => {
    set({ isLoading: true, error: null })
    try {
      const timeBlocks = await timeblocksApi.listByLocalDate(localDate, timezone)
      set({ timeBlocks, isLoading: false })
    } catch (error) {
      set({ error: (error as Error).message, isLoading: false })
    }
  },

  fetchTimeEntriesForLocalDate: async (localDate, timezone) => {
    set({ isLoading: true, error: null })
    try {
      const timeEntries = await timeentriesApi.listByLocalDate(localDate, timezone)
      set({ timeEntries, isLoading: false })
    } catch (error) {
      set({ error: (error as Error).message, isLoading: false })
    }
  },

  fetchTimeBlocksRange: async (startDate, endDate) => {
    set({ isLoading: true, error: null })
    try {
      const timezone = getTimezone()
      const timeBlocks = await timeblocksApi.listByDateRange(startDate, endDate, timezone)
      set({ timeBlocks, isLoading: false })
    } catch (error) {
      set({ error: (error as Error).message, isLoading: false })
    }
  },

  addTimeBlock: async (localStartDate, localStartTime, localEndDate, localEndTime, data) => {
    try {
      const timezone = getTimezone()
      const newBlock = await timeblocksApi.createFromLocal(
        localStartDate, localStartTime, localEndDate, localEndTime, data, timezone
      )
      set((state) => ({
        timeBlocks: [...state.timeBlocks, newBlock],
      }))
    } catch (error) {
      set({ error: (error as Error).message })
    }
  },

  updateTimeBlock: async (id, localStartDate, localStartTime, localEndDate, localEndTime, data) => {
    try {
      const timezone = getTimezone()
      const updatedBlock = await timeblocksApi.updateFromLocal(
        id, localStartDate, localStartTime, localEndDate, localEndTime, data || {}, timezone
      )
      set((state) => ({
        timeBlocks: state.timeBlocks.map((b) =>
          b.id === id ? updatedBlock : b
        ),
      }))
    } catch (error) {
      set({ error: (error as Error).message })
    }
  },

  deleteTimeBlock: async (id) => {
    try {
      await timeblocksApi.delete(id)
      set((state) => ({
        timeBlocks: state.timeBlocks.filter((b) => b.id !== id),
      }))
    } catch (error) {
      set({ error: (error as Error).message })
    }
  },

  fetchTimeEntriesRange: async (startDate, endDate) => {
    set({ isLoading: true, error: null })
    try {
      const timezone = getTimezone()
      const timeEntries = await timeentriesApi.listByDateRange(startDate, endDate, timezone)
      set({ timeEntries, isLoading: false })
    } catch (error) {
      set({ error: (error as Error).message, isLoading: false })
    }
  },

  // TimeEntry CRUD operations (timezone-aware)
  addTimeEntry: async (localStartDate, localStartTime, localEndDate, localEndTime, data) => {
    try {
      const timezone = getTimezone()
      const newEntry = await timeentriesApi.createFromLocal(
        localStartDate, localStartTime, localEndDate, localEndTime, data, timezone
      )
      set((state) => ({
        timeEntries: [...state.timeEntries, newEntry],
      }))
    } catch (error) {
      set({ error: (error as Error).message })
    }
  },

  updateTimeEntry: async (id, localStartDate, localStartTime, localEndDate, localEndTime, data) => {
    try {
      const timezone = getTimezone()
      const updatedEntry = await timeentriesApi.updateFromLocal(
        id, localStartDate, localStartTime, localEndDate, localEndTime, data || {}, timezone
      )
      set((state) => ({
        timeEntries: state.timeEntries.map((e) =>
          e.id === id ? updatedEntry : e
        ),
      }))
    } catch (error) {
      set({ error: (error as Error).message })
    }
  },

  deleteTimeEntry: async (id) => {
    try {
      await timeentriesApi.delete(id)
      set((state) => ({
        timeEntries: state.timeEntries.filter((e) => e.id !== id),
      }))
    } catch (error) {
      set({ error: (error as Error).message })
    }
  },

  getTimeBlocksByDate: (localDate) => {
    const timezone = getTimezone()
    return get().timeBlocks.filter((b) => getLocalDate(b.startDatetime, timezone) === localDate)
  },

  getTimeEntriesByDate: (localDate) => {
    const timezone = getTimezone()
    return get().timeEntries.filter((e) => getLocalDate(e.startDatetime, timezone) === localDate)
  },

  getTodayTimeBlocks: () => {
    // When using timezone-aware fetch (fetchTimeBlocksForLocalDate), the store
    // already contains only the data for the requested local date range.
    // Return all entries without additional filtering.
    return get().timeBlocks
  },

  getTodayTimeEntries: () => {
    // When using timezone-aware fetch (fetchTimeEntriesForLocalDate), the store
    // already contains only the data for the requested local date range.
    // Return all entries without additional filtering.
    return get().timeEntries
  },

  // Insert a time entry locally without API call (used when timer stops)
  insertTimeEntryLocal: (entry) => {
    set((state) => ({
      timeEntries: [...state.timeEntries, entry],
    }))
  },
}))
