export const GPS_TRACKING_INTERVALS = {
  highAccuracy: {
    intervalMs: 5000,
    distanceFilter: 5,
    description: 'Updates every 5 seconds or 5 meters',
  },
  mediumAccuracy: {
    intervalMs: 15000,
    distanceFilter: 20,
    description: 'Updates every 15 seconds or 20 meters',
  },
  lowAccuracy: {
    intervalMs: 60000,
    distanceFilter: 100,
    description: 'Updates every 60 seconds or 100 meters',
  },
};

export const SYNC_INTERVALS = {
  location: {
    intervalMs: 30000,
    description: 'Sync location data every 30 seconds',
  },
  tasks: {
    intervalMs: 60000,
    description: 'Sync tasks every minute',
  },
  fullSync: {
    intervalMs: 300000,
    description: 'Full sync every 5 minutes',
  },
};

export const PHOTO_LIMITS = {
  maxCount: 5,
  maxSizeBytes: 5 * 1024 * 1024,
  maxWidth: 1920,
  maxHeight: 1080,
  quality: 0.8,
  allowedFormats: ['jpg', 'jpeg', 'png'],
};

export const LOCATION_CONFIG = {
  verificationRadiusMeters: 100,
  gpsTimeoutMs: 30000,
  minAccuracyMeters: 50,
  maxAgeMs: 60000,
};

export const SYNC_CONFIG = {
  batchSize: 50,
  maxRetries: 3,
  retryDelayMs: 1000,
  backoffMultiplier: 2,
  maxRetryDelayMs: 30000,
};

export const BATTERY_THRESHOLDS = {
  lowBattery: 20,
  criticalBattery: 10,
  highAccuracyMin: 30,
  mediumAccuracyMin: 15,
};

export const AUTH_CONFIG = {
  sessionTimeoutMs: 24 * 60 * 60 * 1000,
  refreshTokenThresholdMs: 60 * 60 * 1000,
  maxPinAttempts: 5,
  lockoutDurationMs: 5 * 60 * 1000,
};

export const CACHE_CONFIG = {
  defaultTtlMs: 5 * 60 * 1000,
  maxEntries: 100,
};

export const APP_CONFIG = {
  appName: 'Logistics Driver',
  supportPhone: '+251911234567',
  defaultCurrency: 'ETB',
  dateFormat: 'DD/MM/YYYY',
  timeFormat: 'HH:mm',
};