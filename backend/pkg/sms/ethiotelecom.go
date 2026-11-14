package sms

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// EthioTelecomProvider sends through an HTTP SMS gateway: a token from
// POST /auth/token, then POST /sms/send for each message. Ethio telecom
// publishes no HTTP API, so these calls are Menged's side of the contract,
// to be matched to the one signed with the gateway. Delivery reports and
// texts from phones come back through Menged's webhooks.
//
// Each Send is one attempt, retried once with a new token when the gateway
// says the token is no longer valid. Other retries are the outbox's.
type EthioTelecomProvider struct {
	apiURL      string
	apiKey      string
	senderID    string
	client      *http.Client
	logger      Logger
	token       string
	tokenExpiry time.Time
	tokenMutex  sync.Mutex
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

func NewEthioTelecomProvider(apiURL, apiKey, senderID string, logger Logger) *EthioTelecomProvider {
	return &EthioTelecomProvider{
		apiURL:   strings.TrimRight(apiURL, "/"),
		apiKey:   apiKey,
		senderID: senderID,
		client:   &http.Client{Timeout: 25 * time.Second},
		logger:   logger,
	}
}

func (e *EthioTelecomProvider) Name() string {
	return "ethiotelecom"
}

// Send hands a message to the gateway and returns the gateway's ID for it.
func (e *EthioTelecomProvider) Send(ctx context.Context, to, message string, msgType MessageType) (string, error) {
	phone, err := NormalizeEthiopianPhone(to)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrRejected, err)
	}
	id, err := e.send(ctx, phone, message, false)
	if errors.Is(err, errTokenRefused) {
		id, err = e.send(ctx, phone, message, true)
	}
	if err != nil {
		return "", err
	}
	e.logger.Debug("SMS accepted", "message_id", id, "type", msgType, "provider", "ethiotelecom")
	return id, nil
}

// errTokenRefused is the gateway answering 401 to a message.
var errTokenRefused = errors.New("the gateway refused the token")

func (e *EthioTelecomProvider) send(ctx context.Context, to, message string, freshToken bool) (string, error) {
	token, err := e.ensureToken(ctx, freshToken)
	if err != nil {
		return "", err
	}
	body, err := json.Marshal(ethioTelecomSMSRequest{To: to, From: e.senderID, Message: message})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.apiURL+"/sms/send", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := e.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("SMS gateway: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))

	if resp.StatusCode == http.StatusUnauthorized {
		return "", errTokenRefused
	}
	if err := statusError(resp.StatusCode, respBody); err != nil {
		return "", err
	}
	var result ethioTelecomSMSResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("SMS gateway: unreadable answer: %w", err)
	}
	switch result.Status {
	case "success", "accepted", "queued":
		return result.MessageID, nil
	case "rejected", "invalid":
		return "", fmt.Errorf("%w: %s", ErrRejected, result.Error)
	}
	return "", fmt.Errorf("SMS gateway: status %q: %s", result.Status, result.Error)
}

// ensureToken returns a token valid for a few more minutes, asking for a
// new one when there is none, it is about to expire, or fresh is set.
func (e *EthioTelecomProvider) ensureToken(ctx context.Context, fresh bool) (string, error) {
	e.tokenMutex.Lock()
	defer e.tokenMutex.Unlock()
	if !fresh && e.token != "" && time.Now().Before(e.tokenExpiry.Add(-time.Minute)) {
		return e.token, nil
	}

	body, err := json.Marshal(ethioTelecomAuthRequest{APIKey: e.apiKey})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.apiURL+"/auth/token", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := e.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("SMS gateway login: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode != http.StatusOK {
		// A refused key is the deployment's to fix, but the messages
		// should wait for it rather than be dropped.
		return "", fmt.Errorf("SMS gateway login: status %d: %s", resp.StatusCode, excerpt(respBody))
	}
	var auth ethioTelecomAuthResponse
	if err := json.Unmarshal(respBody, &auth); err != nil || auth.Token == "" {
		return "", fmt.Errorf("SMS gateway login: no token in the answer")
	}
	e.token = auth.Token
	e.tokenExpiry = time.Now().Add(time.Duration(auth.ExpiresIn) * time.Second)
	e.logger.Info("signed in to the SMS gateway", "expires_in", auth.ExpiresIn)
	return e.token, nil
}

// statusError classifies a gateway's HTTP answer: success, a message it
// refuses for good (ErrRejected), or a failure worth trying again.
func statusError(status int, body []byte) error {
	switch {
	case status < 300:
		return nil
	case status == http.StatusRequestTimeout, status == http.StatusTooManyRequests, status >= 500:
		return fmt.Errorf("SMS gateway: status %d: %s", status, excerpt(body))
	default:
		return fmt.Errorf("%w: status %d: %s", ErrRejected, status, excerpt(body))
	}
}

func excerpt(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 200 {
		s = s[:200] + "…"
	}
	return s
}
