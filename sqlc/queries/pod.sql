-- name: CreatePOD :one
INSERT INTO proof_of_deliveries (
    tenant_id, shipment_id, driver_id,
    recipient_name, recipient_phone,
    signature_data, signature_url, photo_urls,
    delivery_address, delivery_coordinates,
    delivery_notes, location_verified, location_mismatch_meters, location_reason,
    recorded_at, synced_at, client_id, status, held_reason
) VALUES (
    sqlc.arg(tenant_id), sqlc.arg(shipment_id), sqlc.arg(driver_id),
    sqlc.arg(recipient_name), sqlc.narg(recipient_phone),
    sqlc.narg(signature_data), sqlc.narg(signature_url), sqlc.arg(photo_urls)::text[],
    sqlc.narg(delivery_address),
    CASE WHEN sqlc.narg(latitude)::float8 IS NULL OR sqlc.narg(longitude)::float8 IS NULL THEN NULL
         ELSE ST_SetSRID(ST_MakePoint(sqlc.narg(longitude)::float8, sqlc.narg(latitude)::float8), 4326)
    END,
    sqlc.narg(delivery_notes), sqlc.arg(location_verified), sqlc.narg(location_mismatch_meters), sqlc.narg(location_reason),
    sqlc.arg(recorded_at), NOW(), sqlc.narg(client_id), sqlc.arg(status), sqlc.narg(held_reason)
) RETURNING id;

-- name: GetPODByShipmentID :one
SELECT p.id, p.tenant_id, p.shipment_id, p.driver_id,
       p.recipient_name, p.recipient_phone,
       p.signature_data, p.signature_url, p.photo_urls,
       p.delivery_address,
       -- 0,0 stands for "no fix": it is in the Gulf of Guinea, never a delivery here.
       COALESCE(ST_Y(p.delivery_coordinates), 0)::float8 AS delivery_latitude,
       COALESCE(ST_X(p.delivery_coordinates), 0)::float8 AS delivery_longitude,
       p.delivery_notes, p.location_verified, p.location_mismatch_meters, p.location_reason,
       p.recorded_at, p.synced_at,
       u.full_name AS driver_name
FROM proof_of_deliveries p
LEFT JOIN users u ON u.id = p.driver_id
WHERE p.shipment_id = $1 AND p.tenant_id = $2 AND p.status = 'accepted';

-- name: GetPODByClientID :one
-- The device's identifier for a proof: a replayed upload finds the first.
SELECT id, shipment_id, status, held_reason
FROM proof_of_deliveries
WHERE tenant_id = $1 AND driver_id = $2 AND client_id = $3;

-- name: ListReviewPODs :many
-- Proofs of delivery held for the office to review, and those it dismissed.
SELECT p.id, p.driver_id, p.recipient_name, p.recipient_phone,
       p.signature_url, p.photo_urls, p.delivery_notes,
       COALESCE(ST_Y(p.delivery_coordinates), 0)::float8 AS delivery_latitude,
       COALESCE(ST_X(p.delivery_coordinates), 0)::float8 AS delivery_longitude,
       p.location_verified, p.location_mismatch_meters, p.location_reason,
       p.recorded_at, p.status, p.held_reason, p.reviewed_at,
       u.full_name AS driver_name, r.full_name AS reviewer_name
FROM proof_of_deliveries p
LEFT JOIN users u ON u.id = p.driver_id
LEFT JOIN users r ON r.id = p.reviewed_by
WHERE p.shipment_id = $1 AND p.tenant_id = $2 AND p.status <> 'accepted'
ORDER BY p.recorded_at DESC;

-- name: GetHeldPOD :one
SELECT p.id, p.driver_id, p.recipient_name, p.recipient_phone, p.recorded_at,
       u.full_name AS driver_name, u.is_active AS driver_active
FROM proof_of_deliveries p
JOIN users u ON u.id = p.driver_id
WHERE p.id = $1 AND p.shipment_id = $2 AND p.tenant_id = $3 AND p.status = 'held'
FOR UPDATE OF p;

-- name: CountHeldPODs :one
SELECT COUNT(*) FROM proof_of_deliveries
WHERE shipment_id = $1 AND tenant_id = $2 AND status = 'held';

-- name: ReviewPOD :exec
UPDATE proof_of_deliveries
SET status = sqlc.arg(status), reviewed_at = NOW(), reviewed_by = sqlc.arg(reviewed_by)
WHERE id = sqlc.arg(id) AND tenant_id = sqlc.arg(tenant_id);

-- name: ListRecentRecipients :many
-- Who signed for each customer's deliveries in the last year, newest first,
-- so the driver app can suggest the name (AC-020.4).
SELECT s.customer_phone, p.recipient_name, MAX(p.recorded_at)::timestamptz AS last_at
FROM proof_of_deliveries p
JOIN shipments s ON s.id = p.shipment_id AND s.tenant_id = p.tenant_id
WHERE p.tenant_id = sqlc.arg(tenant_id)
  AND p.status = 'accepted'
  AND s.customer_phone = ANY(sqlc.arg(phones)::text[])
  AND p.recorded_at > NOW() - INTERVAL '365 days'
GROUP BY s.customer_phone, p.recipient_name
ORDER BY s.customer_phone, last_at DESC;
