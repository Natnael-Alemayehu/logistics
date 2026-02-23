import { SyncQueueManager, syncQueueManager } from './SyncQueueManager';
import { conflictResolver } from './ConflictResolver';
import ConflictResolver from './ConflictResolver';
import { SyncQueueItem, SyncBatch, SyncBatchResult, QueueStats } from './types';
import { api } from '@/services/api/client';
import { API_ENDPOINTS } from '@/services/constants';
import * as trackingEventsRepo from '@/db/repositories/trackingEvents';
import * as podsRepo from '@/db/repositories/pods';
import * as shipmentsRepo from '@/db/repositories/shipments';
import * as syncMetadataRepo from '@/db/repositories/syncMetadata';
import { ResolutionStrategy, SyncConflict, ResolvedData } from '@/types/conflict';
import { getDatabase } from '@/db/database';

export interface SyncOptions {
  forceFullSync?: boolean;
  entityTypes?: ('tracking_events' | 'pods' | 'status_updates')[];
  skipPull?: boolean;
  skipPush?: boolean;
}

export interface SyncProgress {
  phase: 'idle' | 'pulling' | 'pushing' | 'processing_queue' | 'completed' | 'error';
  currentEntity: string | null;
  itemsProcessed: number;
  totalItems: number;
  percentage: number;
  message: string;
}

export interface SyncResult {
  success: boolean;
  pulled: number;
  pushed: number;
  conflicts: number;
  errors: SyncError[];
  duration: number;
  syncTime: string;
}

export interface SyncError {
  entityType: string;
  entityId: string;
  error: string;
  timestamp: string;
}

export interface PullResult {
  shipments: number;
  deletedShipments: number;
  timestamp: string;
}

export interface PushResult {
  trackingEvents: number;
  pods: number;
  statusUpdates: number;
}

export interface BatchResult {
  succeeded: number[];
  failed: Array<{ id: number; error: string }>;
  conflicts: Array<{ item: SyncQueueItem; conflict: SyncConflict }>;
}

export type ProgressCallback = (progress: SyncProgress) => void;

const BATCH_SIZES = {
  tracking_events: 50,
  pods: 1,
  status_updates: 20,
} as const;

const BATCH_DELAY_MS = 100;
const MAX_RETRIES_ON_NETWORK_ERROR = 3;
const NETWORK_ERROR_RETRY_DELAY_MS = 5000;

class SyncService {
  private static instance: SyncService;

  private queueManager: SyncQueueManager;
  private conflictResolver: ConflictResolver;

  private isSyncing: boolean = false;
  private syncProgress: SyncProgress = this.getInitialProgress();
  private progressCallbacks: Set<ProgressCallback> = new Set();
  private errors: SyncError[] = [];

  private constructor() {
    this.queueManager = syncQueueManager;
    this.conflictResolver = conflictResolver;
  }

  static getInstance(): SyncService {
    if (!SyncService.instance) {
      SyncService.instance = new SyncService();
    }
    return SyncService.instance;
  }

  private getInitialProgress(): SyncProgress {
    return {
      phase: 'idle',
      currentEntity: null,
      itemsProcessed: 0,
      totalItems: 0,
      percentage: 0,
      message: 'Ready to sync',
    };
  }

