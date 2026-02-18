// Auth & User MSW handlers
import { http, HttpResponse } from 'msw'
import { userSettings } from '../data'

const BASE_URL = 'http://localhost:8081/api/v1'

// Mock user data
const mockUser = {
  id: 'user-1',
  email: 'demo@kensan.app',
  name: 'Yu',
  createdAt: new Date().toISOString(),
  updatedAt: new Date().toISOString(),
}

// Mock token
const MOCK_TOKEN = 'mock-jwt-token-xxxxx'

export const authHandlers = [
  // POST /auth/login
  http.post(`${BASE_URL}/auth/login`, async ({ request }) => {
    const body = await request.json() as { email: string; password: string }

    // Simple validation
    if (!body.email || !body.password) {
      return HttpResponse.json(
        { error: { code: 'INVALID_CREDENTIALS', message: 'Invalid credentials' } },
        { status: 401 }
      )
    }

    return HttpResponse.json({
      data: {
        token: MOCK_TOKEN,
        user: mockUser,
      },
    })
  }),

  // POST /auth/demo-login
  http.post(`${BASE_URL}/auth/demo-login`, async ({ request }) => {
    const body = await request.json() as { persona: string }

    const personaNames: Record<string, string> = {
      tanaka: '田中翔太',
      misaki: '鈴木美咲',
      takuya: '山田拓也',
      aya: '高橋彩',
    }

    const name = personaNames[body.persona] || body.persona

    return HttpResponse.json({
      data: {
        token: MOCK_TOKEN,
        user: {
          ...mockUser,
          email: `${body.persona}@kensan.dev`,
          name,
        },
      },
    })
  }),

  // POST /auth/register
  http.post(`${BASE_URL}/auth/register`, async ({ request }) => {
    const body = await request.json() as { email: string; password: string; name: string }

    return HttpResponse.json({
      data: {
        token: MOCK_TOKEN,
        user: {
          ...mockUser,
          email: body.email,
          name: body.name,
        },
      },
    }, { status: 201 })
  }),

  // GET /users/me
  http.get(`${BASE_URL}/users/me`, () => {
    return HttpResponse.json({
      data: mockUser,
    })
  }),

  // GET /users/me/settings
  http.get(`${BASE_URL}/users/me/settings`, () => {
    return HttpResponse.json({
      data: {
        userId: mockUser.id,
        timezone: userSettings.timezone,
        theme: userSettings.theme,
        isConfigured: true,
        aiEnabled: true,
        aiConsentGiven: true,
      },
    })
  }),

  // PUT /users/me/settings
  http.put(`${BASE_URL}/users/me/settings`, async ({ request }) => {
    const body = await request.json() as Partial<typeof userSettings>
    Object.assign(userSettings, body)

    return HttpResponse.json({
      data: {
        userId: mockUser.id,
        timezone: userSettings.timezone,
        theme: userSettings.theme,
        isConfigured: true,
        aiEnabled: true,
        aiConsentGiven: true,
      },
    })
  }),

  // POST /users/me/ai-consent
  http.post(`${BASE_URL}/users/me/ai-consent`, () => {
    return new HttpResponse(null, { status: 204 })
  }),
]
