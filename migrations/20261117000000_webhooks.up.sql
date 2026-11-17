-- +goose Up
-- Webhooks (FR-047): addresses a company's own systems are told at when
-- its shipments change, and every message sent to them.

-- secret signs each message. It is kept as it is, since signing needs it,
-- and shown only to the company's admins.
CREATE TABLE IF NOT EXISTS webhook_endpoints (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    url text NOT NULL,
    events text[] NOT NULL,
    secret text NOT NULL,
    active boolean DEFAULT true NOT NULL,
    created_by uuid,
    created_at TIMESTAMPTZ DEFAULT now() NOT NULL,
    CONSTRAINT webhook_endpoints_pkey PRIMARY KEY (id),
    CONSTRAINT webhook_endpoints_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
    CONSTRAINT webhook_endpoints_created_by_fkey FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL
);
CREATE INDEX IF NOT EXISTS idx_webhook_endpoints_tenant ON webhook_endpoints (tenant_id);

-- One message to one endpoint, an outbox like emails'. event_id is the
-- same for every endpoint told of one event, so a receiver can tell a
-- message sent again from a new one.
CREATE TABLE IF NOT EXISTS webhook_deliveries (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    endpoint_id uuid NOT NULL,
    event_id uuid NOT NULL,
    event_type text NOT NULL,
    shipment_id uuid,
    payload jsonb NOT NULL,
    status text DEFAULT 'queued' NOT NULL,
    attempts integer DEFAULT 0 NOT NULL,
    next_attempt_at TIMESTAMPTZ DEFAULT now() NOT NULL,
    response_status integer,
    response_body text,
    last_error text,
    duration_ms integer,
    created_at TIMESTAMPTZ DEFAULT now() NOT NULL,
    delivered_at TIMESTAMPTZ,
    CONSTRAINT webhook_deliveries_pkey PRIMARY KEY (id),
    CONSTRAINT webhook_deliveries_status_check CHECK (status IN ('queued', 'sending', 'delivered', 'failed')),
    CONSTRAINT webhook_deliveries_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
    CONSTRAINT webhook_deliveries_endpoint_id_fkey FOREIGN KEY (endpoint_id) REFERENCES webhook_endpoints(id) ON DELETE CASCADE,
    CONSTRAINT webhook_deliveries_shipment_id_fkey FOREIGN KEY (shipment_id) REFERENCES shipments(id) ON DELETE SET NULL
);
CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_due ON webhook_deliveries (next_attempt_at) WHERE status = 'queued';
CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_endpoint ON webhook_deliveries (endpoint_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_created ON webhook_deliveries (created_at);

-- +goose Down
DROP TABLE IF EXISTS webhook_deliveries;
DROP TABLE IF EXISTS webhook_endpoints;
