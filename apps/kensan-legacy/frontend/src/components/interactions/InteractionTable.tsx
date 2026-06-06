import { useState } from 'react'
import { ChevronRight, Copy, Check, ExternalLink } from 'lucide-react'
import { cn } from '@/lib/utils'
import type { Interaction } from '@/api/services/observability'
import { ConversationFlow } from './ConversationFlow'

interface InteractionTableProps {
  interactions: Interaction[]
  grafanaBaseUrl?: string
}

function OutcomeBadge({ outcome }: { outcome: string }) {
  const colors: Record<string, string> = {
    success: 'bg-emerald-100 text-emerald-800 dark:bg-emerald-900/40 dark:text-emerald-300',
    error: 'bg-red-100 text-red-800 dark:bg-red-900/40 dark:text-red-300',
    max_turns_reached: 'bg-amber-100 text-amber-800 dark:bg-amber-900/40 dark:text-amber-300',
    in_progress: 'bg-blue-100 text-blue-800 dark:bg-blue-900/40 dark:text-blue-300',
  }

  return (
    <span className={cn('px-2 py-0.5 rounded text-xs font-medium', colors[outcome] || 'bg-muted text-muted-foreground')}>
      {outcome}
    </span>
  )
}

function CopyableTraceId({ traceId, grafanaBaseUrl }: { traceId: string; grafanaBaseUrl?: string }) {
  const [copied, setCopied] = useState(false)

  const handleCopy = async (e: React.MouseEvent) => {
    e.stopPropagation()
    await navigator.clipboard.writeText(traceId)
    setCopied(true)
    setTimeout(() => setCopied(false), 1500)
  }

  const tempoUrl = grafanaBaseUrl
    ? `${grafanaBaseUrl}/explore?left=${encodeURIComponent(JSON.stringify({ datasource: 'tempo', queries: [{ queryType: 'traceql', query: traceId }] }))}`
    : undefined

  return (
    <span className="flex items-center gap-1 font-mono text-xs">
      <span className="truncate max-w-[120px]" title={traceId}>
        {traceId.substring(0, 16)}…
      </span>
      <button
        onClick={handleCopy}
        className="p-0.5 rounded hover:bg-muted transition-colors"
        title="Copy Trace ID"
      >
        {copied ? <Check className="h-3 w-3 text-emerald-500" /> : <Copy className="h-3 w-3 text-muted-foreground" />}
      </button>
      {tempoUrl && (
        <a
          href={tempoUrl}
          target="_blank"
          rel="noopener noreferrer"
          onClick={(e) => e.stopPropagation()}
          className="p-0.5 rounded hover:bg-muted transition-colors"
          title="Open in Tempo"
        >
          <ExternalLink className="h-3 w-3 text-muted-foreground" />
        </a>
      )}
    </span>
  )
}

function formatTime(date: Date): string {
  return date.toLocaleTimeString('ja-JP', { hour: '2-digit', minute: '2-digit', second: '2-digit' })
}

function formatDate(date: Date): string {
  return date.toLocaleDateString('ja-JP', { month: '2-digit', day: '2-digit' })
}

export function InteractionTable({ interactions, grafanaBaseUrl }: InteractionTableProps) {
  const [expandedId, setExpandedId] = useState<string | null>(null)

  const toggleExpand = (traceId: string) => {
    setExpandedId(expandedId === traceId ? null : traceId)
  }

  if (interactions.length === 0) {
    return (
      <div className="text-center py-12 text-muted-foreground">
        No interactions found in the selected time range
      </div>
    )
  }

  return (
    <div className="border rounded-lg overflow-hidden">
      {/* Header */}
      <div className="grid grid-cols-[28px_80px_minmax(200px,1fr)_100px_120px_110px_60px_80px_100px_100px] gap-2 px-3 py-2 bg-muted/50 text-xs font-medium text-muted-foreground border-b">
        <div></div>
        <div>Time</div>
        <div>User Input</div>
        <div>Outcome</div>
        <div>Model</div>
        <div>Context</div>
        <div>Tools</div>
        <div>Turns</div>
        <div>In Tokens</div>
        <div>Out Tokens</div>
      </div>

      {/* Rows */}
      {interactions.map((interaction) => {
        const isExpanded = expandedId === interaction.traceId
        return (
          <div key={interaction.traceId} className="border-b last:border-b-0">
            {/* Summary row */}
            <button
              onClick={() => toggleExpand(interaction.traceId)}
              className={cn(
                'w-full grid grid-cols-[28px_80px_minmax(200px,1fr)_100px_120px_110px_60px_80px_100px_100px] gap-2 px-3 py-2.5 text-sm text-left hover:bg-muted/30 transition-colors',
                isExpanded && 'bg-muted/20'
              )}
            >
              <div className="flex items-center">
                <ChevronRight
                  className={cn('h-4 w-4 text-muted-foreground transition-transform', isExpanded && 'rotate-90')}
                />
              </div>
              <div className="text-muted-foreground tabular-nums">
                <div>{formatDate(interaction.timestamp)}</div>
                <div>{formatTime(interaction.timestamp)}</div>
              </div>
              <div className="truncate pr-2" title={interaction.userMessage}>
                {interaction.userMessage || '(no message)'}
              </div>
              <div>
                <OutcomeBadge outcome={interaction.outcome} />
              </div>
              <div className="text-xs text-muted-foreground truncate" title={interaction.model}>
                {interaction.model || '-'}
              </div>
              <div className="text-xs text-muted-foreground truncate" title={interaction.contextName}>
                {interaction.contextName || '-'}
              </div>
              <div className="tabular-nums text-amber-600 dark:text-amber-400" title={interaction.toolNames.join(', ')}>
                {interaction.toolCount || '-'}
              </div>
              <div className="tabular-nums">{interaction.totalTurns}</div>
              <div className="tabular-nums text-blue-600 dark:text-blue-400">
                {interaction.totalInputTokens.toLocaleString()}
              </div>
              <div className="tabular-nums text-orange-600 dark:text-orange-400">
                {interaction.totalOutputTokens.toLocaleString()}
              </div>
            </button>

            {/* Expanded detail */}
            {isExpanded && (
              <div className="border-t bg-muted/10">
                {/* Trace ID bar */}
                <div className="px-4 py-2 border-b bg-muted/20 flex items-center gap-4 text-xs flex-wrap">
                  <span className="text-muted-foreground">Trace ID:</span>
                  <CopyableTraceId traceId={interaction.traceId} grafanaBaseUrl={grafanaBaseUrl} />
                  <span className="text-muted-foreground ml-auto flex items-center gap-3">
                    {interaction.toolCount > 0 && (
                      <span>
                        Tools: <span className="font-medium text-amber-600 dark:text-amber-400 tabular-nums">{interaction.toolCount}</span>
                        {interaction.toolDefinitionsLength > 0 && (
                          <span className="ml-1">(~{Math.round(interaction.toolDefinitionsLength * 0.5).toLocaleString()} tokens est.)</span>
                        )}
                      </span>
                    )}
                    <span>
                      Total: {(interaction.totalInputTokens + interaction.totalOutputTokens).toLocaleString()} tokens
                    </span>
                  </span>
                </div>

                {/* Conversation flow */}
                <div className="p-4">
                  <ConversationFlow events={interaction.events} />
                </div>
              </div>
            )}
          </div>
        )
      })}
    </div>
  )
}
