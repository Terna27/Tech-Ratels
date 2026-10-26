package service_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"ratels-api/internal/db"
	"ratels-api/internal/payment"
	"ratels-api/internal/service"
)

type mockCustomGateway struct {
	providerName string
	verifyFn     func(ctx context.Context, reference string) (*payment.VerifyPaymentOutput, error)
	verifySigFn  func(signatureHeader string, body []byte) bool
	refundFn     func(ctx context.Context, input payment.RefundPaymentInput) (*payment.RefundPaymentOutput, error)
}

func (m *mockCustomGateway) ProviderName() string {
	if m.providerName != "" {
		return m.providerName
	}
	return "paystack"
}

func (m *mockCustomGateway) InitializePayment(ctx context.Context, input payment.InitializePaymentInput) (*payment.InitializePaymentOutput, error) {
	return &payment.InitializePaymentOutput{
		PaymentReference: input.OrderReference,
		CheckoutURL:      "https://checkout.paystack.co/test",
		Provider:         m.ProviderName(),
	}, nil
}

func (m *mockCustomGateway) VerifyPayment(ctx context.Context, reference string) (*payment.VerifyPaymentOutput, error) {
	if m.verifyFn != nil {
		return m.verifyFn(ctx, reference)
	}
	now := time.Now()
	return &payment.VerifyPaymentOutput{
		PaymentReference:  reference,
		ProviderReference: "ps_ref_" + reference,
		Status:            "successful",
		AmountKobo:        250000,
		Currency:          "NGN",
		PaidAt:            &now,
		Channel:           "card",
		CustomerEmail:     "buyer@example.org",
	}, nil
}

func (m *mockCustomGateway) VerifyWebhookSignature(signatureHeader string, body []byte) bool {
	if m.verifySigFn != nil {
		return m.verifySigFn(signatureHeader, body)
	}
	return signatureHeader == "valid-paystack-signature"
}

func (m *mockCustomGateway) RefundPayment(ctx context.Context, input payment.RefundPaymentInput) (*payment.RefundPaymentOutput, error) {
	if m.refundFn != nil {
		return m.refundFn(ctx, input)
	}
	return &payment.RefundPaymentOutput{
		RefundReference: "mock_ref_" + input.PaymentReference,
		Status:          "pending",
		AmountKobo:      input.AmountKobo,
		Currency:        input.Currency,
		RawResponse:     []byte(`{"status":true,"data":{"status":"pending"}}`),
	}, nil
}

func helperCreateOrderWithGateway(t *testing.T, env *TestEnv, gw payment.PaymentGateway, amountKobo int64) (*service.MarketplaceService, *service.CreateOrderResult, *db.User) {
	t.Helper()
	ctx := context.Background()

	seller := createRegisteredUser(t, env, fmt.Sprintf("seller_%d", time.Now().UnixNano()), "member")
	admin := createRegisteredUser(t, env, fmt.Sprintf("admin_%d", time.Now().UnixNano()), "super_admin")
	buyer := createRegisteredUser(t, env, fmt.Sprintf("buyer_%d", time.Now().UnixNano()), "member")

	mktSvc := service.NewMarketplaceService(env.Repo, env.AuditService, gw)

	biz, err := mktSvc.RegisterBusiness(ctx, seller, service.RegisterBusinessInput{
		Name:          fmt.Sprintf("Store %d", time.Now().UnixNano()),
		Category:      "tools_hardware",
		OwnershipType: "member_owned",
		ContactEmail:  "store@example.org",
		ContactPhone:  "+2348011223344",
	})
	if err != nil {
		t.Fatalf("RegisterBusiness failed: %v", err)
	}
	_, err = mktSvc.ApproveBusiness(ctx, admin, biz.ID, true, "")
	if err != nil {
		t.Fatalf("ApproveBusiness failed: %v", err)
	}

	prod, err := mktSvc.CreateProduct(ctx, seller, service.CreateProductInput{
		BusinessID:    biz.ID,
		Title:         "Integrity Test Tool",
		Category:      "tools_hardware",
		SKU:           fmt.Sprintf("SKU-INT-%d", time.Now().UnixNano()),
		PriceKobo:     amountKobo,
		StockQuantity: 100,
	})
	if err != nil {
		t.Fatalf("CreateProduct failed: %v", err)
	}

	orderRes, err := mktSvc.CreateOrder(ctx, buyer, service.CheckoutInput{
		Items:        []service.CartItemInput{{ProductID: prod.ID, Quantity: 1}},
		BuyerName:    "Buyer Integrity",
		BuyerEmail:   "buyer_int@example.org",
		BuyerPhone:   "+2348000000000",
		DeliveryType: "pickup",
	})
	if err != nil {
		t.Fatalf("CreateOrder failed: %v", err)
	}

	return mktSvc, orderRes, buyer
}

