package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/natnael-alemayehu/kiflet/internal/db"
	"github.com/natnael-alemayehu/kiflet/internal/model"
)

// The papers a driver and a truck need on the road: the driver's licence,
// and the truck's insurance and annual inspection (ቦሎ). Each is stored as
// the last day it is valid. The office is reminded PapersNotice before, by
// an alert, and a driver whose licence has run out can't be given a trip.

// PapersNotice is how long before papers run out the office is reminded.
const PapersNotice = 30 * 24 * time.Hour

// Alert types for papers.
const (
	AlertLicenceDue    = "licence_due"
	AlertInsuranceDue  = "insurance_due"
	AlertInspectionDue = "inspection_due"
)

// Paper states.
const (
	PaperNone   = ""       // no date recorded
	PaperValid  = "valid"  // more than PapersNotice to go
	PaperDue    = "due"    // runs out within PapersNotice
	PaperLapsed = "lapsed" // the last valid day has passed
)

// PaperState is where papers valid to the end of day stand on today.
func PaperState(day pgtype.Date, today time.Time) string {
	if !day.Valid {
		return PaperNone
	}
	last := time.Date(day.Time.Year(), day.Time.Month(), day.Time.Day(), 0, 0, 0, 0, time.UTC)
	now := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
	switch {
	case now.After(last):
		return PaperLapsed
	case !last.After(now.Add(PapersNotice)):
		return PaperDue
	}
	return PaperValid
}

// Today is the date in the company's time zone.
func (c *core) Today() time.Time {
	return c.now().In(c.cfg.Location)
}

// FormatDay writes a date as "2 Dec 2026".
func FormatDay(d pgtype.Date) string {
	if !d.Valid {
		return ""
	}
	return d.Time.Format("2 Jan 2006")
}

// parseDay reads a date from a form (2026-12-02). Empty is no date.
func parseDay(field, what, s string) (pgtype.Date, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return pgtype.Date{}, nil
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil || t.Year() < 2000 || t.Year() > 2100 {
		return pgtype.Date{}, invalid(field, "Enter the date "+what+" as a day, month and year.")
	}
	return toDate(t), nil
}

func sameDay(a, b pgtype.Date) bool {
	return a.Valid == b.Valid && (!a.Valid || a.Time.Equal(b.Time))
}

// papersMessage is what the alert about papers says.
func papersMessage(kind, name string, due pgtype.Date, lapsed bool) string {
	day := FormatDay(due)
	switch kind {
	case "licence":
		if lapsed {
			return fmt.Sprintf("%s's driving licence ran out on %s. They can't be given trips until the renewed one is recorded.", name, day)
		}
		return fmt.Sprintf("%s's driving licence runs out on %s. After that they can't be given trips.", name, day)
	case "insurance":
		if lapsed {
			return fmt.Sprintf("The insurance on %s ran out on %s.", name, day)
		}
		return fmt.Sprintf("The insurance on %s runs out on %s.", name, day)
	}
	if lapsed {
		return fmt.Sprintf("The annual inspection (ቦሎ) of %s was due on %s.", name, day)
	}
	return fmt.Sprintf("The annual inspection (ቦሎ) of %s is due on %s.", name, day)
}

var papersAlert = map[string]string{"licence": AlertLicenceDue, "insurance": AlertInsuranceDue, "inspection": AlertInspectionDue}

// CheckPapers reminds each company of papers that run out within
// PapersNotice, and again, critically, once they have. Each reminder is
// raised once per date: renewing the papers resolves it.
func (j *Jobs) CheckPapers(ctx context.Context) error {
	today := j.Today()
	rows, err := j.q.ListPapersDue(ctx, toDate(today.Add(PapersNotice)))
	if err != nil {
		return err
	}
	for _, r := range rows {
		lapsed := PaperState(r.DueOn, today) == PaperLapsed
		episode, severity := r.DueOn.Time.Format("2006-01-02"), SeverityWarning
		if lapsed {
			episode, severity = episode+"/lapsed", SeverityCritical
		}
		a := newAlert{TenantID: r.TenantID, Type: papersAlert[r.Kind], Severity: severity,
			Message: papersMessage(r.Kind, r.Name, r.DueOn, lapsed), Episode: episode}
		if r.Kind == "licence" {
			a.DriverID = r.SubjectID
		} else {
			a.VehicleID = r.SubjectID
		}
		err := j.tx(ctx, func(q *db.Queries, after *afterCommit) error {
			// The reminder that it was coming gives way to the one that it
			// has come.
			if err := q.AcknowledgePapersAlerts(ctx, db.AcknowledgePapersAlertsParams{
				TenantID: r.TenantID, AlertType: a.Type, DriverID: a.DriverID, VehicleID: a.VehicleID, KeepEpisode: &episode,
			}); err != nil {
				return err
			}
			j.raise(ctx, q, after, a)
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// papersRenewed resolves the alerts about papers whose date has changed.
func (c *core) papersRenewed(ctx context.Context, q *db.Queries, tenantID pgtype.UUID, alertType string, driverID, vehicleID pgtype.UUID) error {
	return q.AcknowledgePapersAlerts(ctx, db.AcknowledgePapersAlertsParams{
		TenantID: tenantID, AlertType: alertType, DriverID: driverID, VehicleID: vehicleID,
	})
}

// licenceLapsed is why a driver can't be given a trip, or nil.
func (c *core) licenceLapsed(d db.User) error {
	if PaperState(d.LicenceExpiresOn, c.Today()) != PaperLapsed {
		return nil
	}
	return invalid("driver_id", fmt.Sprintf("%s's driving licence ran out on %s. Record the renewed licence on their page before giving them a trip.",
		d.FullName, FormatDay(d.LicenceExpiresOn)))
}

// RemindLicence texts a driver whose licence runs out soon, or has, to
// renew it.
func (s *UserService) RemindLicence(ctx context.Context, p *Principal, id pgtype.UUID, ci ClientInfo) error {
	if !p.CanManageFleet() {
		return ErrUnauthorized
	}
	return s.tx(ctx, func(q *db.Queries, after *afterCommit) error {
		d, err := s.Get(ctx, p.TenantID, id)
		if err != nil {
			return err
		}
		if d.Role != model.RoleDriver || !d.IsActive {
			return notFound("Driver not found.")
		}
		state := PaperState(d.LicenceExpiresOn, s.Today())
		if state != PaperDue && state != PaperLapsed {
			return conflict(d.FullName + "'s licence isn't due for renewal.")
		}
		if d.Phone == nil {
			return conflict(d.FullName + " has no phone number to text.")
		}
		tenant, err := q.GetTenantByID(ctx, p.TenantID)
		if err != nil {
			return err
		}
		verb := "runs out on"
		if state == PaperLapsed {
			verb = "ran out on"
		}
		s.enqueueSMS(ctx, q, after, outboundSMS{
			TenantID: p.TenantID, Phone: *d.Phone, Purpose: PurposeDriverMessage,
			Body: fmt.Sprintf("%s: your driving licence %s %s. Please renew it and show the office the new one.",
				tenant.Name, verb, FormatDay(d.LicenceExpiresOn)),
		})
		s.audit(ctx, q, p.Actor(SourceDashboard, ci.IP, ci.UserAgent), "licence_reminder_sent", "user", id, nil,
			map[string]any{"expires": FormatDay(d.LicenceExpiresOn)})
		return nil
	})
}
