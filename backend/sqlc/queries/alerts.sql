-- name: CreateAlert :one
-- An open alert for the same subject and type already covers this one; the
-- partial unique index turns the duplicate into a no-op and no row returns.
INSERT INTO alerts (tenant_id, shipment_id, driver_id, alert_type, severity, message)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT DO NOTHING
RETURNING *;

-- name: ListAlerts :many
SELECT a.*, s.tracking_number, d.full_name AS driver_name
FROM alerts a
LEFT JOIN shipments s ON s.id = a.shipment_id
LEFT JOIN users d ON d.id = a.driver_id
WHERE a.tenant_id = sqlc.arg(tenant_id)
  AND (NOT sqlc.arg(open_only)::bool OR a.acknowledged_at IS NULL)
  AND (cardinality(sqlc.arg(severities)::text[]) = 0 OR a.severity = ANY(sqlc.arg(severities)::text[]))
ORDER BY a.created_at DESC
LIMIT sqlc.arg(row_limit) OFFSET sqlc.arg(row_offset);

-- name: CountAlerts :one
SELECT COUNT(*) FROM alerts
WHERE tenant_id = sqlc.arg(tenant_id)
  AND (NOT sqlc.arg(open_only)::bool OR acknowledged_at IS NULL)
  AND (cardinality(sqlc.arg(severities)::text[]) = 0 OR severity = ANY(sqlc.arg(severities)::text[]));

-- name: CountUnreadAlerts :one
SELECT COUNT(*) FROM alerts
WHERE tenant_id = $1 AND read_at IS NULL;

-- name: MarkAlertRead :exec
UPDATE alerts SET read_at = COALESCE(read_at, NOW())
WHERE id = $1 AND tenant_id = $2;

-- name: MarkAllAlertsRead :exec
UPDATE alerts SET read_at = NOW()
WHERE tenant_id = $1 AND read_at IS NULL;

-- name: AcknowledgeAlert :exec
UPDATE alerts
SET acknowledged_at = COALESCE(acknowledged_at, NOW()),
    acknowledged_by = COALESCE(acknowledged_by, $3),
    read_at = COALESCE(read_at, NOW())
WHERE id = $1 AND tenant_id = $2;

-- name: AcknowledgeShipmentAlerts :exec
-- A shipment that reaches a final state resolves whatever was open on it.
UPDATE alerts
SET acknowledged_at = NOW(), read_at = COALESCE(read_at, NOW())
WHERE shipment_id = $1 AND acknowledged_at IS NULL
  AND alert_type = ANY(sqlc.arg(alert_types)::text[]);

-- name: DeleteOldAlerts :exec
DELETE FROM alerts WHERE created_at < $1;

-- name: AcknowledgeDriverAlerts :exec
-- A driver who syncs again is no longer offline.
UPDATE alerts
SET acknowledged_at = NOW(), read_at = COALESCE(read_at, NOW())
WHERE driver_id = $1 AND alert_type = $2 AND acknowledged_at IS NULL;
