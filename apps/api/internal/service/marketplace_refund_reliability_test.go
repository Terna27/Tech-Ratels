package service_test

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"ratels-api/internal/db"
	"ratels-api/internal/security"
	"ratels-api/internal/service"
	"ratels-api/internal/worker"
)

// -------------------------------------------------------------
// Step 63.5.3: Financial Integrity and Refund Reliability Tests
// -------------------------------------------------------------

// 1. Paid cancellation followed by refund settlement
func TestRefundReliability_PaidCancellationFollowedByRefundSettlement(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	admin, sellerA, _, prodA, _, orderRes := helperCreateMultiVendorOrder(t, env, ctx)
	_ = sellerA
	_, err := env.MarketplaceService.ConfirmPayment(ctx, orderRes.Payment.Reference)
	if err != nil {
		t.Fatalf("ConfirmPayment failed: %v", err)
	}

	buyerUser := &db.User{ID: orderRes.Order.BuyerUserID.String, PrimaryRole: "member"}

	// Buyer cancels paid order before dispatch
	cancelledOrder, err := env.MarketplaceService.CancelOrder(ctx, buyerUser, orderRes.Order.ID, "Customer cancellation request")
	if err != nil {
		t.Fatalf("CancelOrder failed: %v", err)
	}

	if cancelledOrder.OrderStatus != "cancelled" {
		t.Fatalf("Expected order status 'cancelled', got %s", cancelledOrder.OrderStatus)
	}
	// Authoritative payment status MUST NOT be prematurely set to 'refunded'
	if cancelledOrder.PaymentStatus == "refunded" {
		t.Fatalf("Payment status was prematurely set to 'refunded' upon cancellation without verified settlement")
	}

	// Verify pending refund obligation in database
	refunds, err := env.MarketplaceService.ListRefundsByOrder(ctx, admin, orderRes.Order.ID)
	if err != nil {
		t.Fatalf("ListRefundsByOrder failed: %v", err)
	}
	if len(refunds) != 1 {
		t.Fatalf("Expected 1 pending refund obligation, got %d", len(refunds))
	}
	refund := refunds[0]
	if refund.AmountKobo != cancelledOrder.TotalKobo {
		t.Errorf("Expected refund amount %d, got %d", cancelledOrder.TotalKobo, refund.AmountKobo)
	}

	// Now settle the refund authoritatively via ConfirmRefund
	confirmedRefund, settledOrder, err := env.MarketplaceService.ConfirmRefund(ctx, admin, refund.ID, service.ConfirmRefundInput{
		ProviderRefundReference: refund.ProviderRefundReference,
	})
	if err != nil {
		t.Fatalf("ConfirmRefund failed: %v", err)
	}
	if confirmedRefund.Status != "successful" {
		t.Errorf("Expected refund status 'successful', got %s", confirmedRefund.Status)
	}
	if settledOrder.PaymentStatus != "refunded" {
		t.Errorf("Expected payment status 'refunded', got %s", settledOrder.PaymentStatus)
	}

	// Verify inventory release logs exist exactly once per product
	countA, err := env.Repo.Queries().CountOrderInventoryReleaseLogs(ctx, db.CountOrderInventoryReleaseLogsParams{
		OrderID:   service.TextFromString(orderRes.Order.ID),
		ProductID: prodA.ID,
	})
	if err != nil || countA != 1 {
		t.Errorf("Expected exactly 1 inventory release log for Product A, got %d (err: %v)", countA, err)
	}

	// Settle again (idempotent duplicate confirmation)
	dupRefund, dupOrder, err := env.MarketplaceService.ConfirmRefund(ctx, admin, refund.ID, service.ConfirmRefundInput{
		ProviderRefundReference: refund.ProviderRefundReference,
	})
	if err != nil {
		t.Fatalf("Duplicate ConfirmRefund failed: %v", err)
	}
	if dupRefund.Status != "successful" || dupOrder.PaymentStatus != "refunded" {
		t.Errorf("Expected duplicate settlement to remain successful/refunded, got %s / %s", dupRefund.Status, dupOrder.PaymentStatus)
	}
}

