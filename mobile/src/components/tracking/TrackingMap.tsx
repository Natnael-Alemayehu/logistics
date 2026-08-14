import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { View, StyleSheet, ActivityIndicator, Text } from 'react-native';
import type { StyleProp, ViewStyle } from 'react-native';
import {
  DEFAULT_MAP_CENTER,
  DEFAULT_ZOOM_LEVEL,
  MARKER_COLORS,
  type Coordinates,
  type ShipmentMapData,
  type MapMarker,
  type MapRegion,
  type BoundingBox,
} from '@/types/maps';
import type { Shipment } from '@/types/shipment';
import { useLocation } from '@/hooks/useLocation';
import {
  MAP_STYLE,
  DEFAULT_CENTER,
  DEFAULT_ZOOM,
  calculateDistance,
  getBoundingBox,
} from '@/services/maps';
import { colors, spacing } from '@/utils/theme';
import { FallbackMap } from './FallbackMap';
import { isMapAvailable, getMapLibre } from '@/services/maps/native';

interface TrackingMapProps {
  shipments?: Shipment[];
  activeShipmentId?: string;
  showCurrentLocation?: boolean;
  onMarkerPress?: (marker: MapMarker) => void;
  onMapPress?: (coordinate: Coordinates) => void;
  showRoute?: boolean;
  showAccuracyCircle?: boolean;
  followUser?: boolean;
  style?: StyleProp<ViewStyle>;
}

function shipmentToMapData(shipment: Shipment): ShipmentMapData {
  return {
    id: shipment.id,
    trackingNumber: shipment.tracking_number,
    status: shipment.status,
    origin: shipment.origin_lat && shipment.origin_lng
      ? { latitude: shipment.origin_lat, longitude: shipment.origin_lng }
      : undefined,
    destination: shipment.destination_lat && shipment.destination_lng
      ? { latitude: shipment.destination_lat, longitude: shipment.destination_lng }
      : undefined,
    customerName: shipment.customer_name,
    customerPhone: shipment.customer_phone,
    destinationAddress: shipment.destination_address,
  };
}

// Check if map is available
const mapAvailable = isMapAvailable();

function TrackingMapComponent({
  shipments = [],
  activeShipmentId,
  showCurrentLocation = true,
  onMarkerPress,
  onMapPress,
  showRoute = false,
  showAccuracyCircle = true,
  followUser = false,
  style,
}: TrackingMapProps) {
  const { lastKnownLocation, isTracking } = useLocation();
  
  const mapShipments = useMemo(() => 
    shipments.map(shipmentToMapData), [shipments]
  );
  
  const activeShipment = useMemo(() => 
    mapShipments.find(s => s.id === activeShipmentId), 
    [mapShipments, activeShipmentId]
  );
  
  const currentLocationForMarker = useMemo(() => {
    if (!lastKnownLocation) return undefined;
    return {
      latitude: lastKnownLocation.latitude,
      longitude: lastKnownLocation.longitude,
    };
  }, [lastKnownLocation]);

  // If map is not available, show fallback
  if (!mapAvailable) {
    return (
      <FallbackMap
        center={currentLocationForMarker || DEFAULT_MAP_CENTER}
        shipments={shipments}
        currentLocation={currentLocationForMarker}
        onNavigate={(lat, lng) => {
          const url = `https://www.google.com/maps/dir/?api=1&destination=${lat},${lng}`;
          const { Linking } = require('react-native');
          Linking.openURL(url);
        }}
      />
    );
  }

  // Only load the native map component if available
  const MapLibreGL = getMapLibre();
  if (!MapLibreGL) {
    return (
      <FallbackMap
        center={currentLocationForMarker || DEFAULT_MAP_CENTER}
        shipments={shipments}
        currentLocation={currentLocationForMarker}
      />
    );
  }

  return (
    <NativeTrackingMap
      shipments={shipments}
      activeShipmentId={activeShipmentId}
      showCurrentLocation={showCurrentLocation}
      onMarkerPress={onMarkerPress}
      onMapPress={onMapPress}
      showRoute={showRoute}
      showAccuracyCircle={showAccuracyCircle}
      followUser={followUser}
      style={style}
      MapLibreGL={MapLibreGL}
    />
  );
}

