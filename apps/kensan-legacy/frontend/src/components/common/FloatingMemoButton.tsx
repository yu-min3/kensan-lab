import { useState, useRef, useEffect } from 'react'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'
import { useMemoStore } from '@/stores/useMemoStore'
import { Lightbulb, X, Send, Loader2, Archive, Trash2 } from 'lucide-react'
import { cn } from '@/lib/utils'
import { toast } from 'sonner'
import { formatTime } from '@/lib/dateFormat'
import { ConfirmPopover } from './ConfirmPopover'
import { handleSubmitOrCancel } from '@/lib/keyboardHandlers'

export function FloatingMemoButton() {
  const [isOpen, setIsOpen] = useState(false)
  const [content, setContent] = useState('')
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [editingId, setEditingId] = useState<string | null>(null)
  const [editContent, setEditContent] = useState('')
  const [isSaving, setIsSaving] = useState(false)
  const textareaRef = useRef<HTMLTextAreaElement>(null)
  const editTextareaRef = useRef<HTMLTextAreaElement>(null)
  const { addMemo, updateMemo, fetchMemos, archiveMemo, deleteMemo, getActiveMemos } = useMemoStore()

  // パネルを開いたときにメモを取得
  useEffect(() => {
    if (isOpen) {
      fetchMemos()
    }
  }, [isOpen, fetchMemos])

  // パネルを開いたときにテキストエリアにフォーカス
  useEffect(() => {
    if (isOpen && textareaRef.current) {
      textareaRef.current.focus()
    }
  }, [isOpen])

  const handleSubmit = async () => {
    if (!content.trim() || isSubmitting) return

    setIsSubmitting(true)
    const result = await addMemo(content.trim())
    setIsSubmitting(false)

    if (result) {
      toast.success('メモを保存しました', { duration: 2000 })
      setContent('')
    } else {
      toast.error('メモの保存に失敗しました', { duration: 4000 })
    }
  }

  const handleNewMemoEscape = () => {
    if (content.trim()) {
      setContent('')
    } else {
      setIsOpen(false)
    }
  }

  const handleArchive = async (id: string) => {
    await archiveMemo(id)
    toast.success('アーカイブしました', { duration: 2000 })
  }

  const handleDelete = async (id: string) => {
    await deleteMemo(id)
    toast.success('削除しました', { duration: 2000 })
  }

  const handleStartEdit = (memo: { id: string; content: string }) => {
    setEditingId(memo.id)
    setEditContent(memo.content)
    setTimeout(() => {
      editTextareaRef.current?.focus()
      editTextareaRef.current?.setSelectionRange(
        editTextareaRef.current.value.length,
        editTextareaRef.current.value.length
      )
    }, 0)
  }

  const handleSaveEdit = async () => {
    if (editingId === null) return
    const trimmed = editContent.trim()
    const original = activeMemos.find((m) => m.id === editingId)?.content
    if (trimmed === original || trimmed === '') {
      setEditingId(null)
      setEditContent('')
      return
    }
    setIsSaving(true)
    const success = await updateMemo(editingId, trimmed)
    setIsSaving(false)
    if (success) {
      toast.success('保存しました', { duration: 1500 })
    } else {
      toast.error('保存に失敗しました', { duration: 3000 })
    }
    setEditingId(null)
    setEditContent('')
  }

  const cancelEdit = () => {
    setEditingId(null)
    setEditContent('')
  }

  const activeMemos = getActiveMemos()

  return (
    <>
      {/* Backdrop */}
      {isOpen && (
        <div
          className="fixed inset-0 bg-black/20 z-40"
          onClick={() => setIsOpen(false)}
        />
      )}

      {/* Floating Panel */}
      <div
        className={cn(
          'fixed bottom-20 right-6 z-50 transition-all duration-200 ease-out',
          isOpen
            ? 'opacity-100 translate-y-0'
            : 'opacity-0 translate-y-4 pointer-events-none'
        )}
      >
        <div className="bg-background border rounded-lg shadow-lg w-96 max-h-[70vh] flex flex-col">
          {/* Header */}
          <div className="flex items-center justify-between p-3 border-b">
            <div className="flex items-center gap-2 text-sm font-medium">
              <Lightbulb className="h-4 w-4 text-yellow-500" />
              メモ
            </div>
            <Button
              variant="ghost"
              size="sm"
              className="h-7 w-7 p-0"
              onClick={() => setIsOpen(false)}
            >
              <X className="h-4 w-4" />
            </Button>
          </div>

          {/* Memo List */}
          <div className="flex-1 overflow-y-auto max-h-[40vh]">
            {activeMemos.length === 0 ? (
              <div className="p-6 text-center">
                <Lightbulb className="h-8 w-8 text-muted-foreground/50 mx-auto mb-2" />
                <p className="text-sm text-muted-foreground">
                  メモがありません
                </p>
                <p className="text-xs text-muted-foreground mt-1">
                  下の入力欄からメモを追加
                </p>
              </div>
            ) : (
              <div className="divide-y">
                {activeMemos.map((memo) => (
                  <div
                    key={memo.id}
                    className="p-3 hover:bg-muted/50 transition-colors group"
                  >
                    {editingId === memo.id ? (
                      <div className="relative">
                        <Textarea
                          ref={editTextareaRef}
                          value={editContent}
                          onChange={(e) => setEditContent(e.target.value)}
                          onBlur={handleSaveEdit}
                          onKeyDown={handleSubmitOrCancel(handleSaveEdit, cancelEdit)}
                          className="min-h-[60px] resize-none text-sm"
                          disabled={isSaving}
                        />
                        {isSaving && (
                          <div className="absolute top-2 right-2">
                            <Loader2 className="h-4 w-4 animate-spin text-muted-foreground" />
                          </div>
                        )}
                      </div>
                    ) : (
                      <p
                        className="text-sm whitespace-pre-wrap break-words cursor-text hover:bg-muted/50 rounded px-1 -mx-1"
                        onClick={() => handleStartEdit(memo)}
                      >
                        {memo.content}
                      </p>
                    )}
                    <div className="flex items-center justify-between mt-2">
                      <span className="text-xs text-muted-foreground">
                        {formatTime(memo.createdAt)}
                      </span>
                      <div className={cn(
                        "flex gap-1 transition-opacity",
                        editingId === memo.id ? "opacity-0" : "opacity-0 group-hover:opacity-100"
                      )}>
                        <Button
                          variant="ghost"
                          size="sm"
                          className="h-6 w-6 p-0"
                          onClick={() => handleArchive(memo.id)}
                          title="アーカイブ"
                        >
                          <Archive className="h-3.5 w-3.5" />
                        </Button>
                        <ConfirmPopover
                          message="このメモを削除しますか？"
                          confirmLabel="削除"
                          onConfirm={() => handleDelete(memo.id)}
                          variant="destructive"
                        >
                          <Button
                            variant="ghost"
                            size="sm"
                            className="h-6 w-6 p-0 text-destructive hover:text-destructive"
                            title="削除"
                          >
                            <Trash2 className="h-3.5 w-3.5" />
                          </Button>
                        </ConfirmPopover>
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>

          {/* Input Area - Always visible at bottom */}
          <div className="border-t p-3 bg-muted/30">
            <Textarea
              ref={textareaRef}
              value={content}
              onChange={(e) => setContent(e.target.value)}
              onKeyDown={handleSubmitOrCancel(handleSubmit, handleNewMemoEscape)}
              placeholder="新しいメモを入力..."
              className="min-h-[60px] max-h-[100px] resize-none bg-background"
              rows={2}
            />
            <div className="flex items-center justify-between mt-2">
              <span className="text-xs text-muted-foreground">
                Ctrl+Enter で保存
              </span>
              <Button
                size="sm"
                onClick={handleSubmit}
                disabled={!content.trim() || isSubmitting}
                className="gap-1"
              >
                {isSubmitting ? (
                  <Loader2 className="h-4 w-4 animate-spin" />
                ) : (
                  <Send className="h-4 w-4" />
                )}
                保存
              </Button>
            </div>
          </div>
        </div>
      </div>

      {/* Floating Button */}
      <Button
        onClick={() => setIsOpen(!isOpen)}
        className={cn(
          'fixed bottom-6 right-6 z-50 h-14 w-14 rounded-full shadow-lg',
          'transition-all duration-200',
          isOpen && 'rotate-45'
        )}
        size="icon"
      >
        <Lightbulb className={cn('h-6 w-6', isOpen && 'hidden')} />
        <X className={cn('h-6 w-6', !isOpen && 'hidden')} />
      </Button>
    </>
  )
}
