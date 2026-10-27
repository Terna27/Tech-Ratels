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
	refunds         sync.Map
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

func (s *SandboxGateway) SetAmount(reference string, amountKobo int64) {
	s.amounts.Store(reference, amountKobo)
}

func (s *SandboxGateway) AddSimulatedRefund(reference string, summary TransactionRefundSummary) {
	var list []TransactionRefundSummary
	if val, ok := s.refunds.Load(reference); ok {
		list = val.([]TransactionRefundSummary)
	}
	list = append(list, summary)
	s.refunds.Store(reference, list)
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

	var refundList []TransactionRefundSummary
	if val, ok := s.refunds.Load(reference); ok {
		refundList = val.([]TransactionRefundSummary)
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
		Refunds:           refundList,
	}, nil
}

func (s *SandboxGateway) VerifyWebhookSignature(signatureHeader string, body []byte) bool {
	// In sandbox mode, allow valid sandbox secret or test header
	return signatureHeader == "sandbox-signature-ok" || signatureHeader != ""
}

func (s *SandboxGateway) RefundPayment(ctx context.Context, input RefundPaymentInput) (*RefundPaymentOutput, error) {
	if strings.Contains(strings.ToLower(input.MerchantNote), "fail") || strings.Contains(strings.ToLower(input.CustomerNote), "fail") {
		return &RefundPaymentOutput{
			RefundReference: "sandbox_ref_fail_" + input.PaymentReference,
			Status:          "failed",
			AmountKobo:      input.AmountKobo,
			Currency:        input.Currency,
			RawResponse:     []byte(`{"status":false,"message":"Simulated sandbox refund failure"}`),
		}, nil
	}

	ref := "sandbox_ref_" + input.PaymentReference
	s.AddSimulatedRefund(input.PaymentReference, TransactionRefundSummary{
		RefundReference: ref,
		AmountKobo:      input.AmountKobo,
		Status:          "pending",
	})

	return &RefundPaymentOutput{
		RefundReference: ref,
		Status:          "pending",
		AmountKobo:      input.AmountKobo,
		Currency:        input.Currency,
		RawResponse:     []byte(`{"status":true,"message":"Refund queued in sandbox"}`),
	}, nil
}

func (s *SandboxGateway) VerifyRefund(ctx context.Context, refundReference string) (*VerifyRefundOutput, error) {
	lowerRef := strings.ToLower(refundReference)
	if strings.Contains(lowerRef, "fail") {
		return &VerifyRefundOutput{
			RefundReference: refundReference,
			Status:          "failed",
			RawResponse:     []byte(`{"status":"failed","message":"Simulated sandbox refund failure"}`),
		}, nil
	}
	if strings.Contains(lowerRef, "pending") || strings.Contains(lowerRef, "processing") {
		return &VerifyRefundOutput{
			RefundReference: refundReference,
			Status:          "pending",
			RawResponse:     []byte(`{"status":"pending","message":"Simulated sandbox refund pending"}`),
		}, nil
	}

	return &VerifyRefundOutput{
		RefundReference: refundReference,
		Status:          "successful",
		RawResponse:     []byte(`{"status":"processed","message":"Simulated sandbox refund processed successfully"}`),
	}, nil
}
