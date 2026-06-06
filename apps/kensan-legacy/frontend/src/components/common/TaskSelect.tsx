import { useState, useMemo, useRef, useEffect } from 'react'
import { Search, ChevronDown, Check } from 'lucide-react'
import { cn } from '@/lib/utils'
import type { Task, Goal, Milestone } from '@/types'

interface TaskSelectProps {
  tasks: Task[]
  goals: Goal[]
  milestones: Milestone[]
  value: string
  onValueChange: (value: string) => void
  placeholder?: string
  /** Filter function for tasks (e.g., exclude completed) */
  filterTask?: (task: Task) => boolean
  /** Get milestone by ID */
  getMilestoneById: (id: string) => Milestone | undefined
  /** Get goal by ID */
  getGoalById: (id: string) => Goal | undefined
}

interface GroupedTasks {
  goal: Goal | null
  milestones: {
    milestone: Milestone | null
    tasks: Task[]
  }[]
}

export function TaskSelect({
  tasks,
  goals,
  milestones,
  value,
  onValueChange,
  placeholder = 'タスクを選択',
  filterTask = (t) => !t.completed && !t.parentTaskId,
  getMilestoneById,
  getGoalById,
}: TaskSelectProps) {
  const [open, setOpen] = useState(false)
  const [search, setSearch] = useState('')
  const inputRef = useRef<HTMLInputElement>(null)
  const containerRef = useRef<HTMLDivElement>(null)

  // Focus input when opened
  useEffect(() => {
    if (open && inputRef.current) {
      inputRef.current.focus()
    }
  }, [open])

  // Close on outside click
  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
        setOpen(false)
      }
    }
    if (open) {
      document.addEventListener('mousedown', handleClickOutside)
      return () => document.removeEventListener('mousedown', handleClickOutside)
    }
  }, [open])

  // Filter tasks
  const filteredTasks = useMemo(() => {
    return tasks.filter(filterTask)
  }, [tasks, filterTask])

  // Search filter
  const searchedTasks = useMemo(() => {
    if (!search.trim()) return filteredTasks
    const query = search.toLowerCase()
    return filteredTasks.filter(task => {
      // Search in task name
      if (task.name.toLowerCase().includes(query)) return true
      // Search in milestone name
      if (task.milestoneId) {
        const milestone = getMilestoneById(task.milestoneId)
        if (milestone?.name.toLowerCase().includes(query)) return true
        // Search in goal name
        if (milestone) {
          const goal = getGoalById(milestone.goalId)
          if (goal?.name.toLowerCase().includes(query)) return true
        }
      }
      return false
    })
  }, [filteredTasks, search, getMilestoneById, getGoalById])

  // Group tasks by Goal > Milestone
  const groupedTasks = useMemo((): GroupedTasks[] => {
    const groups: GroupedTasks[] = []
    const goalMap = new Map<string | null, Map<string | null, Task[]>>()

    searchedTasks.forEach(task => {
      const milestone = task.milestoneId ? getMilestoneById(task.milestoneId) : undefined
      const goalId = milestone?.goalId || null
      const milestoneId = task.milestoneId || null

      if (!goalMap.has(goalId)) {
        goalMap.set(goalId, new Map())
      }
      const milestoneMap = goalMap.get(goalId)!
      if (!milestoneMap.has(milestoneId)) {
        milestoneMap.set(milestoneId, [])
      }
      milestoneMap.get(milestoneId)!.push(task)
    })

    // Convert to array, goals with tasks first, then standalone
    const goalsWithTasks = goals.filter(g => g.status !== 'archived' && goalMap.has(g.id))

    goalsWithTasks.forEach(goal => {
      const milestoneMap = goalMap.get(goal.id)!
      const goalMilestones = milestones
        .filter(m => m.goalId === goal.id && milestoneMap.has(m.id))
        .map(milestone => ({
          milestone,
          tasks: milestoneMap.get(milestone.id) || [],
        }))

      if (goalMilestones.length > 0) {
        groups.push({ goal, milestones: goalMilestones })
      }
    })

    // Standalone tasks (no milestone/goal)
    if (goalMap.has(null)) {
      const standaloneTasks = goalMap.get(null)?.get(null) || []
      if (standaloneTasks.length > 0) {
        groups.push({
          goal: null,
          milestones: [{ milestone: null, tasks: standaloneTasks }],
        })
      }
    }

    return groups
  }, [searchedTasks, goals, milestones, getMilestoneById])

  // Get selected task info for display
  const selectedTask = value ? tasks.find(t => t.id === value) : undefined
  const selectedMilestone = selectedTask?.milestoneId
    ? getMilestoneById(selectedTask.milestoneId)
    : undefined
  const selectedGoal = selectedMilestone
    ? getGoalById(selectedMilestone.goalId)
    : undefined

  const handleSelect = (taskId: string) => {
    onValueChange(taskId)
    setOpen(false)
    setSearch('')
  }

  return (
    <div ref={containerRef} className="relative">
      {/* Trigger */}
      <button
        type="button"
        onClick={() => setOpen(!open)}
        className={cn(
          'flex h-9 w-full items-center justify-between rounded-md border border-input bg-transparent px-3 py-2 text-sm shadow-sm ring-offset-background',
          'focus:outline-none focus:ring-1 focus:ring-ring',
          'disabled:cursor-not-allowed disabled:opacity-50'
        )}
      >
        <span className={cn(!selectedTask && 'text-muted-foreground')}>
          {selectedTask ? (
            <span className="flex items-center gap-2">
              {selectedGoal && (
                <span
                  className="w-2 h-2 rounded-full flex-shrink-0"
                  style={{ backgroundColor: selectedGoal.color }}
                />
              )}
              {selectedTask.name}
            </span>
          ) : (
            placeholder
          )}
        </span>
        <ChevronDown className="h-4 w-4 opacity-50" />
      </button>

      {/* Dropdown */}
      {open && (
        <div className="absolute z-50 mt-1 w-full rounded-md border bg-popover text-popover-foreground shadow-md">
          {/* Search input */}
          <div className="flex items-center border-b px-3 py-2">
            <Search className="h-4 w-4 text-muted-foreground mr-2 flex-shrink-0" />
            <input
              ref={inputRef}
              type="text"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder="タスクを検索..."
              className="flex-1 bg-transparent text-sm outline-none placeholder:text-muted-foreground"
            />
          </div>

          {/* Task list */}
          <div className="max-h-72 overflow-y-auto p-1">
            {groupedTasks.length === 0 ? (
              <div className="py-6 text-center text-sm text-muted-foreground">
                タスクが見つかりません
              </div>
            ) : (
              groupedTasks.map((group, groupIndex) => (
                <div key={group.goal?.id || '__standalone__'}>
                  {/* Separator before standalone */}
                  {group.goal === null && groupIndex > 0 && (
                    <div className="my-1 border-t" />
                  )}

                  {/* Goal header */}
                  {group.goal ? (
                    <div className="px-2 py-1.5 text-xs font-medium text-muted-foreground flex items-center gap-2 sticky top-0 bg-popover">
                      <span
                        className="w-2 h-2 rounded-full flex-shrink-0"
                        style={{ backgroundColor: group.goal.color }}
                      />
                      {group.goal.name}
                    </div>
                  ) : (
                    <div className="px-2 py-1.5 text-xs font-medium text-muted-foreground">
                      目標なし
                    </div>
                  )}

                  {/* Milestones and tasks */}
                  {group.milestones.map(({ milestone, tasks: milestoneTasks }) => (
                    <div key={milestone?.id || '__no_milestone__'}>
                      {/* Milestone header */}
                      {milestone && (
                        <div className="px-4 py-1 text-xs text-muted-foreground flex items-center gap-1">
                          <span className="opacity-50">┈</span>
                          {milestone.name}
                        </div>
                      )}

                      {/* Tasks */}
                      {milestoneTasks.map(task => {
                        const isSelected = value === task.id
                        return (
                          <div
                            key={task.id}
                            onClick={() => handleSelect(task.id)}
                            className={cn(
                              'relative flex cursor-pointer select-none items-center rounded-sm py-1.5 pr-8 text-sm outline-none',
                              'hover:bg-accent hover:text-accent-foreground',
                              milestone ? 'pl-8' : 'pl-4',
                              isSelected && 'bg-accent'
                            )}
                          >
                            {task.name}
                            {isSelected && (
                              <Check className="absolute right-2 h-4 w-4" />
                            )}
                          </div>
                        )
                      })}
                    </div>
                  ))}
                </div>
              ))
            )}
          </div>
        </div>
      )}
    </div>
  )
}
