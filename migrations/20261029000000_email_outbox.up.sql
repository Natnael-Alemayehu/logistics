-- +goose Up
-- +goose StatementBegin

-- ---------------------------------------------------------------------------
-- Email outbox: written in the same transaction as the change that causes a
-- message, delivered by a background loop with retries, like SMS.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS emails (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID REFERENCES tenants(id) ON DELETE CASCADE,
    to_address TEXT NOT NULL,
    subject TEXT NOT NULL,
    body TEXT NOT NULL,
    purpose TEXT NOT NULL,
    -- Sensitive messages carry a link that resets a password or proves
    -- ownership of an address; their body is redacted once sent.
    sensitive BOOLEAN NOT NULL DEFAULT false,
    status TEXT NOT NULL DEFAULT 'queued'
        CHECK (status IN ('queued', 'sending', 'sent', 'failed')),
    attempts INTEGER NOT NULL DEFAULT 0,
    last_error TEXT,
    provider TEXT,
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    sent_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_emails_queue ON emails(next_attempt_at) WHERE status = 'queued';
CREATE INDEX IF NOT EXISTS idx_emails_tenant ON emails(tenant_id, created_at DESC);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS emails;
-- +goose StatementEnd
