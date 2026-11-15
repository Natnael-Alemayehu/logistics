package model

import (
	"errors"
	"fmt"
	"slices"
)

// Shipment statuses (FR-008).
const (
	StatusPending   = "pending"
	StatusAssigned  = "assigned"
	StatusInTransit = "in_transit"
	StatusDelayed   = "delayed"
	StatusArrived   = "arrived"
	StatusDelivered = "delivered"
	StatusIssue     = "issue"
	StatusCancelled = "cancelled"
)

// AllStatuses is every status in lifecycle order.
var AllStatuses = []string{
	StatusPending, StatusAssigned, StatusInTransit, StatusDelayed,
	StatusArrived, StatusIssue, StatusDelivered, StatusCancelled,
}

// ActiveStatuses are the statuses in which a shipment is someone's current
// work: it counts toward a driver's load and appears on their device.
var ActiveStatuses = []string{StatusAssigned, StatusInTransit, StatusDelayed, StatusArrived, StatusIssue}

// OnRoadStatuses are the statuses in which the cargo has left the origin.
var OnRoadStatuses = []string{StatusInTransit, StatusDelayed, StatusArrived, StatusIssue}

func IsValidStatus(s string) bool  { return slices.Contains(AllStatuses, s) }
func IsActiveStatus(s string) bool { return slices.Contains(ActiveStatuses, s) }
func IsOnRoad(s string) bool       { return slices.Contains(OnRoadStatuses, s) }
func IsFinalStatus(s string) bool  { return s == StatusDelivered || s == StatusCancelled }

var statusLabels = map[string]string{
	StatusPending:   "Pending",
	StatusAssigned:  "Assigned",
	StatusInTransit: "In transit",
	StatusDelayed:   "Delayed",
	StatusArrived:   "Arrived",
	StatusDelivered: "Delivered",
	StatusIssue:     "Issue",
	StatusCancelled: "Cancelled",
}

// StatusLabel is the human-readable name of a status.
func StatusLabel(s string) string {
	if l, ok := statusLabels[s]; ok {
		return l
	}
	return s
}

// Actor identifies who is asking for a transition. The same move can be legal
// for one and not another: a driver cannot declare a delivery without proof,
// and only a dispatcher can override the normal flow.
type Actor int

const (
	// ActorDriver is a driver using the app.
	ActorDriver Actor = iota
	// ActorDriverSMS is a driver reporting by SMS. Feature phones cannot capture
	// proof of delivery, so a delivery reported this way is accepted and flagged.
	ActorDriverSMS
	// ActorDispatcher is a dispatcher, fleet manager or admin.
	ActorDispatcher
	// ActorPOD is the system applying a recorded proof of delivery.
	ActorPOD
	// ActorReview is the office accepting a proof of delivery that was held
	// because the shipment changed while the driver was offline. The proof is
	// the evidence, so it completes the shipment from any state, cancelled
	// included.
	ActorReview
)

var (
	// ErrSameStatus means the shipment is already in the requested status.
	// Callers treat it as success: replays and double-clicks are harmless.
	ErrSameStatus = errors.New("shipment already has this status")
	// ErrPODRequired means a driver tried to mark a shipment delivered without
	// recording proof of delivery.
	ErrPODRequired = errors.New("record proof of delivery to complete this shipment")
)

// TransitionError explains why a status change is not allowed.
type TransitionError struct {
	From, To string
}

func (e *TransitionError) Error() string {
	return fmt.Sprintf("cannot change a shipment from %s to %s",
		StatusLabel(e.From), StatusLabel(e.To))
}

// driverMoves is the road lifecycle a driver walks through. "Arrived" can go
// back on the road because drivers do reach the wrong gate.
var driverMoves = map[string][]string{
	StatusAssigned:  {StatusInTransit},
	StatusInTransit: {StatusDelayed, StatusArrived, StatusIssue},
	StatusDelayed:   {StatusInTransit, StatusArrived, StatusIssue},
	StatusIssue:     {StatusInTransit, StatusDelayed, StatusArrived},
	StatusArrived:   {StatusInTransit, StatusIssue},
}

