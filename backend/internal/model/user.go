package model

import (
	"time"
)

type User struct {
	ID        string    `json:"id"`
	TenantID  string    `json:"tenant_id"`
	Role      string    `json:"role"`
	FullName  string    `json:"full_name"`
	Phone     string    `json:"phone,omitempty"`
	Email     string    `json:"email,omitempty"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type UserResponse struct {
	ID       string `json:"id"`
	Role     string `json:"role"`
	FullName string `json:"full_name"`
	Phone    string `json:"phone,omitempty"`
	Email    string `json:"email,omitempty"`
}
