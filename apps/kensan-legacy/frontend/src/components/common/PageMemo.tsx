/**
 * ページ固有の常時表示メモ
 * - 各ページに配置して自由にメモを書ける
 * - localStorageに自動保存
 * - 下端ドラッグでリサイズ可能（高さも永続化）
 */
import { useState, useEffect, useCallback, useRef } from 'react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Textarea } from '@/components/ui/textarea'
import { StickyNote, GripHorizontal } from 'lucide-react'
import { cn } from '@/lib/utils'
import { useAuthStore } from '@/stores/useAuthStore'

interface PageMemoProps {
  /** ページ識別子（localStorageのキーに使用） */
  pageId: string
  /** タイトル */
  title?: string
  /** プレースホルダー */
  placeholder?: string
  /** 追加のクラス名 */
  className?: string
  /** 最小高さ */
  minHeight?: number
}

const STORAGE_PREFIX = 'kensan-page-memo-'
const HEIGHT_STORAGE_PREFIX = 'kensan-page-memo-height-'
const MIN_HEIGHT = 80
const DEFAULT_HEIGHT = 150

export function PageMemo({
  pageId,
  title = 'メモ',
  placeholder = '自由にメモを書けます...',
  className,
  minHeight = MIN_HEIGHT,
}: PageMemoProps) {
  const [content, setContent] = useState('')
  const [isSaving, setIsSaving] = useState(false)
  const [height, setHeight] = useState(DEFAULT_HEIGHT)
  const userId = useAuthStore((state) => state.user?.id)

  // リサイズ用ref
  const isResizing = useRef(false)
  const startY = useRef(0)
  const startHeight = useRef(0)

  // localStorageキー（ユーザーごとに分離）
  const storageKey = userId ? `${STORAGE_PREFIX}${userId}-${pageId}` : `${STORAGE_PREFIX}${pageId}`
  const heightKey = userId ? `${HEIGHT_STORAGE_PREFIX}${userId}-${pageId}` : `${HEIGHT_STORAGE_PREFIX}${pageId}`

  // 初期読み込み
  useEffect(() => {
    const saved = localStorage.getItem(storageKey)
    if (saved) {
      setContent(saved)
    }
    const savedHeight = localStorage.getItem(heightKey)
    if (savedHeight) {
      const h = parseInt(savedHeight, 10)
      if (!isNaN(h) && h >= minHeight) setHeight(h)
    }
  }, [storageKey, heightKey, minHeight])

  // 自動保存（デバウンス）
  useEffect(() => {
    const timer = setTimeout(() => {
      localStorage.setItem(storageKey, content)
      setIsSaving(false)
    }, 500)

    return () => clearTimeout(timer)
  }, [content, storageKey])

  const handleChange = useCallback((e: React.ChangeEvent<HTMLTextAreaElement>) => {
    setContent(e.target.value)
    setIsSaving(true)
  }, [])

  // リサイズハンドラ
  const handleResizeStart = useCallback((e: React.MouseEvent | React.TouchEvent) => {
    e.preventDefault()
    isResizing.current = true
    startHeight.current = height
    startY.current = 'touches' in e ? e.touches[0].clientY : e.clientY
    document.body.style.cursor = 'ns-resize'
    document.body.style.userSelect = 'none'
  }, [height])

  useEffect(() => {
    const handleMove = (e: MouseEvent | TouchEvent) => {
      if (!isResizing.current) return
      const clientY = 'touches' in e ? e.touches[0].clientY : e.clientY
      const delta = clientY - startY.current
      const newHeight = Math.max(minHeight, startHeight.current + delta)
      setHeight(newHeight)
    }

    const handleEnd = () => {
      if (!isResizing.current) return
      isResizing.current = false
      document.body.style.cursor = ''
      document.body.style.userSelect = ''
      // 高さを永続化
      setHeight(h => {
        localStorage.setItem(heightKey, String(h))
        return h
      })
    }

    window.addEventListener('mousemove', handleMove)
    window.addEventListener('mouseup', handleEnd)
    window.addEventListener('touchmove', handleMove)
    window.addEventListener('touchend', handleEnd)
    return () => {
      window.removeEventListener('mousemove', handleMove)
      window.removeEventListener('mouseup', handleEnd)
      window.removeEventListener('touchmove', handleMove)
      window.removeEventListener('touchend', handleEnd)
    }
  }, [heightKey, minHeight])

  return (
    <Card className={cn('flex flex-col', className)}>
      <CardHeader className="py-2 px-3 border-b flex-shrink-0">
        <CardTitle className="text-sm font-medium flex items-center gap-2">
          <StickyNote className="h-4 w-4 text-yellow-500" />
          {title}
          {isSaving && (
            <span className="text-[10px] text-muted-foreground ml-auto">保存中...</span>
          )}
        </CardTitle>
      </CardHeader>
      <CardContent className="p-2 flex-1">
        <Textarea
          value={content}
          onChange={handleChange}
          placeholder={placeholder}
          className="resize-none border-0 focus-visible:ring-0 focus-visible:ring-offset-0 p-1 text-sm h-full"
          style={{ height: `${height}px` }}
        />
      </CardContent>
      {/* リサイズハンドル */}
      <div
        onMouseDown={handleResizeStart}
        onTouchStart={handleResizeStart}
        className="flex items-center justify-center h-3 cursor-ns-resize hover:bg-muted/50 transition-colors rounded-b-lg group"
      >
        <GripHorizontal className="h-3 w-3 text-muted-foreground/40 group-hover:text-muted-foreground/70" />
      </div>
    </Card>
  )
}
