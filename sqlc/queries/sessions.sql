-- name: CreateSession :one
INSERT INTO sessions (
    user_id, tenant_id, refresh_token_hash, device_id, user_agent, ip_address, expires_at
) VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetSessionByTokenHash :one
SELECT * FROM sessions WHERE refresh_token_hash = $1;

-- name: GetActiveSessionByTokenHash :one
SELECT * FROM sessions 
WHERE refresh_token_hash = $1 
  AND revoked_at IS NULL 
  AND expires_at > NOW();

-- name: GetSessionByID :one
SELECT * FROM sessions WHERE id = $1;

-- name: ListSessionsByUser :many
SELECT * FROM sessions
WHERE user_id = $1 AND revoked_at IS NULL
ORDER BY created_at DESC;

-- name: ListActiveSessionsByUser :many
SELECT * FROM sessions
WHERE user_id = $1 
  AND revoked_at IS NULL 
  AND expires_at > NOW()
ORDER BY created_at DESC;

-- name: RevokeSession :exec
UPDATE sessions
SET revoked_at = NOW()
WHERE id = $1;

-- name: RevokeSessionByUser :exec
UPDATE sessions
SET revoked_at = NOW()
WHERE id = $1 AND user_id = $2;

-- name: RevokeAllUserSessions :exec
UPDATE sessions
SET revoked_at = NOW()
WHERE user_id = $1 AND revoked_at IS NULL;

-- name: RevokeOtherUserSessions :exec
UPDATE sessions
SET revoked_at = NOW()
WHERE user_id = $1 AND id != $2 AND revoked_at IS NULL;

-- name: DeleteExpiredSessions :exec
DELETE FROM sessions WHERE expires_at < NOW();

-- name: CountActiveSessionsByUser :one
SELECT COUNT(*) FROM sessions
WHERE user_id = $1 AND revoked_at IS NULL AND expires_at > NOW();