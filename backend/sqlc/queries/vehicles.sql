-- name: GetVehicleByID :one
SELECT * FROM vehicles
WHERE id = $1 AND tenant_id = $2;

-- name: ListVehiclesByTenant :many
SELECT * FROM vehicles
WHERE tenant_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListActiveVehiclesByTenant :many
SELECT * FROM vehicles
WHERE tenant_id = $1 AND is_active = true
ORDER BY plate_number;

-- name: CreateVehicle :one
INSERT INTO vehicles (
    tenant_id, plate_number, vehicle_type, is_active
) VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: UpdateVehicle :one
UPDATE vehicles
SET
    plate_number = COALESCE(sqlc.narg(plate_number), plate_number),
    vehicle_type = COALESCE(sqlc.narg(vehicle_type), vehicle_type),
    is_active = COALESCE(sqlc.narg(is_active), is_active)
WHERE id = $1 AND tenant_id = $2
RETURNING *;

-- name: DeleteVehicle :exec
UPDATE vehicles
SET is_active = false
WHERE id = $1 AND tenant_id = $2;

-- name: HardDeleteVehicle :exec
DELETE FROM vehicles
WHERE id = $1 AND tenant_id = $2;

-- name: CountVehiclesByTenant :one
SELECT COUNT(*) FROM vehicles WHERE tenant_id = $1 AND is_active = true;