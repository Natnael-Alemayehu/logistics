package service

import (
	"context"
	"fmt"

	"github.com/natnael-alemayehu/logistics/internal/db"
	"github.com/natnael-alemayehu/logistics/internal/model"
)

type DeviceTokenService struct {
	queries *db.Queries
}

func NewDeviceTokenService(queries *db.Queries) *DeviceTokenService {
	return &DeviceTokenService{queries: queries}
}

type RegisterPushTokenInput struct {
	DeviceID   string `json:"device_id" validate:"required"`
	PushToken  string `json:"push_token" validate:"required"`
	Platform   string `json:"platform" validate:"required,oneof=ios android web"`
	AppVersion string `json:"app_version"`
}

func (s *DeviceTokenService) Register(ctx context.Context, userID string, input RegisterPushTokenInput) (*model.DeviceToken, error) {
	token, err := s.queries.UpsertDeviceToken(ctx, db.UpsertDeviceTokenParams{
		UserID:     toUUID(userID),
		DeviceID:   input.DeviceID,
		PushToken:  input.PushToken,
		Platform:   input.Platform,
		AppVersion: toText(input.AppVersion),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to register device token: %w", err)
	}

	return dbDeviceTokenToModel(&token), nil
}

func (s *DeviceTokenService) Unregister(ctx context.Context, userID, deviceID string) error {
	err := s.queries.DeleteDeviceToken(ctx, db.DeleteDeviceTokenParams{
		UserID:   toUUID(userID),
		DeviceID: deviceID,
	})
	if err != nil {
		return fmt.Errorf("failed to unregister device: %w", err)
	}
	return nil
}

func dbDeviceTokenToModel(dt *db.DeviceToken) *model.DeviceToken {
	m := &model.DeviceToken{
		ID:        dt.ID.String(),
		UserID:    dt.UserID.String(),
		DeviceID:  dt.DeviceID,
		PushToken: dt.PushToken,
		Platform:  dt.Platform,
		CreatedAt: dt.CreatedAt.Time,
		UpdatedAt: dt.UpdatedAt.Time,
	}

	if dt.AppVersion != nil {
		m.AppVersion = *dt.AppVersion
	}

	return m
}