  async sync(options?: SyncOptions): Promise<SyncResult> {
    if (this.isSyncing) {
      return {
        success: false,
        pulled: 0,
        pushed: 0,
        conflicts: 0,
        errors: [{ entityType: 'system', entityId: 'sync', error: 'Sync already in progress', timestamp: new Date().toISOString() }],
        duration: 0,
        syncTime: new Date().toISOString(),
      };
    }

    const startTime = Date.now();
    this.isSyncing = true;
    this.errors = [];
    this.syncProgress = this.getInitialProgress();

    let pulled = 0;
    let pushed = 0;
    let conflicts = 0;

    try {
      if (!options?.skipPull) {
        this.updateProgress({ phase: 'pulling', message: 'Pulling changes from server...' });
        const pullResult = await this.pullChanges(
          options?.forceFullSync ? undefined : await this.getLastSyncTimestamp()
        );
        pulled = pullResult.shipments + pullResult.deletedShipments;
      }

      if (!options?.skipPush) {
        this.updateProgress({ phase: 'pushing', message: 'Pushing local changes...' });
        const pushResult = await this.pushChanges(options?.entityTypes);
        pushed = pushResult.trackingEvents + pushResult.pods + pushResult.statusUpdates;
      }

      this.updateProgress({ phase: 'processing_queue', message: 'Processing sync queue...' });
      const queueResult = await this.processSyncQueue();
      conflicts = queueResult.conflicts;

      const syncTime = new Date().toISOString();
      await syncMetadataRepo.updateLastSync(new Date(syncTime));

      this.updateProgress({
        phase: 'completed',
        message: 'Sync completed successfully',
        percentage: 100,
      });

      return {
        success: true,
        pulled,
        pushed,
        conflicts,
        errors: this.errors,
        duration: Date.now() - startTime,
        syncTime,
      };
    } catch (error) {
      const errorMessage = error instanceof Error ? error.message : 'Unknown sync error';
      this.errors.push({
        entityType: 'system',
        entityId: 'sync',
        error: errorMessage,
        timestamp: new Date().toISOString(),
      });

      this.updateProgress({
        phase: 'error',
        message: errorMessage,
      });

      return {
        success: false,
        pulled,
        pushed,
        conflicts,
        errors: this.errors,
        duration: Date.now() - startTime,
        syncTime: new Date().toISOString(),
      };
    } finally {
      this.isSyncing = false;
    }
  }

  async syncTrackingEvents(): Promise<void> {
    await this.sync({ entityTypes: ['tracking_events'] });
  }

  async syncPODs(): Promise<void> {
    await this.sync({ entityTypes: ['pods'] });
  }

  async syncStatusUpdates(): Promise<void> {
    await this.sync({ entityTypes: ['status_updates'] });
  }

  async pullChanges(since?: Date): Promise<PullResult> {
    const payload: Record<string, unknown> = {
      last_sync_at: since?.toISOString(),
    };

    try {
      const response = await api.post<{
        sync_time: string;
        shipments?: shipmentsRepo.Shipment[];
        deleted_shipment_ids?: string[];
      }>(API_ENDPOINTS.sync.sync, payload);

      let shipmentsCount = 0;
      let deletedCount = 0;

      if (response.shipments?.length) {
        await shipmentsRepo.upsertMany(response.shipments);
        shipmentsCount = response.shipments.length;
      }

      if (response.deleted_shipment_ids?.length) {
        await shipmentsRepo.batchDelete(response.deleted_shipment_ids);
        deletedCount = response.deleted_shipment_ids.length;
      }

      return {
        shipments: shipmentsCount,
        deletedShipments: deletedCount,
        timestamp: response.sync_time,
      };
    } catch (error) {
      const errorMessage = error instanceof Error ? error.message : 'Pull failed';
      this.errors.push({
        entityType: 'shipments',
        entityId: 'pull',
        error: errorMessage,
        timestamp: new Date().toISOString(),
      });
      throw error;
    }
  }

  async pushChanges(entityTypes?: ('tracking_events' | 'pods' | 'status_updates')[]): Promise<PushResult> {
    const types = entityTypes ?? ['tracking_events', 'pods', 'status_updates'];
    const result: PushResult = {
      trackingEvents: 0,
      pods: 0,
      statusUpdates: 0,
    };

    if (types.includes('tracking_events')) {
      result.trackingEvents = await this.pushTrackingEvents();
    }

    if (types.includes('pods')) {
      result.pods = await this.pushPODs();
    }

    if (types.includes('status_updates')) {
      result.statusUpdates = await this.pushStatusUpdates();
    }

    return result;
  }

