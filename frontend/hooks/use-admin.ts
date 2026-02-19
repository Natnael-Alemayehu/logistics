import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '@/lib/api-client'
import { API_ENDPOINTS } from '@/lib/constants'
import { toast } from 'sonner'
import type { User, Tenant, UserRole } from '@/types'

interface UsersListResponse {
  users: User[]
  total: number
}

interface TenantsListResponse {
  tenants: Tenant[]
  total: number
}

interface CreateUserInput {
  email: string
  full_name: string
  phone?: string
  role: UserRole
  tenant_id: string
  password?: string
}

interface UpdateUserInput {
  full_name?: string
  email?: string
  phone?: string
  role?: UserRole
  is_active?: boolean
}

interface UpdateTenantInput {
  name?: string
  plan?: string
  max_drivers?: number
  max_vehicles?: number
  is_active?: boolean
}

interface CreateTenantInput {
  name: string
  slug: string
  plan: string
  max_drivers: number
  max_vehicles: number
}

export function useUsers(filters?: { tenant_id?: string; role?: UserRole }) {
  return useQuery({
    queryKey: ['admin-users', filters],
    queryFn: () => {
      const params = new URLSearchParams()
      if (filters?.tenant_id) params.append('tenant_id', filters.tenant_id)
      if (filters?.role) params.append('role', filters.role)
      const query = params.toString() ? `?${params.toString()}` : ''
      return api.get<UsersListResponse>(`${API_ENDPOINTS.users.list}${query}`)
    },
  })
}

export function useCreateUser() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (data: CreateUserInput) =>
      api.post<User>(API_ENDPOINTS.users.list, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin-users'] })
      toast.success('User created successfully')
    },
    onError: (error: Error) => {
      toast.error(error.message || 'Failed to create user')
    },
  })
}

export function useUpdateUser() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: UpdateUserInput }) =>
      api.put<User>(API_ENDPOINTS.users.update(id), data),
    onSuccess: (_, { id }) => {
      queryClient.invalidateQueries({ queryKey: ['admin-users'] })
      queryClient.invalidateQueries({ queryKey: ['user', id] })
      toast.success('User updated successfully')
    },
    onError: (error: Error) => {
      toast.error(error.message || 'Failed to update user')
    },
  })
}

export function useTenants() {
  return useQuery({
    queryKey: ['admin-tenants'],
    queryFn: () => api.get<TenantsListResponse>(API_ENDPOINTS.tenants.list),
  })
}

export function useTenant(id: string) {
  return useQuery({
    queryKey: ['admin-tenant', id],
    queryFn: () => api.get<Tenant>(API_ENDPOINTS.tenants.get(id)),
    enabled: !!id,
  })
}

export function useCreateTenant() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (data: CreateTenantInput) =>
      api.post<Tenant>(API_ENDPOINTS.tenants.create, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin-tenants'] })
      toast.success('Tenant created successfully')
    },
    onError: (error: Error) => {
      toast.error(error.message || 'Failed to create tenant')
    },
  })
}

export function useUpdateTenant() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: UpdateTenantInput }) =>
      api.put<Tenant>(API_ENDPOINTS.tenants.update(id), data),
    onSuccess: (_, { id }) => {
      queryClient.invalidateQueries({ queryKey: ['admin-tenants'] })
      queryClient.invalidateQueries({ queryKey: ['admin-tenant', id] })
      toast.success('Tenant updated successfully')
    },
    onError: (error: Error) => {
      toast.error(error.message || 'Failed to update tenant')
    },
  })
}

export function usePlatformStats() {
  return useQuery({
    queryKey: ['admin-platform-stats'],
    queryFn: () =>
      api.get<{
        total_tenants: number
        total_users: number
        total_shipments: number
        active_tenants: number
        total_drivers: number
        total_vehicles: number
      }>('/api/v1/admin/stats'),
  })
}
