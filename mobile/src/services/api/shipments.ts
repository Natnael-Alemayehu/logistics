import { api } from './client'
import { API_ENDPOINTS } from '../constants'

export interface Shipment {
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
  return api.get<Shipment[]>(API_ENDPOINTS.shipments.myShipments)
}

export async function getShipment(id: string): Promise<Shipment> {
  return api.get<Shipment>(API_ENDPOINTS.shipments.get(id))
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
