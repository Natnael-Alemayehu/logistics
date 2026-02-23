import * as shipmentsApi from './api/shipments';
import * as shipmentsDb from '@/db/repositories/shipments';
import * as syncMetadataRepo from '@/db/repositories/syncMetadata';
import { syncService } from './sync/SyncService';

export interface InitialSyncResult {
  success: boolean;
  shipmentsCount: number;
  error?: string;
  fromCache: boolean;
}

export interface InitialSyncProgress {
  phase: 'idle' | 'fetching' | 'caching' | 'completed' | 'error';
  message: string;
  percentage: number;
}

export type ProgressCallback = (progress: InitialSyncProgress) => void;

export async function performInitialSync(
  driverId: string,
  onProgress?: ProgressCallback
): Promise<InitialSyncResult> {
  const updateProgress = (progress: InitialSyncProgress) => {
    onProgress?.(progress);
  };

  updateProgress({
    phase: 'fetching',
    message: 'Fetching your shipments...',
    percentage: 0,
  });

  try {
    await syncMetadataRepo.upsert({ driver_id: driverId });

    const isOnline = await checkConnectivity();
    
    if (!isOnline) {
      updateProgress({
        phase: 'caching',
        message: 'Loading cached data...',
        percentage: 50,
      });

      const cachedShipments = await shipmentsDb.getByDriverId(driverId);
      
      updateProgress({
        phase: 'completed',
        message: 'Loaded from cache (offline)',
        percentage: 100,
      });

      return {
        success: true,
        shipmentsCount: cachedShipments.length,
        fromCache: true,
      };
    }

    updateProgress({
      phase: 'fetching',
      message: 'Fetching shipments from server...',
      percentage: 20,
    });

    const apiShipments = await shipmentsApi.getMyShipments();

    updateProgress({
      phase: 'caching',
      message: `Caching ${apiShipments.length} shipments...`,
      percentage: 60,
    });

    if (apiShipments.length > 0) {
      await shipmentsDb.upsertMany(
        apiShipments.map((s) => ({
          id: s.id,
          tracking_number: s.tracking_number,
          origin_address: s.origin,
          destination_address: s.destination,
          customer_name: s.customer_name,
          customer_phone: s.customer_phone,
          status: s.status,
          driver_id: driverId,
          created_at: s.created_at,
          updated_at: s.updated_at,
        }))
      );
    }

    updateProgress({
      phase: 'caching',
      message: 'Updating sync metadata...',
      percentage: 80,
    });

    await syncMetadataRepo.upsert({
      driver_id: driverId,
      last_sync_at: new Date().toISOString(),
    });

    updateProgress({
      phase: 'completed',
      message: 'Initial sync completed',
      percentage: 100,
    });

    return {
      success: true,
      shipmentsCount: apiShipments.length,
      fromCache: false,
    };
  } catch (error) {
    const errorMessage = error instanceof Error ? error.message : 'Initial sync failed';
    
    updateProgress({
      phase: 'error',
      message: errorMessage,
      percentage: 100,
    });

    const cachedShipments = await shipmentsDb.getByDriverId(driverId);
    if (cachedShipments.length > 0) {
      return {
        success: true,
        shipmentsCount: cachedShipments.length,
        fromCache: true,
        error: errorMessage,
      };
    }

    return {
      success: false,
      shipmentsCount: 0,
      error: errorMessage,
      fromCache: false,
    };
  }
}

export async function hasCachedData(driverId: string): Promise<boolean> {
  const shipments = await shipmentsDb.getByDriverId(driverId);
  return shipments.length > 0;
}

export async function getCachedShipmentsCount(driverId: string): Promise<number> {
  const shipments = await shipmentsDb.getByDriverId(driverId);
  return shipments.length;
}

async function checkConnectivity(): Promise<boolean> {
  try {
    const NetInfo = require('@react-native-community/netinfo').default;
    const state = await NetInfo.fetch();
    return state.isConnected === true && state.isInternetReachable !== false;
  } catch {
    return false;
  }
}

export async function performBackgroundSync(): Promise<void> {
  try {
    const metadata = await syncMetadataRepo.get();
    if (!metadata?.driver_id) return;

    const isOnline = await checkConnectivity();
    if (!isOnline) return;

    await syncService.sync({ skipPush: true });
  } catch {
    // Silently fail background sync
  }
}
