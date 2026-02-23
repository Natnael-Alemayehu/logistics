import { getDatabase } from '../database';

export interface SyncQueueItem {
  id: number;
  entity_type: string;
  entity_id: string;
  operation: string;
  priority: number;
  attempts: number;
  last_error?: string;
  created_at: string;
  retry_at?: string;
  status: string;
  synced_at?: string;
}

export interface SyncQueueInput {
  entity_type: string;
  entity_id: string;
  operation: string;
  priority?: number;
}

export async function add(item: SyncQueueInput): Promise<string> {
  const db = await getDatabase();
  const result = await db.runAsync(
    `INSERT INTO sync_queue (entity_type, entity_id, operation, priority, attempts, created_at, status)
     VALUES (?, ?, ?, ?, 0, ?, 'pending')`,
    [
      item.entity_type,
      item.entity_id,
      item.operation,
      item.priority ?? 0,
      new Date().toISOString(),
    ]
  );
  
  return result.lastInsertRowId.toString();
}

export async function getNext(): Promise<SyncQueueItem | null> {
  const db = await getDatabase();
  const now = new Date().toISOString();
  return await db.getFirstAsync<SyncQueueItem>(
    `SELECT * FROM sync_queue 
     WHERE status = 'pending' 
     AND (retry_at IS NULL OR retry_at <= ?)
     ORDER BY priority DESC, created_at ASC LIMIT 1`,
    [now]
  );
}

export async function getByPriority(limit: number): Promise<SyncQueueItem[]> {
  const db = await getDatabase();
  const now = new Date().toISOString();
  return await db.getAllAsync<SyncQueueItem>(
    `SELECT * FROM sync_queue 
     WHERE status = 'pending' 
     AND (retry_at IS NULL OR retry_at <= ?)
     ORDER BY priority DESC, created_at ASC LIMIT ?`,
    [now, limit]
  );
}

export async function incrementAttempts(id: string, error?: string): Promise<void> {
  const db = await getDatabase();
  await db.runAsync(
    `UPDATE sync_queue SET attempts = attempts + 1, last_error = ?, status = 'failed' WHERE id = ?`,
    [error ?? null, id]
  );
}

export async function remove(id: string): Promise<void> {
  const db = await getDatabase();
  await db.runAsync('DELETE FROM sync_queue WHERE id = ?', [id]);
}

export async function getCount(): Promise<number> {
  const db = await getDatabase();
  const result = await db.getFirstAsync<{ count: number }>(
    "SELECT COUNT(*) as count FROM sync_queue WHERE status != 'synced'"
  );
  return result?.count ?? 0;
}

export async function clearAll(): Promise<void> {
  const db = await getDatabase();
  await db.runAsync('DELETE FROM sync_queue');
}

export async function removeByEntity(entityType: string, entityId: string): Promise<void> {
  const db = await getDatabase();
  await db.runAsync(
    'DELETE FROM sync_queue WHERE entity_type = ? AND entity_id = ?',
    [entityType, entityId]
  );
}

export async function getFailedItems(): Promise<SyncQueueItem[]> {
  const db = await getDatabase();
  return await db.getAllAsync<SyncQueueItem>(
    "SELECT * FROM sync_queue WHERE status = 'failed' ORDER BY created_at DESC"
  );
}

export async function resetAttempts(id: number): Promise<void> {
  const db = await getDatabase();
  await db.runAsync(
    "UPDATE sync_queue SET attempts = 0, last_error = NULL, retry_at = NULL, status = 'pending' WHERE id = ?",
    [id]
  );
}

export async function cleanupOlderThan(date: Date): Promise<void> {
  const db = await getDatabase();
  await db.runAsync(
    "DELETE FROM sync_queue WHERE created_at < ? AND status = 'failed'",
    [date.toISOString()]
  );
}

export async function getCountsByEntityType(): Promise<Record<string, number>> {
  const db = await getDatabase();
  const results = await db.getAllAsync<{ entity_type: string; count: number }>(
    "SELECT entity_type, COUNT(*) as count FROM sync_queue WHERE status != 'synced' GROUP BY entity_type"
  );
  
  const counts: Record<string, number> = {};
  for (const row of results) {
    counts[row.entity_type] = row.count;
  }
  return counts;
}

export async function getDeadLetterQueue(): Promise<SyncQueueItem[]> {
  const db = await getDatabase();
  return await db.getAllAsync<SyncQueueItem>(
    "SELECT * FROM sync_queue WHERE status = 'dead_letter' ORDER BY created_at DESC"
  );
}

export async function markSynced(id: number): Promise<void> {
  const db = await getDatabase();
  const now = new Date().toISOString();
  await db.runAsync(
    "UPDATE sync_queue SET status = 'synced', synced_at = ? WHERE id = ?",
    [now, id]
  );
}

export async function setRetryAt(id: number, retryAt: Date): Promise<void> {
  const db = await getDatabase();
  await db.runAsync(
    'UPDATE sync_queue SET retry_at = ? WHERE id = ?',
    [retryAt.toISOString(), id]
  );
}
