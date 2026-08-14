import { Coordinates } from '@/types/maps';
import { isWithinGeofence, calculateDistance } from '@/services/maps';
import { insertTrackingEvent, type TrackingEventInput } from '@db';
import { getTrackingContext } from './trackingContext';
import { generateUUID } from '@utils/helpers';

export interface Geofence {
  id: string;
  coordinate: Coordinates;
  radiusMeters: number;
  type: 'origin' | 'destination' | 'checkpoint';
  shipmentId: string;
  metadata?: Record<string, any>;
}

export interface GeofenceEvent {
  geofenceId: string;
  event: 'enter' | 'exit' | 'dwell';
  timestamp: Date;
  coordinate: Coordinates;
  geofence: Geofence;
}

export const GEOFENCE_RADII = {
  origin: 200,
  destination: 500,
  checkpoint: 100,
};

const DWELL_TIME_MS = 60000;

interface GeofenceState {
  isInside: boolean;
  enteredAt: Date | null;
  lastCheckTime: Date | null;
}

const activeGeofences: Map<string, Geofence> = new Map();
const geofenceStates: Map<string, GeofenceState> = new Map();
const recentEvents: GeofenceEvent[] = [];
const MAX_RECENT_EVENTS = 50;
let isMonitoring = false;
let eventCallbacks: ((event: GeofenceEvent) => void)[] = [];

function addRecentEvent(event: GeofenceEvent): void {
  recentEvents.unshift(event);
  if (recentEvents.length > MAX_RECENT_EVENTS) {
    recentEvents.pop();
  }
  eventCallbacks.forEach(cb => cb(event));
}

export function addGeofence(geofence: Geofence): void {
  activeGeofences.set(geofence.id, geofence);
  geofenceStates.set(geofence.id, {
    isInside: false,
    enteredAt: null,
    lastCheckTime: null,
  });
  console.log(`[Geofence] Added geofence ${geofence.id} (${geofence.type}) at`, geofence.coordinate);
}

export function removeGeofence(id: string): void {
  activeGeofences.delete(id);
  geofenceStates.delete(id);
  console.log(`[Geofence] Removed geofence ${id}`);
}

export function clearGeofences(): void {
  activeGeofences.clear();
  geofenceStates.clear();
  recentEvents.length = 0;
  console.log('[Geofence] Cleared all geofences');
}

export function getGeofences(): Geofence[] {
  return Array.from(activeGeofences.values());
}

export function getRecentEvents(): GeofenceEvent[] {
  return [...recentEvents];
}

export function isGeofenceMonitoring(): boolean {
  return isMonitoring;
}

export function checkGeofences(
  position: Coordinates | null | undefined,
  callback?: (event: GeofenceEvent) => void
): void {
  if (!position || position.latitude === undefined || position.longitude === undefined) {
    console.warn('[Geofence] Invalid position provided, skipping check');
    return;
  }

  const now = new Date();

  activeGeofences.forEach((geofence) => {
    const state = geofenceStates.get(geofence.id);
    if (!state) return;

    const isInside = isWithinGeofence(position, geofence.coordinate, geofence.radiusMeters);
    const wasInside = state.isInside;

    if (isInside && !wasInside) {
      const event: GeofenceEvent = {
        geofenceId: geofence.id,
        event: 'enter',
        timestamp: now,
        coordinate: position,
        geofence,
      };
      
      state.isInside = true;
      state.enteredAt = now;
      
      addRecentEvent(event);
      callback?.(event);
      recordGeofenceEvent(event, 'enter');
      
      console.log(`[Geofence] ENTER ${geofence.type} (${geofence.id})`);
    } else if (!isInside && wasInside) {
      const event: GeofenceEvent = {
        geofenceId: geofence.id,
        event: 'exit',
        timestamp: now,
        coordinate: position,
        geofence,
      };
      
      state.isInside = false;
      state.enteredAt = null;
      
      addRecentEvent(event);
      callback?.(event);
      recordGeofenceEvent(event, 'exit');
      
      console.log(`[Geofence] EXIT ${geofence.type} (${geofence.id})`);
    } else if (isInside && wasInside && state.enteredAt) {
      const dwellTime = now.getTime() - state.enteredAt.getTime();
      if (dwellTime >= DWELL_TIME_MS && state.lastCheckTime === null) {
        const event: GeofenceEvent = {
          geofenceId: geofence.id,
          event: 'dwell',
          timestamp: now,
          coordinate: position,
          geofence,
        };
        
        state.lastCheckTime = now;
        
        addRecentEvent(event);
        callback?.(event);
        recordGeofenceEvent(event, 'dwell');
        
        console.log(`[Geofence] DWELL ${geofence.type} (${geofence.id}) - ${Math.round(dwellTime / 1000)}s`);
      }
    }

    state.lastCheckTime = now;
  });
}

