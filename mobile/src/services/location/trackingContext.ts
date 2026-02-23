import * as SecureStore from 'expo-secure-store';

const TRACKING_CONTEXT_KEY = 'tracking_context';

export interface TrackingContext {
  shipmentId: string;
  driverId: string;
  startedAt: string;
  config: TrackingConfig;
}

export interface TrackingConfig {
  accuracy: 'high' | 'balanced' | 'low';
  timeInterval: number;
  distanceInterval: number;
  batteryOptimized: boolean;
}

export const DEFAULT_TRACKING_CONFIG: TrackingConfig = {
  accuracy: 'high',
  timeInterval: 10000,
  distanceInterval: 50,
  batteryOptimized: true,
};

export async function setTrackingContext(context: TrackingContext): Promise<void> {
  try {
    await SecureStore.setItemAsync(TRACKING_CONTEXT_KEY, JSON.stringify(context));
  } catch (error) {
    console.error('Failed to set tracking context:', error);
    throw error;
  }
}

export async function getTrackingContext(): Promise<TrackingContext | null> {
  try {
    const data = await SecureStore.getItemAsync(TRACKING_CONTEXT_KEY);
    if (data) {
      return JSON.parse(data) as TrackingContext;
    }
    return null;
  } catch (error) {
    console.error('Failed to get tracking context:', error);
    return null;
  }
}

export async function clearTrackingContext(): Promise<void> {
  try {
    await SecureStore.deleteItemAsync(TRACKING_CONTEXT_KEY);
  } catch (error) {
    console.error('Failed to clear tracking context:', error);
    throw error;
  }
}