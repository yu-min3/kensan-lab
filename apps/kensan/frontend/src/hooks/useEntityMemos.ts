import { useState, useEffect, useCallback } from 'react'
import { entityMemosApi } from '@/api/services/tasks'
import type { EntityMemo, EntityType } from '@/types'

interface UseEntityMemosOptions {
  entityType: EntityType
  entityId: string
  enabled?: boolean
}

interface UseEntityMemosReturn {
  memos: EntityMemo[]
  isLoading: boolean
  error: string | null
  addMemo: (content: string, pinned?: boolean) => Promise<void>
  updateMemo: (memoId: string, updates: { content?: string; pinned?: boolean }) => Promise<void>
  deleteMemo: (memoId: string) => Promise<void>
  togglePin: (memoId: string) => Promise<void>
  refresh: () => Promise<void>
}

export function useEntityMemos({
  entityType,
  entityId,
  enabled = true,
}: UseEntityMemosOptions): UseEntityMemosReturn {
  const [memos, setMemos] = useState<EntityMemo[]>([])
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const fetchMemos = useCallback(async () => {
    if (!entityId || !enabled) return

    setIsLoading(true)
    setError(null)
    try {
      const result = await entityMemosApi.list({ entityType, entityId })
      setMemos(result)
    } catch (err) {
      setError((err as Error).message)
    } finally {
      setIsLoading(false)
    }
  }, [entityType, entityId, enabled])

  useEffect(() => {
    fetchMemos()
  }, [fetchMemos])

  const addMemo = useCallback(
    async (content: string, pinned = false) => {
      try {
        const newMemo = await entityMemosApi.create({
          entityType,
          entityId,
          content,
          pinned,
        })
        setMemos((prev) => [newMemo, ...prev].sort((a, b) => {
          if (a.pinned !== b.pinned) return a.pinned ? -1 : 1
          return b.createdAt.getTime() - a.createdAt.getTime()
        }))
      } catch (err) {
        setError((err as Error).message)
        throw err
      }
    },
    [entityType, entityId]
  )

  const updateMemo = useCallback(
    async (memoId: string, updates: { content?: string; pinned?: boolean }) => {
      try {
        const updated = await entityMemosApi.update(memoId, updates)
        setMemos((prev) =>
          prev.map((m) => (m.id === memoId ? updated : m)).sort((a, b) => {
            // ピン留めを上に、その中で新しい順
            if (a.pinned !== b.pinned) return a.pinned ? -1 : 1
            return b.createdAt.getTime() - a.createdAt.getTime()
          })
        )
      } catch (err) {
        setError((err as Error).message)
        throw err
      }
    },
    []
  )

  const deleteMemo = useCallback(async (memoId: string) => {
    try {
      await entityMemosApi.delete(memoId)
      setMemos((prev) => prev.filter((m) => m.id !== memoId))
    } catch (err) {
      setError((err as Error).message)
      throw err
    }
  }, [])

  const togglePin = useCallback(
    async (memoId: string) => {
      const memo = memos.find((m) => m.id === memoId)
      if (!memo) return
      await updateMemo(memoId, { pinned: !memo.pinned })
    },
    [memos, updateMemo]
  )

  return {
    memos,
    isLoading,
    error,
    addMemo,
    updateMemo,
    deleteMemo,
    togglePin,
    refresh: fetchMemos,
  }
}
