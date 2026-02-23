import '../i18n';

import { useEffect, useState, useRef } from 'react';
import { Stack } from 'expo-router';
import { StatusBar } from 'expo-status-bar';
import { SafeAreaProvider } from 'react-native-safe-area-context';
import { GestureHandlerRootView } from 'react-native-gesture-handler';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { useNetInfo } from '@react-native-community/netinfo';
import { useNetworkStore } from '@store/networkStore';
import { useSyncStore } from '@store/syncStore';
import { restoreAuthSession } from '@store/authStore';
import * as Notifications from 'expo-notifications';
import { Platform } from 'react-native';
import SyncStatusBar from '@components/layout/SyncStatusBar';

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: 2,
      staleTime: 5 * 60 * 1000,
      gcTime: 10 * 60 * 1000,
      networkMode: 'offlineFirst',
    },
  },
});

Notifications.setNotificationHandler({
  handleNotification: async () => ({
    shouldShowAlert: true,
    shouldPlaySound: true,
    shouldSetBadge: true,
    shouldShowBanner: true,
    shouldShowList: true,
  }),
});

function NetworkListener() {
  const netInfo = useNetInfo();
  const setOnline = useNetworkStore((state) => state.setOnline);
  const setConnectionType = useNetworkStore((state) => state.setConnectionType);
  const sync = useSyncStore((state) => state.sync);
  const prevOnlineRef = useRef<boolean | null>(null);

  useEffect(() => {
    const isConnected = netInfo.isConnected ?? false;
    setOnline(isConnected);

    const connectionType = netInfo.type === 'wifi'
      ? 'wifi'
      : netInfo.type === 'cellular'
        ? 'cellular'
        : netInfo.type === 'none'
          ? 'none'
          : 'unknown';
    setConnectionType(connectionType);

    if (prevOnlineRef.current === false && isConnected) {
      sync().catch((err) => console.warn('Auto-sync failed:', err));
    }
    prevOnlineRef.current = isConnected;
  }, [netInfo.isConnected, netInfo.type, setOnline, setConnectionType, sync]);

  return null;
}

function NotificationHandler() {
  useEffect(() => {
    const subscription = Notifications.addNotificationReceivedListener((notification) => {
      console.log('Notification received:', notification);
    });

    const responseSubscription = Notifications.addNotificationResponseReceivedListener((response) => {
      console.log('Notification response:', response);
    });

    return () => {
      subscription.remove();
      responseSubscription.remove();
    };
  }, []);

  return null;
}

export default function RootLayout() {
  const [isReady, setIsReady] = useState(false);

  useEffect(() => {
    async function prepare() {
      try {
        await restoreAuthSession();
        
        if (Platform.OS !== 'web') {
          const { status: existingStatus } = await Notifications.getPermissionsAsync();
          if (existingStatus !== 'granted') {
            await Notifications.requestPermissionsAsync();
          }
        }
      } catch (e) {
        console.warn('Failed to restore session:', e);
      } finally {
        setIsReady(true);
      }
    }

    prepare();
  }, []);

  if (!isReady) {
    return null;
  }

  return (
    <GestureHandlerRootView style={{ flex: 1 }}>
      <SafeAreaProvider>
        <QueryClientProvider client={queryClient}>
          <NetworkListener />
          <NotificationHandler />
          <StatusBar style="auto" />
          <SyncStatusBar />
          <Stack screenOptions={{ headerShown: false }}>
            <Stack.Screen name="index" />
            <Stack.Screen name="(auth)" options={{ headerShown: false }} />
            <Stack.Screen name="(main)" options={{ headerShown: false }} />
            <Stack.Screen 
              name="pod/[shipmentId]" 
              options={{ 
                headerShown: true, 
                title: 'Proof of Delivery',
                headerStyle: { backgroundColor: '#2563eb' },
                headerTintColor: '#fff',
                headerTitleStyle: { fontWeight: '600' },
              }} 
            />
          </Stack>
        </QueryClientProvider>
      </SafeAreaProvider>
    </GestureHandlerRootView>
  );
}
