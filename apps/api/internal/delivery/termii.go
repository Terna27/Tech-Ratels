package delivery

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"ratels-api/internal/security"
)

var (
	ErrTermiiNotConfigured = errors.New("termii API key or sender ID is not configured")
	ErrSMSDeliveryDisabled = errors.New("SMS delivery is disabled in current environment")
)

type SMSSender interface {
	SendSMS(ctx context.Context, phone, message string) (messageID string, err error)
	ProviderName() string
	IsConfigured() bool
}

type TermiiDeliveryAdapter struct {
	apiKey     string
	senderID   string
	channel    string
	baseURL    string
	httpClient *http.Client
}

func NewTermiiDeliveryAdapter(apiKey, senderID string) *TermiiDeliveryAdapter {
	if senderID == "" {
		senderID = "RATELS"
	}
	return &TermiiDeliveryAdapter{
		apiKey:     strings.TrimSpace(apiKey),
		senderID:   strings.TrimSpace(senderID),
		channel:    "generic",
		baseURL:    "https://api.ng.termii.com/api/sms/send",
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (t *TermiiDeliveryAdapter) WithBaseURL(url string) *TermiiDeliveryAdapter {
	t.baseURL = strings.TrimSpace(url)
	return t
}

func (t *TermiiDeliveryAdapter) WithChannel(channel string) *TermiiDeliveryAdapter {
	if channel != "" {
		t.channel = strings.TrimSpace(channel)
	}
	return t
}

func (t *TermiiDeliveryAdapter) ProviderName() string {
	return "termii"
}

func (t *TermiiDeliveryAdapter) IsConfigured() bool {
	return t.apiKey != "" && t.senderID != ""
}

func (t *TermiiDeliveryAdapter) WithHTTPClient(client *http.Client) *TermiiDeliveryAdapter {
	if client != nil {
		t.httpClient = client
	}
	return t
}

type termiiSendRequest struct {
	To      string `json:"to"`
	From    string `json:"from"`
	SMS     string `json:"sms"`
	Type    string `json:"type"`
	Channel string `json:"channel"`
	APIKey  string `json:"api_key"`
}

type termiiSuccessResponse struct {
	MessageID string  `json:"message_id"`
	Message   string  `json:"message"`
	Balance   float64 `json:"balance,omitempty"`
	User      string  `json:"user,omitempty"`
}

type termiiErrorResponse struct {
	Message string `json:"message"`
	Error   string `json:"error,omitempty"`
}

func (t *TermiiDeliveryAdapter) SendSMS(ctx context.Context, phone, message string) (string, error) {
	if t.apiKey == "" || t.senderID == "" {
		return "", ErrTermiiNotConfigured
	}

	_, providerPhone, err := NormalizeNigerianPhone(phone)
	if err != nil {
		return "", err
	}

	payload := termiiSendRequest{
		To:      providerPhone,
		From:    t.senderID,
		SMS:     strings.TrimSpace(message),
		Type:    "plain",
		Channel: t.channel,
		APIKey:  t.apiKey,
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal termii payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.baseURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("failed to create termii request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		sanitizedErr := security.RedactString(err.Error(), t.apiKey)
		if errors.Is(err, context.DeadlineExceeded) || strings.Contains(strings.ToLower(err.Error()), "timeout") {
			return "", fmt.Errorf("termii request timed out: %s", sanitizedErr)
		}
		return "", fmt.Errorf("termii request failed: %s", sanitizedErr)
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusAccepted {
		var succ termiiSuccessResponse
		if err := json.Unmarshal(respBytes, &succ); err == nil && succ.MessageID != "" {
			log.Printf("[TERMII_DELIVERY] SMS accepted by provider for %s (messageId: %s)", phone, succ.MessageID)
			return succ.MessageID, nil
		}
		return fmt.Sprintf("termii-%d", time.Now().UnixNano()), nil
	}

	var errResp termiiErrorResponse
	_ = json.Unmarshal(respBytes, &errResp)
	errMsg := strings.TrimSpace(errResp.Message)
	if errMsg == "" {
		errMsg = strings.TrimSpace(errResp.Error)
	}
	if errMsg == "" {
		errMsg = strings.TrimSpace(string(respBytes))
	}
	errMsg = security.RedactString(errMsg, t.apiKey)

	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return "", fmt.Errorf("termii authentication failed: invalid or expired API key (HTTP 401: %s)", errMsg)
	case http.StatusBadRequest:
		return "", fmt.Errorf("termii rejected parameter (HTTP 400): %s", errMsg)
	case http.StatusForbidden:
		return "", fmt.Errorf("termii forbidden or sender ID restricted (HTTP 403): %s", errMsg)
	case http.StatusTooManyRequests:
		return "", fmt.Errorf("termii rate limit exceeded (HTTP 429): %s", errMsg)
	default:
		return "", fmt.Errorf("termii SMS delivery failed (HTTP %d): %s", resp.StatusCode, errMsg)
	}
}

// DevSafeSMSSender provides a safe in-memory SMS mock for development and test execution.
type DevSafeSMSSender struct {
	mu           sync.RWMutex
	sentMessages []SentSMSMessage
	SimulateFail bool
}

type SentSMSMessage struct {
	Phone     string
	Message   string
	MessageID string
	SentAt    time.Time
}

func NewDevSafeSMSSender() *DevSafeSMSSender {
	return &DevSafeSMSSender{
		sentMessages: make([]SentSMSMessage, 0),
	}
}

func (d *DevSafeSMSSender) SendSMS(ctx context.Context, phone, message string) (string, error) {
	if d.SimulateFail {
		return "", errors.New("simulated SMS provider delivery failure")
	}

	_, providerPhone, err := NormalizeNigerianPhone(phone)
	if err != nil {
		return "", err
	}

	msgID := fmt.Sprintf("dev-sms-%d", time.Now().UnixNano())

	d.mu.Lock()
	d.sentMessages = append(d.sentMessages, SentSMSMessage{
		Phone:     providerPhone,
		Message:   message,
		MessageID: msgID,
		SentAt:    time.Now(),
	})
	d.mu.Unlock()

	log.Printf("[DEV_SMS_SAFE_MODE] Dispatched SMS to %s (%s): %s", phone, providerPhone, message)
	return msgID, nil
}

func (d *DevSafeSMSSender) ProviderName() string {
	return "dev_sandbox"
}

func (d *DevSafeSMSSender) IsConfigured() bool {
	return true
}

func (d *DevSafeSMSSender) GetSentMessages() []SentSMSMessage {
	d.mu.RLock()
	defer d.mu.RUnlock()
	res := make([]SentSMSMessage, len(d.sentMessages))
	copy(res, d.sentMessages)
	return res
}

// UnconfiguredSMSSender is used when no SMS provider is configured in production.
type UnconfiguredSMSSender struct{}

func NewUnconfiguredSMSSender() *UnconfiguredSMSSender {
	return &UnconfiguredSMSSender{}
}

func (u *UnconfiguredSMSSender) SendSMS(ctx context.Context, phone, message string) (string, error) {
	return "", ErrTermiiNotConfigured
}

func (u *UnconfiguredSMSSender) ProviderName() string {
	return "unconfigured"
}

func (u *UnconfiguredSMSSender) IsConfigured() bool {
	return false
}