  private async pushTrackingEvents(): Promise<number> {
    const events = await trackingEventsRepo.getPendingSync();
    if (events.length === 0) return 0;

    const batchSize = BATCH_SIZES.tracking_events;
    let processed = 0;

    this.updateProgress({
      currentEntity: 'tracking_events',
      totalItems: events.length,
      message: `Syncing ${events.length} tracking events...`,
    });

    for (let i = 0; i < events.length; i += batchSize) {
      const batch = events.slice(i, i + batchSize);

      try {
        const payload = {
          events: batch.map((e) => ({
            shipment_id: e.shipment_id,
            latitude: e.latitude,
            longitude: e.longitude,
            accuracy: e.accuracy ?? 0,
            speed: e.speed,
            heading: e.heading,
            event_type: e.event_type,
            status: e.status,
            note: e.note,
            recorded_at: e.recorded_at,
            battery_level: e.battery_level,
            device_id: e.device_id,
          })),
        };

        await api.post(API_ENDPOINTS.sync.sync, payload);

        const now = new Date().toISOString();
        await Promise.all(batch.map((e) => trackingEventsRepo.markSynced(e.id, now)));
        processed += batch.length;

        this.updateProgress({
          itemsProcessed: processed,
          percentage: Math.round((processed / events.length) * 100),
        });

        await this.delay(BATCH_DELAY_MS);
      } catch (error) {
        const errorMessage = error instanceof Error ? error.message : 'Failed to sync tracking events batch';
        for (const event of batch) {
          this.errors.push({
            entityType: 'tracking_events',
            entityId: event.id,
            error: errorMessage,
            timestamp: new Date().toISOString(),
          });
        }
      }
    }

    return processed;
  }

  private async pushPODs(): Promise<number> {
    const pendingPODs = await podsRepo.getPendingSync();
    if (pendingPODs.length === 0) return 0;

    this.updateProgress({
      currentEntity: 'pods',
      totalItems: pendingPODs.length,
      message: `Syncing ${pendingPODs.length} PODs...`,
    });

    let processed = 0;

    for (const pod of pendingPODs) {
      try {
        const payload = {
          pods: [{
            shipment_id: pod.shipment_id,
            recipient_name: pod.recipient_name,
            recipient_phone: pod.recipient_phone,
            signature_data: pod.signature_data,
            photo_urls: pod.photo_paths ?? [],
            delivery_address: pod.delivery_address,
            delivery_lat: pod.delivery_lat ?? 0,
            delivery_lng: pod.delivery_lng ?? 0,
            delivery_notes: pod.delivery_notes,
            location_verified: pod.location_verified,
            location_mismatch_meters: pod.location_mismatch_meters,
            recorded_at: pod.recorded_at,
          }],
        };

        await api.post(API_ENDPOINTS.sync.sync, payload);

        const now = new Date().toISOString();
        await podsRepo.markSynced(pod.id, now);
        processed++;

        this.updateProgress({
          itemsProcessed: processed,
          percentage: Math.round((processed / pendingPODs.length) * 100),
        });

        await this.delay(BATCH_DELAY_MS);
      } catch (error) {
        const errorMessage = error instanceof Error ? error.message : 'Failed to sync POD';
        this.errors.push({
          entityType: 'pods',
          entityId: pod.id,
          error: errorMessage,
          timestamp: new Date().toISOString(),
        });
        await podsRepo.updateSyncStatus(pod.id, 'failed');
      }
    }

    return processed;
  }

  private async pushStatusUpdates(): Promise<number> {
    const db = await getDatabase();
    const statusUpdates = await db.getAllAsync<{
      id: string;
      shipment_id: string;
      status: string;
      note?: string;
      reason?: string;
      recorded_at: string;
      synced_at?: string;
    }>(
      "SELECT * FROM status_updates WHERE synced_at IS NULL ORDER BY recorded_at ASC"
    );

    if (statusUpdates.length === 0) return 0;

    const batchSize = BATCH_SIZES.status_updates;
    let processed = 0;

    this.updateProgress({
      currentEntity: 'status_updates',
      totalItems: statusUpdates.length,
      message: `Syncing ${statusUpdates.length} status updates...`,
    });

    for (let i = 0; i < statusUpdates.length; i += batchSize) {
      const batch = statusUpdates.slice(i, i + batchSize);

      try {
        const payload = {
          status_updates: batch.map((s) => ({
            shipment_id: s.shipment_id,
            status: s.status,
            note: s.note,
            reason: s.reason,
            timestamp: s.recorded_at,
          })),
        };

        await api.post(API_ENDPOINTS.sync.sync, payload);

        const now = new Date().toISOString();
        await Promise.all(
          batch.map((s) =>
            db.runAsync('UPDATE status_updates SET synced_at = ? WHERE id = ?', [now, s.id])
          )
        );
        processed += batch.length;

        this.updateProgress({
          itemsProcessed: processed,
          percentage: Math.round((processed / statusUpdates.length) * 100),
        });

        await this.delay(BATCH_DELAY_MS);
      } catch (error) {
        const errorMessage = error instanceof Error ? error.message : 'Failed to sync status updates batch';
        for (const update of batch) {
          this.errors.push({
            entityType: 'status_updates',
            entityId: update.id,
            error: errorMessage,
            timestamp: new Date().toISOString(),
          });
        }
      }
    }

    return processed;
  }

