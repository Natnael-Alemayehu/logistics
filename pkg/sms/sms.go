package sms

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type MessageType string

const (
	MessageTypeOTP          MessageType = "otp"
	MessageTypeNotification MessageType = "notification"
	MessageTypeAlert        MessageType = "alert"
)

type ProviderType string

const (
	ProviderTwilio       ProviderType = "twilio"
	ProviderEthioTelecom ProviderType = "ethiotelecom"
	ProviderMock         ProviderType = "mock"
)

var (
	ErrInvalidPhone        = errors.New("invalid phone number format")
	ErrOptedOut            = errors.New("recipient has opted out")
	ErrRateLimited         = errors.New("rate limit exceeded")
	ErrProviderUnavailable = errors.New("SMS provider unavailable")
)

type SMSProvider interface {
	Send(ctx context.Context, to, message string, msgType MessageType) error
	SendBatch(ctx context.Context, recipients []string, message string, msgType MessageType) error
	Name() string
}

type Message struct {
	ID         string
	To         string
	Message    string
	Type       MessageType
	Status     string
	Provider   string
	SentAt     time.Time
	Error      string
	RetryCount int
}

type Config struct {
	Provider             ProviderType
	TwilioAccountSID     string
	TwilioAuthToken      string
	TwilioFromNumber     string
	EthioTelecomAPIURL   string
	EthioTelecomAPIKey   string
	EthioTelecomSenderID string
}

func NewProvider(cfg Config, logger Logger, rateLimiter RateLimiter, optOutStore OptOutStore) (SMSProvider, error) {
	switch cfg.Provider {
	case ProviderTwilio:
		if cfg.TwilioAccountSID == "" || cfg.TwilioAuthToken == "" || cfg.TwilioFromNumber == "" {
			return nil, errors.New("missing Twilio configuration")
		}
		return NewTwilioProvider(cfg.TwilioAccountSID, cfg.TwilioAuthToken, cfg.TwilioFromNumber, logger, rateLimiter, optOutStore), nil
	case ProviderEthioTelecom:
		if cfg.EthioTelecomAPIURL == "" || cfg.EthioTelecomAPIKey == "" {
			return nil, errors.New("missing Ethio Telecom configuration")
		}
		return NewEthioTelecomProvider(cfg.EthioTelecomAPIURL, cfg.EthioTelecomAPIKey, cfg.EthioTelecomSenderID, logger, rateLimiter, optOutStore), nil
	case ProviderMock:
		return NewMockProvider(logger), nil
	default:
		return nil, fmt.Errorf("unknown SMS provider: %s", cfg.Provider)
	}
}

func NormalizeEthiopianPhone(phone string) (string, error) {
	if len(phone) == 0 {
		return "", ErrInvalidPhone
	}

	if len(phone) >= 4 && phone[:4] == "+251" {
		if len(phone) != 13 {
			return "", ErrInvalidPhone
		}
		return phone, nil
	}

	if len(phone) >= 3 && phone[:3] == "251" {
		if len(phone) != 12 {
			return "", ErrInvalidPhone
		}
		return "+" + phone, nil
	}

	if len(phone) == 10 && (phone[:2] == "09" || phone[:2] == "07") {
		return "+251" + phone[1:], nil
	}

	if len(phone) == 9 && (phone[0] == '9' || phone[0] == '7') {
		return "+251" + phone, nil
	}

	return "", ErrInvalidPhone
}

type Logger interface {
	Info(msg string, fields ...interface{})
	Error(msg string, fields ...interface{})
	Warn(msg string, fields ...interface{})
	Debug(msg string, fields ...interface{})
}

type RateLimiter interface {
	Allow(phone string) bool
	Record(phone string)
}

type OptOutStore interface {
	IsOptedOut(phone string) bool
	AddOptOut(phone string)
	RemoveOptOut(phone string)
}
