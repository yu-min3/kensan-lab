import { useEffect, useState, useCallback } from 'react'

interface SpotlightOverlayProps {
  targetSelector: string
  onClick: () => void
}

interface TargetRect {
  top: number
  left: number
  width: number
  height: number
}

const PADDING = 8

export function SpotlightOverlay({ targetSelector, onClick }: SpotlightOverlayProps) {
  const [rect, setRect] = useState<TargetRect | null>(null)

  const updateRect = useCallback(() => {
    const el = document.querySelector(`[data-guide="${targetSelector}"]`)
    if (!el) {
      setRect(null)
      return
    }
    const r = el.getBoundingClientRect()
    setRect({
      top: r.top - PADDING,
      left: r.left - PADDING,
      width: r.width + PADDING * 2,
      height: r.height + PADDING * 2,
    })
  }, [targetSelector])

  useEffect(() => {
    updateRect()

    // Listen to scroll on main and window
    const mainEl = document.querySelector('main')
    const handleUpdate = () => requestAnimationFrame(updateRect)

    window.addEventListener('resize', handleUpdate)
    window.addEventListener('scroll', handleUpdate, true)
    mainEl?.addEventListener('scroll', handleUpdate)

    const observer = new ResizeObserver(handleUpdate)
    const targetEl = document.querySelector(`[data-guide="${targetSelector}"]`)
    if (targetEl) observer.observe(targetEl)

    return () => {
      window.removeEventListener('resize', handleUpdate)
      window.removeEventListener('scroll', handleUpdate, true)
      mainEl?.removeEventListener('scroll', handleUpdate)
      observer.disconnect()
    }
  }, [targetSelector, updateRect])

  if (!rect) return null

  // Build clip-path polygon that cuts out the target area
  const clipPath = `polygon(
    0% 0%, 0% 100%, 100% 100%, 100% 0%, 0% 0%,
    ${rect.left}px ${rect.top}px,
    ${rect.left}px ${rect.top + rect.height}px,
    ${rect.left + rect.width}px ${rect.top + rect.height}px,
    ${rect.left + rect.width}px ${rect.top}px,
    ${rect.left}px ${rect.top}px
  )`

  return (
    <>
      {/* Dark overlay with cutout */}
      <div
        className="fixed inset-0 z-[100]"
        style={{
          backgroundColor: 'rgba(0, 0, 0, 0.5)',
          clipPath,
        }}
        onClick={onClick}
      />
      {/* Highlight ring on target */}
      <div
        className="fixed z-[100] pointer-events-none rounded-lg"
        style={{
          top: rect.top,
          left: rect.left,
          width: rect.width,
          height: rect.height,
          animation: 'spotlight-pulse 2s ease-in-out infinite',
        }}
      />
    </>
  )
}

export function useTargetRect(targetSelector: string) {
  const [rect, setRect] = useState<DOMRect | null>(null)

  useEffect(() => {
    const update = () => {
      const el = document.querySelector(`[data-guide="${targetSelector}"]`)
      if (el) setRect(el.getBoundingClientRect())
      else setRect(null)
    }

    update()

    const mainEl = document.querySelector('main')
    const handleUpdate = () => requestAnimationFrame(update)

    window.addEventListener('resize', handleUpdate)
    window.addEventListener('scroll', handleUpdate, true)
    mainEl?.addEventListener('scroll', handleUpdate)

    return () => {
      window.removeEventListener('resize', handleUpdate)
      window.removeEventListener('scroll', handleUpdate, true)
      mainEl?.removeEventListener('scroll', handleUpdate)
    }
  }, [targetSelector])

  return rect
}
