package geo

import (
	"math"
	"sort"
	"time"
)

const earthRadiusMeters = 6371000.0

// DistanceMeters is the great-circle distance between two coordinates.
func DistanceMeters(lat1, lng1, lat2, lng2 float64) float64 {
	toRad := func(d float64) float64 { return d * math.Pi / 180 }
	dLat := toRad(lat2 - lat1)
	dLng := toRad(lng2 - lng1)
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(toRad(lat1))*math.Cos(toRad(lat2))*math.Sin(dLng/2)*math.Sin(dLng/2)
	return 2 * earthRadiusMeters * math.Asin(math.Min(1, math.Sqrt(a)))
}

// Delivery-location tolerance (FR-022). A pin the dispatcher placed is held to
// the 500 m the requirement asks for. A city-centre coordinate says nothing
// about the street, so the check only confirms the driver is in the right town.
const (
	PreciseDeliveryToleranceMeters = 500
	CityDeliveryToleranceMeters    = 15000
)

// DeliveryTolerance returns how far from the destination coordinate a delivery
// may be recorded before it is flagged.
func DeliveryTolerance(precise bool) float64 {
	if precise {
		return PreciseDeliveryToleranceMeters
	}
	return CityDeliveryToleranceMeters
}

// Transit-time model for heavy trucks on Ethiopian roads.
//
// Straight-line distance understates road distance by roughly a third on these
// routes (Addis–Djibouti is ~760 km as the crow flies, ~910 km by road).
// Loaded trucks average around 45 km/h including towns and checkpoints, most
// operators do not drive at night, and every route loses time to loading,
// weighbridges and customs, so the estimate adds a rest period per driving day
// and a fixed handling allowance.
const (
	roadFactor        = 1.3
	averageSpeedKPH   = 45.0
	drivingHoursDay   = 10.0
	restHoursPerDay   = 14.0
	handlingHours     = 3.0
	MinHistorySamples = 5
)

// EstimateTransit returns the expected time from departure to delivery between
// two coordinates.
//
// When the tenant has delivered at least five shipments on the same route the
// median of those real transit times is used instead, since it already
// accounts for the particular road, border and customer.
func EstimateTransit(fromLat, fromLng, toLat, toLng float64, historyHours []float64) time.Duration {
	if len(historyHours) >= MinHistorySamples {
		sorted := append([]float64(nil), historyHours...)
		sort.Float64s(sorted)
		mid := len(sorted) / 2
		median := sorted[mid]
		if len(sorted)%2 == 0 {
			median = (sorted[mid-1] + sorted[mid]) / 2
		}
		return time.Duration(median * float64(time.Hour))
	}

	roadKM := RoadKM(fromLat, fromLng, toLat, toLng)
	driving := roadKM / averageSpeedKPH
	restDays := math.Floor(driving / drivingHoursDay)
	total := driving + restDays*restHoursPerDay + handlingHours
	return time.Duration(total * float64(time.Hour))
}

// RoadKM is the expected distance by road between two points: the straight
// line, lengthened by roadFactor.
func RoadKM(fromLat, fromLng, toLat, toLng float64) float64 {
	return DistanceMeters(fromLat, fromLng, toLat, toLng) / 1000 * roadFactor
}
