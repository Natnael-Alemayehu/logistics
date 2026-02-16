CREATE EXTENSION IF NOT EXISTS postgis;

CREATE TABLE tenants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    slug TEXT UNIQUE NOT NULL,
    plan TEXT NOT NULL DEFAULT 'starter',
    max_drivers INTEGER DEFAULT 5,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    role TEXT NOT NULL CHECK (role IN ('driver', 'dispatcher', 'fleet_manager', 'admin', 'platform_admin')),
    full_name TEXT NOT NULL,
    phone TEXT,
    email TEXT,
    password_hash TEXT,
    pin_hash TEXT,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(tenant_id, phone),
    UNIQUE(tenant_id, email)
);

CREATE UNIQUE INDEX idx_users_phone_unique ON users(phone) WHERE phone IS NOT NULL;
CREATE UNIQUE INDEX idx_users_email_unique ON users(email) WHERE email IS NOT NULL;

CREATE INDEX idx_users_tenant ON users(tenant_id);
CREATE INDEX idx_users_phone ON users(phone) WHERE phone IS NOT NULL;
CREATE INDEX idx_users_email ON users(email) WHERE email IS NOT NULL;

CREATE TABLE vehicles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    plate_number TEXT NOT NULL,
    vehicle_type TEXT,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(tenant_id, plate_number)
);

CREATE INDEX idx_vehicles_tenant ON vehicles(tenant_id);

CREATE TABLE shipments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    tracking_number TEXT UNIQUE NOT NULL,
    origin_address TEXT NOT NULL,
    origin_coordinates GEOMETRY(POINT, 4326),
    destination_address TEXT NOT NULL,
    destination_coordinates GEOMETRY(POINT, 4326),
    customer_name TEXT NOT NULL,
    customer_phone TEXT NOT NULL,
    cargo_description TEXT,
    cargo_weight DECIMAL(10, 2),
    cargo_value DECIMAL(15, 2),
    special_instructions TEXT,
    driver_id UUID REFERENCES users(id) ON DELETE SET NULL,
    vehicle_id UUID REFERENCES vehicles(id) ON DELETE SET NULL,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'assigned', 'in_transit', 'delayed', 'arrived', 'delivered', 'issue', 'cancelled')),
    status_note TEXT,
    status_reason TEXT,
    estimated_delivery TIMESTAMPTZ,
    actual_delivery TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    created_by UUID REFERENCES users(id)
);

CREATE INDEX idx_shipments_tenant ON shipments(tenant_id);
CREATE INDEX idx_shipments_tracking ON shipments(tracking_number);
CREATE INDEX idx_shipments_driver ON shipments(driver_id);
CREATE INDEX idx_shipments_status ON shipments(status);
CREATE INDEX idx_shipments_created ON shipments(created_at);
CREATE INDEX idx_shipments_destination_coords ON shipments USING GIST(destination_coordinates);

CREATE TABLE tracking_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    shipment_id UUID NOT NULL REFERENCES shipments(id) ON DELETE CASCADE,
    driver_id UUID REFERENCES users(id) ON DELETE SET NULL,
    coordinates GEOMETRY(POINT, 4326) NOT NULL,
    accuracy_meters DECIMAL(10, 2),
    speed_kph DECIMAL(10, 2),
    heading DECIMAL(5, 2),
    event_type TEXT NOT NULL CHECK (event_type IN ('gps_ping', 'checkpoint', 'status_change')),
    status TEXT,
    note TEXT,
    recorded_at TIMESTAMPTZ NOT NULL,
    synced_at TIMESTAMPTZ,
    device_id TEXT,
    battery_level INTEGER
);

CREATE INDEX idx_tracking_tenant ON tracking_events(tenant_id);
CREATE INDEX idx_tracking_shipment ON tracking_events(shipment_id);
CREATE INDEX idx_tracking_driver ON tracking_events(driver_id);
CREATE INDEX idx_tracking_recorded ON tracking_events(recorded_at);
CREATE INDEX idx_tracking_coords ON tracking_events USING GIST(coordinates);

CREATE TABLE proof_of_deliveries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    shipment_id UUID UNIQUE NOT NULL REFERENCES shipments(id) ON DELETE CASCADE,
    driver_id UUID NOT NULL REFERENCES users(id),
    recipient_name TEXT NOT NULL,
    recipient_phone TEXT,
    signature_data TEXT,
    signature_url TEXT,
    photo_urls TEXT[],
    delivery_address TEXT,
    delivery_coordinates GEOMETRY(POINT, 4326),
    delivery_notes TEXT,
    location_verified BOOLEAN DEFAULT false,
    location_mismatch_meters DECIMAL(10, 2),
    recorded_at TIMESTAMPTZ NOT NULL,
    synced_at TIMESTAMPTZ
);

CREATE INDEX idx_pod_tenant ON proof_of_deliveries(tenant_id);
CREATE INDEX idx_pod_shipment ON proof_of_deliveries(shipment_id);
CREATE INDEX idx_pod_driver ON proof_of_deliveries(driver_id);

CREATE TABLE sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    refresh_token_hash TEXT NOT NULL UNIQUE,
    device_id TEXT,
    user_agent TEXT,
    ip_address TEXT,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    revoked_at TIMESTAMPTZ
);

CREATE INDEX idx_sessions_user ON sessions(user_id);
CREATE INDEX idx_sessions_token ON sessions(refresh_token_hash);
CREATE INDEX idx_sessions_expires ON sessions(expires_at);

CREATE TABLE audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID REFERENCES tenants(id) ON DELETE SET NULL,
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    action TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    entity_id UUID,
    old_value JSONB,
    new_value JSONB,
    ip_address TEXT,
    user_agent TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_audit_tenant ON audit_logs(tenant_id);
CREATE INDEX idx_audit_user ON audit_logs(user_id);
CREATE INDEX idx_audit_action ON audit_logs(action);
CREATE INDEX idx_audit_created ON audit_logs(created_at);

CREATE TABLE sync_metadata (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    driver_id UUID UNIQUE NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    last_sync_at TIMESTAMPTZ,
    last_event_id UUID,
    pending_sync_count INTEGER DEFAULT 0,
    storage_used_kb INTEGER DEFAULT 0,
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER update_tenants_updated_at BEFORE UPDATE ON tenants
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_users_updated_at BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_vehicles_updated_at BEFORE UPDATE ON vehicles
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_shipments_updated_at BEFORE UPDATE ON shipments
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_sync_metadata_updated_at BEFORE UPDATE ON sync_metadata
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();