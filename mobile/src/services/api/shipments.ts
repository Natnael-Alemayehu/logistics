import { api } from './client'
import { API_ENDPOINTS } from '../constants'
import type { ShipmentStatus } from '@/types/shipment'

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

export interface TrackingEvent {
  id: string
  shipment_id: string
  status: string
  location?: string
  note?: string
  reason?: string
  created_at: string
}

export async function getMyShipments(): Promise<Shipment[]> {
  const data = await api.get<Shipment[]>(API_ENDPOINTS.shipments.myShipments)
  // Cast status to ShipmentStatus for type safety
  return data.map(s => ({ ...s, status: s.status as ShipmentStatus }))
}

export async function getShipment(id: string): Promise<Shipment> {
  const data = await api.get<Shipment>(API_ENDPOINTS.shipments.get(id))
  return { ...data, status: data.status as ShipmentStatus }
}

export async function updateStatus(
  shipmentId: string,
  status: string,
  note?: string,
  reason?: string
): Promise<void> {
  return api.patch<void>(API_ENDPOINTS.shipments.updateStatus(shipmentId), {
    status,
    note,
    reason,
  })
}

export async function getTrackingEvents(shipmentId: string): Promise<TrackingEvent[]> {
  return api.get<TrackingEvent[]>(API_ENDPOINTS.shipments.trackingEvents(shipmentId))
}
