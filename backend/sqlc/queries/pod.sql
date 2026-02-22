-- name: CreatePOD :one
INSERT INTO proof_of_deliveries (
    tenant_id, shipment_id, driver_id,
    recipient_name, recipient_phone,
    signature_data, signature_url, photo_urls,
    delivery_address, delivery_coordinates,
    delivery_notes, location_verified, location_mismatch_meters,
    recorded_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, 
    ST_MakePoint($10, $11)::geometry(Point, 4326),
    $12, $13, $14, $15
) RETURNING *;

-- name: GetPODByShipmentID :one
SELECT id, tenant_id, shipment_id, driver_id,
       recipient_name, recipient_phone,
       signature_data, signature_url, photo_urls,
       delivery_address,
       ST_X(delivery_coordinates::geometry) as delivery_longitude,
       ST_Y(delivery_coordinates::geometry) as delivery_latitude,
       delivery_notes, location_verified, location_mismatch_meters,
       recorded_at, synced_at
FROM proof_of_deliveries
WHERE shipment_id = $1 AND tenant_id = $2;

-- name: UpdatePODSyncedAt :exec
UPDATE proof_of_deliveries
SET synced_at = NOW()
WHERE id = $1;

-- name: GetUnsyncedPODs :many
SELECT id, tenant_id, shipment_id, driver_id,
       recipient_name, recipient_phone,
       signature_data, signature_url, photo_urls,
       delivery_address,
       ST_X(delivery_coordinates::geometry) as delivery_longitude,
       ST_Y(delivery_coordinates::geometry) as delivery_latitude,
       delivery_notes, location_verified, location_mismatch_meters,
       recorded_at, synced_at
FROM proof_of_deliveries
WHERE driver_id = $1 AND tenant_id = $2 AND synced_at IS NULL
ORDER BY recorded_at ASC;