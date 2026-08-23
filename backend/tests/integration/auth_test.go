//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/natnael-alemayehu/logistics/internal/db"
	"github.com/natnael-alemayehu/logistics/internal/service"
	"github.com/natnael-alemayehu/logistics/pkg/hash"
)

func TestDriverLogin_Success(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	ctx := context.Background()
	tenant := env.CreateTestTenant(ctx, "driver-login-success")
	driver := env.CreateTestDriver(ctx, tenant.ID.String(), "0912345678")

	authService := env.GetAuthService()

	result, err := authService.DriverLogin(ctx, service.DriverLoginInput{
		Phone: "0912345678",
		PIN:   "1234",
	}, "127.0.0.1", "test-agent")

	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	if result.User.ID != driver.ID {
		t.Errorf("expected user ID %s, got %s", driver.ID, result.User.ID)
	}

	if result.AccessToken == "" {
		t.Error("expected access token to be set")
	}

	if result.RefreshToken == "" {
		t.Error("expected refresh token to be set")
	}

	if result.User.Role != "driver" {
		t.Errorf("expected role driver, got %s", result.User.Role)
	}
}

func TestDriverLogin_InvalidPIN(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	ctx := context.Background()
	tenant := env.CreateTestTenant(ctx, "driver-invalid-pin")
	_ = env.CreateTestDriver(ctx, tenant.ID.String(), "0922345678")

	authService := env.GetAuthService()

	_, err := authService.DriverLogin(ctx, service.DriverLoginInput{
		Phone: "0922345678",
		PIN:   "0000",
	}, "127.0.0.1", "test-agent")

	if err != service.ErrInvalidCredentials {
		t.Errorf("expected invalid credentials error, got: %v", err)
	}
}

func TestDriverLogin_NonexistentPhone(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	ctx := context.Background()
	_ = env.CreateTestTenant(ctx, "driver-nonexistent")

	authService := env.GetAuthService()

	_, err := authService.DriverLogin(ctx, service.DriverLoginInput{
		Phone: "0999999999",
		PIN:   "1234",
	}, "127.0.0.1", "test-agent")

	if err != service.ErrInvalidCredentials {
		t.Errorf("expected invalid credentials error, got: %v", err)
	}
}

func TestDriverLogin_InactiveAccount(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	ctx := context.Background()
	tenant := env.CreateTestTenant(ctx, "driver-inactive")

	isActive := false
	pinHash, _ := hash.PIN("1234")
	_, err := env.GetQueries().CreateUser(ctx, db.CreateUserParams{
		TenantID: toUUID(tenant.ID.String()),
		Role:     "driver",
		FullName: "Inactive Driver",
		Phone:    ptr("0933345678"),
		PinHash:  &pinHash,
		IsActive: &isActive,
	})
	if err != nil {
		t.Fatalf("failed to create inactive driver: %v", err)
	}

	authService := env.GetAuthService()

	_, err = authService.DriverLogin(ctx, service.DriverLoginInput{
		Phone: "0933345678",
		PIN:   "1234",
	}, "127.0.0.1", "test-agent")

	if err != service.ErrAccountInactive {
		t.Errorf("expected account inactive error, got: %v", err)
	}
}

func TestDispatcherLogin_Success(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	ctx := context.Background()
	tenant := env.CreateTestTenant(ctx, "dispatcher-login-success")
	dispatcher := env.CreateTestDispatcher(ctx, tenant.ID.String(), "dispatcher@test.com")

	authService := env.GetAuthService()

	result, err := authService.DispatcherLogin(ctx, service.DispatcherLoginInput{
		Email:    "dispatcher@test.com",
		Password: "password123",
	}, "127.0.0.1", "test-agent")

	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	if result.User.ID != dispatcher.ID {
		t.Errorf("expected user ID %s, got %s", dispatcher.ID, result.User.ID)
	}

	if result.AccessToken == "" {
		t.Error("expected access token to be set")
	}

	if result.User.Role != "dispatcher" {
		t.Errorf("expected role dispatcher, got %s", result.User.Role)
	}
}

