package middleware

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/natnael-alemayehu/logistics/internal/db"
	"github.com/natnael-alemayehu/logistics/pkg/jwt"
	"github.com/natnael-alemayehu/logistics/pkg/response"
)

type mockDBTX struct {
	queryRowFunc func(ctx context.Context, sql string, args ...interface{}) pgx.Row
}

func (m *mockDBTX) Exec(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (m *mockDBTX) Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	return nil, nil
}

func (m *mockDBTX) QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row {
	if m.queryRowFunc != nil {
		return m.queryRowFunc(ctx, sql, args...)
	}
	return nil
}

type mockRow struct {
	scanFunc func(dest ...interface{}) error
}

func (m *mockRow) Scan(dest ...interface{}) error {
	if m.scanFunc != nil {
		return m.scanFunc(dest...)
	}
	return nil
}

func generateTestJWTManager(t *testing.T) *jwt.JWTManager {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate private key: %v", err)
	}
	return jwt.NewManager(privateKey, &privateKey.PublicKey, "test-issuer", time.Hour, 24*time.Hour)
}

func TestAuth_MissingAuthorizationHeader(t *testing.T) {
	jwtManager := generateTestJWTManager(t)
	handler := Auth(jwtManager)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	if resp.Error.Message != "Missing authorization header" {
		t.Errorf("expected message 'Missing authorization header', got %s", resp.Error.Message)
	}
}

func TestAuth_InvalidAuthorizationHeaderFormat(t *testing.T) {
	tests := []struct {
		name   string
		header string
	}{
		{"no bearer prefix", "token123"},
		{"wrong prefix", "Basic token123"},
		{"empty token", "Bearer "},
		{"multiple spaces", "Bearer  token123 extra"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jwtManager := generateTestJWTManager(t)
			handler := Auth(jwtManager)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				t.Error("next handler should not be called")
			}))

			req := httptest.NewRequest("GET", "/protected", nil)
			req.Header.Set("Authorization", tt.header)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
			}
		})
	}
}

func TestAuth_BearerCaseInsensitive(t *testing.T) {
	jwtManager := generateTestJWTManager(t)
	token, err := jwtManager.GenerateAccessToken("user-123", "tenant-456", "session-789", "driver", "", "")
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	var capturedCtx context.Context
	handler := Auth(jwtManager)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedCtx = r.Context()
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if GetUserID(capturedCtx) != "user-123" {
		t.Errorf("expected userID user-123, got %s", GetUserID(capturedCtx))
	}
}

func TestAuth_ValidToken(t *testing.T) {
	jwtManager := generateTestJWTManager(t)
	token, err := jwtManager.GenerateAccessToken("user-123", "tenant-456", "session-789", "driver", "", "")
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	var capturedCtx context.Context
	handler := Auth(jwtManager)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedCtx = r.Context()
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	if capturedCtx == nil {
		t.Fatal("context was not captured")
	}

	if GetUserID(capturedCtx) != "user-123" {
		t.Errorf("expected userID user-123, got %s", GetUserID(capturedCtx))
	}
	if GetTenantID(capturedCtx) != "tenant-456" {
		t.Errorf("expected tenantID tenant-456, got %s", GetTenantID(capturedCtx))
	}
	if GetSessionID(capturedCtx) != "session-789" {
		t.Errorf("expected sessionID session-789, got %s", GetSessionID(capturedCtx))
	}
	if GetRole(capturedCtx) != "driver" {
		t.Errorf("expected role driver, got %s", GetRole(capturedCtx))
	}

	claims := GetClaims(capturedCtx)
	if claims == nil {
		t.Error("expected claims, got nil")
	} else if claims.UserID != "user-123" {
		t.Errorf("expected claims.UserID user-123, got %s", claims.UserID)
	}
}

func TestAuth_InvalidToken(t *testing.T) {
	jwtManager := generateTestJWTManager(t)

	handler := Auth(jwtManager)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("next handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}

	var resp response.Response
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Error.Message != "Invalid or expired token" {
		t.Errorf("expected message 'Invalid or expired token', got %s", resp.Error.Message)
	}
}

func TestAuth_ExpiredToken(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate private key: %v", err)
	}

	expiredJWTManager := jwt.NewManager(privateKey, &privateKey.PublicKey, "test-issuer", -time.Hour, 24*time.Hour)
	validJWTManager := jwt.NewManager(privateKey, &privateKey.PublicKey, "test-issuer", time.Hour, 24*time.Hour)

	token, err := expiredJWTManager.GenerateAccessToken("user-123", "tenant-456", "session-789", "driver", "", "")
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	handler := Auth(validJWTManager)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("next handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}

func TestGetUserID(t *testing.T) {
	t.Run("value exists", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), UserIDKey, "user-123")
		if got := GetUserID(ctx); got != "user-123" {
			t.Errorf("expected user-123, got %s", got)
		}
	})

	t.Run("value not exists", func(t *testing.T) {
		if got := GetUserID(context.Background()); got != "" {
			t.Errorf("expected empty string, got %s", got)
		}
	})

	t.Run("wrong type", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), UserIDKey, 123)
		if got := GetUserID(ctx); got != "" {
			t.Errorf("expected empty string for wrong type, got %s", got)
		}
	})
}

