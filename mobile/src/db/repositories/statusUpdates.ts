import { getDatabase } from '../database';
import { generateUUID } from '@utils/helpers';

export interface StatusUpdate {
  id: string;
  shipment_id: string;
  status: string;
  note?: string;
  reason?: string;
  recorded_at: string;
  synced_at?: string;
}

export interface StatusUpdateInput {
  id?: string;
  shipment_id: string;
  status: string;
  note?: string;
  reason?: string;
  recorded_at?: string;
}

export async function insert(statusUpdate: StatusUpdateInput): Promise<string> {
  const db = await getDatabase();
  const id = statusUpdate.id ?? generateUUID();
  const recordedAt = statusUpdate.recorded_at ?? new Date().toISOString();
  
  await db.runAsync(
    `INSERT INTO status_updates (id, shipment_id, status, note, reason, recorded_at, synced_at)
     VALUES (?, ?, ?, ?, ?, ?, ?)`,
    [
      id,
      statusUpdate.shipment_id,
      statusUpdate.status,
      statusUpdate.note ?? null,
      statusUpdate.reason ?? null,
      recordedAt,
      null,
    ]
  );
  
  return id;
}

export async function getPendingSync(): Promise<StatusUpdate[]> {
  const db = await getDatabase();
  return await db.getAllAsync<StatusUpdate>(
    'SELECT * FROM status_updates WHERE synced_at IS NULL ORDER BY recorded_at ASC'
  );
}

export async function markSynced(id: string, syncedAt: string): Promise<void> {
  const db = await getDatabase();
  await db.runAsync(
    'UPDATE status_updates SET synced_at = ? WHERE id = ?',
    [syncedAt, id]
  );
}

export async function getByShipmentId(shipmentId: string): Promise<StatusUpdate[]> {
  const db = await getDatabase();
  return await db.getAllAsync<StatusUpdate>(
    'SELECT * FROM status_updates WHERE shipment_id = ? ORDER BY recorded_at DESC',
    [shipmentId]
  );
}

export async function getUnsyncedCount(): Promise<number> {
  const db = await getDatabase();
  const result = await db.getFirstAsync<{ count: number }>(
    'SELECT COUNT(*) as count FROM status_updates WHERE synced_at IS NULL'
  );
  return result?.count ?? 0;
}
