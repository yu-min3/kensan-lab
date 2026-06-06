import { Label } from '@/components/ui/label'
import { Input } from '@/components/ui/input'
import { MarkdownEditor } from '@/components/editor/MarkdownEditor'
import { DrawioEditor } from '@/components/editor/DrawioEditor'
import { MindmapEditor } from '@/components/editor/MindmapEditor'
import { Badge } from '@/components/ui/badge'
import { useNoteTypeStore } from '@/stores/useNoteTypeStore'
import { Target, Milestone, ListTodo } from 'lucide-react'
import type { NoteType, NoteFormat } from '@/types'

export interface NoteEditorValue {
  type: NoteType
  title?: string
  content: string
  format: NoteFormat
  date?: string
  taskId?: string
  milestoneId?: string
  goalId?: string
  tagIds?: string[]
  typeMetadata?: Record<string, string>
  drawioContent?: string
  hasDrawio: boolean
  mindmapContent?: string
  hasMindmap: boolean
}

interface NoteEditorProps {
  value: NoteEditorValue
  onChange: (value: NoteEditorValue) => void
  placeholder?: string
  onImageUpload?: (file: File) => Promise<string>
}

export function NoteEditor({
  value,
  onChange,
  placeholder,
  onImageUpload,
}: NoteEditorProps) {
  const { getConstraints } = useNoteTypeStore()
  const typeConstraints = getConstraints(value.type)
  const isTitleRequired = typeConstraints?.titleRequired ?? true

  const handleChange = (partial: Partial<NoteEditorValue>) => {
    onChange({ ...value, ...partial })
  }

  return (
    <div className="space-y-4">
      {/* Title */}
      <div className="space-y-2">
        <Label>タイトル {isTitleRequired && '*'}</Label>
        <Input
          value={value.title || ''}
          onChange={(e) => handleChange({ title: e.target.value })}
          placeholder="タイトルを入力"
        />
      </div>

      {/* Markdown editor (always shown) */}
      <div className="space-y-2">
        <Label>内容 *</Label>
        <MarkdownEditor
          value={value.content}
          onChange={(content) => handleChange({ content })}
          placeholder={placeholder}
          onImageUpload={onImageUpload}
        />
      </div>

      {/* Draw.io editor (shown when hasDrawio is true) */}
      {value.hasDrawio && (
        <div className="space-y-2">
          <Label>draw.io 図</Label>
          <DrawioEditor
            value={value.drawioContent || ''}
            onChange={(drawioContent) => handleChange({ drawioContent })}
          />
        </div>
      )}

      {/* Mindmap editor (shown when hasMindmap is true) */}
      {value.hasMindmap && (
        <div className="space-y-2">
          <Label>マインドマップ</Label>
          <MindmapEditor
            value={value.mindmapContent || ''}
            onChange={(mindmapContent) => handleChange({ mindmapContent })}
          />
        </div>
      )}
    </div>
  )
}

// Helper component for displaying note metadata badges
export function NoteMetadataBadges({
  goalName,
  goalColor,
  milestoneName,
  taskName,
  tagNames,
}: {
  goalName?: string
  goalColor?: string
  milestoneName?: string
  taskName?: string
  tagNames?: string[]
}) {
  if (!goalName && !milestoneName && !taskName && !tagNames?.length) {
    return null
  }

  return (
    <div className="flex flex-wrap gap-1">
      {goalName && (
        <Badge
          variant="outline"
          className="text-xs"
          style={goalColor ? { borderColor: goalColor, color: goalColor } : undefined}
        >
          <Target className="h-3 w-3 mr-1" />
          {goalName}
        </Badge>
      )}
      {milestoneName && (
        <Badge variant="outline" className="text-xs">
          <Milestone className="h-3 w-3 mr-1" />
          {milestoneName}
        </Badge>
      )}
      {taskName && (
        <Badge variant="outline" className="text-xs">
          <ListTodo className="h-3 w-3 mr-1" />
          {taskName}
        </Badge>
      )}
      {tagNames?.map((name, i) => (
        <Badge key={i} variant="secondary" className="text-xs">
          {name}
        </Badge>
      ))}
    </div>
  )
}
