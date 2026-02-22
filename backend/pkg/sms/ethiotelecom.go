package sms

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

type EthioTelecomProvider struct {
	apiURL      string
	apiKey      string
	senderID    string
	client      *http.Client
	logger      Logger
	rateLimiter RateLimiter
	optOutStore OptOutStore
	token       string
	tokenExpiry time.Time
	tokenMutex  sync.RWMutex
}

type ethioTelecomAuthRequest struct {
	APIKey string `json:"api_key"`
}

type ethioTelecomAuthResponse struct {
	Token     string `json:"token"`
	ExpiresIn int    `json:"expires_in"`
}

type ethioTelecomSMSRequest struct {
	To      string `json:"to"`
	From    string `json:"from"`
	Message string `json:"message"`
}

type ethioTelecomSMSResponse struct {
	MessageID string `json:"message_id"`
	Status    string `json:"status"`
	Error     string `json:"error,omitempty"`
}

type ethioTelecomBatchRequest struct {
	Messages []ethioTelecomSMSRequest `json:"messages"`
}

type ethioTelecomBatchResponse struct {
	Results []struct {
		To        string `json:"to"`
		MessageID string `json:"message_id"`
		Status    string `json:"status"`
		Error     string `json:"error,omitempty"`
	} `json:"results"`
}

func NewEthioTelecomProvider(apiURL, apiKey, senderID string, logger Logger, rateLimiter RateLimiter, optOutStore OptOutStore) *EthioTelecomProvider {
	return &EthioTelecomProvider{
		apiURL:      apiURL,
		apiKey:      apiKey,
		senderID:    senderID,
		client:      &http.Client{Timeout: 30 * time.Second},
		logger:      logger,
		rateLimiter: rateLimiter,
		optOutStore: optOutStore,
	}
}

func (e *EthioTelecomProvider) Name() string {
	return "ethiotelecom"
}

func (e *EthioTelecomProvider) Send(ctx context.Context, to, message string, msgType MessageType) error {
	normalizedPhone, err := NormalizeEthiopianPhone(to)
	if err != nil {
		e.logger.Error("invalid phone number", "phone", to, "error", err)
		return err
	}

	if e.optOutStore != nil && e.optOutStore.IsOptedOut(normalizedPhone) {
		e.logger.Warn("recipient opted out", "phone", normalizedPhone)
		return ErrOptedOut
	}

	if e.rateLimiter != nil && !e.rateLimiter.Allow(normalizedPhone) {
		e.logger.Warn("rate limit exceeded", "phone", normalizedPhone)
		return ErrRateLimited
	}

	msgID, err := e.sendWithRetry(ctx, normalizedPhone, message, 3)
	if err != nil {
		e.logger.Error("failed to send SMS", "phone", normalizedPhone, "error", err, "type", msgType)
		return err
	}

	if e.rateLimiter != nil {
		e.rateLimiter.Record(normalizedPhone)
	}

	e.logger.Info("SMS sent successfully", "message_id", msgID, "phone", normalizedPhone, "type", msgType, "provider", "ethiotelecom")
	return nil
}

func (e *EthioTelecomProvider) SendBatch(ctx context.Context, recipients []string, message string, msgType MessageType) error {
	var validRecipients []string
	var errors []error

	for _, recipient := range recipients {
		normalizedPhone, err := NormalizeEthiopianPhone(recipient)
		if err != nil {
			errors = append(errors, fmt.Errorf("%s: invalid phone", recipient))
			continue
		}

		if e.optOutStore != nil && e.optOutStore.IsOptedOut(normalizedPhone) {
			e.logger.Warn("recipient opted out, skipping", "phone", normalizedPhone)
			continue
		}

		if e.rateLimiter != nil && !e.rateLimiter.Allow(normalizedPhone) {
			errors = append(errors, fmt.Errorf("%s: rate limited", normalizedPhone))
			continue
		}

		validRecipients = append(validRecipients, normalizedPhone)
	}

	if len(validRecipients) == 0 {
		if len(errors) > 0 {
			return fmt.Errorf("no valid recipients: %v", errors)
		}
		return nil
	}

	if err := e.ensureToken(ctx); err != nil {
		return fmt.Errorf("failed to authenticate: %w", err)
	}

	msgs := make([]ethioTelecomSMSRequest, len(validRecipients))
	for i, phone := range validRecipients {
		msgs[i] = ethioTelecomSMSRequest{
			To:      phone,
			From:    e.senderID,
			Message: message,
		}
	}

	resp, err := e.sendBatchRequest(ctx, msgs)
	if err != nil {
		return fmt.Errorf("batch send failed: %w", err)
	}

	for _, result := range resp.Results {
		if result.Status != "success" && result.Status != "accepted" {
			errors = append(errors, fmt.Errorf("%s: %s", result.To, result.Error))
		}
		if e.rateLimiter != nil {
			e.rateLimiter.Record(result.To)
		}
		e.logger.Info("batch SMS result", "to", result.To, "status", result.Status, "message_id", result.MessageID)
	}

	if len(errors) > 0 {
		return fmt.Errorf("batch send completed with %d errors: %v", len(errors), errors)
	}

	return nil
}