func TestDispatcherLogin_InvalidPassword(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	ctx := context.Background()
	tenant := env.CreateTestTenant(ctx, "dispatcher-invalid-pwd")
	_ = env.CreateTestDispatcher(ctx, tenant.ID.String(), "invalid-pwd@test.com")

	authService := env.GetAuthService()

	_, err := authService.DispatcherLogin(ctx, service.DispatcherLoginInput{
		Email:    "invalid-pwd@test.com",
		Password: "wrongpassword",
	}, "127.0.0.1", "test-agent")

	if err != service.ErrInvalidCredentials {
		t.Errorf("expected invalid credentials error, got: %v", err)
	}
}

func TestDispatcherLogin_NonexistentEmail(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	ctx := context.Background()
	_ = env.CreateTestTenant(ctx, "dispatcher-nonexistent")

	authService := env.GetAuthService()

	_, err := authService.DispatcherLogin(ctx, service.DispatcherLoginInput{
		Email:    "nonexistent@test.com",
		Password: "password123",
	}, "127.0.0.1", "test-agent")

	if err != service.ErrInvalidCredentials {
		t.Errorf("expected invalid credentials error, got: %v", err)
	}
}

func TestDispatcherLogin_DriverCannotUseDispatcherEndpoint(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	ctx := context.Background()
	tenant := env.CreateTestTenant(ctx, "driver-dispatch-endpoint")

	passwordHash, _ := hash.Password("password123")
	isActive := true
	_, err := env.GetQueries().CreateUser(ctx, db.CreateUserParams{
		TenantID:     toUUID(tenant.ID.String()),
		Role:         "driver",
		FullName:     "Driver User",
		Email:        ptr("driver-dispatch@test.com"),
		PasswordHash: &passwordHash,
		IsActive:     &isActive,
	})
	if err != nil {
		t.Fatalf("failed to create driver with email: %v", err)
	}

	authService := env.GetAuthService()

	_, err = authService.DispatcherLogin(ctx, service.DispatcherLoginInput{
		Email:    "driver-dispatch@test.com",
		Password: "password123",
	}, "127.0.0.1", "test-agent")

	if err == nil {
		t.Error("expected error for driver using dispatcher endpoint")
	}
}

func TestAccountLockout_AfterFiveFailedAttempts(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	ctx := context.Background()
	tenant := env.CreateTestTenant(ctx, "account-lockout")
	_ = env.CreateTestDispatcher(ctx, tenant.ID.String(), "lockout@test.com")

	authService := env.GetAuthService()

	for i := 0; i < 5; i++ {
		_, err := authService.DispatcherLogin(ctx, service.DispatcherLoginInput{
			Email:    "lockout@test.com",
			Password: "wrongpassword",
		}, "127.0.0.1", "test-agent")

		if err == nil {
			t.Fatalf("expected login to fail on attempt %d", i+1)
		}
	}

	_, err := authService.DispatcherLogin(ctx, service.DispatcherLoginInput{
		Email:    "lockout@test.com",
		Password: "password123",
	}, "127.0.0.1", "test-agent")

	if err != service.ErrAccountLocked {
		t.Errorf("expected account locked error, got: %v", err)
	}
}

func TestAccountLockout_LockedUserCannotLogin(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	ctx := context.Background()
	tenant := env.CreateTestTenant(ctx, "locked-user")
	driver := env.CreateTestDriver(ctx, tenant.ID.String(), "0955566677")

	env.LockUserAccount(ctx, driver.ID)

	authService := env.GetAuthService()

	_, err := authService.DriverLogin(ctx, service.DriverLoginInput{
		Phone: "0955566677",
		PIN:   "1234",
	}, "127.0.0.1", "test-agent")

	if err != service.ErrAccountLocked {
		t.Errorf("expected account locked error, got: %v", err)
	}
}

