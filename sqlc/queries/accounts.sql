-- Links sent by email, and sign-ups waiting for their SMS code.

-- name: CreateEmailToken :exec
INSERT INTO email_tokens (tenant_id, user_id, purpose, email, token_hash, expires_at)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: CountEmailTokensSince :one
SELECT COUNT(*) FROM email_tokens
WHERE tenant_id = $1 AND user_id = $2 AND purpose = $3 AND created_at > $4;

-- name: GetEmailToken :one
-- A link that still works. Whoever follows it is not signed in, so the
-- token itself says which company and person it is for.
SELECT e.id, e.tenant_id, e.user_id, e.email, u.full_name, u.role, u.is_active,
       u.email AS current_email, t.is_active AS tenant_active
FROM email_tokens e
JOIN users u ON u.id = e.user_id AND u.tenant_id = e.tenant_id
JOIN tenants t ON t.id = e.tenant_id
WHERE e.token_hash = $1 AND e.purpose = $2 AND e.used_at IS NULL AND e.expires_at > NOW()
FOR UPDATE OF e;

-- name: ExpireEmailTokens :exec
-- Once a reset link is used, the others sent to that person stop working.
UPDATE email_tokens SET used_at = NOW()
WHERE tenant_id = $1 AND user_id = $2 AND purpose = $3 AND used_at IS NULL;

-- name: SetEmailVerified :execrows
UPDATE users SET email_verified_at = NOW(), updated_at = NOW()
WHERE id = $1 AND tenant_id = $2 AND email = sqlc.arg(email) AND email_verified_at IS NULL;

-- name: ResetUserPassword :exec
-- Following a reset link proves the address, and ends any lockout.
UPDATE users
SET password_hash = $3, password_reset_required = false,
    failed_login_attempts = 0, locked_until = NULL,
    email_verified_at = COALESCE(email_verified_at, NOW()), updated_at = NOW()
WHERE id = $1 AND tenant_id = $2;

-- name: DeleteOldEmailTokens :exec
DELETE FROM email_tokens WHERE created_at < $1;

-- name: CreatePendingSignup :exec
INSERT INTO pending_signups (token_hash, company, city, full_name, email, phone, password_hash, plan, billing_cycle, expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10);

-- name: GetPendingSignup :one
SELECT * FROM pending_signups WHERE token_hash = $1 AND expires_at > NOW()
FOR UPDATE;

-- name: DeletePendingSignup :exec
DELETE FROM pending_signups WHERE id = $1;

-- name: DeleteExpiredPendingSignups :exec
DELETE FROM pending_signups WHERE expires_at < $1;
