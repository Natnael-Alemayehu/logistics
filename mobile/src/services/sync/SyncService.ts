import { SyncQueueManager, syncQueueManager } from './SyncQueueManager';
import { conflictResolver } from './ConflictResolver';
import ConflictResolver from './ConflictResolver';
import { SyncQueueItem, SyncBatch, SyncBatchResult, QueueStats } from './types';
import { SyncItemResult, SyncResponse, indexResults, isPersisted } from './contract';
import { readPhotoAsBase64 } from '@/services/photos';
import type { PODPhoto } from '@/types/pod';
import { api } from '@/services/api/client';
import { API_ENDPOINTS } from '@/services/constants';
import * as trackingEventsRepo from '@/db/repositories/trackingEvents';
import * as podsRepo from '@/db/repositories/pods';
import * as shipmentsRepo from '@/db/repositories/shipments';
import * as statusUpdatesRepo from '@/db/repositories/statusUpdates';
import * as syncMetadataRepo from '@/db/repositories/syncMetadata';
import { ResolutionStrategy, SyncConflict, ResolvedData } from '@/types/conflict';
import { getDatabase } from '@/db/database';
import { useSettingsStore } from '@/store/settingsStore';
import { getCurrentConnectionInfo, ConnectionInfo } from '@/services/connectivity';
import Constants from 'expo-constants';

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

  private getDeviceId(): string {
    return Constants.deviceId || Constants.sessionId || 'unknown';
  }

  private async getSyncDeviceId(): Promise<string> {
    const metadata = await syncMetadataRepo.get();
    if (metadata?.device_id) {
      return metadata.device_id;
    }
    const deviceId = this.getDeviceId();
    await syncMetadataRepo.upsert({ device_id: deviceId });
    return deviceId;
  }

  private getBatteryLevel(): number | null {
    return null;
  }

  private getStorageRemainingKb(): number | null {
    return null;
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

  private async canSync(): Promise<{ allowed: boolean; reason?: string }> {
    const settings = useSettingsStore.getState();
    
    if (!settings.wifiOnlySync) {
      return { allowed: true };
    }

    const connectionInfo = await getCurrentConnectionInfo();
    
    if (connectionInfo.connectionType === 'wifi') {
      return { allowed: true };
    }

    return { 
      allowed: false, 
      reason: 'WiFi-only sync is enabled. Waiting for WiFi connection.' 
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

    const syncCheck = await this.canSync();
    if (!syncCheck.allowed) {
      return {
        success: false,
        pulled: 0,
        pushed: 0,
        conflicts: 0,
        errors: [{ entityType: 'system', entityId: 'sync', error: syncCheck.reason ?? 'Sync not allowed', timestamp: new Date().toISOString() }],
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
      // Push before pull. pullChanges upserts server rows over local ones, so
      // pulling first would overwrite local edits that have not been sent yet
      // and the driver's work would vanish before it was ever transmitted.
      if (!options?.skipPush) {
        this.updateProgress({ phase: 'pushing', message: 'Pushing local changes...' });
        const pushResult = await this.pushChanges(options?.entityTypes);
        pushed = pushResult.trackingEvents + pushResult.pods + pushResult.statusUpdates;
      }

      if (!options?.skipPull) {
        this.updateProgress({ phase: 'pulling', message: 'Pulling changes from server...' });
        const pullResult = await this.pullChanges(
          options?.forceFullSync ? undefined : await this.getLastSyncTimestamp()
        );
        pulled = pullResult.shipments + pullResult.deletedShipments;
      }

      this.updateProgress({ phase: 'processing_queue', message: 'Processing sync queue...' });
      const queueResult = await this.processSyncQueue();
      conflicts = queueResult.conflicts;

      const syncTime = new Date().toISOString();

      // Anything still unsent means the device and server disagree, so the
      // cursor must not move: advancing it would skip the server-side changes
      // that arrive between now and the next successful sync. Reporting success
      // here is what let syncStore clear the error banner after a total failure.
      const succeeded = this.errors.length === 0;
      if (succeeded) {
        await syncMetadataRepo.updateLastSync(new Date(syncTime));
      }

      this.updateProgress({
        phase: succeeded ? 'completed' : 'error',
        message: succeeded
          ? 'Sync completed successfully'
          : `Sync finished with ${this.errors.length} unsynced item(s)`,
        percentage: 100,
      });

      return {
        success: succeeded,
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
      device_id: await this.getSyncDeviceId(),
      battery_level: this.getBatteryLevel(),
      last_sync_at: since?.toISOString(),
    };

    try {
      const response = await api.post<{
        sync_time?: string;
        server_time?: string;
        shipments?: shipmentsRepo.Shipment[];
        pull?: {
          shipments?: shipmentsRepo.Shipment[];
          deleted_shipment_ids?: string[];
        };
        deleted_shipment_ids?: string[];
      }>(API_ENDPOINTS.sync.sync, payload);

      let shipmentsCount = 0;
      let deletedCount = 0;

      const shipments = response.shipments ?? response.pull?.shipments ?? [];
      const deletedIds = response.deleted_shipment_ids ?? response.pull?.deleted_shipment_ids ?? [];

      if (shipments.length) {
        await shipmentsRepo.upsertMany(shipments);
        shipmentsCount = shipments.length;
      }

      if (deletedIds.length) {
        await shipmentsRepo.batchDelete(deletedIds);
        deletedCount = deletedIds.length;
      }

      const syncTime = response.sync_time ?? response.server_time ?? new Date().toISOString();

      return {
        shipments: shipmentsCount,
        deletedShipments: deletedCount,
        timestamp: syncTime,
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

  /**
   * Applies the server's per-item verdicts to a batch.
   *
   * Only rows the server confirmed are marked synced. Anything it rejected keeps
   * synced_at NULL and records the reason, and anything missing from the
   * response is left untouched so the next sync retries it. Marking a whole
   * batch synced because the request did not throw is what silently discarded
   * drivers' work.
   */
  private async applyResults(
    entityType: 'tracking_events' | 'pods' | 'status_updates',
    localIds: string[],
    results: SyncItemResult[] | undefined,
    markSynced: (id: string, syncedAt: string) => Promise<void>,
    markRejected: (id: string, error: string) => Promise<void>
  ): Promise<number> {
    const byLocalId = indexResults(results, localIds);
    const syncedAt = new Date().toISOString();
    let persisted = 0;

    for (const localId of localIds) {
      const result = byLocalId.get(localId);

      if (!result) {
        // No verdict for this row. It stays pending and is retried; assuming
        // success here is exactly the bug being fixed.
        this.errors.push({
          entityType,
          entityId: localId,
          error: 'server returned no result for this item',
          timestamp: syncedAt,
        });
        continue;
      }

      if (isPersisted(result)) {
        await markSynced(localId, syncedAt);
        persisted++;
        continue;
      }

      const reason = result.message ?? result.code ?? 'rejected by server';
      await markRejected(localId, `${result.code ?? 'rejected'}: ${reason}`);
      this.errors.push({
        entityType,
        entityId: localId,
        error: reason,
        timestamp: syncedAt,
      });
    }

    return persisted;
  }

  /** Records a transport-level failure; the batch stays pending for retry. */
  private recordBatchFailure(
    entityType: 'tracking_events' | 'pods' | 'status_updates',
    localIds: string[],
    error: unknown,
    fallbackMessage: string
  ): void {
    const message = error instanceof Error ? error.message : fallbackMessage;
    const timestamp = new Date().toISOString();

    for (const localId of localIds) {
      this.errors.push({ entityType, entityId: localId, error: message, timestamp });
    }
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
      const localIds = batch.map((e) => e.id);

      try {
        const payload = {
          device_id: await this.getSyncDeviceId(),
          battery_level: this.getBatteryLevel(),
          storage_remaining_kb: this.getStorageRemainingKb(),
          events: batch.map((e) => ({
            // The local row id doubles as the server's dedup key, so replaying a
            // batch whose response was lost cannot duplicate telemetry.
            client_id: e.id,
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
          })),
        };

        const response = await api.post<SyncResponse>(API_ENDPOINTS.sync.sync, payload);

        processed += await this.applyResults(
          'tracking_events',
          localIds,
          response?.events,
          trackingEventsRepo.markSynced,
          trackingEventsRepo.markRejected
        );

        this.updateProgress({
          itemsProcessed: processed,
          percentage: Math.round((processed / events.length) * 100),
        });

        await this.delay(BATCH_DELAY_MS);
      } catch (error) {
        this.recordBatchFailure('tracking_events', localIds, error, 'Failed to sync tracking events batch');
      }
    }

    return processed;
  }

  /**
   * Reads a captured photo back as base64 for upload.
   *
   * Photos previously went to the server as their localUri — a file:// path
   * meaningful only on the device — so proof of delivery arrived as unusable
   * paths and the images never left the handset. Already-uploaded photos are
   * sent as their remote URL instead, which the server passes through.
   */
  private async encodePhoto(photo: PODPhoto): Promise<string> {
    if (photo.uploaded && photo.remoteUrl) {
      return photo.remoteUrl;
    }
    return readPhotoAsBase64(photo.localUri);
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
        const photoPayloads = await Promise.all(pod.photos.map((photo) => this.encodePhoto(photo)));

        const payload = {
          device_id: await this.getSyncDeviceId(),
          battery_level: this.getBatteryLevel(),
          storage_remaining_kb: this.getStorageRemainingKb(),
          pods: [{
            client_id: pod.id,
            shipment_id: pod.shipmentId,
            recipient_name: pod.recipientName,
            recipient_phone: pod.recipientPhone,
            signature_data: pod.signatureData,
            photo_urls: photoPayloads,
            delivery_address: pod.deliveryAddress,
            delivery_lat: pod.deliveryLat ?? 0,
            delivery_lng: pod.deliveryLng ?? 0,
            delivery_notes: pod.deliveryNotes,
            location_verified: pod.locationVerified,
            location_mismatch_meters: pod.locationMismatchMeters,
            recorded_at: pod.recordedAt,
          }],
        };

        const response = await api.post<SyncResponse>(API_ENDPOINTS.sync.sync, payload);

        processed += await this.applyResults(
          'pods',
          [pod.id],
          response?.pods,
          podsRepo.markSynced,
          podsRepo.markRejected
        );

        this.updateProgress({
          itemsProcessed: processed,
          percentage: Math.round((processed / pendingPODs.length) * 100),
        });

        await this.delay(BATCH_DELAY_MS);
      } catch (error) {
        // Left pending rather than marked failed: a lost connection is not the
        // POD's fault, and the delivery evidence must survive to be retried.
        this.recordBatchFailure('pods', [pod.id], error, 'Failed to sync POD');
      }
    }

    return processed;
  }

  private async pushStatusUpdates(): Promise<number> {
    // Uses the repository rather than hand-rolled SQL so the pending-selection
    // rule lives in one place.
    const statusUpdates = await statusUpdatesRepo.getPendingSync();
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
      const localIds = batch.map((s) => s.id);

      try {
        const payload = {
          device_id: await this.getSyncDeviceId(),
          battery_level: this.getBatteryLevel(),
          storage_remaining_kb: this.getStorageRemainingKb(),
          statuses: batch.map((s) => ({
            client_id: s.id,
            shipment_id: s.shipment_id,
            status: s.status,
            note: s.note,
            reason: s.reason,
            // The server compares this against the shipment's last status change
            // and rejects updates it has already moved past.
            recorded_at: s.recorded_at,
          })),
        };

        const response = await api.post<SyncResponse>(API_ENDPOINTS.sync.sync, payload);

        processed += await this.applyResults(
          'status_updates',
          localIds,
          response?.statuses,
          statusUpdatesRepo.markSynced,
          statusUpdatesRepo.markRejected
        );

        this.updateProgress({
          itemsProcessed: processed,
          percentage: Math.round((processed / statusUpdates.length) * 100),
        });

        await this.delay(BATCH_DELAY_MS);
      } catch (error) {
        this.recordBatchFailure('status_updates', localIds, error, 'Failed to sync status updates batch');
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
        await podsRepo.update(item.entity_id, resolved.resolved_value as Partial<podsRepo.ProofOfDelivery>);
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
