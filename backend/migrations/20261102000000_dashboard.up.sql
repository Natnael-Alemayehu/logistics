-- +goose Up
-- +goose StatementBegin

-- ---------------------------------------------------------------------------
-- A dispatcher's saved views of the shipment list (FR-009 AC-009.6): a name
-- and the filter's query string, shown as chips above the list.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS saved_filters (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL CHECK (length(name) BETWEEN 1 AND 40),
    query TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, name)
);

CREATE INDEX IF NOT EXISTS idx_saved_filters_user ON saved_filters(tenant_id, user_id);

-- ---------------------------------------------------------------------------
-- One message to every driver on the road (FR-035 AC-035.4). It belongs to
-- no shipment, so it has its own table; the driver app gets it with its
-- other messages, and a driver whose app is quiet gets it by SMS.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS broadcasts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    body TEXT NOT NULL,
    sent_by UUID REFERENCES users(id) ON DELETE SET NULL,
    sender_name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_broadcasts_tenant ON broadcasts(tenant_id, created_at DESC);

CREATE TABLE IF NOT EXISTS broadcast_recipients (
    broadcast_id UUID NOT NULL REFERENCES broadcasts(id) ON DELETE CASCADE,
    driver_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    by_sms BOOLEAN NOT NULL DEFAULT false,
    PRIMARY KEY (broadcast_id, driver_id)
);

CREATE INDEX IF NOT EXISTS idx_broadcast_recipients_driver ON broadcast_recipients(driver_id);

-- ---------------------------------------------------------------------------
-- Alert preferences (FR-033): a company can turn off the alert raised for
-- each delivery, and each person chooses which alert types are also
-- emailed to them.
-- ---------------------------------------------------------------------------
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS alert_on_delivery BOOLEAN NOT NULL DEFAULT true;
ALTER TABLE users ADD COLUMN IF NOT EXISTS alert_emails TEXT[] NOT NULL DEFAULT '{}';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE users DROP COLUMN IF EXISTS alert_emails;
ALTER TABLE tenants DROP COLUMN IF EXISTS alert_on_delivery;
DROP TABLE IF EXISTS broadcast_recipients;
DROP TABLE IF EXISTS broadcasts;
DROP TABLE IF EXISTS saved_filters;
-- +goose StatementEnd
