import { useState, useCallback } from 'react';
import {
  performInitialSync,
  InitialSyncResult,
  InitialSyncProgress,
} from '@/services/initialSync';

interface InitialSyncState {
  isInitialSyncing: boolean;
  initialSyncProgress: number;
  initialSyncMessage: string;
  initialSyncError: string | null;
  initialSyncResult: InitialSyncResult | null;
}

export function useInitialSync() {
  const [state, setState] = useState<InitialSyncState>({
    isInitialSyncing: false,
    initialSyncProgress: 0,
    initialSyncMessage: '',
    initialSyncError: null,
    initialSyncResult: null,
  });

  const startInitialSync = useCallback(async (driverId: string): Promise<InitialSyncResult> => {
    setState((prev) => ({
      ...prev,
      isInitialSyncing: true,
      initialSyncProgress: 0,
      initialSyncMessage: 'Starting sync...',
      initialSyncError: null,
      initialSyncResult: null,
    }));

    const result = await performInitialSync(driverId, (progress: InitialSyncProgress) => {
      setState((prev) => ({
        ...prev,
        initialSyncProgress: progress.percentage,
        initialSyncMessage: progress.message,
      }));
    });

    setState((prev) => ({
      ...prev,
      isInitialSyncing: false,
      initialSyncProgress: 100,
      initialSyncMessage: result.success
        ? `Synced ${result.shipmentsCount} shipments`
        : 'Sync failed',
      initialSyncError: result.error ?? null,
      initialSyncResult: result,
    }));

    return result;
  }, []);

  const retryInitialSync = useCallback(async (driverId: string): Promise<void> => {
    await startInitialSync(driverId);
  }, [startInitialSync]);

  const reset = useCallback(() => {
    setState({
      isInitialSyncing: false,
      initialSyncProgress: 0,
      initialSyncMessage: '',
      initialSyncError: null,
      initialSyncResult: null,
    });
  }, []);

  return {
    isInitialSyncing: state.isInitialSyncing,
    initialSyncProgress: state.initialSyncProgress,
    initialSyncMessage: state.initialSyncMessage,
    initialSyncError: state.initialSyncError,
    initialSyncResult: state.initialSyncResult,
    startInitialSync,
    retryInitialSync,
    reset,
  };
}