// 2. Provider timeout and retry via durable recovery worker
func TestRefundReliability_ProviderTimeoutAndRetry(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	admin, _, _, _, _, orderRes := helperCreateMultiVendorOrder(t, env, ctx)
	_, _ = env.MarketplaceService.ConfirmPayment(ctx, orderRes.Payment.Reference)

	buyerUser := &db.User{ID: orderRes.Order.BuyerUserID.String, PrimaryRole: "member"}
	_, err := env.MarketplaceService.CancelOrder(ctx, buyerUser, orderRes.Order.ID, "Cancellation before dispatch")
	if err != nil {
		t.Fatalf("CancelOrder failed: %v", err)
	}

	refunds, err := env.MarketplaceService.ListRefundsByOrder(ctx, admin, orderRes.Order.ID)
	if err != nil || len(refunds) == 0 {
		t.Fatalf("Failed to retrieve pending refund: %v", err)
	}
	refundID := refunds[0].ID

	// Simulate provider timeout during cancellation: clear provider_refund_reference and keep status pending
	_, err = env.Pool.Exec(ctx, `
		UPDATE marketplace_refunds
		SET provider_refund_reference = '',
		    status = 'pending',
		    settlement_notes = 'Simulated timeout during initial gateway call'
		WHERE id = $1
	`, refundID)
	if err != nil {
		t.Fatalf("Failed to reset refund reference for timeout test: %v", err)
	}

	// Verify refund is pending with empty provider reference
	refBefore, err := env.Repo.Queries().GetMarketplaceRefundByID(ctx, refundID)
	if err != nil || refBefore.Status != "pending" || refBefore.ProviderRefundReference != "" {
		t.Fatalf("Expected pending refund with empty provider ref, got status=%s, ref=%s", refBefore.Status, refBefore.ProviderRefundReference)
	}

	// Run durable recovery worker cycle 1: retries provider initiation and acquires provider reference
	result1, err := env.MarketplaceService.ProcessPendingRefunds(ctx, 50)
	if err != nil {
		t.Fatalf("ProcessPendingRefunds cycle 1 failed: %v", err)
	}
	if result1.RetriedCount == 0 && result1.SettledCount == 0 {
		t.Fatalf("Expected recovery worker cycle 1 to retry provider initiation, got result: %+v", result1)
	}

	// Verify provider reference was acquired and saved in database
	refAfterRetry, err := env.Repo.Queries().GetMarketplaceRefundByID(ctx, refundID)
	if err != nil || refAfterRetry.ProviderRefundReference == "" {
		t.Fatalf("Expected provider refund reference to be acquired after cycle 1, got ref=%s (err: %v)", refAfterRetry.ProviderRefundReference, err)
	}

	// Run durable recovery worker cycle 2: verifies provider settlement and marks refund and payment refunded
	result2, err := env.MarketplaceService.ProcessPendingRefunds(ctx, 50)
	if err != nil {
		t.Fatalf("ProcessPendingRefunds cycle 2 failed: %v", err)
	}
	_ = result2

	// Verify authoritative refund status has progressed to successful
	refAfter, err := env.Repo.Queries().GetMarketplaceRefundByID(ctx, refundID)
	if err != nil {
		t.Fatalf("GetMarketplaceRefundByID failed: %v", err)
	}
	if refAfter.Status != "successful" {
		t.Errorf("Expected refund status 'successful' after recovery verification, got '%s'", refAfter.Status)
	}

	// Verify order payment status is now refunded
	ord, _, _, _ := env.MarketplaceService.GetOrderByID(ctx, admin, orderRes.Order.ID)
	if ord.PaymentStatus != "refunded" {
		t.Errorf("Expected order payment status 'refunded', got %s", ord.PaymentStatus)
	}
}

