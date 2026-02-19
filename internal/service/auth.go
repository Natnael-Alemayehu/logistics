package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/natnael-alemayehu/logistics/internal/db"
	"github.com/natnael-alemayehu/logistics/internal/model"
	"github.com/natnael-alemayehu/logistics/pkg/hash"
	"github.com/natnael-alemayehu/logistics/pkg/jwt"
)

const (
	MaxFailedAttempts = 5
	LockoutDuration   = 15 * time.Minute
)

var (
	ErrAccountLocked         = errors.New("account is locked")
	ErrInvalidCredentials    = errors.New("invalid credentials")
	ErrAccountInactive       = errors.New("account is inactive")
	ErrPasswordResetRequired = errors.New("password reset required")
	ErrSessionRevoked        = errors.New("session has been revoked")
	ErrSessionNotFound       = errors.New("session not found")
	ErrUnauthorized          = errors.New("unauthorized")
	ErrNotFound              = errors.New("not found")
	ErrSamePassword          = errors.New("new password must be different from current password")
)

type AuthService struct {
	queries      *db.Queries
	jwtManager   *jwt.JWTManager
	auditService *AuditService
}

func NewAuthService(queries *db.Queries, jwtManager *jwt.JWTManager, auditService *AuditService) *AuthService {
	return &AuthService{
		queries:      queries,
		jwtManager:   jwtManager,
		auditService: auditService,
	}
}

type DriverLoginInput struct {
	Phone string `json:"phone" validate:"required,ethiopian_phone"`
	PIN   string `json:"pin" validate:"required,pin"`
}

type DispatcherLoginInput struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
}

type LoginOutput struct {
	AccessToken  string      `json:"access_token"`
	RefreshToken string      `json:"refresh_token"`
	User         *model.User `json:"user"`
}

type SessionOutput struct {
	ID        string `json:"id"`
	DeviceID  string `json:"device_id,omitempty"`
	UserAgent string `json:"user_agent,omitempty"`
	IPAddress string `json:"ip_address,omitempty"`
	CreatedAt string `json:"created_at"`
	ExpiresAt string `json:"expires_at"`
	IsCurrent bool   `json:"is_current"`
}

func (s *AuthService) DriverLogin(ctx context.Context, input DriverLoginInput, ipAddress, userAgent string) (*LoginOutput, error) {
	phone := input.Phone
	user, err := s.queries.GetUserByPhone(ctx, &phone)
	if err != nil {
		s.auditService.Log(ctx, AuditLogInput{
			Action:     model.ActionLoginFailed,
			EntityType: model.EntityUser,
			IPAddress:  ipAddress,
			UserAgent:  userAgent,
		})
		return nil, ErrInvalidCredentials
	}

	if user.IsActive == nil || !*user.IsActive {
		return nil, ErrAccountInactive
	}

	if user.LockedUntil.Valid && user.LockedUntil.Time.After(time.Now()) {
		s.auditService.Log(ctx, AuditLogInput{
			TenantID:   user.TenantID.String(),
			UserID:     user.ID.String(),
			Action:     model.ActionLoginFailed,
			EntityType: model.EntityUser,
			EntityID:   user.ID.String(),
			IPAddress:  ipAddress,
			UserAgent:  userAgent,
		})
		return nil, ErrAccountLocked
	}

	if user.PinHash == nil || !hash.CheckPIN(input.PIN, *user.PinHash) {
		s.handleFailedLogin(ctx, user.ID, ipAddress, userAgent)
		return nil, ErrInvalidCredentials
	}

	s.queries.ResetFailedLoginAttempts(ctx, user.ID)

	return s.generateTokens(ctx, &user, ipAddress, userAgent)
}

