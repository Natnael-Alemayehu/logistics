import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useAuthStore } from '@/stores/auth-store'
import { api } from '@/lib/api-client'
import { refreshAccessToken } from '@/lib/auth-interceptor'
import { API_ENDPOINTS } from '@/lib/constants'
import { useRouter } from 'next/navigation'
import { toast } from 'sonner'
import type { User } from '@/types'

interface LoginInput {
  email: string
  password: string
}

interface DriverLoginInput {
  phone: string
  pin: string
}

interface LoginResponse {
  access_token: string
  refresh_token: string
  user: {
    id: string
    tenant_id: string
    role: string
    full_name: string
    email?: string
    phone?: string
  }
}

export function useLogin() {
  const { setUser, setTokens } = useAuthStore()
  const router = useRouter()

  return useMutation({
    mutationFn: (input: LoginInput) =>
      api.post<LoginResponse>(API_ENDPOINTS.auth.login, input),
    onSuccess: ({ access_token, refresh_token, user }) => {
      setTokens(access_token, refresh_token)
      setUser({
        id: user.id,
        tenant_id: user.tenant_id,
        role: user.role as User['role'],
        full_name: user.full_name,
        email: user.email,
        phone: user.phone,
        is_active: true,
        created_at: new Date().toISOString(),
        updated_at: new Date().toISOString(),
      })
      toast.success('Welcome back!')
      router.push('/dashboard')
    },
    onError: (error: Error) => {
      toast.error(error.message || 'Invalid email or password')
    },
  })
}

export function useDriverLogin() {
  const { setUser, setTokens } = useAuthStore()
  const router = useRouter()

  return useMutation({
    mutationFn: (input: DriverLoginInput) =>
      api.post<LoginResponse>(API_ENDPOINTS.auth.driverLogin, input),
    onSuccess: ({ access_token, refresh_token, user }) => {
      setTokens(access_token, refresh_token)
      setUser({
        id: user.id,
        tenant_id: user.tenant_id,
        role: user.role as User['role'],
        full_name: user.full_name,
        email: user.email,
        phone: user.phone,
        is_active: true,
        created_at: new Date().toISOString(),
        updated_at: new Date().toISOString(),
      })
      toast.success('Welcome back!')
      router.push('/dashboard')
    },
    onError: (error: Error) => {
      toast.error(error.message || 'Invalid phone or PIN')
    },
  })
}

export function useLogout() {
  const { logout } = useAuthStore()
  const router = useRouter()

  return useMutation({
    mutationFn: () => api.post(API_ENDPOINTS.auth.logout, {}),
    onSettled: () => {
      logout()
      toast.success('Logged out successfully')
      router.push('/login')
    },
  })
}

export function useCurrentUser() {
  const { user, isAuthenticated, isLoading } = useAuthStore()
  return { user, isAuthenticated, isLoading }
}

export function useSessions() {
  return useQuery({
    queryKey: ['sessions'],
    queryFn: () => api.get(API_ENDPOINTS.auth.sessions),
  })
}

export function useRevokeSession() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (sessionId: string) =>
      api.delete(API_ENDPOINTS.auth.revokeSession(sessionId)),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['sessions'] })
      toast.success('Session revoked')
    },
    onError: (error: Error) => {
      toast.error(error.message || 'Failed to revoke session')
    },
  })
}

export function useRevokeOtherSessions() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: () => api.delete(API_ENDPOINTS.auth.revokeOtherSessions),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['sessions'] })
      toast.success('All other sessions revoked')
    },
    onError: (error: Error) => {
      toast.error(error.message || 'Failed to revoke sessions')
    },
  })
}

export function useRefreshToken() {
  const { logout } = useAuthStore()

  return useMutation({
    mutationFn: async () => {
      const accessToken = await refreshAccessToken()
      return accessToken
    },
    onSuccess: () => {
    },
    onError: () => {
      logout()
      if (typeof window !== 'undefined') {
        window.location.href = '/login'
      }
    },
  })
}