// 3. Duplicate refund requests rejected and webhook duplicate handling
func TestRefundReliability_DuplicateRefundRequests(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	admin, _, _, prodA, _, orderRes := helperCreateMultiVendorOrder(t, env, ctx)
	_, _ = env.MarketplaceService.ConfirmPayment(ctx, orderRes.Payment.Reference)

	// Initiate full refund
	ref1, _, err := env.MarketplaceService.InitiateRefund(ctx, admin, orderRes.Order.ID, service.RefundOrderInput{
		AmountKobo: orderRes.Order.TotalKobo,
		Reason:     "First refund request",
	})
	if err != nil {
		t.Fatalf("Initial refund initiation failed: %v", err)
	}

	// Immediately attempt duplicate refund request for the same order -> MUST BE REJECTED
	_, _, dupErr := env.MarketplaceService.InitiateRefund(ctx, admin, orderRes.Order.ID, service.RefundOrderInput{
		AmountKobo: orderRes.Order.TotalKobo,
		Reason:     "Duplicate refund request",
	})
	if dupErr == nil {
		t.Fatal("Expected duplicate refund request to be rejected, got nil")
	}
	if !strings.Contains(dupErr.Error(), "already fully refunded") && !strings.Contains(dupErr.Error(), "pending refund reservations") {
		t.Fatalf("Expected already refunded or pending reservation error, got: %v", dupErr)
	}

	// Also attempt partial refund exceeding remaining amount -> MUST BE REJECTED
	_, _, excessErr := env.MarketplaceService.InitiateRefund(ctx, admin, orderRes.Order.ID, service.RefundOrderInput{
		AmountKobo: prodA.PriceKobo,
		Reason:     "Excess partial refund",
	})
	if excessErr == nil {
		t.Fatal("Expected excess partial refund to be rejected, got nil")
	}

	// Simulate idempotent webhook delivery for refund.processed
	pRef := ref1.ProviderRefundReference
	webhookPayload := []byte(fmt.Sprintf(`{
		"event": "refund.processed",
		"data": {
			"status": "processed",
			"refund_reference": "%s",
			"transaction_reference": "%s",
			"amount": %d,
			"currency": "NGN"
		}
	}`, pRef, orderRes.Payment.Reference, orderRes.Order.TotalKobo))

	// First webhook delivery -> succeeds
	if err := env.MarketplaceService.HandleWebhook(ctx, "valid-paystack-signature", webhookPayload); err != nil {
		t.Fatalf("First webhook delivery failed: %v", err)
	}

	// Second duplicate webhook delivery -> idempotent harmless no-op
	if err := env.MarketplaceService.HandleWebhook(ctx, "valid-paystack-signature", webhookPayload); err != nil {
		t.Fatalf("Duplicate webhook delivery returned error: %v", err)
	}

	// Assert final order payment status is 'refunded'
	ord, _, _, _ := env.MarketplaceService.GetOrderByID(ctx, admin, orderRes.Order.ID)
	if ord.PaymentStatus != "refunded" {
		t.Errorf("Expected order payment status 'refunded', got %s", ord.PaymentStatus)
	}
}

