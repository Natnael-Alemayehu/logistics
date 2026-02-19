export interface Driver {
  id: string
  tenant_id: string
  full_name: string
  phone: string
  email?: string
  role: string
  is_active: boolean
  assigned_vehicle_id?: string
  assigned_vehicle_plate?: string
  current_lat?: number
  current_lng?: number
  last_location_update?: string
  created_at: string
  updated_at: string
}

export interface DriverLocation {
  driver_id: string
  driver_name: string
  lat: number
  lng: number
  last_update: string
  status: string
  battery_level?: number
  speed_kph?: number
}

export interface CreateDriverInput {
  full_name: string
  phone: string
  email?: string
  pin?: string
  vehicle_id?: string
}


