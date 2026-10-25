package delivery

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

var (
	ErrResendNotConfigured = errors.New("resend API key is not configured")
	ErrDeliveryFailed      = errors.New("failed to dispatch verification email via provider")
)

type ResendDeliveryAdapter struct {
	apiKey     string
	fromEmail  string
	httpClient *http.Client
}

func NewResendDeliveryAdapter(apiKey, fromEmail string) *ResendDeliveryAdapter {
	if fromEmail == "" {
		fromEmail = "Martins Vincent Otse Foundation <onboarding@resend.dev>"
	}
	return &ResendDeliveryAdapter{
		apiKey:     strings.TrimSpace(apiKey),
		fromEmail:  strings.TrimSpace(fromEmail),
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

type resendEmailRequest struct {
	From    string   `json:"from"`
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	HTML    string   `json:"html"`
	Text    string   `json:"text"`
}

type resendErrorResponse struct {
	StatusCode int    `json:"statusCode"`
	Message    string `json:"message"`
	Name       string `json:"name"`
}

func (r *ResendDeliveryAdapter) SendVerificationCode(identifier, code string) error {
	// SMS / OTP gateway not configured on Resend
	return nil
}

func (r *ResendDeliveryAdapter) SendVerificationEmail(recipientEmail, recipientName, verificationURL string) error {
	if r.apiKey == "" {
		return ErrResendNotConfigured
	}

	subject := "Verify your Ratels Worldwide account"
	htmlBody := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Verify your Ratels Worldwide account</title>
</head>
<body style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif; background-color: #f8fafc; margin: 0; padding: 24px; color: #1e293b;">
  <div style="max-width: 600px; margin: 0 auto; background-color: #ffffff; border-radius: 12px; border: 1px solid #e2e8f0; padding: 32px; box-shadow: 0 1px 3px rgba(0,0,0,0.05);">
    <div style="text-align: center; margin-bottom: 24px;">
      <h2 style="color: #064e3b; margin: 0; font-size: 24px; font-weight: 800;">Ratels Worldwide Movement</h2>
      <p style="color: #047857; margin: 4px 0 0 0; font-size: 14px; font-weight: 600;">Martins Vincent Otse Foundation</p>
    </div>
    <div style="border-top: 1px solid #e2e8f0; padding-top: 24px;">
      <p style="font-size: 16px; line-height: 1.6; margin: 0 0 16px 0;">Hello %s,</p>
      <p style="font-size: 16px; line-height: 1.6; margin: 0 0 24px 0;">
        Thank you for joining the Ratels Worldwide Movement. Please verify your email address to activate your membership account and access your member dashboard.
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
        If you did not create an account with the Ratels Worldwide Movement, you can safely disregard this email.
      </p>
      <div style="border-top: 1px solid #e2e8f0; padding-top: 16px; font-size: 12px; color: #94a3b8;">
        <p style="margin: 0;">If the button above does not work, copy and paste this link into your browser:</p>
        <p style="word-break: break-all; color: #047857; margin: 4px 0 0 0;">%s</p>
      </div>
    </div>
  </div>
</body>
</html>`, recipientName, verificationURL, verificationURL)

	textBody := fmt.Sprintf("Hello %s,\n\nThank you for joining the Ratels Worldwide Movement (Martins Vincent Otse Foundation).\n\nPlease verify your email address by visiting this link:\n%s\n\nThis verification link expires in 24 hours and can only be used once.\n\nIf you did not register for an account, please disregard this email.\n", recipientName, verificationURL)

	payload := resendEmailRequest{
		From:    r.fromEmail,
		To:      []string{recipientEmail},
		Subject: subject,
		HTML:    htmlBody,
		Text:    textBody,
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal email payload: %w", err)
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "https://api.resend.com/emails", bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("failed to create resend request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+r.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("resend request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		var resendErr resendErrorResponse
		_ = json.Unmarshal(respBytes, &resendErr)
		errMsg := resendErr.Message
		if errMsg == "" {
			errMsg = string(respBytes)
		}
		return fmt.Errorf("resend email rejected (HTTP %d): %s", resp.StatusCode, errMsg)
	}

	return nil
}

func (r *ResendDeliveryAdapter) GetLastCode(identifier string) string {
	return ""
}

func (r *ResendDeliveryAdapter) GetLastVerificationURL(identifier string) string {
	return ""
}
