package service_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"ratels-api/internal/db"
	"ratels-api/internal/payment"
	"ratels-api/internal/security"
	"ratels-api/internal/service"
)

// Helper mock gateway for inventory & expiry testing
type mockInventoryExpiryGateway struct {
	providerName string
	verifyFn     func(ctx context.Context, reference string) (*payment.VerifyPaymentOutput, error)
	refundFn     func(ctx context.Context, input payment.RefundPaymentInput) (*payment.RefundPaymentOutput, error)
}

func (m *mockInventoryExpiryGateway) ProviderName() string {
	if m.providerName == "" {
		return "paystack"
	}
	return m.providerName
}

func (m *mockInventoryExpiryGateway) InitializePayment(ctx context.Context, input payment.InitializePaymentInput) (*payment.InitializePaymentOutput, error) {
	return &payment.InitializePaymentOutput{
		CheckoutURL:      "https://checkout.paystack.com/" + input.OrderReference,
		PaymentReference: input.OrderReference,
	}, nil
}

func (m *mockInventoryExpiryGateway) VerifyPayment(ctx context.Context, reference string) (*payment.VerifyPaymentOutput, error) {
	if m.verifyFn != nil {
		return m.verifyFn(ctx, reference)
	}
	now := time.Now()
	return &payment.VerifyPaymentOutput{
		PaymentReference:  reference,
		ProviderReference: "ps_ref_" + reference,
		Status:            "successful",
		AmountKobo:        100000,
		Currency:          "NGN",
		PaidAt:            &now,
		Channel:           "card",
		CustomerEmail:     "buyer@example.org",
	}, nil
}

func (m *mockInventoryExpiryGateway) VerifyWebhookSignature(signatureHeader string, body []byte) bool {
	return true
}

func (m *mockInventoryExpiryGateway) RefundPayment(ctx context.Context, input payment.RefundPaymentInput) (*payment.RefundPaymentOutput, error) {
	if m.refundFn != nil {
		return m.refundFn(ctx, input)
	}
	return &payment.RefundPaymentOutput{
		RefundReference: "mock_ref_" + security.GenerateUUID()[:8],
		Status:          "successful",
		AmountKobo:      input.AmountKobo,
		Currency:        "NGN",
	}, nil
}

func (m *mockInventoryExpiryGateway) VerifyRefund(ctx context.Context, refundReference string) (*payment.VerifyRefundOutput, error) {
	return &payment.VerifyRefundOutput{
		RefundReference: refundReference,
		Status:          "successful",
		AmountKobo:      100000,
		Currency:        "NGN",
	}, nil
}

// 1. Successful inventory reservation
func TestInventory_SuccessfulReservation(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	admin := createRegisteredUser(t, env, "inv_admin_1", "super_admin")
	seller, _, prod := helperCreateSellerWithProduct(t, env, ctx, "inv_seller_1", "Store 1", "Hammer", 50, admin)
	buyer := createRegisteredUser(t, env, "inv_buyer_1", "member")

	gw := &mockInventoryExpiryGateway{providerName: "paystack"}
	mktSvc := service.NewMarketplaceService(env.Repo, env.AuditService, gw)
	_, _ = env.Pool.Exec(ctx, "UPDATE marketplace_orders SET reservation_expires_at = NOW() + INTERVAL '24 hours' WHERE order_status = 'placed' AND stock_released = FALSE")

	orderRes, err := mktSvc.CreateOrder(ctx, buyer, service.CheckoutInput{
		Items:        []service.CartItemInput{{ProductID: prod.ID, Quantity: 5}},
		BuyerName:    "Buyer One",
		BuyerEmail:   "buyer1@example.org",
		BuyerPhone:   "+2348000000001",
		DeliveryType: "pickup",
	})
	if err != nil {
		t.Fatalf("CreateOrder failed: %v", err)
	}

	// Verify product stock decremented
	pAfter, err := mktSvc.GetProductByID(ctx, prod.ID)
	if err != nil {
		t.Fatalf("GetProductByID failed: %v", err)
	}
	if pAfter.StockQuantity != 45 {
		t.Fatalf("Expected stock to be 45, got %d", pAfter.StockQuantity)
	}

	// Verify order reservation fields
	ord := orderRes.Order
	if !ord.ReservationExpiresAt.Valid {
		t.Fatalf("Expected valid ReservationExpiresAt")
	}
	if ord.ReservationExpiresAt.Time.Before(time.Now().Add(25 * time.Minute)) {
		t.Fatalf("Expected ReservationExpiresAt to be ~30m in future, got %v", ord.ReservationExpiresAt.Time)
	}
	if ord.StockReleased {
		t.Fatalf("Expected StockReleased to be false")
	}

	// Verify ledger entry
	logs, err := mktSvc.ListProductInventoryLogs(ctx, seller, prod.ID)
	if err != nil {
		t.Fatalf("ListProductInventoryLogs failed: %v", err)
	}
	if len(logs) == 0 {
		t.Fatalf("Expected at least one inventory log")
	}
	resLog := logs[0]
	if resLog.ActionType != "reservation" {
		t.Errorf("Expected action_type 'reservation', got %s", resLog.ActionType)
	}
	if resLog.QuantityDelta != -5 {
		t.Errorf("Expected quantity_delta -5, got %d", resLog.QuantityDelta)
	}
	if resLog.BalanceAfter != 45 {
		t.Errorf("Expected balance_after 45, got %d", resLog.BalanceAfter)
	}
	if !resLog.OrderID.Valid || resLog.OrderID.String != ord.ID {
		t.Errorf("Expected order_id %s, got %v", ord.ID, resLog.OrderID)
	}
}

// 2. Insufficient stock rejection
func TestInventory_InsufficientStockRejection(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	admin := createRegisteredUser(t, env, "inv_admin_2", "super_admin")
	seller, _, prod := helperCreateSellerWithProduct(t, env, ctx, "inv_seller_2", "Store 2", "Nails", 3, admin)
	buyer := createRegisteredUser(t, env, "inv_buyer_2", "member")

	gw := &mockInventoryExpiryGateway{providerName: "paystack"}
	mktSvc := service.NewMarketplaceService(env.Repo, env.AuditService, gw)

	// Attempt to order 5 when only 3 in stock
	_, err := mktSvc.CreateOrder(ctx, buyer, service.CheckoutInput{
		Items:        []service.CartItemInput{{ProductID: prod.ID, Quantity: 5}},
		BuyerName:    "Buyer Two",
		BuyerEmail:   "buyer2@example.org",
		BuyerPhone:   "+2348000000002",
		DeliveryType: "pickup",
	})
	if err == nil {
		t.Fatalf("Expected error for insufficient stock, got nil")
	}
	if !errors.Is(err, service.ErrInsufficientStock) {
		t.Fatalf("Expected ErrInsufficientStock, got: %v", err)
	}

	// Stock must remain unchanged at 3
	pAfter, err := mktSvc.GetProductByID(ctx, prod.ID)
	if err != nil {
		t.Fatalf("GetProductByID failed: %v", err)
	}
	if pAfter.StockQuantity != 3 {
		t.Fatalf("Expected stock to remain 3, got %d", pAfter.StockQuantity)
	}

	// No inventory logs created
	logs, err := mktSvc.ListProductInventoryLogs(ctx, seller, prod.ID)
	if err != nil {
		t.Fatalf("ListProductInventoryLogs failed: %v", err)
	}
	if len(logs) != 0 {
		t.Fatalf("Expected 0 inventory logs, got %d", len(logs))
	}
}

// 3. Concurrent purchases without overselling
func TestInventory_ConcurrentPurchasesWithoutOverselling(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	admin := createRegisteredUser(t, env, "inv_admin_3", "super_admin")
	_, _, prod := helperCreateSellerWithProduct(t, env, ctx, "inv_seller_3", "Store 3", "Flashlight", 5, admin)

	gw := &mockInventoryExpiryGateway{providerName: "paystack"}
	mktSvc := service.NewMarketplaceService(env.Repo, env.AuditService, gw)

	var successCount int64
	var failureCount int64

	var wg sync.WaitGroup
	totalBuyers := 10
	wg.Add(totalBuyers)

	for i := 0; i < totalBuyers; i++ {
		go func(idx int) {
			defer wg.Done()
			b := createRegisteredUser(t, env, fmt.Sprintf("buyer_race_%d_%d", idx, time.Now().UnixNano()), "member")
			_, err := mktSvc.CreateOrder(ctx, b, service.CheckoutInput{
				Items:        []service.CartItemInput{{ProductID: prod.ID, Quantity: 1}},
				BuyerName:    fmt.Sprintf("Buyer %d", idx),
				BuyerEmail:   fmt.Sprintf("buyer_%d@example.org", idx),
				BuyerPhone:   "+2348000000000",
				DeliveryType: "pickup",
			})
			if err == nil {
				atomic.AddInt64(&successCount, 1)
			} else {
				atomic.AddInt64(&failureCount, 1)
			}
		}(i)
	}

	wg.Wait()

	if successCount != 5 {
		t.Errorf("Expected exactly 5 successful orders, got %d", successCount)
	}
	if failureCount != 5 {
		t.Errorf("Expected exactly 5 failed orders, got %d", failureCount)
	}

	pAfter, err := mktSvc.GetProductByID(ctx, prod.ID)
	if err != nil {
		t.Fatalf("GetProductByID failed: %v", err)
	}
	if pAfter.StockQuantity != 0 {
		t.Fatalf("Expected final stock to be 0, got %d", pAfter.StockQuantity)
	}
}

