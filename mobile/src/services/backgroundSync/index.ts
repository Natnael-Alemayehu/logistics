import * as BackgroundFetch from 'expo-background-fetch';
import * as TaskManager from 'expo-task-manager';
import {
  getDatabase,
  getPendingTrackingEvents,
  markTrackingEventSynced,
  getPendingPODs,
  markPODSynced,
  upsertShipments,
} from '@db';
import { api } from '@/services/api';
import { API_ENDPOINTS } from '@/services/constants';

const SYNC_TASK_NAME = 'background-sync';

interface SyncResult {
  success: boolean;
  syncedEvents: number;
  syncedPODs: number;
  errors: string[];
}

export async function registerBackgroundSync(): Promise<void> {
  const status = await BackgroundFetch.getStatusAsync();
  if (status === BackgroundFetch.BackgroundFetchStatus.Available) {
    await BackgroundFetch.registerTaskAsync(SYNC_TASK_NAME, {
      minimumInterval: 60 * 15,
      stopOnTerminate: false,
      startOnBoot: true,
    });
  }
}

export async function unregisterBackgroundSync(): Promise<void> {
  await BackgroundFetch.unregisterTaskAsync(SYNC_TASK_NAME);
}

export async function performSync(): Promise<SyncResult> {
  const result: SyncResult = {
    success: true,
    syncedEvents: 0,
    syncedPODs: 0,
    errors: [],
  };

  try {
    await getDatabase();
    
    const unsyncedEvents = await getPendingTrackingEvents();
    const unsyncedPODs = await getPendingPODs();

    if (unsyncedEvents.length === 0 && unsyncedPODs.length === 0) {
      return result;
    }

    const syncPayload = {
      device_id: 'mobile-device',
      last_sync_at: null,
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
      pods: unsyncedPODs.map((p) => ({
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

    try {
      const response = await api.post<{
        server_time: string;
        pull?: { shipments?: Array<Record<string, unknown>> };
      }>(API_ENDPOINTS.sync.sync, syncPayload);

      const now = response.server_time ?? new Date().toISOString();

      // Mark events as synced
      for (const event of unsyncedEvents) {
        await markTrackingEventSynced(event.id, now);
      }
      result.syncedEvents = unsyncedEvents.length;

      // Mark PODs as synced
      for (const pod of unsyncedPODs) {
        await markPODSynced(pod.id, now);
      }
      result.syncedPODs = unsyncedPODs.length;

      // Pull new shipments if any
      if (response.pull?.shipments?.length) {
        await upsertShipments(response.pull.shipments as any);
      }
    } catch (error) {
      result.errors.push(`API sync failed: ${error}`);
    }

    result.success = result.errors.length === 0;
  } catch (error) {
    result.success = false;
    result.errors.push(`Sync failed: ${error}`);
  }

  return result;
}

TaskManager.defineTask(SYNC_TASK_NAME, async () => {
  const result = await performSync();
  
  if (result.success) {
    return BackgroundFetch.BackgroundFetchResult.NewData;
  } else {
    return BackgroundFetch.BackgroundFetchResult.Failed;
  }
});
