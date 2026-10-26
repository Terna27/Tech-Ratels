package service_test

import (
	"context"
	"errors"
	"fmt"
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

// 10. Partial and full refund boundaries and bounds enforcement
func TestMarketplacePaymentIntegrity_RefundBoundaries(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	admin := createRegisteredUser(t, env, fmt.Sprintf("admin_ref_%d", time.Now().UnixNano()), "super_admin")
	gw := &mockCustomGateway{providerName: "paystack"}
	mktSvc, orderRes, _ := helperCreateOrderWithGateway(t, env, gw, 600000) // Total ₦6,000

	// Pay the order
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

	// Boundary 1: First partial refund (200,000 kobo)
	part1, err := mktSvc.InitiateRefund(ctx, admin, orderRes.Order.ID, service.RefundOrderInput{
		AmountKobo: 200000,
		Reason:     "First partial concession",
	})
	if err != nil {
		t.Fatalf("first partial refund failed: %v", err)
	}
	if part1.PaymentStatus != "partially_refunded" {
		t.Errorf("expected 'partially_refunded', got %s", part1.PaymentStatus)
	}

	// Boundary 2: Excessive partial refund (500,000 kobo when remaining eligible is 400,000 kobo)
	_, err = mktSvc.InitiateRefund(ctx, admin, orderRes.Order.ID, service.RefundOrderInput{
		AmountKobo: 500000,
		Reason:     "Over-refund exploit attempt",
	})
	if err == nil {
		t.Fatal("expected error when refund exceeds remaining eligible amount, got nil")
	}

	// Boundary 3: Exact remaining balance refund (400,000 kobo)
	fullOrder, err := mktSvc.InitiateRefund(ctx, admin, orderRes.Order.ID, service.RefundOrderInput{
		AmountKobo: 400000,
		Reason:     "Final settlement refund",
	})
	if err != nil {
		t.Fatalf("final settlement refund failed: %v", err)
	}
	if fullOrder.PaymentStatus != "refunded" || fullOrder.OrderStatus != "refunded" {
		t.Errorf("expected payment_status=refunded, order_status=refunded, got %s / %s",
			fullOrder.PaymentStatus, fullOrder.OrderStatus)
	}

	// Boundary 4: Duplicate refund attempt on already fully refunded order
	_, err = mktSvc.InitiateRefund(ctx, admin, orderRes.Order.ID, service.RefundOrderInput{
		AmountKobo: 100000,
		Reason:     "Duplicate refund",
	})
	if err == nil {
		t.Fatal("expected error when refunding an already refunded order, got nil")
	}
}

// 11. Unauthorized order and payment access protection
func TestMarketplacePaymentIntegrity_UnauthorizedAccess(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	gw := &mockCustomGateway{providerName: "paystack"}
	mktSvc, orderRes, _ := helperCreateOrderWithGateway(t, env, gw, 100000)

	// An unrelated normal member
	unrelatedUser := createRegisteredUser(t, env, fmt.Sprintf("unrelated_%d", time.Now().UnixNano()), "member")

	// 1. Unrelated user attempts to read Buyer's order
	_, _, _, err := mktSvc.GetOrderByID(ctx, unrelatedUser, orderRes.Order.ID)
	if err == nil {
		t.Fatal("expected ErrUnauthorizedBuyerAction for unrelated user viewing order, got nil")
	}
	if !errors.Is(err, service.ErrUnauthorizedBuyerAction) {
		t.Fatalf("expected ErrUnauthorizedBuyerAction, got %v", err)
	}

	// 2. Unrelated user attempts to list marketplace payments (admin-only)
	_, err = mktSvc.ListPayments(ctx, unrelatedUser, nil, nil)
	if err == nil {
		t.Fatal("expected unauthorized error for non-admin listing payments, got nil")
	}

	// 3. Unrelated non-admin user attempts to initiate a refund
	_, err = mktSvc.InitiateRefund(ctx, unrelatedUser, orderRes.Order.ID, service.RefundOrderInput{
		AmountKobo: 100000,
		Reason:     "Malicious refund",
	})
	if err == nil {
		t.Fatal("expected unauthorized error for non-admin initiating refund, got nil")
	}
}

// 12. Database failure during confirmation proving atomic rollback
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
