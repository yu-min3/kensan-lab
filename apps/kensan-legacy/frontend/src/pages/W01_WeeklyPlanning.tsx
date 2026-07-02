import { useState, useEffect, useMemo, useCallback } from 'react'
import { Sheet, SheetContent } from '@/components/ui/sheet'
import { TaskDetailPanel } from '@/components/task/TaskDetailPanel'
import { TimeBlockDialog } from '@/components/common/TimeBlockDialog'
import { useTaskDetailPanel } from '@/hooks/useTaskDetailPanel'
import { useTimeBlockDialog } from '@/hooks/useTimeBlockDialog'
import { WeeklyHeader } from '@/components/weekly/WeeklyHeader'
import { WeeklySummaryBar } from '@/components/weekly/WeeklySummaryBar'
import { WeeklyCalendarGrid, calculateTimeFromYWeekly } from '@/components/weekly/WeeklyCalendarGrid'
import { WeeklyTaskCards } from '@/components/weekly/WeeklyTaskCards'
import { useTimeBlockStore } from '@/stores/useTimeBlockStore'
import { useTaskManagerStore } from '@/stores/useTaskManagerStore'
import { useSettingsStore } from '@/stores/useSettingsStore'
import { getLocalDate } from '@/lib/timezone'
import { formatDateIso } from '@/lib/dateFormat'
import {
  DndContext,
  DragOverlay,
  useSensor,
  useSensors,
  PointerSensor,
  type DragStartEvent,
  type DragEndEvent,
} from '@dnd-kit/core'
import { PageGuide } from '@/components/guide/PageGuide'
import type { TimeBlock, TimeEntry } from '@/types'
import type { TaskDragData } from '@/components/daily/TaskListWidget'

function getMonday(date: Date): Date {
  const d = new Date(date)
  const dayOfWeek = d.getDay()
  const mondayOffset = dayOfWeek === 0 ? -6 : 1 - dayOfWeek
  d.setDate(d.getDate() + mondayOffset)
  d.setHours(0, 0, 0, 0)
  return d
}

function getWeekDates(monday: Date): string[] {
  const dates: string[] = []
  for (let i = 0; i < 7; i++) {
    const d = new Date(monday)
    d.setDate(monday.getDate() + i)
    dates.push(formatDateIso(d))
  }
  return dates
}

function groupByLocalDate<T extends { startDatetime: string }>(
  items: T[],
  timezone: string
): Record<string, T[]> {
  const grouped: Record<string, T[]> = {}
  for (const item of items) {
    const localDate = getLocalDate(item.startDatetime, timezone)
    if (!grouped[localDate]) grouped[localDate] = []
    grouped[localDate].push(item)
  }
  return grouped
}

