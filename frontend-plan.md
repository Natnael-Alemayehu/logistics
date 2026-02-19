# Frontend Development Plan
## Ethiopian Logistics Tracking Platform

**Framework**: Next.js 14 (App Router)  
**UI Library**: shadcn/ui + Tailwind CSS  
**State Management**: React Query (TanStack Query) + Zustand  
**Language**: TypeScript  

---

## Technology Stack Summary

| Category | Technology | Rationale |
|----------|------------|-----------|
| Framework | Next.js 14 | SSR/SSG for performance, API routes, App Router |
| UI Components | shadcn/ui | Accessible, customizable, built on Radix UI |
| Styling | Tailwind CSS | Utility-first, small bundle, RTL-ready |
| State (Server) | React Query | Caching, optimistic updates, offline support |
| State (Client) | Zustand | Simple, lightweight, TypeScript-friendly |
| Forms | React Hook Form + Zod | Validation, performance, TypeScript |
| Maps | Leaflet + React Leaflet | Free, offline-capable, lightweight |
| Charts | Recharts | React-native, composable, small bundle |
| Date/Time | date-fns | Modular, tree-shakeable |
| i18n | next-intl | Built for Next.js, ICU message format |

---

## Project Structure

```
frontend/
├── app/
│   ├── [locale]/
│   │   ├── (auth)/
│   │   │   ├── login/
│   │   │   │   └── page.tsx
│   │   │   └── layout.tsx
│   │   ├── (dashboard)/
│   │   │   ├── dashboard/
│   │   │   │   └── page.tsx
│   │   │   ├── shipments/
│   │   │   │   ├── page.tsx
│   │   │   │   ├── [id]/
│   │   │   │   │   └── page.tsx
│   │   │   │   └── new/
│   │   │   │       └── page.tsx
│   │   │   ├── drivers/
│   │   │   │   └── page.tsx
│   │   │   ├── vehicles/
│   │   │   │   └── page.tsx
│   │   │   ├── settings/
│   │   │   │   └── page.tsx
│   │   │   └── layout.tsx
│   │   ├── (admin)/
│   │   │   ├── admin/
│   │   │   │   ├── tenants/
│   │   │   │   │   └── page.tsx
│   │   │   │   └── page.tsx
│   │   │   └── layout.tsx
│   │   ├── track/
│   │   │   └── page.tsx
│   │   └── layout.tsx
│   ├── api/
│   │   └── [...]/
│   └── layout.tsx
├── components/
│   ├── ui/                    # shadcn/ui components
│   ├── layout/
│   │   ├── sidebar.tsx
│   │   ├── header.tsx
│   │   ├── footer.tsx
│   │   └── mobile-nav.tsx
│   ├── dashboard/
│   │   ├── stats-cards.tsx
│   │   ├── active-map.tsx
│   │   ├── recent-activity.tsx
│   │   └── alerts-panel.tsx
│   ├── shipments/
│   │   ├── shipment-list.tsx
│   │   ├── shipment-detail.tsx
│   │   ├── shipment-form.tsx
│   │   ├── status-badge.tsx
│   │   └── tracking-timeline.tsx
│   ├── drivers/
│   │   ├── driver-list.tsx
│   │   ├── driver-card.tsx
│   │   └── driver-form.tsx
│   ├── tracking/
│   │   ├── tracking-form.tsx
│   │   ├── tracking-result.tsx
│   │   └── tracking-map.tsx
│   └── shared/
│       ├── data-table.tsx
│       ├── confirm-dialog.tsx
│       ├── loading-skeleton.tsx
│       └── error-boundary.tsx
├── hooks/
│   ├── use-shipments.ts
│   ├── use-drivers.ts
│   ├── use-tracking.ts
│   ├── use-websocket.ts
│   └── use-auth.ts
├── lib/
│   ├── api-client.ts
│   ├── auth.ts
│   ├── utils.ts
│   └── constants.ts
├── stores/
│   ├── auth-store.ts
│   ├── ui-store.ts
│   └── filter-store.ts
├── types/
│   ├── api.ts
│   ├── shipment.ts
│   ├── driver.ts
│   └── user.ts
├── i18n/
│   ├── config.ts
│   ├── am/
│   │   └── messages.json
│   └── en/
│       └── messages.json
├── public/
│   ├── icons/
│   └── images/
├── middleware.ts
├── next.config.js
├── tailwind.config.ts
├── tsconfig.json
└── package.json
```