// 1. Valid Paystack payment confirmation
func TestMarketplacePaymentIntegrity_ValidPaystackConfirmation(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	gw := &mockCustomGateway{
		providerName: "paystack",
	}

	mktSvc, orderRes, _ := helperCreateOrderWithGateway(t, env, gw, 300000)

	gw.verifyFn = func(ctx context.Context, reference string) (*payment.VerifyPaymentOutput, error) {
		now := time.Now()
		return &payment.VerifyPaymentOutput{
			PaymentReference:  reference,
			ProviderReference: "pstk_tx_" + reference,
			Status:            "successful",
			AmountKobo:        300000,
			Currency:          "NGN",
			PaidAt:            &now,
			Channel:           "card",
		}, nil
	}

	order, err := mktSvc.ConfirmPayment(ctx, orderRes.Payment.Reference)
	if err != nil {
		t.Fatalf("ConfirmPayment failed: %v", err)
	}

	if order.PaymentStatus != "paid" {
		t.Errorf("expected payment status 'paid', got %s", order.PaymentStatus)
	}
	if order.OrderStatus != "processing" {
		t.Errorf("expected order status 'processing', got %s", order.OrderStatus)
	}
}

// 2. Incorrect payment reference
func TestMarketplacePaymentIntegrity_IncorrectPaymentReference(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	gw := &mockCustomGateway{
		providerName: "paystack",
	}

	mktSvc, orderRes, _ := helperCreateOrderWithGateway(t, env, gw, 300000)

	gw.verifyFn = func(ctx context.Context, reference string) (*payment.VerifyPaymentOutput, error) {
		now := time.Now()
		return &payment.VerifyPaymentOutput{
			PaymentReference:  "DIFFERENT_REFERENCE_999",
			ProviderReference: "pstk_tx_diff",
			Status:            "successful",
			AmountKobo:        300000,
			Currency:          "NGN",
			PaidAt:            &now,
		}, nil
	}

	_, err := mktSvc.ConfirmPayment(ctx, orderRes.Payment.Reference)
	if err == nil {
		t.Fatal("expected error on reference mismatch, got nil")
	}
	if !errors.Is(err, payment.ErrReferenceMismatch) {
		t.Fatalf("expected ErrReferenceMismatch, got %v", err)
	}
}

// 3. Underpayment and Overpayment
func TestMarketplacePaymentIntegrity_UnderAndOverPayment(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	gw := &mockCustomGateway{providerName: "paystack"}
	mktSvc, orderRes, _ := helperCreateOrderWithGateway(t, env, gw, 500000)

	// Underpayment: returned 400,000 kobo for a 500,000 kobo order
	gw.verifyFn = func(ctx context.Context, reference string) (*payment.VerifyPaymentOutput, error) {
		now := time.Now()
		return &payment.VerifyPaymentOutput{
			PaymentReference:  reference,
			ProviderReference: "pstk_tx_under",
			Status:            "successful",
			AmountKobo:        400000,
			Currency:          "NGN",
			PaidAt:            &now,
		}, nil
	}

	_, err := mktSvc.ConfirmPayment(ctx, orderRes.Payment.Reference)
	if err == nil {
		t.Fatal("expected ErrAmountMismatch for underpayment, got nil")
	}
	if !errors.Is(err, payment.ErrAmountMismatch) {
		t.Fatalf("expected ErrAmountMismatch, got %v", err)
	}

	// Overpayment: returned 600,000 kobo for a 500,000 kobo order
	gw.verifyFn = func(ctx context.Context, reference string) (*payment.VerifyPaymentOutput, error) {
		now := time.Now()
		return &payment.VerifyPaymentOutput{
			PaymentReference:  reference,
			ProviderReference: "pstk_tx_over",
			Status:            "successful",
			AmountKobo:        600000,
			Currency:          "NGN",
			PaidAt:            &now,
		}, nil
	}

	_, err = mktSvc.ConfirmPayment(ctx, orderRes.Payment.Reference)
	if err == nil {
		t.Fatal("expected ErrAmountMismatch for overpayment, got nil")
	}
	if !errors.Is(err, payment.ErrAmountMismatch) {
		t.Fatalf("expected ErrAmountMismatch, got %v", err)
	}
}

// 4. Missing or incorrect currency
func TestMarketplacePaymentIntegrity_CurrencyValidations(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	gw := &mockCustomGateway{providerName: "paystack"}
	mktSvc, orderRes, _ := helperCreateOrderWithGateway(t, env, gw, 200000)

	// Missing currency
	gw.verifyFn = func(ctx context.Context, reference string) (*payment.VerifyPaymentOutput, error) {
		now := time.Now()
		return &payment.VerifyPaymentOutput{
			PaymentReference:  reference,
			ProviderReference: "pstk_tx_nocurr",
			Status:            "successful",
			AmountKobo:        200000,
			Currency:          "",
			PaidAt:            &now,
		}, nil
	}

	_, err := mktSvc.ConfirmPayment(ctx, orderRes.Payment.Reference)
	if err == nil {
		t.Fatal("expected error for missing currency, got nil")
	}
	if !errors.Is(err, payment.ErrCurrencyMismatch) {
		t.Fatalf("expected ErrCurrencyMismatch, got %v", err)
	}

	// Incorrect currency (e.g. USD)
	gw.verifyFn = func(ctx context.Context, reference string) (*payment.VerifyPaymentOutput, error) {
		now := time.Now()
		return &payment.VerifyPaymentOutput{
			PaymentReference:  reference,
			ProviderReference: "pstk_tx_usd",
			Status:            "successful",
			AmountKobo:        200000,
			Currency:          "USD",
			PaidAt:            &now,
		}, nil
	}

	_, err = mktSvc.ConfirmPayment(ctx, orderRes.Payment.Reference)
	if err == nil {
		t.Fatal("expected error for currency mismatch, got nil")
	}
	if !errors.Is(err, payment.ErrCurrencyMismatch) {
		t.Fatalf("expected ErrCurrencyMismatch, got %v", err)
	}
}

