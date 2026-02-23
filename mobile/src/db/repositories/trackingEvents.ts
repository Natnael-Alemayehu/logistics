import { getDatabase } from '../database';
import { generateUUID } from '@utils/helpers';

export interface TrackingEvent {
  id: string;
  shipment_id: string;
  driver_id: string;
  latitude: number;
  longitude: number;
  accuracy?: number;
  speed?: number;
  heading?: number;
  event_type: string;
  status?: string;
  note?: string;
  recorded_at: string;
  synced_at?: string;
  device_id?: string;
  battery_level?: number;
  sync_priority: number;
}

export interface TrackingEventInput {
  id?: string;
  shipment_id: string;
  driver_id: string;
  latitude: number;
  longitude: number;
  accuracy?: number;
  speed?: number;
  heading?: number;
  event_type: string;
  status?: string;
  note?: string;
  recorded_at?: string;
  device_id?: string;
  battery_level?: number;
  sync_priority?: number;
}

export async function insert(event: TrackingEventInput): Promise<string> {
  const db = await getDatabase();
  const id = event.id ?? generateUUID();
  const recordedAt = event.recorded_at ?? new Date().toISOString();
  
  await db.runAsync(
    `INSERT INTO tracking_events (
      id, shipment_id, driver_id, latitude, longitude, accuracy, speed, heading,
      event_type, status, note, recorded_at, synced_at, device_id, battery_level, sync_priority
    ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
    [
      id,
      event.shipment_id,
      event.driver_id,
      event.latitude,
      event.longitude,
      event.accuracy ?? null,
      event.speed ?? null,
      event.heading ?? null,
      event.event_type,
      event.status ?? null,
      event.note ?? null,
      recordedAt,
      null,
      event.device_id ?? null,
      event.battery_level ?? null,
      event.sync_priority ?? 0,
    ]
  );
  
  return id;
}

export async function getPendingSync(): Promise<TrackingEvent[]> {
  const db = await getDatabase();
  return await db.getAllAsync<TrackingEvent>(
    'SELECT * FROM tracking_events WHERE synced_at IS NULL ORDER BY sync_priority DESC, recorded_at ASC'
  );
}

export async function markSynced(id: string, syncedAt: string): Promise<void> {
  const db = await getDatabase();
  await db.runAsync(
    'UPDATE tracking_events SET synced_at = ? WHERE id = ?',
    [syncedAt, id]
  );
}

export async function getByShipmentId(shipmentId: string): Promise<TrackingEvent[]> {
  const db = await getDatabase();
  return await db.getAllAsync<TrackingEvent>(
    'SELECT * FROM tracking_events WHERE shipment_id = ? ORDER BY recorded_at DESC',
    [shipmentId]
  );
}

export async function getUnsyncedCount(): Promise<number> {
  const db = await getDatabase();
  const result = await db.getFirstAsync<{ count: number }>(
    'SELECT COUNT(*) as count FROM tracking_events WHERE synced_at IS NULL'
  );
  return result?.count ?? 0;
}

export async function deleteOlderThan(days: number): Promise<void> {
  const db = await getDatabase();
  const cutoff = new Date();
  cutoff.setDate(cutoff.getDate() - days);
  
  await db.runAsync(
    'DELETE FROM tracking_events WHERE synced_at IS NOT NULL AND recorded_at < ?',
    [cutoff.toISOString()]
  );
}

export async function update(id: string, updates: Partial<TrackingEvent>): Promise<void> {
  const db = await getDatabase();
  
  const fields: string[] = [];
  const values: (string | number | null)[] = [];
  
  for (const [key, value] of Object.entries(updates)) {
    fields.push(`${key} = ?`);
    values.push(value ?? null);
  }
  
  if (fields.length === 0) return;
  
  values.push(id);
  
  await db.runAsync(
    `UPDATE tracking_events SET ${fields.join(', ')} WHERE id = ?`,
    values
  );
}

export async function deleteById(id: string): Promise<void> {
  const db = await getDatabase();
  await db.runAsync('DELETE FROM tracking_events WHERE id = ?', [id]);
}

export async function batchInsert(events: TrackingEventInput[]): Promise<void> {
  const db = await getDatabase();
  
  await db.withTransactionAsync(async () => {
    for (const event of events) {
      await insert(event);
    }
  });
}

export async function getCountsBySyncStatus(): Promise<{ synced: number; unsynced: number }> {
  const db = await getDatabase();
  const syncedResult = await db.getFirstAsync<{ count: number }>(
    'SELECT COUNT(*) as count FROM tracking_events WHERE synced_at IS NOT NULL'
  );
  const unsyncedResult = await db.getFirstAsync<{ count: number }>(
    'SELECT COUNT(*) as count FROM tracking_events WHERE synced_at IS NULL'
  );
  
  return {
    synced: syncedResult?.count ?? 0,
    unsynced: unsyncedResult?.count ?? 0,
  };
}

export async function getFailedSyncs(): Promise<TrackingEvent[]> {
  const db = await getDatabase();
  return await db.getAllAsync<TrackingEvent>(
    "SELECT * FROM tracking_events WHERE synced_at IS NULL ORDER BY recorded_at DESC"
  );
}
