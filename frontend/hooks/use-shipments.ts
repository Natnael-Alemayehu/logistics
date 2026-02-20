import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '@/lib/api-client'
import { API_ENDPOINTS } from '@/lib/constants'
import { toast } from 'sonner'
import { useOfflineSync } from './use-offline-sync'
import { offlineStorage } from '@/lib/offline-storage'
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
    queryFn: async () => {
      try {
        const response = await api.get<{ data: Shipment[]; meta: { total: number; page: number; per_page: number } }>(endpoint)
        const shipments = response.data ?? (response as unknown as Shipment[])
        const meta = response.meta ?? { total: shipments.length, page: 1, per_page: 10 }
        await offlineStorage.setShipments(shipments)
        return { shipments, ...meta }
      } catch (error) {
        const cached = await offlineStorage.getShipments()
        if (cached.length > 0) {
          return { shipments: cached, total: cached.length, page: 1, per_page: 10 }
        }
        throw error
      }
    },
  })
}

export function useShipment(id: string) {
  return useQuery({
    queryKey: ['shipment', id],
    queryFn: async () => {
      try {
        const data = await api.get<Shipment>(API_ENDPOINTS.shipments.get(id))
        await offlineStorage.setShipment(data)
        return data
      } catch (error) {
        const cached = await offlineStorage.getShipment(id)
        if (cached) return cached
        throw error
      }
    },
    enabled: !!id,
  })
}

export function useCreateShipment() {
  const queryClient = useQueryClient()
  const { addToQueue } = useOfflineSync()

  return useMutation({
    mutationFn: async (data: CreateShipmentInput) => {
      if (!navigator.onLine) {
        const tempId = `temp-${Date.now()}`
        const tempShipment: Shipment = {
          id: tempId,
          tenant_id: '',
          tracking_number: `TEMP-${Date.now()}`,
          origin_address: data.origin_address,
          destination_address: data.destination_address,
          customer_name: data.customer_name,
          customer_phone: data.customer_phone,
          cargo_description: data.cargo_description,
          status: 'pending',
          created_at: new Date().toISOString(),
          updated_at: new Date().toISOString(),
        }
        await addToQueue('create_shipment', API_ENDPOINTS.shipments.create, 'POST', data)
        return tempShipment
      }
      return api.post<Shipment>(API_ENDPOINTS.shipments.create, data)
    },
    onSuccess: (data) => {
      queryClient.invalidateQueries({ queryKey: ['shipments'] })
      toast.success(navigator.onLine ? 'Shipment created successfully' : 'Shipment queued for sync')
    },
    onError: (error: Error) => {
      toast.error(error.message || 'Failed to create shipment')
    },
  })
}

export function useUpdateShipment() {
  const queryClient = useQueryClient()
  const { addToQueue } = useOfflineSync()

  return useMutation({
    mutationFn: async ({ id, data }: { id: string; data: UpdateShipmentInput }) => {
      if (!navigator.onLine) {
        await addToQueue('update_shipment', API_ENDPOINTS.shipments.update(id), 'PUT', data)
        return { id, ...data } as Shipment
      }
      return api.put<Shipment>(API_ENDPOINTS.shipments.update(id), data)
    },
    onSuccess: (_, { id }) => {
      queryClient.invalidateQueries({ queryKey: ['shipment', id] })
      queryClient.invalidateQueries({ queryKey: ['shipments'] })
      toast.success(navigator.onLine ? 'Shipment updated successfully' : 'Update queued for sync')
    },
    onError: (error: Error) => {
      toast.error(error.message || 'Failed to update shipment')
    },
  })
}

export function useCancelShipment() {
  const queryClient = useQueryClient()
  const { addToQueue } = useOfflineSync()

  return useMutation({
    mutationFn: async ({ id, reason }: { id: string; reason: string }) => {
      if (!navigator.onLine) {
        await addToQueue('delete_shipment', API_ENDPOINTS.shipments.cancel(id), 'POST', { reason })
        return { id, status: 'cancelled' }
      }
      return api.post(API_ENDPOINTS.shipments.cancel(id), { reason })
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['shipments'] })
      toast.success(navigator.onLine ? 'Shipment cancelled successfully' : 'Cancellation queued for sync')
    },
    onError: (error: Error) => {
      toast.error(error.message || 'Failed to cancel shipment')
    },
  })
}

export function useAssignDriver() {
  const queryClient = useQueryClient()
  const { addToQueue } = useOfflineSync()

  return useMutation({
    mutationFn: async ({
      shipmentId,
      driverId,
    }: {
      shipmentId: string
      driverId: string
    }) => {
      if (!navigator.onLine) {
        await addToQueue('assign_driver', API_ENDPOINTS.shipments.assignDriver(shipmentId), 'PUT', { driver_id: driverId })
        return { id: shipmentId, driver_id: driverId, status: 'assigned' } as Shipment
      }
      return api.put<Shipment>(API_ENDPOINTS.shipments.assignDriver(shipmentId), {
        driver_id: driverId,
      })
    },
    onSuccess: (_, { shipmentId }) => {
      queryClient.invalidateQueries({ queryKey: ['shipment', shipmentId] })
      queryClient.invalidateQueries({ queryKey: ['shipments'] })
      toast.success(navigator.onLine ? 'Driver assigned successfully' : 'Assignment queued for sync')
    },
    onError: (error: Error) => {
      toast.error(error.message || 'Failed to assign driver')
    },
  })
}

export function useUpdateShipmentStatus() {
  const queryClient = useQueryClient()
  const { addToQueue } = useOfflineSync()

  return useMutation({
    mutationFn: async ({
      shipmentId,
      data,
    }: {
      shipmentId: string
      data: UpdateStatusInput
    }) => {
      if (!navigator.onLine) {
        await addToQueue('update_status', API_ENDPOINTS.shipments.updateStatus(shipmentId), 'PUT', data)
        return { id: shipmentId, status: data.status } as Shipment
      }
      return api.put<Shipment>(API_ENDPOINTS.shipments.updateStatus(shipmentId), data)
    },
    onSuccess: (_, { shipmentId }) => {
      queryClient.invalidateQueries({ queryKey: ['shipment', shipmentId] })
      queryClient.invalidateQueries({ queryKey: ['shipments'] })
      toast.success(navigator.onLine ? 'Status updated successfully' : 'Status update queued for sync')
    },
    onError: (error: Error) => {
      toast.error(error.message || 'Failed to update status')
    },
  })
}

export function useTrackingEvents(shipmentId: string) {
  return useQuery({
    queryKey: ['tracking-events', shipmentId],
    queryFn: async () => {
      const response = await api.get<{ data: TrackingEvent[]; meta?: { total: number } } | { events: TrackingEvent[] }>(
        API_ENDPOINTS.shipments.trackingEvents(shipmentId)
      )
      if ('events' in response) return response
      return { events: response.data }
    },
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
