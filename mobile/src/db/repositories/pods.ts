import { getDatabase } from '../database';

export interface POD {
  id: string;
  shipment_id: string;
  driver_id: string;
  recipient_name: string;
  recipient_phone?: string;
  signature_data?: string;
  photo_paths?: string[];
  delivery_address?: string;
  delivery_lat?: number;
  delivery_lng?: number;
  delivery_notes?: string;
  location_verified: boolean;
  location_mismatch_meters?: number;
  recorded_at: string;
  synced_at?: string;
  sync_status: string;
}

export interface PODInput {
  id?: string;
  shipment_id: string;
  driver_id: string;
  recipient_name: string;
  recipient_phone?: string;
  signature_data?: string;
  photo_paths?: string[];
  delivery_address?: string;
  delivery_lat?: number;
  delivery_lng?: number;
  delivery_notes?: string;
  location_verified?: boolean;
  location_mismatch_meters?: number;
  recorded_at?: string;
}

function parsePOD(row: Record<string, unknown>): POD {
  return {
    ...row,
    photo_paths: row.photo_paths ? JSON.parse(row.photo_paths as string) : [],
    location_verified: row.location_verified === 1,
  } as POD;
}

export async function insert(pod: PODInput): Promise<string> {
  const db = await getDatabase();
  const id = pod.id ?? crypto.randomUUID();
  const recordedAt = pod.recorded_at ?? new Date().toISOString();
  
  await db.runAsync(
    `INSERT INTO proof_of_delivery (
      id, shipment_id, driver_id, recipient_name, recipient_phone,
      signature_data, photo_paths, delivery_address, delivery_lat, delivery_lng,
      delivery_notes, location_verified, location_mismatch_meters, recorded_at, synced_at, sync_status
    ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
    [
      id,
      pod.shipment_id,
      pod.driver_id,
      pod.recipient_name,
      pod.recipient_phone ?? null,
      pod.signature_data ?? null,
      pod.photo_paths ? JSON.stringify(pod.photo_paths) : null,
      pod.delivery_address ?? null,
      pod.delivery_lat ?? null,
      pod.delivery_lng ?? null,
      pod.delivery_notes ?? null,
      pod.location_verified ? 1 : 0,
      pod.location_mismatch_meters ?? null,
      recordedAt,
      null,
      'pending',
    ]
  );
  
  return id;
}

export async function getById(id: string): Promise<POD | null> {
  const db = await getDatabase();
  const result = await db.getFirstAsync<Record<string, unknown>>(
    'SELECT * FROM proof_of_delivery WHERE id = ?',
    [id]
  );
  
  return result ? parsePOD(result) : null;
}

export async function getByShipmentId(shipmentId: string): Promise<POD | null> {
  const db = await getDatabase();
  const result = await db.getFirstAsync<Record<string, unknown>>(
    'SELECT * FROM proof_of_delivery WHERE shipment_id = ?',
    [shipmentId]
  );
  
  return result ? parsePOD(result) : null;
}

export async function getPendingSync(): Promise<POD[]> {
  const db = await getDatabase();
  const results = await db.getAllAsync<Record<string, unknown>>(
    "SELECT * FROM proof_of_delivery WHERE sync_status = 'pending' OR synced_at IS NULL"
  );
  
  return results.map(parsePOD);
}

export async function markSynced(id: string, syncedAt: string): Promise<void> {
  const db = await getDatabase();
  await db.runAsync(
    "UPDATE proof_of_delivery SET synced_at = ?, sync_status = 'synced' WHERE id = ?",
    [syncedAt, id]
  );
}

export async function updateSyncStatus(id: string, status: string): Promise<void> {
  const db = await getDatabase();
  await db.runAsync(
    'UPDATE proof_of_delivery SET sync_status = ? WHERE id = ?',
    [status, id]
  );
}
