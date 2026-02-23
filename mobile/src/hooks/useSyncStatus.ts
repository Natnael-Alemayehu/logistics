import { useEffect, useCallback, useState } from 'react';
import { useSyncStore } from '@/store/syncStore';
import { useConnectionQuality } from './useConnectionQuality';
import { syncService, SyncError, SyncProgress } from '@/services/sync/SyncService';
import * as syncMetadataRepo from '@/db/repositories/syncMetadata';

export type SyncStatus = 'idle' | 'syncing' | 'success' | 'error' | 'offline';

interface UseSyncStatusResult {
  syncStatus: SyncStatus;
  pendingCount: number;
  lastSyncAt: Date | null;
  syncProgress: number;
  currentSyncEntity: string | null;
  errors: SyncError[];
  isSyncing: boolean;
  canSync: boolean;
  syncProgressDetail: SyncProgress;
  triggerSync: () => Promise<void>;
  triggerFullSync: () => Promise<void>;
  clearErrors: () => void;
}

export function useSyncStatus(): UseSyncStatusResult {
  const {
    isSyncing,
    lastSyncAt,
    pendingCount,
    syncProgress,
    lastError,
    errors,
    sync,
    fullSync,
    clearErrors,
    getPendingCount,
  } = useSyncStore();

  const { isOffline, shouldDeferSync } = useConnectionQuality();
  const [localLastSyncAt, setLocalLastSyncAt] = useState<Date | null>(null);

  const syncStatus: SyncStatus = isOffline
    ? 'offline'
    : isSyncing
    ? 'syncing'
    : lastError
    ? 'error'
    : pendingCount === 0 && lastSyncAt
    ? 'success'
    : 'idle';

  const canSync = !isOffline && !isSyncing && !shouldDeferSync;

  const triggerSync = useCallback(async () => {
    if (!canSync) return;

    try {
      await sync();
    } catch (error) {
      console.error('Sync failed:', error);
    }
  }, [canSync, sync]);

  const triggerFullSync = useCallback(async () => {
    if (!canSync) return;

    try {
      await fullSync();
    } catch (error) {
      console.error('Full sync failed:', error);
    }
  }, [canSync, fullSync]);

  useEffect(() => {
    const loadLastSyncAt = async () => {
      const metadata = await syncMetadataRepo.get();
      if (metadata?.last_sync_at) {
        setLocalLastSyncAt(new Date(metadata.last_sync_at));
      }
    };

    loadLastSyncAt();
  }, []);

  useEffect(() => {
    if (lastSyncAt) {
      setLocalLastSyncAt(new Date(lastSyncAt));
    }
  }, [lastSyncAt]);

  useEffect(() => {
    getPendingCount();
  }, [getPendingCount]);

  useEffect(() => {
    const unsubscribe = syncService.onProgress((progress) => {
      // Progress is already handled by the store
    });

    return () => {
      unsubscribe();
    };
  }, []);

  return {
    syncStatus,
    pendingCount,
    lastSyncAt: localLastSyncAt,
    syncProgress: syncProgress.percentage,
    currentSyncEntity: syncProgress.currentEntity,
    errors,
    isSyncing,
    canSync,
    syncProgressDetail: syncProgress,
    triggerSync,
    triggerFullSync,
    clearErrors,
  };
}

export default useSyncStatus;
