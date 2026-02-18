import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { GoalBadge } from '@/components/common/GoalBadge'
import { TagBadge } from '@/components/common/TagBadge'
import { ConfirmPopover } from '@/components/common/ConfirmPopover'
import { GoalDialog } from '@/components/task/GoalDialog'
import { MilestoneDialog } from '@/components/task/MilestoneDialog'
import { TagDialog } from '@/components/task/TagDialog'
import { SortableGoalItem } from '@/components/task/SortableGoalItem'
import { SortableTaskItem } from '@/components/task/SortableTaskItem'
import { ChildTaskList } from '@/components/task/ChildTaskList'
import { TaskDetailPanel } from '@/components/task/TaskDetailPanel'
import { Sheet, SheetContent } from '@/components/ui/sheet'
import { EmptyState } from '@/components/common/EmptyState'
import { useTaskManagement } from '@/hooks/useTaskManagement'
import {
  FolderKanban, Plus, Search, ChevronRight,
  Target, Flag, Edit, Trash2, CheckCircle2,
  ListTodo, Tags, X, Eye, EyeOff,
} from 'lucide-react'
import { Progress } from '@/components/ui/progress'
import { ScrollArea } from '@/components/ui/scroll-area'
import { cn } from '@/lib/utils'
import { GanttChartWidget } from '@/components/task/GanttChartWidget'
import { PageMemo } from '@/components/common/PageMemo'
import { PageGuide } from '@/components/guide/PageGuide'
import { DndContext, closestCenter } from '@dnd-kit/core'
import { SortableContext, verticalListSortingStrategy } from '@dnd-kit/sortable'

