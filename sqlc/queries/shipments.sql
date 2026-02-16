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
    destination_address = COALESCE(sqlc.narg(destination_address), destination_address),
    customer_name = COALESCE(sqlc.narg(customer_name), customer_name),
    customer_phone = COALESCE(sqlc.narg(customer_phone), customer_phone),
    cargo_description = COALESCE(sqlc.narg(cargo_description), cargo_description),
    cargo_weight = COALESCE(sqlc.narg(cargo_weight), cargo_weight),
    special_instructions = COALESCE(sqlc.narg(special_instructions), special_instructions),
    driver_id = COALESCE(sqlc.narg(driver_id), driver_id),
    vehicle_id = COALESCE(sqlc.narg(vehicle_id), vehicle_id),
    status = COALESCE(sqlc.narg(status), status),
    status_note = COALESCE(sqlc.narg(status_note), status_note),
    status_reason = COALESCE(sqlc.narg(status_reason), status_reason),
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

-- name: GetShipmentsForSync :many
SELECT * FROM shipments
WHERE driver_id = $1
  AND tenant_id = $2
  AND (status IN ('assigned', 'in_transit', 'delayed', 'arrived') OR updated_at > $3)
ORDER BY created_at DESC;