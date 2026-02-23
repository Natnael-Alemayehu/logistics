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
    `INSERT INTO sync_queue (entity_type, entity_id, operation, priority, attempts, created_at)
     VALUES (?, ?, ?, ?, 0, ?)`,
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
  return await db.getFirstAsync<SyncQueueItem>(
    'SELECT * FROM sync_queue ORDER BY priority DESC, created_at ASC LIMIT 1'
  );
}

export async function getByPriority(limit: number): Promise<SyncQueueItem[]> {
  const db = await getDatabase();
  return await db.getAllAsync<SyncQueueItem>(
    'SELECT * FROM sync_queue ORDER BY priority DESC, created_at ASC LIMIT ?',
    [limit]
  );
}

export async function incrementAttempts(id: string, error?: string): Promise<void> {
  const db = await getDatabase();
  await db.runAsync(
    'UPDATE sync_queue SET attempts = attempts + 1, last_error = ? WHERE id = ?',
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
    'SELECT COUNT(*) as count FROM sync_queue'
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
