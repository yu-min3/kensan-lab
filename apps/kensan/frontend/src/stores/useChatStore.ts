import { create } from 'zustand'
import type { Conversation, ConversationMessage } from '@/api/services/agent'
import { getConversations, getConversationMessages } from '@/api/services/agent'

export interface ActionItem {
  id: string
  type: string
  description: string
  input: Record<string, unknown>
}

export type ChatSituation = 'auto' | 'review' | 'chat' | 'daily_advice'

interface ChatMessageBase {
  id: string
  role: 'user' | 'assistant'
  timestamp: Date
}

export interface TextMessage extends ChatMessageBase {
  type: 'text'
  content: string
}

export interface ToolCallMessage extends ChatMessageBase {
  type: 'tool_call'
  content: string
  toolName: string
  toolCompleted?: boolean
}

export interface ToolResultMessage extends ChatMessageBase {
  type: 'tool_result'
  content: string
  toolName: string
}

export interface ActionProposalMessage extends ChatMessageBase {
  type: 'action_proposal'
  content: string
  actions: ActionItem[]
}

export type ChatMessage = TextMessage | ToolCallMessage | ToolResultMessage | ActionProposalMessage

interface ChatState {
  isOpen: boolean
  prefilledMessage: { message: string; situation?: ChatSituation } | null
  conversationRating: number | null

  // History
  conversations: Conversation[]
  isLoadingHistory: boolean
  isViewingHistory: boolean

  toggle: () => void
  open: () => void
  close: () => void
  setConversationRating: (rating: number | null) => void
  newConversation: () => void
  sendPrefilled: (message: string, situation?: ChatSituation) => void
  clearPrefilled: () => void

  // History actions
  fetchConversations: () => Promise<void>
  loadConversation: (id: string) => Promise<ChatMessage[]>
  clearHistory: () => void
}

export const useChatStore = create<ChatState>((set) => ({
  isOpen: false,
  prefilledMessage: null,
  conversationRating: null,

  // History
  conversations: [],
  isLoadingHistory: false,
  isViewingHistory: false,

  toggle: () => set((state) => ({ isOpen: !state.isOpen })),
  open: () => set({ isOpen: true }),
  close: () => set({ isOpen: false }),

  setConversationRating: (rating) => set({ conversationRating: rating }),

  newConversation: () =>
    set({ isViewingHistory: false, conversationRating: null }),

  sendPrefilled: (message, situation) =>
    set({
      isOpen: true,
      prefilledMessage: { message, situation },
      isViewingHistory: false,
      conversationRating: null,
    }),

  clearPrefilled: () => set({ prefilledMessage: null }),

  fetchConversations: async () => {
    set({ isLoadingHistory: true })
    try {
      const result = await getConversations()
      set({ conversations: result.conversations })
    } catch (err) {
      console.error('Failed to fetch conversations:', err)
    } finally {
      set({ isLoadingHistory: false })
    }
  },

  loadConversation: async (id: string) => {
    set({ isLoadingHistory: true })
    try {
      const result = await getConversationMessages(id)
      const messages: ChatMessage[] = result.messages.map((m: ConversationMessage) => ({
        id: m.id,
        role: m.role,
        content: m.content,
        type: 'text' as const,
        timestamp: new Date(m.createdAt),
      }))
      set({ isViewingHistory: true, isLoadingHistory: false })
      return messages
    } catch (err) {
      console.error('Failed to load conversation:', err)
      set({ isLoadingHistory: false })
      return []
    }
  },

  clearHistory: () => set({ conversations: [], isViewingHistory: false }),
}))
