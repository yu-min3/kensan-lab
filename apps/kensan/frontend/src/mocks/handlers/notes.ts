// Notes MSW handlers (unified diary, learning, memo)
import { http, HttpResponse } from 'msw'
import { notes, generateId } from '../data'
import type { Note, NoteType } from '@/types'

const BASE_URL = 'http://localhost:8091/api/v1'

// In-memory storage for note contents (keyed by noteId)
const noteContentsStore: Record<string, Array<{
  id: string
  noteId: string
  contentType: string
  content?: string
  storageProvider?: string
  storageKey?: string
  fileName?: string
  mimeType?: string
  fileSizeBytes?: number
  sortOrder: number
  metadata: Record<string, unknown>
  createdAt: string
  updatedAt: string
}>> = {}

// Transform to API response format (with content)
const toNoteResponse = (n: Note) => ({
  id: n.id,
  userId: n.userId,
  type: n.type,
  title: n.title,
  content: n.content,
  format: n.format,
  date: n.date,
  taskId: n.taskId,
  milestoneId: n.milestoneId,
  milestoneName: n.milestoneName,
  goalId: n.goalId,
  goalName: n.goalName,
  goalColor: n.goalColor,
  tagIds: n.tagIds,
  relatedTimeEntryIds: n.relatedTimeEntryIds,
  fileUrl: n.fileUrl,
  archived: n.archived,
  createdAt: n.createdAt.toISOString(),
  updatedAt: n.updatedAt.toISOString(),
})

// Transform to list item response (without content)
const toNoteListItemResponse = (n: Note) => ({
  id: n.id,
  userId: n.userId,
  type: n.type,
  title: n.title,
  format: n.format,
  date: n.date,
  taskId: n.taskId,
  milestoneId: n.milestoneId,
  milestoneName: n.milestoneName,
  goalId: n.goalId,
  goalName: n.goalName,
  goalColor: n.goalColor,
  tagIds: n.tagIds,
  relatedTimeEntryIds: n.relatedTimeEntryIds,
  fileUrl: n.fileUrl,
  archived: n.archived,
  createdAt: n.createdAt.toISOString(),
  updatedAt: n.updatedAt.toISOString(),
})

// Transform to search result response
const toSearchResultResponse = (n: Note, score: number) => ({
  note: toNoteListItemResponse(n),
  score,
})

// Mock note type configurations
const mockNoteTypes = [
  {
    id: 'nt-1',
    slug: 'diary',
    displayName: '日記',
    displayNameEn: 'Diary',
    description: '日々の振り返りや気づきを記録します',
    icon: 'calendar-days',
    color: '#3B82F6',
    constraints: { dateRequired: true, titleRequired: true, contentRequired: true, dailyUnique: true },
    metadataSchema: [],
    sortOrder: 0,
    isSystem: true,
    isActive: true,
  },
  {
    id: 'nt-2',
    slug: 'learning',
    displayName: '学習記録',
    displayNameEn: 'Learning Record',
    description: '技術的な学びやナレッジを記録します',
    icon: 'book-open',
    color: '#10B981',
    constraints: { dateRequired: true, titleRequired: true, contentRequired: true, dailyUnique: true },
    metadataSchema: [],
    sortOrder: 1,
    isSystem: true,
    isActive: true,
  },
  {
    id: 'nt-3',
    slug: 'general',
    displayName: '一般ノート',
    displayNameEn: 'General Note',
    description: '自由形式のノートです',
    icon: 'file-text',
    color: '#6B7280',
    constraints: { dateRequired: false, titleRequired: true, contentRequired: true, dailyUnique: false },
    metadataSchema: [],
    sortOrder: 2,
    isSystem: false,
    isActive: true,
  },
  {
    id: 'nt-4',
    slug: 'book_review',
    displayName: '読書レビュー',
    displayNameEn: 'Book Review',
    description: '読んだ本のレビューや感想を記録します',
    icon: 'book-open-check',
    color: '#8B5CF6',
    constraints: { dateRequired: false, titleRequired: true, contentRequired: true, dailyUnique: false },
    metadataSchema: [
      { key: 'author', label: '著者', labelEn: 'Author', type: 'string', required: true, constraints: {} },
      { key: 'rating', label: '評価', labelEn: 'Rating', type: 'integer', required: false, constraints: { min: 1, max: 5 } },
      { key: 'isbn', label: 'ISBN', labelEn: 'ISBN', type: 'string', required: false, constraints: {} },
      { key: 'publisher', label: '出版社', labelEn: 'Publisher', type: 'string', required: false, constraints: {} },
      { key: 'finished_date', label: '読了日', labelEn: 'Finished Date', type: 'date', required: false, constraints: {} },
      { key: 'category', label: 'カテゴリ', labelEn: 'Category', type: 'enum', required: false, constraints: { values: ['技術書', 'ビジネス', '自己啓発', '小説', 'その他'] } },
    ],
    sortOrder: 3,
    isSystem: false,
    isActive: true,
  },
]

