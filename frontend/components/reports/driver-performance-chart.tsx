'use client'

import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  Cell,
} from 'recharts'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { DriverPerformance } from '@/hooks/use-reports'
import { User } from 'lucide-react'

interface DriverPerformanceChartProps {
  data: DriverPerformance[]
  isLoading: boolean
}

function getPerformanceColor(rate: number): string {
  if (rate >= 90) return 'hsl(142 76% 36%)'
  if (rate >= 75) return 'hsl(38 92% 50%)'
  return 'hsl(0 84% 60%)'
}

export function DriverPerformanceChart({ data, isLoading }: DriverPerformanceChartProps) {
  if (isLoading) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>Driver Performance</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="h-[300px] animate-pulse rounded bg-muted" />
        </CardContent>
      </Card>
    )
  }

  const chartData = data.map((driver) => ({
    name: driver.driver_name.split(' ')[0],
    onTimeRate: driver.on_time_rate,
    deliveries: driver.total_deliveries,
  }))

  return (
    <Card>
      <CardHeader>
        <CardTitle>Driver On-Time Rate</CardTitle>
      </CardHeader>
      <CardContent>
        <ResponsiveContainer width="100%" height={300}>
          <BarChart 
            data={chartData} 
            layout="vertical"
            margin={{ top: 20, right: 30, left: 20, bottom: 5 }}
          >
            <CartesianGrid strokeDasharray="3 3" className="stroke-muted" />
            <XAxis 
              type="number" 
              domain={[0, 100]}
              tick={{ fill: 'hsl(var(--muted-foreground))' }}
              tickFormatter={(value) => `${value}%`}
            />
            <YAxis 
              type="category" 
              dataKey="name"
              tick={{ fill: 'hsl(var(--muted-foreground))' }}
              width={80}
            />
            <Tooltip
              formatter={(value) => value !== undefined ? [`${value}%`, 'On-Time Rate'] : ['', 'On-Time Rate']}
              contentStyle={{
                backgroundColor: 'hsl(var(--card))',
                border: '1px solid hsl(var(--border))',
                borderRadius: '8px',
              }}
            />
            <Bar dataKey="onTimeRate" radius={[0, 4, 4, 0]}>
              {chartData.map((entry, index) => (
                <Cell key={`cell-${index}`} fill={getPerformanceColor(entry.onTimeRate)} />
              ))}
            </Bar>
          </BarChart>
        </ResponsiveContainer>

        <div className="mt-6 space-y-3">
          <h4 className="text-sm font-medium text-muted-foreground">Driver Details</h4>
          <div className="space-y-2">
            {data.map((driver) => (
              <div 
                key={driver.driver_id}
                className="flex items-center justify-between rounded-lg border p-3"
              >
                <div className="flex items-center gap-3">
                  <div className="flex h-8 w-8 items-center justify-center rounded-full bg-muted">
                    <User className="h-4 w-4" />
                  </div>
                  <div>
                    <p className="text-sm font-medium">{driver.driver_name}</p>
                    <p className="text-xs text-muted-foreground">
                      {driver.total_deliveries} deliveries
                    </p>
                  </div>
                </div>
                <div className="text-right">
                  <p className="text-sm font-medium">{driver.on_time_rate}%</p>
                  <p className="text-xs text-muted-foreground">on-time</p>
                </div>
              </div>
            ))}
          </div>
        </div>
      </CardContent>
    </Card>
  )
}
