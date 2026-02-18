/**
 * TimelineCore - Daily/Weekly共通のタイムライングリッドコンポーネント
 *
 * 機能:
 * - 時間軸グリッド表示
 * - ブロックのドラッグ（移動）・リサイズ
 * - ズーム（拡大・縮小）
 * - ドロップゾーン（DnD）
 * - ダブルクリックでダイアログを開く
 * - 現在時刻インジケーター
 * - オーバーレイコンテンツ（実績、タイマー等）
 *
 * 内部で TimelineColumnContext を使用し、全カラム共通の設定・ユーティリティ・
 * コールバックを Context 経由で渡すことで、props の数を削減している。
 */
import { useState, useCallback, useEffect, useRef, useMemo, createContext, useContext } from 'react'
import { useDroppable } from '@dnd-kit/core'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'
import { Plus, Minus, Trash2 } from 'lucide-react'
import {
  getMinutesFromTime,
  minutesToTimeString,
  snapToInterval,
  yToMinutes,
  calculateTimeFromY,
} from './utils'
import type { ResizeEdge, PreviewTime } from './types'

// ズーム設定
const MIN_SCALE = 0.5
const MAX_SCALE = 2.0
const SCALE_STEP = 0.25

export interface TimelineBlock {
  id: string
  columnId: string
  startTime: string
  endTime: string
  label: string
  sublabel?: string
  color?: string
}

export interface TimelineColumn {
  id: string
  header: React.ReactNode
  isToday?: boolean
}

// オーバーレイ描画用のコンテキスト
export interface OverlayRenderContext {
  getTopPosition: (time: string) => number
  getHeight: (startTime: string, endTime: string) => number
  hourHeight: number
  totalMinutes: number
  startHour: number
  endHour: number
}

// カスタムブロックレンダラーのコンテキスト
export interface BlockRenderContext {
  block: TimelineBlock
  displayTimes: { startTime: string; endTime: string }
  isResizing: boolean
  getTopPx: (time: string) => number
  getHeightPx: (startTime: string, endTime: string) => number
  onDragStart: (e: React.MouseEvent) => void
  onResizeStart: (e: React.MouseEvent, edge: ResizeEdge) => void
  shouldSuppressClick: () => boolean
}

export interface TimelineCoreProps {
  // レイアウト
  columns: TimelineColumn[]
  blocks: TimelineBlock[]
  startHour?: number
  endHour?: number
  baseHourHeight?: number

  // ズーム
  scale?: number
  onScaleChange?: (scale: number) => void
  showZoomControls?: boolean

  // インタラクション
  onBlockClick?: (blockId: string) => void
  onBlockDelete?: (blockId: string) => void
  onBlockResize?: (blockId: string, columnId: string, startTime: string, endTime: string) => void
  onCellClick?: (columnId: string, hour: number) => void
  onEmptyDoubleClick?: (columnId: string, startTime: string, endTime: string) => void

  // DnD
  isDragging?: boolean
  getDroppableId?: (columnId: string) => string
  getDroppableData?: (columnId: string) => Record<string, unknown>

  // Daily 用拡張機能
  currentTimeMinutes?: number // 現在時刻（分）
  initialScrollHour?: number // 初期スクロール位置（時）
  renderOverlay?: (columnId: string, context: OverlayRenderContext) => React.ReactNode
  renderDropIndicator?: (columnId: string, context: OverlayRenderContext) => React.ReactNode

  // カスタムブロックレンダラー（指定しない場合はデフォルトのシンプルな表示）
  renderBlock?: (context: BlockRenderContext) => React.ReactNode
}

interface DragResizeState {
  blockId: string
  columnId: string
  type: 'drag' | 'resize'
  edge?: ResizeEdge
  initialStartTime: string
  initialEndTime: string
  duration: number
  // ドラッグ開始時にクリック位置とブロック開始位置の差（分）を事前計算して保存。
  // ドラッグ中は getBoundingClientRect() をライブで取得するが（スクロール対応）、
  // offsetFromStart は再計算しないので再レンダリングによるドリフトが起きない。
  offsetFromStart: number
}

