'use client'

import dynamic from 'next/dynamic'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { useDashboardStats, useDashboardAlerts, useRecentActivity } from '@/hooks'
import { useDriverLocationsRealtime } from '@/hooks/use-driver-locations-realtime'
import { DashboardStatsSkeleton } from '@/components/shared'
import { ConnectionStatusIndicator } from '@/components/shared/connection-status'
import { Package, Truck, CheckCircle, AlertTriangle, Clock, MapPin } from 'lucide-react'
import { formatDistanceToNow } from 'date-fns'

const RealtimeActiveMap = dynamic(
  () => import('@/components/dashboard/active-map').then((mod) => mod.RealtimeActiveMap),
  {
    ssr: false,
    loading: () => (
      <div className="flex h-[300px] items-center justify-center rounded-lg border bg-muted">
        <div className="space-y-3 text-center">
          <div className="mx-auto h-12 w-12 animate-pulse rounded-full bg-muted-foreground/20" />
          <p className="text-sm text-muted-foreground">Loading map...</p>
        </div>
      </div>
    ),
  }
)

export default function DashboardPage() {
  const { data: stats, isLoading: statsLoading } = useDashboardStats()
  const { data: alertsData } = useDashboardAlerts()
  const { data: activityData } = useRecentActivity()
  const { connectionStatus, reconnect, lastUpdate } = useDriverLocationsRealtime()

  const statsCards = [
    {
      title: 'Active Shipments',
      value: stats?.active_shipments ?? 0,
      icon: Package,
      color: 'text-blue-600',
      bgColor: 'bg-blue-50 dark:bg-blue-950',
    },
    {
      title: 'Drivers On Duty',
      value: stats?.drivers_on_duty ?? 0,
      icon: Truck,
      color: 'text-green-600',
      bgColor: 'bg-green-50 dark:bg-green-950',
    },
    {
      title: 'Deliveries Today',
      value: stats?.deliveries_today ?? 0,
      icon: CheckCircle,
      color: 'text-purple-600',
      bgColor: 'bg-purple-50 dark:bg-purple-950',
    },
    {
      title: 'Issues',
      value: stats?.issues_count ?? 0,
      icon: AlertTriangle,
      color: 'text-orange-600',
      bgColor: 'bg-orange-50 dark:bg-orange-950',
    },
  ]

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold">Dashboard</h1>
        <p className="text-muted-foreground">
          Overview of your logistics operations
        </p>
      </div>

      {statsLoading ? (
        <DashboardStatsSkeleton />
      ) : (
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
          {statsCards.map((card) => (
            <Card key={card.title}>
              <CardHeader className="flex flex-row items-center justify-between pb-2">
                <CardTitle className="text-sm font-medium text-muted-foreground">
                  {card.title}
                </CardTitle>
                <div className={`rounded-lg p-2 ${card.bgColor}`}>
                  <card.icon className={`h-4 w-4 ${card.color}`} />
                </div>
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold">{card.value}</div>
              </CardContent>
            </Card>
          ))}
        </div>
      )}

      <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <AlertTriangle className="h-5 w-5 text-orange-500" />
              Alerts
            </CardTitle>
          </CardHeader>
          <CardContent>
            {alertsData?.alerts?.length === 0 ? (
              <p className="text-sm text-muted-foreground">No active alerts</p>
            ) : (
              <div className="space-y-3">
                {alertsData?.alerts?.slice(0, 5).map((alert) => (
                  <div
                    key={alert.id}
                    className="flex items-start gap-3 rounded-lg border p-3"
                  >
                    <div className="flex h-8 w-8 items-center justify-center rounded-full bg-orange-100 dark:bg-orange-950">
                      <AlertTriangle className="h-4 w-4 text-orange-600" />
                    </div>
                    <div className="flex-1">
                      <p className="text-sm font-medium">{alert.message}</p>
                      <p className="text-xs text-muted-foreground">
                        {formatDistanceToNow(new Date(alert.created_at), {
                          addSuffix: true,
                        })}
                      </p>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Clock className="h-5 w-5 text-blue-500" />
              Recent Activity
            </CardTitle>
          </CardHeader>
          <CardContent>
            {activityData?.events?.length === 0 ? (
              <p className="text-sm text-muted-foreground">No recent activity</p>
            ) : (
              <div className="space-y-3">
                {activityData?.events?.slice(0, 5).map((event) => (
                  <div
                    key={event.id}
                    className="flex items-start gap-3 rounded-lg border p-3"
                  >
                    <div className="flex h-8 w-8 items-center justify-center rounded-full bg-blue-100 dark:bg-blue-950">
                      <MapPin className="h-4 w-4 text-blue-600" />
                    </div>
                    <div className="flex-1">
                      <p className="text-sm font-medium">{event.description}</p>
                      <p className="text-xs text-muted-foreground">
                        {formatDistanceToNow(new Date(event.created_at), {
                          addSuffix: true,
                        })}
                      </p>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <CardTitle>Active Drivers Map</CardTitle>
            <div className="flex items-center gap-2">
              {lastUpdate && (
                <span className="text-xs text-muted-foreground">
                  Updated {formatDistanceToNow(lastUpdate, { addSuffix: true })}
                </span>
              )}
              <ConnectionStatusIndicator
                status={connectionStatus}
                onReconnect={reconnect}
              />
            </div>
          </div>
        </CardHeader>
        <CardContent>
          <RealtimeActiveMap />
        </CardContent>
      </Card>
    </div>
  )
}
