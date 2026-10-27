package payment

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type PaystackGateway struct {
	secretKey  string
	publicKey  string
	httpClient *http.Client
	baseURL    string
}

func NewPaystackGateway(secretKey, publicKey string) *PaystackGateway {
	return &PaystackGateway{
		secretKey:  secretKey,
		publicKey:  publicKey,
		httpClient: &http.Client{Timeout: 15 * time.Second},
		baseURL:    "https://api.paystack.co",
	}
}

func (p *PaystackGateway) ProviderName() string {
	return "paystack"
}

// SetBaseURL overrides the API base URL (useful for testing against httptest.Server).
func (p *PaystackGateway) SetBaseURL(baseURL string) {
	p.baseURL = strings.TrimRight(baseURL, "/")
}

// SetHTTPClient overrides the internal http.Client (useful for testing bounded timeouts).
func (p *PaystackGateway) SetHTTPClient(client *http.Client) {
	if client != nil {
		p.httpClient = client
	}
}

func (p *PaystackGateway) InitializePayment(ctx context.Context, input InitializePaymentInput) (*InitializePaymentOutput, error) {
	if p.secretKey == "" {
		return nil, ErrUnconfiguredProvider
	}

	payload := map[string]interface{}{
		"email":        input.CustomerEmail,
		"amount":       input.AmountKobo,
		"reference":    input.OrderReference,
		"currency":     input.Currency,
		"callback_url": input.CallbackURL,
	}
	if len(input.Metadata) > 0 {
		payload["metadata"] = input.Metadata
	}

	reqBody, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to encode paystack request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/transaction/initialize", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create paystack request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+p.secretKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("paystack initialize request failed: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read paystack response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("paystack initialize returned unexpected HTTP status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var psResp struct {
		Status  bool   `json:"status"`
		Message string `json:"message"`
		Data    struct {
			AuthorizationURL string `json:"authorization_url"`
			AccessCode       string `json:"access_code"`
			Reference        string `json:"reference"`
		} `json:"data"`
	}

	if err := json.Unmarshal(bodyBytes, &psResp); err != nil {
		return nil, fmt.Errorf("failed to parse paystack response: %w", err)
	}

	if !psResp.Status {
		return nil, fmt.Errorf("paystack initialization error: %s", psResp.Message)
	}

	return &InitializePaymentOutput{
		PaymentReference: psResp.Data.Reference,
		CheckoutURL:      psResp.Data.AuthorizationURL,
		AccessCode:       psResp.Data.AccessCode,
		Provider:         "paystack",
	}, nil
}

func (p *PaystackGateway) VerifyPayment(ctx context.Context, reference string) (*VerifyPaymentOutput, error) {
	if p.secretKey == "" {
		return nil, ErrUnconfiguredProvider
	}
	if strings.TrimSpace(reference) == "" {
		return nil, errors.New("payment reference is required")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/transaction/verify/%s", p.baseURL, reference), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create verify request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+p.secretKey)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("paystack verify request failed: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read verify response: %w", err)
	}

	// Reject unexpected HTTP response status codes
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("paystack verify returned unexpected HTTP status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var psResp struct {
		Status  bool   `json:"status"`
		Message string `json:"message"`
		Data    struct {
			Status      string `json:"status"` // "success", "failed", "abandoned"
			Reference   string `json:"reference"`
			Amount      int64  `json:"amount"`
			Currency    string `json:"currency"`
			PaidAt      string `json:"paid_at"`
			Channel     string `json:"channel"`
			GatewayResp string `json:"gateway_response"`
			Customer    struct {
				Email string `json:"email"`
			} `json:"customer"`
			Refunds []struct {
				ID              int64  `json:"id"`
				RefundReference string `json:"refund_reference"`
				Reference       string `json:"reference"`
				Amount          int64  `json:"amount"`
				Status          string `json:"status"`
			} `json:"refunds"`
		} `json:"data"`
	}

	if err := json.Unmarshal(bodyBytes, &psResp); err != nil {
		return nil, fmt.Errorf("%w: failed to parse verify response: %v", ErrMalformedResponse, err)
	}

	if !psResp.Status {
		return nil, fmt.Errorf("paystack verification returned false: %s", psResp.Message)
	}

	// Reject malformed or incomplete provider responses
	if strings.TrimSpace(psResp.Data.Reference) == "" {
		return nil, fmt.Errorf("%w: missing payment reference in provider response", ErrMalformedResponse)
	}
	if psResp.Data.Amount <= 0 {
		return nil, fmt.Errorf("%w: non-positive amount (%d) in provider response", ErrMalformedResponse, psResp.Data.Amount)
	}
	if strings.TrimSpace(psResp.Data.Currency) == "" {
		return nil, fmt.Errorf("%w: missing currency in provider response", ErrMalformedResponse)
	}
	if strings.TrimSpace(psResp.Data.Status) == "" {
		return nil, fmt.Errorf("%w: missing payment status in provider response", ErrMalformedResponse)
	}

	status := "pending"
	switch psResp.Data.Status {
	case "success":
		status = "successful"
	case "failed", "abandoned":
		status = "failed"
	}

	var paidTime *time.Time
	if psResp.Data.PaidAt != "" {
		if pt, err := time.Parse(time.RFC3339, psResp.Data.PaidAt); err == nil {
			paidTime = &pt
		}
	}

	var refundSummaries []TransactionRefundSummary
	for _, r := range psResp.Data.Refunds {
		refRef := r.RefundReference
		if refRef == "" {
			refRef = r.Reference
		}
		rStatus := "pending"
		switch r.Status {
		case "processed", "success", "successful":
			rStatus = "successful"
		case "failed":
			rStatus = "failed"
		}
		refundSummaries = append(refundSummaries, TransactionRefundSummary{
			RefundReference: refRef,
			AmountKobo:      r.Amount,
			Status:          rStatus,
		})
	}

	return &VerifyPaymentOutput{
		PaymentReference:  psResp.Data.Reference,
		ProviderReference: psResp.Data.Reference,
		Status:            status,
		AmountKobo:        psResp.Data.Amount,
		Currency:          psResp.Data.Currency,
		PaidAt:            paidTime,
		Channel:           psResp.Data.Channel,
		CustomerEmail:     psResp.Data.Customer.Email,
		RawResponse:       bodyBytes,
		Refunds:           refundSummaries,
	}, nil
}

