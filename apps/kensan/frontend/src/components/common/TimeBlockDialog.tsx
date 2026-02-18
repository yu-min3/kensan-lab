import { useState } from 'react'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from '@/components/ui/dialog'
import { DateTimeRangePicker } from '@/components/common/DateTimeRangePicker'
import { useTaskManagerStore } from '@/stores/useTaskManagerStore'
import type { TaskInputMode } from '@/hooks/useTimeBlockDialog'
import type { Goal, Milestone } from '@/types'
import { Plus } from 'lucide-react'

interface TimeBlockDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  title: string

  // Mode: plan (予定) or entry (実績)
  mode?: 'plan' | 'entry'

  // Form state
  taskName: string
  startDatetime: string  // YYYY-MM-DDTHH:mm (local)
  endDatetime: string    // YYYY-MM-DDTHH:mm (local)
  taskId: string | undefined
  milestoneId: string | undefined
  taskInputMode: TaskInputMode
  selectedGoal: Goal | undefined

  // Entry-specific state
  description?: string

  // Callbacks
  onTaskNameChange: (value: string) => void
  onStartDatetimeChange: (value: string) => void
  onEndDatetimeChange: (value: string) => void
  onTaskIdChange: (value: string | undefined) => void
  onMilestoneIdChange: (value: string | undefined) => void
  onTaskInputModeChange: (mode: TaskInputMode) => void
  onSave: () => void

  // Entry-specific callbacks
  onDescriptionChange?: (value: string) => void

  // Options
  showTaskInputModeToggle?: boolean
  isEditMode?: boolean
  isSubmitting?: boolean
}