export function T01TaskManagement() {
  const tm = useTaskManagement()

  // Task list rendering helper (used for both milestone and standalone tasks)
  const renderTaskList = (taskList: typeof tm.sortedMilestoneTasks) => (
    <DndContext sensors={tm.sensors} collisionDetection={closestCenter} onDragEnd={tm.handleDragEnd}>
      <SortableContext items={taskList.map(t => t.id)} strategy={verticalListSortingStrategy}>
        {taskList.map(task => {
          const childTasks = tm.getChildTasks(task.id)
          const filteredChildTasks = tm.filterTasks(childTasks)
          const hasChildren = filteredChildTasks.length > 0
          const isTaskExpanded = tm.expandedTasks.has(task.id)

          return (
            <SortableTaskItem
              key={task.id}
              task={task}
              isSelected={tm.selectedTaskIds.has(task.id)}
              isSelectionMode={tm.isSelectionMode}
              onSelect={tm.handleSelectTask}
              onToggleComplete={tm.handleToggleTaskComplete}
              onEdit={tm.openEditTaskDialog}
              onDelete={tm.handleDeleteTask}
              onAddSubtask={tm.openNewTaskDialog}
              getTagsByIds={tm.getTagsByIds}
              hasChildren={hasChildren}
              isExpanded={isTaskExpanded}
              onToggleExpand={tm.toggleTask}
              recentlyCompleted={tm.recentlyCompleted.has(task.id)}
            >
              {hasChildren && isTaskExpanded && (
                <ChildTaskList
                  childTasks={filteredChildTasks}
                  isSelectionMode={tm.isSelectionMode}
                  selectedTaskIds={tm.selectedTaskIds}
                  recentlyCompleted={tm.recentlyCompleted}
                  onSelect={tm.handleSelectTask}
                  onToggleComplete={tm.handleToggleTaskComplete}
                  onEdit={tm.openEditTaskDialog}
                  onDelete={tm.handleDeleteTask}
                />
              )}
            </SortableTaskItem>
          )
        })}
      </SortableContext>

      {taskList.length === 0 && (
        <EmptyState icon={ListTodo} message="タスクがありません" actionLabel="追加する" onAction={() => tm.openNewTaskDialog()} />
      )}
    </DndContext>
  )

  return (
    <div className="space-y-4">
      <PageGuide pageId="tasks" />

      {/* ヘッダー */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <FolderKanban className="h-8 w-8 text-slate-500" />
          <h1 className="text-2xl font-bold">タスク管理</h1>
        </div>
        <div className="flex gap-4 items-center" data-guide="task-search">
          <div className="relative">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
            <Input
              placeholder="検索..."
              value={tm.searchQuery}
              onChange={e => tm.setSearchQuery(e.target.value)}
              className="pl-10 w-64"
            />
          </div>
          <label className="flex items-center gap-2 cursor-pointer">
            {tm.hideCompleted ? <EyeOff className="h-4 w-4 text-muted-foreground" /> : <Eye className="h-4 w-4 text-muted-foreground" />}
            <span className="text-sm text-muted-foreground">完了済み</span>
            <Switch checked={!tm.hideCompleted} onCheckedChange={checked => tm.setHideCompleted(!checked)} />
          </label>
        </div>
      </div>

      {/* メモ + ガントチャート */}
      <PageMemo pageId="task-management" title="タスク管理メモ" placeholder="タスクの優先度、検討事項、メモなど..." />
      <GanttChartWidget goals={tm.goals} milestones={tm.milestones} tasks={tm.tasks} hideCompleted={tm.hideCompleted} />

      {/* 3カラムレイアウト */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-4 h-[calc(100vh-280px)]">
        {/* カラム1: 目標 */}
        <Card className="flex flex-col" data-guide="task-goals">
          <CardHeader className="py-3 px-4 border-b flex-shrink-0">
            <CardTitle className="text-sm font-medium flex items-center gap-2">
              <Target className="h-4 w-4" />
              目標
              <Button variant="ghost" size="sm" className="h-7 w-7 p-0 ml-auto" onClick={() => tm.goalDialog.open()}>
                <Plus className="h-4 w-4" />
              </Button>
            </CardTitle>
          </CardHeader>
          <ScrollArea className="flex-1">
            <CardContent className="p-2 space-y-1">
              <DndContext sensors={tm.sensors} collisionDetection={closestCenter} onDragEnd={tm.handleGoalDragEnd}>
                <SortableContext items={tm.filteredGoals.map(g => g.id)} strategy={verticalListSortingStrategy}>
                  {tm.filteredGoals.map(goal => (
                    <SortableGoalItem
                      key={goal.id}
                      goal={goal}
                      isSelected={tm.selectedGoalId === goal.id}
                      progress={tm.calculateGoalProgress(goal.id)}
                      milestoneCount={tm.milestones.filter(m => m.goalId === goal.id && m.status !== 'archived').length}
                      onSelect={tm.setSelectedGoalId}
                      onEdit={tm.openEditGoalDialog}
                      onDelete={tm.handleDeleteGoal}
                      onComplete={tm.handleCompleteGoal}
                    />
                  ))}
                </SortableContext>
              </DndContext>

              {tm.filteredGoals.length === 0 && (
                <EmptyState icon={Target} message="目標がありません" />
              )}

              {/* 目標なしタスクセクション */}
              {tm.standaloneTasks.length > 0 && (
                <div
                  className={cn(
                    'p-3 rounded-lg cursor-pointer transition-colors mt-4 border-t pt-4',
                    tm.isStandaloneSelected ? 'bg-muted/50 border border-muted' : 'hover:bg-muted/30'
                  )}
                  onClick={() => {
                    tm.setSelectedGoalId('__standalone__')
                    tm.setSelectedMilestoneId(null)
                  }}
                >
                  <div className="flex items-center gap-2 text-muted-foreground">
                    <ListTodo className="h-4 w-4" />
                    <span className="text-sm">目標なしタスク</span>
                    <span className="text-xs ml-auto">{tm.standaloneTasks.length}件</span>
                  </div>
                </div>
              )}

              {/* タグ管理セクション */}
              <div className="mt-4 pt-4 border-t">
                <div className="flex items-center gap-2 mb-2">
                  <Tags className="h-4 w-4 text-muted-foreground" />
                  <span className="text-sm font-medium">タグ</span>
                  <Button variant="ghost" size="sm" className="h-6 w-6 p-0 ml-auto" onClick={() => tm.tagDialog.open()}>
                    <Plus className="h-3.5 w-3.5" />
                  </Button>
                </div>
                <div className="flex flex-wrap gap-1.5">
                  {tm.tags.length === 0 ? (
                    <span className="text-xs text-muted-foreground">タグがありません</span>
                  ) : (
                    tm.tags.map(tag => (
                      <div key={tag.id} className="group relative">
                        <TagBadge name={tag.name} color={tag.color} />
                        <div className="absolute -top-1 -right-1 hidden group-hover:flex gap-0.5">
                          <button
                            onClick={() => tm.openEditTagDialog(tag)}
                            className="w-4 h-4 bg-background border rounded-full flex items-center justify-center hover:bg-muted"
                          >
                            <Edit className="h-2.5 w-2.5" />
                          </button>
                          <ConfirmPopover
                            message="このタグを削除しますか？"
                            confirmLabel="削除"
                            onConfirm={() => tm.handleDeleteTag(tag.id)}
                            variant="destructive"
                          >
                            <button className="w-4 h-4 bg-background border rounded-full flex items-center justify-center hover:bg-destructive hover:text-destructive-foreground">
                              <Trash2 className="h-2.5 w-2.5" />
                            </button>
                          </ConfirmPopover>
                        </div>
                      </div>
                    ))
                  )}
                </div>
              </div>
            </CardContent>
          </ScrollArea>
        </Card>

        {/* カラム2: マイルストーン */}
        <Card className="flex flex-col" data-guide="task-milestones">
          <CardHeader className="py-3 px-4 border-b flex-shrink-0">
            <CardTitle className="text-sm font-medium flex items-center gap-2">
              <Flag className="h-4 w-4" />
              マイルストーン
              {tm.selectedGoal && (
                <>
                  <ChevronRight className="h-3 w-3 text-muted-foreground" />
                  <GoalBadge name={tm.selectedGoal.name} color={tm.selectedGoal.color} size="sm" />
                </>
              )}
              <Button
                variant="ghost" size="sm" className="h-7 w-7 p-0 ml-auto"
                onClick={() => tm.openNewMilestoneDialog()}
                disabled={!tm.selectedGoalId || tm.isStandaloneSelected}
              >
                <Plus className="h-4 w-4" />
              </Button>
            </CardTitle>
          </CardHeader>
          <ScrollArea className="flex-1">
            <CardContent className="p-2 space-y-1">
              {tm.selectedGoalId && !tm.isStandaloneSelected ? (
                <>
                  {tm.selectedGoalMilestones.map(milestone => {
                    const progress = tm.calculateMilestoneProgress(milestone.id)
                    const isSelected = tm.selectedMilestoneId === milestone.id
                    const isCompleted = milestone.status === 'completed'

                    return (
                      <div
                        key={milestone.id}
                        className={cn(
                          'p-3 rounded-lg cursor-pointer transition-colors group',
                          isSelected ? 'bg-primary/10 border border-primary/30' : 'hover:bg-muted/50',
                          isCompleted && 'opacity-60'
                        )}
                        onClick={() => tm.setSelectedMilestoneId(milestone.id)}
                      >
                        <div className="flex items-center gap-2">
                          {isCompleted ? (
                            <CheckCircle2 className="h-4 w-4 text-green-500 flex-shrink-0" />
                          ) : (
                            <Flag className="h-4 w-4 text-muted-foreground flex-shrink-0" />
                          )}
                          <span className={cn('font-medium text-sm flex-1', isSelected && 'text-primary', isCompleted && 'line-through')}>
                            {milestone.name}
                          </span>
                          <div className="opacity-0 group-hover:opacity-100 flex gap-1">
                            {!isCompleted && (
                              <ConfirmPopover
                                message={tm.getMilestoneCompleteMessage(milestone.id)}
                                confirmLabel="完了"
                                onConfirm={() => tm.handleCompleteMilestone(milestone.id)}
                              >
                                <Button variant="ghost" size="sm" className="h-6 w-6 p-0 text-green-600" onClick={e => e.stopPropagation()} title="完了">
                                  <CheckCircle2 className="h-3 w-3" />
                                </Button>
                              </ConfirmPopover>
                            )}
                            <Button variant="ghost" size="sm" className="h-6 w-6 p-0" onClick={e => { e.stopPropagation(); tm.openEditMilestoneDialog(milestone) }}>
                              <Edit className="h-3 w-3" />
                            </Button>
                            <ConfirmPopover message="このマイルストーンと配下のタスクを削除しますか？" confirmLabel="削除" onConfirm={() => tm.handleDeleteMilestone(milestone.id)} variant="destructive">
                              <Button variant="ghost" size="sm" className="h-6 w-6 p-0" onClick={e => e.stopPropagation()}>
                                <Trash2 className="h-3 w-3" />
                              </Button>
                            </ConfirmPopover>
                          </div>
                        </div>
                        {(milestone.startDate || milestone.targetDate) && (
                          <div className="mt-1 text-xs text-muted-foreground">
                            {milestone.startDate && milestone.targetDate
                              ? `${milestone.startDate} 〜 ${milestone.targetDate}`
                              : milestone.targetDate ? `期限: ${milestone.targetDate}` : `開始: ${milestone.startDate}`}
                          </div>
                        )}
                        <div className="mt-2 flex items-center gap-2">
                          <Progress value={progress.percentage} className="h-1.5 flex-1" />
                          <span className="text-xs text-muted-foreground w-16 text-right">{progress.completed}/{progress.total}</span>
                        </div>
                      </div>
                    )
                  })}

                  {tm.selectedGoalMilestones.length === 0 && (
                    <EmptyState icon={Flag} message="マイルストーンがありません" actionLabel="追加する" onAction={() => tm.openNewMilestoneDialog()} />
                  )}
                </>
              ) : tm.isStandaloneSelected ? (
                <EmptyState icon={ListTodo} message="目標なしタスクを選択中" />
              ) : (
                <EmptyState icon={Flag} message="目標を選択してください" />
              )}
            </CardContent>
          </ScrollArea>
        </Card>

        {/* カラム3: タスク */}
        <Card className="flex flex-col" data-guide="task-tasks">
          <CardHeader className="py-3 px-4 border-b flex-shrink-0">
            {tm.isSelectionMode ? (
              <div className="flex items-center gap-2">
                <Button variant="ghost" size="sm" className="h-7 w-7 p-0" onClick={tm.handleClearSelection}>
                  <X className="h-4 w-4" />
                </Button>
                <span className="text-sm font-medium">{tm.selectedTaskIds.size}件選択中</span>
                <div className="ml-auto flex gap-1">
                  <Button variant="outline" size="sm" className="h-7 text-xs" onClick={tm.handleSelectAll}>全選択</Button>
                  <Button variant="outline" size="sm" className="h-7 text-xs text-green-600 hover:text-green-700" onClick={() => tm.handleBulkComplete(true)}>
                    <CheckCircle2 className="h-3 w-3 mr-1" />完了
                  </Button>
                  <ConfirmPopover message={`${tm.selectedTaskIds.size}件のタスクを削除しますか？`} confirmLabel="削除" onConfirm={tm.handleBulkDelete} variant="destructive">
                    <Button variant="outline" size="sm" className="h-7 text-xs text-destructive hover:text-destructive">
                      <Trash2 className="h-3 w-3 mr-1" />削除
                    </Button>
                  </ConfirmPopover>
                </div>
              </div>
            ) : (
              <CardTitle className="text-sm font-medium flex items-center gap-2">
                <ListTodo className="h-4 w-4" />
                タスク
                {tm.selectedMilestone && (
                  <>
                    <ChevronRight className="h-3 w-3 text-muted-foreground" />
                    <span className="text-xs text-muted-foreground truncate max-w-[120px]">{tm.selectedMilestone.name}</span>
                  </>
                )}
                <Button
                  variant="ghost" size="sm" className="h-7 w-7 p-0 ml-auto"
                  onClick={() => tm.openNewTaskDialog()}
                  disabled={!tm.selectedMilestoneId && !tm.isStandaloneSelected}
                >
                  <Plus className="h-4 w-4" />
                </Button>
              </CardTitle>
            )}
          </CardHeader>
          <ScrollArea className="flex-1">
            <CardContent className="p-2 space-y-1">
              {tm.selectedMilestoneId && renderTaskList(tm.sortedMilestoneTasks)}
              {tm.isStandaloneSelected && tm.sortedStandaloneTasks.length > 0 && renderTaskList(tm.sortedStandaloneTasks)}

              {!tm.selectedMilestoneId && tm.selectedGoalId && !tm.isStandaloneSelected && (
                <EmptyState icon={ListTodo} message="マイルストーンを選択してください" />
              )}
            </CardContent>
          </ScrollArea>
        </Card>
      </div>

      {/* Dialogs */}
      <GoalDialog dialog={tm.goalDialog} onSave={tm.handleSaveGoal} />
      <MilestoneDialog dialog={tm.milestoneDialog} goals={tm.goals} onSave={tm.handleSaveMilestone} />
      <TagDialog dialog={tm.tagDialog} onSave={tm.handleSaveTag} />

      {/* Task Detail Panel (create + edit) */}
      <Sheet open={tm.taskDetailPanel.isOpen} onOpenChange={open => { if (!open) tm.taskDetailPanel.closeTask() }}>
        <SheetContent>
          <TaskDetailPanel
            taskId={tm.taskDetailPanel.selectedTaskId}
            createContext={tm.taskDetailPanel.newTaskContext}
            onCreated={tm.taskDetailPanel.switchToCreatedTask}
            onClose={tm.taskDetailPanel.closeTask}
          />
        </SheetContent>
      </Sheet>
    </div>
  )
}