// 4. Server restart during pending refund
func TestRefundReliability_ServerRestartDuringPendingRefund(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()

	admin, _, _, _, _, orderRes := helperCreateMultiVendorOrder(t, env, ctx)
	_, _ = env.MarketplaceService.ConfirmPayment(ctx, orderRes.Payment.Reference)

	buyerUser := &db.User{ID: orderRes.Order.BuyerUserID.String, PrimaryRole: "member"}
	_, err := env.MarketplaceService.CancelOrder(ctx, buyerUser, orderRes.Order.ID, "Cancellation before restart")
	if err != nil {
		t.Fatalf("CancelOrder failed: %v", err)
	}

	// Close old connection pool to simulate complete server shutdown
	env.Pool.Close()

	// Simulate server restart: initialize fresh environment from persistent database
	newEnv := setupTestEnv(t)
	defer newEnv.Pool.Close()

	// Verify pending refund persisted across server restart
	foundRefund, err := newEnv.Repo.Queries().GetPendingRefundByOrder(ctx, orderRes.Order.ID)
	if err != nil {
		t.Fatalf("Pending refund for order %s was not found after server restart: %v", orderRes.Order.ID, err)
	}
	if foundRefund.Status != "pending" {
		t.Errorf("Expected refund status 'pending' after restart, got %s", foundRefund.Status)
	}

	// Verify durable RefundWorker lifecycle
	refundWorker := worker.NewRefundWorker(newEnv.MarketplaceService, &worker.RefundWorkerConfig{
		PollInterval: 50 * time.Millisecond,
		BatchSize:    50,
	})
	refundWorker.Start(ctx)
	time.Sleep(100 * time.Millisecond)
	refundWorker.Stop()

	// Drain recovery batches until this order's refund is settled
	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	for {
		r, rErr := newEnv.Repo.Queries().GetMarketplaceRefundByID(ctx, foundRefund.ID)
		if rErr == nil && r.Status == "successful" {
			break
		}
		_, _ = newEnv.MarketplaceService.ProcessPendingRefunds(ctx, 100)
		select {
		case <-timeoutCtx.Done():
			t.Fatalf("Timeout waiting for pending refund %s to be settled by recovery worker", foundRefund.ID)
		case <-time.After(20 * time.Millisecond):
		}
	}

	// Verify refund was authoritatively settled by durable recovery worker
	settledRefund, err := newEnv.Repo.Queries().GetMarketplaceRefundByID(ctx, foundRefund.ID)
	if err != nil {
		t.Fatalf("GetMarketplaceRefundByID failed: %v", err)
	}
	if settledRefund.Status != "successful" {
		t.Errorf("Expected refund status 'successful' after recovery cycle, got %s", settledRefund.Status)
	}
	if settledRefund.Status != "successful" {
		t.Errorf("Expected refund status 'successful' after recovery cycle, got %s", settledRefund.Status)
	}

	// Verify order payment status is now refunded
	ord, _, _, _ := newEnv.MarketplaceService.GetOrderByID(ctx, admin, orderRes.Order.ID)
	if ord.PaymentStatus != "refunded" {
		t.Errorf("Expected payment status 'refunded', got %s", ord.PaymentStatus)
	}
}

