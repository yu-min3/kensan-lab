import { useState } from 'react'
import {
  User,
  Bot,
  Wrench,
  CheckCircle2,
  XCircle,
  ChevronDown,
  ChevronRight,
  FileText,
} from 'lucide-react'
import { cn } from '@/lib/utils'
import type {
  AiEvent,
  AiPromptEvent,
  AiSystemPromptEvent,
  AiTurnEvent,
  AiToolCallEvent,
  AiCompleteEvent,
} from '@/api/services/observability'

interface ConversationFlowProps {
  events: AiEvent[]
}

function formatTime(date: Date): string {
  return date.toLocaleTimeString('ja-JP', {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  } as Intl.DateTimeFormatOptions)
}

function ExpandableContent({
  content,
  label,
  maxPreviewLength = 200,
}: {
  content: string
  label: string
  maxPreviewLength?: number
}) {
  const [expanded, setExpanded] = useState(false)
  const needsExpand = content.length > maxPreviewLength

  return (
    <div>
      <button
        onClick={() => needsExpand && setExpanded(!expanded)}
        className={cn(
          'text-left text-sm leading-relaxed',
          needsExpand && 'cursor-pointer'
        )}
      >
        {expanded ? (
          <div className="whitespace-pre-wrap break-words">{content}</div>
        ) : (
          <span>
            {content.substring(0, maxPreviewLength)}
            {needsExpand && (
              <span className="text-muted-foreground ml-1">
                … ({content.length} chars)
              </span>
            )}
          </span>
        )}
      </button>
      {needsExpand && (
        <button
          onClick={() => setExpanded(!expanded)}
          className="flex items-center gap-1 text-xs text-muted-foreground mt-1 hover:text-foreground transition-colors"
        >
          {expanded ? (
            <>
              <ChevronDown className="h-3 w-3" /> Hide {label}
            </>
          ) : (
            <>
              <ChevronRight className="h-3 w-3" /> Show full {label}
            </>
          )}
        </button>
      )}
    </div>
  )
}

const CATEGORIES = [
  {
    id: 'instructions',
    label: 'ルール・指示',
    bar: 'bg-blue-400 dark:bg-blue-600',
    dot: 'bg-blue-400 dark:bg-blue-600',
    keywords: ['ルール', '指示', '出力', '思考', 'プロセス', 'モード', '手順', 'トーン', '役割', 'スタイル', 'ガイド', 'フォーマット', 'ツール連携'],
  },
  {
    id: 'context',
    label: 'コンテキスト',
    bar: 'bg-emerald-400 dark:bg-emerald-600',
    dot: 'bg-emerald-400 dark:bg-emerald-600',
    keywords: ['タスク', 'スケジュール', '予定', '計画', '実績', '目標', '進捗', 'サマリー', 'やりとり', 'データ', '日時', '定期'],
  },
  {
    id: 'profile',
    label: 'ユーザープロファイル',
    bar: 'bg-amber-400 dark:bg-amber-600',
    dot: 'bg-amber-400 dark:bg-amber-600',
    keywords: ['ユーザー', '行動', '感情', '関心', '特性'],
  },
  {
    id: 'other',
    label: 'その他',
    bar: 'bg-slate-400 dark:bg-slate-600',
    dot: 'bg-slate-400 dark:bg-slate-600',
    keywords: [],
  },
] as const

type CategoryId = (typeof CATEGORIES)[number]['id']

function categorize(sectionName: string): CategoryId {
  const name = sectionName.toLowerCase()
  for (const cat of CATEGORIES) {
    if (cat.id === 'other') continue
    if (cat.keywords.some((kw) => name.includes(kw.toLowerCase()))) return cat.id
  }
  return 'other'
}