func (s *AuthService) DispatcherLogin(ctx context.Context, input DispatcherLoginInput, ipAddress, userAgent string) (*LoginOutput, error) {
	email := input.Email
	user, err := s.queries.GetUserByEmail(ctx, &email)
	if err != nil {
		s.auditService.Log(ctx, AuditLogInput{
			Action:     model.ActionLoginFailed,
			EntityType: model.EntityUser,
			IPAddress:  ipAddress,
			UserAgent:  userAgent,
		})
		return nil, ErrInvalidCredentials
	}

	if user.IsActive == nil || !*user.IsActive {
		return nil, ErrAccountInactive
	}

	if user.LockedUntil.Valid && user.LockedUntil.Time.After(time.Now()) {
		s.auditService.Log(ctx, AuditLogInput{
			TenantID:   user.TenantID.String(),
			UserID:     user.ID.String(),
			Action:     model.ActionLoginFailed,
			EntityType: model.EntityUser,
			EntityID:   user.ID.String(),
			IPAddress:  ipAddress,
			UserAgent:  userAgent,
		})
		return nil, ErrAccountLocked
	}

	if user.Role == "driver" {
		return nil, errors.New("invalid login type for this endpoint")
	}

	if user.PasswordHash == nil || !hash.CheckPassword(input.Password, *user.PasswordHash) {
		s.handleFailedLogin(ctx, user.ID, ipAddress, userAgent)
		return nil, ErrInvalidCredentials
	}

	if user.PasswordResetRequired != nil && *user.PasswordResetRequired {
		return nil, ErrPasswordResetRequired
	}

	s.queries.ResetFailedLoginAttempts(ctx, user.ID)

	return s.generateTokens(ctx, &user, ipAddress, userAgent)
}

func (s *AuthService) handleFailedLogin(ctx context.Context, userID pgtype.UUID, ipAddress, userAgent string) {
	user, _ := s.queries.IncrementFailedLoginAttempts(ctx, userID)

	if user.FailedLoginAttempts != nil && *user.FailedLoginAttempts >= MaxFailedAttempts {
		lockUntil := time.Now().Add(LockoutDuration)
		s.queries.LockUserAccount(ctx, db.LockUserAccountParams{
			ID:          userID,
			LockedUntil: pgtype.Timestamptz{Time: lockUntil, Valid: true},
		})
	}

	s.auditService.Log(ctx, AuditLogInput{
		TenantID:   user.TenantID.String(),
		UserID:     user.ID.String(),
		Action:     model.ActionLoginFailed,
		EntityType: model.EntityUser,
		EntityID:   user.ID.String(),
		IPAddress:  ipAddress,
		UserAgent:  userAgent,
	})
}

func (s *AuthService) RefreshToken(ctx context.Context, refreshToken string) (*LoginOutput, error) {
	// 1. Validate JWT signature and expiration
	claims, err := s.jwtManager.Validate(refreshToken)
	if err != nil {
		return nil, errors.New("invalid refresh token")
	}

	// 2. Check if session exists and is active
	tokenHash := sha256.Sum256([]byte(refreshToken))
	tokenHashStr := hex.EncodeToString(tokenHash[:])

	session, err := s.queries.GetActiveSessionByTokenHash(ctx, tokenHashStr)
	if err != nil {
		return nil, ErrSessionRevoked
	}

	// 3. Verify session belongs to the user in the token
	if session.UserID.String() != claims.UserID {
		return nil, errors.New("token session mismatch")
	}

	// 4. Get user and verify account status
	user, err := s.queries.GetUserByID(ctx, db.GetUserByIDParams{
		ID:       toUUID(claims.UserID),
		TenantID: toUUID(claims.TenantID),
	})
	if err != nil {
		return nil, errors.New("user not found")
	}

	if user.IsActive == nil || !*user.IsActive {
		return nil, ErrAccountInactive
	}

	if user.LockedUntil.Valid && user.LockedUntil.Time.After(time.Now()) {
		return nil, ErrAccountLocked
	}

	// 5. Revoke the old session
	s.queries.RevokeSession(ctx, session.ID)

	// 6. Generate new tokens (creates new session)
	return s.generateTokens(ctx, &user, "", "")
}

