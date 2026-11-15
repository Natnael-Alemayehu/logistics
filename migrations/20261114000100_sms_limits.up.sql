-- +goose Up
-- +goose StatementBegin

-- ---------------------------------------------------------------------------
-- How many SMS parts a text goes as: 160 characters in one, or 153 to a part
-- when longer, and 70 and 67 when it needs Unicode, as Amharic does. Quotas,
-- limits and invoices count parts, which is what the provider charges for.
-- ---------------------------------------------------------------------------
ALTER TABLE sms_messages
    ADD COLUMN IF NOT EXISTS segments INTEGER NOT NULL DEFAULT 1 CHECK (segments > 0);

-- ---------------------------------------------------------------------------
-- Inbound texts carry the provider's ID too, so a webhook the provider sends
-- again is handled once. IDs are unique per provider and direction.
-- ---------------------------------------------------------------------------
DROP INDEX IF EXISTS idx_sms_provider_message;
CREATE UNIQUE INDEX IF NOT EXISTS idx_sms_provider_message
    ON sms_messages(provider, direction, provider_message_id) WHERE provider_message_id IS NOT NULL;

-- Codes texted across the platform in the last day, for the daily budget,
-- and texts to one number, for the limit on replies.
CREATE INDEX IF NOT EXISTS idx_sms_otp_created ON sms_messages(created_at) WHERE purpose = 'otp';
CREATE INDEX IF NOT EXISTS idx_sms_phone_created ON sms_messages(phone, created_at DESC);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_sms_phone_created;
DROP INDEX IF EXISTS idx_sms_otp_created;
DROP INDEX IF EXISTS idx_sms_provider_message;
CREATE UNIQUE INDEX IF NOT EXISTS idx_sms_provider_message
    ON sms_messages(provider, provider_message_id) WHERE provider_message_id IS NOT NULL;
ALTER TABLE sms_messages DROP COLUMN IF EXISTS segments;
-- +goose StatementEnd
