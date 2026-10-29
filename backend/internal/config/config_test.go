package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("APP_SECRET_FILE", filepath.Join(t.TempDir(), "secret"))
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.PublicURL != "http://localhost:8080" {
		t.Errorf("PublicURL = %q", c.PublicURL)
	}
	if c.Timezone.String() != "Africa/Addis_Ababa" {
		t.Errorf("Timezone = %v", c.Timezone)
	}
	if len(c.Secret) < 32 {
		t.Errorf("generated secret too short: %d", len(c.Secret))
	}
	if c.SecureCookies() {
		t.Error("cookies over plain http must not be marked Secure")
	}
}

func TestSecretIsPersisted(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keys", "secret")
	t.Setenv("APP_SECRET_FILE", path)
	a, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	b, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if string(a.Secret) != string(b.Secret) {
		t.Error("the generated secret should be reused on the next start")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("secret file mode = %v, want 0600", info.Mode().Perm())
	}
}

func TestShortSecretRejected(t *testing.T) {
	t.Setenv("APP_SECRET", "too-short")
	if _, err := Load(); err == nil {
		t.Error("a short APP_SECRET should be rejected")
	}
}

func TestProductionRequiresPublicURL(t *testing.T) {
	t.Setenv("APP_SECRET_FILE", filepath.Join(t.TempDir(), "secret"))
	t.Setenv("ENVIRONMENT", "production")
	if _, err := Load(); err == nil {
		t.Error("production without PUBLIC_URL should fail")
	}
	t.Setenv("PUBLIC_URL", "https://app.example.et/")
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.PublicURL != "https://app.example.et" || !c.SecureCookies() {
		t.Errorf("PublicURL = %q, secure = %v", c.PublicURL, c.SecureCookies())
	}
}

func TestBadTimezone(t *testing.T) {
	t.Setenv("APP_SECRET_FILE", filepath.Join(t.TempDir(), "secret"))
	t.Setenv("APP_TIMEZONE", "Mars/Olympus")
	if _, err := Load(); err == nil {
		t.Error("an unknown time zone should fail")
	}
}
