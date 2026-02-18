// Notes API Service (unified diary, learning, general, book_review, etc.)
import { API_CONFIG } from '../config'
import { httpClient } from '../client'
import { createApiService, extendApiService, buildQueryParams } from '../createApiService'
import type { Note, NoteListItem, NoteSearchResult, NoteType, NoteFormat, NoteContent, ContentType, StorageProvider, NoteMetadataItem, NoteTypeConfig } from '@/types'

// ============================================
// API Response Types
// ============================================
interface NoteMetadataItemResponse {
  id: string
  noteId: string
  key: string
  value?: string
  createdAt: string
  updatedAt: string
}

interface NoteResponse {
  id: string
  userId: string
  type: NoteType
  title?: string
  content: string
  format: NoteFormat
  date?: string
  taskId?: string
  milestoneId?: string
  milestoneName?: string
  goalId?: string
  goalName?: string
  goalColor?: string
  tagIds?: string[]
  metadata?: NoteMetadataItemResponse[]
  relatedTimeEntryIds?: string[]
  fileUrl?: string
  archived: boolean
  createdAt: string
  updatedAt: string
}

interface NoteListItemResponse {
  id: string
  userId: string
  type: NoteType
  title?: string
  format: NoteFormat
  date?: string
  taskId?: string
  milestoneId?: string
  milestoneName?: string
  goalId?: string
  goalName?: string
  goalColor?: string
  tagIds?: string[]
  relatedTimeEntryIds?: string[]
  fileUrl?: string
  archived: boolean
  createdAt: string
  updatedAt: string
}

interface NoteSearchResultResponse {
  note: NoteListItemResponse
  score: number
}

// ============================================
// Transform Functions
// ============================================
const transformNoteMetadataItem = (m: NoteMetadataItemResponse): NoteMetadataItem => ({
  id: m.id,
  noteId: m.noteId,
  key: m.key,
  value: m.value,
  createdAt: new Date(m.createdAt),
  updatedAt: new Date(m.updatedAt),
})

const transformNote = (n: NoteResponse): Note => ({
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
  metadata: n.metadata?.map(transformNoteMetadataItem),
  relatedTimeEntryIds: n.relatedTimeEntryIds,
  fileUrl: n.fileUrl,
  archived: n.archived,
  createdAt: new Date(n.createdAt),
  updatedAt: new Date(n.updatedAt),
})

const transformNoteListItem = (n: NoteListItemResponse): NoteListItem => ({
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
  createdAt: new Date(n.createdAt),
  updatedAt: new Date(n.updatedAt),
})

const transformSearchResult = (r: NoteSearchResultResponse): NoteSearchResult => ({
  note: transformNoteListItem(r.note),
  score: r.score,
})

// ============================================
// NoteContent Types
// ============================================
interface NoteContentResponse {
  id: string
  noteId: string
  contentType: ContentType
  content?: string
  storageProvider?: StorageProvider
  storageKey?: string
  fileName?: string
  mimeType?: string
  fileSizeBytes?: number
  checksum?: string
  thumbnailBase64?: string
  sortOrder: number
  metadata?: Record<string, unknown>
  createdAt: string
  updatedAt: string
}

interface UploadURLResponse {
  uploadUrl: string
  contentId: string
  storageKey: string
}

interface DownloadURLResponse {
  downloadUrl: string
}

const transformNoteContent = (c: NoteContentResponse): NoteContent => ({
  id: c.id,
  noteId: c.noteId,
  contentType: c.contentType,
  content: c.content,
  storageProvider: c.storageProvider,
  storageKey: c.storageKey,
  fileName: c.fileName,
  mimeType: c.mimeType,
  fileSizeBytes: c.fileSizeBytes,
  checksum: c.checksum,
  thumbnailBase64: c.thumbnailBase64,
  sortOrder: c.sortOrder,
  metadata: c.metadata,
  createdAt: new Date(c.createdAt),
  updatedAt: new Date(c.updatedAt),
})

// ============================================
// Input Types
// ============================================
export interface SetNoteMetadataInput {
  key: string
  value?: string
}

