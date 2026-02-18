import { useState } from 'react'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'
import { ScrollArea } from '@/components/ui/scroll-area'
import { ConfirmPopover } from './ConfirmPopover'
import { useEntityMemos } from '@/hooks/useEntityMemos'
import { cn } from '@/lib/utils'
import { formatDistanceToNow } from 'date-fns'
import { ja } from 'date-fns/locale'
import {
  MessageSquarePlus,
  Pin,
  PinOff,
  Trash2,
  Edit2,
  Check,
  X,
  Loader2,
} from 'lucide-react'
import type { EntityType, EntityMemo } from '@/types'
import { handleSubmitOrCancel } from '@/lib/keyboardHandlers'

interface EntityMemoSectionProps {
  entityType: EntityType
  entityId: string
  className?: string
  maxHeight?: string
}

export function EntityMemoSection({
  entityType,
  entityId,
  className,
  maxHeight = '200px',
}: EntityMemoSectionProps) {
  const { memos, isLoading, addMemo, updateMemo, deleteMemo, togglePin } = useEntityMemos({
    entityType,
    entityId,
    enabled: !!entityId,
  })

  const [newMemoContent, setNewMemoContent] = useState('')
  const [isAdding, setIsAdding] = useState(false)
  const [editingMemoId, setEditingMemoId] = useState<string | null>(null)
  const [editingContent, setEditingContent] = useState('')

  const handleAddMemo = async () => {
    if (!newMemoContent.trim()) return
    setIsAdding(true)
    try {
      await addMemo(newMemoContent.trim())
      setNewMemoContent('')
    } finally {
      setIsAdding(false)
    }
  }

  const handleStartEdit = (memo: EntityMemo) => {
    setEditingMemoId(memo.id)
    setEditingContent(memo.content)
  }

  const handleCancelEdit = () => {
    setEditingMemoId(null)
    setEditingContent('')
  }

  const handleSaveEdit = async () => {
    if (!editingMemoId || !editingContent.trim()) return
    await updateMemo(editingMemoId, { content: editingContent.trim() })
    setEditingMemoId(null)
    setEditingContent('')
  }

  const handleDelete = async (memoId: string) => {
    await deleteMemo(memoId)
  }

  if (!entityId) {
    return null
  }

  return (
    <div className={cn('space-y-3', className)}>
      <div className="flex items-center gap-2 text-sm font-medium text-muted-foreground">
        <MessageSquarePlus className="h-4 w-4" />
        メモ
        {memos.length > 0 && (
          <span className="text-xs bg-muted px-1.5 py-0.5 rounded">{memos.length}</span>
        )}
      </div>

      {/* 新規メモ入力 */}
      <div className="space-y-2">
        <Textarea
          value={newMemoContent}
          onChange={(e) => setNewMemoContent(e.target.value)}
          placeholder="メモを追加..."
          className="min-h-[60px] text-sm resize-none"
          onKeyDown={handleSubmitOrCancel(handleAddMemo, () => setNewMemoContent(''))}
        />
        <div className="flex justify-end">
          <Button
            size="sm"
            onClick={handleAddMemo}
            disabled={!newMemoContent.trim() || isAdding}
          >
            {isAdding ? (
              <Loader2 className="h-3 w-3 animate-spin mr-1" />
            ) : (
              <MessageSquarePlus className="h-3 w-3 mr-1" />
            )}
            追加
          </Button>
        </div>
      </div>

      {/* メモ一覧 */}
      {isLoading ? (
        <div className="flex items-center justify-center py-4">
          <Loader2 className="h-4 w-4 animate-spin text-muted-foreground" />
        </div>
      ) : memos.length > 0 ? (
        <ScrollArea className="pr-2" style={{ maxHeight }}>
          <div className="space-y-2">
            {memos.map((memo) => (
              <MemoItem
                key={memo.id}
                memo={memo}
                isEditing={editingMemoId === memo.id}
                editingContent={editingContent}
                onEditingContentChange={setEditingContent}
                onStartEdit={() => handleStartEdit(memo)}
                onCancelEdit={handleCancelEdit}
                onSaveEdit={handleSaveEdit}
                onTogglePin={() => togglePin(memo.id)}
                onDelete={() => handleDelete(memo.id)}
              />
            ))}
          </div>
        </ScrollArea>
      ) : (
        <p className="text-xs text-muted-foreground text-center py-2">
          メモはありません
        </p>
      )}
    </div>
  )
}

interface MemoItemProps {
  memo: EntityMemo
  isEditing: boolean
  editingContent: string
  onEditingContentChange: (content: string) => void
  onStartEdit: () => void
  onCancelEdit: () => void
  onSaveEdit: () => void
  onTogglePin: () => void
  onDelete: () => void
}

function MemoItem({
  memo,
  isEditing,
  editingContent,
  onEditingContentChange,
  onStartEdit,
  onCancelEdit,
  onSaveEdit,
  onTogglePin,
  onDelete,
}: MemoItemProps) {
  return (
    <div
      className={cn(
        'group relative p-2 rounded-md border text-sm',
        memo.pinned ? 'bg-amber-50 border-amber-200 dark:bg-amber-950/20 dark:border-amber-800' : 'bg-muted/30'
      )}
    >
      {/* ピンアイコン（ピン留め時） */}
      {memo.pinned && (
        <Pin className="absolute -top-1 -right-1 h-3 w-3 text-amber-500 fill-amber-500" />
      )}

      {isEditing ? (
        <div className="space-y-2">
          <Textarea
            value={editingContent}
            onChange={(e) => onEditingContentChange(e.target.value)}
            onKeyDown={handleSubmitOrCancel(onSaveEdit, onCancelEdit)}
            className="min-h-[60px] text-sm resize-none"
            autoFocus
          />
          <div className="flex justify-end gap-1">
            <Button size="sm" variant="ghost" onClick={onCancelEdit}>
              <X className="h-3 w-3" />
            </Button>
            <Button size="sm" onClick={onSaveEdit}>
              <Check className="h-3 w-3" />
            </Button>
          </div>
        </div>
      ) : (
        <>
          <p className="whitespace-pre-wrap text-sm pr-16">{memo.content}</p>
          <p className="text-xs text-muted-foreground mt-1">
            {formatDistanceToNow(memo.createdAt, { addSuffix: true, locale: ja })}
          </p>

          {/* アクションボタン */}
          <div className="absolute top-1 right-1 flex gap-0.5 opacity-0 group-hover:opacity-100 transition-opacity">
            <Button
              size="sm"
              variant="ghost"
              className="h-6 w-6 p-0"
              onClick={onTogglePin}
              title={memo.pinned ? 'ピン解除' : 'ピン留め'}
            >
              {memo.pinned ? (
                <PinOff className="h-3 w-3" />
              ) : (
                <Pin className="h-3 w-3" />
              )}
            </Button>
            <Button
              size="sm"
              variant="ghost"
              className="h-6 w-6 p-0"
              onClick={onStartEdit}
              title="編集"
            >
              <Edit2 className="h-3 w-3" />
            </Button>
            <ConfirmPopover
              message="このメモを削除しますか？"
              confirmLabel="削除"
              onConfirm={onDelete}
              variant="destructive"
            >
              <Button
                size="sm"
                variant="ghost"
                className="h-6 w-6 p-0 text-destructive hover:text-destructive"
                title="削除"
              >
                <Trash2 className="h-3 w-3" />
              </Button>
            </ConfirmPopover>
          </div>
        </>
      )}
    </div>
  )
}
