import { useState, useEffect, useRef, useCallback } from 'react'
import { X, Plus, History, ArrowLeft, Loader2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { useChatStore } from '@/stores/useChatStore'
import { ChatMessage } from './ChatMessage'
import { ChatInput } from './ChatInput'
import { ActionProposal } from './ActionProposal'
import { WelcomeMessage } from './WelcomeMessage'
import { ConversationRating } from './ConversationRating'
import { streamApproveActions, rejectActions } from '@/api/services/agent'
import { usePanelResize } from '@/hooks/usePanelResize'
import { useChatStream } from '@/hooks/useChatStream'

export function ChatPanel() {
  const {
    isOpen,
    conversations,
    isLoadingHistory,
    isViewingHistory,
    conversationRating,
    close,
    newConversation,
    fetchConversations,
    loadConversation,
  } = useChatStore()

  const stream = useChatStream()
  const {
    messages,
    isStreaming,
    conversationId,
    pendingActions,
    sendMessage,
    processSSEEvent,
    setMessages,
    setConversationId,
    setPendingActions,
    setStreaming,
    reset,
  } = stream

  const [showHistory, setShowHistory] = useState(false)
  const messagesEndRef = useRef<HTMLDivElement>(null)

  const { panelWidth, handleResizeStart } = usePanelResize()

  // Global keyboard shortcut: Ctrl+Shift+A to toggle panel
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.ctrlKey && e.shiftKey && e.key === 'A') {
        e.preventDefault()
        useChatStore.getState().toggle()
      }
    }
    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [])

  // Auto-scroll to bottom
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages])

  const handleSend = useCallback(
    (text: string) => sendMessage(text, 'auto'),
    [sendMessage]
  )

  // Handle prefilled messages (e.g., from A02_AIReview)
  useEffect(() => {
    const prefilled = useChatStore.getState().prefilledMessage
    if (prefilled && isOpen && !isStreaming) {
      useChatStore.getState().clearPrefilled()
      // Reset stream state for new prefilled conversation
      reset()
      sendMessage(prefilled.message, prefilled.situation || 'auto')
    }
  }, [isOpen, isStreaming, sendMessage, reset])

  const handleApprove = useCallback(
    async (actionIds: string[]) => {
      if (!conversationId) return

      setPendingActions(null)
      setStreaming(true)

      try {
        const sseStream = streamApproveActions({
          conversation_id: conversationId,
          action_ids: actionIds,
        })

        for await (const event of sseStream) {
          processSSEEvent(event)
        }
      } catch {
        setMessages(prev => [
          ...prev,
          {
            id: crypto.randomUUID(),
            role: 'assistant' as const,
            content: 'アクションの実行中にエラーが発生しました。',
            type: 'text' as const,
            timestamp: new Date(),
          },
        ])
      } finally {
        setStreaming(false)
      }
    },
    [conversationId, setPendingActions, setStreaming, processSSEEvent, setMessages]
  )

  const handleReject = useCallback(() => {
    setPendingActions(null)
    setMessages(prev => [
      ...prev,
      {
        id: crypto.randomUUID(),
        role: 'assistant' as const,
        content: '提案をキャンセルしました。他にお手伝いできることはありますか？',
        type: 'text' as const,
        timestamp: new Date(),
      },
    ])
    if (conversationId) {
      rejectActions(conversationId).catch(() => {
        // Non-critical: rejection recording failure doesn't affect UX
      })
    }
  }, [conversationId, setPendingActions, setMessages])

  const handleShowHistory = useCallback(() => {
    setShowHistory(true)
    fetchConversations()
  }, [fetchConversations])

  const handleSelectConversation = useCallback(
    async (id: string) => {
      const loadedMessages = await loadConversation(id)
      setMessages(loadedMessages)
      setConversationId(id)
      setPendingActions(null)
      setShowHistory(false)
    },
    [loadConversation, setMessages, setConversationId, setPendingActions]
  )

  const handleBackFromHistory = useCallback(() => {
    setShowHistory(false)
  }, [])

  const handleNewFromHistory = useCallback(() => {
    newConversation()
    reset()
    setShowHistory(false)
  }, [newConversation, reset])

  if (!isOpen) return null

  return (
    <div role="complementary" aria-label="AI Chat" className="border-l flex flex-col bg-background h-full relative" style={{ width: panelWidth }}>
      {/* Resize handle */}
      <div
        className="absolute left-0 top-0 bottom-0 w-1 cursor-col-resize hover:bg-primary/20 active:bg-primary/30 z-10"
        onMouseDown={handleResizeStart}
      />
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-3 border-b">
        {showHistory ? (
          <>
            <div className="flex items-center gap-2">
              <Button
                variant="ghost"
                size="icon"
                className="h-7 w-7"
                onClick={handleBackFromHistory}
                aria-label="Back"
              >
                <ArrowLeft className="h-4 w-4" />
              </Button>
              <h2 className="text-sm font-semibold">履歴</h2>
            </div>
            <Button variant="ghost" size="icon" className="h-7 w-7" onClick={close} aria-label="Close">
              <X className="h-4 w-4" />
            </Button>
          </>
        ) : (
          <>
            <h2 className="text-sm font-semibold">
              AI Assistant
              {isViewingHistory && (
                <span className="ml-2 text-xs text-muted-foreground font-normal">(閲覧中)</span>
              )}
            </h2>
            <div className="flex items-center gap-1">
              <Button
                variant="ghost"
                size="icon"
                className="h-7 w-7"
                onClick={handleShowHistory}
                title="履歴"
                aria-label="History"
              >
                <History className="h-4 w-4" />
              </Button>
              <Button
                variant="ghost"
                size="icon"
                className="h-7 w-7"
                onClick={handleNewFromHistory}
                title="新しい会話"
                aria-label="New conversation"
              >
                <Plus className="h-4 w-4" />
              </Button>
              <Button variant="ghost" size="icon" className="h-7 w-7" onClick={close} aria-label="Close">
                <X className="h-4 w-4" />
              </Button>
            </div>
          </>
        )}
      </div>

      {showHistory ? (
        /* History List */
        <div className="flex-1 overflow-y-auto">
          {isLoadingHistory ? (
            <div className="flex items-center justify-center h-32">
              <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
            </div>
          ) : conversations.length === 0 ? (
            <div className="flex items-center justify-center h-32 text-muted-foreground text-sm">
              履歴がありません
            </div>
          ) : (
            <div className="divide-y">
              {conversations.map((conv) => (
                <button
                  key={conv.id}
                  className="w-full text-left px-4 py-3 hover:bg-muted/50 transition-colors"
                  onClick={() => handleSelectConversation(conv.id)}
                >
                  <p className="text-sm truncate">{conv.lastMessage}</p>
                  <div className="flex items-center gap-2 mt-1">
                    <span className="text-xs text-muted-foreground">
                      {new Date(conv.lastMessageAt).toLocaleDateString('ja-JP')}
                    </span>
                    <span className="text-xs text-muted-foreground">
                      {conv.messageCount}件
                    </span>
                  </div>
                </button>
              ))}
            </div>
          )}
        </div>
      ) : (
        <>
          {/* Messages */}
          <div role="log" aria-live="polite" className="flex-1 overflow-y-auto py-2">
            {messages.length === 0 && <WelcomeMessage onSend={handleSend} />}
            {messages.map((msg) => {
              return msg.type === 'action_proposal' && msg.actions ? (
                <ActionProposal
                  key={msg.id}
                  actions={msg.actions}
                  onApprove={handleApprove}
                  onReject={handleReject}
                  disabled={isStreaming || !pendingActions}
                />
              ) : (
                <ChatMessage key={msg.id} message={msg} />
              )
            })}
            {isStreaming && !pendingActions && (
              <div className="flex items-center gap-2 px-4 py-3 text-sm text-muted-foreground">
                <Loader2 className="h-4 w-4 animate-spin" />
                <span>考え中...</span>
              </div>
            )}
            <div ref={messagesEndRef} />
          </div>

          {/* Conversation rating */}
          {conversationRating ? (
            <div className="flex items-center px-4 py-2 border-t text-xs text-muted-foreground">
              評価しました — ありがとうございます
            </div>
          ) : (
            <ConversationRating
              messages={messages}
              isStreaming={isStreaming}
              conversationId={conversationId}
            />
          )}

          {/* Input - disabled when viewing history */}
          <ChatInput
            onSend={handleSend}
            disabled={isStreaming || isViewingHistory}
            placeholder={isViewingHistory ? '閲覧モード（新しい会話を開始してください）' : undefined}
          />
        </>
      )}
    </div>
  )
}