---

## Phase 1: Project Setup & Foundation

### 1.1 Initialize Next.js Project

```bash
npx create-next-app@latest frontend --typescript --tailwind --eslint --app --src-dir=false --import-alias="@/*"
cd frontend
```

### 1.2 Install Dependencies

```bash
# Core
npm install @tanstack/react-query zustand

# UI Components
npx shadcn-ui@latest init
npx shadcn-ui@latest add button card input label select dialog table tabs badge dropdown-menu avatar sheet toast form

# Forms & Validation
npm install react-hook-form @hookform/resolvers zod

# Maps
npm install leaflet react-leaflet @types/leaflet

# Charts
npm install recharts

# Date/Time
npm install date-fns

# Internationalization
npm install next-intl

# Utilities
npm install clsx tailwind-merge class-variance-authority
npm install lucide-react

# WebSocket
npm install sockjs-client @types/sockjs-client

# Offline Support
npm install localforage
```

### 1.3 Configure Tailwind

```typescript
// tailwind.config.ts
import type { Config } from 'tailwindcss'

const config: Config = {
  darkMode: ['class'],
  content: [
    './app/**/*.{js,ts,jsx,tsx}',
    './components/**/*.{js,ts,jsx,tsx}',
  ],
  theme: {
    extend: {
      colors: {
        primary: {
          DEFAULT: 'hsl(217, 91%, 60%)',
          foreground: 'hsl(0, 0%, 100%)',
        },
        success: 'hsl(142, 71%, 45%)',
        warning: 'hsl(38, 92%, 50%)',
        danger: 'hsl(0, 84%, 60%)',
      },
    },
  },
  plugins: [require('tailwindcss-animate')],
}
```

### 1.4 API Client Setup

```typescript
// lib/api-client.ts
import { getAuthToken } from './auth'

const API_BASE = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080'

interface FetchOptions extends RequestInit {
  token?: string
}

export class ApiClient {
  private baseUrl: string

  constructor(baseUrl: string) {
    this.baseUrl = baseUrl
  }

  async request<T>(endpoint: string, options: FetchOptions = {}): Promise<T> {
    const { token, ...fetchOptions } = options
    const authToken = token || getAuthToken()

    const headers: HeadersInit = {
      'Content-Type': 'application/json',
      ...(authToken && { Authorization: `Bearer ${authToken}` }),
      ...options.headers,
    }

    const response = await fetch(`${this.baseUrl}${endpoint}`, {
      ...fetchOptions,
      headers,
    })

    if (!response.ok) {
      const error = await response.json().catch(() => ({}))
      throw new ApiError(response.status, error.error?.message || 'Request failed', error.error?.code)
    }

    const data = await response.json()
    return data.data ?? data
  }

  get<T>(endpoint: string, options?: FetchOptions) {
    return this.request<T>(endpoint, { ...options, method: 'GET' })
  }

  post<T>(endpoint: string, body: unknown, options?: FetchOptions) {
    return this.request<T>(endpoint, {
      ...options,
      method: 'POST',
      body: JSON.stringify(body),
    })
  }

  put<T>(endpoint: string, body: unknown, options?: FetchOptions) {
    return this.request<T>(endpoint, {
      ...options,
      method: 'PUT',
      body: JSON.stringify(body),
    })
  }

  delete<T>(endpoint: string, options?: FetchOptions) {
    return this.request<T>(endpoint, { ...options, method: 'DELETE' })
  }
}

export class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
    public code?: string
  ) {
    super(message)
    this.name = 'ApiError'
  }
}

export const api = new ApiClient(API_BASE)
```

### 1.5 React Query Setup

