// Memos API Service
import { API_CONFIG } from '../config'
import { httpClient } from '../client'
import { createApiService } from '../createApiService'

// Types
export interface Memo {
  id: string
  userId: string
  content: string
  archived: boolean
  createdAt: string
  updatedAt: string
}

interface MemoResponse {
  id: string
  userId: string
  content: string
  archived: boolean
  createdAt: string
  updatedAt: string
}

const transformMemo = (m: MemoResponse): Memo => ({
  id: m.id,
  userId: m.userId,
  content: m.content,
  archived: m.archived,
  createdAt: m.createdAt,
  updatedAt: m.updatedAt,
})

// Input types
export interface CreateMemoInput {
  content: string
}

export interface UpdateMemoInput {
  content?: string
  archived?: boolean
}

export interface MemoFilter {
  archived?: boolean
  includeAll?: boolean
  date?: string
  limit?: number
}

// Base CRUD API
const baseMemosApi = createApiService<
  MemoResponse,
  Memo,
  CreateMemoInput,
  UpdateMemoInput,
  MemoFilter
>(
  {
    baseUrl: API_CONFIG.baseUrls.memo,
    resourcePath: '/memos',
    transform: transformMemo,
  },
  {
    filterMappings: {
      includeAll: 'include_all',
    },
  }
)

// Extended API with PATCH update and archive method
export const memosApi = {
  // Inherit list, get, create, delete from base
  list: baseMemosApi.list,
  get: baseMemosApi.get,
  create: baseMemosApi.create,
  delete: baseMemosApi.delete,

  // Override update to use PATCH instead of PUT
  async update(id: string, data: UpdateMemoInput): Promise<Memo> {
    const response = await httpClient.patch<MemoResponse>(
      API_CONFIG.baseUrls.memo,
      `/memos/${id}`,
      data
    )
    return transformMemo(response)
  },

  // Archive a memo
  async archive(id: string): Promise<Memo> {
    const response = await httpClient.post<MemoResponse>(
      API_CONFIG.baseUrls.memo,
      `/memos/${id}/archive`,
      {}
    )
    return transformMemo(response)
  },
}
