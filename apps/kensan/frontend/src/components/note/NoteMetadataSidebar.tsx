import { Label } from '@/components/ui/label'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible'
import { TagInput } from '@/components/common/TagInput'
import { MetadataForm } from '@/components/note/MetadataForm'
import { getNoteTypeIcon } from '@/lib/noteTypeIcons'
import { useNoteTypeStore } from '@/stores/useNoteTypeStore'
import {
  Shapes,
  GitFork,
  Calendar,
  Target,
  Milestone,
  ListTodo,
  ChevronDown,
  Clock,
  Archive,
  Info,
} from 'lucide-react'
import type {
  NoteType,
  Goal,
  Milestone as MilestoneType,
  Task,
  Tag as TagType,
  Note,
} from '@/types'
import type { NoteEditorValue } from '@/components/note/NoteEditor'
import { format } from 'date-fns'
import { useState } from 'react'

interface NoteMetadataSidebarProps {
  value: NoteEditorValue
  onChange: (value: NoteEditorValue) => void
  goals?: Goal[]
  milestones?: MilestoneType[]
  tasks?: Task[]
  tags?: TagType[]
  onCreateTag?: (name: string, color: string) => Promise<TagType>
  onUpdateTag?: (id: string, data: { name: string; color: string }) => Promise<TagType>
  showTypeSelector?: boolean
  existingNote?: Note | null
}

