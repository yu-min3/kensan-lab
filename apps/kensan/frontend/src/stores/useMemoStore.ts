import { create } from 'zustand'
import { memosApi, type Memo } from '@/api/services/memos'

interface MemoState {
  memos: Memo[]
  isLoading: boolean
  error: string | null

  // Actions
  fetchMemos: (options?: { includeAll?: boolean; date?: string }) => Promise<void>
  addMemo: (content: string) => Promise<Memo | null>
  updateMemo: (id: string, content: string) => Promise<boolean>
  archiveMemo: (id: string) => Promise<void>
  deleteMemo: (id: string) => Promise<void>

  // Getters
  getActiveMemos: () => Memo[]
  getTodayMemos: () => Memo[]
}

export const useMemoStore = create<MemoState>((set, get) => ({
  memos: [],
  isLoading: false,
  error: null,

  fetchMemos: async (options) => {
    set({ isLoading: true, error: null })
    try {
      const memos = await memosApi.list({
        includeAll: options?.includeAll,
        date: options?.date,
      })
      set({ memos, isLoading: false })
    } catch (error) {
      set({ error: (error as Error).message, isLoading: false })
    }
  },

  addMemo: async (content) => {
    try {
      const newMemo = await memosApi.create({ content })
      set((state) => ({
        memos: [newMemo, ...state.memos],
      }))
      return newMemo
    } catch (error) {
      set({ error: (error as Error).message })
      return null
    }
  },

  updateMemo: async (id, content) => {
    try {
      const updatedMemo = await memosApi.update(id, { content })
      set((state) => ({
        memos: state.memos.map((m) => (m.id === id ? updatedMemo : m)),
      }))
      return true
    } catch (error) {
      set({ error: (error as Error).message })
      return false
    }
  },

  archiveMemo: async (id) => {
    try {
      const updatedMemo = await memosApi.archive(id)
      set((state) => ({
        memos: state.memos.map((m) => (m.id === id ? updatedMemo : m)),
      }))
    } catch (error) {
      set({ error: (error as Error).message })
    }
  },

  deleteMemo: async (id) => {
    try {
      await memosApi.delete(id)
      set((state) => ({
        memos: state.memos.filter((m) => m.id !== id),
      }))
    } catch (error) {
      set({ error: (error as Error).message })
    }
  },

  getActiveMemos: () => {
    return get().memos.filter((m) => !m.archived)
  },

  getTodayMemos: () => {
    const today = new Date().toISOString().split('T')[0]
    return get().memos.filter((m) => {
      const memoDate = new Date(m.createdAt).toISOString().split('T')[0]
      return memoDate === today && !m.archived
    })
  },
}))