// 5. Partial refund followed by remaining seller dispatch
func TestRefundReliability_PartialRefundFollowedByRemainingSellerDispatch(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	admin, sellerA, sellerB, prodA, prodB, orderRes := helperCreateMultiVendorOrder(t, env, ctx)
	admin2 := createRegisteredUser(t, env, fmt.Sprintf("conf_adm_%d", time.Now().UnixNano()), "super_admin")
	_, _ = env.MarketplaceService.ConfirmPayment(ctx, orderRes.Payment.Reference)

	shipments, err := env.MarketplaceService.ListOrderShipments(ctx, admin, orderRes.Order.ID)
	if err != nil {
		t.Fatalf("ListOrderShipments failed: %v", err)
	}
	var shipAId, shipBId string
	for _, s := range shipments {
		if s.Shipment.BusinessID.String == prodA.BusinessID {
			shipAId = s.Shipment.ID
		} else if s.Shipment.BusinessID.String == prodB.BusinessID {
			shipBId = s.Shipment.ID
		}
	}

	// Get an order item for Seller B
	orderItems, _ := env.Repo.Queries().ListMarketplaceOrderItemsByOrder(ctx, orderRes.Order.ID)
	var itemBId string
	for _, itm := range orderItems {
		if itm.BusinessID == prodB.BusinessID {
			itemBId = itm.ID
		}
	}

	// Partial refund specifically attributed to item and shipment
	refB, _, err := env.MarketplaceService.InitiateRefund(ctx, admin, orderRes.Order.ID, service.RefundOrderInput{
		AmountKobo:  prodB.PriceKobo * 3,
		Reason:      "Seller B item damaged in transit",
		ShipmentID:  shipBId,
		OrderItemID: itemBId,
	})
	if err != nil {
		t.Fatalf("InitiateRefund failed: %v", err)
	}
	if !refB.ShipmentID.Valid || refB.ShipmentID.String != shipBId {
		t.Errorf("Expected refund attributed to shipment B %s, got %v", shipBId, refB.ShipmentID)
	}
	if !refB.OrderItemID.Valid || refB.OrderItemID.String != itemBId {
		t.Errorf("Expected refund attributed to item B %s, got %v", itemBId, refB.OrderItemID)
	}

	// Settle refund
	_, _, err = env.MarketplaceService.ConfirmRefund(ctx, admin2, refB.ID, service.ConfirmRefundInput{
		ProviderRefundReference: refB.ProviderRefundReference,
	})
	if err != nil {
		t.Fatalf("ConfirmRefund failed: %v", err)
	}

	// Check shipment item attribution in DB
	si, err := env.Repo.Queries().GetShipmentItemByOrderItemID(ctx, itemBId)
	if err != nil {
		t.Fatalf("GetShipmentItemByOrderItemID failed: %v", err)
	}
	if si.RefundedQuantity != si.Quantity {
		t.Errorf("Expected shipment item refunded quantity %d, got %d", si.Quantity, si.RefundedQuantity)
	}
	if !si.RefundID.Valid || si.RefundID.String != refB.ID {
		t.Errorf("Expected shipment item refund ID %s, got %v", refB.ID, si.RefundID)
	}

	// Verify Shipment B is cancelled
	shB, _ := env.MarketplaceService.GetShipmentByID(ctx, admin, shipBId)
	if shB.Shipment.Status != "cancelled" {
		t.Errorf("Expected shipment B status cancelled, got %s", shB.Shipment.Status)
	}

	// Seller B attempting to dispatch is REJECTED
	_, err = env.MarketplaceService.DispatchShipment(ctx, sellerB, shipBId, service.DispatchOrderInput{
		CarrierName:    "Rogue Delivery",
		TrackingNumber: "ROGUE-B",
	})
	if err == nil {
		t.Fatal("Expected dispatch of cancelled/refunded shipment to be rejected, got nil")
	}

	// Seller A dispatches their unaffected paid consignment -> SUCCEEDS!
	shA, err := env.MarketplaceService.DispatchShipment(ctx, sellerA, shipAId, service.DispatchOrderInput{
		CarrierName:    "DHL Nigeria",
		TrackingNumber: "DHL-A-SUCCESS",
	})
	if err != nil {
		t.Fatalf("Seller A dispatch failed: %v", err)
	}
	if shA.Status != "dispatched" {
		t.Errorf("Expected shipment A status 'dispatched', got %s", shA.Status)
	}

	// Verify order status
	ord, _, _, _ := env.MarketplaceService.GetOrderByID(ctx, admin, orderRes.Order.ID)
	if ord.PaymentStatus != "partially_refunded" {
		t.Errorf("Expected payment status 'partially_refunded', got %s", ord.PaymentStatus)
	}
	if ord.FulfilmentStatus != "dispatched" && ord.FulfilmentStatus != "partially_dispatched" {
		t.Errorf("Expected fulfilment status dispatched or partially_dispatched, got %s", ord.FulfilmentStatus)
	}
}

