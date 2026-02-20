import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '@/lib/api-client'
import { API_ENDPOINTS } from '@/lib/constants'
import { toast } from 'sonner'
import type { Driver, DriverLocation, CreateDriverInput } from '@/types'

export function useDrivers() {
  return useQuery({
    queryKey: ['drivers'],
    queryFn: async () => {
      const response = await api.get<{ data: Driver[]; meta?: { total: number; page: number; per_page: number } }>(API_ENDPOINTS.drivers.list)
      const drivers = response.data ?? (response as unknown as Driver[])
      return { users: drivers }
    },
  })
}

export function useDriver(id: string) {
  return useQuery({
    queryKey: ['driver', id],
    queryFn: () => api.get<Driver>(API_ENDPOINTS.drivers.get(id)),
    enabled: !!id,
  })
}

export function useDriverLocations() {
  return useQuery({
    queryKey: ['driver-locations'],
    queryFn: async () => {
      const response = await api.get<DriverLocation[] | { data: DriverLocation[] }>(API_ENDPOINTS.drivers.locations)
      const locations = Array.isArray(response) ? response : (response as { data: DriverLocation[] }).data
      return { locations }
    },
    refetchInterval: 30000,
  })
}

export function useCreateDriver() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (data: CreateDriverInput) =>
      api.post<Driver>(API_ENDPOINTS.drivers.create, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['drivers'] })
      toast.success('Driver created successfully')
    },
    onError: (error: Error) => {
      toast.error(error.message || 'Failed to create driver')
    },
  })
}

export function useUpdateDriver() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: ({
      id,
      data,
    }: {
      id: string
      data: Partial<CreateDriverInput>
    }) => api.put<Driver>(API_ENDPOINTS.drivers.update(id), data),
    onSuccess: (_, { id }) => {
      queryClient.invalidateQueries({ queryKey: ['driver', id] })
      queryClient.invalidateQueries({ queryKey: ['drivers'] })
      toast.success('Driver updated successfully')
    },
    onError: (error: Error) => {
      toast.error(error.message || 'Failed to update driver')
    },
  })
}

export function useDeleteDriver() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (id: string) => api.delete(API_ENDPOINTS.drivers.delete(id)),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['drivers'] })
      toast.success('Driver deleted successfully')
    },
    onError: (error: Error) => {
      toast.error(error.message || 'Failed to delete driver')
    },
  })
}

export function useResetDriverPassword() {
  return useMutation({
    mutationFn: (driverId: string) =>
      api.post(API_ENDPOINTS.drivers.resetPassword(driverId), {}),
    onSuccess: () => {
      toast.success('Password reset successfully')
    },
    onError: (error: Error) => {
      toast.error(error.message || 'Failed to reset password')
    },
  })
}
