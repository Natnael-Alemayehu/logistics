import { getDatabase } from '../database';

export interface SyncMetadata {
  id: number;
  driver_id: string;
  last_sync_at?: string;
  last_event_id?: string;
  pending_count: number;
  storage_used_kb: number;
  device_id?: string;
}

export async function get(): Promise<SyncMetadata | null> {
  const db = await getDatabase();
  return await db.getFirstAsync<SyncMetadata>(
    'SELECT * FROM sync_metadata WHERE id = 1'
  );
}

export async function upsert(metadata: Partial<SyncMetadata>): Promise<void> {
  const db = await getDatabase();
  
  const existing = await get();
  
  if (existing) {
    const fields: string[] = [];
    const values: (string | number | null)[] = [];
    
    for (const [key, value] of Object.entries(metadata)) {
      if (key !== 'id') {
        fields.push(`${key} = ?`);
        values.push(value ?? null);
      }
    }
    
    if (fields.length === 0) return;
    
    values.push(1);
    await db.runAsync(
      `UPDATE sync_metadata SET ${fields.join(', ')} WHERE id = ?`,
      values
    );
  } else {
    await db.runAsync(
      `INSERT INTO sync_metadata (id, driver_id, last_sync_at, last_event_id, pending_count, storage_used_kb, device_id)
       VALUES (1, ?, ?, ?, ?, ?, ?)`,
      [
        metadata.driver_id ?? '',
        metadata.last_sync_at ?? null,
        metadata.last_event_id ?? null,
        metadata.pending_count ?? 0,
        metadata.storage_used_kb ?? 0,
        metadata.device_id ?? null,
      ]
    );
  }
}

export async function updateLastSync(timestamp: Date): Promise<void> {
  const db = await getDatabase();
  await db.runAsync(
    'UPDATE sync_metadata SET last_sync_at = ? WHERE id = 1',
    [timestamp.toISOString()]
  );
}

export async function incrementPendingCount(count: number): Promise<void> {
  const db = await getDatabase();
  await db.runAsync(
    'UPDATE sync_metadata SET pending_count = pending_count + ? WHERE id = 1',
    [count]
  );
}

export async function reset(): Promise<void> {
  const db = await getDatabase();
  await db.runAsync('DELETE FROM sync_metadata WHERE id = 1');
}
