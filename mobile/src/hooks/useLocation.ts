import { useState, useEffect, useCallback, useRef } from 'react';
import * as Location from 'expo-location';
import { useSyncStore } from '@/store/syncStore';
import { useTrackingStore } from '@/store/trackingStore';
import { useAuthStore } from '@/store/authStore';
import { useShipmentsStore } from '@/store/shipmentsStore';
import {
  startLocationTracking as startBackgroundTracking,
  stopLocationTracking as stopBackgroundTracking,
  isLocationTrackingActive,
} from '@/services/location';
import type { TrackingConfig } from '@/services/location/trackingContext';

export interface LocationCoords {
  latitude: number;
  longitude: number;
  accuracy: number | null;
  altitude: number | null;
  altitudeAccuracy: number | null;
  heading: number | null;
  speed: number | null;
}

export interface LocationError {
  code: 'permission_denied' | 'location_unavailable' | 'timeout' | 'unknown';
  message: string;
}

export interface UseLocationOptions {
  enableBackgroundTracking?: boolean;
  trackingConfig?: Partial<TrackingConfig>;
}

export function useLocation(options: UseLocationOptions = {}) {
  const [lastKnownLocation, setLastKnownLocation] = useState<LocationCoords | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<LocationError | null>(null);
  const [hasPermission, setHasPermission] = useState<boolean | null>(null);

  const locationSubscription = useRef<Location.LocationSubscription | null>(null);

  const {
    isTracking,
    activeShipmentId,
    driverId,
    startTracking: setTrackingActive,
    stopTracking: setTrackingInactive,
    updatePosition,
    incrementEventsCount,
  } = useTrackingStore();

  const user = useAuthStore((state) => state.user);
  const shipmentId = useShipmentsStore((state) => state.activeShipmentId);

  const requestPermissions = useCallback(async (): Promise<boolean> => {
    try {
      const { status } = await Location.requestForegroundPermissionsAsync();

      if (status !== 'granted') {
        setError({
          code: 'permission_denied',
          message: 'Location permission denied. Please enable location access in settings.',
        });
        setHasPermission(false);
        return false;
      }

      const backgroundStatus = await Location.requestBackgroundPermissionsAsync();
      if (backgroundStatus.status !== 'granted') {
        console.log('Background location permission not granted');
      }

      setHasPermission(true);
      setError(null);
      return true;
    } catch (err) {
      setError({
        code: 'unknown',
        message: err instanceof Error ? err.message : 'Failed to request permissions',
      });
      return false;
    }
  }, []);

  const getCurrentLocation = useCallback(async (): Promise<LocationCoords | null> => {
    setIsLoading(true);
    setError(null);

    try {
      if (hasPermission === null) {
        const granted = await requestPermissions();
        if (!granted) {
          setIsLoading(false);
          return null;
        }
      } else if (!hasPermission) {
        setError({
          code: 'permission_denied',
          message: 'Location permission not granted',
        });
        setIsLoading(false);
        return null;
      }

      const location = await Location.getCurrentPositionAsync({
        accuracy: Location.Accuracy.High,
      });

      const coords: LocationCoords = {
        latitude: location.coords.latitude,
        longitude: location.coords.longitude,
        accuracy: location.coords.accuracy,
        altitude: location.coords.altitude,
        altitudeAccuracy: location.coords.altitudeAccuracy,
        heading: location.coords.heading,
        speed: location.coords.speed,
      };

      setLastKnownLocation(coords);
      setIsLoading(false);
      return coords;
    } catch (err) {
      const locationError: LocationError = {
        code: 'location_unavailable',
        message: err instanceof Error ? err.message : 'Failed to get location',
      };
      setError(locationError);
      setIsLoading(false);
      return null;
    }
  }, [hasPermission, requestPermissions]);

  const startTracking = useCallback(
    async (trackingOptions?: {
      accuracy?: Location.Accuracy;
      distanceInterval?: number;
      timeInterval?: number;
      shipmentId?: string;
    }) => {
      if (isTracking) return;

      if (hasPermission === null) {
        const granted = await requestPermissions();
        if (!granted) return;
      } else if (!hasPermission) {
        return;
      }

      const effectiveShipmentId = trackingOptions?.shipmentId ?? shipmentId;
      const effectiveDriverId = user?.id;

      if (!effectiveShipmentId) {
        setError({
          code: 'unknown',
          message: 'No active shipment. Please select a shipment to track.',
        });
        return;
      }

      if (!effectiveDriverId) {
        setError({
          code: 'unknown',
          message: 'Driver not authenticated. Please log in.',
        });
        return;
      }

      try {
        locationSubscription.current = await Location.watchPositionAsync(
          {
            accuracy: trackingOptions?.accuracy ?? Location.Accuracy.High,
            distanceInterval: trackingOptions?.distanceInterval ?? 10,
            timeInterval: trackingOptions?.timeInterval ?? 5000,
          },
          (location) => {
            const coords: LocationCoords = {
              latitude: location.coords.latitude,
              longitude: location.coords.longitude,
              accuracy: location.coords.accuracy,
              altitude: location.coords.altitude,
              altitudeAccuracy: location.coords.altitudeAccuracy,
              heading: location.coords.heading,
              speed: location.coords.speed,
            };
            setLastKnownLocation(coords);
            updatePosition(coords);
            incrementEventsCount();
          }
        );

        if (options.enableBackgroundTracking !== false) {
          await startBackgroundTracking(effectiveShipmentId, effectiveDriverId, {
            timeInterval: trackingOptions?.timeInterval ?? 10000,
            distanceInterval: trackingOptions?.distanceInterval ?? 50,
          });
        }

        setTrackingActive(effectiveShipmentId, effectiveDriverId);
        setError(null);
      } catch (err) {
        setError({
          code: 'unknown',
          message: err instanceof Error ? err.message : 'Failed to start tracking',
        });
      }
    },
    [
      isTracking,
      hasPermission,
      requestPermissions,
      shipmentId,
      user?.id,
      options.enableBackgroundTracking,
      setTrackingActive,
      updatePosition,
      incrementEventsCount,
    ]
  );

  const stopTracking = useCallback(async () => {
    if (locationSubscription.current) {
      locationSubscription.current.remove();
      locationSubscription.current = null;
    }

    if (options.enableBackgroundTracking !== false) {
      await stopBackgroundTracking();
    }

    setTrackingInactive();
  }, [options.enableBackgroundTracking, setTrackingInactive]);

  useEffect(() => {
    return () => {
      if (locationSubscription.current) {
        locationSubscription.current.remove();
      }
    };
  }, []);

  useEffect(() => {
    requestPermissions();
  }, [requestPermissions]);

  useEffect(() => {
    isLocationTrackingActive().then((active) => {
      if (active && !isTracking) {
        stopBackgroundTracking().catch(console.error);
      }
    });
  }, [isTracking]);

  return {
    lastKnownLocation,
    isTracking,
    isLoading,
    error,
    hasPermission,
    requestPermissions,
    getCurrentLocation,
    startTracking,
    stopTracking,
    activeShipmentId,
    driverId,
  };
}