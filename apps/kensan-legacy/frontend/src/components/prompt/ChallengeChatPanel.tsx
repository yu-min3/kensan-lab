import { useEffect, useRef, forwardRef, useImperativeHandle } from 'react'
import { Loader2 } from 'lucide-react'
import { ChatMessage } from '@/components/agent/ChatMessage'
import { ActionProposal } from '@/components/agent/ActionProposal'
import { ChatInput } from '@/components/agent/ChatInput'
import { useChatStream } from '@/hooks/useChatStream'

export interface ChallengeChatPanelHandle {
  sendMessage: (text: string) => void
  getIsStreaming: () => boolean
}

interface ChallengeChatPanelProps {
  label: string
  contextId: string
  versionNumber?: number
  onStreamEnd?: () => void
}

export const ChallengeChatPanel = forwardRef<ChallengeChatPanelHandle, ChallengeChatPanelProps>(
  function ChallengeChatPanel({ label, contextId, versionNumber, onStreamEnd }, ref) {
    const { messages, isStreaming, pendingActions, sendMessage, abort } = useChatStream({
      contextId,
      versionNumber,
      onStreamEnd,
    })

    const messagesEndRef = useRef<HTMLDivElement>(null)

    useEffect(() => {
      messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' })
    }, [messages])

    useImperativeHandle(ref, () => ({
      sendMessage: (text: string) => sendMessage(text),
      getIsStreaming: () => isStreaming,
    }), [sendMessage, isStreaming])

    // Cleanup abort on unmount
    useEffect(() => {
      return () => { abort() }
    }, [abort])

    return (
      <div className="flex flex-col border rounded-lg overflow-hidden h-full">
        <div className="flex items-center gap-2 px-3 py-2 border-b bg-muted/30">
          <span className="text-xs font-semibold">{label}</span>
          {isStreaming && <Loader2 className="h-3 w-3 animate-spin text-muted-foreground" />}
        </div>
        <div className="flex-1 overflow-y-auto py-2 min-h-0">
          {messages.map(msg => {
            if (msg.type === 'action_proposal' && msg.actions) {
              return (
                <ActionProposal
                  key={msg.id}
                  actions={msg.actions}
                  readOnly
                  disabled={isStreaming || !pendingActions}
                />
              )
            }
            return <ChatMessage key={msg.id} message={msg} />
          })}
          {isStreaming && messages[messages.length - 1]?.role !== 'assistant' && (
            <div className="flex items-center gap-2 px-3 py-2 text-xs text-muted-foreground">
              <Loader2 className="h-3 w-3 animate-spin" />
              <span>考え中...</span>
            </div>
          )}
          <div ref={messagesEndRef} />
        </div>
        <ChatInput
          onSend={(text) => sendMessage(text)}
          disabled={isStreaming}
          placeholder="このパネルにメッセージ..."
        />
      </div>
    )
  },
)