// 4. Abandoned checkout expiry
func TestInventory_AbandonedCheckoutExpiry(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	admin := createRegisteredUser(t, env, "inv_admin_4", "super_admin")
	seller, _, prod := helperCreateSellerWithProduct(t, env, ctx, "inv_seller_4", "Store 4", "Tape", 20, admin)
	buyer := createRegisteredUser(t, env, "inv_buyer_4", "member")

	// Gateway returns unpaid/pending for payment check
	gw := &mockInventoryExpiryGateway{
		providerName: "paystack",
		verifyFn: func(ctx context.Context, reference string) (*payment.VerifyPaymentOutput, error) {
			return &payment.VerifyPaymentOutput{
				PaymentReference: reference,
				Status:           "abandoned",
			}, nil
		},
	}
	mktSvc := service.NewMarketplaceService(env.Repo, env.AuditService, gw)
	_, _ = env.Pool.Exec(ctx, "UPDATE marketplace_orders SET reservation_expires_at = NOW() + INTERVAL '24 hours' WHERE order_status = 'placed' AND stock_released = FALSE")

	orderRes, err := mktSvc.CreateOrder(ctx, buyer, service.CheckoutInput{
		Items:        []service.CartItemInput{{ProductID: prod.ID, Quantity: 3}},
		BuyerName:    "Buyer Four",
		BuyerEmail:   "buyer4@example.org",
		BuyerPhone:   "+2348000000004",
		DeliveryType: "pickup",
	})
	if err != nil {
		t.Fatalf("CreateOrder failed: %v", err)
	}

	// Stock is now 17
	pMid, _ := mktSvc.GetProductByID(ctx, prod.ID)
	if pMid.StockQuantity != 17 {
		t.Fatalf("Expected intermediate stock 17, got %d", pMid.StockQuantity)
	}

	// Simulate reservation expiry: set reservation_expires_at to 1 hour ago
	_, err = env.Pool.Exec(ctx, "UPDATE marketplace_orders SET reservation_expires_at = NOW() - INTERVAL '1 hour' WHERE id = $1", orderRes.Order.ID)
	if err != nil {
		t.Fatalf("Failed to expire order in DB: %v", err)
	}

	// Run expiration
	res, err := mktSvc.ExpireStaleReservations(ctx, 50)
	if err != nil {
		t.Fatalf("ExpireStaleReservations failed: %v", err)
	}
	if res.ExpiredCount != 1 {
		t.Fatalf("Expected ExpiredCount 1, got %d", res.ExpiredCount)
	}

	// Verify order status and stock release
	ordAfter, err := env.Repo.Queries().GetMarketplaceOrderByID(ctx, orderRes.Order.ID)
	if err != nil {
		t.Fatalf("GetOrderByID failed: %v", err)
	}
	if ordAfter.OrderStatus != "cancelled" {
		t.Errorf("Expected order status 'cancelled', got %s", ordAfter.OrderStatus)
	}
	if ordAfter.CancellationReason != "payment_timeout" {
		t.Errorf("Expected cancellation_reason 'payment_timeout', got %s", ordAfter.CancellationReason)
	}
	if !ordAfter.StockReleased {
		t.Errorf("Expected stock_released = true")
	}

	// Verify stock restored to 20
	pAfter, _ := mktSvc.GetProductByID(ctx, prod.ID)
	if pAfter.StockQuantity != 20 {
		t.Fatalf("Expected restored stock 20, got %d", pAfter.StockQuantity)
	}

	// Verify ledger release log
	logs, err := mktSvc.ListProductInventoryLogs(ctx, seller, prod.ID)
	if err != nil {
		t.Fatalf("ListProductInventoryLogs failed: %v", err)
	}
	if len(logs) < 2 {
		t.Fatalf("Expected at least 2 logs (reserve and release), got %d", len(logs))
	}
	releaseLog := logs[0] // most recent
	if releaseLog.ActionType != "reservation_release" {
		t.Errorf("Expected action_type 'reservation_release', got %s", releaseLog.ActionType)
	}
	if releaseLog.QuantityDelta != 3 {
		t.Errorf("Expected quantity_delta +3, got %d", releaseLog.QuantityDelta)
	}
	if releaseLog.BalanceAfter != 20 {
		t.Errorf("Expected balance_after 20, got %d", releaseLog.BalanceAfter)
	}
}

// 5. Duplicate expiry worker executions (idempotency)
func TestInventory_DuplicateExpiryIdempotency(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	admin := createRegisteredUser(t, env, "inv_admin_5", "super_admin")
	_, _, prod := helperCreateSellerWithProduct(t, env, ctx, "inv_seller_5", "Store 5", "Rope", 10, admin)
	buyer := createRegisteredUser(t, env, "inv_buyer_5", "member")

	gw := &mockInventoryExpiryGateway{
		providerName: "paystack",
		verifyFn: func(ctx context.Context, reference string) (*payment.VerifyPaymentOutput, error) {
			return &payment.VerifyPaymentOutput{PaymentReference: reference, Status: "abandoned"}, nil
		},
	}
	mktSvc := service.NewMarketplaceService(env.Repo, env.AuditService, gw)
	_, _ = env.Pool.Exec(ctx, "UPDATE marketplace_orders SET reservation_expires_at = NOW() + INTERVAL '24 hours' WHERE order_status = 'placed' AND stock_released = FALSE")

	orderRes, err := mktSvc.CreateOrder(ctx, buyer, service.CheckoutInput{
		Items:        []service.CartItemInput{{ProductID: prod.ID, Quantity: 2}},
		BuyerName:    "Buyer Five",
		BuyerEmail:   "buyer5@example.org",
		BuyerPhone:   "+2348000000005",
		DeliveryType: "pickup",
	})
	if err != nil {
		t.Fatalf("CreateOrder failed: %v", err)
	}

	_, _ = env.Pool.Exec(ctx, "UPDATE marketplace_orders SET reservation_expires_at = NOW() - INTERVAL '1 hour' WHERE id = $1", orderRes.Order.ID)

	// First execution expires order
	res1, err := mktSvc.ExpireStaleReservations(ctx, 50)
	if err != nil || res1.ExpiredCount != 1 {
		t.Fatalf("First run expected 1 expired, got %d (err: %v)", res1.ExpiredCount, err)
	}

	// Second execution must be a complete no-op
	res2, err := mktSvc.ExpireStaleReservations(ctx, 50)
	if err != nil {
		t.Fatalf("Second run failed: %v", err)
	}
	if res2.ExpiredCount != 0 {
		t.Errorf("Second run expected 0 expired, got %d", res2.ExpiredCount)
	}

	// Stock must remain 10, not double incremented to 12
	pAfter, _ := mktSvc.GetProductByID(ctx, prod.ID)
	if pAfter.StockQuantity != 10 {
		t.Fatalf("Expected stock to remain 10, got %d", pAfter.StockQuantity)
	}
}

