import { getDatabase } from '@/db/database';
import {
  SyncQueueItem,
  SyncQueueInput,
  DeadLetterItem,
  SyncBatch,
  SyncBatchResult,
  RetryConfig,
  DEFAULT_RETRY_CONFIG,
  QueueStats,
  CleanupResult,
} from './types';

export class SyncQueueManager {
  private config: RetryConfig;

  constructor(config: Partial<RetryConfig> = {}) {
    this.config = { ...DEFAULT_RETRY_CONFIG, ...config };
  }

  async add(item: SyncQueueInput): Promise<number> {
    const db = await getDatabase();
    const now = new Date().toISOString();
    const result = await db.runAsync(
      `INSERT INTO sync_queue (entity_type, entity_id, operation, priority, attempts, created_at, status)
       VALUES (?, ?, ?, ?, 0, ?, 'pending')`,
      [item.entity_type, item.entity_id, item.operation, item.priority ?? 0, now]
    );
    return result.lastInsertRowId;
  }

  async getNext(): Promise<SyncQueueItem | null> {
    const db = await getDatabase();
    const now = new Date().toISOString();
    return await db.getFirstAsync<SyncQueueItem>(
      `SELECT * FROM sync_queue 
       WHERE status = 'pending' 
       AND (retry_at IS NULL OR retry_at <= ?)
       ORDER BY priority DESC, created_at ASC 
       LIMIT 1`,
      [now]
    );
  }

  async getByPriority(limit: number): Promise<SyncQueueItem[]> {
    const db = await getDatabase();
    const now = new Date().toISOString();
    return await db.getAllAsync<SyncQueueItem>(
      `SELECT * FROM sync_queue 
       WHERE status = 'pending' 
       AND (retry_at IS NULL OR retry_at <= ?)
       ORDER BY priority DESC, created_at ASC 
       LIMIT ?`,
      [now, limit]
    );
  }

  async getReadyForSync(): Promise<SyncQueueItem[]> {
    const db = await getDatabase();
    const now = new Date().toISOString();
    return await db.getAllAsync<SyncQueueItem>(
      `SELECT * FROM sync_queue 
       WHERE status IN ('pending', 'failed')
       AND (retry_at IS NULL OR retry_at <= ?)
       ORDER BY priority DESC, created_at ASC`,
      [now]
    );
  }

  calculateBackoff(attempts: number): Date {
    const intervalIndex = Math.min(attempts, this.config.backoffIntervals.length - 1);
    const interval = this.config.backoffIntervals[intervalIndex];
    return new Date(Date.now() + interval);
  }

  async incrementAttempts(id: number, error?: string): Promise<void> {
    const db = await getDatabase();
    const item = await this.getById(id);
    
    if (!item) return;

    const newAttempts = item.attempts + 1;
    const retryAt = this.calculateBackoff(newAttempts);

    if (newAttempts >= this.config.deadLetterThreshold) {
      await this.moveToDeadLetter(id, error ?? 'Max retries exceeded');
    } else {
      await db.runAsync(
        `UPDATE sync_queue 
         SET attempts = ?, last_error = ?, retry_at = ?, status = 'failed'
         WHERE id = ?`,
        [newAttempts, error ?? null, retryAt.toISOString(), id]
      );
    }
  }

  async markSynced(id: number): Promise<void> {
    const db = await getDatabase();
    const now = new Date().toISOString();
    await db.runAsync(
      `UPDATE sync_queue SET status = 'synced', synced_at = ? WHERE id = ?`,
      [now, id]
    );
  }

  async markFailed(id: number, error: string): Promise<void> {
    await this.incrementAttempts(id, error);
  }

  async remove(id: number): Promise<void> {
    const db = await getDatabase();
    await db.runAsync('DELETE FROM sync_queue WHERE id = ?', [id]);
  }

  async getById(id: number): Promise<SyncQueueItem | null> {
    const db = await getDatabase();
    return await db.getFirstAsync<SyncQueueItem>(
      'SELECT * FROM sync_queue WHERE id = ?',
      [id]
    );
  }

  async getCount(): Promise<number> {
    const db = await getDatabase();
    const result = await db.getFirstAsync<{ count: number }>(
      "SELECT COUNT(*) as count FROM sync_queue WHERE status != 'synced'"
    );
    return result?.count ?? 0;
  }

  async getStats(): Promise<QueueStats> {
    const db = await getDatabase();
    const results = await db.getAllAsync<{ status: string; count: number }>(
      'SELECT status, COUNT(*) as count FROM sync_queue GROUP BY status'
    );

    const stats: QueueStats = {
      pending: 0,
      inProgress: 0,
      synced: 0,
      failed: 0,
      deadLetter: 0,
      total: 0,
    };

    for (const row of results) {
      if (row.status in stats) {
        stats[row.status as keyof QueueStats] = row.count;
      }
      stats.total += row.count;
    }

    const deadLetterCount = await this.getDeadLetterCount();
    stats.deadLetter = deadLetterCount;

    return stats;
  }

  async cleanupCompleted(): Promise<number> {
    const db = await getDatabase();
    const cutoffDate = new Date(Date.now() - this.config.cleanupAge);
    const result = await db.runAsync(
      "DELETE FROM sync_queue WHERE status = 'synced' AND synced_at < ?",
      [cutoffDate.toISOString()]
    );
    return result.changes;
  }

  async cleanupFailed(): Promise<number> {
    const db = await getDatabase();
    const cutoffDate = new Date(Date.now() - this.config.cleanupAge * 7);
    const result = await db.runAsync(
      "DELETE FROM sync_queue WHERE status = 'failed' AND created_at < ?",
      [cutoffDate.toISOString()]
    );
    return result.changes;
  }

