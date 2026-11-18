package service

import (
	"context"
	"crypto/hmac"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/natnael-alemayehu/kiflet/internal/db"
	"github.com/natnael-alemayehu/kiflet/internal/geo"
	"github.com/natnael-alemayehu/kiflet/internal/model"
	"github.com/natnael-alemayehu/kiflet/internal/phone"
	"github.com/natnael-alemayehu/kiflet/internal/text"
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
	// KmLeft is roughly how far the truck has still to drive, by road, from
	// where it was last seen; 0 when that isn't known.
	KmLeft int
	// ConfirmByText: the receiver is texted a link to confirm the delivery.
	ConfirmByText bool
	// Lang is the language the labels are in: the one asked for, or the
	// one the company writes to its customers in.
	Lang string
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
	At      *time.Time // when it was reached, if it was
}

// PublicEvent is one line of the public timeline.
type PublicEvent struct {
	At     time.Time
	Label  string
	Detail string
}

// Track looks a shipment up by its tracking number. Its labels are in lang,
// or, when lang is empty, in the language the company writes to its
// customers in.
func (s *ShipmentService) Track(ctx context.Context, number, lang string) (*PublicTracking, error) {
	number = strings.ToUpper(strings.TrimSpace(number))
	sh, err := s.q.GetShipmentByTrackingNumber(ctx, number)
	if isNoRows(err) {
		return nil, &Error{Kind: KindNotFound, Key: "error.number_unknown", Message: text.T(text.English, "error.number_unknown")}
	}
	if err != nil {
		return nil, err
	}
	tenant, err := s.q.GetTenantByID(ctx, sh.TenantID)
	if err != nil {
		return nil, err
	}
	if !text.Valid(lang) {
		lang = tenant.CustomerLanguage
	}

	t := &PublicTracking{
		TrackingNumber: sh.TrackingNumber,
		Status:         sh.Status,
		StatusLabel:    text.Status(lang, sh.Status),
		CompanyName:    tenant.Name,
		CompanyPhone:   deref(tenant.ContactPhone),
		Origin:         place(sh.OriginCity, sh.OriginAddress),
		Destination:    place(sh.DestinationCity, sh.DestinationAddress),
		ETA:            timePtr(sh.EstimatedDelivery),
		DeliveredAt:    timePtr(sh.ActualDelivery),
		Confirmed:      sh.CustomerConfirmedAt.Valid,
		ConfirmByText:  tenant.SmsOnDelivered,
		Lang:           lang,
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
			t.LastSeen = coarsePosition(lang, pos.Latitude, pos.Longitude, pos.RecordedAt.Time)
			if t.DestLat != 0 {
				t.KmLeft = int(math.Round(geo.RoadKM(pos.Latitude, pos.Longitude, t.DestLat, t.DestLng)))
			}
		}
	}

	t.Steps = trackingSteps(lang, sh.Status)

	events, err := s.q.ListPublicShipmentEvents(ctx, sh.ID)
	if err != nil {
		return nil, err
	}
	reach := func(step int, at time.Time) {
		if step < len(t.Steps) && t.Steps[step].Done && t.Steps[step].At == nil {
			t.Steps[step].At = &at
		}
	}
	for _, e := range events {
		pe := PublicEvent{At: e.OccurredAt.Time}
		switch e.EventType {
		case "created":
			reach(0, pe.At)
		case "assigned":
			reach(1, pe.At)
		case "status_changed", "status_updated":
			switch deref(e.ToStatus) {
			case model.StatusInTransit:
				reach(2, pe.At)
			case model.StatusArrived:
				reach(3, pe.At)
			case model.StatusDelivered:
				reach(4, pe.At)
			}
		}
		switch e.EventType {
		case "created":
			pe.Label = text.T(lang, "event.booked")
		case "assigned":
			pe.Label = text.T(lang, "event.assigned")
		case "customer_confirmed":
			pe.Label = text.T(lang, "event.confirmed")
		case "status_changed", "status_updated":
			to := deref(e.ToStatus)
			pe.Label = text.Status(lang, to)
			if to == model.StatusDelayed && e.Reason != nil {
				pe.Detail = text.Reason(lang, *e.Reason)
			}
			if to == model.StatusCancelled {
				pe.Detail = text.T(lang, "event.contact", tenant.Name)
			}
			if e.Latitude != nil && e.Longitude != nil && to != model.StatusDelivered {
				if near, d := geo.Nearest(*e.Latitude, *e.Longitude); d < 50000 {
					pe.Detail = strings.TrimPrefix(strings.Join([]string{pe.Detail, text.T(lang, "place.near", near.Name)}, " · "), " · ")
				}
			}
		default:
			continue
		}
		t.Timeline = append(t.Timeline, pe)
	}
	return t, nil
}

