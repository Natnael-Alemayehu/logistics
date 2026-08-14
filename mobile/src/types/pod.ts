export interface PODPhoto {
  id: string;
  localUri: string;
  thumbnailUri?: string;
  latitude?: number;
  longitude?: number;
  capturedAt: string;
  fileSize: number;
  width: number;
  height: number;
  uploaded?: boolean;
  remoteUrl?: string;
}

export interface ProofOfDelivery {
  id: string;
  shipmentId: string;
  driverId: string;
  recipientName: string;
  recipientPhone?: string;
  signatureData?: string;
  photos: PODPhoto[];
  deliveryAddress?: string;
  deliveryLat?: number;
  deliveryLng?: number;
  deliveryNotes?: string;
  locationVerified: boolean;
  locationMismatchMeters?: number;
  recordedAt: string;
  syncedAt?: string;
  syncStatus: 'pending' | 'syncing' | 'synced' | 'failed';
}

export interface ProofOfDeliveryDB {
  id: string;
  shipment_id: string;
  driver_id: string;
  recipient_name: string;
  recipient_phone?: string;
  signature_data?: string;
  photo_paths?: string;
  delivery_address?: string;
  delivery_lat?: number;
  delivery_lng?: number;
  delivery_notes?: string;
  location_verified: number;
  location_mismatch_meters?: number;
  recorded_at: string;
  synced_at?: string;
  sync_status: 'pending' | 'syncing' | 'synced' | 'failed';
}

export interface PODInput {
  id?: string;
  shipmentId: string;
  driverId: string;
  recipientName: string;
  recipientPhone?: string;
  signatureData?: string;
  photos?: PODPhoto[];
  deliveryAddress?: string;
  deliveryLat?: number;
  deliveryLng?: number;
  deliveryNotes?: string;
  locationVerified?: boolean;
  locationMismatchMeters?: number;
  recordedAt?: string;
}

export function toDBFormat(pod: ProofOfDelivery): ProofOfDeliveryDB {
  return {
    id: pod.id,
    shipment_id: pod.shipmentId,
    driver_id: pod.driverId,
    recipient_name: pod.recipientName,
    recipient_phone: pod.recipientPhone,
    signature_data: pod.signatureData,
    photo_paths: pod.photos.length > 0 ? JSON.stringify(pod.photos) : undefined,
    delivery_address: pod.deliveryAddress,
    delivery_lat: pod.deliveryLat,
    delivery_lng: pod.deliveryLng,
    delivery_notes: pod.deliveryNotes,
    location_verified: pod.locationVerified ? 1 : 0,
    location_mismatch_meters: pod.locationMismatchMeters,
    recorded_at: pod.recordedAt,
    synced_at: pod.syncedAt,
    sync_status: pod.syncStatus,
  };
}

export function fromDBFormat(dbPod: ProofOfDeliveryDB): ProofOfDelivery {
  let photos: PODPhoto[] = [];
  if (dbPod.photo_paths) {
    try {
      photos = JSON.parse(dbPod.photo_paths);
    } catch {
      photos = [];
    }
  }

  return {
    id: dbPod.id,
    shipmentId: dbPod.shipment_id,
    driverId: dbPod.driver_id,
    recipientName: dbPod.recipient_name,
    recipientPhone: dbPod.recipient_phone,
    signatureData: dbPod.signature_data,
    photos,
    deliveryAddress: dbPod.delivery_address,
    deliveryLat: dbPod.delivery_lat,
    deliveryLng: dbPod.delivery_lng,
    deliveryNotes: dbPod.delivery_notes,
    locationVerified: dbPod.location_verified === 1,
    locationMismatchMeters: dbPod.location_mismatch_meters,
    recordedAt: dbPod.recorded_at,
    syncedAt: dbPod.synced_at,
    syncStatus: dbPod.sync_status,
  };
}

export function inputToDBFormat(input: PODInput): Record<string, unknown> {
  const result: Record<string, unknown> = {};
  
  if (input.id !== undefined) result.id = input.id;
  if (input.shipmentId !== undefined) result.shipment_id = input.shipmentId;
  if (input.driverId !== undefined) result.driver_id = input.driverId;
  if (input.recipientName !== undefined) result.recipient_name = input.recipientName;
  if (input.recipientPhone !== undefined) result.recipient_phone = input.recipientPhone;
  if (input.signatureData !== undefined) result.signature_data = input.signatureData;
  if (input.photos !== undefined) result.photo_paths = JSON.stringify(input.photos);
  if (input.deliveryAddress !== undefined) result.delivery_address = input.deliveryAddress;
  if (input.deliveryLat !== undefined) result.delivery_lat = input.deliveryLat;
  if (input.deliveryLng !== undefined) result.delivery_lng = input.deliveryLng;
  if (input.deliveryNotes !== undefined) result.delivery_notes = input.deliveryNotes;
  if (input.locationVerified !== undefined) result.location_verified = input.locationVerified ? 1 : 0;
  if (input.locationMismatchMeters !== undefined) result.location_mismatch_meters = input.locationMismatchMeters;
  if (input.recordedAt !== undefined) result.recorded_at = input.recordedAt;
  
  return result;
}

export interface Coordinates {
  latitude: number;
  longitude: number;
}

export interface PhotoWithMetadata {
  uri: string;
  width: number;
  height: number;
  fileSize: number;
  latitude?: number;
  longitude?: number;
  capturedAt: string;
}

export interface CompressedPhoto {
  uri: string;
  width: number;
  height: number;
  fileSize: number;
}
