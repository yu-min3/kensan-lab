import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '@/components/ui/dialog'
import type { DialogState } from '@/hooks/useDialogState'
import { DEFAULT_COLORS, type TagCategory } from '@/types'
import { cn } from '@/lib/utils'

export interface TagFormData {
  name: string
  color: string
  category?: TagCategory
}

const CATEGORIES: { value: TagCategory; label: string; icon: string }[] = [
  { value: 'general', label: '一般', icon: '' },
  { value: 'trait', label: '性質', icon: '🏷️' },
  { value: 'tech', label: '技術', icon: '💻' },
  { value: 'project', label: 'プロジェクト', icon: '📁' },
]

interface TagDialogProps {
  dialog: DialogState<TagFormData>
  onSave: (data: TagFormData, editingId: string | null) => Promise<void>
}

const TAG_COLORS = [
  ...DEFAULT_COLORS,
  '#EC4899', // Pink
  '#14B8A6', // Teal
  '#6366F1', // Indigo
  '#84CC16', // Lime
  '#F97316', // Orange
]

export function TagDialog({ dialog, onSave }: TagDialogProps) {
  const handleSave = async () => {
    if (!dialog.data.name.trim()) return
    await onSave(dialog.data, dialog.editingId)
    dialog.close()
  }

  return (
    <Dialog open={dialog.isOpen} onOpenChange={dialog.setIsOpen}>
      <DialogContent className="max-w-sm">
        <DialogHeader>
          <DialogTitle>
            {dialog.isEditing ? 'タグを編集' : 'タグを追加'}
          </DialogTitle>
        </DialogHeader>

        <div className="space-y-4 py-4">
          <div>
            <Label htmlFor="tagName">タグ名</Label>
            <Input
              id="tagName"
              value={dialog.data.name}
              onChange={(e) => dialog.setField('name', e.target.value)}
              placeholder="例: 勉強"
              className="mt-1"
              autoFocus
            />
          </div>

          <div>
            <Label>カラー</Label>
            <div className="flex flex-wrap gap-2 mt-2">
              {TAG_COLORS.map((color) => (
                <button
                  key={color}
                  type="button"
                  onClick={() => dialog.setField('color', color)}
                  className={cn(
                    'w-8 h-8 rounded-full transition-all',
                    dialog.data.color === color
                      ? 'ring-2 ring-offset-2 ring-primary scale-110'
                      : 'hover:scale-105'
                  )}
                  style={{ backgroundColor: color }}
                />
              ))}
            </div>
          </div>

          <div>
            <Label>カテゴリ</Label>
            <div className="flex flex-wrap gap-2 mt-2">
              {CATEGORIES.map((cat) => (
                <button
                  key={cat.value}
                  type="button"
                  onClick={() => dialog.setField('category', cat.value)}
                  className={cn(
                    'px-3 py-1.5 rounded-md text-sm border transition-all',
                    (dialog.data.category || 'general') === cat.value
                      ? 'bg-primary text-primary-foreground border-primary'
                      : 'bg-background hover:bg-accent border-border'
                  )}
                >
                  {cat.icon && <span className="mr-1">{cat.icon}</span>}
                  {cat.label}
                </button>
              ))}
            </div>
          </div>

          {/* Preview */}
          {dialog.data.name && (
            <div>
              <Label className="text-muted-foreground">プレビュー</Label>
              <div className="mt-2">
                <span
                  className="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium"
                  style={{
                    backgroundColor: `${dialog.data.color}20`,
                    color: dialog.data.color,
                    border: `1px solid ${dialog.data.color}40`,
                  }}
                >
                  <span
                    className="w-1.5 h-1.5 rounded-full"
                    style={{ backgroundColor: dialog.data.color }}
                  />
                  {dialog.data.name}
                </span>
              </div>
            </div>
          )}
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={dialog.close}>
            キャンセル
          </Button>
          <Button onClick={handleSave} disabled={!dialog.data.name.trim()}>
            保存
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
