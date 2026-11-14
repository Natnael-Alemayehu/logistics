package service

import (
	"context"
	"crypto/hmac"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/natnael-alemayehu/logistics/internal/db"
	"github.com/natnael-alemayehu/logistics/internal/geo"
	"github.com/natnael-alemayehu/logistics/internal/model"
	"github.com/natnael-alemayehu/logistics/internal/phone"
)

// PublicTracking is what anyone holding a tracking number may see (FR-029).
// It names no people and gives the truck's position only to the nearest town.
type PublicTracking struct {
	TrackingNumber string
	Status         string
	StatusLabel    string
	CompanyName    string
	CompanyPhone   string
	Origin         string
	Destination    string
	OriginLat      float64
	OriginLng      float64
	DestLat        float64
	DestLng        float64
	ETA            *time.Time
	DeliveredAt    *time.Time
	Late           bool
	LastSeen       *PublicPosition
	Steps          []TrackingStep
	Timeline       []PublicEvent
	Confirmed      bool
}

// PublicPosition is a coarse last-known location.
type PublicPosition struct {
	Near string // a phrase: "near Adama", "45 km from Adama", "on the road"
	Lat  float64
	Lng  float64
	At   time.Time
}

// TrackingStep is one stage of the progress bar.
type TrackingStep struct {
	Label   string
	Done    bool
	Current bool
}

// PublicEvent is one line of the public timeline.
type PublicEvent struct {
	At     time.Time
	Label  string
	Detail string
}

// customerStatusLabel softens internal states for customers: they see a
// problem on the road as a delay while the company deals with it.
func customerStatusLabel(status string) string {
	switch status {
	case model.StatusIssue:
		return "Delayed"
	case model.StatusAssigned:
		return "Preparing"
	case model.StatusPending:
		return "Booked"
	}
	return model.StatusLabel(status)
}

// Track looks a shipment up by its tracking number.
func (s *ShipmentService) Track(ctx context.Context, number string) (*PublicTracking, error) {
	number = strings.ToUpper(strings.TrimSpace(number))
	sh, err := s.q.GetShipmentByTrackingNumber(ctx, number)
	if isNoRows(err) {
		return nil, notFound("We couldn't find that tracking number. Check it and try again.")
	}
	if err != nil {
		return nil, err
	}
	tenant, err := s.q.GetTenantByID(ctx, sh.TenantID)
	if err != nil {
		return nil, err
	}

	t := &PublicTracking{
		TrackingNumber: sh.TrackingNumber,
		Status:         sh.Status,
		StatusLabel:    customerStatusLabel(sh.Status),
		CompanyName:    tenant.Name,
		CompanyPhone:   deref(tenant.ContactPhone),
		Origin:         place(sh.OriginCity, sh.OriginAddress),
		Destination:    place(sh.DestinationCity, sh.DestinationAddress),
		ETA:            timePtr(sh.EstimatedDelivery),
		DeliveredAt:    timePtr(sh.ActualDelivery),
		Confirmed:      sh.CustomerConfirmedAt.Valid,
	}
	// The map shows towns, not the customer's gate.
	if o, ok := geo.Lookup(deref(sh.OriginCity)); ok {
		t.OriginLat, t.OriginLng = o.Lat, o.Lng
	}
	if d, ok := geo.Lookup(deref(sh.DestinationCity)); ok {
		t.DestLat, t.DestLng = d.Lat, d.Lng
	}
	if t.ETA != nil && !model.IsFinalStatus(sh.Status) && s.now().After(*t.ETA) {
		t.Late = true
	}

	if model.IsOnRoad(sh.Status) {
		start, _ := shipmentWindow(sh, s.now())
		pos, err := s.q.GetLatestShipmentPosition(ctx, db.GetLatestShipmentPositionParams{
			TenantID: sh.TenantID, ShipmentID: sh.ID, DriverID: sh.DriverID, WindowStart: toTimestamp(start),
		})
		if err == nil {
			t.LastSeen = coarsePosition(pos.Latitude, pos.Longitude, pos.RecordedAt.Time)
		}
	}

	t.Steps = trackingSteps(sh.Status)

	events, err := s.q.ListPublicShipmentEvents(ctx, sh.ID)
	if err != nil {
		return nil, err
	}
	for _, e := range events {
		pe := PublicEvent{At: e.OccurredAt.Time}
		switch e.EventType {
		case "created":
			pe.Label = "Shipment booked"
		case "assigned":
			pe.Label = "Driver assigned"
		case "customer_confirmed":
			pe.Label = "Receipt confirmed by customer"
		case "status_changed", "status_updated":
			to := deref(e.ToStatus)
			pe.Label = customerStatusLabel(to)
			if to == model.StatusDelayed && e.Reason != nil {
				pe.Detail = model.ReasonLabel(*e.Reason)
			}
			if to == model.StatusCancelled {
				pe.Detail = "Contact " + tenant.Name + " for details"
			}
			if e.Latitude != nil && e.Longitude != nil && to != model.StatusDelivered {
				if near, d := geo.Nearest(*e.Latitude, *e.Longitude); d < 50000 {
					pe.Detail = strings.TrimPrefix(strings.Join([]string{pe.Detail, "near " + near.Name}, " · "), " · ")
				}
			}
		default:
			continue
		}
		t.Timeline = append(t.Timeline, pe)
	}
	return t, nil
}