export interface CreateNoteInput {
  type: NoteType
  title?: string
  content: string
  format: NoteFormat
  date?: string // YYYY-MM-DD, required for diary and learning
  taskId?: string
  milestoneId?: string
  milestoneName?: string
  goalId?: string
  goalName?: string
  goalColor?: string
  tagIds?: string[]
  metadata?: SetNoteMetadataInput[]
  relatedTimeEntryIds?: string[]
  fileUrl?: string
}

export interface UpdateNoteInput {
  title?: string
  content?: string
  format?: NoteFormat
  date?: string
  taskId?: string | null
  milestoneId?: string | null
  milestoneName?: string | null
  goalId?: string | null
  goalName?: string | null
  goalColor?: string | null
  tagIds?: string[]
  metadata?: SetNoteMetadataInput[]
  relatedTimeEntryIds?: string[]
  fileUrl?: string | null
  archived?: boolean
}

export interface NoteFilter {
  types?: NoteType[] // Filter by note types
  goalId?: string
  milestoneId?: string
  taskId?: string
  tagIds?: string[] // Filter by tags (AND condition)
  dateFrom?: string // YYYY-MM-DD
  dateTo?: string // YYYY-MM-DD
  archived?: boolean
  format?: NoteFormat
  q?: string // Search query
}

// ============================================
// NoteContent Input Types
// ============================================
export interface CreateNoteContentInput {
  contentType: ContentType
  content?: string
  storageProvider?: StorageProvider
  storageKey?: string
  fileName?: string
  mimeType?: string
  fileSizeBytes?: number
  checksum?: string
  thumbnailBase64?: string
  sortOrder?: number
  metadata?: Record<string, unknown>
}

export interface UpdateNoteContentInput {
  content?: string
  sortOrder?: number
  metadata?: Record<string, unknown>
  thumbnailBase64?: string
}

export interface UploadURLRequest {
  fileName: string
  mimeType: string
  fileSize: number
}

// ============================================
// Base API Service
// ============================================
const baseNotesApi = createApiService<
  NoteResponse,
  Note,
  CreateNoteInput,
  UpdateNoteInput,
  NoteFilter
>(
  {
    baseUrl: API_CONFIG.baseUrls.note,
    resourcePath: '/notes',
    transform: transformNote,
  },
  {
    filterMappings: {
      goalId: 'goal_id',
      milestoneId: 'milestone_id',
      taskId: 'task_id',
      tagIds: 'tag_ids',
      dateFrom: 'date_from',
      dateTo: 'date_to',
    },
  }
)

