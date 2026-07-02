import { useState, useMemo } from 'react'
import { cn } from '@/lib/utils'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible'
import { ChevronRight, Eye, Pencil, AlertTriangle } from 'lucide-react'
import type { ToolMetadata } from '@/api/services/prompts'

interface ToolSelectorProps {
  tools: ToolMetadata[]
  selectedTools: string[]
  onChange: (tools: string[]) => void
}

const CATEGORY_LABELS: Record<string, string> = {
  db: 'データベース',
  memory: 'メモリ',
  search: '検索',
  review: 'レビュー',
  analytics: 'アナリティクス',
  pattern: 'パターン',
  web: 'Web',
  other: 'その他',
}

const CATEGORY_ORDER = ['db', 'memory', 'search', 'review', 'analytics', 'pattern', 'web', 'other']

// Sub-groups only for db category
const DB_SUBGROUPS: { label: string; tools: string[] }[] = [
  {
    label: 'Goal / Milestone',
    tools: [
      'get_goals_and_milestones',
      'create_goal',
      'update_goal',
      'delete_goal',
      'create_milestone',
      'update_milestone',
      'delete_milestone',
    ],
  },
  {
    label: 'Task',
    tools: ['get_tasks', 'create_task', 'update_task', 'delete_task'],
  },
  {
    label: 'TimeBlock',
    tools: ['get_time_blocks', 'create_time_block', 'update_time_block', 'delete_time_block'],
  },
  {
    label: 'TimeEntry',
    tools: ['get_time_entries'],
  },
  {
    label: 'Memo',
    tools: ['get_memos', 'create_memo'],
  },
  {
    label: 'Note',
    tools: ['get_notes', 'create_note', 'update_note'],
  },
]

function ToolRow({
  tool,
  checked,
  onToggle,
}: {
  tool: ToolMetadata
  checked: boolean
  onToggle: () => void
}) {
  return (
    <label className="flex items-center gap-2 rounded px-2 py-1 hover:bg-muted/50 cursor-pointer">
      <Checkbox checked={checked} onCheckedChange={onToggle} />
      <span className="font-mono text-xs">{tool.name}</span>
      {tool.readonly ? (
        <Eye className="h-3 w-3 shrink-0 text-muted-foreground" />
      ) : (
        <Pencil className="h-3 w-3 shrink-0 text-orange-500" />
      )}
      <span className="truncate text-xs text-muted-foreground">{tool.description}</span>
    </label>
  )
}

function GroupCheckbox({
  label,
  selectedCount,
  totalCount,
  onToggle,
}: {
  label: string
  selectedCount: number
  totalCount: number
  onToggle: () => void
}) {
  const allSelected = selectedCount === totalCount
  const someSelected = selectedCount > 0 && !allSelected

  return (
    <label className="flex items-center gap-2 cursor-pointer">
      <div className="relative">
        <Checkbox
          checked={allSelected}
          onCheckedChange={onToggle}
          className={someSelected ? 'opacity-60' : ''}
        />
      </div>
      <span className="text-xs font-medium text-muted-foreground">{label}</span>
      <span className="text-xs text-muted-foreground">
        {selectedCount}/{totalCount}
      </span>
    </label>
  )
}

function SubGroup({
  label,
  toolNames,
  toolMap,
  selectedTools,
  onToggle,
  onToggleGroup,
}: {
  label: string
  toolNames: string[]
  toolMap: Map<string, ToolMetadata>
  selectedTools: Set<string>
  onToggle: (name: string) => void
  onToggleGroup: (names: string[]) => void
}) {
  const tools = toolNames.map((n) => toolMap.get(n)).filter(Boolean) as ToolMetadata[]
  if (tools.length === 0) return null

  const selectedCount = tools.filter((t) => selectedTools.has(t.name)).length

  return (
    <div className="ml-2 border-l border-border pl-3">
      <GroupCheckbox
        label={label}
        selectedCount={selectedCount}
        totalCount={tools.length}
        onToggle={() => onToggleGroup(tools.map((t) => t.name))}
      />
      <div className="mt-0.5">
        {tools.map((t) => (
          <ToolRow
            key={t.name}
            tool={t}
            checked={selectedTools.has(t.name)}
            onToggle={() => onToggle(t.name)}
          />
        ))}
      </div>
    </div>
  )
}

