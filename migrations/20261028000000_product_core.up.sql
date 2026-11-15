-- +goose Up
-- +goose StatementBegin

-- ---------------------------------------------------------------------------
-- Tenants: company profile, operating region and notification preferences.
-- ---------------------------------------------------------------------------
ALTER TABLE tenants
    ADD COLUMN IF NOT EXISTS contact_phone TEXT,
    ADD COLUMN IF NOT EXISTS contact_email TEXT,
    ADD COLUMN IF NOT EXISTS region_city TEXT NOT NULL DEFAULT 'Addis Ababa',
    ADD COLUMN IF NOT EXISTS max_active_per_driver INTEGER NOT NULL DEFAULT 5,
    ADD COLUMN IF NOT EXISTS sms_on_created BOOLEAN NOT NULL DEFAULT true,
    ADD COLUMN IF NOT EXISTS sms_on_dispatched BOOLEAN NOT NULL DEFAULT true,
    ADD COLUMN IF NOT EXISTS sms_on_arrived BOOLEAN NOT NULL DEFAULT true,
    ADD COLUMN IF NOT EXISTS sms_on_delivered BOOLEAN NOT NULL DEFAULT true,
    ADD COLUMN IF NOT EXISTS sms_on_cancelled BOOLEAN NOT NULL DEFAULT true,
    ADD COLUMN IF NOT EXISTS sms_on_delayed BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS sms_driver_on_assign BOOLEAN NOT NULL DEFAULT true,
    ADD COLUMN IF NOT EXISTS trial_ends_at TIMESTAMPTZ;

UPDATE tenants SET max_drivers = 5 WHERE max_drivers IS NULL;
UPDATE tenants SET is_active = true WHERE is_active IS NULL;
ALTER TABLE tenants
    ALTER COLUMN max_drivers SET NOT NULL,
    ALTER COLUMN is_active SET NOT NULL;

