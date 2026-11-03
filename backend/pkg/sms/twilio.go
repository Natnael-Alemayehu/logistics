package sms

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type TwilioProvider struct {
	accountSID string
	authToken  string
	fromNumber string
	apiURL     string
	client     *http.Client
	logger     Logger
	mutex      sync.RWMutex
}

func NewTwilioProvider(accountSID, authToken, fromNumber string, logger Logger) *TwilioProvider {
	return &TwilioProvider{
		accountSID: accountSID,
		authToken:  authToken,
		fromNumber: fromNumber,
		apiURL:     fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%s/Messages.json", accountSID),
		client:     &http.Client{Timeout: 30 * time.Second},
		logger:     logger,
	}
}

func (t *TwilioProvider) Name() string {
	return "twilio"
}

func (t *TwilioProvider) Send(ctx context.Context, to, message string, msgType MessageType) error {
	normalizedPhone, err := NormalizeEthiopianPhone(to)
	if err != nil {
		t.logger.Error("invalid phone number", "phone", to, "error", err)
		return err
	}

	msgID, err := t.sendWithRetry(ctx, normalizedPhone, message, 3)
	if err != nil {
		t.logger.Error("failed to send SMS", "phone", normalizedPhone, "error", err, "type", msgType)
		return err
	}

	t.logger.Info("SMS sent successfully", "message_id", msgID, "phone", normalizedPhone, "type", msgType, "provider", "twilio")
	return nil
}

func (t *TwilioProvider) SendBatch(ctx context.Context, recipients []string, message string, msgType MessageType) error {
	var errors []error
	for _, recipient := range recipients {
		if err := t.Send(ctx, recipient, message, msgType); err != nil {
			errors = append(errors, fmt.Errorf("%s: %w", recipient, err))
		}
	}
	if len(errors) > 0 {
		return fmt.Errorf("batch send completed with %d errors: %v", len(errors), errors)
	}
	return nil
}

func (t *TwilioProvider) sendWithRetry(ctx context.Context, to, message string, maxRetries int) (string, error) {
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		if i > 0 {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(time.Duration(i*i) * time.Second):
			}
		}

		msgID, err := t.doSend(ctx, to, message)
		if err == nil {
			return msgID, nil
		}
		lastErr = err
		t.logger.Warn("SMS send attempt failed, retrying", "attempt", i+1, "error", err)
	}
	return "", fmt.Errorf("failed after %d retries: %w", maxRetries, lastErr)
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
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		var twilioErr struct {
			Code     int    `json:"code"`
			Message  string `json:"message"`
			MoreInfo string `json:"more_info"`
		}
		if err := json.Unmarshal(body, &twilioErr); err == nil {
			return "", fmt.Errorf("twilio error %d: %s", twilioErr.Code, twilioErr.Message)
		}
		return "", fmt.Errorf("twilio API error: status %d, body: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Sid          string `json:"sid"`
		Status       string `json:"status"`
		ErrorCode    int    `json:"error_code"`
		ErrorMessage string `json:"error_message"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if result.ErrorCode != 0 {
		return "", fmt.Errorf("twilio error %d: %s", result.ErrorCode, result.ErrorMessage)
	}

	return result.Sid, nil
}

func (t *TwilioProvider) CheckStatus(ctx context.Context, messageSid string) (string, error) {
	url := fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%s/Messages/%s.json", t.accountSID, messageSid)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}

	req.SetBasicAuth(t.accountSID, t.authToken)

	resp, err := t.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("status check failed: %s", string(body))
	}

	var result struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}

	return result.Status, nil
}

func isTwilioErrorRetryable(code int) bool {
	retryableCodes := map[int]bool{
		20429: true,
		20491: true,
		21603: false,
		21614: false,
		21612: false,
	}
	if retryable, exists := retryableCodes[code]; exists {
		return retryable
	}
	return code >= 500
}
