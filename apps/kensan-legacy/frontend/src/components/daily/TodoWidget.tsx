import { useState } from 'react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { ScrollArea } from '@/components/ui/scroll-area'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from '@/components/ui/dialog'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Label } from '@/components/ui/label'
import { ConfirmPopover } from '@/components/common/ConfirmPopover'
import { useTodos } from '@/hooks/useTodos'
import { useTimerStore } from '@/stores/useTimerStore'
import { cn } from '@/lib/utils'
import {
  Plus,
  RefreshCw,
  AlertCircle,
  Loader2,
  Trash2,
  CheckCircle2,
  Circle,
  Play,
} from 'lucide-react'
import type { TodoFrequency, TodoWithStatus } from '@/types'

interface TodoWidgetProps {
  date: string // YYYY-MM-DD
  className?: string
}

const DAYS_OF_WEEK = ['日', '月', '火', '水', '木', '金', '土']

function formatFrequency(todo: TodoWithStatus): string {
  if (!todo.frequency) return ''
  if (todo.frequency === 'daily') return '毎日'
  if (todo.frequency === 'weekly' && todo.daysOfWeek?.length === 7) return '毎日'
  if (todo.frequency === 'weekly' || todo.frequency === 'custom') {
    const days = todo.daysOfWeek?.map((d) => DAYS_OF_WEEK[d]).join('')
    return days || ''
  }
  if (todo.frequency === 'monthly') return '毎月'
  return ''
}

