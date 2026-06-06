import fs from 'node:fs'
import path from 'node:path'

const AUTH_FILE = path.join(import.meta.dirname, '..', '.auth', 'user.json')

/** Extract JWT token from the storageState file */
export function getAuthToken(): string {
  const raw = fs.readFileSync(AUTH_FILE, 'utf-8')
  const state = JSON.parse(raw)

  // Look for the Zustand auth store in localStorage
  for (const origin of state.origins ?? []) {
    for (const item of origin.localStorage ?? []) {
      if (item.name === 'kensan-auth') {
        const parsed = JSON.parse(item.value)
        return parsed?.state?.token ?? ''
      }
    }
  }
  return ''
}

/** Make an authenticated API request */
export async function apiRequest(
  url: string,
  options: RequestInit = {}
): Promise<Response> {
  const token = getAuthToken()
  return fetch(url, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${token}`,
      ...(options.headers ?? {}),
    },
  })
}

/** Create a note via API and return it */
export async function createNote(data: {
  type: string
  title: string
  content: string
  format?: string
  date?: string
}) {
  const res = await apiRequest('http://localhost:8091/api/v1/notes', {
    method: 'POST',
    body: JSON.stringify({
      type: data.type,
      title: data.title,
      content: data.content,
      format: data.format ?? 'markdown',
      date: data.date,
    }),
  })
  const json = await res.json()
  return json.data
}

/** Delete a note via API */
export async function deleteNote(id: string) {
  await apiRequest(`http://localhost:8091/api/v1/notes/${id}`, {
    method: 'DELETE',
  })
}

/** Update a milestone via API */
export async function updateMilestone(id: string, data: {
  name?: string
  description?: string
  startDate?: string | null
  targetDate?: string | null
  status?: string
}) {
  const body: Record<string, unknown> = {}
  if (data.name !== undefined) body.name = data.name
  if (data.description !== undefined) body.description = data.description
  if (data.startDate !== undefined) body.start_date = data.startDate
  if (data.targetDate !== undefined) body.target_date = data.targetDate
  if (data.status !== undefined) body.status = data.status

  const res = await apiRequest(`http://localhost:8082/api/v1/milestones/${id}`, {
    method: 'PUT',
    body: JSON.stringify(body),
  })
  const json = await res.json()
  return json.data
}

/** List milestones via API */
export async function listMilestones() {
  const res = await apiRequest('http://localhost:8082/api/v1/milestones')
  const json = await res.json()
  return json.data as Array<{ id: string; name: string; start_date?: string; target_date?: string; status: string }>
}

/** List note contents via API */
export async function listNoteContents(noteId: string) {
  const res = await apiRequest(`http://localhost:8091/api/v1/notes/${noteId}/contents`)
  const json = await res.json()
  return json.data as Array<{ id: string; noteId: string; contentType: string; content?: string; sortOrder: number }>
}

/** Create note content via API */
export async function createNoteContent(noteId: string, data: {
  contentType: string
  content?: string
  sortOrder?: number
}) {
  const res = await apiRequest(`http://localhost:8091/api/v1/notes/${noteId}/contents`, {
    method: 'POST',
    body: JSON.stringify(data),
  })
  const json = await res.json()
  return json.data as { id: string; noteId: string; contentType: string; content?: string; sortOrder: number }
}

/** Delete note content via API */
export async function deleteNoteContent(noteId: string, contentId: string) {
  await apiRequest(`http://localhost:8091/api/v1/notes/${noteId}/contents/${contentId}`, {
    method: 'DELETE',
  })
}
