import { useEffect, useRef, useCallback, useState } from 'react';
import * as Notifications from 'expo-notifications';
import { useRouter } from 'expo-router';
import { Platform } from 'react-native';
import Constants from 'expo-constants';
import {
  registerPushToken as registerPushTokenWithBackend,
  unregisterPushToken as unregisterPushTokenWithBackend,
} from '@/services/api/notifications';
import { useSettingsStore } from '@/store/settingsStore';

export interface NotificationData {
  type?: 'shipment_update' | 'new_assignment' | 'pod_required' | 'alert' | 'urgent_message';
  shipmentId?: string;
  screen?: string;
  [key: string]: unknown;
}

export interface NotificationState {
  hasPermission: boolean | null;
  expoPushToken: string | null;
  isLoading: boolean;
  error: string | null;
  isRegistered: boolean;
}

// Notification handler is set in _layout.tsx — do not duplicate here.

const PROJECT_ID = Constants.expoConfig?.extra?.eas?.projectId || 'logistics-mobile';

export function useNotifications() {
  const router = useRouter();
  const notificationsEnabled = useSettingsStore((state) => state.notificationsEnabled);
  const [state, setState] = useState<NotificationState>({
    hasPermission: null,
    expoPushToken: null,
    isLoading: false,
    error: null,
    isRegistered: false,
  });

  const notificationListener = useRef<Notifications.EventSubscription | null>(null);
  const responseListener = useRef<Notifications.EventSubscription | null>(null);
  const lastRegisteredToken = useRef<string | null>(null);

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
        projectId: PROJECT_ID,
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

  const registerWithBackend = useCallback(async (token: string): Promise<boolean> => {
    if (lastRegisteredToken.current === token) {
      return true;
    }

    try {
      await registerPushTokenWithBackend(token);
      lastRegisteredToken.current = token;
      setState((prev) => ({ ...prev, isRegistered: true }));
      return true;
    } catch (error) {
      console.error('Failed to register push token with backend:', error);
      setState((prev) => ({
        ...prev,
        error: error instanceof Error ? error.message : 'Failed to register with server',
      }));
      return false;
    }
  }, []);

  const unregisterFromBackend = useCallback(async (): Promise<void> => {
    try {
      await unregisterPushTokenWithBackend();
      lastRegisteredToken.current = null;
      setState((prev) => ({
        ...prev,
        isRegistered: false,
        expoPushToken: null,
      }));
    } catch (error) {
      console.error('Failed to unregister push token from backend:', error);
    }
  }, []);

  const registerForPushNotifications = useCallback(async (): Promise<boolean> => {
    setState((prev) => ({ ...prev, isLoading: true, error: null }));

    try {
      const token = await registerForPushToken();
      if (!token) {
        return false;
      }

      const registered = await registerWithBackend(token);
      return registered;
    } catch (error) {
      setState((prev) => ({
        ...prev,
        isLoading: false,
        error: error instanceof Error ? error.message : 'Failed to register for notifications',
      }));
      return false;
    }
  }, [registerForPushToken, registerWithBackend]);

  const handleNotification = useCallback(
    (notification: Notifications.Notification) => {
      if (!useSettingsStore.getState().notificationsEnabled) {
        return;
      }

      const data = notification.request.content.data as NotificationData | undefined;

      switch (data?.type) {
        case 'shipment_update':
          if (data.shipmentId) {
            router.push(`/shipment/${data.shipmentId}`);
          }
          break;
        case 'new_assignment':
          if (data.shipmentId) {
            router.push(`/shipment/${data.shipmentId}`);
          }
          break;
        case 'pod_required':
          if (data.shipmentId) {
            router.push(`/pod/${data.shipmentId}`);
          }
          break;
        case 'urgent_message':
          break;
        default:
          if (data?.shipmentId) {
            router.push(`/shipment/${data.shipmentId}`);
          } else if (data?.screen) {
            router.push(data.screen as any);
          }
      }
    },
    [router]
  );

  const handleNotificationResponse = useCallback(
    (response: Notifications.NotificationResponse) => {
      const data = response.notification.request.content.data as NotificationData | undefined;

      switch (data?.type) {
        case 'shipment_update':
        case 'new_assignment':
          if (data.shipmentId) {
            router.push(`/shipment/${data.shipmentId}`);
          }
          break;
        case 'pod_required':
          if (data.shipmentId) {
            router.push(`/pod/${data.shipmentId}`);
          }
          break;
        case 'urgent_message':
          router.push('/(main)');
          break;
        default:
          if (data?.shipmentId) {
            router.push(`/shipment/${data.shipmentId}`);
          } else if (data?.screen) {
            router.push(data.screen as any);
          } else {
            router.push('/(main)');
          }
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
      if (!useSettingsStore.getState().notificationsEnabled) {
        return null;
      }

      const notificationId = await Notifications.scheduleNotificationAsync({
        content: {
          title,
          body,
          data: data || {},
          sound: 'default',
        },
        trigger: trigger || null,
      });
      return notificationId;
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

  useEffect(() => {
    if (notificationsEnabled && state.hasPermission && !state.isRegistered) {
      registerForPushNotifications();
    } else if (!notificationsEnabled && state.isRegistered) {
      unregisterFromBackend();
    }
  }, [notificationsEnabled, state.hasPermission, state.isRegistered, registerForPushNotifications, unregisterFromBackend]);

  return {
    ...state,
    requestPermissions,
    registerForPushToken,
    registerForPushNotifications,
    registerWithBackend,
    unregisterFromBackend,
    scheduleLocalNotification,
    cancelAllNotifications,
    setBadgeCount,
  };
}