// 6. Duplicate inventory restoration prevention
func TestRefundReliability_DuplicateInventoryRestorationPrevention(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	admin := createRegisteredUser(t, env, fmt.Sprintf("dup_adm_%d", time.Now().UnixNano()), "super_admin")
	seller, _, prod := helperCreateSellerWithProduct(t, env, ctx, fmt.Sprintf("dup_sel_%d", time.Now().UnixNano()), "Stock Store", "Stock Widget", 100, admin)
	_ = seller
	buyer := createRegisteredUser(t, env, fmt.Sprintf("dup_buy_%d", time.Now().UnixNano()), "member")

	// Order with two cart line items of the same product
	orderRes, err := env.MarketplaceService.CreateOrder(ctx, buyer, service.CheckoutInput{
		Items: []service.CartItemInput{
			{ProductID: prod.ID, Quantity: 4},
			{ProductID: prod.ID, Quantity: 6},
		},
		BuyerName:       "Duplicate Test Buyer",
		BuyerEmail:      "dupbuyer@example.org",
		BuyerPhone:      "+2348000000000",
		DeliveryType:    "delivery",
		ShippingAddress: "Victoria Island, Lagos",
	})
	if err != nil {
		t.Fatalf("CreateOrder failed: %v", err)
	}

	// Product stock after checkout: 100 - 10 = 90
	pAfterOrder, _ := env.MarketplaceService.GetProductByID(ctx, prod.ID)
	if pAfterOrder.StockQuantity != 90 {
		t.Fatalf("Expected stock 90 after checkout, got %d", pAfterOrder.StockQuantity)
	}

	// Confirm payment
	_, _ = env.MarketplaceService.ConfirmPayment(ctx, orderRes.Payment.Reference)

	// Cancel order
	_, err = env.MarketplaceService.CancelOrder(ctx, buyer, orderRes.Order.ID, "Cancellation test")
	if err != nil {
		t.Fatalf("CancelOrder failed: %v", err)
	}

	// Stock after cancellation: restored back to 100
	pAfterCancel, _ := env.MarketplaceService.GetProductByID(ctx, prod.ID)
	if pAfterCancel.StockQuantity != 100 {
		t.Fatalf("Expected stock 100 after cancellation, got %d", pAfterCancel.StockQuantity)
	}

	// Verify exactly 1 release log exists in inventory ledger for this order and product
	count, err := env.Repo.Queries().CountOrderInventoryReleaseLogs(ctx, db.CountOrderInventoryReleaseLogsParams{
		OrderID:   service.TextFromString(orderRes.Order.ID),
		ProductID: prod.ID,
	})
	if err != nil || count != 1 {
		t.Fatalf("Expected exactly 1 release log for product, got %d (err: %v)", count, err)
	}

	// Settle refund (must NOT double-increment stock)
	refunds, _ := env.MarketplaceService.ListRefundsByOrder(ctx, admin, orderRes.Order.ID)
	if len(refunds) > 0 {
		_, _, _ = env.MarketplaceService.ConfirmRefund(ctx, admin, refunds[0].ID, service.ConfirmRefundInput{
			ProviderRefundReference: refunds[0].ProviderRefundReference,
		})
	}

	pAfterRefundSettled, _ := env.MarketplaceService.GetProductByID(ctx, prod.ID)
	if pAfterRefundSettled.StockQuantity != 100 {
		t.Fatalf("Stock was improperly altered upon refund settlement! Expected 100, got %d", pAfterRefundSettled.StockQuantity)
	}
}