```typescript
// lib/query-provider.tsx
'use client'

import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { ReactQueryDevtools } from '@tanstack/react-query-devtools'
import { useState } from 'react'

export function QueryProvider({ children }: { children: React.ReactNode }) {
  const [queryClient] = useState(
    () =>
      new QueryClient({
        defaultOptions: {
          queries: {
            staleTime: 60 * 1000,
            gcTime: 5 * 60 * 1000,
            retry: (failureCount, error) => {
              if (error instanceof ApiError && error.status === 401) return false
              return failureCount < 3
            },
            refetchOnWindowFocus: false,
          },
        },
      })
  )

  return (
    <QueryClientProvider client={queryClient}>
      {children}
      <ReactQueryDevtools initialIsOpen={false} />
    </QueryClientProvider>
  )
}
```

---

## Phase 2: Authentication & Layout

### 2.1 Auth Store (Zustand)

```typescript
// stores/auth-store.ts
import { create } from 'zustand'
import { persist } from 'zustand/middleware'

interface User {
  id: string
  tenantId: string
  role: string
  fullName: string
  email?: string
  phone?: string
}

interface AuthState {
  user: User | null
  accessToken: string | null
  refreshToken: string | null
  isAuthenticated: boolean
  setUser: (user: User | null) => void
  setTokens: (accessToken: string, refreshToken: string) => void
  logout: () => void
}

export const useAuthStore = create<AuthState>()(
  persist(
    (set) => ({
      user: null,
      accessToken: null,
      refreshToken: null,
      isAuthenticated: false,
      setUser: (user) =>
        set({
          user,
          isAuthenticated: !!user,
        }),
      setTokens: (accessToken, refreshToken) =>
        set({
          accessToken,
          refreshToken,
        }),
      logout: () =>
        set({
          user: null,
          accessToken: null,
          refreshToken: null,
          isAuthenticated: false,
        }),
    }),
    {
      name: 'auth-storage',
      partialize: (state) => ({
        accessToken: state.accessToken,
        refreshToken: state.refreshToken,
      }),
    }
  )
)
```

### 2.2 Auth Hooks

```typescript
// hooks/use-auth.ts
import { useMutation, useQuery } from '@tanstack/react-query'
import { useAuthStore } from '@/stores/auth-store'
import { api } from '@/lib/api-client'
import { useRouter } from 'next/navigation'

interface LoginInput {
  email: string
  password: string
}

interface DriverLoginInput {
  phone: string
  pin: string
}

export function useLogin() {
  const { setUser, setTokens } = useAuthStore()
  const router = useRouter()

  return useMutation({
    mutationFn: (input: LoginInput) =>
      api.post<{ accessToken: string; refreshToken: string; user: any }>('/api/v1/auth/login', input),
    onSuccess: ({ accessToken, refreshToken, user }) => {
      setTokens(accessToken, refreshToken)
      setUser({
        id: user.id,
        tenantId: user.tenant_id,
        role: user.role,
        fullName: user.full_name,
        email: user.email,
      })
      router.push('/dashboard')
    },
  })
}

export function useLogout() {
  const { logout } = useAuthStore()
  const router = useRouter()

  return () => {
    logout()
    router.push('/login')
  }
}

export function useCurrentUser() {
  const { user, isAuthenticated } = useAuthStore()
  return { user, isAuthenticated }
}
```

### 2.3 Login Page

