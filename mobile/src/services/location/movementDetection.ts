export interface LocationCoords {
  latitude: number;
  longitude: number;
  timestamp: number;
  speed?: number | null;
}

export interface MovementDetectionOptions {
  stationaryThreshold: number;
  minPositionsForDetection: number;
  maxPositionAge: number;
  movementSpeedThreshold: number;
}

export interface MovementState {
  isMoving: boolean;
  stationarySince: Date | null;
  lastPositions: LocationCoords[];
  averageSpeed: number;
}

const DEFAULT_OPTIONS: MovementDetectionOptions = {
  stationaryThreshold: 5 * 60 * 1000,
  minPositionsForDetection: 3,
  maxPositionAge: 10 * 60 * 1000,
  movementSpeedThreshold: 2,
};

export function detectMovement(
  positions: LocationCoords[],
  options?: Partial<MovementDetectionOptions>
): MovementState {
  const opts = { ...DEFAULT_OPTIONS, ...options };
  const now = Date.now();

  const recentPositions = positions.filter(
    pos => now - pos.timestamp < opts.maxPositionAge
  );

  if (recentPositions.length < opts.minPositionsForDetection) {
    return {
      isMoving: true,
      stationarySince: null,
      lastPositions: recentPositions,
      averageSpeed: 0,
    };
  }

  const speeds = recentPositions
    .map(p => p.speed)
    .filter((s): s is number => s !== null && s !== undefined && s >= 0);

  const averageSpeed = speeds.length > 0
    ? speeds.reduce((sum, s) => sum + s, 0) / speeds.length
    : 0;

  const isMoving = averageSpeed > opts.movementSpeedThreshold;

  let stationarySince: Date | null = null;
  if (!isMoving && recentPositions.length > 0) {
    const oldestRecent = recentPositions[recentPositions.length - 1];
    const stationaryDuration = now - oldestRecent.timestamp;
    
    if (stationaryDuration >= opts.stationaryThreshold) {
      stationarySince = new Date(oldestRecent.timestamp);
    }
  }

  return {
    isMoving,
    stationarySince,
    lastPositions: recentPositions,
    averageSpeed,
  };
}

export function shouldPauseTracking(state: MovementState): boolean {
  if (state.isMoving) {
    return false;
  }

  if (!state.stationarySince) {
    return false;
  }

  const stationaryDuration = Date.now() - state.stationarySince.getTime();
  return stationaryDuration >= DEFAULT_OPTIONS.stationaryThreshold;
}

type MovementCallback = (isMoving: boolean) => void;
const movementListeners: Set<MovementCallback> = new Set();
let lastMovementState: boolean | null = null;

export function onMovementStateChange(
  callback: (isMoving: boolean) => void
): () => void {
  movementListeners.add(callback);
  
  return () => {
    movementListeners.delete(callback);
  };
}

export function notifyMovementStateChange(isMoving: boolean): void {
  if (lastMovementState !== isMoving) {
    lastMovementState = isMoving;
    movementListeners.forEach(callback => callback(isMoving));
  }
}

export function calculateDistance(
  pos1: LocationCoords,
  pos2: LocationCoords
): number {
  const R = 6371000;
  const lat1 = pos1.latitude * Math.PI / 180;
  const lat2 = pos2.latitude * Math.PI / 180;
  const deltaLat = (pos2.latitude - pos1.latitude) * Math.PI / 180;
  const deltaLon = (pos2.longitude - pos1.longitude) * Math.PI / 180;

  const a = Math.sin(deltaLat / 2) * Math.sin(deltaLat / 2) +
    Math.cos(lat1) * Math.cos(lat2) *
    Math.sin(deltaLon / 2) * Math.sin(deltaLon / 2);
  const c = 2 * Math.atan2(Math.sqrt(a), Math.sqrt(1 - a));

  return R * c;
}

export function isPositionStagnant(
  positions: LocationCoords[],
  thresholdMeters: number = 10
): boolean {
  if (positions.length < 2) return false;

  const recent = positions.slice(0, Math.min(5, positions.length));
  
  for (let i = 1; i < recent.length; i++) {
    if (calculateDistance(recent[0], recent[i]) > thresholdMeters) {
      return false;
    }
  }

  return true;
}

export { DEFAULT_OPTIONS };
