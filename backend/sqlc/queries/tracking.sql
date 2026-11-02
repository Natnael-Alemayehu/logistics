-- name: CreateTrackingEvent :one
-- client_id carries the offline client's own row identifier. Paired with
-- device_id it is what a retried batch conflicts on, so replaying a batch whose
-- response was lost cannot duplicate telemetry.
INSERT INTO tracking_events (
    tenant_id, shipment_id, driver_id,
    coordinates, accuracy_meters, speed_kph, heading,
    event_type, status, note,
    recorded_at, synced_at, device_id, battery_level, client_id
) VALUES (
    sqlc.arg(tenant_id), sqlc.arg(shipment_id), sqlc.arg(driver_id),
    ST_SetSRID(ST_MakePoint(sqlc.arg(longitude)::float8, sqlc.arg(latitude)::float8), 4326),
    sqlc.narg(accuracy_meters), sqlc.narg(speed_kph), sqlc.narg(heading),
    sqlc.arg(event_type), sqlc.narg(status), sqlc.narg(note),
    sqlc.arg(recorded_at), NOW(), sqlc.narg(device_id), sqlc.narg(battery_level), sqlc.narg(client_id)
)
ON CONFLICT (device_id, client_id) WHERE client_id IS NOT NULL AND device_id IS NOT NULL DO NOTHING
RETURNING id;

-- name: ListShipmentRoute :many
-- A truck carries several shipments at once and its pings are attributed to
-- only one of them, so a shipment's route is the driver's track over the time
-- the shipment was on board, plus anything explicitly tagged to it.
-- The newest max_points positions, oldest first, so a long trip still ends
-- where the truck is now.
SELECT latest.* FROM (
    SELECT id, shipment_id, event_type, note,
           ST_Y(coordinates)::float8 AS latitude,
           ST_X(coordinates)::float8 AS longitude,
           accuracy_meters, speed_kph, battery_level, recorded_at
    FROM tracking_events
    WHERE tenant_id = sqlc.arg(tenant_id)
      AND (
        shipment_id = sqlc.arg(shipment_id)
        OR (sqlc.narg(driver_id)::uuid IS NOT NULL
            AND driver_id = sqlc.narg(driver_id)::uuid
            AND recorded_at >= sqlc.arg(window_start)
            AND recorded_at <= sqlc.arg(window_end))
      )
    ORDER BY recorded_at DESC
    LIMIT sqlc.arg(max_points)
) latest
ORDER BY latest.recorded_at ASC;

-- name: GetLatestDriverLocations :many
SELECT DISTINCT ON (t.driver_id)
       t.driver_id,
       u.full_name AS driver_name,
       u.phone AS driver_phone,
       ST_Y(t.coordinates)::float8 AS latitude,
       ST_X(t.coordinates)::float8 AS longitude,
       t.speed_kph, t.battery_level, t.recorded_at,
       sm.last_sync_at, sm.tracking_paused
FROM tracking_events t
JOIN users u ON u.id = t.driver_id
LEFT JOIN sync_metadata sm ON sm.driver_id = t.driver_id
WHERE t.tenant_id = $1
  AND t.driver_id IS NOT NULL
  AND u.is_active
  AND t.recorded_at > NOW() - INTERVAL '7 days'
ORDER BY t.driver_id, t.recorded_at DESC;

-- name: GetLatestDriverLocation :one
SELECT ST_Y(coordinates)::float8 AS latitude,
       ST_X(coordinates)::float8 AS longitude,
       accuracy_meters, speed_kph, battery_level, recorded_at, synced_at
FROM tracking_events
WHERE tenant_id = $1 AND driver_id = $2
ORDER BY recorded_at DESC
LIMIT 1;

-- name: GetLatestShipmentPosition :one
-- Where the shipment was last seen, for the public tracking page.
SELECT ST_Y(coordinates)::float8 AS latitude,
       ST_X(coordinates)::float8 AS longitude,
       recorded_at
FROM tracking_events
WHERE tenant_id = sqlc.arg(tenant_id)
  AND (
    shipment_id = sqlc.arg(shipment_id)
    OR (sqlc.narg(driver_id)::uuid IS NOT NULL
        AND driver_id = sqlc.narg(driver_id)::uuid
        AND recorded_at >= sqlc.arg(window_start))
  )
ORDER BY recorded_at DESC
LIMIT 1;

-- name: CountTrackingEventsByShipment :one
SELECT COUNT(*) FROM tracking_events WHERE shipment_id = $1 AND tenant_id = $2;

-- name: DeleteOldTrackingEvents :exec
DELETE FROM tracking_events WHERE recorded_at < $1;