func TestAccountLockout_UnlockAllowsLogin(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	ctx := context.Background()
	tenant := env.CreateTestTenant(ctx, "unlock-account")
	dispatcher := env.CreateTestDispatcher(ctx, tenant.ID.String(), "unlock@test.com")

	env.LockUserAccount(ctx, dispatcher.ID)

	authService := env.GetAuthService()

	err := authService.UnlockAccount(ctx, dispatcher.ID)
	if err != nil {
		t.Fatalf("failed to unlock account: %v", err)
	}

	result, err := authService.DispatcherLogin(ctx, service.DispatcherLoginInput{
		Email:    "unlock@test.com",
		Password: "password123",
	}, "127.0.0.1", "test-agent")

	if err != nil {
		t.Fatalf("expected login to succeed after unlock, got: %v", err)
	}

	if result.User.ID != dispatcher.ID {
		t.Errorf("expected user ID %s, got %s", dispatcher.ID, result.User.ID)
	}
}

func TestTokenRefresh_Success(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	ctx := context.Background()
	tenant := env.CreateTestTenant(ctx, "token-refresh")
	dispatcher := env.CreateTestDispatcher(ctx, tenant.ID.String(), "refresh@test.com")

	authService := env.GetAuthService()

	loginResult, err := authService.DispatcherLogin(ctx, service.DispatcherLoginInput{
		Email:    "refresh@test.com",
		Password: "password123",
	}, "127.0.0.1", "test-agent")
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	refreshResult, err := authService.RefreshToken(ctx, loginResult.RefreshToken)
	if err != nil {
		t.Fatalf("token refresh failed: %v", err)
	}

	if refreshResult.AccessToken == "" {
		t.Error("expected access token to be set")
	}

	if refreshResult.RefreshToken == "" {
		t.Error("expected refresh token to be set")
	}

	if refreshResult.User.ID != dispatcher.ID {
		t.Errorf("expected user ID %s, got %s", dispatcher.ID, refreshResult.User.ID)
	}
}

func TestTokenRefresh_OldTokenRevoked(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	ctx := context.Background()
	tenant := env.CreateTestTenant(ctx, "token-revoked")
	_ = env.CreateTestDispatcher(ctx, tenant.ID.String(), "revoked@test.com")

	authService := env.GetAuthService()

	loginResult, err := authService.DispatcherLogin(ctx, service.DispatcherLoginInput{
		Email:    "revoked@test.com",
		Password: "password123",
	}, "127.0.0.1", "test-agent")
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	_, err = authService.RefreshToken(ctx, loginResult.RefreshToken)
	if err != nil {
		t.Fatalf("first refresh failed: %v", err)
	}

	_, err = authService.RefreshToken(ctx, loginResult.RefreshToken)
	if err == nil {
		t.Error("expected error when using revoked token")
	}
}

func TestTokenRefresh_InvalidToken(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	ctx := context.Background()

	authService := env.GetAuthService()

	_, err := authService.RefreshToken(ctx, "invalid-refresh-token")
	if err == nil {
		t.Error("expected error for invalid refresh token")
	}
}

func TestTokenRefresh_SessionRevoked(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	ctx := context.Background()
	tenant := env.CreateTestTenant(ctx, "session-revoked")
	dispatcher := env.CreateTestDispatcher(ctx, tenant.ID.String(), "session-revoked@test.com")

	authService := env.GetAuthService()

	loginResult, err := authService.DispatcherLogin(ctx, service.DispatcherLoginInput{
		Email:    "session-revoked@test.com",
		Password: "password123",
	}, "127.0.0.1", "test-agent")
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	sessions, err := authService.ListSessions(ctx, dispatcher.ID)
	if err != nil {
		t.Fatalf("failed to list sessions: %v", err)
	}
	if len(sessions) == 0 {
		t.Fatal("expected at least one session")
	}

	err = authService.RevokeSession(ctx, dispatcher.ID, sessions[0].ID)
	if err != nil {
		t.Fatalf("failed to revoke session: %v", err)
	}

	_, err = authService.RefreshToken(ctx, loginResult.RefreshToken)
	if err != service.ErrSessionRevoked {
		t.Errorf("expected session revoked error, got: %v", err)
	}
}

