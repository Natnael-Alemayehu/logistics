package service

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/natnael-alemayehu/logistics/internal/db"
)

// AuditService records security-relevant actions: sign-ins, account and
// credential changes, settings. What happens to shipments is recorded in
// their own timeline (shipment_events) instead.
type AuditService struct{ *core }

// write records an action within the caller's transaction. A failure is
// logged rather than returned: losing an audit row must not undo the action.
func (c *core) audit(ctx context.Context, q *db.Queries, a Actor, action, entityType string, entityID pgtype.UUID, oldValue, newValue any) {
	_, err := q.CreateAuditLog(ctx, db.CreateAuditLogParams{
		TenantID:   a.TenantID,
		UserID:     a.UserID,
		Action:     action,
		EntityType: entityType,
		EntityID:   entityID,
		OldValue:   mustJSON(oldValue),
		NewValue:   mustJSON(newValue),
		IpAddress:  toText(a.IP),
		UserAgent:  toText(a.UserAgent),
	})
	if err != nil {
		c.log.Error().Err(err).Str("action", action).Msg("write audit log")
	}
}

// List returns a page of a tenant's audit log, newest first.
func (s *AuditService) List(ctx context.Context, tenantID pgtype.UUID, page, perPage int) ([]db.AuditLog, error) {
	limit, offset := pageBounds(page, perPage)
	return s.q.ListAuditLogsByTenant(ctx, db.ListAuditLogsByTenantParams{TenantID: tenantID, Limit: limit, Offset: offset})
}

// pageBounds converts a 1-based page into LIMIT/OFFSET, bounding the page size.
func pageBounds(page, perPage int) (limit, offset int32) {
	if perPage <= 0 {
		perPage = 25
	}
	if perPage > 200 {
		perPage = 200
	}
	if page < 1 {
		page = 1
	}
	return int32(perPage), int32((page - 1) * perPage)
}
