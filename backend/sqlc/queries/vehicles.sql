-- name: GetVehicleByID :one
SELECT * FROM vehicles
WHERE id = $1 AND tenant_id = $2;

-- name: ListVehiclesWithDrivers :many
SELECT v.*,
       COALESCE((SELECT string_agg(u.full_name, ', ' ORDER BY u.full_name)
        FROM users u WHERE u.vehicle_id = v.id AND u.is_active), '')::text AS driver_names,
       (SELECT COUNT(*) FROM shipments s
        WHERE s.vehicle_id = v.id
          AND s.status IN ('assigned', 'in_transit', 'delayed', 'arrived', 'issue'))::bigint AS active_shipments
FROM vehicles v
WHERE v.tenant_id = $1
ORDER BY v.is_active DESC, v.plate_number;

-- name: ListActiveVehiclesByTenant :many
SELECT * FROM vehicles
WHERE tenant_id = $1 AND is_active
ORDER BY plate_number;

-- name: CreateVehicle :one
INSERT INTO vehicles (tenant_id, plate_number, vehicle_type, capacity_kg, is_active)
VALUES ($1, $2, $3, $4, true)
RETURNING *;

-- name: UpdateVehicle :one
UPDATE vehicles
SET plate_number = $3,
    vehicle_type = $4,
    capacity_kg = $5,
    is_active = $6
WHERE id = $1 AND tenant_id = $2
RETURNING *;

-- name: CountVehiclesByTenant :one
SELECT COUNT(*) FROM vehicles WHERE tenant_id = $1 AND is_active;
