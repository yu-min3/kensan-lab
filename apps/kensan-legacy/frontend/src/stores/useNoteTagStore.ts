import { create } from 'zustand'
import { noteTagsApi, type CreateNoteTagInput, type UpdateNoteTagInput } from '@/api/services/notes'
import type { Tag } from '@/types'

interface NoteTagState {
  items: Tag[]
  isLoading: boolean
  error: string | null
  fetchAll: () => Promise<void>
  add: (input: CreateNoteTagInput) => Promise<Tag>
  update: (id: string, input: UpdateNoteTagInput) => Promise<Tag>
  remove: (id: string) => Promise<void>
  getById: (id: string) => Tag | undefined
  getTagsByIds: (ids: string[]) => Tag[]
}

export const useNoteTagStore = create<NoteTagState>((set, get) => ({
  items: [],
  isLoading: false,
  error: null,

  fetchAll: async () => {
    set({ isLoading: true, error: null })
    try {
      const items = await noteTagsApi.list()
      set({ items, isLoading: false })
    } catch (err) {
      set({ error: (err as Error).message, isLoading: false })
    }
  },

  add: async (input: CreateNoteTagInput) => {
    const tag = await noteTagsApi.create(input)
    set((state) => ({ items: [...state.items, tag] }))
    return tag
  },

  update: async (id: string, input: UpdateNoteTagInput) => {
    const tag = await noteTagsApi.update(id, input)
    set((state) => ({
      items: state.items.map((t) => (t.id === id ? tag : t)),
    }))
    return tag
  },

  remove: async (id: string) => {
    await noteTagsApi.delete(id)
    set((state) => ({
      items: state.items.filter((t) => t.id !== id),
    }))
  },

  getById: (id: string) => get().items.find((t) => t.id === id),

  getTagsByIds: (ids: string[]) => get().items.filter((t) => ids.includes(t.id)),
}))
