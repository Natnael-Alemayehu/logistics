export const SCHEMA_VERSION = 2;

export const CREATE_SHIPMENTS_TABLE = `
CREATE TABLE IF NOT EXISTS shipments (
  id TEXT PRIMARY KEY,
  tenant_id TEXT,
  tracking_number TEXT UNIQUE NOT NULL,
  origin_address TEXT NOT NULL,
  origin_lat REAL,
  origin_lng REAL,
  destination_address TEXT NOT NULL,
  destination_lat REAL,
  destination_lng REAL,
  customer_name TEXT NOT NULL,
  customer_phone TEXT NOT NULL,
  cargo_description TEXT,
  cargo_weight REAL,
  cargo_value REAL,
  status TEXT NOT NULL,
  status_note TEXT,
  status_reason TEXT,
  driver_id TEXT,
  vehicle_id TEXT,
  estimated_delivery TEXT,
  actual_delivery TEXT,
  special_instructions TEXT,
  created_by TEXT,
  synced_at TEXT,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
`;

export const CREATE_TRACKING_EVENTS_TABLE = `
CREATE TABLE IF NOT EXISTS tracking_events (
  id TEXT PRIMARY KEY,
  shipment_id TEXT NOT NULL,
  driver_id TEXT NOT NULL,
  latitude REAL NOT NULL,
  longitude REAL NOT NULL,
  accuracy REAL,
  speed REAL,
  heading REAL,
  event_type TEXT NOT NULL,
  status TEXT,
  note TEXT,
  recorded_at TEXT NOT NULL,
  synced_at TEXT,
  device_id TEXT,
  battery_level INTEGER,
  sync_priority INTEGER DEFAULT 0,
  geofence_id TEXT,
  geofence_type TEXT,
  attempts INTEGER NOT NULL DEFAULT 0,
  last_error TEXT
);
`;

export const CREATE_PROOF_OF_DELIVERY_TABLE = `
CREATE TABLE IF NOT EXISTS proof_of_delivery (
  id TEXT PRIMARY KEY,
  shipment_id TEXT UNIQUE NOT NULL,
  driver_id TEXT NOT NULL,
  recipient_name TEXT NOT NULL,
  recipient_phone TEXT,
  signature_data TEXT,
  photo_paths TEXT,
  delivery_address TEXT,
  delivery_lat REAL,
  delivery_lng REAL,
  delivery_notes TEXT,
  location_verified INTEGER DEFAULT 0,
  location_mismatch_meters INTEGER,
  recorded_at TEXT NOT NULL,
  synced_at TEXT,
  sync_status TEXT DEFAULT 'pending',
  attempts INTEGER NOT NULL DEFAULT 0,
  last_error TEXT
);
`;

export const CREATE_SYNC_QUEUE_TABLE = `
CREATE TABLE IF NOT EXISTS sync_queue (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  entity_type TEXT NOT NULL,
  entity_id TEXT NOT NULL,
  operation TEXT NOT NULL,
  priority INTEGER DEFAULT 0,
  attempts INTEGER DEFAULT 0,
  last_error TEXT,
  created_at TEXT NOT NULL,
  retry_at TEXT,
  status TEXT DEFAULT 'pending',
  synced_at TEXT
);
`;

export const CREATE_DEAD_LETTER_QUEUE_TABLE = `
CREATE TABLE IF NOT EXISTS dead_letter_queue (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  entity_type TEXT NOT NULL,
  entity_id TEXT NOT NULL,
  operation TEXT NOT NULL,
  priority INTEGER DEFAULT 0,
  attempts INTEGER NOT NULL,
  last_error TEXT NOT NULL,
  created_at TEXT NOT NULL,
  moved_to_dead_letter_at TEXT NOT NULL,
  original_queue_id INTEGER NOT NULL
);
`;

export const CREATE_SYNC_METADATA_TABLE = `
CREATE TABLE IF NOT EXISTS sync_metadata (
  id INTEGER PRIMARY KEY CHECK (id = 1),
  driver_id TEXT NOT NULL,
  last_sync_at TEXT,
  last_event_id TEXT,
  pending_count INTEGER DEFAULT 0,
  storage_used_kb INTEGER DEFAULT 0,
  device_id TEXT
);
`;

