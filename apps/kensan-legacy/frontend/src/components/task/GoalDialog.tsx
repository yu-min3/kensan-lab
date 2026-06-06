import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '@/components/ui/dialog'
import { EntityMemoSection } from '@/components/common/EntityMemoSection'
import type { DialogState } from '@/hooks/useDialogState'
import { DEFAULT_COLORS, type GoalStatus } from '@/types'

export interface GoalFormData {
  name: string
  description: string
  color: string
  status?: GoalStatus
}

interface GoalDialogProps {
  dialog: DialogState<GoalFormData>
  onSave: (data: GoalFormData, editingId: string | null) => Promise<void>
}

export function GoalDialog({ dialog, onSave }: GoalDialogProps) {
  const handleSave = async () => {
    if (!dialog.data.name) return
    await onSave(dialog.data, dialog.editingId)
    dialog.close()
  }

  return (
    <Dialog open={dialog.isOpen} onOpenChange={dialog.setIsOpen}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>
            {dialog.isEditing ? '目標を編集' : '目標を追加'}
          </DialogTitle>
        </DialogHeader>

        <div className="space-y-4 py-4">
          <div>
            <Label htmlFor="goalName">目標名</Label>
            <Input
              id="goalName"
              value={dialog.data.name}
              onChange={(e) => dialog.setField('name', e.target.value)}
              placeholder="例: GCPスキルアップ"
              className="mt-1"
            />
          </div>

          <div>
            <Label htmlFor="goalDescription">説明（任意）</Label>
            <Input
              id="goalDescription"
              value={dialog.data.description}
              onChange={(e) => dialog.setField('description', e.target.value)}
              placeholder="例: GCP認定資格を取得する"
              className="mt-1"
            />
          </div>

          <div>
            <Label htmlFor="goalColor">カラー</Label>
            <div className="flex flex-wrap gap-2 mt-2 mb-2">
              {DEFAULT_COLORS.map((color) => (
                <button
                  key={color}
                  type="button"
                  className={`w-8 h-8 rounded-full border-2 ${
                    dialog.data.color === color ? 'border-foreground' : 'border-transparent'
                  }`}
                  style={{ backgroundColor: color }}
                  onClick={() => dialog.setField('color', color)}
                />
              ))}
            </div>
            <div className="flex items-center gap-2">
              <Input
                id="goalColor"
                type="color"
                value={dialog.data.color}
                onChange={(e) => dialog.setField('color', e.target.value)}
                className="w-12 h-10 p-1"
              />
              <Input
                value={dialog.data.color}
                onChange={(e) => dialog.setField('color', e.target.value)}
                className="flex-1"
              />
            </div>
          </div>

          {/* Status - only show when editing existing goal */}
          {dialog.editingId && (
            <div>
              <Label>ステータス</Label>
              <Select
                value={dialog.data.status || 'active'}
                onValueChange={(v) => dialog.setField('status', v as GoalStatus)}
              >
                <SelectTrigger className="mt-1">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="active">アクティブ</SelectItem>
                  <SelectItem value="completed">完了</SelectItem>
                  <SelectItem value="archived">アーカイブ</SelectItem>
                </SelectContent>
              </Select>
            </div>
          )}

          {/* Entity Memos - only show when editing existing goal */}
          {dialog.editingId && (
            <EntityMemoSection
              entityType="goal"
              entityId={dialog.editingId}
              maxHeight="150px"
            />
          )}
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={dialog.close}>
            キャンセル
          </Button>
          <Button onClick={handleSave} disabled={!dialog.data.name}>
            保存
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
