import { useCallback, useEffect } from 'react';
import { useSyncStore } from '@/store/syncStore';
import { useConnectivity } from './useConnectivity';

export function useSync() {
  const {
    isSyncing,
    lastSyncAt,
    pendingCount,
    syncProgress,
    lastError,
    sync,
    getPendingCount,
    setSyncing,
    setError,
  } = useSyncStore();

  const { isOnline } = useConnectivity();

  useEffect(() => {
    getPendingCount();
  }, [getPendingCount]);

  const triggerSync = useCallback(async () => {
    if (!isOnline) {
      setError('Cannot sync while offline');
      return { success: false, error: 'Cannot sync while offline' };
    }

    if (isSyncing) {
      return { success: false, error: 'Sync already in progress' };
    }

    try {
      await sync();
      return { success: true };
    } catch (error) {
      return {
        success: false,
        error: error instanceof Error ? error.message : 'Sync failed',
      };
    }
  }, [isOnline, isSyncing, sync, setError]);

  const syncState = {
    isSyncing,
    progress: syncProgress,
    pendingCount,
    lastSyncAt,
    lastError,
    isOnline,
  };

  return {
    ...syncState,
    triggerSync,
    getPendingCount,
    refreshPendingCount: getPendingCount,
  };
}
