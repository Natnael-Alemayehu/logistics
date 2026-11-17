package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

type auditRow struct {
	At      time.Time
	Who     string
	Action  string
	Entity  string
	Link    string
	Changes []auditChange
	IP      string
}

type auditChange struct{ Field, From, To string }

var auditActions = map[string]string{
	"login":                      "Signed in",
	"login_failed":               "Failed sign-in",
	"login_locked":               "Account locked after failed sign-ins",
	"password_changed":           "Changed their password",
	"pin_changed":                "Changed their PIN",
	"pin_reset":                  "Reset their PIN by SMS",
	"credential_reset":           "Issued a temporary sign-in",
	"driver_created":             "Added a driver",
	"driver_updated":             "Edited a driver",
	"staff_created":              "Added a team member",
	"staff_updated":              "Edited a team member",
	"user_deactivated":           "Deactivated an account",
	"user_reactivated":           "Reactivated an account",
	"vehicle_created":            "Added a vehicle",
	"vehicle_updated":            "Edited a vehicle",
	"settings_updated":           "Changed company settings",
	"shipments_imported":         "Imported shipments from a file",
	"company_signup":             "Created the company",
	"password_reset_requested":   "Asked for a password reset link",
	"password_reset":             "Reset their password by email",
	"email_verified":             "Confirmed their email address",
	"pod_accepted":               "Accepted a held delivery",
	"pod_dismissed":              "Dismissed a held delivery",
	"tenant_suspended":           "Suspended a company",
	"tenant_restored":            "Restored a company",
	"plan_chosen":                "Chose a plan",
	"payment_started":            "Started a payment",
	"invoice_paid":               "Invoice paid",
	"invoice_marked_paid":        "Recorded a bank transfer",
	"invoice_voided":             "Cancelled an invoice",
	"payment_refund_due":         "Payment to refund",
	"payment_refunded":           "Payment refunded",
	"subscription_renewed":       "Subscription renewed",
	"renewal_cancelled":          "Turned off renewal",
	"renewal_resumed":            "Turned renewal back on",
	"trial_extended":             "Extended a free trial",
	"enterprise_offer_set":       "Set Enterprise terms",
	"enterprise_offer_withdrawn": "Withdrew Enterprise terms",
	"drivers_messaged":           "Messaged every driver on the road",
	"logout":                     "Signed out",
	"user_anonymised":            "Removed a former member's details",
	"data_exported":              "Downloaded the company's data",
	"tenant_deleted":             "Deleted a company",
	"api_key_created":            "Made an API key",
	"api_key_revoked":            "Revoked an API key",
	"webhook_created":            "Added a webhook",
	"webhook_updated":            "Changed a webhook",
	"webhook_paused":             "Paused a webhook",
	"webhook_resumed":            "Resumed a webhook",
	"webhook_secret_rolled":      "Gave a webhook a new secret",
	"webhook_deleted":            "Removed a webhook",
}

func (s *Server) userNames(ctx context.Context, tenantID pgtype.UUID) map[pgtype.UUID]string {
	out := s.driverNames(ctx, tenantID)
	if staff, err := s.svc.Users.Staff(ctx, tenantID); err == nil {
		for _, u := range staff {
			out[u.ID] = u.FullName
		}
	}
	return out
}

func (s *Server) auditLog(w http.ResponseWriter, r *http.Request) {
	p := principal(r)
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	page = max(page, 1)
	const per = 50
	logs, err := s.svc.Audit.List(r.Context(), p.TenantID, page, per+1)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	more := len(logs) > per
	if more {
		logs = logs[:per]
	}
	names := s.userNames(r.Context(), p.TenantID)
	drivers := s.driverNames(r.Context(), p.TenantID)
	rows := make([]auditRow, 0, len(logs))
	for _, l := range logs {
		row := auditRow{At: l.CreatedAt.Time, Who: names[l.UserID], Action: auditActions[l.Action], Entity: l.EntityType, IP: deref(l.IpAddress)}
		if row.Who == "" {
			row.Who = "System"
		}
		if row.Action == "" {
			row.Action = strings.ReplaceAll(l.Action, "_", " ")
		}
		if l.EntityID.Valid {
			switch l.EntityType {
			case "user":
				if n, ok := names[l.EntityID]; ok {
					row.Entity = n
				}
				if _, isDriver := drivers[l.EntityID]; isDriver {
					row.Link = "/app/drivers/" + l.EntityID.String()
				}
			case "vehicle":
				row.Link = "/app/vehicles/" + l.EntityID.String()
				row.Entity = "vehicle"
			case "shipment":
				row.Link = "/app/shipments/" + l.EntityID.String()
			}
		}
		row.Changes = auditDiff(l.OldValue, l.NewValue)
		rows = append(rows, row)
	}
	v := s.view(w, r, "Audit log", "settings", map[string]any{"Rows": rows, "Page": page, "More": more})
	s.render(w, r, http.StatusOK, "app/audit", v)
}

// auditDiff lists the fields that differ between the old and new values.
func auditDiff(oldRaw, newRaw []byte) []auditChange {
	var before, after map[string]any
	_ = json.Unmarshal(oldRaw, &before)
	_ = json.Unmarshal(newRaw, &after)
	var out []auditChange
	for k, nv := range after {
		ov, had := before[k]
		if had && fmt.Sprint(ov) == fmt.Sprint(nv) {
			continue
		}
		c := auditChange{Field: strings.ReplaceAll(k, "_", " "), To: fmt.Sprint(nv)}
		if had {
			c.From = fmt.Sprint(ov)
		}
		out = append(out, c)
	}
	return out
}