// ---------- Internal Context ----------
// TimelineCore → TimelineColumn 間で共通の設定・ユーティリティ・コールバックを渡す。
// ファイル内部のみで使用し export しない。
interface TimelineColumnContextValue {
  // レイアウト
  startHour: number
  endHour: number
  totalMinutes: number
  hourHeight: number
  gridHeight: number
  hours: number[]
  // ユーティリティ
  getTopPx: (time: string) => number
  getHeightPx: (startTime: string, endTime: string) => number
  getDisplayTimes: (block: TimelineBlock) => { startTime: string; endTime: string }
  shouldSuppressClick: () => boolean
  // レンダリング
  overlayContext: OverlayRenderContext
  renderOverlay?: (columnId: string, context: OverlayRenderContext) => React.ReactNode
  renderDropIndicator?: (columnId: string, context: OverlayRenderContext) => React.ReactNode
  renderBlock?: (context: BlockRenderContext) => React.ReactNode
  // コールバック
  onBlockClick?: (blockId: string) => void
  onBlockDelete?: (blockId: string) => void
  onBlockResize?: (blockId: string, columnId: string, startTime: string, endTime: string) => void
  onDragStart: (e: React.MouseEvent, block: TimelineBlock) => void
  onResizeStart: (e: React.MouseEvent, block: TimelineBlock, edge: ResizeEdge) => void
  onCellClick?: (columnId: string, hour: number) => void
  onEmptyDoubleClick?: (columnId: string, startTime: string, endTime: string) => void
}

const TimelineColumnContext = createContext<TimelineColumnContextValue | null>(null)

function useTimelineColumnContext(): TimelineColumnContextValue {
  const ctx = useContext(TimelineColumnContext)
  if (!ctx) throw new Error('useTimelineColumnContext must be used within TimelineColumnContext.Provider')
  return ctx
}