// 5. Expected payment provider mismatch
func TestMarketplacePaymentIntegrity_ProviderMismatch(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	paystackGw := &mockCustomGateway{providerName: "paystack"}
	_, orderRes, _ := helperCreateOrderWithGateway(t, env, paystackGw, 200000)

	// Now try to verify using a service configured with dev_sandbox provider
	sandboxGw := payment.NewSandboxGateway("http://localhost:3000")
	mktSvcWithSandbox := service.NewMarketplaceService(env.Repo, env.AuditService, sandboxGw)

	_, err := mktSvcWithSandbox.ConfirmPayment(ctx, orderRes.Payment.Reference)
	if err == nil {
		t.Fatal("expected error on provider mismatch, got nil")
	}
	if !errors.Is(err, payment.ErrProviderMismatch) {
		t.Fatalf("expected ErrProviderMismatch, got %v", err)
	}
}

// 6. Duplicate successful webhooks (Idempotency)
func TestMarketplacePaymentIntegrity_DuplicateSuccessfulWebhooks(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	gw := &mockCustomGateway{providerName: "paystack"}
	mktSvc, orderRes, buyer := helperCreateOrderWithGateway(t, env, gw, 150000)

	gw.verifyFn = func(ctx context.Context, reference string) (*payment.VerifyPaymentOutput, error) {
		now := time.Now()
		return &payment.VerifyPaymentOutput{
			PaymentReference:  reference,
			ProviderReference: "pstk_tx_" + reference,
			Status:            "successful",
			AmountKobo:        150000,
			Currency:          "NGN",
			PaidAt:            &now,
		}, nil
	}

	payload := []byte(fmt.Sprintf(`{"event":"charge.success","data":{"reference":"%s","status":"success","amount":150000,"currency":"NGN"}}`, orderRes.Payment.Reference))

	// First webhook delivery
	err := mktSvc.HandleWebhook(ctx, "valid-paystack-signature", payload)
	if err != nil {
		t.Fatalf("first HandleWebhook failed: %v", err)
	}

	order1, _, _, err := mktSvc.GetOrderByID(ctx, buyer, orderRes.Order.ID)
	if err != nil {
		t.Fatalf("GetOrderByID failed: %v", err)
	}
	if order1.PaymentStatus != "paid" {
		t.Fatalf("expected paid after first webhook, got %s", order1.PaymentStatus)
	}

	// Second webhook delivery (exact duplicate)
	err = mktSvc.HandleWebhook(ctx, "valid-paystack-signature", payload)
	if err != nil {
		t.Fatalf("second HandleWebhook failed: %v", err)
	}

	order2, _, _, err := mktSvc.GetOrderByID(ctx, buyer, orderRes.Order.ID)
	if err != nil {
		t.Fatalf("GetOrderByID after second webhook failed: %v", err)
	}
	if order2.PaymentStatus != "paid" {
		t.Fatalf("expected paid after second webhook, got %s", order2.PaymentStatus)
	}
}

// 7. Concurrent payment confirmation requests
func TestMarketplacePaymentIntegrity_ConcurrentPaymentConfirmations(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	gw := &mockCustomGateway{providerName: "paystack"}
	mktSvc, orderRes, buyer := helperCreateOrderWithGateway(t, env, gw, 400000)

	gw.verifyFn = func(ctx context.Context, reference string) (*payment.VerifyPaymentOutput, error) {
		now := time.Now()
		return &payment.VerifyPaymentOutput{
			PaymentReference:  reference,
			ProviderReference: "pstk_tx_" + reference,
			Status:            "successful",
			AmountKobo:        400000,
			Currency:          "NGN",
			PaidAt:            &now,
		}, nil
	}

	const concurrency = 6
	var wg sync.WaitGroup
	errCh := make(chan error, concurrency)

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := mktSvc.ConfirmPayment(ctx, orderRes.Payment.Reference)
			if err != nil {
				errCh <- err
			}
		}()
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Errorf("concurrent ConfirmPayment returned unexpected error: %v", err)
	}

	order, _, _, err := mktSvc.GetOrderByID(ctx, buyer, orderRes.Order.ID)
	if err != nil {
		t.Fatalf("GetOrderByID failed: %v", err)
	}
	if order.PaymentStatus != "paid" {
		t.Errorf("expected payment status 'paid', got %s", order.PaymentStatus)
	}
	if order.OrderStatus != "processing" {
		t.Errorf("expected order status 'processing', got %s", order.OrderStatus)
	}
}

