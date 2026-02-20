import { useQuery } from '@tanstack/react-query'
import { api } from '@/lib/api-client'
import { API_ENDPOINTS } from '@/lib/constants'

interface DashboardStats {
  active_shipments: number
  drivers_on_duty: number
  deliveries_today: number
  issues_count: number
}

interface DashboardAlert {
  id: string
  type: string
  message: string
  shipment_id?: string
  driver_id?: string
  created_at: string
  read: boolean
}

interface ActivityEvent {
  id: string
  type: string
  description: string
  shipment_id?: string
  driver_id?: string
  user_id?: string
  created_at: string
}

export function useDashboardStats() {
  return useQuery({
    queryKey: ['dashboard-stats'],
    queryFn: () => api.get<DashboardStats>(API_ENDPOINTS.dashboard.stats),
    refetchInterval: 30000,
  })
}

export function useDashboardAlerts() {
  return useQuery({
    queryKey: ['dashboard-alerts'],
    queryFn: async () => {
      const response = await api.get<{ alerts: DashboardAlert[] } | { data: { alerts: DashboardAlert[] } }>(API_ENDPOINTS.dashboard.alerts)
      if ('alerts' in response) return response
      return { alerts: response.data.alerts }
    },
    refetchInterval: 60000,
  })
}

export function useRecentActivity() {
  return useQuery({
    queryKey: ['recent-activity'],
    queryFn: async () => {
      const response = await api.get<{ events: ActivityEvent[] } | { data: { events: ActivityEvent[] } }>(API_ENDPOINTS.dashboard.activity)
      if ('events' in response) return response
      return { events: response.data.events }
    },
  })
}
