package config

import (
	"os"
	"path/filepath"
	"strings"
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
	t.Setenv("BILLING_BANK_DETAILS", "Commercial Bank of Ethiopia, account 1000123456789, Kiflet PLC")
	if _, err := Load(); err == nil {
		t.Error("production without PUBLIC_URL should fail")
	}
	t.Setenv("PUBLIC_URL", "http://app.example.et")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "https://") {
		t.Errorf("production over plain HTTP: %v, want it refused", err)
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

func TestProvidersDefaultToMocksInDevelopment(t *testing.T) {
	t.Setenv("APP_SECRET_FILE", filepath.Join(t.TempDir(), "secret"))
	t.Setenv("PORT", "8090")
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got := c.MockedProviders(); len(got) != 3 {
		t.Errorf("MockedProviders = %v, want SMS, email and payments", got)
	}
	if c.MailFrom != "Kiflet <no-reply@localhost>" {
		t.Errorf("MailFrom = %q", c.MailFrom)
	}
}

func TestProductionHasNoPaymentStandIn(t *testing.T) {
	t.Setenv("APP_SECRET_FILE", filepath.Join(t.TempDir(), "secret"))
	t.Setenv("ENVIRONMENT", "production")
	t.Setenv("PUBLIC_URL", "https://kiflet.example.et/")
	if _, err := Load(); err == nil {
		t.Error("production with no way to pay should fail")
	}
	t.Setenv("BILLING_BANK_DETAILS", "Commercial Bank of Ethiopia, account 1000123456789, Kiflet PLC")
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.PaymentProvider != "none" {
		t.Errorf("PaymentProvider = %q, want none", c.PaymentProvider)
	}
	if c.MailFrom != "Kiflet <no-reply@kiflet.example.et>" {
		t.Errorf("MailFrom = %q", c.MailFrom)
	}
	t.Setenv("PAYMENT_PROVIDER", "paypal")
	if _, err := Load(); err == nil {
		t.Error("an unknown PAYMENT_PROVIDER should be rejected")
	}
}

func TestSupportTelegram(t *testing.T) {
	t.Setenv("APP_SECRET_FILE", filepath.Join(t.TempDir(), "secret"))
	for in, want := range map[string]string{"@kiflet_support": "kiflet_support", "https://t.me/KifletHelp": "KifletHelp", "": ""} {
		t.Setenv("SUPPORT_TELEGRAM", in)
		c, err := Load()
		if err != nil || c.SupportTelegram != want {
			t.Errorf("%q: %q, %v; want %q", in, c.SupportTelegram, err, want)
		}
	}
	t.Setenv("SUPPORT_TELEGRAM", "no spaces allowed")
	if _, err := Load(); err == nil {
		t.Error("a username with spaces was taken")
	}
}
