import { useState, useRef, useEffect } from 'react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'
import { useMemoStore } from '@/stores/useMemoStore'
import { formatTime } from '@/lib/dateFormat'
import { handleSubmitOrCancel } from '@/lib/keyboardHandlers'
import { Lightbulb, Archive, Trash2, Loader2 } from 'lucide-react'
import { toast } from 'sonner'

export function MemoSection() {
  const { fetchMemos, updateMemo, archiveMemo, deleteMemo, getActiveMemos } = useMemoStore()
  const [editingId, setEditingId] = useState<string | null>(null)
  const [editContent, setEditContent] = useState('')
  const [isSaving, setIsSaving] = useState(false)
  const editTextareaRef = useRef<HTMLTextAreaElement>(null)

  useEffect(() => {
    fetchMemos()
  }, [fetchMemos])

  const activeMemos = getActiveMemos()

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

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2 text-base">
          <Lightbulb className="h-4 w-4 text-slate-500" />
          今日のメモ
        </CardTitle>
      </CardHeader>
      <CardContent>
        {activeMemos.length === 0 ? (
          <p className="text-sm text-muted-foreground text-center py-4">
            メモがありません。右下のボタンからメモを追加できます。
          </p>
        ) : (
          <div className="space-y-2">
            {activeMemos.map((memo) => (
              <div
                key={memo.id}
                className="flex items-start gap-3 p-3 rounded-lg bg-muted/50 group"
              >
                <div className="flex-1 min-w-0">
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
                      className="text-sm whitespace-pre-wrap break-words cursor-text hover:bg-muted rounded px-1 -mx-1"
                      onClick={() => handleStartEdit(memo)}
                    >
                      {memo.content}
                    </p>
                  )}
                  <p className="text-xs text-muted-foreground mt-1">
                    {formatTime(memo.createdAt)}
                  </p>
                </div>
                <div className={`flex gap-1 transition-opacity ${editingId === memo.id ? 'opacity-0' : 'opacity-0 group-hover:opacity-100'}`}>
                  <Button
                    variant="ghost"
                    size="sm"
                    className="h-7 w-7 p-0"
                    onClick={() => archiveMemo(memo.id)}
                    title="アーカイブ"
                  >
                    <Archive className="h-3.5 w-3.5" />
                  </Button>
                  <Button
                    variant="ghost"
                    size="sm"
                    className="h-7 w-7 p-0 text-destructive hover:text-destructive"
                    onClick={() => deleteMemo(memo.id)}
                    title="削除"
                  >
                    <Trash2 className="h-3.5 w-3.5" />
                  </Button>
                </div>
              </div>
            ))}
          </div>
        )}
      </CardContent>
    </Card>
  )
}