export const noteHandlers = [
  // GET /note-types - List note type configurations
  http.get(`${BASE_URL}/note-types`, () => {
    return HttpResponse.json(mockNoteTypes)
  }),

  // GET /notes - List notes (returns items without content)
  http.get(`${BASE_URL}/notes`, ({ request }) => {
    const url = new URL(request.url)
    let result = [...notes]

    // Filter by types (comma-separated)
    const typesParam = url.searchParams.get('types')
    if (typesParam) {
      const types = typesParam.split(',') as NoteType[]
      result = result.filter((n) => types.includes(n.type))
    }

    // Filter by goal_id
    const goalId = url.searchParams.get('goal_id')
    if (goalId) {
      result = result.filter((n) => n.goalId === goalId)
    }

    // Filter by milestone_id
    const milestoneId = url.searchParams.get('milestone_id')
    if (milestoneId) {
      result = result.filter((n) => n.milestoneId === milestoneId)
    }

    // Filter by task_id
    const taskId = url.searchParams.get('task_id')
    if (taskId) {
      result = result.filter((n) => n.taskId === taskId)
    }

    // Filter by format
    const format = url.searchParams.get('format')
    if (format) {
      result = result.filter((n) => n.format === format)
    }

    // Filter by archived
    const archived = url.searchParams.get('archived')
    if (archived !== null) {
      result = result.filter((n) => n.archived === (archived === 'true'))
    }

    // Filter by date_from
    const dateFrom = url.searchParams.get('date_from')
    if (dateFrom) {
      result = result.filter((n) => n.date && n.date >= dateFrom)
    }

    // Filter by date_to
    const dateTo = url.searchParams.get('date_to')
    if (dateTo) {
      result = result.filter((n) => n.date && n.date <= dateTo)
    }

    // Filter by tag_ids (comma-separated, AND condition)
    const tagIds = url.searchParams.get('tag_ids')
    if (tagIds) {
      const filterTagIds = tagIds.split(',')
      result = result.filter((n) =>
        filterTagIds.every((tagId) => n.tagIds?.includes(tagId))
      )
    }

    // Search query
    const q = url.searchParams.get('q')
    if (q) {
      const query = q.toLowerCase()
      result = result.filter(
        (n) =>
          n.title?.toLowerCase().includes(query) ||
          n.content.toLowerCase().includes(query)
      )
    }

    // Sort by createdAt desc
    result.sort((a, b) => b.createdAt.getTime() - a.createdAt.getTime())

    return HttpResponse.json(result.map(toNoteListItemResponse))
  }),

  // GET /notes/search - Search notes
  http.get(`${BASE_URL}/notes/search`, ({ request }) => {
    const url = new URL(request.url)
    const q = url.searchParams.get('q')

    if (!q) {
      return HttpResponse.json(
        { code: 'VALIDATION_ERROR', message: 'Query is required' },
        { status: 400 }
      )
    }

    const query = q.toLowerCase()
    let result = [...notes]

    // Filter by types
    const typesParam = url.searchParams.get('types')
    if (typesParam) {
      const types = typesParam.split(',') as NoteType[]
      result = result.filter((n) => types.includes(n.type))
    }

    // Filter by archived
    const archived = url.searchParams.get('archived')
    if (archived !== null) {
      result = result.filter((n) => n.archived === (archived === 'true'))
    }

    // Search and score
    const searchResults = result
      .map((n) => {
        let score = 0
        if (n.title?.toLowerCase().includes(query)) {
          score = 1.0
        } else if (n.content.toLowerCase().includes(query)) {
          score = 0.5
        }
        return { note: n, score }
      })
      .filter((r) => r.score > 0)
      .sort((a, b) => b.score - a.score || b.note.createdAt.getTime() - a.note.createdAt.getTime())

    // Apply limit
    const limit = parseInt(url.searchParams.get('limit') || '20', 10)
    const limited = searchResults.slice(0, limit)

    return HttpResponse.json(limited.map((r) => toSearchResultResponse(r.note, r.score)))
  }),

  // GET /notes/:id - Get note by ID
  http.get(`${BASE_URL}/notes/:id`, ({ params }) => {
    const note = notes.find((n) => n.id === params.id)
    if (!note) {
      return HttpResponse.json(
        { code: 'NOT_FOUND', message: 'Note not found' },
        { status: 404 }
      )
    }
    return HttpResponse.json(toNoteResponse(note))
  }),

  // POST /notes - Create note
  http.post(`${BASE_URL}/notes`, async ({ request }) => {
    const body = (await request.json()) as Partial<Note>
    const now = new Date()

    const newNote: Note = {
      id: generateId('note'),
      userId: 'user-1',
      type: body.type || 'diary',
      title: body.title,
      content: body.content || '',
      format: body.format || 'markdown',
      date: body.date,
      taskId: body.taskId,
      milestoneId: body.milestoneId,
      milestoneName: body.milestoneName,
      goalId: body.goalId,
      goalName: body.goalName,
      goalColor: body.goalColor,
      tagIds: body.tagIds,
      relatedTimeEntryIds: body.relatedTimeEntryIds,
      fileUrl: body.fileUrl,
      archived: false,
      createdAt: now,
      updatedAt: now,
    }

    notes.unshift(newNote)
    return HttpResponse.json(toNoteResponse(newNote), { status: 201 })
  }),

  // PUT /notes/:id - Update note
  http.put(`${BASE_URL}/notes/:id`, async ({ params, request }) => {
    const index = notes.findIndex((n) => n.id === params.id)
    if (index === -1) {
      return HttpResponse.json(
        { code: 'NOT_FOUND', message: 'Note not found' },
        { status: 404 }
      )
    }

    const body = (await request.json()) as Partial<Note>
    notes[index] = {
      ...notes[index],
      ...body,
      updatedAt: new Date(),
    }

    return HttpResponse.json(toNoteResponse(notes[index]))
  }),

  // DELETE /notes/:id - Delete note
  http.delete(`${BASE_URL}/notes/:id`, ({ params }) => {
    const index = notes.findIndex((n) => n.id === params.id)
    if (index === -1) {
      return HttpResponse.json(
        { code: 'NOT_FOUND', message: 'Note not found' },
        { status: 404 }
      )
    }

    notes.splice(index, 1)
    return new HttpResponse(null, { status: 204 })
  }),

  // POST /notes/:id/contents/upload-url - Get presigned upload URL
  http.post(`${BASE_URL}/notes/:id/contents/upload-url`, async ({ params, request }) => {
    const note = notes.find((n) => n.id === params.id)
    if (!note) {
      return HttpResponse.json(
        { code: 'NOT_FOUND', message: 'Note not found' },
        { status: 404 }
      )
    }

    const body = (await request.json()) as { fileName: string; mimeType: string; fileSize: number }
    const contentId = generateId('content')
    const storageKey = `notes/${params.id}/${contentId}/${body.fileName}`

    return HttpResponse.json({
      uploadUrl: `http://localhost:9000/kensan-notes/${storageKey}`,
      contentId,
      storageKey,
    })
  }),

  // GET /notes/:id/contents - List note contents
  http.get(`${BASE_URL}/notes/:id/contents`, ({ params }) => {
    const noteId = params.id as string
    const note = notes.find((n) => n.id === noteId)
    if (!note) {
      return HttpResponse.json(
        { code: 'NOT_FOUND', message: 'Note not found' },
        { status: 404 }
      )
    }

    const contents = noteContentsStore[noteId] ?? []
    return HttpResponse.json(contents.sort((a, b) => a.sortOrder - b.sortOrder))
  }),

  // POST /notes/:id/contents - Create note content
  http.post(`${BASE_URL}/notes/:id/contents`, async ({ params, request }) => {
    const noteId = params.id as string
    const note = notes.find((n) => n.id === noteId)
    if (!note) {
      return HttpResponse.json(
        { code: 'NOT_FOUND', message: 'Note not found' },
        { status: 404 }
      )
    }

    const body = (await request.json()) as Record<string, unknown>
    const now = new Date().toISOString()
    const contentId = (body.contentId as string) || generateId('content')

    const newContent = {
      id: contentId,
      noteId,
      contentType: (body.contentType as string) || 'image',
      content: body.content as string | undefined,
      storageProvider: body.storageProvider as string | undefined,
      storageKey: body.storageKey as string | undefined,
      fileName: body.fileName as string | undefined,
      mimeType: body.mimeType as string | undefined,
      fileSizeBytes: body.fileSizeBytes as number | undefined,
      sortOrder: (body.sortOrder as number) || 0,
      metadata: (body.metadata as Record<string, unknown>) || {},
      createdAt: now,
      updatedAt: now,
    }

    if (!noteContentsStore[noteId]) {
      noteContentsStore[noteId] = []
    }
    noteContentsStore[noteId].push(newContent)

    return HttpResponse.json(newContent, { status: 201 })
  }),

  // PUT /notes/:id/contents/:contentId - Update note content
  http.put(`${BASE_URL}/notes/:id/contents/:contentId`, async ({ params, request }) => {
    const noteId = params.id as string
    const contentId = params.contentId as string
    const contents = noteContentsStore[noteId]
    if (!contents) {
      return HttpResponse.json(
        { code: 'NOT_FOUND', message: 'Content not found' },
        { status: 404 }
      )
    }

    const index = contents.findIndex((c) => c.id === contentId)
    if (index === -1) {
      return HttpResponse.json(
        { code: 'NOT_FOUND', message: 'Content not found' },
        { status: 404 }
      )
    }

    const body = (await request.json()) as Record<string, unknown>
    contents[index] = {
      ...contents[index],
      ...(body.content !== undefined ? { content: body.content as string } : {}),
      ...(body.sortOrder !== undefined ? { sortOrder: body.sortOrder as number } : {}),
      ...(body.metadata !== undefined ? { metadata: body.metadata as Record<string, unknown> } : {}),
      updatedAt: new Date().toISOString(),
    }

    return HttpResponse.json(contents[index])
  }),

  // DELETE /notes/:id/contents/:contentId - Delete note content
  http.delete(`${BASE_URL}/notes/:id/contents/:contentId`, ({ params }) => {
    const noteId = params.id as string
    const contentId = params.contentId as string
    const contents = noteContentsStore[noteId]
    if (!contents) {
      return HttpResponse.json(
        { code: 'NOT_FOUND', message: 'Content not found' },
        { status: 404 }
      )
    }

    const index = contents.findIndex((c) => c.id === contentId)
    if (index === -1) {
      return HttpResponse.json(
        { code: 'NOT_FOUND', message: 'Content not found' },
        { status: 404 }
      )
    }

    contents.splice(index, 1)
    return new HttpResponse(null, { status: 204 })
  }),

  // GET /notes/:id/contents/:contentId/download-url - Get download URL
  http.get(`${BASE_URL}/notes/:id/contents/:contentId/download-url`, ({ params }) => {
    const note = notes.find((n) => n.id === params.id)
    if (!note) {
      return HttpResponse.json(
        { code: 'NOT_FOUND', message: 'Note not found' },
        { status: 404 }
      )
    }

    return HttpResponse.json({
      downloadUrl: `http://localhost:9000/kensan-notes/notes/${params.id}/${params.contentId}/image.png`,
    })
  }),

  // POST /notes/:id/archive - Archive/unarchive note
  http.post(`${BASE_URL}/notes/:id/archive`, async ({ params, request }) => {
    const index = notes.findIndex((n) => n.id === params.id)
    if (index === -1) {
      return HttpResponse.json(
        { code: 'NOT_FOUND', message: 'Note not found' },
        { status: 404 }
      )
    }

    const body = (await request.json()) as { archived: boolean }
    notes[index] = {
      ...notes[index],
      archived: body.archived,
      updatedAt: new Date(),
    }

    return HttpResponse.json(toNoteResponse(notes[index]))
  }),
]