export function TodoWidget({ date, className }: TodoWidgetProps) {
  const { todos, isLoading, error, addTodo, deleteTodo, toggleComplete, refresh } = useTodos({
    date,
    enabled: true,
  })
  const { currentTimer, startTimer } = useTimerStore()

  const [isDialogOpen, setIsDialogOpen] = useState(false)
  const [newTodoName, setNewTodoName] = useState('')
  const [newTodoFrequency, setNewTodoFrequency] = useState<TodoFrequency | ''>('')
  const [newTodoDaysOfWeek, setNewTodoDaysOfWeek] = useState<number[]>([])
  const [newTodoDueDate, setNewTodoDueDate] = useState('')
  const [isSubmitting, setIsSubmitting] = useState(false)

  // Separate recurring and one-off todos
  const recurringTodos = todos.filter((t) => t.frequency)
  const oneOffTodos = todos.filter((t) => !t.frequency)

  const handleAddTodo = async () => {
    if (!newTodoName.trim()) return

    setIsSubmitting(true)
    try {
      await addTodo({
        name: newTodoName.trim(),
        frequency: newTodoFrequency || undefined,
        daysOfWeek: newTodoFrequency && newTodoDaysOfWeek.length > 0 ? newTodoDaysOfWeek : undefined,
        dueDate: !newTodoFrequency && newTodoDueDate ? newTodoDueDate : undefined,
      })
      setNewTodoName('')
      setNewTodoFrequency('')
      setNewTodoDaysOfWeek([])
      setNewTodoDueDate('')
      setIsDialogOpen(false)
    } finally {
      setIsSubmitting(false)
    }
  }

  const handleToggleDayOfWeek = (day: number) => {
    setNewTodoDaysOfWeek((prev) =>
      prev.includes(day) ? prev.filter((d) => d !== day) : [...prev, day].sort()
    )
  }

  const handleDelete = async (todoId: string) => {
    await deleteTodo(todoId)
  }

  const handleStartTimer = async (todo: TodoWithStatus) => {
    await startTimer({
      taskName: todo.name,
      // Todos don't have milestone/goal associations
    })
  }

  return (
    <Card className={cn('flex flex-col', className)}>
      <CardHeader className="pb-2">
        <div className="flex items-center justify-between">
          <CardTitle className="text-base">今日のタスク</CardTitle>
          <div className="flex gap-1">
            <Button variant="ghost" size="icon" className="h-7 w-7" onClick={refresh}>
              <RefreshCw className="h-4 w-4" />
            </Button>
            <Button variant="ghost" size="icon" className="h-7 w-7" onClick={() => setIsDialogOpen(true)}>
              <Plus className="h-4 w-4" />
            </Button>
          </div>
        </div>
      </CardHeader>

      <CardContent className="flex-1 pt-0">
        {error && (
          <div className="flex items-center gap-2 text-destructive text-sm mb-2">
            <AlertCircle className="h-4 w-4" />
            {error}
          </div>
        )}

        {isLoading ? (
          <div className="flex items-center justify-center py-8">
            <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
          </div>
        ) : todos.length === 0 ? (
          <div className="text-center py-8 text-muted-foreground text-sm">
            タスクがありません
          </div>
        ) : (
          <ScrollArea className="h-[300px] pr-2">
            <div className="space-y-3">
              {/* Recurring todos */}
              {recurringTodos.length > 0 && (
                <div className="space-y-1">
                  {recurringTodos.map((todo) => (
                    <TodoItem
                      key={todo.id}
                      todo={todo}
                      onToggle={() => toggleComplete(todo.id)}
                      onDelete={() => handleDelete(todo.id)}
                      onStartTimer={() => handleStartTimer(todo)}
                      isTimerRunning={!!currentTimer}
                    />
                  ))}
                </div>
              )}

              {/* Separator if both types exist */}
              {recurringTodos.length > 0 && oneOffTodos.length > 0 && (
                <div className="border-t my-2" />
              )}

              {/* One-off todos */}
              {oneOffTodos.length > 0 && (
                <div className="space-y-1">
                  {oneOffTodos.map((todo) => (
                    <TodoItem
                      key={todo.id}
                      todo={todo}
                      onToggle={() => toggleComplete(todo.id)}
                      onDelete={() => handleDelete(todo.id)}
                      onStartTimer={() => handleStartTimer(todo)}
                      isTimerRunning={!!currentTimer}
                    />
                  ))}
                </div>
              )}
            </div>
          </ScrollArea>
        )}
      </CardContent>

      {/* Add Todo Dialog */}
      <Dialog open={isDialogOpen} onOpenChange={setIsDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>新しいタスクを追加</DialogTitle>
          </DialogHeader>

          <div className="space-y-4 py-4">
            <div>
              <Label htmlFor="todoName">タスク名</Label>
              <Input
                id="todoName"
                value={newTodoName}
                onChange={(e) => setNewTodoName(e.target.value)}
                placeholder="例: 書類提出"
                className="mt-1"
              />
            </div>

            <div>
              <Label>繰り返し設定（任意）</Label>
              <Select
                value={newTodoFrequency}
                onValueChange={(v) => setNewTodoFrequency(v as TodoFrequency | '')}
              >
                <SelectTrigger className="mt-1">
                  <SelectValue placeholder="繰り返しなし（単発）" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="">繰り返しなし（単発）</SelectItem>
                  <SelectItem value="daily">毎日</SelectItem>
                  <SelectItem value="weekly">週次（曜日指定）</SelectItem>
                  <SelectItem value="custom">カスタム（曜日指定）</SelectItem>
                </SelectContent>
              </Select>
            </div>

            {/* Days of week selection for weekly/custom */}
            {(newTodoFrequency === 'weekly' || newTodoFrequency === 'custom') && (
              <div>
                <Label>曜日</Label>
                <div className="flex gap-1 mt-1">
                  {DAYS_OF_WEEK.map((day, index) => (
                    <Button
                      key={index}
                      type="button"
                      variant={newTodoDaysOfWeek.includes(index) ? 'default' : 'outline'}
                      size="sm"
                      className="w-8 h-8 p-0"
                      onClick={() => handleToggleDayOfWeek(index)}
                    >
                      {day}
                    </Button>
                  ))}
                </div>
              </div>
            )}

            {/* Due date for one-off tasks */}
            {!newTodoFrequency && (
              <div>
                <Label htmlFor="dueDate">期日（任意）</Label>
                <Input
                  id="dueDate"
                  type="date"
                  value={newTodoDueDate}
                  onChange={(e) => setNewTodoDueDate(e.target.value)}
                  className="mt-1"
                />
              </div>
            )}
          </div>

          <DialogFooter>
            <Button variant="outline" onClick={() => setIsDialogOpen(false)}>
              キャンセル
            </Button>
            <Button onClick={handleAddTodo} disabled={!newTodoName.trim() || isSubmitting}>
              {isSubmitting && <Loader2 className="h-4 w-4 animate-spin mr-1" />}
              追加
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </Card>
  )
}

interface TodoItemProps {
  todo: TodoWithStatus
  onToggle: () => void
  onDelete: () => void
  onStartTimer: () => void
  isTimerRunning: boolean
}

function TodoItem({ todo, onToggle, onDelete, onStartTimer, isTimerRunning }: TodoItemProps) {
  const isRecurring = !!todo.frequency
  const frequencyLabel = formatFrequency(todo)

  return (
    <div
      className={cn(
        'group flex items-start gap-2 p-2 rounded-md hover:bg-muted/50 transition-colors',
        todo.isOverdue && !todo.completedToday && 'bg-destructive/10'
      )}
    >
      <button
        onClick={onToggle}
        className="mt-0.5 flex-shrink-0"
      >
        {todo.completedToday ? (
          <CheckCircle2 className="h-5 w-5 text-primary" />
        ) : (
          <Circle className="h-5 w-5 text-muted-foreground" />
        )}
      </button>

      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2">
          {isRecurring && (
            <RefreshCw className="h-3 w-3 text-muted-foreground flex-shrink-0" />
          )}
          <span
            className={cn(
              'text-sm truncate',
              todo.completedToday && 'line-through text-muted-foreground'
            )}
          >
            {todo.name}
          </span>
        </div>

        <div className="flex items-center gap-2 mt-0.5">
          {frequencyLabel && (
            <span className="text-xs text-muted-foreground">{frequencyLabel}</span>
          )}
          {todo.dueDate && !isRecurring && (
            <span
              className={cn(
                'text-xs',
                todo.isOverdue ? 'text-destructive' : 'text-muted-foreground'
              )}
            >
              期限: {todo.dueDate}
            </span>
          )}
          {todo.isOverdue && !todo.completedToday && (
            <AlertCircle className="h-3 w-3 text-destructive" />
          )}
        </div>
      </div>

      {/* Hover actions */}
      <div className="flex gap-1 opacity-0 group-hover:opacity-100 transition-opacity flex-shrink-0">
        {/* Timer start button - only show if no timer running */}
        {!isTimerRunning && !todo.completedToday && (
          <Button
            variant="ghost"
            size="icon"
            className="h-6 w-6"
            onClick={onStartTimer}
            title="タイマー開始"
          >
            <Play className="h-3 w-3 text-primary" />
          </Button>
        )}
        <ConfirmPopover
          message="このTodoを削除しますか？"
          confirmLabel="削除"
          onConfirm={onDelete}
          variant="destructive"
        >
          <Button
            variant="ghost"
            size="icon"
            className="h-6 w-6"
            title="削除"
          >
            <Trash2 className="h-3 w-3 text-muted-foreground" />
          </Button>
        </ConfirmPopover>
      </div>
    </div>
  )
}
