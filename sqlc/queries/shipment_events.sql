-- name: CreateShipmentEvent :one
INSERT INTO shipment_events (
    tenant_id, shipment_id, event_type, from_status, to_status,
    driver_id, previous_driver_id, actor_id, actor_name, source,
    note, reason, latitude, longitude, visible_to_driver, public, details, client_id, occurred_at
) VALUES (
    $1, $2, $3, $4, $5,
    $6, $7, $8, $9, $10,
    $11, $12, $13, $14, $15, $16, $17, $18, $19
)
ON CONFLICT (shipment_id, client_id) WHERE client_id IS NOT NULL DO NOTHING
RETURNING *;

-- name: ListShipmentEvents :many
SELECT * FROM shipment_events
WHERE shipment_id = $1 AND tenant_id = $2
ORDER BY occurred_at ASC, created_at ASC;

-- name: ListPublicShipmentEvents :many
SELECT event_type, to_status, reason, occurred_at, latitude, longitude
FROM shipment_events
WHERE shipment_id = $1 AND public
ORDER BY occurred_at ASC, created_at ASC;

-- name: ListTenantActivity :many
SELECT e.*, s.tracking_number
FROM shipment_events e
JOIN shipments s ON s.id = e.shipment_id
WHERE e.tenant_id = $1
ORDER BY e.created_at DESC
LIMIT $2;

-- name: ListDriverMessagesSince :many
-- Notes and messages a dispatcher addressed to this driver: not the
-- driver's own. The LIMIT is maxDriverMessages in the sync service.
SELECT e.id, e.shipment_id, e.event_type, e.to_status, e.note, e.actor_name, e.created_at, s.tracking_number
FROM shipment_events e
JOIN shipments s ON s.id = e.shipment_id
WHERE e.tenant_id = sqlc.arg(tenant_id)
  AND e.driver_id = sqlc.arg(driver_id)
  AND e.visible_to_driver
  AND e.actor_id IS DISTINCT FROM sqlc.arg(driver_id)
  AND e.created_at > sqlc.arg(since)
ORDER BY e.created_at ASC
LIMIT 200;

-- name: ListShipmentsUnassignedFromDriverSince :many
-- Shipments taken away from a driver since their last sync and not handed
-- back. Their device should stop showing them.
SELECT DISTINCT e.shipment_id
FROM shipment_events e
JOIN shipments s ON s.id = e.shipment_id
WHERE e.tenant_id = sqlc.arg(tenant_id)
  AND e.previous_driver_id = sqlc.arg(driver_id)
  AND e.created_at > sqlc.arg(since)
  AND (s.driver_id IS NULL OR s.driver_id <> sqlc.arg(driver_id));

-- name: WasAssignedTo :one
-- Whether a driver ever had the shipment, for a proof of delivery sent after
-- it was handed to someone else.
SELECT EXISTS (
    SELECT 1 FROM shipment_events
    WHERE tenant_id = $1 AND shipment_id = $2 AND event_type = 'assigned' AND driver_id = $3
);

-- name: GetShipmentEventByClientID :one
-- What an offline device's change was recorded as, if it was: a device
-- sends a batch again when the answer to it was lost.
SELECT event_type FROM shipment_events
WHERE tenant_id = sqlc.arg(tenant_id) AND shipment_id = sqlc.arg(shipment_id) AND client_id = sqlc.arg(client_id)::text
LIMIT 1;
