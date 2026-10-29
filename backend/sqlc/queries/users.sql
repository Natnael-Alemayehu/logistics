-- name: GetUserByID :one
SELECT * FROM users
WHERE id = $1 AND tenant_id = $2;

-- name: GetUserByPhone :one
SELECT * FROM users
WHERE phone = $1;

-- name: GetUserByEmail :one
SELECT * FROM users
WHERE lower(email) = lower(sqlc.arg(email));

-- name: ListUsersByTenant :many
SELECT * FROM users
WHERE tenant_id = $1 AND role <> 'driver'
ORDER BY is_active DESC, full_name
LIMIT $2 OFFSET $3;

-- name: CountStaffByTenant :one
SELECT COUNT(*) FROM users WHERE tenant_id = $1 AND role <> 'driver';

-- name: CreateUser :one
INSERT INTO users (
    tenant_id, role, full_name, phone, email, password_hash, pin_hash,
    is_active, password_reset_required, vehicle_id
) VALUES ($1, $2, $3, $4, $5, $6, $7, true, $8, $9)
RETURNING *;

-- name: UpdateUserProfile :one
UPDATE users
SET full_name = $3,
    phone = $4,
    email = $5,
    updated_at = NOW()
WHERE id = $1 AND tenant_id = $2
RETURNING *;

-- name: SetUserRole :exec
UPDATE users SET role = $3 WHERE id = $1 AND tenant_id = $2 AND role <> 'driver';

-- name: SetUserActive :exec
UPDATE users SET is_active = $3 WHERE id = $1 AND tenant_id = $2;

-- name: SetDriverVehicle :exec
UPDATE users SET vehicle_id = $3 WHERE id = $1 AND tenant_id = $2 AND role = 'driver';

-- name: ClearVehicleFromDrivers :exec
UPDATE users SET vehicle_id = NULL WHERE vehicle_id = $1 AND tenant_id = $2;

-- name: CountActiveDriversByTenant :one
SELECT COUNT(*) FROM users WHERE tenant_id = $1 AND role = 'driver' AND is_active;

-- name: IncrementFailedLoginAttempts :one
UPDATE users
SET failed_login_attempts = failed_login_attempts + 1
WHERE id = $1
RETURNING *;

-- name: RecordSuccessfulLogin :exec
UPDATE users
SET failed_login_attempts = 0, locked_until = NULL, last_login_at = NOW()
WHERE id = $1;

-- name: LockUserAccount :exec
UPDATE users
SET locked_until = $2
WHERE id = $1;

-- name: UpdatePassword :exec
UPDATE users
SET password_hash = $2, password_reset_required = $3, failed_login_attempts = 0, locked_until = NULL
WHERE id = $1;

-- name: UpdatePIN :exec
UPDATE users
SET pin_hash = $2, password_reset_required = $3, failed_login_attempts = 0, locked_until = NULL
WHERE id = $1;

-- name: ListDriversWithStatus :many
-- The drivers page and the assignment picker: who is on duty, how loaded they
-- are, and when their device last talked to the server.
SELECT u.id, u.full_name, u.phone, u.is_active, u.vehicle_id, u.created_at,
       v.plate_number AS vehicle_plate,
       sm.last_sync_at, sm.battery_level, sm.tracking_paused, sm.app_version,
       (SELECT COUNT(*) FROM shipments s
        WHERE s.driver_id = u.id
          AND s.status IN ('assigned', 'in_transit', 'delayed', 'arrived', 'issue'))::bigint AS active_shipments,
       (SELECT COUNT(*) FROM shipments s
        WHERE s.driver_id = u.id AND s.status = 'delivered'
          AND s.actual_delivery >= date_trunc('month', NOW()))::bigint AS delivered_this_month
FROM users u
LEFT JOIN vehicles v ON v.id = u.vehicle_id
LEFT JOIN sync_metadata sm ON sm.driver_id = u.id
WHERE u.tenant_id = sqlc.arg(tenant_id)
  AND u.role = 'driver'
  AND (NOT sqlc.arg(active_only)::bool OR u.is_active)
  AND (sqlc.narg(query)::text IS NULL
       OR u.full_name ILIKE '%' || sqlc.narg(query)::text || '%'
       OR u.phone ILIKE '%' || sqlc.narg(query)::text || '%')
ORDER BY u.is_active DESC, u.full_name;

-- name: ListAllUsersForPlatform :many
SELECT u.id, u.tenant_id, u.role, u.full_name, u.email, u.phone, u.is_active
FROM users u
WHERE u.tenant_id = $1
ORDER BY u.role, u.full_name;
