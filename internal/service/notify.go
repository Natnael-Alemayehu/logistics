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

// customerStatusText is what a status change says to the receiver and to
// the customer who booked, if the tenant sends a text for it. They are
// told apart only when the shipment names a consignee: the receiver is the
// one the driver hands over to, so the arrival and the link to confirm
// receipt are theirs, and the customer is told the delivery was made.
func (s *ShipmentService) customerStatusText(t db.Tenant, sh db.Shipment, status string, firstDeparture bool, reason, recipient string) (toReceiver, toCustomer string) {
	lang := t.CustomerLanguage
	dest := place(sh.DestinationCity, sh.DestinationAddress)
	switch status {
	case model.StatusInTransit:
		if !t.SmsOnDispatched || !firstDeparture {
			return "", ""
		}
		eta := ""
		if sh.EstimatedDelivery.Valid {
			eta = text.T(lang, "sms.eta", text.ShortWhen(lang, sh.EstimatedDelivery.Time.In(s.cfg.Location)))
		}
		body := text.T(lang, "sms.on_way", t.Name, sh.TrackingNumber, dest, eta, s.trackURL(sh))
		return body, body
	case model.StatusDelayed:
		if !t.SmsOnDelayed {
			return "", ""
		}
		body := text.T(lang, "sms.delayed",
			t.Name, sh.TrackingNumber, strings.ToLower(text.Reason(lang, reason)), s.trackURL(sh))
		return body, body
	case model.StatusArrived:
		if !t.SmsOnArrived {
			return "", ""
		}
		return text.T(lang, "sms.arrived", t.Name, sh.TrackingNumber, dest), ""
	case model.StatusDelivered:
		if !t.SmsOnDelivered {
			return "", ""
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
		when := text.ShortWhen(lang, at.In(s.cfg.Location))
		return text.T(lang, "sms.delivered", t.Name, sh.TrackingNumber, who, when, link),
			text.T(lang, "sms.delivered_booker", t.Name, sh.TrackingNumber, who, when, s.trackURL(sh))
	}
	return "", ""
}

func (s *ShipmentService) customerCancelledText(t db.Tenant, sh db.Shipment) string {
	contact := ""
	if t.ContactPhone != nil && *t.ContactPhone != "" {
		contact = text.T(t.CustomerLanguage, "sms.questions", *t.ContactPhone)
	}
	return text.T(t.CustomerLanguage, "sms.cancelled", t.Name, sh.TrackingNumber, contact)
}

// receiver is who takes delivery of a shipment: its consignee, or the
// customer who booked it when it names none.
func receiver(sh db.Shipment) (name, phone string) {
	if sh.ConsigneePhone != nil {
		return deref(sh.ConsigneeName), *sh.ConsigneePhone
	}
	return sh.CustomerName, sh.CustomerPhone
}

// notifyCustomer texts the people a shipment is for: toReceiver goes to
// whoever takes delivery, and toCustomer to the customer who booked, when
// that is someone else. An empty text is not sent.
func (s *ShipmentService) notifyCustomer(ctx context.Context, q *db.Queries, after *afterCommit, t db.Tenant, sh db.Shipment, toReceiver, toCustomer string) {
	send := func(phone, body string) {
		if body == "" {
			return
		}
		s.enqueueSMS(ctx, q, after, outboundSMS{
			TenantID: t.ID, ShipmentID: sh.ID, Phone: phone, Body: body, Purpose: PurposeCustomerUpdate,
		})
	}
	_, to := receiver(sh)
	send(to, toReceiver)
	if sh.ConsigneePhone != nil {
		send(sh.CustomerPhone, toCustomer)
	}
}

// meet is who the driver is told to meet: "Customer Name 09…", or "Deliver
// to Name 09…" when the receiver isn't the customer.
func meet(sh db.Shipment) string {
	name, phone := receiver(sh)
	if sh.ConsigneePhone != nil {
		return "Deliver to " + name + " " + phone
	}
	return "Customer " + name + " " + phone
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
	body := fmt.Sprintf("%s: %snew shipment %s, %s to %s.%s %s.%s%s",
		t.Name, priority, ref, place(sh.OriginCity, sh.OriginAddress), place(sh.DestinationCity, sh.DestinationAddress),
		pickup, meet(sh), cargo, report)
	s.enqueueSMS(ctx, q, after, outboundSMS{
		TenantID: t.ID, ShipmentID: sh.ID, Phone: *d.Phone, Body: body, Purpose: PurposeDriverAssigned,
	})
}
