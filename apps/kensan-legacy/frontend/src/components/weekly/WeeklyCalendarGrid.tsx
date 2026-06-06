import { useMemo } from 'react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Calendar } from 'lucide-react'
import { formatDayOfWeekJa, formatDayOfMonth, formatDateIso } from '@/lib/dateFormat'
import { getLocalTime } from '@/lib/timezone'
import { useSettingsStore } from '@/stores/useSettingsStore'
import { cn } from '@/lib/utils'
import type { TimeBlock } from '@/types'

// TimelineCore を使用
import { TimelineCore, calculateTimeFromYWithDuration } from '@/components/common/timeline'
import type { TimelineBlock, TimelineColumn } from '@/components/common/timeline'

interface WeeklyCalendarGridProps {
  blocksByDate: Record<string, TimeBlock[]>
  weekDates: string[]
  onCellClick?: (date: string, startHour: number) => void
  onBlockClick?: (block: TimeBlock) => void
  onBlockDelete?: (blockId: string) => void
  onBlockResize?: (blockId: string, date: string, startTime: string, endTime: string) => void
  isDraggingTask?: boolean
}

// 週間カレンダー固有の定数
const START_HOUR = 0
const END_HOUR = 24
const SNAP_INTERVAL = 15

// Y座標から時間を計算（エクスポート - 親コンポーネントで使用）
export function calculateTimeFromYWeekly(
  clientY: number,
  rect: DOMRect,
  durationMinutes: number = 60
): { startTime: string; endTime: string } {
  return calculateTimeFromYWithDuration(clientY, rect, START_HOUR, END_HOUR, durationMinutes, SNAP_INTERVAL)
}

export function WeeklyCalendarGrid({
  blocksByDate,
  weekDates,
  onCellClick,
  onBlockClick,
  onBlockDelete,
  onBlockResize,
  isDraggingTask = false,
}: WeeklyCalendarGridProps) {
  const { timezone } = useSettingsStore()
  const todayIso = formatDateIso(new Date())

  // TimeBlock[] から blockId → TimeBlock のマップを作成
  const blockMap = useMemo(() => {
    const map = new Map<string, TimeBlock>()
    for (const blocks of Object.values(blocksByDate)) {
      for (const block of blocks) {
        map.set(block.id, block)
      }
    }
    return map
  }, [blocksByDate])

  // weekDates を TimelineColumn[] に変換
  const columns: TimelineColumn[] = useMemo(() => {
    return weekDates.map(dateStr => {
      const date = new Date(dateStr + 'T00:00:00')
      const isToday = dateStr === todayIso
      return {
        id: dateStr,
        isToday,
        header: (
          <div className={cn(isToday && 'bg-brand/10')}>
            <div className={cn(
              'text-xs',
              isToday ? 'text-brand font-medium' : 'text-muted-foreground'
            )}>
              {formatDayOfWeekJa(date)}
            </div>
            <div className={cn(
              'text-sm font-semibold inline-flex items-center justify-center',
              isToday && 'bg-brand text-white rounded-full w-6 h-6'
            )}>
              {formatDayOfMonth(date)}
            </div>
          </div>
        ),
      }
    })
  }, [weekDates, todayIso])

  // blocksByDate を TimelineBlock[] に変換
  const blocks: TimelineBlock[] = useMemo(() => {
    const result: TimelineBlock[] = []
    for (const [dateStr, dateBlocks] of Object.entries(blocksByDate)) {
      for (const block of dateBlocks) {
        const startTime = getLocalTime(block.startDatetime, timezone)
        const endTime = getLocalTime(block.endDatetime, timezone)
        result.push({
          id: block.id,
          columnId: dateStr,
          startTime,
          endTime,
          label: block.taskName,
          sublabel: `${startTime}-${endTime}`,
          color: block.goalColor,
        })
      }
    }
    return result
  }, [blocksByDate, timezone])

  // onBlockClick をラップ（blockId → TimeBlock）
  const handleBlockClick = onBlockClick
    ? (blockId: string) => {
        const block = blockMap.get(blockId)
        if (block) {
          onBlockClick(block)
        }
      }
    : undefined

  // onBlockResize をラップ（columnId を date として使用）
  const handleBlockResize = onBlockResize
    ? (blockId: string, columnId: string, startTime: string, endTime: string) => {
        onBlockResize(blockId, columnId, startTime, endTime)
      }
    : undefined

  return (
    <Card>
      <CardHeader className="pb-2">
        <CardTitle className="text-sm font-medium flex items-center gap-2">
          <Calendar className="h-4 w-4" />
          週間カレンダー
          {isDraggingTask && (
            <span className="text-xs text-primary ml-auto animate-pulse">
              ドロップして配置
            </span>
          )}
        </CardTitle>
      </CardHeader>
      <CardContent className="pt-0">
        <TimelineCore
          columns={columns}
          blocks={blocks}
          startHour={START_HOUR}
          endHour={END_HOUR}
          baseHourHeight={40}
          onCellClick={onCellClick}
          onBlockClick={handleBlockClick}
          onBlockDelete={onBlockDelete}
          onBlockResize={handleBlockResize}
          isDragging={isDraggingTask}
          getDroppableId={(columnId) => `weekly-droppable-${columnId}`}
          getDroppableData={(columnId) => ({ date: columnId })}
        />
      </CardContent>
    </Card>
  )
}
