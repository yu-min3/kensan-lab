import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '@/components/ui/dialog'
import { EntityMemoSection } from '@/components/common/EntityMemoSection'
import type { DialogState } from '@/hooks/useDialogState'
import type { Goal, MilestoneStatus } from '@/types'

export interface MilestoneFormData {
  name: string
  description: string
  goalId: string
  startDate: string
  targetDate: string
  status: MilestoneStatus
}

interface MilestoneDialogProps {
  dialog: DialogState<MilestoneFormData>
  goals: Goal[]
  onSave: (data: MilestoneFormData, editingId: string | null) => Promise<void>
}

export function MilestoneDialog({ dialog, goals, onSave }: MilestoneDialogProps) {
  const handleSave = async () => {
    if (!dialog.data.name || !dialog.data.goalId) return
    await onSave(dialog.data, dialog.editingId)
    dialog.close()
  }

  const selectedGoal = goals.find((g) => g.id === dialog.data.goalId)

  return (
    <Dialog open={dialog.isOpen} onOpenChange={dialog.setIsOpen}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>
            {dialog.isEditing ? 'マイルストーンを編集' : 'マイルストーンを追加'}
          </DialogTitle>
        </DialogHeader>

        <div className="space-y-4 py-4">
          <div>
            <Label htmlFor="milestoneName">マイルストーン名</Label>
            <Input
              id="milestoneName"
              value={dialog.data.name}
              onChange={(e) => dialog.setField('name', e.target.value)}
              placeholder="例: ICA合格"
              className="mt-1"
            />
          </div>

          <div>
            <Label htmlFor="milestoneDescription">説明（任意）</Label>
            <Input
              id="milestoneDescription"
              value={dialog.data.description}
              onChange={(e) => dialog.setField('description', e.target.value)}
              placeholder="例: Istio Certified Associate"
              className="mt-1"
            />
          </div>

          <div>
            <Label>目標</Label>
            <Select
              value={dialog.data.goalId}
              onValueChange={(v) => dialog.setField('goalId', v)}
            >
              <SelectTrigger className="mt-1">
                <SelectValue placeholder="目標を選択" />
              </SelectTrigger>
              <SelectContent>
                {goals
                  .filter(g => g.status !== 'archived')
                  .map((goal) => (
                    <SelectItem key={goal.id} value={goal.id} label={goal.name}>
                      <div className="flex items-center gap-2">
                        <span
                          className="w-2 h-2 rounded-full"
                          style={{ backgroundColor: goal.color }}
                        />
                        {goal.name}
                      </div>
                    </SelectItem>
                  ))}
              </SelectContent>
            </Select>
            {selectedGoal && (
              <p className="text-xs text-muted-foreground mt-1 flex items-center gap-1">
                <span
                  className="w-2 h-2 rounded-full"
                  style={{ backgroundColor: selectedGoal.color }}
                />
                {selectedGoal.description || '説明なし'}
              </p>
            )}
          </div>

          <div className="grid grid-cols-2 gap-4">
            <div>
              <Label htmlFor="startDate">開始日（任意）</Label>
              <Input
                id="startDate"
                type="date"
                value={dialog.data.startDate}
                onChange={(e) => dialog.setField('startDate', e.target.value)}
                className="mt-1"
              />
            </div>
            <div>
              <Label htmlFor="targetDate">期限（任意）</Label>
              <Input
                id="targetDate"
                type="date"
                value={dialog.data.targetDate}
                onChange={(e) => dialog.setField('targetDate', e.target.value)}
                className="mt-1"
              />
            </div>
          </div>

          {dialog.isEditing && (
            <div>
              <Label>ステータス</Label>
              <Select
                value={dialog.data.status}
                onValueChange={(v) => dialog.setField('status', v as MilestoneStatus)}
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

          {/* Entity Memos - only show when editing existing milestone */}
          {dialog.editingId && (
            <EntityMemoSection
              entityType="milestone"
              entityId={dialog.editingId}
              maxHeight="150px"
            />
          )}
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={dialog.close}>
            キャンセル
          </Button>
          <Button onClick={handleSave} disabled={!dialog.data.name || !dialog.data.goalId}>
            保存
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
