-- name: EnqueueSMS :one
INSERT INTO sms_messages (tenant_id, shipment_id, direction, phone, body, purpose, sensitive, status)
VALUES ($1, $2, 'outbound', $3, $4, $5, $6, 'queued')
RETURNING *;

-- name: RecordSMS :one
-- For messages that are logged rather than sent through the queue: inbound
-- messages and outbound ones skipped for opt-out.
INSERT INTO sms_messages (tenant_id, shipment_id, direction, phone, body, purpose, status, last_error)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: ClaimQueuedSMS :many
-- SKIP LOCKED lets several API replicas drain the queue without sending the
-- same message twice.
-- The claim is a five-minute lease: next_attempt_at moves to when the row may
-- be reclaimed if this process dies before recording the outcome.
UPDATE sms_messages
SET status = 'sending', attempts = attempts + 1, next_attempt_at = NOW() + INTERVAL '5 minutes'
WHERE id IN (
    SELECT id FROM sms_messages
    WHERE status = 'queued' AND next_attempt_at <= NOW()
    ORDER BY next_attempt_at
    LIMIT $1
    FOR UPDATE SKIP LOCKED
)
RETURNING *;

-- name: MarkSMSSent :exec
UPDATE sms_messages
SET status = 'sent',
    sent_at = NOW(),
    provider = sqlc.arg(provider),
    provider_message_id = sqlc.narg(provider_message_id),
    last_error = NULL,
    body = CASE WHEN sensitive THEN '[redacted]' ELSE body END
WHERE id = sqlc.arg(id);

-- name: RecordSMSDelivery :execrows
-- A provider's delivery report. Its message IDs are unique per provider, so
-- this finds one company's text. Delivered is final; a text reported
-- undelivered may still arrive when the network tries again. The error is
-- the report's reason, none once delivered.
UPDATE sms_messages
SET delivery = sqlc.arg(delivery),
    delivery_at = sqlc.arg(delivery_at),
    last_error = sqlc.narg(last_error)
WHERE provider = sqlc.arg(provider) AND provider_message_id = sqlc.arg(provider_message_id)
  AND direction = 'outbound' AND (delivery IS NULL OR delivery = 'undelivered');

-- name: MarkSMSRetry :exec
UPDATE sms_messages
SET status = 'queued', last_error = $2, next_attempt_at = $3
WHERE id = $1;

-- name: MarkSMSFailed :exec
UPDATE sms_messages
SET status = 'failed',
    last_error = $2,
    body = CASE WHEN sensitive THEN '[redacted]' ELSE body END
WHERE id = $1;

-- name: MarkSMSSkipped :exec
UPDATE sms_messages
SET status = 'skipped',
    last_error = $2,
    body = CASE WHEN sensitive THEN '[redacted]' ELSE body END
WHERE id = $1;

-- name: RequeueStuckSMS :exec
-- A process that died mid-send leaves rows in 'sending' past their lease.
UPDATE sms_messages
SET status = 'queued'
WHERE status = 'sending' AND next_attempt_at < NOW();

-- name: ListSMSByTenant :many
SELECT m.*, s.tracking_number
FROM sms_messages m
LEFT JOIN shipments s ON s.id = m.shipment_id
WHERE m.tenant_id = $1
ORDER BY m.created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountSMSByTenant :one
SELECT COUNT(*) FROM sms_messages WHERE tenant_id = $1;

-- name: CountSMSSentSince :one
SELECT COUNT(*) FROM sms_messages
WHERE tenant_id = $1 AND direction = 'outbound' AND status = 'sent' AND sent_at >= $2;

-- name: IsPhoneOptedOut :one
SELECT EXISTS (SELECT 1 FROM sms_opt_outs WHERE phone = $1);

-- name: AddSMSOptOut :exec
INSERT INTO sms_opt_outs (phone) VALUES ($1) ON CONFLICT DO NOTHING;

-- name: RemoveSMSOptOut :exec
DELETE FROM sms_opt_outs WHERE phone = $1;

-- name: DeleteOldSMS :exec
DELETE FROM sms_messages WHERE created_at < $1;
