// User Settings API Service
import { API_CONFIG } from '../config'
import { httpClient } from '../client'
import type { UserSettings, Theme } from '@/types'

// API Response types (matches backend SettingsResponse)
interface UserSettingsResponse {
  userId: string
  timezone: string
  theme: Theme
  isConfigured: boolean
  aiEnabled: boolean
  aiConsentGiven: boolean
}

interface UserProfileResponse {
  id: string
  email: string
  name: string
  createdAt: string
  updatedAt: string
}

// Transform API response to frontend type
const transformUserSettings = (s: UserSettingsResponse, profile?: UserProfileResponse): UserSettings => ({
  timezone: s.timezone,
  theme: s.theme,
  isConfigured: s.isConfigured,
  userName: profile?.name || '',
})

export interface UpdateSettingsInput {
  timezone?: string
  theme?: Theme
}

export const userApi = {
  async getSettings(): Promise<UserSettings> {
    const [settings, profile] = await Promise.all([
      httpClient.get<UserSettingsResponse>(
        API_CONFIG.baseUrls.user,
        '/users/me/settings'
      ),
      httpClient.get<UserProfileResponse>(
        API_CONFIG.baseUrls.user,
        '/users/me'
      ).catch(() => undefined),
    ])
    return transformUserSettings(settings, profile)
  },

  async updateSettings(data: UpdateSettingsInput): Promise<UserSettings> {
    const settings = await httpClient.put<UserSettingsResponse>(
      API_CONFIG.baseUrls.user,
      '/users/me/settings',
      data
    )
    return transformUserSettings(settings)
  },

  async giveAIConsent(consent: boolean): Promise<void> {
    await httpClient.post(
      API_CONFIG.baseUrls.user,
      '/users/me/ai-consent',
      { consent }
    )
  },
}
