import { useState, useEffect, useCallback } from 'react';
import NetInfo, { NetInfoState, NetInfoCellularGeneration } from '@react-native-community/netinfo';
import {
  useNetworkStore,
  ConnectionQuality,
  ConnectionType,
  ConnectionHistoryEntry,
} from '@/store/networkStore';

interface ReconnectCallback {
  (): void | Promise<void>;
}

interface UseConnectivityOptions {
  shouldUseCellular?: boolean;
  onReconnect?: ReconnectCallback;
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

export function useConnectivity(options: UseConnectivityOptions = {}) {
  const { shouldUseCellular = true, onReconnect } = options;
  
  const {
    isOnline,
    lastOnlineTime,
    lastOnlineAt,
    lastOfflineAt,
    connectionType: storedConnectionType,
    connectionQuality: storedConnectionQuality,
    isMetered,
    connectionHistory,
    setOnline,
    setConnectionType,
    setConnectionQuality,
    setIsMetered,
    addConnectionHistory,
  } = useNetworkStore();

  const [wasOffline, setWasOffline] = useState(false);
  const [isInternetReachable, setIsInternetReachable] = useState<boolean | null>(null);
  const [cellularGeneration, setCellularGeneration] = useState<NetInfoCellularGeneration | null>(null);

  const handleNetInfoChange = useCallback(async (state: NetInfoState) => {
    const online = state.isConnected === true && state.isInternetReachable !== false;
    const connectionType = mapConnectionType(state);
    const connectionQuality = determineConnectionQuality(state);
    const metered = state.type === 'cellular';

    const wasPreviouslyOffline = !isOnline;
    
    if (wasPreviouslyOffline && online && onReconnect) {
      setWasOffline(true);
      try {
        await onReconnect();
      } catch (error) {
        console.error('Reconnect callback failed:', error);
      }
    } else if (!online) {
      setWasOffline(false);
    }

    setOnline(online);
    setConnectionType(connectionType);
    setConnectionQuality(connectionQuality);
    setIsMetered(metered);
    setIsInternetReachable(state.isInternetReachable ?? null);

    if (state.type === 'cellular' && state.details?.cellularGeneration) {
      setCellularGeneration(state.details.cellularGeneration);
    } else {
      setCellularGeneration(null);
    }

    addConnectionHistory({
      isOnline: online,
      connectionType,
      connectionQuality,
    });
  }, [isOnline, onReconnect, setOnline, setConnectionType, setConnectionQuality, setIsMetered, addConnectionHistory]);

  useEffect(() => {
    const unsubscribe = NetInfo.addEventListener(handleNetInfoChange);
    
    NetInfo.fetch().then(handleNetInfoChange);
    
    return () => {
      unsubscribe();
    };
  }, [handleNetInfoChange]);

  const checkConnectivity = useCallback(async (): Promise<boolean> => {
    const state = await NetInfo.fetch();
    return state.isConnected === true && state.isInternetReachable !== false;
  }, []);

  const getConnectionHistory = useCallback((): ConnectionHistoryEntry[] => {
    return connectionHistory;
  }, [connectionHistory]);

  return {
    isOnline: isOnline ?? false,
    isOffline: !(isOnline ?? false),
    connectionType: storedConnectionType,
    connectionQuality: storedConnectionQuality,
    isMetered,
    shouldUseCellular,
    wasOffline,
    isInternetReachable: isInternetReachable ?? false,
    lastOnlineTime,
    lastOnlineAt,
    lastOfflineAt,
    cellularGeneration,
    checkConnectivity,
    getConnectionHistory,
  };
}

export type { ConnectionQuality, ConnectionType, ConnectionHistoryEntry };
