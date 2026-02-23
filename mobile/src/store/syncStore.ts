import { create } from 'zustand';
import { api } from '@/services/api';
import * as trackingEvents from '@/db/repositories/trackingEvents';
import * as pods from '@/db/repositories/pods';
import * as shipments from '@/db/repositories/shipments';

interface SyncState {
  isSyncing: boolean;
  lastSyncAt: string | null;
  pendingCount: number;
  syncProgress: number;
  lastError: string | null;

  setSyncing: (isSyncing: boolean) => void;
  setLastSyncAt: (time: string) => void;
  setPendingCount: (count: number) => void;
  setProgress: (progress: number) => void;
  setError: (error: string | null) => void;
  sync: () => Promise<void>;
  getPendingCount: () => Promise<number>;
}

export const useSyncStore = create<SyncState>((set, get) => ({
  isSyncing: false,
  lastSyncAt: null,
  pendingCount: 0,
  syncProgress: 0,
  lastError: null,

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

  getPendingCount: async () => {
    try {
      const trackingCount = await trackingEvents.getUnsyncedCount();
      const pendingPods = await pods.getPendingSync();
      const total = trackingCount + pendingPods.length;
      set({ pendingCount: total });
      return total;
    } catch {
      return 0;
    }
  },

  sync: async () => {
    const { isSyncing } = get();
    if (isSyncing) return;

    set({ isSyncing: true, syncProgress: 0, lastError: null });

    try {
      set({ syncProgress: 10 });

      const [unsyncedEvents, unsyncedPods] = await Promise.all([
        trackingEvents.getPendingSync(),
        pods.getPendingSync(),
      ]);

      set({ syncProgress: 30 });

      const syncPayload = {
        device_id: 'mobile-device',
        last_sync_at: get().lastSyncAt,
        events: unsyncedEvents.map((e) => ({
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
        })),
        pods: unsyncedPods.map((p) => ({
          shipment_id: p.shipment_id,
          recipient_name: p.recipient_name,
          recipient_phone: p.recipient_phone,
          signature_data: p.signature_data,
          photo_urls: p.photo_paths ?? [],
          delivery_address: p.delivery_address,
          delivery_lat: p.delivery_lat ?? 0,
          delivery_lng: p.delivery_lng ?? 0,
          delivery_notes: p.delivery_notes,
          location_verified: p.location_verified,
          location_mismatch_meters: p.location_mismatch_meters,
          recorded_at: p.recorded_at,
        })),
        statuses: [],
        battery_level: 100,
        storage_remaining_kb: 0,
      };

      set({ syncProgress: 50 });

      const response = await api.post<{
        server_time: string;
        pull: { shipments: shipments.Shipment[] };
      }>('/sync', syncPayload);

      set({ syncProgress: 70 });

      const now = new Date().toISOString();
      await Promise.all([
        ...unsyncedEvents.map((e) => trackingEvents.markSynced(e.id, now)),
        ...unsyncedPods.map((p) => pods.markSynced(p.id, now)),
      ]);

      if (response.pull?.shipments?.length) {
        await shipments.upsertMany(response.pull.shipments);
      }

      set({
        syncProgress: 100,
        lastSyncAt: response.server_time ?? now,
        pendingCount: 0,
      });
    } catch (error) {
      set({
        lastError: error instanceof Error ? error.message : 'Sync failed',
        syncProgress: 0,
      });
      throw error;
    } finally {
      set({ isSyncing: false });
    }
  },
}));
