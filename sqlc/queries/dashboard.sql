-- name: ListSavedFilters :many
SELECT * FROM saved_filters
WHERE tenant_id = $1 AND user_id = $2
ORDER BY name;

-- name: CountSavedFilters :one
SELECT COUNT(*) FROM saved_filters WHERE tenant_id = $1 AND user_id = $2;

-- name: SaveFilter :one
-- Saving under an existing name replaces that view.
INSERT INTO saved_filters (tenant_id, user_id, name, query)
VALUES ($1, $2, $3, $4)
ON CONFLICT (user_id, name) DO UPDATE SET query = EXCLUDED.query, created_at = NOW()
RETURNING *;

-- name: DeleteSavedFilter :execrows
DELETE FROM saved_filters WHERE id = $1 AND tenant_id = $2 AND user_id = $3;

-- name: ListDriversOnTheRoad :many
-- Active drivers with a shipment assigned or under way: who a message to
-- "every driver on the road" reaches.
SELECT u.id, u.full_name, u.phone, sm.last_sync_at,
       COUNT(s.id)::bigint AS shipments
FROM users u
JOIN shipments s ON s.driver_id = u.id
     AND s.status IN ('assigned', 'in_transit', 'delayed', 'arrived', 'issue')
LEFT JOIN sync_metadata sm ON sm.driver_id = u.id
WHERE u.tenant_id = $1 AND u.role = 'driver' AND u.is_active
GROUP BY u.id, u.full_name, u.phone, sm.last_sync_at
ORDER BY u.full_name;

-- name: CreateBroadcast :one
INSERT INTO broadcasts (tenant_id, body, sent_by, sender_name)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: AddBroadcastRecipient :exec
INSERT INTO broadcast_recipients (broadcast_id, driver_id, by_sms)
VALUES ($1, $2, $3)
ON CONFLICT DO NOTHING;

-- name: ListBroadcasts :many
SELECT b.id, b.body, b.sender_name, b.created_at,
       COUNT(r.driver_id)::bigint AS recipients,
       COUNT(r.driver_id) FILTER (WHERE r.by_sms)::bigint AS by_sms
FROM broadcasts b
LEFT JOIN broadcast_recipients r ON r.broadcast_id = b.id
WHERE b.tenant_id = $1
GROUP BY b.id
ORDER BY b.created_at DESC
LIMIT $2;

-- name: ListDriverBroadcastsSince :many
-- Messages to every driver on the road that reached this driver. The LIMIT
-- is maxDriverBroadcasts in the sync service.
SELECT b.id, b.body, b.sender_name, b.created_at
FROM broadcasts b
JOIN broadcast_recipients r ON r.broadcast_id = b.id
WHERE b.tenant_id = sqlc.arg(tenant_id)
  AND r.driver_id = sqlc.arg(driver_id)
  AND b.created_at > sqlc.arg(since)
ORDER BY b.created_at ASC
LIMIT 50;

-- name: DeleteOldBroadcasts :exec
DELETE FROM broadcasts WHERE created_at < $1;

-- name: ListAlertEmailRecipients :many
-- Office staff who asked to have alerts of this type emailed to them, at a
-- confirmed address.
SELECT full_name, email
FROM users
WHERE tenant_id = sqlc.arg(tenant_id)
  AND role IN ('dispatcher', 'fleet_manager', 'admin')
  AND is_active
  AND email IS NOT NULL
  AND email_verified_at IS NOT NULL
  AND sqlc.arg(alert_type)::text = ANY(alert_emails);

-- name: ListAlertSMSRecipients :many
-- Office staff who asked to have alerts of this type texted to them.
SELECT full_name, phone
FROM users
WHERE tenant_id = sqlc.arg(tenant_id)
  AND role IN ('dispatcher', 'fleet_manager', 'admin')
  AND is_active
  AND phone IS NOT NULL
  AND sqlc.arg(alert_type)::text = ANY(alert_sms);