// 8. Failed payment notification arriving after successful payment
func TestMarketplacePaymentIntegrity_FailedNotificationAfterSuccess(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	gw := &mockCustomGateway{providerName: "paystack"}
	mktSvc, orderRes, buyer := helperCreateOrderWithGateway(t, env, gw, 350000)

	// Step 1: Successful payment confirmation
	gw.verifyFn = func(ctx context.Context, reference string) (*payment.VerifyPaymentOutput, error) {
		now := time.Now()
		return &payment.VerifyPaymentOutput{
			PaymentReference:  reference,
			ProviderReference: "pstk_tx_" + reference,
			Status:            "successful",
			AmountKobo:        350000,
			Currency:          "NGN",
			PaidAt:            &now,
		}, nil
	}

	order, err := mktSvc.ConfirmPayment(ctx, orderRes.Payment.Reference)
	if err != nil {
		t.Fatalf("ConfirmPayment failed: %v", err)
	}
	if order.PaymentStatus != "paid" {
		t.Fatalf("expected payment status 'paid', got %s", order.PaymentStatus)
	}

	// Step 2: Out-of-order or late failed webhook arrives
	gw.verifyFn = func(ctx context.Context, reference string) (*payment.VerifyPaymentOutput, error) {
		return &payment.VerifyPaymentOutput{
			PaymentReference:  reference,
			ProviderReference: "pstk_tx_failed_" + reference,
			Status:            "failed",
			AmountKobo:        0,
			Currency:          "NGN",
		}, nil
	}

	failedPayload := []byte(fmt.Sprintf(`{"event":"charge.failed","data":{"reference":"%s","status":"failed","amount":350000,"currency":"NGN"}}`, orderRes.Payment.Reference))
	_ = mktSvc.HandleWebhook(ctx, "valid-paystack-signature", failedPayload)

	// Verify order and payment status were NOT downgraded
	checkOrder, _, _, err := mktSvc.GetOrderByID(ctx, buyer, orderRes.Order.ID)
	if err != nil {
		t.Fatalf("GetOrderByID failed: %v", err)
	}
	if checkOrder.PaymentStatus != "paid" {
		t.Errorf("order payment status was improperly downgraded to: %s", checkOrder.PaymentStatus)
	}
	if checkOrder.OrderStatus != "processing" {
		t.Errorf("order status was improperly modified to: %s", checkOrder.OrderStatus)
	}
}

// 9. Cancelled order remains cancelled and is not reset to processing
func TestMarketplacePaymentIntegrity_CancelledOrderNotReset(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	gw := &mockCustomGateway{providerName: "paystack"}
	mktSvc, orderRes, buyer := helperCreateOrderWithGateway(t, env, gw, 250000)

	// Cancel order before payment
	_, err := mktSvc.CancelOrder(ctx, buyer, orderRes.Order.ID, "Changed my mind")
	if err != nil {
		t.Fatalf("CancelOrder failed: %v", err)
	}

	// Now simulated payment arrives
	gw.verifyFn = func(ctx context.Context, reference string) (*payment.VerifyPaymentOutput, error) {
		now := time.Now()
		return &payment.VerifyPaymentOutput{
			PaymentReference:  reference,
			ProviderReference: "pstk_tx_" + reference,
			Status:            "successful",
			AmountKobo:        250000,
			Currency:          "NGN",
			PaidAt:            &now,
		}, nil
	}

	_, _ = mktSvc.ConfirmPayment(ctx, orderRes.Payment.Reference)

	// Recheck order: order_status MUST remain 'cancelled' and NOT be reset to 'processing'
	cancelledOrder, _, _, err := mktSvc.GetOrderByID(ctx, buyer, orderRes.Order.ID)
	if err != nil {
		t.Fatalf("GetOrderByID failed: %v", err)
	}
	if cancelledOrder.OrderStatus != "cancelled" {
		t.Errorf("expected order status to remain 'cancelled', got '%s'", cancelledOrder.OrderStatus)
	}
}

