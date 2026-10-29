package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/natnael-alemayehu/logistics/internal/db"
	"github.com/natnael-alemayehu/logistics/internal/model"
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

func (s *ShipmentService) customerCreatedText(t db.Tenant, sh db.Shipment) string {
	return fmt.Sprintf("%s: shipment %s from %s to %s is booked. Track it: %s",
		t.Name, sh.TrackingNumber, place(sh.OriginCity, sh.OriginAddress),
		place(sh.DestinationCity, sh.DestinationAddress), s.trackURL(sh))
}

// customerStatusText is the customer's message for a status change, if the
// tenant sends one for it.
func (s *ShipmentService) customerStatusText(t db.Tenant, sh db.Shipment, status string, firstDeparture bool, reason, recipient string) (string, bool) {
	dest := place(sh.DestinationCity, sh.DestinationAddress)
	switch status {
	case model.StatusInTransit:
		if !t.SmsOnDispatched || !firstDeparture {
			return "", false
		}
		eta := ""
		if sh.EstimatedDelivery.Valid {
			eta = " Expected " + s.formatWhen(sh.EstimatedDelivery.Time) + "."
		}
		return fmt.Sprintf("%s: shipment %s is on its way to %s.%s Track: %s",
			t.Name, sh.TrackingNumber, dest, eta, s.trackURL(sh)), true
	case model.StatusDelayed:
		if !t.SmsOnDelayed {
			return "", false
		}
		return fmt.Sprintf("%s: shipment %s is delayed (%s). We'll update you. Track: %s",
			t.Name, sh.TrackingNumber, strings.ToLower(model.ReasonLabel(reason)), s.trackURL(sh)), true
	case model.StatusArrived:
		if !t.SmsOnArrived {
			return "", false
		}
		return fmt.Sprintf("%s: shipment %s has arrived in %s. The driver will contact you for handover.",
			t.Name, sh.TrackingNumber, dest), true
	case model.StatusDelivered:
		if !t.SmsOnDelivered {
			return "", false
		}
		who := ""
		if recipient != "" {
			who = " to " + recipient
		}
		link := s.trackURL(sh)
		if sh.ConfirmToken != nil {
			link = s.confirmURL(*sh.ConfirmToken)
		}
		return fmt.Sprintf("%s: shipment %s was delivered%s. Confirm receipt or report a problem: %s",
			t.Name, sh.TrackingNumber, who, link), true
	}
	return "", false
}

func (s *ShipmentService) customerCancelledText(t db.Tenant, sh db.Shipment) string {
	contact := ""
	if t.ContactPhone != nil && *t.ContactPhone != "" {
		contact = " Questions: " + *t.ContactPhone
	}
	return fmt.Sprintf("%s: shipment %s has been cancelled.%s", t.Name, sh.TrackingNumber, contact)
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
	body := fmt.Sprintf("%s: %snew shipment %s, %s to %s. Customer %s %s.%s Open the driver app, or text ET %s 1 when you leave.",
		t.Name, priority, ref, place(sh.OriginCity, sh.OriginAddress), place(sh.DestinationCity, sh.DestinationAddress),
		sh.CustomerName, sh.CustomerPhone, cargo, ref)
	s.enqueueSMS(ctx, q, after, outboundSMS{
		TenantID: t.ID, ShipmentID: sh.ID, Phone: *d.Phone, Body: body, Purpose: PurposeDriverAssigned,
	})
}
