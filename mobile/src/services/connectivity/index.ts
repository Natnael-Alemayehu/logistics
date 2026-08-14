import NetInfo, { NetInfoState, NetInfoCellularGeneration } from '@react-native-community/netinfo';
import {
  useNetworkStore,
  ConnectionQuality,
  ConnectionType,
  ConnectionHistoryEntry,
} from '@/store/networkStore';

export interface ConnectionInfo {
  isOnline: boolean;
  connectionType: ConnectionType;
  connectionQuality: ConnectionQuality;
  isMetered: boolean;
  cellularGeneration: NetInfoCellularGeneration | null;
  effectiveType: string;
}

export interface ConnectionChangeCallback {
  (info: ConnectionInfo): void;
}

const CELLULAR_GENERATION_QUALITY: Record<string, ConnectionQuality> = {
  '2g': 'slow',
  '3g': 'medium',
  '4g': 'fast',
  '5g': 'fast',
};

function mapConnectionType(state: NetInfoState): ConnectionType {
  if (state.type === 'wifi') return 'wifi';
  if (state.type === 'cellular') return 'cellular';
  if (state.isConnected === false) return 'none';
  return 'unknown';
}

function determineConnectionQuality(state: NetInfoState): ConnectionQuality {
  if (state.isConnected !== true || state.isInternetReachable === false) {
    return 'offline';
  }

  if (state.type === 'wifi') {
    return 'fast';
  }

  if (state.type === 'cellular' && state.details?.cellularGeneration) {
    return CELLULAR_GENERATION_QUALITY[state.details.cellularGeneration] || 'slow';
  }

  return 'slow';
}

function getConnectionInfo(state: NetInfoState): ConnectionInfo {
  const connectionType = mapConnectionType(state);
  const connectionQuality = determineConnectionQuality(state);
  const isMetered = state.type === 'cellular';

  return {
    isOnline: state.isConnected === true && state.isInternetReachable !== false,
    connectionType,
    connectionQuality,
    isMetered,
    cellularGeneration: state.type === 'cellular' ? state.details?.cellularGeneration ?? null : null,
    effectiveType: state.type,
  };
}

export async function getCurrentConnectionInfo(): Promise<ConnectionInfo> {
  const state = await NetInfo.fetch();
  return getConnectionInfo(state);
}

export function subscribeToChanges(callback: ConnectionChangeCallback): () => void {
  return NetInfo.addEventListener((state) => {
    const info = getConnectionInfo(state);
    callback(info);
  });
}

export function isGoodConnection(info: ConnectionInfo): boolean {
  return info.isOnline && (info.connectionQuality === 'fast' || info.connectionQuality === 'medium');
}

export async function waitForConnection(timeout: number = 30000): Promise<ConnectionInfo> {
  const currentState = await NetInfo.fetch();
  const currentInfo = getConnectionInfo(currentState);

  if (currentInfo.isOnline) {
    return currentInfo;
  }

  return new Promise((resolve, reject) => {
    const timeoutId = setTimeout(() => {
      unsubscribe();
      reject(new Error('Connection timeout'));
    }, timeout);

    const unsubscribe = NetInfo.addEventListener((state) => {
      const info = getConnectionInfo(state);
      if (info.isOnline) {
        clearTimeout(timeoutId);
        unsubscribe();
        resolve(info);
      }
    });
  });
}

export function getConnectionHistory(): ConnectionHistoryEntry[] {
  return useNetworkStore.getState().connectionHistory;
}

export function canSyncLargeData(
  info: ConnectionInfo,
  shouldUseCellular: boolean = true
): boolean {
  if (!info.isOnline) return false;
  if (info.connectionQuality === 'slow') return false;
  if (info.isMetered && !shouldUseCellular) return false;
  return true;
}

export function getRecommendedBatchSize(quality: ConnectionQuality): number {
  switch (quality) {
    case 'fast':
      return 50;
    case 'medium':
      return 20;
    case 'slow':
      return 5;
    case 'offline':
    default:
      return 0;
  }
}

export function shouldDeferSync(
  info: ConnectionInfo,
  shouldUseCellular: boolean = true
): boolean {
  if (!info.isOnline) return true;
  if (info.connectionQuality === 'slow') return true;
  if (info.isMetered && !shouldUseCellular) return true;
  return false;
}

export const connectivityService = {
  getConnectionInfo: getCurrentConnectionInfo,
  subscribeToChanges,
  isGoodConnection,
  waitForConnection,
  getConnectionHistory,
  canSyncLargeData,
  getRecommendedBatchSize,
  shouldDeferSync,
};

export default connectivityService;
