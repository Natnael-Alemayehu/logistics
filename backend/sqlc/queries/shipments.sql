-- name: GetShipmentByID :one
SELECT * FROM shipments
WHERE id = $1 AND tenant_id = $2;

-- name: GetShipmentByIDForUpdate :one
-- Row lock for the read-modify-write of a status transition, so two
-- concurrent updates (a driver's sync and a dispatcher's click) serialise
-- instead of both validating against the same stale status.
SELECT * FROM shipments
WHERE id = $1 AND tenant_id = $2
FOR UPDATE;

-- name: GetShipmentByTrackingNumber :one
SELECT * FROM shipments
WHERE tracking_number = $1;

-- name: GetShipmentByConfirmToken :one
SELECT * FROM shipments
WHERE confirm_token = $1;

-- name: CreateShipment :one
INSERT INTO shipments (
    tenant_id, tracking_number, priority,
    origin_city, origin_address, origin_lat, origin_lng,
    destination_city, destination_address, destination_lat, destination_lng, destination_precise,
    customer_name, customer_phone,
    cargo_description, cargo_weight, cargo_value, special_instructions,
    driver_id, vehicle_id, status, estimated_delivery, eta_manual, created_by, status_changed_at
) VALUES (
    $1, $2, $3,
    $4, $5, $6, $7,
    $8, $9, $10, $11, $12,
    $13, $14,
    $15, $16, $17, $18,
    $19, $20, $21, $22, $23, $24, NOW()
) RETURNING *;

-- name: UpdateShipmentDetails :one
-- Edits replace every editable field; the form always submits all of them.
UPDATE shipments
SET priority = $3,
    origin_city = $4,
    origin_address = $5,
    origin_lat = $6,
    origin_lng = $7,
    destination_city = $8,
    destination_address = $9,
    destination_lat = $10,
    destination_lng = $11,
    destination_precise = $12,
    customer_name = $13,
    customer_phone = $14,
    cargo_description = $15,
    cargo_weight = $16,
    cargo_value = $17,
    special_instructions = $18,
    estimated_delivery = $19,
    eta_manual = $20,
    updated_at = NOW()
WHERE id = $1 AND tenant_id = $2
RETURNING *;

-- name: SetShipmentETA :exec
-- Recalculated estimates never overwrite one a dispatcher entered by hand.
UPDATE shipments SET estimated_delivery = $3
WHERE id = $1 AND tenant_id = $2 AND NOT eta_manual;

-- name: SetShipmentAssignment :one
UPDATE shipments
SET driver_id = $3,
    vehicle_id = $4,
    status = $5,
    status_changed_at = CASE WHEN status <> $5 THEN NOW() ELSE status_changed_at END,
    updated_at = NOW()
WHERE id = $1 AND tenant_id = $2
RETURNING *;

-- name: SetShipmentStatus :one
-- actual_delivery is stamped the first time a shipment is delivered and kept
-- thereafter; picked_up_at likewise records the first departure. Both use the
-- time the change happened, which for an offline driver is earlier than now.
UPDATE shipments
SET status = sqlc.arg(status),
    status_note = sqlc.narg(status_note),
    status_reason = sqlc.narg(status_reason),
    status_changed_at = sqlc.arg(changed_at),
    actual_delivery = CASE
        WHEN sqlc.arg(status) = 'delivered' THEN COALESCE(actual_delivery, sqlc.arg(changed_at))
        ELSE actual_delivery
    END,
    picked_up_at = CASE
        WHEN sqlc.arg(status) = 'in_transit' THEN COALESCE(picked_up_at, sqlc.arg(changed_at))
        ELSE picked_up_at
    END,
    updated_at = NOW()
WHERE id = sqlc.arg(id) AND tenant_id = sqlc.arg(tenant_id)
RETURNING *;

-- name: SetShipmentConfirmToken :exec
UPDATE shipments SET confirm_token = $2 WHERE id = $1 AND confirm_token IS NULL;

-- name: SetCustomerConfirmation :one
UPDATE shipments
SET customer_confirmed_at = COALESCE(customer_confirmed_at, NOW()),
    customer_feedback = COALESCE(sqlc.narg(feedback), customer_feedback),
    updated_at = NOW()
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: ListShipments :many
-- Every filter is optional: an empty status list or a NULL argument means "no
-- constraint". Sorting goes through a whitelist of keys rather than
-- interpolated SQL.
SELECT sqlc.embed(s),
       d.full_name AS driver_name,
       v.plate_number AS vehicle_plate
