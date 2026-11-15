-- name: EnqueueEmail :one
INSERT INTO emails (tenant_id, to_address, subject, body, purpose, sensitive)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ClaimQueuedEmails :many
-- The same five-minute lease as ClaimQueuedSMS.
UPDATE emails
SET status = 'sending', attempts = attempts + 1, next_attempt_at = NOW() + INTERVAL '5 minutes'
WHERE id IN (
    SELECT id FROM emails
    WHERE status = 'queued' AND next_attempt_at <= NOW()
    ORDER BY next_attempt_at
    LIMIT $1
    FOR UPDATE SKIP LOCKED
)
RETURNING *;

-- name: MarkEmailSent :exec
UPDATE emails
SET status = 'sent',
    sent_at = NOW(),
    provider = $2,
    last_error = NULL,
    body = CASE WHEN sensitive THEN '[redacted]' ELSE body END
WHERE id = $1;

-- name: MarkEmailRetry :exec
UPDATE emails
SET status = 'queued', last_error = $2, next_attempt_at = $3
WHERE id = $1;

-- name: MarkEmailFailed :exec
UPDATE emails
SET status = 'failed',
    last_error = $2,
    body = CASE WHEN sensitive THEN '[redacted]' ELSE body END
WHERE id = $1;

-- name: ReleaseEmails :exec
-- As ReleaseSMS.
UPDATE emails
SET status = 'queued', attempts = attempts - 1, next_attempt_at = NOW()
WHERE id = ANY(sqlc.arg(ids)::uuid[]) AND status = 'sending';

-- name: RequeueStuckEmails :exec
UPDATE emails
SET status = 'queued'
WHERE status = 'sending' AND next_attempt_at < NOW();

-- name: DeleteOldEmails :exec
DELETE FROM emails WHERE created_at < $1;
