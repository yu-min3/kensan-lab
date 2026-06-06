import { useState, useRef, useCallback } from 'react'
import { streamAgentChat } from '@/api/services/agent'
import type { AgentStreamEvent, AgentStreamRequest } from '@/api/services/agent'
import type { ChatMessage, ActionItem, ChatSituation } from '@/stores/useChatStore'

export interface UseChatStreamOptions {
  contextId?: string
  versionNumber?: number
  situation?: ChatSituation
  onPendingActions?: (actions: ActionItem[] | null) => void
  onConversationId?: (id: string) => void
  onStreamEnd?: () => void
}

export interface UseChatStreamReturn {
  messages: ChatMessage[]
  isStreaming: boolean
  conversationId: string | null
  pendingActions: ActionItem[] | null
  sendMessage: (text: string, situation?: AgentStreamRequest['situation']) => Promise<void>
  processSSEEvent: (event: AgentStreamEvent) => void
  setMessages: React.Dispatch<React.SetStateAction<ChatMessage[]>>
  setConversationId: (id: string | null) => void
  setPendingActions: (actions: ActionItem[] | null) => void
  setStreaming: (streaming: boolean) => void
  reset: () => void
  abort: () => void
}

export function useChatStream(options: UseChatStreamOptions = {}): UseChatStreamReturn {
  const { contextId, versionNumber, situation: defaultSituation = 'auto', onPendingActions, onConversationId, onStreamEnd } = options

  const [messages, setMessages] = useState<ChatMessage[]>([])
  const [conversationId, setConversationIdState] = useState<string | null>(null)
  const [isStreaming, setIsStreaming] = useState(false)
  const [pendingActions, setPendingActionsState] = useState<ActionItem[] | null>(null)
  const abortControllerRef = useRef<AbortController | null>(null)

  // Use refs to access latest state in callbacks without re-creating them
  const messagesRef = useRef(messages)
  messagesRef.current = messages

  const conversationIdRef = useRef(conversationId)
  conversationIdRef.current = conversationId

  const setConversationId = useCallback((id: string | null) => {
    setConversationIdState(id)
    conversationIdRef.current = id
  }, [])

  const setPendingActions = useCallback((actions: ActionItem[] | null) => {
    setPendingActionsState(actions)
    onPendingActions?.(actions)
  }, [onPendingActions])

  const addMessage = useCallback((message: ChatMessage) => {
    setMessages(prev => [...prev, message])
  }, [])

  const processSSEEvent = useCallback((event: AgentStreamEvent) => {
    switch (event.type) {
      case 'text': {
        setMessages(prev => {
          const last = prev[prev.length - 1]
          if (last && last.role === 'assistant' && last.type === 'text') {
            return [
              ...prev.slice(0, -1),
              { ...last, content: last.content + (event.data.content as string) },
            ]
          }
          return [
            ...prev,
            {
              id: crypto.randomUUID(),
              role: 'assistant' as const,
              content: event.data.content as string,
              type: 'text' as const,
              timestamp: new Date(),
            },
          ]
        })
        break
      }

      case 'tool_call':
        addMessage({
          id: event.data.id as string,
          role: 'assistant',
          content: '',
          type: 'tool_call',
          toolName: event.data.name as string,
          timestamp: new Date(),
        })
        break

      case 'tool_result': {
        setMessages(prev =>
          prev.map(m =>
            m.type === 'tool_call' && m.id === event.data.id
              ? { ...m, toolCompleted: true }
              : m
          )
        )

        if (event.data.error) {
          addMessage({
            id: crypto.randomUUID(),
            role: 'assistant',
            content: `⚠ ${event.data.name || 'アクション'}の実行に失敗しました: ${event.data.error}`,
            type: 'text',
            timestamp: new Date(),
          })
        }
        break
      }

      case 'action_proposal': {
        const rawActions = event.data.actions as Array<Record<string, unknown>>
        const mappedActions: ActionItem[] = rawActions.map(a => ({
          id: a.id as string,
          type: (a.tool_name as string) || (a.type as string) || '',
          description: (a.description as string) || '',
          input: (a.input as Record<string, unknown>) || {},
        }))
        setPendingActions(mappedActions)
        addMessage({
          id: crypto.randomUUID(),
          role: 'assistant',
          content: '',
          type: 'action_proposal',
          actions: mappedActions,
          timestamp: new Date(),
        })
        break
      }

      case 'done':
        if (event.data.conversation_id) {
          const id = event.data.conversation_id as string
          setConversationId(id)
          onConversationId?.(id)
        }
        break
    }
  }, [addMessage, setPendingActions, setConversationId, onConversationId])

  const sendMessage = useCallback(
    async (text: string, situation?: AgentStreamRequest['situation']) => {
      addMessage({
        id: crypto.randomUUID(),
        role: 'user',
        content: text,
        type: 'text',
        timestamp: new Date(),
      })

      setIsStreaming(true)
      abortControllerRef.current = new AbortController()

      try {
        const stream = streamAgentChat(
          {
            message: text,
            conversation_id: conversationIdRef.current,
            situation: situation ?? defaultSituation,
            context_id: contextId,
            version_number: versionNumber,
          },
          abortControllerRef.current.signal,
        )

        for await (const event of stream) {
          processSSEEvent(event)
        }
      } catch (err) {
        if ((err as Error).name !== 'AbortError') {
          const errorMessage = (err as Error).message?.includes('タイムアウト')
            ? 'AIサービスからの応答がタイムアウトしました。もう一度お試しください。'
            : 'エラーが発生しました。もう一度お試しください。'
          addMessage({
            id: crypto.randomUUID(),
            role: 'assistant',
            content: errorMessage,
            type: 'text',
            timestamp: new Date(),
          })
        }
      } finally {
        setIsStreaming(false)
        onStreamEnd?.()
      }
    },
    [addMessage, processSSEEvent, defaultSituation, contextId, versionNumber, onStreamEnd],
  )

  const reset = useCallback(() => {
    abortControllerRef.current?.abort()
    setMessages([])
    setConversationId(null)
    setIsStreaming(false)
    setPendingActions(null)
  }, [setConversationId, setPendingActions])

  const abort = useCallback(() => {
    abortControllerRef.current?.abort()
  }, [])

  return {
    messages,
    isStreaming,
    conversationId,
    pendingActions,
    sendMessage,
    processSSEEvent,
    setMessages,
    setConversationId,
    setPendingActions,
    setStreaming: setIsStreaming,
    reset,
    abort,
  }
}
