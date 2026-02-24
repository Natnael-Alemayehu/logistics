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
import { useState, useEffect, useCallback } from 'react';
import type { Shipment, ShipmentStatus } from '@/types/shipment';
import { useShipmentsStore } from '@store/shipmentsStore';
import { useNetworkStore } from '@store/networkStore';
import { useTrackingStore } from '@store/trackingStore';
import { useAuthStore } from '@store/authStore';
import { api } from '@/services/api';
import { API_ENDPOINTS } from '@/services/constants';
import { getShipmentById as getLocalShipment, updateShipment as updateLocalShipment } from '@db';
import { ShipmentMiniMap } from '@/components/shipments/ShipmentMiniMap';
import { TrackingStatusBanner } from '@/components/tracking/TrackingStatusBanner';
import { TrackingEventTimeline } from '@/components/tracking/TrackingEventTimeline';
import { startLocationTracking, stopLocationTracking } from '@/services/location';
import { insert as insertTrackingEvent } from '@/db/repositories/trackingEvents';
import { useTranslation } from 'react-i18next';

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

function calculateDistance(lat1: number, lon1: number, lat2: number, lon2: number): number {
  const R = 6371000;
  const dLat = toRadians(lat2 - lat1);
  const dLon = toRadians(lon2 - lon1);
  const a =
    Math.sin(dLat / 2) * Math.sin(dLat / 2) +
    Math.cos(toRadians(lat1)) * Math.cos(toRadians(lat2)) * Math.sin(dLon / 2) * Math.sin(dLon / 2);
  const c = 2 * Math.atan2(Math.sqrt(a), Math.sqrt(1 - a));
  return R * c;
}

function toRadians(degrees: number): number {
  return degrees * (Math.PI / 180);
}

function formatStatus(status: ShipmentStatus): string {
  return status.replace('_', ' ').replace(/\b\w/g, (l) => l.toUpperCase());
}