```typescript
// app/[locale]/(auth)/login/page.tsx
'use client'

import { useLogin } from '@/hooks/use-auth'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { useToast } from '@/components/ui/use-toast'

const loginSchema = z.object({
  email: z.string().email('Invalid email'),
  password: z.string().min(8, 'Password must be at least 8 characters'),
})

type LoginFormData = z.infer<typeof loginSchema>

export default function LoginPage() {
  const login = useLogin()
  const { toast } = useToast()
  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm<LoginFormData>({
    resolver: zodResolver(loginSchema),
  })

  const onSubmit = (data: LoginFormData) => {
    login.mutate(data, {
      onError: (error: any) => {
        toast({
          title: 'Login Failed',
          description: error.message,
          variant: 'destructive',
        })
      },
    })
  }

  return (
    <div className="flex min-h-screen items-center justify-center p-4">
      <Card className="w-full max-w-md">
        <CardHeader>
          <CardTitle>Dispatcher Login</CardTitle>
          <CardDescription>Enter your credentials to access the dashboard</CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="email">Email</Label>
              <Input
                id="email"
                type="email"
                placeholder="dispatcher@company.com"
                {...register('email')}
              />
              {errors.email && (
                <p className="text-sm text-destructive">{errors.email.message}</p>
              )}
            </div>
            <div className="space-y-2">
              <Label htmlFor="password">Password</Label>
              <Input
                id="password"
                type="password"
                {...register('password')}
              />
              {errors.password && (
                <p className="text-sm text-destructive">{errors.password.message}</p>
              )}
            </div>
            <Button type="submit" className="w-full" disabled={login.isPending}>
              {login.isPending ? 'Signing in...' : 'Sign In'}
            </Button>
          </form>
        </CardContent>
      </Card>
    </div>
  )
}
```

### 2.4 Dashboard Layout with Sidebar

```typescript
// app/[locale]/(dashboard)/layout.tsx
'use client'

import { Sidebar } from '@/components/layout/sidebar'
import { Header } from '@/components/layout/header'
import { useCurrentUser } from '@/hooks/use-auth'
import { redirect } from 'next/navigation'

export default function DashboardLayout({
  children,
}: {
  children: React.ReactNode
}) {
  const { isAuthenticated, user } = useCurrentUser()

  if (!isAuthenticated) {
    redirect('/login')
  }

  return (
    <div className="flex h-screen overflow-hidden">
      <Sidebar />
      <div className="flex flex-1 flex-col overflow-hidden">
        <Header user={user} />
        <main className="flex-1 overflow-y-auto bg-muted/30 p-6">
          {children}
        </main>
      </div>
    </div>
  )
}
```

---

## Phase 3: Core Dashboard Features

### 3.1 Dashboard Overview Page

```typescript
// app/[locale]/(dashboard)/dashboard/page.tsx
'use client'

import { StatsCards } from '@/components/dashboard/stats-cards'
import { ActiveMap } from '@/components/dashboard/active-map'
import { RecentActivity } from '@/components/dashboard/recent-activity'
import { AlertsPanel } from '@/components/dashboard/alerts-panel'
import { useQuery } from '@tanstack/react-query'
import { api } from '@/lib/api-client'

export default function DashboardPage() {
  const { data: stats } = useQuery({
    queryKey: ['dashboard-stats'],
    queryFn: () => api.get('/api/v1/dashboard/stats'),
    refetchInterval: 30000,
  })

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold">Dashboard</h1>
      <StatsCards stats={stats} />
      <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
        <ActiveMap />
        <div className="space-y-6">
          <AlertsPanel />
          <RecentActivity />
        </div>
      </div>
    </div>
  )
}
```

### 3.2 Stats Cards Component

```typescript
// components/dashboard/stats-cards.tsx
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Truck, Package, CheckCircle, AlertTriangle } from 'lucide-react'

interface Stats {
  activeShipments: number
  driversOnDuty: number
  deliveriesToday: number
  issuesCount: number
}

export function StatsCards({ stats }: { stats?: Stats }) {
  const cards = [
    {
      title: 'Active Shipments',
      value: stats?.activeShipments ?? 0,
      icon: Package,
      color: 'text-blue-600',
      bgColor: 'bg-blue-50',
    },
    {
      title: 'Drivers On Duty',
      value: stats?.driversOnDuty ?? 0,
      icon: Truck,
      color: 'text-green-600',
      bgColor: 'bg-green-50',
    },
    {
      title: 'Deliveries Today',
      value: stats?.deliveriesToday ?? 0,
      icon: CheckCircle,
      color: 'text-purple-600',
      bgColor: 'bg-purple-50',
    },
    {
      title: 'Issues',
      value: stats?.issuesCount ?? 0,
      icon: AlertTriangle,
      color: 'text-orange-600',
      bgColor: 'bg-orange-50',
    },
  ]

  return (
    <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
      {cards.map((card) => (
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
  )
}
```

### 3.3 Shipments List Page