// 6. Successful payment before expiry
func TestInventory_SuccessfulPaymentBeforeExpiry(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	admin := createRegisteredUser(t, env, "inv_admin_6", "super_admin")
	seller, _, prod := helperCreateSellerWithProduct(t, env, ctx, "inv_seller_6", "Store 6", "Pliers", 10, admin)
	buyer := createRegisteredUser(t, env, "inv_buyer_6", "member")

	gw := &mockInventoryExpiryGateway{
		providerName: "paystack",
		verifyFn: func(ctx context.Context, reference string) (*payment.VerifyPaymentOutput, error) {
			now := time.Now()
			return &payment.VerifyPaymentOutput{
				PaymentReference:  reference,
				ProviderReference: "ps_" + reference,
				Status:            "successful",
				AmountKobo:        250000,
				Currency:          "NGN",
				PaidAt:            &now,
				Channel:           "card",
			}, nil
		},
	}
	mktSvc := service.NewMarketplaceService(env.Repo, env.AuditService, gw)
	_, _ = env.Pool.Exec(ctx, "UPDATE marketplace_orders SET reservation_expires_at = NOW() + INTERVAL '24 hours' WHERE order_status = 'placed' AND stock_released = FALSE")

	orderRes, err := mktSvc.CreateOrder(ctx, buyer, service.CheckoutInput{
		Items:        []service.CartItemInput{{ProductID: prod.ID, Quantity: 1}},
		BuyerName:    "Buyer Six",
		BuyerEmail:   "buyer6@example.org",
		BuyerPhone:   "+2348000000006",
		DeliveryType: "pickup",
	})
	if err != nil {
		t.Fatalf("CreateOrder failed: %v", err)
	}

	// Confirm payment
	confOrder, err := mktSvc.ConfirmPayment(ctx, orderRes.Payment.Reference)
	if err != nil {
		t.Fatalf("ConfirmPayment failed: %v", err)
	}
	if confOrder.PaymentStatus != "paid" || confOrder.OrderStatus != "processing" {
		t.Fatalf("Expected paid and processing, got %s / %s", confOrder.PaymentStatus, confOrder.OrderStatus)
	}

	// Stock must remain 9 (no double decrement!)
	pAfter, _ := mktSvc.GetProductByID(ctx, prod.ID)
	if pAfter.StockQuantity != 9 {
		t.Fatalf("Expected stock to remain 9, got %d", pAfter.StockQuantity)
	}

	// Ledger should have sale_confirmed entry
	logs, err := mktSvc.ListProductInventoryLogs(ctx, seller, prod.ID)
	if err != nil {
		t.Fatalf("ListProductInventoryLogs failed: %v", err)
	}
	var foundSaleConfirmed bool
	for _, l := range logs {
		if l.ActionType == "sale_confirmed" {
			foundSaleConfirmed = true
			if l.QuantityDelta != 0 {
				t.Errorf("Expected quantity_delta 0 for sale_confirmed, got %d", l.QuantityDelta)
			}
			if l.BalanceAfter != 9 {
				t.Errorf("Expected balance_after 9, got %d", l.BalanceAfter)
			}
		}
	}
	if !foundSaleConfirmed {
		t.Errorf("Expected sale_confirmed inventory log entry")
	}

	// Set reservation expiry to the past; ExpireStaleReservations must not touch paid orders
	_, _ = env.Pool.Exec(ctx, "UPDATE marketplace_orders SET reservation_expires_at = NOW() - INTERVAL '1 hour' WHERE id = $1", confOrder.ID)
	res, err := mktSvc.ExpireStaleReservations(ctx, 50)
	if err != nil {
		t.Fatalf("ExpireStaleReservations failed: %v", err)
	}
	if res.ExpiredCount != 0 {
		t.Errorf("Expected 0 expired for paid order, got %d", res.ExpiredCount)
	}
}

// 7. Payment and expiry racing concurrently
func TestInventory_PaymentAndExpiryRace(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	admin := createRegisteredUser(t, env, "inv_admin_7", "super_admin")
	_, _, prod := helperCreateSellerWithProduct(t, env, ctx, "inv_seller_7", "Store 7", "Wrench", 20, admin)
	buyer := createRegisteredUser(t, env, "inv_buyer_7", "member")

	gw := &mockInventoryExpiryGateway{
		providerName: "paystack",
		verifyFn: func(ctx context.Context, reference string) (*payment.VerifyPaymentOutput, error) {
			now := time.Now()
			return &payment.VerifyPaymentOutput{
				PaymentReference:  reference,
				ProviderReference: "ps_" + reference,
				Status:            "successful",
				AmountKobo:        500000,
				Currency:          "NGN",
				PaidAt:            &now,
				Channel:           "card",
			}, nil
		},
	}
	mktSvc := service.NewMarketplaceService(env.Repo, env.AuditService, gw)
	_, _ = env.Pool.Exec(ctx, "UPDATE marketplace_orders SET reservation_expires_at = NOW() + INTERVAL '24 hours' WHERE order_status = 'placed' AND stock_released = FALSE")

	orderRes, err := mktSvc.CreateOrder(ctx, buyer, service.CheckoutInput{
		Items:        []service.CartItemInput{{ProductID: prod.ID, Quantity: 2}},
		BuyerName:    "Buyer Seven",
		BuyerEmail:   "buyer7@example.org",
		BuyerPhone:   "+2348000000007",
		DeliveryType: "pickup",
	})
	if err != nil {
		t.Fatalf("CreateOrder failed: %v", err)
	}

	// Set order expiry to past
	_, _ = env.Pool.Exec(ctx, "UPDATE marketplace_orders SET reservation_expires_at = NOW() - INTERVAL '1 hour' WHERE id = $1", orderRes.Order.ID)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		_, _ = mktSvc.ConfirmPayment(ctx, orderRes.Payment.Reference)
	}()

	go func() {
		defer wg.Done()
		_, _ = mktSvc.ExpireStaleReservations(ctx, 50)
	}()

	wg.Wait()

	// Check final state: Must be cleanly resolved
	ord, err := env.Repo.Queries().GetMarketplaceOrderByID(ctx, orderRes.Order.ID)
	if err != nil {
		t.Fatalf("GetOrderByID failed: %v", err)
	}

	pAfter, _ := mktSvc.GetProductByID(ctx, prod.ID)

	if ord.OrderStatus == "processing" {
		// Payment won before expiry released stock: Stock must be 18
		if pAfter.StockQuantity != 18 {
			t.Errorf("Expected stock 18 when payment won, got %d", pAfter.StockQuantity)
		}
	} else if ord.OrderStatus == "cancelled" || ord.OrderStatus == "requires_reconciliation" {
		// Expiry ran and released stock (stock must be 20)
		if pAfter.StockQuantity != 20 {
			t.Errorf("Expected stock 20 when stock released, got %d", pAfter.StockQuantity)
		}
	} else {
		t.Errorf("Unexpected terminal order status: %s", ord.OrderStatus)
	}
}

// 8. Delayed Paystack success after expiry (reconciliation required)
func TestInventory_DelayedPaymentAfterExpiry(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	admin := createRegisteredUser(t, env, "inv_admin_8", "super_admin")
	_, _, prod := helperCreateSellerWithProduct(t, env, ctx, "inv_seller_8", "Store 8", "Level", 15, admin)
	buyer := createRegisteredUser(t, env, "inv_buyer_8", "member")

	verifyCalled := false
	gw := &mockInventoryExpiryGateway{
		providerName: "paystack",
		verifyFn: func(ctx context.Context, reference string) (*payment.VerifyPaymentOutput, error) {
			if !verifyCalled {
				// During expiry check, customer had not yet paid
				return &payment.VerifyPaymentOutput{PaymentReference: reference, Status: "abandoned"}, nil
			}
			// Delayed verification: customer paid outside window!
			now := time.Now()
			return &payment.VerifyPaymentOutput{
				PaymentReference:  reference,
				ProviderReference: "ps_delayed_" + reference,
				Status:            "successful",
				AmountKobo:        750000,
				Currency:          "NGN",
				PaidAt:            &now,
				Channel:           "card",
			}, nil
		},
	}
	mktSvc := service.NewMarketplaceService(env.Repo, env.AuditService, gw)
	_, _ = env.Pool.Exec(ctx, "UPDATE marketplace_orders SET reservation_expires_at = NOW() + INTERVAL '24 hours' WHERE order_status = 'placed' AND stock_released = FALSE")

	orderRes, err := mktSvc.CreateOrder(ctx, buyer, service.CheckoutInput{
		Items:        []service.CartItemInput{{ProductID: prod.ID, Quantity: 3}},
		BuyerName:    "Buyer Eight",
		BuyerEmail:   "buyer8@example.org",
		BuyerPhone:   "+2348000000008",
		DeliveryType: "pickup",
	})
	if err != nil {
		t.Fatalf("CreateOrder failed: %v", err)
	}

	// Stock is 12
	// Expire order
	_, _ = env.Pool.Exec(ctx, "UPDATE marketplace_orders SET reservation_expires_at = NOW() - INTERVAL '1 hour' WHERE id = $1", orderRes.Order.ID)
	res, err := mktSvc.ExpireStaleReservations(ctx, 50)
	if err != nil || res.ExpiredCount != 1 {
		t.Fatalf("Expected 1 expired, got %d (err: %v)", res.ExpiredCount, err)
	}

	// Stock is restored to 15
	pAfterExpiry, _ := mktSvc.GetProductByID(ctx, prod.ID)
	if pAfterExpiry.StockQuantity != 15 {
		t.Fatalf("Expected stock 15 after expiry, got %d", pAfterExpiry.StockQuantity)
	}

	// Now delayed payment webhook / verification arrives
	verifyCalled = true
	confOrder, err := mktSvc.ConfirmPayment(ctx, orderRes.Payment.Reference)
	if err != nil {
		t.Fatalf("ConfirmPayment failed on delayed payment: %v", err)
	}

	// Order must enter 'requires_reconciliation' state
	if confOrder.OrderStatus != "requires_reconciliation" {
		t.Errorf("Expected order status 'requires_reconciliation', got %s", confOrder.OrderStatus)
	}
	if !confOrder.ReconciliationRequired {
		t.Errorf("Expected reconciliation_required = true")
	}
	if confOrder.PaymentStatus != "paid" {
		t.Errorf("Expected payment status 'paid', got %s", confOrder.PaymentStatus)
	}

	// Stock must NOT be deducted again (must remain 15)
	pAfterDelayed, _ := mktSvc.GetProductByID(ctx, prod.ID)
	if pAfterDelayed.StockQuantity != 15 {
		t.Fatalf("Expected stock to remain 15, got %d", pAfterDelayed.StockQuantity)
	}
}

