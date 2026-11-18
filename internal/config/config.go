// Package config loads settings from the environment.
package config

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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
	// TrustProxy takes the client address from the last X-Forwarded-For
	// entry, the one the proxy adds. Enable only behind a reverse proxy
	// that sets it.
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
	// MapSatelliteURL is the imagery offered as a satellite view on the maps;
	// "off" leaves the maps without one.
	MapSatelliteURL         string
	MapSatelliteAttribution string

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
	EthioTelecomAPIURL   string
	EthioTelecomAPIKey   string
	EthioTelecomSenderID string

	// MailProvider is mock (printed to the terminal) or smtp.
	MailProvider string
	MailFrom     string
	SMTPAddr     string
	SMTPUsername string
	SMTPPassword string

	// PaymentProvider is mock (telebirr and CBE Birr stand-ins played by
	// pages on this server), none (bank transfer only), or the providers
	// offered, comma-separated: telebirr, cbebirr. It defaults to none in
	// production.
	PaymentProvider string
	Telebirr        TelebirrSettings
	CBEBirr         CBEBirrSettings
	// SandboxURL is the provider sandbox's address, set when the providers'
	// settings point at it, so that pages can link to its simulated phones.
	SandboxURL string
	// BankDetails tells companies where to send a bank transfer, which the
	// platform operator then records by hand. Empty: transfers aren't
	// offered.
	BankDetails string
	// SupportEmail is shown on invoices and is the sender companies see for
	// support answers.
	SupportEmail string
	// SupportPhone, SupportTelegram (a username, without the @) and
	// SupportHours are the other ways to reach support, shown on the Help
	// page. Each is left off when empty.
	SupportPhone    string
	SupportTelegram string
	SupportHours    string

	// RetainShipmentDays is how long a delivered or cancelled shipment keeps
	// its customer details, signatures and photos (FR-041).
	RetainShipmentDays int
	// CodesPerDay is how many verification codes (sign-up, PIN and password
	// resets, track by phone) the platform texts in 24 hours at most; 0
	// means no limit.
	CodesPerDay int

	// AllowMockProviders lets a production deployment start with stand-in
	// SMS, email or payments, for a demo or staging site.
	AllowMockProviders bool
	// PrivateWebhooks lets companies' webhooks go to private and loopback
	// addresses and over plain HTTP. It defaults to on in development, to
	// try webhooks against a receiver on the same machine, and off in
	// production, where it would let a company reach the server's own
	// network.
	PrivateWebhooks bool
}

// TelebirrSettings are a telebirr merchant's (H5 C2B web checkout). The
// keys are files: the merchant's private key, and telebirr's public key that
// checks its notifications.
type TelebirrSettings struct {
	APIURL         string
	CheckoutURL    string
	FabricAppID    string
	AppSecret      string
	MerchantAppID  string
	MerchantCode   string
	PrivateKeyFile string
	PublicKeyFile  string
}

// CBEBirrSettings are a CBE Birr merchant's: the ID payers pay to, and the
// login CBE Birr's servers use when they call Kiflet about a payment.
type CBEBirrSettings struct {
	MerchantID       string
	CallbackUsername string
	CallbackPassword string
}

