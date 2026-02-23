import { useMemo } from 'react';
import { useNetworkStore, ConnectionQuality } from '@/store/networkStore';
import {
  canSyncLargeData,
  getRecommendedBatchSize,
  shouldDeferSync,
  ConnectionInfo,
} from '@/services/connectivity';

interface UseConnectionQualityOptions {
  shouldUseCellular?: boolean;
}

interface ConnectionQualityResult {
  quality: ConnectionQuality;
  canSyncLargeData: boolean;
  recommendedBatchSize: number;
  shouldDeferSync: boolean;
  isFastConnection: boolean;
  isSlowConnection: boolean;
  isOffline: boolean;
}

export function useConnectionQuality(options: UseConnectionQualityOptions = {}): ConnectionQualityResult {
  const { shouldUseCellular = true } = options;
  
  const {
    isOnline,
    connectionType,
    connectionQuality,
    isMetered,
  } = useNetworkStore();

  const connectionInfo: ConnectionInfo = useMemo(() => ({
    isOnline: isOnline ?? false,
    connectionType,
    connectionQuality,
    isMetered,
    cellularGeneration: null,
    effectiveType: connectionType,
  }), [isOnline, connectionType, connectionQuality, isMetered]);

  const result = useMemo((): ConnectionQualityResult => {
    const quality = isOnline ? connectionQuality : 'offline';
    
    return {
      quality,
      canSyncLargeData: canSyncLargeData(connectionInfo, shouldUseCellular),
      recommendedBatchSize: getRecommendedBatchSize(quality),
      shouldDeferSync: shouldDeferSync(connectionInfo, shouldUseCellular),
      isFastConnection: quality === 'fast',
      isSlowConnection: quality === 'slow',
      isOffline: quality === 'offline',
    };
  }, [isOnline, connectionQuality, connectionInfo, shouldUseCellular]);

  return result;
}

export type { ConnectionQualityResult };