  async compactQueue(): Promise<CleanupResult> {
    const completed = await this.cleanupCompleted();
    const failed = await this.cleanupFailed();
    const deadLetter = await this.cleanupDeadLetter();

    return { completed, failed, deadLetter };
  }

  async getBatchForSync(batchSize: number, entityType?: string): Promise<SyncBatch> {
    const db = await getDatabase();
    const now = new Date().toISOString();
    
    let query = `SELECT * FROM sync_queue 
                 WHERE status IN ('pending', 'failed')
                 AND (retry_at IS NULL OR retry_at <= ?)`;
    const params: (string | number)[] = [now];

    if (entityType) {
      query += ' AND entity_type = ?';
      params.push(entityType);
    }

    query += ' ORDER BY priority DESC, created_at ASC LIMIT ?';
    params.push(batchSize);

    const items = await db.getAllAsync<SyncQueueItem>(query, params);

    if (items.length > 0) {
      const ids = items.map((item) => item.id);
      await db.runAsync(
        `UPDATE sync_queue SET status = 'in_progress' WHERE id IN (${ids.map(() => '?').join(',')})`,
        ids
      );
    }

    return { items, batchSize, entityType };
  }

  async markBatchSynced(ids: number[]): Promise<void> {
    if (ids.length === 0) return;
    
    const db = await getDatabase();
    const now = new Date().toISOString();
    await db.runAsync(
      `UPDATE sync_queue SET status = 'synced', synced_at = ? WHERE id IN (${ids.map(() => '?').join(',')})`,
      [now, ...ids]
    );
  }

  async markBatchFailed(results: SyncBatchResult): Promise<void> {
    for (const { id, error } of results.failed) {
      await this.markFailed(id, error);
    }
  }

  async moveToDeadLetter(id: number, error: string): Promise<void> {
    const db = await getDatabase();
    const item = await this.getById(id);

    if (!item) return;

    const now = new Date().toISOString();

    await db.runAsync(
      `INSERT INTO dead_letter_queue 
       (entity_type, entity_id, operation, priority, attempts, last_error, created_at, moved_to_dead_letter_at, original_queue_id)
       VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
      [
        item.entity_type,
        item.entity_id,
        item.operation,
        item.priority,
        item.attempts,
        error,
        item.created_at,
        now,
        id,
      ]
    );

    await db.runAsync(
      "UPDATE sync_queue SET status = 'dead_letter' WHERE id = ?",
      [id]
    );
  }

  async getDeadLetterItems(): Promise<DeadLetterItem[]> {
    const db = await getDatabase();
    return await db.getAllAsync<DeadLetterItem>(
      'SELECT * FROM dead_letter_queue ORDER BY created_at DESC'
    );
  }

  async getDeadLetterCount(): Promise<number> {
    const db = await getDatabase();
    const result = await db.getFirstAsync<{ count: number }>(
      'SELECT COUNT(*) as count FROM dead_letter_queue'
    );
    return result?.count ?? 0;
  }

  async retryFromDeadLetter(id: number): Promise<number | null> {
    const db = await getDatabase();
    const item = await db.getFirstAsync<DeadLetterItem>(
      'SELECT * FROM dead_letter_queue WHERE id = ?',
      [id]
    );

    if (!item) return null;

    const now = new Date().toISOString();
    const result = await db.runAsync(
      `INSERT INTO sync_queue (entity_type, entity_id, operation, priority, attempts, created_at, status)
       VALUES (?, ?, ?, ?, 0, ?, 'pending')`,
      [item.entity_type, item.entity_id, item.operation, item.priority, now]
    );

    await db.runAsync('DELETE FROM dead_letter_queue WHERE id = ?', [id]);

    return result.lastInsertRowId;
  }

  async cleanupDeadLetter(): Promise<number> {
    const db = await getDatabase();
    const cutoffDate = new Date(Date.now() - this.config.cleanupAge * 30);
    const result = await db.runAsync(
      'DELETE FROM dead_letter_queue WHERE created_at < ?',
      [cutoffDate.toISOString()]
    );
    return result.changes;
  }

  async clearAll(): Promise<void> {
    const db = await getDatabase();
    await db.runAsync('DELETE FROM sync_queue');
  }

  async removeByEntity(entityType: string, entityId: string): Promise<void> {
    const db = await getDatabase();
    await db.runAsync(
      'DELETE FROM sync_queue WHERE entity_type = ? AND entity_id = ?',
      [entityType, entityId]
    );
  }

  async getFailedItems(): Promise<SyncQueueItem[]> {
    const db = await getDatabase();
    return await db.getAllAsync<SyncQueueItem>(
      "SELECT * FROM sync_queue WHERE status = 'failed' ORDER BY created_at DESC"
    );
  }

  async resetAttempts(id: number): Promise<void> {
    const db = await getDatabase();
    await db.runAsync(
      "UPDATE sync_queue SET attempts = 0, last_error = NULL, retry_at = NULL, status = 'pending' WHERE id = ?",
      [id]
    );
  }

  async getCountsByEntityType(): Promise<Record<string, number>> {
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

  async getScheduledRetries(): Promise<SyncQueueItem[]> {
    const db = await getDatabase();
    const now = new Date().toISOString();
    return await db.getAllAsync<SyncQueueItem>(
      `SELECT * FROM sync_queue 
       WHERE status = 'failed' 
       AND retry_at IS NOT NULL 
       AND retry_at > ?
       ORDER BY retry_at ASC`,
      [now]
    );
  }
}

export const syncQueueManager = new SyncQueueManager();
