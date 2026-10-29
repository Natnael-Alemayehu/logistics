-- name: GetTenantByID :one
SELECT * FROM tenants WHERE id = $1;

-- name: GetTenantBySlug :one
SELECT * FROM tenants WHERE slug = $1;

-- name: CreateTenant :one
INSERT INTO tenants (name, slug, plan, max_drivers, contact_phone, contact_email, region_city, trial_ends_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: UpdateTenantSettings :one
UPDATE tenants
SET name = $2,
    contact_phone = $3,
    contact_email = $4,
    region_city = $5,
    max_active_per_driver = $6,
    sms_on_created = $7,
    sms_on_dispatched = $8,
    sms_on_arrived = $9,
    sms_on_delivered = $10,
    sms_on_cancelled = $11,
    sms_on_delayed = $12,
    sms_driver_on_assign = $13
WHERE id = $1
RETURNING *;

-- name: UpdateTenantPlan :exec
UPDATE tenants SET plan = $2, max_drivers = $3 WHERE id = $1;

-- name: SetTenantActive :exec
UPDATE tenants SET is_active = $2 WHERE id = $1;

-- name: ListTenantsWithStats :many
SELECT t.*,
       (SELECT COUNT(*) FROM users u WHERE u.tenant_id = t.id AND u.role = 'driver' AND u.is_active)::bigint AS drivers,
       (SELECT COUNT(*) FROM users u WHERE u.tenant_id = t.id AND u.role <> 'driver' AND u.is_active)::bigint AS staff,
       (SELECT COUNT(*) FROM shipments s WHERE s.tenant_id = t.id AND s.created_at > NOW() - INTERVAL '30 days')::bigint AS shipments_30d,
       (SELECT MAX(s.created_at) FROM shipments s WHERE s.tenant_id = t.id)::timestamptz AS last_shipment_at
FROM tenants t
ORDER BY t.created_at DESC;
