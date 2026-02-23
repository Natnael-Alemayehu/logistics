import { getDatabase } from '../database';
import { generateUUID } from '@utils/helpers';

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
  const id = pod.id ?? generateUUID();
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

export async function update(id: string, updates: Partial<POD>): Promise<void> {
  const db = await getDatabase();
  
  const fields: string[] = [];
  const values: (string | number | null)[] = [];
  
  for (const [key, value] of Object.entries(updates)) {
    if (key === 'photo_paths' && Array.isArray(value)) {
      fields.push(`${key} = ?`);
      values.push(JSON.stringify(value));
    } else if (key === 'location_verified') {
      fields.push(`${key} = ?`);
      values.push(value ? 1 : 0);
    } else if (typeof value === 'string' || typeof value === 'number' || value === null) {
      fields.push(`${key} = ?`);
      values.push(value);
    } else if (value === undefined) {
      fields.push(`${key} = ?`);
      values.push(null);
    } else {
      fields.push(`${key} = ?`);
      values.push(String(value));
    }
  }
  
  if (fields.length === 0) return;
  
  values.push(id);
  
  await db.runAsync(
    `UPDATE proof_of_delivery SET ${fields.join(', ')} WHERE id = ?`,
    values
  );
}

export async function getAll(): Promise<POD[]> {
  const db = await getDatabase();
  const results = await db.getAllAsync<Record<string, unknown>>(
    'SELECT * FROM proof_of_delivery ORDER BY recorded_at DESC'
  );
  
  return results.map(parsePOD);
}

export async function getFailedSyncs(): Promise<POD[]> {
  const db = await getDatabase();
  const results = await db.getAllAsync<Record<string, unknown>>(
    "SELECT * FROM proof_of_delivery WHERE sync_status = 'failed' ORDER BY recorded_at DESC"
  );
  
  return results.map(parsePOD);
}

export async function clearOlderThan(date: Date): Promise<void> {
  const db = await getDatabase();
  await db.runAsync(
    'DELETE FROM proof_of_delivery WHERE recorded_at < ?',
    [date.toISOString()]
  );
}

export async function retrySync(id: string): Promise<void> {
  const db = await getDatabase();
  await db.runAsync(
    "UPDATE proof_of_delivery SET sync_status = 'pending', synced_at = NULL WHERE id = ?",
    [id]
  );
}
