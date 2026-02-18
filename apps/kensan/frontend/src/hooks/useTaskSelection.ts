import { useState } from 'react'
import { toast } from 'sonner'
import { useTaskManagerStore } from '@/stores/useTaskManagerStore'
import type { Task } from '@/types'

interface UseTaskSelectionParams {
  getCurrentTasks: () => Task[]
}

export function useTaskSelection({ getCurrentTasks }: UseTaskSelectionParams) {
  const {
    bulkDeleteTasks,
    bulkCompleteTasks,
  } = useTaskManagerStore()

  // Multi-select state
  const [selectedTaskIds, setSelectedTaskIds] = useState<Set<string>>(new Set())
  const isSelectionMode = selectedTaskIds.size > 0

  // Selection handlers
  const handleSelectTask = (taskId: string, checked: boolean) => {
    setSelectedTaskIds(prev => {
      const next = new Set(prev)
      if (checked) next.add(taskId)
      else next.delete(taskId)
      return next
    })
  }

  const handleSelectAll = () => {
    const currentTasks = getCurrentTasks()
    setSelectedTaskIds(new Set(currentTasks.map(t => t.id)))
  }

  const handleClearSelection = () => setSelectedTaskIds(new Set())

  // Bulk operations
  const handleBulkDelete = async () => {
    if (selectedTaskIds.size === 0) return
    const count = selectedTaskIds.size
    try {
      await bulkDeleteTasks(Array.from(selectedTaskIds))
      setSelectedTaskIds(new Set())
      toast.success(`${count}件のタスクを削除しました`)
    } catch {
      // エラートーストはhttpClientで表示される
    }
  }

  const handleBulkComplete = async (completed: boolean) => {
    if (selectedTaskIds.size === 0) return
    const count = selectedTaskIds.size
    try {
      await bulkCompleteTasks(Array.from(selectedTaskIds), completed)
      setSelectedTaskIds(new Set())
      toast.success(`${count}件のタスクを${completed ? '完了' : '未完了に変更'}しました`)
    } catch {
      // エラートーストはhttpClientで表示される
    }
  }

  return {
    // Multi-select
    selectedTaskIds,
    isSelectionMode,
    handleSelectTask,
    handleSelectAll,
    handleClearSelection,

    // Bulk operations
    handleBulkDelete,
    handleBulkComplete,
  }
}
