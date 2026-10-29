-- name: GetDriverDeliveryCounts :one
SELECT
    COUNT(*) FILTER (WHERE status = 'delivered' AND actual_delivery >= date_trunc('day', NOW()))::bigint AS today,
    COUNT(*) FILTER (WHERE status = 'delivered' AND actual_delivery >= date_trunc('week', NOW()))::bigint AS week,
    COUNT(*) FILTER (WHERE status = 'delivered' AND actual_delivery >= date_trunc('month', NOW()))::bigint AS month,
    COUNT(*) FILTER (WHERE status = 'delivered')::bigint AS total,
    COUNT(*) FILTER (WHERE status IN ('assigned', 'in_transit', 'delayed', 'arrived', 'issue'))::bigint AS active
FROM shipments
WHERE driver_id = $1 AND tenant_id = $2;

-- name: GetDriverPerformance :one
-- One driver's record over a period, for the driver page.
SELECT
    COUNT(*) FILTER (WHERE shipments.status = 'delivered')::bigint AS delivered,
    COUNT(*) FILTER (WHERE shipments.status = 'delivered' AND shipments.estimated_delivery IS NOT NULL
                       AND shipments.actual_delivery <= shipments.estimated_delivery)::bigint AS on_time,
    COUNT(*) FILTER (WHERE shipments.status = 'delivered' AND shipments.estimated_delivery IS NOT NULL)::bigint AS with_eta,
    COALESCE(AVG(EXTRACT(EPOCH FROM (shipments.actual_delivery - COALESCE(shipments.picked_up_at, shipments.created_at))) / 3600.0)
        FILTER (WHERE shipments.status = 'delivered'), 0)::float8 AS avg_hours,
    COUNT(*) FILTER (WHERE EXISTS (
        SELECT 1 FROM shipment_events e
        WHERE e.shipment_id = shipments.id AND e.event_type = 'status_changed' AND e.to_status = 'issue'
    ))::bigint AS with_issues
FROM shipments
WHERE shipments.driver_id = sqlc.arg(driver_id) AND shipments.tenant_id = sqlc.arg(tenant_id)
  AND shipments.created_at >= sqlc.arg(since);

-- name: UpsertSyncMetadata :exec
INSERT INTO sync_metadata (
    driver_id, last_sync_at, device_id, app_version, battery_level,
    pending_sync_count, storage_used_kb, clock_offset_seconds, tracking_paused
) VALUES ($1, NOW(), $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (driver_id) DO UPDATE SET
    last_sync_at = NOW(),
    device_id = EXCLUDED.device_id,
    app_version = EXCLUDED.app_version,
    battery_level = EXCLUDED.battery_level,
    pending_sync_count = EXCLUDED.pending_sync_count,
    storage_used_kb = EXCLUDED.storage_used_kb,
    clock_offset_seconds = EXCLUDED.clock_offset_seconds,
    tracking_paused = EXCLUDED.tracking_paused;

-- name: GetSyncMetadata :one
SELECT * FROM sync_metadata WHERE driver_id = $1;

-- name: ListStaleActiveDrivers :many
-- Drivers carrying a shipment who have not been heard from since the cutoff.
-- Last contact is the later of the last app sync and the shipment's last
-- status change, so a driver reporting by SMS counts as in touch and one who
-- never installed the app is still caught.
SELECT DISTINCT ON (s.driver_id)
       s.tenant_id, s.driver_id, s.id AS shipment_id, s.tracking_number,
       u.full_name AS driver_name,
       GREATEST(sm.last_sync_at, s.status_changed_at, s.created_at)::timestamptz AS last_contact
FROM shipments s
JOIN users u ON u.id = s.driver_id
LEFT JOIN sync_metadata sm ON sm.driver_id = s.driver_id
WHERE s.status IN ('in_transit', 'delayed', 'arrived', 'issue')
  AND GREATEST(sm.last_sync_at, s.status_changed_at, s.created_at) < $1
ORDER BY s.driver_id, s.status_changed_at DESC;
