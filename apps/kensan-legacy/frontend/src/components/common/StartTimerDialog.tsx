import { useState, useEffect } from 'react'
import { Play } from 'lucide-react'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { GoalBadge } from '@/components/common/GoalBadge'
import { TaskSelect } from '@/components/common/TaskSelect'
import { useTimerStore } from '@/stores/useTimerStore'
import { useTaskManagerStore } from '@/stores/useTaskManagerStore'

interface StartTimerDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function StartTimerDialog({ open, onOpenChange }: StartTimerDialogProps) {
  const { startTimer, isLoading } = useTimerStore()
  const { tasks, goals, milestones, getMilestoneById, getGoalById } = useTaskManagerStore()

  const [inputMode, setInputMode] = useState<'manual' | 'existing'>('existing')
  const [taskName, setTaskName] = useState('')
  const [milestoneId, setMilestoneId] = useState<string>('')
  const [selectedTaskId, setSelectedTaskId] = useState<string>('')

  // Reset form when dialog opens
  useEffect(() => {
    if (open) {
      setInputMode('existing')
      setTaskName('')
      setMilestoneId('')
      setSelectedTaskId('')
    }
  }, [open])

  // Get goal from selected milestone (for manual mode)
  const selectedMilestone = milestoneId ? getMilestoneById(milestoneId) : undefined
  const selectedGoal = selectedMilestone ? getGoalById(selectedMilestone.goalId) : undefined

  // Get goal from selected task (for existing mode)
  const selectedTask = selectedTaskId ? tasks.find(t => t.id === selectedTaskId) : undefined
  const taskMilestone = selectedTask?.milestoneId ? getMilestoneById(selectedTask.milestoneId) : undefined
  const taskGoal = taskMilestone ? getGoalById(taskMilestone.goalId) : undefined

  const handleStartTimer = async () => {
    let finalTaskName = ''
    let finalMilestoneId: string | undefined
    let finalGoalId: string | undefined
    let finalGoalName: string | undefined
    let finalGoalColor: string | undefined

    if (inputMode === 'manual') {
      if (!taskName.trim()) return
      finalTaskName = taskName.trim()
      finalMilestoneId = milestoneId || undefined
      finalGoalId = selectedGoal?.id
      finalGoalName = selectedGoal?.name
      finalGoalColor = selectedGoal?.color
    } else {
      if (!selectedTask) return
      finalTaskName = selectedTask.name
      finalMilestoneId = selectedTask.milestoneId
      finalGoalId = taskGoal?.id
      finalGoalName = taskGoal?.name
      finalGoalColor = taskGoal?.color
    }

    await startTimer({
      taskName: finalTaskName,
      milestoneId: finalMilestoneId,
      goalId: finalGoalId,
      goalName: finalGoalName,
      goalColor: finalGoalColor,
    })

    onOpenChange(false)
  }

  const canStart = inputMode === 'manual' ? taskName.trim() : selectedTaskId

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>タイマーを開始</DialogTitle>
        </DialogHeader>

        <div className="space-y-4 py-4">
          {/* Input mode toggle */}
          <div>
            <Label className="mb-2 block">タスクの指定方法</Label>
            <div className="flex rounded-lg border p-1 bg-muted/30">
              <button
                type="button"
                className={`flex-1 py-1.5 px-3 text-sm rounded-md transition-colors ${
                  inputMode === 'manual'
                    ? 'bg-background shadow-sm font-medium'
                    : 'text-muted-foreground hover:text-foreground'
                }`}
                onClick={() => {
                  setInputMode('manual')
                  setSelectedTaskId('')
                }}
              >
                タスク名を入力
              </button>
              <button
                type="button"
                className={`flex-1 py-1.5 px-3 text-sm rounded-md transition-colors ${
                  inputMode === 'existing'
                    ? 'bg-background shadow-sm font-medium'
                    : 'text-muted-foreground hover:text-foreground'
                }`}
                onClick={() => {
                  setInputMode('existing')
                  setTaskName('')
                  setMilestoneId('')
                }}
              >
                既存タスクから選択
              </button>
            </div>
          </div>

          {inputMode === 'manual' ? (
            <>
              {/* Task name input */}
              <div className="space-y-2">
                <Label htmlFor="taskName">タスク名 *</Label>
                <Input
                  id="taskName"
                  value={taskName}
                  onChange={(e) => setTaskName(e.target.value)}
                  placeholder="何に取り組みますか？"
                  autoFocus
                />
              </div>

              {/* Milestone selection */}
              <div className="space-y-2">
                <Label htmlFor="milestone">マイルストーン</Label>
                <Select value={milestoneId} onValueChange={setMilestoneId}>
                  <SelectTrigger>
                    <SelectValue placeholder="マイルストーンを選択（任意）" />
                  </SelectTrigger>
                  <SelectContent>
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
                                <div className="flex items-center gap-2">
                                  <span>{milestone.name}</span>
                                  {milestone.targetDate && (
                                    <span className="text-xs text-muted-foreground">
                                      ({milestone.targetDate})
                                    </span>
                                  )}
                                </div>
                              </SelectItem>
                            ))}
                          </div>
                        )
                      })}
                  </SelectContent>
                </Select>
                {selectedGoal && (
                  <div className="flex items-center gap-2 text-sm">
                    <span className="text-muted-foreground">Goal:</span>
                    <GoalBadge
                      name={selectedGoal.name}
                      color={selectedGoal.color}
                      size="sm"
                    />
                  </div>
                )}
              </div>
            </>
          ) : (
            /* Existing task selection */
            <div className="space-y-2">
              <Label>タスクを選択 *</Label>
              <TaskSelect
                tasks={tasks}
                goals={goals}
                milestones={milestones}
                value={selectedTaskId}
                onValueChange={setSelectedTaskId}
                placeholder="タスクを検索・選択"
                getMilestoneById={getMilestoneById}
                getGoalById={getGoalById}
              />
              {taskGoal && (
                <div className="flex items-center gap-2 text-sm">
                  <span className="text-muted-foreground">Goal:</span>
                  <GoalBadge
                    name={taskGoal.name}
                    color={taskGoal.color}
                    size="sm"
                  />
                </div>
              )}
            </div>
          )}
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            キャンセル
          </Button>
          <Button onClick={handleStartTimer} disabled={!canStart || isLoading}>
            <Play className="h-4 w-4 mr-2" />
            開始
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
