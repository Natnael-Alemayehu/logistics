import { Coordinates, BoundingBox } from '@/types/maps';

export function calculateDistance(
  coord1: Coordinates,
  coord2: Coordinates
): number {
  const toRad = (deg: number) => (deg * Math.PI) / 180;
  const R = 6371000;

  const lat1 = toRad(coord1.latitude);
  const lat2 = toRad(coord2.latitude);
  const deltaLat = toRad(coord2.latitude - coord1.latitude);
  const deltaLng = toRad(coord2.longitude - coord1.longitude);

  const a =
    Math.sin(deltaLat / 2) * Math.sin(deltaLat / 2) +
    Math.cos(lat1) * Math.cos(lat2) * Math.sin(deltaLng / 2) * Math.sin(deltaLng / 2);
  const c = 2 * Math.atan2(Math.sqrt(a), Math.sqrt(1 - a));

  return R * c;
}

export function calculateBearing(from: Coordinates, to: Coordinates): number {
  const toRad = (deg: number) => (deg * Math.PI) / 180;
  const toDeg = (rad: number) => (rad * 180) / Math.PI;

  const lat1 = toRad(from.latitude);
  const lat2 = toRad(to.latitude);
  const deltaLng = toRad(to.longitude - from.longitude);

  const y = Math.sin(deltaLng) * Math.cos(lat2);
  const x =
    Math.cos(lat1) * Math.sin(lat2) -
    Math.sin(lat1) * Math.cos(lat2) * Math.cos(deltaLng);

  let bearing = toDeg(Math.atan2(y, x));
  return (bearing + 360) % 360;
}

export function getBoundingBox(coordinates: Coordinates[]): BoundingBox {
  if (coordinates.length === 0) {
    throw new Error('Cannot calculate bounding box for empty coordinates array');
  }

  let minLat = coordinates[0].latitude;
  let maxLat = coordinates[0].latitude;
  let minLng = coordinates[0].longitude;
  let maxLng = coordinates[0].longitude;

  for (const coord of coordinates) {
    minLat = Math.min(minLat, coord.latitude);
    maxLat = Math.max(maxLat, coord.latitude);
    minLng = Math.min(minLng, coord.longitude);
    maxLng = Math.max(maxLng, coord.longitude);
  }

  return {
    ne: { latitude: maxLat, longitude: maxLng },
    sw: { latitude: minLat, longitude: minLng },
  };
}

export function isWithinGeofence(
  point: Coordinates,
  center: Coordinates,
  radiusMeters: number
): boolean {
  const distance = calculateDistance(point, center);
  return distance <= radiusMeters;
}

export function formatCoordinates(coords: Coordinates): string {
  const lat = coords.latitude.toFixed(6);
  const lng = coords.longitude.toFixed(6);
  const latDir = coords.latitude >= 0 ? 'N' : 'S';
  const lngDir = coords.longitude >= 0 ? 'E' : 'W';
  return `${Math.abs(parseFloat(lat))}° ${latDir}, ${Math.abs(parseFloat(lng))}° ${lngDir}`;
}

export function formatDistance(meters: number): string {
  if (meters < 1000) {
    return `${Math.round(meters)}m`;
  }
  return `${(meters / 1000).toFixed(1)}km`;
}

export function estimateETA(distanceMeters: number, speedKph: number = 30): number {
  const speedMps = (speedKph * 1000) / 3600;
  return distanceMeters / speedMps;
}

export * from './config';