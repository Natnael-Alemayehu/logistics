-- name: CreateVerificationCode :one
INSERT INTO verification_codes (phone, purpose, code_hash, expires_at)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetActiveVerificationCode :one
SELECT * FROM verification_codes
WHERE phone = $1 AND purpose = $2 AND consumed_at IS NULL AND expires_at > NOW()
ORDER BY created_at DESC
LIMIT 1;

-- name: CountVerificationAttempt :one
-- Counts a try at a code, unless it has had all its tries: then no row.
UPDATE verification_codes SET attempts = attempts + 1
WHERE id = sqlc.arg(id) AND attempts < sqlc.arg(max_attempts)::int
RETURNING attempts;

-- name: ConsumeVerificationCode :execrows
UPDATE verification_codes SET consumed_at = NOW() WHERE id = $1 AND consumed_at IS NULL;

-- name: LockVerificationSubject :exec
-- Held until the transaction ends: requests for one number and purpose
-- take turns.
SELECT pg_advisory_xact_lock(hashtext(sqlc.arg(subject)::text));

-- name: CountVerificationCodesSince :one
SELECT COUNT(*) FROM verification_codes
WHERE phone = $1 AND purpose = $2 AND created_at > $3;

-- name: DeleteExpiredVerificationCodes :exec
DELETE FROM verification_codes WHERE expires_at < NOW() - INTERVAL '1 day';
