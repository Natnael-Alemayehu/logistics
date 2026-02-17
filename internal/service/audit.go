package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/natnael-alemayehu/logistics/internal/db"
	"github.com/natnael-alemayehu/logistics/internal/model"
)

type AuditService struct {
	queries *db.Queries
}

func NewAuditService(queries *db.Queries) *AuditService {
	return &AuditService{queries: queries}
}

type AuditLogInput struct {
	TenantID   string
	UserID     string
	Action     string
	EntityType string
	EntityID   string
	OldValue   interface{}
	NewValue   interface{}
	IPAddress  string
	UserAgent  string
}

func (s *AuditService) Log(ctx context.Context, input AuditLogInput) error {
	var oldValueBytes, newValueBytes []byte
	var err error

	if input.OldValue != nil {
		oldValueBytes, err = json.Marshal(input.OldValue)
		if err != nil {
			return fmt.Errorf("failed to marshal old value: %w", err)
		}
	}

	if input.NewValue != nil {
		newValueBytes, err = json.Marshal(input.NewValue)
		if err != nil {
			return fmt.Errorf("failed to marshal new value: %w", err)
		}
	}

	_, err = s.queries.CreateAuditLog(ctx, db.CreateAuditLogParams{
		TenantID:   toUUID(input.TenantID),
		UserID:     toUUID(input.UserID),
		Action:     input.Action,
		EntityType: input.EntityType,
		EntityID:   toUUID(input.EntityID),
		OldValue:   oldValueBytes,
		NewValue:   newValueBytes,
		IpAddress:  toText(input.IPAddress),
		UserAgent:  toText(input.UserAgent),
	})
	if err != nil {
		return fmt.Errorf("failed to create audit log: %w", err)
	}

	return nil
}

func (s *AuditService) ListByTenant(ctx context.Context, tenantID string, page, perPage int) ([]model.AuditLog, int, error) {
	offset := (page - 1) * perPage

	logs, err := s.queries.ListAuditLogsByTenant(ctx, db.ListAuditLogsByTenantParams{
		TenantID: toUUID(tenantID),
		Limit:    int32(perPage),
		Offset:   int32(offset),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list audit logs: %w", err)
	}

	result := make([]model.AuditLog, len(logs))
	for i, l := range logs {
		result[i] = *dbAuditLogToModel(&l)
	}

	return result, len(result), nil
}

func (s *AuditService) ListByUser(ctx context.Context, userID string, page, perPage int) ([]model.AuditLog, error) {
	offset := (page - 1) * perPage

	logs, err := s.queries.ListAuditLogsByUser(ctx, db.ListAuditLogsByUserParams{
		UserID: toUUID(userID),
		Limit:  int32(perPage),
		Offset: int32(offset),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list audit logs: %w", err)
	}

	result := make([]model.AuditLog, len(logs))
	for i, l := range logs {
		result[i] = *dbAuditLogToModel(&l)
	}

	return result, nil
}

func (s *AuditService) ListByEntity(ctx context.Context, entityType, entityID string, page, perPage int) ([]model.AuditLog, error) {
	offset := (page - 1) * perPage

	logs, err := s.queries.ListAuditLogsByEntity(ctx, db.ListAuditLogsByEntityParams{
		EntityType: entityType,
		EntityID:   toUUID(entityID),
		Limit:      int32(perPage),
		Offset:     int32(offset),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list audit logs: %w", err)
	}

	result := make([]model.AuditLog, len(logs))
	for i, l := range logs {
		result[i] = *dbAuditLogToModel(&l)
	}

	return result, nil
}

func (s *AuditService) ListByDateRange(ctx context.Context, tenantID string, start, end time.Time, page, perPage int) ([]model.AuditLog, error) {
	offset := (page - 1) * perPage

	logs, err := s.queries.ListAuditLogsByDateRange(ctx, db.ListAuditLogsByDateRangeParams{
		TenantID: toUUID(tenantID),
		CreatedAt: pgtype.Timestamptz{
			Time:  start,
			Valid: true,
		},
		CreatedAt_2: pgtype.Timestamptz{
			Time:  end,
			Valid: true,
		},
		Limit:  int32(perPage),
		Offset: int32(offset),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list audit logs: %w", err)
	}

	result := make([]model.AuditLog, len(logs))
	for i, l := range logs {
		result[i] = *dbAuditLogToModel(&l)
	}

	return result, nil
}

func dbAuditLogToModel(l *db.AuditLog) *model.AuditLog {
	m := &model.AuditLog{
		ID:         l.ID.String(),
		Action:     l.Action,
		EntityType: l.EntityType,
		CreatedAt:  l.CreatedAt.Time,
	}

	if l.TenantID.Valid {
		m.TenantID = l.TenantID.String()
	}
	if l.UserID.Valid {
		m.UserID = l.UserID.String()
	}
	if l.EntityID.Valid {
		m.EntityID = l.EntityID.String()
	}
	if len(l.OldValue) > 0 {
		m.OldValue = string(l.OldValue)
	}
	if len(l.NewValue) > 0 {
		m.NewValue = string(l.NewValue)
	}
	if l.IpAddress != nil {
		m.IPAddress = *l.IpAddress
	}
	if l.UserAgent != nil {
		m.UserAgent = *l.UserAgent
	}

	return m
}
