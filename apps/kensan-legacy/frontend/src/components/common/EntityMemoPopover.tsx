import { useState } from 'react'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'
import { ScrollArea } from '@/components/ui/scroll-area'
import { ConfirmPopover } from './ConfirmPopover'
import { useEntityMemos } from '@/hooks/useEntityMemos'
import { cn } from '@/lib/utils'
import { formatDistanceToNow } from 'date-fns'
import { ja } from 'date-fns/locale'
import {
  MessageSquareText,
  MessageSquarePlus,
  Pin,
  PinOff,
  Trash2,
  Edit2,
  Check,
  X,
  Loader2,
  Send,
} from 'lucide-react'
import type { EntityType, EntityMemo } from '@/types'
import { handleSubmitOrCancel } from '@/lib/keyboardHandlers'

interface EntityMemoPopoverProps {
  entityType: EntityType
  entityId: string
  className?: string
}

export function EntityMemoPopover({ entityType, entityId, className }: EntityMemoPopoverProps) {
  const [open, setOpen] = useState(false)
  const { memos, isLoading, addMemo, updateMemo, deleteMemo, togglePin } = useEntityMemos({
    entityType,
    entityId,
    enabled: open,
  })

  const [newContent, setNewContent] = useState('')
  const [isAdding, setIsAdding] = useState(false)
  const [editingMemoId, setEditingMemoId] = useState<string | null>(null)
  const [editingContent, setEditingContent] = useState('')

  const pinnedMemos = memos.filter(m => m.pinned)
  const normalMemos = memos.filter(m => !m.pinned)
  const hasPinned = pinnedMemos.length > 0

  const handleAdd = async () => {
    if (!newContent.trim()) return
    setIsAdding(true)
    try {
      await addMemo(newContent.trim())
      setNewContent('')
    } finally {
      setIsAdding(false)
    }
  }

  const handleStartEdit = (memo: EntityMemo) => {
    setEditingMemoId(memo.id)
    setEditingContent(memo.content)
  }

  const handleSaveEdit = async () => {
    if (!editingMemoId || !editingContent.trim()) return
    await updateMemo(editingMemoId, { content: editingContent.trim() })
    setEditingMemoId(null)
    setEditingContent('')
  }

  const handleCancelEdit = () => {
    setEditingMemoId(null)
    setEditingContent('')
  }

  const renderMemoItem = (memo: EntityMemo) => (
    <div
      key={memo.id}
      className={cn(
        'group/memo relative p-2 text-sm',
        memo.pinned && 'bg-amber-50 dark:bg-amber-950/20'
      )}
    >
      {editingMemoId === memo.id ? (
        <div className="space-y-1.5">
          <Textarea
            value={editingContent}
            onChange={(e) => setEditingContent(e.target.value)}
            onKeyDown={handleSubmitOrCancel(handleSaveEdit, handleCancelEdit)}
            className="min-h-[50px] text-sm resize-none"
            autoFocus
          />
          <div className="flex justify-end gap-1">
            <Button size="sm" variant="ghost" className="h-6 w-6 p-0" onClick={handleCancelEdit}>
              <X className="h-3 w-3" />
            </Button>
            <Button size="sm" variant="ghost" className="h-6 w-6 p-0" onClick={handleSaveEdit}>
              <Check className="h-3 w-3" />
            </Button>
          </div>
        </div>
      ) : (
        <>
          <p className="whitespace-pre-wrap text-sm pr-14 break-words">{memo.content}</p>
          <p className="text-[10px] text-muted-foreground mt-0.5">
            {formatDistanceToNow(memo.createdAt, { addSuffix: true, locale: ja })}
          </p>
          <div className="absolute top-1.5 right-1.5 flex gap-0.5 opacity-0 group-hover/memo:opacity-100 transition-opacity">
            <Button size="sm" variant="ghost" className="h-5 w-5 p-0" onClick={() => togglePin(memo.id)} title={memo.pinned ? 'ピン解除' : 'ピン留め'}>
              {memo.pinned ? <PinOff className="h-3 w-3" /> : <Pin className="h-3 w-3" />}
            </Button>
            <Button size="sm" variant="ghost" className="h-5 w-5 p-0" onClick={() => handleStartEdit(memo)} title="編集">
              <Edit2 className="h-3 w-3" />
            </Button>
            <ConfirmPopover message="このメモを削除しますか？" confirmLabel="削除" onConfirm={() => deleteMemo(memo.id)} variant="destructive">
              <Button size="sm" variant="ghost" className="h-5 w-5 p-0 text-destructive hover:text-destructive" title="削除">
                <Trash2 className="h-3 w-3" />
              </Button>
            </ConfirmPopover>
          </div>
        </>
      )}
    </div>
  )

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <button
          className={cn(
            'inline-flex items-center gap-0.5 rounded p-0.5 transition-opacity',
            memos.length > 0 ? 'opacity-100' : 'opacity-0 group-hover:opacity-100',
            className
          )}
          onClick={(e) => e.stopPropagation()}
        >
          {memos.length > 0 ? (
            <>
              <MessageSquareText className="h-3.5 w-3.5 text-amber-500" />
              <span className="text-[10px] text-amber-600 font-medium">{memos.length}</span>
              {hasPinned && <span className="w-1 h-1 rounded-full bg-amber-500 -mt-2" />}
            </>
          ) : (
            <MessageSquarePlus className="h-3.5 w-3.5 text-muted-foreground" />
          )}
        </button>
      </PopoverTrigger>
      <PopoverContent className="w-80 p-0" align="start" side="bottom">
        <div className="px-3 py-2 border-b text-xs font-medium text-muted-foreground flex items-center gap-1.5">
          <MessageSquareText className="h-3.5 w-3.5" />
          メモ ({memos.length})
        </div>

        {isLoading ? (
          <div className="flex items-center justify-center py-6">
            <Loader2 className="h-4 w-4 animate-spin text-muted-foreground" />
          </div>
        ) : memos.length > 0 ? (
          <ScrollArea className="max-h-[250px]">
            {pinnedMemos.length > 0 && (
              <div className="border-b">
                {pinnedMemos.map(renderMemoItem)}
              </div>
            )}
            <div className="divide-y">
              {normalMemos.map(renderMemoItem)}
            </div>
          </ScrollArea>
        ) : (
          <p className="text-xs text-muted-foreground text-center py-4">メモはありません</p>
        )}

        <div className="border-t p-2">
          <Textarea
            value={newContent}
            onChange={(e) => setNewContent(e.target.value)}
            onKeyDown={handleSubmitOrCancel(handleAdd, () => setNewContent(''))}
            placeholder="メモを追加... (Ctrl+Enter)"
            className="min-h-[50px] text-sm resize-none"
          />
          <div className="flex justify-end mt-1.5">
            <Button
              size="sm"
              onClick={handleAdd}
              disabled={!newContent.trim() || isAdding}
              className="h-7 gap-1 text-xs"
            >
              {isAdding ? <Loader2 className="h-3 w-3 animate-spin" /> : <Send className="h-3 w-3" />}
              追加
            </Button>
          </div>
        </div>
      </PopoverContent>
    </Popover>
  )
}