func coarsePosition(lang string, lat, lng float64, at time.Time) *PublicPosition {
	p := &PublicPosition{
		Lat: math.Round(lat*20) / 20, // ~5 km
		Lng: math.Round(lng*20) / 20,
		At:  at,
	}
	near, d := geo.Nearest(lat, lng)
	switch {
	case d < 10000:
		p.Near = text.T(lang, "place.near", near.Name)
	case d < 80000:
		p.Near = text.T(lang, "place.km_from", int(math.Round(d/1000)), near.Name)
	default:
		p.Near = text.T(lang, "place.on_road")
	}
	return p
}

func trackingSteps(lang, status string) []TrackingStep {
	labels := []string{"step.booked", "step.preparing", "step.on_the_way", "step.arrived", "step.delivered"}
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
		steps[i] = TrackingStep{Label: text.T(lang, l), Done: i <= at, Current: i == at}
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
	// Language is the one the company writes to its customers in.
	Language string
	POD      *db.GetPODByShipmentIDRow
}

// ByConfirmToken loads the shipment a confirmation link refers to.
func (s *ShipmentService) ByConfirmToken(ctx context.Context, token string) (*ConfirmView, error) {
	gone := &Error{Kind: KindNotFound, Key: "error.link_gone", Message: text.T(text.English, "error.link_gone")}
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
	v := &ConfirmView{Shipment: sh, CompanyName: tenant.Name, Language: tenant.CustomerLanguage}
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
		return &Error{Kind: KindInvalid, Field: "feedback", Key: "error.feedback_required", Message: text.T(text.English, "error.feedback_required")}
	}
	sh := v.Shipment
	// The link went to whoever took delivery, so the answer is theirs.
	who, _ := receiver(sh)
	a := Actor{TenantID: sh.TenantID, Name: who, Source: SourceCustomer}
	return s.tx(ctx, func(q *db.Queries, after *afterCommit) error {
		if ok {
			if sh.CustomerConfirmedAt.Valid {
				return nil
			}
			if _, err := q.SetCustomerConfirmation(ctx, db.SetCustomerConfirmationParams{ID: sh.ID, TenantID: sh.TenantID, Feedback: toText(feedback)}); err != nil {
				return err
			}
			s.event(ctx, q, after, a, sh, shipmentEvent{Type: "customer_confirmed", Note: feedback, Public: true})
			return nil
		}
		if _, err := q.SetCustomerFeedback(ctx, db.SetCustomerFeedbackParams{ID: sh.ID, TenantID: sh.TenantID, Feedback: &feedback}); err != nil {
			return err
		}
		s.event(ctx, q, after, a, sh, shipmentEvent{Type: "customer_issue", Note: feedback})
		s.raise(ctx, q, after, newAlert{
			TenantID: sh.TenantID, ShipmentID: sh.ID, DriverID: sh.DriverID, Type: AlertCustomerIssue, Severity: SeverityCritical,
			Message: fmt.Sprintf("%s reported a problem with %s: %s", who, sh.TrackingNumber, truncate(feedback, 200)),
		})
		return nil
	})
}

// ---------------------------------------------------------------------------
// Track by phone: a customer who lost the number proves they hold the phone
// the shipments were booked under.
// ---------------------------------------------------------------------------

var errLookupPhone = &Error{Kind: KindInvalid, Field: "phone", Key: "error.phone", Message: text.T(text.English, "error.phone")}

// RequestLookupCode texts a code to a phone that has shipments. It answers
// the same whether or not any exist, limit included, so the form reveals
// nothing about a number. The text is in lang, the language of the page
// that asked.
func (s *ShipmentService) RequestLookupCode(ctx context.Context, rawPhone, lang string) (string, error) {
	p, err := phone.Normalize(rawPhone)
	if err != nil {
		return "", errLookupPhone
	}
	rows, err := s.q.ListShipmentsByCustomerPhone(ctx, p)
	if err != nil {
		return "", err
	}
	// A number without shipments gets no text, but its code is recorded all
	// the same, so it reaches the hourly limit like any other number.
	var body func(string) string
	if len(rows) > 0 {
		body = func(code string) string {
			return text.T(lang, "sms.lookup_code", s.cfg.AppName, code)
		}
	}
	return p, s.issueCode(ctx, toUUID(""), p, codePurposeTrackLookup, body)
}

// VerifyLookupCode checks the code and returns a token proving the phone was
// verified, valid for half an hour.
func (s *ShipmentService) VerifyLookupCode(ctx context.Context, rawPhone, code string) (string, error) {
	p, err := phone.Normalize(rawPhone)
	if err != nil {
		return "", errLookupPhone
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
