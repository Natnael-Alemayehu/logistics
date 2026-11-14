-- name: CreateAlert :one
-- An open alert for the same subject and type already covers this one, and
-- an alert of the same episode was raised before, even if dismissed since:
-- the unique indexes turn either duplicate into a no-op and no row returns.
INSERT INTO alerts (tenant_id, shipment_id, driver_id, alert_type, severity, message, episode)
VALUES ($1, $2, $3, $4, $5, $6, sqlc.narg(episode))
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
-- One person's badge: open alerts raised since they last opened the list.
SELECT COUNT(*) FROM alerts a
WHERE a.tenant_id = sqlc.arg(tenant_id) AND a.read_at IS NULL
  AND a.created_at > COALESCE(
      (SELECT u.alerts_seen_at FROM users u WHERE u.id = sqlc.arg(user_id) AND u.tenant_id = sqlc.arg(tenant_id)),
      '-infinity'::timestamptz);

-- name: MarkAlertsSeen :exec
UPDATE users SET alerts_seen_at = NOW()
WHERE id = sqlc.arg(user_id) AND tenant_id = sqlc.arg(tenant_id);

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

-- name: AcknowledgeStaleETAAlerts :exec
-- A "past ETA" alert is moot once its shipment has a new ETA, is no longer on
-- the road, or is no longer late.
UPDATE alerts a
SET acknowledged_at = NOW(), read_at = COALESCE(a.read_at, NOW())
FROM shipments s
WHERE a.shipment_id = s.id
  AND a.alert_type = 'eta_exceeded'
  AND a.acknowledged_at IS NULL
  AND (s.status NOT IN ('in_transit', 'delayed', 'arrived', 'issue')
       OR s.estimated_delivery IS NULL
       OR s.estimated_delivery > NOW()
       OR a.episode IS DISTINCT FROM floor(extract(epoch FROM s.estimated_delivery))::bigint::text);

-- name: AcknowledgeStalePickupAlerts :exec
-- Likewise a "not collected" alert once the load has left or been rescheduled.
UPDATE alerts a
SET acknowledged_at = NOW(), read_at = COALESCE(a.read_at, NOW())
FROM shipments s
WHERE a.shipment_id = s.id
  AND a.alert_type = 'pickup_overdue'
  AND a.acknowledged_at IS NULL
  AND (s.status NOT IN ('pending', 'assigned')
       OR s.planned_pickup_at IS NULL
       OR a.episode IS DISTINCT FROM floor(extract(epoch FROM s.planned_pickup_at))::bigint::text);
