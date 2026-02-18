import { useState, useMemo, useCallback } from 'react'
import { toast } from 'sonner'
import { useTaskManagerStore } from '@/stores/useTaskManagerStore'
import { useSettingsStore } from '@/stores/useSettingsStore'
import { useTaskDetailPanel } from '@/hooks/useTaskDetailPanel'
import { useGoalManagement } from '@/hooks/useGoalManagement'
import { useMilestoneManagement } from '@/hooks/useMilestoneManagement'
import { useTagManagement } from '@/hooks/useTagManagement'
import { useTaskSelection } from '@/hooks/useTaskSelection'
import type { Task } from '@/types'
import type { DragEndEvent } from '@dnd-kit/core'

export function useTaskManagement() {
  const {
    tasks,
    toggleTaskComplete,
    getTasksByMilestone,
    getChildTasks,
    getStandaloneTasks,
    getTagsByIds,
    deleteTask,
    reorderTasks,
  } = useTaskManagerStore()

  // Filters
  const [searchQuery, setSearchQuery] = useState('')
  const { hideCompleted, setHideCompleted } = useSettingsStore()

  // Task expansion
  const [expandedTasks, setExpandedTasks] = useState<Set<string>>(new Set())
  const [recentlyCompleted, setRecentlyCompleted] = useState<Set<string>>(new Set())

  // Sub-hooks
  const goalMgmt = useGoalManagement()
  const milestoneMgmt = useMilestoneManagement({
    selectedGoalId: goalMgmt.selectedGoalId,
    isStandaloneSelected: goalMgmt.isStandaloneSelected,
  })
  const tagMgmt = useTagManagement()

  // Task detail panel
  const taskDetailPanel = useTaskDetailPanel()

  // Milestone tasks and standalone tasks
  const selectedMilestoneTasks = milestoneMgmt.selectedMilestoneId
    ? getTasksByMilestone(milestoneMgmt.selectedMilestoneId).filter(t => !t.parentTaskId)
    : []
  const standaloneTasks = getStandaloneTasks().filter(t => !t.parentTaskId)

  // Task filtering and sorting (default: due date ascending, no due date last)
  const filterTasks = useCallback((taskList: Task[]) => {
    return taskList
      .filter(task => {
        const matchesSearch = searchQuery === '' || task.name.toLowerCase().includes(searchQuery.toLowerCase())
        const matchesCompleted = !hideCompleted || !task.completed
        return matchesSearch && matchesCompleted
      })
      .sort((a, b) => {
        // Completed tasks go to the bottom
        if (a.completed !== b.completed) return a.completed ? 1 : -1
        // Both have due dates: sort ascending (nearest deadline first)
        if (a.dueDate && b.dueDate) return a.dueDate.localeCompare(b.dueDate)
        // Only one has a due date: it comes first
        if (a.dueDate && !b.dueDate) return -1
        if (!a.dueDate && b.dueDate) return 1
        // Neither has a due date: fall back to sortOrder
        return a.sortOrder - b.sortOrder
      })
  }, [searchQuery, hideCompleted])

  const sortedMilestoneTasks = useMemo(
    () => filterTasks(selectedMilestoneTasks),
    [selectedMilestoneTasks, filterTasks]
  )

  const sortedStandaloneTasks = useMemo(
    () => filterTasks(standaloneTasks),
    [standaloneTasks, filterTasks]
  )

  // Task selection (needs access to sorted tasks)
  const taskSelection = useTaskSelection({
    getCurrentTasks: () =>
      milestoneMgmt.selectedMilestoneId ? sortedMilestoneTasks : sortedStandaloneTasks,
  })

  // Task DnD handler
  const handleDragEnd = async (event: DragEndEvent) => {
    const { active, over } = event
    if (over && active.id !== over.id) {
      const currentTasks = milestoneMgmt.selectedMilestoneId ? sortedMilestoneTasks : sortedStandaloneTasks
      const oldIndex = currentTasks.findIndex(t => t.id === active.id)
      const newIndex = currentTasks.findIndex(t => t.id === over.id)
      if (oldIndex !== -1 && newIndex !== -1) {
        const newOrder = [...currentTasks]
        const [removed] = newOrder.splice(oldIndex, 1)
        newOrder.splice(newIndex, 0, removed)
        await reorderTasks(newOrder.map(t => t.id))
      }
    }
  }

  // Task handlers
  const handleToggleTaskComplete = async (taskId: string) => {
    const task = tasks.find(t => t.id === taskId)
    if (task && !task.completed) {
      setRecentlyCompleted(prev => new Set(prev).add(taskId))
      setTimeout(() => {
        setRecentlyCompleted(prev => {
          const next = new Set(prev)
          next.delete(taskId)
          return next
        })
      }, 1500)
    }
    await toggleTaskComplete(taskId)
  }

  const toggleTask = (taskId: string) => {
    const newExpanded = new Set(expandedTasks)
    if (newExpanded.has(taskId)) newExpanded.delete(taskId)
    else newExpanded.add(taskId)
    setExpandedTasks(newExpanded)
  }

  // Task CRUD handlers
  const openNewTaskDialog = (milestoneId?: string, parentId?: string) => {
    taskDetailPanel.openNewTask({
      milestoneId: milestoneId || milestoneMgmt.selectedMilestoneId || undefined,
      parentTaskId: parentId,
    })
  }

  const openEditTaskDialog = (task: Task) => {
    taskDetailPanel.openTask(task.id)
  }

  const handleDeleteTask = async (id: string) => {
    try {
      await deleteTask(id)
      toast.success('タスクを削除しました')
    } catch {
      // エラートーストはhttpClientで表示される
    }
  }

  // Wrap handleDeleteGoal to also clear milestone selection
  const handleDeleteGoal = async (id: string) => {
    const wasSelected = goalMgmt.selectedGoalId === id
    await goalMgmt.handleDeleteGoal(id)
    if (wasSelected) {
      milestoneMgmt.setSelectedMilestoneId(null)
    }
  }

  return {
    // Data
    goals: goalMgmt.goals,
    milestones: milestoneMgmt.milestones,
    tags: tagMgmt.tags,
    tasks,

    // Selection
    selectedGoalId: goalMgmt.selectedGoalId,
    setSelectedGoalId: goalMgmt.setSelectedGoalId,
    selectedMilestoneId: milestoneMgmt.selectedMilestoneId,
    setSelectedMilestoneId: milestoneMgmt.setSelectedMilestoneId,
    isStandaloneSelected: goalMgmt.isStandaloneSelected,
    selectedGoal: goalMgmt.selectedGoal,
    selectedMilestone: milestoneMgmt.selectedMilestone,

    // Filters
    searchQuery, setSearchQuery,
    hideCompleted, setHideCompleted,

    // Task expansion
    expandedTasks, recentlyCompleted,

    // Multi-select
    selectedTaskIds: taskSelection.selectedTaskIds,
    isSelectionMode: taskSelection.isSelectionMode,
    handleSelectTask: taskSelection.handleSelectTask,
    handleSelectAll: taskSelection.handleSelectAll,
    handleClearSelection: taskSelection.handleClearSelection,

    // Bulk operations
    handleBulkDelete: taskSelection.handleBulkDelete,
    handleBulkComplete: taskSelection.handleBulkComplete,

    // DnD
    sensors: goalMgmt.sensors,
    handleDragEnd,
    handleGoalDragEnd: goalMgmt.handleGoalDragEnd,

    // Computed data
    filteredGoals: goalMgmt.filteredGoals,
    selectedGoalMilestones: milestoneMgmt.selectedGoalMilestones,
    sortedMilestoneTasks, sortedStandaloneTasks,
    standaloneTasks,

    // Progress
    calculateMilestoneProgress: milestoneMgmt.calculateMilestoneProgress,
    calculateGoalProgress: goalMgmt.calculateGoalProgress,

    // Task operations
    handleToggleTaskComplete, toggleTask,
    getChildTasks, getTagsByIds, filterTasks,

    // CRUD handlers
    handleSaveGoal: goalMgmt.handleSaveGoal,
    handleDeleteGoal,
    handleCompleteGoal: goalMgmt.handleCompleteGoal,
    openEditGoalDialog: goalMgmt.openEditGoalDialog,
    openNewMilestoneDialog: milestoneMgmt.openNewMilestoneDialog,
    openEditMilestoneDialog: milestoneMgmt.openEditMilestoneDialog,
    handleSaveMilestone: milestoneMgmt.handleSaveMilestone,
    handleDeleteMilestone: milestoneMgmt.handleDeleteMilestone,
    handleCompleteMilestone: milestoneMgmt.handleCompleteMilestone,
    getMilestoneCompleteMessage: milestoneMgmt.getMilestoneCompleteMessage,
    openNewTaskDialog, openEditTaskDialog,
    handleDeleteTask,
    openEditTagDialog: tagMgmt.openEditTagDialog,
    handleSaveTag: tagMgmt.handleSaveTag,
    handleDeleteTag: tagMgmt.handleDeleteTag,

    // Dialogs
    goalDialog: goalMgmt.goalDialog,
    milestoneDialog: milestoneMgmt.milestoneDialog,
    tagDialog: tagMgmt.tagDialog,

    // Detail panel
    taskDetailPanel,
  }
}