  private async processSyncQueue(): Promise<{ processed: number; conflicts: number }> {
    const stats = await this.queueManager.getStats();
    const pendingCount = stats.pending + stats.failed;

    if (pendingCount === 0) {
      return { processed: 0, conflicts: 0 };
    }

    this.updateProgress({
      phase: 'processing_queue',
      totalItems: pendingCount,
      message: `Processing ${pendingCount} queued items...`,
    });

    let processed = 0;
    let conflicts = 0;

    for (const entityType of ['tracking_events', 'pods', 'status_updates']) {
      const batchSize = BATCH_SIZES[entityType as keyof typeof BATCH_SIZES] ?? 20;
      let hasMore = true;

      while (hasMore) {
        const batch = await this.queueManager.getBatchForSync(batchSize, entityType);

        if (batch.items.length === 0) {
          hasMore = false;
          continue;
        }

        const result = await this.processBatch(batch.items);
        processed += result.succeeded.length + result.failed.length;
        conflicts += result.conflicts.length;

        if (result.succeeded.length > 0) {
          await this.queueManager.markBatchSynced(result.succeeded);
        }

        if (result.failed.length > 0) {
          await this.queueManager.markBatchFailed({ succeeded: [], failed: result.failed });
        }

        for (const { item, conflict } of result.conflicts) {
          const resolved = await this.handleConflict(item, conflict);
          if (!resolved) {
            conflicts++;
            this.errors.push({
              entityType: item.entity_type,
              entityId: item.entity_id,
              error: 'Conflict resolution failed',
              timestamp: new Date().toISOString(),
            });
          }
        }

        this.updateProgress({
          itemsProcessed: processed,
          percentage: Math.round((processed / pendingCount) * 100),
        });

        await this.delay(BATCH_DELAY_MS);
      }
    }

    return { processed, conflicts };
  }

  private async processBatch(items: SyncQueueItem[]): Promise<BatchResult> {
    const result: BatchResult = {
      succeeded: [],
      failed: [],
      conflicts: [],
    };

    for (const item of items) {
      try {
        const entityData = await this.getEntityData(item.entity_type, item.entity_id);

        if (!entityData) {
          result.failed.push({ id: item.id, error: 'Entity not found' });
          continue;
        }

        const success = await this.pushEntityToServer(item, entityData);

        if (success) {
          result.succeeded.push(item.id);
        } else {
          result.failed.push({ id: item.id, error: 'Push failed' });
        }
      } catch (error) {
        const errorMessage = error instanceof Error ? error.message : 'Unknown error';
        result.failed.push({ id: item.id, error: errorMessage });
      }
    }

    return result;
  }

  private async getEntityData(entityType: string, entityId: string): Promise<Record<string, unknown> | null> {
    switch (entityType) {
      case 'tracking_events':
        const db = await getDatabase();
        return await db.getFirstAsync<Record<string, unknown>>(
          'SELECT * FROM tracking_events WHERE id = ?',
          [entityId]
        );
      case 'pods':
        const pod = await podsRepo.getById(entityId);
        return pod ? (pod as unknown as Record<string, unknown>) : null;
      case 'status_updates':
        const statusDb = await getDatabase();
        return await statusDb.getFirstAsync<Record<string, unknown>>(
          'SELECT * FROM status_updates WHERE id = ?',
          [entityId]
        );
      default:
        return null;
    }
  }

