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
  status: SyncStatus;
  synced_at?: string;
}

export interface SyncQueueInput {
  entity_type: string;
  entity_id: string;
  operation: string;
  priority?: number;
  payload?: unknown;
}

export interface DeadLetterItem {
  id: number;
  entity_type: string;
  entity_id: string;
  operation: string;
  priority: number;
  attempts: number;
  last_error: string;
  created_at: string;
  moved_to_dead_letter_at: string;
  original_queue_id: number;
}

export interface ScheduledRetry {
  id: number;
  queue_item_id: number;
  retry_at: string;
  retry_count: number;
  created_at: string;
}

export interface SyncBatch {
  items: SyncQueueItem[];
  batchSize: number;
  entityType?: string;
}

export interface SyncBatchResult {
  succeeded: number[];
  failed: Array<{ id: number; error: string }>;
}

export interface RetryConfig {
  maxAttempts: number;
  backoffIntervals: number[];
  deadLetterThreshold: number;
  cleanupAge: number;
}

export const DEFAULT_RETRY_CONFIG: RetryConfig = {
  maxAttempts: 5,
  backoffIntervals: [60000, 300000, 900000, 3600000, 21600000],
  deadLetterThreshold: 5,
  cleanupAge: 86400000,
};

export type SyncStatus = 'pending' | 'in_progress' | 'synced' | 'failed' | 'dead_letter';

export interface QueueStats {
  pending: number;
  inProgress: number;
  synced: number;
  failed: number;
  deadLetter: number;
  total: number;
}

export interface CleanupResult {
  completed: number;
  failed: number;
  deadLetter: number;
}
