import { useState, useEffect, useCallback, useRef } from 'react'
import { useParams, useNavigate, useSearchParams } from 'react-router-dom'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { ConfirmPopover } from '@/components/common/ConfirmPopover'
import { NoteEditor, NoteEditorValue } from '@/components/note/NoteEditor'
import { NoteMetadataSidebar } from '@/components/note/NoteMetadataSidebar'
import { useNoteStore } from '@/stores/useNoteStore'
import { useNoteTypeStore } from '@/stores/useNoteTypeStore'
import { useTaskManagerStore } from '@/stores/useTaskManagerStore'
import { useNoteTagStore } from '@/stores/useNoteTagStore'
import { useNoteContents } from '@/hooks/useNoteContents'
import { getNoteTypeIcon } from '@/lib/noteTypeIcons'
import { validateMetadata } from '@/components/note/MetadataForm'
import { notesApi } from '@/api/services/notes'
import type { NoteType, Note } from '@/types'
import { format } from 'date-fns'
import {
  Save,
  Trash2,
  X,
  Archive,
  ArchiveRestore,
} from 'lucide-react'
import { PageGuide } from '@/components/guide/PageGuide'

export function N02NoteEdit() {
  const { id } = useParams()
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()

  const {
    fetchNote,
    createNote,
    updateNote,
    deleteNote,
    archiveNote,
  } = useNoteStore()
  const { types, getBySlug, getConstraints, getMetadataSchema } = useNoteTypeStore()
  const { goals, milestones, tasks, getMilestoneById, getGoalById } = useTaskManagerStore()
  const noteTags = useNoteTagStore((s) => s.items)
  const addNoteTag = useNoteTagStore((s) => s.add)
  const updateNoteTag = useNoteTagStore((s) => s.update)

  const isNew = !id
  const initialType = (searchParams.get('type') as NoteType) || (types[0]?.slug ?? 'diary')

  const typeConstraints = getConstraints(initialType)
  const isDateRequired = typeConstraints?.dateRequired ?? false

  const noteContents = useNoteContents()

  // Local state for the editor
  const [editorValue, setEditorValue] = useState<NoteEditorValue>({
    type: initialType,
    title: '',
    content: '',
    format: 'markdown',
    date: isDateRequired ? format(new Date(), 'yyyy-MM-dd') : undefined,
    taskId: undefined,
    milestoneId: undefined,
    goalId: undefined,
    tagIds: [],
    typeMetadata: {},
    hasDrawio: false,
    hasMindmap: false,
  })

  const [isLoading, setIsLoading] = useState(!isNew)
  const [isSaving, setIsSaving] = useState(false)
  const [existingNote, setExistingNote] = useState<Note | null>(null)
  const currentNoteIdRef = useRef<string | undefined>(id)
  // Track content IDs for create/update decisions on save
  const markdownContentIdRef = useRef<string | undefined>(undefined)
  const drawioContentIdRef = useRef<string | undefined>(undefined)
  const mindmapContentIdRef = useRef<string | undefined>(undefined)

  // Fetch existing note + note_contents
  useEffect(() => {
    if (id) {
      setIsLoading(true)
      Promise.all([
        fetchNote(id),
        noteContents.fetchContents(id),
      ])
        .then(([note, contents]) => {
          setExistingNote(note)
          // Convert note metadata to typeMetadata record
          const typeMetadata: Record<string, string> = {}
          if (note.metadata) {
            for (const m of note.metadata) {
              if (m.value) {
                typeMetadata[m.key] = m.value
              }
            }
          }

          // Separate markdown and drawio from note_contents
          const mdContent = contents.find(c => c.contentType === 'markdown')
          const drawioContent = contents.find(c => c.contentType === 'drawio')
          const mindmapContent = contents.find(c => c.contentType === 'mindmap')

          markdownContentIdRef.current = mdContent?.id
          drawioContentIdRef.current = drawioContent?.id
          mindmapContentIdRef.current = mindmapContent?.id

          // Determine content values
          let markdownText = note.content
          let drawioText: string | undefined
          let hasDrawio = false
          let mindmapText: string | undefined
          let hasMindmap = false

          if (mdContent?.content) {
            markdownText = mdContent.content
          }
          if (drawioContent?.content) {
            drawioText = drawioContent.content
            hasDrawio = true
          }
          if (mindmapContent?.content) {
            mindmapText = mindmapContent.content
            hasMindmap = true
          }

          if (note.format === 'drawio' && !mdContent) {
            // Legacy drawio-only note: treat notes.content as drawio
            drawioText = note.content
            hasDrawio = true
            markdownText = ''
          }

          setEditorValue({
            type: note.type,
            title: note.title,
            content: markdownText,
            format: 'markdown',
            date: note.date,
            taskId: note.taskId,
            milestoneId: note.milestoneId,
            goalId: note.goalId,
            tagIds: note.tagIds || [],
            typeMetadata,
            drawioContent: drawioText,
            hasDrawio,
            mindmapContent: mindmapText,
            hasMindmap,
          })
          setIsLoading(false)
        })
        .catch(() => {
          setIsLoading(false)
          navigate('/notes', { replace: true })
        })
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id, fetchNote, navigate])

  // Get denormalized names for saving
  const getDenormalizedData = () => {
    const milestone = editorValue.milestoneId
      ? getMilestoneById(editorValue.milestoneId)
      : undefined
    const goal = editorValue.goalId
      ? getGoalById(editorValue.goalId)
      : milestone
      ? getGoalById(milestone.goalId)
      : undefined

    return {
      milestoneName: milestone?.name,
      goalId: goal?.id || editorValue.goalId,
      goalName: goal?.name,
      goalColor: goal?.color,
    }
  }

  // Convert typeMetadata to API metadata format
  const buildMetadata = () => {
    if (!editorValue.typeMetadata) return undefined
    const entries = Object.entries(editorValue.typeMetadata).filter(([, v]) => v !== '')
    if (entries.length === 0) return undefined
    return entries.map(([key, value]) => ({ key, value }))
  }

  // Store editorValue in a ref so ensureNoteId always reads the latest
  const editorValueRef = useRef(editorValue)
  editorValueRef.current = editorValue

  const ensureNoteId = useCallback(async (): Promise<string> => {
    if (currentNoteIdRef.current) return currentNoteIdRef.current

    // Create a draft note to get an ID for file uploads.
    const ev = editorValueRef.current
    const note = await createNote({
      type: ev.type,
      title: ev.title?.trim() || '(下書き)',
      content: ev.content,
      format: ev.format,
      date: ev.date,
    })
    currentNoteIdRef.current = note.id
    setExistingNote(note)
    navigate(`/notes/${note.id}`, { replace: true })
    return note.id
  }, [createNote, navigate])

  const handleImageUpload = useCallback(async (file: File): Promise<string> => {
    const noteId = await ensureNoteId()
    const content = await notesApi.createContentWithFile(noteId, file, 'image')
    const downloadUrl = await notesApi.getDownloadURL(noteId, content.id)
    return downloadUrl
  }, [ensureNoteId])

  const handleSave = async () => {
    setIsSaving(true)
    try {
      const denormalized = getDenormalizedData()
      const noteData = {
        type: editorValue.type,
        title: editorValue.title?.trim() || undefined,
        content: editorValue.content,
        format: 'markdown' as const,
        date: editorValue.date,
        taskId: editorValue.taskId,
        milestoneId: editorValue.milestoneId,
        milestoneName: denormalized.milestoneName,
        goalId: denormalized.goalId,
        goalName: denormalized.goalName,
        goalColor: denormalized.goalColor,
        tagIds: editorValue.tagIds,
        metadata: buildMetadata(),
      }

      let savedNoteId: string | undefined
      if (currentNoteIdRef.current && isNew) {
        await updateNote(currentNoteIdRef.current, noteData)
        savedNoteId = currentNoteIdRef.current
      } else if (isNew) {
        const created = await createNote(noteData)
        savedNoteId = created.id
        currentNoteIdRef.current = created.id
      } else if (id) {
        await updateNote(id, noteData)
        savedNoteId = id
      }

      // Save note_contents (markdown + drawio)
      if (savedNoteId) {
        // Markdown content
        if (markdownContentIdRef.current) {
          await noteContents.updateContent(savedNoteId, markdownContentIdRef.current, {
            content: editorValue.content,
          })
        } else {
          const created = await noteContents.createContent(savedNoteId, {
            contentType: 'markdown',
            content: editorValue.content,
            sortOrder: 0,
          })
          markdownContentIdRef.current = created.id
        }

        // Drawio content
        if (editorValue.hasDrawio) {
          if (drawioContentIdRef.current) {
            await noteContents.updateContent(savedNoteId, drawioContentIdRef.current, {
              content: editorValue.drawioContent || '',
            })
          } else {
            const created = await noteContents.createContent(savedNoteId, {
              contentType: 'drawio',
              content: editorValue.drawioContent || '',
              sortOrder: 1,
            })
            drawioContentIdRef.current = created.id
          }
        } else if (drawioContentIdRef.current) {
          // hasDrawio toggled off — delete the drawio content
          await noteContents.deleteContent(savedNoteId, drawioContentIdRef.current)
          drawioContentIdRef.current = undefined
        }

        // Mindmap content
        if (editorValue.hasMindmap) {
          if (mindmapContentIdRef.current) {
            await noteContents.updateContent(savedNoteId, mindmapContentIdRef.current, {
              content: editorValue.mindmapContent || '',
            })
          } else {
            const created = await noteContents.createContent(savedNoteId, {
              contentType: 'mindmap',
              content: editorValue.mindmapContent || '',
              sortOrder: 2,
            })
            mindmapContentIdRef.current = created.id
          }
        } else if (mindmapContentIdRef.current) {
          // hasMindmap toggled off — delete the mindmap content
          await noteContents.deleteContent(savedNoteId, mindmapContentIdRef.current)
          mindmapContentIdRef.current = undefined
        }
      }

      toast.success('保存しました')
      navigate('/notes')
    } catch {
      // エラートーストはhttpClientで表示されるため、ここでは何もしない
    } finally {
      setIsSaving(false)
    }
  }

  const handleDelete = async () => {
    if (id) {
      try {
        await deleteNote(id)
        toast.success('削除しました')
        navigate('/notes')
      } catch {
        // エラートーストはhttpClientで表示される
      }
    }
  }

  const handleArchive = async () => {
    if (id && existingNote) {
      try {
        const newArchived = !existingNote.archived
        await archiveNote(id, newArchived)
        const updated = await fetchNote(id)
        setExistingNote(updated)
        toast.success(newArchived ? 'アーカイブしました' : '復元しました')
      } catch {
        // エラートーストはhttpClientで表示される
      }
    }
  }

  // Validation using dynamic constraints
  const isValid = () => {
    const constraints = getConstraints(editorValue.type)

    if (constraints?.contentRequired && !editorValue.content.trim()) return false
    if (constraints?.titleRequired && !editorValue.title?.trim()) return false
    if (constraints?.dateRequired && !editorValue.date) return false

    const metadataSchema = getMetadataSchema(editorValue.type)
    if (metadataSchema.length > 0) {
      const metadataErrors = validateMetadata(metadataSchema, editorValue.typeMetadata ?? {})
      if (Object.keys(metadataErrors).length > 0) return false
    }

    return true
  }

  // Get type display info
  const typeConfig = getBySlug(editorValue.type)
  const TypeIcon = getNoteTypeIcon(typeConfig?.icon ?? 'file-text')
  const typeDisplayName = typeConfig?.displayName ?? editorValue.type

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary" />
      </div>
    )
  }

  return (
    <div className="space-y-4">
      <PageGuide pageId="note-edit" />

      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <TypeIcon className="h-8 w-8 text-slate-500" />
          <h1 className="text-2xl font-bold">
            {isNew
              ? `${typeDisplayName}を作成`
              : `${typeDisplayName}を編集`}
          </h1>
        </div>
        <div className="flex gap-2">
          {!isNew && existingNote && (
            <>
              <Button
                variant="outline"
                size="sm"
                onClick={handleArchive}
                className="gap-1"
              >
                {existingNote.archived ? (
                  <>
                    <ArchiveRestore className="h-4 w-4" />
                    復元
                  </>
                ) : (
                  <>
                    <Archive className="h-4 w-4" />
                    アーカイブ
                  </>
                )}
              </Button>
              <ConfirmPopover
                message="このノートを削除しますか？"
                confirmLabel="削除"
                onConfirm={handleDelete}
                variant="destructive"
              >
                <Button
                  variant="destructive"
                  size="sm"
                  className="gap-1"
                >
                  <Trash2 className="h-4 w-4" />
                  削除
                </Button>
              </ConfirmPopover>
            </>
          )}
          <Button
            variant="outline"
            size="sm"
            onClick={() => navigate('/notes')}
            className="gap-1"
          >
            <X className="h-4 w-4" />
            閉じる
          </Button>
          <Button
            size="sm"
            onClick={handleSave}
            disabled={!isValid() || isSaving}
            className="gap-1"
          >
            <Save className="h-4 w-4" />
            {isSaving ? '保存中...' : '保存'}
          </Button>
        </div>
      </div>

      {/* Left: Editor / Right: Metadata */}
      <div className="grid gap-6 lg:grid-cols-4">
        {/* Left panel - Title + Content */}
        <div className="lg:col-span-3">
          <NoteEditor
            value={editorValue}
            onChange={setEditorValue}
            onImageUpload={handleImageUpload}
          />
        </div>

        {/* Right panel - All metadata */}
        <div>
          <NoteMetadataSidebar
            value={editorValue}
            onChange={setEditorValue}
            goals={goals}
            milestones={milestones}
            tasks={tasks}
            tags={noteTags}
            onCreateTag={(name, color) => addNoteTag({ name, color })}
            onUpdateTag={(id, data) => updateNoteTag(id, data)}
            showTypeSelector={isNew}
            existingNote={existingNote}
          />
        </div>
      </div>
    </div>
  )
}
