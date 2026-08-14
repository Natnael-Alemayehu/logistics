import { create } from 'zustand';
import { syncService, SyncProgress, SyncResult, SyncError } from '@/services/sync/SyncService';

interface SyncState {
  isSyncing: boolean;
  lastSyncAt: string | null;
  pendingCount: number;
  syncProgress: SyncProgress;
  lastError: string | null;
  errors: SyncError[];

  setSyncing: (isSyncing: boolean) => void;
  setLastSyncAt: (time: string) => void;
  setPendingCount: (count: number) => void;
  setProgress: (progress: SyncProgress) => void;
  setError: (error: string | null) => void;
  setErrors: (errors: SyncError[]) => void;
  sync: () => Promise<SyncResult>;
  fullSync: () => Promise<SyncResult>;
  getPendingCount: () => Promise<number>;
  clearErrors: () => void;
}

let progressUnsubscribe: (() => void) | null = null;

export const useSyncStore = create<SyncState>((set, get) => ({
  isSyncing: false,
  lastSyncAt: null,
  pendingCount: 0,
  syncProgress: {
    phase: 'idle',
    currentEntity: null,
    itemsProcessed: 0,
    totalItems: 0,
    percentage: 0,
    message: 'Ready to sync',
  },
  lastError: null,
  errors: [],

  setSyncing: (isSyncing) => {
    set({ isSyncing });
  },

  setLastSyncAt: (time) => {
    set({ lastSyncAt: time });
  },

  setPendingCount: (count) => {
    set({ pendingCount: count });
  },

  setProgress: (progress) => {
    set({ syncProgress: progress });
  },

  setError: (error) => {
    set({ lastError: error });
  },

  setErrors: (errors) => {
    set({ errors });
  },

  clearErrors: () => {
    syncService.clearErrors();
    set({ errors: [], lastError: null });
  },

  getPendingCount: async () => {
    try {
      const count = await syncService.getPendingCount();
      set({ pendingCount: count });
      return count;
    } catch {
      return 0;
    }
  },

  sync: async () => {
    const { isSyncing } = get();
    if (isSyncing) {
      return {
        success: false,
        pulled: 0,
        pushed: 0,
        conflicts: 0,
        errors: [],
        duration: 0,
        syncTime: new Date().toISOString(),
      };
    }

    set({ isSyncing: true, lastError: null, errors: [] });

    if (progressUnsubscribe) {
      progressUnsubscribe();
    }

    progressUnsubscribe = syncService.onProgress((progress) => {
      set({ syncProgress: progress });
    });

    try {
      const result = await syncService.sync();

      if (result.success) {
        set({
          lastSyncAt: result.syncTime,
          pendingCount: await syncService.getPendingCount(),
        });
      } else {
        set({
          lastError: result.errors[0]?.error ?? 'Sync failed',
          errors: result.errors,
        });
      }

      return result;
    } catch (error) {
      const errorMessage = error instanceof Error ? error.message : 'Sync failed';
      set({ lastError: errorMessage });
      throw error;
    } finally {
      if (progressUnsubscribe) {
        progressUnsubscribe();
        progressUnsubscribe = null;
      }
      set({ isSyncing: false });
    }
  },

  fullSync: async () => {
    const { isSyncing } = get();
    if (isSyncing) {
      return {
        success: false,
        pulled: 0,
        pushed: 0,
        conflicts: 0,
        errors: [],
        duration: 0,
        syncTime: new Date().toISOString(),
      };
    }

    set({ isSyncing: true, lastError: null, errors: [] });

    if (progressUnsubscribe) {
      progressUnsubscribe();
    }

    progressUnsubscribe = syncService.onProgress((progress) => {
      set({ syncProgress: progress });
    });

    try {
      const result = await syncService.fullSync();

      if (result.success) {
        set({
          lastSyncAt: result.syncTime,
          pendingCount: await syncService.getPendingCount(),
        });
      } else {
        set({
          lastError: result.errors[0]?.error ?? 'Full sync failed',
          errors: result.errors,
        });
      }

      return result;
    } catch (error) {
      const errorMessage = error instanceof Error ? error.message : 'Full sync failed';
      set({ lastError: errorMessage });
      throw error;
    } finally {
      if (progressUnsubscribe) {
        progressUnsubscribe();
        progressUnsubscribe = null;
      }
      set({ isSyncing: false });
    }
  },
}));
