-- name: CreateTrackingEvent :one
INSERT INTO tracking_events (
    tenant_id, shipment_id, driver_id,
    coordinates, accuracy_meters, speed_kph, heading,
    event_type, status, note,
    recorded_at, device_id, battery_level
) VALUES (
    $1, $2, $3, ST_MakePoint($4, $5)::geometry(Point, 4326), $6, $7, $8, $9, $10, $11, $12, $13, $14
) RETURNING *;

-- name: ListTrackingEventsByShipment :many
SELECT id, tenant_id, shipment_id, driver_id,
       ST_X(coordinates::geometry) as longitude,
       ST_Y(coordinates::geometry) as latitude,
       accuracy_meters, speed_kph, heading,
       event_type, status, note,
       recorded_at, synced_at, device_id, battery_level
FROM tracking_events
WHERE shipment_id = $1 AND tenant_id = $2
ORDER BY recorded_at DESC
LIMIT $3 OFFSET $4;

-- name: ListTrackingEventsByDriver :many
SELECT id, tenant_id, shipment_id, driver_id,
       ST_X(coordinates::geometry) as longitude,
       ST_Y(coordinates::geometry) as latitude,
       accuracy_meters, speed_kph, heading,
       event_type, status, note,
       recorded_at, synced_at, device_id, battery_level
FROM tracking_events
WHERE driver_id = $1 AND tenant_id = $2
ORDER BY recorded_at DESC
LIMIT $3 OFFSET $4;

-- name: GetTrackingEventsForSync :many
SELECT id, tenant_id, shipment_id, driver_id,
       ST_X(coordinates::geometry) as longitude,
       ST_Y(coordinates::geometry) as latitude,
       accuracy_meters, speed_kph, heading,
       event_type, status, note,
       recorded_at, synced_at, device_id, battery_level
FROM tracking_events
WHERE driver_id = $1
  AND tenant_id = $2
  AND recorded_at > $3
  AND synced_at IS NULL
ORDER BY recorded_at ASC;

-- name: MarkTrackingEventsSynced :exec
UPDATE tracking_events
SET synced_at = NOW()
WHERE id = ANY($1::uuid[]);

-- name: CountTrackingEventsByShipment :one
SELECT COUNT(*) FROM tracking_events WHERE shipment_id = $1;

-- name: GetLatestDriverLocations :many
SELECT DISTINCT ON (t.driver_id)
       t.id, t.tenant_id, t.shipment_id, t.driver_id,
       ST_X(t.coordinates::geometry) as longitude,
       ST_Y(t.coordinates::geometry) as latitude,
       t.accuracy_meters, t.speed_kph, t.heading,
       t.event_type, t.status, t.note,
       t.recorded_at, t.synced_at, t.device_id, t.battery_level,
       u.full_name as driver_name
FROM tracking_events t
JOIN users u ON t.driver_id = u.id
WHERE t.tenant_id = $1 AND t.driver_id IS NOT NULL
ORDER BY t.driver_id, t.recorded_at DESC;