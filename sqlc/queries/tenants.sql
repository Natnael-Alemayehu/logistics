-- name: GetTenantByID :one
SELECT * FROM tenants WHERE id = $1;

-- name: GetTenantBySlug :one
SELECT * FROM tenants WHERE slug = $1;

-- name: CreateTenant :one
INSERT INTO tenants (name, slug, plan, max_drivers)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: UpdateTenant :one
UPDATE tenants
SET
    name = COALESCE(sqlc.narg(name), name),
    plan = COALESCE(sqlc.narg(plan), plan),
    max_drivers = COALESCE(sqlc.narg(max_drivers), max_drivers),
    is_active = COALESCE(sqlc.narg(is_active), is_active)
WHERE id = $1
RETURNING *;