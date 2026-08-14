-- name: GetDriverDeliveriesToday :one
SELECT COUNT(*) FROM shipments 
WHERE driver_id = $1 
  AND tenant_id = $2 
  AND status = 'delivered' 
  AND actual_delivery >= DATE_TRUNC('day', NOW());

-- name: GetDriverDeliveriesThisWeek :one
SELECT COUNT(*) FROM shipments 
WHERE driver_id = $1 
  AND tenant_id = $2 
  AND status = 'delivered' 
  AND actual_delivery >= DATE_TRUNC('week', NOW());

-- name: GetDriverDeliveriesThisMonth :one
SELECT COUNT(*) FROM shipments 
WHERE driver_id = $1 
  AND tenant_id = $2 
  AND status = 'delivered' 
  AND actual_delivery >= DATE_TRUNC('month', NOW());

-- name: GetDriverTotalDeliveries :one
SELECT COUNT(*) FROM shipments 
WHERE driver_id = $1 
  AND tenant_id = $2 
  AND status = 'delivered';

-- name: GetDriverActiveShipmentCount :one
SELECT COUNT(*) FROM shipments 
WHERE driver_id = $1 
  AND tenant_id = $2 
  AND status IN ('assigned', 'in_transit', 'delayed', 'arrived');

-- name: GetDriverVehicleAssignment :one
SELECT v.id, v.tenant_id, v.plate_number, v.vehicle_type, v.is_active, v.created_at, v.updated_at
FROM vehicles v
INNER JOIN shipments s ON s.vehicle_id = v.id
WHERE s.driver_id = $1 
  AND s.tenant_id = $2 
  AND s.status IN ('assigned', 'in_transit', 'delayed', 'arrived')
LIMIT 1;
