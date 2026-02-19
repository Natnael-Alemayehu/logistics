import { useQuery } from '@tanstack/react-query'
import { api } from '@/lib/api-client'
import { API_ENDPOINTS } from '@/lib/constants'
import type { Shipment } from '@/types'

export function useTracking(trackingNumber: string) {
  return useQuery({
    queryKey: ['tracking', trackingNumber],
    queryFn: () => api.get<Shipment>(API_ENDPOINTS.tracking.track(trackingNumber)),
    enabled: !!trackingNumber && trackingNumber.length >= 10,
    retry: false,
  })
}