// Separate component for native map that only loads when MapLibreGL is available
interface NativeTrackingMapProps extends TrackingMapProps {
  MapLibreGL: NonNullable<ReturnType<typeof getMapLibre>>;
}

function NativeTrackingMap({
  shipments = [],
  activeShipmentId,
  showCurrentLocation = true,
  onMarkerPress,
  onMapPress,
  showRoute = false,
  showAccuracyCircle = true,
  followUser = false,
  style,
  MapLibreGL,
}: NativeTrackingMapProps) {
  const cameraRef = useRef<React.ComponentRef<typeof MapLibreGL.Camera>>(null);
  const mapRef = useRef<React.ComponentRef<typeof MapLibreGL.MapView>>(null);
  
  const [isMapLoaded, setIsMapLoaded] = useState(false);
  const [selectedMarkerId, setSelectedMarkerId] = useState<string | null>(null);
  
  const { lastKnownLocation, isTracking } = useLocation();
  
  const mapShipments = useMemo(() => 
    shipments.map(shipmentToMapData), [shipments]
  );
  
  const activeShipment = useMemo(() => 
    mapShipments.find(s => s.id === activeShipmentId), 
    [mapShipments, activeShipmentId]
  );
  
  const destinationMarkers = useMemo(() => {
    return mapShipments
      .filter(s => s.destination)
      .map((s, index) => ({
        id: `dest-${s.id}`,
        coordinate: s.destination!,
        shipmentId: s.id,
        trackingNumber: s.trackingNumber,
        status: s.status,
        customerName: s.customerName,
        isActive: s.id === activeShipmentId,
        sequenceNumber: index + 1,
      }));
  }, [mapShipments, activeShipmentId]);
  
  const checkpointMarkers = useMemo(() => {
    const markers: Array<{
      id: string;
      coordinate: Coordinates;
      checkpointType: 'pickup' | 'dropoff' | 'waypoint' | 'custom';
      passed: boolean;
      sequence?: number;
      name?: string;
    }> = [];
    
    mapShipments.forEach(shipment => {
      if (shipment.checkpoints) {
        shipment.checkpoints.forEach(cp => {
          markers.push({
            id: `cp-${cp.id}`,
            coordinate: cp.coordinate,
            checkpointType: cp.type,
            passed: cp.passed,
            sequence: cp.sequence,
            name: cp.name,
          });
        });
      }
    });
    
    return markers;
  }, [mapShipments]);
  
  const initialCenter = useMemo(() => {
    if (followUser && lastKnownLocation) {
      return [lastKnownLocation.longitude, lastKnownLocation.latitude];
    }
    if (activeShipment?.destination) {
      return [activeShipment.destination.longitude, activeShipment.destination.latitude];
    }
    return DEFAULT_CENTER;
  }, [followUser, lastKnownLocation, activeShipment]);
  
  const fitToMarkers = useCallback(() => {
    if (!cameraRef.current) return;
    
    const coordinates: Coordinates[] = [];
    
    if (showCurrentLocation && lastKnownLocation) {
      coordinates.push({
        latitude: lastKnownLocation.latitude,
        longitude: lastKnownLocation.longitude,
      });
    }
    
    destinationMarkers.forEach(m => coordinates.push(m.coordinate));
    checkpointMarkers.forEach(m => coordinates.push(m.coordinate));
    
    if (coordinates.length > 1) {
      const bounds = getBoundingBox(coordinates);
      cameraRef.current.fitBounds(
        [bounds.ne.longitude, bounds.ne.latitude],
        [bounds.sw.longitude, bounds.sw.latitude],
        [50, 50, 50, 50],
        1000
      );
    } else if (coordinates.length === 1) {
      cameraRef.current.flyTo(
        [coordinates[0].longitude, coordinates[0].latitude],
        1000
      );
    }
  }, [showCurrentLocation, lastKnownLocation, destinationMarkers, checkpointMarkers]);
  
  useEffect(() => {
    if (followUser && lastKnownLocation && cameraRef.current) {
      cameraRef.current.flyTo(
        [lastKnownLocation.longitude, lastKnownLocation.latitude],
        500
      );
    }
  }, [followUser, lastKnownLocation]);
  
  const handleMarkerPress = useCallback((marker: MapMarker) => {
    setSelectedMarkerId(marker.id);
    onMarkerPress?.(marker);
  }, [onMarkerPress]);
  
  const handleMapPress = useCallback((feature: GeoJSON.Feature) => {
    if (feature.geometry && feature.geometry.type === 'Point' && onMapPress) {
      const coords = feature.geometry.coordinates as [number, number];
      const coordinate: Coordinates = {
        latitude: coords[1],
        longitude: coords[0],
      };
      onMapPress(coordinate);
    }
    setSelectedMarkerId(null);
  }, [onMapPress]);
  
  const handleDestinationPress = useCallback((marker: typeof destinationMarkers[0]) => {
    handleMarkerPress({
      id: marker.id,
      coordinate: marker.coordinate,
      type: 'destination',
      title: marker.trackingNumber,
      description: marker.customerName,
    });
  }, [handleMarkerPress]);
  
  const handleCheckpointPress = useCallback((marker: typeof checkpointMarkers[0]) => {
    handleMarkerPress({
      id: marker.id,
      coordinate: marker.coordinate,
      type: 'checkpoint',
      title: marker.name || 'Checkpoint',
    });
  }, [handleMarkerPress]);
  
  const currentLocationForMarker = useMemo(() => {
    if (!lastKnownLocation) return undefined;
    return {
      latitude: lastKnownLocation.latitude,
      longitude: lastKnownLocation.longitude,
    };
  }, [lastKnownLocation]);
  
  if (!isMapLoaded) {
    return (
      <View style={[styles.loadingContainer, style]}>
        <ActivityIndicator size="large" color={colors.primary} />
        <Text style={styles.loadingText}>Loading map...</Text>
      </View>
    );
  }
  
  return (
    <View style={[styles.container, style]}>
      <MapLibreGL.MapView
        ref={mapRef}
        style={styles.map}
        mapStyle={MAP_STYLE}
        onDidFinishLoadingMap={() => setIsMapLoaded(true)}
        onPress={handleMapPress}
        logoEnabled={false}
        attributionEnabled={false}
        compassEnabled={true}
      >
        <MapLibreGL.Camera
          ref={cameraRef}
          zoomLevel={DEFAULT_ZOOM}
          centerCoordinate={initialCenter as [number, number]}
          animationMode="flyTo"
          animationDuration={1000}
        />
        
        {showCurrentLocation && currentLocationForMarker && (
          <MapLibreGL.PointAnnotation
            id="current-location"
            coordinate={[currentLocationForMarker.longitude, currentLocationForMarker.latitude]}
          >
            <View style={styles.currentLocationMarker}>
              <View style={[
                styles.currentLocationDot,
                isTracking && styles.currentLocationDotActive
              ]} />
              {showAccuracyCircle && lastKnownLocation?.accuracy && (
                <View style={[
                  styles.accuracyCircle,
                  { 
                    width: lastKnownLocation.accuracy * 2,
                    height: lastKnownLocation.accuracy * 2,
                  }
                ]} />
              )}
            </View>
          </MapLibreGL.PointAnnotation>
        )}
        
        {destinationMarkers.map(marker => (
          <MapLibreGL.PointAnnotation
            key={marker.id}
            id={marker.id}
            coordinate={[marker.coordinate.longitude, marker.coordinate.latitude]}
            onSelected={() => handleDestinationPress(marker)}
          >
            <View style={[
              styles.destinationMarker,
              marker.isActive && styles.destinationMarkerActive
            ]}>
              <Text style={styles.markerText}>{marker.sequenceNumber}</Text>
            </View>
          </MapLibreGL.PointAnnotation>
        ))}
        
        {checkpointMarkers.map(marker => (
          <MapLibreGL.PointAnnotation
            key={marker.id}
            id={marker.id}
            coordinate={[marker.coordinate.longitude, marker.coordinate.latitude]}
            onSelected={() => handleCheckpointPress(marker)}
          >
            <View style={[
              styles.checkpointMarker,
              marker.passed && styles.checkpointMarkerPassed
            ]}>
              <Text style={styles.checkpointText}>
                {marker.checkpointType === 'pickup' ? 'P' : 
                 marker.checkpointType === 'dropoff' ? 'D' : 'W'}
              </Text>
            </View>
          </MapLibreGL.PointAnnotation>
        ))}
        
        {showRoute && activeShipment?.origin && activeShipment?.destination && (
          <MapLibreGL.ShapeSource
            id="route-source"
            shape={{
              type: 'FeatureCollection',
              features: [
                {
                  type: 'Feature',
                  properties: {},
                  geometry: {
                    type: 'LineString',
                    coordinates: [
                      [activeShipment.origin.longitude, activeShipment.origin.latitude],
                      [activeShipment.destination.longitude, activeShipment.destination.latitude],
                    ],
                  },
                },
              ],
            }}
          >
            <MapLibreGL.LineLayer
              id="route-line"
              style={{
                lineColor: MARKER_COLORS.current,
                lineWidth: 3,
                lineOpacity: 0.8,
                lineDasharray: [2, 2],
              }}
            />
          </MapLibreGL.ShapeSource>
        )}
      </MapLibreGL.MapView>
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
  },
  map: {
    flex: 1,
  },
  loadingContainer: {
    flex: 1,
    justifyContent: 'center',
    alignItems: 'center',
    backgroundColor: colors.background,
  },
  loadingText: {
    marginTop: spacing.md,
    fontSize: 14,
    color: colors.textSecondary,
  },
  currentLocationMarker: {
    alignItems: 'center',
    justifyContent: 'center',
  },
  currentLocationDot: {
    width: 16,
    height: 16,
    borderRadius: 8,
    backgroundColor: colors.primary,
    borderWidth: 3,
    borderColor: '#fff',
  },
  currentLocationDotActive: {
    backgroundColor: '#22c55e',
  },
  accuracyCircle: {
    position: 'absolute',
    borderRadius: 100,
    backgroundColor: 'rgba(37, 99, 235, 0.15)',
    borderWidth: 1,
    borderColor: 'rgba(37, 99, 235, 0.3)',
  },
  destinationMarker: {
    width: 32,
    height: 32,
    borderRadius: 16,
    backgroundColor: MARKER_COLORS.destination,
    alignItems: 'center',
    justifyContent: 'center',
    borderWidth: 2,
    borderColor: '#fff',
  },
  destinationMarkerActive: {
    backgroundColor: MARKER_COLORS.current,
    transform: [{ scale: 1.2 }],
  },
  markerText: {
    color: '#fff',
    fontWeight: '700',
    fontSize: 14,
  },
  checkpointMarker: {
    width: 24,
    height: 24,
    borderRadius: 12,
    backgroundColor: MARKER_COLORS.checkpoint,
    alignItems: 'center',
    justifyContent: 'center',
    borderWidth: 2,
    borderColor: '#fff',
  },
  checkpointMarkerPassed: {
    backgroundColor: MARKER_COLORS.passed,
  },
  checkpointText: {
    color: '#fff',
    fontWeight: '700',
    fontSize: 12,
  },
});

export const TrackingMap = React.memo(TrackingMapComponent);
