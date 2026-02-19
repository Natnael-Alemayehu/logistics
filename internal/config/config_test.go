package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLoad_Defaults(t *testing.T) {
	cfg := Load()

	assert.Equal(t, "8080", cfg.Port)
	assert.Equal(t, "info", cfg.LogLevel)
	assert.Equal(t, "development", cfg.Environment)
	assert.Equal(t, "local", cfg.StorageType)
	assert.Equal(t, "./uploads", cfg.StoragePath)
	assert.Equal(t, time.Hour, cfg.JWTAccessTTL)
	assert.Equal(t, 30*24*time.Hour, cfg.JWTRefreshTTL)
	assert.Equal(t, "logistics.et", cfg.JWTIssuer)
	assert.Equal(t, "mock", cfg.SMSProvider)
	assert.Equal(t, "localhost:6379", cfg.RedisURL)
	assert.Equal(t, 100, cfg.RateLimitIP)
	assert.Equal(t, 200, cfg.RateLimitUser)
	assert.Equal(t, 10, cfg.RateLimitAuth)
}

func TestLoad_EnvironmentVariables(t *testing.T) {
	os.Setenv("PORT", "3000")
	defer os.Unsetenv("PORT")

	os.Setenv("LOG_LEVEL", "debug")
	defer os.Unsetenv("LOG_LEVEL")

	os.Setenv("ENVIRONMENT", "production")
	defer os.Unsetenv("ENVIRONMENT")

	os.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/testdb")
	defer os.Unsetenv("DATABASE_URL")

	cfg := Load()

	assert.Equal(t, "3000", cfg.Port)
	assert.Equal(t, "debug", cfg.LogLevel)
	assert.Equal(t, "production", cfg.Environment)
	assert.Equal(t, "postgres://user:pass@localhost:5432/testdb", cfg.DatabaseURL)
}

func TestLoad_JWTConfig(t *testing.T) {
	os.Setenv("JWT_ACCESS_TTL", "3600")
	defer os.Unsetenv("JWT_ACCESS_TTL")

	os.Setenv("JWT_REFRESH_TTL", "2592000")
	defer os.Unsetenv("JWT_REFRESH_TTL")

	os.Setenv("JWT_ISSUER", "custom.issuer")
	defer os.Unsetenv("JWT_ISSUER")

	cfg := Load()

	assert.Equal(t, time.Hour, cfg.JWTAccessTTL)
	assert.Equal(t, 30*24*time.Hour, cfg.JWTRefreshTTL)
	assert.Equal(t, "custom.issuer", cfg.JWTIssuer)
}

func TestLoad_S3Config(t *testing.T) {
	os.Setenv("STORAGE_TYPE", "s3")
	defer os.Unsetenv("STORAGE_TYPE")

	os.Setenv("S3_BUCKET", "my-bucket")
	defer os.Unsetenv("S3_BUCKET")

	os.Setenv("S3_REGION", "eu-west-1")
	defer os.Unsetenv("S3_REGION")

	os.Setenv("S3_ENDPOINT", "https://s3.eu-west-1.amazonaws.com")
	defer os.Unsetenv("S3_ENDPOINT")

	os.Setenv("S3_ACCESS_KEY", "access-key")
	defer os.Unsetenv("S3_ACCESS_KEY")

	os.Setenv("S3_SECRET_KEY", "secret-key")
	defer os.Unsetenv("S3_SECRET_KEY")

	os.Setenv("S3_USE_SSL", "true")
	defer os.Unsetenv("S3_USE_SSL")

	cfg := Load()

	assert.Equal(t, "s3", cfg.StorageType)
	assert.Equal(t, "my-bucket", cfg.S3Bucket)
	assert.Equal(t, "eu-west-1", cfg.S3Region)
	assert.Equal(t, "https://s3.eu-west-1.amazonaws.com", cfg.S3Endpoint)
	assert.Equal(t, "access-key", cfg.S3AccessKey)
	assert.Equal(t, "secret-key", cfg.S3SecretKey)
	assert.True(t, cfg.S3UseSSL)
}

func TestLoad_SMSConfig(t *testing.T) {
	os.Setenv("SMS_PROVIDER", "twilio")
	defer os.Unsetenv("SMS_PROVIDER")

	os.Setenv("TWILIO_ACCOUNT_SID", "AC123")
	defer os.Unsetenv("TWILIO_ACCOUNT_SID")

	os.Setenv("TWILIO_AUTH_TOKEN", "token123")
	defer os.Unsetenv("TWILIO_AUTH_TOKEN")

	os.Setenv("TWILIO_FROM_NUMBER", "+1234567890")
	defer os.Unsetenv("TWILIO_FROM_NUMBER")

	cfg := Load()

	assert.Equal(t, "twilio", cfg.SMSProvider)
	assert.Equal(t, "AC123", cfg.TwilioAccountSID)
	assert.Equal(t, "token123", cfg.TwilioAuthToken)
	assert.Equal(t, "+1234567890", cfg.TwilioFromNumber)
}

func TestLoad_RedisConfig(t *testing.T) {
	os.Setenv("REDIS_URL", "redis.example.com:6380")
	defer os.Unsetenv("REDIS_URL")

	os.Setenv("REDIS_PASSWORD", "redis-pass")
	defer os.Unsetenv("REDIS_PASSWORD")

	os.Setenv("REDIS_DB", "1")
	defer os.Unsetenv("REDIS_DB")

	cfg := Load()

	assert.Equal(t, "redis.example.com:6380", cfg.RedisURL)
	assert.Equal(t, "redis-pass", cfg.RedisPassword)
	assert.Equal(t, 1, cfg.RedisDB)
}

