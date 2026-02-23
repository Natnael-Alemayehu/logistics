import { 
  View, 
  Text, 
  StyleSheet, 
  Pressable,
  ActivityIndicator,
} from 'react-native';
import { router, Stack } from 'expo-router';
import { useState, useEffect } from 'react';
import * as Location from 'expo-location';
import type { Shipment } from '@/types/shipment';
import { api } from '@/services/api';
import { Linking } from 'react-native';

interface ShipmentWithLocation extends Shipment {
  distance?: number;
}

export default function MapScreen() {
  const [shipments, setShipments] = useState<ShipmentWithLocation[]>([]);
  const [location, setLocation] = useState<Location.LocationObject | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [selectedShipment, setSelectedShipment] = useState<ShipmentWithLocation | null>(null);

  useEffect(() => {
    loadData();
  }, []);

  const loadData = async () => {
    setIsLoading(true);
    try {
      const { status } = await Location.requestForegroundPermissionsAsync();
      if (status === 'granted') {
        const loc = await Location.getCurrentPositionAsync({});
        setLocation(loc);
      }

      const response = await api.get<Shipment[]>('/my-shipments');
      const activeShipments = response.filter(
        (s) => !['delivered', 'cancelled'].includes(s.status)
      );
      setShipments(activeShipments);
    } catch (error) {
      console.error('Failed to load data:', error);
    } finally {
      setIsLoading(false);
    }
  };

  const handleNavigate = (shipment: Shipment) => {
    if (shipment.destination_lat && shipment.destination_lng) {
      const url = `https://www.google.com/maps/dir/?api=1&destination=${shipment.destination_lat},${shipment.destination_lng}`;
      if (location) {
        Linking.openURL(
          `${url}&origin=${location.coords.latitude},${location.coords.longitude}`
        );
      } else {
        Linking.openURL(url);
      }
    }
  };

  const handleViewDetails = (shipment: Shipment) => {
    router.push(`/shipment/${shipment.id}`);
  };

  if (isLoading) {
    return (
      <View style={styles.loadingContainer}>
        <ActivityIndicator size="large" color="#2563eb" />
      </View>
    );
  }

  return (
    <>
      <Stack.Screen
        options={{
          title: 'Delivery Map',
          headerStyle: { backgroundColor: '#2563eb' },
          headerTintColor: '#fff',
        }}
      />
      <View style={styles.container}>
        <View style={styles.mapPlaceholder}>
          {location ? (
            <View style={styles.currentLocation}>
              <Text style={styles.locationIcon}>📍</Text>
              <Text style={styles.locationText}>
                Your location: {location.coords.latitude.toFixed(4)},{' '}
                {location.coords.longitude.toFixed(4)}
              </Text>
            </View>
          ) : (
            <Text style={styles.noLocation}>Location not available</Text>
          )}

          <View style={styles.shipmentPins}>
            <Text style={styles.pinsTitle}>Active Destinations</Text>
            {shipments.length === 0 ? (
              <Text style={styles.noShipments}>No active shipments</Text>
            ) : (
              shipments.map((shipment) => (
                <Pressable
                  key={shipment.id}
                  style={[
                    styles.pinCard,
                    selectedShipment?.id === shipment.id && styles.pinCardSelected,
                  ]}
                  onPress={() => setSelectedShipment(shipment)}
                >
                  <View style={styles.pinHeader}>
                    <Text style={styles.pinTracking}>{shipment.tracking_number}</Text>
                    <Text style={styles.pinIcon}>📍</Text>
                  </View>
                  <Text style={styles.pinAddress} numberOfLines={2}>
                    {shipment.destination_address}
                  </Text>
                  <Text style={styles.pinCustomer}>{shipment.customer_name}</Text>
                </Pressable>
              ))
            )}
          </View>
        </View>

        {selectedShipment && (
          <View style={styles.selectedCard}>
            <View style={styles.selectedHeader}>
              <View>
                <Text style={styles.selectedTracking}>
                  {selectedShipment.tracking_number}
                </Text>
                <Text style={styles.selectedCustomer}>
                  {selectedShipment.customer_name}
                </Text>
              </View>
              <Pressable onPress={() => setSelectedShipment(null)}>
                <Text style={styles.closeButton}>✕</Text>
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
                <Text style={styles.navigateButtonText}>🗺️ Navigate</Text>
              </Pressable>
              <Pressable
                style={styles.detailsButton}
                onPress={() => handleViewDetails(selectedShipment)}
              >
                <Text style={styles.detailsButtonText}>View Details</Text>
              </Pressable>
            </View>
          </View>
        )}
      </View>
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
  },
  mapPlaceholder: {
    flex: 1,
    padding: 16,
  },
  currentLocation: {
    backgroundColor: '#dbeafe',
    padding: 16,
    borderRadius: 12,
    alignItems: 'center',
    marginBottom: 16,
  },
  locationIcon: {
    fontSize: 24,
    marginBottom: 8,
  },
  locationText: {
    fontSize: 14,
    color: '#1e40af',
  },
  noLocation: {
    textAlign: 'center',
    color: '#6b7280',
    padding: 16,
  },
  shipmentPins: {
    flex: 1,
  },
  pinsTitle: {
    fontSize: 16,
    fontWeight: '600',
    color: '#374151',
    marginBottom: 12,
  },
  noShipments: {
    textAlign: 'center',
    color: '#6b7280',
    marginTop: 24,
  },
  pinCard: {
    backgroundColor: '#fff',
    padding: 12,
    borderRadius: 12,
    marginBottom: 8,
    borderWidth: 2,
    borderColor: 'transparent',
  },
  pinCardSelected: {
    borderColor: '#2563eb',
  },
  pinHeader: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
    marginBottom: 4,
  },
  pinTracking: {
    fontSize: 14,
    fontWeight: '600',
    color: '#1f2937',
  },
  pinIcon: {
    fontSize: 16,
  },
  pinAddress: {
    fontSize: 12,
    color: '#4b5563',
    marginBottom: 4,
  },
  pinCustomer: {
    fontSize: 12,
    color: '#6b7280',
  },
  selectedCard: {
    backgroundColor: '#fff',
    padding: 16,
    borderTopWidth: 1,
    borderTopColor: '#e5e7eb',
    shadowColor: '#000',
    shadowOffset: { width: 0, height: -2 },
    shadowOpacity: 0.1,
    shadowRadius: 4,
    elevation: 4,
  },
  selectedHeader: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'flex-start',
    marginBottom: 8,
  },
  selectedTracking: {
    fontSize: 16,
    fontWeight: '600',
    color: '#1f2937',
  },
  selectedCustomer: {
    fontSize: 14,
    color: '#6b7280',
  },
  closeButton: {
    fontSize: 20,
    color: '#6b7280',
    padding: 4,
  },
  selectedAddress: {
    fontSize: 14,
    color: '#374151',
    marginBottom: 16,
  },
  selectedActions: {
    flexDirection: 'row',
    gap: 12,
  },
  navigateButton: {
    flex: 1,
    backgroundColor: '#2563eb',
    padding: 12,
    borderRadius: 8,
    alignItems: 'center',
  },
  navigateButtonText: {
    color: '#fff',
    fontWeight: '600',
  },
  detailsButton: {
    flex: 1,
    backgroundColor: '#f3f4f6',
    padding: 12,
    borderRadius: 8,
    alignItems: 'center',
  },
  detailsButtonText: {
    color: '#374151',
    fontWeight: '600',
  },
});