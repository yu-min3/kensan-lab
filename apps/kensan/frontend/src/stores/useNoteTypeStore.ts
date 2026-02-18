import { create } from 'zustand'
import type { NoteTypeConfig, TypeConstraints, FieldSchema } from '@/types'
import { noteTypesApi } from '@/api/services/notes'

interface NoteTypeState {
  types: NoteTypeConfig[]
  isLoaded: boolean
  isLoading: boolean
  error: string | null
}

interface NoteTypeActions {
  fetchTypes: () => Promise<void>
  getBySlug: (slug: string) => NoteTypeConfig | undefined
  getDisplayName: (slug: string) => string
  getIcon: (slug: string) => string
  getColor: (slug: string) => string
  getConstraints: (slug: string) => TypeConstraints | undefined
  getMetadataSchema: (slug: string) => FieldSchema[]
}

export type NoteTypeStore = NoteTypeState & NoteTypeActions

export const useNoteTypeStore = create<NoteTypeStore>()((set, get) => ({
  types: [],
  isLoaded: false,
  isLoading: false,
  error: null,

  fetchTypes: async () => {
    // Skip if already loaded
    if (get().isLoaded) return

    set({ isLoading: true, error: null })
    try {
      const types = await noteTypesApi.list()
      // Sort by sortOrder
      types.sort((a, b) => a.sortOrder - b.sortOrder)
      set({ types, isLoaded: true, isLoading: false })
    } catch (error) {
      set({ error: (error as Error).message, isLoading: false })
    }
  },

  getBySlug: (slug: string) => {
    return get().types.find((t) => t.slug === slug)
  },

  getDisplayName: (slug: string) => {
    const type = get().types.find((t) => t.slug === slug)
    return type?.displayName ?? slug
  },

  getIcon: (slug: string) => {
    const type = get().types.find((t) => t.slug === slug)
    return type?.icon ?? 'file-text'
  },

  getColor: (slug: string) => {
    const type = get().types.find((t) => t.slug === slug)
    return type?.color ?? '#6B7280'
  },

  getConstraints: (slug: string) => {
    const type = get().types.find((t) => t.slug === slug)
    return type?.constraints
  },

  getMetadataSchema: (slug: string) => {
    const type = get().types.find((t) => t.slug === slug)
    return type?.metadataSchema ?? []
  },
}))
