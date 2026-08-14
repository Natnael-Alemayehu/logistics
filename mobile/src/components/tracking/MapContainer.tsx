import React, { forwardRef, useState, useCallback, useRef } from 'react';
import { View, StyleSheet, ActivityIndicator, Text, Pressable, Linking } from 'react-native';
import * as Location from 'expo-location';
import { MAP_STYLE, DEFAULT_CENTER, DEFAULT_ZOOM, MAX_ZOOM, MIN_ZOOM } from '@/services/maps/config';
import { Coordinates } from '@/types/maps';
import { isMapAvailable, getMapLibre } from '@/services/maps/native';
import { colors } from '@/utils/theme';

interface MapContainerProps {
  children?: React.ReactNode;
  style?: object;
  showUserLocation?: boolean;
  initialCenter?: Coordinates;
  onMapReady?: () => void;
  onError?: (error: Error) => void;
}

export interface MapRef {
  getCamera: () => unknown | null;
  moveTo: (coordinates: Coordinates, zoom?: number) => void;
  fitBounds: (ne: Coordinates, sw: Coordinates, padding?: number) => void;
}

// Check if map is available
const mapAvailable = isMapAvailable();

const MapContainer = forwardRef<MapRef, MapContainerProps>(
  (
    {
      children,
      style,
      showUserLocation = true,
      initialCenter,
      onMapReady,
      onError,
    },
    ref
  ) => {
    const [isLoading, setIsLoading] = useState(true);
    const [hasError, setHasError] = useState(false);
    const [errorMessage, setErrorMessage] = useState<string>('');
    const [hasLocationPermission, setHasLocationPermission] = useState(false);
    const cameraRef = useRef<React.ComponentRef<NonNullable<ReturnType<typeof getMapLibre>>['Camera']>>(null);
    const mapRef = useRef<React.ComponentRef<NonNullable<ReturnType<typeof getMapLibre>>['MapView']>>(null);

    React.useImperativeHandle(ref, () => ({
      getCamera: () => cameraRef.current,
      moveTo: (coordinates: Coordinates, zoom?: number) => {
        cameraRef.current?.setCamera({
          centerCoordinate: [coordinates.longitude, coordinates.latitude],
          zoomLevel: zoom ?? DEFAULT_ZOOM,
          animationMode: 'flyTo',
          animationDuration: 1000,
        });
      },
      fitBounds: (ne: Coordinates, sw: Coordinates, padding = 50) => {
        cameraRef.current?.fitBounds(
          [sw.longitude, sw.latitude],
          [ne.longitude, ne.latitude],
          [padding, padding, padding, padding],
          1000
        );
      },
    }));

    const checkLocationPermission = useCallback(async () => {
      if (!showUserLocation) return;

      try {
        const { status } = await Location.getForegroundPermissionsAsync();
        if (status === 'granted') {
          setHasLocationPermission(true);
        } else {
          const { status: newStatus } = await Location.requestForegroundPermissionsAsync();
          setHasLocationPermission(newStatus === 'granted');
        }
      } catch (error) {
        console.warn('Location permission check failed:', error);
      }
    }, [showUserLocation]);

    React.useEffect(() => {
      checkLocationPermission();
    }, [checkLocationPermission]);

    const handleMapReady = useCallback(() => {
      setIsLoading(false);
      onMapReady?.();
    }, [onMapReady]);

    const handleMapError = useCallback(
      (message: string) => {
        setHasError(true);
        setErrorMessage(message);
        setIsLoading(false);
        onError?.(new Error(message));
      },
      [onError]
    );

    const handleWillStartLoadingMap = useCallback(() => {
      setIsLoading(true);
      setHasError(false);
    }, []);

    const handleDidFinishLoadingMap = useCallback(() => {
      setIsLoading(false);
    }, []);

    const handleDidFailLoadingMap = useCallback(() => {
      handleMapError('Failed to load map');
    }, [handleMapError]);

    const handleOpenInMaps = () => {
      const center = initialCenter 
        ? { lat: initialCenter.latitude, lng: initialCenter.longitude }
        : { lat: DEFAULT_CENTER[0], lng: DEFAULT_CENTER[1] };
      Linking.openURL(`https://www.google.com/maps?q=${center.lat},${center.lng}`);
    };

    // If map is not available (Expo Go), show fallback
    if (!mapAvailable) {
      return (
        <View style={[styles.container, style, styles.fallbackContainer]}>
          <View style={styles.fallbackContent}>
            <Text style={styles.fallbackIcon}>🗺️</Text>
            <Text style={styles.fallbackTitle}>Map View</Text>
            <Text style={styles.fallbackMessage}>
              Interactive map requires a development build.
            </Text>
            <Text style={styles.fallbackSubtext}>
              You're running in Expo Go, which doesn't support the native map library.
            </Text>
            <Pressable style={styles.fallbackButton} onPress={handleOpenInMaps}>
              <Text style={styles.fallbackButtonText}>Open in Google Maps</Text>
            </Pressable>
          </View>
        </View>
      );
    }

    const MapLibreGL = getMapLibre();
    if (!MapLibreGL) {
      return (
        <View style={[styles.container, style, styles.fallbackContainer]}>
          <Text style={styles.fallbackMessage}>Map unavailable</Text>
        </View>
      );
    }

    if (hasError) {
      return (
        <View style={[styles.container, style, styles.errorContainer]}>
          <Text style={styles.errorText}>Unable to load map</Text>
          <Text style={styles.errorDetail}>{errorMessage}</Text>
        </View>
      );
    }

    const centerCoordinate = initialCenter
      ? [initialCenter.longitude, initialCenter.latitude]
      : DEFAULT_CENTER;

    return (
      <View style={[styles.container, style]}>
        <MapLibreGL.MapView
          ref={mapRef}
          style={styles.map}
          mapStyle={MAP_STYLE}
          onDidFailLoadingMap={handleDidFailLoadingMap}
          onWillStartLoadingMap={handleWillStartLoadingMap}
          onDidFinishLoadingMap={handleDidFinishLoadingMap}
          onDidFinishRenderingMapFully={handleMapReady}
          logoEnabled={false}
          attributionEnabled={false}
          compassEnabled={true}
        >
          <MapLibreGL.Camera
            ref={cameraRef}
            centerCoordinate={centerCoordinate as [number, number]}
            zoomLevel={DEFAULT_ZOOM}
            minZoomLevel={MIN_ZOOM}
            maxZoomLevel={MAX_ZOOM}
            animationMode="flyTo"
          />

          {showUserLocation && hasLocationPermission && (
            <MapLibreGL.UserLocation
              visible={true}
              showsUserHeadingIndicator={true}
              animated={true}
            />
          )}

          {children}
        </MapLibreGL.MapView>

        {isLoading && (
          <View style={styles.loadingOverlay}>
            <ActivityIndicator size="large" color="#2563eb" />
            <Text style={styles.loadingText}>Loading map...</Text>
          </View>
        )}
      </View>
    );
  }
);

