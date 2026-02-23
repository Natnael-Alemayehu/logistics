import { useEffect, useRef, useCallback, useState } from 'react';
import * as Notifications from 'expo-notifications';
import { useRouter } from 'expo-router';
import { Platform } from 'react-native';

export interface NotificationData {
  type?: 'shipment_update' | 'new_assignment' | 'pod_required' | 'alert';
  shipmentId?: string;
  screen?: string;
  [key: string]: unknown;
}

export interface NotificationState {
  hasPermission: boolean | null;
  expoPushToken: string | null;
  isLoading: boolean;
  error: string | null;
}

// Notification handler is set in _layout.tsx — do not duplicate here.

export function useNotifications() {
  const router = useRouter();
  const [state, setState] = useState<NotificationState>({
    hasPermission: null,
    expoPushToken: null,
    isLoading: false,
    error: null,
  });
  
  const notificationListener = useRef<Notifications.EventSubscription | null>(null);
  const responseListener = useRef<Notifications.EventSubscription | null>(null);

  const requestPermissions = useCallback(async (): Promise<boolean> => {
    setState((prev) => ({ ...prev, isLoading: true, error: null }));
    
    try {
      const { status: existingStatus } = await Notifications.getPermissionsAsync();
      let finalStatus = existingStatus;
      
      if (existingStatus !== 'granted') {
        const { status } = await Notifications.requestPermissionsAsync();
        finalStatus = status;
      }
      
      if (finalStatus !== 'granted') {
        setState((prev) => ({
          ...prev,
          hasPermission: false,
          isLoading: false,
          error: 'Push notification permission denied',
        }));
        return false;
      }
      
      if (Platform.OS === 'android') {
        await Notifications.setNotificationChannelAsync('default', {
          name: 'default',
          importance: Notifications.AndroidImportance.HIGH,
        });
      }
      
      setState((prev) => ({ ...prev, hasPermission: true, isLoading: false }));
      return true;
    } catch (error) {
      setState((prev) => ({
        ...prev,
        isLoading: false,
        error: error instanceof Error ? error.message : 'Failed to request permissions',
      }));
      return false;
    }
  }, []);

  const registerForPushToken = useCallback(async (): Promise<string | null> => {
    setState((prev) => ({ ...prev, isLoading: true, error: null }));
    
    try {
      const hasPermission = await requestPermissions();
      if (!hasPermission) {
        return null;
      }
      
      const token = await Notifications.getExpoPushTokenAsync({
        projectId: 'your-project-id',
      });
      
      setState((prev) => ({
        ...prev,
        expoPushToken: token.data,
        isLoading: false,
      }));
      
      return token.data;
    } catch (error) {
      setState((prev) => ({
        ...prev,
        isLoading: false,
        error: error instanceof Error ? error.message : 'Failed to get push token',
      }));
      return null;
    }
  }, [requestPermissions]);

  const handleNotification = useCallback(
    (notification: Notifications.Notification) => {
      const data = notification.request.content.data as NotificationData | undefined;
      
      if (data?.shipmentId) {
        switch (data.type) {
          case 'shipment_update':
          case 'new_assignment':
            router.push(`/shipment/${data.shipmentId}`);
            break;
          case 'pod_required':
            router.push(`/pod/${data.shipmentId}`);
            break;
          default:
            if (data.screen) {
              router.push(data.screen as any);
            }
        }
      }
    },
    [router]
  );

  const handleNotificationResponse = useCallback(
    (response: Notifications.NotificationResponse) => {
      const data = response.notification.request.content.data as NotificationData | undefined;
      
      if (data?.shipmentId) {
        switch (data.type) {
          case 'shipment_update':
          case 'new_assignment':
            router.push(`/shipment/${data.shipmentId}`);
            break;
          case 'pod_required':
            router.push(`/pod/${data.shipmentId}`);
            break;
          default:
            if (data.screen) {
              router.push(data.screen as any);
            } else {
              router.push('/(main)');
            }
        }
      } else {
        router.push('/');
      }
    },
    [router]
  );

  const scheduleLocalNotification = useCallback(
    async (
      title: string,
      body: string,
      data?: NotificationData,
      trigger?: Notifications.NotificationRequestInput['trigger']
    ) => {
      await Notifications.scheduleNotificationAsync({
        content: {
          title,
          body,
          data: data || {},
          sound: 'default',
        },
        trigger: trigger || null,
      });
    },
    []
  );

  const cancelAllNotifications = useCallback(async () => {
    await Notifications.cancelAllScheduledNotificationsAsync();
  }, []);

  const setBadgeCount = useCallback(async (count: number) => {
    await Notifications.setBadgeCountAsync(count);
  }, []);

  useEffect(() => {
    notificationListener.current = Notifications.addNotificationReceivedListener(
      (notification) => handleNotification(notification)
    );
    
    responseListener.current = Notifications.addNotificationResponseReceivedListener(
      (response) => handleNotificationResponse(response)
    );
    
    return () => {
      if (notificationListener.current) {
        notificationListener.current.remove();
      }
      if (responseListener.current) {
        responseListener.current.remove();
      }
    };
  }, [handleNotification, handleNotificationResponse]);

  useEffect(() => {
    requestPermissions();
  }, [requestPermissions]);

  return {
    ...state,
    requestPermissions,
    registerForPushToken,
    scheduleLocalNotification,
    cancelAllNotifications,
    setBadgeCount,
  };
}
