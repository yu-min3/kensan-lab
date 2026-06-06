import { useState, useCallback } from 'react'

/**
 * Custom hook for managing dialog state with optional edit data.
 *
 * @example
 * ```tsx
 * interface TaskForm {
 *   name: string
 *   projectId: string
 * }
 *
 * const taskDialog = useDialogState<TaskForm>({
 *   name: '',
 *   projectId: '',
 * })
 *
 * // Open for new item
 * taskDialog.open()
 *
 * // Open for editing
 * taskDialog.openEdit('task-123', { name: 'Existing task', projectId: 'p1' })
 *
 * // In JSX
 * <Dialog open={taskDialog.isOpen} onOpenChange={taskDialog.setIsOpen}>
 *   <Input value={taskDialog.data.name} onChange={(e) => taskDialog.setField('name', e.target.value)} />
 * </Dialog>
 * ```
 */
export function useDialogState<T extends object>(initialData: T) {
  const [isOpen, setIsOpen] = useState(false)
  const [editingId, setEditingId] = useState<string | null>(null)
  const [data, setData] = useState<T>(initialData)

  const reset = useCallback(() => {
    setEditingId(null)
    setData(initialData)
  }, [initialData])

  const open = useCallback((overrides?: Partial<T>) => {
    setEditingId(null)
    setData({ ...initialData, ...overrides })
    setIsOpen(true)
  }, [initialData])

  const openEdit = useCallback((id: string, editData: Partial<T>) => {
    setEditingId(id)
    setData({ ...initialData, ...editData })
    setIsOpen(true)
  }, [initialData])

  const close = useCallback(() => {
    setIsOpen(false)
  }, [])

  const setField = useCallback(<K extends keyof T>(field: K, value: T[K]) => {
    setData((prev) => ({ ...prev, [field]: value }))
  }, [])

  return {
    isOpen,
    setIsOpen,
    editingId,
    isEditing: editingId !== null,
    data,
    setData,
    setField,
    open,
    openEdit,
    close,
    reset,
  }
}

export type DialogState<T extends object> = ReturnType<typeof useDialogState<T>>
