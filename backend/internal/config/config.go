// Package config loads settings from the environment.
package config

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Config is the deployment's settings. Every value has a default that works
// for local development; production needs at least DATABASE_URL and
// PUBLIC_URL.
type Config struct {
	Port        string
	DatabaseURL string
	// PublicURL is the origin people reach the app at, used in links sent by
	// SMS and to decide whether cookies are Secure.
	PublicURL   string
	AppName     string
	Timezone    *time.Location
	Environment string
	LogLevel    string
	// AutoMigrate applies pending migrations at startup.
	AutoMigrate bool
	// TrustProxy takes the client address from X-Forwarded-For / X-Real-IP.
	// Enable only behind a reverse proxy that sets them.
	TrustProxy bool
	// Secret keys one-time codes and signed cookies. Generated into SecretFile
	// on first start when not set.
	Secret     []byte
	SecretFile string
	// DevReload re-reads templates and static files from disk on every request.
	DevReload bool
	// MetricsToken, when set, enables /metrics for requests bearing it.
	MetricsToken string

	MapTileURL         string
	MapTileAttribution string

	StorageType string
	StoragePath string
	S3Bucket    string
	S3Region    string
	S3Endpoint  string
	S3AccessKey string
	S3SecretKey string
	S3UseSSL    bool

	SMSProvider          string
	SMSNumber            string
	SMSInboundSecret     string
	TwilioAccountSID     string
	TwilioAuthToken      string
	TwilioFromNumber     string
	EthioTelecomAPIURL   string
	EthioTelecomAPIKey   string
	EthioTelecomSenderID string

	// MailProvider is mock (printed to the terminal) or smtp.
	MailProvider string
	MailFrom     string
	SMTPAddr     string
	SMTPUsername string
	SMTPPassword string

	// PaymentProvider is mock (telebirr and CBE Birr stand-ins printed to the
	// terminal) or none (bank transfer only). It defaults to none in
	// production, since there is no real adapter yet.
	PaymentProvider string
	// BankDetails tells companies where to send a bank transfer, which the
	// platform operator then records by hand. Empty: transfers aren't
	// offered.
	BankDetails string
	// SupportEmail is where companies write to the platform operator.
	SupportEmail string

	// AllowMockProviders lets a production deployment start with stand-in
	// SMS, email or payments, for a demo or staging site.
	AllowMockProviders bool
}

