import { create } from 'zustand';
import type { LocationCoords } from '@/hooks/useLocation';
import type { TrackingConfig } from '@/services/location/trackingContext';
import { DEFAULT_TRACKING_CONFIG } from '@/services/location/trackingContext';

interface TrackingState {
  isTracking: boolean;
  activeShipmentId: string | null;
  driverId: string | null;
  trackingConfig: TrackingConfig;
  batteryLevel: number;
  isLowBattery: boolean;
  lastPosition: LocationCoords | null;
  isStationary: boolean;
  stationarySince: Date | null;
  totalDistanceMeters: number;
  trackingStartTime: Date | null;
  eventsRecorded: number;

  startTracking: (shipmentId: string, driverId: string) => void;
  stopTracking: () => void;
  updatePosition: (position: LocationCoords) => void;
  updateBatteryLevel: (level: number) => void;
  setStationary: (isStationary: boolean) => void;
  incrementEventsCount: () => void;
  resetStats: () => void;
  setTrackingConfig: (config: Partial<TrackingConfig>) => void;
}

export const useTrackingStore = create<TrackingState>((set, get) => ({
  isTracking: false,
  activeShipmentId: null,
  driverId: null,
  trackingConfig: DEFAULT_TRACKING_CONFIG,
  batteryLevel: 100,
  isLowBattery: false,
  lastPosition: null,
  isStationary: false,
  stationarySince: null,
  totalDistanceMeters: 0,
  trackingStartTime: null,
  eventsRecorded: 0,

  startTracking: (shipmentId: string, driverId: string) => {
    set({
      isTracking: true,
      activeShipmentId: shipmentId,
      driverId: driverId,
      trackingStartTime: new Date(),
      totalDistanceMeters: 0,
      eventsRecorded: 0,
      isStationary: false,
      stationarySince: null,
    });
  },

  stopTracking: () => {
    set({
      isTracking: false,
      activeShipmentId: null,
      driverId: null,
      lastPosition: null,
      isStationary: false,
      stationarySince: null,
    });
  },

  updatePosition: (position: LocationCoords) => {
    const { lastPosition, totalDistanceMeters } = get();
    let newDistance = totalDistanceMeters;

    if (lastPosition) {
      const distance = calculateDistance(
        lastPosition.latitude,
        lastPosition.longitude,
        position.latitude,
        position.longitude
      );
      newDistance += distance;
    }

    set({
      lastPosition: position,
      totalDistanceMeters: newDistance,
    });
  },

  updateBatteryLevel: (level: number) => {
    set({
      batteryLevel: level,
      isLowBattery: level < 20,
    });
  },

  setStationary: (isStationary: boolean) => {
    const current = get();
    if (current.isStationary !== isStationary) {
      set({
        isStationary,
        stationarySince: isStationary ? new Date() : null,
      });
    }
  },

  incrementEventsCount: () => {
    set((state) => ({ eventsRecorded: state.eventsRecorded + 1 }));
  },

  resetStats: () => {
    set({
      totalDistanceMeters: 0,
      eventsRecorded: 0,
      trackingStartTime: new Date(),
      isStationary: false,
      stationarySince: null,
    });
  },

  setTrackingConfig: (config: Partial<TrackingConfig>) => {
    set((state) => ({
      trackingConfig: { ...state.trackingConfig, ...config },
    }));
  },
}));

function calculateDistance(lat1: number, lon1: number, lat2: number, lon2: number): number {
  const R = 6371000;
  const dLat = toRadians(lat2 - lat1);
  const dLon = toRadians(lon2 - lon1);
  const a =
    Math.sin(dLat / 2) * Math.sin(dLat / 2) +
    Math.cos(toRadians(lat1)) * Math.cos(toRadians(lat2)) * Math.sin(dLon / 2) * Math.sin(dLon / 2);
  const c = 2 * Math.atan2(Math.sqrt(a), Math.sqrt(1 - a));
  return R * c;
}

function toRadians(degrees: number): number {
  return degrees * (Math.PI / 180);
}