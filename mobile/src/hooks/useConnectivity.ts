import { useEffect, useState, useCallback } from 'react';
import NetInfo, { NetInfoState } from '@react-native-community/netinfo';
import { useNetworkStore } from '@/store/networkStore';

export type ConnectionType = 'wifi' | 'cellular' | 'none' | 'unknown';

export function useConnectivity() {
  const { isOnline, lastOnlineTime, setOnline } = useNetworkStore();
  const [connectionType, setConnectionType] = useState<ConnectionType>('unknown');
  const [isInternetReachable, setIsInternetReachable] = useState<boolean | null>(null);

  const handleNetInfoChange = useCallback((state: NetInfoState) => {
    const online = state.isConnected === true && state.isInternetReachable !== false;
    
    setOnline(online);
    setIsInternetReachable(state.isInternetReachable);
    
    if (state.type === 'wifi') {
      setConnectionType('wifi');
    } else if (state.type === 'cellular') {
      setConnectionType('cellular');
    } else if (state.isConnected !== true) {
      setConnectionType('none');
    } else {
      setConnectionType('unknown');
    }
  }, [setOnline]);

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

  return {
    isOnline: isOnline ?? false,
    isOffline: !(isOnline ?? false),
    connectionType,
    isInternetReachable: isInternetReachable ?? false,
    lastOnlineTime,
    checkConnectivity,
  };
}
