import { handleTokenRefresh } from './auth-interceptor'
import { useAuthStore } from '@/stores/auth-store'

const API_BASE = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080'

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

  private getToken(): string | null {
    if (typeof window === 'undefined') return null
    const stored = localStorage.getItem('auth-storage')
    if (!stored) return null
    try {
      const parsed = JSON.parse(stored)
      return parsed.state?.accessToken || null
    } catch {
      return null
    }
  }

  async request<T>(endpoint: string, options: FetchOptions = {}): Promise<T> {
    const { token, _retry, ...fetchOptions } = options
    const authToken = token || this.getToken()

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
      const refreshToken = useAuthStore.getState().refreshToken
      
      if (refreshToken) {
        try {
          const newToken = await handleTokenRefresh()
          return this.request<T>(endpoint, {
            ...options,
            token: newToken,
            _retry: true,
          })
        } catch {
          throw new ApiError(401, 'Session expired. Please login again.', 'UNAUTHORIZED')
        }
      }
    }

    if (!response.ok) {
      const error = await response.json().catch(() => ({}))
      throw new ApiError(
        response.status,
        error.error?.message || 'Request failed',
        error.error?.code
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

export const api = new ApiClient(API_BASE)
