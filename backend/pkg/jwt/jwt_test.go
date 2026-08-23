package jwt

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func generateTestKeyPair(t *testing.T) (*rsa.PrivateKey, *rsa.PublicKey) {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate test key pair: %v", err)
	}
	return privateKey, &privateKey.PublicKey
}

func TestJWTManager_GenerateAccessToken(t *testing.T) {
	privateKey, publicKey := generateTestKeyPair(t)
	manager := NewManager(privateKey, publicKey, "test-issuer", time.Hour, 24*time.Hour)

	tests := []struct {
		name      string
		userID    string
		tenantID  string
		sessionID string
		role      string
		phone     string
		email     string
	}{
		{
			name:      "full claims",
			userID:    "user-123",
			tenantID:  "tenant-456",
			sessionID: "session-789",
			role:      "admin",
			phone:     "+1234567890",
			email:     "test@example.com",
		},
		{
			name:      "minimal claims",
			userID:    "user-abc",
			tenantID:  "tenant-def",
			sessionID: "session-ghi",
			role:      "user",
			phone:     "",
			email:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := manager.GenerateAccessToken(tt.userID, tt.tenantID, tt.sessionID, tt.role, tt.phone, tt.email)
			if err != nil {
				t.Fatalf("GenerateAccessToken() error = %v", err)
			}
			if token == "" {
				t.Fatal("GenerateAccessToken() returned empty token")
			}

			claims, err := manager.Validate(token)
			if err != nil {
				t.Fatalf("Validate() error = %v", err)
			}

			if claims.UserID != tt.userID {
				t.Errorf("UserID = %v, want %v", claims.UserID, tt.userID)
			}
			if claims.TenantID != tt.tenantID {
				t.Errorf("TenantID = %v, want %v", claims.TenantID, tt.tenantID)
			}
			if claims.SessionID != tt.sessionID {
				t.Errorf("SessionID = %v, want %v", claims.SessionID, tt.sessionID)
			}
			if claims.Role != tt.role {
				t.Errorf("Role = %v, want %v", claims.Role, tt.role)
			}
			if claims.Phone != tt.phone {
				t.Errorf("Phone = %v, want %v", claims.Phone, tt.phone)
			}
			if claims.Email != tt.email {
				t.Errorf("Email = %v, want %v", claims.Email, tt.email)
			}
			if claims.Issuer != "test-issuer" {
				t.Errorf("Issuer = %v, want test-issuer", claims.Issuer)
			}
			if claims.Subject != tt.userID {
				t.Errorf("Subject = %v, want %v", claims.Subject, tt.userID)
			}
		})
	}
}

func TestJWTManager_GenerateRefreshToken(t *testing.T) {
	privateKey, publicKey := generateTestKeyPair(t)
	manager := NewManager(privateKey, publicKey, "test-issuer", time.Hour, 24*time.Hour)

	token, err := manager.GenerateRefreshToken("user-123", "tenant-456")
	if err != nil {
		t.Fatalf("GenerateRefreshToken() error = %v", err)
	}
	if token == "" {
		t.Fatal("GenerateRefreshToken() returned empty token")
	}

	claims, err := manager.Validate(token)
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	if claims.UserID != "user-123" {
		t.Errorf("UserID = %v, want user-123", claims.UserID)
	}
	if claims.TenantID != "tenant-456" {
		t.Errorf("TenantID = %v, want tenant-456", claims.TenantID)
	}
	if claims.SessionID != "" {
		t.Errorf("SessionID = %v, want empty", claims.SessionID)
	}
	if claims.Role != "" {
		t.Errorf("Role = %v, want empty", claims.Role)
	}
}

func TestJWTManager_Validate_ValidToken(t *testing.T) {
	privateKey, publicKey := generateTestKeyPair(t)
	manager := NewManager(privateKey, publicKey, "test-issuer", time.Hour, 24*time.Hour)

	token, err := manager.GenerateAccessToken("user-123", "tenant-456", "session-789", "admin", "+1234567890", "test@example.com")
	if err != nil {
		t.Fatalf("GenerateAccessToken() error = %v", err)
	}

	claims, err := manager.Validate(token)
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	if claims.UserID != "user-123" {
		t.Errorf("UserID = %v, want user-123", claims.UserID)
	}
	if claims.TenantID != "tenant-456" {
		t.Errorf("TenantID = %v, want tenant-456", claims.TenantID)
	}
	if claims.SessionID != "session-789" {
		t.Errorf("SessionID = %v, want session-789", claims.SessionID)
	}
	if claims.Role != "admin" {
		t.Errorf("Role = %v, want admin", claims.Role)
	}
	if claims.Phone != "+1234567890" {
		t.Errorf("Phone = %v, want +1234567890", claims.Phone)
	}
	if claims.Email != "test@example.com" {
		t.Errorf("Email = %v, want test@example.com", claims.Email)
	}
}

