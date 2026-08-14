import * as SecureStore from 'expo-secure-store'
import { API_BASE_URL, API_ENDPOINTS } from '../constants'

interface FetchOptions extends RequestInit {
  token?: string
  _retry?: boolean
}

export class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
    public code?: string
  ) {
    super(message)
    this.name = 'ApiError'
  }
}

export class ApiClient {
  private baseUrl: string

  constructor(baseUrl: string) {
    this.baseUrl = baseUrl
  }

  private async getToken(): Promise<string | null> {
    return SecureStore.getItemAsync('access_token')
  }

  private async getRefreshToken(): Promise<string | null> {
    return SecureStore.getItemAsync('refresh_token')
  }

  private async refreshAccessToken(): Promise<string | null> {
    const refreshToken = await this.getRefreshToken()
    if (!refreshToken) return null

    try {
      const response = await fetch(`${this.baseUrl}${API_ENDPOINTS.auth.refresh}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ refresh_token: refreshToken }),
      })

      if (!response.ok) return null

      const data = await response.json()
      const newAccessToken = data.data?.access_token ?? data.access_token
      const newRefreshToken = data.data?.refresh_token ?? data.refresh_token

      if (newAccessToken) {
        await SecureStore.setItemAsync('access_token', newAccessToken)
        if (newRefreshToken) {
          await SecureStore.setItemAsync('refresh_token', newRefreshToken)
        }
        return newAccessToken
      }

      return null
    } catch {
      return null
    }
  }

  async request<T>(endpoint: string, options: FetchOptions = {}): Promise<T> {
    const { token, _retry, ...fetchOptions } = options
    const authToken = token || (await this.getToken())

    const headers: HeadersInit = {
      'Content-Type': 'application/json',
      ...(authToken && { Authorization: `Bearer ${authToken}` }),
      ...options.headers,
    }

    const response = await fetch(`${this.baseUrl}${endpoint}`, {
      ...fetchOptions,
      headers,
    })

    if (response.status === 401 && !_retry) {
      const newToken = await this.refreshAccessToken()

      if (newToken) {
        return this.request<T>(endpoint, {
          ...options,
          token: newToken,
          _retry: true,
        })
      }
    }

    if (!response.ok) {
      const error = await response.json().catch(() => ({}))
      throw new ApiError(
        response.status,
        error.error?.message || error.message || 'Request failed',
        error.error?.code || error.code
      )
    }

    const data = await response.json()
    return data.data ?? data
  }

  get<T>(endpoint: string, options?: FetchOptions): Promise<T> {
    return this.request<T>(endpoint, { ...options, method: 'GET' })
  }

  post<T>(endpoint: string, body: unknown, options?: FetchOptions): Promise<T> {
    return this.request<T>(endpoint, {
      ...options,
      method: 'POST',
      body: JSON.stringify(body),
    })
  }

  put<T>(endpoint: string, body: unknown, options?: FetchOptions): Promise<T> {
    return this.request<T>(endpoint, {
      ...options,
      method: 'PUT',
      body: JSON.stringify(body),
    })
  }

  patch<T>(endpoint: string, body: unknown, options?: FetchOptions): Promise<T> {
    return this.request<T>(endpoint, {
      ...options,
      method: 'PATCH',
      body: JSON.stringify(body),
    })
  }

  delete<T>(endpoint: string, options?: FetchOptions): Promise<T> {
    return this.request<T>(endpoint, { ...options, method: 'DELETE' })
  }
}

export const api = new ApiClient(API_BASE_URL)