// Load reads the environment.
// telegramName is a Telegram username.
var telegramName = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]{4,31}$`)

func Load() (*Config, error) {
	c := &Config{
		Port:                    getEnv("PORT", "8080"),
		DatabaseURL:             getEnv("DATABASE_URL", "postgres://logistics:logistics@localhost:5434/logistics?sslmode=disable"),
		PublicURL:               strings.TrimRight(getEnv("PUBLIC_URL", ""), "/"),
		AppName:                 getEnv("APP_NAME", "Kiflet"),
		Environment:             getEnv("ENVIRONMENT", "development"),
		LogLevel:                getEnv("LOG_LEVEL", "info"),
		AutoMigrate:             getBoolEnv("AUTO_MIGRATE", true),
		TrustProxy:              getBoolEnv("TRUST_PROXY", false),
		SecretFile:              getEnv("APP_SECRET_FILE", "keys/app_secret"),
		DevReload:               getBoolEnv("DEV_RELOAD", false),
		MetricsToken:            getEnv("METRICS_TOKEN", ""),
		MapTileURL:              getEnv("MAP_TILE_URL", "https://tile.openstreetmap.org/{z}/{x}/{y}.png"),
		MapTileAttribution:      getEnv("MAP_TILE_ATTRIBUTION", `&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors`),
		MapSatelliteURL:         getEnv("MAP_SATELLITE_URL", "https://server.arcgisonline.com/ArcGIS/rest/services/World_Imagery/MapServer/tile/{z}/{y}/{x}"),
		MapSatelliteAttribution: getEnv("MAP_SATELLITE_ATTRIBUTION", "Imagery &copy; Esri, Maxar, Earthstar Geographics"),
		StorageType:             getEnv("STORAGE_TYPE", "local"),
		StoragePath:             getEnv("STORAGE_PATH", "./uploads"),
		S3Bucket:                getEnv("S3_BUCKET", ""),
		S3Region:                getEnv("S3_REGION", "us-east-1"),
		S3Endpoint:              getEnv("S3_ENDPOINT", ""),
		S3AccessKey:             getEnv("S3_ACCESS_KEY", ""),
		S3SecretKey:             getEnv("S3_SECRET_KEY", ""),
		S3UseSSL:                getBoolEnv("S3_USE_SSL", true),
		SMSProvider:             getEnv("SMS_PROVIDER", "mock"),
		SMSNumber:               getEnv("SMS_NUMBER", ""),
		SMSInboundSecret:        getEnv("SMS_INBOUND_SECRET", ""),
		EthioTelecomAPIURL:      getEnv("ETHIO_TELECOM_API_URL", ""),
		EthioTelecomAPIKey:      getEnv("ETHIO_TELECOM_API_KEY", ""),
		EthioTelecomSenderID:    getEnv("ETHIO_TELECOM_SENDER_ID", ""),
		MailProvider:            getEnv("MAIL_PROVIDER", "mock"),
		MailFrom:                getEnv("MAIL_FROM", ""),
		SMTPAddr:                getEnv("SMTP_ADDR", ""),
		SMTPUsername:            getEnv("SMTP_USERNAME", ""),
		SMTPPassword:            getEnv("SMTP_PASSWORD", ""),
		PaymentProvider:         strings.ReplaceAll(strings.ToLower(getEnv("PAYMENT_PROVIDER", "")), " ", ""),
		Telebirr: TelebirrSettings{
			APIURL:         getEnv("TELEBIRR_API_URL", ""),
			CheckoutURL:    getEnv("TELEBIRR_CHECKOUT_URL", ""),
			FabricAppID:    getEnv("TELEBIRR_FABRIC_APP_ID", ""),
			AppSecret:      getEnv("TELEBIRR_APP_SECRET", ""),
			MerchantAppID:  getEnv("TELEBIRR_MERCHANT_APP_ID", ""),
			MerchantCode:   getEnv("TELEBIRR_MERCHANT_CODE", ""),
			PrivateKeyFile: getEnv("TELEBIRR_PRIVATE_KEY_FILE", ""),
			PublicKeyFile:  getEnv("TELEBIRR_PUBLIC_KEY_FILE", ""),
		},
		CBEBirr: CBEBirrSettings{
			MerchantID:       getEnv("CBEBIRR_MERCHANT_ID", ""),
			CallbackUsername: getEnv("CBEBIRR_CALLBACK_USERNAME", ""),
			CallbackPassword: getEnv("CBEBIRR_CALLBACK_PASSWORD", ""),
		},
		SandboxURL:         strings.TrimRight(getEnv("SANDBOX_URL", ""), "/"),
		BankDetails:        getEnv("BILLING_BANK_DETAILS", ""),
		SupportEmail:       getEnv("SUPPORT_EMAIL", ""),
		SupportPhone:       getEnv("SUPPORT_PHONE", ""),
		SupportTelegram:    strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(getEnv("SUPPORT_TELEGRAM", "")), "https://t.me/"), "@"),
		SupportHours:       getEnv("SUPPORT_HOURS", ""),
		AllowMockProviders: getBoolEnv("ALLOW_MOCK_PROVIDERS", false),
	}
	c.PrivateWebhooks = getBoolEnv("WEBHOOK_PRIVATE_TARGETS", !c.IsProduction())
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
	if err := c.checkPaymentProviders(); err != nil {
		return nil, err
	}
	if c.SupportEmail == "" {
		c.SupportEmail = "support@" + publicHost(c.PublicURL)
	}
	if c.BankDetails == "" && !c.IsProduction() {
		c.BankDetails = "Commercial Bank of Ethiopia, account 1000000000000, " + c.AppName +
			" (example details: set BILLING_BANK_DETAILS)"
	}

	days, err := strconv.Atoi(getEnv("RETAIN_SHIPMENT_DAYS", "365"))
	if err != nil || days < 30 {
		return nil, errors.New("RETAIN_SHIPMENT_DAYS must be a number of days, at least 30")
	}
	c.RetainShipmentDays = days

	codes, err := strconv.Atoi(getEnv("OTP_DAILY_BUDGET", "2000"))
	if err != nil || codes < 0 {
		return nil, errors.New("OTP_DAILY_BUDGET must be a number of codes, or 0 for no limit")
	}
	c.CodesPerDay = codes

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
	if c.PaymentProvider == "mock" {
		c.CBEBirr = standInCBEBirr(c.CBEBirr, c.Secret)
	}

	if c.IsProduction() {
		switch {
		case strings.HasPrefix(c.PublicURL, "http://localhost"):
			return nil, errors.New("PUBLIC_URL must be set in production")
		case !strings.HasPrefix(c.PublicURL, "https://"):
			// Over plain HTTP, sign-in cookies can't be marked Secure and
			// browsers refuse the driver app its service worker, so it
			// stops working offline.
			return nil, fmt.Errorf("PUBLIC_URL must be an https:// address in production, not %q", c.PublicURL)
		}
	}
	if c.SupportTelegram != "" && !telegramName.MatchString(c.SupportTelegram) {
		return nil, fmt.Errorf("SUPPORT_TELEGRAM must be a Telegram username (5 to 32 letters, digits or _), not %q", c.SupportTelegram)
	}
	if c.IsProduction() && c.PaymentProvider == "none" && c.BankDetails == "" {
		return nil, errors.New("BILLING_BANK_DETAILS must be set in production: without a payment provider, a bank transfer is the only way companies can pay")
	}
	return c, nil
}

// PaymentProviders are the providers PAYMENT_PROVIDER lists, none for mock
// or none.
func (c *Config) PaymentProviders() []string {
	if c.PaymentProvider == "mock" || c.PaymentProvider == "none" {
		return nil
	}
	return strings.Split(c.PaymentProvider, ",")
}

// checkPaymentProviders refuses a provider it doesn't know, or one whose
// settings are missing, naming them.
func (c *Config) checkPaymentProviders() error {
	if c.PaymentProvider == "mock" || c.PaymentProvider == "none" {
		return nil
	}
	seen := map[string]bool{}
	for _, code := range c.PaymentProviders() {
		var missing []string
		switch code {
		case "telebirr":
			t := c.Telebirr
			missing = unset(
				"TELEBIRR_API_URL", t.APIURL, "TELEBIRR_CHECKOUT_URL", t.CheckoutURL,
				"TELEBIRR_FABRIC_APP_ID", t.FabricAppID, "TELEBIRR_APP_SECRET", t.AppSecret,
				"TELEBIRR_MERCHANT_APP_ID", t.MerchantAppID, "TELEBIRR_MERCHANT_CODE", t.MerchantCode,
				"TELEBIRR_PRIVATE_KEY_FILE", t.PrivateKeyFile, "TELEBIRR_PUBLIC_KEY_FILE", t.PublicKeyFile)
		case "cbebirr":
			b := c.CBEBirr
			missing = unset("CBEBIRR_MERCHANT_ID", b.MerchantID,
				"CBEBIRR_CALLBACK_USERNAME", b.CallbackUsername, "CBEBIRR_CALLBACK_PASSWORD", b.CallbackPassword)
		default:
			return fmt.Errorf("PAYMENT_PROVIDER %q: want mock, none, or telebirr and cbebirr separated by commas", c.PaymentProvider)
		}
		if seen[code] {
			return fmt.Errorf("PAYMENT_PROVIDER lists %s twice", code)
		}
		seen[code] = true
		if len(missing) > 0 {
			return fmt.Errorf("PAYMENT_PROVIDER offers %s, which needs %s", code, strings.Join(missing, ", "))
		}
	}
	return nil
}

// unset names the settings among name, value pairs that are empty.
func unset(pairs ...string) []string {
	var out []string
	for i := 0; i+1 < len(pairs); i += 2 {
		if strings.TrimSpace(pairs[i+1]) == "" {
			out = append(out, pairs[i])
		}
	}
	return out
}

// standInCBEBirr fills in what the stand-in CBE Birr needs. It is the real
// adapter, called by the page that plays CBE Birr's servers, so it needs a
// merchant ID and a login: the login is derived from the secret, so it is
// the deployment's own.
func standInCBEBirr(b CBEBirrSettings, secret []byte) CBEBirrSettings {
	if b.MerchantID == "" {
		b.MerchantID = "100100"
	}
	if b.CallbackUsername == "" {
		b.CallbackUsername = "cbebirr"
	}
	if b.CallbackPassword == "" {
		mac := hmac.New(sha256.New, secret)
		mac.Write([]byte("cbebirr stand-in callbacks"))
		b.CallbackPassword = hex.EncodeToString(mac.Sum(nil))[:32]
	}
	return b
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
