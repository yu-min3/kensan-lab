import { toast } from 'sonner'
import { useTaskManagerStore } from '@/stores/useTaskManagerStore'
import { useDialogState } from '@/hooks/useDialogState'
import { DEFAULT_COLORS } from '@/types'
import type { Tag } from '@/types'
import type { TagFormData } from '@/components/task/TagDialog'

const initialTagFormData: TagFormData = {
  name: '',
  color: DEFAULT_COLORS[0],
}

export function useTagManagement() {
  const {
    tags,
    addTag,
    updateTag,
    deleteTag,
  } = useTaskManagerStore()

  // Dialog
  const tagDialog = useDialogState<TagFormData>(initialTagFormData)

  // Tag CRUD handlers
  const openEditTagDialog = (tag: Tag) => {
    tagDialog.openEdit(tag.id, { name: tag.name, color: tag.color })
  }

  const handleSaveTag = async (data: TagFormData, editingId: string | null) => {
    try {
      if (editingId) {
        await updateTag(editingId, { name: data.name, color: data.color })
        toast.success('タグを更新しました')
      } else {
        await addTag({ name: data.name, color: data.color })
        toast.success('タグを追加しました')
      }
    } catch {
      // エラートーストはhttpClientで表示される
    }
  }

  const handleDeleteTag = async (id: string) => {
    try {
      await deleteTag(id)
      toast.success('タグを削除しました')
    } catch {
      // エラートーストはhttpClientで表示される
    }
  }

  return {
    // Data
    tags,

    // CRUD
    tagDialog,
    openEditTagDialog,
    handleSaveTag,
    handleDeleteTag,
  }
}
