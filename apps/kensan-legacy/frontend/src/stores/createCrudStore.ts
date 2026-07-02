// Generic CRUD Store Factory
// Creates standardized Zustand stores with common CRUD operations

import { create, StateCreator } from 'zustand'
import type { CrudApi } from '@/api/createApiService'

/**
 * Base state for CRUD stores
 */
export interface CrudState<T> {
  items: T[]
  isLoading: boolean
  error: string | null
}

/**
 * Base actions for CRUD stores
 */
export interface CrudActions<T, TCreateInput, TUpdateInput> {
  fetchAll: () => Promise<void>
  add: (data: TCreateInput) => Promise<T>
  update: (id: string, data: TUpdateInput) => Promise<T>
  remove: (id: string) => Promise<void>
  getById: (id: string) => T | undefined
  clearError: () => void
}

export type CrudStore<T, TCreateInput, TUpdateInput> =
  CrudState<T> & CrudActions<T, TCreateInput, TUpdateInput>

/**
 * Configuration for creating a CRUD store
 */
export interface CrudStoreConfig<T, TCreateInput, TUpdateInput, TFilter> {
  /** API service with CRUD operations */
  api: CrudApi<T, TCreateInput, TUpdateInput, TFilter>
  /** Function to get the ID from an entity */
  getId: (item: T) => string
  /** Whether to prepend new items to the list (default: true) */
  prependOnAdd?: boolean
}

/**
 * Creates a base CRUD store slice
 *
 * @example
 * ```ts
 * const useItemStore = create<ItemState>()(
 *   createCrudSlice({
 *     api: itemsApi,
 *     getId: (item) => item.id,
 *   })
 * )
 * ```
 */
export function createCrudSlice<
  T extends { id: string },
  TCreateInput,
  TUpdateInput,
  TFilter = unknown
>(
  config: CrudStoreConfig<T, TCreateInput, TUpdateInput, TFilter>
): StateCreator<CrudStore<T, TCreateInput, TUpdateInput>> {
  const { api, getId, prependOnAdd = true } = config

  return (set, get) => ({
    items: [],
    isLoading: false,
    error: null,

    fetchAll: async () => {
      set({ isLoading: true, error: null })
      try {
        const items = await api.list()
        set({ items, isLoading: false })
      } catch (error) {
        set({ error: (error as Error).message, isLoading: false })
      }
    },

    add: async (data: TCreateInput) => {
      try {
        const newItem = await api.create(data)
        set((state) => ({
          items: prependOnAdd
            ? [newItem, ...state.items]
            : [...state.items, newItem],
        }))
        return newItem
      } catch (error) {
        set({ error: (error as Error).message })
        throw error
      }
    },

    update: async (id: string, data: TUpdateInput) => {
      try {
        const updatedItem = await api.update(id, data)
        set((state) => ({
          items: state.items.map((item) =>
            getId(item) === id ? updatedItem : item
          ),
        }))
        return updatedItem
      } catch (error) {
        set({ error: (error as Error).message })
        throw error
      }
    },

    remove: async (id: string) => {
      try {
        await api.delete(id)
        set((state) => ({
          items: state.items.filter((item) => getId(item) !== id),
        }))
      } catch (error) {
        set({ error: (error as Error).message })
        throw error
      }
    },

    getById: (id: string) => get().items.find((item) => getId(item) === id),

    clearError: () => set({ error: null }),
  })
}

/**
 * Creates a CRUD store with additional custom state and actions
 *
 * @example
 * ```ts
 * interface ItemExtensions {
 *   getByDate: (date: string) => Item | undefined
 * }
 *
 * const useItemStore = createCrudStore<Item, CreateInput, UpdateInput, ItemFilter, ItemExtensions>(
 *   {
 *     api: itemsApi,
 *     getId: (item) => item.id,
 *   },
 *   (set, get, baseActions) => ({
 *     getByDate: (date) => get().items.find((e) => e.date === date),
 *   })
 * )
 * ```
 */
export function createCrudStore<
  T extends { id: string },
  TCreateInput,
  TUpdateInput,
  TFilter = unknown,
  TExtensions = Record<string, never>
>(
  config: CrudStoreConfig<T, TCreateInput, TUpdateInput, TFilter>,
  extensions?: (
    set: (partial: Partial<CrudState<T>>) => void,
    get: () => CrudStore<T, TCreateInput, TUpdateInput> & TExtensions,
    base: CrudStore<T, TCreateInput, TUpdateInput>
  ) => TExtensions
) {
  type StoreState = CrudStore<T, TCreateInput, TUpdateInput> & TExtensions

  return create<StoreState>()((set, get) => {
    const baseSlice = createCrudSlice(config)(
      set as Parameters<StateCreator<CrudStore<T, TCreateInput, TUpdateInput>>>[0],
      get as Parameters<StateCreator<CrudStore<T, TCreateInput, TUpdateInput>>>[1],
      undefined as never
    )

    const extensionSlice = extensions
      ? extensions(
          set as (partial: Partial<CrudState<T>>) => void,
          get as () => StoreState,
          baseSlice
        )
      : ({} as TExtensions)

    return {
      ...baseSlice,
      ...extensionSlice,
    } as StoreState
  })
}
