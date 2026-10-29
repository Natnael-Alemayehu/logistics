-- name: CreatePOD :one
INSERT INTO proof_of_deliveries (
    tenant_id, shipment_id, driver_id,
    recipient_name, recipient_phone,
    signature_data, signature_url, photo_urls,
    delivery_address, delivery_coordinates,
    delivery_notes, location_verified, location_mismatch_meters,
    recorded_at, synced_at
) VALUES (
    sqlc.arg(tenant_id), sqlc.arg(shipment_id), sqlc.arg(driver_id),
    sqlc.arg(recipient_name), sqlc.narg(recipient_phone),
    sqlc.narg(signature_data), sqlc.narg(signature_url), sqlc.arg(photo_urls)::text[],
    sqlc.narg(delivery_address),
    CASE WHEN sqlc.narg(latitude)::float8 IS NULL OR sqlc.narg(longitude)::float8 IS NULL THEN NULL
         ELSE ST_SetSRID(ST_MakePoint(sqlc.narg(longitude)::float8, sqlc.narg(latitude)::float8), 4326)
    END,
    sqlc.narg(delivery_notes), sqlc.arg(location_verified), sqlc.narg(location_mismatch_meters),
    sqlc.arg(recorded_at), NOW()
) RETURNING id;

-- name: GetPODByShipmentID :one
SELECT p.id, p.tenant_id, p.shipment_id, p.driver_id,
       p.recipient_name, p.recipient_phone,
       p.signature_data, p.signature_url, p.photo_urls,
       p.delivery_address,
       -- 0,0 stands for "no fix": it is in the Gulf of Guinea, never a delivery here.
       COALESCE(ST_Y(p.delivery_coordinates), 0)::float8 AS delivery_latitude,
       COALESCE(ST_X(p.delivery_coordinates), 0)::float8 AS delivery_longitude,
       p.delivery_notes, p.location_verified, p.location_mismatch_meters,
       p.recorded_at, p.synced_at,
       u.full_name AS driver_name
FROM proof_of_deliveries p
LEFT JOIN users u ON u.id = p.driver_id
WHERE p.shipment_id = $1 AND p.tenant_id = $2;