export function TimelineCore({
  columns,
  blocks,
  startHour = 0,
  endHour = 24,
  baseHourHeight = 28,
  scale: propScale,
  onScaleChange,
  showZoomControls = true,
  onBlockClick,
  onBlockDelete,
  onBlockResize,
  onCellClick,
  onEmptyDoubleClick,
  isDragging = false,
  getDroppableId = (id) => `timeline-droppable-${id}`,
  getDroppableData = (id) => ({ columnId: id }),
  currentTimeMinutes,
  initialScrollHour,
  renderOverlay,
  renderDropIndicator,
  renderBlock,
}: TimelineCoreProps) {
  // ズーム状態
  const [internalScale, setInternalScale] = useState(1.0)
  const scale = propScale ?? internalScale
  const setScale = onScaleChange ?? setInternalScale
  const hourHeight = baseHourHeight * scale

  // ドラッグ・リサイズ状態
  const [dragResizeState, setDragResizeState] = useState<DragResizeState | null>(null)
  const [previewTime, setPreviewTime] = useState<PreviewTime | null>(null)
  // previewTime の最新値を ref で保持。useEffect 内の handleMouseUp が
  // 常に最新値を参照できるようにし、previewTime を依存配列から除外する。
  const previewTimeRef = useRef<PreviewTime | null>(null)
  const updatePreviewTime = useCallback((value: PreviewTime | null) => {
    previewTimeRef.current = value
    setPreviewTime(value)
  }, [])
  // onBlockResize を ref で保持（ドラッグ中にコールバック参照が変わっても
  // リスナーが再アタッチされないようにする）
  const onBlockResizeRef = useRef(onBlockResize)
  onBlockResizeRef.current = onBlockResize

  const columnRefs = useRef<Record<string, HTMLDivElement | null>>({})
  const scrollContainerRef = useRef<HTMLDivElement | null>(null)
  const justFinishedDragRef = useRef(false)

  const hours = useMemo(
    () => Array.from({ length: endHour - startHour }, (_, i) => startHour + i),
    [startHour, endHour]
  )
  const totalMinutes = (endHour - startHour) * 60
  const gridHeight = hours.length * hourHeight

  // 初期スクロール位置
  useEffect(() => {
    if (scrollContainerRef.current && initialScrollHour !== undefined && startHour < initialScrollHour) {
      const scrollPosition = (initialScrollHour - startHour) * hourHeight
      scrollContainerRef.current.scrollTop = scrollPosition
    }
  }, [startHour, hourHeight, initialScrollHour])

  // ホイールズーム（Ctrl+wheel）
  const handleWheel = useCallback((e: React.WheelEvent) => {
    if (e.ctrlKey || e.metaKey) {
      e.preventDefault()
      const delta = e.deltaY > 0 ? -SCALE_STEP : SCALE_STEP
      const newScale = Math.max(MIN_SCALE, Math.min(MAX_SCALE, scale + delta))
      if (newScale !== scale) {
        setScale(newScale)
      }
    }
  }, [scale, setScale])

  // ピクセル計算
  const getTopPx = useCallback((time: string): number => {
    const [h, m] = time.split(':').map(Number)
    const hours = h + m / 60
    return Math.max(0, (hours - startHour) * hourHeight)
  }, [startHour, hourHeight])

  const getHeightPx = useCallback((st: string, et: string): number => {
    const [sh, sm] = st.split(':').map(Number)
    const [eh, em] = et.split(':').map(Number)
    const startH = Math.max(startHour, sh + sm / 60)
    const endH = Math.min(endHour, eh + em / 60)
    return Math.max(hourHeight / 2, (endH - startH) * hourHeight)
  }, [startHour, endHour, hourHeight])

  // パーセンテージ計算（オーバーレイ用）
  const getTopPosition = useCallback((time: string): number => {
    const minutes = getMinutesFromTime(time) - startHour * 60
    return (minutes / totalMinutes) * 100
  }, [startHour, totalMinutes])

  const getHeight = useCallback((st: string, et: string): number => {
    const startMinutes = getMinutesFromTime(st)
    const endMinutes = getMinutesFromTime(et)
    const duration = endMinutes - startMinutes
    return (duration / totalMinutes) * 100
  }, [totalMinutes])

  // オーバーレイコンテキスト
  const overlayContext: OverlayRenderContext = useMemo(() => ({
    getTopPosition,
    getHeight,
    hourHeight,
    totalMinutes,
    startHour,
    endHour,
  }), [getTopPosition, getHeight, hourHeight, totalMinutes, startHour, endHour])

  // ドラッグ開始
  const handleDragStart = useCallback((e: React.MouseEvent, block: TimelineBlock) => {
    if (!onBlockResizeRef.current) return
    e.preventDefault()
    e.stopPropagation()
    const colEl = columnRefs.current[block.columnId]
    if (!colEl) return
    const duration = getMinutesFromTime(block.endTime) - getMinutesFromTime(block.startTime)
    // クリック位置とブロック開始位置の差（分）を事前計算
    const rect = colEl.getBoundingClientRect()
    const clickMinutes = yToMinutes(e.clientY, rect, startHour, endHour, 15)
    const blockStartMinutes = getMinutesFromTime(block.startTime)
    setDragResizeState({
      blockId: block.id,
      columnId: block.columnId,
      type: 'drag',
      initialStartTime: block.startTime,
      initialEndTime: block.endTime,
      duration,
      offsetFromStart: clickMinutes - blockStartMinutes,
    })
    updatePreviewTime({ startTime: block.startTime, endTime: block.endTime })
  }, [updatePreviewTime, startHour, endHour])

  // リサイズ開始
  const handleResizeStart = useCallback((e: React.MouseEvent, block: TimelineBlock, edge: ResizeEdge) => {
    if (!onBlockResizeRef.current) return
    e.preventDefault()
    e.stopPropagation()
    const duration = getMinutesFromTime(block.endTime) - getMinutesFromTime(block.startTime)
    setDragResizeState({
      blockId: block.id,
      columnId: block.columnId,
      type: 'resize',
      edge,
      initialStartTime: block.startTime,
      initialEndTime: block.endTime,
      duration,
      offsetFromStart: 0, // リサイズでは使用しない
    })
    updatePreviewTime({ startTime: block.startTime, endTime: block.endTime })
  }, [updatePreviewTime])

  // マウス移動・終了イベント
  // 依存配列から previewTime と onBlockResize を除外し、ref 経由で最新値を参照する。
  // これにより、リスナーはドラッグ開始時に1回だけ登録され、終了時に1回だけ除去される。
  useEffect(() => {
    if (!dragResizeState) return

    // ライブで getBoundingClientRect() を取得するヘルパー。
    // スクロールに追従するが、offsetFromStart は事前計算済みなのでドリフトしない。
    const getLiveRect = () => columnRefs.current[dragResizeState.columnId]?.getBoundingClientRect()
    const yToMin = (clientY: number) => {
      const rect = getLiveRect()
      if (!rect) return startHour * 60
      return yToMinutes(clientY, rect, startHour, endHour, 15)
    }

    const handleMouseMove = (e: MouseEvent) => {
      const newMinutes = yToMin(e.clientY)
      const initialStartMinutes = getMinutesFromTime(dragResizeState.initialStartTime)
      const initialEndMinutes = getMinutesFromTime(dragResizeState.initialEndTime)

      if (dragResizeState.type === 'resize') {
        let newStartMinutes = initialStartMinutes
        let newEndMinutes = initialEndMinutes

        if (dragResizeState.edge === 'top') {
          newStartMinutes = Math.min(newMinutes, initialEndMinutes - 15)
          newStartMinutes = Math.max(startHour * 60, newStartMinutes)
        } else {
          newEndMinutes = Math.max(newMinutes, initialStartMinutes + 15)
          newEndMinutes = Math.min(endHour * 60, newEndMinutes)
        }

        updatePreviewTime({
          startTime: minutesToTimeString(newStartMinutes),
          endTime: minutesToTimeString(newEndMinutes),
        })
      } else {
        // drag (move) — offsetFromStart はドラッグ開始時に事前計算済み
        let newStartMinutes = newMinutes - dragResizeState.offsetFromStart
        let newEndMinutes = newStartMinutes + dragResizeState.duration

        // 範囲内にクランプ
        if (newStartMinutes < startHour * 60) {
          newStartMinutes = startHour * 60
          newEndMinutes = newStartMinutes + dragResizeState.duration
        }
        if (newEndMinutes > endHour * 60) {
          newEndMinutes = endHour * 60
          newStartMinutes = newEndMinutes - dragResizeState.duration
        }

        newStartMinutes = snapToInterval(newStartMinutes, 15)
        newEndMinutes = newStartMinutes + dragResizeState.duration

        updatePreviewTime({
          startTime: minutesToTimeString(newStartMinutes),
          endTime: minutesToTimeString(newEndMinutes),
        })
      }
    }

    const handleMouseUp = () => {
      const currentPreview = previewTimeRef.current
      const resize = onBlockResizeRef.current
      if (dragResizeState && currentPreview && resize) {
        const hasChanged =
          currentPreview.startTime !== dragResizeState.initialStartTime ||
          currentPreview.endTime !== dragResizeState.initialEndTime
        if (hasChanged) {
          resize(dragResizeState.blockId, dragResizeState.columnId, currentPreview.startTime, currentPreview.endTime)
        }
      }
      setDragResizeState(null)
      updatePreviewTime(null)
      // ドラッグ終了後のクリックイベント抑制
      justFinishedDragRef.current = true
      setTimeout(() => { justFinishedDragRef.current = false }, 100)
    }

    window.addEventListener('mousemove', handleMouseMove)
    window.addEventListener('mouseup', handleMouseUp)
    document.body.style.cursor = dragResizeState.type === 'resize' ? 'ns-resize' : 'grabbing'
    document.body.style.userSelect = 'none'

    return () => {
      window.removeEventListener('mousemove', handleMouseMove)
      window.removeEventListener('mouseup', handleMouseUp)
      document.body.style.cursor = ''
      document.body.style.userSelect = ''
    }
  }, [dragResizeState, updatePreviewTime, startHour, endHour])

  // 表示用の時間を取得
  const getDisplayTimes = useCallback((block: TimelineBlock) => {
    if (dragResizeState?.blockId === block.id && previewTime) {
      return previewTime
    }
    return { startTime: block.startTime, endTime: block.endTime }
  }, [dragResizeState, previewTime])

  // クリック抑制チェック
  const shouldSuppressClick = useCallback(() => justFinishedDragRef.current, [])

  // カラムごとにブロックをグループ化
  const blocksByColumn = useMemo(() => {
    const map: Record<string, TimelineBlock[]> = {}
    for (const col of columns) {
      map[col.id] = blocks.filter(b => b.columnId === col.id)
    }
    return map
  }, [columns, blocks])

  // Context value（全カラム共通の設定・ユーティリティ・コールバック）
  const contextValue: TimelineColumnContextValue = useMemo(() => ({
    startHour,
    endHour,
    totalMinutes,
    hourHeight,
    gridHeight,
    hours,
    getTopPx,
    getHeightPx,
    getDisplayTimes,
    shouldSuppressClick,
    overlayContext,
    renderOverlay,
    renderDropIndicator,
    renderBlock,
    onBlockClick,
    onBlockDelete,
    onBlockResize,
    onDragStart: handleDragStart,
    onResizeStart: handleResizeStart,
    onCellClick,
    onEmptyDoubleClick,
  }), [
    startHour, endHour, totalMinutes, hourHeight, gridHeight, hours,
    getTopPx, getHeightPx, getDisplayTimes, shouldSuppressClick,
    overlayContext, renderOverlay, renderDropIndicator, renderBlock,
    onBlockClick, onBlockDelete, onBlockResize,
    handleDragStart, handleResizeStart,
    onCellClick, onEmptyDoubleClick,
  ])

  return (
    <div className="relative">
      {/* ズームコントロール */}
      {showZoomControls && (
        <div className="absolute top-2 right-2 z-30 flex items-center gap-0.5 bg-background/95 border rounded-md shadow-sm">
          <Button
            variant="ghost"
            size="sm"
            className="h-6 w-6 p-0"
            onClick={() => setScale(Math.max(MIN_SCALE, scale - SCALE_STEP))}
            disabled={scale <= MIN_SCALE}
            title="縮小"
          >
            <Minus className="h-3 w-3" />
          </Button>
          <button
            className="px-1.5 text-xs text-muted-foreground hover:text-foreground min-w-[40px] text-center"
            onClick={() => setScale(1.0)}
            title="リセット (100%)"
          >
            {Math.round(scale * 100)}%
          </button>
          <Button
            variant="ghost"
            size="sm"
            className="h-6 w-6 p-0"
            onClick={() => setScale(Math.min(MAX_SCALE, scale + SCALE_STEP))}
            disabled={scale >= MAX_SCALE}
            title="拡大"
          >
            <Plus className="h-3 w-3" />
          </Button>
        </div>
      )}

      {/* スクロールコンテナ */}
      <div
        ref={scrollContainerRef}
        className="relative flex overflow-y-auto overflow-x-auto"
        style={{ maxHeight: `${12 * baseHourHeight}px` }}
        onWheel={handleWheel}
      >
        {/* 時間軸ラベル */}
        <div className="w-8 flex-shrink-0 pt-8 sticky left-0 bg-background z-10">
          {hours.map(hour => (
            <div
              key={hour}
              className="text-[10px] text-muted-foreground/60 text-right pr-1 leading-none"
              style={{ height: hourHeight }}
            >
              {hour}
            </div>
          ))}
        </div>

        {/* カラム */}
        <TimelineColumnContext.Provider value={contextValue}>
          <div className="flex-1 flex gap-0.5 min-w-0">
            {columns.map(col => (
              <TimelineColumnComponent
                key={col.id}
                column={col}
                blocks={blocksByColumn[col.id] || []}
                isDragging={isDragging}
                isResizing={dragResizeState?.columnId === col.id}
                droppableId={getDroppableId(col.id)}
                droppableData={getDroppableData(col.id)}
                setColumnRef={(el) => { columnRefs.current[col.id] = el }}
                currentTimeMinutes={currentTimeMinutes}
              />
            ))}
          </div>
        </TimelineColumnContext.Provider>
      </div>
    </div>
  )
}