// 10. Refund Lifecycle: Pending, Confirmation, Stock Preservation & Manual Settlement
func TestMarketplaceRefund_PendingAndConfirmation(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	admin := createRegisteredUser(t, env, fmt.Sprintf("admin_pen_%d", time.Now().UnixNano()), "super_admin")
	gw := &mockCustomGateway{providerName: "paystack"}
	mktSvc, orderRes, buyer := helperCreateOrderWithGateway(t, env, gw, 600000)

	// Confirm payment first
	gw.verifyFn = func(ctx context.Context, reference string) (*payment.VerifyPaymentOutput, error) {
		now := time.Now()
		return &payment.VerifyPaymentOutput{
			PaymentReference:  reference,
			ProviderReference: "pstk_tx_" + reference,
			Status:            "successful",
			AmountKobo:        600000,
			Currency:          "NGN",
			PaidAt:            &now,
		}, nil
	}
	_, err := mktSvc.ConfirmPayment(ctx, orderRes.Payment.Reference)
	if err != nil {
		t.Fatalf("ConfirmPayment failed: %v", err)
	}

	// 1. Initiate refund (pending by default)
	refund, orderAfterInit, err := mktSvc.InitiateRefund(ctx, admin, orderRes.Order.ID, service.RefundOrderInput{
		AmountKobo: 200000,
		Reason:     "Damaged accessory",
	})
	if err != nil {
		t.Fatalf("InitiateRefund failed: %v", err)
	}

	// Assert refund status is pending
	if refund.Status != "pending" {
		t.Errorf("expected refund status 'pending', got %s", refund.Status)
	}

	// Assert order status and payment status are NOT prematurely refunded
	if orderAfterInit.PaymentStatus != "paid" {
		t.Errorf("order payment status should remain 'paid' while refund is pending, got %s", orderAfterInit.PaymentStatus)
	}
	if orderAfterInit.OrderStatus != "processing" {
		t.Errorf("order status should remain 'processing', got %s", orderAfterInit.OrderStatus)
	}

	// Stock must NOT be restored while refund is pending
	pRow, _ := mktSvc.GetProductByID(ctx, orderRes.Items[0].ProductID.String)
	if pRow.StockQuantity != 99 {
		t.Errorf("stock must not be restored while refund is pending (expected 99, got %d)", pRow.StockQuantity)
	}

	// 2. Confirm refund (manual settlement or verified provider confirmation)
	confirmedRefund, confirmedOrder, err := mktSvc.ConfirmRefund(ctx, admin, refund.ID, service.ConfirmRefundInput{
		ProviderRefundReference: "pstk_ref_123",
		SettlementNotes:         "Settlement confirmed with bank",
	})
	if err != nil {
		t.Fatalf("ConfirmRefund failed: %v", err)
	}

	if confirmedRefund.Status != "successful" {
		t.Errorf("expected confirmed refund status 'successful', got %s", confirmedRefund.Status)
	}
	if confirmedOrder.PaymentStatus != "partially_refunded" {
		t.Errorf("expected order payment status 'partially_refunded', got %s", confirmedOrder.PaymentStatus)
	}

	// Buyer can view refunds
	refundsList, err := mktSvc.ListRefundsByOrder(ctx, buyer, orderRes.Order.ID)
	if err != nil {
		t.Fatalf("ListRefundsByOrder failed: %v", err)
	}
	if len(refundsList) != 1 {
		t.Fatalf("expected 1 refund in list, got %d", len(refundsList))
	}
	if refundsList[0].ID != refund.ID {
		t.Errorf("expected refund ID %s, got %s", refund.ID, refundsList[0].ID)
	}
}

// 11. Multiple partial refunds, over-refund rejection, and full refund stock restoration
func TestMarketplaceRefund_MultiplePartialRefundsAndFull(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	admin := createRegisteredUser(t, env, fmt.Sprintf("admin_mult_%d", time.Now().UnixNano()), "super_admin")
	gw := &mockCustomGateway{providerName: "paystack"}
	mktSvc, orderRes, _ := helperCreateOrderWithGateway(t, env, gw, 600000) // Total ₦6,000

	gw.verifyFn = func(ctx context.Context, reference string) (*payment.VerifyPaymentOutput, error) {
		now := time.Now()
		return &payment.VerifyPaymentOutput{
			PaymentReference:  reference,
			ProviderReference: "pstk_tx_" + reference,
			Status:            "successful",
			AmountKobo:        600000,
			Currency:          "NGN",
			PaidAt:            &now,
		}, nil
	}
	_, err := mktSvc.ConfirmPayment(ctx, orderRes.Payment.Reference)
	if err != nil {
		t.Fatalf("ConfirmPayment failed: %v", err)
	}

	// 1. First partial refund (200,000 kobo)
	r1, _, err := mktSvc.InitiateRefund(ctx, admin, orderRes.Order.ID, service.RefundOrderInput{
		AmountKobo: 200000,
		Reason:     "First partial refund",
	})
	if err != nil {
		t.Fatalf("first partial refund initiation failed: %v", err)
	}
	_, _, err = mktSvc.ConfirmRefund(ctx, admin, r1.ID, service.ConfirmRefundInput{})
	if err != nil {
		t.Fatalf("first partial refund confirmation failed: %v", err)
	}

	// 2. Second partial refund (200,000 kobo)
	r2, _, err := mktSvc.InitiateRefund(ctx, admin, orderRes.Order.ID, service.RefundOrderInput{
		AmountKobo: 200000,
		Reason:     "Second partial refund",
	})
	if err != nil {
		t.Fatalf("second partial refund initiation failed: %v", err)
	}
	_, o2, err := mktSvc.ConfirmRefund(ctx, admin, r2.ID, service.ConfirmRefundInput{})
	if err != nil {
		t.Fatalf("second partial refund confirmation failed: %v", err)
	}
	if o2.PaymentStatus != "partially_refunded" {
		t.Errorf("expected 'partially_refunded', got %s", o2.PaymentStatus)
	}

	// 3. Excessive partial refund (300,000 kobo when remaining eligible is 200,000 kobo)
	_, _, err = mktSvc.InitiateRefund(ctx, admin, orderRes.Order.ID, service.RefundOrderInput{
		AmountKobo: 300000,
		Reason:     "Over-refund exploit attempt",
	})
	if err == nil {
		t.Fatal("expected error when refund exceeds remaining eligible amount, got nil")
	}

	// 4. Exact remaining balance refund (200,000 kobo)
	r3, _, err := mktSvc.InitiateRefund(ctx, admin, orderRes.Order.ID, service.RefundOrderInput{
		AmountKobo: 200000,
		Reason:     "Final settlement refund",
	})
	if err != nil {
		t.Fatalf("final settlement refund initiation failed: %v", err)
	}
	_, fullOrder, err := mktSvc.ConfirmRefund(ctx, admin, r3.ID, service.ConfirmRefundInput{})
	if err != nil {
		t.Fatalf("final settlement refund confirmation failed: %v", err)
	}
	if fullOrder.PaymentStatus != "refunded" || fullOrder.OrderStatus != "refunded" {
		t.Errorf("expected payment_status=refunded, order_status=refunded, got %s / %s",
			fullOrder.PaymentStatus, fullOrder.OrderStatus)
	}

	// Product stock restored to original 100 on full refund
	pRow, _ := mktSvc.GetProductByID(ctx, orderRes.Items[0].ProductID.String)
	if pRow.StockQuantity != 100 {
		t.Errorf("expected stock to be restored to 100, got %d", pRow.StockQuantity)
	}

	// 5. Duplicate refund attempt on already fully refunded order
	_, _, err = mktSvc.InitiateRefund(ctx, admin, orderRes.Order.ID, service.RefundOrderInput{
		AmountKobo: 100000,
		Reason:     "Duplicate refund",
	})
	if err == nil {
		t.Fatal("expected error when refunding an already refunded order, got nil")
	}
}

