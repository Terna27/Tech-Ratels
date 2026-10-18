package payment

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
)

type SandboxGateway struct {
	frontendBaseURL string
	amounts         sync.Map
}

func NewSandboxGateway(frontendBaseURL string) *SandboxGateway {
	if frontendBaseURL == "" {
		frontendBaseURL = "http://localhost:3000"
	}
	return &SandboxGateway{
		frontendBaseURL: strings.TrimRight(frontendBaseURL, "/"),
	}
}

func (s *SandboxGateway) ProviderName() string {
	return "dev_sandbox"
}

func (s *SandboxGateway) InitializePayment(ctx context.Context, input InitializePaymentInput) (*InitializePaymentOutput, error) {
	log.Printf("[SANDBOX_PAYMENT] Initializing sandbox checkout for Order %s: ₦%.2f (%d kobo) for %s\n",
		input.OrderReference, float64(input.AmountKobo)/100.0, input.AmountKobo, input.CustomerEmail)

	s.amounts.Store(input.OrderReference, input.AmountKobo)

	checkoutURL := fmt.Sprintf("%s/marketplace/checkout/success?reference=%s&sandbox=true", s.frontendBaseURL, input.OrderReference)
	if input.CallbackURL != "" {
		checkoutURL = input.CallbackURL
	}

	return &InitializePaymentOutput{
		PaymentReference: input.OrderReference,
		CheckoutURL:      checkoutURL,
		AccessCode:       "sandbox_code_" + input.OrderReference,
		Provider:         "dev_sandbox",
	}, nil
}

func (s *SandboxGateway) VerifyPayment(ctx context.Context, reference string) (*VerifyPaymentOutput, error) {
	now := time.Now()
	// Deterministic failure simulation if reference contains "fail"
	if strings.Contains(strings.ToLower(reference), "fail") {
		return &VerifyPaymentOutput{
			PaymentReference:  reference,
			ProviderReference: "sandbox_fail_" + reference,
			Status:            "failed",
			AmountKobo:        0,
			Currency:          "NGN",
			Channel:           "sandbox_simulated",
		}, nil
	}

	amountKobo := int64(1000000000)
	if val, ok := s.amounts.Load(reference); ok {
		amountKobo = val.(int64)
	}

	return &VerifyPaymentOutput{
		PaymentReference:  reference,
		ProviderReference: "sandbox_txn_" + reference,
		Status:            "successful",
		AmountKobo:        amountKobo,
		Currency:          "NGN",
		PaidAt:            &now,
		Channel:           "sandbox_transfer",
		RawResponse:       []byte(`{"status":"success","sandbox":true}`),
	}, nil
}

func (s *SandboxGateway) VerifyWebhookSignature(signatureHeader string, body []byte) bool {
	// In sandbox mode, allow valid sandbox secret or test header
	return signatureHeader == "sandbox-signature-ok" || signatureHeader != ""
}
