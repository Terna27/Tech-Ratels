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
	"time"

	"ratels-api/internal/security"
)

var (
	ErrBrevoNotConfigured = errors.New("brevo API key or sender email is not configured")
)

type BrevoDeliveryAdapter struct {
	apiKey      string
	fromName    string
	fromAddress string
	baseURL     string
	httpClient  *http.Client
}

func NewBrevoDeliveryAdapter(apiKey, fromName, fromAddress string) *BrevoDeliveryAdapter {
	if fromName == "" {
		fromName = "Nigerian Ratels"
	}
	return &BrevoDeliveryAdapter{
		apiKey:      strings.TrimSpace(apiKey),
		fromName:    strings.TrimSpace(fromName),
		fromAddress: strings.TrimSpace(fromAddress),
		baseURL:     "https://api.brevo.com/v3/smtp/email",
		httpClient:  &http.Client{Timeout: 10 * time.Second},
	}
}

// WithBaseURL allows overriding the Brevo API endpoint for testing with mock servers.
func (b *BrevoDeliveryAdapter) WithBaseURL(url string) *BrevoDeliveryAdapter {
	b.baseURL = strings.TrimSpace(url)
	return b
}

// WithHTTPClient allows injecting a custom http.Client (e.g. for testing custom timeouts or transport).
func (b *BrevoDeliveryAdapter) WithHTTPClient(client *http.Client) *BrevoDeliveryAdapter {
	if client != nil {
		b.httpClient = client
	}
	return b
}

type brevoSender struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type brevoRecipient struct {
	Email string `json:"email"`
	Name  string `json:"name,omitempty"`
}

type brevoEmailRequest struct {
	Sender      brevoSender      `json:"sender"`
	To          []brevoRecipient `json:"to"`
	Subject     string           `json:"subject"`
	HTMLContent string           `json:"htmlContent"`
	TextContent string           `json:"textContent"`
}

type brevoSuccessResponse struct {
	MessageID string `json:"messageId"`
}

type brevoErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (b *BrevoDeliveryAdapter) SendVerificationCode(identifier, code string) error {
	// SMS / OTP gateway is decoupled from transactional email
	return nil
}

