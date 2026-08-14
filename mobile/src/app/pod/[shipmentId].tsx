import { View, StyleSheet, ActivityIndicator, Text, Alert } from 'react-native';
import { router, useLocalSearchParams, Stack } from 'expo-router';
import { useState, useEffect, useCallback } from 'react';
import { useAuthStore } from '@store/authStore';
import { useNetworkStore } from '@store/networkStore';
import { api } from '@/services/api';
import { API_ENDPOINTS } from '@/services/constants';
import { insertPOD, updateShipment } from '@db';
import { performSync } from '@/services/backgroundSync';
import type { Shipment } from '@/types/shipment';
import PODForm, { PODFormData } from '@/components/pod/PODForm';
import PODSuccess from '@/components/pod/PODSuccess';
import type { Coordinates, ProofOfDelivery } from '@/types/pod';

export default function PODScreen() {
  const { shipmentId } = useLocalSearchParams<{ shipmentId: string }>();
  const user = useAuthStore((state) => state.user);
  const isOnline = useNetworkStore((state) => state.isOnline);

  const [currentStep, setCurrentStep] = useState<'form' | 'submitting' | 'success'>('form');
  const [podData, setPodData] = useState<Partial<ProofOfDelivery>>({});
  const [shipment, setShipment] = useState<Shipment | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    loadShipment();
  }, [shipmentId]);

  const loadShipment = async () => {
    setIsLoading(true);
    setError(null);
    try {
      const response = await api.get<Shipment>(`/api/v1/shipments/${shipmentId}`);
      setShipment(response);
    } catch {
      try {
        const { getShipmentById } = await import('@db');
        const local = await getShipmentById(shipmentId!);
        if (local) {
          setShipment(local as unknown as Shipment);
        } else {
          setError('Failed to load shipment details');
        }
      } catch {
        setError('Failed to load shipment details');
      }
    } finally {
      setIsLoading(false);
    }
  };

  const destinationCoords: Coordinates | undefined = shipment?.destination_lat && shipment?.destination_lng
    ? { latitude: shipment.destination_lat, longitude: shipment.destination_lng }
    : undefined;

  const handleFormSubmit = useCallback(async (formData: PODFormData) => {
    if (!shipment || !user) return;

    setCurrentStep('submitting');
    setError(null);

    try {
      const podInput = {
        shipmentId: shipment.id,
        driverId: user.id,
        recipientName: formData.recipientName,
        recipientPhone: formData.recipientPhone,
        signatureData: formData.signatureBase64,
        photos: formData.photos,
        deliveryNotes: formData.notes,
        locationVerified: formData.locationVerified,
        locationMismatchMeters: formData.locationMismatchMeters,
        recordedAt: new Date().toISOString(),
      };

      setPodData(podInput);

      if (isOnline) {
        try {
          await api.post(API_ENDPOINTS.shipments.pod(shipment.id), podInput);
        } catch (apiError: unknown) {
          const errorMessage = apiError instanceof Error ? apiError.message : 'Unknown error';
          if (errorMessage.includes('Network') || errorMessage.includes('timeout')) {
            await insertPOD(podInput);
            performSync().catch(() => {});
          } else {
            throw apiError;
          }
        }
      } else {
        await insertPOD(podInput);
        performSync().catch(() => {});
      }

      await updateShipment(shipment.id, { status: 'delivered' }).catch(() => {});
      setCurrentStep('success');
    } catch (err: unknown) {
      const errorMessage = err instanceof Error ? err.message : 'An unexpected error occurred';
      setError(`Failed to submit proof of delivery: ${errorMessage}`);
      Alert.alert('Submission Failed', errorMessage);
      setCurrentStep('form');
    }
  }, [shipment, user, isOnline]);

  const handleDone = useCallback(() => {
    router.replace('/(main)');
  }, []);

  if (isLoading) {
    return (
      <View style={styles.loadingContainer}>
        <ActivityIndicator size="large" color="#2563eb" />
      </View>
    );
  }

  if (error && !shipment) {
    return (
      <View style={styles.errorContainer}>
        <Text style={styles.errorText}>{error}</Text>
      </View>
    );
  }

  if (currentStep === 'success' && shipment) {
    return (
      <PODSuccess
        shipmentId={shipment.id}
        trackingNumber={shipment.tracking_number}
        onDone={handleDone}
      />
    );
  }

  return (
    <>
      <Stack.Screen
        options={{
          title: 'Proof of Delivery',
          headerStyle: { backgroundColor: '#2563eb' },
          headerTintColor: '#fff',
        }}
      />
      <View style={styles.container}>
        {shipment && (
          <PODForm
            shipmentId={shipment.id}
            destinationCoords={destinationCoords}
            onSubmit={handleFormSubmit}
            isOffline={!isOnline}
          />
        )}
        
        {error && (
          <View style={styles.errorBanner}>
            <Text style={styles.errorBannerText}>{error}</Text>
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
  errorContainer: {
    flex: 1,
    justifyContent: 'center',
    alignItems: 'center',
    padding: 20,
  },
  errorText: {
    fontSize: 16,
    color: '#ef4444',
    textAlign: 'center',
  },
  errorBanner: {
    backgroundColor: '#fee2e2',
    padding: 12,
    margin: 16,
    borderRadius: 8,
  },
  errorBannerText: {
    fontSize: 14,
    color: '#dc2626',
    textAlign: 'center',
  },
});
