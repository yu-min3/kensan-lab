import { useCallback, useSyncExternalStore } from 'react'

const STORAGE_KEY = 'kensan-version-seen'

type SeenMap = Record<string, number>

function getSeenMap(): SeenMap {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    return raw ? JSON.parse(raw) : {}
  } catch {
    return {}
  }
}

function setSeenMap(map: SeenMap) {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(map))
  // Notify subscribers
  listeners.forEach((fn) => fn())
}

// External store pattern for React 18 compatibility
const listeners = new Set<() => void>()
function subscribe(callback: () => void) {
  listeners.add(callback)
  return () => listeners.delete(callback)
}
function getSnapshot() {
  return localStorage.getItem(STORAGE_KEY) ?? '{}'
}

export function useVersionSeen() {
  // Re-render when localStorage changes
  const raw = useSyncExternalStore(subscribe, getSnapshot)
  const seenMap: SeenMap = JSON.parse(raw)

  const getLastSeen = useCallback(
    (contextId: string): number | null => {
      return seenMap[contextId] ?? null
    },
    [seenMap],
  )

  const markSeen = useCallback((contextId: string, versionNumber: number) => {
    const current = getSeenMap()
    const existing = current[contextId] ?? 0
    if (versionNumber > existing) {
      setSeenMap({ ...current, [contextId]: versionNumber })
    }
  }, [])

  const hasUnseen = useCallback(
    (contextId: string, currentVersionNumber: number | null): boolean => {
      if (currentVersionNumber == null) return false
      const lastSeen = seenMap[contextId]
      if (lastSeen == null) return false
      return currentVersionNumber > lastSeen
    },
    [seenMap],
  )

  const markUnseen = useCallback((contextId: string) => {
    const current = getSeenMap()
    if (contextId in current && current[contextId] > 0) {
      setSeenMap({ ...current, [contextId]: current[contextId] - 1 })
    }
  }, [])

  const initializeIfNeeded = useCallback(
    (contexts: { id: string; current_version_number: number | null }[]) => {
      const current = getSeenMap()
      let changed = false
      for (const ctx of contexts) {
        if (!(ctx.id in current) && ctx.current_version_number != null) {
          current[ctx.id] = ctx.current_version_number
          changed = true
        }
      }
      if (changed) {
        setSeenMap(current)
      }
    },
    [],
  )

  return { getLastSeen, markSeen, markUnseen, hasUnseen, initializeIfNeeded }
}