```typescript
// app/[locale]/(dashboard)/shipments/page.tsx
'use client'

import { ShipmentList } from '@/components/shipments/shipment-list'
import { Button } from '@/components/ui/button'
import { Plus } from 'lucide-react'
import Link from 'next/link'

export default function ShipmentsPage() {
  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Shipments</h1>
        <Link href="/shipments/new">
          <Button>
            <Plus className="mr-2 h-4 w-4" />
            New Shipment
          </Button>
        </Link>
      </div>
      <ShipmentList />
    </div>
  )
}
```

### 3.4 Shipments Data Hook

```typescript
// hooks/use-shipments.ts
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '@/lib/api-client'

export interface Shipment {
  id: string
  trackingNumber: string
  status: string
  customerName: string
  customerPhone: string
  originAddress: string
  destinationAddress: string
  driverId?: string
  driverName?: string
  estimatedDelivery?: string
  actualDelivery?: string
  createdAt: string
  updatedAt: string
}

export interface ShipmentFilters {
  status?: string
  driverId?: string
  search?: string
  page?: number
  limit?: number
}

export function useShipments(filters: ShipmentFilters = {}) {
  const params = new URLSearchParams()
  if (filters.status) params.set('status', filters.status)
  if (filters.driverId) params.set('driver_id', filters.driverId)
  if (filters.search) params.set('search', filters.search)
  if (filters.page) params.set('page', String(filters.page))
  if (filters.limit) params.set('limit', String(filters.limit))

  const queryString = params.toString()
  const endpoint = `/api/v1/shipments${queryString ? `?${queryString}` : ''}`

  return useQuery({
    queryKey: ['shipments', filters],
    queryFn: () => api.get<{ shipments: Shipment[]; total: number }>(endpoint),
  })
}

export function useShipment(id: string) {
  return useQuery({
    queryKey: ['shipment', id],
    queryFn: () => api.get<Shipment>(`/api/v1/shipments/${id}`),
    enabled: !!id,
  })
}

export function useCreateShipment() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (data: Partial<Shipment>) =>
      api.post<Shipment>('/api/v1/shipments', data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['shipments'] })
    },
  })
}

export function useUpdateShipmentStatus() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: ({ id, status, note }: { id: string; status: string; note?: string }) =>
      api.put(`/api/v1/shipments/${id}/status`, { status, status_note: note }),
    onSuccess: (_, { id }) => {
      queryClient.invalidateQueries({ queryKey: ['shipment', id] })
      queryClient.invalidateQueries({ queryKey: ['shipments'] })
    },
  })
}
```

---

## Phase 4: Real-Time Features

### 4.1 WebSocket Hook

```typescript
// hooks/use-websocket.ts
import { useEffect, useRef, useCallback } from 'react'
import SockJS from 'sockjs-client'
import { useAuthStore } from '@/stores/auth-store'

type MessageHandler = (data: any) => void

export function useWebSocket(channel: string, onMessage: MessageHandler) {
  const socketRef = useRef<WebSocket | null>(null)
  const { accessToken } = useAuthStore()
  const reconnectTimeoutRef = useRef<NodeJS.Timeout>()

  const connect = useCallback(() => {
    if (!accessToken) return

    const wsUrl = `${process.env.NEXT_PUBLIC_WS_URL}/ws?token=${accessToken}`
    const socket = new SockJS(wsUrl)

    socket.onopen = () => {
      socket.send(JSON.stringify({ action: 'subscribe', channel }))
    }

    socket.onmessage = (event) => {
      const data = JSON.parse(event.data)
      if (data.channel === channel) {
        onMessage(data.payload)
      }
    }

    socket.onclose = () => {
      reconnectTimeoutRef.current = setTimeout(connect, 5000)
    }

    socketRef.current = socket as any
  }, [accessToken, channel, onMessage])

  useEffect(() => {
    connect()

    return () => {
      if (reconnectTimeoutRef.current) {
        clearTimeout(reconnectTimeoutRef.current)
      }
      if (socketRef.current) {
        socketRef.current.close()
      }
    }
  }, [connect])

  const send = useCallback((data: any) => {
    if (socketRef.current?.readyState === WebSocket.OPEN) {
      socketRef.current.send(JSON.stringify(data))
    }
  }, [])

  return { send }
}
```

