import React from 'react';
import { View, Text, StyleSheet, Pressable } from 'react-native';
import { router } from 'expo-router';
import { colors, spacing } from '@/utils/theme';

type TrackingMode = 'active' | 'paused' | 'checkpoint-only';

interface TrackingStatusBannerProps {
  shipmentId: string;
  trackingNumber: string;
  trackingMode: TrackingMode;
  batteryLevel: number;
  isStationary?: boolean;
}

export function TrackingStatusBanner({
  shipmentId,
  trackingNumber,
  trackingMode,
  batteryLevel,
  isStationary = false,
}: TrackingStatusBannerProps) {
  const handlePress = () => {
    router.push(`/map?shipmentId=${shipmentId}`);
  };

  const getModeLabel = (): string => {
    switch (trackingMode) {
      case 'active':
        return isStationary ? 'Paused (Stationary)' : 'Active';
      case 'paused':
        return 'Paused';
      case 'checkpoint-only':
        return 'Checkpoint Mode';
      default:
        return 'Unknown';
    }
  };

  const getModeColor = (): string => {
    switch (trackingMode) {
      case 'active':
        return isStationary ? colors.warning : colors.success;
      case 'paused':
        return colors.warning;
      case 'checkpoint-only':
        return colors.info;
      default:
        return colors.gray[500];
    }
  };

  const getBatteryIcon = (): string => {
    if (batteryLevel >= 80) return '🔋';
    if (batteryLevel >= 50) return '🔋';
    if (batteryLevel >= 20) return '🪫';
    return '🪫';
  };

  const getBatteryColor = (): string => {
    if (batteryLevel >= 50) return colors.success;
    if (batteryLevel >= 20) return colors.warning;
    return colors.error;
  };

  return (
    <Pressable style={styles.container} onPress={handlePress}>
      <View style={styles.leftSection}>
        <View style={[styles.indicator, { backgroundColor: getModeColor() }]} />
        <View style={styles.textContainer}>
          <Text style={styles.title}>Tracking active for #{trackingNumber}</Text>
          <Text style={[styles.mode, { color: getModeColor() }]}>
            {getModeLabel()}
          </Text>
        </View>
      </View>
      <View style={styles.rightSection}>
        <View style={styles.batteryContainer}>
          <Text style={styles.batteryIcon}>{getBatteryIcon()}</Text>
          <Text style={[styles.batteryLevel, { color: getBatteryColor() }]}>
            {batteryLevel}%
          </Text>
        </View>
        <Text style={styles.tapHint}>View Map →</Text>
      </View>
    </Pressable>
  );
}

const styles = StyleSheet.create({
  container: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    backgroundColor: colors.surface,
    paddingHorizontal: spacing.md,
    paddingVertical: spacing.sm,
    borderBottomWidth: 1,
    borderBottomColor: colors.border,
  },
  leftSection: {
    flexDirection: 'row',
    alignItems: 'center',
    flex: 1,
  },
  indicator: {
    width: 8,
    height: 8,
    borderRadius: 4,
    marginRight: spacing.sm,
  },
  textContainer: {
    flex: 1,
  },
  title: {
    fontSize: 14,
    fontWeight: '600',
    color: colors.text,
  },
  mode: {
    fontSize: 12,
    fontWeight: '500',
    marginTop: 2,
  },
  rightSection: {
    alignItems: 'flex-end',
  },
  batteryContainer: {
    flexDirection: 'row',
    alignItems: 'center',
  },
  batteryIcon: {
    fontSize: 14,
    marginRight: 4,
  },
  batteryLevel: {
    fontSize: 12,
    fontWeight: '600',
  },
  tapHint: {
    fontSize: 10,
    color: colors.primary,
    marginTop: 2,
  },
});
