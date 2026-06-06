import { useState, useCallback } from 'react'

export interface NewTaskContext {
  milestoneId?: string
  parentTaskId?: string
}

export function useTaskDetailPanel() {
  const [selectedTaskId, setSelectedTaskId] = useState<string | null>(null)
  const [newTaskContext, setNewTaskContext] = useState<NewTaskContext | null>(null)

  const openTask = useCallback((taskId: string) => {
    setSelectedTaskId(taskId)
    setNewTaskContext(null)
  }, [])

  const openNewTask = useCallback((context?: NewTaskContext) => {
    setSelectedTaskId(null)
    setNewTaskContext(context ?? {})
  }, [])

  // Called after task is created to switch to edit mode
  const switchToCreatedTask = useCallback((taskId: string) => {
    setSelectedTaskId(taskId)
    setNewTaskContext(null)
  }, [])

  const closeTask = useCallback(() => {
    setSelectedTaskId(null)
    setNewTaskContext(null)
  }, [])

  const isOpen = selectedTaskId !== null || newTaskContext !== null
  const isCreateMode = newTaskContext !== null

  return {
    selectedTaskId,
    newTaskContext,
    isCreateMode,
    openTask,
    openNewTask,
    switchToCreatedTask,
    closeTask,
    isOpen,
  }
}
