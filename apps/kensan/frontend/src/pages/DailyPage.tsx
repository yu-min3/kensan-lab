import { useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { Button } from '@/components/ui/button'
import { Sheet, SheetContent } from '@/components/ui/sheet'
import { TaskDetailPanel } from '@/components/task/TaskDetailPanel'
import { useTaskDetailPanel } from '@/hooks/useTaskDetailPanel'
import { DailySummary } from '@/components/daily/DailySummary'
import { TimeBlockSection } from '@/components/daily/TimeBlockSection'
import { TaskListWidget, type TaskDragData } from '@/components/daily/TaskListWidget'
import { PageMemo } from '@/components/common/PageMemo'
import { AIAdviceCard } from '@/components/daily/AIAdviceCard'
import { calculateTimeFromYWithDuration } from '@/components/common/TimeBlockTimeline'
import { useSettingsStore } from '@/stores/useSettingsStore'
import { useTimeBlockStore } from '@/stores/useTimeBlockStore'
import { formatDateJa, formatDateIso } from '@/lib/dateFormat'
import { PageGuide } from '@/components/guide/PageGuide'
import {
  Sun,
  Moon,
  BookOpen,
  BookMarked,
} from 'lucide-react'
import {
  DndContext,
  DragOverlay,
  useSensor,
  useSensors,
  PointerSensor,
  type DragStartEvent,
  type DragMoveEvent,
  type DragEndEvent,
} from '@dnd-kit/core'

export function DailyPage() {
  const { userName } = useSettingsStore()
  const { addTimeBlock } = useTimeBlockStore()
  const taskDetailPanel = useTaskDetailPanel()
  const [searchParams] = useSearchParams()

  // 選択中の日付（URLパラメータ or 今日）
  const [selectedDate, setSelectedDate] = useState<Date>(() => {
    const dateParam = searchParams.get('date')
    if (dateParam && /^\d{4}-\d{2}-\d{2}$/.test(dateParam)) {
      const d = new Date(dateParam + 'T00:00:00')
      if (!isNaN(d.getTime())) return d
    }
    return new Date()
  })
  const selectedDateIso = formatDateIso(selectedDate)
  const isToday = formatDateIso(new Date()) === selectedDateIso

  // ドラッグ&ドロップ状態
  const [isDraggingTask, setIsDraggingTask] = useState(false)
  const [dragOverY, setDragOverY] = useState<number | null>(null)
  const [activeDragData, setActiveDragData] = useState<TaskDragData | null>(null)

  // DnD sensors
  const sensors = useSensors(
    useSensor(PointerSensor, {
      activationConstraint: {
        distance: 8,
      },
    })
  )

  const hour = new Date().getHours()
  const isEvening = hour >= 17

  // 時間帯に応じた挨拶（今日の場合のみ）
  const Icon = isEvening ? Moon : Sun
  const iconColor = isEvening ? 'text-indigo-400' : 'text-slate-500'
  const greeting = isToday
    ? isEvening
      ? `お疲れさまです、${userName}さん`
      : `おはようございます、${userName}さん`
    : formatDateJa(selectedDate)

  // ドラッグ開始
  const handleDragStart = (event: DragStartEvent) => {
    const data = event.active.data.current as TaskDragData | undefined
    if (data?.type === 'task') {
      setIsDraggingTask(true)
      setActiveDragData(data)
    }
  }

  // ドラッグ中
  const handleDragMove = (event: DragMoveEvent) => {
    if (!isDraggingTask) return

    // ドロップ先がタイムラインの場合のみY座標を更新
    if (event.over?.id === 'timeblock-timeline-droppable') {
      const { activatorEvent, delta } = event
      if (activatorEvent && 'clientY' in activatorEvent) {
        const initialY = (activatorEvent as MouseEvent).clientY
        setDragOverY(initialY + delta.y)
      }
    } else {
      setDragOverY(null)
    }
  }

  // ドラッグ終了
  const handleDragEnd = async (event: DragEndEvent) => {
    const { over } = event

    if (over?.id === 'timeblock-timeline-droppable' && activeDragData && dragOverY !== null) {
      // タイムライン上にドロップされた場合
      const timelineElement = document.querySelector('[data-timeline-container]')
      if (timelineElement) {
        const rect = timelineElement.getBoundingClientRect()
        // TimeBlockTimelineの実際の表示範囲を読み取る（動的に変わるため）
        const actualStartHour = parseInt(timelineElement.getAttribute('data-start-hour') || '6', 10)
        const actualEndHour = parseInt(timelineElement.getAttribute('data-end-hour') || '24', 10)
        const durationMinutes = activeDragData.estimatedMinutes || 60
        const { startTime, endTime } = calculateTimeFromYWithDuration(dragOverY, rect, actualStartHour, actualEndHour, durationMinutes, 15)

        // タイムブロックを作成（選択中の日付を使用）
        await addTimeBlock(selectedDateIso, startTime, selectedDateIso, endTime, {
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

    // 状態をリセット
    setIsDraggingTask(false)
    setDragOverY(null)
    setActiveDragData(null)
  }

  // ドラッグキャンセル
  const handleDragCancel = () => {
    setIsDraggingTask(false)
    setDragOverY(null)
    setActiveDragData(null)
  }

  return (
    <DndContext
      sensors={sensors}
      onDragStart={handleDragStart}
      onDragMove={handleDragMove}
      onDragEnd={handleDragEnd}
      onDragCancel={handleDragCancel}
    >
      <div className="space-y-6">
        <PageGuide pageId="daily" />

        {/* ヘッダー + サマリー（インライン） */}
        <div className="flex items-center justify-between gap-4 flex-wrap" data-guide="daily-header">
          <div className="flex items-center gap-3">
            <Icon className={`h-8 w-8 ${iconColor}`} />
            <div>
              <h1 className="text-2xl font-bold">{greeting}</h1>
              {isToday && <p className="text-muted-foreground">{formatDateJa(new Date())}</p>}
            </div>
          </div>
          <DailySummary mode="compact" selectedDate={selectedDateIso} />
        </div>

        {/* メモ */}
        <PageMemo
          pageId="daily"
          title="今日のメモ"
          placeholder="今日の予定、気づき、やることなど..."
        />

        {/* タイムブロック + タスクリスト */}
        <div className="grid gap-4 lg:grid-cols-3">
          <div className="lg:col-span-2" data-guide="daily-timeblocks">
            <TimeBlockSection
              showAddButtons={true}
              isDraggingTask={isDraggingTask}
              dragOverY={dragOverY}
              dragDurationMinutes={activeDragData?.estimatedMinutes || 60}
              selectedDate={selectedDate}
              onDateChange={setSelectedDate}
            />
          </div>

          {/* 右サイド: タスクリスト */}
          <div className="hidden lg:block" data-guide="daily-tasks">
            <TaskListWidget onTaskClick={taskDetailPanel.openTask} />
          </div>
        </div>

        {/* AI Planning（今日の場合のみ） */}
        {isToday && <div data-guide="daily-ai"><AIAdviceCard selectedDate={selectedDateIso} /></div>}

        {/* 記録 */}
        <section className="space-y-4">
          <h2 className="text-sm font-medium text-muted-foreground uppercase tracking-wide">記録</h2>
          <div className="flex gap-2">
            <Link to="/notes/new?type=learning">
              <Button variant="outline" className="gap-2">
                <BookOpen className="h-4 w-4" />
                学習記録を作成
              </Button>
            </Link>
            <Link to="/notes/new?type=diary">
              <Button variant="outline" className="gap-2">
                <BookMarked className="h-4 w-4" />
                日記を書く
              </Button>
            </Link>
          </div>
        </section>

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
