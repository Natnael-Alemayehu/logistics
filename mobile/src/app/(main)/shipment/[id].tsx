import { 
  View, 
  Text, 
  StyleSheet, 
  ScrollView, 
  Pressable, 
  Alert,
  Linking,
  ActivityIndicator,
} from 'react-native';
import { router, useLocalSearchParams, Stack } from 'expo-router';
import { useState, useEffect } from 'react';
import type { Shipment, ShipmentStatus } from '@/types/shipment';
import { useShipmentsStore } from '@store/shipmentsStore';
import { useNetworkStore } from '@store/networkStore';
import { api } from '@/services/api';
import { API_ENDPOINTS } from '@/services/constants';
import { getShipmentById as getLocalShipment, updateShipment as updateLocalShipment } from '@db';

const STATUS_COLORS: Record<ShipmentStatus, string> = {
  pending: '#fef3c7',
  assigned: '#dbeafe',
  in_transit: '#dbeafe',
  delayed: '#fee2e2',
  arrived: '#d1fae5',
  delivered: '#d1fae5',
  issue: '#fee2e2',
  cancelled: '#f3f4f6',
};

const STATUS_TEXT_COLORS: Record<ShipmentStatus, string> = {
  pending: '#92400e',
  assigned: '#1e40af',
  in_transit: '#1e40af',
  delayed: '#dc2626',
  arrived: '#059669',
  delivered: '#059669',
  issue: '#dc2626',
  cancelled: '#6b7280',
};

const DELAY_REASONS = [
  'Road conditions',
  'Weather',
  'Security checkpoint',
  'Mechanical issue',
  'Traffic',
  'Other',
];

function formatStatus(status: ShipmentStatus): string {
  return status.replace('_', ' ').replace(/\b\w/g, (l) => l.toUpperCase());
}

