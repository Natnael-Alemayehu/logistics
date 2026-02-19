import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '@/lib/api-client'
import { API_ENDPOINTS } from '@/lib/constants'
import { toast } from 'sonner'
import type { Vehicle, CreateVehicleInput } from '@/types'

export function useVehicles() {
  return useQuery({
    queryKey: ['vehicles'],
    queryFn: () => api.get<{ vehicles: Vehicle[] }>(API_ENDPOINTS.vehicles.list),
  })
}

export function useActiveVehicles() {
  return useQuery({
    queryKey: ['vehicles', 'active'],
    queryFn: () => api.get<{ vehicles: Vehicle[] }>(API_ENDPOINTS.vehicles.active),
  })
}

export function useVehicle(id: string) {
  return useQuery({
    queryKey: ['vehicle', id],
    queryFn: () => api.get<Vehicle>(API_ENDPOINTS.vehicles.get(id)),
    enabled: !!id,
  })
}

export function useCreateVehicle() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (data: CreateVehicleInput) =>
      api.post<Vehicle>(API_ENDPOINTS.vehicles.create, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['vehicles'] })
      toast.success('Vehicle created successfully')
    },
    onError: (error: Error) => {
      toast.error(error.message || 'Failed to create vehicle')
    },
  })
}

export function useUpdateVehicle() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: ({
      id,
      data,
    }: {
      id: string
      data: Partial<CreateVehicleInput>
    }) => api.put<Vehicle>(API_ENDPOINTS.vehicles.update(id), data),
    onSuccess: (_, { id }) => {
      queryClient.invalidateQueries({ queryKey: ['vehicle', id] })
      queryClient.invalidateQueries({ queryKey: ['vehicles'] })
      toast.success('Vehicle updated successfully')
    },
    onError: (error: Error) => {
      toast.error(error.message || 'Failed to update vehicle')
    },
  })
}

export function useDeleteVehicle() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (id: string) => api.delete(API_ENDPOINTS.vehicles.delete(id)),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['vehicles'] })
      toast.success('Vehicle deleted successfully')
    },
    onError: (error: Error) => {
      toast.error(error.message || 'Failed to delete vehicle')
    },
  })
}
