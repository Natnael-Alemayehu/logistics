import * as FileSystem from 'expo-file-system/legacy';
import { getDatabase } from '@/db/database';
import * as trackingEventsRepo from '@/db/repositories/trackingEvents';
import * as podsRepo from '@/db/repositories/pods';

export interface CacheInfo {
  totalSize: number;
  photosSize: number;
  databaseSize: number;
  trackingEventsCount: number;
  syncedEventsCount: number;
  podsCount: number;
  formattedSize: string;
}

export interface ClearCacheOptions {
  clearSyncedEvents?: boolean;
  clearOldEventsDays?: number;
  clearPhotos?: boolean;
}

const PHOTOS_DIR = `${FileSystem.documentDirectory}photos`;
const DB_PATH = `${FileSystem.documentDirectory}SQLite/logistics.db`;

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(1))} ${sizes[i]}`;
}

class CacheService {
  private static instance: CacheService;

  private constructor() {}

  static getInstance(): CacheService {
    if (!CacheService.instance) {
      CacheService.instance = new CacheService();
    }
    return CacheService.instance;
  }

  async getCacheInfo(): Promise<CacheInfo> {
    let photosSize = 0;
    let databaseSize = 0;
    let trackingEventsCount = 0;
    let syncedEventsCount = 0;
    let podsCount = 0;

    try {
      const photosDirInfo = await FileSystem.getInfoAsync(PHOTOS_DIR);
      if (photosDirInfo.exists && 'size' in photosDirInfo) {
        photosSize = photosDirInfo.size ?? 0;
      }
    } catch {
      photosSize = 0;
    }

    try {
      const dbInfo = await FileSystem.getInfoAsync(DB_PATH);
      if (dbInfo.exists && 'size' in dbInfo) {
        databaseSize = dbInfo.size ?? 0;
      }
    } catch {
      databaseSize = 0;
    }

    try {
      const counts = await trackingEventsRepo.getCountsBySyncStatus();
      trackingEventsCount = counts.synced + counts.unsynced;
      syncedEventsCount = counts.synced;
    } catch {
      trackingEventsCount = 0;
      syncedEventsCount = 0;
    }

    try {
      const pods = await podsRepo.getAll();
      podsCount = pods.length;
    } catch {
      podsCount = 0;
    }

    const totalSize = photosSize + databaseSize;

    return {
      totalSize,
      photosSize,
      databaseSize,
      trackingEventsCount,
      syncedEventsCount,
      podsCount,
      formattedSize: formatBytes(totalSize),
    };
  }

  async clearCache(options: ClearCacheOptions = {}): Promise<{
    clearedEvents: number;
    clearedPhotos: number;
    freedBytes: number;
  }> {
    const result = {
      clearedEvents: 0,
      clearedPhotos: 0,
      freedBytes: 0,
    };

    const beforeInfo = await this.getCacheInfo();

    if (options.clearSyncedEvents || options.clearOldEventsDays) {
      const days = options.clearOldEventsDays ?? 7;
      const before = await trackingEventsRepo.getCountsBySyncStatus();
      await trackingEventsRepo.deleteOlderThan(days);
      const after = await trackingEventsRepo.getCountsBySyncStatus();
      result.clearedEvents = before.synced - after.synced;
    }

    if (options.clearPhotos) {
      try {
        const photosDirInfo = await FileSystem.getInfoAsync(PHOTOS_DIR);
        if (photosDirInfo.exists) {
          const syncedPods = await podsRepo.getAll();
          const syncedPhotoPaths = new Set<string>();
          
          for (const pod of syncedPods) {
            for (const photo of pod.photos) {
              syncedPhotoPaths.add(photo.localUri);
            }
          }

          if (photosDirInfo.isDirectory) {
            const files = await FileSystem.readDirectoryAsync(PHOTOS_DIR);
            for (const file of files) {
              const filePath = `${PHOTOS_DIR}/${file}`;
              if (!syncedPhotoPaths.has(filePath)) {
                await FileSystem.deleteAsync(filePath, { idempotent: true });
                result.clearedPhotos++;
              }
            }
          }
        }
      } catch (error) {
        console.error('Failed to clear photos:', error);
      }
    }

    const afterInfo = await this.getCacheInfo();
    result.freedBytes = beforeInfo.totalSize - afterInfo.totalSize;

    return result;
  }

  async clearAllCache(): Promise<{
    clearedEvents: number;
    clearedPhotos: number;
    freedBytes: number;
  }> {
    const beforeInfo = await this.getCacheInfo();
    
    try {
      const photosDirInfo = await FileSystem.getInfoAsync(PHOTOS_DIR);
      if (photosDirInfo.exists) {
        await FileSystem.deleteAsync(PHOTOS_DIR, { idempotent: true });
        await FileSystem.makeDirectoryAsync(PHOTOS_DIR, { intermediates: true });
      }
    } catch (error) {
      console.error('Failed to clear photos directory:', error);
    }

    try {
      const db = await getDatabase();
      await db.runAsync('DELETE FROM tracking_events WHERE synced_at IS NOT NULL');
      await db.runAsync('DELETE FROM sync_queue');
    } catch (error) {
      console.error('Failed to clear database cache:', error);
    }

    const afterInfo = await this.getCacheInfo();

    return {
      clearedEvents: beforeInfo.syncedEventsCount,
      clearedPhotos: 0,
      freedBytes: beforeInfo.totalSize - afterInfo.totalSize,
    };
  }

  async getStorageUsage(): Promise<string> {
    const info = await this.getCacheInfo();
    return info.formattedSize;
  }
}

export const cacheService = CacheService.getInstance();
export default CacheService;
