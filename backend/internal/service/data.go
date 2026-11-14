package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/natnael-alemayehu/logistics/internal/db"
	"github.com/natnael-alemayehu/logistics/internal/model"
)

// DataService covers what happens to a company's data (FR-041): its export,
// removing a person's details, removing customers' details once the keeping
// period is over, and deleting a company (AC-049.3).
type DataService struct {
	*core
	media *MediaService
}

// RetainShipments is how long a finished shipment keeps its customer and
// delivery details.
func (s *DataService) RetainShipments() time.Duration { return s.cfg.RetainShipments }

// purgeTx is tx for the few changes the append-only audit log and shipment
// timeline allow: removing personal data and deleting a company.
func (c *core) purgeTx(ctx context.Context, fn func(q *db.Queries, after *afterCommit) error) error {
	return c.tx(ctx, func(q *db.Queries, after *afterCommit) error {
		if err := q.AllowLogRewrite(ctx); err != nil {
			return err
		}
		return fn(q, after)
	})
}

// Export is everything a company holds, for its download (AC-041.5).
type Export struct {
	Company   db.Tenant
	At        time.Time
	Shipments []db.ExportShipmentsRow
	Events    []db.ExportShipmentEventsRow
	PODs      []db.ExportPODsRow
	People    []db.ExportPeopleRow
	Vehicles  []db.ExportVehiclesRow
}

