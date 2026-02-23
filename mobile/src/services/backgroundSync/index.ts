import * as BackgroundFetch from 'expo-background-fetch';
import * as TaskManager from 'expo-task-manager';
import {
  getDatabase,
  getPendingTrackingEvents,
  markTrackingEventSynced,
  getPendingPODs,
  markPODSynced,
} from '@db';

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

    if (unsyncedEvents.length > 0) {
      try {
        const now = new Date().toISOString();
        for (const event of unsyncedEvents) {
          await markTrackingEventSynced(event.id, now);
        }
        result.syncedEvents = unsyncedEvents.length;
      } catch (error) {
        result.errors.push(`Failed to sync events: ${error}`);
      }
    }

    if (unsyncedPODs.length > 0) {
      try {
        const now = new Date().toISOString();
        for (const pod of unsyncedPODs) {
          await markPODSynced(pod.id, now);
          result.syncedPODs++;
        }
      } catch (error) {
        result.errors.push(`Failed to sync PODs: ${error}`);
      }
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
