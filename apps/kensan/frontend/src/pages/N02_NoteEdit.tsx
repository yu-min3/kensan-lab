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
  Check,
  Loader2,
  AlertCircle,
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

  // Auto-save state
  const [autoSaveStatus, setAutoSaveStatus] = useState<'idle' | 'saving' | 'saved' | 'error'>('idle')
  const autoSaveTimerRef = useRef<ReturnType<typeof setTimeout>>()
  const lastSavedJsonRef = useRef<string>('')
  const isLoadedRef = useRef(false)
  const skipNextFetchRef = useRef(false)

  // Fetch existing note + note_contents
  useEffect(() => {
    if (id) {
      // Skip re-fetch after auto-save created a new note
      if (skipNextFetchRef.current) {
        skipNextFetchRef.current = false
        return
      }
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

          const loadedValue: NoteEditorValue = {
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
          }
          setEditorValue(loadedValue)
          lastSavedJsonRef.current = JSON.stringify(loadedValue)
          isLoadedRef.current = true
          setIsLoading(false)
        })
        .catch(() => {
          setIsLoading(false)
          navigate('/notes', { replace: true })
        })
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id, fetchNote, navigate])

  // Mark new notes as loaded immediately
  useEffect(() => {
    if (isNew) {
      isLoadedRef.current = true
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  // TODO(human): Draft recovery from localStorage
  // When auto-save fails (e.g., session expired → 401), the content is saved to
  // localStorage under key `kensan-draft-${id || 'new'}` as a JSON NoteEditorValue.
  // Implement a useEffect that checks for a saved draft on mount and recovers it.
  // Consider: auto-restore silently? Or show a toast asking the user?

  // Store editorValue in a ref so ensureNoteId always reads the latest
  const editorValueRef = useRef(editorValue)
  editorValueRef.current = editorValue

  // Core save logic (shared by manual save and auto-save)
  const performSave = async (): Promise<void> => {
    const ev = editorValueRef.current

    const milestone = ev.milestoneId ? getMilestoneById(ev.milestoneId) : undefined
    const goal = ev.goalId
      ? getGoalById(ev.goalId)
      : milestone
        ? getGoalById(milestone.goalId)
        : undefined

    const metadataEntries = ev.typeMetadata
      ? Object.entries(ev.typeMetadata).filter(([, v]) => v !== '')
      : []

    const noteData = {
      type: ev.type,
      title: ev.title?.trim() || undefined,
      content: ev.content,
      format: 'markdown' as const,
      date: ev.date,
      taskId: ev.taskId,
      milestoneId: ev.milestoneId,
      milestoneName: milestone?.name,
      goalId: goal?.id || ev.goalId,
      goalName: goal?.name,
      goalColor: goal?.color,
      tagIds: ev.tagIds,
      metadata: metadataEntries.length > 0
        ? metadataEntries.map(([key, value]) => ({ key, value }))
        : undefined,
    }

    let savedNoteId: string | undefined
    if (currentNoteIdRef.current && isNew) {
      await updateNote(currentNoteIdRef.current, noteData)
      savedNoteId = currentNoteIdRef.current
    } else if (isNew) {
      const created = await createNote(noteData)
      savedNoteId = created.id
      currentNoteIdRef.current = created.id
      setExistingNote(created)
      skipNextFetchRef.current = true
      navigate(`/notes/${created.id}`, { replace: true })
    } else if (id) {
      await updateNote(id, noteData)
      savedNoteId = id
    }

    if (savedNoteId) {
      // Markdown content
      if (markdownContentIdRef.current) {
        await noteContents.updateContent(savedNoteId, markdownContentIdRef.current, {
          content: ev.content,
        })
      } else {
        const created = await noteContents.createContent(savedNoteId, {
          contentType: 'markdown',
          content: ev.content,
          sortOrder: 0,
        })
        markdownContentIdRef.current = created.id
      }

      // Drawio content
      if (ev.hasDrawio) {
        if (drawioContentIdRef.current) {
          await noteContents.updateContent(savedNoteId, drawioContentIdRef.current, {
            content: ev.drawioContent || '',
          })
        } else {
          const created = await noteContents.createContent(savedNoteId, {
            contentType: 'drawio',
            content: ev.drawioContent || '',
            sortOrder: 1,
          })
          drawioContentIdRef.current = created.id
        }
      } else if (drawioContentIdRef.current) {
        await noteContents.deleteContent(savedNoteId, drawioContentIdRef.current)
        drawioContentIdRef.current = undefined
      }

      // Mindmap content
      if (ev.hasMindmap) {
        if (mindmapContentIdRef.current) {
          await noteContents.updateContent(savedNoteId, mindmapContentIdRef.current, {
            content: ev.mindmapContent || '',
          })
        } else {
          const created = await noteContents.createContent(savedNoteId, {
            contentType: 'mindmap',
            content: ev.mindmapContent || '',
            sortOrder: 2,
          })
          mindmapContentIdRef.current = created.id
        }
      } else if (mindmapContentIdRef.current) {
        await noteContents.deleteContent(savedNoteId, mindmapContentIdRef.current)
        mindmapContentIdRef.current = undefined
      }
    }
  }

  const performSaveRef = useRef(performSave)
  performSaveRef.current = performSave

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
    if (autoSaveTimerRef.current) clearTimeout(autoSaveTimerRef.current)
    setIsSaving(true)
    try {
      await performSave()
      lastSavedJsonRef.current = JSON.stringify(editorValueRef.current)
      setAutoSaveStatus('saved')
      // Clear any emergency draft from localStorage
      const draftKey = `kensan-draft-${currentNoteIdRef.current || 'new'}`
      localStorage.removeItem(draftKey)
      toast.success('保存しました')
      navigate('/notes')
    } catch {
      // エラートーストはhttpClientで表示されるため、ここでは何もしない
    } finally {
      setIsSaving(false)
    }
  }

  // Auto-save: debounce 3s after editorValue changes
  useEffect(() => {
    if (!isLoadedRef.current) return

    const currentJson = JSON.stringify(editorValue)
    if (currentJson === lastSavedJsonRef.current) return

    // Don't auto-save empty new notes
    if (isNew && !currentNoteIdRef.current && !editorValue.title?.trim() && !editorValue.content?.trim()) return

    setAutoSaveStatus('idle')
    if (autoSaveTimerRef.current) clearTimeout(autoSaveTimerRef.current)
    autoSaveTimerRef.current = setTimeout(async () => {
      setAutoSaveStatus('saving')
      try {
        await performSaveRef.current()
        lastSavedJsonRef.current = JSON.stringify(editorValueRef.current)
        setAutoSaveStatus('saved')
        // Clear any emergency draft
        const draftKey = `kensan-draft-${currentNoteIdRef.current || 'new'}`
        localStorage.removeItem(draftKey)
      } catch {
        setAutoSaveStatus('error')
        // Emergency: save to localStorage so content survives session expiry
        try {
          const draftKey = `kensan-draft-${currentNoteIdRef.current || 'new'}`
          localStorage.setItem(draftKey, currentJson)
        } catch { /* localStorage full */ }
      }
    }, 3000)

    return () => {
      if (autoSaveTimerRef.current) clearTimeout(autoSaveTimerRef.current)
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [editorValue, isNew])

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
        <div className="flex items-center gap-2">
          {autoSaveStatus === 'saving' && (
            <span className="flex items-center gap-1 text-xs text-muted-foreground">
              <Loader2 className="h-3 w-3 animate-spin" />
              自動保存中...
            </span>
          )}
          {autoSaveStatus === 'saved' && (
            <span className="flex items-center gap-1 text-xs text-green-600">
              <Check className="h-3 w-3" />
              保存済み
            </span>
          )}
          {autoSaveStatus === 'error' && (
            <span className="flex items-center gap-1 text-xs text-destructive">
              <AlertCircle className="h-3 w-3" />
              保存失敗
            </span>
          )}
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
