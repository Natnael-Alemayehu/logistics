export interface Vehicle {
  id: string
  plate_number: string
  vehicle_type?: string
  is_active: boolean
  created_at: string
  updated_at: string
}

export interface CreateVehicleInput {
  plate_number: string
  vehicle_type?: string
}

export interface UpdateVehicleInput {
  plate_number?: string
  vehicle_type?: string
  is_active?: boolean
}