### 4.2 Real-Time Map Component

```typescript
// components/dashboard/active-map.tsx
'use client'

import { MapContainer, TileLayer, Marker, Popup, useMap } from 'react-leaflet'
import { useWebSocket } from '@/hooks/use-websocket'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { api } from '@/lib/api-client'
import { useEffect, useState } from 'react'
import L from 'leaflet'
import 'leaflet/dist/leaflet.css'

const driverIcon = new L.Icon({
  iconUrl: '/icons/truck.png',
  iconSize: [32, 32],
  iconAnchor: [16, 32],
})

interface DriverLocation {
  driverId: string
  driverName: string
  lat: number
  lng: number
  lastUpdate: string
  status: string
}

export function ActiveMap() {
  const queryClient = useQueryClient()
  const [locations, setLocations] = useState<DriverLocation[]>([])

  const { data: initialLocations } = useQuery({
    queryKey: ['driver-locations'],
    queryFn: () => api.get<DriverLocation[]>('/api/v1/drivers/locations'),
  })

  useWebSocket('driver-locations', (update) => {
    setLocations((prev) => {
      const index = prev.findIndex((l) => l.driverId === update.driverId)
      if (index >= 0) {
        const updated = [...prev]
        updated[index] = { ...updated[index], ...update }
        return updated
      }
      return [...prev, update]
    })
  })

  useEffect(() => {
    if (initialLocations) {
      setLocations(initialLocations)
    }
  }, [initialLocations])

  return (
    <div className="h-[400px] rounded-lg border overflow-hidden">
      <MapContainer
        center={[9.0, 38.7]}
        zoom={12}
        className="h-full w-full"
      >
        <TileLayer
          attribution='&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a>'
          url="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png"
        />
        {locations.map((loc) => (
          <Marker
            key={loc.driverId}
            position={[loc.lat, loc.lng]}
            icon={driverIcon}
          >
            <Popup>
              <div className="text-sm">
                <p className="font-semibold">{loc.driverName}</p>
                <p className="text-muted-foreground">{loc.status}</p>
                <p className="text-xs mt-1">
                  Updated: {new Date(loc.lastUpdate).toLocaleTimeString()}
                </p>
              </div>
            </Popup>
          </Marker>
        ))}
      </MapContainer>
    </div>
  )
}
```

---

## Phase 5: Customer Tracking Portal

### 5.1 Tracking Page (Public)

