import { Shipment } from './shipment';

export interface SyncRequest {
  device_id: string
  last_sync_at: string | null
  events: TrackingEventInput[]
  pods: PODInput[]
  statuses: StatusUpdateInput[]
  battery_level: number
  storage_remaining_kb: number
}

export interface TrackingEventInput {
  shipment_id: string
  latitude: number
  longitude: number
  accuracy: number
  speed?: number
  heading?: number
  event_type: 'gps_ping' | 'checkpoint' | 'status_change'
  status?: string
  note?: string
  recorded_at: string
  battery_level?: number
}

export interface PODInput {
  shipment_id: string
  recipient_name: string
  recipient_phone?: string
  signature_data?: string
  photo_urls: string[]
  delivery_address?: string
  delivery_lat: number
  delivery_lng: number
  delivery_notes?: string
  location_verified: boolean
  location_mismatch_meters?: number
  recorded_at: string
}

export interface StatusUpdateInput {
  shipment_id: string
  status: string
  note?: string
  reason?: string
  recorded_at: string
}

export interface SyncResponse {
  server_time: string
  events_received: number
  conflicts?: SyncConflict[]
  pull: SyncPullData
}

export interface SyncConflict {
  entity_type: string
  entity_id: string
  local_value: any
  server_value: any
}

export interface SyncPullData {
  shipments: Shipment[]
  messages: string[]
  updates: string[]
}

export interface SyncQueueItem {
  id: string
  entity_type: 'tracking_event' | 'pod' | 'status'
  entity_id: string
  operation: 'create' | 'update'
  priority: number
  attempts: number
  last_error?: string
  created_at: string
}