// dispatcherMoves adds the office's overrides to the driver lifecycle:
// departing on a driver's behalf, confirming a delivery reported by phone, and
// reopening a shipment cancelled by mistake. Assignment and cancellation have
// their own operations and are not plain status changes.
var dispatcherMoves = map[string][]string{
	StatusAssigned:  {StatusInTransit},
	StatusInTransit: {StatusDelayed, StatusArrived, StatusIssue, StatusDelivered},
	StatusDelayed:   {StatusInTransit, StatusArrived, StatusIssue, StatusDelivered},
	StatusIssue:     {StatusInTransit, StatusDelayed, StatusArrived, StatusDelivered},
	StatusArrived:   {StatusInTransit, StatusIssue, StatusDelivered},
}

// CheckTransition reports whether actor may move a shipment from one status to
// another. It returns ErrSameStatus when there is nothing to do.
func CheckTransition(from, to string, actor Actor) error {
	if !IsValidStatus(to) {
		return fmt.Errorf("unknown status %q", to)
	}
	if from == to {
		return ErrSameStatus
	}

	switch actor {
	case ActorPOD:
		if IsOnRoad(from) || from == StatusAssigned {
			return nil
		}
	case ActorReview:
		if to == StatusDelivered {
			return nil
		}
	case ActorDriver:
		if to == StatusDelivered && (IsOnRoad(from) || from == StatusAssigned) {
			return ErrPODRequired
		}
		if slices.Contains(driverMoves[from], to) {
			return nil
		}
	case ActorDriverSMS:
		if to == StatusDelivered && IsOnRoad(from) {
			return nil
		}
		if slices.Contains(driverMoves[from], to) {
			return nil
		}
	case ActorDispatcher:
		if slices.Contains(dispatcherMoves[from], to) {
			return nil
		}
	}

	return &TransitionError{From: from, To: to}
}

// AllowedTransitions lists the statuses actor may move a shipment to next, for
// building context-aware action buttons (FR-017 AC-017.5).
func AllowedTransitions(from string, actor Actor) []string {
	var out []string
	for _, to := range AllStatuses {
		if CheckTransition(from, to, actor) == nil {
			out = append(out, to)
		}
	}
	return out
}

// CanCancel reports whether a shipment in this status can be cancelled. Only a
// delivered shipment is past the point of cancellation; one already cancelled
// has nothing left to do.
func CanCancel(status string) bool {
	return !IsFinalStatus(status)
}

// CanReopen reports whether a cancelled shipment can be put back to pending.
func CanReopen(status string) bool {
	return status == StatusCancelled
}

// CanAssign reports whether a shipment in this status can be given to a driver.
// Reassignment mid-route is allowed: trucks break down and loads are handed
// over.
func CanAssign(status string) bool {
	return !IsFinalStatus(status)
}

// CanUnassign reports whether the driver can be taken off without a handover:
// only before the cargo has left.
func CanUnassign(status string) bool {
	return status == StatusAssigned
}

// Reason codes. Delay reasons are the predefined list drivers pick from
// (FR-018 AC-018.3); issue and cancellation reasons follow the same pattern.
var (
	DelayReasons = []string{
		"road_conditions", "weather", "security_checkpoint",
		"mechanical_issue", "traffic", "other",
	}
	IssueReasons = []string{
		"accident", "breakdown", "recipient_unavailable", "wrong_address",
		"cargo_damaged", "recipient_refused", "other",
	}
	CancelReasons = []string{
		"customer_request", "duplicate", "address_problem",
		"cargo_unavailable", "payment_problem",
		"recipient_refused", "returned_to_origin", "cargo_lost_damaged", "other",
	}
	// Cancelling before the cargo leaves withdraws a booking; cancelling on
	// the road records how a trip ended without a delivery.
	cancelBeforeDeparture = []string{
		"customer_request", "duplicate", "address_problem",
		"cargo_unavailable", "payment_problem", "other",
	}
	cancelOnRoad = []string{
		"recipient_refused", "returned_to_origin", "cargo_lost_damaged",
		"customer_request", "address_problem", "payment_problem", "other",
	}
	// Why a delivery was recorded away from its destination, chosen by the
	// driver when the app warns them (FR-022 AC-022.2).
	LocationReasons = []string{"met_elsewhere", "address_wrong", "gps_inaccurate", "other"}
)

