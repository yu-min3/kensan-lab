/**
 * useTaskManagerStore - Combined hook for backward compatibility
 *
 * This hook provides a unified interface to the separated domain stores
 * (useGoalStore, useMilestoneStore, useTagStore, useTaskOnlyStore).
 *
 * Use this hook when:
 * - Migrating from the old useTaskStore
 * - You need access to multiple domains in a single component
 *
 * Prefer using individual stores directly when possible for better performance.
 */

import { useGoalStore } from './useGoalStore'
import { useMilestoneStore } from './useMilestoneStore'
import { useTagStore } from './useTagStore'
import { useTaskOnlyStore } from './useTaskStore'

/**
 * Combined hook that aggregates all task-related stores
 * Provides a similar API to the original useTaskStore
 */
export function useTaskManagerStore() {
  // Goal store (uses createCrudStore generic names)
  const goals = useGoalStore((state) => state.items)
  const goalsLoading = useGoalStore((state) => state.isLoading)
  const goalsError = useGoalStore((state) => state.error)
  const fetchGoals = useGoalStore((state) => state.fetchAll)
  const addGoal = useGoalStore((state) => state.add)
  const updateGoal = useGoalStore((state) => state.update)
  const deleteGoal = useGoalStore((state) => state.remove)
  const getGoalById = useGoalStore((state) => state.getById)
  const reorderGoals = useGoalStore((state) => state.reorderGoals)

  // Milestone store (uses createCrudStore generic names)
  const milestones = useMilestoneStore((state) => state.items)
  const milestonesLoading = useMilestoneStore((state) => state.isLoading)
  const milestonesError = useMilestoneStore((state) => state.error)
  const fetchMilestones = useMilestoneStore((state) => state.fetchAll)
  const addMilestone = useMilestoneStore((state) => state.add)
  const updateMilestone = useMilestoneStore((state) => state.update)
  const deleteMilestone = useMilestoneStore((state) => state.remove)
  const getMilestoneById = useMilestoneStore((state) => state.getById)
  const getMilestonesByGoal = useMilestoneStore((state) => state.getMilestonesByGoal)

  // Tag store (uses createCrudStore generic names)
  const tags = useTagStore((state) => state.items)
  const tagsLoading = useTagStore((state) => state.isLoading)
  const tagsError = useTagStore((state) => state.error)
  const fetchTags = useTagStore((state) => state.fetchAll)
  const addTag = useTagStore((state) => state.add)
  const updateTag = useTagStore((state) => state.update)
  const deleteTag = useTagStore((state) => state.remove)
  const getTagById = useTagStore((state) => state.getById)
  const getTagsByIds = useTagStore((state) => state.getTagsByIds)

  // Task store
  const tasks = useTaskOnlyStore((state) => state.tasks)
  const tasksLoading = useTaskOnlyStore((state) => state.isLoading)
  const tasksError = useTaskOnlyStore((state) => state.error)
  const fetchTasks = useTaskOnlyStore((state) => state.fetchTasks)
  const addTask = useTaskOnlyStore((state) => state.addTask)
  const updateTask = useTaskOnlyStore((state) => state.updateTask)
  const deleteTask = useTaskOnlyStore((state) => state.deleteTask)
  const toggleTaskComplete = useTaskOnlyStore((state) => state.toggleTaskComplete)
  const reorderTasks = useTaskOnlyStore((state) => state.reorderTasks)
  const bulkDeleteTasks = useTaskOnlyStore((state) => state.bulkDeleteTasks)
  const bulkCompleteTasks = useTaskOnlyStore((state) => state.bulkCompleteTasks)
  const getTaskById = useTaskOnlyStore((state) => state.getTaskById)
  const getTasksByMilestone = useTaskOnlyStore((state) => state.getTasksByMilestone)
  const getChildTasks = useTaskOnlyStore((state) => state.getChildTasks)
  const getStandaloneTasks = useTaskOnlyStore((state) => state.getStandaloneTasks)

  // Aggregate loading and error states
  const isLoading = goalsLoading || milestonesLoading || tagsLoading || tasksLoading
  const error = goalsError || milestonesError || tagsError || tasksError

  // Fetch all data
  const fetchAll = async () => {
    await Promise.all([fetchGoals(), fetchMilestones(), fetchTags(), fetchTasks()])
  }

  return {
    // Data
    goals,
    milestones,
    tags,
    tasks,
    isLoading,
    error,

    // Fetch
    fetchAll,
    fetchGoals,
    fetchMilestones,
    fetchTags,
    fetchTasks,

    // Goal operations
    addGoal,
    updateGoal,
    deleteGoal,
    reorderGoals,
    getGoalById,

    // Milestone operations
    addMilestone,
    updateMilestone,
    deleteMilestone,
    getMilestoneById,
    getMilestonesByGoal,

    // Tag operations
    addTag,
    updateTag,
    deleteTag,
    getTagById,
    getTagsByIds,

    // Task operations
    addTask,
    updateTask,
    deleteTask,
    toggleTaskComplete,
    reorderTasks,
    bulkDeleteTasks,
    bulkCompleteTasks,
    getTaskById,
    getTasksByMilestone,
    getChildTasks,
    getStandaloneTasks,

    // Legacy compatibility
    /** @deprecated Use goals instead */
    projects: goals,
  }
}
