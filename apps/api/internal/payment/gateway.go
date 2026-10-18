package payment

import (
	"context"
	"errors"
	"time"
)

var (
	ErrPaymentFailed          = errors.New("payment processing failed")
	ErrInvalidSignature       = errors.New("invalid webhook signature")
	ErrPaymentNotFound       = errors.New("payment transaction not found")
	ErrUnconfiguredProvider   = errors.New("payment provider is not configured")
	ErrAmountMismatch         = errors.New("payment amount does not match order total")
	ErrCurrencyMismatch       = errors.New("payment currency mismatch")
)

type InitializePaymentInput struct {
	OrderReference string                 `json:"order_reference"`
	AmountKobo     int64                  `json:"amount_kobo"`
	Currency       string                 `json:"currency"`
	CustomerEmail  string                 `json:"customer_email"`
	CustomerName   string                 `json:"customer_name"`
	CallbackURL    string                 `json:"callback_url"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

type InitializePaymentOutput struct {
	PaymentReference string `json:"payment_reference"`
	CheckoutURL      string `json:"checkout_url"`
	AccessCode       string `json:"access_code,omitempty"`
	Provider         string `json:"provider"`
}

type VerifyPaymentOutput struct {
	PaymentReference  string     `json:"payment_reference"`
	ProviderReference string     `json:"provider_reference"`
	Status            string     `json:"status"` // "successful", "failed", "pending"
	AmountKobo        int64      `json:"amount_kobo"`
	Currency          string     `json:"currency"`
	PaidAt            *time.Time `json:"paid_at,omitempty"`
	Channel           string     `json:"channel,omitempty"`
	CustomerEmail     string     `json:"customer_email,omitempty"`
	RawResponse       []byte     `json:"raw_response,omitempty"`
}

type PaymentGateway interface {
	ProviderName() string
	InitializePayment(ctx context.Context, input InitializePaymentInput) (*InitializePaymentOutput, error)
	VerifyPayment(ctx context.Context, reference string) (*VerifyPaymentOutput, error)
	VerifyWebhookSignature(signatureHeader string, body []byte) bool
}
