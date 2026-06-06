// Agent API Service - SSE streaming for AI agent interactions
import { API_CONFIG } from '../config'
import { httpClient } from '../client'
import { injectTraceHeaders } from '../telemetry'

// Types
export interface AgentStreamEvent {
  type: 'text' | 'tool_call' | 'tool_result' | 'action_proposal' | 'done' | 'error' | 'keepalive'
  data: Record<string, unknown>
}

export interface AgentStreamRequest {
  message: string
  conversation_id?: string | null
  situation?: 'auto' | 'review' | 'chat' | 'daily_advice'
  context?: Record<string, string> | null
  context_id?: string | null
  version_number?: number | null
}

export interface AgentApproveRequest {
  conversation_id: string
  action_ids: string[]
}

/**
 * Parse SSE-formatted text into AgentStreamEvent array.
 * SSE format: `event: <type>\ndata: <json>\n\n`
 */
export function parseSSEEvents(text: string): AgentStreamEvent[] {
  const events: AgentStreamEvent[] = []
  const blocks = text.split('\n\n')

  for (const block of blocks) {
    const trimmed = block.trim()
    if (!trimmed) continue

    let eventType: string | undefined
    let eventData: string | undefined

    const lines = trimmed.split('\n')
    for (const line of lines) {
      if (line.startsWith('event: ')) {
        eventType = line.slice('event: '.length).trim()
      } else if (line.startsWith('data: ')) {
        eventData = line.slice('data: '.length)
      }
    }

    if (eventType && eventData !== undefined) {
      try {
        const parsed = JSON.parse(eventData) as Record<string, unknown>
        events.push({
          type: eventType as AgentStreamEvent['type'],
          data: parsed,
        })
      } catch {
        // Skip malformed JSON data
      }
    }
  }

  return events
}

/**
 * Stream agent chat responses via SSE.
 * Uses raw fetch (not httpClient) to access ReadableStream on the response body.
 */
export async function* streamAgentChat(
  request: AgentStreamRequest,
  signal?: AbortSignal
): AsyncGenerator<AgentStreamEvent> {
  const url = `${API_CONFIG.baseUrls.ai}/api/v1/agent/stream`
  const authToken = httpClient.getAuthToken()

  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
  }
  injectTraceHeaders(headers)
  if (authToken) {
    headers['Authorization'] = `Bearer ${authToken}`
  }

  // Create a timeout signal (60s) combined with the caller's signal
  const timeoutController = new AbortController()
  const timeoutId = setTimeout(() => timeoutController.abort(), 60_000)
  const combinedSignal = signal
    ? AbortSignal.any([signal, timeoutController.signal])
    : timeoutController.signal

  let response: Response
  try {
    response = await fetch(url, {
      method: 'POST',
      headers,
      body: JSON.stringify(request),
      signal: combinedSignal,
    })
  } catch (err) {
    clearTimeout(timeoutId)
    if (timeoutController.signal.aborted) {
      throw new Error('リクエストがタイムアウトしました')
    }
    throw err
  }

  clearTimeout(timeoutId)

  if (!response.ok) {
    const errorText = await response.text().catch(() => 'Unknown error')
    throw new Error(`Agent stream request failed [${response.status}]: ${errorText}`)
  }

  if (!response.body) {
    throw new Error('Response body is not available for streaming')
  }

  const reader = response.body.getReader()
  const decoder = new TextDecoder()
  let buffer = ''

  // Per-chunk timeout: reset on each chunk received (30s between chunks)
  const chunkTimeoutMs = 120_000
  let chunkTimeoutId: ReturnType<typeof setTimeout> | undefined

  try {
    while (true) {
      const readPromise = reader.read()
      const timeoutPromise = new Promise<never>((_, reject) => {
        chunkTimeoutId = setTimeout(
          () => reject(new Error('ストリームがタイムアウトしました')),
          chunkTimeoutMs
        )
      })

      const { done, value } = await Promise.race([readPromise, timeoutPromise])
      clearTimeout(chunkTimeoutId)

      if (done) {
        // Process any remaining data in the buffer
        if (buffer.trim()) {
          const events = parseSSEEvents(buffer)
          for (const event of events) {
            yield event
          }
        }
        break
      }

      buffer += decoder.decode(value, { stream: true })

      // Split on double newline boundaries, keeping incomplete parts in buffer
      const parts = buffer.split('\n\n')
      // The last part may be incomplete, keep it in the buffer
      buffer = parts.pop() ?? ''

      // Process all complete blocks
      const completePart = parts.join('\n\n')
      if (completePart.trim()) {
        const events = parseSSEEvents(completePart)
        for (const event of events) {
          yield event
        }
      }
    }
  } finally {
    clearTimeout(chunkTimeoutId)
    reader.releaseLock()
  }
}