func TestLogout_Success(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	ctx := context.Background()
	tenant := env.CreateTestTenant(ctx, "logout-success")
	dispatcher := env.CreateTestDispatcher(ctx, tenant.ID.String(), "logout@test.com")

	authService := env.GetAuthService()

	loginResult, err := authService.DispatcherLogin(ctx, service.DispatcherLoginInput{
		Email:    "logout@test.com",
		Password: "password123",
	}, "127.0.0.1", "test-agent")
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	sessions, err := authService.ListSessions(ctx, dispatcher.ID)
	if err != nil {
		t.Fatalf("failed to list sessions: %v", err)
	}
	if len(sessions) == 0 {
		t.Fatal("expected at least one session")
	}

	err = authService.Logout(ctx, dispatcher.ID, sessions[0].ID)
	if err != nil {
		t.Fatalf("logout failed: %v", err)
	}

	_, err = authService.RefreshToken(ctx, loginResult.RefreshToken)
	if err == nil {
		t.Error("expected error when using token after logout")
	}
}

func TestListSessions(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	ctx := context.Background()
	tenant := env.CreateTestTenant(ctx, "list-sessions")
	dispatcher := env.CreateTestDispatcher(ctx, tenant.ID.String(), "sessions@test.com")

	authService := env.GetAuthService()

	_, _ = authService.DispatcherLogin(ctx, service.DispatcherLoginInput{
		Email:    "sessions@test.com",
		Password: "password123",
	}, "127.0.0.1", "test-agent-1")

	_, _ = authService.DispatcherLogin(ctx, service.DispatcherLoginInput{
		Email:    "sessions@test.com",
		Password: "password123",
	}, "127.0.0.1", "test-agent-2")

	sessions, err := authService.ListSessions(ctx, dispatcher.ID)
	if err != nil {
		t.Fatalf("failed to list sessions: %v", err)
	}

	if len(sessions) < 2 {
		t.Errorf("expected at least 2 sessions, got %d", len(sessions))
	}

	for _, s := range sessions {
		if s.UserID != dispatcher.ID {
			t.Errorf("expected session user ID %s, got %s", dispatcher.ID, s.UserID)
		}
	}
}

func TestRevokeOtherSessions(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	ctx := context.Background()
	tenant := env.CreateTestTenant(ctx, "revoke-others")
	dispatcher := env.CreateTestDispatcher(ctx, tenant.ID.String(), "revoke-others@test.com")

	authService := env.GetAuthService()

	_, _ = authService.DispatcherLogin(ctx, service.DispatcherLoginInput{
		Email:    "revoke-others@test.com",
		Password: "password123",
	}, "127.0.0.1", "test-agent-1")

	result2, _ := authService.DispatcherLogin(ctx, service.DispatcherLoginInput{
		Email:    "revoke-others@test.com",
		Password: "password123",
	}, "127.0.0.1", "test-agent-2")

	sessions, _ := authService.ListSessions(ctx, dispatcher.ID)
	if len(sessions) < 2 {
		t.Fatalf("expected at least 2 sessions before revoke, got %d", len(sessions))
	}

	err := authService.RevokeOtherSessions(ctx, dispatcher.ID, sessions[1].ID)
	if err != nil {
		t.Fatalf("failed to revoke other sessions: %v", err)
	}

	_, err = authService.RefreshToken(ctx, result2.RefreshToken)
	if err == nil {
		t.Error("expected error when using revoked token")
	}
}

