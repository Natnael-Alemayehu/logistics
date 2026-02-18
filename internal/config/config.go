package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port        string
	DatabaseURL string

	JWTPrivateKeyPath string
	JWTPublicKeyPath  string
	JWTAccessTTL      time.Duration
	JWTRefreshTTL     time.Duration
	JWTIssuer         string

	LogLevel    string
	Environment string

	StorageType string
	StoragePath string
	S3Bucket    string
	S3Region    string
	S3Endpoint  string
	S3AccessKey string
	S3SecretKey string
	S3UseSSL    bool

	SMSProvider          string
	TwilioAccountSID     string
	TwilioAuthToken      string
	TwilioFromNumber     string
	EthioTelecomAPIURL   string
	EthioTelecomAPIKey   string
	EthioTelecomSenderID string
	TrackingURLBase      string
}

func Load() *Config {
	return &Config{
		Port:                 getEnv("PORT", "8080"),
		DatabaseURL:          getEnv("DATABASE_URL", "postgres://logistics:logistics@localhost:5432/logistics?sslmode=disable"),
		JWTPrivateKeyPath:    getEnv("JWT_PRIVATE_KEY_PATH", "keys/private.pem"),
		JWTPublicKeyPath:     getEnv("JWT_PUBLIC_KEY_PATH", "keys/public.pem"),
		JWTAccessTTL:         getDurationEnv("JWT_ACCESS_TTL", time.Hour),
		JWTRefreshTTL:        getDurationEnv("JWT_REFRESH_TTL", 30*24*time.Hour),
		JWTIssuer:            getEnv("JWT_ISSUER", "logistics.et"),
		LogLevel:             getEnv("LOG_LEVEL", "info"),
		Environment:          getEnv("ENVIRONMENT", "development"),
		StorageType:          getEnv("STORAGE_TYPE", "local"),
		StoragePath:          getEnv("STORAGE_PATH", "./uploads"),
		S3Bucket:             getEnv("S3_BUCKET", ""),
		S3Region:             getEnv("S3_REGION", "us-east-1"),
		S3Endpoint:           getEnv("S3_ENDPOINT", ""),
		S3AccessKey:          getEnv("S3_ACCESS_KEY", ""),
		S3SecretKey:          getEnv("S3_SECRET_KEY", ""),
		S3UseSSL:             getBoolEnv("S3_USE_SSL", true),
		SMSProvider:          getEnv("SMS_PROVIDER", "mock"),
		TwilioAccountSID:     getEnv("TWILIO_ACCOUNT_SID", ""),
		TwilioAuthToken:      getEnv("TWILIO_AUTH_TOKEN", ""),
		TwilioFromNumber:     getEnv("TWILIO_FROM_NUMBER", ""),
		EthioTelecomAPIURL:   getEnv("ETHIO_TELECOM_API_URL", ""),
		EthioTelecomAPIKey:   getEnv("ETHIO_TELECOM_API_KEY", ""),
		EthioTelecomSenderID: getEnv("ETHIO_TELECOM_SENDER_ID", ""),
		TrackingURLBase:      getEnv("TRACKING_URL_BASE", "https://track.logistics.et"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getDurationEnv(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if seconds, err := strconv.Atoi(value); err == nil {
			return time.Duration(seconds) * time.Second
		}
	}
	return defaultValue
}

func getBoolEnv(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		b, err := strconv.ParseBool(value)
		if err != nil {
			return defaultValue
		}
		return b
	}
	return defaultValue
}
