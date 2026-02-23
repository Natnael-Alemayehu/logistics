package service

import (
	"context"

	"github.com/natnael-alemayehu/logistics/internal/db"
	"github.com/natnael-alemayehu/logistics/internal/model"
	"github.com/natnael-alemayehu/logistics/pkg/hash"
	"github.com/natnael-alemayehu/logistics/pkg/validation"
)

type UserService struct {
	queries      *db.Queries
	auditService *AuditService
}

func NewUserService(queries *db.Queries, auditService *AuditService) *UserService {
	return &UserService{queries: queries, auditService: auditService}
}

type CreateDriverInput struct {
	FullName string `json:"full_name" validate:"required"`
	Phone    string `json:"phone" validate:"required,ethiopian_phone"`
	PIN      string `json:"pin" validate:"required,pin"`
}

type CreateUserInput struct {
	FullName string `json:"full_name" validate:"required"`
	Phone    string `json:"phone" validate:"omitempty,ethiopian_phone"`
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
	Role     string `json:"role" validate:"required,oneof=dispatcher fleet_manager admin"`
}

func (s *UserService) CreateDriver(ctx context.Context, tenantID string, input CreateDriverInput) (*model.User, error) {
	pinHash, err := hash.PIN(input.PIN)
	if err != nil {
		return nil, err
	}

	isActive := true
	phone := validation.NormalizeEthiopianPhone(input.Phone)
	user, err := s.queries.CreateUser(ctx, db.CreateUserParams{
		TenantID: toUUID(tenantID),
		Role:     "driver",
		FullName: input.FullName,
		Phone:    &phone,
		PinHash:  &pinHash,
		IsActive: &isActive,
	})
	if err != nil {
		return nil, err
	}

	return dbUserToModel(&user), nil
}

func (s *UserService) CreateUser(ctx context.Context, tenantID string, input CreateUserInput) (*model.User, error) {
	passwordHash, err := hash.Password(input.Password)
	if err != nil {
		return nil, err
	}

	isActive := true
	var phonePtr *string
	if input.Phone != "" {
		phone := validation.NormalizeEthiopianPhone(input.Phone)
		phonePtr = &phone
	}
	user, err := s.queries.CreateUser(ctx, db.CreateUserParams{
		TenantID:     toUUID(tenantID),
		Role:         input.Role,
		FullName:     input.FullName,
		Phone:        phonePtr,
		Email:        &input.Email,
		PasswordHash: &passwordHash,
		IsActive:     &isActive,
	})
	if err != nil {
		return nil, err
	}

	return dbUserToModel(&user), nil
}

func (s *UserService) ListDrivers(ctx context.Context, tenantID string, page, perPage int) ([]model.User, int, error) {
	offset := (page - 1) * perPage

	users, err := s.queries.ListDriversByTenant(ctx, db.ListDriversByTenantParams{
		TenantID: toUUID(tenantID),
		Limit:    int32(perPage),
		Offset:   int32(offset),
	})
	if err != nil {
		return nil, 0, err
	}

	total, err := s.queries.CountDriversByTenant(ctx, toUUID(tenantID))
	if err != nil {
		return nil, 0, err
	}

	result := make([]model.User, len(users))
	for i, u := range users {
		result[i] = *dbUserToModel(&u)
	}

	return result, int(total), nil
}

func (s *UserService) ListUsers(ctx context.Context, tenantID string, page, perPage int) ([]model.User, int, error) {
	offset := (page - 1) * perPage

	users, err := s.queries.ListUsersByTenant(ctx, db.ListUsersByTenantParams{
		TenantID: toUUID(tenantID),
		Limit:    int32(perPage),
		Offset:   int32(offset),
	})
	if err != nil {
		return nil, 0, err
	}

	total, err := s.queries.CountUsersByTenant(ctx, toUUID(tenantID))
	if err != nil {
		return nil, 0, err
	}

	result := make([]model.User, len(users))
	for i, u := range users {
		result[i] = *dbUserToModel(&u)
	}

	return result, int(total), nil
}

func (s *UserService) UpdateUser(ctx context.Context, tenantID, requesterID, requesterRole, targetUserID, fullName, phone, email, ipAddress, userAgent string) (*model.User, error) {
	if requesterRole != "admin" && requesterRole != "fleet_manager" && requesterID != targetUserID {
		return nil, ErrUnauthorized
	}

	user, err := s.queries.GetUserByID(ctx, db.GetUserByIDParams{
		ID:       toUUID(targetUserID),
		TenantID: toUUID(tenantID),
	})
	if err != nil {
		return nil, ErrNotFound
	}

	oldUser := user

	params := db.UpdateUserParams{
		ID:       toUUID(targetUserID),
		TenantID: toUUID(tenantID),
	}

	if fullName != "" {
		params.FullName = &fullName
	}
	if phone != "" {
		normalizedPhone := validation.NormalizeEthiopianPhone(phone)
		params.Phone = &normalizedPhone
	}
	if email != "" {
		params.Email = &email
	}

	updatedUser, err := s.queries.UpdateUser(ctx, params)
	if err != nil {
		return nil, err
	}

	s.auditService.Log(ctx, AuditLogInput{
		TenantID:   tenantID,
		UserID:     requesterID,
		Action:     model.ActionUserUpdated,
		EntityType: model.EntityUser,
		EntityID:   targetUserID,
		OldValue:   oldUser,
		NewValue:   updatedUser,
		IPAddress:  ipAddress,
		UserAgent:  userAgent,
	})

	return dbUserToModel(&updatedUser), nil
}
