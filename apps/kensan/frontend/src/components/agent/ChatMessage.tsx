import { Bot, User, Loader2, Check } from 'lucide-react'
import { cn } from '@/lib/utils'
import type { ChatMessage as ChatMessageType } from '@/stores/useChatStore'
import { MarkdownContent } from './MarkdownContent'

interface ChatMessageProps {
  message: ChatMessageType
}

export function ChatMessage({ message }: ChatMessageProps) {
  if (message.type === 'tool_call') {
    return (
      <div className="flex items-center gap-2 px-4 py-1 text-xs text-muted-foreground">
        {message.toolCompleted ? (
          <Check className="h-3 w-3" />
        ) : (
          <Loader2 className="h-3 w-3 animate-spin" />
        )}
        <span>{getToolLabel(message.toolName || '', message.toolCompleted)}</span>
      </div>
    )
  }

  if (message.type === 'tool_result') {
    return null
  }

  const isUser = message.role === 'user'

  return (
    <div className={cn('flex gap-2 px-4 py-2', isUser ? 'flex-row-reverse' : 'flex-row')}>
      <div
        className={cn(
          'flex h-7 w-7 shrink-0 items-center justify-center rounded-full text-xs',
          isUser ? 'bg-primary text-primary-foreground' : 'bg-muted text-muted-foreground'
        )}
      >
        {isUser ? <User className="h-4 w-4" /> : <Bot className="h-4 w-4" />}
      </div>
      <div
        className={cn(
          'rounded-lg text-sm max-w-[85%]',
          isUser ? 'bg-primary text-primary-foreground whitespace-pre-wrap px-3 py-2' : 'bg-muted px-3.5 py-2.5'
        )}
      >
        {isUser ? message.content : <MarkdownContent content={message.content} />}
      </div>
    </div>
  )
}

function getToolLabel(toolName: string, completed?: boolean): string {
  const activeLabels: Record<string, string> = {
    get_tasks: 'タスクを確認中',
    get_goals_and_milestones: '目標を確認中',
    get_time_blocks: 'タイムブロックを確認中',
    get_time_entries: '実績を確認中',
    get_memos: 'メモを確認中',
    get_notes: 'ノートを確認中',
    create_task: 'タスクを作成中',
    create_time_block: 'タイムブロックを作成中',
    create_memo: 'メモを作成中',
  }

  const completedLabels: Record<string, string> = {
    get_tasks: 'タスクを確認しました',
    get_goals_and_milestones: '目標を確認しました',
    get_time_blocks: 'タイムブロックを確認しました',
    get_time_entries: '実績を確認しました',
    get_memos: 'メモを確認しました',
    get_notes: 'ノートを確認しました',
    create_task: 'タスクを作成しました',
    create_time_block: 'タイムブロックを作成しました',
    create_memo: 'メモを作成しました',
  }

  if (completed) {
    return completedLabels[toolName] || `${toolName} を実行しました`
  }
  return activeLabels[toolName] || `${toolName} を実行中...`
}
