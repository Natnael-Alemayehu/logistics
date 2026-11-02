-- Reports cover the half-open interval [from, to). Days are bucketed in the
-- caller's time zone so "deliveries on Tuesday" means Tuesday in Addis Ababa,
-- not in UTC.
--
-- Every report takes the same optional filters (FR-034 AC-034.5): one
-- driver, and a route given by its origin town, its destination town or
-- both.

-- name: ReportDeliveriesPerDay :many
SELECT (date_trunc('day', actual_delivery AT TIME ZONE sqlc.arg(tz)::text))::date AS day,
       COUNT(*)::bigint AS delivered,
       COUNT(*) FILTER (WHERE estimated_delivery IS NOT NULL AND actual_delivery <= estimated_delivery)::bigint AS on_time
FROM shipments
WHERE tenant_id = sqlc.arg(tenant_id)
  AND status = 'delivered'
  AND actual_delivery >= sqlc.arg(date_from)
  AND actual_delivery < sqlc.arg(date_to)
  AND (sqlc.narg(driver_id)::uuid IS NULL OR driver_id = sqlc.narg(driver_id)::uuid)
  AND (sqlc.narg(origin)::text IS NULL OR lower(origin_city) = lower(sqlc.narg(origin)::text))
  AND (sqlc.narg(destination)::text IS NULL OR lower(destination_city) = lower(sqlc.narg(destination)::text))
GROUP BY 1
ORDER BY 1;

-- name: ReportSummary :one
SELECT
    (SELECT COUNT(*) FROM shipments c
     WHERE c.tenant_id = sqlc.arg(tenant_id)
       AND c.created_at >= sqlc.arg(date_from) AND c.created_at < sqlc.arg(date_to)
       AND (sqlc.narg(driver_id)::uuid IS NULL OR c.driver_id = sqlc.narg(driver_id)::uuid)
       AND (sqlc.narg(origin)::text IS NULL OR lower(c.origin_city) = lower(sqlc.narg(origin)::text))
       AND (sqlc.narg(destination)::text IS NULL OR lower(c.destination_city) = lower(sqlc.narg(destination)::text)))::bigint AS created,
    COUNT(*) FILTER (WHERE status = 'delivered')::bigint AS delivered,
    COUNT(*) FILTER (WHERE status = 'delivered' AND estimated_delivery IS NOT NULL
                       AND actual_delivery <= estimated_delivery)::bigint AS on_time,
    COUNT(*) FILTER (WHERE status = 'delivered' AND estimated_delivery IS NOT NULL)::bigint AS with_eta,
    COALESCE(AVG(EXTRACT(EPOCH FROM (actual_delivery - COALESCE(picked_up_at, created_at))) / 3600.0)
        FILTER (WHERE status = 'delivered'), 0)::float8 AS avg_transit_hours,
    (SELECT COUNT(DISTINCT e.shipment_id) FROM shipment_events e JOIN shipments c ON c.id = e.shipment_id
     WHERE e.tenant_id = sqlc.arg(tenant_id) AND e.event_type = 'status_changed' AND e.to_status = 'issue'
       AND e.occurred_at >= sqlc.arg(date_from) AND e.occurred_at < sqlc.arg(date_to)
       AND (sqlc.narg(driver_id)::uuid IS NULL OR c.driver_id = sqlc.narg(driver_id)::uuid)
       AND (sqlc.narg(origin)::text IS NULL OR lower(c.origin_city) = lower(sqlc.narg(origin)::text))
       AND (sqlc.narg(destination)::text IS NULL OR lower(c.destination_city) = lower(sqlc.narg(destination)::text)))::bigint AS issues,
    (SELECT COUNT(*) FROM shipments c
     WHERE c.tenant_id = sqlc.arg(tenant_id) AND c.status = 'cancelled'
       AND c.status_changed_at >= sqlc.arg(date_from) AND c.status_changed_at < sqlc.arg(date_to)
       AND (sqlc.narg(driver_id)::uuid IS NULL OR c.driver_id = sqlc.narg(driver_id)::uuid)
       AND (sqlc.narg(origin)::text IS NULL OR lower(c.origin_city) = lower(sqlc.narg(origin)::text))
       AND (sqlc.narg(destination)::text IS NULL OR lower(c.destination_city) = lower(sqlc.narg(destination)::text)))::bigint AS cancelled,
    (SELECT COUNT(*) FROM proof_of_deliveries p JOIN shipments c ON c.id = p.shipment_id
     WHERE p.tenant_id = sqlc.arg(tenant_id) AND p.status = 'accepted'
       AND p.recorded_at >= sqlc.arg(date_from) AND p.recorded_at < sqlc.arg(date_to)
       AND (sqlc.narg(driver_id)::uuid IS NULL OR c.driver_id = sqlc.narg(driver_id)::uuid)
       AND (sqlc.narg(origin)::text IS NULL OR lower(c.origin_city) = lower(sqlc.narg(origin)::text))
       AND (sqlc.narg(destination)::text IS NULL OR lower(c.destination_city) = lower(sqlc.narg(destination)::text)))::bigint AS with_pod
