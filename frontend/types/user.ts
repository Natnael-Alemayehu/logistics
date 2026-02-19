export interface User {
  id: string
  tenant_id: string
  role: UserRole
  full_name: string
  email?: string
  phone?: string
  is_active: boolean
  created_at: string
  updated_at: string
}

export type UserRole = 'driver' | 'dispatcher' | 'fleet_manager' | 'admin' | 'platform_admin'

export interface Tenant {
  id: string
  name: string
  slug: string
  plan: string
  max_drivers: number
  max_vehicles: number
  is_active: boolean
  created_at: string
  updated_at: string
}

export interface Session {
  id: string
  user_id: string
  tenant_id: string
  device_id?: string
  user_agent?: string
  ip_address?: string
  expires_at: string
  created_at: string
  revoked_at?: string
  is_current_session?: boolean
}
