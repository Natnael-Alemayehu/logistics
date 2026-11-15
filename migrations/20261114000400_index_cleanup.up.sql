-- +goose Up
-- +goose StatementBegin

-- ---------------------------------------------------------------------------
-- Indexes from the first schema that another index already covers: the
-- same column under a unique constraint, or the leading column of a wider
-- index. Each one only slowed writes.
-- ---------------------------------------------------------------------------
DROP INDEX IF EXISTS idx_users_email;              -- idx_users_email_unique
DROP INDEX IF EXISTS idx_users_phone;              -- idx_users_phone_unique
DROP INDEX IF EXISTS idx_users_tenant;             -- users_tenant_id_email_key
DROP INDEX IF EXISTS idx_shipments_tracking;       -- shipments_tracking_number_key
DROP INDEX IF EXISTS idx_shipments_tenant;         -- idx_shipments_tenant_created
DROP INDEX IF EXISTS idx_sessions_token;           -- sessions_refresh_token_hash_key
DROP INDEX IF EXISTS idx_audit_created;            -- idx_audit_created_desc
DROP INDEX IF EXISTS idx_vehicles_tenant;          -- vehicles_tenant_id_plate_number_key
DROP INDEX IF EXISTS idx_tracking_tenant;          -- idx_tracking_driver_recorded
DROP INDEX IF EXISTS idx_tracking_shipment;        -- idx_tracking_shipment_recorded

-- Signing in by email compares lower(email), which no index served.
CREATE INDEX IF NOT EXISTS idx_users_email_lower ON users (lower(email));

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_users_email_lower;
CREATE INDEX IF NOT EXISTS idx_users_tenant ON users(tenant_id);
CREATE INDEX IF NOT EXISTS idx_users_phone ON users(phone) WHERE phone IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email) WHERE email IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_vehicles_tenant ON vehicles(tenant_id);
CREATE INDEX IF NOT EXISTS idx_shipments_tenant ON shipments(tenant_id);
CREATE INDEX IF NOT EXISTS idx_shipments_tracking ON shipments(tracking_number);
CREATE INDEX IF NOT EXISTS idx_tracking_tenant ON tracking_events(tenant_id);
CREATE INDEX IF NOT EXISTS idx_tracking_shipment ON tracking_events(shipment_id);
CREATE INDEX IF NOT EXISTS idx_sessions_token ON sessions(refresh_token_hash);
CREATE INDEX IF NOT EXISTS idx_audit_created ON audit_logs(created_at);
-- +goose StatementEnd
