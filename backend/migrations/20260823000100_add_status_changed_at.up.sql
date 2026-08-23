-- +goose Up
-- +goose StatementBegin

-- Offline drivers queue status updates for hours, and applying one that the
-- server has already moved past would silently roll the shipment backwards.
-- Detecting that needs the time of the last *status* change specifically:
-- updated_at is bumped by any modification at all — creation, driver
-- assignment, a dispatcher editing the address — so comparing against it would
-- reject perfectly good updates from a driver who observed the change earlier.
--
-- Existing rows are backfilled from updated_at, which is the best available
-- approximation, and NULL is treated as "unknown, accept the update".
ALTER TABLE shipments ADD COLUMN IF NOT EXISTS status_changed_at TIMESTAMPTZ;

UPDATE shipments SET status_changed_at = updated_at WHERE status_changed_at IS NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE shipments DROP COLUMN IF EXISTS status_changed_at;

-- +goose StatementEnd
