-- Webhooks (FR-047): a company's endpoints, and the outbox of messages to
-- them.

-- name: CreateWebhookEndpoint :one
INSERT INTO webhook_endpoints (tenant_id, url, events, secret, created_by)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListWebhookEndpoints :many
SELECT e.*,
       (SELECT COUNT(*) FROM webhook_deliveries d WHERE d.endpoint_id = e.id AND d.status = 'failed'
          AND d.created_at > NOW() - INTERVAL '7 days')::bigint AS failed_week,
       (SELECT MAX(d.delivered_at) FROM webhook_deliveries d WHERE d.endpoint_id = e.id)::timestamptz AS last_delivered_at
FROM webhook_endpoints e
WHERE e.tenant_id = $1
ORDER BY e.created_at;

-- name: CountWebhookEndpoints :one
SELECT COUNT(*) FROM webhook_endpoints WHERE tenant_id = $1;

-- name: GetWebhookEndpoint :one
SELECT * FROM webhook_endpoints WHERE id = $1 AND tenant_id = $2;

-- name: UpdateWebhookEndpoint :one
UPDATE webhook_endpoints SET url = $3, events = $4
WHERE id = $1 AND tenant_id = $2
RETURNING *;

-- name: SetWebhookEndpointActive :one
UPDATE webhook_endpoints SET active = $3
WHERE id = $1 AND tenant_id = $2
RETURNING *;

-- name: SetWebhookSecret :one
UPDATE webhook_endpoints SET secret = $3
WHERE id = $1 AND tenant_id = $2
RETURNING *;

-- name: DeleteWebhookEndpoint :one
DELETE FROM webhook_endpoints WHERE id = $1 AND tenant_id = $2
RETURNING *;

-- name: EnqueueWebhooks :execrows
-- One message to each of the company's endpoints that asked for this
-- event. A paused endpoint's wait for it to be resumed.
INSERT INTO webhook_deliveries (tenant_id, endpoint_id, event_id, event_type, shipment_id, payload)
SELECT e.tenant_id, e.id, sqlc.arg(event_id), sqlc.arg(event_type)::text, sqlc.narg(shipment_id), sqlc.arg(payload)
FROM webhook_endpoints e
WHERE e.tenant_id = sqlc.arg(tenant_id) AND sqlc.arg(event_type)::text = ANY(e.events);

-- name: EnqueueWebhookTo :one
-- A message to one endpoint, whatever it asked for: a test.
INSERT INTO webhook_deliveries (tenant_id, endpoint_id, event_id, event_type, payload)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ClaimQueuedWebhooks :many
-- The five-minute lease of ClaimQueuedEmails. A paused endpoint's messages
-- wait for it.
UPDATE webhook_deliveries
SET status = 'sending', attempts = attempts + 1, next_attempt_at = NOW() + INTERVAL '5 minutes'
WHERE id IN (
    SELECT d.id FROM webhook_deliveries d
    JOIN webhook_endpoints e ON e.id = d.endpoint_id
    WHERE d.status = 'queued' AND d.next_attempt_at <= NOW() AND e.active
    ORDER BY d.next_attempt_at
    LIMIT $1
    FOR UPDATE OF d SKIP LOCKED
)
RETURNING *;

-- name: GetWebhookTarget :one
-- Where a claimed message goes, read as it is sent: a changed address or
-- secret applies to messages still waiting.
SELECT url, secret, active FROM webhook_endpoints WHERE id = $1;

-- name: MarkWebhookDelivered :exec
UPDATE webhook_deliveries
SET status = 'delivered', delivered_at = NOW(), response_status = $2, response_body = $3, duration_ms = $4, last_error = NULL
WHERE id = $1;

-- name: MarkWebhookRetry :exec
UPDATE webhook_deliveries
SET status = 'queued', response_status = $2, response_body = $3, duration_ms = $4, last_error = $5, next_attempt_at = $6
WHERE id = $1;

-- name: MarkWebhookFailed :exec
UPDATE webhook_deliveries
SET status = 'failed', response_status = $2, response_body = $3, duration_ms = $4, last_error = $5
WHERE id = $1;

-- name: ReleaseWebhooks :exec
-- As ReleaseEmails.
UPDATE webhook_deliveries
SET status = 'queued', attempts = attempts - 1, next_attempt_at = NOW()
WHERE id = ANY(sqlc.arg(ids)::uuid[]) AND status = 'sending';

-- name: RequeueStuckWebhooks :exec
UPDATE webhook_deliveries
SET status = 'queued'
WHERE status = 'sending' AND next_attempt_at < NOW();

-- name: ResendWebhook :one
-- Sends a message again, from the start of its retries.
UPDATE webhook_deliveries
SET status = 'queued', attempts = 0, next_attempt_at = NOW()
WHERE id = $1 AND tenant_id = $2 AND status IN ('delivered', 'failed')
RETURNING *;

-- name: ListWebhookDeliveries :many
SELECT d.*, s.tracking_number
FROM webhook_deliveries d
LEFT JOIN shipments s ON s.id = d.shipment_id
WHERE d.endpoint_id = $1 AND d.tenant_id = $2
ORDER BY d.created_at DESC, d.id
LIMIT $3 OFFSET $4;

-- name: DeleteOldWebhookDeliveries :exec
DELETE FROM webhook_deliveries WHERE created_at < $1;
