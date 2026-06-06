// Generic API Service Factory
// Creates standardized CRUD operations for any resource type

import { httpClient } from './client'

/**
 * Configuration for creating an API service
 */
export interface ApiServiceConfig<TResponse, TEntity> {
  /** Base URL for the service (e.g., API_CONFIG.baseUrls.diary) */
  baseUrl: string
  /** Resource path (e.g., '/diaries', '/records') */
  resourcePath: string
  /** Transform function to convert API response to frontend entity */
  transform: (response: TResponse) => TEntity
}

/**
 * Filter parameters builder - converts filter object to URLSearchParams
 */
export function buildQueryParams(
  filters: Record<string, string | number | boolean | undefined> | undefined,
  fieldMappings?: Record<string, string>
): string {
  if (!filters) return ''

  const params = new URLSearchParams()

  for (const [key, value] of Object.entries(filters)) {
    if (value === undefined) continue

    // Apply field name mapping if provided (e.g., goalTag -> goal_tag)
    const paramName = fieldMappings?.[key] ?? key
    params.set(paramName, String(value))
  }

  const query = params.toString()
  return query ? `?${query}` : ''
}

/**
 * Standard CRUD operations interface
 */
export interface CrudApi<TEntity, TCreateInput, TUpdateInput, TFilter = unknown> {
  list(filters?: TFilter): Promise<TEntity[]>
  get(id: string): Promise<TEntity>
  create(data: TCreateInput): Promise<TEntity>
  update(id: string, data: TUpdateInput): Promise<TEntity>
  delete(id: string): Promise<void>
}

/**
 * Creates a standardized API service with CRUD operations
 *
 * @example
 * ```ts
 * const diariesApi = createApiService<DiaryResponse, DiaryEntry, CreateInput, UpdateInput, DiaryFilter>({
 *   baseUrl: API_CONFIG.baseUrls.diary,
 *   resourcePath: '/diaries',
 *   transform: transformDiaryEntry,
 * })
 * ```
 */
export function createApiService<
  TResponse,
  TEntity,
  TCreateInput,
  TUpdateInput,
  TFilter = Record<string, unknown>
>(
  config: ApiServiceConfig<TResponse, TEntity>,
  options?: {
    /** Field name mappings for query params (e.g., { goalTag: 'goal_tag' }) */
    filterMappings?: Record<string, string>
  }
): CrudApi<TEntity, TCreateInput, TUpdateInput, TFilter> {
  const { baseUrl, resourcePath, transform } = config
  const { filterMappings } = options ?? {}

  return {
    async list(filters?: TFilter): Promise<TEntity[]> {
      const query = buildQueryParams(
        filters as Record<string, string | number | boolean | undefined>,
        filterMappings
      )
      const response = await httpClient.get<TResponse[]>(
        baseUrl,
        `${resourcePath}${query}`
      )
      return response.map(transform)
    },

    async get(id: string): Promise<TEntity> {
      const response = await httpClient.get<TResponse>(
        baseUrl,
        `${resourcePath}/${id}`
      )
      return transform(response)
    },

    async create(data: TCreateInput): Promise<TEntity> {
      const response = await httpClient.post<TResponse>(
        baseUrl,
        resourcePath,
        data
      )
      return transform(response)
    },

    async update(id: string, data: TUpdateInput): Promise<TEntity> {
      const response = await httpClient.put<TResponse>(
        baseUrl,
        `${resourcePath}/${id}`,
        data
      )
      return transform(response)
    },

    async delete(id: string): Promise<void> {
      await httpClient.delete(baseUrl, `${resourcePath}/${id}`)
    },
  }
}

/**
 * Extends a base CRUD API with additional custom methods
 *
 * @example
 * ```ts
 * const diariesApi = extendApiService(baseDiariesApi, (api) => ({
 *   getByDate: async (date: string) => { ... }
 * }))
 * ```
 */
export function extendApiService<
  TBase extends CrudApi<unknown, unknown, unknown, unknown>,
  TExtension extends Record<string, unknown>
>(
  base: TBase,
  extension: (base: TBase) => TExtension
): TBase & TExtension {
  return {
    ...base,
    ...extension(base),
  }
}
