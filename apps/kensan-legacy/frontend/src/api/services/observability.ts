// Observability API: Lakehouse Silver via kensan-ai API for AI Interaction Explorer

import { httpClient } from '../client'
import { API_CONFIG } from '../config'

// ============================================
// Types (unchanged from Loki-based version)
// ============================================

export type AiEventType = 'agent.prompt' | 'agent.turn' | 'agent.tool_call' | 'agent.complete' | 'agent.system_prompt'

export interface AiEventBase {
  event: AiEventType
  conversation_id?: string
  timestamp: Date
  traceId: string
}

export interface AiPromptEvent extends AiEventBase {
  event: 'agent.prompt'
  model: string
  user_message: string
  context_id: string
  context_name: string
  context_version: string
  experiment_id: string
  system_prompt_length: number
  system_prompt_sections: Record<string, number>
  tool_count: number
  tool_names: string[]
  tool_definitions_length: number
}

export interface AiSystemPromptEvent extends AiEventBase {
  event: 'agent.system_prompt'
  context_id: string
  context_name: string
  context_version: string
  system_prompt: string
}

export interface AiTurnEvent extends AiEventBase {
  event: 'agent.turn'
  turn_number: number
  input_tokens: number
  output_tokens: number
  cache_creation_input_tokens: number
  cache_read_input_tokens: number
  tool_call_count: number
  response_text: string
}

export interface AiToolCallEvent extends AiEventBase {
  event: 'agent.tool_call'
  tool_name: string
  tool_input: string
  tool_output: string
  success: boolean
  error?: string
}

export interface AiCompleteEvent extends AiEventBase {
  event: 'agent.complete'
  outcome: string
  total_turns: number
  total_input_tokens: number
  total_output_tokens: number
  pending_action_count: number
}

export type AiEvent = AiPromptEvent | AiSystemPromptEvent | AiTurnEvent | AiToolCallEvent | AiCompleteEvent

/** Aggregated interaction (one agent execution = one trace) */
export interface Interaction {
  traceId: string
  timestamp: Date
  outcome: string
  model: string
  totalTurns: number
  totalInputTokens: number
  totalOutputTokens: number
  pendingActionCount: number
  userMessage: string
  contextId: string
  contextName: string
  contextVersion: string
  experimentId: string
  systemPromptLength: number
  systemPromptSections: Record<string, number>
  toolCount: number
  toolNames: string[]
  toolDefinitionsLength: number
  events: AiEvent[]
}

// ============================================
// API response types
// ============================================

interface ExplorerInteractionResponse {
  traceId: string
  timestamp: string
  outcome: string
  model: string
  totalTurns: number
  totalInputTokens: number
  totalOutputTokens: number
  pendingActionCount: number
  userMessage: string
  contextId: string
  contextName: string
  contextVersion: string
  experimentId: string
  systemPromptLength: number
  systemPromptSections: Record<string, number>
  toolCount: number
  toolNames: string[]
  toolDefinitionsLength: number
  events: Array<{
    event: AiEventType
    traceId: string
    conversation_id?: string
    timestamp: string
    [key: string]: unknown
  }>
}

// ============================================
// Public API
// ============================================

/** Fetch all events and group into Interactions by traceId */
export async function fetchInteractions(start: Date, end: Date): Promise<Interaction[]> {
  const params = new URLSearchParams({
    start_timestamp: start.toISOString(),
    end_timestamp: end.toISOString(),
  })

  const data = await httpClient.get<{ interactions: ExplorerInteractionResponse[] }>(
    API_CONFIG.baseUrls.ai,
    `/explorer/interactions?${params}`,
  )

  return (data.interactions || []).map((item): Interaction => ({
    traceId: item.traceId,
    timestamp: new Date(item.timestamp),
    outcome: item.outcome,
    model: item.model,
    totalTurns: item.totalTurns,
    totalInputTokens: item.totalInputTokens,
    totalOutputTokens: item.totalOutputTokens,
    pendingActionCount: item.pendingActionCount,
    userMessage: item.userMessage,
    contextId: item.contextId,
    contextName: item.contextName,
    contextVersion: item.contextVersion,
    experimentId: item.experimentId,
    systemPromptLength: item.systemPromptLength,
    systemPromptSections: item.systemPromptSections || {},
    toolCount: item.toolCount,
    toolNames: item.toolNames || [],
    toolDefinitionsLength: item.toolDefinitionsLength,
    events: (item.events || []).map((ev) => ({
      ...ev,
      timestamp: new Date(ev.timestamp),
    } as AiEvent)),
  }))
}

/** Fetch events for a single trace */
export async function fetchTraceEvents(traceId: string, start: Date, end: Date): Promise<AiEvent[]> {
  const interactions = await fetchInteractions(start, end)
  const match = interactions.find((i) => i.traceId === traceId)
  return match?.events || []
}
