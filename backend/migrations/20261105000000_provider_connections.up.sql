-- +goose Up
-- +goose StatementBegin

-- ---------------------------------------------------------------------------
-- Delivery reports: the provider's ID for each text it accepted, and what it
-- later reported about the phone receiving it. The status stays 'sent': the
-- text was handed over, which is what quotas and invoices count.
-- ---------------------------------------------------------------------------
ALTER TABLE sms_messages
    ADD COLUMN IF NOT EXISTS provider_message_id TEXT,
    ADD COLUMN IF NOT EXISTS delivery TEXT CHECK (delivery IN ('delivered', 'undelivered')),
    ADD COLUMN IF NOT EXISTS delivery_at TIMESTAMPTZ;

CREATE UNIQUE INDEX IF NOT EXISTS idx_sms_provider_message
    ON sms_messages(provider, provider_message_id) WHERE provider_message_id IS NOT NULL;

-- ---------------------------------------------------------------------------
-- When the provider was last asked how a payment ended, for payments whose
-- notification never arrived.
-- ---------------------------------------------------------------------------
ALTER TABLE payments ADD COLUMN IF NOT EXISTS checked_at TIMESTAMPTZ;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE payments DROP COLUMN IF EXISTS checked_at;
DROP INDEX IF EXISTS idx_sms_provider_message;
ALTER TABLE sms_messages
    DROP COLUMN IF EXISTS delivery_at,
    DROP COLUMN IF EXISTS delivery,
    DROP COLUMN IF EXISTS provider_message_id;
-- +goose StatementEnd
