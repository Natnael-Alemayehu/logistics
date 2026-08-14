import * as Location from 'expo-location';
import * as TaskManager from 'expo-task-manager';
import { insertTrackingEvent } from '@db';
import {
  setTrackingContext,
  getTrackingContext,
  clearTrackingContext,
  type TrackingContext,
  type TrackingConfig,
  DEFAULT_TRACKING_CONFIG,
} from './trackingContext';
import {
  checkGeofences,
  startGeofenceMonitoring,
  stopGeofenceMonitoring,
  createShipmentGeofences,
  addGeofence,
  clearGeofences,
} from './geofencing';
import { useSettingsStore } from '@/store/settingsStore';

const LOCATION_TASK_NAME = 'background-location-task';

interface LocationData {
  latitude: number;
  longitude: number;
  accuracy: number | null;
  speed: number | null;
  heading: number | null;
}

export async function requestLocationPermissions(): Promise<boolean> {
  const { status: foregroundStatus } = await Location.requestForegroundPermissionsAsync();
  if (foregroundStatus !== 'granted') {
    return false;
  }

  const { status: backgroundStatus } = await Location.requestBackgroundPermissionsAsync();
  return backgroundStatus === 'granted';
}

export async function getCurrentLocation(): Promise<LocationData | null> {
  const { status } = await Location.getForegroundPermissionsAsync();
  if (status !== 'granted') {
    return null;
  }

  const location = await Location.getCurrentPositionAsync({
    accuracy: Location.Accuracy.High,
  });

  return {
    latitude: location.coords.latitude,
    longitude: location.coords.longitude,
    accuracy: location.coords.accuracy,
    speed: location.coords.speed,
    heading: location.coords.heading,
  };
}

export async function startLocationTracking(
  shipmentId: string,
  driverId: string,
  config: Partial<TrackingConfig> = {}
): Promise<void> {
  const settings = useSettingsStore.getState();
  if (!settings.trackingEnabled) {
    throw new Error('Tracking is disabled in settings');
  }

  const hasPermission = await requestLocationPermissions();
  if (!hasPermission) {
    throw new Error('Location permission not granted');
  }

  const trackingConfig: TrackingConfig = {
    ...DEFAULT_TRACKING_CONFIG,
    ...config,
  };

  const context: TrackingContext = {
    shipmentId,
    driverId,
    startedAt: new Date().toISOString(),
    config: trackingConfig,
  };

  await setTrackingContext(context);
  
  startGeofenceMonitoring();

  const accuracyMap: Record<string, Location.Accuracy> = {
    high: Location.Accuracy.High,
    balanced: Location.Accuracy.Balanced,
    low: Location.Accuracy.Low,
  };

  await Location.startLocationUpdatesAsync(LOCATION_TASK_NAME, {
    accuracy: accuracyMap[trackingConfig.accuracy] ?? Location.Accuracy.High,
    timeInterval: trackingConfig.timeInterval,
    distanceInterval: trackingConfig.distanceInterval,
    foregroundService: {
      notificationTitle: 'Tracking Active',
      notificationBody: 'Your location is being tracked for delivery',
    },
  });
}

export async function stopLocationTracking(): Promise<void> {
  const isTracking = await Location.hasStartedLocationUpdatesAsync(LOCATION_TASK_NAME);
  if (isTracking) {
    await Location.stopLocationUpdatesAsync(LOCATION_TASK_NAME);
  }
  stopGeofenceMonitoring();
  clearGeofences();
  await clearTrackingContext();
}

export async function isLocationTrackingActive(): Promise<boolean> {
  return Location.hasStartedLocationUpdatesAsync(LOCATION_TASK_NAME);
}

export async function stopTrackingIfDisabled(): Promise<void> {
  const settings = useSettingsStore.getState();
  if (!settings.trackingEnabled) {
    const isActive = await isLocationTrackingActive();
    if (isActive) {
      await stopLocationTracking();
    }
  }
}

export function isTrackingEnabledInSettings(): boolean {
  return useSettingsStore.getState().trackingEnabled;
}

export function setupShipmentGeofences(shipment: {
  id: string;
  origin_lat?: number;
  origin_lng?: number;
  destination_lat?: number;
  destination_lng?: number;
}): void {
  const geofences = createShipmentGeofences(shipment);
  geofences.forEach(g => addGeofence(g));
}

export { 
  addGeofence, 
  removeGeofence, 
  clearGeofences, 
  getGeofences,
  checkGeofences,
  startGeofenceMonitoring,
  stopGeofenceMonitoring,
  createShipmentGeofences,
  type Geofence,
  type GeofenceEvent,
} from './geofencing';

TaskManager.defineTask(LOCATION_TASK_NAME, async ({ data, error }) => {
  if (error) {
    console.error('Location task error:', error);
    return;
  }

  if (data) {
    const { locations } = data as { locations: Location.LocationObject[] };
    const location = locations[0];

    if (location) {
      const context = await getTrackingContext();

      if (!context) {
        console.warn('No tracking context found, skipping location update');
        return;
      }

      const coordinate = {
        latitude: location.coords.latitude,
        longitude: location.coords.longitude,
      };
      
      checkGeofences(coordinate);

      await insertTrackingEvent({
        shipment_id: context.shipmentId,
        driver_id: context.driverId,
        latitude: location.coords.latitude,
        longitude: location.coords.longitude,
        accuracy: location.coords.accuracy ?? undefined,
        speed: location.coords.speed ? location.coords.speed * 3.6 : undefined,
        heading: location.coords.heading ?? undefined,
        event_type: 'gps_ping',
        recorded_at: new Date().toISOString(),
      });
    }
  }
});