FROM shipments
WHERE tenant_id = sqlc.arg(tenant_id)
  AND actual_delivery >= sqlc.arg(date_from)
  AND actual_delivery < sqlc.arg(date_to)
  AND (sqlc.narg(driver_id)::uuid IS NULL OR driver_id = sqlc.narg(driver_id)::uuid)
  AND (sqlc.narg(origin)::text IS NULL OR lower(origin_city) = lower(sqlc.narg(origin)::text))
  AND (sqlc.narg(destination)::text IS NULL OR lower(destination_city) = lower(sqlc.narg(destination)::text));

-- name: ReportDriverPerformance :many
-- Every active driver, and any other who delivered in the period; with a
-- route filter, only those who drove it.
SELECT u.id AS driver_id, u.full_name AS driver_name,
       COUNT(s.id) FILTER (WHERE s.status = 'delivered')::bigint AS delivered,
       COUNT(s.id) FILTER (WHERE s.status = 'delivered' AND s.estimated_delivery IS NOT NULL
                             AND s.actual_delivery <= s.estimated_delivery)::bigint AS on_time,
       COUNT(s.id) FILTER (WHERE s.status = 'delivered' AND s.estimated_delivery IS NOT NULL)::bigint AS with_eta,
       COALESCE(AVG(EXTRACT(EPOCH FROM (s.actual_delivery - COALESCE(s.picked_up_at, s.created_at))) / 3600.0)
           FILTER (WHERE s.status = 'delivered'), 0)::float8 AS avg_transit_hours,
       (SELECT COUNT(DISTINCT e.shipment_id) FROM shipment_events e JOIN shipments c ON c.id = e.shipment_id
        WHERE e.tenant_id = sqlc.arg(tenant_id) AND e.event_type = 'status_changed' AND e.to_status = 'issue'
          AND e.actor_id = u.id
          AND e.occurred_at >= sqlc.arg(date_from) AND e.occurred_at < sqlc.arg(date_to)
          AND (sqlc.narg(origin)::text IS NULL OR lower(c.origin_city) = lower(sqlc.narg(origin)::text))
          AND (sqlc.narg(destination)::text IS NULL OR lower(c.destination_city) = lower(sqlc.narg(destination)::text))
       )::bigint AS issues
FROM users u
LEFT JOIN shipments s ON s.driver_id = u.id
     AND s.actual_delivery >= sqlc.arg(date_from) AND s.actual_delivery < sqlc.arg(date_to)
     AND (sqlc.narg(origin)::text IS NULL OR lower(s.origin_city) = lower(sqlc.narg(origin)::text))
     AND (sqlc.narg(destination)::text IS NULL OR lower(s.destination_city) = lower(sqlc.narg(destination)::text))
