-- A shipment's documents and charges. Every query names the tenant.

-- name: ListShipmentDocuments :many
SELECT * FROM shipment_documents
WHERE tenant_id = $1 AND shipment_id = $2
ORDER BY created_at DESC;

-- name: GetShipmentDocument :one
SELECT * FROM shipment_documents
WHERE tenant_id = $1 AND shipment_id = $2 AND id = $3;

-- name: CountShipmentDocuments :one
SELECT count(*) FROM shipment_documents WHERE tenant_id = $1 AND shipment_id = $2;

-- name: CreateShipmentDocument :one
INSERT INTO shipment_documents (tenant_id, shipment_id, kind, name, storage_key, content_type, size_bytes, uploaded_by, uploaded_by_name)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: DeleteShipmentDocument :one
DELETE FROM shipment_documents
WHERE tenant_id = $1 AND shipment_id = $2 AND id = $3
RETURNING storage_key;

-- name: ListShipmentCharges :many
SELECT * FROM shipment_charges
WHERE tenant_id = $1 AND shipment_id = $2
ORDER BY created_at, id;

-- name: CreateShipmentCharge :one
INSERT INTO shipment_charges (tenant_id, shipment_id, description, amount, created_by)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: DeleteShipmentCharge :execrows
DELETE FROM shipment_charges
WHERE tenant_id = $1 AND shipment_id = $2 AND id = $3;

-- name: CountShipmentCharges :one
SELECT count(*) FROM shipment_charges WHERE tenant_id = $1 AND shipment_id = $2;
