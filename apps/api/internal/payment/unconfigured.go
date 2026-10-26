package payment

import (
	"context"
	"errors"
)

var (
	ErrPaymentProcessingUnavailable = errors.New("live payment processing is currently unavailable on Ratels Marketplace; please contact administration")
)

// UnconfiguredGateway is deployed in production when live payment credentials (such as Paystack API keys)
// have not yet been configured. It guarantees that the system will NEVER accept simulated or sandbox payments
// as real transactions in a live production environment.
type UnconfiguredGateway struct{}

func NewUnconfiguredGateway() *UnconfiguredGateway {
	return &UnconfiguredGateway{}
}

func (u *UnconfiguredGateway) ProviderName() string {
	return "unconfigured"
}

func (u *UnconfiguredGateway) InitializePayment(ctx context.Context, input InitializePaymentInput) (*InitializePaymentOutput, error) {
	return nil, ErrPaymentProcessingUnavailable
}

func (u *UnconfiguredGateway) VerifyPayment(ctx context.Context, reference string) (*VerifyPaymentOutput, error) {
	return nil, ErrPaymentProcessingUnavailable
}

func (u *UnconfiguredGateway) VerifyWebhookSignature(signatureHeader string, body []byte) bool {
	return false
}

func (u *UnconfiguredGateway) RefundPayment(ctx context.Context, input RefundPaymentInput) (*RefundPaymentOutput, error) {
	return nil, ErrPaymentProcessingUnavailable
}

func (u *UnconfiguredGateway) VerifyRefund(ctx context.Context, refundReference string) (*VerifyRefundOutput, error) {
	return nil, ErrPaymentProcessingUnavailable
}