export function TimeBlockDialog({
  open,
  onOpenChange,
  title,
  mode = 'plan',
  taskName,
  startDatetime,
  endDatetime,
  taskId,
  taskInputMode,
  selectedGoal,
  description,
  onTaskNameChange,
  onStartDatetimeChange,
  onEndDatetimeChange,
  onTaskIdChange,
  onMilestoneIdChange,
  onTaskInputModeChange,
  onSave,
  onDescriptionChange,
  showTaskInputModeToggle = true,
  isEditMode = false,
  isSubmitting = false,
}: TimeBlockDialogProps) {
  const { tasks, goals, milestones, addTask } = useTaskManagerStore()
  const [isAddingTask, setIsAddingTask] = useState(false)
  const [newTaskName, setNewTaskName] = useState('')
  const [newTaskMilestoneId, setNewTaskMilestoneId] = useState<string | undefined>(undefined)

  // 新規タスクを追加
  const handleAddTask = async () => {
    if (!newTaskName.trim()) return

    const taskName = newTaskName.trim()

    await addTask({
      name: taskName,
      milestoneId: newTaskMilestoneId,
    })

    // タスク追加後、タスク名で直接設定（storeのtasksリストは非同期で更新されるため）
    onTaskNameChange(taskName)
    if (newTaskMilestoneId) {
      onMilestoneIdChange(newTaskMilestoneId)
    }
    // taskIdは次回のtasks更新後に自動的に反映されるため、一旦undefinedのまま
    // ただしtaskNameとmilestoneIdが設定されているので、保存時に正しいgoalが紐づく

    // リセット
    setNewTaskName('')
    setNewTaskMilestoneId(undefined)
    setIsAddingTask(false)
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
        </DialogHeader>

        <div className="space-y-4 py-4">
          {/* Task input mode toggle */}
          {showTaskInputModeToggle && (
            <div>
              <Label className="mb-2 block">タスクの指定方法</Label>
              <div className="flex rounded-lg border p-1 bg-muted/30">
                <button
                  type="button"
                  className={`flex-1 py-1.5 px-3 text-sm rounded-md transition-colors ${
                    taskInputMode === 'existing'
                      ? 'bg-background shadow-sm font-medium'
                      : 'text-muted-foreground hover:text-foreground'
                  }`}
                  onClick={() => {
                    onTaskInputModeChange('existing')
                    onTaskNameChange('')
                    onMilestoneIdChange(undefined)
                  }}
                >
                  タスクから選択
                </button>
                <button
                  type="button"
                  className={`flex-1 py-1.5 px-3 text-sm rounded-md transition-colors ${
                    taskInputMode === 'manual'
                      ? 'bg-background shadow-sm font-medium'
                      : 'text-muted-foreground hover:text-foreground'
                  }`}
                  onClick={() => {
                    onTaskInputModeChange('manual')
                    onTaskIdChange(undefined)
                  }}
                >
                  予定を入力
                </button>
              </div>
            </div>
          )}

          {/* Existing task selection mode (デフォルト) */}
          {taskInputMode === 'existing' && showTaskInputModeToggle && (
            <div className="space-y-3">
              <div>
                <Label>タスクを選択</Label>
                <Select
                  value={taskId || ''}
                  onValueChange={(v) => {
                    const task = tasks.find((t) => t.id === v)
                    if (task) {
                      onTaskIdChange(v)
                      onTaskNameChange(task.name)
                      if (task.milestoneId) {
                        onMilestoneIdChange(task.milestoneId)
                      }
                      if (task.estimatedMinutes && startDatetime) {
                        const start = new Date(startDatetime)
                        start.setMinutes(start.getMinutes() + task.estimatedMinutes)
                        const end = `${start.getFullYear()}-${String(start.getMonth() + 1).padStart(2, '0')}-${String(start.getDate()).padStart(2, '0')}T${String(start.getHours()).padStart(2, '0')}:${String(start.getMinutes()).padStart(2, '0')}`
                        onEndDatetimeChange(end)
                      }
                    }
                  }}
                >
                  <SelectTrigger className="mt-1">
                    <SelectValue placeholder="タスクを選択" />
                  </SelectTrigger>
                  <SelectContent>
                    <GroupedTaskList
                      tasks={tasks}
                      goals={goals}
                      milestones={milestones}
                    />
                  </SelectContent>
                </Select>
                {taskId && selectedGoal && (
                  <p className="text-xs text-muted-foreground mt-1 flex items-center gap-1">
                    <span
                      className="w-2 h-2 rounded-full"
                      style={{ backgroundColor: selectedGoal.color }}
                    />
                    {selectedGoal.name}
                  </p>
                )}
              </div>

              {/* タスク追加セクション */}
              {!isAddingTask ? (
                <Button
                  variant="ghost"
                  size="sm"
                  className="text-muted-foreground"
                  onClick={() => setIsAddingTask(true)}
                >
                  <Plus className="h-4 w-4 mr-1" />
                  新しいタスクを作成
                </Button>
              ) : (
                <div className="p-3 rounded-md border bg-muted/30 space-y-3">
                  <div>
                    <Label htmlFor="newTaskName" className="text-xs">タスク名</Label>
                    <Input
                      id="newTaskName"
                      value={newTaskName}
                      onChange={(e) => setNewTaskName(e.target.value)}
                      placeholder="新しいタスク名"
                      className="mt-1 h-8 text-sm"
                    />
                  </div>
                  <div>
                    <Label className="text-xs">マイルストーン</Label>
                    <Select
                      value={newTaskMilestoneId || ''}
                      onValueChange={(v) => setNewTaskMilestoneId(v || undefined)}
                    >
                      <SelectTrigger className="mt-1 h-8 text-sm">
                        <SelectValue placeholder="選択（任意）" />
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
                  <div className="flex gap-2">
                    <Button
                      size="sm"
                      className="h-7 text-xs"
                      onClick={handleAddTask}
                      disabled={!newTaskName.trim()}
                    >
                      作成して選択
                    </Button>
                    <Button
                      variant="ghost"
                      size="sm"
                      className="h-7 text-xs"
                      onClick={() => {
                        setIsAddingTask(false)
                        setNewTaskName('')
                        setNewTaskMilestoneId(undefined)
                      }}
                    >
                      キャンセル
                    </Button>
                  </div>
                </div>
              )}
            </div>
          )}

          {/* Manual input mode (予定を入力) */}
          {(taskInputMode === 'manual' || !showTaskInputModeToggle) && (
            <>
              <div>
                <Label htmlFor="blockTaskName">予定名</Label>
                <Input
                  id="blockTaskName"
                  value={taskName}
                  onChange={(e) => onTaskNameChange(e.target.value)}
                  placeholder="例: MTG、休憩、作業など"
                  className="mt-1"
                />
              </div>
            </>
          )}

          {/* 日時 */}
          <div>
            <Label className="text-xs text-muted-foreground mb-2 block">日時</Label>
            <DateTimeRangePicker
              startDatetime={startDatetime}
              endDatetime={endDatetime}
              onStartChange={onStartDatetimeChange}
              onEndChange={onEndDatetimeChange}
            />
          </div>

          {/* 説明 (実績モードのみ) */}
          {mode === 'entry' && onDescriptionChange && (
            <div>
              <Label htmlFor="entryDescription">説明（任意）</Label>
              <Textarea
                id="entryDescription"
                value={description || ''}
                onChange={(e) => onDescriptionChange(e.target.value)}
                placeholder="作業内容の詳細..."
                className="mt-1"
                rows={2}
              />
            </div>
          )}

          {/* Alternative: existing task selection for non-toggle mode */}
          {!showTaskInputModeToggle && (
            <div>
              <Label>または既存タスクから選択</Label>
              <Select
                value={taskId || ''}
                onValueChange={(v) => {
                  const task = tasks.find((t) => t.id === v)
                  if (task) {
                    onTaskIdChange(v)
                    onTaskNameChange(task.name)
                    if (task.milestoneId) {
                      onMilestoneIdChange(task.milestoneId)
                    }
                    if (task.estimatedMinutes && startDatetime) {
                      const start = new Date(startDatetime)
                      start.setMinutes(start.getMinutes() + task.estimatedMinutes)
                      const end = `${start.getFullYear()}-${String(start.getMonth() + 1).padStart(2, '0')}-${String(start.getDate()).padStart(2, '0')}T${String(start.getHours()).padStart(2, '0')}:${String(start.getMinutes()).padStart(2, '0')}`
                      onEndDatetimeChange(end)
                    }
                  }
                }}
              >
                <SelectTrigger className="mt-1">
                  <SelectValue placeholder="タスクを選択（任意）" />
                </SelectTrigger>
                <SelectContent>
                  {tasks.filter((t) => !t.completed && !t.parentTaskId).map((task) => (
                    <SelectItem key={task.id} value={task.id} label={task.name}>
                      {task.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          )}
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)} disabled={isSubmitting}>
            キャンセル
          </Button>
          <Button onClick={onSave} disabled={!taskName || isSubmitting}>
            {isSubmitting ? '保存中...' : isEditMode ? '保存' : '追加'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

// 目標・マイルストーン別にグループ化されたタスクリスト
interface GroupedTaskListProps {
  tasks: import('@/types').Task[]
  goals: Goal[]
  milestones: Milestone[]
}

function GroupedTaskList({
  tasks,
  goals,
  milestones,
}: GroupedTaskListProps) {
  // 未完了で親タスクでないタスク（マイルストーンに紐づくもののみ）
  const activeTasks = tasks.filter((t) => !t.completed && !t.parentTaskId && t.milestoneId)

  // 目標別にグループ化
  const activeGoals = goals.filter((g) => g.status !== 'archived')

  return (
    <>
      {/* 目標ごとにグループ表示 */}
      {activeGoals.map((goal) => {
        const goalMilestones = milestones.filter(
          (m) => m.goalId === goal.id && m.status === 'active'
        )
        // この目標に紐づくタスクを取得
        const goalTaskIds = new Set(
          goalMilestones.flatMap((m) =>
            activeTasks.filter((t) => t.milestoneId === m.id).map((t) => t.id)
          )
        )
        if (goalTaskIds.size === 0) return null

        return (
          <div key={goal.id}>
            {/* 目標ヘッダー */}
            <div className="px-2 py-1.5 text-xs font-medium text-muted-foreground flex items-center gap-2 bg-muted/50 sticky top-0">
              <span
                className="w-2 h-2 rounded-full"
                style={{ backgroundColor: goal.color }}
              />
              {goal.name}
            </div>
            {/* マイルストーンごとにタスク表示 */}
            {goalMilestones.map((milestone) => {
              const milestoneTasks = activeTasks.filter(
                (t) => t.milestoneId === milestone.id
              )
              if (milestoneTasks.length === 0) return null

              return (
                <div key={milestone.id}>
                  {/* マイルストーンヘッダー */}
                  <div className="px-4 py-1 text-xs text-muted-foreground">
                    {milestone.name}
                  </div>
                  {/* タスク */}
                  {milestoneTasks.map((task) => (
                    <SelectItem key={task.id} value={task.id} label={task.name}>
                      <span className="pl-2">{task.name}</span>
                    </SelectItem>
                  ))}
                </div>
              )
            })}
          </div>
        )
      })}

    </>
  )
}
