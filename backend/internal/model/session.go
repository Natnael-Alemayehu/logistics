package model

import (
	"time"
)

type Session struct {
	ID               string     `json:"id"`
	UserID           string     `json:"user_id"`
	TenantID         string     `json:"tenant_id"`
	DeviceID         string     `json:"device_id,omitempty"`
	UserAgent        string     `json:"user_agent,omitempty"`
	IPAddress        string     `json:"ip_address,omitempty"`
	ExpiresAt        time.Time  `json:"expires_at"`
	CreatedAt        time.Time  `json:"created_at"`
	RevokedAt        *time.Time `json:"revoked_at,omitempty"`
	IsCurrentSession bool       `json:"is_current_session,omitempty"`
}

type SessionSummary struct {
	ID        string    `json:"id"`
	DeviceID  string    `json:"device_id,omitempty"`
	UserAgent string    `json:"user_agent,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}