export default function ShipmentDetailScreen() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const [shipment, setShipment] = useState<Shipment | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [updating, setUpdating] = useState(false);
  const isOnline = useNetworkStore((state) => state.isOnline);
  const updateShipment = useShipmentsStore((state) => state.updateShipment);

  useEffect(() => {
    loadShipment();
  }, [id]);

  const loadShipment = async () => {
    setIsLoading(true);
    try {
      if (isOnline) {
        const response = await api.get<Shipment>(API_ENDPOINTS.shipments.get(id!));
        setShipment(response);
      } else {
        throw new Error('Offline');
      }
    } catch (error) {
      // Fallback to local DB
      try {
        const local = await getLocalShipment(id!);
        if (local) {
          setShipment(local as unknown as Shipment);
        }
      } catch {
        console.error('Failed to load shipment:', error);
      }
    } finally {
      setIsLoading(false);
    }
  };

  const handleCallCustomer = () => {
    if (shipment?.customer_phone) {
      Linking.openURL(`tel:${shipment.customer_phone}`);
    }
  };

  const handleNavigate = () => {
    if (shipment?.destination_lat && shipment?.destination_lng) {
      const url = `https://www.google.com/maps/dir/?api=1&destination=${shipment.destination_lat},${shipment.destination_lng}`;
      Linking.openURL(url);
    }
  };

  const updateStatus = async (status: ShipmentStatus, reason?: string) => {
    if (!shipment) return;
    setUpdating(true);
    try {
      if (isOnline) {
        await api.patch(API_ENDPOINTS.shipments.updateStatus(shipment.id), {
          status,
          status_reason: reason,
        });
      }
      // Always update locally
      await updateLocalShipment(shipment.id, { status }).catch(() => {});
      updateShipment(shipment.id, { status, status_reason: reason });
      setShipment({ ...shipment, status, status_reason: reason });
      Alert.alert('Success', `Status updated to ${formatStatus(status)}`);
    } catch (error) {
      Alert.alert('Error', 'Failed to update status');
    } finally {
      setUpdating(false);
    }
  };

  const handleDelay = () => {
    Alert.alert(
      'Mark Delayed',
      'Select a reason:',
      DELAY_REASONS.map((reason) => ({
        text: reason,
        onPress: () => updateStatus('delayed', reason),
      })),
      { cancelable: true }
    );
  };

  const handleCompleteDelivery = () => {
    if (shipment) {
      router.push(`/pod/${shipment.id}`);
    }
  };

  if (isLoading) {
    return (
      <View style={styles.loadingContainer}>
        <ActivityIndicator size="large" color="#2563eb" />
      </View>
    );
  }

  if (!shipment) {
    return (
      <View style={styles.loadingContainer}>
        <Text style={styles.errorText}>Shipment not found</Text>
      </View>
    );
  }

  const renderActionButtons = () => {
    const buttons: { label: string; onPress: () => void; color: string }[] = [];

    switch (shipment.status) {
      case 'assigned':
        buttons.push({
          label: 'Start Trip',
          onPress: () => updateStatus('in_transit'),
          color: '#2563eb',
        });
        break;
      case 'in_transit':
        buttons.push(
          {
            label: 'Mark Arrived',
            onPress: () => updateStatus('arrived'),
            color: '#059669',
          },
          {
            label: 'Mark Delayed',
            onPress: handleDelay,
            color: '#d97706',
          }
        );
        break;
      case 'delayed':
        buttons.push(
          {
            label: 'Resume Trip',
            onPress: () => updateStatus('in_transit'),
            color: '#2563eb',
          },
          {
            label: 'Mark Arrived',
            onPress: () => updateStatus('arrived'),
            color: '#059669',
          }
        );
        break;
      case 'arrived':
        buttons.push({
          label: 'Complete Delivery',
          onPress: handleCompleteDelivery,
          color: '#059669',
        });
        break;
    }

    if (buttons.length === 0) return null;

    return (
      <View style={styles.actions}>
        {buttons.map((btn, index) => (
          <Pressable
            key={index}
            style={[styles.actionButton, { backgroundColor: btn.color }]}
            onPress={btn.onPress}
            disabled={updating}
          >
            {updating ? (
              <ActivityIndicator color="#fff" />
            ) : (
              <Text style={styles.actionButtonText}>{btn.label}</Text>
            )}
          </Pressable>
        ))}
      </View>
    );
  };

  return (
    <>
      <Stack.Screen
        options={{
          title: shipment.tracking_number,
          headerStyle: { backgroundColor: '#2563eb' },
          headerTintColor: '#fff',
        }}
      />
      <ScrollView style={styles.container}>
        <View style={styles.header}>
          <View
            style={[
              styles.statusBadge,
              { backgroundColor: STATUS_COLORS[shipment.status] },
            ]}
          >
            <Text
              style={[
                styles.statusText,
                { color: STATUS_TEXT_COLORS[shipment.status] },
              ]}
            >
              {formatStatus(shipment.status)}
            </Text>
          </View>
        </View>

        <Pressable style={styles.card} onPress={handleCallCustomer}>
          <Text style={styles.cardTitle}>Customer</Text>
          <Text style={styles.customerName}>{shipment.customer_name}</Text>
          <Text style={styles.customerPhone}>📞 {shipment.customer_phone}</Text>
          <Text style={styles.tapHint}>Tap to call</Text>
        </Pressable>

        <View style={styles.card}>
          <Text style={styles.cardTitle}>Origin</Text>
          <Text style={styles.address}>{shipment.origin_address}</Text>
        </View>

        <View style={styles.card}>
          <Text style={styles.cardTitle}>Destination</Text>
          <Text style={styles.address}>{shipment.destination_address}</Text>
          {(shipment.destination_lat || shipment.destination_lng) && (
            <Pressable style={styles.navigateButton} onPress={handleNavigate}>
              <Text style={styles.navigateButtonText}>🗺️ Navigate</Text>
            </Pressable>
          )}
        </View>

        {shipment.cargo_description && (
          <View style={styles.card}>
            <Text style={styles.cardTitle}>Cargo Details</Text>
            <Text style={styles.cargoText}>{shipment.cargo_description}</Text>
            {shipment.cargo_weight && (
              <Text style={styles.cargoMeta}>Weight: {shipment.cargo_weight} kg</Text>
            )}
          </View>
        )}

        {shipment.special_instructions && (
          <View style={[styles.card, styles.specialCard]}>
            <Text style={styles.cardTitle}>⚠️ Special Instructions</Text>
            <Text style={styles.specialText}>{shipment.special_instructions}</Text>
          </View>
        )}

        <View style={styles.card}>
          <Text style={styles.cardTitle}>Map Preview</Text>
          <View style={styles.mapPlaceholder}>
            <Text style={styles.mapPlaceholderText}>
              📍 {shipment.destination_address}
            </Text>
            {(shipment.destination_lat || shipment.destination_lng) && (
              <Pressable style={styles.openMapButton} onPress={handleNavigate}>
                <Text style={styles.openMapButtonText}>Open in Maps</Text>
              </Pressable>
            )}
          </View>
        </View>

        <View style={styles.card}>
          <Text style={styles.cardTitle}>Status History</Text>
          <View style={styles.timeline}>
            <View style={styles.timelineItem}>
              <View style={styles.timelineDot} />
              <View style={styles.timelineContent}>
                <Text style={styles.timelineStatus}>
                  {formatStatus(shipment.status)}
                </Text>
                <Text style={styles.timelineTime}>Current</Text>
              </View>
            </View>
            {shipment.status_reason && (
              <Text style={styles.reasonText}>Reason: {shipment.status_reason}</Text>
            )}
          </View>
        </View>

        {renderActionButtons()}
      </ScrollView>
    </>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: '#f9fafb',
  },
  loadingContainer: {
    flex: 1,
    justifyContent: 'center',
    alignItems: 'center',
    backgroundColor: '#f9fafb',
  },
  errorText: {
    fontSize: 16,
    color: '#6b7280',
  },
  header: {
    padding: 16,
    alignItems: 'center',
  },
  statusBadge: {
    paddingHorizontal: 16,
    paddingVertical: 8,
    borderRadius: 20,
  },
  statusText: {
    fontSize: 14,
    fontWeight: '600',
  },
  card: {
    backgroundColor: '#fff',
    marginHorizontal: 16,
    marginBottom: 12,
    padding: 16,
    borderRadius: 12,
    shadowColor: '#000',
    shadowOffset: { width: 0, height: 1 },
    shadowOpacity: 0.05,
    shadowRadius: 2,
    elevation: 2,
  },
  cardTitle: {
    fontSize: 12,
    fontWeight: '600',
    color: '#6b7280',
    textTransform: 'uppercase',
    marginBottom: 8,
  },
  customerName: {
    fontSize: 18,
    fontWeight: '600',
    color: '#1f2937',
    marginBottom: 4,
  },
  customerPhone: {
    fontSize: 16,
    color: '#2563eb',
  },
  tapHint: {
    fontSize: 12,
    color: '#9ca3af',
    marginTop: 4,
  },
  address: {
    fontSize: 16,
    color: '#374151',
    lineHeight: 24,
  },
  navigateButton: {
    marginTop: 12,
    paddingVertical: 8,
    paddingHorizontal: 16,
    backgroundColor: '#eff6ff',
    borderRadius: 8,
    alignSelf: 'flex-start',
  },
  navigateButtonText: {
    color: '#2563eb',
    fontWeight: '500',
  },
  cargoText: {
    fontSize: 14,
    color: '#374151',
    marginBottom: 4,
  },
  cargoMeta: {
    fontSize: 12,
    color: '#6b7280',
  },
  specialCard: {
    backgroundColor: '#fffbeb',
    borderLeftWidth: 4,
    borderLeftColor: '#f59e0b',
  },
  specialText: {
    fontSize: 14,
    color: '#92400e',
    lineHeight: 20,
  },
  mapPlaceholder: {
    backgroundColor: '#f3f4f6',
    borderRadius: 8,
    padding: 24,
    alignItems: 'center',
  },
  mapPlaceholderText: {
    fontSize: 14,
    color: '#4b5563',
    textAlign: 'center',
    marginBottom: 12,
  },
  openMapButton: {
    backgroundColor: '#2563eb',
    paddingVertical: 8,
    paddingHorizontal: 16,
    borderRadius: 8,
  },
  openMapButtonText: {
    color: '#fff',
    fontWeight: '500',
  },
  timeline: {
    marginTop: 8,
  },
  timelineItem: {
    flexDirection: 'row',
    alignItems: 'flex-start',
  },
  timelineDot: {
    width: 12,
    height: 12,
    borderRadius: 6,
    backgroundColor: '#2563eb',
    marginRight: 12,
    marginTop: 4,
  },
  timelineContent: {
    flex: 1,
  },
  timelineStatus: {
    fontSize: 14,
    fontWeight: '500',
    color: '#1f2937',
  },
  timelineTime: {
    fontSize: 12,
    color: '#6b7280',
  },
  reasonText: {
    fontSize: 12,
    color: '#dc2626',
    marginTop: 8,
    marginLeft: 24,
  },
  actions: {
    padding: 16,
    gap: 12,
  },
  actionButton: {
    padding: 16,
    borderRadius: 12,
    alignItems: 'center',
  },
  actionButtonText: {
    color: '#fff',
    fontSize: 16,
    fontWeight: '600',
  },
});