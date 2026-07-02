/**
 * TimeBlockTimeline - Daily ページ用タイムラインコンポーネント
 *
 * TimelineCore を使用し、Daily 固有の機能（実績表示、タイマー表示等）を追加
 */
import { useState, useCallback, useEffect, useMemo } from 'react'
import type { TimeBlock, TimeEntry } from '@/types'
import { cn } from '@/lib/utils'
import {
  TimelineCore,
  getMinutesFromTime,
  minutesToTimeString,
  snapToInterval,
  formatTime,
  calculateTimeFromY,
  calculateTimeFromYWithDuration,
} from './timeline'
import type { TimelineBlock as CoreBlock, TimelineColumn, OverlayRenderContext, RunningTimerData, BlockRenderContext } from './timeline'
import { TimelineItemContent } from './timeline/TimelineItemContent'
import type { ActionButton } from './timeline/TimelineItemContent'
import { getLocalTime, getLocalDate } from '@/lib/timezone'
import { useSettingsStore } from '@/stores/useSettingsStore'

interface TimeBlockTimelineProps {
  timeBlocks?: TimeBlock[]
  timeEntries?: TimeEntry[]
  showComparison?: boolean
  startHour?: number
  endHour?: number
  onBlockClick?: (block: TimeBlock) => void
  onBlockDelete?: (blockId: string) => void
  onBlockResize?: (blockId: string, startTime: string, endTime: string) => void
  onBlockStartTimer?: (block: TimeBlock) => void
  onEntryClick?: (entry: TimeEntry) => void
  onEntryDelete?: (entryId: string) => void
  onEmptyDoubleClick?: (startTime: string, endTime: string) => void
  isDraggingTask?: boolean
  dragOverY?: number | null
  dragDurationMinutes?: number
  isTimerRunning?: boolean
  runningTimer?: RunningTimerData | null
  scale?: number
  onScaleChange?: (scale: number) => void
  isToday?: boolean
}

// Daily 用定数
const BASE_HOUR_HEIGHT = 48
const COLUMN_ID = 'daily'