-- ---------------------------------------------------------------------------
-- Users: a driver's default vehicle and login bookkeeping.
-- ---------------------------------------------------------------------------
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS vehicle_id UUID REFERENCES vehicles(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS last_login_at TIMESTAMPTZ;

UPDATE users SET is_active = true WHERE is_active IS NULL;
UPDATE users SET failed_login_attempts = 0 WHERE failed_login_attempts IS NULL;
UPDATE users SET password_reset_required = false WHERE password_reset_required IS NULL;
ALTER TABLE users
    ALTER COLUMN is_active SET NOT NULL,
    ALTER COLUMN failed_login_attempts SET NOT NULL,
    ALTER COLUMN password_reset_required SET NOT NULL;

ALTER TABLE vehicles ADD COLUMN IF NOT EXISTS capacity_kg DECIMAL(10, 2);
UPDATE vehicles SET is_active = true WHERE is_active IS NULL;
ALTER TABLE vehicles ALTER COLUMN is_active SET NOT NULL;

-- ---------------------------------------------------------------------------
-- Shipments.
--
-- Coordinates move from PostGIS geometry to plain lat/lng. Nothing ever wrote
-- the geometry columns, every consumer needs the numbers rather than WKB, and
-- distances are computed in Go. Any value that does exist is carried over.
--
-- origin_city/destination_city name a known place (internal/geo) and are what
-- reports group routes by; *_address keeps the street or landmark.
-- destination_precise records whether the dispatcher dropped a pin or the
-- coordinates are a city centre, which decides how strictly a delivery
-- location is checked against them.
-- ---------------------------------------------------------------------------
ALTER TABLE shipments
    ADD COLUMN IF NOT EXISTS priority TEXT NOT NULL DEFAULT 'normal'
        CHECK (priority IN ('normal', 'high', 'urgent')),
    ADD COLUMN IF NOT EXISTS origin_city TEXT,
    ADD COLUMN IF NOT EXISTS destination_city TEXT,
    ADD COLUMN IF NOT EXISTS origin_lat DOUBLE PRECISION,
    ADD COLUMN IF NOT EXISTS origin_lng DOUBLE PRECISION,
    ADD COLUMN IF NOT EXISTS destination_lat DOUBLE PRECISION,
    ADD COLUMN IF NOT EXISTS destination_lng DOUBLE PRECISION,
    ADD COLUMN IF NOT EXISTS destination_precise BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS picked_up_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS confirm_token TEXT UNIQUE,
    ADD COLUMN IF NOT EXISTS customer_confirmed_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS customer_feedback TEXT,
    ADD COLUMN IF NOT EXISTS eta_manual BOOLEAN NOT NULL DEFAULT false;

UPDATE shipments
SET origin_lat = ST_Y(origin_coordinates), origin_lng = ST_X(origin_coordinates)
WHERE origin_coordinates IS NOT NULL;

UPDATE shipments
SET destination_lat = ST_Y(destination_coordinates),
    destination_lng = ST_X(destination_coordinates),
    destination_precise = true
WHERE destination_coordinates IS NOT NULL;

DROP INDEX IF EXISTS idx_shipments_destination_coords;
ALTER TABLE shipments
    DROP COLUMN IF EXISTS origin_coordinates,
    DROP COLUMN IF EXISTS destination_coordinates;

UPDATE shipments SET created_at = NOW() WHERE created_at IS NULL;
UPDATE shipments SET updated_at = created_at WHERE updated_at IS NULL;
ALTER TABLE shipments
    ALTER COLUMN created_at SET NOT NULL,
    ALTER COLUMN updated_at SET NOT NULL;

CREATE INDEX IF NOT EXISTS idx_shipments_tenant_status ON shipments(tenant_id, status);
CREATE INDEX IF NOT EXISTS idx_shipments_tenant_created ON shipments(tenant_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_shipments_tenant_delivered ON shipments(tenant_id, actual_delivery)
    WHERE status = 'delivered';
CREATE INDEX IF NOT EXISTS idx_shipments_customer_phone ON shipments(customer_phone);

-- ---------------------------------------------------------------------------
-- Shipment history: every status change, assignment, edit, note, message and
-- customer action, in one append-only timeline (FR-008, FR-030, FR-035).
--
-- occurred_at is when it happened — device time, drift-corrected, for offline
-- updates — and created_at is when the server learned of it.
-- driver_id is the driver the event concerns: the new assignee, or the
-- recipient of a driver-visible note or message. previous_driver_id is set
-- when an assignment moves away from someone, which is how a driver's device
-- learns to drop a shipment that is no longer theirs.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS shipment_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    shipment_id UUID NOT NULL REFERENCES shipments(id) ON DELETE CASCADE,
    event_type TEXT NOT NULL CHECK (event_type IN (
        'created', 'edited', 'assigned', 'unassigned', 'status_changed',
        'note', 'message', 'pod_recorded', 'customer_confirmed',
        'customer_issue', 'conflict'
    )),
    from_status TEXT,
    to_status TEXT,
    driver_id UUID REFERENCES users(id) ON DELETE SET NULL,
    previous_driver_id UUID REFERENCES users(id) ON DELETE SET NULL,
    actor_id UUID REFERENCES users(id) ON DELETE SET NULL,
    actor_name TEXT NOT NULL DEFAULT '',
    source TEXT NOT NULL DEFAULT 'dashboard'
        CHECK (source IN ('dashboard', 'app', 'sms', 'system', 'customer', 'api')),
    note TEXT,
    reason TEXT,
    latitude DOUBLE PRECISION,
    longitude DOUBLE PRECISION,
    visible_to_driver BOOLEAN NOT NULL DEFAULT false,
    public BOOLEAN NOT NULL DEFAULT false,
    details JSONB,
    client_id TEXT,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- An offline device retries a batch whose response it never received; the
-- device's own identifier for the event makes the retry a no-op.
CREATE UNIQUE INDEX IF NOT EXISTS idx_shipment_events_client
    ON shipment_events(shipment_id, client_id) WHERE client_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_shipment_events_shipment ON shipment_events(shipment_id, occurred_at);
CREATE INDEX IF NOT EXISTS idx_shipment_events_tenant ON shipment_events(tenant_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_shipment_events_driver_feed ON shipment_events(driver_id, created_at)
    WHERE visible_to_driver;
CREATE INDEX IF NOT EXISTS idx_shipment_events_unassigned ON shipment_events(previous_driver_id, created_at)
    WHERE previous_driver_id IS NOT NULL;

-- Backfill a creation event so existing shipments have a timeline.
INSERT INTO shipment_events (tenant_id, shipment_id, event_type, to_status, actor_id, source, public, occurred_at, created_at)
SELECT s.tenant_id, s.id, 'created', 'pending', s.created_by, 'system', true, s.created_at, s.created_at
FROM shipments s
WHERE NOT EXISTS (SELECT 1 FROM shipment_events e WHERE e.shipment_id = s.id);

-- ---------------------------------------------------------------------------
-- Alerts: the dispatcher notification centre (FR-033).
--
-- At most one open alert per subject and type, so periodic checks such as
-- "driver offline" can re-run every minute without piling up duplicates.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS alerts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    shipment_id UUID REFERENCES shipments(id) ON DELETE CASCADE,
    driver_id UUID REFERENCES users(id) ON DELETE SET NULL,
    alert_type TEXT NOT NULL,
    severity TEXT NOT NULL DEFAULT 'warning' CHECK (severity IN ('info', 'warning', 'critical')),
    message TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    read_at TIMESTAMPTZ,
    acknowledged_at TIMESTAMPTZ,
    acknowledged_by UUID REFERENCES users(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_alerts_tenant_created ON alerts(tenant_id, created_at DESC);
CREATE UNIQUE INDEX IF NOT EXISTS idx_alerts_open_subject
    ON alerts(tenant_id, alert_type, COALESCE(shipment_id, driver_id))
    WHERE acknowledged_at IS NULL;

-- ---------------------------------------------------------------------------
-- SMS: an outbox for reliable delivery with retries, which doubles as the log
-- of every message sent and received (FR-024 AC-024.6, FR-037).
--
-- Messages carrying credentials or one-time codes are marked sensitive; their
-- body is redacted once sent and never shown in the dashboard.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS sms_messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID REFERENCES tenants(id) ON DELETE CASCADE,
    shipment_id UUID REFERENCES shipments(id) ON DELETE SET NULL,
    direction TEXT NOT NULL CHECK (direction IN ('outbound', 'inbound')),
    phone TEXT NOT NULL,
    body TEXT NOT NULL,
    purpose TEXT NOT NULL,
    sensitive BOOLEAN NOT NULL DEFAULT false,
    status TEXT NOT NULL DEFAULT 'queued'
        CHECK (status IN ('queued', 'sending', 'sent', 'failed', 'received', 'skipped')),
    attempts INTEGER NOT NULL DEFAULT 0,
    last_error TEXT,
    provider TEXT,
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    sent_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_sms_queue ON sms_messages(next_attempt_at) WHERE status = 'queued';
CREATE INDEX IF NOT EXISTS idx_sms_tenant ON sms_messages(tenant_id, created_at DESC);

CREATE TABLE IF NOT EXISTS sms_opt_outs (
    phone TEXT PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ---------------------------------------------------------------------------
-- One-time codes for driver PIN reset and customer "track by phone".
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS verification_codes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    phone TEXT NOT NULL,
    purpose TEXT NOT NULL CHECK (purpose IN ('pin_reset', 'track_lookup')),
    code_hash TEXT NOT NULL,
    attempts INTEGER NOT NULL DEFAULT 0,
    expires_at TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_verification_codes_lookup ON verification_codes(phone, purpose, created_at DESC);

-- ---------------------------------------------------------------------------
-- Sessions: browser sessions for the dashboard live alongside API sessions.
-- A web session's token hash is stored in refresh_token_hash; last_seen_at
-- drives the 8-hour inactivity timeout unless the user chose "remember me".
-- ---------------------------------------------------------------------------
ALTER TABLE sessions
    ADD COLUMN IF NOT EXISTS kind TEXT NOT NULL DEFAULT 'api' CHECK (kind IN ('api', 'web')),
    ADD COLUMN IF NOT EXISTS remember BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS last_seen_at TIMESTAMPTZ;

-- ---------------------------------------------------------------------------
-- Driver device state reported on every sync, for the dashboard's
-- synced / pending / offline indicator.
-- ---------------------------------------------------------------------------
ALTER TABLE sync_metadata
    ADD COLUMN IF NOT EXISTS device_id TEXT,
    ADD COLUMN IF NOT EXISTS app_version TEXT,
    ADD COLUMN IF NOT EXISTS battery_level INTEGER,
    ADD COLUMN IF NOT EXISTS clock_offset_seconds INTEGER,
    ADD COLUMN IF NOT EXISTS tracking_paused BOOLEAN NOT NULL DEFAULT false;

CREATE INDEX IF NOT EXISTS idx_tracking_driver_recorded ON tracking_events(tenant_id, driver_id, recorded_at DESC);
CREATE INDEX IF NOT EXISTS idx_tracking_shipment_recorded ON tracking_events(shipment_id, recorded_at);

-- Push notifications were never delivered and the driver app is now a PWA that
-- learns about changes when it syncs.
DROP TABLE IF EXISTS device_tokens CASCADE;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

CREATE TABLE IF NOT EXISTS device_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    device_id TEXT NOT NULL,
    push_token TEXT NOT NULL,
    platform TEXT NOT NULL CHECK (platform IN ('ios', 'android', 'web')),
    app_version TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(user_id, device_id)
);
CREATE INDEX IF NOT EXISTS idx_device_tokens_user ON device_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_device_tokens_device ON device_tokens(device_id);
CREATE INDEX IF NOT EXISTS idx_device_tokens_push_token ON device_tokens(push_token);
CREATE TRIGGER update_device_tokens_updated_at BEFORE UPDATE ON device_tokens
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

DROP INDEX IF EXISTS idx_tracking_shipment_recorded;
DROP INDEX IF EXISTS idx_tracking_driver_recorded;

ALTER TABLE sync_metadata
    DROP COLUMN IF EXISTS tracking_paused,
    DROP COLUMN IF EXISTS clock_offset_seconds,
    DROP COLUMN IF EXISTS battery_level,
    DROP COLUMN IF EXISTS app_version,
    DROP COLUMN IF EXISTS device_id;

ALTER TABLE sessions
    DROP COLUMN IF EXISTS last_seen_at,
    DROP COLUMN IF EXISTS remember,
    DROP COLUMN IF EXISTS kind;

DROP TABLE IF EXISTS verification_codes;
DROP TABLE IF EXISTS sms_opt_outs;
DROP TABLE IF EXISTS sms_messages;
DROP TABLE IF EXISTS alerts;
DROP TABLE IF EXISTS shipment_events;

DROP INDEX IF EXISTS idx_shipments_customer_phone;
DROP INDEX IF EXISTS idx_shipments_tenant_delivered;
DROP INDEX IF EXISTS idx_shipments_tenant_created;
DROP INDEX IF EXISTS idx_shipments_tenant_status;

ALTER TABLE shipments
    ADD COLUMN IF NOT EXISTS origin_coordinates GEOMETRY(POINT, 4326),
    ADD COLUMN IF NOT EXISTS destination_coordinates GEOMETRY(POINT, 4326);

UPDATE shipments SET origin_coordinates = ST_SetSRID(ST_MakePoint(origin_lng, origin_lat), 4326)
WHERE origin_lat IS NOT NULL AND origin_lng IS NOT NULL;
UPDATE shipments SET destination_coordinates = ST_SetSRID(ST_MakePoint(destination_lng, destination_lat), 4326)
WHERE destination_lat IS NOT NULL AND destination_lng IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_shipments_destination_coords ON shipments USING GIST(destination_coordinates);

ALTER TABLE shipments
    DROP COLUMN IF EXISTS eta_manual,
    DROP COLUMN IF EXISTS customer_feedback,
    DROP COLUMN IF EXISTS customer_confirmed_at,
    DROP COLUMN IF EXISTS confirm_token,
    DROP COLUMN IF EXISTS picked_up_at,
    DROP COLUMN IF EXISTS destination_precise,
    DROP COLUMN IF EXISTS destination_lng,
    DROP COLUMN IF EXISTS destination_lat,
    DROP COLUMN IF EXISTS origin_lng,
    DROP COLUMN IF EXISTS origin_lat,
    DROP COLUMN IF EXISTS destination_city,
    DROP COLUMN IF EXISTS origin_city,
    DROP COLUMN IF EXISTS priority;

ALTER TABLE vehicles DROP COLUMN IF EXISTS capacity_kg;

ALTER TABLE users
    DROP COLUMN IF EXISTS last_login_at,
    DROP COLUMN IF EXISTS vehicle_id;

ALTER TABLE tenants
    DROP COLUMN IF EXISTS trial_ends_at,
    DROP COLUMN IF EXISTS sms_driver_on_assign,
    DROP COLUMN IF EXISTS sms_on_delayed,
    DROP COLUMN IF EXISTS sms_on_cancelled,
    DROP COLUMN IF EXISTS sms_on_delivered,
    DROP COLUMN IF EXISTS sms_on_arrived,
    DROP COLUMN IF EXISTS sms_on_dispatched,
    DROP COLUMN IF EXISTS sms_on_created,
    DROP COLUMN IF EXISTS max_active_per_driver,
    DROP COLUMN IF EXISTS region_city,
    DROP COLUMN IF EXISTS contact_email,
    DROP COLUMN IF EXISTS contact_phone;

-- +goose StatementEnd
