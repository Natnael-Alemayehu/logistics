import React, { useMemo, useEffect, useState } from 'react';
import { View, Text, StyleSheet, Pressable, ActivityIndicator } from 'react-native';
import MapLibreGL from '@maplibre/maplibre-react-native';
import { router } from 'expo-router';
import { colors, spacing } from '@/utils/theme';
import { MAP_STYLE, DEFAULT_CENTER, DEFAULT_ZOOM, calculateDistance } from '@/services/maps';
import { getCurrentLocation } from '@/services/location';
import type { Shipment } from '@/types/shipment';
import type { Coordinates } from '@/types/maps';

interface ShipmentMiniMapProps {
  shipment: Shipment;
  isTracking?: boolean;
  height?: number;
}

export function ShipmentMiniMap({ shipment, isTracking = false, height = 150 }: ShipmentMiniMapProps) {
  const [currentLocation, setCurrentLocation] = useState<{ latitude: number; longitude: number } | null>(null);
  const [distance, setDistance] = useState<number | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  const destination = useMemo(() => {
    if (shipment.destination_lat && shipment.destination_lng) {
      return {
        latitude: shipment.destination_lat,
        longitude: shipment.destination_lng,
      };
    }
    return null;
  }, [shipment.destination_lat, shipment.destination_lng]);

  const mapCenter = useMemo(() => {
    if (destination) {
      return [destination.longitude, destination.latitude] as [number, number];
    }
    return DEFAULT_CENTER;
  }, [destination]);

  useEffect(() => {
    if (isTracking && destination) {
      const loadLocation = async () => {
        try {
          const location = await getCurrentLocation();
          if (location) {
            setCurrentLocation({ latitude: location.latitude, longitude: location.longitude });
            const dist = calculateDistance(
              { latitude: location.latitude, longitude: location.longitude } as Coordinates,
              { latitude: destination.latitude, longitude: destination.longitude } as Coordinates
            );
            setDistance(dist);
          }
        } catch (error) {
          console.error('Failed to get current location:', error);
        } finally {
          setIsLoading(false);
        }
      };
      loadLocation();
      
      const interval = setInterval(loadLocation, 10000);
      return () => clearInterval(interval);
    } else {
      setIsLoading(false);
    }
    return undefined;
  }, [isTracking, destination]);

  const handlePress = () => {
    if (shipment.destination_lat && shipment.destination_lng) {
      router.push(`/map?shipmentId=${shipment.id}`);
    }
  };

  const formatDistance = (meters: number): string => {
    if (meters >= 1000) {
      return `${(meters / 1000).toFixed(1)} km`;
    }
    return `${Math.round(meters)} m`;
  };

  if (!destination) {
    return (
      <View style={[styles.placeholder, { height }]}>
        <Text style={styles.placeholderText}>No destination coordinates</Text>
      </View>
    );
  }

  return (
    <Pressable style={[styles.container, { height }]} onPress={handlePress}>
      <MapLibreGL.MapView
        style={styles.map}
        mapStyle={MAP_STYLE}
        logoEnabled={false}
        attributionEnabled={false}
        scrollEnabled={false}
        zoomEnabled={false}
        rotateEnabled={false}
      >
        <MapLibreGL.Camera
          zoomLevel={DEFAULT_ZOOM}
          centerCoordinate={mapCenter}
        />
        <MapLibreGL.PointAnnotation
          id="destination"
          coordinate={[destination.longitude, destination.latitude]}
        >
          <View style={styles.destinationMarker}>
            <Text style={styles.destinationMarkerText}>📍</Text>
          </View>
        </MapLibreGL.PointAnnotation>
        {currentLocation && (
          <MapLibreGL.PointAnnotation
            id="current"
            coordinate={[currentLocation.longitude, currentLocation.latitude]}
          >
            <View style={styles.currentLocationMarker}>
              <View style={styles.currentLocationDot} />
            </View>
          </MapLibreGL.PointAnnotation>
        )}
      </MapLibreGL.MapView>
      <View style={styles.overlay}>
        {isTracking && distance !== null && (
          <View style={styles.distanceBadge}>
            <Text style={styles.distanceText}>{formatDistance(distance)} away</Text>
          </View>
        )}
        <View style={styles.tapHint}>
          <Text style={styles.tapHintText}>Tap to open map</Text>
        </View>
      </View>
    </Pressable>
  );
}

const styles = StyleSheet.create({
  container: {
    borderRadius: 12,
    overflow: 'hidden',
    position: 'relative',
  },
  map: {
    flex: 1,
  },
  placeholder: {
    backgroundColor: colors.gray[100],
    justifyContent: 'center',
    alignItems: 'center',
    borderRadius: 12,
  },
  placeholderText: {
    color: colors.textSecondary,
    fontSize: 14,
  },
  destinationMarker: {
    width: 30,
    height: 30,
    justifyContent: 'center',
    alignItems: 'center',
  },
  destinationMarkerText: {
    fontSize: 20,
  },
  currentLocationMarker: {
    width: 16,
    height: 16,
    borderRadius: 8,
    backgroundColor: colors.primary,
    borderWidth: 3,
    borderColor: '#fff',
    shadowColor: '#000',
    shadowOffset: { width: 0, height: 1 },
    shadowOpacity: 0.3,
    shadowRadius: 2,
    elevation: 3,
  },
  currentLocationDot: {
    width: 8,
    height: 8,
    borderRadius: 4,
    backgroundColor: '#fff',
  },
  overlay: {
    position: 'absolute',
    bottom: 0,
    left: 0,
    right: 0,
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'flex-end',
    padding: spacing.sm,
  },
  distanceBadge: {
    backgroundColor: colors.primary,
    paddingHorizontal: spacing.sm,
    paddingVertical: spacing.xs,
    borderRadius: 4,
  },
  distanceText: {
    color: '#fff',
    fontSize: 12,
    fontWeight: '600',
  },
  tapHint: {
    backgroundColor: 'rgba(0, 0, 0, 0.5)',
    paddingHorizontal: spacing.sm,
    paddingVertical: spacing.xs,
    borderRadius: 4,
  },
  tapHintText: {
    color: '#fff',
    fontSize: 10,
  },
});
