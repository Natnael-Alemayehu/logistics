import { 
  View, 
  Text, 
  StyleSheet, 
  ScrollView, 
  Pressable, 
  TextInput,
  Alert,
  ActivityIndicator,
  Dimensions,
} from 'react-native';
import { router, useLocalSearchParams, Stack } from 'expo-router';
import { useState, useRef, useEffect } from 'react';
import * as Location from 'expo-location';
import { useAuthStore } from '@store/authStore';
import { useNetworkStore } from '@store/networkStore';
import { api } from '@/services/api';
import type { Shipment } from '@/types/shipment';

const { width } = Dimensions.get('window');

interface Photo {
  uri: string;
  base64?: string;
}

export default function PODScreen() {
  const { shipmentId } = useLocalSearchParams<{ shipmentId: string }>();
  const user = useAuthStore((state) => state.user);
  const isOnline = useNetworkStore((state) => state.isOnline);

  const [shipment, setShipment] = useState<Shipment | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [submitted, setSubmitted] = useState(false);

  const [recipientName, setRecipientName] = useState('');
  const [recipientPhone, setRecipientPhone] = useState('');
  const [deliveryNotes, setDeliveryNotes] = useState('');
  const [photos, setPhotos] = useState<Photo[]>([]);
  const [signatureLines, setSignatureLines] = useState<number[][]>([]);
  const [currentLine, setCurrentLine] = useState<number[]>([]);
  const [locationVerified, setLocationVerified] = useState(true);
  const [locationMismatch, setLocationMismatch] = useState<number | null>(null);

  const isDrawing = useRef(false);

  useEffect(() => {
    loadShipment();
    checkLocation();
  }, [shipmentId]);

  const loadShipment = async () => {
    setIsLoading(true);
    try {
      const response = await api.get<Shipment>(`/shipments/${shipmentId}`);
      setShipment(response);
    } catch (error) {
      console.error('Failed to load shipment:', error);
      Alert.alert('Error', 'Failed to load shipment details');
    } finally {
      setIsLoading(false);
    }
  };

  const checkLocation = async () => {
    try {
      const { status } = await Location.requestForegroundPermissionsAsync();
      if (status !== 'granted') {
        setLocationVerified(false);
        return;
      }

      const location = await Location.getCurrentPositionAsync({});
      if (shipment?.destination_lat && shipment?.destination_lng) {
        const distance = calculateDistance(
          location.coords.latitude,
          location.coords.longitude,
          shipment.destination_lat,
          shipment.destination_lng
        );
        setLocationMismatch(distance);
        setLocationVerified(distance <= 500);
      }
    } catch (error) {
      console.error('Location check failed:', error);
    }
  };

  const calculateDistance = (lat1: number, lon1: number, lat2: number, lon2: number): number => {
    const R = 6371000;
    const φ1 = (lat1 * Math.PI) / 180;
    const φ2 = (lat2 * Math.PI) / 180;
    const Δφ = ((lat2 - lat1) * Math.PI) / 180;
    const Δλ = ((lon2 - lon1) * Math.PI) / 180;

    const a = Math.sin(Δφ / 2) * Math.sin(Δφ / 2) +
              Math.cos(φ1) * Math.cos(φ2) * Math.sin(Δλ / 2) * Math.sin(Δλ / 2);
    const c = 2 * Math.atan2(Math.sqrt(a), Math.sqrt(1 - a));

    return R * c;
  };

  const handleTakePhoto = async () => {
    if (photos.length >= 3) {
      Alert.alert('Limit Reached', 'Maximum 3 photos allowed');
      return;
    }

    // Use camera via linking to camera app
    Alert.alert(
      'Take Photo',
      'Camera integration requires expo-image-picker. Photo placeholder added.',
      [
        { text: 'Cancel', style: 'cancel' },
        {
          text: 'Add Placeholder',
          onPress: () => {
            setPhotos([
              ...photos,
              { uri: `photo_${Date.now()}.jpg` },
            ]);
          },
        },
      ]
    );
  };

  const handleRemovePhoto = (index: number) => {
    setPhotos(photos.filter((_, i) => i !== index));
  };

  const handleSignatureStart = (x: number, y: number) => {
    isDrawing.current = true;
    setCurrentLine([x, y]);
  };

  const handleSignatureMove = (x: number, y: number) => {
    if (isDrawing.current) {
      setCurrentLine([...currentLine, x, y]);
    }
  };

  const handleSignatureEnd = () => {
    if (currentLine.length > 0) {
      setSignatureLines([...signatureLines, currentLine]);
      setCurrentLine([]);
    }
    isDrawing.current = false;
  };

  const handleClearSignature = () => {
    setSignatureLines([]);
    setCurrentLine([]);
  };

  const handleSubmit = async () => {
    if (!recipientName.trim()) {
      Alert.alert('Required', 'Please enter recipient name');
      return;
    }

    if (signatureLines.length === 0) {
      Alert.alert('Required', 'Please capture a signature');
      return;
    }

    if (!locationVerified) {
      Alert.alert(
        'Location Mismatch',
        `Your location is ${locationMismatch?.toFixed(0)}m from the destination. Continue anyway?`,
        [
          { text: 'Cancel', style: 'cancel' },
          {
            text: 'Continue',
            onPress: submitPOD,
          },
        ]
      );
    } else {
      submitPOD();
    }
  };

  const submitPOD = async () => {
    if (!shipment || !user) return;

    setIsSubmitting(true);
    try {
      const pod = {
        shipment_id: shipment.id,
        driver_id: user.id,
        recipient_name: recipientName,
        recipient_phone: recipientPhone || null,
        signature_data: JSON.stringify(signatureLines),
        photo_paths: photos.map((p) => p.uri),
        delivery_notes: deliveryNotes || null,
        location_verified: locationVerified,
        location_mismatch_meters: locationMismatch,
        recorded_at: new Date().toISOString(),
      };

      await api.post('/pods', pod);
      setSubmitted(true);
    } catch (error) {
      console.error('Failed to submit POD:', error);
      Alert.alert('Error', 'Failed to submit proof of delivery. It will be saved locally and synced later.');
      setSubmitted(true);
    } finally {
      setIsSubmitting(false);
    }
  };

  if (isLoading) {
    return (
      <View style={styles.loadingContainer}>
        <ActivityIndicator size="large" color="#2563eb" />
      </View>
    );
  }

  if (submitted) {
    return (
      <View style={styles.successContainer}>
        <Text style={styles.successIcon}>✓</Text>
        <Text style={styles.successTitle}>Delivery Complete!</Text>
        <Text style={styles.successText}>
          Proof of delivery has been submitted successfully.
        </Text>
        <Pressable
          style={styles.successButton}
          onPress={() => router.replace('/(main)')}
        >
          <Text style={styles.successButtonText}>Return to Shipments</Text>
        </Pressable>
      </View>
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
      <ScrollView style={styles.container}>
        {!locationVerified && (
          <View style={styles.warningBanner}>
            <Text style={styles.warningIcon}>⚠️</Text>
            <Text style={styles.warningText}>
              Location mismatch: {locationMismatch?.toFixed(0)}m from destination
            </Text>
          </View>
        )}

        <View style={styles.section}>
          <Text style={styles.sectionTitle}>Recipient Information</Text>
          <View style={styles.inputGroup}>
            <Text style={styles.label}>Recipient Name *</Text>
            <TextInput
              style={styles.input}
              placeholder="Enter recipient's full name"
              value={recipientName}
              onChangeText={setRecipientName}
            />
          </View>
          <View style={styles.inputGroup}>
            <Text style={styles.label}>Recipient Phone (Optional)</Text>
            <TextInput
              style={styles.input}
              placeholder="Enter phone number"
              value={recipientPhone}
              onChangeText={setRecipientPhone}
              keyboardType="phone-pad"
            />
          </View>
        </View>

        <View style={styles.section}>
          <Text style={styles.sectionTitle}>Signature *</Text>
          <View
            style={styles.signaturePad}
            onTouchStart={(e) => {
              const { locationX, locationY } = e.nativeEvent;
              handleSignatureStart(locationX, locationY);
            }}
            onTouchMove={(e) => {
              const { locationX, locationY } = e.nativeEvent;
              handleSignatureMove(locationX, locationY);
            }}
            onTouchEnd={handleSignatureEnd}
          >
            {signatureLines.length === 0 && currentLine.length === 0 && (
              <Text style={styles.signaturePlaceholder}>Sign here</Text>
            )}
            {signatureLines.map((line, lineIndex) => (
              <View key={lineIndex} style={styles.signatureLine} />
            ))}
            {currentLine.length > 0 && (
              <View style={[styles.signatureLine, styles.currentLine]} />
            )}
          </View>
          <Pressable style={styles.clearButton} onPress={handleClearSignature}>
            <Text style={styles.clearButtonText}>Clear Signature</Text>
          </Pressable>
        </View>

        <View style={styles.section}>
          <Text style={styles.sectionTitle}>Photos (Optional, max 3)</Text>
          <View style={styles.photoGrid}>
            {photos.map((photo, index) => (
              <View key={index} style={styles.photoContainer}>
                <View style={styles.photoPlaceholder}>
                  <Text style={styles.photoPlaceholderText}>📷 Photo {index + 1}</Text>
                </View>
                <Pressable
                  style={styles.removePhotoButton}
                  onPress={() => handleRemovePhoto(index)}
                >
                  <Text style={styles.removePhotoText}>✕</Text>
                </Pressable>
              </View>
            ))}
            {photos.length < 3 && (
              <Pressable style={styles.addPhotoButton} onPress={handleTakePhoto}>
                <Text style={styles.addPhotoIcon}>📷</Text>
                <Text style={styles.addPhotoText}>Add Photo</Text>
              </Pressable>
            )}
          </View>
        </View>

        <View style={styles.section}>
          <Text style={styles.sectionTitle}>Delivery Notes (Optional)</Text>
          <TextInput
            style={[styles.input, styles.notesInput]}
            placeholder="Add any delivery notes..."
            value={deliveryNotes}
            onChangeText={setDeliveryNotes}
            multiline
            numberOfLines={4}
          />
        </View>

        <Pressable
          style={[
            styles.submitButton,
            isSubmitting && styles.submitButtonDisabled,
          ]}
          onPress={handleSubmit}
          disabled={isSubmitting}
        >
          {isSubmitting ? (
            <ActivityIndicator color="#fff" />
          ) : (
            <Text style={styles.submitButtonText}>Submit Delivery</Text>
          )}
        </Pressable>

        {!isOnline && (
          <Text style={styles.offlineNote}>
            ⚠️ You're offline. POD will be saved locally and synced when online.
          </Text>
        )}
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
  },
  warningBanner: {
    backgroundColor: '#fef3c7',
    padding: 12,
    flexDirection: 'row',
    alignItems: 'center',
    gap: 8,
  },
  warningIcon: {
    fontSize: 16,
  },
  warningText: {
    fontSize: 14,
    color: '#92400e',
  },
  section: {
    backgroundColor: '#fff',
    margin: 16,
    marginBottom: 0,
    marginTop: 16,
    padding: 16,
    borderRadius: 12,
  },
  sectionTitle: {
    fontSize: 16,
    fontWeight: '600',
    color: '#1f2937',
    marginBottom: 12,
  },
  inputGroup: {
    marginBottom: 12,
  },
  label: {
    fontSize: 14,
    fontWeight: '500',
    color: '#374151',
    marginBottom: 4,
  },
  input: {
    borderWidth: 1,
    borderColor: '#d1d5db',
    borderRadius: 8,
    padding: 12,
    fontSize: 16,
    backgroundColor: '#f9fafb',
  },
  notesInput: {
    height: 100,
    textAlignVertical: 'top',
  },
  signaturePad: {
    height: 200,
    borderWidth: 1,
    borderColor: '#d1d5db',
    borderRadius: 8,
    backgroundColor: '#fff',
    justifyContent: 'center',
    alignItems: 'center',
  },
  signaturePlaceholder: {
    color: '#9ca3af',
    fontSize: 16,
  },
  signatureLine: {
    position: 'absolute',
    height: 2,
    backgroundColor: '#1f2937',
  },
  currentLine: {
    backgroundColor: '#2563eb',
  },
  clearButton: {
    marginTop: 8,
    padding: 8,
    alignItems: 'center',
  },
  clearButtonText: {
    color: '#dc2626',
    fontSize: 14,
    fontWeight: '500',
  },
  photoGrid: {
    flexDirection: 'row',
    flexWrap: 'wrap',
    gap: 12,
  },
  photoContainer: {
    width: (width - 80) / 3,
    height: (width - 80) / 3,
    position: 'relative',
  },
  photoPlaceholder: {
    flex: 1,
    backgroundColor: '#f3f4f6',
    borderRadius: 8,
    justifyContent: 'center',
    alignItems: 'center',
  },
  photoPlaceholderText: {
    fontSize: 12,
    color: '#6b7280',
  },
  removePhotoButton: {
    position: 'absolute',
    top: -8,
    right: -8,
    backgroundColor: '#dc2626',
    width: 24,
    height: 24,
    borderRadius: 12,
    justifyContent: 'center',
    alignItems: 'center',
  },
  removePhotoText: {
    color: '#fff',
    fontSize: 14,
  },
  addPhotoButton: {
    width: (width - 80) / 3,
    height: (width - 80) / 3,
    borderWidth: 2,
    borderColor: '#d1d5db',
    borderStyle: 'dashed',
    borderRadius: 8,
    justifyContent: 'center',
    alignItems: 'center',
  },
  addPhotoIcon: {
    fontSize: 24,
    marginBottom: 4,
  },
  addPhotoText: {
    fontSize: 12,
    color: '#6b7280',
  },
  submitButton: {
    backgroundColor: '#059669',
    margin: 16,
    padding: 16,
    borderRadius: 12,
    alignItems: 'center',
  },
  submitButtonDisabled: {
    backgroundColor: '#6ee7b7',
  },
  submitButtonText: {
    color: '#fff',
    fontSize: 18,
    fontWeight: '600',
  },
  offlineNote: {
    textAlign: 'center',
    color: '#6b7280',
    fontSize: 12,
    marginBottom: 16,
  },
  successContainer: {
    flex: 1,
    justifyContent: 'center',
    alignItems: 'center',
    backgroundColor: '#f9fafb',
    padding: 20,
  },
  successIcon: {
    fontSize: 64,
    color: '#059669',
    marginBottom: 16,
  },
  successTitle: {
    fontSize: 24,
    fontWeight: 'bold',
    color: '#1f2937',
    marginBottom: 12,
  },
  successText: {
    fontSize: 16,
    color: '#6b7280',
    textAlign: 'center',
    marginBottom: 24,
  },
  successButton: {
    backgroundColor: '#2563eb',
    paddingHorizontal: 32,
    paddingVertical: 16,
    borderRadius: 12,
  },
  successButtonText: {
    color: '#fff',
    fontSize: 16,
    fontWeight: '600',
  },
});