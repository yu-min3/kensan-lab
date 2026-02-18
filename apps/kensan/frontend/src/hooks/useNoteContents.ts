// Hook for managing NoteContent operations
import { useState, useCallback } from 'react'
import { notesApi } from '@/api'
import type { NoteContent, ContentType } from '@/types'
import type { CreateNoteContentInput, UpdateNoteContentInput } from '@/api/services/notes'

interface UseNoteContentsReturn {
  contents: NoteContent[]
  loading: boolean
  error: string | null
  fetchContents: (noteId: string) => Promise<NoteContent[]>
  createContent: (noteId: string, input: CreateNoteContentInput) => Promise<NoteContent>
  updateContent: (noteId: string, contentId: string, input: UpdateNoteContentInput) => Promise<NoteContent>
  deleteContent: (noteId: string, contentId: string) => Promise<void>
  reorderContents: (noteId: string, contentIds: string[]) => Promise<void>
  uploadFile: (noteId: string, file: File, contentType: ContentType, metadata?: Record<string, unknown>) => Promise<NoteContent>
}

export function useNoteContents(): UseNoteContentsReturn {
  const [contents, setContents] = useState<NoteContent[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const fetchContents = useCallback(async (noteId: string): Promise<NoteContent[]> => {
    setLoading(true)
    setError(null)
    try {
      const data = await notesApi.listContents(noteId)
      setContents(data)
      return data
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to fetch contents')
      return []
    } finally {
      setLoading(false)
    }
  }, [])

  const createContent = useCallback(async (noteId: string, input: CreateNoteContentInput) => {
    const content = await notesApi.createContent(noteId, input)
    setContents(prev => [...prev, content].sort((a, b) => a.sortOrder - b.sortOrder))
    return content
  }, [])

  const updateContent = useCallback(async (noteId: string, contentId: string, input: UpdateNoteContentInput) => {
    const updated = await notesApi.updateContent(noteId, contentId, input)
    setContents(prev => prev.map(c => c.id === contentId ? updated : c))
    return updated
  }, [])

  const deleteContent = useCallback(async (noteId: string, contentId: string) => {
    await notesApi.deleteContent(noteId, contentId)
    setContents(prev => prev.filter(c => c.id !== contentId))
  }, [])

  const reorderContents = useCallback(async (noteId: string, contentIds: string[]) => {
    await notesApi.reorderContents(noteId, contentIds)
    // Reorder local state
    setContents(prev => {
      const ordered = contentIds
        .map(id => prev.find(c => c.id === id))
        .filter((c): c is NoteContent => c !== undefined)
        .map((c, i) => ({ ...c, sortOrder: i }))
      return ordered
    })
  }, [])

  const uploadFile = useCallback(async (
    noteId: string,
    file: File,
    contentType: ContentType,
    metadata?: Record<string, unknown>
  ) => {
    const content = await notesApi.createContentWithFile(noteId, file, contentType, metadata)
    setContents(prev => [...prev, content].sort((a, b) => a.sortOrder - b.sortOrder))
    return content
  }, [])

  return {
    contents,
    loading,
    error,
    fetchContents,
    createContent,
    updateContent,
    deleteContent,
    reorderContents,
    uploadFile,
  }
}