function CategorySection({
  categoryKey,
  tools,
  selectedTools,
  onToggle,
  onToggleGroup,
  defaultOpen,
}: {
  categoryKey: string
  tools: ToolMetadata[]
  selectedTools: Set<string>
  onToggle: (name: string) => void
  onToggleGroup: (names: string[]) => void
  defaultOpen: boolean
}) {
  const [open, setOpen] = useState(defaultOpen)
  const selectedCount = tools.filter((t) => selectedTools.has(t.name)).length
  const label = CATEGORY_LABELS[categoryKey] || categoryKey
  const toolMap = new Map(tools.map((t) => [t.name, t]))
  const hasSubGroups = categoryKey === 'db'

  return (
    <Collapsible open={open} onOpenChange={setOpen}>
      <CollapsibleTrigger className="flex w-full items-center gap-2 rounded-md px-2 py-1.5 hover:bg-muted/50">
        <ChevronRight
          className={cn('h-3.5 w-3.5 shrink-0 transition-transform', open && 'rotate-90')}
        />
        <span className="text-sm font-medium">{label}</span>
        <span className="ml-auto text-xs text-muted-foreground">
          {selectedCount}/{tools.length}
        </span>
        <div
          onClick={(e) => {
            e.stopPropagation()
          }}
        >
          <GroupCheckbox
            label=""
            selectedCount={selectedCount}
            totalCount={tools.length}
            onToggle={() => onToggleGroup(tools.map((t) => t.name))}
          />
        </div>
      </CollapsibleTrigger>
      <CollapsibleContent className="pl-5">
        {hasSubGroups ? (
          <div className="space-y-2 py-1">
            {DB_SUBGROUPS.map((sg) => (
              <SubGroup
                key={sg.label}
                label={sg.label}
                toolNames={sg.tools}
                toolMap={toolMap}
                selectedTools={selectedTools}
                onToggle={onToggle}
                onToggleGroup={onToggleGroup}
              />
            ))}
          </div>
        ) : (
          <div className="py-1">
            {tools.map((t) => (
              <ToolRow
                key={t.name}
                tool={t}
                checked={selectedTools.has(t.name)}
                onToggle={() => onToggle(t.name)}
              />
            ))}
          </div>
        )}
      </CollapsibleContent>
    </Collapsible>
  )
}

export function ToolSelector({ tools, selectedTools, onChange }: ToolSelectorProps) {
  const selectedSet = useMemo(() => new Set(selectedTools), [selectedTools])

  // Group tools by category
  const grouped = useMemo(() => {
    const map = new Map<string, ToolMetadata[]>()
    for (const t of tools) {
      const cat = t.category || 'other'
      if (!map.has(cat)) map.set(cat, [])
      map.get(cat)!.push(t)
    }
    return map
  }, [tools])

  // Find unknown tools (in selectedTools but not in metadata)
  const knownNames = useMemo(() => new Set(tools.map((t) => t.name)), [tools])
  const unknownTools = useMemo(
    () => selectedTools.filter((n) => !knownNames.has(n)),
    [selectedTools, knownNames],
  )

  const handleToggle = (name: string) => {
    if (selectedSet.has(name)) {
      onChange(selectedTools.filter((n) => n !== name))
    } else {
      onChange([...selectedTools, name])
    }
  }

  const handleToggleGroup = (names: string[]) => {
    const allSelected = names.every((n) => selectedSet.has(n))
    if (allSelected) {
      // Deselect all in group
      const removeSet = new Set(names)
      onChange(selectedTools.filter((n) => !removeSet.has(n)))
    } else {
      // Select all in group
      const toAdd = names.filter((n) => !selectedSet.has(n))
      onChange([...selectedTools, ...toAdd])
    }
  }

  const sortedCategories = CATEGORY_ORDER.filter((c) => grouped.has(c))

  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between">
        <span className="text-sm font-medium">Allowed Tools</span>
        <span className="text-xs text-muted-foreground">
          {selectedTools.length}/{tools.length} 選択中
        </span>
      </div>
      <p className="text-xs text-muted-foreground">
        会話中にAIが呼び出せるアクション
      </p>
      <div className="rounded-md border">
        {sortedCategories.map((cat) => (
          <CategorySection
            key={cat}
            categoryKey={cat}
            tools={grouped.get(cat)!}
            selectedTools={selectedSet}
            onToggle={handleToggle}
            onToggleGroup={handleToggleGroup}
            defaultOpen={grouped.get(cat)!.some((t) => selectedSet.has(t.name))}
          />
        ))}

        {/* Unknown tools warning */}
        {unknownTools.length > 0 && (
          <div className="border-t px-3 py-2">
            <div className="flex items-center gap-1.5 text-xs text-destructive">
              <AlertTriangle className="h-3.5 w-3.5" />
              <span>不明なツール ({unknownTools.length})</span>
            </div>
            <div className="mt-1 flex flex-wrap gap-1">
              {unknownTools.map((name) => (
                <label
                  key={name}
                  className="flex items-center gap-1.5 rounded bg-destructive/10 px-2 py-0.5 text-xs cursor-pointer"
                >
                  <Checkbox
                    checked={true}
                    onCheckedChange={() => handleToggle(name)}
                  />
                  <span className="font-mono">{name}</span>
                </label>
              ))}
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