func (s *AuthService) Logout(ctx context.Context, userID, sessionID string) error {
	return s.queries.RevokeSessionByUser(ctx, db.RevokeSessionByUserParams{
		ID:     toUUID(sessionID),
		UserID: toUUID(userID),
	})
}

func (s *AuthService) ListSessions(ctx context.Context, userID string) ([]model.Session, error) {
	sessions, err := s.queries.ListActiveSessionsByUser(ctx, toUUID(userID))
	if err != nil {
		return nil, err
	}

	result := make([]model.Session, len(sessions))
	for i, sess := range sessions {
		result[i] = *dbSessionToModel(&sess)
	}

	return result, nil
}

func (s *AuthService) RevokeSession(ctx context.Context, userID, sessionID string) error {
	return s.queries.RevokeSessionByUser(ctx, db.RevokeSessionByUserParams{
		ID:     toUUID(sessionID),
		UserID: toUUID(userID),
	})
}

func (s *AuthService) RevokeOtherSessions(ctx context.Context, userID, currentSessionID string) error {
	return s.queries.RevokeOtherUserSessions(ctx, db.RevokeOtherUserSessionsParams{
		UserID: toUUID(userID),
		ID:     toUUID(currentSessionID),
	})
}

func (s *AuthService) UnlockAccount(ctx context.Context, userID string) error {
	return s.queries.UnlockUserAccount(ctx, toUUID(userID))
}

func (s *AuthService) UpdatePassword(ctx context.Context, userID, newPassword string) (*model.User, error) {
	passwordHash, err := hash.Password(newPassword)
	if err != nil {
		return nil, err
	}

	user, err := s.queries.UpdatePassword(ctx, db.UpdatePasswordParams{
		ID:           toUUID(userID),
		PasswordHash: &passwordHash,
	})
	if err != nil {
		return nil, err
	}

	return dbUserToModel(&user), nil
}

func (s *AuthService) UpdatePIN(ctx context.Context, userID, newPIN string) (*model.User, error) {
	pinHash, err := hash.PIN(newPIN)
	if err != nil {
		return nil, err
	}

	user, err := s.queries.UpdatePIN(ctx, db.UpdatePINParams{
		ID:      toUUID(userID),
		PinHash: &pinHash,
	})
	if err != nil {
		return nil, err
	}

	return dbUserToModel(&user), nil
}

func (s *AuthService) ChangePassword(ctx context.Context, tenantID, userID, userRole, currentPassword, newPassword, ipAddress, userAgent string) error {
	user, err := s.queries.GetUserByID(ctx, db.GetUserByIDParams{
		ID:       toUUID(userID),
		TenantID: toUUID(tenantID),
	})
	if err != nil {
		return ErrNotFound
	}

	if user.PasswordHash == nil || !hash.CheckPassword(currentPassword, *user.PasswordHash) {
		return ErrInvalidCredentials
	}

	if hash.CheckPassword(newPassword, *user.PasswordHash) {
		return ErrSamePassword
	}

	passwordHash, err := hash.Password(newPassword)
	if err != nil {
		return err
	}

	_, err = s.queries.UpdatePassword(ctx, db.UpdatePasswordParams{
		ID:           toUUID(userID),
		PasswordHash: &passwordHash,
	})
	if err != nil {
		return err
	}

	s.auditService.Log(ctx, AuditLogInput{
		TenantID:   tenantID,
		UserID:     userID,
		Action:     "PASSWORD_CHANGED",
		EntityType: "user",
		EntityID:   userID,
		IPAddress:  ipAddress,
		UserAgent:  userAgent,
	})

	return nil
}