/**
 * Stream approved agent action results via SSE.
 * Uses the /agent/approve endpoint which returns SSE events for each action execution.
 */
export async function* streamApproveActions(
  request: AgentApproveRequest,
  signal?: AbortSignal
): AsyncGenerator<AgentStreamEvent> {
  const url = `${API_CONFIG.baseUrls.ai}/api/v1/agent/approve`
  const authToken = httpClient.getAuthToken()

  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
  }
  injectTraceHeaders(headers)
  if (authToken) {
    headers['Authorization'] = `Bearer ${authToken}`
  }

  const timeoutController = new AbortController()
  const timeoutId = setTimeout(() => timeoutController.abort(), 60_000)
  const combinedSignal = signal
    ? AbortSignal.any([signal, timeoutController.signal])
    : timeoutController.signal

  let response: Response
  try {
    response = await fetch(url, {
      method: 'POST',
      headers,
      body: JSON.stringify(request),
      signal: combinedSignal,
    })
  } catch (err) {
    clearTimeout(timeoutId)
    if (timeoutController.signal.aborted) {
      throw new Error('リクエストがタイムアウトしました')
    }
    throw err
  }

  clearTimeout(timeoutId)

  if (!response.ok) {
    const errorText = await response.text().catch(() => 'Unknown error')
    throw new Error(`Agent approve request failed [${response.status}]: ${errorText}`)
  }

  if (!response.body) {
    throw new Error('Response body is not available for streaming')
  }

  const reader = response.body.getReader()
  const decoder = new TextDecoder()
  let buffer = ''

  const chunkTimeoutMs = 120_000
  let chunkTimeoutId: ReturnType<typeof setTimeout> | undefined

  try {
    while (true) {
      const readPromise = reader.read()
      const timeoutPromise = new Promise<never>((_, reject) => {
        chunkTimeoutId = setTimeout(
          () => reject(new Error('ストリームがタイムアウトしました')),
          chunkTimeoutMs
        )
      })

      const { done, value } = await Promise.race([readPromise, timeoutPromise])
      clearTimeout(chunkTimeoutId)

      if (done) {
        if (buffer.trim()) {
          const events = parseSSEEvents(buffer)
          for (const event of events) {
            yield event
          }
        }
        break
      }

      buffer += decoder.decode(value, { stream: true })

      const parts = buffer.split('\n\n')
      buffer = parts.pop() ?? ''

      const completePart = parts.join('\n\n')
      if (completePart.trim()) {
        const events = parseSSEEvents(completePart)
        for (const event of events) {
          yield event
        }
      }
    }
  } finally {
    clearTimeout(chunkTimeoutId)
    reader.releaseLock()
  }
}

/**
 * Notify the backend that the user rejected proposed actions.
 * Records the rejection in conversation history so the agent
 * doesn't re-propose the same actions.
 */
export async function rejectActions(conversationId: string): Promise<void> {
  const url = `${API_CONFIG.baseUrls.ai}/api/v1/agent/reject`
  const authToken = httpClient.getAuthToken()

  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
  }
  if (authToken) {
    headers['Authorization'] = `Bearer ${authToken}`
  }

  await fetch(url, {
    method: 'POST',
    headers,
    body: JSON.stringify({ conversation_id: conversationId }),
  })
}

/**
 * Rate a conversation.
 */
export async function rateConversation(
  conversationId: string,
  rating: number,
): Promise<{ success: boolean; message: string }> {
  return httpClient.post<{ success: boolean; message: string }>(
    API_CONFIG.baseUrls.ai,
    `/conversations/${conversationId}/rate`,
    { rating },
  )
}

// Conversations history API

export interface Conversation {
  id: string
  lastMessage: string
  lastMessageAt: string
  messageCount: number
}

export interface ConversationMessage {
  id: string
  role: 'user' | 'assistant'
  content: string
  situation: string
  toolCalls: Array<Record<string, unknown>>
  createdAt: string
}

/**
 * Get list of past conversations.
 */
export async function getConversations(
  limit = 20,
  offset = 0
): Promise<{ conversations: Conversation[] }> {
  return httpClient.get<{ conversations: Conversation[] }>(
    API_CONFIG.baseUrls.ai,
    `/conversations?limit=${limit}&offset=${offset}`
  )
}

/**
 * Get messages for a specific conversation.
 */
export async function getConversationMessages(
  conversationId: string
): Promise<{ messages: ConversationMessage[] }> {
  return httpClient.get<{ messages: ConversationMessage[] }>(
    API_CONFIG.baseUrls.ai,
    `/conversations/${conversationId}`
  )
}
