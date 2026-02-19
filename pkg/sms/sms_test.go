package sms

import (
	"testing"
)

func TestNormalizeEthiopianPhone(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expected  string
		expectErr error
	}{
		{
			name:      "already normalized with +251 prefix",
			input:     "+251912345678",
			expected:  "+251912345678",
			expectErr: nil,
		},
		{
			name:      "251 prefix needs + prefix",
			input:     "251912345678",
			expected:  "+251912345678",
			expectErr: nil,
		},
		{
			name:      "local format 09XXXXXXXX",
			input:     "0912345678",
			expected:  "+251912345678",
			expectErr: nil,
		},
		{
			name:      "local format 07XXXXXXXX",
			input:     "0712345678",
			expected:  "+251712345678",
			expectErr: nil,
		},
		{
			name:      "without leading 0 - 9XXXXXXX",
			input:     "912345678",
			expected:  "+251912345678",
			expectErr: nil,
		},
		{
			name:      "without leading 0 - 7XXXXXXX",
			input:     "712345678",
			expected:  "+251712345678",
			expectErr: nil,
		},
		{
			name:      "empty string",
			input:     "",
			expected:  "",
			expectErr: ErrInvalidPhone,
		},
		{
			name:      "invalid format - too short",
			input:     "123",
			expected:  "",
			expectErr: ErrInvalidPhone,
		},
		{
			name:      "invalid format - wrong prefix",
			input:     "+1234567890123",
			expected:  "",
			expectErr: ErrInvalidPhone,
		},
		{
			name:      "invalid +251 format - wrong length",
			input:     "+25191234567",
			expected:  "",
			expectErr: ErrInvalidPhone,
		},
		{
			name:      "invalid 251 format - wrong length",
			input:     "25191234567",
			expected:  "",
			expectErr: ErrInvalidPhone,
		},
		{
			name:      "invalid local format - wrong prefix",
			input:     "0812345678",
			expected:  "",
			expectErr: ErrInvalidPhone,
		},
		{
			name:      "invalid short format - wrong digit",
			input:     "812345678",
			expected:  "",
			expectErr: ErrInvalidPhone,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := NormalizeEthiopianPhone(tt.input)
			if err != tt.expectErr {
				t.Errorf("NormalizeEthiopianPhone(%q) error = %v, want %v", tt.input, err, tt.expectErr)
			}
			if result != tt.expected {
				t.Errorf("NormalizeEthiopianPhone(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestNewProvider_Twilio(t *testing.T) {
	logger := &mockLogger{}
	rateLimiter := NewInMemoryRateLimiter(RateLimiterConfig{})
	optOutStore := NewInMemoryOptOutStore()

	cfg := Config{
		Provider:         ProviderTwilio,
		TwilioAccountSID: "test_sid",
		TwilioAuthToken:  "test_token",
		TwilioFromNumber: "+1234567890",
	}

	provider, err := NewProvider(cfg, logger, rateLimiter, optOutStore)
	if err != nil {
		t.Fatalf("NewProvider() error = %v", err)
	}
	if provider == nil {
		t.Fatal("NewProvider() returned nil provider")
	}
	if provider.Name() != "twilio" {
		t.Errorf("provider.Name() = %q, want %q", provider.Name(), "twilio")
	}
}

func TestNewProvider_TwilioMissingConfig(t *testing.T) {
	logger := &mockLogger{}
	rateLimiter := NewInMemoryRateLimiter(RateLimiterConfig{})
	optOutStore := NewInMemoryOptOutStore()

	tests := []struct {
		name   string
		config Config
	}{
		{
			name: "missing AccountSID",
			config: Config{
				Provider:         ProviderTwilio,
				TwilioAuthToken:  "test_token",
				TwilioFromNumber: "+1234567890",
			},
		},
		{
			name: "missing AuthToken",
			config: Config{
				Provider:         ProviderTwilio,
				TwilioAccountSID: "test_sid",
				TwilioFromNumber: "+1234567890",
			},
		},
		{
			name: "missing FromNumber",
			config: Config{
				Provider:         ProviderTwilio,
				TwilioAccountSID: "test_sid",
				TwilioAuthToken:  "test_token",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewProvider(tt.config, logger, rateLimiter, optOutStore)
			if err == nil {
				t.Error("NewProvider() expected error for missing config, got nil")
			}
		})
	}
}

func TestNewProvider_EthioTelecom(t *testing.T) {
	logger := &mockLogger{}
	rateLimiter := NewInMemoryRateLimiter(RateLimiterConfig{})
	optOutStore := NewInMemoryOptOutStore()

	cfg := Config{
		Provider:             ProviderEthioTelecom,
		EthioTelecomAPIURL:   "https://api.example.com",
		EthioTelecomAPIKey:   "test_key",
		EthioTelecomSenderID: "SENDER",
	}

	provider, err := NewProvider(cfg, logger, rateLimiter, optOutStore)
	if err != nil {
		t.Fatalf("NewProvider() error = %v", err)
	}
	if provider == nil {
		t.Fatal("NewProvider() returned nil provider")
	}
	if provider.Name() != "ethiotelecom" {
		t.Errorf("provider.Name() = %q, want %q", provider.Name(), "ethiotelecom")
	}
}

func TestNewProvider_EthioTelecomMissingConfig(t *testing.T) {
	logger := &mockLogger{}
	rateLimiter := NewInMemoryRateLimiter(RateLimiterConfig{})
	optOutStore := NewInMemoryOptOutStore()

	tests := []struct {
		name   string
		config Config
	}{
		{
			name: "missing APIURL",
			config: Config{
				Provider:           ProviderEthioTelecom,
				EthioTelecomAPIKey: "test_key",
			},
		},
		{
			name: "missing APIKey",
			config: Config{
				Provider:           ProviderEthioTelecom,
				EthioTelecomAPIURL: "https://api.example.com",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewProvider(tt.config, logger, rateLimiter, optOutStore)
			if err == nil {
				t.Error("NewProvider() expected error for missing config, got nil")
			}
		})
	}
}

func TestNewProvider_Mock(t *testing.T) {
	logger := &mockLogger{}
	rateLimiter := NewInMemoryRateLimiter(RateLimiterConfig{})
	optOutStore := NewInMemoryOptOutStore()

	cfg := Config{
		Provider: ProviderMock,
	}

	provider, err := NewProvider(cfg, logger, rateLimiter, optOutStore)
	if err != nil {
		t.Fatalf("NewProvider() error = %v", err)
	}
	if provider == nil {
		t.Fatal("NewProvider() returned nil provider")
	}
	if provider.Name() != "mock" {
		t.Errorf("provider.Name() = %q, want %q", provider.Name(), "mock")
	}
}

func TestNewProvider_UnknownProvider(t *testing.T) {
	logger := &mockLogger{}
	rateLimiter := NewInMemoryRateLimiter(RateLimiterConfig{})
	optOutStore := NewInMemoryOptOutStore()

	cfg := Config{
		Provider: ProviderType("unknown"),
	}

	_, err := NewProvider(cfg, logger, rateLimiter, optOutStore)
	if err == nil {
		t.Error("NewProvider() expected error for unknown provider, got nil")
	}
}

type mockLogger struct{}

func (m *mockLogger) Info(msg string, fields ...interface{})  {}
func (m *mockLogger) Error(msg string, fields ...interface{}) {}
func (m *mockLogger) Warn(msg string, fields ...interface{})  {}
func (m *mockLogger) Debug(msg string, fields ...interface{}) {}
