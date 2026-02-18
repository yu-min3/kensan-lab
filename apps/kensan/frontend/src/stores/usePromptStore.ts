import { create } from 'zustand'
import {
  fetchContexts,
  fetchVersions,
  updateContext,
  rollbackToVersion,
  fetchMetadata,
  type AIContext,
  type AIContextVersion,
  type AIContextUpdateInput,
  type PromptMetadata,
} from '@/api/services/prompts'

interface PromptStore {
  contexts: AIContext[]
  versions: Record<string, AIContextVersion[]>
  metadata: PromptMetadata | null
  isLoading: boolean
  error: string | null

  fetchContexts: (situation?: string) => Promise<void>
  updateContext: (id: string, data: AIContextUpdateInput) => Promise<void>
  fetchVersions: (contextId: string) => Promise<void>
  rollback: (contextId: string, versionNumber: number) => Promise<void>
  fetchMetadata: () => Promise<void>
}

export const usePromptStore = create<PromptStore>()((set, get) => ({
  contexts: [],
  versions: {},
  metadata: null,
  isLoading: false,
  error: null,

  fetchContexts: async (situation?: string) => {
    set({ isLoading: true, error: null })
    try {
      const contexts = await fetchContexts(situation)
      set({ contexts, isLoading: false })
    } catch (e) {
      set({ error: (e as Error).message, isLoading: false })
    }
  },

  updateContext: async (id: string, data: AIContextUpdateInput) => {
    set({ isLoading: true, error: null })
    try {
      const updated = await updateContext(id, data)
      set((state) => ({
        contexts: state.contexts.map((c) => (c.id === id ? updated : c)),
        isLoading: false,
      }))
    } catch (e) {
      set({ error: (e as Error).message, isLoading: false })
    }
  },

  fetchVersions: async (contextId: string) => {
    try {
      const versions = await fetchVersions(contextId)
      set((state) => ({
        versions: { ...state.versions, [contextId]: versions },
      }))
    } catch (e) {
      set({ error: (e as Error).message })
    }
  },

  rollback: async (contextId: string, versionNumber: number) => {
    set({ isLoading: true, error: null })
    try {
      const updated = await rollbackToVersion(contextId, versionNumber)
      set((state) => ({
        contexts: state.contexts.map((c) => (c.id === contextId ? updated : c)),
        isLoading: false,
      }))
      // Refresh versions after rollback
      await get().fetchVersions(contextId)
    } catch (e) {
      set({ error: (e as Error).message, isLoading: false })
    }
  },

  fetchMetadata: async () => {
    // Cache: only fetch once
    if (get().metadata) return
    try {
      const metadata = await fetchMetadata()
      set({ metadata })
    } catch (e) {
      set({ error: (e as Error).message })
    }
  },
}))
