-- +goose Up
-- Each driver's newest position, kept as the positions arrive, so the fleet
-- map reads one row a driver where it used to search a week of
-- tracking_events (audit O3).
CREATE TABLE IF NOT EXISTS driver_positions (
    driver_id uuid NOT NULL,
    tenant_id uuid NOT NULL,
    coordinates geometry(Point,4326) NOT NULL,
    speed_kph numeric(10,2),
    battery_level integer,
    recorded_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT driver_positions_pkey PRIMARY KEY (driver_id),
    CONSTRAINT driver_positions_driver_id_fkey FOREIGN KEY (driver_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT driver_positions_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_driver_positions_tenant ON driver_positions (tenant_id, recorded_at DESC);

-- What the map showed before: the newest position of the last week.
INSERT INTO driver_positions (driver_id, tenant_id, coordinates, speed_kph, battery_level, recorded_at)
SELECT DISTINCT ON (driver_id) driver_id, tenant_id, coordinates, speed_kph, battery_level, recorded_at
FROM tracking_events
WHERE driver_id IS NOT NULL AND recorded_at > NOW() - INTERVAL '7 days'
ORDER BY driver_id, recorded_at DESC
ON CONFLICT (driver_id) DO NOTHING;

-- +goose Down
DROP TABLE IF EXISTS driver_positions;
