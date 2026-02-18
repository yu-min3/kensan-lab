import { createCrudStore } from './createCrudStore'
import { tagsApi } from '@/api/services/tasks'
import type { Tag } from '@/types'
import type { CreateTagInput, UpdateTagInput } from '@/api/services/tasks'

interface TagExtensions {
  getTagsByIds: (ids: string[]) => Tag[]
}

export const useTagStore = createCrudStore<
  Tag,
  CreateTagInput,
  UpdateTagInput,
  Record<string, never>,
  TagExtensions
>(
  {
    api: tagsApi,
    getId: (item) => item.id,
    prependOnAdd: false,
  },
  (_set, get) => ({
    getTagsByIds: (ids: string[]) =>
      get().items.filter((t) => ids.includes(t.id)),
  })
)