func TestJWTManager_Validate_ExpiredToken(t *testing.T) {
	privateKey, publicKey := generateTestKeyPair(t)
	manager := NewManager(privateKey, publicKey, "test-issuer", time.Hour, 24*time.Hour)

	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "test-issuer",
			Subject:   "user-123",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
			NotBefore: jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
		},
		TenantID:  "tenant-456",
		UserID:    "user-123",
		SessionID: "session-789",
		Role:      "admin",
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tokenString, err := token.SignedString(privateKey)
	if err != nil {
		t.Fatalf("failed to sign expired token: %v", err)
	}

	_, err = manager.Validate(tokenString)
	if err == nil {
		t.Fatal("Validate() expected error for expired token, got nil")
	}
	if !errors.Is(err, jwt.ErrTokenExpired) && !errors.Is(errors.Unwrap(err), jwt.ErrTokenExpired) {
		if !containsString(err.Error(), "token is expired") && !containsString(err.Error(), "expired") {
			t.Errorf("Validate() error = %v, want expired token error", err)
		}
	}
}

func TestJWTManager_Validate_InvalidSigningMethod(t *testing.T) {
	privateKey, publicKey := generateTestKeyPair(t)
	manager := NewManager(privateKey, publicKey, "test-issuer", time.Hour, 24*time.Hour)

	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "test-issuer",
			Subject:   "user-123",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
		TenantID: "tenant-456",
		UserID:   "user-123",
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte("secret"))
	if err != nil {
		t.Fatalf("failed to sign token with HS256: %v", err)
	}

	_, err = manager.Validate(tokenString)
	if err == nil {
		t.Fatal("Validate() expected error for invalid signing method, got nil")
	}
	if !containsString(err.Error(), "unexpected signing method") {
		t.Errorf("Validate() error = %v, want unexpected signing method error", err)
	}
}

func TestJWTManager_RefreshTTL(t *testing.T) {
	privateKey, publicKey := generateTestKeyPair(t)
	expectedTTL := 24 * time.Hour
	manager := NewManager(privateKey, publicKey, "test-issuer", time.Hour, expectedTTL)

	result := manager.RefreshTTL()
	if result != expectedTTL {
		t.Errorf("RefreshTTL() = %v, want %v", result, expectedTTL)
	}
}

func TestLoadPrivateKey_ValidPEMFile(t *testing.T) {
	privateKey, _ := generateTestKeyPair(t)

	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "private.pem")

	privatePEM := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	}
	privateFile, err := os.Create(keyPath)
	if err != nil {
		t.Fatalf("failed to create temp key file: %v", err)
	}
	if err := pem.Encode(privateFile, privatePEM); err != nil {
		t.Fatalf("failed to write PEM: %v", err)
	}
	privateFile.Close()

	loadedKey, err := LoadPrivateKey(keyPath)
	if err != nil {
		t.Fatalf("LoadPrivateKey() error = %v", err)
	}

	if loadedKey.N.Cmp(privateKey.N) != 0 {
		t.Error("loaded key does not match original key")
	}
}

func TestLoadPrivateKey_InvalidFilePath(t *testing.T) {
	_, err := LoadPrivateKey("/nonexistent/path/private.pem")
	if err == nil {
		t.Fatal("LoadPrivateKey() expected error for invalid path, got nil")
	}
}

func TestLoadPrivateKey_InvalidPEMContent(t *testing.T) {
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "invalid.pem")

	err := os.WriteFile(keyPath, []byte("not a valid PEM content"), 0644)
	if err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	_, err = LoadPrivateKey(keyPath)
	if err == nil {
		t.Fatal("LoadPrivateKey() expected error for invalid PEM, got nil")
	}
	if !containsString(err.Error(), "failed to decode PEM block") {
		t.Errorf("LoadPrivateKey() error = %v, want PEM decode error", err)
	}
}

func TestLoadPrivateKey_InvalidKeyType(t *testing.T) {
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "invalid_key.pem")

	invalidPEM := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: []byte("not a valid key"),
	}
	file, err := os.Create(keyPath)
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	if err := pem.Encode(file, invalidPEM); err != nil {
		t.Fatalf("failed to write PEM: %v", err)
	}
	file.Close()

	_, err = LoadPrivateKey(keyPath)
	if err == nil {
		t.Fatal("LoadPrivateKey() expected error for invalid key type, got nil")
	}
	if !containsString(err.Error(), "failed to parse private key") {
		t.Errorf("LoadPrivateKey() error = %v, want parse error", err)
	}
}

