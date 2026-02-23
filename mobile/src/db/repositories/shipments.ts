import { getDatabase } from '../database';

export interface Shipment {
  id: string;
  tracking_number: string;
  origin_address: string;
  origin_lat?: number;
  origin_lng?: number;
  destination_address: string;
  destination_lat?: number;
  destination_lng?: number;
  customer_name: string;
  customer_phone: string;
  cargo_description?: string;
  status: string;
  driver_id?: string;
  vehicle_id?: string;
  estimated_delivery?: string;
  special_instructions?: string;
  synced_at?: string;
  created_at: string;
  updated_at: string;
}

export async function getAll(): Promise<Shipment[]> {
  const db = await getDatabase();
  return await db.getAllAsync<Shipment>(
    'SELECT * FROM shipments ORDER BY created_at DESC'
  );
}

export async function getById(id: string): Promise<Shipment | null> {
  const db = await getDatabase();
  return await db.getFirstAsync<Shipment>(
    'SELECT * FROM shipments WHERE id = ?',
    [id]
  );
}

export async function getByStatus(status: string): Promise<Shipment[]> {
  const db = await getDatabase();
  return await db.getAllAsync<Shipment>(
    'SELECT * FROM shipments WHERE status = ? ORDER BY created_at DESC',
    [status]
  );
}

export async function upsert(shipment: Shipment): Promise<void> {
  const db = await getDatabase();
  await db.runAsync(
    `INSERT OR REPLACE INTO shipments (
      id, tracking_number, origin_address, origin_lat, origin_lng,
      destination_address, destination_lat, destination_lng, customer_name,
      customer_phone, cargo_description, status, driver_id, vehicle_id,
      estimated_delivery, special_instructions, synced_at, created_at, updated_at
    ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
    [
      shipment.id,
      shipment.tracking_number,
      shipment.origin_address,
      shipment.origin_lat ?? null,
      shipment.origin_lng ?? null,
      shipment.destination_address,
      shipment.destination_lat ?? null,
      shipment.destination_lng ?? null,
      shipment.customer_name,
      shipment.customer_phone,
      shipment.cargo_description ?? null,
      shipment.status,
      shipment.driver_id ?? null,
      shipment.vehicle_id ?? null,
      shipment.estimated_delivery ?? null,
      shipment.special_instructions ?? null,
      shipment.synced_at ?? null,
      shipment.created_at,
      shipment.updated_at,
    ]
  );
}

export async function upsertMany(shipments: Shipment[]): Promise<void> {
  const db = await getDatabase();
  
  await db.withTransactionAsync(async () => {
    for (const shipment of shipments) {
      await upsert(shipment);
    }
  });
}

export async function update(id: string, updates: Partial<Shipment>): Promise<void> {
  const db = await getDatabase();
  
  const fields: string[] = [];
  const values: (string | number | null)[] = [];
  
  for (const [key, value] of Object.entries(updates)) {
    fields.push(`${key} = ?`);
    values.push(value ?? null);
  }
  
  fields.push('updated_at = ?');
  values.push(new Date().toISOString());
  values.push(id);
  
  await db.runAsync(
    `UPDATE shipments SET ${fields.join(', ')} WHERE id = ?`,
    values
  );
}

export async function deleteShipment(id: string): Promise<void> {
  const db = await getDatabase();
  await db.runAsync('DELETE FROM shipments WHERE id = ?', [id]);
}

export async function clearAll(): Promise<void> {
  const db = await getDatabase();
  await db.runAsync('DELETE FROM shipments');
}