/** Horizontal bar showing section sizes proportionally */
function SectionBreakdown({ sections }: { sections: Record<string, number> }) {
  const [expanded, setExpanded] = useState<Set<string>>(new Set())
  const entries = Object.entries(sections).sort(([, a], [, b]) => b - a)
  const total = entries.reduce((sum, [, v]) => sum + v, 0)
  if (total === 0) return null

  // Group entries by category
  const grouped = new Map<CategoryId, { total: number; items: [string, number][] }>()
  for (const cat of CATEGORIES) {
    grouped.set(cat.id, { total: 0, items: [] })
  }
  for (const [name, size] of entries) {
    const catId = categorize(name)
    const group = grouped.get(catId)!
    group.total += size
    group.items.push([name, size])
  }

  // Sorted categories by total descending, omit empty
  const sortedCategories = CATEGORIES
    .filter((cat) => grouped.get(cat.id)!.total > 0)
    .sort((a, b) => grouped.get(b.id)!.total - grouped.get(a.id)!.total)

  const catMap = Object.fromEntries(CATEGORIES.map((c) => [c.id, c]))

  const toggleExpanded = (id: string) => {
    setExpanded((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  return (
    <div className="space-y-1.5">
      {/* Stacked bar — category level */}
      <div className="flex h-3 rounded-full overflow-hidden bg-muted">
        {sortedCategories.map((cat) => {
          const pct = (grouped.get(cat.id)!.total / total) * 100
          if (pct < 1) return null
          return (
            <div
              key={cat.id}
              className={cn('h-full', cat.bar)}
              style={{ width: `${pct}%` }}
              title={`${cat.label}: ${grouped.get(cat.id)!.total.toLocaleString()} chars (${pct.toFixed(1)}%)`}
            />
          )
        })}
      </div>
      {/* Legend — collapsible categories */}
      <div className="space-y-0.5">
        {sortedCategories.map((cat) => {
          const group = grouped.get(cat.id)!
          const pct = (group.total / total) * 100
          const isOpen = expanded.has(cat.id)
          return (
            <div key={cat.id}>
              <button
                type="button"
                onClick={() => toggleExpanded(cat.id)}
                className="flex items-center gap-1 text-xs w-full hover:bg-muted/50 rounded px-0.5 -mx-0.5"
              >
                {isOpen ? (
                  <ChevronDown className="w-3 h-3 text-muted-foreground shrink-0" />
                ) : (
                  <ChevronRight className="w-3 h-3 text-muted-foreground shrink-0" />
                )}
                <div className={cn('w-2 h-2 rounded-sm shrink-0', catMap[cat.id].dot)} />
                <span className="text-muted-foreground">{cat.label}</span>
                <span className="tabular-nums font-medium">{group.total.toLocaleString()}</span>
                <span className="text-muted-foreground">({pct.toFixed(0)}%)</span>
              </button>
              {isOpen && (
                <div className="ml-5 space-y-0.5 mt-0.5">
                  {group.items.map(([name, size]) => {
                    const itemPct = (size / total) * 100
                    return (
                      <div key={name} className="flex items-center gap-1 text-xs text-muted-foreground">
                        <span>{name}</span>
                        <span className="tabular-nums font-medium text-foreground">{size.toLocaleString()}</span>
                        <span>({itemPct.toFixed(0)}%)</span>
                      </div>
                    )
                  })}
                </div>
              )}
            </div>
          )
        })}
      </div>
    </div>
  )
}

function ToolDefinitionsSummary({ promptEvent }: { promptEvent: AiPromptEvent }) {
  const [showTools, setShowTools] = useState(false)
  const toolCount = promptEvent.tool_count
  const toolDefLength = promptEvent.tool_definitions_length
  const toolNames = promptEvent.tool_names

  if (toolCount === 0) return null

  const estTokens = Math.round(toolDefLength * 0.5)

  return (
    <div className="flex gap-3">
      <div className="flex flex-col items-center">
        <div className="rounded-full p-1.5 bg-amber-200 dark:bg-amber-800 text-amber-700 dark:text-amber-300">
          <Wrench className="h-4 w-4" />
        </div>
        <div className="flex-1 w-px bg-border mt-2" />
      </div>
      <div className="flex-1 pb-4">
        <div className="flex items-center gap-2 mb-1.5">
          <span className="text-xs font-medium text-amber-700 dark:text-amber-300">TOOL DEFINITIONS</span>
          <span className="text-xs font-medium tabular-nums">{toolCount} tools</span>
        </div>

        <div className="flex items-center gap-3 text-xs text-muted-foreground mb-2">
          <span>
            <span className="font-medium text-foreground tabular-nums">
              {toolDefLength.toLocaleString()}
            </span>
            {' '}chars
          </span>
          <span>
            ~<span className="font-medium text-foreground tabular-nums">
              {estTokens.toLocaleString()}
            </span>
            {' '}tokens (est.)
          </span>
        </div>

        <button
          onClick={() => setShowTools(!showTools)}
          className="flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground transition-colors"
        >
          {showTools ? (
            <><ChevronDown className="h-3 w-3" /> Hide tool list</>
          ) : (
            <><ChevronRight className="h-3 w-3" /> Show tool list</>
          )}
        </button>
        {showTools && toolNames.length > 0 && (
          <div className="mt-2 flex flex-wrap gap-1.5">
            {toolNames.map((name) => (
              <code key={name} className="text-xs bg-muted px-1.5 py-0.5 rounded font-mono">
                {name}
              </code>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}

function SystemPromptEntry({ promptEvent, systemPromptEvent }: { promptEvent: AiPromptEvent; systemPromptEvent?: AiSystemPromptEvent }) {
  const [showFull, setShowFull] = useState(false)
  const hasSections = Object.keys(promptEvent.system_prompt_sections).length > 0
  const hasFullText = !!systemPromptEvent?.system_prompt

  return (
    <div className="flex gap-3">
      <div className="flex flex-col items-center">
        <div className="rounded-full p-1.5 bg-slate-200 dark:bg-slate-700 text-slate-600 dark:text-slate-300">
          <FileText className="h-4 w-4" />
        </div>
        <div className="flex-1 w-px bg-border mt-2" />
      </div>
      <div className="flex-1 pb-4">
        <div className="flex items-center gap-2 mb-1.5 flex-wrap">
          <span className="text-xs font-medium text-slate-600 dark:text-slate-300">SYSTEM CONTEXT</span>
          {promptEvent.context_name && (
            <code className="text-xs bg-muted px-1.5 py-0.5 rounded font-mono">
              {promptEvent.context_name}
            </code>
          )}
          {promptEvent.context_version && (
            <span className="text-xs text-muted-foreground">
              v{promptEvent.context_version}
            </span>
          )}
          {promptEvent.experiment_id && (
            <span className="text-xs px-1.5 py-0.5 rounded bg-violet-100 text-violet-800 dark:bg-violet-900/40 dark:text-violet-300">
              experiment
            </span>
          )}
        </div>

        {/* Size summary */}
        <div className="flex items-center gap-3 text-xs text-muted-foreground mb-2">
          <span>
            <span className="font-medium text-foreground tabular-nums">
              {promptEvent.system_prompt_length.toLocaleString()}
            </span>
            {' '}chars
          </span>
          <span>
            ~<span className="font-medium text-foreground tabular-nums">
              {Math.round(promptEvent.system_prompt_length * 0.5).toLocaleString()}
            </span>
            {' '}tokens (est.)
          </span>
          {promptEvent.context_id && (
            <span className="font-mono truncate max-w-[160px]" title={promptEvent.context_id}>
              id: {promptEvent.context_id.substring(0, 8)}…
            </span>
          )}
        </div>

        {/* Section breakdown bar */}
        {hasSections && (
          <div className="mb-2">
            <SectionBreakdown sections={promptEvent.system_prompt_sections} />
          </div>
        )}

        {/* Full text toggle */}
        {hasFullText && (
          <>
            <button
              onClick={() => setShowFull(!showFull)}
              className="flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground transition-colors"
            >
              {showFull ? (
                <><ChevronDown className="h-3 w-3" /> Hide full prompt</>
              ) : (
                <><ChevronRight className="h-3 w-3" /> Show full prompt</>
              )}
            </button>
            {showFull && (
              <pre className="mt-2 text-xs bg-muted/50 rounded p-3 overflow-x-auto whitespace-pre-wrap break-words">
                {systemPromptEvent.system_prompt}
              </pre>
            )}
          </>
        )}
      </div>
    </div>
  )
}

function PromptEntry({ event }: { event: AiPromptEvent }) {
  return (
    <div className="flex gap-3">
      <div className="flex flex-col items-center">
        <div className="rounded-full p-1.5 bg-blue-100 dark:bg-blue-900/40 text-blue-700 dark:text-blue-300">
          <User className="h-4 w-4" />
        </div>
        <div className="flex-1 w-px bg-border mt-2" />
      </div>
      <div className="flex-1 pb-4">
        <div className="flex items-center gap-2 mb-1">
          <span className="text-xs font-medium text-blue-700 dark:text-blue-300">USER</span>
          <span className="text-xs text-muted-foreground">{formatTime(event.timestamp)}</span>
          <span className="text-xs text-muted-foreground bg-muted px-1.5 py-0.5 rounded">
            {event.model}
          </span>
        </div>
        <ExpandableContent content={event.user_message} label="message" />
      </div>
    </div>
  )
}

function TurnEntry({ event }: { event: AiTurnEvent }) {
  return (
    <div className="flex gap-3">
      <div className="flex flex-col items-center">
        <div className="rounded-full p-1.5 bg-emerald-100 dark:bg-emerald-900/40 text-emerald-700 dark:text-emerald-300">
          <Bot className="h-4 w-4" />
        </div>
        <div className="flex-1 w-px bg-border mt-2" />
      </div>
      <div className="flex-1 pb-4">
        <div className="flex items-center gap-2 mb-1 flex-wrap">
          <span className="text-xs font-medium text-emerald-700 dark:text-emerald-300">
            AGENT (Turn {event.turn_number})
          </span>
          <span className="text-xs text-muted-foreground">{formatTime(event.timestamp)}</span>
          <span className="text-xs tabular-nums text-blue-600 dark:text-blue-400">
            in={event.input_tokens.toLocaleString()}
          </span>
          <span className="text-xs tabular-nums text-orange-600 dark:text-orange-400">
            out={event.output_tokens.toLocaleString()}
          </span>
          {event.cache_read_input_tokens > 0 && (
            <span className="text-xs tabular-nums text-green-600 dark:text-green-400" title="Tokens read from cache">
              cache={event.cache_read_input_tokens.toLocaleString()}
            </span>
          )}
          {event.tool_call_count > 0 && (
            <span className="text-xs text-muted-foreground">
              tools={event.tool_call_count}
            </span>
          )}
        </div>
        <ExpandableContent content={event.response_text} label="response" />
      </div>
    </div>
  )
}

function ToolCallEntry({ event }: { event: AiToolCallEvent }) {
  const [showDetail, setShowDetail] = useState(false)

  return (
    <div className="flex gap-3">
      <div className="flex flex-col items-center">
        <div className={cn(
          'rounded-full p-1.5',
          event.success
            ? 'bg-amber-100 dark:bg-amber-900/40 text-amber-700 dark:text-amber-300'
            : 'bg-red-100 dark:bg-red-900/40 text-red-700 dark:text-red-300'
        )}>
          <Wrench className="h-4 w-4" />
        </div>
        <div className="flex-1 w-px bg-border mt-2" />
      </div>
      <div className="flex-1 pb-3">
        <div className="flex items-center gap-2 mb-1">
          <span className={cn(
            'text-xs font-medium',
            event.success
              ? 'text-amber-700 dark:text-amber-300'
              : 'text-red-700 dark:text-red-300'
          )}>
            TOOL
          </span>
          <span className="text-xs text-muted-foreground">{formatTime(event.timestamp)}</span>
          <code className="text-xs bg-muted px-1.5 py-0.5 rounded font-mono">
            {event.tool_name}
          </code>
          {event.success ? (
            <CheckCircle2 className="h-3.5 w-3.5 text-emerald-500" />
          ) : (
            <XCircle className="h-3.5 w-3.5 text-red-500" />
          )}
        </div>
        <button
          onClick={() => setShowDetail(!showDetail)}
          className="flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground transition-colors"
        >
          {showDetail ? (
            <ChevronDown className="h-3 w-3" />
          ) : (
            <ChevronRight className="h-3 w-3" />
          )}
          {showDetail ? 'Hide' : 'Show'} input/output
        </button>
        {showDetail && (
          <div className="mt-2 space-y-2">
            <div>
              <div className="text-xs text-muted-foreground mb-0.5">Input:</div>
              <pre className="text-xs bg-muted/50 rounded p-2 overflow-x-auto whitespace-pre-wrap break-all">
                {event.tool_input}
              </pre>
            </div>
            <div>
              <div className="text-xs text-muted-foreground mb-0.5">Output:</div>
              <pre className="text-xs bg-muted/50 rounded p-2 overflow-x-auto whitespace-pre-wrap break-all">
                {event.tool_output || (event.error ? `Error: ${event.error}` : '(empty)')}
              </pre>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}

function CompleteEntry({ event }: { event: AiCompleteEvent }) {
  return (
    <div className="flex gap-3">
      <div className="flex flex-col items-center">
        <div className={cn(
          'rounded-full p-1.5',
          event.outcome === 'success'
            ? 'bg-purple-100 dark:bg-purple-900/40 text-purple-700 dark:text-purple-300'
            : 'bg-red-100 dark:bg-red-900/40 text-red-700 dark:text-red-300'
        )}>
          {event.outcome === 'success' ? (
            <CheckCircle2 className="h-4 w-4" />
          ) : (
            <XCircle className="h-4 w-4" />
          )}
        </div>
      </div>
      <div className="flex-1">
        <div className="flex items-center gap-2 flex-wrap">
          <span className="text-xs font-medium text-purple-700 dark:text-purple-300">COMPLETE</span>
          <span className="text-xs text-muted-foreground">{formatTime(event.timestamp)}</span>
          <span className={cn(
            'text-xs px-1.5 py-0.5 rounded',
            event.outcome === 'success'
              ? 'bg-emerald-100 text-emerald-800 dark:bg-emerald-900/40 dark:text-emerald-300'
              : 'bg-red-100 text-red-800 dark:bg-red-900/40 dark:text-red-300'
          )}>
            {event.outcome}
          </span>
          <span className="text-xs text-muted-foreground tabular-nums">
            {event.total_turns} turns |
            {' '}{event.total_input_tokens.toLocaleString()} + {event.total_output_tokens.toLocaleString()} tokens
          </span>
        </div>
      </div>
    </div>
  )
}

export function ConversationFlow({ events }: ConversationFlowProps) {
  if (events.length === 0) {
    return <div className="text-sm text-muted-foreground">No events found</div>
  }

  // Find prompt and system_prompt events for the SystemPromptEntry
  const promptEvent = events.find((e): e is AiPromptEvent => e.event === 'agent.prompt')
  const systemPromptEvent = events.find((e): e is AiSystemPromptEvent => e.event === 'agent.system_prompt')
  const hasContextInfo = promptEvent && (promptEvent.system_prompt_length > 0 || promptEvent.context_name)

  return (
    <div className="space-y-0">
      {/* System Context entry (before the conversation) */}
      {hasContextInfo && (
        <SystemPromptEntry promptEvent={promptEvent} systemPromptEvent={systemPromptEvent} />
      )}

      {/* Tool Definitions entry */}
      {promptEvent && promptEvent.tool_count > 0 && (
        <ToolDefinitionsSummary promptEvent={promptEvent} />
      )}

      {events.map((event, i) => {
        switch (event.event) {
          case 'agent.prompt':
            return <PromptEntry key={i} event={event} />
          case 'agent.system_prompt':
            return null // rendered above as SystemPromptEntry
          case 'agent.turn':
            return <TurnEntry key={i} event={event} />
          case 'agent.tool_call':
            return <ToolCallEntry key={i} event={event} />
          case 'agent.complete':
            return <CompleteEntry key={i} event={event} />
          default:
            return null
        }
      })}
    </div>
  )
}
