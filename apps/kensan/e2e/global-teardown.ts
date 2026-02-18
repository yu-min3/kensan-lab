import { getAuthToken, apiRequest, deleteNote } from './helpers/api'

const NOTE_SERVICE_URL = 'http://localhost:8091'

export default async function globalTeardown() {
  const token = getAuthToken()
  if (!token) {
    console.log('[teardown] No auth token found, skipping cleanup')
    return
  }

  // Fetch all notes
  const res = await apiRequest(`${NOTE_SERVICE_URL}/api/v1/notes`)
  if (!res.ok) {
    console.log(`[teardown] Failed to fetch notes: ${res.status}`)
    return
  }

  const json = await res.json()
  const notes: { id: string; title: string }[] = json.data ?? []

  // Filter notes with [E2E] prefix
  const e2eNotes = notes.filter((n) => n.title?.startsWith('[E2E]'))

  if (e2eNotes.length === 0) {
    console.log('[teardown] No E2E notes to clean up')
    return
  }

  console.log(`[teardown] Cleaning up ${e2eNotes.length} E2E note(s)...`)
  for (const note of e2eNotes) {
    await deleteNote(note.id)
    console.log(`[teardown] Deleted: ${note.title}`)
  }
}
