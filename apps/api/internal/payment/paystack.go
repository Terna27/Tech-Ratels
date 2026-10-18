package payment

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

	var psResp struct {
		Status  bool   `json:"status"`
		Message string `json:"message"`
		Data    struct {
			Status        string `json:"status"` // "success", "failed", "abandoned"
			Reference     string `json:"reference"`
			Amount        int64  `json:"amount"`
			Currency      string `json:"currency"`
			PaidAt        string `json:"paid_at"`
			Channel       string `json:"channel"`
			GatewayResp   string `json:"gateway_response"`
			Customer      struct {
				Email string `json:"email"`
			} `json:"customer"`
		} `json:"data"`
	}

	if err := json.Unmarshal(bodyBytes, &psResp); err != nil {
		return nil, fmt.Errorf("failed to parse verify response: %w", err)
	}

	if !psResp.Status {
		return nil, fmt.Errorf("paystack verification returned false: %s", psResp.Message)
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