```typescript
// app/[locale]/track/page.tsx
'use client'

import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { api } from '@/lib/api-client'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { TrackingTimeline } from '@/components/shipments/tracking-timeline'
import { TrackingMap } from '@/components/tracking/tracking-map'
import { StatusBadge } from '@/components/shipments/status-badge'
import { Package, Search } from 'lucide-react'

export default function TrackPage() {
  const [trackingNumber, setTrackingNumber] = useState('')
  const [searchQuery, setSearchQuery] = useState('')

  const { data: shipment, isFetching, error } = useQuery({
    queryKey: ['track', searchQuery],
    queryFn: () => api.get(`/api/v1/track/${searchQuery}`),
    enabled: !!searchQuery,
    retry: false,
  })

  const handleSearch = (e: React.FormEvent) => {
    e.preventDefault()
    if (trackingNumber.trim()) {
      setSearchQuery(trackingNumber.trim().toUpperCase())
    }
  }

  return (
    <div className="min-h-screen bg-muted/30">
      <div className="container mx-auto px-4 py-12">
        <div className="mx-auto max-w-2xl space-y-8">
          <div className="text-center">
            <Package className="mx-auto h-12 w-12 text-primary mb-4" />
            <h1 className="text-3xl font-bold">Track Your Shipment</h1>
            <p className="text-muted-foreground mt-2">
              Enter your tracking number to see delivery status
            </p>
          </div>

          <form onSubmit={handleSearch} className="flex gap-2">
            <Input
              type="text"
              placeholder="Enter tracking number (e.g., ET-20260219-1234)"
              value={trackingNumber}
              onChange={(e) => setTrackingNumber(e.target.value)}
              className="text-lg"
            />
            <Button type="submit" disabled={isFetching}>
              <Search className="h-4 w-4 mr-2" />
              Track
            </Button>
          </form>

          {error && (
            <Card className="border-destructive">
              <CardContent className="pt-6 text-center text-destructive">
                Shipment not found. Please check your tracking number.
              </CardContent>
            </Card>
          )}

          {shipment && (
            <div className="space-y-6">
              <Card>
                <CardHeader>
                  <div className="flex items-center justify-between">
                    <CardTitle>Shipment {shipment.tracking_number}</CardTitle>
                    <StatusBadge status={shipment.status} />
                  </div>
                </CardHeader>
                <CardContent className="space-y-4">
                  <div className="grid grid-cols-2 gap-4 text-sm">
                    <div>
                      <p className="text-muted-foreground">From</p>
                      <p className="font-medium">{shipment.origin_address}</p>
                    </div>
                    <div>
                      <p className="text-muted-foreground">To</p>
                      <p className="font-medium">{shipment.destination_address}</p>
                    </div>
                    <div>
                      <p className="text-muted-foreground">Customer</p>
                      <p className="font-medium">{shipment.customer_name}</p>
                    </div>
                    <div>
                      <p className="text-muted-foreground">Estimated Delivery</p>
                      <p className="font-medium">
                        {shipment.estimated_delivery
                          ? new Date(shipment.estimated_delivery).toLocaleDateString()
                          : 'Pending'}
                      </p>
                    </div>
                  </div>
                </CardContent>
              </Card>

              <TrackingMap destination={shipment.destination_coordinates} />

              <Card>
                <CardHeader>
                  <CardTitle>Tracking History</CardTitle>
                </CardHeader>
                <CardContent>
                  <TrackingTimeline events={shipment.tracking_events} />
                </CardContent>
              </Card>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
```

---

## Phase 6: Internationalization

### 6.1 i18n Configuration

```typescript
// i18n/config.ts
export const locales = ['en', 'am'] as const
export type Locale = (typeof locales)[number]

export const defaultLocale: Locale = 'en'

export const localeNames: Record<Locale, string> = {
  en: 'English',
  am: 'አማርኛ',
}
```

### 6.2 Amharic Messages Sample

```json
// i18n/am/messages.json
{
  "common": {
    "search": "ፈልግ",
    "save": "አስቀምጥ",
    "cancel": "ሰርዝ",
    "delete": "ሰርዝ",
    "edit": "አርትዕ",
    "loading": "በመጫን ላይ...",
    "error": "ስህተት"
  },
  "auth": {
    "login": "ግባ",
    "logout": "ውጣ",
    "email": "ኢሜይል",
    "password": "የይለፍ ቃል",
    "loginTitle": "ዲስፓቸር ግባ",
    "loginDescription": "ዳሽቦርዱን ለማሳየት ማረጋገጫዎን ያስገቡ"
  },
  "dashboard": {
    "title": "ዳሽቦርድ",
    "activeShipments": "ንቁ ማጓጓዣዎች",
    "driversOnDuty": "በስራ ላይ ያሉ ነጂዎች",
    "deliveriesToday": "ዛሬ የተላሉ እቃዎች",
    "issues": "ችግሮች"
  },
  "shipments": {
    "title": "ማጓጓዣዎች",
    "newShipment": "አዲስ ማጓጓዣ",
    "trackingNumber": "የመከታተያ ቁጥር",
    "status": "ሁኔታ",
    "customer": "ደንበኛ",
    "origin": "መነሻ",
    "destination": "መድረሻ"
  },
  "track": {
    "title": "ማጓጓዣዎን ይከታተሉ",
    "placeholder": "የመከታተያ ቁጥር ያስገቡ",
    "notFound": "ማጓጓዣ አልተገኘም"
  },
  "status": {
    "pending": "በመጠባበቅ ላይ",
    "assigned": "ተመድቧል",
    "in_transit": "በመንገድ ላይ",
    "delayed": "ዘግይቷል",
    "arrived": "ደርሷል",
    "delivered": "ተልኳል",
    "issue": "ችግር",
    "cancelled": "ተሰርዟል"
  }
}
```

