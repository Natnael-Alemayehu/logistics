import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '@/lib/api-client'
import { API_ENDPOINTS } from '@/lib/constants'
import { toast } from 'sonner'
import type {
  Shipment,
  ShipmentFilters,
  ShipmentListResponse,
  CreateShipmentInput,
  UpdateShipmentInput,
  UpdateStatusInput,
  TrackingEvent,
  ProofOfDelivery,
} from '@/types'

export function useShipments(filters: ShipmentFilters = {}) {
  const params = new URLSearchParams()
  if (filters.status && filters.status !== 'all') {
    params.set('status', filters.status)
  }
  if (filters.driverId && filters.driverId !== 'all') {
    params.set('driver_id', filters.driverId)
  }
  if (filters.search) {
    params.set('search', filters.search)
  }
  if (filters.page) {
    params.set('page', String(filters.page))
  }
  if (filters.limit) {
    params.set('limit', String(filters.limit))
  }

  const queryString = params.toString()
  const endpoint = queryString
    ? `${API_ENDPOINTS.shipments.list}?${queryString}`
    : API_ENDPOINTS.shipments.list

  return useQuery({
    queryKey: ['shipments', filters],
    queryFn: () => api.get<ShipmentListResponse>(endpoint),
  })
}

export function useShipment(id: string) {
  return useQuery({
    queryKey: ['shipment', id],
    queryFn: () => api.get<Shipment>(API_ENDPOINTS.shipments.get(id)),
    enabled: !!id,
  })
}

export function useCreateShipment() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (data: CreateShipmentInput) =>
      api.post<Shipment>(API_ENDPOINTS.shipments.create, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['shipments'] })
      toast.success('Shipment created successfully')
    },
    onError: (error: Error) => {
      toast.error(error.message || 'Failed to create shipment')
    },
  })
}

export function useUpdateShipment() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: UpdateShipmentInput }) =>
      api.put<Shipment>(API_ENDPOINTS.shipments.update(id), data),
    onSuccess: (_, { id }) => {
      queryClient.invalidateQueries({ queryKey: ['shipment', id] })
      queryClient.invalidateQueries({ queryKey: ['shipments'] })
      toast.success('Shipment updated successfully')
    },
    onError: (error: Error) => {
      toast.error(error.message || 'Failed to update shipment')
    },
  })
}

export function useDeleteShipment() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (id: string) => api.delete(API_ENDPOINTS.shipments.delete(id)),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['shipments'] })
      toast.success('Shipment deleted successfully')
    },
    onError: (error: Error) => {
      toast.error(error.message || 'Failed to delete shipment')
    },
  })
}

export function useAssignDriver() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: ({
      shipmentId,
      driverId,
    }: {
      shipmentId: string
      driverId: string
    }) =>
      api.put<Shipment>(API_ENDPOINTS.shipments.assignDriver(shipmentId), {
        driver_id: driverId,
      }),
    onSuccess: (_, { shipmentId }) => {
      queryClient.invalidateQueries({ queryKey: ['shipment', shipmentId] })
      queryClient.invalidateQueries({ queryKey: ['shipments'] })
      toast.success('Driver assigned successfully')
    },
    onError: (error: Error) => {
      toast.error(error.message || 'Failed to assign driver')
    },
  })
}

export function useUpdateShipmentStatus() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: ({
      shipmentId,
      data,
    }: {
      shipmentId: string
      data: UpdateStatusInput
    }) =>
      api.put<Shipment>(API_ENDPOINTS.shipments.updateStatus(shipmentId), data),
    onSuccess: (_, { shipmentId }) => {
      queryClient.invalidateQueries({ queryKey: ['shipment', shipmentId] })
      queryClient.invalidateQueries({ queryKey: ['shipments'] })
      toast.success('Status updated successfully')
    },
    onError: (error: Error) => {
      toast.error(error.message || 'Failed to update status')
    },
  })
}

export function useTrackingEvents(shipmentId: string) {
  return useQuery({
    queryKey: ['tracking-events', shipmentId],
    queryFn: () =>
      api.get<{ events: TrackingEvent[] }>(
        API_ENDPOINTS.shipments.trackingEvents(shipmentId)
      ),
    enabled: !!shipmentId,
  })
}

export function useProofOfDelivery(shipmentId: string) {
  return useQuery({
    queryKey: ['pod', shipmentId],
    queryFn: () =>
      api.get<ProofOfDelivery>(API_ENDPOINTS.shipments.pod(shipmentId)),
    enabled: !!shipmentId,
  })
}
