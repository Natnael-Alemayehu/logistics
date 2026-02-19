'use client'

import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { usePlatformStats, useTenants, useUsers } from '@/hooks'
import { Building2, Users, Package, Truck, Activity } from 'lucide-react'

export default function AdminDashboardPage() {
  const { data: stats, isLoading: statsLoading } = usePlatformStats()
  const { data: tenantsData } = useTenants()
  const { data: usersData } = useUsers()

  const statsCards = [
    {
      title: 'Total Tenants',
      value: stats?.total_tenants ?? 0,
      icon: Building2,
      color: 'text-blue-600',
      bgColor: 'bg-blue-50 dark:bg-blue-950',
    },
    {
      title: 'Total Users',
      value: stats?.total_users ?? 0,
      icon: Users,
      color: 'text-green-600',
      bgColor: 'bg-green-50 dark:bg-green-950',
    },
    {
      title: 'Total Shipments',
      value: stats?.total_shipments ?? 0,
      icon: Package,
      color: 'text-purple-600',
      bgColor: 'bg-purple-50 dark:bg-purple-950',
    },
    {
      title: 'Active Tenants',
      value: stats?.active_tenants ?? 0,
      icon: Activity,
      color: 'text-orange-600',
      bgColor: 'bg-orange-50 dark:bg-orange-950',
    },
  ]

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold">Platform Admin Dashboard</h1>
        <p className="text-muted-foreground">
          Overview of the Ethiopian Logistics Platform
        </p>
      </div>

      {statsLoading ? (
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
          {[...Array(4)].map((_, i) => (
            <Card key={i}>
              <CardHeader className="flex flex-row items-center justify-between pb-2">
                <div className="h-4 w-24 animate-pulse rounded bg-muted"></div>
                <div className="h-8 w-8 animate-pulse rounded bg-muted"></div>
              </CardHeader>
              <CardContent>
                <div className="h-8 w-16 animate-pulse rounded bg-muted"></div>
              </CardContent>
            </Card>
          ))}
        </div>
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
              <Building2 className="h-5 w-5 text-blue-500" />
              Recent Tenants
            </CardTitle>
          </CardHeader>
          <CardContent>
            {tenantsData?.tenants?.length === 0 ? (
              <p className="text-sm text-muted-foreground">No tenants found</p>
            ) : (
              <div className="space-y-3">
                {tenantsData?.tenants?.slice(0, 5).map((tenant) => (
                  <div
                    key={tenant.id}
                    className="flex items-center justify-between rounded-lg border p-3"
                  >
                    <div>
                      <p className="text-sm font-medium">{tenant.name}</p>
                      <p className="text-xs text-muted-foreground">
                        Plan: {tenant.plan}
                      </p>
                    </div>
                    <span
                      className={`text-xs px-2 py-1 rounded ${
                        tenant.is_active
                          ? 'bg-green-100 text-green-800 dark:bg-green-950 dark:text-green-200'
                          : 'bg-red-100 text-red-800 dark:bg-red-950 dark:text-red-200'
                      }`}
                    >
                      {tenant.is_active ? 'Active' : 'Inactive'}
                    </span>
                  </div>
                ))}
              </div>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Users className="h-5 w-5 text-green-500" />
              Platform Stats
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="space-y-4">
              <div className="flex items-center justify-between">
                <span className="text-sm text-muted-foreground">Total Drivers</span>
                <span className="font-semibold">{stats?.total_drivers ?? 0}</span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-sm text-muted-foreground">Total Vehicles</span>
                <span className="font-semibold">{stats?.total_vehicles ?? 0}</span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-sm text-muted-foreground">Active Users</span>
                <span className="font-semibold">{usersData?.users?.filter(u => u.is_active).length ?? 0}</span>
              </div>
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  )
}