// 9. Provider verification unavailable during expiry check (flags reconciliation, no cancellation)
func TestInventory_ProviderUnreachableDuringExpiry(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	admin := createRegisteredUser(t, env, "inv_admin_9", "super_admin")
	_, _, prod := helperCreateSellerWithProduct(t, env, ctx, "inv_seller_9", "Store 9", "Saw", 10, admin)
	buyer := createRegisteredUser(t, env, "inv_buyer_9", "member")

	// Gateway returns network timeout error
	gw := &mockInventoryExpiryGateway{
		providerName: "paystack",
		verifyFn: func(ctx context.Context, reference string) (*payment.VerifyPaymentOutput, error) {
			return nil, errors.New("gateway network connection timeout: upstream service unavailable")
		},
	}
	mktSvc := service.NewMarketplaceService(env.Repo, env.AuditService, gw)
	_, _ = env.Pool.Exec(ctx, "UPDATE marketplace_orders SET reservation_expires_at = NOW() + INTERVAL '24 hours' WHERE order_status = 'placed' AND stock_released = FALSE")

	orderRes, err := mktSvc.CreateOrder(ctx, buyer, service.CheckoutInput{
		Items:        []service.CartItemInput{{ProductID: prod.ID, Quantity: 2}},
		BuyerName:    "Buyer Nine",
		BuyerEmail:   "buyer9@example.org",
		BuyerPhone:   "+2348000000009",
		DeliveryType: "pickup",
	})
	if err != nil {
		t.Fatalf("CreateOrder failed: %v", err)
	}

	// Stock is 8
	_, _ = env.Pool.Exec(ctx, "UPDATE marketplace_orders SET reservation_expires_at = NOW() - INTERVAL '1 hour' WHERE id = $1", orderRes.Order.ID)

	// Run expiry worker
	res, err := mktSvc.ExpireStaleReservations(ctx, 50)
	if err != nil {
		t.Fatalf("ExpireStaleReservations failed: %v", err)
	}
	if res.ReconciliationCount != 1 {
		t.Errorf("Expected ReconciliationCount 1, got %d", res.ReconciliationCount)
	}
	if res.ExpiredCount != 0 {
		t.Errorf("Expected ExpiredCount 0 (must not cancel during outage), got %d", res.ExpiredCount)
	}

	// Order must NOT be cancelled, stock must NOT be released
	ord, err := env.Repo.Queries().GetMarketplaceOrderByID(ctx, orderRes.Order.ID)
	if err != nil {
		t.Fatalf("GetOrderByID failed: %v", err)
	}
	if ord.OrderStatus != "requires_reconciliation" {
		t.Errorf("Expected order status 'requires_reconciliation', got %s", ord.OrderStatus)
	}
	if !ord.ReconciliationRequired {
		t.Errorf("Expected reconciliation_required = true")
	}
	if ord.StockReleased {
		t.Errorf("Expected stock_released = false")
	}

	// Stock remains reserved at 8
	pAfter, _ := mktSvc.GetProductByID(ctx, prod.ID)
	if pAfter.StockQuantity != 8 {
		t.Fatalf("Expected stock to remain 8, got %d", pAfter.StockQuantity)
	}
}

// 10. Cancellation and expiry racing
func TestInventory_CancellationAndExpiryRace(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	admin := createRegisteredUser(t, env, "inv_admin_10", "super_admin")
	seller, _, prod := helperCreateSellerWithProduct(t, env, ctx, "inv_seller_10", "Store 10", "Drill", 10, admin)
	buyer := createRegisteredUser(t, env, "inv_buyer_10", "member")

	gw := &mockInventoryExpiryGateway{
		providerName: "paystack",
		verifyFn: func(ctx context.Context, reference string) (*payment.VerifyPaymentOutput, error) {
			return &payment.VerifyPaymentOutput{PaymentReference: reference, Status: "abandoned"}, nil
		},
	}
	mktSvc := service.NewMarketplaceService(env.Repo, env.AuditService, gw)
	_, _ = env.Pool.Exec(ctx, "UPDATE marketplace_orders SET reservation_expires_at = NOW() + INTERVAL '24 hours' WHERE order_status = 'placed' AND stock_released = FALSE")

	orderRes, err := mktSvc.CreateOrder(ctx, buyer, service.CheckoutInput{
		Items:        []service.CartItemInput{{ProductID: prod.ID, Quantity: 2}},
		BuyerName:    "Buyer Ten",
		BuyerEmail:   "buyer10@example.org",
		BuyerPhone:   "+2348000000010",
		DeliveryType: "pickup",
	})
	if err != nil {
		t.Fatalf("CreateOrder failed: %v", err)
	}

	// Stock is 8
	_, _ = env.Pool.Exec(ctx, "UPDATE marketplace_orders SET reservation_expires_at = NOW() - INTERVAL '1 hour' WHERE id = $1", orderRes.Order.ID)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		_, _ = mktSvc.CancelOrder(ctx, buyer, orderRes.Order.ID, "Changed my mind")
	}()

	go func() {
		defer wg.Done()
		_, _ = mktSvc.ExpireStaleReservations(ctx, 50)
	}()

	wg.Wait()

	// Final stock must be exactly 10 (restored once, never twice!)
	pAfter, _ := mktSvc.GetProductByID(ctx, prod.ID)
	if pAfter.StockQuantity != 10 {
		t.Fatalf("Expected restored stock 10, got %d", pAfter.StockQuantity)
	}

	// Ledger must have exactly one release log
	logs, err := mktSvc.ListProductInventoryLogs(ctx, seller, prod.ID)
	if err != nil {
		t.Fatalf("ListProductInventoryLogs failed: %v", err)
	}
	releaseLogCount := 0
	for _, l := range logs {
		if l.ActionType == "reservation_release" {
			releaseLogCount++
		}
	}
	if releaseLogCount != 1 {
		t.Errorf("Expected exactly 1 release log, got %d", releaseLogCount)
	}
}

// 11. Refund and cancellation double-restock prevention
func TestInventory_RefundAndCancellationDoubleRestockPrevention(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	admin := createRegisteredUser(t, env, "inv_admin_11", "super_admin")
	adminApprover := createRegisteredUser(t, env, "inv_admin_11_appr", "super_admin")
	_, _, prod := helperCreateSellerWithProduct(t, env, ctx, "inv_seller_11", "Store 11", "Tape Measure", 10, admin)
	buyer := createRegisteredUser(t, env, "inv_buyer_11", "member")

	gw := &mockInventoryExpiryGateway{
		providerName: "paystack",
		verifyFn: func(ctx context.Context, reference string) (*payment.VerifyPaymentOutput, error) {
			now := time.Now()
			return &payment.VerifyPaymentOutput{
				PaymentReference:  reference,
				ProviderReference: "ps_" + reference,
				Status:            "successful",
				AmountKobo:        250000,
				Currency:          "NGN",
				PaidAt:            &now,
				Channel:           "card",
			}, nil
		},
	}
	mktSvc := service.NewMarketplaceService(env.Repo, env.AuditService, gw)
	_, _ = env.Pool.Exec(ctx, "UPDATE marketplace_orders SET reservation_expires_at = NOW() + INTERVAL '24 hours' WHERE order_status = 'placed' AND stock_released = FALSE")

	orderRes, err := mktSvc.CreateOrder(ctx, buyer, service.CheckoutInput{
		Items:        []service.CartItemInput{{ProductID: prod.ID, Quantity: 1}},
		BuyerName:    "Buyer Eleven",
		BuyerEmail:   "buyer11@example.org",
		BuyerPhone:   "+2348000000011",
		DeliveryType: "pickup",
	})
	if err != nil {
		t.Fatalf("CreateOrder failed: %v", err)
	}

	// Payment succeeds -> stock 9
	_, err = mktSvc.ConfirmPayment(ctx, orderRes.Payment.Reference)
	if err != nil {
		t.Fatalf("ConfirmPayment failed: %v", err)
	}

	pAfterPaid, _ := mktSvc.GetProductByID(ctx, prod.ID)
	if pAfterPaid.StockQuantity != 9 {
		t.Fatalf("Expected stock 9 after payment, got %d", pAfterPaid.StockQuantity)
	}

	// 1. Full refund is initiated and settled -> stock is restored to 10 and stock_released becomes true
	refund, _, err := mktSvc.InitiateRefund(ctx, admin, orderRes.Order.ID, service.RefundOrderInput{
		AmountKobo: 250000,
		Reason:     "Customer returned item before dispatch",
	})
	if err != nil {
		t.Fatalf("InitiateRefund failed: %v", err)
	}

	_, _, err = mktSvc.ConfirmRefund(ctx, adminApprover, refund.ID, service.ConfirmRefundInput{
		ProviderRefundReference: "mock_refund_ref",
	})
	if err != nil {
		t.Fatalf("ConfirmRefund failed: %v", err)
	}

	// Stock is restored to 10
	pAfterRefund, _ := mktSvc.GetProductByID(ctx, prod.ID)
	if pAfterRefund.StockQuantity != 10 {
		t.Fatalf("Expected stock 10 after refund settlement, got %d", pAfterRefund.StockQuantity)
	}

	// Verify order has stock_released = true
	ordAfterRefund, err := env.Repo.Queries().GetMarketplaceOrderByID(ctx, orderRes.Order.ID)
	if err != nil {
		t.Fatalf("GetMarketplaceOrderByID failed: %v", err)
	}
	if !ordAfterRefund.StockReleased {
		t.Fatalf("Expected stock_released = true after full refund settlement")
	}

	// 2. Cancellation attempt must be rejected and must NOT restock again
	_, err = mktSvc.CancelOrder(ctx, buyer, orderRes.Order.ID, "Attempting double cancel")
	if err == nil {
		t.Fatalf("Expected error attempting to cancel already refunded order, got nil")
	}

	// Stock must NOT be incremented again! Must remain 10, NOT 11!
	pAfterCancelAttempt, _ := mktSvc.GetProductByID(ctx, prod.ID)
	if pAfterCancelAttempt.StockQuantity != 10 {
		t.Fatalf("Expected stock to remain 10, got %d (double restock occurred!)", pAfterCancelAttempt.StockQuantity)
	}
}

