-- +goose Up
-- +goose StatementBegin

-- ---------------------------------------------------------------------------
-- Driver app settings (FR-026 AC-026.6), chosen on the phone and kept here so
-- a new phone starts the way the driver left the last one. language stays
-- NULL until the driver picks one.
-- ---------------------------------------------------------------------------
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS language TEXT CHECK (language IN ('en', 'am')),
    ADD COLUMN IF NOT EXISTS app_battery_saver BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS app_vibrate BOOLEAN NOT NULL DEFAULT true;

-- Whether the phone is saving battery right now, by choice or because the
-- battery is low. It then records a position every 15 minutes (FR-019).
ALTER TABLE sync_metadata
    ADD COLUMN IF NOT EXISTS power_saving BOOLEAN NOT NULL DEFAULT false;

-- Why the driver recorded a delivery away from the destination (AC-022.2).
ALTER TABLE proof_of_deliveries
    ADD COLUMN IF NOT EXISTS location_reason TEXT;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE proof_of_deliveries DROP COLUMN IF EXISTS location_reason;
ALTER TABLE sync_metadata DROP COLUMN IF EXISTS power_saving;
ALTER TABLE users
    DROP COLUMN IF EXISTS app_vibrate,
    DROP COLUMN IF EXISTS app_battery_saver,
    DROP COLUMN IF EXISTS language;
-- +goose StatementEnd