func (p *PaystackGateway) VerifyWebhookSignature(signatureHeader string, body []byte) bool {
	if p.secretKey == "" || signatureHeader == "" {
		return false
	}

	mac := hmac.New(sha512.New, []byte(p.secretKey))
	mac.Write(body)
	expectedMAC := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(expectedMAC), []byte(signatureHeader))
}

func (p *PaystackGateway) RefundPayment(ctx context.Context, input RefundPaymentInput) (*RefundPaymentOutput, error) {
	if p.secretKey == "" {
		return nil, ErrUnconfiguredProvider
	}
	if strings.TrimSpace(input.PaymentReference) == "" {
		return nil, errors.New("payment reference is required for refund")
	}
	if input.AmountKobo <= 0 {
		return nil, errors.New("refund amount must be greater than zero")
	}

	payload := map[string]interface{}{
		"transaction":   input.PaymentReference,
		"amount":        input.AmountKobo,
		"currency":      input.Currency,
		"merchant_note": input.MerchantNote,
		"customer_note": input.CustomerNote,
	}

	reqBody, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to encode paystack refund request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/refund", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create paystack refund request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+p.secretKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("paystack refund request failed: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read paystack refund response: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusAccepted {
		return nil, fmt.Errorf("paystack refund returned unexpected HTTP status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var psResp struct {
		Status  bool   `json:"status"`
		Message string `json:"message"`
		Data    struct {
			Status          string `json:"status"` // "pending", "processing", "processed", "failed"
			Reference       string `json:"reference"`
			RefundReference string `json:"refund_reference"`
			Amount          int64  `json:"amount"`
			Currency        string `json:"currency"`
		} `json:"data"`
	}

	if err := json.Unmarshal(bodyBytes, &psResp); err != nil {
		return nil, fmt.Errorf("%w: failed to parse refund response: %v", ErrMalformedResponse, err)
	}

	if !psResp.Status {
		return nil, fmt.Errorf("paystack refund error: %s", psResp.Message)
	}

	refRef := psResp.Data.RefundReference
	if refRef == "" {
		refRef = psResp.Data.Reference
	}

	status := "pending"
	switch psResp.Data.Status {
	case "processed", "success", "successful":
		status = "successful"
	case "failed":
		status = "failed"
	}

	return &RefundPaymentOutput{
		RefundReference: refRef,
		Status:          status,
		AmountKobo:      psResp.Data.Amount,
		Currency:        psResp.Data.Currency,
		RawResponse:     bodyBytes,
	}, nil
}

func (p *PaystackGateway) VerifyRefund(ctx context.Context, refundReference string) (*VerifyRefundOutput, error) {
	if p.secretKey == "" {
		return nil, ErrUnconfiguredProvider
	}
	ref := strings.TrimSpace(refundReference)
	if ref == "" {
		return nil, errors.New("refund reference is required for verification")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/refund/"+url.PathEscape(ref), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create paystack refund verification request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+p.secretKey)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("paystack refund verification request failed: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read paystack refund verification response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("paystack refund verification returned unexpected HTTP status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var psResp struct {
		Status  bool   `json:"status"`
		Message string `json:"message"`
		Data    struct {
			Status               string `json:"status"` // "processed", "processing", "pending", "failed"
			Reference            string `json:"reference"`
			RefundReference      string `json:"refund_reference"`
			TransactionReference string `json:"transaction_reference"`
			Amount               int64  `json:"amount"`
			Currency             string `json:"currency"`
		} `json:"data"`
	}

	if err := json.Unmarshal(bodyBytes, &psResp); err != nil {
		return nil, fmt.Errorf("%w: failed to parse refund verification response: %v", ErrMalformedResponse, err)
	}

	if !psResp.Status {
		return nil, fmt.Errorf("paystack refund verification error: %s", psResp.Message)
	}

	retRef := psResp.Data.RefundReference
	if retRef == "" {
		retRef = psResp.Data.Reference
	}

	status := "pending"
	switch psResp.Data.Status {
	case "processed", "success", "successful":
		status = "successful"
	case "failed":
		status = "failed"
	default:
		status = "pending"
	}

	return &VerifyRefundOutput{
		RefundReference:      retRef,
		TransactionReference: psResp.Data.TransactionReference,
		Status:               status,
		AmountKobo:           psResp.Data.Amount,
		Currency:             psResp.Data.Currency,
		RawResponse:          bodyBytes,
	}, nil
}
