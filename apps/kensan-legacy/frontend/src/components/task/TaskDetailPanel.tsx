import { useState, useEffect, useCallback, useRef } from 'react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Checkbox } from '@/components/ui/checkbox'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { ScrollArea } from '@/components/ui/scroll-area'
import { TagSelect } from '@/components/common/TagSelect'
import { EntityMemoSection } from '@/components/common/EntityMemoSection'
import { ConfirmPopover } from '@/components/common/ConfirmPopover'
import { GoalBadge } from '@/components/common/GoalBadge'
import { TagBadge } from '@/components/common/TagBadge'
import { useTaskManagerStore } from '@/stores/useTaskManagerStore'
import { cn } from '@/lib/utils'
import { formatDistanceToNow } from 'date-fns'
import { ja } from 'date-fns/locale'
import {
  X,
  Flag,
  Tags,
  Calendar,
  Clock,
  RefreshCw,
  Plus,
  Trash2,
  ChevronRight,
} from 'lucide-react'
import type { Task, TaskFrequency } from '@/types'
import type { NewTaskContext } from '@/hooks/useTaskDetailPanel'

interface TaskDetailPanelProps {
  taskId: string | null
  createContext?: NewTaskContext | null
  onCreated?: (taskId: string) => void
  onClose: () => void
}

