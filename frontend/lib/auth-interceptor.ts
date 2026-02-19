import { useAuthStore } from '@/stores/auth-store'
import { API_ENDPOINTS } from './constants'

const API_BASE = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080'

let isRefreshing = false
let failedQueue: Array<{
  resolve: (token: string) => void
  reject: (error: Error) => void
}> = []

const processQueue = (error: Error | null, token: string | null) => {
  failedQueue.forEach((promise) => {
    if (error) {
      promise.reject(error)
    } else if (token) {
      promise.resolve(token)
    }
  })
  failedQueue = []
}

interface RefreshResponse {
  access_token: string
  refresh_token: string
}

export async function refreshAccessToken(): Promise<string> {
  const refreshToken = useAuthStore.getState().refreshToken

  if (!refreshToken) {
    throw new Error('No refresh token available')
  }

  const response = await fetch(`${API_BASE}${API_ENDPOINTS.auth.refresh}`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ refresh_token: refreshToken }),
  })

  if (!response.ok) {
    throw new Error('Failed to refresh token')
  }

  const data: RefreshResponse = await response.json()
  const { access_token, refresh_token } = data

  useAuthStore.getState().setTokens(access_token, refresh_token)

  return access_token
}

export async function handleTokenRefresh(): Promise<string> {
  if (isRefreshing) {
    return new Promise((resolve, reject) => {
      failedQueue.push({ resolve, reject })
    })
  }

  isRefreshing = true

  try {
    const newToken = await refreshAccessToken()
    processQueue(null, newToken)
    return newToken
  } catch (error) {
    processQueue(error as Error, null)
    useAuthStore.getState().logout()
    if (typeof window !== 'undefined') {
      window.location.href = '/login'
    }
    throw error
  } finally {
    isRefreshing = false
  }
}

export function getRefreshState() {
  return { isRefreshing }
}