export default function ShipmentDetailScreen() {
  const { t } = useTranslation();
  const { id } = useLocalSearchParams<{ id: string }>();
  const [shipment, setShipment] = useState<Shipment | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [updating, setUpdating] = useState(false);
  const [distanceToDestination, setDistanceToDestination] = useState<number | null>(null);
  const isOnline = useNetworkStore((state) => state.isOnline);
  const updateShipment = useShipmentsStore((state) => state.updateShipment);
  const { 
    isTracking, 
    activeShipmentId, 
    batteryLevel,
    isStationary,
    trackingConfig,
    startTracking, 
    stopTracking 
  } = useTrackingStore();
  const user = useAuthStore((state) => state.user);
  
  const isTrackingThisShipment = isTracking && activeShipmentId === id;

  const DELAY_REASONS = [
    t('delayReasons.road_conditions'),
    t('delayReasons.weather'),
    t('delayReasons.security_checkpoint'),
    t('delayReasons.mechanical_issue'),
    t('delayReasons.traffic'),
    t('delayReasons.other'),
  ];

  useEffect(() => {
    loadShipment();
  }, [id]);

  useEffect(() => {
    if (isTrackingThisShipment && shipment?.destination_lat && shipment?.destination_lng) {
      const updateDistance = async () => {
        try {
          const location = await import('@/services/location').then(m => m.getCurrentLocation());
          if (location) {
            const dist = calculateDistance(
              location.latitude,
              location.longitude,
              shipment.destination_lat!,
              shipment.destination_lng!
            );
            setDistanceToDestination(dist);
          }
        } catch (error) {
          console.error('Failed to calculate distance:', error);
        }
      };
      updateDistance();
      const interval = setInterval(updateDistance, 10000);
      return () => clearInterval(interval);
    }
    return undefined;
  }, [isTrackingThisShipment, shipment?.destination_lat, shipment?.destination_lng]);

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

      // Handle tracking based on status
      if (status === 'in_transit' && !isTrackingThisShipment) {
        await handleStartTracking();
      } else if (status === 'delivered' && isTrackingThisShipment) {
        await handleStopTracking();
      }

      Alert.alert(t('common.success'), t('shipments.statusUpdated', { status: t(`status.${status}`) }));
    } catch (error) {
      Alert.alert(t('common.error'), t('shipments.statusUpdateError'));
    } finally {
      setUpdating(false);
    }
  };

  const handleStartTracking = async () => {
    if (!shipment || !user?.id) return;
    
    try {
      if (isTracking && activeShipmentId && activeShipmentId !== shipment.id) {
        Alert.alert(
          t('shipments.stopCurrentTracking'),
          t('shipments.stopCurrentTrackingMessage', { id: activeShipmentId }),
          [
            { text: t('common.cancel'), style: 'cancel' },
            {
              text: t('shipments.yesSwitch'),
              onPress: async () => {
                await handleStopTracking();
                await startTrackingForShipment();
              },
            },
          ]
        );
        return;
      }

      await startTrackingForShipment();
    } catch (error) {
      Alert.alert(t('common.error'), t('shipments.trackingStartError'));
    }
  };

  const startTrackingForShipment = async () => {
    if (!shipment || !user?.id) return;
    
    startTracking(shipment.id, user.id);
    await startLocationTracking(shipment.id, user.id);
    
    await insertTrackingEvent({
      shipment_id: shipment.id,
      driver_id: user.id,
      latitude: 0,
      longitude: 0,
      event_type: 'status_change',
      status: shipment.status,
      note: 'Tracking started',
      recorded_at: new Date().toISOString(),
    });
  };

  const handleStopTracking = async () => {
    try {
      await stopLocationTracking();
      stopTracking();
    } catch (error) {
      console.error('Failed to stop tracking:', error);
    }
  };

  const handleDelay = () => {
    Alert.alert(
      t('shipments.markDelayed'),
      t('delayReasons.title'),
      DELAY_REASONS.map((reason) => ({
        text: reason,
        onPress: () => updateStatus('delayed', reason),
      })),
      { cancelable: true }
    );
  };

  const handleCompleteDelivery = async () => {
    if (shipment) {
      if (isTrackingThisShipment) {
        await handleStopTracking();
      }
      router.push(`/pod/${shipment.id}`);
    }
  };

  const handleViewOnMap = () => {
    if (shipment) {
      router.push(`/map?shipmentId=${shipment.id}`);
    }
  };

  const getTrackingMode = (): 'active' | 'paused' | 'checkpoint-only' => {
    if (isStationary) return 'paused';
    if (trackingConfig?.batteryOptimized) return 'active';
    return 'active';
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
        <Text style={styles.errorText}>{t('shipments.notFound')}</Text>
      </View>
    );
  }

  const renderActionButtons = () => {
    const buttons: { label: string; onPress: () => void; color: string; disabled?: boolean }[] = [];

    switch (shipment.status) {
      case 'assigned':
        buttons.push({
          label: t('shipments.startTrip'),
          onPress: () => updateStatus('in_transit'),
          color: '#2563eb',
        });
        break;
      case 'in_transit':
        buttons.push(
          {
            label: t('shipments.markArrived'),
            onPress: () => updateStatus('arrived'),
            color: '#059669',
          },
          {
            label: t('shipments.markDelayed'),
            onPress: handleDelay,
            color: '#d97706',
          }
        );
        break;
      case 'delayed':
        buttons.push(
          {
            label: t('shipments.resumeTrip'),
            onPress: () => updateStatus('in_transit'),
            color: '#2563eb',
          },
          {
            label: t('shipments.markArrived'),
            onPress: () => updateStatus('arrived'),
            color: '#059669',
          }
        );
        break;
      case 'arrived':
        buttons.push({
          label: t('shipments.completeDelivery'),
          onPress: handleCompleteDelivery,
          color: '#059669',
        });
        break;
    }

    if (buttons.length === 0) return null;

    const isDisabled = isTracking && activeShipmentId && activeShipmentId !== shipment.id;

    return (
      <View style={styles.actions}>
        {isTrackingThisShipment && (
          <View style={styles.trackingActiveIndicator}>
            <View style={styles.trackingDot} />
            <Text style={styles.trackingActiveText}>{t('shipments.trackingActive')}</Text>
          </View>
        )}
        {isDisabled && (
          <View style={styles.trackingWarning}>
            <Text style={styles.trackingWarningText}>
              {t('shipments.stopTrackingWarning')}
            </Text>
          </View>
        )}
        {buttons.map((btn, index) => (
          <Pressable
            key={index}
            style={[
              styles.actionButton, 
              { backgroundColor: btn.color },
              (isDisabled || btn.disabled) && styles.actionButtonDisabled
            ]}
            onPress={btn.onPress}
            disabled={updating || isDisabled || btn.disabled}
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
        {isTrackingThisShipment && (
          <TrackingStatusBanner
            shipmentId={shipment.id}
            trackingNumber={shipment.tracking_number}
            trackingMode={getTrackingMode()}
            batteryLevel={batteryLevel}
            isStationary={isStationary}
          />
        )}
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
              {t(`status.${shipment.status}`)}
            </Text>
          </View>
          {isTrackingThisShipment && distanceToDestination !== null && (
            <Text style={styles.distanceText}>
              {formatDistance(distanceToDestination)} {t('shipments.toDestination')}
            </Text>
          )}
        </View>

        <Pressable style={styles.card} onPress={handleCallCustomer}>
          <Text style={styles.cardTitle}>{t('shipments.customer')}</Text>
          <Text style={styles.customerName}>{shipment.customer_name}</Text>
          <Text style={styles.customerPhone}>📞 {shipment.customer_phone}</Text>
          <Text style={styles.tapHint}>{t('shipments.tapToCall')}</Text>
        </Pressable>

        <View style={styles.card}>
          <Text style={styles.cardTitle}>{t('shipments.origin')}</Text>
          <Text style={styles.address}>{shipment.origin_address}</Text>
        </View>

        <View style={styles.card}>
          <Text style={styles.cardTitle}>{t('shipments.destination')}</Text>
          <Text style={styles.address}>{shipment.destination_address}</Text>
          {(shipment.destination_lat || shipment.destination_lng) && (
            <Pressable style={styles.navigateButton} onPress={handleNavigate}>
              <Text style={styles.navigateButtonText}>🗺️ {t('shipments.navigate')}</Text>
            </Pressable>
          )}
        </View>

        {shipment.cargo_description && (
          <View style={styles.card}>
            <Text style={styles.cardTitle}>{t('shipments.cargoDetails')}</Text>
            <Text style={styles.cargoText}>{shipment.cargo_description}</Text>
            {shipment.cargo_weight && (
              <Text style={styles.cargoMeta}>{t('shipments.weight')}: {shipment.cargo_weight} kg</Text>
            )}
          </View>
        )}

        {shipment.special_instructions && (
          <View style={[styles.card, styles.specialCard]}>
            <Text style={styles.cardTitle}>⚠️ {t('shipments.specialInstructions')}</Text>
            <Text style={styles.specialText}>{shipment.special_instructions}</Text>
          </View>
        )}

        <View style={styles.card}>
          <Text style={styles.cardTitle}>{t('shipments.mapPreview')}</Text>
          <ShipmentMiniMap 
            shipment={shipment} 
            isTracking={isTrackingThisShipment}
            height={180}
          />
          <Pressable style={styles.viewMapButton} onPress={handleViewOnMap}>
            <Text style={styles.viewMapButtonText}>{t('shipments.viewOnMap')}</Text>
          </Pressable>
        </View>

        <View style={styles.card}>
          <Text style={styles.cardTitle}>{t('shipments.recentActivity')}</Text>
          <TrackingEventTimeline shipmentId={shipment.id} limit={5} />
        </View>

        <View style={styles.card}>
          <Text style={styles.cardTitle}>{t('shipments.statusHistory')}</Text>
          <View style={styles.timeline}>
            <View style={styles.timelineItem}>
              <View style={styles.timelineDot} />
              <View style={styles.timelineContent}>
                <Text style={styles.timelineStatus}>
                  {t(`status.${shipment.status}`)}
                </Text>
                <Text style={styles.timelineTime}>{t('shipments.current')}</Text>
              </View>
            </View>
            {shipment.status_reason && (
              <Text style={styles.reasonText}>{t('shipments.reason')}: {shipment.status_reason}</Text>
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
  distanceText: {
    fontSize: 14,
    color: '#059669',
    marginTop: 8,
    fontWeight: '500',
  },
  trackingActiveIndicator: {
    flexDirection: 'row',
    alignItems: 'center',
    backgroundColor: '#d1fae5',
    paddingHorizontal: 12,
    paddingVertical: 8,
    borderRadius: 8,
    marginBottom: 12,
  },
  trackingDot: {
    width: 8,
    height: 8,
    borderRadius: 4,
    backgroundColor: '#059669',
    marginRight: 8,
  },
  trackingActiveText: {
    fontSize: 12,
    fontWeight: '600',
    color: '#065f46',
  },
  trackingWarning: {
    backgroundColor: '#fef3c7',
    paddingHorizontal: 12,
    paddingVertical: 8,
    borderRadius: 8,
    marginBottom: 12,
  },
  trackingWarningText: {
    fontSize: 12,
    color: '#92400e',
  },
  viewMapButton: {
    marginTop: 12,
    paddingVertical: 10,
    backgroundColor: '#2563eb',
    borderRadius: 8,
    alignItems: 'center',
  },
  viewMapButtonText: {
    color: '#fff',
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
  actionButtonDisabled: {
    opacity: 0.5,
  },
  actionButtonText: {
    color: '#fff',
    fontSize: 16,
    fontWeight: '600',
  },
});

function formatDistance(meters: number): string {
  if (meters >= 1000) {
    return `${(meters / 1000).toFixed(1)} km`;
  }
  return `${Math.round(meters)} m`;
}