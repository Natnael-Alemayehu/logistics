import { generateUUID } from './helpers';
import { ProofOfDelivery, PODPhoto } from '../types/pod';
import { Paths, Directory, File } from 'expo-file-system';
import { Platform } from 'react-native';

export interface ValidationResult {
  valid: boolean;
  errors: string[];
}

export function generatePODId(): string {
  return generateUUID();
}

export function validatePOD(pod: Partial<ProofOfDelivery>): ValidationResult {
  const errors: string[] = [];

  if (!pod.shipmentId) {
    errors.push('Shipment ID is required');
  }

  if (!pod.driverId) {
    errors.push('Driver ID is required');
  }

  if (!pod.recipientName || pod.recipientName.trim().length === 0) {
    errors.push('Recipient name is required');
  }

  if (!pod.recordedAt) {
    errors.push('Recorded timestamp is required');
  }

  if (pod.photos && pod.photos.length === 0) {
    errors.push('At least one photo is required');
  }

  if (pod.signatureData) {
    if (!pod.signatureData.startsWith('data:image/') && !pod.signatureData.startsWith('/9j/')) {
      errors.push('Invalid signature data format');
    }
  }

  if (pod.deliveryLat !== undefined && (pod.deliveryLat < -90 || pod.deliveryLat > 90)) {
    errors.push('Invalid latitude value');
  }

  if (pod.deliveryLng !== undefined && (pod.deliveryLng < -180 || pod.deliveryLng > 180)) {
    errors.push('Invalid longitude value');
  }

  return {
    valid: errors.length === 0,
    errors,
  };
}

export async function saveSignatureToFile(
  base64Data: string,
  shipmentId: string
): Promise<string> {
  const fileName = `signature_${shipmentId}_${Date.now()}.png`;
  const signaturesDir = new Directory(Paths.document, 'signatures');
  
  if (!signaturesDir.exists) {
    signaturesDir.create();
  }

  const signatureFile = new File(signaturesDir, fileName);
  
  let base64Content = base64Data;
  if (base64Data.startsWith('data:image/png;base64,')) {
    base64Content = base64Data.replace('data:image/png;base64,', '');
  }

  signatureFile.write(base64Content);

  return signatureFile.uri;
}

export function preparePODForSync(pod: ProofOfDelivery): FormData {
  const formData = new FormData();

  formData.append('id', pod.id);
  formData.append('shipment_id', pod.shipmentId);
  formData.append('driver_id', pod.driverId);
  formData.append('recipient_name', pod.recipientName);
  
  if (pod.recipientPhone) {
    formData.append('recipient_phone', pod.recipientPhone);
  }

  if (pod.signatureData) {
    formData.append('signature_data', pod.signatureData);
  }

  if (pod.deliveryAddress) {
    formData.append('delivery_address', pod.deliveryAddress);
  }

  if (pod.deliveryLat !== undefined) {
    formData.append('delivery_lat', pod.deliveryLat.toString());
  }

  if (pod.deliveryLng !== undefined) {
    formData.append('delivery_lng', pod.deliveryLng.toString());
  }

  if (pod.deliveryNotes) {
    formData.append('delivery_notes', pod.deliveryNotes);
  }

  formData.append('location_verified', pod.locationVerified ? 'true' : 'false');

  if (pod.locationMismatchMeters !== undefined) {
    formData.append('location_mismatch_meters', pod.locationMismatchMeters.toString());
  }

  formData.append('recorded_at', pod.recordedAt);

  pod.photos.forEach((photo: PODPhoto, index: number) => {
    if (photo.localUri) {
      const uri = Platform.OS === 'ios' ? photo.localUri.replace('file://', '') : photo.localUri;
      const fileName = uri.split('/').pop() || `photo_${index}.jpg`;
      
      formData.append('photos', {
        uri: photo.localUri,
        type: 'image/jpeg',
        name: fileName,
      } as unknown as Blob);
    }
  });

  return formData;
}

export function extractBase64FromDataUrl(dataUrl: string): string {
  if (dataUrl.startsWith('data:')) {
    const base64Match = dataUrl.match(/base64,(.+)/);
    return base64Match ? base64Match[1] : dataUrl;
  }
  return dataUrl;
}

export function createBase64Png(base64Data: string): string {
  if (base64Data.startsWith('data:image/')) {
    return base64Data;
  }
  return `data:image/png;base64,${base64Data}`;
}