export function TaskDetailPanel({ taskId, createContext, onCreated, onClose }: TaskDetailPanelProps) {
  const {
    tasks,
    goals,
    milestones,
    tags,
    updateTask,
    deleteTask,
    addTask,
    toggleTaskComplete,
    getChildTasks,
    getTagsByIds,
  } = useTaskManagerStore()

  const task = taskId ? tasks.find(t => t.id === taskId) : null
  const isCreateMode = !taskId && createContext !== null && createContext !== undefined

  // Local state for task name (saved on blur/Enter)
  const [editingName, setEditingName] = useState('')
  const [navigatedTaskId, setNavigatedTaskId] = useState<string | null>(null)
  const nameInputRef = useRef<HTMLInputElement>(null)

  // The actual task to display (supports navigating to child tasks within the panel)
  const displayTaskId = navigatedTaskId ?? taskId
  const displayTask = displayTaskId ? tasks.find(t => t.id === displayTaskId) : null

  // Sync editing name when task changes
  useEffect(() => {
    if (displayTask) {
      setEditingName(displayTask.name)
    } else if (isCreateMode) {
      setEditingName('')
    }
  }, [displayTask?.id, displayTask?.name, isCreateMode])

  // Auto-focus name input in create mode
  useEffect(() => {
    if (isCreateMode) {
      setTimeout(() => nameInputRef.current?.focus(), 100)
    }
  }, [isCreateMode])

  // Reset navigation when panel closes or taskId changes
  useEffect(() => {
    setNavigatedTaskId(null)
  }, [taskId])

  const handleSaveName = useCallback(async () => {
    if (isCreateMode) {
      if (!editingName.trim()) return
      try {
        const newTask = await addTask({
          name: editingName.trim(),
          milestoneId: createContext?.milestoneId || undefined,
          parentTaskId: createContext?.parentTaskId,
        })
        if (newTask) {
          toast.success('タスクを追加しました')
          onCreated?.(newTask.id)
        }
      } catch {
        // エラートーストはhttpClientで表示される
      }
      return
    }
    if (!displayTask || !editingName.trim() || editingName === displayTask.name) return
    await updateTask(displayTask.id, { name: editingName.trim() })
  }, [isCreateMode, displayTask, editingName, updateTask, addTask, createContext, onCreated])

  const handlePropertyChange = useCallback(async (updates: Partial<Task>) => {
    if (!displayTask) return
    await updateTask(displayTask.id, updates)
  }, [displayTask, updateTask])

  const handleDelete = useCallback(async () => {
    if (!displayTask) return
    try {
      await deleteTask(displayTask.id)
      toast.success('タスクを削除しました')
      if (navigatedTaskId) {
        setNavigatedTaskId(null)
      } else {
        onClose()
      }
    } catch {
      // エラートーストはhttpClientで表示される
    }
  }, [displayTask, deleteTask, navigatedTaskId, onClose])

  const handleAddSubtask = useCallback(async () => {
    if (!displayTask) return
    await addTask({
      name: '新しいサブタスク',
      milestoneId: displayTask.milestoneId,
      parentTaskId: displayTask.id,
    })
  }, [displayTask, addTask])

  // Create mode: show minimal UI for name input
  if (isCreateMode) {
    // Get parent task and milestone info for context display
    const parentTask = createContext?.parentTaskId
      ? tasks.find(t => t.id === createContext.parentTaskId)
      : null
    const milestone = createContext?.milestoneId
      ? milestones.find(m => m.id === createContext.milestoneId)
      : undefined
    const goal = milestone
      ? goals.find(g => g.id === milestone.goalId)
      : undefined

    return (
      <div className="flex flex-col h-full">
        {/* Header */}
        <div className="flex items-center gap-2 border-b px-4 py-3 flex-shrink-0">
          <Checkbox checked={false} disabled className="flex-shrink-0" />
          <input
            ref={nameInputRef}
            value={editingName}
            onChange={e => setEditingName(e.target.value)}
            onBlur={handleSaveName}
            onKeyDown={e => {
              if (e.key === 'Enter' && editingName.trim()) {
                e.currentTarget.blur()
              } else if (e.key === 'Escape') {
                onClose()
              }
            }}
            placeholder="タスク名を入力..."
            className="flex-1 text-lg font-semibold bg-transparent border-none outline-none focus:ring-0"
          />
          <button
            className="rounded-sm opacity-70 hover:opacity-100 transition-opacity"
            onClick={onClose}
          >
            <X className="h-4 w-4" />
          </button>
        </div>

        {/* Context info */}
        <div className="p-4 space-y-3">
          {goal && milestone && (
            <div className="flex items-center gap-3">
              <Flag className="h-4 w-4 text-muted-foreground flex-shrink-0" />
              <span className="text-sm text-muted-foreground w-20 flex-shrink-0">マイルストーン</span>
              <div className="flex items-center gap-2">
                <GoalBadge name={goal.name} color={goal.color} size="sm" />
                <span className="text-sm">{milestone.name}</span>
              </div>
            </div>
          )}
          {parentTask && (
            <div className="flex items-center gap-3">
              <ChevronRight className="h-4 w-4 text-muted-foreground flex-shrink-0" />
              <span className="text-sm text-muted-foreground w-20 flex-shrink-0">親タスク</span>
              <span className="text-sm">{parentTask.name}</span>
            </div>
          )}
          <p className="text-sm text-muted-foreground">
            タスク名を入力してEnterで作成。プロパティは作成後に編集できます。
          </p>
        </div>
      </div>
    )
  }

  if (!displayTask) return null

  const childTasks = getChildTasks(displayTask.id)

  // Get the goal for the selected milestone
  const selectedMilestone = displayTask.milestoneId
    ? milestones.find(m => m.id === displayTask.milestoneId)
    : undefined
  const selectedGoal = selectedMilestone
    ? goals.find(g => g.id === selectedMilestone.goalId)
    : undefined

  // Parent task for breadcrumb
  const parentTask = displayTask.parentTaskId
    ? tasks.find(t => t.id === displayTask.parentTaskId)
    : null

  return (
    <div className="flex flex-col h-full">
      {/* Header */}
      <div className="flex items-center gap-2 border-b px-4 py-3 flex-shrink-0">
        {/* Breadcrumb for sub-task navigation */}
        {navigatedTaskId && task && (
          <button
            className="text-xs text-muted-foreground hover:text-foreground flex items-center gap-1"
            onClick={() => setNavigatedTaskId(null)}
          >
            {task.name}
            <ChevronRight className="h-3 w-3" />
          </button>
        )}
        <Checkbox
          checked={displayTask.completed}
          onCheckedChange={() => toggleTaskComplete(displayTask.id)}
          className="flex-shrink-0"
        />
        <input
          ref={nameInputRef}
          value={editingName}
          onChange={e => setEditingName(e.target.value)}
          onBlur={handleSaveName}
          onKeyDown={e => {
            if (e.key === 'Enter') {
              e.currentTarget.blur()
            }
          }}
          className={cn(
            'flex-1 text-lg font-semibold bg-transparent border-none outline-none focus:ring-0',
            displayTask.completed && 'line-through text-muted-foreground'
          )}
        />
        <button
          className="rounded-sm opacity-70 hover:opacity-100 transition-opacity"
          onClick={onClose}
        >
          <X className="h-4 w-4" />
        </button>
      </div>

      {/* Scrollable content */}
      <ScrollArea className="flex-1">
        <div className="p-4 space-y-4">
          {/* Properties */}
          <div className="space-y-3">
            {/* Milestone */}
            <div className="flex items-center gap-3">
              <Flag className="h-4 w-4 text-muted-foreground flex-shrink-0" />
              <span className="text-sm text-muted-foreground w-20 flex-shrink-0">マイルストーン</span>
              <Select
                value={displayTask.milestoneId || ''}
                onValueChange={v => handlePropertyChange({ milestoneId: v || undefined })}
              >
                <SelectTrigger className="h-8 text-sm flex-1">
                  <SelectValue placeholder="なし" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="">なし</SelectItem>
                  {goals
                    .filter(g => g.status !== 'archived')
                    .map(goal => {
                      const goalMilestones = milestones.filter(
                        m => m.goalId === goal.id && m.status === 'active'
                      )
                      if (goalMilestones.length === 0) return null
                      return (
                        <div key={goal.id}>
                          <div className="px-2 py-1 text-xs text-muted-foreground flex items-center gap-2">
                            <span
                              className="w-2 h-2 rounded-full"
                              style={{ backgroundColor: goal.color }}
                            />
                            {goal.name}
                          </div>
                          {goalMilestones.map(milestone => (
                            <SelectItem key={milestone.id} value={milestone.id} label={milestone.name}>
                              {milestone.name}
                            </SelectItem>
                          ))}
                        </div>
                      )
                    })}
                </SelectContent>
              </Select>
            </div>
            {selectedGoal && (
              <div className="ml-[7.5rem] text-xs text-muted-foreground flex items-center gap-1">
                <GoalBadge name={selectedGoal.name} color={selectedGoal.color} size="sm" />
              </div>
            )}

            {/* Tags */}
            <div className="flex items-center gap-3">
              <Tags className="h-4 w-4 text-muted-foreground flex-shrink-0" />
              <span className="text-sm text-muted-foreground w-20 flex-shrink-0">タグ</span>
              <div className="flex-1">
                <TagSelect
                  tags={tags}
                  selectedTagIds={displayTask.tagIds || []}
                  onSelectedTagIdsChange={tagIds => handlePropertyChange({ tagIds })}
                  placeholder="タグを追加"
                />
              </div>
            </div>

            {/* Due date */}
            <div className="flex items-center gap-3">
              <Calendar className="h-4 w-4 text-muted-foreground flex-shrink-0" />
              <span className="text-sm text-muted-foreground w-20 flex-shrink-0">期限日</span>
              <Input
                type="date"
                value={displayTask.dueDate || ''}
                onChange={e => handlePropertyChange({ dueDate: e.target.value || undefined })}
                className="h-8 text-sm flex-1"
              />
            </div>

            {/* Estimated minutes */}
            <div className="flex items-center gap-3">
              <Clock className="h-4 w-4 text-muted-foreground flex-shrink-0" />
              <span className="text-sm text-muted-foreground w-20 flex-shrink-0">見積時間</span>
              <div className="flex items-center gap-2 flex-1">
                <Input
                  type="number"
                  min={0}
                  value={displayTask.estimatedMinutes ?? ''}
                  onChange={e => handlePropertyChange({
                    estimatedMinutes: e.target.value ? Number(e.target.value) : undefined,
                  })}
                  className="h-8 text-sm w-24"
                  placeholder="0"
                />
                <span className="text-sm text-muted-foreground">分</span>
              </div>
            </div>

            {/* Frequency */}
            <div className="flex items-center gap-3">
              <RefreshCw className="h-4 w-4 text-muted-foreground flex-shrink-0" />
              <span className="text-sm text-muted-foreground w-20 flex-shrink-0">繰り返し</span>
              <Select
                value={displayTask.frequency || ''}
                onValueChange={v => {
                  const freq = v as TaskFrequency | ''
                  handlePropertyChange({
                    frequency: freq || undefined,
                    daysOfWeek: (freq === 'daily' || freq === 'weekly' || !freq) ? undefined : displayTask.daysOfWeek,
                  })
                }}
              >
                <SelectTrigger className="h-8 text-sm flex-1">
                  <SelectValue placeholder="繰り返しなし" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="">繰り返しなし</SelectItem>
                  <SelectItem value="daily">毎日</SelectItem>
                  <SelectItem value="weekly">毎週（平日のみ）</SelectItem>
                  <SelectItem value="custom">カスタム（曜日指定）</SelectItem>
                </SelectContent>
              </Select>
            </div>

            {/* Custom days of week */}
            {displayTask.frequency === 'custom' && (
              <div className="ml-[7.5rem] flex flex-wrap gap-2">
                {['日', '月', '火', '水', '木', '金', '土'].map((day, index) => {
                  const isSelected = displayTask.daysOfWeek?.includes(index) ?? false
                  return (
                    <button
                      key={index}
                      type="button"
                      onClick={() => {
                        const current = displayTask.daysOfWeek || []
                        const next = isSelected
                          ? current.filter(d => d !== index)
                          : [...current, index].sort((a, b) => a - b)
                        handlePropertyChange({ daysOfWeek: next.length > 0 ? next : undefined })
                      }}
                      className={cn(
                        'px-3 py-1.5 text-sm rounded-md border transition-colors',
                        isSelected
                          ? 'bg-primary text-primary-foreground border-primary'
                          : 'bg-background hover:bg-muted border-input'
                      )}
                    >
                      {day}
                    </button>
                  )
                })}
              </div>
            )}

            {/* Parent task info */}
            {parentTask && (
              <div className="flex items-center gap-3">
                <ChevronRight className="h-4 w-4 text-muted-foreground flex-shrink-0" />
                <span className="text-sm text-muted-foreground w-20 flex-shrink-0">親タスク</span>
                <span className="text-sm">{parentTask.name}</span>
              </div>
            )}
          </div>

          {/* Memo section */}
          <div className="border-t pt-4">
            <EntityMemoSection
              entityType="task"
              entityId={displayTask.id}
              maxHeight="300px"
            />
          </div>

          {/* Subtasks */}
          <div className="border-t pt-4">
            <div className="flex items-center gap-2 mb-3">
              <span className="text-sm font-medium text-muted-foreground">サブタスク</span>
              {childTasks.length > 0 && (
                <span className="text-xs bg-muted px-1.5 py-0.5 rounded">{childTasks.length}</span>
              )}
              <Button
                variant="ghost"
                size="sm"
                className="h-6 w-6 p-0 ml-auto"
                onClick={handleAddSubtask}
              >
                <Plus className="h-3.5 w-3.5" />
              </Button>
            </div>
            <div className="space-y-1">
              {childTasks.map(child => (
                <div
                  key={child.id}
                  className="flex items-center gap-2 p-2 rounded-lg hover:bg-muted/50 group"
                >
                  <Checkbox
                    checked={child.completed}
                    onCheckedChange={() => toggleTaskComplete(child.id)}
                  />
                  <span
                    className={cn(
                      'flex-1 text-sm cursor-pointer hover:text-primary',
                      child.completed && 'line-through text-muted-foreground'
                    )}
                    onClick={() => setNavigatedTaskId(child.id)}
                  >
                    {child.name}
                  </span>
                  {child.tagIds && child.tagIds.length > 0 && (
                    <div className="flex gap-1">
                      {getTagsByIds(child.tagIds).map(tag => (
                        <TagBadge key={tag.id} name={tag.name} color={tag.color} size="sm" />
                      ))}
                    </div>
                  )}
                </div>
              ))}
              {childTasks.length === 0 && (
                <p className="text-xs text-muted-foreground text-center py-2">
                  サブタスクはありません
                </p>
              )}
            </div>
          </div>
        </div>
      </ScrollArea>

      {/* Footer */}
      <div className="flex items-center justify-between border-t px-4 py-3 flex-shrink-0">
        <ConfirmPopover
          message="このタスクを削除しますか？"
          confirmLabel="削除"
          onConfirm={handleDelete}
          variant="destructive"
        >
          <Button variant="ghost" size="sm" className="text-destructive hover:text-destructive">
            <Trash2 className="h-4 w-4 mr-1" />
            削除
          </Button>
        </ConfirmPopover>
        <div className="text-xs text-muted-foreground">
          作成: {formatDistanceToNow(displayTask.createdAt, { addSuffix: true, locale: ja })}
          {displayTask.updatedAt && displayTask.updatedAt !== displayTask.createdAt && (
            <> / 更新: {formatDistanceToNow(displayTask.updatedAt, { addSuffix: true, locale: ja })}</>
          )}
        </div>
      </div>
    </div>
  )
}