func coarsePosition(lat, lng float64, at time.Time) *PublicPosition {
	p := &PublicPosition{
		Lat: math.Round(lat*20) / 20, // ~5 km
		Lng: math.Round(lng*20) / 20,
		At:  at,
	}
	near, d := geo.Nearest(lat, lng)
	switch {
	case d < 10000:
		p.Near = "near " + near.Name
	case d < 80000:
		p.Near = fmt.Sprintf("%.0f km from %s", d/1000, near.Name)
	default:
		p.Near = "on the road"
	}
	return p
}

func trackingSteps(status string) []TrackingStep {
	labels := []string{"Booked", "Preparing", "On the way", "Arrived", "Delivered"}
	reached := map[string]int{
		model.StatusPending: 0, model.StatusAssigned: 1,
		model.StatusInTransit: 2, model.StatusDelayed: 2, model.StatusIssue: 2,
		model.StatusArrived: 3, model.StatusDelivered: 4,
	}
	at, ok := reached[status]
	if !ok { // cancelled
		at = -1
	}
	steps := make([]TrackingStep, len(labels))
	for i, l := range labels {
		steps[i] = TrackingStep{Label: l, Done: i <= at, Current: i == at}
	}
	return steps
}

// ---------------------------------------------------------------------------
// Customer confirmation (/c/{token}), sent with the delivery SMS.
// ---------------------------------------------------------------------------

// ConfirmLinkTTL is how long after delivery the confirmation link works.
const ConfirmLinkTTL = 30 * 24 * time.Hour

// ConfirmView is the delivery-confirmation page.
type ConfirmView struct {
	Shipment    db.Shipment
	CompanyName string
	POD         *db.GetPODByShipmentIDRow
}

// ByConfirmToken loads the shipment a confirmation link refers to.
func (s *ShipmentService) ByConfirmToken(ctx context.Context, token string) (*ConfirmView, error) {
	gone := notFound("This link has expired or is not valid.")
	if len(token) < 20 {
		return nil, gone
	}
	sh, err := s.q.GetShipmentByConfirmToken(ctx, &token)
	if isNoRows(err) {
		return nil, gone
	}
	if err != nil {
		return nil, err
	}
	if sh.ActualDelivery.Valid && s.now().Sub(sh.ActualDelivery.Time) > ConfirmLinkTTL {
		return nil, gone
	}
	tenant, err := s.q.GetTenantByID(ctx, sh.TenantID)
	if err != nil {
		return nil, err
	}
	v := &ConfirmView{Shipment: sh, CompanyName: tenant.Name}
	if pod, err := s.q.GetPODByShipmentID(ctx, db.GetPODByShipmentIDParams{ShipmentID: sh.ID, TenantID: sh.TenantID}); err == nil {
		v.POD = &pod
	}
	return v, nil
}

