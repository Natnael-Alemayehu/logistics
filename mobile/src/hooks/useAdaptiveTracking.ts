import { useState, useEffect, useCallback, useRef } from 'react';
import * as Location from 'expo-location';
import {
  getTrackingConfig,
  getBatteryLevel,
  subscribeToBatteryChanges,
  TrackingConfig,
  TrackingOptions,
} from '@/services/location/adaptiveTracking';
import {
  detectMovement,
  shouldPauseTracking,
  onMovementStateChange,
  notifyMovementStateChange,
  LocationCoords,
  MovementState,
} from '@/services/location/movementDetection';
import { useNetworkStore } from '@/store/networkStore';
import { useTrackingStore } from '@/store/trackingStore';
import { useAuthStore } from '@/store/authStore';
import { startLocationTracking, stopLocationTracking, getCurrentLocation } from '@/services/location';
import { insertTrackingEvent } from '@db';

interface UseAdaptiveTrackingResult {
  isTracking: boolean;
  currentConfig: TrackingConfig | null;
  batteryLevel: number;
  isStationary: boolean;
  startAdaptiveTracking: (shipmentId: string) => Promise<void>;
  stopAdaptiveTracking: () => Promise<void>;
  forceCheckpoint: () => Promise<void>;
  overrideConfig: (config: Partial<TrackingConfig>) => void;
  clearOverride: () => void;
}

type InternalLocationCoords = {
  latitude: number;
  longitude: number;
  timestamp: number;
  speed?: number | null;
};