// 7. Migration 22 compatibility, constraints, and data preservation
func TestRefundReliability_Migration22Compatibility(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	// 1. Verify schema columns on marketplace_refunds
	var colCount int
	err := env.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM information_schema.columns
		WHERE table_name = 'marketplace_refunds'
		  AND column_name IN ('shipment_id', 'business_id', 'order_item_id')
	`).Scan(&colCount)
	if err != nil || colCount != 3 {
		t.Fatalf("Expected 3 attribution columns on marketplace_refunds, found %d (err: %v)", colCount, err)
	}

	// 2. Verify schema columns on marketplace_shipment_items
	var itemColCount int
	err = env.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM information_schema.columns
		WHERE table_name = 'marketplace_shipment_items'
		  AND column_name IN ('refunded_quantity', 'refund_id')
	`).Scan(&itemColCount)
	if err != nil || itemColCount != 2 {
		t.Fatalf("Expected 2 refund tracking columns on marketplace_shipment_items, found %d (err: %v)", itemColCount, err)
	}

	// 3. Test check constraint on marketplace_shipment_items: refunded_quantity cannot exceed quantity
	testShipmentID := security.GenerateUUID()
	testOrderID := security.GenerateUUID()
	testOrderNum := fmt.Sprintf("MIG22-T-%d", time.Now().UnixNano())
	_, err = env.Pool.Exec(ctx, `
		INSERT INTO marketplace_orders (
			id, order_number, buyer_name, buyer_email, buyer_phone,
			delivery_type, shipping_address, subtotal_kobo, delivery_fee_kobo, total_kobo,
			order_status, payment_status, fulfilment_status
		) VALUES (
			$1, $2, 'Mig Test', 'mig@example.org', '+2348000000000',
			'delivery', 'Test Address', 1000, 0, 1000,
			'placed', 'paid', 'unfulfilled'
		)
	`, testOrderID, testOrderNum)
	if err != nil {
		t.Fatalf("Failed to create test order for migration test: %v", err)
	}

	_, err = env.Pool.Exec(ctx, `
		INSERT INTO marketplace_shipments (id, order_id, status)
		VALUES ($1, $2, 'pending')
	`, testShipmentID, testOrderID)
	if err != nil {
		t.Fatalf("Failed to create test shipment: %v", err)
	}

	admin := createRegisteredUser(t, env, fmt.Sprintf("mig_adm_%d", time.Now().UnixNano()), "super_admin")
	_, biz, prod := helperCreateSellerWithProduct(t, env, ctx, fmt.Sprintf("mig_sel_%d", time.Now().UnixNano()), "Mig Store", "Mig Prod", 10, admin)

	testOrderItemID := security.GenerateUUID()
	_, err = env.Pool.Exec(ctx, `
		INSERT INTO marketplace_order_items (
			id, order_id, product_id, title_snapshot, sku_snapshot, price_kobo_snapshot, quantity, subtotal_kobo, business_id
		) VALUES (
			$1, $2, $3, 'Item', 'SKU', 1000, 2, 2000, $4
		)
	`, testOrderItemID, testOrderID, prod.ID, biz.ID)
	if err != nil {
		t.Fatalf("Failed to create test order item: %v", err)
	}

	// Inserting refunded_quantity > quantity MUST violate check constraint
	_, checkErr := env.Pool.Exec(ctx, `
		INSERT INTO marketplace_shipment_items (id, shipment_id, order_item_id, quantity, refunded_quantity)
		VALUES ($1, $2, $3, 2, 5)
	`, security.GenerateUUID(), testShipmentID, testOrderItemID)
	if checkErr == nil {
		t.Fatal("Expected check constraint violation when refunded_quantity > quantity, got nil")
	}

	// Inserting negative refunded_quantity MUST violate check constraint
	_, negErr := env.Pool.Exec(ctx, `
		INSERT INTO marketplace_shipment_items (id, shipment_id, order_item_id, quantity, refunded_quantity)
		VALUES ($1, $2, $3, 2, -1)
	`, security.GenerateUUID(), testShipmentID, testOrderItemID)
	if negErr == nil {
		t.Fatal("Expected check constraint violation when refunded_quantity < 0, got nil")
	}

	// 4. Test unique constraint uq_idx_mkt_shipments_order_business with NULLS NOT DISTINCT
	testOrderUq := security.GenerateUUID()
	testOrderUqNum := fmt.Sprintf("MIG22-UQ-%d", time.Now().UnixNano())
	_, err = env.Pool.Exec(ctx, `
		INSERT INTO marketplace_orders (
			id, order_number, buyer_name, buyer_email, buyer_phone,
			delivery_type, shipping_address, subtotal_kobo, delivery_fee_kobo, total_kobo,
			order_status, payment_status, fulfilment_status
		) VALUES (
			$1, $2, 'Mig UQ', 'miguq@example.org', '+2348000000000',
			'delivery', 'Test Address', 1000, 0, 1000,
			'placed', 'paid', 'unfulfilled'
		)
	`, testOrderUq, testOrderUqNum)
	if err != nil {
		t.Fatalf("Failed to create test order UQ: %v", err)
	}

	// First shipment with NULL business_id succeeds
	_, err = env.Pool.Exec(ctx, `
		INSERT INTO marketplace_shipments (id, order_id, business_id, status)
		VALUES ($1, $2, NULL, 'pending')
	`, security.GenerateUUID(), testOrderUq)
	if err != nil {
		t.Fatalf("First shipment with NULL business_id failed: %v", err)
	}

	// Second shipment with NULL business_id on same order MUST be rejected by uq_idx_mkt_shipments_order_business
	_, uqErr := env.Pool.Exec(ctx, `
		INSERT INTO marketplace_shipments (id, order_id, business_id, status)
		VALUES ($1, $2, NULL, 'pending')
	`, security.GenerateUUID(), testOrderUq)
	if uqErr == nil {
		t.Fatal("Expected unique constraint violation for duplicate shipment with NULL business_id, got nil")
	}
}