func TestLoadPublicKey_ValidPEMFile(t *testing.T) {
	_, publicKey := generateTestKeyPair(t)

	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "public.pem")

	publicKeyBytes, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		t.Fatalf("failed to marshal public key: %v", err)
	}

	publicPEM := &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyBytes,
	}
	publicFile, err := os.Create(keyPath)
	if err != nil {
		t.Fatalf("failed to create temp key file: %v", err)
	}
	if err := pem.Encode(publicFile, publicPEM); err != nil {
		t.Fatalf("failed to write PEM: %v", err)
	}
	publicFile.Close()

	loadedKey, err := LoadPublicKey(keyPath)
	if err != nil {
		t.Fatalf("LoadPublicKey() error = %v", err)
	}

	if loadedKey.N.Cmp(publicKey.N) != 0 {
		t.Error("loaded key does not match original key")
	}
}

func TestLoadPublicKey_InvalidFilePath(t *testing.T) {
	_, err := LoadPublicKey("/nonexistent/path/public.pem")
	if err == nil {
		t.Fatal("LoadPublicKey() expected error for invalid path, got nil")
	}
}

func TestLoadPublicKey_InvalidPEMContent(t *testing.T) {
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "invalid.pem")

	err := os.WriteFile(keyPath, []byte("not a valid PEM content"), 0644)
	if err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	_, err = LoadPublicKey(keyPath)
	if err == nil {
		t.Fatal("LoadPublicKey() expected error for invalid PEM, got nil")
	}
	if !containsString(err.Error(), "failed to decode PEM block") {
		t.Errorf("LoadPublicKey() error = %v, want PEM decode error", err)
	}
}

func TestLoadPublicKey_InvalidKeyType(t *testing.T) {
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "invalid_key.pem")

	invalidPEM := &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: []byte("not a valid key"),
	}
	file, err := os.Create(keyPath)
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	if err := pem.Encode(file, invalidPEM); err != nil {
		t.Fatalf("failed to write PEM: %v", err)
	}
	file.Close()

	_, err = LoadPublicKey(keyPath)
	if err == nil {
		t.Fatal("LoadPublicKey() expected error for invalid key type, got nil")
	}
	if !containsString(err.Error(), "failed to parse public key") {
		t.Errorf("LoadPublicKey() error = %v, want parse error", err)
	}
}

func TestGenerateKeyPair(t *testing.T) {
	tmpDir := t.TempDir()
	privatePath := filepath.Join(tmpDir, "private.pem")
	publicPath := filepath.Join(tmpDir, "public.pem")

	err := GenerateKeyPair(privatePath, publicPath)
	if err != nil {
		t.Fatalf("GenerateKeyPair() error = %v", err)
	}

	if _, err := os.Stat(privatePath); os.IsNotExist(err) {
		t.Fatal("private key file was not created")
	}
	if _, err := os.Stat(publicPath); os.IsNotExist(err) {
		t.Fatal("public key file was not created")
	}

	privateKey, err := LoadPrivateKey(privatePath)
	if err != nil {
		t.Fatalf("failed to load generated private key: %v", err)
	}

	publicKey, err := LoadPublicKey(publicPath)
	if err != nil {
		t.Fatalf("failed to load generated public key: %v", err)
	}

	if privateKey.N.Cmp(publicKey.N) != 0 {
		t.Error("private and public key N values do not match")
	}

	manager := NewManager(privateKey, publicKey, "test-issuer", time.Hour, 24*time.Hour)
	token, err := manager.GenerateAccessToken("user-123", "tenant-456", "session-789", "admin", "", "")
	if err != nil {
		t.Fatalf("failed to generate token with generated key pair: %v", err)
	}

	_, err = manager.Validate(token)
	if err != nil {
		t.Fatalf("failed to validate token with generated key pair: %v", err)
	}
}

func TestGenerateKeyPair_InvalidDirectory(t *testing.T) {
	err := GenerateKeyPair("/nonexistent/path/private.pem", "/nonexistent/path/public.pem")
	if err == nil {
		t.Fatal("GenerateKeyPair() expected error for invalid directory, got nil")
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Refresh tokens are stored as a hash under a unique index, so two tokens
// issued for the same user in the same second must still differ. They did not:
// the claims held only the user, tenant and second-granularity timestamps, and
// RS256 signatures are deterministic.
func TestJWTManager_GenerateRefreshToken_UniquePerCall(t *testing.T) {
	privateKey, publicKey := generateTestKeyPair(t)
	manager := NewManager(privateKey, publicKey, "test-issuer", time.Hour, 24*time.Hour)

	const issueCount = 50
	seen := make(map[string]struct{}, issueCount)

	for i := 0; i < issueCount; i++ {
		token, err := manager.GenerateRefreshToken("user-123", "tenant-456")
		if err != nil {
			t.Fatalf("GenerateRefreshToken() error = %v", err)
		}
		if _, duplicate := seen[token]; duplicate {
			t.Fatalf("GenerateRefreshToken() returned a duplicate token on call %d", i+1)
		}
		seen[token] = struct{}{}
	}
}
