import { createCrudStore } from './createCrudStore'
import { goalsApi } from '@/api/services/tasks'
import type { Goal } from '@/types'
import type { CreateGoalInput, UpdateGoalInput, GoalFilter } from '@/api/services/tasks'

interface GoalExtensions {
  reorderGoals: (goalIds: string[]) => Promise<void>
}

export const useGoalStore = createCrudStore<
  Goal,
  CreateGoalInput,
  UpdateGoalInput,
  GoalFilter,
  GoalExtensions
>(
  {
    api: goalsApi,
    getId: (item) => item.id,
    prependOnAdd: false,
  },
  (set, get) => ({
    reorderGoals: async (goalIds: string[]) => {
      try {
        const updatedGoals = await goalsApi.reorder(goalIds)
        set({
          items: get().items.map((g) => {
            const updated = updatedGoals.find((u) => u.id === g.id)
            return updated || g
          }).sort((a, b) => a.sortOrder - b.sortOrder),
        })
      } catch (error) {
        set({ error: (error as Error).message })
      }
    },
  })
)
