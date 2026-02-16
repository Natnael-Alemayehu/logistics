package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/natnael-alemayehu/logistics/internal/db"
	"github.com/natnael-alemayehu/logistics/internal/model"
	"github.com/natnael-alemayehu/logistics/pkg/hash"
	"github.com/natnael-alemayehu/logistics/pkg/jwt"
)

type AuthService struct {
	queries    *db.Queries
	jwtManager *jwt.JWTManager
}

func NewAuthService(queries *db.Queries, jwtManager *jwt.JWTManager) *AuthService {
	return &AuthService{
		queries:    queries,
		jwtManager: jwtManager,
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

func (s *AuthService) DriverLogin(ctx context.Context, input DriverLoginInput) (*LoginOutput, error) {
	phone := input.Phone
	user, err := s.queries.GetUserByPhone(ctx, &phone)
	if err != nil {
		return nil, errors.New("invalid phone or PIN")
	}

	if user.IsActive == nil || !*user.IsActive {
		return nil, errors.New("account is inactive")
	}

	if user.Role != "driver" {
		return nil, errors.New("invalid login type for this endpoint")
	}

	if user.PinHash == nil || !hash.CheckPIN(input.PIN, *user.PinHash) {
		return nil, errors.New("invalid phone or PIN")
	}

	return s.generateTokens(ctx, &user)
}

func (s *AuthService) DispatcherLogin(ctx context.Context, input DispatcherLoginInput) (*LoginOutput, error) {
	email := input.Email
	user, err := s.queries.GetUserByEmail(ctx, &email)
	if err != nil {
		return nil, errors.New("invalid email or password")
	}

	if user.IsActive == nil || !*user.IsActive {
		return nil, errors.New("account is inactive")
	}

	if user.Role == "driver" {
		return nil, errors.New("invalid login type for this endpoint")
	}

	if user.PasswordHash == nil || !hash.CheckPassword(input.Password, *user.PasswordHash) {
		return nil, errors.New("invalid email or password")
	}

	return s.generateTokens(ctx, &user)
}

func (s *AuthService) RefreshToken(ctx context.Context, refreshToken string) (*LoginOutput, error) {
	claims, err := s.jwtManager.Validate(refreshToken)
	if err != nil {
		return nil, errors.New("invalid refresh token")
	}

	user, err := s.queries.GetUserByID(ctx, db.GetUserByIDParams{
		ID:       toUUID(claims.UserID),
		TenantID: toUUID(claims.TenantID),
	})
	if err != nil {
		return nil, errors.New("user not found")
	}

	if user.IsActive == nil || !*user.IsActive {
		return nil, errors.New("account is inactive")
	}

	return s.generateTokens(ctx, &user)
}

func (s *AuthService) Logout(ctx context.Context, refreshToken string) error {
	tokenHash := sha256.Sum256([]byte(refreshToken))
	tokenHashStr := hex.EncodeToString(tokenHash[:])

	session, err := s.queries.GetActiveSessionByTokenHash(ctx, tokenHashStr)
	if err != nil {
		return nil
	}

	return s.queries.RevokeSession(ctx, session.ID)
}

func (s *AuthService) generateTokens(ctx context.Context, user *db.User) (*LoginOutput, error) {
	var phone, email string
	if user.Phone != nil {
		phone = *user.Phone
	}
	if user.Email != nil {
		email = *user.Email
	}

	accessToken, err := s.jwtManager.GenerateAccessToken(
		user.ID.String(),
		user.TenantID.String(),
		user.Role,
		phone,
		email,
	)
	if err != nil {
		return nil, err
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

	_, err = s.queries.CreateSession(ctx, db.CreateSessionParams{
		UserID:           user.ID,
		TenantID:         user.TenantID,
		RefreshTokenHash: tokenHashStr,
		ExpiresAt:        pgtype.Timestamptz{Time: time.Now().Add(s.jwtManager.RefreshTTL()), Valid: true},
	})
	if err != nil {
		return nil, err
	}

	return &LoginOutput{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User: &model.User{
			ID:       user.ID.String(),
			TenantID: user.TenantID.String(),
			Role:     user.Role,
			FullName: user.FullName,
			Phone:    phone,
			Email:    email,
			IsActive: *user.IsActive,
		},
	}, nil
}