// 12. Manual stock adjustment permissions and bounds
func TestInventory_ManualStockAdjustmentPermissions(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	admin := createRegisteredUser(t, env, "inv_admin_12", "super_admin")
	seller1, _, prod1 := helperCreateSellerWithProduct(t, env, ctx, "inv_seller_12a", "Store 12A", "Screw Gun", 20, admin)
	seller2 := createRegisteredUser(t, env, "inv_seller_12b", "member")

	gw := &mockInventoryExpiryGateway{providerName: "paystack"}
	mktSvc := service.NewMarketplaceService(env.Repo, env.AuditService, gw)

	// Seller 1 adjusts own product (+10)
	upProd, log1, err := mktSvc.AdjustInventory(ctx, seller1, prod1.ID, service.AdjustInventoryInput{
		IdempotencyKey: fmt.Sprintf("test-adj-seller1-%d", time.Now().UnixNano()),
		QuantityDelta:  10,
		Reason:         "Received new stock shipment",
	})
	if err != nil {
		t.Fatalf("Seller adjust inventory failed: %v", err)
	}
	if upProd.StockQuantity != 30 {
		t.Errorf("Expected stock 30, got %d", upProd.StockQuantity)
	}
	if log1.QuantityDelta != 10 || log1.BalanceAfter != 30 || log1.ActionType != "manual_adjustment" {
		t.Errorf("Unexpected log entry: %+v", log1)
	}

	// Seller 2 attempts to adjust Seller 1's product -> Rejected!
	_, _, err = mktSvc.AdjustInventory(ctx, seller2, prod1.ID, service.AdjustInventoryInput{
		IdempotencyKey: fmt.Sprintf("test-adj-seller2-%d", time.Now().UnixNano()),
		QuantityDelta:  -5,
		Reason:         "Unauthorized tampering attempt",
	})
	if err == nil {
		t.Fatalf("Expected unauthorized error for cross-seller adjustment, got nil")
	}

	// Super Admin adjusts product (-5) -> Allowed!
	upProd2, log2, err := mktSvc.AdjustInventory(ctx, admin, prod1.ID, service.AdjustInventoryInput{
		IdempotencyKey: fmt.Sprintf("test-adj-admin-%d", time.Now().UnixNano()),
		QuantityDelta:  -5,
		Reason:         "Damaged units write-off",
	})
	if err != nil {
		t.Fatalf("Admin adjust inventory failed: %v", err)
	}
	if upProd2.StockQuantity != 25 {
		t.Errorf("Expected stock 25, got %d", upProd2.StockQuantity)
	}
	if log2.QuantityDelta != -5 || log2.BalanceAfter != 25 {
		t.Errorf("Unexpected log entry: %+v", log2)
	}

	// Negative balance rejection: attempt to subtract 100 when only 25 in stock
	_, _, err = mktSvc.AdjustInventory(ctx, seller1, prod1.ID, service.AdjustInventoryInput{
		IdempotencyKey: fmt.Sprintf("test-adj-neg-%d", time.Now().UnixNano()),
		QuantityDelta:  -100,
		Reason:         "Attempting negative balance",
	})
	if err == nil {
		t.Fatalf("Expected error for negative balance adjustment, got nil")
	}
}

// 13. Accurate inventory ledger running balance
func TestInventory_LedgerRunningBalanceIntegrity(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	admin := createRegisteredUser(t, env, "inv_admin_13", "super_admin")
	seller, _, prod := helperCreateSellerWithProduct(t, env, ctx, "inv_seller_13", "Store 13", "Workbench", 10, admin)
	buyer := createRegisteredUser(t, env, "inv_buyer_13", "member")

	gw := &mockInventoryExpiryGateway{
		providerName: "paystack",
		verifyFn: func(ctx context.Context, reference string) (*payment.VerifyPaymentOutput, error) {
			now := time.Now()
			return &payment.VerifyPaymentOutput{
				PaymentReference:  reference,
				ProviderReference: "ps_" + reference,
				Status:            "successful",
				AmountKobo:        750000,
				Currency:          "NGN",
				PaidAt:            &now,
				Channel:           "card",
			}, nil
		},
	}
	mktSvc := service.NewMarketplaceService(env.Repo, env.AuditService, gw)

	// Step A: Manual +5 (10 -> 15)
	_, _, err := mktSvc.AdjustInventory(ctx, seller, prod.ID, service.AdjustInventoryInput{
		IdempotencyKey: fmt.Sprintf("adj-ledger-%d", time.Now().UnixNano()),
		QuantityDelta:  5,
		Reason:         "Restock",
	})
	if err != nil {
		t.Fatalf("AdjustInventory failed: %v", err)
	}

	// Step B: Order 1 reserve -3 (15 -> 12)
	ord1, err := mktSvc.CreateOrder(ctx, buyer, service.CheckoutInput{
		Items:        []service.CartItemInput{{ProductID: prod.ID, Quantity: 3}},
		BuyerName:    "Buyer Thirteen",
		BuyerEmail:   "buyer13@example.org",
		BuyerPhone:   "+2348000000013",
		DeliveryType: "pickup",
	})
	if err != nil {
		t.Fatalf("CreateOrder 1 failed: %v", err)
	}

	// Step C: Order 1 confirmed sale (delta 0, balance 12)
	_, err = mktSvc.ConfirmPayment(ctx, ord1.Payment.Reference)
	if err != nil {
		t.Fatalf("ConfirmPayment failed: %v", err)
	}

	// Step D: Order 2 reserve -4 (12 -> 8)
	ord2, err := mktSvc.CreateOrder(ctx, buyer, service.CheckoutInput{
		Items:        []service.CartItemInput{{ProductID: prod.ID, Quantity: 4}},
		BuyerName:    "Buyer Thirteen",
		BuyerEmail:   "buyer13@example.org",
		BuyerPhone:   "+2348000000013",
		DeliveryType: "pickup",
	})
	if err != nil {
		t.Fatalf("CreateOrder 2 failed: %v", err)
	}

	// Step E: Order 2 cancel (8 -> 12)
	_, err = mktSvc.CancelOrder(ctx, buyer, ord2.Order.ID, "Cancelled by buyer")
	if err != nil {
		t.Fatalf("CancelOrder failed: %v", err)
	}

	// Verify all logs
	logs, err := mktSvc.ListProductInventoryLogs(ctx, seller, prod.ID)
	if err != nil {
		t.Fatalf("ListProductInventoryLogs failed: %v", err)
	}

	for i, l := range logs {
		t.Logf("log[%d]: action=%s delta=%d balance=%d order=%v reason=%s", i, l.ActionType, l.QuantityDelta, l.BalanceAfter, l.OrderID, l.Reason)
	}

	if len(logs) != 5 {
		t.Fatalf("Expected 5 logs, got %d", len(logs))
	}

	// List is in reverse chronological order (newest first):
	// [0]: Order 2 release (+4 -> 12)
	// [1]: Order 2 reserve (-4 -> 8)
	// [2]: Order 1 sale_confirmed (0 -> 12)
	// [3]: Order 1 reserve (-3 -> 12)
	// [4]: Manual restock (+5 -> 15)

	if logs[0].ActionType != "reservation_release" || logs[0].QuantityDelta != 4 || logs[0].BalanceAfter != 12 {
		t.Errorf("Log[0] mismatch: %+v", logs[0])
	}
	if logs[1].ActionType != "reservation" || logs[1].QuantityDelta != -4 || logs[1].BalanceAfter != 8 {
		t.Errorf("Log[1] mismatch: %+v", logs[1])
	}
	if logs[2].ActionType != "sale_confirmed" || logs[2].QuantityDelta != 0 || logs[2].BalanceAfter != 12 {
		t.Errorf("Log[2] mismatch: %+v", logs[2])
	}
	if logs[3].ActionType != "reservation" || logs[3].QuantityDelta != -3 || logs[3].BalanceAfter != 12 {
		t.Errorf("Log[3] mismatch: %+v", logs[3])
	}
	if logs[4].ActionType != "manual_adjustment" || logs[4].QuantityDelta != 5 || logs[4].BalanceAfter != 15 {
		t.Errorf("Log[4] mismatch: %+v", logs[4])
	}
}

