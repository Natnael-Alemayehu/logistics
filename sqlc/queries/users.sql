-- name: GetUserByID :one
SELECT * FROM users
WHERE id = $1 AND tenant_id = $2;

-- name: GetUserByPhone :one
SELECT * FROM users
WHERE phone = $1;

-- name: GetUserByEmail :one
SELECT * FROM users
WHERE email = $1;

-- name: GetUserByPhoneAndTenant :one
SELECT * FROM users
WHERE phone = $1 AND tenant_id = $2;

-- name: GetUserByEmailAndTenant :one
SELECT * FROM users
WHERE email = $1 AND tenant_id = $2;

-- name: ListUsersByTenant :many
SELECT * FROM users
WHERE tenant_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListDriversByTenant :many
SELECT * FROM users
WHERE tenant_id = $1 AND role = 'driver'
ORDER BY full_name
LIMIT $2 OFFSET $3;

-- name: CreateUser :one
INSERT INTO users (
    tenant_id, role, full_name, phone, email, password_hash, pin_hash, is_active
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: UpdateUser :one
UPDATE users
SET 
    full_name = COALESCE(sqlc.narg(full_name), full_name),
    phone = COALESCE(sqlc.narg(phone), phone),
    email = COALESCE(sqlc.narg(email), email),
    password_hash = COALESCE(sqlc.narg(password_hash), password_hash),
    pin_hash = COALESCE(sqlc.narg(pin_hash), pin_hash),
    is_active = COALESCE(sqlc.narg(is_active), is_active)
WHERE id = $1 AND tenant_id = $2
RETURNING *;

-- name: DeleteUser :exec
UPDATE users
SET is_active = false
WHERE id = $1 AND tenant_id = $2;

-- name: CountUsersByTenant :one
SELECT COUNT(*) FROM users WHERE tenant_id = $1 AND is_active = true;

-- name: CountDriversByTenant :one
SELECT COUNT(*) FROM users WHERE tenant_id = $1 AND role = 'driver' AND is_active = true;

-- name: IncrementFailedLoginAttempts :one
UPDATE users
SET failed_login_attempts = COALESCE(failed_login_attempts, 0) + 1
WHERE id = $1
RETURNING *;

-- name: ResetFailedLoginAttempts :exec
UPDATE users
SET failed_login_attempts = 0, locked_until = NULL
WHERE id = $1;

-- name: LockUserAccount :exec
UPDATE users
SET locked_until = $2
WHERE id = $1;

-- name: UnlockUserAccount :exec
UPDATE users
SET failed_login_attempts = 0, locked_until = NULL
WHERE id = $1;

-- name: UpdatePassword :one
UPDATE users
SET password_hash = $2, password_reset_required = false, failed_login_attempts = 0, locked_until = NULL
WHERE id = $1
RETURNING *;

-- name: UpdatePIN :one
UPDATE users
SET pin_hash = $2, failed_login_attempts = 0, locked_until = NULL
WHERE id = $1
RETURNING *;

-- name: CountDriversWithActiveShipments :one
SELECT COUNT(DISTINCT s.driver_id) 
FROM shipments s
INNER JOIN users u ON s.driver_id = u.id
WHERE s.tenant_id = $1 
  AND s.status IN ('assigned', 'in_transit', 'delayed', 'arrived')
  AND u.is_active = true;