func TestPasswordUpdate(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	ctx := context.Background()
	tenant := env.CreateTestTenant(ctx, "password-update")
	dispatcher := env.CreateTestDispatcher(ctx, tenant.ID.String(), "pwd-update@test.com")

	authService := env.GetAuthService()

	_, err := authService.UpdatePassword(ctx, dispatcher.ID, "newpassword123")
	if err != nil {
		t.Fatalf("failed to update password: %v", err)
	}

	_, err = authService.DispatcherLogin(ctx, service.DispatcherLoginInput{
		Email:    "pwd-update@test.com",
		Password: "newpassword123",
	}, "127.0.0.1", "test-agent")
	if err != nil {
		t.Fatalf("login with new password failed: %v", err)
	}

	_, err = authService.DispatcherLogin(ctx, service.DispatcherLoginInput{
		Email:    "pwd-update@test.com",
		Password: "password123",
	}, "127.0.0.1", "test-agent")
	if err == nil {
		t.Error("expected login with old password to fail")
	}
}

func TestPINUpdate(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	ctx := context.Background()
	tenant := env.CreateTestTenant(ctx, "pin-update")
	driver := env.CreateTestDriver(ctx, tenant.ID.String(), "0977766655")

	authService := env.GetAuthService()

	_, err := authService.UpdatePIN(ctx, driver.ID, "9999")
	if err != nil {
		t.Fatalf("failed to update PIN: %v", err)
	}

	_, err = authService.DriverLogin(ctx, service.DriverLoginInput{
		Phone: "0977766655",
		PIN:   "9999",
	}, "127.0.0.1", "test-agent")
	if err != nil {
		t.Fatalf("login with new PIN failed: %v", err)
	}

	_, err = authService.DriverLogin(ctx, service.DriverLoginInput{
		Phone: "0977766655",
		PIN:   "1234",
	}, "127.0.0.1", "test-agent")
	if err == nil {
		t.Error("expected login with old PIN to fail")
	}
}

