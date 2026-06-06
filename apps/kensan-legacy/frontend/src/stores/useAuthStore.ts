import { create } from 'zustand'
import { persist } from 'zustand/middleware'
import { httpClient } from '@/api/client'
import { API_CONFIG } from '@/api/config'

interface User {
  id: string
  email: string
  name: string
}

interface AuthState {
  // State
  token: string | null
  user: User | null
  isAuthenticated: boolean
  isLoading: boolean
  error: string | null

  // Actions
  login: (email: string, password: string) => Promise<void>
  demoLogin: (persona: string) => Promise<void>
  register: (email: string, password: string, name: string) => Promise<void>
  logout: () => void
  restoreSession: () => void
  clearError: () => void
}

export const useAuthStore = create<AuthState>()(
  persist(
    (set, get) => ({
      token: null,
      user: null,
      isAuthenticated: false,
      isLoading: false,
      error: null,

      login: async (email: string, password: string) => {
        set({ isLoading: true, error: null })
        try {
          const response = await httpClient.post<{ token: string; user: User }>(
            API_CONFIG.baseUrls.user,
            '/auth/login',
            { email, password }
          )

          httpClient.setAuthToken(response.token)
          set({
            token: response.token,
            user: response.user,
            isAuthenticated: true,
            isLoading: false,
          })
        } catch (error) {
          set({
            error: (error as Error).message || 'ログインに失敗しました',
            isLoading: false,
          })
          throw error
        }
      },

      demoLogin: async (persona: string) => {
        set({ isLoading: true, error: null })
        try {
          const response = await httpClient.post<{ token: string; user: User }>(
            API_CONFIG.baseUrls.user,
            '/auth/demo-login',
            { persona }
          )

          httpClient.setAuthToken(response.token)
          set({
            token: response.token,
            user: response.user,
            isAuthenticated: true,
            isLoading: false,
          })
        } catch (error) {
          set({
            error: (error as Error).message || 'デモログインに失敗しました',
            isLoading: false,
          })
          throw error
        }
      },

      register: async (email: string, password: string, name: string) => {
        set({ isLoading: true, error: null })
        try {
          const response = await httpClient.post<{ token: string; user: User }>(
            API_CONFIG.baseUrls.user,
            '/auth/register',
            { email, password, name }
          )

          httpClient.setAuthToken(response.token)
          set({
            token: response.token,
            user: response.user,
            isAuthenticated: true,
            isLoading: false,
          })
        } catch (error) {
          set({
            error: (error as Error).message || '登録に失敗しました',
            isLoading: false,
          })
          throw error
        }
      },

      logout: () => {
        httpClient.setAuthToken(null)
        set({
          token: null,
          user: null,
          isAuthenticated: false,
          error: null,
        })
      },

      restoreSession: () => {
        const { token } = get()
        if (token) {
          httpClient.setAuthToken(token)
          set({ isAuthenticated: true })
        }
      },

      clearError: () => set({ error: null }),
    }),
    {
      name: 'kensan-auth',
      partialize: (state) => ({
        token: state.token,
        user: state.user,
        isAuthenticated: state.isAuthenticated,
      }),
      onRehydrateStorage: () => (state) => {
        // 永続化されたトークンをhttpClientに設定
        if (state?.token) {
          httpClient.setAuthToken(state.token)
        }
      },
    }
  )
)