// 14. Duplicate inventory event rejection (unique idempotency key)
func TestInventory_UniqueIdempotencyKeyConstraint(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	admin := createRegisteredUser(t, env, "inv_admin_14", "super_admin")
	_, biz, prod := helperCreateSellerWithProduct(t, env, ctx, "inv_seller_14", "Store 14", "Anvil", 10, admin)

	idempKey := fmt.Sprintf("test_idemp_%s", security.GenerateUUID())

	// First insert succeeds
	_, err := env.Repo.Queries().CreateMarketplaceInventoryLog(ctx, db.CreateMarketplaceInventoryLogParams{
		ID:             security.GenerateUUID(),
		ProductID:      prod.ID,
		BusinessID:     biz.ID,
		ActorUserID:    pgtype.Text{Valid: false},
		ActionType:     "reservation",
		QuantityDelta:  -1,
		BalanceAfter:   9,
		OrderID:        pgtype.Text{Valid: false},
		ReferenceID:    "TEST-1",
		Reason:         "Test initial insert",
		IdempotencyKey: service.TextFromString(idempKey),
	})
	if err != nil {
		t.Fatalf("First insert failed: %v", err)
	}

	// Second insert with same idempotency key must fail with unique constraint violation
	_, err = env.Repo.Queries().CreateMarketplaceInventoryLog(ctx, db.CreateMarketplaceInventoryLogParams{
		ID:             security.GenerateUUID(),
		ProductID:      prod.ID,
		BusinessID:     biz.ID,
		ActorUserID:    pgtype.Text{Valid: false},
		ActionType:     "reservation",
		QuantityDelta:  -1,
		BalanceAfter:   8,
		OrderID:        pgtype.Text{Valid: false},
		ReferenceID:    "TEST-2",
		Reason:         "Duplicate insert attempt",
		IdempotencyKey: service.TextFromString(idempKey),
	})
	if err == nil {
		t.Fatalf("Expected error for duplicate idempotency key, got nil")
	}
}

// 15. Pending provider verification preserves reservation
func TestInventory_PendingProviderVerificationPreservesReservation(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	admin := createRegisteredUser(t, env, "inv_admin_pend", "super_admin")
	_, _, prod := helperCreateSellerWithProduct(t, env, ctx, "inv_seller_pend", "Pending Store", "Pliers", 10, admin)
	buyer := createRegisteredUser(t, env, "inv_buyer_pend", "member")

	gw := &mockInventoryExpiryGateway{
		providerName: "paystack",
		verifyFn: func(ctx context.Context, reference string) (*payment.VerifyPaymentOutput, error) {
			return &payment.VerifyPaymentOutput{
				PaymentReference: reference,
				Status:           "pending",
			}, nil
		},
	}
	mktSvc := service.NewMarketplaceService(env.Repo, env.AuditService, gw)
	_, _ = env.Pool.Exec(ctx, "UPDATE marketplace_orders SET reservation_expires_at = NOW() + INTERVAL '24 hours' WHERE order_status = 'placed' AND stock_released = FALSE")

	orderRes, err := mktSvc.CreateOrder(ctx, buyer, service.CheckoutInput{
		Items:        []service.CartItemInput{{ProductID: prod.ID, Quantity: 3}},
		BuyerName:    "Pending Buyer",
		BuyerEmail:   "pend_buyer@example.org",
		BuyerPhone:   "+2348000000015",
		DeliveryType: "pickup",
	})
	if err != nil {
		t.Fatalf("CreateOrder failed: %v", err)
	}

	// Stock is reserved: 10 - 3 = 7
	pMid, _ := mktSvc.GetProductByID(ctx, prod.ID)
	if pMid.StockQuantity != 7 {
		t.Fatalf("Expected stock 7 after reservation, got %d", pMid.StockQuantity)
	}

	// ConfirmPayment during pending status must NOT mark payment or order as failed
	_, err = mktSvc.ConfirmPayment(ctx, orderRes.Payment.Reference)
	if err == nil {
		t.Fatal("Expected error on pending payment confirmation, got nil")
	}
	if !strings.Contains(err.Error(), "pending") {
		t.Fatalf("Expected pending verification error, got %v", err)
	}

	// Verify order status is still placed and pending
	ord, err := env.Repo.Queries().GetMarketplaceOrderByID(ctx, orderRes.Order.ID)
	if err != nil {
		t.Fatalf("GetMarketplaceOrderByID failed: %v", err)
	}
	if ord.OrderStatus != "placed" {
		t.Errorf("Expected order status 'placed', got %s", ord.OrderStatus)
	}
	if ord.PaymentStatus != "pending" {
		t.Errorf("Expected payment status 'pending', got %s", ord.PaymentStatus)
	}
	if ord.StockReleased {
		t.Errorf("Stock must NOT be released while pending")
	}

	// Product stock must still be 7
	pAfter, _ := mktSvc.GetProductByID(ctx, prod.ID)
	if pAfter.StockQuantity != 7 {
		t.Errorf("Stock must remain 7, got %d", pAfter.StockQuantity)
	}
}

// 16. Failed payment order cleanup by expiry worker
func TestInventory_FailedPaymentCleanup(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	admin := createRegisteredUser(t, env, "inv_admin_fail", "super_admin")
	_, _, prod := helperCreateSellerWithProduct(t, env, ctx, "inv_seller_fail", "Fail Store", "Hammer", 10, admin)
	buyer := createRegisteredUser(t, env, "inv_buyer_fail", "member")

	verifyStatus := "failed"
	gw := &mockInventoryExpiryGateway{
		providerName: "paystack",
		verifyFn: func(ctx context.Context, reference string) (*payment.VerifyPaymentOutput, error) {
			return &payment.VerifyPaymentOutput{
				PaymentReference: reference,
				Status:           verifyStatus,
			}, nil
		},
	}
	mktSvc := service.NewMarketplaceService(env.Repo, env.AuditService, gw)
	_, _ = env.Pool.Exec(ctx, "UPDATE marketplace_orders SET reservation_expires_at = NOW() + INTERVAL '24 hours' WHERE order_status = 'placed' AND stock_released = FALSE")

	orderRes, err := mktSvc.CreateOrder(ctx, buyer, service.CheckoutInput{
		Items:        []service.CartItemInput{{ProductID: prod.ID, Quantity: 2}},
		BuyerName:    "Failed Buyer",
		BuyerEmail:   "failed_buyer@example.org",
		BuyerPhone:   "+2348000000016",
		DeliveryType: "pickup",
	})
	if err != nil {
		t.Fatalf("CreateOrder failed: %v", err)
	}

	// Terminal failure reported during checkout
	_, err = mktSvc.ConfirmPayment(ctx, orderRes.Payment.Reference)
	if err == nil {
		t.Fatal("Expected error on failed payment confirmation, got nil")
	}

	// Order is now placed with payment_status = failed, stock_released = false
	ordMid, _ := env.Repo.Queries().GetMarketplaceOrderByID(ctx, orderRes.Order.ID)
	if ordMid.PaymentStatus != "failed" || ordMid.StockReleased {
		t.Fatalf("Expected payment_status failed and stock_released false, got %+v", ordMid)
	}

	// Expire order in DB
	_, _ = env.Pool.Exec(ctx, "UPDATE marketplace_orders SET reservation_expires_at = NOW() - INTERVAL '1 hour' WHERE id = $1", orderRes.Order.ID)

	// Clean up stale orders
	res, err := mktSvc.ExpireStaleReservations(ctx, 50)
	if err != nil {
		t.Fatalf("ExpireStaleReservations failed: %v", err)
	}
	if res.ExpiredCount < 1 {
		t.Fatalf("Expected at least 1 expired order, got %d", res.ExpiredCount)
	}

	// Verify order is now cancelled with stock released
	ordAfter, _ := env.Repo.Queries().GetMarketplaceOrderByID(ctx, orderRes.Order.ID)
	if ordAfter.OrderStatus != "cancelled" || !ordAfter.StockReleased {
		t.Errorf("Expected cancelled and stock_released=true, got %+v", ordAfter)
	}

	// Stock is restored to 10
	pAfter, _ := mktSvc.GetProductByID(ctx, prod.ID)
	if pAfter.StockQuantity != 10 {
		t.Errorf("Expected stock restored to 10, got %d", pAfter.StockQuantity)
	}
}