// ============================================
// Extended API with custom methods
// ============================================
export const notesApi = extendApiService(baseNotesApi, () => ({
  /**
   * List notes (returns items without content)
   */
  async listItems(filters?: NoteFilter): Promise<NoteListItem[]> {
    const params: Record<string, string> = {}

    if (filters?.types && filters.types.length > 0) {
      params.types = filters.types.join(',')
    }
    if (filters?.goalId) params.goal_id = filters.goalId
    if (filters?.milestoneId) params.milestone_id = filters.milestoneId
    if (filters?.taskId) params.task_id = filters.taskId
    if (filters?.tagIds && filters.tagIds.length > 0) {
      params.tag_ids = filters.tagIds.join(',')
    }
    if (filters?.dateFrom) params.date_from = filters.dateFrom
    if (filters?.dateTo) params.date_to = filters.dateTo
    if (filters?.archived !== undefined) params.archived = String(filters.archived)
    if (filters?.format) params.format = filters.format
    if (filters?.q) params.q = filters.q

    const query = buildQueryParams(params)
    const response = await httpClient.get<NoteListItemResponse[] | null>(
      API_CONFIG.baseUrls.note,
      `/notes${query}`
    )
    return (response ?? []).map(transformNoteListItem)
  },

  /**
   * Search notes
   */
  async search(
    query: string,
    filters?: Pick<NoteFilter, 'types' | 'archived'>,
    limit?: number
  ): Promise<NoteSearchResult[]> {
    const params: Record<string, string> = { q: query }

    if (filters?.types && filters.types.length > 0) {
      params.types = filters.types.join(',')
    }
    if (filters?.archived !== undefined) params.archived = String(filters.archived)
    if (limit) params.limit = String(limit)

    const queryStr = buildQueryParams(params)
    const response = await httpClient.get<NoteSearchResultResponse[] | null>(
      API_CONFIG.baseUrls.note,
      `/notes/search${queryStr}`
    )
    return (response ?? []).map(transformSearchResult)
  },

  /**
   * Archive/unarchive a note
   */
  async archive(id: string, archived: boolean): Promise<Note> {
    const response = await httpClient.post<NoteResponse>(
      API_CONFIG.baseUrls.note,
      `/notes/${id}/archive`,
      { archived }
    )
    return transformNote(response)
  },

  // ============================================
  // NoteContent Methods
  // ============================================

  /**
   * List all contents for a note
   */
  async listContents(noteId: string): Promise<NoteContent[]> {
    const response = await httpClient.get<NoteContentResponse[]>(
      API_CONFIG.baseUrls.note,
      `/notes/${noteId}/contents`
    )
    return response.map(transformNoteContent)
  },

  /**
   * Get a specific content
   */
  async getContent(noteId: string, contentId: string): Promise<NoteContent> {
    const response = await httpClient.get<NoteContentResponse>(
      API_CONFIG.baseUrls.note,
      `/notes/${noteId}/contents/${contentId}`
    )
    return transformNoteContent(response)
  },

  /**
   * Create a new content
   */
  async createContent(noteId: string, input: CreateNoteContentInput): Promise<NoteContent> {
    const response = await httpClient.post<NoteContentResponse>(
      API_CONFIG.baseUrls.note,
      `/notes/${noteId}/contents`,
      input
    )
    return transformNoteContent(response)
  },

  /**
   * Update a content
   */
  async updateContent(noteId: string, contentId: string, input: UpdateNoteContentInput): Promise<NoteContent> {
    const response = await httpClient.put<NoteContentResponse>(
      API_CONFIG.baseUrls.note,
      `/notes/${noteId}/contents/${contentId}`,
      input
    )
    return transformNoteContent(response)
  },

  /**
   * Delete a content
   */
  async deleteContent(noteId: string, contentId: string): Promise<void> {
    await httpClient.delete(
      API_CONFIG.baseUrls.note,
      `/notes/${noteId}/contents/${contentId}`
    )
  },

  /**
   * Reorder contents
   */
  async reorderContents(noteId: string, contentIds: string[]): Promise<void> {
    await httpClient.patch(
      API_CONFIG.baseUrls.note,
      `/notes/${noteId}/contents/reorder`,
      { contentIds }
    )
  },

  // ============================================
  // Storage Methods
  // ============================================

  /**
   * Get presigned URL for uploading a file
   */
  async getUploadURL(noteId: string, request: UploadURLRequest): Promise<UploadURLResponse> {
    return httpClient.post<UploadURLResponse>(
      API_CONFIG.baseUrls.note,
      `/notes/${noteId}/contents/upload-url`,
      request
    )
  },

  /**
   * Get presigned URL for downloading a file
   */
  async getDownloadURL(noteId: string, contentId: string): Promise<string> {
    const response = await httpClient.get<DownloadURLResponse>(
      API_CONFIG.baseUrls.note,
      `/notes/${noteId}/contents/${contentId}/download-url`
    )
    return response.downloadUrl
  },

  /**
   * Upload a file to storage using presigned URL
   */
  async uploadFile(uploadUrl: string, file: File): Promise<void> {
    const response = await fetch(uploadUrl, {
      method: 'PUT',
      body: file,
      headers: {
        'Content-Type': file.type,
      },
    })
    if (!response.ok) {
      throw new Error(`Upload failed: ${response.status}`)
    }
  },

  /**
   * Create content with file upload
   * Convenience method that handles the full upload flow
   */
  async createContentWithFile(
    noteId: string,
    file: File,
    contentType: ContentType,
    metadata?: Record<string, unknown>
  ): Promise<NoteContent> {
    // 1. Get upload URL
    const { uploadUrl, storageKey } = await this.getUploadURL(noteId, {
      fileName: file.name,
      mimeType: file.type,
      fileSize: file.size,
    })

    // 2. Upload file to storage
    await this.uploadFile(uploadUrl, file)

    // 3. Create content record
    return this.createContent(noteId, {
      contentType,
      storageProvider: 'minio',
      storageKey,
      fileName: file.name,
      mimeType: file.type,
      fileSizeBytes: file.size,
      metadata,
    })
  },

}))

