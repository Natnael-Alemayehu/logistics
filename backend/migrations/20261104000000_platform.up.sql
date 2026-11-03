-- +goose Up
-- +goose StatementBegin

-- ---------------------------------------------------------------------------
-- The audit log and the shipment timeline are append-only (FR-042 AC-042.5).
-- Only a transaction that sets menged.purge may change or remove rows:
-- deleting a company, and removing personal data after the retention period
-- or on request (FR-041).
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION append_only() RETURNS trigger AS $$
BEGIN
    IF current_setting('menged.purge', true) = 'on' THEN
        IF TG_OP = 'DELETE' THEN
            RETURN OLD;
        END IF;
        RETURN NEW;
    END IF;
    RAISE EXCEPTION '% is append-only', TG_TABLE_NAME USING ERRCODE = 'insufficient_privilege';
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER audit_logs_append_only BEFORE UPDATE OR DELETE ON audit_logs
    FOR EACH ROW EXECUTE FUNCTION append_only();
CREATE TRIGGER audit_logs_no_truncate BEFORE TRUNCATE ON audit_logs
    FOR EACH STATEMENT EXECUTE FUNCTION append_only();
CREATE TRIGGER shipment_events_append_only BEFORE UPDATE OR DELETE ON shipment_events
    FOR EACH ROW EXECUTE FUNCTION append_only();
CREATE TRIGGER shipment_events_no_truncate BEFORE TRUNCATE ON shipment_events
    FOR EACH STATEMENT EXECUTE FUNCTION append_only();

-- The platform's audit view lists every company's actions, newest first.
CREATE INDEX IF NOT EXISTS idx_audit_created_desc ON audit_logs (created_at DESC);

-- ---------------------------------------------------------------------------
-- Personal data removed: shipments a year after they finished (customer and
-- recipient details, signatures and photos), and people on request.
-- ---------------------------------------------------------------------------
ALTER TABLE shipments ADD COLUMN IF NOT EXISTS anonymised_at TIMESTAMPTZ;
CREATE INDEX IF NOT EXISTS idx_shipments_to_anonymise ON shipments (updated_at)
    WHERE anonymised_at IS NULL AND status IN ('delivered', 'cancelled');
ALTER TABLE users ADD COLUMN IF NOT EXISTS anonymised_at TIMESTAMPTZ;

-- ---------------------------------------------------------------------------
-- Support tickets between a company and the people running Menged
-- (FR-049 AC-049.5). open: waiting for the platform; answered: waiting for
-- the company.
-- ---------------------------------------------------------------------------
CREATE TABLE support_tickets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    number BIGINT NOT NULL GENERATED ALWAYS AS IDENTITY (START WITH 1001) UNIQUE,
    subject TEXT NOT NULL CHECK (length(subject) BETWEEN 1 AND 200),
    status TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'answered', 'closed')),
    opened_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    closed_at TIMESTAMPTZ
);
CREATE INDEX idx_support_tickets_tenant ON support_tickets (tenant_id, updated_at DESC);
CREATE INDEX idx_support_tickets_status ON support_tickets (status, updated_at DESC);

CREATE TABLE support_messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ticket_id UUID NOT NULL REFERENCES support_tickets(id) ON DELETE CASCADE,
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    author_id UUID REFERENCES users(id) ON DELETE SET NULL,
    author_name TEXT NOT NULL,
    from_platform BOOLEAN NOT NULL DEFAULT false,
    body TEXT NOT NULL CHECK (length(body) BETWEEN 1 AND 5000),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_support_messages_ticket ON support_messages (ticket_id, created_at);

-- ---------------------------------------------------------------------------
-- The background jobs' latest runs, for the health panel (AC-049.6). One row
-- per job, so every replica sees the same.
-- ---------------------------------------------------------------------------
CREATE TABLE job_runs (
    name TEXT PRIMARY KEY,
    started_at TIMESTAMPTZ NOT NULL,
    duration_ms INTEGER NOT NULL,
    error TEXT,
    last_ok_at TIMESTAMPTZ,
    runs BIGINT NOT NULL DEFAULT 1,
    failures BIGINT NOT NULL DEFAULT 0
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS job_runs;
DROP TABLE IF EXISTS support_messages;
DROP TABLE IF EXISTS support_tickets;
ALTER TABLE users DROP COLUMN IF EXISTS anonymised_at;
DROP INDEX IF EXISTS idx_shipments_to_anonymise;
ALTER TABLE shipments DROP COLUMN IF EXISTS anonymised_at;
DROP INDEX IF EXISTS idx_audit_created_desc;
DROP TRIGGER IF EXISTS shipment_events_no_truncate ON shipment_events;
DROP TRIGGER IF EXISTS shipment_events_append_only ON shipment_events;
DROP TRIGGER IF EXISTS audit_logs_no_truncate ON audit_logs;
DROP TRIGGER IF EXISTS audit_logs_append_only ON audit_logs;
DROP FUNCTION IF EXISTS append_only();
-- +goose StatementEnd
