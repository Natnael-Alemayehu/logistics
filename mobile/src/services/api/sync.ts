import { api } from './client'
import { API_ENDPOINTS } from '../constants'

export interface SyncRequest {
  last_sync_at?: string
  location_updates?: Array<{
    latitude: number
    longitude: number
    timestamp: string
    accuracy?: number
  }>
  status_updates?: Array<{
    shipment_id: string
    status: string
    note?: string
    reason?: string
    timestamp: string
  }>
}

export interface SyncResponse {
  sync_time: string
  shipments?: Array<{
    id: string
    tracking_number: string
    status: string
    origin: string
    destination: string
    customer_name: string
    customer_phone: string
    scheduled_date: string
    notes?: string
    created_at: string
    updated_at: string
  }>
  deleted_shipment_ids?: string[]
}

export async function sync(request: SyncRequest): Promise<SyncResponse> {
  return api.post<SyncResponse>(API_ENDPOINTS.sync.sync, request)
}
