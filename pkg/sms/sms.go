// Package sms is a package that will be used to handle the text messages logic
package sms

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/natnael-alemayehu/kiflet/pkg/console"
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
	ErrProviderUnavailable = errors.New("SMS provider unavailable")
	// ErrRejected is a message the gateway will never take, such as one to
	// a number it doesn't serve. Sending it again won't help.
	ErrRejected = errors.New("the gateway rejected the message")
)

type SMSProvider interface {
	// Send hands a message to the gateway and returns the gateway's ID for
	// it, which its delivery report names. Each call is one attempt.
	Send(ctx context.Context, to, message string, msgType MessageType) (string, error)
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
	// Console is where the mock provider prints messages.
	Console *console.Console
}

// NewProvider returns the configured provider. Opt-outs, pacing and retries
// across restarts are the caller's: Kiflet's outbox handles them.
func NewProvider(cfg Config, logger Logger) (SMSProvider, error) {
	switch cfg.Provider {
	case ProviderTwilio:
		if cfg.TwilioAccountSID == "" || cfg.TwilioAuthToken == "" || cfg.TwilioFromNumber == "" {
			return nil, errors.New("missing Twilio configuration")
		}
		return NewTwilioProvider(cfg.TwilioAccountSID, cfg.TwilioAuthToken, cfg.TwilioFromNumber, logger), nil
	case ProviderEthioTelecom:
		if cfg.EthioTelecomAPIURL == "" || cfg.EthioTelecomAPIKey == "" {
			return nil, errors.New("missing Ethio Telecom configuration")
		}
		return NewEthioTelecomProvider(cfg.EthioTelecomAPIURL, cfg.EthioTelecomAPIKey, cfg.EthioTelecomSenderID, logger), nil
	case ProviderMock:
		p := NewMockProvider(logger)
		p.Console = cfg.Console
		return p, nil
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