// 12. Concurrent refund requests cannot exceed the original payment
func TestMarketplaceRefund_ConcurrentRefundRequests(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	admin := createRegisteredUser(t, env, fmt.Sprintf("admin_conc_%d", time.Now().UnixNano()), "super_admin")
	gw := &mockCustomGateway{providerName: "paystack"}
	mktSvc, orderRes, _ := helperCreateOrderWithGateway(t, env, gw, 500000) // Total ₦5,000

	gw.verifyFn = func(ctx context.Context, reference string) (*payment.VerifyPaymentOutput, error) {
		now := time.Now()
		return &payment.VerifyPaymentOutput{
			PaymentReference:  reference,
			ProviderReference: "pstk_tx_" + reference,
			Status:            "successful",
			AmountKobo:        500000,
			Currency:          "NGN",
			PaidAt:            &now,
		}, nil
	}
	_, err := mktSvc.ConfirmPayment(ctx, orderRes.Payment.Reference)
	if err != nil {
		t.Fatalf("ConfirmPayment failed: %v", err)
	}

	// 5 concurrent requests of 250,000 kobo each.
	// Exactly 2 can succeed (2 * 250k = 500k). The other 3 must fail.
	const concurrency = 5
	var wg sync.WaitGroup
	successCount := 0
	failCount := 0
	var mu sync.Mutex

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, _, err := mktSvc.InitiateRefund(ctx, admin, orderRes.Order.ID, service.RefundOrderInput{
				AmountKobo: 250000,
				Reason:     fmt.Sprintf("Concurrent refund attempt %d", idx),
			})
			mu.Lock()
			if err == nil {
				successCount++
			} else {
				failCount++
			}
			mu.Unlock()
		}(i)
	}

	wg.Wait()

	if successCount != 2 {
		t.Errorf("expected exactly 2 concurrent refunds to succeed, got %d (failed: %d)", successCount, failCount)
	}
	if failCount != 3 {
		t.Errorf("expected exactly 3 concurrent refunds to fail, got %d", failCount)
	}

	// Verify total reserved amount in DB does not exceed total order amount
	sums, err := env.Repo.Queries().GetMarketplaceRefundSumsByOrder(ctx, orderRes.Order.ID)
	if err != nil {
		t.Fatalf("GetMarketplaceRefundSumsByOrder failed: %v", err)
	}
	totalCommitted := sums.SuccessfulAmountKobo + sums.PendingAmountKobo
	if totalCommitted > 500000 {
		t.Fatalf("Total reserved refunds (%d kobo) exceeded order total (500000 kobo)!", totalCommitted)
	}
}