func (b *BrevoDeliveryAdapter) SendVerificationEmail(recipientEmail, recipientName, verificationURL string) error {
	if b.apiKey == "" || b.fromAddress == "" {
		return ErrBrevoNotConfigured
	}

	subject := "Verify your Nigerian Ratels account"
	htmlBody := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Verify your Nigerian Ratels account</title>
</head>
<body style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif; background-color: #f8fafc; margin: 0; padding: 24px; color: #1e293b;">
  <div style="max-width: 600px; margin: 0 auto; background-color: #ffffff; border-radius: 12px; border: 1px solid #e2e8f0; padding: 32px; box-shadow: 0 1px 3px rgba(0,0,0,0.05);">
    <div style="text-align: center; margin-bottom: 24px;">
      <h2 style="color: #064e3b; margin: 0; font-size: 24px; font-weight: 800;">Nigerian Ratels Movement</h2>
      <p style="color: #047857; margin: 4px 0 0 0; font-size: 14px; font-weight: 600;">Martins Vincent Otse Foundation</p>
    </div>
    <div style="border-top: 1px solid #e2e8f0; padding-top: 24px;">
      <p style="font-size: 16px; line-height: 1.6; margin: 0 0 16px 0;">Hello %s,</p>
      <p style="font-size: 16px; line-height: 1.6; margin: 0 0 24px 0;">
        Thank you for joining the Nigerian Ratels Movement. Please verify your email address to activate your membership account and access your member dashboard.
      </p>
      <div style="text-align: center; margin: 32px 0;">
        <a href="%s" style="background-color: #047857; color: #ffffff; padding: 14px 28px; font-size: 16px; font-weight: 700; text-decoration: none; border-radius: 8px; display: inline-block;">
          Verify My Email Address
        </a>
      </div>
      <p style="font-size: 14px; line-height: 1.6; color: #64748b; margin: 0 0 16px 0;">
        This verification link expires in 24 hours and can only be used once.
      </p>
      <p style="font-size: 14px; line-height: 1.6; color: #64748b; margin: 0 0 24px 0;">
        If you did not create an account with the Nigerian Ratels Movement, you can safely disregard this email.
      </p>
      <div style="border-top: 1px solid #e2e8f0; padding-top: 16px; font-size: 12px; color: #94a3b8;">
        <p style="margin: 0;">If the button above does not work, copy and paste this link into your browser:</p>
        <p style="word-break: break-all; color: #047857; margin: 4px 0 0 0;">%s</p>
      </div>
    </div>
  </div>
</body>
</html>`, recipientName, verificationURL, verificationURL)

	textBody := fmt.Sprintf("Hello %s,\n\nThank you for joining the Nigerian Ratels Movement (Martins Vincent Otse Foundation).\n\nPlease verify your email address by visiting this link:\n%s\n\nThis verification link expires in 24 hours and can only be used once.\n\nIf you did not register for an account, please disregard this email.\n", recipientName, verificationURL)

	payload := brevoEmailRequest{
		Sender: brevoSender{
			Name:  b.fromName,
			Email: b.fromAddress,
		},
		To: []brevoRecipient{
			{
				Email: recipientEmail,
				Name:  recipientName,
			},
		},
		Subject:     subject,
		HTMLContent: htmlBody,
		TextContent: textBody,
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal brevo email payload: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, b.baseURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("failed to create brevo request: %w", err)
	}

	req.Header.Set("accept", "application/json")
	req.Header.Set("content-type", "application/json")
	req.Header.Set("api-key", b.apiKey)

	resp, err := b.httpClient.Do(req)
	if err != nil {
		sanitizedErr := security.RedactString(err.Error(), b.apiKey)
		if errors.Is(err, context.DeadlineExceeded) || strings.Contains(strings.ToLower(err.Error()), "timeout") {
			return fmt.Errorf("brevo request timed out: %s", sanitizedErr)
		}
		return fmt.Errorf("brevo request failed: %s", sanitizedErr)
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))

	// Status 201 Created indicates the email was accepted by Brevo for dispatch.
	// We distinguish provider acceptance from confirmed inbox delivery.
	if resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusAccepted {
		var succ brevoSuccessResponse
		_ = json.Unmarshal(respBytes, &succ)
		log.Printf("[BREVO_DELIVERY] Verification email accepted by provider for %s (messageId: %s; Note: provider acceptance indicates message is queued/dispatched, not guaranteed inbox delivery)", recipientEmail, succ.MessageID)
		return nil
	}

	var brevoErr brevoErrorResponse
	_ = json.Unmarshal(respBytes, &brevoErr)
	errMsg := strings.TrimSpace(brevoErr.Message)
	if errMsg == "" {
		errMsg = strings.TrimSpace(string(respBytes))
	}
	errMsg = security.RedactString(errMsg, b.apiKey)

	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return fmt.Errorf("brevo authentication failed: invalid, expired, or missing API key (HTTP 401: %s)", errMsg)
	case http.StatusForbidden:
		return fmt.Errorf("brevo sender rejected or account restricted (HTTP 403 %s): %s", brevoErr.Code, errMsg)
	case http.StatusBadRequest:
		return fmt.Errorf("brevo rejected request parameter (HTTP 400 %s): %s", brevoErr.Code, errMsg)
	case http.StatusTooManyRequests:
		return fmt.Errorf("brevo rate limit exceeded (HTTP 429): %s", errMsg)
	default:
		return fmt.Errorf("brevo email delivery failed (HTTP %d): %s", resp.StatusCode, errMsg)
	}
}

func (b *BrevoDeliveryAdapter) GetLastCode(identifier string) string {
	return ""
}

func (b *BrevoDeliveryAdapter) GetLastVerificationURL(identifier string) string {
	return ""
}
