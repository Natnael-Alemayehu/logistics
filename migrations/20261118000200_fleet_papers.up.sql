-- +goose Up
-- The papers a driver and a truck need on the road: the driver's licence,
-- and the truck's insurance and annual inspection (ቦሎ). Each is the last
-- day it is valid. The office is reminded 30 days before.
ALTER TABLE users ADD COLUMN IF NOT EXISTS licence_expires_on date;
ALTER TABLE vehicles
    ADD COLUMN IF NOT EXISTS insurance_expires_on date,
    ADD COLUMN IF NOT EXISTS inspection_due_on date;

-- An alert may be about a vehicle, and its subject (for the one-open-alert
-- and once-per-episode rules) is then the vehicle.
ALTER TABLE alerts ADD COLUMN IF NOT EXISTS vehicle_id uuid
    CONSTRAINT alerts_vehicle_id_fkey REFERENCES vehicles(id) ON DELETE CASCADE;
DROP INDEX IF EXISTS idx_alerts_episode;
DROP INDEX IF EXISTS idx_alerts_open_subject;
CREATE UNIQUE INDEX IF NOT EXISTS idx_alerts_episode ON alerts (tenant_id, alert_type, COALESCE(shipment_id, driver_id, vehicle_id, tenant_id), episode) WHERE (episode IS NOT NULL);
CREATE UNIQUE INDEX IF NOT EXISTS idx_alerts_open_subject ON alerts (tenant_id, alert_type, COALESCE(shipment_id, driver_id, vehicle_id, tenant_id)) WHERE (acknowledged_at IS NULL);

-- +goose Down
DELETE FROM alerts WHERE vehicle_id IS NOT NULL;
DROP INDEX IF EXISTS idx_alerts_episode;
DROP INDEX IF EXISTS idx_alerts_open_subject;
ALTER TABLE alerts DROP COLUMN IF EXISTS vehicle_id;
CREATE UNIQUE INDEX IF NOT EXISTS idx_alerts_episode ON alerts (tenant_id, alert_type, COALESCE(shipment_id, driver_id, tenant_id), episode) WHERE (episode IS NOT NULL);
CREATE UNIQUE INDEX IF NOT EXISTS idx_alerts_open_subject ON alerts (tenant_id, alert_type, COALESCE(shipment_id, driver_id, tenant_id)) WHERE (acknowledged_at IS NULL);
ALTER TABLE vehicles DROP COLUMN IF EXISTS insurance_expires_on, DROP COLUMN IF EXISTS inspection_due_on;
ALTER TABLE users DROP COLUMN IF EXISTS licence_expires_on;