func TestLoad_RateLimitConfig(t *testing.T) {
	os.Setenv("RATE_LIMIT_IP", "50")
	defer os.Unsetenv("RATE_LIMIT_IP")

	os.Setenv("RATE_LIMIT_USER", "100")
	defer os.Unsetenv("RATE_LIMIT_USER")

	os.Setenv("RATE_LIMIT_AUTH", "5")
	defer os.Unsetenv("RATE_LIMIT_AUTH")

	cfg := Load()

	assert.Equal(t, 50, cfg.RateLimitIP)
	assert.Equal(t, 100, cfg.RateLimitUser)
	assert.Equal(t, 5, cfg.RateLimitAuth)
}

func TestGetEnv(t *testing.T) {
	t.Run("returns value when set", func(t *testing.T) {
		os.Setenv("TEST_VAR", "test_value")
		defer os.Unsetenv("TEST_VAR")

		result := getEnv("TEST_VAR", "default")
		assert.Equal(t, "test_value", result)
	})

	t.Run("returns default when not set", func(t *testing.T) {
		result := getEnv("NONEXISTENT_VAR", "default_value")
		assert.Equal(t, "default_value", result)
	})

	t.Run("returns empty string value when set", func(t *testing.T) {
		os.Setenv("EMPTY_VAR", "")
		defer os.Unsetenv("EMPTY_VAR")

		result := getEnv("EMPTY_VAR", "default")
		assert.Equal(t, "default", result)
	})
}

func TestGetDurationEnv(t *testing.T) {
	t.Run("parses duration in seconds", func(t *testing.T) {
		os.Setenv("DURATION_VAR", "3600")
		defer os.Unsetenv("DURATION_VAR")

		result := getDurationEnv("DURATION_VAR", time.Minute)
		assert.Equal(t, time.Hour, result)
	})

	t.Run("returns default when not set", func(t *testing.T) {
		result := getDurationEnv("NONEXISTENT_DURATION", time.Minute)
		assert.Equal(t, time.Minute, result)
	})

	t.Run("returns default for invalid value", func(t *testing.T) {
		os.Setenv("INVALID_DURATION", "not-a-number")
		defer os.Unsetenv("INVALID_DURATION")

		result := getDurationEnv("INVALID_DURATION", time.Minute)
		assert.Equal(t, time.Minute, result)
	})
}

func TestGetBoolEnv(t *testing.T) {
	tests := []struct {
		name         string
		envValue     string
		defaultValue bool
		expected     bool
	}{
		{"true value", "true", false, true},
		{"false value", "false", true, false},
		{"1 value", "1", false, true},
		{"0 value", "0", true, false},
		{"invalid value", "invalid", true, true},
		{"not set", "", true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv("BOOL_VAR", tt.envValue)
				defer os.Unsetenv("BOOL_VAR")
			}

			result := getBoolEnv("BOOL_VAR", tt.defaultValue)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetIntEnv(t *testing.T) {
	t.Run("parses integer value", func(t *testing.T) {
		os.Setenv("INT_VAR", "42")
		defer os.Unsetenv("INT_VAR")

		result := getIntEnv("INT_VAR", 0)
		assert.Equal(t, 42, result)
	})

	t.Run("returns default when not set", func(t *testing.T) {
		result := getIntEnv("NONEXISTENT_INT", 10)
		assert.Equal(t, 10, result)
	})

	t.Run("returns default for invalid value", func(t *testing.T) {
		os.Setenv("INVALID_INT", "not-a-number")
		defer os.Unsetenv("INVALID_INT")

		result := getIntEnv("INVALID_INT", 5)
		assert.Equal(t, 5, result)
	})
}

func TestConfig_Struct(t *testing.T) {
	cfg := &Config{
		Port:              "8080",
		DatabaseURL:       "postgres://localhost/db",
		JWTPrivateKeyPath: "keys/private.pem",
		JWTPublicKeyPath:  "keys/public.pem",
		JWTAccessTTL:      time.Hour,
		JWTRefreshTTL:     24 * time.Hour,
		JWTIssuer:         "test",
		LogLevel:          "debug",
		Environment:       "test",
		StorageType:       "s3",
		StoragePath:       "/data",
		S3Bucket:          "bucket",
		S3Region:          "region",
		S3Endpoint:        "endpoint",
		S3AccessKey:       "key",
		S3SecretKey:       "secret",
		S3UseSSL:          true,
		SMSProvider:       "twilio",
		TwilioAccountSID:  "sid",
		TwilioAuthToken:   "token",
		TwilioFromNumber:  "number",
		RedisURL:          "redis:6379",
		RedisPassword:     "pass",
		RedisDB:           1,
	}

	assert.Equal(t, "8080", cfg.Port)
	assert.Equal(t, "postgres://localhost/db", cfg.DatabaseURL)
	assert.Equal(t, time.Hour, cfg.JWTAccessTTL)
	assert.True(t, cfg.S3UseSSL)
	assert.Equal(t, 1, cfg.RedisDB)
}

func BenchmarkLoad(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Load()
	}
}

func BenchmarkGetEnv(b *testing.B) {
	os.Setenv("BENCH_VAR", "value")
	defer os.Unsetenv("BENCH_VAR")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = getEnv("BENCH_VAR", "default")
	}
}

func BenchmarkGetIntEnv(b *testing.B) {
	os.Setenv("BENCH_INT", "42")
	defer os.Unsetenv("BENCH_INT")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = getIntEnv("BENCH_INT", 0)
	}
}
