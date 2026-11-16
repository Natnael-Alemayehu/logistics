package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/natnael-alemayehu/kiflet/internal/db"
	"github.com/natnael-alemayehu/kiflet/internal/model"
	"github.com/natnael-alemayehu/kiflet/internal/text"
)

// The texts customers and drivers receive. They are kept short — a single
// 160-character SMS where possible — and always name the company, since the
// sender ID is the platform's rather than theirs.

func (c *core) trackURL(sh db.Shipment) string {
	return c.cfg.BaseURL + "/track/" + sh.TrackingNumber
}

func (c *core) confirmURL(token string) string {
	return c.cfg.BaseURL + "/c/" + token
}

func (c *core) formatWhen(t time.Time) string {
	return t.In(c.cfg.Location).Format("Mon 2 Jan 15:04")
}

func place(city *string, address string) string {
	if city != nil && *city != "" {
		return *city
	}
	return address
}

// The customer's texts are written in the language the company chose for
// its customers (tenants.customer_language), from the catalogue in
// internal/text.

func (s *ShipmentService) customerCreatedText(t db.Tenant, sh db.Shipment) string {
	return text.T(t.CustomerLanguage, "sms.created",
		t.Name, sh.TrackingNumber, place(sh.OriginCity, sh.OriginAddress),
		place(sh.DestinationCity, sh.DestinationAddress), s.trackURL(sh))
}

// customerStatusText is the customer's message for a status change, if the
// tenant sends one for it.
func (s *ShipmentService) customerStatusText(t db.Tenant, sh db.Shipment, status string, firstDeparture bool, reason, recipient string) (string, bool) {
	lang := t.CustomerLanguage
	dest := place(sh.DestinationCity, sh.DestinationAddress)
	switch status {
	case model.StatusInTransit:
		if !t.SmsOnDispatched || !firstDeparture {
			return "", false
		}
		eta := ""
		if sh.EstimatedDelivery.Valid {
			eta = text.T(lang, "sms.eta", text.ShortWhen(lang, sh.EstimatedDelivery.Time.In(s.cfg.Location)))
		}
		return text.T(lang, "sms.on_way", t.Name, sh.TrackingNumber, dest, eta, s.trackURL(sh)), true
	case model.StatusDelayed:
		if !t.SmsOnDelayed {
			return "", false
		}
		return text.T(lang, "sms.delayed",
			t.Name, sh.TrackingNumber, strings.ToLower(text.Reason(lang, reason)), s.trackURL(sh)), true
	case model.StatusArrived:
		if !t.SmsOnArrived {
			return "", false
		}
		return text.T(lang, "sms.arrived", t.Name, sh.TrackingNumber, dest), true
	case model.StatusDelivered:
		if !t.SmsOnDelivered {
			return "", false
		}
		who := ""
		if recipient != "" {
			who = text.T(lang, "sms.delivered_to", recipient)
		}
		at := s.now()
		if sh.ActualDelivery.Valid {
			at = sh.ActualDelivery.Time
		}
		link := s.trackURL(sh)
		if sh.ConfirmToken != nil {
			link = s.confirmURL(*sh.ConfirmToken)
		}
		return text.T(lang, "sms.delivered",
			t.Name, sh.TrackingNumber, who, text.ShortWhen(lang, at.In(s.cfg.Location)), link), true
	}
	return "", false
}

func (s *ShipmentService) customerCancelledText(t db.Tenant, sh db.Shipment) string {
	contact := ""
	if t.ContactPhone != nil && *t.ContactPhone != "" {
		contact = text.T(t.CustomerLanguage, "sms.questions", *t.ContactPhone)
	}
	return text.T(t.CustomerLanguage, "sms.cancelled", t.Name, sh.TrackingNumber, contact)
}

func (s *ShipmentService) notifyCustomer(ctx context.Context, q *db.Queries, after *afterCommit, t db.Tenant, sh db.Shipment, body string) {
	s.enqueueSMS(ctx, q, after, outboundSMS{
		TenantID: t.ID, ShipmentID: sh.ID, Phone: sh.CustomerPhone, Body: body, Purpose: PurposeCustomerUpdate,
	})
}

func (s *ShipmentService) notifyDriverAssigned(ctx context.Context, q *db.Queries, after *afterCommit, t db.Tenant, sh db.Shipment, d db.User) {
	if d.Phone == nil {
		return
	}
	ref := ShortRef(sh.TrackingNumber)
	cargo := ""
	if sh.CargoDescription != nil {
		cargo = " " + truncate(*sh.CargoDescription, 40) + "."
	}
	priority := ""
	if sh.Priority != model.PriorityNormal {
		priority = strings.ToUpper(sh.Priority) + " "
	}
	pickup := ""
	if sh.PlannedPickupAt.Valid {
		pickup = " Pickup " + s.formatWhen(sh.PlannedPickupAt.Time) + "."
	}
	// Without an inbound number there is nothing to text, only the app.
	report := " Open the driver app when you leave."
	if s.cfg.SMSNumber != "" {
		report = fmt.Sprintf(" When you leave, open the driver app or text ET %s 1 to %s.", ref, s.cfg.SMSNumber)
	}
	body := fmt.Sprintf("%s: %snew shipment %s, %s to %s.%s Customer %s %s.%s%s",
		t.Name, priority, ref, place(sh.OriginCity, sh.OriginAddress), place(sh.DestinationCity, sh.DestinationAddress),
		pickup, sh.CustomerName, sh.CustomerPhone, cargo, report)
	s.enqueueSMS(ctx, q, after, outboundSMS{
		TenantID: t.ID, ShipmentID: sh.ID, Phone: *d.Phone, Body: body, Purpose: PurposeDriverAssigned,
	})
}