func (e *EthioTelecomProvider) sendWithRetry(ctx context.Context, to, message string, maxRetries int) (string, error) {
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		if i > 0 {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(time.Duration(i*i) * time.Second):
			}
		}

		msgID, err := e.doSend(ctx, to, message)
		if err == nil {
			return msgID, nil
		}
		lastErr = err
		e.logger.Warn("SMS send attempt failed, retrying", "attempt", i+1, "error", err)

		if isAuthError(err) {
			e.invalidateToken()
		}
	}
	return "", fmt.Errorf("failed after %d retries: %w", maxRetries, lastErr)
}

func (e *EthioTelecomProvider) doSend(ctx context.Context, to, message string) (string, error) {
	if err := e.ensureToken(ctx); err != nil {
		return "", err
	}

	reqBody := ethioTelecomSMSRequest{
		To:      to,
		From:    e.senderID,
		Message: message,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.apiURL+"/sms/send", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	e.tokenMutex.RLock()
	req.Header.Set("Authorization", "Bearer "+e.token)
	e.tokenMutex.RUnlock()
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode == 401 {
		e.invalidateToken()
		return "", fmt.Errorf("authentication failed")
	}

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("API error: status %d, body: %s", resp.StatusCode, string(respBody))
	}

	var result ethioTelecomSMSResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if result.Status != "success" && result.Status != "accepted" && result.Status != "queued" {
		return "", fmt.Errorf("send failed: %s", result.Error)
	}

	return result.MessageID, nil
}

func (e *EthioTelecomProvider) sendBatchRequest(ctx context.Context, msgs []ethioTelecomSMSRequest) (*ethioTelecomBatchResponse, error) {
	body, err := json.Marshal(ethioTelecomBatchRequest{Messages: msgs})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.apiURL+"/sms/bulk", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	e.tokenMutex.RLock()
	req.Header.Set("Authorization", "Bearer "+e.token)
	e.tokenMutex.RUnlock()
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode == 401 {
		e.invalidateToken()
		return nil, fmt.Errorf("authentication failed")
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API error: status %d, body: %s", resp.StatusCode, string(respBody))
	}

	var result ethioTelecomBatchResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &result, nil
}

func (e *EthioTelecomProvider) ensureToken(ctx context.Context) error {
	e.tokenMutex.RLock()
	if e.token != "" && time.Now().Before(e.tokenExpiry.Add(-5*time.Minute)) {
		e.tokenMutex.RUnlock()
		return nil
	}
	e.tokenMutex.RUnlock()

	return e.authenticate(ctx)
}

func (e *EthioTelecomProvider) authenticate(ctx context.Context) error {
	e.tokenMutex.Lock()
	defer e.tokenMutex.Unlock()

	if e.token != "" && time.Now().Before(e.tokenExpiry.Add(-5*time.Minute)) {
		return nil
	}

	reqBody := ethioTelecomAuthRequest{APIKey: e.apiKey}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal auth request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.apiURL+"/auth/token", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create auth request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		return fmt.Errorf("auth request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read auth response: %w", err)
	}

	if resp.StatusCode != 200 {
		return fmt.Errorf("auth failed: status %d, body: %s", resp.StatusCode, string(respBody))
	}

	var authResp ethioTelecomAuthResponse
	if err := json.Unmarshal(respBody, &authResp); err != nil {
		return fmt.Errorf("failed to parse auth response: %w", err)
	}

	e.token = authResp.Token
	e.tokenExpiry = time.Now().Add(time.Duration(authResp.ExpiresIn) * time.Second)

	e.logger.Info("authenticated with Ethio Telecom SMS gateway", "expires_in", authResp.ExpiresIn)
	return nil
}

func (e *EthioTelecomProvider) invalidateToken() {
	e.tokenMutex.Lock()
	defer e.tokenMutex.Unlock()
	e.token = ""
	e.tokenExpiry = time.Time{}
}

func (e *EthioTelecomProvider) CheckStatus(ctx context.Context, messageID string) (string, error) {
	if err := e.ensureToken(ctx); err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, e.apiURL+"/sms/status/"+messageID, nil)
	if err != nil {
		return "", err
	}

	e.tokenMutex.RLock()
	req.Header.Set("Authorization", "Bearer "+e.token)
	e.tokenMutex.RUnlock()

	resp, err := e.client.Do(req)
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

func (e *EthioTelecomProvider) HandleOptOutMessage(body string, from string) {
	upperBody := normalizeOptOutBody(body)
	normalized, err := NormalizeEthiopianPhone(from)
	if err != nil {
		return
	}

	if e.optOutStore == nil {
		return
	}

	switch upperBody {
	case "STOP", "UNSUBSCRIBE", "CANCEL", "QUIT":
		e.optOutStore.AddOptOut(normalized)
		e.logger.Info("phone opted out", "phone", normalized)
	case "START", "YES", "UNSTOP", "SUBSCRIBE":
		e.optOutStore.RemoveOptOut(normalized)
		e.logger.Info("phone opted back in", "phone", normalized)
	}
}

func normalizeOptOutBody(body string) string {
	return bytes.NewBufferString(body).String()
}

func isAuthError(err error) bool {
	return err != nil && (err.Error() == "authentication failed" || err.Error() == "token expired")
}