export function W01WeeklyPlanning() {
  const [selectedWeekStart, setSelectedWeekStart] = useState<Date>(() => getMonday(new Date()))
  const { fetchTimeBlocksRange, fetchTimeEntriesRange, timeBlocks, timeEntries, addTimeBlock, updateTimeBlock } = useTimeBlockStore()
  const { tasks } = useTaskManagerStore()
  const { timezone } = useSettingsStore()
  const taskDetailPanel = useTaskDetailPanel()

  const weekDates = useMemo(() => getWeekDates(selectedWeekStart), [selectedWeekStart])
  const startStr = weekDates[0]
  const endStr = weekDates[weekDates.length - 1]

  // TimeBlockDialog - defaultDate は週の最初の日にしておくが、openDialogWithTime で上書き可能
  const blockDialog = useTimeBlockDialog({ defaultDate: startStr })

  // DnD sensors
  const sensors = useSensors(
    useSensor(PointerSensor, {
      activationConstraint: {
        distance: 8,
      },
    })
  )

  // ドラッグ状態
  const [isDraggingTask, setIsDraggingTask] = useState(false)
  const [activeDragData, setActiveDragData] = useState<TaskDragData | null>(null)

  useEffect(() => {
    fetchTimeBlocksRange(startStr, endStr)
    fetchTimeEntriesRange(startStr, endStr)
  }, [startStr, endStr, fetchTimeBlocksRange, fetchTimeEntriesRange])

  const blocksByDate = useMemo(
    () => groupByLocalDate<TimeBlock>(timeBlocks, timezone),
    [timeBlocks, timezone]
  )
  const entriesByDate = useMemo(
    () => groupByLocalDate<TimeEntry>(timeEntries, timezone),
    [timeEntries, timezone]
  )

  // カレンダーのセルクリック → TimeBlockDialog を開く
  const handleCellClick = useCallback((date: string, startHour: number) => {
    const start = `${startHour.toString().padStart(2, '0')}:00`
    const end = `${(startHour + 1).toString().padStart(2, '0')}:00`
    blockDialog.openDialogWithTime(start, end, date)
  }, [blockDialog])

  // カレンダーのブロッククリック → 編集ダイアログを開く
  const handleBlockClick = useCallback((block: TimeBlock) => {
    blockDialog.openEditDialog(block)
  }, [blockDialog])

  // カレンダーのブロックリサイズ・移動
  const handleBlockResize = useCallback((blockId: string, date: string, startTime: string, endTime: string) => {
    updateTimeBlock(blockId, date, startTime, date, endTime)
  }, [updateTimeBlock])

  // ドラッグ開始
  const handleDragStart = (event: DragStartEvent) => {
    const data = event.active.data.current as TaskDragData | undefined
    if (data?.type === 'task') {
      setIsDraggingTask(true)
      setActiveDragData(data)
    }
  }

  // ドラッグ終了
  const handleDragEnd = async (event: DragEndEvent) => {
    const { over, activatorEvent, delta } = event

    // ドロップ先がweekly-droppable-YYYY-MM-DD形式かチェック
    if (over?.id && String(over.id).startsWith('weekly-droppable-') && activeDragData) {
      const dropDate = over.data.current?.date as string
      if (dropDate) {
        // ドロップ位置のY座標から時間を計算
        const droppableElement = document.querySelector(`[data-droppable-date="${dropDate}"]`)
        if (droppableElement && activatorEvent && 'clientY' in activatorEvent) {
          const currentY = (activatorEvent as MouseEvent).clientY + delta.y
          const rect = droppableElement.getBoundingClientRect()

          // タスクの見積もり時間を取得（なければ60分）
          const task = tasks.find(t => t.id === activeDragData.taskId)
          const durationMinutes = task?.estimatedMinutes || 60

          const { startTime, endTime } = calculateTimeFromYWeekly(currentY, rect, durationMinutes)

          // タイムブロックを作成
          await addTimeBlock(dropDate, startTime, dropDate, endTime, {
            taskId: activeDragData.taskId,
            taskName: activeDragData.taskName,
            milestoneId: activeDragData.milestoneId,
            milestoneName: activeDragData.milestoneName,
            goalId: activeDragData.goalId,
            goalName: activeDragData.goalName,
            goalColor: activeDragData.goalColor,
          })
        }
      }
    }

    // 状態をリセット
    setIsDraggingTask(false)
    setActiveDragData(null)
  }

  // ドラッグキャンセル
  const handleDragCancel = () => {
    setIsDraggingTask(false)
    setActiveDragData(null)
  }

  return (
    <DndContext
      sensors={sensors}
      onDragStart={handleDragStart}
      onDragEnd={handleDragEnd}
      onDragCancel={handleDragCancel}
    >
      <div className="space-y-4">
        <PageGuide pageId="weekly" />

        <div data-guide="weekly-header">
          <WeeklyHeader
            selectedWeekStart={selectedWeekStart}
            onWeekChange={setSelectedWeekStart}
          />
        </div>

        <WeeklySummaryBar
          blocksByDate={blocksByDate}
          entriesByDate={entriesByDate}
          weekDates={weekDates}
        />

        <div data-guide="weekly-calendar">
          <WeeklyCalendarGrid
            blocksByDate={blocksByDate}
            weekDates={weekDates}
            onCellClick={handleCellClick}
            onBlockClick={handleBlockClick}
            onBlockDelete={blockDialog.deleteBlock}
            onBlockResize={handleBlockResize}
            isDraggingTask={isDraggingTask}
          />
        </div>

        <div data-guide="weekly-tasks">
          <WeeklyTaskCards
            onTaskClick={taskDetailPanel.openTask}
          />
        </div>

        {/* TimeBlock Dialog */}
        <TimeBlockDialog
          open={blockDialog.isOpen}
          onOpenChange={(open) => { if (!open) blockDialog.closeDialog() }}
          title={blockDialog.editingBlockId ? 'タイムブロックを編集' : 'タイムブロックを追加'}
          taskName={blockDialog.taskName}
          startDatetime={blockDialog.startDatetime}
          endDatetime={blockDialog.endDatetime}
          taskId={blockDialog.taskId}
          milestoneId={blockDialog.milestoneId}
          taskInputMode={blockDialog.taskInputMode}
          selectedGoal={blockDialog.selectedGoal}
          onTaskNameChange={(v) => blockDialog.setField('taskName', v)}
          onStartDatetimeChange={(v) => blockDialog.setField('startDatetime', v)}
          onEndDatetimeChange={(v) => blockDialog.setField('endDatetime', v)}
          onTaskIdChange={(v) => blockDialog.setField('taskId', v)}
          onMilestoneIdChange={(v) => blockDialog.setField('milestoneId', v)}
          onTaskInputModeChange={(v) => blockDialog.setField('taskInputMode', v)}
          onSave={blockDialog.save}
          isEditMode={!!blockDialog.editingBlockId}
        />

        {/* Task Detail Panel */}
        <Sheet open={taskDetailPanel.isOpen} onOpenChange={open => { if (!open) taskDetailPanel.closeTask() }}>
          <SheetContent>
            <TaskDetailPanel
              taskId={taskDetailPanel.selectedTaskId}
              createContext={taskDetailPanel.newTaskContext}
              onCreated={taskDetailPanel.switchToCreatedTask}
              onClose={taskDetailPanel.closeTask}
            />
          </SheetContent>
        </Sheet>

        {/* ドラッグオーバーレイ */}
        <DragOverlay>
          {activeDragData ? (
            <div className="px-3 py-2 bg-background border border-primary rounded-md shadow-lg text-sm font-medium max-w-48 truncate">
              {activeDragData.taskName}
            </div>
          ) : null}
        </DragOverlay>
      </div>
    </DndContext>
  )
}
