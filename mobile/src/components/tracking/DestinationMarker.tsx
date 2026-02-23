import React, { memo, useMemo } from 'react';
import { View, Text, StyleSheet, Pressable } from 'react-native';
import MapLibreGL from '@maplibre/maplibre-react-native';
import { STATUS_COLORS, MARKER_COLORS, type Coordinates } from '@/types/maps';
import type { ShipmentStatus } from '@/types/shipment';
import { colors, spacing } from '@/utils/theme';
import { calculateDistance, formatDistance } from '@/services/maps';

interface DestinationMarkerProps {
  id: string;
  coordinate: {
    latitude: number;
    longitude: number;
  };
  shipmentId: string;
  trackingNumber: string;
  status: ShipmentStatus;
  customerName: string;
  distance?: number;
  sequenceNumber?: number;
  isSelected?: boolean;
  onPress?: () => void;
  currentLocation?: {
    latitude: number;
    longitude: number;
  };
}

function getStatusColor(status: ShipmentStatus): string {
  return STATUS_COLORS[status] || MARKER_COLORS.destination;
}

function DestinationMarkerComponent({
  id,
  coordinate,
  shipmentId,
  trackingNumber,
  status,
  customerName,
  distance,
  sequenceNumber,
  isSelected = false,
  onPress,
  currentLocation,
}: DestinationMarkerProps) {
  const statusColor = getStatusColor(status);
  
  const calculatedDistance = useMemo(() => {
    if (distance !== undefined) return distance;
    if (currentLocation) {
      return calculateDistance(currentLocation, coordinate);
    }
    return undefined;
  }, [distance, currentLocation, coordinate]);

  return (
    <MapLibreGL.PointAnnotation
      id={id}
      coordinate={[coordinate.longitude, coordinate.latitude]}
      onSelected={onPress}
      anchor={{ x: 0.5, y: 1 }}
    >
      <View style={styles.container}>
        <View style={[styles.markerPin, { backgroundColor: statusColor }]}>
          {sequenceNumber !== undefined ? (
            <Text style={styles.sequenceNumber}>{sequenceNumber}</Text>
          ) : (
            <Text style={styles.pinIcon}>📍</Text>
          )}
        </View>
        
        {isSelected && (
          <View style={[styles.selectedIndicator, { borderColor: statusColor }]} />
        )}
        
        {calculatedDistance !== undefined && (
          <View style={styles.distanceLabel}>
            <Text style={styles.distanceText}>
              {formatDistance(calculatedDistance)}
            </Text>
          </View>
        )}
      </View>
    </MapLibreGL.PointAnnotation>
  );
}

interface DestinationMarkerCalloutProps {
  trackingNumber: string;
  customerName: string;
  status: ShipmentStatus;
  distance?: number;
  onPress?: () => void;
}

export function DestinationMarkerCallout({
  trackingNumber,
  customerName,
  status,
  distance,
  onPress,
}: DestinationMarkerCalloutProps) {
  const statusColor = getStatusColor(status);
  
  return (
    <Pressable onPress={onPress} style={styles.calloutContainer}>
      <View style={styles.calloutHeader}>
        <Text style={styles.calloutTracking}>{trackingNumber}</Text>
        <View style={[styles.statusDot, { backgroundColor: statusColor }]} />
      </View>
      <Text style={styles.calloutCustomer}>{customerName}</Text>
      {distance !== undefined && (
        <Text style={styles.calloutDistance}>{formatDistance(distance)} away</Text>
      )}
    </Pressable>
  );
}

const styles = StyleSheet.create({
  container: {
    alignItems: 'center',
  },
  markerPin: {
    width: 32,
    height: 32,
    borderRadius: 16,
    alignItems: 'center',
    justifyContent: 'center',
    borderWidth: 2,
    borderColor: '#fff',
    shadowColor: '#000',
    shadowOffset: { width: 0, height: 2 },
    shadowOpacity: 0.25,
    shadowRadius: 4,
    elevation: 4,
  },
  sequenceNumber: {
    color: '#fff',
    fontWeight: '700',
    fontSize: 14,
  },
  pinIcon: {
    fontSize: 14,
  },
  selectedIndicator: {
    position: 'absolute',
    width: 40,
    height: 40,
    borderRadius: 20,
    borderWidth: 3,
    top: -4,
  },
  distanceLabel: {
    position: 'absolute',
    top: -24,
    backgroundColor: 'rgba(0, 0, 0, 0.75)',
    paddingHorizontal: spacing.sm,
    paddingVertical: 2,
    borderRadius: 4,
  },
  distanceText: {
    color: '#fff',
    fontSize: 10,
    fontWeight: '600',
  },
  calloutContainer: {
    backgroundColor: colors.surface,
    padding: spacing.md,
    borderRadius: 8,
    minWidth: 150,
    shadowColor: '#000',
    shadowOffset: { width: 0, height: 2 },
    shadowOpacity: 0.15,
    shadowRadius: 4,
    elevation: 4,
  },
  calloutHeader: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    marginBottom: 4,
  },
  calloutTracking: {
    fontSize: 14,
    fontWeight: '600',
    color: colors.text,
    flex: 1,
  },
  statusDot: {
    width: 8,
    height: 8,
    borderRadius: 4,
    marginLeft: spacing.sm,
  },
  calloutCustomer: {
    fontSize: 12,
    color: colors.textSecondary,
    marginBottom: 4,
  },
  calloutDistance: {
    fontSize: 11,
    color: colors.primary,
    fontWeight: '500',
  },
});

export const DestinationMarker = memo(DestinationMarkerComponent);