// 17. Provider HTTP errors (429, 500, 502) must NOT cancel orders
func TestInventory_ProviderHTTPErrorsInconclusive(t *testing.T) {
	testCases := []struct {
		name     string
		errToRet error
	}{
		{"HTTP 429 Too Many Requests", fmt.Errorf("paystack verify returned unexpected HTTP status 429: Too Many Requests")},
		{"HTTP 500 Internal Error", fmt.Errorf("paystack verify returned unexpected HTTP status 500: Internal Server Error")},
		{"HTTP 502 Bad Gateway", fmt.Errorf("paystack verify returned unexpected HTTP status 502: Bad Gateway")},
		{"Malformed JSON", fmt.Errorf("failed to parse verify response: unexpected end of JSON input")},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			env := setupTestEnv(t)
			defer env.Pool.Close()
			ctx := context.Background()

			admin := createRegisteredUser(t, env, fmt.Sprintf("adm_err_%d", time.Now().UnixNano()), "super_admin")
			_, _, prod := helperCreateSellerWithProduct(t, env, ctx, fmt.Sprintf("sel_err_%d", time.Now().UnixNano()), "Store Err", "Chisel", 10, admin)
			buyer := createRegisteredUser(t, env, fmt.Sprintf("buy_err_%d", time.Now().UnixNano()), "member")

			gw := &mockInventoryExpiryGateway{
				providerName: "paystack",
				verifyFn: func(ctx context.Context, reference string) (*payment.VerifyPaymentOutput, error) {
					return nil, tc.errToRet
				},
			}
			mktSvc := service.NewMarketplaceService(env.Repo, env.AuditService, gw)

			orderRes, err := mktSvc.CreateOrder(ctx, buyer, service.CheckoutInput{
				Items:        []service.CartItemInput{{ProductID: prod.ID, Quantity: 2}},
				BuyerName:    "Buyer Err",
				BuyerEmail:   "buyer_err@example.org",
				BuyerPhone:   "+2348000000017",
				DeliveryType: "pickup",
			})
			if err != nil {
				t.Fatalf("CreateOrder failed: %v", err)
			}

			// Expire in DB
			_, _ = env.Pool.Exec(ctx, "UPDATE marketplace_orders SET reservation_expires_at = NOW() - INTERVAL '1 hour' WHERE id = $1", orderRes.Order.ID)

			res, err := mktSvc.ExpireStaleReservations(ctx, 50)
			if err != nil {
				t.Fatalf("ExpireStaleReservations returned unexpected fatal error: %v", err)
			}
			if res.ReconciliationCount < 1 {
				t.Errorf("Expected order to be counted in ReconciliationCount, got %d", res.ReconciliationCount)
			}

			// Order MUST NOT be cancelled; stock MUST NOT be released!
			ord, _ := env.Repo.Queries().GetMarketplaceOrderByID(ctx, orderRes.Order.ID)
			if ord.OrderStatus != "requires_reconciliation" {
				t.Errorf("Expected requires_reconciliation, got %s", ord.OrderStatus)
			}
			if ord.StockReleased {
				t.Errorf("Stock must NOT be released when provider verification is inconclusive")
			}
			if !ord.ReconciliationRequired {
				t.Errorf("Expected reconciliation_required = true")
			}

			// Stock remains 8 (reserved)
			p, _ := mktSvc.GetProductByID(ctx, prod.ID)
			if p.StockQuantity != 8 {
				t.Errorf("Stock must remain 8, got %d", p.StockQuantity)
			}
		})
	}
}

// 18. Unknown or non-terminal provider statuses must NOT release stock
func TestInventory_UnknownProviderStatusInconclusive(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	admin := createRegisteredUser(t, env, "inv_admin_unk", "super_admin")
	_, _, prod := helperCreateSellerWithProduct(t, env, ctx, "inv_seller_unk", "Unknown Store", "Drill Bit", 10, admin)
	buyer := createRegisteredUser(t, env, "inv_buyer_unk", "member")

	gw := &mockInventoryExpiryGateway{
		providerName: "paystack",
		verifyFn: func(ctx context.Context, reference string) (*payment.VerifyPaymentOutput, error) {
			return &payment.VerifyPaymentOutput{
				PaymentReference: reference,
				Status:           "queued", // Non-terminal / unknown status
			}, nil
		},
	}
	mktSvc := service.NewMarketplaceService(env.Repo, env.AuditService, gw)
	_, _ = env.Pool.Exec(ctx, "UPDATE marketplace_orders SET reservation_expires_at = NOW() + INTERVAL '24 hours' WHERE order_status = 'placed' AND stock_released = FALSE")

	orderRes, err := mktSvc.CreateOrder(ctx, buyer, service.CheckoutInput{
		Items:        []service.CartItemInput{{ProductID: prod.ID, Quantity: 2}},
		BuyerName:    "Buyer Unk",
		BuyerEmail:   "buyer_unk@example.org",
		BuyerPhone:   "+2348000000018",
		DeliveryType: "pickup",
	})
	if err != nil {
		t.Fatalf("CreateOrder failed: %v", err)
	}

	_, _ = env.Pool.Exec(ctx, "UPDATE marketplace_orders SET reservation_expires_at = NOW() - INTERVAL '1 hour' WHERE id = $1", orderRes.Order.ID)

	res, err := mktSvc.ExpireStaleReservations(ctx, 50)
	if err != nil {
		t.Fatalf("ExpireStaleReservations failed: %v", err)
	}
	if res.ReconciliationCount < 1 {
		t.Errorf("Expected reconciliation count >= 1, got %d", res.ReconciliationCount)
	}

	ord, _ := env.Repo.Queries().GetMarketplaceOrderByID(ctx, orderRes.Order.ID)
	if ord.OrderStatus != "requires_reconciliation" {
		t.Errorf("Expected requires_reconciliation, got %s", ord.OrderStatus)
	}
	if ord.StockReleased {
		t.Errorf("Stock must NOT be released for non-terminal status")
	}
}

// 19. Duplicate inventory adjustment idempotency
func TestInventory_DuplicateInventoryAdjustment(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	admin := createRegisteredUser(t, env, "inv_admin_dup_adj", "super_admin")
	seller, _, prod := helperCreateSellerWithProduct(t, env, ctx, "inv_seller_dup_adj", "Store Dup Adj", "Screws", 50, admin)

	mktSvc := service.NewMarketplaceService(env.Repo, env.AuditService, payment.NewSandboxGateway("http://localhost:3000"))

	idempKey := fmt.Sprintf("adj-dup-%d", time.Now().UnixNano())

	// First adjustment: +10
	up1, log1, err := mktSvc.AdjustInventory(ctx, seller, prod.ID, service.AdjustInventoryInput{
		IdempotencyKey: idempKey,
		QuantityDelta:  10,
		Reason:         "Restock batch 1",
	})
	if err != nil {
		t.Fatalf("First adjustment failed: %v", err)
	}
	if up1.StockQuantity != 60 {
		t.Errorf("Expected stock 60, got %d", up1.StockQuantity)
	}
	if log1.BalanceAfter != 60 {
		t.Errorf("Expected log balance 60, got %d", log1.BalanceAfter)
	}

	// Second adjustment with identical payload: must be harmless idempotent replay
	up2, log2, err := mktSvc.AdjustInventory(ctx, seller, prod.ID, service.AdjustInventoryInput{
		IdempotencyKey: idempKey,
		QuantityDelta:  10,
		Reason:         "Restock batch 1",
	})
	if err != nil {
		t.Fatalf("Second adjustment should succeed idempotently, got: %v", err)
	}
	if up2.StockQuantity != 60 {
		t.Errorf("Stock must NOT change on duplicate request (expected 60, got %d)", up2.StockQuantity)
	}
	if log2.ID != log1.ID {
		t.Errorf("Expected same log ID on idempotent replay, got %s vs %s", log2.ID, log1.ID)
	}
}