// ============================================
// Note Types API (data-driven note type configuration)
// ============================================
interface NoteTypeConfigResponse {
  id: string
  slug: string
  displayName: string
  displayNameEn?: string
  description?: string
  icon: string
  color: string
  constraints: {
    dateRequired: boolean
    titleRequired: boolean
    contentRequired: boolean
    dailyUnique: boolean
  }
  metadataSchema: Array<{
    key: string
    label: string
    labelEn?: string
    type: string
    required: boolean
    constraints?: Record<string, unknown>
  }>
  sortOrder: number
  isSystem: boolean
  isActive: boolean
}

const transformNoteTypeConfig = (t: NoteTypeConfigResponse): NoteTypeConfig => ({
  id: t.id,
  slug: t.slug,
  displayName: t.displayName,
  displayNameEn: t.displayNameEn,
  description: t.description,
  icon: t.icon,
  color: t.color,
  constraints: t.constraints,
  metadataSchema: t.metadataSchema.map((f) => ({
    key: f.key,
    label: f.label,
    labelEn: f.labelEn,
    type: f.type as NoteTypeConfig['metadataSchema'][number]['type'],
    required: f.required,
    constraints: f.constraints,
  })),
  sortOrder: t.sortOrder,
  isSystem: t.isSystem,
  isActive: t.isActive,
})

export const noteTypesApi = {
  async list(): Promise<NoteTypeConfig[]> {
    const response = await httpClient.get<NoteTypeConfigResponse[]>(
      API_CONFIG.baseUrls.note,
      '/note-types'
    )
    return response.map(transformNoteTypeConfig)
  },
}

// ============================================
// Note Tags API (note-type tags)
// ============================================
import type { Tag, TagType, TagCategory } from '@/types'

interface NoteTagResponse {
  id: string
  name: string
  color: string
  type: TagType
  category: TagCategory
  pinned: boolean
  usageCount: number
  createdAt: string
  updatedAt: string
}

const transformNoteTag = (t: NoteTagResponse): Tag => ({
  id: t.id,
  name: t.name,
  color: t.color,
  type: t.type,
  category: t.category || 'general',
  pinned: t.pinned,
  usageCount: t.usageCount,
  createdAt: new Date(t.createdAt),
  updatedAt: new Date(t.updatedAt),
})

export interface CreateNoteTagInput {
  name: string
  color: string
  category?: TagCategory
  pinned?: boolean
}

export interface UpdateNoteTagInput {
  name?: string
  color?: string
  category?: TagCategory
  pinned?: boolean
}

export const noteTagsApi = {
  async list(): Promise<Tag[]> {
    const response = await httpClient.get<NoteTagResponse[]>(
      API_CONFIG.baseUrls.task,
      '/note-tags'
    )
    return response.map(transformNoteTag)
  },

  async create(input: CreateNoteTagInput): Promise<Tag> {
    const response = await httpClient.post<NoteTagResponse>(
      API_CONFIG.baseUrls.task,
      '/note-tags',
      input
    )
    return transformNoteTag(response)
  },

  async update(id: string, input: UpdateNoteTagInput): Promise<Tag> {
    const response = await httpClient.put<NoteTagResponse>(
      API_CONFIG.baseUrls.task,
      `/note-tags/${id}`,
      input
    )
    return transformNoteTag(response)
  },

  async delete(id: string): Promise<void> {
    await httpClient.delete(
      API_CONFIG.baseUrls.task,
      `/note-tags/${id}`
    )
  },
}

// ============================================
// Export types for external use
// ============================================
export type { Note, NoteListItem, NoteSearchResult, NoteType, NoteFormat, NoteContent, ContentType, StorageProvider, NoteMetadataItem, NoteTypeConfig }