export function useAdaptiveTracking(
  options?: Partial<TrackingOptions>
): UseAdaptiveTrackingResult {
  const [isTracking, setIsTracking] = useState(false);
  const [currentConfig, setCurrentConfig] = useState<TrackingConfig | null>(null);
  const [batteryLevel, setBatteryLevel] = useState(100);
  const [isStationary, setIsStationary] = useState(false);
  const [configOverride, setConfigOverride] = useState<Partial<TrackingConfig> | null>(null);

  const shipmentIdRef = useRef<string | null>(null);
  const driverIdRef = useRef<string | null>(null);
  const positionHistoryRef = useRef<LocationCoords[]>([]);
  const trackingIntervalRef = useRef<NodeJS.Timeout | null>(null);
  const movementCheckRef = useRef<NodeJS.Timeout | null>(null);

  const isConnected = useNetworkStore(state => state.isOnline);
  const { updatePosition: updateStorePosition, incrementEventsCount } = useTrackingStore();
  const user = useAuthStore(state => state.user);

  useEffect(() => {
    getBatteryLevel().then(setBatteryLevel);
    const unsubscribe = subscribeToBatteryChanges(setBatteryLevel);
    return unsubscribe;
  }, []);

  const updateConfig = useCallback(() => {
    if (!isTracking) return;

    const newConfig = getTrackingConfig(
      batteryLevel,
      isConnected,
      isStationary,
      options
    );

    const finalConfig = configOverride
      ? { ...newConfig, ...configOverride }
      : newConfig;

    setCurrentConfig(finalConfig);
    console.log('[AdaptiveTracking] Config updated:', finalConfig);

    if (finalConfig.checkpointOnly && trackingIntervalRef.current) {
      clearInterval(trackingIntervalRef.current);
      trackingIntervalRef.current = null;
    }
  }, [batteryLevel, isConnected, isStationary, isTracking, configOverride, options]);

  useEffect(() => {
    updateConfig();
  }, [updateConfig]);

  const recordPosition = useCallback(async (force: boolean = false) => {
    if (!shipmentIdRef.current || !driverIdRef.current) return;

    const config = currentConfig;
    if (!config && !force) return;

    try {
      const location = await getCurrentLocation();
      if (!location) return;

      const position: LocationCoords = {
        latitude: location.latitude,
        longitude: location.longitude,
        timestamp: Date.now(),
        speed: location.speed,
      };

      positionHistoryRef.current = [
        position,
        ...positionHistoryRef.current.slice(0, 49),
      ];

      const movementState: MovementState = detectMovement(
        positionHistoryRef.current
      );
      setIsStationary(!movementState.isMoving);
      notifyMovementStateChange(movementState.isMoving);

      const shouldPause = shouldPauseTracking(movementState);
      if (shouldPause && !force) {
        console.log('[AdaptiveTracking] Skipping - stationary');
        return;
      }

      await insertTrackingEvent({
        shipment_id: shipmentIdRef.current,
        driver_id: driverIdRef.current,
        latitude: location.latitude,
        longitude: location.longitude,
        accuracy: location.accuracy ?? undefined,
        speed: location.speed ? location.speed * 3.6 : undefined,
        heading: location.heading ?? undefined,
        event_type: force ? 'checkpoint' : 'gps_ping',
        recorded_at: new Date().toISOString(),
      });

      console.log('[AdaptiveTracking] Position recorded');
    } catch (error) {
      console.error('[AdaptiveTracking] Failed to record position:', error);
    }
  }, [currentConfig]);

  const startAdaptiveTracking = useCallback(async (shipmentId: string) => {
    const driverId = user?.id;
    if (!driverId) {
      console.error('[AdaptiveTracking] No driver ID available');
      return;
    }

    shipmentIdRef.current = shipmentId;
    driverIdRef.current = driverId;
    setIsTracking(true);

    const initialConfig = getTrackingConfig(
      batteryLevel,
      isConnected,
      false,
      options
    );
    setCurrentConfig(initialConfig);

    await startLocationTracking(shipmentId, driverId);

    if (!initialConfig.checkpointOnly && initialConfig.interval) {
      trackingIntervalRef.current = setInterval(() => {
        recordPosition();
      }, initialConfig.interval);
    }

    movementCheckRef.current = setInterval(() => {
      const state = detectMovement(positionHistoryRef.current);
      setIsStationary(!state.isMoving);
    }, 60000);

    console.log('[AdaptiveTracking] Started tracking for shipment:', shipmentId);
  }, [batteryLevel, isConnected, options, recordPosition, user?.id]);

  const stopAdaptiveTracking = useCallback(async () => {
    setIsTracking(false);
    shipmentIdRef.current = null;
    driverIdRef.current = null;
    setCurrentConfig(null);
    positionHistoryRef.current = [];

    if (trackingIntervalRef.current) {
      clearInterval(trackingIntervalRef.current);
      trackingIntervalRef.current = null;
    }

    if (movementCheckRef.current) {
      clearInterval(movementCheckRef.current);
      movementCheckRef.current = null;
    }

    await stopLocationTracking();
    console.log('[AdaptiveTracking] Stopped tracking');
  }, []);

  const forceCheckpoint = useCallback(async () => {
    await recordPosition(true);
    console.log('[AdaptiveTracking] Forced checkpoint recorded');
  }, [recordPosition]);

  const overrideConfig = useCallback((override: Partial<TrackingConfig>) => {
    setConfigOverride(override);
    console.log('[AdaptiveTracking] Config override set:', override);
  }, []);

  const clearOverride = useCallback(() => {
    setConfigOverride(null);
    console.log('[AdaptiveTracking] Config override cleared');
  }, []);

  useEffect(() => {
    const unsubscribe = onMovementStateChange((moving) => {
      console.log('[AdaptiveTracking] Movement state changed:', moving ? 'moving' : 'stationary');
    });
    return unsubscribe;
  }, []);

  useEffect(() => {
    return () => {
      if (trackingIntervalRef.current) {
        clearInterval(trackingIntervalRef.current);
      }
      if (movementCheckRef.current) {
        clearInterval(movementCheckRef.current);
      }
    };
  }, []);

  return {
    isTracking,
    currentConfig,
    batteryLevel,
    isStationary,
    startAdaptiveTracking,
    stopAdaptiveTracking,
    forceCheckpoint,
    overrideConfig,
    clearOverride,
  };
}

export type { UseAdaptiveTrackingResult, TrackingConfig };