// 8. Concurrent refund and shipment operations (no deadlocks, deterministic state)
func TestRefundReliability_ConcurrentRefundAndShipmentOperations(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	admin, sellerA, _, prodA, _, orderRes := helperCreateMultiVendorOrder(t, env, ctx)
	admin2 := createRegisteredUser(t, env, fmt.Sprintf("race_adm2_%d", time.Now().UnixNano()), "super_admin")
	_, _ = env.MarketplaceService.ConfirmPayment(ctx, orderRes.Payment.Reference)

	shipments, _ := env.MarketplaceService.ListOrderShipments(ctx, admin, orderRes.Order.ID)
	var shipAId string
	for _, s := range shipments {
		if s.Shipment.BusinessID.String == prodA.BusinessID {
			shipAId = s.Shipment.ID
		}
	}

	var wg sync.WaitGroup
	wg.Add(2)
	errCh := make(chan error, 2)

	// Goroutine 1: Seller A dispatches shipment A
	go func() {
		defer wg.Done()
		_, err := env.MarketplaceService.DispatchShipment(ctx, sellerA, shipAId, service.DispatchOrderInput{
			CarrierName:    "FedEx Express",
			TrackingNumber: "FEDEX-RACE-01",
		})
		errCh <- err
	}()

	// Goroutine 2: Admin initiates and confirms refund for Shipment A
	go func() {
		defer wg.Done()
		ref, _, err := env.MarketplaceService.InitiateRefund(ctx, admin, orderRes.Order.ID, service.RefundOrderInput{
			AmountKobo: prodA.PriceKobo * 2,
			Reason:     "Concurrent refund test",
			ShipmentID: shipAId,
		})
		if err == nil && ref != nil {
			_, _, err = env.MarketplaceService.ConfirmRefund(ctx, admin2, ref.ID, service.ConfirmRefundInput{
				ProviderRefundReference: ref.ProviderRefundReference,
			})
		}
		errCh <- err
	}()

	wg.Wait()
	close(errCh)

	// Assert NO deadlocks occurred (Order-first lock hierarchy eliminates deadlocks)
	for err := range errCh {
		if err != nil {
			t.Logf("Concurrent error received: %v", err)
			errStr := strings.ToLower(err.Error())
			if strings.Contains(errStr, "deadlock detected") || strings.Contains(errStr, "40p01") {
				t.Fatalf("Deadlock detected during concurrent refund and shipment: %v", err)
			}
		}
	}

	// Verify database state is completely consistent
	ord, _, _, _ := env.MarketplaceService.GetOrderByID(ctx, admin, orderRes.Order.ID)
	shA, _ := env.MarketplaceService.GetShipmentByID(ctx, admin, shipAId)

	// Either dispatch won and shipment is dispatched, OR refund won and shipment is cancelled
	if shA.Shipment.Status != "dispatched" && shA.Shipment.Status != "cancelled" {
		t.Errorf("Unexpected shipment status in concurrent operation: %s", shA.Shipment.Status)
	}

	if shA.Shipment.Status == "cancelled" {
		if ord.PaymentStatus != "partially_refunded" && ord.PaymentStatus != "refunded" {
			t.Errorf("Expected payment status partially_refunded or refunded, got %s", ord.PaymentStatus)
		}
	}
}
