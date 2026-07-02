import { createCrudStore } from './createCrudStore'
import { milestonesApi } from '@/api/services/tasks'
import type { Milestone } from '@/types'
import type { CreateMilestoneInput, UpdateMilestoneInput, MilestoneFilter } from '@/api/services/tasks'

interface MilestoneExtensions {
  getMilestonesByGoal: (goalId: string) => Milestone[]
}

export const useMilestoneStore = createCrudStore<
  Milestone,
  CreateMilestoneInput,
  UpdateMilestoneInput,
  MilestoneFilter,
  MilestoneExtensions
>(
  {
    api: milestonesApi,
    getId: (item) => item.id,
    prependOnAdd: false,
  },
  (_set, get) => ({
    getMilestonesByGoal: (goalId: string) =>
      get().items.filter((m) => m.goalId === goalId),
  })
)
