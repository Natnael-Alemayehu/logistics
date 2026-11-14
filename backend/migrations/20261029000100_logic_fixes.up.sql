-- +goose Up
-- +goose StatementBegin

-- ---------------------------------------------------------------------------
-- Alerts raised by the periodic checks carry an episode: the ETA a "past
-- ETA" alert is about, or when the driver was last heard from. A dismissed
-- alert then stays dismissed until the situation changes, rather than coming
-- back on the next run a minute later.
-- ---------------------------------------------------------------------------
ALTER TABLE alerts ADD COLUMN IF NOT EXISTS episode TEXT;

CREATE UNIQUE INDEX IF NOT EXISTS idx_alerts_episode
    ON alerts(tenant_id, alert_type, COALESCE(shipment_id, driver_id), episode)
    WHERE episode IS NOT NULL;

-- ---------------------------------------------------------------------------
-- When the load is to be collected. The booking ETA counts from it, and a
-- load not on its way two hours after it raises an alert.
-- ---------------------------------------------------------------------------
ALTER TABLE shipments ADD COLUMN IF NOT EXISTS planned_pickup_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_shipments_pickup_due ON shipments(planned_pickup_at)
    WHERE status IN ('pending', 'assigned') AND planned_pickup_at IS NOT NULL;

-- ---------------------------------------------------------------------------
-- A proof of delivery that arrives for a shipment changed while the driver
-- was offline (cancelled, given to another driver, or already delivered) is
-- held for the office to accept or dismiss, instead of being lost. A
-- shipment has at most one accepted proof. The device's identifier for a
-- proof makes a replayed upload harmless.
-- ---------------------------------------------------------------------------
ALTER TABLE proof_of_deliveries DROP CONSTRAINT IF EXISTS proof_of_deliveries_shipment_id_key;

ALTER TABLE proof_of_deliveries
    ADD COLUMN IF NOT EXISTS client_id TEXT,
    ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'accepted'
        CHECK (status IN ('accepted', 'held', 'dismissed')),
    ADD COLUMN IF NOT EXISTS held_reason TEXT,
    ADD COLUMN IF NOT EXISTS reviewed_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS reviewed_by UUID REFERENCES users(id) ON DELETE SET NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_pod_accepted ON proof_of_deliveries(shipment_id) WHERE status = 'accepted';
CREATE UNIQUE INDEX IF NOT EXISTS idx_pod_client ON proof_of_deliveries(driver_id, client_id) WHERE client_id IS NOT NULL;

-- ---------------------------------------------------------------------------
-- New timeline entries: a repeated delay or problem report with a new cause,
-- and the life of a held proof of delivery.
-- ---------------------------------------------------------------------------
ALTER TABLE shipment_events DROP CONSTRAINT IF EXISTS shipment_events_event_type_check;
ALTER TABLE shipment_events ADD CONSTRAINT shipment_events_event_type_check CHECK (event_type IN (
    'created', 'edited', 'assigned', 'unassigned', 'status_changed', 'status_updated',
    'note', 'message', 'pod_recorded', 'pod_held', 'pod_accepted', 'pod_dismissed',
    'customer_confirmed', 'customer_issue', 'conflict'
));

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE shipment_events DROP CONSTRAINT IF EXISTS shipment_events_event_type_check;
DELETE FROM shipment_events WHERE event_type IN ('status_updated', 'pod_held', 'pod_accepted', 'pod_dismissed');
ALTER TABLE shipment_events ADD CONSTRAINT shipment_events_event_type_check CHECK (event_type IN (
    'created', 'edited', 'assigned', 'unassigned', 'status_changed',
    'note', 'message', 'pod_recorded', 'customer_confirmed',
    'customer_issue', 'conflict'
));
DELETE FROM proof_of_deliveries WHERE status <> 'accepted';
DROP INDEX IF EXISTS idx_pod_client;
DROP INDEX IF EXISTS idx_pod_accepted;
ALTER TABLE proof_of_deliveries
    DROP COLUMN IF EXISTS reviewed_by,
    DROP COLUMN IF EXISTS reviewed_at,
    DROP COLUMN IF EXISTS held_reason,
    DROP COLUMN IF EXISTS status,
    DROP COLUMN IF EXISTS client_id;
ALTER TABLE proof_of_deliveries ADD CONSTRAINT proof_of_deliveries_shipment_id_key UNIQUE (shipment_id);
DROP INDEX IF EXISTS idx_shipments_pickup_due;
ALTER TABLE shipments DROP COLUMN IF EXISTS planned_pickup_at;
DROP INDEX IF EXISTS idx_alerts_episode;
ALTER TABLE alerts DROP COLUMN IF EXISTS episode;
-- +goose StatementEnd
