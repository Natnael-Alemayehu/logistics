-- Data rights: a company's export, removing personal data after the keeping
-- period or on request, and deleting a company (FR-041, AC-049.3).

-- name: ExportShipments :many
SELECT s.tracking_number, s.status, COALESCE(s.status_reason, '')::text AS status_reason, s.priority,
       s.created_at, s.planned_pickup_at, s.picked_up_at, s.estimated_delivery, s.actual_delivery,
       COALESCE(s.origin_city, '')::text AS origin_city, s.origin_address,
       COALESCE(s.destination_city, '')::text AS destination_city, s.destination_address,
       s.customer_name, s.customer_phone,
       COALESCE(s.cargo_description, '')::text AS cargo_description,
       COALESCE(s.cargo_weight, 0)::float8 AS cargo_weight_kg, COALESCE(s.cargo_value, 0)::float8 AS cargo_value,
       COALESCE(s.special_instructions, '')::text AS special_instructions,
       COALESCE(u.full_name, '')::text AS driver_name, COALESCE(v.plate_number, '')::text AS vehicle_plate,
       s.customer_confirmed_at, COALESCE(s.customer_feedback, '')::text AS customer_feedback, s.anonymised_at
FROM shipments s
LEFT JOIN users u ON u.id = s.driver_id
LEFT JOIN vehicles v ON v.id = s.vehicle_id
WHERE s.tenant_id = $1
ORDER BY s.created_at;

-- name: ExportShipmentEvents :many
SELECT s.tracking_number, e.occurred_at, e.event_type,
       COALESCE(e.from_status, '')::text AS from_status, COALESCE(e.to_status, '')::text AS to_status,
       COALESCE(e.reason, '')::text AS reason, COALESCE(e.note, '')::text AS note,
       e.actor_name, e.source, e.latitude, e.longitude
FROM shipment_events e
JOIN shipments s ON s.id = e.shipment_id AND s.tenant_id = e.tenant_id
WHERE e.tenant_id = $1
ORDER BY s.created_at, e.occurred_at;

-- name: ExportPODs :many
SELECT p.id, s.tracking_number, p.status, p.recorded_at, p.synced_at, p.recipient_name,
       COALESCE(p.recipient_phone, '')::text AS recipient_phone, COALESCE(u.full_name, '')::text AS driver_name,
       -- 0,0 stands for "no fix", as on the shipment page.
       COALESCE(ST_Y(p.delivery_coordinates), 0)::float8 AS latitude,
       COALESCE(ST_X(p.delivery_coordinates), 0)::float8 AS longitude,
       p.location_verified, COALESCE(p.location_mismatch_meters, 0)::float8 AS mismatch_meters,
       COALESCE(p.location_reason, '')::text AS location_reason, COALESCE(p.delivery_notes, '')::text AS delivery_notes,
       COALESCE(p.signature_url, '')::text AS signature_key, COALESCE(p.photo_urls, '{}')::text[] AS photo_keys
FROM proof_of_deliveries p
JOIN shipments s ON s.id = p.shipment_id AND s.tenant_id = p.tenant_id
LEFT JOIN users u ON u.id = p.driver_id
WHERE p.tenant_id = $1
ORDER BY p.recorded_at;

-- name: ExportPeople :many
SELECT u.full_name, u.role, COALESCE(u.phone, '')::text AS phone, COALESCE(u.email, '')::text AS email,
       u.is_active, COALESCE(v.plate_number, '')::text AS vehicle_plate, u.created_at, u.last_login_at, u.anonymised_at
FROM users u
LEFT JOIN vehicles v ON v.id = u.vehicle_id
WHERE u.tenant_id = $1
ORDER BY u.role = 'driver', u.full_name;

-- name: ExportVehicles :many
SELECT plate_number, vehicle_type, COALESCE(capacity_kg, 0)::float8 AS capacity_kg, is_active, created_at
FROM vehicles
WHERE tenant_id = $1
ORDER BY plate_number;

-- name: ListShipmentsToAnonymise :many
-- Shipments that finished before the keeping period ended and still hold
-- customer and delivery details (AC-041.2, AC-041.3). Across companies: the
-- retention job runs for all of them.
SELECT id FROM shipments
WHERE anonymised_at IS NULL AND status IN ('delivered', 'cancelled')
  AND COALESCE(actual_delivery, status_changed_at, updated_at) < sqlc.arg(before)
ORDER BY updated_at
LIMIT sqlc.arg(max_rows);

-- name: ListPODMediaForShipments :many
SELECT COALESCE(signature_url, '')::text AS signature_key, COALESCE(photo_urls, '{}')::text[] AS photo_keys
FROM proof_of_deliveries
WHERE shipment_id = ANY(sqlc.arg(ids)::uuid[]);

