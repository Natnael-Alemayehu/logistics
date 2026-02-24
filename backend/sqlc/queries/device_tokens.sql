-- name: UpsertDeviceToken :one
INSERT INTO device_tokens (user_id, device_id, push_token, platform, app_version)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (user_id, device_id) 
DO UPDATE SET 
    push_token = EXCLUDED.push_token,
    platform = EXCLUDED.platform,
    app_version = EXCLUDED.app_version,
    updated_at = NOW()
RETURNING *;

-- name: DeleteDeviceToken :exec
DELETE FROM device_tokens 
WHERE user_id = $1 AND device_id = $2;

-- name: GetDeviceTokensByUser :many
SELECT * FROM device_tokens
WHERE user_id = $1
ORDER BY created_at DESC;

-- name: GetDeviceTokenByDeviceID :one
SELECT * FROM device_tokens
WHERE user_id = $1 AND device_id = $2;
