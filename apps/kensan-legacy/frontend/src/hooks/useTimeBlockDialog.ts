import { useState, useCallback } from 'react'
import { toast } from 'sonner'
import { useTaskManagerStore } from '@/stores/useTaskManagerStore'
import { useTimeBlockStore } from '@/stores/useTimeBlockStore'
import { useSettingsStore } from '@/stores/useSettingsStore'
import { getLocalDate, getLocalTime } from '@/lib/timezone'
import type { TimeBlock } from '@/types'

export type TaskInputMode = 'manual' | 'existing'

interface TimeBlockDialogState {
  isOpen: boolean
  editingBlockId: string | null
  taskName: string
  startDatetime: string // YYYY-MM-DDTHH:mm (local)
  endDatetime: string   // YYYY-MM-DDTHH:mm (local)
  taskId: string | undefined
  milestoneId: string | undefined
  taskInputMode: TaskInputMode
}

// Helper: datetime-local 形式 (YYYY-MM-DDTHH:mm) から date と time を分解
function splitDatetime(datetime: string): { date: string; time: string } {
  const [date, time] = datetime.split('T')
  return { date, time }
}

interface UseTimeBlockDialogOptions {
  defaultDate: string
}

export function useTimeBlockDialog(options: UseTimeBlockDialogOptions) {
  const { defaultDate } = options
  const { addTimeBlock, updateTimeBlock, deleteTimeBlock } = useTimeBlockStore()
  const { getMilestoneById, getGoalById } = useTaskManagerStore()
  const timezone = useSettingsStore((s) => s.timezone) || 'Asia/Tokyo'

  const [state, setState] = useState<TimeBlockDialogState>({
    isOpen: false,
    editingBlockId: null,
    taskName: '',
    startDatetime: `${defaultDate}T09:00`,
    endDatetime: `${defaultDate}T10:00`,
    taskId: undefined,
    milestoneId: undefined,
    taskInputMode: 'existing', // デフォルトはタスクから選択
  })

  // Get goal from selected milestone
  const selectedMilestone = state.milestoneId ? getMilestoneById(state.milestoneId) : undefined
  const selectedGoal = selectedMilestone ? getGoalById(selectedMilestone.goalId) : undefined

  const openDialog = useCallback((params?: {
    taskId?: string
    taskName?: string
    milestoneId?: string
    taskInputMode?: TaskInputMode
  }) => {
    // Calculate current time rounded to 15 minutes
    const now = new Date()
    const startH = now.getHours().toString().padStart(2, '0')
    const startM = (Math.floor(now.getMinutes() / 15) * 15).toString().padStart(2, '0')
    const endH = (now.getHours() + 1).toString().padStart(2, '0')

    setState({
      isOpen: true,
      editingBlockId: null,
      taskName: params?.taskName || '',
      startDatetime: `${defaultDate}T${startH}:${startM}`,
      endDatetime: `${defaultDate}T${endH}:${startM}`,
      taskId: params?.taskId,
      milestoneId: params?.milestoneId,
      taskInputMode: params?.taskInputMode || 'existing',
    })
  }, [defaultDate])

  const openEditDialog = useCallback((block: TimeBlock) => {
    const startDate = getLocalDate(block.startDatetime, timezone)
    const startTime = getLocalTime(block.startDatetime, timezone)
    const endDate = getLocalDate(block.endDatetime, timezone)
    const endTime = getLocalTime(block.endDatetime, timezone)

    setState({
      isOpen: true,
      editingBlockId: block.id,
      taskName: block.taskName,
      startDatetime: `${startDate}T${startTime}`,
      endDatetime: `${endDate}T${endTime}`,
      taskId: block.taskId,
      milestoneId: block.milestoneId,
      taskInputMode: block.taskId ? 'existing' : 'manual',
    })
  }, [timezone])

  // 指定した時刻でダイアログを開く（空きエリアダブルクリック用）
  const openDialogWithTime = useCallback((startTime: string, endTime: string, date?: string) => {
    const targetDate = date || defaultDate
    // endTime が startTime より前なら翌日とみなす
    const endDate = endTime <= startTime
      ? new Date(new Date(targetDate).getTime() + 86400000).toISOString().split('T')[0]
      : targetDate

    setState({
      isOpen: true,
      editingBlockId: null,
      taskName: '',
      startDatetime: `${targetDate}T${startTime}`,
      endDatetime: `${endDate}T${endTime}`,
      taskId: undefined,
      milestoneId: undefined,
      taskInputMode: 'existing',
    })
  }, [defaultDate])

  const closeDialog = useCallback(() => {
    setState(prev => ({ ...prev, isOpen: false }))
  }, [])

  const setField = useCallback(<K extends keyof TimeBlockDialogState>(
    field: K,
    value: TimeBlockDialogState[K]
  ) => {
    setState(prev => ({ ...prev, [field]: value }))
  }, [])

  const save = useCallback(async () => {
    if (!state.taskName || !state.startDatetime || !state.endDatetime) return false

    // datetime を date/time に分解して store に渡す
    const start = splitDatetime(state.startDatetime)
    const end = splitDatetime(state.endDatetime)

    const data = {
      taskName: state.taskName,
      taskId: state.taskId,
      milestoneId: state.milestoneId,
      milestoneName: selectedMilestone?.name,
      goalId: selectedGoal?.id,
      goalName: selectedGoal?.name,
      goalColor: selectedGoal?.color,
    }

    try {
      if (state.editingBlockId) {
        await updateTimeBlock(state.editingBlockId, start.date, start.time, end.date, end.time, data)
      } else {
        await addTimeBlock(start.date, start.time, end.date, end.time, data)
      }

      closeDialog()
      toast.success(state.editingBlockId ? '予定を更新しました' : '予定を追加しました')
      return true
    } catch {
      // エラートーストはhttpClientで表示される
      return false
    }
  }, [
    state,
    selectedMilestone,
    selectedGoal,
    addTimeBlock,
    updateTimeBlock,
    closeDialog,
  ])

  const deleteBlock = useCallback(async (id: string) => {
    try {
      await deleteTimeBlock(id)
      toast.success('予定を削除しました')
      return true
    } catch {
      // エラートーストはhttpClientで表示される
      return false
    }
  }, [deleteTimeBlock])

  return {
    // State
    isOpen: state.isOpen,
    editingBlockId: state.editingBlockId,
    taskName: state.taskName,
    startDatetime: state.startDatetime,
    endDatetime: state.endDatetime,
    taskId: state.taskId,
    milestoneId: state.milestoneId,
    taskInputMode: state.taskInputMode,
    selectedMilestone,
    selectedGoal,

    // Actions
    openDialog,
    openDialogWithTime,
    openEditDialog,
    closeDialog,
    setField,
    save,
    deleteBlock,
  }
}
