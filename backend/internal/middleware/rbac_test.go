package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/natnael-alemayehu/logistics/pkg/response"
)

func TestRequireRole_AllowedRole(t *testing.T) {
	tests := []struct {
		name         string
		role         string
		allowedRoles []string
	}{
		{"driver allowed", "driver", []string{"driver"}},
		{"admin allowed", "admin", []string{"admin", "superadmin"}},
		{"one of multiple allowed", "editor", []string{"viewer", "editor", "admin"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			handler := RequireRole(tt.allowedRoles...)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest("GET", "/protected", nil)
			ctx := context.WithValue(req.Context(), RoleKey, tt.role)
			req = req.WithContext(ctx)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if !called {
				t.Error("next handler was not called")
			}
			if rec.Code != http.StatusOK {
				t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
			}
		})
	}
}

func TestRequireRole_DisallowedRole(t *testing.T) {
	tests := []struct {
		name         string
		role         string
		allowedRoles []string
	}{
		{"driver not allowed for admin only", "driver", []string{"admin"}},
		{"viewer not allowed", "viewer", []string{"editor", "admin"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := RequireRole(tt.allowedRoles...)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				t.Error("next handler should not be called")
			}))

			req := httptest.NewRequest("GET", "/protected", nil)
			if tt.role != "" {
				ctx := context.WithValue(req.Context(), RoleKey, tt.role)
				req = req.WithContext(ctx)
			}
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusForbidden {
				t.Errorf("expected status %d, got %d", http.StatusForbidden, rec.Code)
			}

			var resp response.Response
			if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}
			if resp.Error.Code != "FORBIDDEN" {
				t.Errorf("expected error code FORBIDDEN, got %s", resp.Error.Code)
			}
		})
	}
}

func TestRequireRole_NoRoleInContext(t *testing.T) {
	handler := RequireRole("admin")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("next handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/protected", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}

	var resp response.Response
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Error.Code != "UNAUTHORIZED" {
		t.Errorf("expected error code UNAUTHORIZED, got %s", resp.Error.Code)
	}
	if resp.Error.Message != "No role found in context" {
		t.Errorf("expected message 'No role found in context', got %s", resp.Error.Message)
	}
}

func TestRequireDriver(t *testing.T) {
	t.Run("driver role allowed", func(t *testing.T) {
		called := false
		handler := RequireDriver()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/protected", nil)
		ctx := context.WithValue(req.Context(), RoleKey, "driver")
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if !called {
			t.Error("next handler was not called")
		}
		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
		}
	})

	t.Run("admin role denied", func(t *testing.T) {
		handler := RequireDriver()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Error("next handler should not be called")
		}))

		req := httptest.NewRequest("GET", "/protected", nil)
		ctx := context.WithValue(req.Context(), RoleKey, "admin")
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Errorf("expected status %d, got %d", http.StatusForbidden, rec.Code)
		}
	})
}

func TestRequireDispatcher(t *testing.T) {
	tests := []struct {
		name       string
		role       string
		shouldPass bool
	}{
		{"dispatcher allowed", "dispatcher", true},
		{"fleet_manager allowed", "fleet_manager", true},
		{"admin allowed", "admin", true},
		{"driver denied", "driver", false},
		{"platform_admin denied", "platform_admin", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			handler := RequireDispatcher()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest("GET", "/protected", nil)
			ctx := context.WithValue(req.Context(), RoleKey, tt.role)
			req = req.WithContext(ctx)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if tt.shouldPass {
				if !called {
					t.Error("next handler was not called")
				}
				if rec.Code != http.StatusOK {
					t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
				}
			} else {
				if called {
					t.Error("next handler should not be called")
				}
				if rec.Code != http.StatusForbidden {
					t.Errorf("expected status %d, got %d", http.StatusForbidden, rec.Code)
				}
			}
		})
	}
}

func TestRequireAdmin(t *testing.T) {
	tests := []struct {
		name       string
		role       string
		shouldPass bool
	}{
		{"admin allowed", "admin", true},
		{"platform_admin allowed", "platform_admin", true},
		{"driver denied", "driver", false},
		{"dispatcher denied", "dispatcher", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			handler := RequireAdmin()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest("GET", "/protected", nil)
			ctx := context.WithValue(req.Context(), RoleKey, tt.role)
			req = req.WithContext(ctx)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if tt.shouldPass {
				if !called {
					t.Error("next handler was not called")
				}
				if rec.Code != http.StatusOK {
					t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
				}
			} else {
				if called {
					t.Error("next handler should not be called")
				}
				if rec.Code != http.StatusForbidden {
					t.Errorf("expected status %d, got %d", http.StatusForbidden, rec.Code)
				}
			}
		})
	}
}
