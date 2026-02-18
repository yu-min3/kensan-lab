// Memos MSW handlers
import { http, HttpResponse } from 'msw'
import { memos, generateId, type MockMemo } from '../data'

const BASE_URL = 'http://localhost:8090/api/v1'

// Transform to API response format
const toMemoResponse = (m: MockMemo) => ({
  id: m.id,
  userId: m.userId,
  content: m.content,
  archived: m.archived,
  createdAt: m.createdAt.toISOString(),
  updatedAt: m.updatedAt.toISOString(),
})

export const memoHandlers = [
  // GET /memos - List memos
  http.get(`${BASE_URL}/memos`, ({ request }) => {
    const url = new URL(request.url)
    const includeAll = url.searchParams.get('include_all') === 'true'
    const date = url.searchParams.get('date')

    let result = [...memos]

    // Filter by archived status (unless includeAll is true)
    if (!includeAll) {
      result = result.filter((m) => !m.archived)
    }

    // Filter by date if provided
    if (date) {
      result = result.filter((m) => {
        const memoDate = m.createdAt.toISOString().split('T')[0]
        return memoDate === date
      })
    }

    // Sort by createdAt descending (newest first)
    result.sort((a, b) => b.createdAt.getTime() - a.createdAt.getTime())

    return HttpResponse.json(result.map(toMemoResponse))
  }),

  // GET /memos/:id - Get single memo
  http.get(`${BASE_URL}/memos/:id`, ({ params }) => {
    const memo = memos.find((m) => m.id === params.id)
    if (!memo) {
      return HttpResponse.json(
        { code: 'NOT_FOUND', message: 'Memo not found' },
        { status: 404 }
      )
    }
    return HttpResponse.json(toMemoResponse(memo))
  }),

  // POST /memos - Create memo
  http.post(`${BASE_URL}/memos`, async ({ request }) => {
    const body = (await request.json()) as { content: string }
    const now = new Date()
    const newMemo: MockMemo = {
      id: generateId('memo-'),
      userId: 'user-1',
      content: body.content,
      archived: false,
      createdAt: now,
      updatedAt: now,
    }
    memos.unshift(newMemo) // Add to beginning (newest first)
    return HttpResponse.json(toMemoResponse(newMemo), { status: 201 })
  }),

  // PATCH /memos/:id - Update memo
  http.patch(`${BASE_URL}/memos/:id`, async ({ params, request }) => {
    const index = memos.findIndex((m) => m.id === params.id)
    if (index === -1) {
      return HttpResponse.json(
        { code: 'NOT_FOUND', message: 'Memo not found' },
        { status: 404 }
      )
    }
    const body = (await request.json()) as { content?: string; archived?: boolean }
    memos[index] = {
      ...memos[index],
      ...body,
      updatedAt: new Date(),
    }
    return HttpResponse.json(toMemoResponse(memos[index]))
  }),

  // POST /memos/:id/archive - Archive memo
  http.post(`${BASE_URL}/memos/:id/archive`, ({ params }) => {
    const index = memos.findIndex((m) => m.id === params.id)
    if (index === -1) {
      return HttpResponse.json(
        { code: 'NOT_FOUND', message: 'Memo not found' },
        { status: 404 }
      )
    }
    memos[index] = {
      ...memos[index],
      archived: true,
      updatedAt: new Date(),
    }
    return HttpResponse.json(toMemoResponse(memos[index]))
  }),

  // DELETE /memos/:id - Delete memo
  http.delete(`${BASE_URL}/memos/:id`, ({ params }) => {
    const index = memos.findIndex((m) => m.id === params.id)
    if (index === -1) {
      return HttpResponse.json(
        { code: 'NOT_FOUND', message: 'Memo not found' },
        { status: 404 }
      )
    }
    memos.splice(index, 1)
    return new HttpResponse(null, { status: 204 })
  }),
]
