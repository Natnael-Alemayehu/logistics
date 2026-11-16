-- name: CreateTrackingEvents :exec
-- A batch of a driver's GPS fixes in one statement: a phone back from a day
-- without signal sends thousands. The arrays run in parallel, one element
-- a fix, with -1 standing for a missing number and '' for a missing client
-- ID, since an array here can't hold NULL.
--
-- client_id carries the offline client's own row identifier. Paired with
-- device_id it is what a retried batch conflicts on, so replaying a batch whose
-- response was lost cannot duplicate telemetry.
INSERT INTO tracking_events (
    tenant_id, shipment_id, driver_id,
    coordinates, accuracy_meters, speed_kph, heading,
    event_type, recorded_at, synced_at, device_id, battery_level, client_id
)
SELECT sqlc.arg(tenant_id), p.shipment_id, sqlc.arg(driver_id),
       ST_SetSRID(ST_MakePoint(p.longitude, p.latitude), 4326),
       NULLIF(p.accuracy_meters, -1), NULLIF(p.speed_kph, -1), NULLIF(p.heading, -1),
       'gps_ping', p.recorded_at, NOW(), sqlc.narg(device_id), NULLIF(p.battery_level, -1), NULLIF(p.client_id, '')
FROM (
    -- Set-returning functions side by side in one SELECT run in step.
    SELECT unnest(sqlc.arg(shipment_ids)::uuid[]) AS shipment_id,
           unnest(sqlc.arg(longitudes)::float8[]) AS longitude,
           unnest(sqlc.arg(latitudes)::float8[]) AS latitude,
           unnest(sqlc.arg(accuracies)::float8[]) AS accuracy_meters,
           unnest(sqlc.arg(speeds)::float8[]) AS speed_kph,
           unnest(sqlc.arg(headings)::float8[]) AS heading,
           unnest(sqlc.arg(batteries)::int[]) AS battery_level,
           unnest(sqlc.arg(recorded)::timestamptz[]) AS recorded_at,
           unnest(sqlc.arg(client_ids)::text[]) AS client_id
) p
ON CONFLICT (device_id, client_id) WHERE client_id IS NOT NULL AND device_id IS NOT NULL DO NOTHING;

-- name: SetDriverPosition :exec
-- Keeps the driver's newest position for the fleet map. An older fix,
-- from a phone sending what it saved while offline, changes nothing.
INSERT INTO driver_positions (driver_id, tenant_id, coordinates, speed_kph, battery_level, recorded_at)
VALUES (
    sqlc.arg(driver_id), sqlc.arg(tenant_id),
    ST_SetSRID(ST_MakePoint(sqlc.arg(longitude)::float8, sqlc.arg(latitude)::float8), 4326),
    sqlc.narg(speed_kph), sqlc.narg(battery_level), sqlc.arg(recorded_at)
)
ON CONFLICT (driver_id) DO UPDATE
SET tenant_id = EXCLUDED.tenant_id, coordinates = EXCLUDED.coordinates, speed_kph = EXCLUDED.speed_kph,
    battery_level = EXCLUDED.battery_level, recorded_at = EXCLUDED.recorded_at
WHERE EXCLUDED.recorded_at > driver_positions.recorded_at;

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
-- The fleet map: where each active driver was last seen, within a week.
SELECT p.driver_id,
       u.full_name AS driver_name,
       u.phone AS driver_phone,
       ST_Y(p.coordinates)::float8 AS latitude,
       ST_X(p.coordinates)::float8 AS longitude,
       p.speed_kph, p.battery_level, p.recorded_at,
       sm.last_sync_at, sm.tracking_paused,
       COALESCE(sm.power_saving, false)::bool AS power_saving
FROM driver_positions p
JOIN users u ON u.id = p.driver_id
LEFT JOIN sync_metadata sm ON sm.driver_id = p.driver_id
WHERE p.tenant_id = $1
  AND u.is_active
  AND p.recorded_at > NOW() - INTERVAL '7 days'
ORDER BY p.driver_id;

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