WHERE u.tenant_id = sqlc.arg(tenant_id) AND u.role = 'driver'
  AND (sqlc.narg(driver_id)::uuid IS NULL OR u.id = sqlc.narg(driver_id)::uuid)
GROUP BY u.id, u.full_name, u.is_active
HAVING COUNT(s.id) > 0
    OR (u.is_active AND sqlc.narg(origin)::text IS NULL AND sqlc.narg(destination)::text IS NULL)
ORDER BY delivered DESC, u.full_name;

-- name: ReportRoutes :many
SELECT COALESCE(origin_city, '—')::text AS origin_city,
       COALESCE(destination_city, '—')::text AS destination_city,
       COUNT(*)::bigint AS delivered,
       COUNT(*) FILTER (WHERE estimated_delivery IS NOT NULL AND actual_delivery <= estimated_delivery)::bigint AS on_time,
       COUNT(*) FILTER (WHERE estimated_delivery IS NOT NULL)::bigint AS with_eta,
       COALESCE(AVG(EXTRACT(EPOCH FROM (actual_delivery - COALESCE(picked_up_at, created_at))) / 3600.0), 0)::float8 AS avg_transit_hours
FROM shipments
WHERE tenant_id = sqlc.arg(tenant_id)
  AND status = 'delivered'
  AND actual_delivery >= sqlc.arg(date_from)
  AND actual_delivery < sqlc.arg(date_to)
  AND (sqlc.narg(driver_id)::uuid IS NULL OR driver_id = sqlc.narg(driver_id)::uuid)
  AND (sqlc.narg(origin)::text IS NULL OR lower(origin_city) = lower(sqlc.narg(origin)::text))
  AND (sqlc.narg(destination)::text IS NULL OR lower(destination_city) = lower(sqlc.narg(destination)::text))
GROUP BY 1, 2
ORDER BY delivered DESC
LIMIT 20;

-- name: ReportCustomers :many
SELECT customer_name, customer_phone,
       COUNT(*)::bigint AS shipments,
       COUNT(*) FILTER (WHERE status = 'delivered')::bigint AS delivered,
       MAX(created_at)::timestamptz AS last_shipment_at
FROM shipments
WHERE tenant_id = sqlc.arg(tenant_id)
  AND created_at >= sqlc.arg(date_from)
  AND created_at < sqlc.arg(date_to)
  AND (sqlc.narg(driver_id)::uuid IS NULL OR driver_id = sqlc.narg(driver_id)::uuid)
  AND (sqlc.narg(origin)::text IS NULL OR lower(origin_city) = lower(sqlc.narg(origin)::text))
  AND (sqlc.narg(destination)::text IS NULL OR lower(destination_city) = lower(sqlc.narg(destination)::text))
GROUP BY customer_name, customer_phone
ORDER BY shipments DESC
LIMIT 20;

-- name: ReportCancellations :many
-- Why shipments were cancelled, and how many of them after the cargo had
-- left: a refused or returned load cost a trip.
SELECT COALESCE(status_reason, 'other')::text AS reason,
       COUNT(*)::bigint AS total,
       COUNT(*) FILTER (WHERE picked_up_at IS NOT NULL)::bigint AS on_road
FROM shipments
WHERE tenant_id = sqlc.arg(tenant_id)
  AND status = 'cancelled'
  AND status_changed_at >= sqlc.arg(date_from)
  AND status_changed_at < sqlc.arg(date_to)
  AND (sqlc.narg(driver_id)::uuid IS NULL OR driver_id = sqlc.narg(driver_id)::uuid)
  AND (sqlc.narg(origin)::text IS NULL OR lower(origin_city) = lower(sqlc.narg(origin)::text))
  AND (sqlc.narg(destination)::text IS NULL OR lower(destination_city) = lower(sqlc.narg(destination)::text))
GROUP BY 1
ORDER BY total DESC, reason;