export function TimeBlockTimeline({
  timeBlocks = [],
  timeEntries = [],
  showComparison = false,
  startHour = 0,
  endHour = 24,
  onBlockClick,
  onBlockDelete,
  onBlockResize,
  onBlockStartTimer,
  onEntryClick,
  onEntryDelete,
  onEmptyDoubleClick,
  isDraggingTask = false,
  dragOverY = null,
  dragDurationMinutes = 60,
  isTimerRunning: _isTimerRunning, // Used for styling, currently unused
  runningTimer = null,
  scale,
  onScaleChange,
  isToday = true,
}: TimeBlockTimelineProps) {
  const [currentTime, setCurrentTime] = useState<Date>(new Date())
  const timezone = useSettingsStore((s) => s.timezone) || 'Asia/Tokyo'

  // Helper to extract local time from an ISO datetime
  const toLocalTime = useCallback((isoDatetime: string) => getLocalTime(isoDatetime, timezone), [timezone])

  // Update current time (1 second when timer running, 1 minute otherwise)
  useEffect(() => {
    const updateCurrentTime = () => setCurrentTime(new Date())
    const intervalMs = runningTimer ? 1000 : 60000
    const interval = setInterval(updateCurrentTime, intervalMs)
    return () => clearInterval(interval)
  }, [runningTimer])

  // Calculate running timer entry times
  const runningEntryTimes = runningTimer
    ? {
        startTime: toLocalTime(runningTimer.startedAt),
        endTime: toLocalTime(currentTime.toISOString()),
      }
    : null

  const totalMinutes = (endHour - startHour) * 60

  // 現在時刻（分）
  const currentLocalTime = toLocalTime(currentTime.toISOString())
  const currentTimeMinutes = getMinutesFromTime(currentLocalTime)

  // TimeBlock から blockId → TimeBlock のマップ
  const blockMap = useMemo(() => {
    const map = new Map<string, TimeBlock>()
    for (const block of timeBlocks) {
      map.set(block.id, block)
    }
    return map
  }, [timeBlocks])

  // TimelineCore 用のカラム（Daily は単一カラム）
  const columns: TimelineColumn[] = useMemo(() => [{
    id: COLUMN_ID,
    header: null, // Daily はヘッダー不要
    isToday,
  }], [isToday])

  // TimeBlock を TimelineCore 用に変換
  const coreBlocks: CoreBlock[] = useMemo(() => {
    return timeBlocks.map(block => {
      const startTime = toLocalTime(block.startDatetime)
      const endTime = toLocalTime(block.endDatetime)
      return {
        id: block.id,
        columnId: COLUMN_ID,
        startTime,
        endTime,
        label: block.taskName,
        sublabel: block.milestoneName,
        color: block.goalColor,
      }
    })
  }, [timeBlocks, toLocalTime])

  // コールバックの安定化: ドラッグ中の再レンダリングでグローバルイベントリスナーが
  // 再アタッチされるのを防止する
  const handleBlockClick = useCallback(
    (blockId: string) => {
      const block = blockMap.get(blockId)
      if (block) {
        onBlockClick?.(block)
      }
    },
    [blockMap, onBlockClick]
  )

  // onBlockResize をラップ（columnId は無視）
  const handleBlockResize = useCallback(
    (blockId: string, _columnId: string, startTime: string, endTime: string) => {
      onBlockResize?.(blockId, startTime, endTime)
    },
    [onBlockResize]
  )

  // onEmptyDoubleClick をラップ（columnId は無視）
  const handleEmptyDoubleClick = useCallback(
    (_columnId: string, startTime: string, endTime: string) => {
      onEmptyDoubleClick?.(startTime, endTime)
    },
    [onEmptyDoubleClick]
  )

  // 実績・タイマー用オーバーレイ
  const renderOverlay = useCallback((_columnId: string, ctx: OverlayRenderContext) => {
    if (!showComparison) return null

    return (
      <>
        {/* Time entries (actuals) */}
        {timeEntries.map((entry) => {
          const hasGoal = !!(entry.goalId && entry.goalColor)
          const entryStartTime = toLocalTime(entry.startDatetime)
          const entryEndTime = toLocalTime(entry.endDatetime)
          const entryCrossesMidnight = getLocalDate(entry.startDatetime, timezone) !== getLocalDate(entry.endDatetime, timezone)

          return (
            <div
              key={entry.id}
              data-block
              className={cn(
                'absolute left-[52%] right-1 rounded-md px-2 py-1 text-xs group overflow-hidden',
                !hasGoal && 'border border-dashed border-muted-foreground/40'
              )}
              style={{
                backgroundColor: hasGoal
                  ? `color-mix(in srgb, ${entry.goalColor} 12%, transparent)`
                  : 'hsl(var(--muted))',
                top: `${ctx.getTopPosition(entryStartTime)}%`,
                height: `${ctx.getHeight(entryStartTime, entryEndTime)}%`,
                minHeight: '24px',
              }}
            >
              {hasGoal && (
                <div
                  className="absolute left-0 top-0 bottom-0 w-1 rounded-l-md"
                  style={{ backgroundColor: entry.goalColor }}
                />
              )}

              <div className={cn('relative', hasGoal && 'pl-1')}>
                <TimelineItemContent
                  taskName={entry.taskName}
                  goalId={entry.goalId}
                  goalName={entry.goalName}
                  goalColor={entry.goalColor}
                  milestoneName={entry.milestoneName}
                  startTimeLabel={formatTime(entryStartTime)}
                  endTimeLabel={formatTime(entryEndTime)}
                  crossesMidnight={entryCrossesMidnight}
                  actions={(() => {
                    const a: ActionButton[] = []
                    if (onEntryClick) a.push({ type: 'edit', onClick: () => onEntryClick(entry) })
                    if (onEntryDelete) a.push({ type: 'delete', onClick: () => onEntryDelete(entry.id), confirmMessage: 'この実績を削除しますか？' })
                    return a.length > 0 ? a : undefined
                  })()}
                />
              </div>
            </div>
          )
        })}

        {/* Running timer (virtual entry) */}
        {runningTimer && runningEntryTimes && (() => {
          const hasGoal = !!(runningTimer.goalId && runningTimer.goalColor)
          const heightPercent = ctx.getHeight(runningEntryTimes.startTime, runningEntryTimes.endTime)
          const effectiveHeight = Math.max(heightPercent, 2)

          return (
            <div
              data-block
              className={cn(
                'absolute left-[52%] right-1 rounded-md px-2 py-1 text-xs overflow-hidden',
                'animate-pulse',
                'ring-2 ring-primary/50',
                !hasGoal && 'border border-dashed border-muted-foreground/40'
              )}
              style={{
                backgroundColor: hasGoal
                  ? `color-mix(in srgb, ${runningTimer.goalColor} 20%, transparent)`
                  : 'hsl(var(--primary) / 0.15)',
                top: `${ctx.getTopPosition(runningEntryTimes.startTime)}%`,
                height: `${effectiveHeight}%`,
                minHeight: '24px',
              }}
            >
              {hasGoal && (
                <div
                  className="absolute left-0 top-0 bottom-0 w-1 rounded-l-md"
                  style={{ backgroundColor: runningTimer.goalColor }}
                />
              )}

              <div className={cn('relative', hasGoal && 'pl-1')}>
                <TimelineItemContent
                  taskName={runningTimer.taskName}
                  goalId={runningTimer.goalId}
                  goalName={runningTimer.goalName}
                  goalColor={runningTimer.goalColor}
                  milestoneName={runningTimer.milestoneName}
                  startTimeLabel={formatTime(runningEntryTimes.startTime)}
                  endTimeLabel="作業中"
                  noGoalLabel="作業中"
                  trailingBadge={
                    <span className="text-primary text-[10px] font-medium flex-shrink-0">REC</span>
                  }
                />
              </div>
            </div>
          )
        })()}
      </>
    )
  }, [showComparison, timeEntries, toLocalTime, onEntryClick, onEntryDelete, runningTimer, runningEntryTimes])

  // カスタムブロックレンダラー（タイマー開始ボタン等を含む）
  const renderBlock = useCallback((ctx: BlockRenderContext) => {
    const originalBlock = blockMap.get(ctx.block.id)
    if (!originalBlock) return null

    const hasGoal = !!(originalBlock.goalId && originalBlock.goalColor)
    const blockCrossesMidnight = getLocalDate(originalBlock.startDatetime, timezone) !== getLocalDate(originalBlock.endDatetime, timezone)
    const isTimerRunning = !!runningTimer
    const { column, totalColumns } = ctx.overlapLayout

    // アクションボタン
    const actions: ActionButton[] = []
    if (onBlockStartTimer && !isTimerRunning) {
      actions.push({
        type: 'timer',
        onClick: () => {
          if (ctx.shouldSuppressClick()) return
          onBlockStartTimer(originalBlock)
        },
      })
    }
    if (onBlockClick) {
      actions.push({
        type: 'edit',
        onClick: () => {
          if (ctx.shouldSuppressClick()) return
          onBlockClick(originalBlock)
        },
      })
    }
    if (onBlockDelete) {
      actions.push({
        type: 'delete',
        onClick: () => {
          if (ctx.shouldSuppressClick()) return
          onBlockDelete(originalBlock.id)
        },
        confirmMessage: 'このタイムブロックを削除しますか？',
      })
    }

    // 重複レイアウト: サブカラム配置のCSS計算
    const pad = 4 // px (left-1 / right-1 相当)
    const gap = totalColumns > 1 ? 1 : 0 // px
    // comparison モード: 左48%のみ使用、通常: 全幅使用
    const availableWidth = showComparison ? '48%' : `calc(100% - ${pad}px)`
    const colLeft = `calc(${pad}px + ${column} * ${availableWidth} / ${totalColumns} + ${gap}px)`
    const colWidth = `calc(${availableWidth} / ${totalColumns} - ${gap * 2}px)`

    return (
      <div
        data-block
        aria-label={`${originalBlock.taskName || 'Time block'} ${formatTime(ctx.displayTimes.startTime)}-${formatTime(ctx.displayTimes.endTime)}`}
        className={cn(
          'absolute rounded-md px-2 py-1 text-xs group overflow-hidden',
          onBlockResize && 'cursor-grab',
          ctx.isResizing && 'cursor-grabbing',
          !hasGoal && 'border border-dashed border-muted-foreground/40'
        )}
        style={{
          backgroundColor: hasGoal
            ? `color-mix(in srgb, ${originalBlock.goalColor} 12%, transparent)`
            : 'hsl(var(--muted))',
          top: `${ctx.getTopPx(ctx.displayTimes.startTime)}px`,
          height: `${ctx.getHeightPx(ctx.displayTimes.startTime, ctx.displayTimes.endTime)}px`,
          minHeight: '24px',
          left: colLeft,
          width: colWidth,
        }}
      >
        {/* Goal color left border */}
        {hasGoal && (
          <div
            className="absolute left-0 top-0 bottom-0 w-1 rounded-l-md"
            style={{ backgroundColor: originalBlock.goalColor }}
          />
        )}

        {/* Top resize handle */}
        {onBlockResize && (
          <div
            className="absolute left-0 right-0 top-0 h-2 cursor-ns-resize group/handle hover:bg-primary/20 rounded-t-md"
            onMouseDown={(e) => ctx.onResizeStart(e, 'top')}
          >
            <div className="absolute left-1/2 -translate-x-1/2 top-0.5 w-8 h-1 bg-muted-foreground/30 rounded-full opacity-0 group-hover/handle:opacity-100" />
          </div>
        )}

        {/* Content area */}
        <div
          className={cn(
            'relative',
            hasGoal && 'pl-1',
            onBlockResize && 'cursor-grab active:cursor-grabbing'
          )}
          onMouseDown={ctx.onDragStart}
        >
          <TimelineItemContent
            taskName={originalBlock.taskName}
            goalId={originalBlock.goalId}
            goalName={originalBlock.goalName}
            goalColor={originalBlock.goalColor}
            milestoneName={originalBlock.milestoneName}
            startTimeLabel={formatTime(ctx.displayTimes.startTime)}
            endTimeLabel={formatTime(ctx.displayTimes.endTime)}
            crossesMidnight={blockCrossesMidnight}
            actions={actions.length > 0 ? actions : undefined}
          />
        </div>

        {/* Bottom resize handle */}
        {onBlockResize && (
          <div
            className="absolute left-0 right-0 bottom-0 h-2 cursor-ns-resize group/handle hover:bg-primary/20 rounded-b-md"
            onMouseDown={(e) => ctx.onResizeStart(e, 'bottom')}
          >
            <div className="absolute left-1/2 -translate-x-1/2 bottom-0.5 w-8 h-1 bg-muted-foreground/30 rounded-full opacity-0 group-hover/handle:opacity-100" />
          </div>
        )}
      </div>
    )
  }, [blockMap, showComparison, runningTimer, onBlockClick, onBlockDelete, onBlockResize, onBlockStartTimer, timezone])

  // ドロップインジケーター
  const renderDropIndicator = useCallback((_columnId: string, ctx: OverlayRenderContext) => {
    if (!isDraggingTask || dragOverY === null) return null

    // コンテナ要素を取得
    const container = document.querySelector('[data-timeline-container]')
    if (!container) return null

    const rect = container.getBoundingClientRect()
    const relativeY = Math.max(0, Math.min(dragOverY - rect.top, rect.height))
    const percentage = relativeY / rect.height
    const rawMinutes = percentage * totalMinutes + startHour * 60
    const snappedMinutes = snapToInterval(
      Math.max(startHour * 60, Math.min(endHour * 60 - dragDurationMinutes, rawMinutes))
    )
    const timeString = minutesToTimeString(snappedMinutes)
    const topPercentage = ((snappedMinutes - startHour * 60) / totalMinutes) * 100
    const indicatorHeight = ctx.hourHeight * (dragDurationMinutes / 60)

    return (
      <div
        className="absolute left-0 right-0 pointer-events-none z-20"
        style={{ top: `${topPercentage}%` }}
      >
        <div className="absolute left-0 right-0 h-0.5 bg-primary" />
        <div className="absolute -left-16 -top-2.5 bg-primary text-primary-foreground text-xs px-1.5 py-0.5 rounded font-medium">
          {timeString}
        </div>
        <div
          className="absolute left-1 right-1 rounded-md border-2 border-dashed border-primary bg-primary/10"
          style={{
            height: `${indicatorHeight}px`,
            minHeight: `${indicatorHeight}px`,
          }}
        />
      </div>
    )
  }, [isDraggingTask, dragOverY, totalMinutes, startHour, endHour, dragDurationMinutes])

  return (
    <div role="grid" aria-label="Time Block Schedule">
      <TimelineCore
        columns={columns}
        blocks={coreBlocks}
        startHour={startHour}
        endHour={endHour}
        baseHourHeight={BASE_HOUR_HEIGHT}
        scale={scale}
        onScaleChange={onScaleChange}
        onBlockClick={handleBlockClick}
        onBlockDelete={onBlockDelete}
        onBlockResize={handleBlockResize}
        onEmptyDoubleClick={handleEmptyDoubleClick}
        isDragging={isDraggingTask}
        getDroppableId={() => 'timeblock-timeline-droppable'}
        getDroppableData={() => ({})}
        currentTimeMinutes={isToday ? currentTimeMinutes : undefined}
        initialScrollHour={8}
        renderOverlay={renderOverlay}
        renderDropIndicator={renderDropIndicator}
        renderBlock={renderBlock}
      />
    </div>
  )
}

// Re-export for external use
export { calculateTimeFromY, calculateTimeFromYWithDuration }
