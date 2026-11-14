package sms

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type TwilioProvider struct {
	accountSID string
	authToken  string
	fromNumber string
	apiURL     string
	client     *http.Client
	logger     Logger
}

func NewTwilioProvider(accountSID, authToken, fromNumber string, logger Logger) *TwilioProvider {
	return &TwilioProvider{
		accountSID: accountSID,
		authToken:  authToken,
		fromNumber: fromNumber,
		apiURL:     fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%s/Messages.json", accountSID),
		client:     &http.Client{Timeout: 25 * time.Second},
		logger:     logger,
	}
}

func (t *TwilioProvider) Name() string {
	return "twilio"
}

// Send hands a message to Twilio and returns its SID. Each call is one
// attempt; retries are the outbox's.
func (t *TwilioProvider) Send(ctx context.Context, to, message string, msgType MessageType) (string, error) {
	phone, err := NormalizeEthiopianPhone(to)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrRejected, err)
	}
	sid, err := t.doSend(ctx, phone, message)
	if err != nil {
		return "", err
	}
	t.logger.Debug("SMS accepted", "message_id", sid, "type", msgType, "provider", "twilio")
	return sid, nil
}

func (t *TwilioProvider) doSend(ctx context.Context, to, message string) (string, error) {
	data := url.Values{}
	data.Set("To", to)
	data.Set("From", t.fromNumber)
	data.Set("Body", message)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.apiURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.SetBasicAuth(t.accountSID, t.authToken)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := t.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("twilio: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if err != nil {
		return "", fmt.Errorf("twilio: failed to read response: %w", err)
	}

	if resp.StatusCode == http.StatusUnauthorized {
		// The account's to fix; the messages wait for it.
		return "", errors.New("twilio: the account credentials were refused")
	}
	if resp.StatusCode >= 300 {
		var twilioErr struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		}
		if json.Unmarshal(body, &twilioErr) == nil && twilioErr.Message != "" {
			body = fmt.Appendf(nil, "twilio error %d: %s", twilioErr.Code, twilioErr.Message)
		}
		return "", statusError(resp.StatusCode, body)
	}

	var result struct {
		Sid          string `json:"sid"`
		Status       string `json:"status"`
		ErrorCode    int    `json:"error_code"`
		ErrorMessage string `json:"error_message"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("twilio: failed to parse response: %w", err)
	}

	if result.ErrorCode != 0 {
		return "", fmt.Errorf("%w: twilio error %d: %s", ErrRejected, result.ErrorCode, result.ErrorMessage)
	}

	return result.Sid, nil
}