MapContainer.displayName = 'MapContainer';

const styles = StyleSheet.create({
  container: {
    flex: 1,
  },
  map: {
    flex: 1,
  },
  loadingOverlay: {
    ...StyleSheet.absoluteFillObject,
    backgroundColor: 'rgba(255, 255, 255, 0.8)',
    justifyContent: 'center',
    alignItems: 'center',
  },
  loadingText: {
    marginTop: 12,
    fontSize: 16,
    color: '#374151',
  },
  errorContainer: {
    justifyContent: 'center',
    alignItems: 'center',
    backgroundColor: '#fef2f2',
    padding: 20,
  },
  errorText: {
    fontSize: 18,
    fontWeight: '600',
    color: '#dc2626',
    marginBottom: 8,
  },
  errorDetail: {
    fontSize: 14,
    color: '#7f1d1d',
    textAlign: 'center',
  },
  fallbackContainer: {
    justifyContent: 'center',
    alignItems: 'center',
    backgroundColor: colors.gray[100],
  },
  fallbackContent: {
    alignItems: 'center',
    padding: 20,
  },
  fallbackIcon: {
    fontSize: 48,
    marginBottom: 12,
  },
  fallbackTitle: {
    fontSize: 20,
    fontWeight: '600',
    color: colors.text,
    marginBottom: 8,
  },
  fallbackMessage: {
    fontSize: 14,
    color: colors.textSecondary,
    textAlign: 'center',
    marginBottom: 4,
  },
  fallbackSubtext: {
    fontSize: 12,
    color: colors.gray[500],
    textAlign: 'center',
    marginBottom: 16,
  },
  fallbackButton: {
    backgroundColor: colors.primary,
    paddingHorizontal: 20,
    paddingVertical: 12,
    borderRadius: 8,
  },
  fallbackButtonText: {
    color: '#fff',
    fontSize: 14,
    fontWeight: '600',
  },
});

export default MapContainer;
