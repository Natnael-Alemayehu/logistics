import { Platform } from 'react-native';
import Constants from 'expo-constants';
import { api } from './client';
import { API_ENDPOINTS } from '../constants';

export interface NotificationPreferences {
  shipmentUpdates: boolean;
  newAssignments: boolean;
  podReminders: boolean;
  urgentAlerts: boolean;
}

export interface PushTokenRegistration {
  token: string;
  platform: 'ios' | 'android';
  deviceId: string;
  appVersion: string;
}

export interface RegisterPushTokenRequest {
  push_token: string;
  platform: 'ios' | 'android';
  device_id: string;
  app_version: string;
}

export interface UpdatePreferencesRequest {
  shipment_updates: boolean;
  new_assignments: boolean;
  pod_reminders: boolean;
  urgent_alerts: boolean;
}

const getDeviceId = (): string => {
  return Constants.deviceId || Constants.sessionId || 'unknown';
};

const getAppVersion = (): string => {
  return Constants.expoConfig?.version || '1.0.0';
};

export async function registerPushToken(token: string): Promise<void> {
  const payload: RegisterPushTokenRequest = {
    push_token: token,
    platform: Platform.OS as 'ios' | 'android',
    device_id: getDeviceId(),
    app_version: getAppVersion(),
  };

  try {
    await api.post('/notifications/register', payload);
  } catch (error) {
    console.error('Failed to register push token:', error);
    throw error;
  }
}

export async function unregisterPushToken(): Promise<void> {
  const deviceId = getDeviceId();

  try {
    await api.delete(`/notifications/device/${deviceId}`);
  } catch (error) {
    console.error('Failed to unregister push token:', error);
    throw error;
  }
}

export async function updateNotificationPreferences(
  preferences: NotificationPreferences
): Promise<void> {
  const payload: UpdatePreferencesRequest = {
    shipment_updates: preferences.shipmentUpdates,
    new_assignments: preferences.newAssignments,
    pod_reminders: preferences.podReminders,
    urgent_alerts: preferences.urgentAlerts,
  };

  try {
    await api.put('/notifications/preferences', payload);
  } catch (error) {
    console.error('Failed to update notification preferences:', error);
    throw error;
  }
}

export async function getNotificationPreferences(): Promise<NotificationPreferences> {
  try {
    const response = await api.get<{
      shipment_updates: boolean;
      new_assignments: boolean;
      pod_reminders: boolean;
      urgent_alerts: boolean;
    }>('/notifications/preferences');

    return {
      shipmentUpdates: response.shipment_updates,
      newAssignments: response.new_assignments,
      podReminders: response.pod_reminders,
      urgentAlerts: response.urgent_alerts,
    };
  } catch (error) {
    console.error('Failed to get notification preferences:', error);
    throw error;
  }
}
