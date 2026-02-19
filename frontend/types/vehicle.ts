export interface Vehicle {
  id: string
  plate_number: string
  make?: string
  model?: string
  year?: number
  capacity_kg?: number
  status: 'active' | 'maintenance' | 'inactive'
  driver_id?: string
  driver_name?: string
  current_lat?: number
  current_lng?: number
  created_at: string
  updated_at: string
}

export interface CreateVehicleInput {
  plate_number: string
  make?: string
  model?: string
  year?: number
  capacity_kg?: number
  status?: 'active' | 'maintenance' | 'inactive'
}