FROM shipments s
LEFT JOIN users d ON d.id = s.driver_id
LEFT JOIN vehicles v ON v.id = s.vehicle_id
WHERE s.tenant_id = sqlc.arg(tenant_id)
  AND (cardinality(sqlc.arg(statuses)::text[]) = 0 OR s.status = ANY(sqlc.arg(statuses)::text[]))
  AND (sqlc.narg(driver_id)::uuid IS NULL OR s.driver_id = sqlc.narg(driver_id)::uuid)
  AND (sqlc.narg(vehicle_id)::uuid IS NULL OR s.vehicle_id = sqlc.narg(vehicle_id)::uuid)
  AND (sqlc.narg(priority)::text IS NULL OR s.priority = sqlc.narg(priority)::text)
  AND (sqlc.narg(date_from)::timestamptz IS NULL OR s.created_at >= sqlc.narg(date_from)::timestamptz)
  AND (sqlc.narg(date_to)::timestamptz IS NULL OR s.created_at < sqlc.narg(date_to)::timestamptz)
  AND (sqlc.narg(origin)::text IS NULL
       OR s.origin_city ILIKE sqlc.narg(origin)::text
       OR s.origin_address ILIKE '%' || sqlc.narg(origin)::text || '%')
  AND (sqlc.narg(destination)::text IS NULL
       OR s.destination_city ILIKE sqlc.narg(destination)::text
       OR s.destination_address ILIKE '%' || sqlc.narg(destination)::text || '%')
  AND (sqlc.narg(query)::text IS NULL
       OR s.tracking_number ILIKE '%' || sqlc.narg(query)::text || '%'
       OR s.customer_name ILIKE '%' || sqlc.narg(query)::text || '%'
       OR s.customer_phone ILIKE '%' || sqlc.narg(query)::text || '%'
       OR d.full_name ILIKE '%' || sqlc.narg(query)::text || '%')
ORDER BY
  CASE WHEN sqlc.arg(sort)::text = 'created_asc' THEN s.created_at END ASC,
  CASE WHEN sqlc.arg(sort)::text = 'eta_asc' THEN s.estimated_delivery END ASC NULLS LAST,
  CASE WHEN sqlc.arg(sort)::text = 'eta_desc' THEN s.estimated_delivery END DESC NULLS LAST,
  CASE WHEN sqlc.arg(sort)::text = 'updated_desc' THEN s.updated_at END DESC,
  CASE WHEN sqlc.arg(sort)::text = 'status' THEN s.status END ASC,
  CASE WHEN sqlc.arg(sort)::text = 'customer' THEN s.customer_name END ASC,
  CASE WHEN sqlc.arg(sort)::text = 'tracking' THEN s.tracking_number END ASC,
  CASE WHEN sqlc.arg(sort)::text = 'priority' THEN
      CASE s.priority WHEN 'urgent' THEN 0 WHEN 'high' THEN 1 ELSE 2 END
  END ASC,
  s.created_at DESC
LIMIT sqlc.arg(row_limit) OFFSET sqlc.arg(row_offset);

-- name: CountShipments :one
SELECT COUNT(*)
FROM shipments s
LEFT JOIN users d ON d.id = s.driver_id
WHERE s.tenant_id = sqlc.arg(tenant_id)
  AND (cardinality(sqlc.arg(statuses)::text[]) = 0 OR s.status = ANY(sqlc.arg(statuses)::text[]))
  AND (sqlc.narg(driver_id)::uuid IS NULL OR s.driver_id = sqlc.narg(driver_id)::uuid)
  AND (sqlc.narg(vehicle_id)::uuid IS NULL OR s.vehicle_id = sqlc.narg(vehicle_id)::uuid)
  AND (sqlc.narg(priority)::text IS NULL OR s.priority = sqlc.narg(priority)::text)
  AND (sqlc.narg(date_from)::timestamptz IS NULL OR s.created_at >= sqlc.narg(date_from)::timestamptz)
  AND (sqlc.narg(date_to)::timestamptz IS NULL OR s.created_at < sqlc.narg(date_to)::timestamptz)
  AND (sqlc.narg(origin)::text IS NULL
       OR s.origin_city ILIKE sqlc.narg(origin)::text
       OR s.origin_address ILIKE '%' || sqlc.narg(origin)::text || '%')
  AND (sqlc.narg(destination)::text IS NULL
       OR s.destination_city ILIKE sqlc.narg(destination)::text
       OR s.destination_address ILIKE '%' || sqlc.narg(destination)::text || '%')
  AND (sqlc.narg(query)::text IS NULL
       OR s.tracking_number ILIKE '%' || sqlc.narg(query)::text || '%'
       OR s.customer_name ILIKE '%' || sqlc.narg(query)::text || '%'
       OR s.customer_phone ILIKE '%' || sqlc.narg(query)::text || '%'
       OR d.full_name ILIKE '%' || sqlc.narg(query)::text || '%');

