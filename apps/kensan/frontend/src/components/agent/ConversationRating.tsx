import { useState } from 'react'
import { ThumbsDown, Minus, ThumbsUp, Star } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { useChatStore } from '@/stores/useChatStore'
import { rateConversation } from '@/api/services/agent'
import { cn } from '@/lib/utils'
import type { ChatMessage } from '@/stores/useChatStore'

const RATING_OPTIONS = [
  { label: 'イマイチ', rating: 1, Icon: ThumbsDown },
  { label: 'ふつう', rating: 3, Icon: Minus },
  { label: 'いい', rating: 4, Icon: ThumbsUp },
  { label: 'とてもいい', rating: 5, Icon: Star },
] as const

interface ConversationRatingProps {
  messages: ChatMessage[]
  isStreaming: boolean
  conversationId: string | null
}

export function ConversationRating({ messages, isStreaming, conversationId }: ConversationRatingProps) {
  const {
    conversationRating,
    isViewingHistory,
    setConversationRating,
  } = useChatStore()

  const [isSubmitting, setIsSubmitting] = useState(false)

  // Only show when: not streaming, has conversation, not rated, not viewing history,
  // and last message is from assistant
  const lastMsg = messages[messages.length - 1]
  if (
    isStreaming ||
    !conversationId ||
    conversationRating !== null ||
    isViewingHistory ||
    !lastMsg ||
    lastMsg.role !== 'assistant'
  ) {
    return null
  }

  const handleRate = async (rating: number) => {
    setIsSubmitting(true)
    try {
      await rateConversation(conversationId, rating)
      setConversationRating(rating)
    } catch {
      // Non-critical: show brief feedback but don't block UX
      setConversationRating(rating)
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <div className="flex items-center gap-1 px-4 py-2 border-t">
      <span className="text-xs text-muted-foreground mr-1">この会話はどうでしたか？</span>
      {RATING_OPTIONS.map(({ label, rating, Icon }) => (
        <Button
          key={rating}
          variant="ghost"
          size="sm"
          className={cn('h-7 px-2 text-xs gap-1', isSubmitting && 'opacity-50')}
          disabled={isSubmitting}
          onClick={() => handleRate(rating)}
          title={label}
        >
          <Icon className="h-3.5 w-3.5" />
          <span className="hidden sm:inline">{label}</span>
        </Button>
      ))}
    </div>
  )
}