// CancelReasonsFor returns the cancellation reasons that make sense for a
// shipment in this status.
func CancelReasonsFor(status string) []string {
	if IsOnRoad(status) {
		return cancelOnRoad
	}
	return cancelBeforeDeparture
}

var reasonLabels = map[string]string{
	"road_conditions":       "Road conditions",
	"weather":               "Weather",
	"security_checkpoint":   "Security checkpoint",
	"mechanical_issue":      "Mechanical issue",
	"traffic":               "Traffic",
	"accident":              "Accident",
	"breakdown":             "Vehicle breakdown",
	"recipient_unavailable": "Recipient unavailable",
	"wrong_address":         "Wrong address",
	"cargo_damaged":         "Cargo damaged",
	"recipient_refused":     "Recipient refused delivery",
	"customer_request":      "Customer request",
	"duplicate":             "Duplicate shipment",
	"address_problem":       "Address problem",
	"cargo_unavailable":     "Cargo not ready",
	"payment_problem":       "Payment problem",
	"returned_to_origin":    "Returned to origin",
	"cargo_lost_damaged":    "Cargo lost or damaged",
	"met_elsewhere":         "Recipient met the truck elsewhere",
	"address_wrong":         "Destination on the shipment is wrong",
	"gps_inaccurate":        "Phone location inaccurate here",
	"other":                 "Other",
}

// ReasonLabel is the human-readable form of a reason code. Free text passes
// through unchanged, since older clients and SMS reports send prose.
func ReasonLabel(code string) string {
	if l, ok := reasonLabels[code]; ok {
		return l
	}
	return code
}

// ReasonsFor returns the reason list that applies to a status, if any.
func ReasonsFor(status string) []string {
	switch status {
	case StatusDelayed:
		return DelayReasons
	case StatusIssue:
		return IssueReasons
	case StatusCancelled:
		return CancelReasons
	}
	return nil
}

// Priorities.
const (
	PriorityNormal = "normal"
	PriorityHigh   = "high"
	PriorityUrgent = "urgent"
)

var Priorities = []string{PriorityNormal, PriorityHigh, PriorityUrgent}

// Roles.
const (
	RoleDriver        = "driver"
	RoleDispatcher    = "dispatcher"
	RoleFleetManager  = "fleet_manager"
	RoleAdmin         = "admin"
	RolePlatformAdmin = "platform_admin"
)

// StaffRoles are the roles an admin can give a dashboard user.
var StaffRoles = []string{RoleDispatcher, RoleFleetManager, RoleAdmin}

var roleLabels = map[string]string{
	RoleDriver:        "Driver",
	RoleDispatcher:    "Dispatcher",
	RoleFleetManager:  "Fleet manager",
	RoleAdmin:         "Admin",
	RolePlatformAdmin: "Platform admin",
}

func RoleLabel(r string) string {
	if l, ok := roleLabels[r]; ok {
		return l
	}
	return r
}

// CanManageFleet reports whether a role may manage drivers and vehicles
// (FR-004 AC-004.3).
func CanManageFleet(role string) bool {
	return role == RoleFleetManager || role == RoleAdmin || role == RolePlatformAdmin
}

// CanManageCompany reports whether a role may manage staff accounts and
// company settings (FR-004 AC-004.4).
func CanManageCompany(role string) bool {
	return role == RoleAdmin || role == RolePlatformAdmin
}
