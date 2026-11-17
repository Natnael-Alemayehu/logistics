-- The public API's keys (FR-046).

-- name: CreateAPIKey :one
INSERT INTO api_keys (tenant_id, name, prefix, key_hash, created_by)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListAPIKeys :many
SELECT * FROM api_keys WHERE tenant_id = $1 ORDER BY revoked_at IS NOT NULL, created_at DESC;

-- name: CountActiveAPIKeys :one
SELECT COUNT(*) FROM api_keys WHERE tenant_id = $1 AND revoked_at IS NULL;

-- name: GetAPIKeyByHash :one
-- The one query not scoped by company: the key is what says which company.
SELECT k.id, k.tenant_id, k.name, t.is_active AS tenant_active
FROM api_keys k
JOIN tenants t ON t.id = k.tenant_id
WHERE k.key_hash = $1 AND k.revoked_at IS NULL;

-- name: TouchAPIKey :exec
-- At most one write a minute, however busy the key.
UPDATE api_keys SET last_used_at = NOW()
WHERE id = $1 AND (last_used_at IS NULL OR last_used_at < NOW() - INTERVAL '1 minute');

-- name: RevokeAPIKey :one
UPDATE api_keys SET revoked_at = NOW()
WHERE id = $1 AND tenant_id = $2 AND revoked_at IS NULL
RETURNING *;
