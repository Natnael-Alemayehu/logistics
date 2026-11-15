-- +goose Up
-- +goose StatementBegin

-- Offline clients retry any batch whose response was lost, and tracking_events
-- had no natural key, so a retry silently inserted duplicate telemetry. The
-- client's own row identifier, scoped to the device that produced it, gives the
-- insert something to conflict on.
--
-- The index is partial so that clients which do not send a client_id are
-- unaffected: NULLs are excluded rather than colliding with one another.
ALTER TABLE tracking_events ADD COLUMN IF NOT EXISTS client_id TEXT;

CREATE UNIQUE INDEX IF NOT EXISTS idx_tracking_events_device_client
    ON tracking_events(device_id, client_id)
    WHERE client_id IS NOT NULL AND device_id IS NOT NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_tracking_events_device_client;
ALTER TABLE tracking_events DROP COLUMN IF EXISTS client_id;

-- +goose StatementEnd
