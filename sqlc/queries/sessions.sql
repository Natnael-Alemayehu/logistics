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

-- name: RevokeSession :exec
UPDATE sessions
SET revoked_at = NOW()
WHERE id = $1;

-- name: RevokeAllUserSessions :exec
UPDATE sessions
SET revoked_at = NOW()
WHERE user_id = $1 AND revoked_at IS NULL;

-- name: DeleteExpiredSessions :exec
DELETE FROM sessions WHERE expires_at < NOW();