export function NoteMetadataSidebar({
  value,
  onChange,
  goals = [],
  milestones = [],
  tasks = [],
  tags = [],
  onCreateTag,
  onUpdateTag,
  showTypeSelector = false,
  existingNote,
}: NoteMetadataSidebarProps) {
  const { types, getConstraints, getMetadataSchema } = useNoteTypeStore()
  const typeConstraints = getConstraints(value.type)
  const metadataSchema = getMetadataSchema(value.type)
  const isDateRequired = typeConstraints?.dateRequired ?? false

  const [relatedOpen, setRelatedOpen] = useState(false)

  // Filter milestones by selected goal
  const filteredMilestones = value.goalId
    ? milestones.filter((m) => m.goalId === value.goalId)
    : milestones

  // Filter tasks by selected milestone
  const filteredTasks = value.milestoneId
    ? tasks.filter((t) => t.milestoneId === value.milestoneId)
    : value.goalId
    ? tasks.filter((t) => {
        const milestone = milestones.find((m) => m.id === t.milestoneId)
        return milestone?.goalId === value.goalId
      })
    : tasks

  const handleChange = (partial: Partial<NoteEditorValue>) => {
    onChange({ ...value, ...partial })
  }

  const handleGoalChange = (goalId: string | undefined) => {
    handleChange({
      goalId,
      milestoneId: undefined,
      taskId: undefined,
    })
  }

  const handleMilestoneChange = (milestoneId: string | undefined) => {
    handleChange({
      milestoneId,
      taskId: undefined,
    })
  }

  const handleTagsChange = (tagIds: string[]) => {
    handleChange({ tagIds })
  }

  const handleTypeMetadataChange = (typeMetadata: Record<string, string>) => {
    handleChange({ typeMetadata })
  }

  // Display name helpers
  const getGoalDisplayName = (goalId: string | undefined): string | undefined => {
    if (!goalId || goalId === '_none') return undefined
    return goals.find((g) => g.id === goalId)?.name
  }

  const getMilestoneDisplayName = (milestoneId: string | undefined): string | undefined => {
    if (!milestoneId || milestoneId === '_none') return undefined
    return filteredMilestones.find((m) => m.id === milestoneId)?.name
  }

  const getTaskDisplayName = (taskId: string | undefined): string | undefined => {
    if (!taskId || taskId === '_none') return undefined
    return filteredTasks.find((t) => t.id === taskId)?.name
  }

  const typeConfig = types.find((t) => t.slug === value.type)

  return (
    <div className="space-y-5">
      {/* 1. Note type selector (new notes only) */}
      {showTypeSelector && (
        <div className="space-y-1.5">
          <Label className="text-xs text-muted-foreground">ノートタイプ</Label>
          <Select
            value={value.type}
            onValueChange={(t) => handleChange({ type: t as NoteType, typeMetadata: {} })}
          >
            <SelectTrigger className="h-9">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {types.map((noteType) => {
                const Icon = getNoteTypeIcon(noteType.icon)
                return (
                  <SelectItem key={noteType.slug} value={noteType.slug}>
                    <div className="flex items-center gap-2">
                      <Icon className="h-4 w-4" />
                      {noteType.displayName}
                    </div>
                  </SelectItem>
                )
              })}
            </SelectContent>
          </Select>
        </div>
      )}

      {/* 2. Date (when type requires it) */}
      {isDateRequired && (
        <div className="space-y-1.5">
          <Label className="text-xs text-muted-foreground flex items-center gap-1.5">
            <Calendar className="h-3.5 w-3.5" />
            日付 *
          </Label>
          <Input
            type="date"
            className="h-9"
            value={value.date || format(new Date(), 'yyyy-MM-dd')}
            onChange={(e) => handleChange({ date: e.target.value })}
          />
        </div>
      )}

      {/* 3. Draw.io toggle */}
      <div className="space-y-1.5">
        <div className="flex items-center justify-between">
          <Label className="text-xs text-muted-foreground flex items-center gap-1.5">
            <Shapes className="h-3.5 w-3.5" />
            draw.io 図を含む
          </Label>
          <Switch
            checked={value.hasDrawio}
            onCheckedChange={(checked) => handleChange({ hasDrawio: checked })}
          />
        </div>
      </div>

      {/* 3b. Mindmap toggle */}
      <div className="space-y-1.5">
        <div className="flex items-center justify-between">
          <Label className="text-xs text-muted-foreground flex items-center gap-1.5">
            <GitFork className="h-3.5 w-3.5" />
            マインドマップを含む
          </Label>
          <Switch
            checked={value.hasMindmap}
            onCheckedChange={(checked) => handleChange({ hasMindmap: checked })}
          />
        </div>
      </div>

      {/* 4. Tags */}
      <TagInput
        tags={tags}
        selectedTagIds={value.tagIds || []}
        onChange={handleTagsChange}
        onCreateTag={onCreateTag}
        onUpdateTag={onUpdateTag}
      />

      {/* 5. Type-specific metadata */}
      {metadataSchema.length > 0 && (
        <MetadataForm
          schema={metadataSchema}
          values={value.typeMetadata ?? {}}
          onChange={handleTypeMetadataChange}
        />
      )}

      {/* 6. Related info (goal/milestone/task) - collapsible */}
      {(goals.length > 0 || filteredMilestones.length > 0 || filteredTasks.length > 0) && (
        <Collapsible open={relatedOpen} onOpenChange={setRelatedOpen}>
          <CollapsibleTrigger asChild>
            <button className="flex items-center gap-1.5 text-xs text-muted-foreground hover:text-foreground w-full">
              <ChevronDown
                className={`h-3.5 w-3.5 transition-transform ${relatedOpen ? '' : '-rotate-90'}`}
              />
              関連情報（任意）
            </button>
          </CollapsibleTrigger>
          <CollapsibleContent className="space-y-3 pt-2">
            {/* Goal selector */}
            {goals.length > 0 && (
              <div className="space-y-1.5">
                <Label className="text-xs text-muted-foreground flex items-center gap-1.5">
                  <Target className="h-3.5 w-3.5" />
                  目標
                </Label>
                <Select
                  value={value.goalId || '_none'}
                  onValueChange={(v) => handleGoalChange(v === '_none' ? undefined : v)}
                >
                  <SelectTrigger className="h-9">
                    <span className="truncate">
                      {getGoalDisplayName(value.goalId) || '目標を選択'}
                    </span>
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="_none">なし</SelectItem>
                    {goals
                      .filter((g) => g.status !== 'archived')
                      .map((goal) => (
                        <SelectItem key={goal.id} value={goal.id}>
                          <div className="flex items-center gap-2">
                            <div
                              className="w-3 h-3 rounded-full"
                              style={{ backgroundColor: goal.color }}
                            />
                            {goal.name}
                          </div>
                        </SelectItem>
                      ))}
                  </SelectContent>
                </Select>
              </div>
            )}

            {/* Milestone selector */}
            {filteredMilestones.length > 0 && (
              <div className="space-y-1.5">
                <Label className="text-xs text-muted-foreground flex items-center gap-1.5">
                  <Milestone className="h-3.5 w-3.5" />
                  マイルストーン
                </Label>
                <Select
                  value={value.milestoneId || '_none'}
                  onValueChange={(v) => handleMilestoneChange(v === '_none' ? undefined : v)}
                >
                  <SelectTrigger className="h-9">
                    <span className="truncate">
                      {getMilestoneDisplayName(value.milestoneId) || 'マイルストーンを選択'}
                    </span>
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="_none">なし</SelectItem>
                    {filteredMilestones
                      .filter((m) => m.status === 'active')
                      .map((milestone) => (
                        <SelectItem key={milestone.id} value={milestone.id}>
                          {milestone.name}
                        </SelectItem>
                      ))}
                  </SelectContent>
                </Select>
              </div>
            )}

            {/* Task selector */}
            {filteredTasks.length > 0 && (
              <div className="space-y-1.5">
                <Label className="text-xs text-muted-foreground flex items-center gap-1.5">
                  <ListTodo className="h-3.5 w-3.5" />
                  関連タスク
                </Label>
                <Select
                  value={value.taskId || '_none'}
                  onValueChange={(v) => handleChange({ taskId: v === '_none' ? undefined : v })}
                >
                  <SelectTrigger className="h-9">
                    <span className="truncate">
                      {getTaskDisplayName(value.taskId) || 'タスクを選択'}
                    </span>
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="_none">なし</SelectItem>
                    {filteredTasks
                      .filter((t) => !t.completed)
                      .slice(0, 20)
                      .map((task) => (
                        <SelectItem key={task.id} value={task.id}>
                          {task.name}
                        </SelectItem>
                      ))}
                  </SelectContent>
                </Select>
              </div>
            )}
          </CollapsibleContent>
        </Collapsible>
      )}

      {/* 7. Info (created/updated, archive status) */}
      {existingNote && (
        <div className="space-y-1.5 pt-2 border-t">
          <div className="flex items-center gap-1.5 text-xs text-muted-foreground">
            <Clock className="h-3.5 w-3.5" />
            情報
          </div>
          <div className="space-y-1 text-xs">
            <div className="flex justify-between text-muted-foreground">
              <span>作成日</span>
              <span>{format(existingNote.createdAt, 'yyyy-MM-dd HH:mm')}</span>
            </div>
            <div className="flex justify-between text-muted-foreground">
              <span>更新日</span>
              <span>{format(existingNote.updatedAt, 'yyyy-MM-dd HH:mm')}</span>
            </div>
            {existingNote.archived && (
              <div className="flex items-center gap-1.5 text-amber-600 dark:text-amber-400 mt-1">
                <Archive className="h-3.5 w-3.5" />
                アーカイブ済み
              </div>
            )}
          </div>
        </div>
      )}

      {/* 8. Hint */}
      {typeConfig?.description && (
        <div className="flex items-start gap-1.5 text-xs text-muted-foreground pt-2 border-t">
          <Info className="h-3.5 w-3.5 mt-0.5 shrink-0" />
          <span>{typeConfig.description}</span>
        </div>
      )}
    </div>
  )
}
