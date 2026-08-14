import type { ShipmentStatus } from './shipment';

export interface Coordinates {
  latitude: number;
  longitude: number;
}

export interface BoundingBox {
  ne: Coordinates;
  sw: Coordinates;
}

export interface MapMarker {
  id: string;
  coordinate: Coordinates;
  title?: string;
  description?: string;
  type: 'current' | 'destination' | 'checkpoint' | 'custom';
}

export interface LocationMarkerData extends MapMarker {
  type: 'current';
  heading?: number;
  accuracy?: number;
  speed?: number;
  isTracking: boolean;
}

export interface DestinationMarkerData extends MapMarker {
  type: 'destination';
  shipmentId: string;
  trackingNumber: string;
  status: ShipmentStatus;
  customerName: string;
  distance?: number;
  eta?: number;
}

export interface CheckpointMarkerData extends MapMarker {
  type: 'checkpoint';
  checkpointType: 'pickup' | 'dropoff' | 'waypoint' | 'custom';
  timestamp?: string;
  passed: boolean;
  sequence?: number;
}

export type MarkerType = 'current' | 'destination' | 'checkpoint' | 'origin' | 'custom';

export interface MapRegion {
  latitude: number;
  longitude: number;
  latitudeDelta: number;
  longitudeDelta: number;
}

export interface ShipmentMapData {
  id: string;
  trackingNumber: string;
  status: ShipmentStatus;
  origin?: Coordinates;
  destination?: Coordinates;
  customerName: string;
  customerPhone?: string;
  destinationAddress: string;
  checkpoints?: CheckpointData[];
}

export interface CheckpointData {
  id: string;
  coordinate: Coordinates;
  type: 'pickup' | 'dropoff' | 'waypoint' | 'custom';
  name?: string;
  timestamp?: string;
  passed: boolean;
  sequence: number;
}

export interface RouteSegment {
  start: Coordinates;
  end: Coordinates;
  distanceMeters: number;
  durationSeconds: number;
}

export const DEFAULT_MAP_CENTER: Coordinates = {
  latitude: 8.9806,
  longitude: 38.7578,
};

export const DEFAULT_ZOOM_LEVEL = 12;

export const MARKER_COLORS = {
  current: '#2563eb',
  destination: '#dc2626',
  checkpoint: '#f59e0b',
  origin: '#22c55e',
  passed: '#9ca3af',
} as const;

export const STATUS_COLORS: Record<ShipmentStatus, string> = {
  pending: '#f59e0b',
  assigned: '#3b82f6',
  in_transit: '#2563eb',
  delayed: '#ef4444',
  arrived: '#8b5cf6',
  delivered: '#22c55e',
  issue: '#ef4444',
  cancelled: '#6b7280',
};

export function toGeoJSONCoordinates(coords: Coordinates): [number, number] {
  return [coords.longitude, coords.latitude];
}

export function fromGeoJSONCoordinates(coords: [number, number]): Coordinates {
  return { longitude: coords[0], latitude: coords[1] };
}
