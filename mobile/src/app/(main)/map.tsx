import { 
  View, 
  Text, 
  StyleSheet, 
  Pressable,
  ActivityIndicator,
  Linking,
} from 'react-native';
import { router, Stack } from 'expo-router';
import { useState, useEffect, useCallback, useMemo } from 'react';
import type { Shipment } from '@/types/shipment';
import type { MapMarker, Coordinates } from '@/types/maps';
import { api } from '@/services/api';
import { useLocation } from '@/hooks/useLocation';
import { useTrackingStore } from '@/store/trackingStore';
import { TrackingMap } from '@/components/tracking/TrackingMap';
import { TrackingOverlay } from '@/components/tracking/TrackingOverlay';
import { colors, spacing, BorderRadius } from '@/utils/theme';
import { X, Navigation, Info } from 'lucide-react-native';

interface ShipmentWithLocation extends Shipment {
  distance?: number;
}

export default function MapScreen() {
  const [shipments, setShipments] = useState<ShipmentWithLocation[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [selectedShipment, setSelectedShipment] = useState<ShipmentWithLocation | null>(null);
  const [showOfflineMessage, setShowOfflineMessage] = useState(false);
  
  const { lastKnownLocation, isTracking, startTracking, stopTracking } = useLocation();
  const {
    isTracking: storeIsTracking,
    activeShipmentId,
    startTracking: setTrackingActive,
    stopTracking: setTrackingInactive,
    lastPosition,
  } = useTrackingStore();
  
  useEffect(() => {
    loadData();
  }, []);
  
  const loadData = async () => {
    setIsLoading(true);
    try {
      const response = await api.get<Shipment[]>('/my-shipments');
      const activeShipments = response.filter(
        (s) => !['delivered', 'cancelled'].includes(s.status)
      );
      setShipments(activeShipments);
    } catch (error) {
      console.error('Failed to load shipments:', error);
    } finally {
      setIsLoading(false);
    }
  };
  
  const handleMarkerPress = useCallback((marker: MapMarker) => {
    if (marker.type === 'destination') {
      const shipment = shipments.find(s => `dest-${s.id}` === marker.id);
      if (shipment) {
        setSelectedShipment(shipment);
      }
    }
  }, [shipments]);
  
  const handleMapPress = useCallback((coordinate: Coordinates) => {
    setSelectedShipment(null);
  }, []);
  
  const handleNavigate = useCallback((shipment: Shipment) => {
    if (shipment.destination_lat && shipment.destination_lng) {
      const url = `https://www.google.com/maps/dir/?api=1&destination=${shipment.destination_lat},${shipment.destination_lng}`;
      if (lastKnownLocation) {
        Linking.openURL(
          `${url}&origin=${lastKnownLocation.latitude},${lastKnownLocation.longitude}`
        );
      } else {
        Linking.openURL(url);
      }
    }
  }, [lastKnownLocation]);
  
  const handleViewDetails = useCallback((shipment: Shipment) => {
    router.push(`/shipment/${shipment.id}`);
  }, []);
  
  const handleToggleTracking = useCallback(async () => {
    if (storeIsTracking) {
      await stopTracking();
    } else if (selectedShipment) {
      await startTracking({ shipmentId: selectedShipment.id });
    }
  }, [storeIsTracking, selectedShipment, startTracking, stopTracking]);
  
  const selectedDestination = useMemo(() => {
    if (!selectedShipment?.destination_lat || !selectedShipment?.destination_lng) {
      return undefined;
    }
    return {
      latitude: selectedShipment.destination_lat,
      longitude: selectedShipment.destination_lng,
    };
  }, [selectedShipment]);
  
  const handleOfflineMap = useCallback(() => {
    setShowOfflineMessage(true);
    setTimeout(() => setShowOfflineMessage(false), 3000);
  }, []);
  
  if (isLoading) {
    return (
      <>
        <Stack.Screen
          options={{
            title: 'Delivery Map',
            headerStyle: { backgroundColor: colors.primary },
            headerTintColor: '#fff',
          }}
        />
        <View style={styles.loadingContainer}>
          <ActivityIndicator size="large" color={colors.primary} />
        </View>
      </>
    );
  }
  
  return (
    <>
      <Stack.Screen
        options={{
          title: 'Delivery Map',
          headerStyle: { backgroundColor: colors.primary },
          headerTintColor: '#fff',
        }}
      />
      <View style={styles.container}>
        <TrackingMap
          shipments={shipments}
          activeShipmentId={activeShipmentId ?? undefined}
          showCurrentLocation={true}
          onMarkerPress={handleMarkerPress}
          onMapPress={handleMapPress}
          showRoute={!!selectedShipment}
          showAccuracyCircle={true}
          followUser={false}
          style={styles.map}
        />
        
        <TrackingOverlay
          destinationCoordinate={selectedDestination}
          onNavigatePress={() => selectedShipment && handleNavigate(selectedShipment)}
          onToggleTracking={handleToggleTracking}
          estimatedSpeedKph={30}
        />
        
        {showOfflineMessage && (
          <View style={styles.offlineBanner}>
            <Text style={styles.offlineText}>
              Map tiles cached for offline use
            </Text>
          </View>
        )}
        
        {selectedShipment && (
          <View style={styles.selectedCard}>
            <View style={styles.selectedHeader}>
              <View style={styles.selectedInfo}>
                <Text style={styles.selectedTracking}>
                  {selectedShipment.tracking_number}
                </Text>
                <Text style={styles.selectedCustomer}>
                  {selectedShipment.customer_name}
                </Text>
              </View>
              <Pressable 
                style={styles.closeButton}
                onPress={() => setSelectedShipment(null)}
              >
                <X size={20} color={colors.textSecondary} />
              </Pressable>
            </View>
            
            <Text style={styles.selectedAddress}>
              {selectedShipment.destination_address}
            </Text>
            
            <View style={styles.selectedActions}>
              <Pressable
                style={styles.navigateButton}
                onPress={() => handleNavigate(selectedShipment)}
              >
                <Navigation size={18} color="#fff" />
                <Text style={styles.navigateButtonText}>Navigate</Text>
              </Pressable>
              
              <Pressable
                style={styles.detailsButton}
                onPress={() => handleViewDetails(selectedShipment)}
              >
                <Info size={18} color={colors.primary} />
                <Text style={styles.detailsButtonText}>Details</Text>
              </Pressable>
            </View>
          </View>
        )}
        
        {shipments.length === 0 && (
          <View style={styles.emptyOverlay}>
            <Text style={styles.emptyText}>No active deliveries</Text>
            <Text style={styles.emptySubtext}>
              Your delivery locations will appear here
            </Text>
          </View>
        )}
      </View>
    </>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: colors.background,
  },
  loadingContainer: {
    flex: 1,
    justifyContent: 'center',
    alignItems: 'center',
    backgroundColor: colors.background,
  },
  map: {
    flex: 1,
  },
  offlineBanner: {
    position: 'absolute',
    top: 10,
    left: spacing.md,
    right: spacing.md,
    backgroundColor: colors.gray[800],
    padding: spacing.md,
    borderRadius: BorderRadius.md,
    alignItems: 'center',
  },
  offlineText: {
    color: colors.surface,
    fontSize: 14,
  },
  selectedCard: {
    position: 'absolute',
    bottom: 0,
    left: 0,
    right: 0,
    backgroundColor: colors.surface,
    padding: spacing.lg,
    borderTopLeftRadius: BorderRadius.xl,
    borderTopRightRadius: BorderRadius.xl,
    shadowColor: '#000',
    shadowOffset: { width: 0, height: -4 },
    shadowOpacity: 0.15,
    shadowRadius: 8,
    elevation: 8,
  },
  selectedHeader: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'flex-start',
    marginBottom: spacing.sm,
  },
  selectedInfo: {
    flex: 1,
  },
  selectedTracking: {
    fontSize: 16,
    fontWeight: '600',
    color: colors.text,
  },
  selectedCustomer: {
    fontSize: 14,
    color: colors.textSecondary,
    marginTop: 2,
  },
  closeButton: {
    padding: spacing.xs,
  },
  selectedAddress: {
    fontSize: 14,
    color: colors.gray[600],
    marginBottom: spacing.md,
  },
  selectedActions: {
    flexDirection: 'row',
    gap: spacing.md,
  },
  navigateButton: {
    flex: 1,
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'center',
    gap: spacing.xs,
    backgroundColor: colors.primary,
    padding: spacing.md,
    borderRadius: BorderRadius.md,
  },
  navigateButtonText: {
    color: '#fff',
    fontWeight: '600',
    fontSize: 15,
  },
  detailsButton: {
    flex: 1,
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'center',
    gap: spacing.xs,
    backgroundColor: colors.gray[100],
    padding: spacing.md,
    borderRadius: BorderRadius.md,
  },
  detailsButtonText: {
    color: colors.primary,
    fontWeight: '600',
    fontSize: 15,
  },
  emptyOverlay: {
    position: 'absolute',
    top: '40%',
    left: spacing.xxl,
    right: spacing.xxl,
    alignItems: 'center',
  },
  emptyText: {
    fontSize: 18,
    fontWeight: '600',
    color: colors.text,
    marginBottom: spacing.xs,
  },
  emptySubtext: {
    fontSize: 14,
    color: colors.textSecondary,
    textAlign: 'center',
  },
});
