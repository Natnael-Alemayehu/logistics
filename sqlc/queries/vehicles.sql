-- name: GetVehicleByID :one
SELECT * FROM vehicles
WHERE id = $1 AND tenant_id = $2;

-- name: ListVehiclesWithDrivers :many
SELECT v.*,
       COALESCE((SELECT string_agg(u.full_name, ', ' ORDER BY u.full_name)
        FROM users u WHERE u.vehicle_id = v.id AND u.is_active), '')::text AS driver_names,
       (SELECT COUNT(*) FROM shipments s
        WHERE s.vehicle_id = v.id
          AND s.status IN ('assigned', 'in_transit', 'delayed', 'arrived', 'issue'))::bigint AS active_shipments,
       EXISTS (SELECT 1 FROM shipments s
               WHERE s.vehicle_id = v.id AND s.tenant_id = v.tenant_id
                 AND s.status IN ('in_transit', 'delayed', 'arrived', 'issue'))::bool AS on_road
FROM vehicles v
WHERE v.tenant_id = $1
ORDER BY v.is_active DESC, v.plate_number;

-- name: ListActiveVehiclesByTenant :many
SELECT * FROM vehicles
WHERE tenant_id = $1 AND is_active
ORDER BY plate_number;

-- name: CreateVehicle :one
INSERT INTO vehicles (tenant_id, plate_number, vehicle_type, capacity_kg, is_active, insurance_expires_on, inspection_due_on)
VALUES ($1, $2, $3, $4, true, sqlc.narg(insurance_expires_on), sqlc.narg(inspection_due_on))
RETURNING *;

-- name: UpdateVehicle :one
UPDATE vehicles
SET plate_number = $3,
    vehicle_type = $4,
    capacity_kg = $5,
    is_active = $6,
    insurance_expires_on = sqlc.narg(insurance_expires_on),
    inspection_due_on = sqlc.narg(inspection_due_on)
WHERE id = $1 AND tenant_id = $2
RETURNING *;

-- name: CountVehiclesByTenant :one
SELECT COUNT(*) FROM vehicles WHERE tenant_id = $1 AND is_active;

-- name: ListPapersDue :many
-- The papers check: every active driver's licence and every vehicle's
-- insurance and inspection that runs out on or before a day, across
-- companies. The alert's episode names the date, so each is raised once.
SELECT 'licence'::text AS kind, u.tenant_id, u.id AS subject_id, u.full_name AS name, u.licence_expires_on AS due_on
FROM users u
WHERE u.role = 'driver' AND u.is_active AND u.anonymised_at IS NULL
  AND u.licence_expires_on <= sqlc.arg(until)::date
UNION ALL
SELECT 'insurance', v.tenant_id, v.id, v.plate_number, v.insurance_expires_on
FROM vehicles v
WHERE v.is_active AND v.insurance_expires_on <= sqlc.arg(until)::date
UNION ALL
SELECT 'inspection', v.tenant_id, v.id, v.plate_number, v.inspection_due_on
FROM vehicles v
WHERE v.is_active AND v.inspection_due_on <= sqlc.arg(until)::date;