export const CREATE_STATUS_UPDATES_TABLE = `
CREATE TABLE IF NOT EXISTS status_updates (
  id TEXT PRIMARY KEY,
  shipment_id TEXT NOT NULL,
  status TEXT NOT NULL,
  note TEXT,
  reason TEXT,
  recorded_at TEXT NOT NULL,
  synced_at TEXT,
  attempts INTEGER NOT NULL DEFAULT 0,
  last_error TEXT
);
`;

export const CREATE_INDEXES = `
CREATE INDEX IF NOT EXISTS idx_shipments_status ON shipments(status);
CREATE INDEX IF NOT EXISTS idx_shipments_driver ON shipments(driver_id);
CREATE INDEX IF NOT EXISTS idx_shipments_tracking ON shipments(tracking_number);
CREATE INDEX IF NOT EXISTS idx_tracking_shipment ON tracking_events(shipment_id);
CREATE INDEX IF NOT EXISTS idx_tracking_synced ON tracking_events(synced_at);
CREATE INDEX IF NOT EXISTS idx_pod_shipment ON proof_of_delivery(shipment_id);
CREATE INDEX IF NOT EXISTS idx_pod_sync_status ON proof_of_delivery(sync_status);
CREATE INDEX IF NOT EXISTS idx_sync_queue_priority ON sync_queue(priority DESC);
CREATE INDEX IF NOT EXISTS idx_sync_queue_entity ON sync_queue(entity_type, entity_id);
CREATE INDEX IF NOT EXISTS idx_sync_queue_status ON sync_queue(status);
CREATE INDEX IF NOT EXISTS idx_sync_queue_retry ON sync_queue(retry_at);
CREATE INDEX IF NOT EXISTS idx_dead_letter_created ON dead_letter_queue(created_at);
CREATE INDEX IF NOT EXISTS idx_status_updates_shipment ON status_updates(shipment_id);
CREATE INDEX IF NOT EXISTS idx_status_updates_synced ON status_updates(synced_at);
`;

export const ALL_SCHEMA = [
  CREATE_SHIPMENTS_TABLE,
  CREATE_TRACKING_EVENTS_TABLE,
  CREATE_PROOF_OF_DELIVERY_TABLE,
  CREATE_STATUS_UPDATES_TABLE,
  CREATE_SYNC_QUEUE_TABLE,
  CREATE_DEAD_LETTER_QUEUE_TABLE,
  CREATE_SYNC_METADATA_TABLE,
  CREATE_INDEXES,
];

/**
 * Columns added after the initial release, keyed by the schema version that
 * introduces them.
 *
 * SQLite has no ADD COLUMN IF NOT EXISTS, and installs that predate
 * SCHEMA_VERSION tracking report user_version 0 despite already having the v1
 * tables. The migration runner therefore diffs against PRAGMA table_info rather
 * than trusting the recorded version, which makes each step idempotent and safe
 * to re-run.
 */
export const ADDED_COLUMNS: Record<number, { table: string; column: string; definition: string }[]> = {
  2: [
    // Written by shipmentsRepo.upsert since day one, but never created here, so
    // every pull threw "no such column" and aborted the sync.
    { table: 'shipments', column: 'tenant_id', definition: 'TEXT' },
    { table: 'shipments', column: 'cargo_weight', definition: 'REAL' },
    { table: 'shipments', column: 'cargo_value', definition: 'REAL' },
    { table: 'shipments', column: 'status_note', definition: 'TEXT' },
    { table: 'shipments', column: 'status_reason', definition: 'TEXT' },
    { table: 'shipments', column: 'actual_delivery', definition: 'TEXT' },
    { table: 'shipments', column: 'created_by', definition: 'TEXT' },

    // Per-row retry state, so a rejected record can be distinguished from one
    // that has simply not been pushed yet.
    { table: 'tracking_events', column: 'attempts', definition: 'INTEGER NOT NULL DEFAULT 0' },
    { table: 'tracking_events', column: 'last_error', definition: 'TEXT' },
    { table: 'proof_of_delivery', column: 'attempts', definition: 'INTEGER NOT NULL DEFAULT 0' },
    { table: 'proof_of_delivery', column: 'last_error', definition: 'TEXT' },
    { table: 'status_updates', column: 'attempts', definition: 'INTEGER NOT NULL DEFAULT 0' },
    { table: 'status_updates', column: 'last_error', definition: 'TEXT' },
  ],
};