-- name: ListActiveShipmentsByDriver :many
SELECT * FROM shipments
WHERE driver_id = $1
  AND tenant_id = $2
  AND status IN ('assigned', 'in_transit', 'delayed', 'arrived', 'issue')
ORDER BY CASE priority WHEN 'urgent' THEN 0 WHEN 'high' THEN 1 ELSE 2 END,
         estimated_delivery ASC NULLS LAST,
         created_at ASC;

-- name: CountActiveShipmentsByDriver :one
SELECT COUNT(*) FROM shipments
WHERE driver_id = $1
  AND tenant_id = $2
  AND status IN ('assigned', 'in_transit', 'delayed', 'arrived', 'issue');

-- name: ListShipmentsForDriverSync :many
-- Everything the driver's device should hold: active work, plus recently
-- finished work so the completed list survives a reinstall. On an incremental
-- sync anything changed since the cursor is included as well, so a
-- cancellation or a dispatcher edit reaches the device.
SELECT * FROM shipments
WHERE driver_id = sqlc.arg(driver_id)
  AND tenant_id = sqlc.arg(tenant_id)
  AND (
    status IN ('assigned', 'in_transit', 'delayed', 'arrived', 'issue')
    OR updated_at > NOW() - INTERVAL '7 days'
    OR (sqlc.narg(since)::timestamptz IS NOT NULL AND updated_at > sqlc.narg(since)::timestamptz)
  )
ORDER BY created_at DESC
LIMIT 200;

-- name: CountShipmentsByStatus :many
SELECT status, COUNT(*)::bigint AS total
FROM shipments
WHERE tenant_id = $1
GROUP BY status;

-- name: CountDeliveredSince :one
SELECT COUNT(*) FROM shipments
WHERE tenant_id = $1 AND status = 'delivered' AND actual_delivery >= $2;

-- name: CountShipmentsCreatedSince :one
SELECT COUNT(*) FROM shipments
WHERE tenant_id = $1 AND created_at >= $2;

-- name: ListActiveShipmentsForMap :many
SELECT s.id, s.tracking_number, s.status, s.priority,
       s.destination_city, s.destination_lat, s.destination_lng,
       s.origin_lat, s.origin_lng, s.driver_id,
       d.full_name AS driver_name
FROM shipments s
LEFT JOIN users d ON d.id = s.driver_id
WHERE s.tenant_id = $1
  AND s.status IN ('assigned', 'in_transit', 'delayed', 'arrived', 'issue')
ORDER BY s.created_at DESC
LIMIT 500;

-- name: ListOverdueShipments :many
-- Across tenants: the job that raises ETA alerts runs once for the platform.
SELECT s.id, s.tenant_id, s.tracking_number, s.driver_id, s.estimated_delivery
FROM shipments s
WHERE s.status IN ('assigned', 'in_transit', 'delayed', 'arrived', 'issue')
  AND s.estimated_delivery IS NOT NULL
  AND s.estimated_delivery < NOW()
LIMIT 1000;

-- name: ListShipmentsByCustomerPhone :many
SELECT s.tracking_number, s.status, s.origin_city, s.destination_city,
       s.estimated_delivery, s.actual_delivery, s.created_at, s.updated_at,
       t.name AS company_name
FROM shipments s
JOIN tenants t ON t.id = s.tenant_id
WHERE s.customer_phone = $1
  AND s.created_at > NOW() - INTERVAL '90 days'
ORDER BY s.created_at DESC
LIMIT 50;

-- name: ListRecentShipmentsByDriver :many
SELECT * FROM shipments
WHERE driver_id = $1 AND tenant_id = $2
ORDER BY created_at DESC
LIMIT $3;

-- name: ListRecentShipmentsByVehicle :many
SELECT s.*, d.full_name AS driver_name
FROM shipments s
LEFT JOIN users d ON d.id = s.driver_id
WHERE s.vehicle_id = $1 AND s.tenant_id = $2
ORDER BY s.created_at DESC
LIMIT $3;

-- name: RouteTransitHours :many
-- Hours from departure to delivery for recent deliveries between two cities.
-- ETA estimation takes the median once enough history exists.
SELECT (EXTRACT(EPOCH FROM (actual_delivery - COALESCE(picked_up_at, created_at))) / 3600.0)::float8 AS hours
FROM shipments
WHERE tenant_id = $1
  AND status = 'delivered'
  AND origin_city = $2
  AND destination_city = $3
  AND actual_delivery IS NOT NULL
  AND actual_delivery > COALESCE(picked_up_at, created_at)
ORDER BY actual_delivery DESC
LIMIT 50;

-- name: SetCustomerFeedback :one
-- A problem report: the customer has answered but not confirmed receipt.
UPDATE shipments
SET customer_feedback = sqlc.arg(feedback), updated_at = NOW()
WHERE id = sqlc.arg(id)
RETURNING *;