// Export reads the company's data in one snapshot, so the files agree with
// each other.
func (s *DataService) Export(ctx context.Context, p *Principal, ci ClientInfo) (*Export, error) {
	if !p.CanManageCompany() || p.IsPlatformAdmin() {
		return nil, ErrUnauthorized
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // read only
	q := s.q.WithTx(tx)
	out := &Export{At: s.now()}
	if out.Company, err = q.GetTenantByID(ctx, p.TenantID); err != nil {
		return nil, err
	}
	if out.Shipments, err = q.ExportShipments(ctx, p.TenantID); err != nil {
		return nil, err
	}
	if out.Events, err = q.ExportShipmentEvents(ctx, p.TenantID); err != nil {
		return nil, err
	}
	if out.PODs, err = q.ExportPODs(ctx, p.TenantID); err != nil {
		return nil, err
	}
	if out.People, err = q.ExportPeople(ctx, p.TenantID); err != nil {
		return nil, err
	}
	if out.Vehicles, err = q.ExportVehicles(ctx, p.TenantID); err != nil {
		return nil, err
	}
	s.audit(ctx, s.q, p.Actor(SourceDashboard, ci.IP, ci.UserAgent), "data_exported", "tenant", p.TenantID, nil,
		map[string]any{"shipments": len(out.Shipments), "deliveries": len(out.PODs), "people": len(out.People)})
	return out, nil
}

// AnonymisePerson removes what identifies someone who has left the company:
// their name, phone, email and sign-in, and the texts and emails sent to
// them. Their shipments, deliveries and history stay, under "Former driver"
// or "Former staff member" (AC-041.6). The audit log keeps its record of
// what they did.
func (s *DataService) AnonymisePerson(ctx context.Context, p *Principal, id pgtype.UUID, ci ClientInfo) (string, error) {
	if !p.CanManageCompany() || p.IsPlatformAdmin() {
		return "", ErrUnauthorized
	}
	if id == p.UserID {
		return "", invalid("", "You can't remove your own details.")
	}
	var was string
	err := s.purgeTx(ctx, func(q *db.Queries, after *afterCommit) error {
		u, err := q.GetUserByID(ctx, db.GetUserByIDParams{ID: id, TenantID: p.TenantID})
		if isNoRows(err) {
			return notFound("Person not found.")
		}
		if err != nil {
			return err
		}
		if u.AnonymisedAt.Valid {
			return conflict("This person's details were already removed.")
		}
		if u.IsActive {
			return conflict("Deactivate " + u.FullName + " first.")
		}
		was = u.FullName
		name := "Former staff member"
		if u.Role == model.RoleDriver {
			name = "Former driver"
		}
		if _, err := q.AnonymiseUser(ctx, db.AnonymiseUserParams{FullName: name, ID: id, TenantID: p.TenantID}); err != nil {
			return err
		}
		if err := q.AnonymiseActorName(ctx, db.AnonymiseActorNameParams{ActorName: name, ActorID: id, TenantID: p.TenantID}); err != nil {
			return err
		}
		if err := q.AnonymiseSupportAuthor(ctx, db.AnonymiseSupportAuthorParams{AuthorName: name, AuthorID: id, TenantID: p.TenantID}); err != nil {
			return err
		}
		if err := q.DeletePersonDeviceData(ctx, id); err != nil {
			return err
		}
		if err := q.DeleteMessagesToPerson(ctx, db.DeleteMessagesToPersonParams{
			TenantID: p.TenantID, Phone: deref(u.Phone), Email: deref(u.Email),
		}); err != nil {
			return err
		}
		if err := q.RevokeAllUserSessions(ctx, id); err != nil {
			return err
		}
		s.audit(ctx, q, p.Actor(SourceDashboard, ci.IP, ci.UserAgent), "user_anonymised", "user", id, nil, map[string]any{"role": u.Role})
		return nil
	})
	return was, err
}

// DeleteCompany removes a company and everything it holds: people,
// vehicles, shipments, deliveries and their photos (AC-049.3). Its invoices
// stay for the accounts, the open ones cancelled, and the audit log records
// the deletion. The platform admin confirms by typing the company's name.
func (s *DataService) DeleteCompany(ctx context.Context, p *Principal, id pgtype.UUID, confirm string, ci ClientInfo) (string, error) {
	if !p.IsPlatformAdmin() {
		return "", ErrUnauthorized
	}
	if id == p.TenantID {
		return "", invalid("", "You can't delete the platform's own account.")
	}
	var name string
	var keys []string
	err := s.purgeTx(ctx, func(q *db.Queries, after *afterCommit) error {
		t, err := q.TenantForDeletion(ctx, id)
		if isNoRows(err) {
			return notFound("Company not found.")
		}
		if err != nil {
			return err
		}
		if strings.TrimSpace(confirm) != t.Name {
			return invalid("confirm", "Type the company's name exactly as shown to delete it.")
		}
		name = t.Name
		media, err := q.ListTenantPODMedia(ctx, id)
		if err != nil {
			return err
		}
		for _, m := range media {
			keys = append(keys, m.SignatureKey)
			keys = append(keys, m.PhotoKeys...)
		}
		voided, err := q.VoidTenantInvoices(ctx, id)
		if err != nil {
			return err
		}
		if err := q.DeleteTenantAuditLogs(ctx, id); err != nil {
			return err
		}
		if err := q.DeleteTenant(ctx, id); err != nil {
			return err
		}
		s.audit(ctx, q, p.Actor(SourceDashboard, ci.IP, ci.UserAgent), "tenant_deleted", "tenant", id, nil, map[string]any{
			"name": t.Name, "shipments": t.Shipments, "people": t.People, "vehicles": t.Vehicles,
			"invoices_kept": t.Invoices, "invoices_cancelled": strings.Join(voided, ", "),
		})
		// A large fleet has thousands of photos: they go in the background.
		after.do(func() {
			go func() {
				s.media.Delete(context.Background(), keys...)
				s.log.Info().Str("company", name).Int("files", len(keys)).Msg("deleted company's delivery photos")
			}()
		})
		return nil
	})
	return name, err
}

// anonymiseBatch is how many shipments one retention step clears.
const anonymiseBatch = 500

// AnonymiseFinished removes customer and delivery details from shipments
// that finished before the cutoff: names, phones, addresses, notes,
// signatures and photos (AC-041.2, AC-041.3). What reports count stays
// (AC-041.4). It reports how many shipments it cleared.
func (s *DataService) AnonymiseFinished(ctx context.Context, before time.Time) (int, error) {
	total := 0
	for range 20 { // at most 10,000 an hour
		ids, err := s.q.ListShipmentsToAnonymise(ctx, db.ListShipmentsToAnonymiseParams{Before: toTimestamp(before), MaxRows: anonymiseBatch})
		if err != nil || len(ids) == 0 {
			return total, err
		}
		var keys []string
		err = s.purgeTx(ctx, func(q *db.Queries, after *afterCommit) error {
			media, err := q.ListPODMediaForShipments(ctx, ids)
			if err != nil {
				return err
			}
			for _, m := range media {
				keys = append(keys, m.SignatureKey)
				keys = append(keys, m.PhotoKeys...)
			}
			if err := q.AnonymisePODs(ctx, ids); err != nil {
				return err
			}
			if err := q.AnonymiseShipmentEvents(ctx, ids); err != nil {
				return err
			}
			if err := q.DeleteShipmentTexts(ctx, ids); err != nil {
				return err
			}
			return q.AnonymiseShipments(ctx, ids)
		})
		if err != nil {
			return total, fmt.Errorf("anonymise shipments: %w", err)
		}
		s.media.Delete(ctx, keys...)
		total += len(ids)
		if len(ids) < anonymiseBatch {
			break
		}
	}
	return total, nil
}