// 13. Failed provider refund releases pending reservation
func TestMarketplaceRefund_FailedProviderRefund(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	admin := createRegisteredUser(t, env, fmt.Sprintf("admin_fail_%d", time.Now().UnixNano()), "super_admin")
	gw := &mockCustomGateway{providerName: "paystack"}
	mktSvc, orderRes, _ := helperCreateOrderWithGateway(t, env, gw, 400000)

	gw.verifyFn = func(ctx context.Context, reference string) (*payment.VerifyPaymentOutput, error) {
		now := time.Now()
		return &payment.VerifyPaymentOutput{
			PaymentReference:  reference,
			ProviderReference: "pstk_tx_" + reference,
			Status:            "successful",
			AmountKobo:        400000,
			Currency:          "NGN",
			PaidAt:            &now,
		}, nil
	}
	_, err := mktSvc.ConfirmPayment(ctx, orderRes.Payment.Reference)
	if err != nil {
		t.Fatalf("ConfirmPayment failed: %v", err)
	}

	// 1. Initiate refund (pending)
	r, _, err := mktSvc.InitiateRefund(ctx, admin, orderRes.Order.ID, service.RefundOrderInput{
		AmountKobo: 400000,
		Reason:     "Customer returned item",
	})
	if err != nil {
		t.Fatalf("InitiateRefund failed: %v", err)
	}

	// 2. Simulate provider reporting failure (e.g. via webhook refund.failed)
	failPayload := []byte(fmt.Sprintf(`{
		"event": "refund.failed",
		"data": {
			"reference": "%s",
			"refund_reference": "%s",
			"status": "failed",
			"amount": 400000,
			"currency": "NGN"
		}
	}`, orderRes.Payment.Reference, r.ProviderRefundReference))

	err = mktSvc.HandleWebhook(ctx, "valid-paystack-signature", failPayload)
	if err != nil {
		t.Fatalf("HandleWebhook for refund.failed failed: %v", err)
	}

	// 3. Assert refund is failed
	failedRef, err := mktSvc.GetRefundByID(ctx, admin, r.ID)
	if err != nil {
		t.Fatalf("GetRefundByID failed: %v", err)
	}
	if failedRef.Status != "failed" {
		t.Errorf("expected refund status 'failed', got %s", failedRef.Status)
	}

	// 4. Assert pending reservation was freed: order should now allow a new full refund
	rNew, _, err := mktSvc.InitiateRefund(ctx, admin, orderRes.Order.ID, service.RefundOrderInput{
		AmountKobo: 400000,
		Reason:     "Retry refund with new details",
	})
	if err != nil {
		t.Fatalf("expected new refund to be accepted after previous failure released reservation, got %v", err)
	}
	if rNew.Status != "pending" {
		t.Errorf("expected new refund to be pending, got %s", rNew.Status)
	}
}

// 14. Duplicate refund notifications are harmless (Idempotency)
func TestMarketplaceRefund_DuplicateRefundNotifications(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	admin := createRegisteredUser(t, env, fmt.Sprintf("admin_dup_%d", time.Now().UnixNano()), "super_admin")
	gw := &mockCustomGateway{providerName: "paystack"}
	mktSvc, orderRes, _ := helperCreateOrderWithGateway(t, env, gw, 300000)

	gw.verifyFn = func(ctx context.Context, reference string) (*payment.VerifyPaymentOutput, error) {
		now := time.Now()
		return &payment.VerifyPaymentOutput{
			PaymentReference:  reference,
			ProviderReference: "pstk_tx_" + reference,
			Status:            "successful",
			AmountKobo:        300000,
			Currency:          "NGN",
			PaidAt:            &now,
		}, nil
	}
	_, err := mktSvc.ConfirmPayment(ctx, orderRes.Payment.Reference)
	if err != nil {
		t.Fatalf("ConfirmPayment failed: %v", err)
	}

	r, _, err := mktSvc.InitiateRefund(ctx, admin, orderRes.Order.ID, service.RefundOrderInput{
		AmountKobo: 300000,
		Reason:     "Refund item",
	})
	if err != nil {
		t.Fatalf("InitiateRefund failed: %v", err)
	}

	payload := []byte(fmt.Sprintf(`{
		"event": "refund.processed",
		"data": {
			"reference": "%s",
			"refund_reference": "%s",
			"status": "processed",
			"amount": 300000,
			"currency": "NGN"
		}
	}`, orderRes.Payment.Reference, r.ProviderRefundReference))

	// First webhook notification
	err = mktSvc.HandleWebhook(ctx, "valid-paystack-signature", payload)
	if err != nil {
		t.Fatalf("first refund.processed webhook failed: %v", err)
	}

	// Second webhook notification (exact duplicate)
	err = mktSvc.HandleWebhook(ctx, "valid-paystack-signature", payload)
	if err != nil {
		t.Fatalf("second refund.processed webhook failed: %v", err)
	}

	// Total successful refunds should still equal exactly 300,000 (not 600,000!)
	sums, err := env.Repo.Queries().GetMarketplaceRefundSumsByOrder(ctx, orderRes.Order.ID)
	if err != nil {
		t.Fatalf("GetMarketplaceRefundSumsByOrder failed: %v", err)
	}
	if sums.SuccessfulAmountKobo != 300000 {
		t.Errorf("expected 300000 kobo successful refunds, got %d", sums.SuccessfulAmountKobo)
	}
}

