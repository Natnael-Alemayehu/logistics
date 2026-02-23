import React, { forwardRef, useState, useCallback, useRef } from 'react';
import { View, StyleSheet, ActivityIndicator, Text } from 'react-native';
import MapLibreGL, { type CameraRef, type MapViewRef } from '@maplibre/maplibre-react-native';
import * as Location from 'expo-location';
import { MAP_STYLE, DEFAULT_CENTER, DEFAULT_ZOOM, MAX_ZOOM, MIN_ZOOM } from '@/services/maps/config';
import { Coordinates } from '@/types/maps';

MapLibreGL.setAccessToken(null);

interface MapContainerProps {
  children?: React.ReactNode;
  style?: object;
  showUserLocation?: boolean;
  initialCenter?: Coordinates;
  onMapReady?: () => void;
  onError?: (error: Error) => void;
}

export interface MapRef {
  getCamera: () => CameraRef | null;
  moveTo: (coordinates: Coordinates, zoom?: number) => void;
  fitBounds: (ne: Coordinates, sw: Coordinates, padding?: number) => void;
}

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
    const cameraRef = useRef<CameraRef>(null);
    const mapRef = useRef<MapViewRef>(null);

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
});

export default MapContainer;