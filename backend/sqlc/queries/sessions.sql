-- name: CreateSession :one
INSERT INTO sessions (
    user_id, tenant_id, refresh_token_hash, device_id, user_agent, ip_address,
    expires_at, kind, remember, last_seen_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW())
RETURNING *;

-- name: GetActiveSessionByTokenHash :one
SELECT * FROM sessions
WHERE refresh_token_hash = $1
  AND kind = $2
  AND revoked_at IS NULL
  AND expires_at > NOW();

-- name: GetSessionByID :one
SELECT * FROM sessions WHERE id = $1;

-- name: GetSessionPrincipal :one
-- Everything a request needs to know about who is calling, in one lookup.
SELECT s.id AS session_id, s.kind, s.remember, s.last_seen_at, s.expires_at,
       u.id AS user_id, u.tenant_id, u.role, u.full_name, u.phone, u.email,
       u.is_active AS user_active, u.password_reset_required,
       (u.email_verified_at IS NOT NULL)::bool AS email_verified,
       t.name AS tenant_name, t.is_active AS tenant_active, t.plan AS tenant_plan,
       t.trial_ends_at,
       -- The oldest unpaid renewal or usage invoice, on which read-only mode
       -- depends.
       COALESCE((SELECT i.number FROM invoices i
                 WHERE i.tenant_id = t.id AND i.status = 'open' AND i.kind IN ('renewal', 'usage')
                 ORDER BY i.due_at LIMIT 1), '')::text AS debt_number,
       (SELECT i.due_at FROM invoices i
        WHERE i.tenant_id = t.id AND i.status = 'open' AND i.kind IN ('renewal', 'usage')
        ORDER BY i.due_at LIMIT 1)::timestamptz AS debt_due_at
FROM sessions s
JOIN users u ON u.id = s.user_id
JOIN tenants t ON t.id = u.tenant_id
WHERE s.refresh_token_hash = $1
  AND s.kind = $2
  AND s.revoked_at IS NULL
  AND s.expires_at > NOW();

-- name: TouchSession :exec
-- Sliding expiry: a session in use is pushed out, never pulled in.
UPDATE sessions
SET last_seen_at = NOW(), expires_at = GREATEST(expires_at, sqlc.arg(expires_at))
WHERE id = sqlc.arg(id);

-- name: ListActiveSessionsByUser :many
SELECT * FROM sessions
WHERE user_id = $1
  AND revoked_at IS NULL
  AND expires_at > NOW()
ORDER BY COALESCE(last_seen_at, created_at) DESC;

-- name: RevokeSession :exec
UPDATE sessions
SET revoked_at = NOW()
WHERE id = $1 AND revoked_at IS NULL;

-- name: RevokeSessionByUser :exec
UPDATE sessions
SET revoked_at = NOW()
WHERE id = $1 AND user_id = $2 AND revoked_at IS NULL;

-- name: RevokeAllUserSessions :exec
UPDATE sessions
SET revoked_at = NOW()
WHERE user_id = $1 AND revoked_at IS NULL;

-- name: RevokeOtherUserSessions :exec
UPDATE sessions
SET revoked_at = NOW()
WHERE user_id = $1 AND id != $2 AND revoked_at IS NULL;

-- name: DeleteExpiredSessions :exec
DELETE FROM sessions WHERE expires_at < NOW() - INTERVAL '7 days'
   OR revoked_at < NOW() - INTERVAL '7 days';

-- name: SessionStillValid :one
-- Whether a session still lets its holder in, for connections that outlive
-- the request that opened them: not signed out or expired, the person
-- active, and their company not suspended.
SELECT EXISTS (
    SELECT 1 FROM sessions s
    JOIN users u ON u.id = s.user_id
    JOIN tenants t ON t.id = u.tenant_id
    WHERE s.id = $1 AND s.revoked_at IS NULL AND s.expires_at > NOW()
      AND u.is_active AND (t.is_active OR u.role = 'platform_admin')
);