// 15. Original Paystack verification response is preserved in marketplace_payments
func TestMarketplaceRefund_OriginalPaystackPayloadPreserved(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	admin := createRegisteredUser(t, env, fmt.Sprintf("admin_pres_%d", time.Now().UnixNano()), "super_admin")
	gw := &mockCustomGateway{providerName: "paystack"}
	mktSvc, orderRes, _ := helperCreateOrderWithGateway(t, env, gw, 250000)

	originalPayload := []byte(`{"status":"success","data":{"id":998877,"authoritative_paystack_snapshot":"PRISTINE"}}`)

	gw.verifyFn = func(ctx context.Context, reference string) (*payment.VerifyPaymentOutput, error) {
		now := time.Now()
		return &payment.VerifyPaymentOutput{
			PaymentReference:  reference,
			ProviderReference: "pstk_tx_" + reference,
			Status:            "successful",
			AmountKobo:        250000,
			Currency:          "NGN",
			PaidAt:            &now,
			RawResponse:       originalPayload,
		}, nil
	}
	_, err := mktSvc.ConfirmPayment(ctx, orderRes.Payment.Reference)
	if err != nil {
		t.Fatalf("ConfirmPayment failed: %v", err)
	}

	// Capture payment record before refunds
	payBefore, err := env.Repo.Queries().GetMarketplacePaymentByOrder(ctx, orderRes.Order.ID)
	if err != nil {
		t.Fatalf("GetMarketplacePaymentByOrder failed: %v", err)
	}

	// Initiate and confirm a partial refund
	r, _, err := mktSvc.InitiateRefund(ctx, admin, orderRes.Order.ID, service.RefundOrderInput{
		AmountKobo: 100000,
		Reason:     "Partial refund",
	})
	if err != nil {
		t.Fatalf("InitiateRefund failed: %v", err)
	}
	_, _, err = mktSvc.ConfirmRefund(ctx, admin, r.ID, service.ConfirmRefundInput{})
	if err != nil {
		t.Fatalf("ConfirmRefund failed: %v", err)
	}

	// Check payment record raw_webhook_payload
	payAfter, err := env.Repo.Queries().GetMarketplacePaymentByOrder(ctx, orderRes.Order.ID)
	if err != nil {
		t.Fatalf("GetMarketplacePaymentByOrder failed: %v", err)
	}

	if string(payAfter.RawWebhookPayload) != string(payBefore.RawWebhookPayload) {
		t.Errorf("marketplace_payments.raw_webhook_payload was modified! Expected %s, got %s",
			string(payBefore.RawWebhookPayload), string(payAfter.RawWebhookPayload))
	}
	if !strings.Contains(string(payAfter.RawWebhookPayload), "PRISTINE") {
		t.Errorf("expected original payload content to be preserved, got %s", string(payAfter.RawWebhookPayload))
	}
}

// 16. Unauthorized refund attempts
func TestMarketplaceRefund_UnauthorizedAttempts(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	gw := &mockCustomGateway{providerName: "paystack"}
	mktSvc, orderRes, _ := helperCreateOrderWithGateway(t, env, gw, 100000)

	unrelatedUser := createRegisteredUser(t, env, fmt.Sprintf("unrelated_%d", time.Now().UnixNano()), "member")

	// 1. Unrelated non-admin tries InitiateRefund
	_, _, err := mktSvc.InitiateRefund(ctx, unrelatedUser, orderRes.Order.ID, service.RefundOrderInput{
		AmountKobo: 50000,
		Reason:     "Unauthorized attempt",
	})
	if err == nil {
		t.Fatal("expected unauthorized error for non-admin InitiateRefund, got nil")
	}

	// 2. Unrelated non-admin tries ConfirmRefund
	_, _, err = mktSvc.ConfirmRefund(ctx, unrelatedUser, "dummy-id", service.ConfirmRefundInput{})
	if err == nil {
		t.Fatal("expected unauthorized error for non-admin ConfirmRefund, got nil")
	}

	// 3. Unrelated non-admin tries GetRefundByID
	_, err = mktSvc.GetRefundByID(ctx, unrelatedUser, "dummy-id")
	if err == nil {
		t.Fatal("expected unauthorized error for non-admin GetRefundByID, got nil")
	}
}

// 17. Database failure during confirmation proving atomic rollback
func TestMarketplacePaymentIntegrity_AtomicRollbackOnDatabaseFailure(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	gw := &mockCustomGateway{providerName: "paystack"}
	mktSvc, orderRes, buyer := helperCreateOrderWithGateway(t, env, gw, 200000)

	gw.verifyFn = func(ctx context.Context, reference string) (*payment.VerifyPaymentOutput, error) {
		now := time.Now()
		return &payment.VerifyPaymentOutput{
			PaymentReference:  reference,
			ProviderReference: "pstk_tx_" + reference,
			Status:            "successful",
			AmountKobo:        200000,
			Currency:          "NGN",
			PaidAt:            &now,
		}, nil
	}

	// Create an already-cancelled context to simulate DB operation failure
	cancelledCtx, cancel := context.WithCancel(ctx)
	cancel()

	_, err := mktSvc.ConfirmPayment(cancelledCtx, orderRes.Payment.Reference)
	if err == nil {
		t.Fatal("expected error on cancelled context / DB failure, got nil")
	}

	// Verify with a valid context that state was rolled back and untouched
	order, _, _, err := mktSvc.GetOrderByID(ctx, buyer, orderRes.Order.ID)
	if err != nil {
		t.Fatalf("GetOrderByID failed: %v", err)
	}
	if order.PaymentStatus != "pending" {
		t.Errorf("expected payment status to remain 'pending' after rollback, got '%s'", order.PaymentStatus)
	}
	if order.OrderStatus != "placed" {
		t.Errorf("expected order status to remain 'placed' after rollback, got '%s'", order.OrderStatus)
	}
}
