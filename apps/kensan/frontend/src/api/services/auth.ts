// Auth API Service
// NOTE: Prefer useAuthStore for all auth operations.
// This module exists only as a thin API helper; token persistence is
// managed exclusively by Zustand persist (key: 'kensan-auth').
import { API_CONFIG } from '../config'
import { httpClient } from '../client'

export interface LoginRequest {
  email: string
  password: string
}

export interface LoginResponse {
  token: string
  user: {
    id: string
    email: string
    name: string
  }
}

export interface RegisterRequest {
  email: string
  password: string
  name: string
}

export const authApi = {
  async login(data: LoginRequest): Promise<LoginResponse> {
    const response = await httpClient.post<LoginResponse>(
      API_CONFIG.baseUrls.user,
      '/auth/login',
      data
    )
    httpClient.setAuthToken(response.token)
    return response
  },

  async register(data: RegisterRequest): Promise<LoginResponse> {
    const response = await httpClient.post<LoginResponse>(
      API_CONFIG.baseUrls.user,
      '/auth/register',
      data
    )
    httpClient.setAuthToken(response.token)
    return response
  },

  logout() {
    httpClient.setAuthToken(null)
  },
}
