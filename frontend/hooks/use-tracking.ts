import { useQuery } from '@tanstack/react-query'
import { api } from '@/lib/api-client'
import { API_ENDPOINTS } from '@/lib/constants'
import type { PublicShipment } from '@/types'

export function useTracking(trackingNumber: string) {
  return useQuery({
    queryKey: ['tracking', trackingNumber],
    queryFn: () => api.get<PublicShipment>(API_ENDPOINTS.tracking.track(trackingNumber)),
    enabled: !!trackingNumber && trackingNumber.length >= 10,
    retry: false,
  })
}