---

## Phase 7: Performance Optimization

### 7.1 Bundle Optimization

```javascript
// next.config.js
/** @type {import('next').NextConfig} */
const nextConfig = {
  output: 'standalone',
  reactStrictMode: true,
  images: {
    domains: ['localhost', 'api.yourdomain.com'],
    formats: ['image/avif', 'image/webp'],
  },
  experimental: {
    optimizePackageImports: ['lucide-react', 'recharts', 'date-fns'],
  },
  compiler: {
    removeConsole: process.env.NODE_ENV === 'production',
  },
  async headers() {
    return [
      {
        source: '/:path*',
        headers: [
          { key: 'X-Content-Type-Options', value: 'nosniff' },
          { key: 'X-Frame-Options', value: 'DENY' },
          { key: 'X-XSS-Protection', value: '1; mode=block' },
        ],
      },
    ]
  },
}

module.exports = nextConfig
```

### 7.2 Loading Skeleton

```typescript
// components/shared/loading-skeleton.tsx
import { Skeleton } from '@/components/ui/skeleton'

export function ShipmentListSkeleton() {
  return (
    <div className="space-y-4">
      {Array.from({ length: 5 }).map((_, i) => (
        <div key={i} className="rounded-lg border p-4">
          <div className="flex items-center justify-between">
            <Skeleton className="h-5 w-32" />
            <Skeleton className="h-6 w-20" />
          </div>
          <div className="mt-4 grid grid-cols-2 gap-4">
            <Skeleton className="h-4 w-full" />
            <Skeleton className="h-4 w-full" />
          </div>
        </div>
      ))}
    </div>
  )
}
```

---

## Development Timeline

| Week | Phase | Deliverables |
|------|-------|--------------|
| **Week 1** | Project Setup | Next.js project, shadcn/ui, Tailwind, API client, auth store |
| **Week 2** | Authentication | Login pages, protected routes, layout, sidebar navigation |
| **Week 3** | Dashboard | Stats cards, map integration, real-time updates via WebSocket |
| **Week 4** | Shipments | List view, detail view, create/edit forms, status management |
| **Week 5** | Drivers & Vehicles | Driver management, vehicle management, assignment workflow |
| **Week 6** | Customer Portal | Public tracking page, tracking timeline, map display |
| **Week 7** | Admin Dashboard | Tenant management, platform stats, user management |
| **Week 8** | i18n & Polish | Amharic translations, offline support, performance optimization |
| **Week 9** | Testing | Unit tests, integration tests, E2E tests |
| **Week 10** | Deployment | Docker setup, CI/CD, production deployment |

---

## Environment Variables

```bash
# .env.local
NEXT_PUBLIC_API_URL=http://localhost:8080
NEXT_PUBLIC_WS_URL=ws://localhost:8080
NEXT_PUBLIC_MAP_TILE_URL=https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png
NEXT_PUBLIC_DEFAULT_LOCALE=en
```

---

## Files to Create (Implementation Order)

### Phase 1 - Project Setup
1. Initialize Next.js project
2. Configure Tailwind
3. Set up shadcn/ui
4. Create API client
5. Set up React Query provider
6. Create auth store (Zustand)
7. Create types

### Phase 2 - Auth & Layout
8. Login page
9. Dashboard layout
10. Sidebar component
11. Header component
12. Auth middleware

### Phase 3 - Dashboard
13. Dashboard page
14. Stats cards
15. Active map
16. Recent activity
17. Alerts panel

### Phase 4 - Shipments
18. Shipments list
19. Shipment detail
20. Shipment form
21. Status badge
22. Tracking timeline

### Phase 5 - Customer Portal
23. Track page
24. Tracking form
25. Tracking map

### Phase 6 - i18n
26. i18n config
27. English messages
28. Amharic messages
29. Language switcher

---

*Document End*
*Frontend Development Plan - Ethiopian Logistics Tracking Platform*
