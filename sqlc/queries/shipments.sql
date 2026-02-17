-- name: GetShipmentByID :one
SELECT * FROM shipments
WHERE id = $1 AND tenant_id = $2;

-- name: GetShipmentByTrackingNumber :one
SELECT * FROM shipments
WHERE tracking_number = $1;

-- name: ListShipmentsByTenant :many
SELECT * FROM shipments
WHERE tenant_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListShipmentsByDriver :many
SELECT * FROM shipments
WHERE driver_id = $1 AND tenant_id = $2
ORDER BY created_at DESC
LIMIT $3 OFFSET $4;

-- name: ListShipmentsByStatus :many
SELECT * FROM shipments
WHERE tenant_id = $1 AND status = $2
ORDER BY created_at DESC
LIMIT $3 OFFSET $4;

-- name: ListActiveShipmentsByDriver :many
SELECT * FROM shipments
WHERE driver_id = $1 
  AND tenant_id = $2
  AND status IN ('assigned', 'in_transit', 'delayed', 'arrived')
ORDER BY created_at DESC;

-- name: CreateShipment :one
INSERT INTO shipments (
    tenant_id, tracking_number,
    origin_address,
    destination_address,
    customer_name, customer_phone,
    cargo_description, cargo_weight, cargo_value, special_instructions,
    driver_id, vehicle_id,
    status, status_note,
    estimated_delivery,
    created_by
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16
) RETURNING *;

-- name: UpdateShipment :one
UPDATE shipments
SET
    origin_address = COALESCE(sqlc.narg(origin_address), origin_address),
    destination_address = COALESCE(sqlc.narg(destination_address), destination_address),
    customer_name = COALESCE(sqlc.narg(customer_name), customer_name),
    customer_phone = COALESCE(sqlc.narg(customer_phone), customer_phone),
    cargo_description = COALESCE(sqlc.narg(cargo_description), cargo_description),
    cargo_weight = COALESCE(sqlc.narg(cargo_weight), cargo_weight),
    cargo_value = COALESCE(sqlc.narg(cargo_value), cargo_value),
    special_instructions = COALESCE(sqlc.narg(special_instructions), special_instructions),
    driver_id = COALESCE(sqlc.narg(driver_id), driver_id),
    vehicle_id = COALESCE(sqlc.narg(vehicle_id), vehicle_id),
    status = COALESCE(sqlc.narg(status), status),
    status_note = COALESCE(sqlc.narg(status_note), status_note),
    status_reason = COALESCE(sqlc.narg(status_reason), status_reason),
    estimated_delivery = COALESCE(sqlc.narg(estimated_delivery), estimated_delivery),
    actual_delivery = COALESCE(sqlc.narg(actual_delivery), actual_delivery)
WHERE id = $1 AND tenant_id = $2
RETURNING *;

-- name: AssignDriverToShipment :one
UPDATE shipments
SET driver_id = $1, status = 'assigned', updated_at = NOW()
WHERE id = $2 AND tenant_id = $3
RETURNING *;

-- name: UpdateShipmentStatus :one
UPDATE shipments
SET status = $1, status_note = $2, status_reason = $3, updated_at = NOW()
WHERE id = $4 AND tenant_id = $5
RETURNING *;

-- name: CancelShipment :one
UPDATE shipments
SET status = 'cancelled', status_reason = $2, updated_at = NOW()
WHERE id = $1 AND tenant_id = $3
RETURNING *;

-- name: CountShipmentsByTenant :one
SELECT COUNT(*) FROM shipments WHERE tenant_id = $1;

-- name: SearchShipments :many
SELECT * FROM shipments
WHERE tenant_id = $1
  AND (
    tracking_number ILIKE '%' || $2 || '%'
    OR customer_name ILIKE '%' || $2 || '%'
    OR customer_phone ILIKE '%' || $2 || '%'
  )
ORDER BY created_at DESC
LIMIT $3 OFFSET $4;

-- name: AdvancedSearchShipments :many
SELECT * FROM shipments
WHERE tenant_id = $1
  AND ($2::text IS NULL OR status = $2)
  AND ($3::uuid IS NULL OR driver_id = $3)
  AND ($4::timestamptz IS NULL OR created_at >= $4)
  AND ($5::timestamptz IS NULL OR created_at <= $5)
  AND ($6::text IS NULL OR origin_address ILIKE '%' || $6 || '%')
  AND ($7::text IS NULL OR destination_address ILIKE '%' || $7 || '%')
  AND ($8::text IS NULL OR tracking_number ILIKE '%' || $8 || '%'
       OR customer_name ILIKE '%' || $8 || '%'
       OR customer_phone ILIKE '%' || $8 || '%')
ORDER BY created_at DESC
LIMIT $9 OFFSET $10;

-- name: GetShipmentsForSync :many
SELECT * FROM shipments
WHERE driver_id = $1
  AND tenant_id = $2
  AND (status IN ('assigned', 'in_transit', 'delayed', 'arrived') OR updated_at > $3)
ORDER BY created_at DESC;