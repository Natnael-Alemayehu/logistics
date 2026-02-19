'use client'

import { useState } from 'react'
import { DeliveryChart, DriverPerformanceChart, StatsSummary } from '@/components/reports'
import { Button } from '@/components/ui/button'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  useDeliveryReport,
  useDriverPerformanceReport,
  useReportsStats,
} from '@/hooks'
import { Download } from 'lucide-react'

type DateRange = '7d' | '30d' | '90d'

export default function ReportsPage() {
  const [dateRange, setDateRange] = useState<DateRange>('30d')

  const { data: deliveryData, isLoading: deliveryLoading } = useDeliveryReport(dateRange)
  const { data: driverData, isLoading: driverLoading } = useDriverPerformanceReport()
  const { data: stats, isLoading: statsLoading } = useReportsStats()

  const handleExportCSV = () => {
    if (!deliveryData) return

    const headers = ['Date', 'Total Deliveries', 'Completed', 'Failed']
    const rows = deliveryData.map((d) => [d.date, d.deliveries, d.completed, d.failed])

    const csvContent = [
      headers.join(','),
      ...rows.map((row) => row.join(',')),
    ].join('\n')

    const blob = new Blob([csvContent], { type: 'text/csv;charset=utf-8;' })
    const link = document.createElement('a')
    link.href = URL.createObjectURL(blob)
    link.download = `delivery-report-${dateRange}.csv`
    link.click()
    URL.revokeObjectURL(link.href)
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-2xl font-bold">Reports & Analytics</h1>
          <p className="text-muted-foreground">
            Overview of logistics operations performance
          </p>
        </div>

        <div className="flex items-center gap-2">
          <Select
            value={dateRange}
            onValueChange={(value: DateRange) => setDateRange(value)}
          >
            <SelectTrigger className="w-[150px]">
              <SelectValue placeholder="Select range" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="7d">Last 7 days</SelectItem>
              <SelectItem value="30d">Last 30 days</SelectItem>
              <SelectItem value="90d">Last 3 months</SelectItem>
            </SelectContent>
          </Select>

          <Button variant="outline" onClick={handleExportCSV}>
            <Download className="mr-2 h-4 w-4" />
            Export CSV
          </Button>
        </div>
      </div>

      <StatsSummary stats={stats} isLoading={statsLoading} />

      <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
        <DeliveryChart data={deliveryData ?? []} isLoading={deliveryLoading} />
        <DriverPerformanceChart data={driverData ?? []} isLoading={driverLoading} />
      </div>
    </div>
  )
}