// ---------- TimelineColumn (カラム固有のデータのみ props で受け取る) ----------

interface TimelineColumnComponentProps {
  column: TimelineColumn
  blocks: TimelineBlock[]
  isDragging: boolean
  isResizing: boolean
  droppableId: string
  droppableData: Record<string, unknown>
  setColumnRef: (el: HTMLDivElement | null) => void
  currentTimeMinutes?: number
}

function TimelineColumnComponent({
  column,
  blocks,
  isDragging,
  isResizing,
  droppableId,
  droppableData,
  setColumnRef,
  currentTimeMinutes,
}: TimelineColumnComponentProps) {
  const {
    startHour,
    endHour,
    totalMinutes,
    hourHeight,
    gridHeight,
    hours,
    getTopPx,
    getHeightPx,
    getDisplayTimes,
    shouldSuppressClick,
    overlayContext,
    renderOverlay,
    renderDropIndicator,
    renderBlock,
    onBlockClick,
    onBlockDelete,
    onBlockResize,
    onDragStart,
    onResizeStart,
    onCellClick,
    onEmptyDoubleClick,
  } = useTimelineColumnContext()

  const { setNodeRef, isOver } = useDroppable({
    id: droppableId,
    data: droppableData,
  })

  const setRefs = useCallback((node: HTMLDivElement | null) => {
    setNodeRef(node)
    setColumnRef(node)
  }, [setNodeRef, setColumnRef])

  const handleCellClick = (e: React.MouseEvent<HTMLDivElement>) => {
    if (!onCellClick) return
    // ドラッグ・リサイズ直後のクリックを抑制
    if (shouldSuppressClick()) return
    const rect = e.currentTarget.getBoundingClientRect()
    const y = e.clientY - rect.top
    const hour = Math.floor(y / hourHeight) + hours[0]
    onCellClick(column.id, hour)
  }

  const handleDoubleClick = (e: React.MouseEvent<HTMLDivElement>) => {
    if (!onEmptyDoubleClick) return
    // ブロック上のダブルクリックは無視
    const target = e.target as HTMLElement
    if (target.closest('[data-block]')) return
    const rect = e.currentTarget.getBoundingClientRect()
    const { startTime, endTime } = calculateTimeFromY(e.clientY, rect, startHour, endHour)
    onEmptyDoubleClick(column.id, startTime, endTime)
  }

  // 現在時刻インジケーター
  const showCurrentTime = column.isToday && currentTimeMinutes !== undefined
  const currentTimeTop = showCurrentTime
    ? ((currentTimeMinutes - startHour * 60) / totalMinutes) * 100
    : 0
  const isCurrentTimeInRange = showCurrentTime &&
    currentTimeMinutes >= startHour * 60 &&
    currentTimeMinutes <= endHour * 60

  return (
    <div className="flex-1 min-w-[70px]">
      {/* ヘッダー */}
      <div className={cn(
        'text-center pb-1 mb-1 border-b h-8',
        column.isToday && 'border-brand'
      )}>
        {column.header}
      </div>

      {/* グリッドエリア */}
      <div
        ref={setRefs}
        data-timeline-container
        data-start-hour={startHour}
        data-end-hour={endHour}
        className={cn(
          'relative bg-muted/20 rounded cursor-pointer transition-colors',
          !isDragging && !isResizing && 'hover:bg-muted/40',
          isDragging && 'ring-2 ring-dashed ring-primary/30',
          isOver && 'bg-primary/10 ring-primary'
        )}
        style={{ height: gridHeight }}
        onClick={handleCellClick}
        onDoubleClick={handleDoubleClick}
      >
        {/* 時間線 */}
        {hours.map((hour, idx) => (
          <div
            key={hour}
            className="absolute left-0 right-0 border-t border-muted/40 pointer-events-none"
            style={{ top: idx * hourHeight }}
          />
        ))}

        {/* 現在時刻インジケーター */}
        {isCurrentTimeInRange && (
          <div
            className="absolute left-0 right-0 z-30 pointer-events-none"
            style={{ top: `${currentTimeTop}%` }}
          >
            <div className="absolute -left-1 -top-[5px] w-[10px] h-[10px] rounded-full bg-brand shadow-[0_0_6px_hsl(var(--brand)/0.5)]" />
            <div className="absolute left-0 right-0 border-t-2 border-dashed" style={{ borderColor: 'hsl(var(--brand))' }} />
          </div>
        )}

        {/* ドロップインジケーター */}
        {isOver && (
          <div className="absolute inset-0 flex items-center justify-center pointer-events-none">
            <div className="text-xs text-primary font-medium bg-primary/10 px-2 py-1 rounded">
              ここに配置
            </div>
          </div>
        )}

        {/* ブロック */}
        {blocks.map(block => {
          const displayTimes = getDisplayTimes(block)
          const top = getTopPx(displayTimes.startTime)
          const height = getHeightPx(displayTimes.startTime, displayTimes.endTime)

          // カスタムレンダラーが指定されている場合はそれを使用
          if (renderBlock) {
            return (
              <div key={block.id}>
                {renderBlock({
                  block,
                  displayTimes,
                  isResizing,
                  getTopPx,
                  getHeightPx,
                  onDragStart: (e) => onDragStart(e, block),
                  onResizeStart: (e, edge) => onResizeStart(e, block, edge),
                  shouldSuppressClick,
                })}
              </div>
            )
          }

          // デフォルトのシンプルな表示
          return (
            <div
              key={block.id}
              data-block
              className={cn(
                'absolute left-0.5 right-0.5 rounded-sm overflow-hidden text-[9px] px-1 leading-tight transition-opacity group',
                block.color ? 'text-white' : 'text-foreground bg-muted border',
                onBlockResize && 'cursor-grab active:cursor-grabbing'
              )}
              style={{
                top,
                height,
                backgroundColor: block.color || undefined,
              }}
              title={`${block.label} (${displayTimes.startTime}-${displayTimes.endTime})`}
              onClick={(e) => {
                e.stopPropagation()
                if (!shouldSuppressClick()) {
                  onBlockClick?.(block.id)
                }
              }}
            >
              {/* 上部リサイズハンドル */}
              {onBlockResize && (
                <div
                  className="absolute left-0 right-0 top-0 h-1.5 cursor-ns-resize hover:bg-white/20 z-10"
                  onMouseDown={(e) => onResizeStart(e, block, 'top')}
                />
              )}

              {/* 削除ボタン */}
              {onBlockDelete && (
                <Button
                  variant="ghost"
                  size="icon"
                  className="absolute top-0 right-0 h-4 w-4 p-0 opacity-0 group-hover:opacity-100 transition-opacity bg-black/20 hover:bg-destructive hover:text-destructive-foreground rounded-sm z-20"
                  onClick={(e) => {
                    e.stopPropagation()
                    if (confirm('このタイムブロックを削除しますか？')) {
                      onBlockDelete(block.id)
                    }
                  }}
                >
                  <Trash2 className="h-2.5 w-2.5" />
                </Button>
              )}

              {/* コンテンツ（ドラッグエリア） */}
              <div
                className={cn('truncate font-medium pt-0.5', onBlockResize && 'cursor-grab')}
                onMouseDown={(e) => onDragStart(e, block)}
              >
                {block.label}
              </div>
              {height >= hourHeight && block.sublabel && (
                <div className="truncate opacity-80 text-[8px]">
                  {block.sublabel}
                </div>
              )}

              {/* 下部リサイズハンドル */}
              {onBlockResize && (
                <div
                  className="absolute left-0 right-0 bottom-0 h-1.5 cursor-ns-resize hover:bg-white/20 z-10"
                  onMouseDown={(e) => onResizeStart(e, block, 'bottom')}
                />
              )}
            </div>
          )
        })}

        {/* カスタムオーバーレイ（実績、タイマー等） */}
        {renderOverlay?.(column.id, overlayContext)}

        {/* カスタムドロップインジケーター */}
        {renderDropIndicator?.(column.id, overlayContext)}
      </div>
    </div>
  )
}