  private async pushEntityToServer(item: SyncQueueItem, data: Record<string, unknown>): Promise<boolean> {
    let retryCount = 0;

    while (retryCount < MAX_RETRIES_ON_NETWORK_ERROR) {
      try {
        const endpoint = this.getEndpointForEntityType(item.entity_type);
        await api.post(endpoint, { [item.entity_type]: [data] });
        return true;
      } catch (error) {
        if (this.isNetworkError(error)) {
          retryCount++;
          await this.delay(NETWORK_ERROR_RETRY_DELAY_MS);
          continue;
        }
        throw error;
      }
    }

    return false;
  }

  private getEndpointForEntityType(entityType: string): string {
    switch (entityType) {
      case 'tracking_events':
        return API_ENDPOINTS.sync.sync;
      case 'pods':
        return API_ENDPOINTS.sync.sync;
      case 'status_updates':
        return API_ENDPOINTS.sync.sync;
      default:
        return API_ENDPOINTS.sync.sync;
    }
  }

  private isNetworkError(error: unknown): boolean {
    if (error instanceof Error) {
      const message = error.message.toLowerCase();
      return (
        message.includes('network') ||
        message.includes('timeout') ||
        message.includes('econnrefused') ||
        message.includes('enotfound')
      );
    }
    return false;
  }

  private async handleConflict(item: SyncQueueItem, conflict: SyncConflict): Promise<boolean> {
    try {
      const resolved = this.conflictResolver.resolveConflict(conflict);

      if (resolved.strategy_used === ResolutionStrategy.MANUAL) {
        return false;
      }

      await this.applyResolvedData(item, resolved);
      return true;
    } catch {
      return false;
    }
  }

  private async applyResolvedData(item: SyncQueueItem, resolved: ResolvedData): Promise<void> {
    switch (item.entity_type) {
      case 'tracking_events':
        await trackingEventsRepo.update(item.entity_id, resolved.resolved_value as Partial<trackingEventsRepo.TrackingEvent>);
        break;
      case 'pods':
        await podsRepo.update(item.entity_id, resolved.resolved_value as Partial<podsRepo.POD>);
        break;
      case 'shipments':
        await shipmentsRepo.update(item.entity_id, resolved.resolved_value as Partial<shipmentsRepo.Shipment>);
        break;
    }
  }

  private async getLastSyncTimestamp(): Promise<Date | undefined> {
    const metadata = await syncMetadataRepo.get();
    return metadata?.last_sync_at ? new Date(metadata.last_sync_at) : undefined;
  }

  private updateProgress(updates: Partial<SyncProgress>): void {
    this.syncProgress = { ...this.syncProgress, ...updates };
    this.notifyProgressCallbacks();
  }

  private notifyProgressCallbacks(): void {
    for (const callback of this.progressCallbacks) {
      try {
        callback(this.syncProgress);
      } catch {
        // Ignore callback errors
      }
    }
  }

  onProgress(callback: ProgressCallback): () => void {
    this.progressCallbacks.add(callback);
    return () => {
      this.progressCallbacks.delete(callback);
    };
  }

  getProgress(): SyncProgress {
    return { ...this.syncProgress };
  }

  isCurrentlySyncing(): boolean {
    return this.isSyncing;
  }

  async fullSync(): Promise<SyncResult> {
    return this.sync({ forceFullSync: true });
  }

  async getQueueStats(): Promise<QueueStats> {
    return this.queueManager.getStats();
  }

  async getPendingCount(): Promise<number> {
    const stats = await this.queueManager.getStats();
    return stats.pending + stats.failed;
  }

  getErrors(): SyncError[] {
    return [...this.errors];
  }

  clearErrors(): void {
    this.errors = [];
  }

  private delay(ms: number): Promise<void> {
    return new Promise((resolve) => setTimeout(resolve, ms));
  }
}

export const syncService = SyncService.getInstance();
export default SyncService;
