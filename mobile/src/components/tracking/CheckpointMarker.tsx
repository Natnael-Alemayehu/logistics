import React, { memo } from 'react';
import { View, Text, StyleSheet } from 'react-native';
import { MARKER_COLORS, type CheckpointMarkerData } from '@/types/maps';
import { colors, spacing } from '@/utils/theme';
import { formatDateTime } from '@/utils/format';
import { isMapAvailable, getMapLibre } from '@/services/maps/native';

type CheckpointType = 'pickup' | 'dropoff' | 'waypoint' | 'custom';

interface CheckpointMarkerProps {
  id: string;
  coordinate: {
    latitude: number;
    longitude: number;
  };
  checkpointType: CheckpointType;
  name?: string;
  timestamp?: string;
  passed?: boolean;
  sequence?: number;
  isSelected?: boolean;
  onPress?: () => void;
}

function getCheckpointIcon(type: CheckpointType): string {
  switch (type) {
    case 'pickup':
      return '📦';
    case 'dropoff':
      return '📥';
    case 'waypoint':
      return '🔷';
    case 'custom':
    default:
      return '⬡';
  }
}

function getCheckpointColor(type: CheckpointType, passed: boolean): string {
  if (passed) return MARKER_COLORS.passed;
  
  switch (type) {
    case 'pickup':
      return MARKER_COLORS.origin;
    case 'dropoff':
      return MARKER_COLORS.destination;
    case 'waypoint':
    case 'custom':
    default:
      return MARKER_COLORS.checkpoint;
  }
}

function CheckpointMarkerComponent({
  id,
  coordinate,
  checkpointType,
  name,
  timestamp,
  passed = false,
  sequence,
  isSelected = false,
  onPress,
}: CheckpointMarkerProps) {
  const MapLibreGL = getMapLibre();
  
  // If MapLibreGL isn't available, return null
  if (!MapLibreGL || !isMapAvailable()) {
    return null;
  }
  
  const icon = getCheckpointIcon(checkpointType);
  const color = getCheckpointColor(checkpointType, passed);
  
  return (
    <MapLibreGL.PointAnnotation
      id={id}
      coordinate={[coordinate.longitude, coordinate.latitude]}
      onSelected={onPress}
      anchor={{ x: 0.5, y: 0.5 }}
    >
      <View style={styles.container}>
        <View
          style={[
            styles.marker,
            { backgroundColor: color },
            passed && styles.passedMarker,
            isSelected && styles.selectedMarker,
          ]}
        >
          <Text style={styles.icon}>{icon}</Text>
          {sequence !== undefined && (
            <View style={styles.sequenceBadge}>
              <Text style={styles.sequenceText}>{sequence}</Text>
            </View>
          )}
        </View>
        
        {passed && timestamp && (
          <View style={styles.passedIndicator}>
            <Text style={styles.checkmark}>✓</Text>
          </View>
        )}
      </View>
    </MapLibreGL.PointAnnotation>
  );
}

interface CheckpointMarkerCalloutProps {
  name?: string;
  checkpointType: CheckpointType;
  timestamp?: string;
  passed: boolean;
  onPress?: () => void;
}

export function CheckpointMarkerCallout({
  name,
  checkpointType,
  timestamp,
  passed,
}: CheckpointMarkerCalloutProps) {
  const color = getCheckpointColor(checkpointType, passed);
  const typeLabel = checkpointType.charAt(0).toUpperCase() + checkpointType.slice(1);
  
  return (
    <View style={styles.calloutContainer}>
      <View style={styles.calloutHeader}>
        <Text style={styles.calloutType}>{typeLabel}</Text>
        <View style={[styles.statusDot, { backgroundColor: color }]} />
      </View>
      {name && <Text style={styles.calloutName}>{name}</Text>}
      {timestamp && (
        <Text style={styles.calloutTime}>
          {passed ? 'Passed' : 'Expected'}: {formatDateTime(timestamp)}
        </Text>
      )}
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    alignItems: 'center',
    justifyContent: 'center',
  },
  marker: {
    width: 28,
    height: 28,
    borderRadius: 14,
    alignItems: 'center',
    justifyContent: 'center',
    borderWidth: 2,
    borderColor: '#fff',
    shadowColor: '#000',
    shadowOffset: { width: 0, height: 1 },
    shadowOpacity: 0.2,
    shadowRadius: 2,
    elevation: 3,
  },
  passedMarker: {
    opacity: 0.7,
  },
  selectedMarker: {
    borderWidth: 3,
    borderColor: colors.primary,
  },
  icon: {
    fontSize: 12,
  },
  sequenceBadge: {
    position: 'absolute',
    top: -6,
    right: -6,
    backgroundColor: colors.gray[800],
    width: 16,
    height: 16,
    borderRadius: 8,
    alignItems: 'center',
    justifyContent: 'center',
    borderWidth: 1,
    borderColor: '#fff',
  },
  sequenceText: {
    color: '#fff',
    fontSize: 9,
    fontWeight: '700',
  },
  passedIndicator: {
    position: 'absolute',
    bottom: -4,
    right: -4,
    backgroundColor: colors.success,
    width: 14,
    height: 14,
    borderRadius: 7,
    alignItems: 'center',
    justifyContent: 'center',
    borderWidth: 1,
    borderColor: '#fff',
  },
  checkmark: {
    color: '#fff',
    fontSize: 9,
    fontWeight: '700',
  },
  calloutContainer: {
    backgroundColor: colors.surface,
    padding: spacing.md,
    borderRadius: 8,
    minWidth: 120,
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
  calloutType: {
    fontSize: 14,
    fontWeight: '600',
    color: colors.text,
  },
  statusDot: {
    width: 8,
    height: 8,
    borderRadius: 4,
  },
  calloutName: {
    fontSize: 12,
    color: colors.textSecondary,
    marginBottom: 2,
  },
  calloutTime: {
    fontSize: 11,
    color: colors.gray[500],
  },
});

export const CheckpointMarker = memo(CheckpointMarkerComponent);
