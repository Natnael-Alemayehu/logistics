import React, { useMemo } from 'react';
import { View, Text, StyleSheet, Pressable } from 'react-native';
import { colors, spacing, BorderRadius } from '@/utils/theme';
import { useTrackingStore } from '@/store/trackingStore';
import { useLocation } from '@/hooks/useLocation';
import { calculateDistance, formatDistance } from '@/services/maps';
import {
  Battery,
  Navigation,
  Pause,
  Play,
  Radio,
  Zap,
} from 'lucide-react-native';

interface TrackingOverlayProps {
  destinationCoordinate?: {
    latitude: number;
    longitude: number;
  };
  onNavigatePress?: () => void;
  onToggleTracking?: () => void;
  estimatedSpeedKph?: number;
}

export function TrackingOverlay({
  destinationCoordinate,
  onNavigatePress,
  onToggleTracking,
  estimatedSpeedKph = 30,
}: TrackingOverlayProps) {
  const {
    isTracking,
    batteryLevel,
    isLowBattery,
    isStationary,
    totalDistanceMeters,
    trackingStartTime,
    lastPosition,
  } = useTrackingStore();
  
  const { lastKnownLocation } = useLocation();
  
  const currentLocation = lastKnownLocation || lastPosition;
  
  const distanceToDestination = useMemo(() => {
    if (!currentLocation || !destinationCoordinate) return undefined;
    return calculateDistance(currentLocation, destinationCoordinate);
  }, [currentLocation, destinationCoordinate]);
  
  const etaSeconds = useMemo(() => {
    if (!distanceToDestination) return undefined;
    const speedMps = (estimatedSpeedKph * 1000) / 3600;
    return distanceToDestination / speedMps;
  }, [distanceToDestination, estimatedSpeedKph]);
  
  const formatETA = (seconds: number): string => {
    if (seconds < 60) return '<1 min';
    if (seconds < 3600) {
      const minutes = Math.round(seconds / 60);
      return `${minutes} min`;
    }
    const hours = Math.floor(seconds / 3600);
    const minutes = Math.round((seconds % 3600) / 60);
    return `${hours}h ${minutes}m`;
  };
  
  const formatDistanceTraveled = (meters: number): string => {
    if (meters < 1000) return `${Math.round(meters)}m`;
    return `${(meters / 1000).toFixed(1)} km`;
  };
  
  const trackingDuration = useMemo(() => {
    if (!trackingStartTime) return '0:00';
    const diffMs = Date.now() - new Date(trackingStartTime).getTime();
    const diffMins = Math.floor(diffMs / 60000);
    const hours = Math.floor(diffMins / 60);
    const mins = diffMins % 60;
    if (hours > 0) {
      return `${hours}:${mins.toString().padStart(2, '0')}`;
    }
    return `${mins}:00`;
  }, [trackingStartTime]);
  
  const getTrackingStatusText = (): string => {
    if (!isTracking) return 'Not tracking';
    if (isStationary) return 'Paused (stationary)';
    return 'Active';
  };
  
  const getTrackingStatusColor = (): string => {
    if (!isTracking) return colors.gray[500];
    if (isStationary) return colors.warning;
    return colors.success;
  };
  
  const getBatteryIcon = () => {
    if (batteryLevel > 80) return <Battery size={16} color={colors.success} />;
    if (batteryLevel > 50) return <Battery size={16} color={colors.primary} />;
    if (batteryLevel > 20) return <Battery size={16} color={colors.warning} />;
    return <Zap size={16} color={colors.error} />;
  };
  
  return (
    <View style={styles.container}>
      <View style={styles.topRow}>
        <View style={styles.statusContainer}>
          <View style={[styles.statusDot, { backgroundColor: getTrackingStatusColor() }]} />
          <Text style={styles.statusText}>{getTrackingStatusText()}</Text>
          {isTracking && (
            <Text style={styles.durationText}>{trackingDuration}</Text>
          )}
        </View>
        
        <View style={styles.batteryContainer}>
          {getBatteryIcon()}
          <Text style={[styles.batteryText, isLowBattery && styles.lowBatteryText]}>
            {Math.round(batteryLevel)}%
          </Text>
        </View>
      </View>
      
      <View style={styles.infoRow}>
        {distanceToDestination !== undefined && (
          <View style={styles.infoItem}>
            <Navigation size={14} color={colors.textSecondary} />
            <Text style={styles.infoLabel}>Distance</Text>
            <Text style={styles.infoValue}>{formatDistance(distanceToDestination)}</Text>
          </View>
        )}
        
        {etaSeconds !== undefined && (
          <View style={styles.infoItem}>
            <Radio size={14} color={colors.textSecondary} />
            <Text style={styles.infoLabel}>ETA</Text>
            <Text style={styles.infoValue}>{formatETA(etaSeconds)}</Text>
          </View>
        )}
        
        {totalDistanceMeters > 0 && (
          <View style={styles.infoItem}>
            <Text style={styles.infoLabel}>Traveled</Text>
            <Text style={styles.infoValue}>{formatDistanceTraveled(totalDistanceMeters)}</Text>
          </View>
        )}
      </View>
      
      <View style={styles.actionsRow}>
        <Pressable
          style={[styles.actionButton, isTracking ? styles.pauseButton : styles.playButton]}
          onPress={onToggleTracking}
        >
          {isTracking ? (
            <Pause size={20} color={colors.surface} />
          ) : (
            <Play size={20} color={colors.surface} />
          )}
          <Text style={styles.actionButtonText}>
            {isTracking ? 'Pause' : 'Start'}
          </Text>
        </Pressable>
        
        {destinationCoordinate && (
          <Pressable
            style={[styles.actionButton, styles.navigateButton]}
            onPress={onNavigatePress}
          >
            <Navigation size={20} color={colors.surface} />
            <Text style={styles.actionButtonText}>Navigate</Text>
          </Pressable>
        )}
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    backgroundColor: colors.surface,
    borderRadius: BorderRadius.lg,
    padding: spacing.md,
    shadowColor: '#000',
    shadowOffset: { width: 0, height: 2 },
    shadowOpacity: 0.1,
    shadowRadius: 4,
    elevation: 4,
    marginHorizontal: spacing.md,
    marginBottom: spacing.md,
  },
  topRow: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
    marginBottom: spacing.sm,
  },
  statusContainer: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.xs,
  },
  statusDot: {
    width: 8,
    height: 8,
    borderRadius: 4,
  },
  statusText: {
    fontSize: 14,
    fontWeight: '600',
    color: colors.text,
  },
  durationText: {
    fontSize: 12,
    color: colors.textSecondary,
    marginLeft: spacing.xs,
  },
  batteryContainer: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.xs,
    paddingHorizontal: spacing.sm,
    paddingVertical: 4,
    backgroundColor: colors.gray[100],
    borderRadius: BorderRadius.sm,
  },
  batteryText: {
    fontSize: 12,
    fontWeight: '500',
    color: colors.textSecondary,
  },
  lowBatteryText: {
    color: colors.error,
  },
  infoRow: {
    flexDirection: 'row',
    justifyContent: 'space-around',
    paddingVertical: spacing.sm,
    borderTopWidth: 1,
    borderBottomWidth: 1,
    borderColor: colors.border,
    marginBottom: spacing.sm,
  },
  infoItem: {
    alignItems: 'center',
    flexDirection: 'row',
    gap: 4,
  },
  infoLabel: {
    fontSize: 11,
    color: colors.textSecondary,
  },
  infoValue: {
    fontSize: 14,
    fontWeight: '600',
    color: colors.text,
    marginLeft: 4,
  },
  actionsRow: {
    flexDirection: 'row',
    gap: spacing.sm,
  },
  actionButton: {
    flex: 1,
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'center',
    gap: spacing.xs,
    paddingVertical: spacing.md,
    borderRadius: BorderRadius.md,
  },
  playButton: {
    backgroundColor: colors.success,
  },
  pauseButton: {
    backgroundColor: colors.warning,
  },
  navigateButton: {
    backgroundColor: colors.primary,
  },
  actionButtonText: {
    color: colors.surface,
    fontWeight: '600',
    fontSize: 14,
  },
});