-- name: ListLongStops :many
-- Across tenants. Loads on the road whose truck has stood within 300 m of
-- where it is now for the company's stop_minutes (alert_rules) or longer,
-- going by fixes from the last 20 minutes. The stop began at the first fix
-- after the last one more than 300 m away; it is the episode, so each stop
-- is raised once. Whether it is in a town is decided by the caller.
WITH latest AS (
    SELECT DISTINCT ON (s.id)
           s.id, s.tenant_id, s.tracking_number, s.driver_id, s.picked_up_at,
           t.coordinates::geography AS here,
           ST_Y(t.coordinates)::float8 AS latitude, ST_X(t.coordinates)::float8 AS longitude,
           t.recorded_at,
           COALESCE(r.stop_minutes, 45) AS stop_minutes
    FROM shipments s
    JOIN tracking_events t ON t.tenant_id = s.tenant_id AND t.driver_id = s.driver_id
    LEFT JOIN alert_rules r ON r.tenant_id = s.tenant_id
    WHERE s.status IN ('in_transit', 'delayed')
      AND COALESCE(r.stop_on, true)
      AND s.picked_up_at IS NOT NULL
      AND t.recorded_at > NOW() - INTERVAL '20 minutes'
      AND t.recorded_at > s.picked_up_at
    ORDER BY s.id, t.recorded_at DESC
), stops AS (
    SELECT l.*,
           (SELECT min(t.recorded_at) FROM tracking_events t
            WHERE t.tenant_id = l.tenant_id AND t.driver_id = l.driver_id
              AND t.recorded_at > l.picked_up_at
              AND t.recorded_at > COALESCE(
                  (SELECT max(m.recorded_at) FROM tracking_events m
                   WHERE m.tenant_id = l.tenant_id AND m.driver_id = l.driver_id
                     AND m.recorded_at > GREATEST(l.picked_up_at, NOW() - INTERVAL '24 hours')
                     AND NOT ST_DWithin(m.coordinates::geography, l.here, 300)),
                  l.picked_up_at)) AS since
    FROM latest l
)
SELECT id, tenant_id, tracking_number, driver_id, latitude, longitude,
       since::timestamptz AS stopped_since,
       floor(extract(epoch FROM since))::bigint::text AS episode
FROM stops
WHERE since IS NOT NULL
  AND recorded_at - since >= make_interval(mins => stop_minutes)
ORDER BY since, id
LIMIT 500;

-- name: SetAlertEmails :exec
UPDATE users SET alert_emails = sqlc.arg(alert_types)::text[], updated_at = NOW()
WHERE id = sqlc.arg(id) AND tenant_id = sqlc.arg(tenant_id);

-- name: ListRouteDeviations :many
-- Across tenants. Loads on the road whose truck is heading away from the
-- destination: its latest fix, from the last half hour, is more than
-- the company's route_km (alert_rules) further from the destination than a
-- fix from about an hour earlier. There is no road routing offline, so this is the deviation
-- that can be told from GPS alone (FR-033). The episode, the six-hour
-- window of the latest fix, raises at most one alert per load that often.
WITH latest AS (
    SELECT DISTINCT ON (s.id)
           s.id, s.tenant_id, s.tracking_number, s.driver_id, s.destination_city, s.picked_up_at,
           ST_SetSRID(ST_MakePoint(s.destination_lng, s.destination_lat), 4326)::geography AS dest,
           t.coordinates::geography AS here, t.recorded_at,
           COALESCE(r.route_km, 20) * 1000.0 AS min_gain
    FROM shipments s
    JOIN tracking_events t ON t.tenant_id = s.tenant_id AND t.driver_id = s.driver_id
    LEFT JOIN alert_rules r ON r.tenant_id = s.tenant_id
    WHERE s.status IN ('in_transit', 'delayed')
      AND COALESCE(r.route_on, true)
      AND s.destination_lat IS NOT NULL AND s.destination_lng IS NOT NULL
      AND s.picked_up_at IS NOT NULL
      AND t.recorded_at > NOW() - INTERVAL '30 minutes'
      AND t.recorded_at > s.picked_up_at
    ORDER BY s.id, t.recorded_at DESC
)
SELECT l.id, l.tenant_id, l.tracking_number, l.driver_id, l.destination_city,
       ST_Distance(l.here, l.dest)::float8 AS distance_now,
       ST_Distance(e.there, l.dest)::float8 AS distance_before,
       (floor(extract(epoch FROM l.recorded_at) / 21600))::bigint::text AS episode
FROM latest l
JOIN LATERAL (
    SELECT t.coordinates::geography AS there
    FROM tracking_events t
    WHERE t.tenant_id = l.tenant_id AND t.driver_id = l.driver_id
      AND t.recorded_at > l.picked_up_at
      AND t.recorded_at BETWEEN l.recorded_at - INTERVAL '90 minutes' AND l.recorded_at - INTERVAL '45 minutes'
    ORDER BY abs(extract(epoch FROM (t.recorded_at - (l.recorded_at - INTERVAL '60 minutes'))))
    LIMIT 1
) e ON true
WHERE ST_Distance(l.here, l.dest) - ST_Distance(e.there, l.dest) > l.min_gain
ORDER BY l.recorded_at DESC, l.id
LIMIT 500;
