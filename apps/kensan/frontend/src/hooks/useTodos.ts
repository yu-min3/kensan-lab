import { useState, useEffect, useCallback } from 'react'
import { todosApi } from '@/api/services/tasks'
import type { TodoWithStatus, TodoFrequency } from '@/types'

interface UseTodosOptions {
  date?: string // YYYY-MM-DD for getting status
  enabled?: boolean
}

interface UseTodosReturn {
  todos: TodoWithStatus[]
  isLoading: boolean
  error: string | null
  addTodo: (input: {
    name: string
    frequency?: TodoFrequency
    daysOfWeek?: number[]
    dueDate?: string
    estimatedMinutes?: number
  }) => Promise<void>
  updateTodo: (todoId: string, updates: {
    name?: string
    frequency?: TodoFrequency
    daysOfWeek?: number[]
    dueDate?: string
    estimatedMinutes?: number
    enabled?: boolean
  }) => Promise<void>
  deleteTodo: (todoId: string) => Promise<void>
  toggleComplete: (todoId: string) => Promise<void>
  refresh: () => Promise<void>
}

export function useTodos({
  date,
  enabled = true,
}: UseTodosOptions = {}): UseTodosReturn {
  const [todos, setTodos] = useState<TodoWithStatus[]>([])
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const fetchTodos = useCallback(async () => {
    if (!enabled || !date) return

    setIsLoading(true)
    setError(null)
    try {
      const result = await todosApi.listWithStatus(date)
      setTodos(result)
    } catch (err) {
      setError((err as Error).message)
    } finally {
      setIsLoading(false)
    }
  }, [date, enabled])

  useEffect(() => {
    fetchTodos()
  }, [fetchTodos])

  const addTodo = useCallback(
    async (input: {
      name: string
      frequency?: TodoFrequency
      daysOfWeek?: number[]
      dueDate?: string
      estimatedMinutes?: number
    }) => {
      try {
        await todosApi.create(input)
        await fetchTodos() // Refresh to get updated list with status
      } catch (err) {
        setError((err as Error).message)
        throw err
      }
    },
    [fetchTodos]
  )

  const updateTodo = useCallback(
    async (todoId: string, updates: {
      name?: string
      frequency?: TodoFrequency
      daysOfWeek?: number[]
      dueDate?: string
      estimatedMinutes?: number
      enabled?: boolean
    }) => {
      try {
        await todosApi.update(todoId, updates)
        await fetchTodos()
      } catch (err) {
        setError((err as Error).message)
        throw err
      }
    },
    [fetchTodos]
  )

  const deleteTodo = useCallback(
    async (todoId: string) => {
      try {
        await todosApi.delete(todoId)
        setTodos((prev) => prev.filter((t) => t.id !== todoId))
      } catch (err) {
        setError((err as Error).message)
        throw err
      }
    },
    []
  )

  const toggleComplete = useCallback(
    async (todoId: string) => {
      if (!date) return
      try {
        const updated = await todosApi.toggleComplete(todoId, date)
        setTodos((prev) =>
          prev.map((t) => (t.id === todoId ? updated : t))
        )
      } catch (err) {
        setError((err as Error).message)
        throw err
      }
    },
    [date]
  )

  return {
    todos,
    isLoading,
    error,
    addTodo,
    updateTodo,
    deleteTodo,
    toggleComplete,
    refresh: fetchTodos,
  }
}
