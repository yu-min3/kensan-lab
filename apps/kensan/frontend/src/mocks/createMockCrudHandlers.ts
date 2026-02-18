// Generic MSW CRUD Handler Factory
// Creates standardized mock handlers with CRUD operations

import { http, HttpResponse, HttpHandler, JsonBodyType } from 'msw'
import { generateId } from './data'

/**
 * Configuration for creating CRUD handlers
 */
export interface MockCrudConfig<T, TResponse> {
  /** Base URL for the service (e.g., 'http://localhost:8087/api/v1') */
  baseUrl: string
  /** Resource path (e.g., '/diaries', '/records') */
  resourcePath: string
  /** Transform function to convert entity to API response */
  transform: (item: T) => TResponse
  /** Data array reference (mutable) */
  data: T[]
  /** Function to get the ID from an entity */
  getId: (item: T) => string
  /** ID prefix for generateId (e.g., 'd' for diary, 'lr' for learning record) */
  idPrefix: string
  /** Resource name for error messages */
  resourceName: string
  /** Whether to prepend new items (default: false = append) */
  prependOnAdd?: boolean
}

/**
 * Filter handler configuration
 */
export interface FilterConfig {
  /** Query parameter name (e.g., 'project_id') */
  paramName: string
  /** Entity field name (e.g., 'projectId') */
  fieldName: string
  /** Comparison type */
  type?: 'equals' | 'includes' | 'boolean' | 'search'
  /** For search type: array of fields to search in */
  searchFields?: string[]
}

/**
 * Creates standard CRUD handlers for a resource
 *
 * @example
 * ```ts
 * const handlers = createMockCrudHandlers({
 *   baseUrl: BASE_URL,
 *   resourcePath: '/diaries',
 *   transform: toDiaryResponse,
 *   data: diaryEntries,
 *   getId: (d) => d.id,
 *   idPrefix: 'd',
 *   resourceName: 'Diary entry',
 * }, {
 *   filters: [
 *     { paramName: 'tag', fieldName: 'tags', type: 'includes' },
 *     { paramName: 'q', fieldName: '', type: 'search', searchFields: ['title', 'content'] },
 *   ],
 * })
 * ```
 */
export function createMockCrudHandlers<T extends { id: string }, TResponse extends JsonBodyType>(
  config: MockCrudConfig<T, TResponse>,
  options?: {
    /** Filter configurations for the list endpoint */
    filters?: FilterConfig[]
    /** Skip generating the GET single item handler */
    skipGetById?: boolean
  }
): HttpHandler[] {
  const {
    baseUrl,
    resourcePath,
    transform,
    data,
    getId,
    idPrefix,
    resourceName,
    prependOnAdd = false,
  } = config
  const { filters = [], skipGetById = false } = options ?? {}

  const handlers: HttpHandler[] = []
  const fullUrl = `${baseUrl}${resourcePath}`

  // GET list
  handlers.push(
    http.get(fullUrl, ({ request }) => {
      const url = new URL(request.url)
      let result = [...data]

      // Apply filters
      for (const filter of filters) {
        const paramValue = url.searchParams.get(filter.paramName)
        if (paramValue === null) continue

        switch (filter.type) {
          case 'boolean':
            result = result.filter(
              (item) => (item as Record<string, unknown>)[filter.fieldName] === (paramValue === 'true')
            )
            break
          case 'includes':
            result = result.filter((item) => {
              const fieldValue = (item as Record<string, unknown>)[filter.fieldName]
              return Array.isArray(fieldValue) && fieldValue.includes(paramValue)
            })
            break
          case 'search':
            if (filter.searchFields) {
              const q = paramValue.toLowerCase()
              result = result.filter((item) =>
                filter.searchFields!.some((field) => {
                  const value = (item as Record<string, unknown>)[field]
                  return typeof value === 'string' && value.toLowerCase().includes(q)
                })
              )
            }
            break
          case 'equals':
          default:
            result = result.filter((item) => (item as Record<string, unknown>)[filter.fieldName] === paramValue)
            break
        }
      }

      return HttpResponse.json(result.map(transform))
    })
  )

  // GET by ID
  if (!skipGetById) {
    handlers.push(
      http.get(`${fullUrl}/:id`, ({ params }) => {
        const item = data.find((d) => getId(d) === params.id)
        if (!item) {
          return HttpResponse.json(
            { code: 'NOT_FOUND', message: `${resourceName} not found` },
            { status: 404 }
          )
        }
        return HttpResponse.json(transform(item))
      })
    )
  }

  // POST create
  handlers.push(
    http.post(fullUrl, async ({ request }) => {
      const body = (await request.json()) as Partial<T>
      const now = new Date()
      const newItem = {
        ...body,
        id: generateId(idPrefix),
        createdAt: now,
        updatedAt: now,
      } as unknown as T

      if (prependOnAdd) {
        data.unshift(newItem)
      } else {
        data.push(newItem)
      }

      return HttpResponse.json(transform(newItem), { status: 201 })
    })
  )

  // PUT update
  handlers.push(
    http.put(`${fullUrl}/:id`, async ({ params, request }) => {
      const index = data.findIndex((d) => getId(d) === params.id)
      if (index === -1) {
        return HttpResponse.json(
          { code: 'NOT_FOUND', message: `${resourceName} not found` },
          { status: 404 }
        )
      }
      const body = (await request.json()) as Partial<T>
      data[index] = {
        ...data[index],
        ...body,
        updatedAt: new Date(),
      } as T
      return HttpResponse.json(transform(data[index]))
    })
  )

  // DELETE
  handlers.push(
    http.delete(`${fullUrl}/:id`, ({ params }) => {
      const index = data.findIndex((d) => getId(d) === params.id)
      if (index === -1) {
        return HttpResponse.json(
          { code: 'NOT_FOUND', message: `${resourceName} not found` },
          { status: 404 }
        )
      }
      data.splice(index, 1)
      return new HttpResponse(null, { status: 204 })
    })
  )

  return handlers
}

/**
 * Creates a toggle handler for a boolean field
 *
 * @example
 * ```ts
 * const toggleHandler = createToggleHandler({
 *   baseUrl: BASE_URL,
 *   resourcePath: '/tasks',
 *   data: tasks,
 *   getId: (t) => t.id,
 *   transform: toTaskResponse,
 *   resourceName: 'Task',
 *   toggleField: 'completed',
 *   toggleEndpoint: 'toggle',
 * })
 * ```
 */
export function createToggleHandler<T extends { id: string }, TResponse extends JsonBodyType>(config: {
  baseUrl: string
  resourcePath: string
  data: T[]
  getId: (item: T) => string
  transform: (item: T) => TResponse
  resourceName: string
  toggleField: keyof T
  toggleEndpoint: string
}): HttpHandler {
  const { baseUrl, resourcePath, data, getId, transform, resourceName, toggleField, toggleEndpoint } = config

  return http.patch(`${baseUrl}${resourcePath}/:id/${toggleEndpoint}`, ({ params }) => {
    const index = data.findIndex((d) => getId(d) === params.id)
    if (index === -1) {
      return HttpResponse.json(
        { code: 'NOT_FOUND', message: `${resourceName} not found` },
        { status: 404 }
      )
    }
    // Toggle the boolean field
    const item = data[index] as Record<string, unknown>
    item[toggleField as string] = !item[toggleField as string]
    return HttpResponse.json(transform(data[index]))
  })
}
