import { useState, useRef, useCallback, useLayoutEffect } from 'react'

interface PanelResizeOptions {
  minWidth?: number
  maxWidth?: number
  defaultWidth?: number
}

export function usePanelResize({
  minWidth = 380,
  maxWidth = 900,
  defaultWidth = 520,
}: PanelResizeOptions = {}) {
  const [panelWidth, setPanelWidth] = useState(defaultWidth)
  const isResizing = useRef(false)
  const startX = useRef(0)
  const startWidth = useRef(defaultWidth)

  useLayoutEffect(() => {
    const handleMouseMove = (e: MouseEvent) => {
      if (!isResizing.current) return
      const delta = startX.current - e.clientX
      const newWidth = Math.min(maxWidth, Math.max(minWidth, startWidth.current + delta))
      setPanelWidth(newWidth)
    }

    const handleMouseUp = () => {
      if (isResizing.current) {
        isResizing.current = false
        document.body.style.cursor = ''
        document.body.style.userSelect = ''
      }
    }

    document.addEventListener('mousemove', handleMouseMove)
    document.addEventListener('mouseup', handleMouseUp)
    return () => {
      document.removeEventListener('mousemove', handleMouseMove)
      document.removeEventListener('mouseup', handleMouseUp)
    }
  }, [minWidth, maxWidth])

  const handleResizeStart = useCallback(
    (e: React.MouseEvent) => {
      e.preventDefault()
      isResizing.current = true
      startX.current = e.clientX
      startWidth.current = panelWidth
      document.body.style.cursor = 'col-resize'
      document.body.style.userSelect = 'none'
    },
    [panelWidth],
  )

  return { panelWidth, handleResizeStart }
}
