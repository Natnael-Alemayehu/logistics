import * as Location from 'expo-location';
import * as Battery from 'expo-battery';
import type { EventSubscription } from 'expo-modules-core';

export interface TrackingOptions {
  highBatteryThreshold: number;
  lowBatteryThreshold: number;
  highBatteryInterval: number;
  mediumBatteryInterval: number;
  stationaryPauseMinutes: number;
}

export interface TrackingConfig {
  interval: number | null;
  accuracy: Location.Accuracy;
  distanceInterval: number;
  checkpointOnly: boolean;
}

const DEFAULT_OPTIONS: TrackingOptions = {
  highBatteryThreshold: 50,
  lowBatteryThreshold: 20,
  highBatteryInterval: 5 * 60 * 1000,
  mediumBatteryInterval: 15 * 60 * 1000,
  stationaryPauseMinutes: 5,
};

export function getTrackingConfig(
  batteryLevel: number,
  isConnected: boolean,
  isStationary: boolean,
  options?: Partial<TrackingOptions>
): TrackingConfig {
  const opts = { ...DEFAULT_OPTIONS, ...options };

  if (batteryLevel < opts.lowBatteryThreshold) {
    console.log('[AdaptiveTracking] Low battery mode - checkpoint only');
    return {
      interval: null,
      accuracy: Location.Accuracy.Low,
      distanceInterval: 100,
      checkpointOnly: true,
    };
  }

  if (isStationary) {
    console.log('[AdaptiveTracking] Stationary mode - reduced tracking');
    return {
      interval: opts.mediumBatteryInterval * 2,
      accuracy: Location.Accuracy.Balanced,
      distanceInterval: 100,
      checkpointOnly: false,
    };
  }

  if (batteryLevel >= opts.highBatteryThreshold && isConnected) {
    console.log('[AdaptiveTracking] High battery + connected - frequent tracking');
    return {
      interval: opts.highBatteryInterval,
      accuracy: Location.Accuracy.High,
      distanceInterval: 50,
      checkpointOnly: false,
    };
  }

  console.log('[AdaptiveTracking] Medium battery - balanced tracking');
  return {
    interval: opts.mediumBatteryInterval,
    accuracy: Location.Accuracy.Balanced,
    distanceInterval: 75,
    checkpointOnly: false,
  };
}

export async function getBatteryLevel(): Promise<number> {
  try {
    const level = await Battery.getBatteryLevelAsync();
    if (level === null || level === undefined) {
      console.log('[AdaptiveTracking] Battery level unavailable, assuming 100%');
      return 100;
    }
    return level * 100;
  } catch (error) {
    console.warn('[AdaptiveTracking] Failed to get battery level:', error);
    return 100;
  }
}

export function subscribeToBatteryChanges(
  callback: (level: number) => void
): () => void {
  let subscription: EventSubscription | null = null;

  try {
    subscription = Battery.addBatteryLevelListener(({ batteryLevel }) => {
      const level = batteryLevel !== null && batteryLevel !== undefined 
        ? batteryLevel * 100 
        : 100;
      callback(level);
    });
  } catch (error) {
    console.warn('[AdaptiveTracking] Failed to subscribe to battery changes:', error);
  }

  return () => {
    if (subscription) {
      subscription.remove();
    }
  };
}

export function shouldUseLowPowerMode(batteryLevel: number): boolean {
  return batteryLevel < DEFAULT_OPTIONS.lowBatteryThreshold;
}

export function isLowBattery(batteryLevel: number): boolean {
  return batteryLevel < DEFAULT_OPTIONS.lowBatteryThreshold;
}

export function isMediumBattery(batteryLevel: number): boolean {
  return batteryLevel >= DEFAULT_OPTIONS.lowBatteryThreshold && 
         batteryLevel < DEFAULT_OPTIONS.highBatteryThreshold;
}

export function isHighBattery(batteryLevel: number): boolean {
  return batteryLevel >= DEFAULT_OPTIONS.highBatteryThreshold;
}

export { DEFAULT_OPTIONS };
