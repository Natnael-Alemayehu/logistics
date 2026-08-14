import { Platform } from 'react-native';

// State variables
let MapLibreGL: typeof import('@maplibre/maplibre-react-native').default | null = null;
let isMapLibreAvailable = false;
let hasCheckedModule = false;

/**
 * Safely check if MapLibreGL is available without throwing
 * Uses lazy loading to avoid crashes during module initialization
 */
function checkMapLibreAvailability(): boolean {
  if (hasCheckedModule) {
    return isMapLibreAvailable;
  }
  
  hasCheckedModule = true;
  
  // On web, we don't have the native module
  if (Platform.OS === 'web') {
    isMapLibreAvailable = false;
    return false;
  }
  
  try {
    // Try to load the module - this may throw if native code isn't available
    const module = require('@maplibre/maplibre-react-native');
    
    // Check if the module has the expected exports
    if (module && module.default && typeof module.default.MapView === 'function') {
      MapLibreGL = module.default;
      isMapLibreAvailable = true;
    } else {
      console.log('MapLibreGL module exists but missing expected exports');
      isMapLibreAvailable = false;
    }
  } catch (error) {
    // Module not available (Expo Go or not installed)
    const errorMessage = error instanceof Error ? error.message : String(error);
    
    // Only log once and don't show the full error in production
    if (__DEV__) {
      console.log('MapLibreGL not available - using fallback. Reason:', errorMessage);
    }
    
    isMapLibreAvailable = false;
  }
  
  return isMapLibreAvailable;
}

/**
 * Check if MapLibreGL is available (requires development build, not Expo Go)
 */
export function isMapAvailable(): boolean {
  return checkMapLibreAvailability();
}

/**
 * Check if running in Expo Go
 * MapLibreGL requires a development build
 */
export function isExpoGo(): boolean {
  return !isMapAvailable() && Platform.OS !== 'web';
}

/**
 * Get MapLibreGL module or null if not available
 */
export function getMapLibre(): typeof import('@maplibre/maplibre-react-native').default | null {
  checkMapLibreAvailability();
  return MapLibreGL;
}

// Re-export types that don't depend on native module
export type { CameraRef, MapViewRef } from '@maplibre/maplibre-react-native';

// Export the module (will be null if not available)
export { MapLibreGL };