async function recordGeofenceEvent(event: GeofenceEvent, eventType: 'enter' | 'exit' | 'dwell'): Promise<void> {
  try {
    const context = await getTrackingContext();
    if (!context) {
      console.warn('[Geofence] No tracking context, skipping event recording');
      return;
    }

    const trackingEvent: TrackingEventInput = {
      shipment_id: event.geofence.shipmentId,
      driver_id: context.driverId,
      latitude: event.coordinate.latitude,
      longitude: event.coordinate.longitude,
      event_type: `geofence_${eventType}`,
      status: eventType === 'enter' && event.geofence.type === 'destination' ? 'arrived' : undefined,
      note: `${eventType === 'enter' ? 'Entered' : eventType === 'exit' ? 'Left' : 'Dwelling in'} ${event.geofence.type} area`,
      geofence_id: event.geofenceId,
      geofence_type: event.geofence.type,
      recorded_at: event.timestamp.toISOString(),
      sync_priority: event.geofence.type === 'destination' ? 10 : 5,
    };

    await insertTrackingEvent(trackingEvent);
    console.log(`[Geofence] Recorded ${eventType} event for geofence ${event.geofenceId}`);
  } catch (error) {
    console.error('[Geofence] Failed to record event:', error);
  }
}

export function startGeofenceMonitoring(): void {
  if (isMonitoring) {
    console.log('[Geofence] Already monitoring');
    return;
  }
  
  isMonitoring = true;
  console.log('[Geofence] Started monitoring');
}

export function stopGeofenceMonitoring(): void {
  isMonitoring = false;
  eventCallbacks = [];
  console.log('[Geofence] Stopped monitoring');
}

export function onGeofenceEvent(callback: (event: GeofenceEvent) => void): () => void {
  eventCallbacks.push(callback);
  return () => {
    eventCallbacks = eventCallbacks.filter(cb => cb !== callback);
  };
}

export function createShipmentGeofences(shipment: {
  id: string;
  origin_lat?: number;
  origin_lng?: number;
  destination_lat?: number;
  destination_lng?: number;
}): Geofence[] {
  const geofences: Geofence[] = [];

  if (shipment.origin_lat !== undefined && shipment.origin_lng !== undefined) {
    geofences.push({
      id: `${shipment.id}-origin`,
      coordinate: {
        latitude: shipment.origin_lat,
        longitude: shipment.origin_lng,
      },
      radiusMeters: GEOFENCE_RADII.origin,
      type: 'origin',
      shipmentId: shipment.id,
      metadata: { addressType: 'origin' },
    });
  }

  if (shipment.destination_lat !== undefined && shipment.destination_lng !== undefined) {
    geofences.push({
      id: `${shipment.id}-destination`,
      coordinate: {
        latitude: shipment.destination_lat,
        longitude: shipment.destination_lng,
      },
      radiusMeters: GEOFENCE_RADII.destination,
      type: 'destination',
      shipmentId: shipment.id,
      metadata: { addressType: 'destination' },
    });
  }

  return geofences;
}

export function getGeofenceState(id: string): GeofenceState | undefined {
  return geofenceStates.get(id);
}

export function isInsideGeofence(id: string): boolean {
  const state = geofenceStates.get(id);
  return state?.isInside ?? false;
}

export function getDistanceToGeofence(position: Coordinates, geofenceId: string): number | null {
  const geofence = activeGeofences.get(geofenceId);
  if (!geofence) return null;
  return calculateDistance(position, geofence.coordinate);
}
