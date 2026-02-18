import { useState, useEffect } from 'react'
import { toast } from 'sonner'
import { useTaskManagerStore } from '@/stores/useTaskManagerStore'
import { useSettingsStore } from '@/stores/useSettingsStore'
import { useDialogState } from '@/hooks/useDialogState'
import { DEFAULT_COLORS } from '@/types'
import type { Goal } from '@/types'
import type { GoalFormData } from '@/components/task/GoalDialog'
import {
  KeyboardSensor,
  PointerSensor,
  useSensor,
  useSensors,
  type DragEndEvent,
} from '@dnd-kit/core'
import { sortableKeyboardCoordinates } from '@dnd-kit/sortable'

const initialGoalFormData: GoalFormData = {
  name: '',
  description: '',
  color: DEFAULT_COLORS[0],
}

export function useGoalManagement() {
  const {
    goals,
    milestones,
    addGoal,
    updateGoal,
    deleteGoal,
    reorderGoals,
    getMilestonesByGoal,
    getTasksByMilestone,
  } = useTaskManagerStore()

  const { hideCompleted } = useSettingsStore()

  // Selection state
  const [selectedGoalId, setSelectedGoalId] = useState<string | null>(null)
  const isStandaloneSelected = selectedGoalId === '__standalone__'

  // DnD sensors
  const sensors = useSensors(
    useSensor(PointerSensor, {
      activationConstraint: { distance: 8 },
    }),
    useSensor(KeyboardSensor, {
      coordinateGetter: sortableKeyboardCoordinates,
    })
  )

  // Dialog
  const goalDialog = useDialogState<GoalFormData>(initialGoalFormData)

  // Auto-select first goal
  useEffect(() => {
    if (!selectedGoalId && !isStandaloneSelected && goals.length > 0) {
      const firstGoal = goals.find(g => g.status !== 'archived')
      if (firstGoal) setSelectedGoalId(firstGoal.id)
    }
  }, [goals, selectedGoalId, isStandaloneSelected])

  // Filtered goals
  const filteredGoals = goals.filter(g => {
    if (g.status === 'archived') return false
    if (hideCompleted && g.status === 'completed') return false
    return true
  })

  // Derived state
  const selectedGoal = (selectedGoalId && !isStandaloneSelected) ? goals.find(g => g.id === selectedGoalId) : null

  // Progress calculation
  const calculateGoalProgress = (goalId: string) => {
    const goalMilestones = getMilestonesByGoal(goalId)
    let totalTasks = 0
    let completedTasks = 0
    goalMilestones.forEach(m => {
      const mt = getTasksByMilestone(m.id)
      totalTasks += mt.length
      completedTasks += mt.filter(t => t.completed).length
    })
    if (totalTasks === 0) return { completed: 0, total: 0, percentage: 0 }
    return {
      completed: completedTasks,
      total: totalTasks,
      percentage: Math.round((completedTasks / totalTasks) * 100),
    }
  }

  // Goal DnD handler
  const handleGoalDragEnd = async (event: DragEndEvent) => {
    const { active, over } = event
    if (over && active.id !== over.id) {
      const activeGoals = goals.filter(g => g.status !== 'archived')
      const oldIndex = activeGoals.findIndex(g => g.id === active.id)
      const newIndex = activeGoals.findIndex(g => g.id === over.id)
      if (oldIndex !== -1 && newIndex !== -1) {
        const newOrder = [...activeGoals]
        const [removed] = newOrder.splice(oldIndex, 1)
        newOrder.splice(newIndex, 0, removed)
        await reorderGoals(newOrder.map(g => g.id))
      }
    }
  }

  // Goal CRUD handlers
  const openEditGoalDialog = (goal: Goal) => {
    goalDialog.openEdit(goal.id, {
      name: goal.name,
      description: goal.description || '',
      color: goal.color,
      status: goal.status,
    })
  }

  const handleSaveGoal = async (data: GoalFormData, editingId: string | null) => {
    try {
      if (editingId) {
        await updateGoal(editingId, {
          name: data.name,
          description: data.description || undefined,
          color: data.color,
          status: data.status,
        })
        toast.success('目標を更新しました')
      } else {
        await addGoal({
          name: data.name,
          description: data.description || undefined,
          color: data.color,
        })
        toast.success('目標を追加しました')
      }
    } catch {
      // エラートーストはhttpClientで表示される
    }
  }

  const handleDeleteGoal = async (id: string) => {
    try {
      await deleteGoal(id)
      if (selectedGoalId === id) {
        setSelectedGoalId(null)
      }
      toast.success('目標を削除しました')
    } catch {
      // エラートーストはhttpClientで表示される
    }
  }

  const handleCompleteGoal = async (goalId: string) => {
    try {
      await updateGoal(goalId, { status: 'completed' })
      toast.success('目標を完了しました')
    } catch {
      // エラートーストはhttpClientで表示される
    }
  }

  return {
    // Data
    goals,
    milestones,

    // Selection
    selectedGoalId,
    setSelectedGoalId,
    isStandaloneSelected,
    selectedGoal,

    // Filtered
    filteredGoals,

    // DnD
    sensors,
    handleGoalDragEnd,

    // Progress
    calculateGoalProgress,

    // CRUD
    goalDialog,
    openEditGoalDialog,
    handleSaveGoal,
    handleDeleteGoal,
    handleCompleteGoal,
  }
}
