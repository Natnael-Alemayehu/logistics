export type ShipmentStatus = 
  | 'pending' 
  | 'assigned' 
  | 'in_transit' 
  | 'delayed' 
  | 'arrived' 
  | 'delivered' 
  | 'issue' 
  | 'cancelled'

export interface Coordinates {
  lat: number
  lng: number
}

export interface Shipment {
  id: string
  tenant_id: string
  tracking_number: string
  origin_address: string
  origin_lat?: number
  origin_lng?: number
  destination_address: string
  destination_lat?: number
  destination_lng?: number
  customer_name: string
  customer_phone: string
  cargo_description?: string
  cargo_weight?: number
  cargo_value?: number
  special_instructions?: string
  driver_id?: string
  vehicle_id?: string
  status: ShipmentStatus
  status_note?: string
  status_reason?: string
  estimated_delivery?: string
  actual_delivery?: string
  created_at: string
  updated_at: string
  created_by?: string
}

export interface ShipmentWithDriver extends Shipment {
  driver?: {
    id: string
    full_name: string
    phone: string
  }
  vehicle?: {
    id: string
    plate_number: string
    vehicle_type?: string
  }
}

export interface TrackingEvent {
  id: string
  shipment_id: string
  driver_id?: string
  latitude: number
  longitude: number
  accuracy_meters?: number
  speed_kph?: number
  heading?: number
  event_type: 'gps_ping' | 'checkpoint' | 'status_change'
  status?: ShipmentStatus
  note?: string
  recorded_at: string
  synced_at?: string
  device_id?: string
  battery_level?: number
}

export interface ProofOfDelivery {
  id: string
  shipment_id: string
  driver_id: string
  recipient_name: string
  recipient_phone?: string
  signature_data?: string
  signature_url?: string
  photo_urls?: string[]
  delivery_address?: string
  delivery_lat?: number
  delivery_lng?: number
  delivery_notes?: string
  location_verified: boolean
  location_mismatch_meters?: number
  recorded_at: string
  synced_at?: string
}

export interface ShipmentFilters {
  status?: ShipmentStatus | 'all'
  driverId?: string | 'all'
  search?: string
  page?: number
  limit?: number
}

export interface ShipmentListResponse {
  shipments: Shipment[]
  total: number
  page: number
  per_page: number
}

export interface CreateShipmentInput {
  origin_address: string
  origin_lat?: number
  origin_lng?: number
  destination_address: string
  destination_lat?: number
  destination_lng?: number
  customer_name: string
  customer_phone: string
  cargo_description?: string
  cargo_weight?: number
  cargo_value?: number
  special_instructions?: string
  driver_id?: string
  vehicle_id?: string
}

export interface UpdateShipmentInput {
  destination_address?: string
  destination_lat?: number
  destination_lng?: number
  customer_name?: string
  customer_phone?: string
  cargo_description?: string
  special_instructions?: string
}

export interface UpdateStatusInput {
  status: ShipmentStatus
  status_note?: string
  status_reason?: string
}

/**
 * Customer-facing shipment view returned by the public tracking endpoint.
 *
 * Mirrors model.PublicShipment on the backend. Deliberately narrower than
 * Shipment: the endpoint is unauthenticated, so it carries no customer contact
 * details, no cargo information and no internal identifiers. Typing the hook to
 * this rather than Shipment means re-adding a field to the tracking page is a
 * compile error instead of a quiet data leak.
 */
export interface PublicShipment {
  tracking_number: string
  status: ShipmentStatus
  status_note?: string
  origin_address: string
  destination_address: string
  estimated_delivery?: string
  actual_delivery?: string
  updated_at: string
}
