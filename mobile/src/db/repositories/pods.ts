import { getDatabase } from '../database';
import { generateUUID } from '@utils/helpers';
import {
  ProofOfDelivery,
  ProofOfDeliveryDB,
  PODInput,
  PODPhoto,
  toDBFormat,
  fromDBFormat,
  inputToDBFormat,
} from '../../types/pod';

export type { ProofOfDelivery, ProofOfDeliveryDB, PODInput, PODPhoto };

export type POD = ProofOfDelivery;

function parsePODRow(row: Record<string, unknown>): ProofOfDeliveryDB {
  return {
    id: row.id as string,
    shipment_id: row.shipment_id as string,
    driver_id: row.driver_id as string,
    recipient_name: row.recipient_name as string,
    recipient_phone: row.recipient_phone as string | undefined,
    signature_data: row.signature_data as string | undefined,
    photo_paths: row.photo_paths as string | undefined,
    delivery_address: row.delivery_address as string | undefined,
    delivery_lat: row.delivery_lat as number | undefined,
    delivery_lng: row.delivery_lng as number | undefined,
    delivery_notes: row.delivery_notes as string | undefined,
    location_verified: row.location_verified as number,
    location_mismatch_meters: row.location_mismatch_meters as number | undefined,
    recorded_at: row.recorded_at as string,
    synced_at: row.synced_at as string | undefined,
    sync_status: row.sync_status as 'pending' | 'syncing' | 'synced' | 'failed',
  };
}

export async function insert(pod: PODInput): Promise<string> {
  const db = await getDatabase();
  const id = pod.id ?? generateUUID();
  const recordedAt = pod.recordedAt ?? new Date().toISOString();
  
  const photoPaths = pod.photos ? JSON.stringify(pod.photos) : null;

  await db.runAsync(
    `INSERT INTO proof_of_delivery (
      id, shipment_id, driver_id, recipient_name, recipient_phone,
      signature_data, photo_paths, delivery_address, delivery_lat, delivery_lng,
      delivery_notes, location_verified, location_mismatch_meters, recorded_at, synced_at, sync_status
    ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
    [
      id,
      pod.shipmentId,
      pod.driverId,
      pod.recipientName,
      pod.recipientPhone ?? null,
      pod.signatureData ?? null,
      photoPaths,
      pod.deliveryAddress ?? null,
      pod.deliveryLat ?? null,
      pod.deliveryLng ?? null,
      pod.deliveryNotes ?? null,
      pod.locationVerified ? 1 : 0,
      pod.locationMismatchMeters ?? null,
      recordedAt,
      null,
      'pending',
    ]
  );

  return id;
}

export async function getById(id: string): Promise<ProofOfDelivery | null> {
  const db = await getDatabase();
  const result = await db.getFirstAsync<Record<string, unknown>>(
    'SELECT * FROM proof_of_delivery WHERE id = ?',
    [id]
  );

  return result ? fromDBFormat(parsePODRow(result)) : null;
}

export async function getByShipmentId(shipmentId: string): Promise<ProofOfDelivery | null> {
  const db = await getDatabase();
  const result = await db.getFirstAsync<Record<string, unknown>>(
    'SELECT * FROM proof_of_delivery WHERE shipment_id = ?',
    [shipmentId]
  );

  return result ? fromDBFormat(parsePODRow(result)) : null;
}

export async function getPendingSync(): Promise<ProofOfDelivery[]> {
  const db = await getDatabase();
  const results = await db.getAllAsync<Record<string, unknown>>(
    "SELECT * FROM proof_of_delivery WHERE sync_status = 'pending' OR synced_at IS NULL"
  );

  return results.map((row) => fromDBFormat(parsePODRow(row)));
}

export async function markSynced(id: string, syncedAt: string): Promise<void> {
  const db = await getDatabase();
  await db.runAsync(
    "UPDATE proof_of_delivery SET synced_at = ?, sync_status = 'synced' WHERE id = ?",
    [syncedAt, id]
  );
}

export async function updateSyncStatus(
  id: string,
  status: 'pending' | 'syncing' | 'synced' | 'failed'
): Promise<void> {
  const db = await getDatabase();
  await db.runAsync(
    'UPDATE proof_of_delivery SET sync_status = ? WHERE id = ?',
    [status, id]
  );
}

export async function update(id: string, updates: Partial<ProofOfDelivery>): Promise<void> {
  const db = await getDatabase();

  const dbUpdates = toDBFormat(updates as ProofOfDelivery);
  const fields: string[] = [];
  const values: (string | number | null)[] = [];

  const updateableFields: (keyof ProofOfDeliveryDB)[] = [
    'recipient_name',
    'recipient_phone',
    'signature_data',
    'photo_paths',
    'delivery_address',
    'delivery_lat',
    'delivery_lng',
    'delivery_notes',
    'location_verified',
    'location_mismatch_meters',
    'synced_at',
    'sync_status',
  ];

  for (const key of updateableFields) {
    if (key in dbUpdates && dbUpdates[key] !== undefined) {
      fields.push(`${key} = ?`);
      values.push(dbUpdates[key] as string | number | null);
    }
  }

  if (fields.length === 0) return;

  values.push(id);

  await db.runAsync(
    `UPDATE proof_of_delivery SET ${fields.join(', ')} WHERE id = ?`,
    values
  );
}

export async function getAll(): Promise<ProofOfDelivery[]> {
  const db = await getDatabase();
  const results = await db.getAllAsync<Record<string, unknown>>(
    'SELECT * FROM proof_of_delivery ORDER BY recorded_at DESC'
  );

  return results.map((row) => fromDBFormat(parsePODRow(row)));
}

export async function getFailedSyncs(): Promise<ProofOfDelivery[]> {
  const db = await getDatabase();
  const results = await db.getAllAsync<Record<string, unknown>>(
    "SELECT * FROM proof_of_delivery WHERE sync_status = 'failed' ORDER BY recorded_at DESC"
  );

  return results.map((row) => fromDBFormat(parsePODRow(row)));
}

export async function clearOlderThan(date: Date): Promise<void> {
  const db = await getDatabase();
  await db.runAsync(
    'DELETE FROM proof_of_delivery WHERE recorded_at < ?',
    [date.toISOString()]
  );
}

export async function retrySync(id: string): Promise<void> {
  const db = await getDatabase();
  await db.runAsync(
    "UPDATE proof_of_delivery SET sync_status = 'pending', synced_at = NULL WHERE id = ?",
    [id]
  );
}

export { toDBFormat, fromDBFormat };

/**
 * Records a server rejection against a POD.
 *
 * synced_at stays NULL so the record is never mistaken for stored evidence,
 * while sync_status marks it failed for the UI and the attempt count lets the
 * caller give up on something the server will never accept.
 */
export async function markRejected(id: string, error: string): Promise<void> {
  const db = await getDatabase();
  await db.runAsync(
    "UPDATE proof_of_delivery SET attempts = attempts + 1, last_error = ?, sync_status = 'failed' WHERE id = ?",
    [error, id]
  );
}