// CustomerRespond records the customer's answer: receipt confirmed, or a
// problem reported, which alerts the office.
func (s *ShipmentService) CustomerRespond(ctx context.Context, token string, ok bool, feedback string) error {
	v, err := s.ByConfirmToken(ctx, token)
	if err != nil {
		return err
	}
	feedback = strings.TrimSpace(truncate(feedback, 1000))
	if !ok && feedback == "" {
		return invalid("feedback", "Tell us what went wrong.")
	}
	sh := v.Shipment
	a := Actor{TenantID: sh.TenantID, Name: sh.CustomerName, Source: SourceCustomer}
	return s.tx(ctx, func(q *db.Queries, after *afterCommit) error {
		if ok {
			if sh.CustomerConfirmedAt.Valid {
				return nil
			}
			if _, err := q.SetCustomerConfirmation(ctx, db.SetCustomerConfirmationParams{ID: sh.ID, Feedback: toText(feedback)}); err != nil {
				return err
			}
			s.event(ctx, q, a, sh, shipmentEvent{Type: "customer_confirmed", Note: feedback, Public: true})
			return nil
		}
		if _, err := q.SetCustomerFeedback(ctx, db.SetCustomerFeedbackParams{ID: sh.ID, Feedback: &feedback}); err != nil {
			return err
		}
		s.event(ctx, q, a, sh, shipmentEvent{Type: "customer_issue", Note: feedback})
		s.raise(ctx, q, after, newAlert{
			TenantID: sh.TenantID, ShipmentID: sh.ID, DriverID: sh.DriverID, Type: AlertCustomerIssue, Severity: SeverityCritical,
			Message: fmt.Sprintf("%s reported a problem with %s: %s", sh.CustomerName, sh.TrackingNumber, truncate(feedback, 200)),
		})
		return nil
	})
}

// ---------------------------------------------------------------------------
// Track by phone: a customer who lost the number proves they hold the phone
// the shipments were booked under.
// ---------------------------------------------------------------------------

// RequestLookupCode texts a code to a phone that has shipments. It answers
// the same whether or not any exist, limit included, so the form reveals
// nothing about a number.
func (s *ShipmentService) RequestLookupCode(ctx context.Context, rawPhone string) (string, error) {
	p, err := phone.Normalize(rawPhone)
	if err != nil {
		return "", invalid("phone", err.Error())
	}
	rows, err := s.q.ListShipmentsByCustomerPhone(ctx, p)
	if err != nil {
		return "", err
	}
	// A number without shipments gets no text, but its code is recorded all
	// the same, so it reaches the hourly limit like any other number.
	var text func(string) string
	if len(rows) > 0 {
		text = func(code string) string {
			return fmt.Sprintf("%s: your code to see your shipments is %s. It expires in 10 minutes.", s.cfg.AppName, code)
		}
	}
	return p, s.issueCode(ctx, toUUID(""), p, codePurposeTrackLookup, text)
}

// VerifyLookupCode checks the code and returns a token proving the phone was
// verified, valid for half an hour.
func (s *ShipmentService) VerifyLookupCode(ctx context.Context, rawPhone, code string) (string, error) {
	p, err := phone.Normalize(rawPhone)
	if err != nil {
		return "", invalid("phone", err.Error())
	}
	if err := s.checkCode(ctx, p, codePurposeTrackLookup, strings.TrimSpace(code)); err != nil {
		return "", err
	}
	exp := strconv.FormatInt(s.now().Add(30*time.Minute).Unix(), 10)
	return p + "." + exp + "." + s.Sign("lookup|"+p+"|"+exp), nil
}

// LookupPhone validates a token from VerifyLookupCode.
func (s *ShipmentService) LookupPhone(token string) (string, bool) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return "", false
	}
	exp, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || s.now().Unix() > exp {
		return "", false
	}
	want := s.Sign("lookup|" + parts[0] + "|" + parts[1])
	if !hmac.Equal([]byte(want), []byte(parts[2])) {
		return "", false
	}
	return parts[0], true
}

// ShipmentsForPhone lists a verified customer's recent shipments.
func (s *ShipmentService) ShipmentsForPhone(ctx context.Context, p string) ([]db.ListShipmentsByCustomerPhoneRow, error) {
	return s.q.ListShipmentsByCustomerPhone(ctx, p)
}
