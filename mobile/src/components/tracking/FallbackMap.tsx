import React from 'react';
import { View, Text, StyleSheet, Pressable, Linking } from 'react-native';
import { colors, spacing, BorderRadius } from '@/utils/theme';
import type { Coordinates } from '@/types/maps';

interface FallbackMapProps {
  center?: Coordinates;
  shipments?: Array<{
    id: string;
    tracking_number: string;
    destination_lat?: number;
    destination_lng?: number;
    destination_address?: string;
  }>;
  currentLocation?: Coordinates;
  onNavigate?: (lat: number, lng: number) => void;
}

export function FallbackMap({ 
  center, 
  shipments = [], 
  currentLocation,
  onNavigate 
}: FallbackMapProps) {
  const handleOpenInMaps = () => {
    if (currentLocation) {
      Linking.openURL(
        `https://www.google.com/maps?q=${currentLocation.latitude},${currentLocation.longitude}`
      );
    } else if (center) {
      Linking.openURL(
        `https://www.google.com/maps?q=${center.latitude},${center.longitude}`
      );
    }
  };

  return (
    <View style={styles.container}>
      <View style={styles.placeholder}>
        <Text style={styles.icon}>🗺️</Text>
        <Text style={styles.title}>Map View</Text>
        <Text style={styles.message}>
          Interactive map requires a development build.
        </Text>
        <Text style={styles.subtext}>
          You're currently running in Expo Go, which doesn't support 
          the native map library.
        </Text>
        
        <Pressable style={styles.buildButton} onPress={handleOpenInMaps}>
          <Text style={styles.buildButtonText}>Open in Google Maps</Text>
        </Pressable>

        {shipments.length > 0 && (
          <View style={styles.shipmentsList}>
            <Text style={styles.listTitle}>Active Deliveries:</Text>
            {shipments.map((shipment) => (
              <View key={shipment.id} style={styles.shipmentItem}>
                <Text style={styles.trackingNumber}>
                  {shipment.tracking_number}
                </Text>
                {shipment.destination_address && (
                  <Text style={styles.address}>
                    {shipment.destination_address}
                  </Text>
                )}
                {shipment.destination_lat && shipment.destination_lng && (
                  <Pressable 
                    style={styles.navigateButton}
                    onPress={() => onNavigate?.(shipment.destination_lat!, shipment.destination_lng!)}
                  >
                    <Text style={styles.navigateButtonText}>Navigate</Text>
                  </Pressable>
                )}
              </View>
            ))}
          </View>
        )}
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: colors.gray[100],
  },
  placeholder: {
    flex: 1,
    justifyContent: 'center',
    alignItems: 'center',
    padding: spacing.xl,
  },
  icon: {
    fontSize: 64,
    marginBottom: spacing.md,
  },
  title: {
    fontSize: 24,
    fontWeight: '700',
    color: colors.text,
    marginBottom: spacing.sm,
  },
  message: {
    fontSize: 16,
    color: colors.textSecondary,
    textAlign: 'center',
    marginBottom: spacing.xs,
  },
  subtext: {
    fontSize: 14,
    color: colors.gray[500],
    textAlign: 'center',
    marginBottom: spacing.lg,
  },
  buildButton: {
    backgroundColor: colors.primary,
    paddingHorizontal: spacing.lg,
    paddingVertical: spacing.md,
    borderRadius: BorderRadius.md,
    marginBottom: spacing.xl,
  },
  buildButtonText: {
    color: '#fff',
    fontSize: 16,
    fontWeight: '600',
  },
  shipmentsList: {
    width: '100%',
    marginTop: spacing.md,
  },
  listTitle: {
    fontSize: 14,
    fontWeight: '600',
    color: colors.textSecondary,
    marginBottom: spacing.sm,
  },
  shipmentItem: {
    backgroundColor: colors.surface,
    padding: spacing.md,
    borderRadius: BorderRadius.md,
    marginBottom: spacing.sm,
  },
  trackingNumber: {
    fontSize: 16,
    fontWeight: '600',
    color: colors.text,
    marginBottom: 2,
  },
  address: {
    fontSize: 14,
    color: colors.textSecondary,
    marginBottom: spacing.sm,
  },
  navigateButton: {
    backgroundColor: colors.primary,
    paddingHorizontal: spacing.md,
    paddingVertical: spacing.sm,
    borderRadius: BorderRadius.sm,
    alignSelf: 'flex-start',
  },
  navigateButtonText: {
    color: '#fff',
    fontSize: 14,
    fontWeight: '600',
  },
});