func TestGetTenantID(t *testing.T) {
	t.Run("value exists", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), TenantIDKey, "tenant-456")
		if got := GetTenantID(ctx); got != "tenant-456" {
			t.Errorf("expected tenant-456, got %s", got)
		}
	})

	t.Run("value not exists", func(t *testing.T) {
		if got := GetTenantID(context.Background()); got != "" {
			t.Errorf("expected empty string, got %s", got)
		}
	})
}

func TestGetSessionID(t *testing.T) {
	t.Run("value exists", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), SessionIDKey, "session-789")
		if got := GetSessionID(ctx); got != "session-789" {
			t.Errorf("expected session-789, got %s", got)
		}
	})

	t.Run("value not exists", func(t *testing.T) {
		if got := GetSessionID(context.Background()); got != "" {
			t.Errorf("expected empty string, got %s", got)
		}
	})
}

func TestGetRole(t *testing.T) {
	t.Run("value exists", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), RoleKey, "admin")
		if got := GetRole(ctx); got != "admin" {
			t.Errorf("expected admin, got %s", got)
		}
	})

	t.Run("value not exists", func(t *testing.T) {
		if got := GetRole(context.Background()); got != "" {
			t.Errorf("expected empty string, got %s", got)
		}
	})
}

func TestGetClaims(t *testing.T) {
	testClaims := &jwt.Claims{
		UserID:   "user-123",
		TenantID: "tenant-456",
	}

	t.Run("value exists", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), ClaimsKey, testClaims)
		got := GetClaims(ctx)
		if got == nil {
			t.Error("expected claims, got nil")
		} else if got.UserID != testClaims.UserID {
			t.Errorf("expected UserID %s, got %s", testClaims.UserID, got.UserID)
		}
	})

	t.Run("value not exists", func(t *testing.T) {
		if got := GetClaims(context.Background()); got != nil {
			t.Errorf("expected nil, got %+v", got)
		}
	})

	t.Run("wrong type", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), ClaimsKey, "not claims")
		if got := GetClaims(ctx); got != nil {
			t.Errorf("expected nil for wrong type, got %+v", got)
		}
	})
}

func TestValidateSession_NoSessionInContext(t *testing.T) {
	mockDB := &mockDBTX{}
	queries := db.New(mockDB)

	handler := ValidateSession(queries)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	if resp.Error.Message != "Session not found in token" {
		t.Errorf("expected message 'Session not found in token', got %s", resp.Error.Message)
	}
}

func TestValidateSession_SessionNotFound(t *testing.T) {
	mockDB := &mockDBTX{
		queryRowFunc: func(ctx context.Context, sql string, args ...interface{}) pgx.Row {
			return &mockRow{
				scanFunc: func(dest ...interface{}) error {
					return errors.New("session not found")
				},
			}
		},
	}
	queries := db.New(mockDB)

	handler := ValidateSession(queries)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("next handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/protected", nil)
	ctx := context.WithValue(req.Context(), SessionIDKey, "550e8400-e29b-41d4-a716-446655440000")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}

	var resp response.Response
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Error.Code != "SESSION_REVOKED" {
		t.Errorf("expected error code SESSION_REVOKED, got %s", resp.Error.Code)
	}
}

func TestValidateSession_RevokedSession(t *testing.T) {
	mockDB := &mockDBTX{
		queryRowFunc: func(ctx context.Context, sql string, args ...interface{}) pgx.Row {
			return &mockRow{
				scanFunc: func(dest ...interface{}) error {
					for i := range dest {
						switch d := dest[i].(type) {
						case *pgtype.UUID:
							*d = pgtype.UUID{Valid: true}
						case *string:
							*d = "test"
						case **string:
							*d = nil
						case *pgtype.Timestamptz:
							if i == 9 {
								*d = pgtype.Timestamptz{Valid: true}
							} else {
								*d = pgtype.Timestamptz{Valid: true}
							}
						}
					}
					return nil
				},
			}
		},
	}
	queries := db.New(mockDB)

	handler := ValidateSession(queries)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("next handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/protected", nil)
	ctx := context.WithValue(req.Context(), SessionIDKey, "550e8400-e29b-41d4-a716-446655440000")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}

	var resp response.Response
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Error.Message != "Session has been revoked" {
		t.Errorf("expected message 'Session has been revoked', got %s", resp.Error.Message)
	}
}

func TestValidateSession_ValidSession(t *testing.T) {
	mockDB := &mockDBTX{
		queryRowFunc: func(ctx context.Context, sql string, args ...interface{}) pgx.Row {
			return &mockRow{
				scanFunc: func(dest ...interface{}) error {
					if len(dest) >= 10 {
						dest[0] = pgtype.UUID{Valid: true}
						dest[1] = pgtype.UUID{Valid: true}
						dest[2] = pgtype.UUID{Valid: true}
						dest[3] = "hash"
						dest[4] = (*string)(nil)
						dest[5] = (*string)(nil)
						dest[6] = (*string)(nil)
						dest[7] = pgtype.Timestamptz{Valid: true}
						dest[8] = pgtype.Timestamptz{Valid: true}
						dest[9] = pgtype.Timestamptz{Valid: false}
					}
					return nil
				},
			}
		},
	}
	queries := db.New(mockDB)

	called := false
	handler := ValidateSession(queries)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/protected", nil)
	ctx := context.WithValue(req.Context(), SessionIDKey, "550e8400-e29b-41d4-a716-446655440000")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if !called {
		t.Error("next handler was not called")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}