// 20. Conflicting idempotency key reuse rejection
func TestInventory_ConflictingIdempotencyKeyReuse(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	admin := createRegisteredUser(t, env, "inv_admin_conf", "super_admin")
	seller, _, prod := helperCreateSellerWithProduct(t, env, ctx, "inv_seller_conf", "Store Conf", "Nails", 40, admin)

	mktSvc := service.NewMarketplaceService(env.Repo, env.AuditService, payment.NewSandboxGateway("http://localhost:3000"))

	idempKey := fmt.Sprintf("adj-conf-%d", time.Now().UnixNano())

	// First adjustment: +10
	_, _, err := mktSvc.AdjustInventory(ctx, seller, prod.ID, service.AdjustInventoryInput{
		IdempotencyKey: idempKey,
		QuantityDelta:  10,
		Reason:         "Initial add",
	})
	if err != nil {
		t.Fatalf("First adjustment failed: %v", err)
	}

	// Reusing same key with DIFFERENT delta: must be rejected with ErrIdempotencyConflict
	_, _, err = mktSvc.AdjustInventory(ctx, seller, prod.ID, service.AdjustInventoryInput{
		IdempotencyKey: idempKey,
		QuantityDelta:  20, // Different!
		Reason:         "Initial add",
	})
	if err == nil {
		t.Fatal("Expected ErrIdempotencyConflict on conflicting payload, got nil")
	}
	if !errors.Is(err, service.ErrIdempotencyConflict) {
		t.Fatalf("Expected ErrIdempotencyConflict, got %v", err)
	}

	// Reusing same key with DIFFERENT reason: must be rejected with ErrIdempotencyConflict
	_, _, err = mktSvc.AdjustInventory(ctx, seller, prod.ID, service.AdjustInventoryInput{
		IdempotencyKey: idempKey,
		QuantityDelta:  10,
		Reason:         "Different reason", // Different!
	})
	if err == nil {
		t.Fatal("Expected ErrIdempotencyConflict on conflicting reason, got nil")
	}
	if !errors.Is(err, service.ErrIdempotencyConflict) {
		t.Fatalf("Expected ErrIdempotencyConflict, got %v", err)
	}

	// Verify stock is still 50
	p, _ := mktSvc.GetProductByID(ctx, prod.ID)
	if p.StockQuantity != 50 {
		t.Errorf("Stock must remain 50, got %d", p.StockQuantity)
	}
}

// 21. Migration compatibility with historical cancelled and pending orders
func TestInventory_HistoricalOrderMigrationCompatibility(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	admin := createRegisteredUser(t, env, "inv_admin_mig", "super_admin")
	_, _, prod := helperCreateSellerWithProduct(t, env, ctx, "inv_seller_mig", "Store Mig", "Bricks", 100, admin)
	buyer := createRegisteredUser(t, env, "inv_buyer_mig", "member")

	mktSvc := service.NewMarketplaceService(env.Repo, env.AuditService, payment.NewSandboxGateway("http://localhost:3000"))

	// Create order 1: Cancelled order simulating pre-Step 63.4 state (stock_released = false)
	o1, err := mktSvc.CreateOrder(ctx, buyer, service.CheckoutInput{
		Items:        []service.CartItemInput{{ProductID: prod.ID, Quantity: 1}},
		BuyerName:    "Hist Buyer 1",
		BuyerEmail:   "hist1@example.org",
		BuyerPhone:   "+2348000000021",
		DeliveryType: "pickup",
	})
	if err != nil {
		t.Fatalf("CreateOrder 1 failed: %v", err)
	}
	_, _ = env.Pool.Exec(ctx, "UPDATE marketplace_orders SET order_status = 'cancelled', stock_released = false WHERE id = $1", o1.Order.ID)

	// Create order 2: Historical pending order without reservation_expires_at
	o2, err := mktSvc.CreateOrder(ctx, buyer, service.CheckoutInput{
		Items:        []service.CartItemInput{{ProductID: prod.ID, Quantity: 1}},
		BuyerName:    "Hist Buyer 2",
		BuyerEmail:   "hist2@example.org",
		BuyerPhone:   "+2348000000022",
		DeliveryType: "pickup",
	})
	if err != nil {
		t.Fatalf("CreateOrder 2 failed: %v", err)
	}
	_, _ = env.Pool.Exec(ctx, "UPDATE marketplace_orders SET reservation_expires_at = NULL WHERE id = $1", o2.Order.ID)

	// Execute migration 20 Up statements
	_, err = env.Pool.Exec(ctx, `
		UPDATE marketplace_orders
		SET stock_released = TRUE
		WHERE stock_released = FALSE
		  AND order_status IN ('cancelled', 'refunded');

		UPDATE marketplace_orders
		SET reservation_expires_at = created_at + INTERVAL '30 minutes'
		WHERE reservation_expires_at IS NULL
		  AND order_status = 'placed'
		  AND payment_status = 'pending';
	`)
	if err != nil {
		t.Fatalf("Migration statements failed: %v", err)
	}

	// Verify order 1 now has stock_released = true
	ord1, _ := env.Repo.Queries().GetMarketplaceOrderByID(ctx, o1.Order.ID)
	if !ord1.StockReleased {
		t.Errorf("Order 1 stock_released must be true after migration")
	}

	// Verify order 2 now has reservation_expires_at populated
	ord2, _ := env.Repo.Queries().GetMarketplaceOrderByID(ctx, o2.Order.ID)
	if !ord2.ReservationExpiresAt.Valid {
		t.Errorf("Order 2 reservation_expires_at must be valid after migration")
	}
}

// 22. Inventory ledger insertion failure rolls back transaction
func TestInventory_LedgerInsertionFailureRollback(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	admin := createRegisteredUser(t, env, "inv_admin_roll", "super_admin")
	_, biz, prod := helperCreateSellerWithProduct(t, env, ctx, "inv_seller_roll", "Store Roll", "Generator", 10, admin)
	buyer := createRegisteredUser(t, env, "inv_buyer_roll", "member")

	mktSvc := service.NewMarketplaceService(env.Repo, env.AuditService, payment.NewSandboxGateway("http://localhost:3000"))

	orderRes, err := mktSvc.CreateOrder(ctx, buyer, service.CheckoutInput{
		Items:        []service.CartItemInput{{ProductID: prod.ID, Quantity: 2}},
		BuyerName:    "Roll Buyer",
		BuyerEmail:   "roll@example.org",
		BuyerPhone:   "+2348000000023",
		DeliveryType: "pickup",
	})
	if err != nil {
		t.Fatalf("CreateOrder failed: %v", err)
	}

	// Stock is now 8
	pMid, _ := mktSvc.GetProductByID(ctx, prod.ID)
	if pMid.StockQuantity != 8 {
		t.Fatalf("Expected intermediate stock 8, got %d", pMid.StockQuantity)
	}

	// Pre-insert a colliding ledger entry with action_type='audit_note' so CountOrderInventoryReleaseLogs returns 0,
	// but the unique idempotency key constraint on release:<orderID>:<productID> will collide during CancelOrder.
	collidingKey := fmt.Sprintf("release:%s:%s", orderRes.Order.ID, prod.ID)
	_, err = env.Repo.Queries().CreateMarketplaceInventoryLog(ctx, db.CreateMarketplaceInventoryLogParams{
		ID:             security.GenerateUUID(),
		ProductID:      prod.ID,
		BusinessID:     biz.ID,
		ActorUserID:    pgtype.Text{Valid: false},
		ActionType:     "audit_note",
		QuantityDelta:  0,
		BalanceAfter:   8,
		OrderID:        service.TextFromString(orderRes.Order.ID),
		ReferenceID:    "COLLISION-PRE-INSERT",
		Reason:         "Simulate unique key collision",
		IdempotencyKey: service.TextFromString(collidingKey),
	})
	if err != nil {
		t.Fatalf("Failed to pre-insert colliding log: %v", err)
	}

	// Attempting to cancel the order must fail when CreateMarketplaceInventoryLog fails
	_, err = mktSvc.CancelOrder(ctx, buyer, orderRes.Order.ID, "Customer requested cancellation")
	if err == nil {
		t.Fatal("Expected error on CancelOrder due to ledger insertion failure, got nil")
	}

	// PROVE ROLLBACK:
	// 1. Product stock must still be 8 (NOT incremented to 10!)
	pAfter, _ := mktSvc.GetProductByID(ctx, prod.ID)
	if pAfter.StockQuantity != 8 {
		t.Errorf("Stock increment MUST be rolled back (expected 8, got %d)", pAfter.StockQuantity)
	}

	// 2. Order status must still be 'placed' (NOT 'cancelled'!)
	ordAfter, _ := env.Repo.Queries().GetMarketplaceOrderByID(ctx, orderRes.Order.ID)
	if ordAfter.OrderStatus != "placed" {
		t.Errorf("Order status update MUST be rolled back (expected 'placed', got '%s')", ordAfter.OrderStatus)
	}
	if ordAfter.StockReleased {
		t.Errorf("Order stock_released MUST be rolled back (expected false, got true)")
	}
}
