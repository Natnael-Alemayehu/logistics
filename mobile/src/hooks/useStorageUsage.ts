import { useState, useEffect, useCallback } from 'react';
import { getPhotoStorageUsage, formatFileSize } from '@/services/photos';
import { getDatabase } from '@/db/database';
import { Paths } from 'expo-file-system';

export interface StorageUsage {
  photos: number;
  database: number;
  total: number;
  formatted: {
    photos: string;
    database: string;
    total: string;
  };
}

const DEFAULT_STORAGE: StorageUsage = {
  photos: 0,
  database: 0,
  total: 0,
  formatted: {
    photos: '0 B',
    database: '0 B',
    total: '0 B',
  },
};

async function getDatabaseSize(): Promise<number> {
  try {
    const db = await getDatabase();
    const dbPath = `${Paths.document}SQLite/logistics.db`;
    const walPath = `${dbPath}-wal`;
    const shmPath = `${dbPath}-shm`;
    
    let totalSize = 0;
    
    const { File } = await import('expo-file-system');
    
    const dbFile = new File(dbPath);
    if (dbFile.exists) {
      totalSize += dbFile.size;
    }
    
    const walFile = new File(walPath);
    if (walFile.exists) {
      totalSize += walFile.size;
    }
    
    const shmFile = new File(shmPath);
    if (shmFile.exists) {
      totalSize += shmFile.size;
    }
    
    return totalSize;
  } catch {
    return 0;
  }
}

export function useStorageUsage() {
  const [storage, setStorage] = useState<StorageUsage>(DEFAULT_STORAGE);
  const [isLoading, setIsLoading] = useState(true);

  const calculateStorage = useCallback(async () => {
    setIsLoading(true);
    try {
      const [photosSize, dbSize] = await Promise.all([
        getPhotoStorageUsage(),
        getDatabaseSize(),
      ]);

      const total = photosSize + dbSize;

      setStorage({
        photos: photosSize,
        database: dbSize,
        total,
        formatted: {
          photos: formatFileSize(photosSize),
          database: formatFileSize(dbSize),
          total: formatFileSize(total),
        },
      });
    } catch (error) {
      console.error('Failed to calculate storage:', error);
    } finally {
      setIsLoading(false);
    }
  }, []);

  useEffect(() => {
    calculateStorage();
  }, [calculateStorage]);

  return {
    storage,
    isLoading,
    refetch: calculateStorage,
  };
}