func TestDriverLogin_HTTP(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	ctx := context.Background()
	tenant := env.CreateTestTenant(ctx, "driver-http")
	_ = env.CreateTestDriver(ctx, tenant.ID.String(), "0988877766")

	body := `{"phone": "0988877766", "pin": "1234"}`
	req := httptest.NewRequest(http.MethodPost, apiPrefix+"/auth/login/driver", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	env.GetRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var response map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	data, ok := response["data"].(map[string]interface{})
	if !ok {
		t.Fatal("expected data in response")
	}

	if _, ok := data["access_token"]; !ok {
		t.Error("expected access_token in response")
	}
}

func TestDispatcherLogin_HTTP(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	ctx := context.Background()
	tenant := env.CreateTestTenant(ctx, "dispatcher-http")
	_ = env.CreateTestDispatcher(ctx, tenant.ID.String(), "dispatcher-http@test.com")

	body := `{"email": "dispatcher-http@test.com", "password": "password123"}`
	req := httptest.NewRequest(http.MethodPost, apiPrefix+"/auth/login/dispatcher", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	env.GetRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestLogin_InvalidRequest_HTTP(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tests := []struct {
		name       string
		endpoint   string
		body       string
		expectCode int
	}{
		{
			name:       "driver missing phone",
			endpoint:   "/auth/login/driver",
			body:       `{"pin": "1234"}`,
			expectCode: http.StatusBadRequest,
		},
		{
			name:       "driver invalid phone format",
			endpoint:   "/auth/login/driver",
			body:       `{"phone": "invalid", "pin": "1234"}`,
			expectCode: http.StatusBadRequest,
		},
		{
			name:       "dispatcher missing email",
			endpoint:   "/auth/login/dispatcher",
			body:       `{"password": "password123"}`,
			expectCode: http.StatusBadRequest,
		},
		{
			name:       "dispatcher invalid email",
			endpoint:   "/auth/login/dispatcher",
			body:       `{"email": "invalid-email", "password": "password123"}`,
			expectCode: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, apiPrefix+tt.endpoint, strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			env.GetRouter().ServeHTTP(rec, req)

			if rec.Code != tt.expectCode {
				t.Errorf("expected status %d, got %d: %s", tt.expectCode, rec.Code, rec.Body.String())
			}
		})
	}
}

func TestLogout_HTTP(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	ctx := context.Background()
	tenant := env.CreateTestTenant(ctx, "logout-http")
	_ = env.CreateTestDispatcher(ctx, tenant.ID.String(), "logout-http@test.com")

	authService := env.GetAuthService()
	loginResult, _ := authService.DispatcherLogin(ctx, service.DispatcherLoginInput{
		Email:    "logout-http@test.com",
		Password: "password123",
	}, "127.0.0.1", "test-agent")

	req := httptest.NewRequest(http.MethodPost, apiPrefix+"/auth/logout", nil)
	req.Header.Set("Authorization", "Bearer "+loginResult.AccessToken)
	rec := httptest.NewRecorder()

	env.GetRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestRefreshToken_HTTP(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	ctx := context.Background()
	tenant := env.CreateTestTenant(ctx, "refresh-http")
	_ = env.CreateTestDispatcher(ctx, tenant.ID.String(), "refresh-http@test.com")

	authService := env.GetAuthService()
	loginResult, _ := authService.DispatcherLogin(ctx, service.DispatcherLoginInput{
		Email:    "refresh-http@test.com",
		Password: "password123",
	}, "127.0.0.1", "test-agent")

	body := fmt.Sprintf(`{"refresh_token": "%s"}`, loginResult.RefreshToken)
	req := httptest.NewRequest(http.MethodPost, apiPrefix+"/auth/refresh", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	env.GetRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func ptr[T any](v T) *T {
	return &v
}

// Revocation must take effect immediately, not whenever the access token
// happens to expire. ValidateSession is what enforces that: Auth alone only
// checks the signature and expiry, so before it was wired into the router a
// token kept after logout stayed usable for the remainder of JWT_ACCESS_TTL —
// an hour by default. That made "log out" and "revoke this lost device"
// cosmetic for every authenticated endpoint.
func TestAccessTokenRejectedAfterLogout(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	ctx := context.Background()
	tenant := env.CreateTestTenant(ctx, "revoke-after-logout")
	_ = env.CreateTestDispatcher(ctx, tenant.ID.String(), "revoke-logout@test.com")

	authService := env.GetAuthService()
	loginResult, err := authService.DispatcherLogin(ctx, service.DispatcherLoginInput{
		Email:    "revoke-logout@test.com",
		Password: "password123",
	}, "127.0.0.1", "test-agent")
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	authorized := func() int {
		req := httptest.NewRequest(http.MethodGet, apiPrefix+"/sessions", nil)
		req.Header.Set("Authorization", "Bearer "+loginResult.AccessToken)
		rec := httptest.NewRecorder()
		env.GetRouter().ServeHTTP(rec, req)
		return rec.Code
	}

	if code := authorized(); code != http.StatusOK {
		t.Fatalf("expected the token to work before logout, got %d", code)
	}

	logout := httptest.NewRequest(http.MethodPost, apiPrefix+"/auth/logout", nil)
	logout.Header.Set("Authorization", "Bearer "+loginResult.AccessToken)
	logoutRec := httptest.NewRecorder()
	env.GetRouter().ServeHTTP(logoutRec, logout)

	if logoutRec.Code != http.StatusOK {
		t.Fatalf("logout failed with %d: %s", logoutRec.Code, logoutRec.Body.String())
	}

	// Same token, same TTL — the only thing that changed is the revoked session.
	if code := authorized(); code != http.StatusUnauthorized {
		t.Errorf("expected 401 after logout, got %d: the access token outlived its session", code)
	}
}