// Load reads the environment.
func Load() (*Config, error) {
	c := &Config{
		Port:                 getEnv("PORT", "8080"),
		DatabaseURL:          getEnv("DATABASE_URL", "postgres://logistics:logistics@localhost:5434/logistics?sslmode=disable"),
		PublicURL:            strings.TrimRight(getEnv("PUBLIC_URL", ""), "/"),
		AppName:              getEnv("APP_NAME", "Menged"),
		Environment:          getEnv("ENVIRONMENT", "development"),
		LogLevel:             getEnv("LOG_LEVEL", "info"),
		AutoMigrate:          getBoolEnv("AUTO_MIGRATE", true),
		TrustProxy:           getBoolEnv("TRUST_PROXY", false),
		SecretFile:           getEnv("APP_SECRET_FILE", "keys/app_secret"),
		DevReload:            getBoolEnv("DEV_RELOAD", false),
		MetricsToken:         getEnv("METRICS_TOKEN", ""),
		MapTileURL:           getEnv("MAP_TILE_URL", "https://tile.openstreetmap.org/{z}/{x}/{y}.png"),
		MapTileAttribution:   getEnv("MAP_TILE_ATTRIBUTION", `&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors`),
		StorageType:          getEnv("STORAGE_TYPE", "local"),
		StoragePath:          getEnv("STORAGE_PATH", "./uploads"),
		S3Bucket:             getEnv("S3_BUCKET", ""),
		S3Region:             getEnv("S3_REGION", "us-east-1"),
		S3Endpoint:           getEnv("S3_ENDPOINT", ""),
		S3AccessKey:          getEnv("S3_ACCESS_KEY", ""),
		S3SecretKey:          getEnv("S3_SECRET_KEY", ""),
		S3UseSSL:             getBoolEnv("S3_USE_SSL", true),
		SMSProvider:          getEnv("SMS_PROVIDER", "mock"),
		SMSNumber:            getEnv("SMS_NUMBER", ""),
		SMSInboundSecret:     getEnv("SMS_INBOUND_SECRET", ""),
		TwilioAccountSID:     getEnv("TWILIO_ACCOUNT_SID", ""),
		TwilioAuthToken:      getEnv("TWILIO_AUTH_TOKEN", ""),
		TwilioFromNumber:     getEnv("TWILIO_FROM_NUMBER", ""),
		EthioTelecomAPIURL:   getEnv("ETHIO_TELECOM_API_URL", ""),
		EthioTelecomAPIKey:   getEnv("ETHIO_TELECOM_API_KEY", ""),
		EthioTelecomSenderID: getEnv("ETHIO_TELECOM_SENDER_ID", ""),
		MailProvider:         getEnv("MAIL_PROVIDER", "mock"),
		MailFrom:             getEnv("MAIL_FROM", ""),
		SMTPAddr:             getEnv("SMTP_ADDR", ""),
		SMTPUsername:         getEnv("SMTP_USERNAME", ""),
		SMTPPassword:         getEnv("SMTP_PASSWORD", ""),
		PaymentProvider:      getEnv("PAYMENT_PROVIDER", ""),
		BankDetails:          getEnv("BILLING_BANK_DETAILS", ""),
		SupportEmail:         getEnv("SUPPORT_EMAIL", ""),
		AllowMockProviders:   getBoolEnv("ALLOW_MOCK_PROVIDERS", false),
	}
	if c.PublicURL == "" {
		c.PublicURL = "http://localhost:" + c.Port
	}
	if c.SMSNumber == "" && c.SMSProvider == "mock" {
		// A stand-in short code, so the SMS instructions show in development.
		c.SMSNumber = "6060"
	}
	if c.MailFrom == "" {
		c.MailFrom = fmt.Sprintf("%s <no-reply@%s>", c.AppName, publicHost(c.PublicURL))
	}
	if c.PaymentProvider == "" {
		c.PaymentProvider = "mock"
		if c.IsProduction() {
			c.PaymentProvider = "none"
		}
	}
	if c.PaymentProvider != "mock" && c.PaymentProvider != "none" {
		return nil, fmt.Errorf("PAYMENT_PROVIDER %q: want mock or none", c.PaymentProvider)
	}
	if c.SupportEmail == "" {
		c.SupportEmail = "support@" + publicHost(c.PublicURL)
	}
	if c.BankDetails == "" && !c.IsProduction() {
		c.BankDetails = "Commercial Bank of Ethiopia, account 1000000000000, " + c.AppName +
			" (example details: set BILLING_BANK_DETAILS)"
	}

	tz, err := time.LoadLocation(getEnv("APP_TIMEZONE", "Africa/Addis_Ababa"))
	if err != nil {
		return nil, fmt.Errorf("APP_TIMEZONE: %w", err)
	}
	c.Timezone = tz

	if s := os.Getenv("APP_SECRET"); s != "" {
		if len(s) < 32 {
			return nil, errors.New("APP_SECRET must be at least 32 characters")
		}
		c.Secret = []byte(s)
	} else if c.Secret, err = loadOrCreateSecret(c.SecretFile); err != nil {
		return nil, fmt.Errorf("app secret: %w", err)
	}

	if c.IsProduction() && strings.HasPrefix(c.PublicURL, "http://localhost") {
		return nil, errors.New("PUBLIC_URL must be set in production")
	}
	if c.IsProduction() && c.PaymentProvider == "none" && c.BankDetails == "" {
		return nil, errors.New("BILLING_BANK_DETAILS must be set in production: without a payment provider, a bank transfer is the only way companies can pay")
	}
	return c, nil
}

// MockedProviders lists the services that only print to the terminal.
func (c *Config) MockedProviders() []string {
	var out []string
	if c.SMSProvider == "mock" {
		out = append(out, "SMS")
	}
	if c.MailProvider == "mock" {
		out = append(out, "email")
	}
	if c.PaymentProvider == "mock" {
		out = append(out, "payments")
	}
	return out
}

// publicHost is the host part of a URL, without port, for addresses.
func publicHost(u string) string {
	h := u
	if i := strings.Index(h, "://"); i >= 0 {
		h = h[i+3:]
	}
	if i := strings.IndexAny(h, "/:"); i >= 0 {
		h = h[:i]
	}
	if h == "" {
		return "localhost"
	}
	return h
}

// IsProduction reports whether this is a production deployment.
func (c *Config) IsProduction() bool { return c.Environment == "production" }

// SecureCookies reports whether cookies must be HTTPS-only.
func (c *Config) SecureCookies() bool { return strings.HasPrefix(c.PublicURL, "https://") }

func loadOrCreateSecret(path string) ([]byte, error) {
	if b, err := os.ReadFile(path); err == nil {
		s := strings.TrimSpace(string(b))
		if len(s) >= 32 {
			return []byte(s), nil
		}
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return nil, err
	}
	s := hex.EncodeToString(buf)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, []byte(s+"\n"), 0o600); err != nil {
		return nil, err
	}
	return []byte(s), nil
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getBoolEnv(key string, def bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return def
}