-- name: AnonymisePODs :exec
UPDATE proof_of_deliveries
SET recipient_name = 'Removed', recipient_phone = NULL, signature_data = NULL, signature_url = NULL,
    photo_urls = '{}', delivery_notes = NULL, delivery_address = NULL, delivery_coordinates = NULL
WHERE shipment_id = ANY(sqlc.arg(ids)::uuid[]);

-- name: AnonymiseShipmentEvents :exec
-- Notes, positions and details may name the recipient; a customer's own
-- confirmation carries their name.
UPDATE shipment_events
SET note = NULL, details = NULL, latitude = NULL, longitude = NULL,
    actor_name = CASE WHEN source = 'customer' THEN 'Customer' ELSE actor_name END
WHERE shipment_id = ANY(sqlc.arg(ids)::uuid[]);

-- name: DeleteShipmentTexts :exec
DELETE FROM sms_messages WHERE shipment_id = ANY(sqlc.arg(ids)::uuid[]);

-- name: AnonymiseShipments :exec
-- What reports count stays: dates, statuses, reasons, cities, driver,
-- vehicle and cargo.
UPDATE shipments
SET customer_name = 'Former customer', customer_phone = '', special_instructions = NULL, status_note = NULL,
    customer_feedback = NULL, confirm_token = NULL, origin_address = '', destination_address = '',
    origin_lat = NULL, origin_lng = NULL, destination_lat = NULL, destination_lng = NULL,
    anonymised_at = NOW()
WHERE id = ANY(sqlc.arg(ids)::uuid[]);

-- name: AnonymiseUser :one
-- A person who has left: what identifies them goes, their history stays
-- (AC-041.6).
UPDATE users
SET full_name = sqlc.arg(full_name), phone = NULL, email = NULL, password_hash = NULL, pin_hash = NULL,
    email_verified_at = NULL, alert_emails = '{}', language = NULL, vehicle_id = NULL,
    anonymised_at = NOW(), updated_at = NOW()
WHERE id = sqlc.arg(id) AND tenant_id = sqlc.arg(tenant_id) AND NOT is_active AND anonymised_at IS NULL
RETURNING id;

-- name: AnonymiseActorName :exec
UPDATE shipment_events SET actor_name = sqlc.arg(actor_name)
WHERE actor_id = sqlc.arg(actor_id) AND tenant_id = sqlc.arg(tenant_id);

-- name: AnonymiseSupportAuthor :exec
UPDATE support_messages SET author_name = sqlc.arg(author_name)
WHERE author_id = sqlc.arg(author_id) AND tenant_id = sqlc.arg(tenant_id);

-- name: DeletePersonDeviceData :exec
DELETE FROM sync_metadata WHERE driver_id = $1;

-- name: DeleteMessagesToPerson :exec
-- Texts and emails that went to the person, with their number or address.
WITH texts AS (
    DELETE FROM sms_messages WHERE sms_messages.tenant_id = sqlc.arg(tenant_id) AND phone = sqlc.arg(phone)
)
DELETE FROM emails WHERE emails.tenant_id = sqlc.arg(tenant_id) AND to_address = sqlc.arg(email);

-- name: TenantForDeletion :one
SELECT id, name,
       (SELECT COUNT(*) FROM shipments WHERE tenant_id = t.id)::bigint AS shipments,
       (SELECT COUNT(*) FROM users WHERE tenant_id = t.id)::bigint AS people,
       (SELECT COUNT(*) FROM vehicles WHERE tenant_id = t.id)::bigint AS vehicles,
       (SELECT COUNT(*) FROM invoices WHERE tenant_id = t.id)::bigint AS invoices
FROM tenants t
WHERE t.id = $1
FOR UPDATE;

-- name: ListTenantPODMedia :many
SELECT COALESCE(signature_url, '')::text AS signature_key, COALESCE(photo_urls, '{}')::text[] AS photo_keys
FROM proof_of_deliveries
WHERE tenant_id = $1;

-- name: VoidTenantInvoices :many
-- Invoices outlive a deleted company; those still open are cancelled.
UPDATE invoices SET status = 'void', voided_at = NOW()
WHERE tenant_id = $1 AND status = 'open'
RETURNING number;

-- name: DeleteTenantAuditLogs :exec
DELETE FROM audit_logs WHERE tenant_id = $1;

-- name: DeleteTenant :exec
DELETE FROM tenants WHERE id = $1;

-- name: AllowLogRewrite :exec
-- Lets this transaction change the append-only audit log and shipment
-- timeline (see the append_only trigger).
SELECT set_config('kiflet.purge', 'on', true);
