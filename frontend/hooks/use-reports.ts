import { useQuery } from '@tanstack/react-query'
import { api } from '@/lib/api-client'
import { subDays, format } from 'date-fns'

export interface DeliveryDataPoint {
  date: string
  deliveries: number
  completed: number
  failed: number
}

export interface DriverPerformance {
  driver_id: string
  driver_name: string
  total_deliveries: number
  completed_deliveries: number
  on_time_deliveries: number
  on_time_rate: number
  average_delivery_time: number
}

export interface ReportsStats {
  total_deliveries_month: number
  average_delivery_time_hours: number
  on_time_delivery_rate: number
  total_deliveries_week: number
  pending_deliveries: number
}

type DateRange = '7d' | '30d' | '90d'

function getDateRangeDays(range: DateRange): number {
  switch (range) {
    case '7d':
      return 7
    case '30d':
      return 30
    case '90d':
      return 90
    default:
      return 30
  }
}

function generateMockDeliveryData(days: number): DeliveryDataPoint[] {
  const data: DeliveryDataPoint[] = []
  for (let i = days - 1; i >= 0; i--) {
    const date = subDays(new Date(), i)
    const totalDeliveries = Math.floor(Math.random() * 20) + 10
    data.push({
      date: format(date, 'MMM dd'),
      deliveries: totalDeliveries,
      completed: Math.floor(totalDeliveries * 0.85),
      failed: Math.floor(totalDeliveries * 0.05),
    })
  }
  return data
}

function generateMockDriverPerformance(): DriverPerformance[] {
  const drivers = [
    { id: '1', name: 'Abebe Kebede' },
    { id: '2', name: 'Tigist Haile' },
    { id: '3', name: 'Dawit Amare' },
    { id: '4', name: 'Sara Girma' },
    { id: '5', name: 'Yohannes Tesfaye' },
  ]

  return drivers.map((driver) => {
    const total = Math.floor(Math.random() * 50) + 20
    const completed = Math.floor(total * 0.9)
    const onTime = Math.floor(completed * (0.7 + Math.random() * 0.25))
    return {
      driver_id: driver.id,
      driver_name: driver.name,
      total_deliveries: total,
      completed_deliveries: completed,
      on_time_deliveries: onTime,
      on_time_rate: Math.round((onTime / completed) * 100),
      average_delivery_time: Math.floor(Math.random() * 4) + 2,
    }
  })
}

export function useDeliveryReport(dateRange: DateRange = '30d') {
  const days = getDateRangeDays(dateRange)

  return useQuery({
    queryKey: ['delivery-report', dateRange],
    queryFn: async (): Promise<DeliveryDataPoint[]> => {
      try {
        const data = await api.get<DeliveryDataPoint[]>(
          `/api/v1/reports/deliveries?days=${days}`
        )
        return data
      } catch {
        return generateMockDeliveryData(days)
      }
    },
  })
}

export function useDriverPerformanceReport() {
  return useQuery({
    queryKey: ['driver-performance-report'],
    queryFn: async (): Promise<DriverPerformance[]> => {
      try {
        const data = await api.get<DriverPerformance[]>(
          '/api/v1/reports/drivers/performance'
        )
        return data
      } catch {
        return generateMockDriverPerformance()
      }
    },
  })
}

export function useReportsStats() {
  return useQuery({
    queryKey: ['reports-stats'],
    queryFn: async (): Promise<ReportsStats> => {
      try {
        const stats = await api.get<ReportsStats>('/api/v1/dashboard/stats')
        return {
          total_deliveries_month: stats.total_deliveries_month ?? 0,
          average_delivery_time_hours: stats.average_delivery_time_hours ?? 0,
          on_time_delivery_rate: stats.on_time_delivery_rate ?? 0,
          total_deliveries_week: stats.total_deliveries_week ?? 0,
          pending_deliveries: stats.pending_deliveries ?? 0,
        }
      } catch {
        return {
          total_deliveries_month: 245,
          average_delivery_time_hours: 3.5,
          on_time_delivery_rate: 87,
          total_deliveries_week: 62,
          pending_deliveries: 15,
        }
      }
    },
  })
}