func (s *AuthService) ForgotPIN(ctx context.Context, tenantID, requesterID, requesterRole, driverID, ipAddress, userAgent string) error {
	if requesterRole != "admin" && requesterRole != "fleet_manager" && requesterID != driverID {
		return ErrUnauthorized
	}

	user, err := s.queries.GetUserByID(ctx, db.GetUserByIDParams{
		ID:       toUUID(driverID),
		TenantID: toUUID(tenantID),
	})
	if err != nil {
		return ErrNotFound
	}

	if user.Role != "driver" {
		return ErrNotFound
	}

	newPIN := generateRandomPIN()
	pinHash, err := hash.PIN(newPIN)
	if err != nil {
		return err
	}

	_, err = s.queries.UpdatePIN(ctx, db.UpdatePINParams{
		ID:      toUUID(driverID),
		PinHash: &pinHash,
	})
	if err != nil {
		return err
	}

	if user.Phone != nil {
		// In production, send SMS with new PIN
		// For now, we just log the audit
		fmt.Println("In production, send SMS with new PIN, For now, we just log the audit")
	}

	s.auditService.Log(ctx, AuditLogInput{
		TenantID:   tenantID,
		UserID:     requesterID,
		Action:     "PIN_RESET",
		EntityType: "user",
		EntityID:   driverID,
		IPAddress:  ipAddress,
		UserAgent:  userAgent,
	})

	return nil
}

func generateRandomPIN() string {
	return fmt.Sprintf("%06d", time.Now().UnixNano()%1000000)
}

func (s *AuthService) generateTokens(ctx context.Context, user *db.User, ipAddress, userAgent string) (*LoginOutput, error) {
	var phone, email string
	if user.Phone != nil {
		phone = *user.Phone
	}
	if user.Email != nil {
		email = *user.Email
	}

	refreshToken, err := s.jwtManager.GenerateRefreshToken(
		user.ID.String(),
		user.TenantID.String(),
	)
	if err != nil {
		return nil, err
	}

	tokenHash := sha256.Sum256([]byte(refreshToken))
	tokenHashStr := hex.EncodeToString(tokenHash[:])

	session, err := s.queries.CreateSession(ctx, db.CreateSessionParams{
		UserID:           user.ID,
		TenantID:         user.TenantID,
		RefreshTokenHash: tokenHashStr,
		ExpiresAt:        pgtype.Timestamptz{Time: time.Now().Add(s.jwtManager.RefreshTTL()), Valid: true},
		IpAddress:        toText(ipAddress),
		UserAgent:        toText(userAgent),
	})
	if err != nil {
		return nil, err
	}

	accessToken, err := s.jwtManager.GenerateAccessToken(
		user.ID.String(),
		user.TenantID.String(),
		session.ID.String(),
		user.Role,
		phone,
		email,
	)
	if err != nil {
		return nil, err
	}

	s.auditService.Log(ctx, AuditLogInput{
		TenantID:   user.TenantID.String(),
		UserID:     user.ID.String(),
		Action:     model.ActionLogin,
		EntityType: model.EntitySession,
		EntityID:   session.ID.String(),
		IPAddress:  ipAddress,
		UserAgent:  userAgent,
	})

	return &LoginOutput{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User:         dbUserToModel(user),
	}, nil
}

func dbUserToModel(u *db.User) *model.User {
	m := &model.User{
		ID:        u.ID.String(),
		TenantID:  u.TenantID.String(),
		Role:      u.Role,
		FullName:  u.FullName,
		IsActive:  *u.IsActive,
		CreatedAt: u.CreatedAt.Time,
		UpdatedAt: u.UpdatedAt.Time,
	}

	if u.Phone != nil {
		m.Phone = *u.Phone
	}
	if u.Email != nil {
		m.Email = *u.Email
	}

	return m
}

func dbSessionToModel(s *db.Session) *model.Session {
	m := &model.Session{
		ID:        s.ID.String(),
		UserID:    s.UserID.String(),
		TenantID:  s.TenantID.String(),
		ExpiresAt: s.ExpiresAt.Time,
		CreatedAt: s.CreatedAt.Time,
	}

	if s.DeviceID != nil {
		m.DeviceID = *s.DeviceID
	}
	if s.UserAgent != nil {
		m.UserAgent = *s.UserAgent
	}
	if s.IpAddress != nil {
		m.IPAddress = *s.IpAddress
	}
	if s.RevokedAt.Valid {
		m.RevokedAt = &s.RevokedAt.Time
	}

	return m
}
