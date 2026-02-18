import { useState, useEffect } from 'react'
import { toast } from 'sonner'
import { useTaskManagerStore } from '@/stores/useTaskManagerStore'
import { useSettingsStore } from '@/stores/useSettingsStore'
import { useDialogState } from '@/hooks/useDialogState'
import type { Milestone } from '@/types'
import type { MilestoneFormData } from '@/components/task/MilestoneDialog'

const initialMilestoneFormData: MilestoneFormData = {
  name: '',
  description: '',
  goalId: '',
  startDate: '',
  targetDate: '',
  status: 'active',
}

interface UseMilestoneManagementParams {
  selectedGoalId: string | null
  isStandaloneSelected: boolean
}

export function useMilestoneManagement({
  selectedGoalId,
  isStandaloneSelected,
}: UseMilestoneManagementParams) {
  const {
    goals,
    milestones,
    addMilestone,
    updateMilestone,
    deleteMilestone,
    getMilestonesByGoal,
    getTasksByMilestone,
  } = useTaskManagerStore()

  const { hideCompleted } = useSettingsStore()

  // Selection state
  const [selectedMilestoneId, setSelectedMilestoneId] = useState<string | null>(null)

  // Dialog
  const milestoneDialog = useDialogState<MilestoneFormData>(initialMilestoneFormData)

  // Auto-select first milestone when goal changes
  useEffect(() => {
    if (selectedGoalId && !isStandaloneSelected) {
      const goalMilestones = getMilestonesByGoal(selectedGoalId).filter(m => m.status !== 'archived')
      if (goalMilestones.length > 0 && !goalMilestones.find(m => m.id === selectedMilestoneId)) {
        setSelectedMilestoneId(goalMilestones[0].id)
      } else if (goalMilestones.length === 0) {
        setSelectedMilestoneId(null)
      }
    }
  }, [selectedGoalId, getMilestonesByGoal, selectedMilestoneId, isStandaloneSelected])

  // Selected goal milestones (sorted by target date)
  const selectedGoalMilestones = (selectedGoalId && !isStandaloneSelected)
    ? getMilestonesByGoal(selectedGoalId)
        .filter(m => {
          if (hideCompleted && m.status === 'completed') return false
          return m.status !== 'archived'
        })
        .sort((a, b) => {
          if (!a.targetDate && !b.targetDate) return 0
          if (!a.targetDate) return 1
          if (!b.targetDate) return -1
          return a.targetDate.localeCompare(b.targetDate)
        })
    : []

  // Derived state
  const selectedMilestone = selectedMilestoneId ? milestones.find(m => m.id === selectedMilestoneId) : null

  // Progress calculation
  const calculateMilestoneProgress = (milestoneId: string) => {
    const milestoneTasks = getTasksByMilestone(milestoneId)
    if (milestoneTasks.length === 0) return { completed: 0, total: 0, percentage: 0 }
    const completedTasks = milestoneTasks.filter(t => t.completed).length
    return {
      completed: completedTasks,
      total: milestoneTasks.length,
      percentage: Math.round((completedTasks / milestoneTasks.length) * 100),
    }
  }

  // Milestone CRUD handlers
  const openNewMilestoneDialog = (goalId?: string) => {
    milestoneDialog.open({
      goalId: goalId || selectedGoalId || goals[0]?.id || '',
    })
  }

  const openEditMilestoneDialog = (milestone: Milestone) => {
    milestoneDialog.openEdit(milestone.id, {
      name: milestone.name,
      description: milestone.description || '',
      goalId: milestone.goalId,
      startDate: milestone.startDate || '',
      targetDate: milestone.targetDate || '',
      status: milestone.status,
    })
  }

  const handleSaveMilestone = async (data: MilestoneFormData, editingId: string | null) => {
    try {
      if (editingId) {
        await updateMilestone(editingId, {
          name: data.name,
          description: data.description || undefined,
          startDate: data.startDate || undefined,
          targetDate: data.targetDate || undefined,
          status: data.status,
        })
        toast.success('マイルストーンを更新しました')
      } else {
        await addMilestone({
          name: data.name,
          description: data.description || undefined,
          goalId: data.goalId,
          startDate: data.startDate || undefined,
          targetDate: data.targetDate || undefined,
        })
        toast.success('マイルストーンを追加しました')
      }
    } catch {
      // エラートーストはhttpClientで表示される
    }
  }

  const handleDeleteMilestone = async (id: string) => {
    try {
      await deleteMilestone(id)
      if (selectedMilestoneId === id) setSelectedMilestoneId(null)
      toast.success('マイルストーンを削除しました')
    } catch {
      // エラートーストはhttpClientで表示される
    }
  }

  const handleCompleteMilestone = async (milestoneId: string) => {
    try {
      await updateMilestone(milestoneId, { status: 'completed' })
      toast.success('マイルストーンを完了しました')
    } catch {
      // エラートーストはhttpClientで表示される
    }
  }

  const getMilestoneCompleteMessage = (milestoneId: string) => {
    const progress = calculateMilestoneProgress(milestoneId)
    if (progress.total > 0 && progress.completed < progress.total) {
      return `未完了のタスクが${progress.total - progress.completed}件あります。完了しますか？`
    }
    return 'マイルストーンを完了しますか？'
  }

  return {
    // Data
    milestones,

    // Selection
    selectedMilestoneId,
    setSelectedMilestoneId,
    selectedMilestone,

    // Computed
    selectedGoalMilestones,

    // Progress
    calculateMilestoneProgress,

    // CRUD
    milestoneDialog,
    openNewMilestoneDialog,
    openEditMilestoneDialog,
    handleSaveMilestone,
    handleDeleteMilestone,
    handleCompleteMilestone,
    getMilestoneCompleteMessage,
  }
}
