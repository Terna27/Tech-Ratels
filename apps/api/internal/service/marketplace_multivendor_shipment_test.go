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
	"ratels-api/internal/security"
	"ratels-api/internal/service"

	"github.com/jackc/pgx/v5/pgtype"
)

// Helper to create a multi-vendor order with two sellers
func helperCreateMultiVendorOrder(t *testing.T, env *TestEnv, ctx context.Context) (*db.User, *db.User, *db.User, *db.MarketplaceProduct, *db.MarketplaceProduct, *service.CreateOrderResult) {
	t.Helper()
	admin := createRegisteredUser(t, env, fmt.Sprintf("mv_adm_%d", time.Now().UnixNano()), "super_admin")
	sellerA, _, prodA := helperCreateSellerWithProduct(t, env, ctx, fmt.Sprintf("mv_sel_a_%d", time.Now().UnixNano()), "Store A", "Product A", 50, admin)
	sellerB, _, prodB := helperCreateSellerWithProduct(t, env, ctx, fmt.Sprintf("mv_sel_b_%d", time.Now().UnixNano()), "Store B", "Product B", 50, admin)
	buyer := createRegisteredUser(t, env, fmt.Sprintf("mv_buy_%d", time.Now().UnixNano()), "member")

	orderRes, err := env.MarketplaceService.CreateOrder(ctx, buyer, service.CheckoutInput{
		Items: []service.CartItemInput{
			{ProductID: prodA.ID, Quantity: 2},
			{ProductID: prodB.ID, Quantity: 3},
		},
		BuyerName:       "Multi Vendor Buyer",
		BuyerEmail:      "mvbuyer@example.org",
		BuyerPhone:      "+2348001112233",
		DeliveryType:    "delivery",
		ShippingAddress: "12 Marina Road, Lagos",
	})
	if err != nil {
		t.Fatalf("CreateOrder failed: %v", err)
	}

	return admin, sellerA, sellerB, prodA, prodB, orderRes
}

// 1. Single-vendor shipment creation
func TestMultiVendor_SingleVendorShipmentCreation(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	admin := createRegisteredUser(t, env, fmt.Sprintf("sv_adm_%d", time.Now().UnixNano()), "super_admin")
	seller, biz, prod := helperCreateSellerWithProduct(t, env, ctx, fmt.Sprintf("sv_sel_%d", time.Now().UnixNano()), "Single Store", "Solo Product", 30, admin)
	_ = seller
	buyer := createRegisteredUser(t, env, fmt.Sprintf("sv_buy_%d", time.Now().UnixNano()), "member")

	orderRes, err := env.MarketplaceService.CreateOrder(ctx, buyer, service.CheckoutInput{
		Items: []service.CartItemInput{
			{ProductID: prod.ID, Quantity: 2},
		},
		BuyerName:       "Single Buyer",
		BuyerEmail:      "single@example.org",
		BuyerPhone:      "+2348011223344",
		DeliveryType:    "delivery",
		ShippingAddress: "15 Broad Street, Lagos",
	})
	if err != nil {
		t.Fatalf("CreateOrder failed: %v", err)
	}

	// Confirm payment
	_, err = env.MarketplaceService.ConfirmPayment(ctx, orderRes.Payment.Reference)
	if err != nil {
		t.Fatalf("ConfirmPayment failed: %v", err)
	}

	// Verify shipments
	shipments, err := env.MarketplaceService.ListOrderShipments(ctx, buyer, orderRes.Order.ID)
	if err != nil {
		t.Fatalf("ListOrderShipments failed: %v", err)
	}

	if len(shipments) != 1 {
		t.Fatalf("Expected exactly 1 shipment for single-vendor order, got %d", len(shipments))
	}

	sh := shipments[0]
	if sh.Shipment.BusinessID.String != biz.ID {
		t.Errorf("Expected shipment business_id %s, got %s", biz.ID, sh.Shipment.BusinessID.String)
	}
	if sh.Shipment.Status != "pending" {
		t.Errorf("Expected initial shipment status 'pending', got %s", sh.Shipment.Status)
	}
	if len(sh.Items) != 1 || sh.Items[0].Quantity != 2 {
		t.Errorf("Expected 1 shipment item with quantity 2, got %+v", sh.Items)
	}
}

// 2. Multi-vendor shipment splitting
func TestMultiVendor_ShipmentSplitting(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	_, sellerA, sellerB, prodA, prodB, orderRes := helperCreateMultiVendorOrder(t, env, ctx)

	// Confirm payment
	_, err := env.MarketplaceService.ConfirmPayment(ctx, orderRes.Payment.Reference)
	if err != nil {
		t.Fatalf("ConfirmPayment failed: %v", err)
	}

	// Buyer checks shipments
	buyer := createRegisteredUser(t, env, fmt.Sprintf("check_buy_%d", time.Now().UnixNano()), "super_admin") // super_admin to view
	shipments, err := env.MarketplaceService.ListOrderShipments(ctx, buyer, orderRes.Order.ID)
	if err != nil {
		t.Fatalf("ListOrderShipments failed: %v", err)
	}

	if len(shipments) != 2 {
		t.Fatalf("Expected 2 shipments for multi-vendor order, got %d", len(shipments))
	}

	// Verify each shipment belongs to the right seller and contains correct items
	var shipA, shipB *service.ShipmentDetailResult
	for i := range shipments {
		if shipments[i].Shipment.BusinessID.String == prodA.BusinessID {
			shipA = &shipments[i]
		} else if shipments[i].Shipment.BusinessID.String == prodB.BusinessID {
			shipB = &shipments[i]
		}
	}

	if shipA == nil || shipB == nil {
		t.Fatalf("Expected one shipment for Store A and one for Store B, got %+v", shipments)
	}

	if len(shipA.Items) != 1 || shipA.Items[0].ProductID.String != prodA.ID || shipA.Items[0].Quantity != 2 {
		t.Errorf("Shipment A items incorrect: %+v", shipA.Items)
	}
	if len(shipB.Items) != 1 || shipB.Items[0].ProductID.String != prodB.ID || shipB.Items[0].Quantity != 3 {
		t.Errorf("Shipment B items incorrect: %+v", shipB.Items)
	}

	// Check seller-isolated visibility
	selAShipments, err := env.MarketplaceService.ListOrderShipments(ctx, sellerA, orderRes.Order.ID)
	if err != nil {
		t.Fatalf("Seller A ListOrderShipments failed: %v", err)
	}
	if len(selAShipments) != 1 || selAShipments[0].Shipment.BusinessID.String != prodA.BusinessID {
		t.Errorf("Seller A should only see their own shipment, got %d shipments", len(selAShipments))
	}

	selBShipments, err := env.MarketplaceService.ListOrderShipments(ctx, sellerB, orderRes.Order.ID)
	if err != nil {
		t.Fatalf("Seller B ListOrderShipments failed: %v", err)
	}
	if len(selBShipments) != 1 || selBShipments[0].Shipment.BusinessID.String != prodB.BusinessID {
		t.Errorf("Seller B should only see their own shipment, got %d shipments", len(selBShipments))
	}
}

// 3. Duplicate payment webhook idempotency
func TestMultiVendor_DuplicatePaymentWebhookIdempotency(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	admin, _, _, _, _, orderRes := helperCreateMultiVendorOrder(t, env, ctx)

	// First payment confirmation
	_, err := env.MarketplaceService.ConfirmPayment(ctx, orderRes.Payment.Reference)
	if err != nil {
		t.Fatalf("First ConfirmPayment failed: %v", err)
	}

	// Second payment confirmation (webhook retry)
	_, err = env.MarketplaceService.ConfirmPayment(ctx, orderRes.Payment.Reference)
	if err != nil {
		t.Fatalf("Second ConfirmPayment failed: %v", err)
	}

	// Verify shipments were not duplicated
	shipments, err := env.MarketplaceService.ListOrderShipments(ctx, admin, orderRes.Order.ID)
	if err != nil {
		t.Fatalf("ListOrderShipments failed: %v", err)
	}
	if len(shipments) != 2 {
		t.Fatalf("Expected exactly 2 shipments after duplicate webhook, got %d", len(shipments))
	}
}

// 4. Seller cross-tenant access rejection
func TestMultiVendor_SellerCrossTenantAccessRejection(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	admin, sellerA, sellerB, prodA, _, orderRes := helperCreateMultiVendorOrder(t, env, ctx)
	_, err := env.MarketplaceService.ConfirmPayment(ctx, orderRes.Payment.Reference)
	if err != nil {
		t.Fatalf("ConfirmPayment failed: %v", err)
	}

	shipments, _ := env.MarketplaceService.ListOrderShipments(ctx, admin, orderRes.Order.ID)
	var shipBId string
	for _, s := range shipments {
		if s.Shipment.BusinessID.String != prodA.BusinessID {
			shipBId = s.Shipment.ID
		}
	}

	// Seller A attempts to dispatch Seller B's shipment -> Must be rejected!
	_, err = env.MarketplaceService.DispatchShipment(ctx, sellerA, shipBId, service.DispatchOrderInput{
		CarrierName:    "Rogue Courier",
		TrackingNumber: "TRK-ROGUE",
	})
	if err == nil {
		t.Fatal("Expected error when Seller A dispatches Seller B's shipment, got nil")
	}
	if !errors.Is(err, service.ErrUnauthorizedSellerAction) {
		t.Fatalf("Expected ErrUnauthorizedSellerAction, got %v", err)
	}

	// Seller A attempts to update Seller B's tracking info -> Must be rejected!
	_, err = env.MarketplaceService.UpdateShipmentTracking(ctx, sellerA, shipBId, service.UpdateShipmentTrackingInput{
		CarrierName:    "Rogue Courier",
		TrackingNumber: "TRK-ROGUE",
	})
	if err == nil {
		t.Fatal("Expected error when Seller A updates Seller B's tracking, got nil")
	}

	// Seller B can dispatch Seller B's shipment -> Allowed!
	shB, err := env.MarketplaceService.DispatchShipment(ctx, sellerB, shipBId, service.DispatchOrderInput{
		CarrierName:    "GIG Logistics",
		TrackingNumber: "GIG-VALID-B",
	})
	if err != nil {
		t.Fatalf("Seller B failed to dispatch own shipment: %v", err)
	}
	if shB.Status != "dispatched" {
		t.Errorf("Expected dispatched status, got %s", shB.Status)
	}
}

// 5. Unpaid order dispatch rejection
func TestMultiVendor_UnpaidOrderDispatchRejection(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	_, sellerA, _, _, _, orderRes := helperCreateMultiVendorOrder(t, env, ctx)

	// Attempting to dispatch an unpaid order must fail
	_, err := env.MarketplaceService.DispatchOrder(ctx, sellerA, orderRes.Order.ID, service.DispatchOrderInput{
		CarrierName:    "FedEx",
		TrackingNumber: "FDX-UNPAID",
	})
	if err == nil {
		t.Fatal("Expected error dispatching unpaid order, got nil")
	}
	if !errors.Is(err, service.ErrOrderNotPaid) {
		t.Fatalf("Expected ErrOrderNotPaid, got %v", err)
	}
}

// 6. Reconciliation order dispatch rejection
func TestMultiVendor_ReconciliationOrderDispatchRejection(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	admin, sellerA, _, prodA, _, orderRes := helperCreateMultiVendorOrder(t, env, ctx)
	_, _ = env.MarketplaceService.ConfirmPayment(ctx, orderRes.Payment.Reference)

	// Put order into requires_reconciliation
	_, err := env.Pool.Exec(ctx, `
		UPDATE marketplace_orders
		SET order_status = 'requires_reconciliation', reconciliation_required = TRUE
		WHERE id = $1
	`, orderRes.Order.ID)
	if err != nil {
		t.Fatalf("Failed to update order to requires_reconciliation: %v", err)
	}

	shipments, _ := env.MarketplaceService.ListOrderShipments(ctx, admin, orderRes.Order.ID)
	var shipAId string
	for _, s := range shipments {
		if s.Shipment.BusinessID.String == prodA.BusinessID {
			shipAId = s.Shipment.ID
		}
	}

	// Dispatch must be rejected
	_, err = env.MarketplaceService.DispatchShipment(ctx, sellerA, shipAId, service.DispatchOrderInput{
		CarrierName:    "DHL",
		TrackingNumber: "DHL-REC",
	})
	if err == nil {
		t.Fatal("Expected error dispatching shipment of reconciliation order, got nil")
	}
	if !errors.Is(err, service.ErrOrderReconciliationRequired) {
		t.Fatalf("Expected ErrOrderReconciliationRequired, got %v", err)
	}
}

// 7. Released-stock dispatch rejection
func TestMultiVendor_ReleasedStockDispatchRejection(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	admin, sellerA, _, prodA, _, orderRes := helperCreateMultiVendorOrder(t, env, ctx)
	_, _ = env.MarketplaceService.ConfirmPayment(ctx, orderRes.Payment.Reference)

	// Set stock_released = true
	_, err := env.Pool.Exec(ctx, `
		UPDATE marketplace_orders
		SET stock_released = TRUE
		WHERE id = $1
	`, orderRes.Order.ID)
	if err != nil {
		t.Fatalf("Failed to update order stock_released: %v", err)
	}

	shipments, _ := env.MarketplaceService.ListOrderShipments(ctx, admin, orderRes.Order.ID)
	var shipAId string
	for _, s := range shipments {
		if s.Shipment.BusinessID.String == prodA.BusinessID {
			shipAId = s.Shipment.ID
		}
	}

	_, err = env.MarketplaceService.DispatchShipment(ctx, sellerA, shipAId, service.DispatchOrderInput{
		CarrierName:    "DHL",
		TrackingNumber: "DHL-REL",
	})
	if err == nil {
		t.Fatal("Expected error dispatching shipment with released stock, got nil")
	}
	if !errors.Is(err, service.ErrOrderStockAlreadyReleased) {
		t.Fatalf("Expected ErrOrderStockAlreadyReleased, got %v", err)
	}
}

// 8. Independent shipment dispatch
func TestMultiVendor_IndependentShipmentDispatch(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	admin, sellerA, sellerB, prodA, prodB, orderRes := helperCreateMultiVendorOrder(t, env, ctx)
	_ = prodB
	_, _ = env.MarketplaceService.ConfirmPayment(ctx, orderRes.Payment.Reference)

	shipments, _ := env.MarketplaceService.ListOrderShipments(ctx, admin, orderRes.Order.ID)
	var shipAId, shipBId string
	for _, s := range shipments {
		if s.Shipment.BusinessID.String == prodA.BusinessID {
			shipAId = s.Shipment.ID
		} else {
			shipBId = s.Shipment.ID
		}
	}

	// Seller A dispatches
	shA, err := env.MarketplaceService.DispatchShipment(ctx, sellerA, shipAId, service.DispatchOrderInput{
		CarrierName:    "Carrier A",
		TrackingNumber: "TRK-A-100",
	})
	if err != nil {
		t.Fatalf("Seller A dispatch failed: %v", err)
	}
	if shA.Status != "dispatched" {
		t.Errorf("Expected Seller A shipment dispatched, got %s", shA.Status)
	}

	// Verify Seller B's shipment remains pending
	shBDetail, err := env.MarketplaceService.GetShipmentByID(ctx, sellerB, shipBId)
	if err != nil {
		t.Fatalf("GetShipmentByID failed: %v", err)
	}
	if shBDetail.Shipment.Status != "pending" {
		t.Errorf("Expected Seller B shipment to remain pending, got %s", shBDetail.Shipment.Status)
	}

	// Verify order status is partially_dispatched
	orderAfterA, _, _, err := env.MarketplaceService.GetOrderByID(ctx, admin, orderRes.Order.ID)
	if err != nil {
		t.Fatalf("GetOrderByID failed: %v", err)
	}
	if orderAfterA.OrderStatus != "partially_dispatched" || orderAfterA.FulfilmentStatus != "partially_dispatched" {
		t.Errorf("Expected partially_dispatched, got order_status=%s, fulfilment_status=%s",
			orderAfterA.OrderStatus, orderAfterA.FulfilmentStatus)
	}
}

// 9. Independent shipment delivery
func TestMultiVendor_IndependentShipmentDelivery(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	admin, sellerA, sellerB, prodA, _, orderRes := helperCreateMultiVendorOrder(t, env, ctx)
	_, _ = env.MarketplaceService.ConfirmPayment(ctx, orderRes.Payment.Reference)

	shipments, _ := env.MarketplaceService.ListOrderShipments(ctx, admin, orderRes.Order.ID)
	var shipAId, shipBId string
	for _, s := range shipments {
		if s.Shipment.BusinessID.String == prodA.BusinessID {
			shipAId = s.Shipment.ID
		} else {
			shipBId = s.Shipment.ID
		}
	}

	// Both sellers dispatch
	_, _ = env.MarketplaceService.DispatchShipment(ctx, sellerA, shipAId, service.DispatchOrderInput{CarrierName: "Carrier A", TrackingNumber: "A1"})
	_, _ = env.MarketplaceService.DispatchShipment(ctx, sellerB, shipBId, service.DispatchOrderInput{CarrierName: "Carrier B", TrackingNumber: "B1"})

	// Buyer delivers Shipment A only
	buyerUser, _ := env.Repo.Queries().GetUserByID(ctx, orderRes.Order.BuyerUserID.String)
	buyerUserDb := &db.User{ID: buyerUser.ID, PrimaryRole: buyerUser.PrimaryRole}
	shADelivered, err := env.MarketplaceService.ConfirmShipmentDelivery(ctx, buyerUserDb, shipAId, "", "Received package A")
	if err != nil {
		t.Fatalf("ConfirmShipmentDelivery failed: %v", err)
	}
	if shADelivered.Status != "delivered" {
		t.Errorf("Expected shipment A status 'delivered', got %s", shADelivered.Status)
	}

	// Order status should be partially_delivered
	orderAfterDeliv, _, _, _ := env.MarketplaceService.GetOrderByID(ctx, admin, orderRes.Order.ID)
	if orderAfterDeliv.OrderStatus != "partially_delivered" || orderAfterDeliv.FulfilmentStatus != "partially_delivered" {
		t.Errorf("Expected partially_delivered, got order_status=%s, fulfilment_status=%s",
			orderAfterDeliv.OrderStatus, orderAfterDeliv.FulfilmentStatus)
	}
}

// 10. Partial order fulfillment aggregation
func TestMultiVendor_PartialOrderFulfillmentAggregation(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	admin, sellerA, sellerB, prodA, _, orderRes := helperCreateMultiVendorOrder(t, env, ctx)
	_, _ = env.MarketplaceService.ConfirmPayment(ctx, orderRes.Payment.Reference)

	// Before dispatch: processing & unfulfilled
	ord0, _, _, _ := env.MarketplaceService.GetOrderByID(ctx, admin, orderRes.Order.ID)
	if ord0.OrderStatus != "processing" || ord0.FulfilmentStatus != "unfulfilled" {
		t.Fatalf("Expected processing / unfulfilled, got %s / %s", ord0.OrderStatus, ord0.FulfilmentStatus)
	}

	shipments, _ := env.MarketplaceService.ListOrderShipments(ctx, admin, orderRes.Order.ID)
	var shipAId, shipBId string
	for _, s := range shipments {
		if s.Shipment.BusinessID.String == prodA.BusinessID {
			shipAId = s.Shipment.ID
		} else {
			shipBId = s.Shipment.ID
		}
	}

	// Dispatch 1 of 2 -> partially_dispatched
	_, _ = env.MarketplaceService.DispatchShipment(ctx, sellerA, shipAId, service.DispatchOrderInput{CarrierName: "Carrier A", TrackingNumber: "A"})
	ord1, _, _, _ := env.MarketplaceService.GetOrderByID(ctx, admin, orderRes.Order.ID)
	if ord1.OrderStatus != "partially_dispatched" || ord1.FulfilmentStatus != "partially_dispatched" {
		t.Errorf("Expected partially_dispatched, got %s / %s", ord1.OrderStatus, ord1.FulfilmentStatus)
	}

	// Dispatch 2 of 2 -> dispatched
	_, _ = env.MarketplaceService.DispatchShipment(ctx, sellerB, shipBId, service.DispatchOrderInput{CarrierName: "Carrier B", TrackingNumber: "B"})
	ord2, _, _, _ := env.MarketplaceService.GetOrderByID(ctx, admin, orderRes.Order.ID)
	if ord2.OrderStatus != "dispatched" || ord2.FulfilmentStatus != "dispatched" {
		t.Errorf("Expected dispatched, got %s / %s", ord2.OrderStatus, ord2.FulfilmentStatus)
	}
}

// 11. Full order delivery aggregation
func TestMultiVendor_FullOrderDeliveryAggregation(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	admin, sellerA, sellerB, prodA, _, orderRes := helperCreateMultiVendorOrder(t, env, ctx)
	_, _ = env.MarketplaceService.ConfirmPayment(ctx, orderRes.Payment.Reference)

	shipments, _ := env.MarketplaceService.ListOrderShipments(ctx, admin, orderRes.Order.ID)
	var shipAId, shipBId string
	for _, s := range shipments {
		if s.Shipment.BusinessID.String == prodA.BusinessID {
			shipAId = s.Shipment.ID
		} else {
			shipBId = s.Shipment.ID
		}
	}

	_, _ = env.MarketplaceService.DispatchShipment(ctx, sellerA, shipAId, service.DispatchOrderInput{CarrierName: "Carrier A", TrackingNumber: "A"})
	_, _ = env.MarketplaceService.DispatchShipment(ctx, sellerB, shipBId, service.DispatchOrderInput{CarrierName: "Carrier B", TrackingNumber: "B"})

	// Delivery 1 of 2 -> partially_delivered
	_, err := env.MarketplaceService.ConfirmShipmentDelivery(ctx, admin, shipAId, "", "A arrived")
	if err != nil {
		t.Fatalf("ConfirmShipmentDelivery A failed: %v", err)
	}
	ord1, _, _, _ := env.MarketplaceService.GetOrderByID(ctx, admin, orderRes.Order.ID)
	if ord1.OrderStatus != "partially_delivered" || ord1.FulfilmentStatus != "partially_delivered" {
		t.Errorf("Expected partially_delivered, got %s / %s", ord1.OrderStatus, ord1.FulfilmentStatus)
	}

	// Delivery 2 of 2 -> delivered
	_, err = env.MarketplaceService.ConfirmShipmentDelivery(ctx, admin, shipBId, "", "B arrived")
	if err != nil {
		t.Fatalf("ConfirmShipmentDelivery B failed: %v", err)
	}
	ord2, _, _, _ := env.MarketplaceService.GetOrderByID(ctx, admin, orderRes.Order.ID)
	if ord2.OrderStatus != "delivered" || ord2.FulfilmentStatus != "delivered" {
		t.Errorf("Expected delivered, got %s / %s", ord2.OrderStatus, ord2.FulfilmentStatus)
	}
}

// 12. Concurrent shipment state transitions
func TestMultiVendor_ConcurrentShipmentStateTransitions(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	admin, sellerA, sellerB, prodA, _, orderRes := helperCreateMultiVendorOrder(t, env, ctx)
	_, _ = env.MarketplaceService.ConfirmPayment(ctx, orderRes.Payment.Reference)

	shipments, _ := env.MarketplaceService.ListOrderShipments(ctx, admin, orderRes.Order.ID)
	var shipAId, shipBId string
	for _, s := range shipments {
		if s.Shipment.BusinessID.String == prodA.BusinessID {
			shipAId = s.Shipment.ID
		} else {
			shipBId = s.Shipment.ID
		}
	}

	var wg sync.WaitGroup
	var errA, errB error

	wg.Add(2)
	go func() {
		defer wg.Done()
		_, errA = env.MarketplaceService.DispatchShipment(ctx, sellerA, shipAId, service.DispatchOrderInput{CarrierName: "A", TrackingNumber: "TRK-A"})
	}()
	go func() {
		defer wg.Done()
		_, errB = env.MarketplaceService.DispatchShipment(ctx, sellerB, shipBId, service.DispatchOrderInput{CarrierName: "B", TrackingNumber: "TRK-B"})
	}()
	wg.Wait()

	if errA != nil {
		t.Errorf("Concurrent dispatch A failed: %v", errA)
	}
	if errB != nil {
		t.Errorf("Concurrent dispatch B failed: %v", errB)
	}

	// Final aggregated state must be 'dispatched'
	finalOrder, _, _, _ := env.MarketplaceService.GetOrderByID(ctx, admin, orderRes.Order.ID)
	if finalOrder.OrderStatus != "dispatched" || finalOrder.FulfilmentStatus != "dispatched" {
		t.Errorf("Expected final status dispatched, got %s / %s", finalOrder.OrderStatus, finalOrder.FulfilmentStatus)
	}
}

// 13. Coordinator geographic authorization
func TestMultiVendor_CoordinatorGeographicAuthorization(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	admin := createRegisteredUser(t, env, fmt.Sprintf("geo_adm_%d", time.Now().UnixNano()), "super_admin")
	_, _, prodA := helperCreateSellerWithProduct(t, env, ctx, fmt.Sprintf("geo_sel_%d", time.Now().UnixNano()), "Geo Store", "Geo Prod", 10, admin)
	buyer := createRegisteredUser(t, env, fmt.Sprintf("geo_buy_%d", time.Now().UnixNano()), "member")

	states, err := env.Repo.Queries().ListStates(ctx)
	if err != nil || len(states) < 2 {
		t.Fatalf("Need at least 2 states, got %d, err=%v", len(states), err)
	}
	lagosState := states[0]
	kanoState := states[1]

	orderRes, err := env.MarketplaceService.CreateOrder(ctx, buyer, service.CheckoutInput{
		Items:           []service.CartItemInput{{ProductID: prodA.ID, Quantity: 1}},
		BuyerName:       "Kano Buyer",
		BuyerEmail:      "kano@example.org",
		BuyerPhone:      "+2348000000000",
		DeliveryType:    "delivery",
		ShippingAddress: "Bompai Road, Kano",
		ShippingStateID: &kanoState.ID,
	})
	if err != nil {
		t.Fatalf("CreateOrder failed: %v", err)
	}
	_, _ = env.MarketplaceService.ConfirmPayment(ctx, orderRes.Payment.Reference)

	shipments, _ := env.MarketplaceService.ListOrderShipments(ctx, admin, orderRes.Order.ID)
	shipId := shipments[0].Shipment.ID

	// Create Lagos Coordinator (Jurisdiction: lagosState.ID)
	lagosCoord := createRegisteredUser(t, env, fmt.Sprintf("lagos_coord_%d", time.Now().UnixNano()), "state_coordinator")
	_, err = env.Repo.Queries().CreateAppointment(ctx, db.CreateAppointmentParams{
		ID:               security.GenerateUUID(),
		UserID:           lagosCoord.ID,
		RoleTitle:        "State Coordinator (Lagos)",
		RoleType:         "state_coordinator",
		JurisdictionType: "state",
		JurisdictionID:   pgtype.Text{String: lagosState.ID, Valid: true},
		StartDate:        pgtype.Timestamptz{Time: time.Now().Add(-1 * time.Hour), Valid: true},
		Status:           "active",
	})
	if err != nil {
		t.Fatalf("Failed to create Lagos appointment: %v", err)
	}

	// Lagos Coordinator attempting to dispatch Kano shipment -> ErrScopeUnauthorized
	_, err = env.MarketplaceService.DispatchShipment(ctx, lagosCoord, shipId, service.DispatchOrderInput{CarrierName: "Carrier", TrackingNumber: "TRK"})
	if err == nil {
		t.Fatal("Expected ErrScopeUnauthorized for Lagos coordinator dispatching Kano shipment, got nil")
	}
	if !errors.Is(err, service.ErrScopeUnauthorized) {
		t.Fatalf("Expected ErrScopeUnauthorized, got %v", err)
	}

	// Create Kano Coordinator (Jurisdiction: kanoState.ID)
	kanoCoord := createRegisteredUser(t, env, fmt.Sprintf("kano_coord_%d", time.Now().UnixNano()), "state_coordinator")
	_, err = env.Repo.Queries().CreateAppointment(ctx, db.CreateAppointmentParams{
		ID:               security.GenerateUUID(),
		UserID:           kanoCoord.ID,
		RoleTitle:        "State Coordinator (Kano)",
		RoleType:         "state_coordinator",
		JurisdictionType: "state",
		JurisdictionID:   pgtype.Text{String: kanoState.ID, Valid: true},
		StartDate:        pgtype.Timestamptz{Time: time.Now().Add(-1 * time.Hour), Valid: true},
		Status:           "active",
	})
	if err != nil {
		t.Fatalf("Failed to create Kano appointment: %v", err)
	}

	// Kano Coordinator dispatching Kano shipment -> Allowed!
	upSh, err := env.MarketplaceService.DispatchShipment(ctx, kanoCoord, shipId, service.DispatchOrderInput{CarrierName: "Kano Courier", TrackingNumber: "KAN-001"})
	if err != nil {
		t.Fatalf("Kano coordinator dispatch failed: %v", err)
	}
	if upSh.Status != "dispatched" {
		t.Errorf("Expected dispatched, got %s", upSh.Status)
	}
}

// 14. Buyer ownership checks
func TestMultiVendor_BuyerOwnershipChecks(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	admin, sellerA, _, prodA, _, orderRes := helperCreateMultiVendorOrder(t, env, ctx)
	_, _ = env.MarketplaceService.ConfirmPayment(ctx, orderRes.Payment.Reference)

	shipments, _ := env.MarketplaceService.ListOrderShipments(ctx, admin, orderRes.Order.ID)
	var shipAId string
	for _, s := range shipments {
		if s.Shipment.BusinessID.String == prodA.BusinessID {
			shipAId = s.Shipment.ID
		}
	}
	_, _ = env.MarketplaceService.DispatchShipment(ctx, sellerA, shipAId, service.DispatchOrderInput{CarrierName: "C", TrackingNumber: "T"})

	// Rogue buyer
	rogueBuyer := createRegisteredUser(t, env, fmt.Sprintf("rogue_buyer_%d", time.Now().UnixNano()), "member")

	// Rogue buyer attempts to confirm delivery of another buyer's shipment -> Must be rejected!
	_, err := env.MarketplaceService.ConfirmShipmentDelivery(ctx, rogueBuyer, shipAId, "", "Rogue confirmation")
	if err == nil {
		t.Fatal("Expected error when rogue buyer confirms another buyer's shipment, got nil")
	}

	// Genuine buyer confirms delivery -> Allowed!
	buyerUser, _ := env.Repo.Queries().GetUserByID(ctx, orderRes.Order.BuyerUserID.String)
	buyerUserDb := &db.User{ID: buyerUser.ID, PrimaryRole: buyerUser.PrimaryRole}
	_, err = env.MarketplaceService.ConfirmShipmentDelivery(ctx, buyerUserDb, shipAId, "", "Legitimate confirmation")
	if err != nil {
		t.Fatalf("Legitimate buyer confirm delivery failed: %v", err)
	}
}

// 15. Cancellation after partial dispatch
func TestMultiVendor_CancellationAfterPartialDispatch(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	admin, sellerA, _, prodA, _, orderRes := helperCreateMultiVendorOrder(t, env, ctx)
	_, _ = env.MarketplaceService.ConfirmPayment(ctx, orderRes.Payment.Reference)

	shipments, _ := env.MarketplaceService.ListOrderShipments(ctx, admin, orderRes.Order.ID)
	var shipAId string
	for _, s := range shipments {
		if s.Shipment.BusinessID.String == prodA.BusinessID {
			shipAId = s.Shipment.ID
		}
	}

	// Dispatch 1 shipment
	_, err := env.MarketplaceService.DispatchShipment(ctx, sellerA, shipAId, service.DispatchOrderInput{CarrierName: "C", TrackingNumber: "T"})
	if err != nil {
		t.Fatalf("DispatchShipment failed: %v", err)
	}

	// Buyer attempts to cancel order -> Must fail explicitly!
	buyerUser, _ := env.Repo.Queries().GetUserByID(ctx, orderRes.Order.BuyerUserID.String)
	buyerUserDb := &db.User{ID: buyerUser.ID, PrimaryRole: buyerUser.PrimaryRole}

	_, err = env.MarketplaceService.CancelOrder(ctx, buyerUserDb, orderRes.Order.ID, "Changed mind")
	if err == nil {
		t.Fatal("Expected cancellation after partial dispatch to be rejected, got nil")
	}
	if !strings.Contains(err.Error(), "already been dispatched") {
		t.Fatalf("Expected error message indicating dispatched shipments, got %v", err)
	}

	// Verify order status was NOT changed to cancelled
	ord, _, _, _ := env.MarketplaceService.GetOrderByID(ctx, admin, orderRes.Order.ID)
	if ord.OrderStatus == "cancelled" {
		t.Errorf("Order should NOT be cancelled after partial dispatch!")
	}
}

// 16. Refund and inventory ledger consistency
func TestMultiVendor_RefundAndInventoryLedgerConsistency(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	admin, _, _, prodA, prodB, orderRes := helperCreateMultiVendorOrder(t, env, ctx)
	_, _ = env.MarketplaceService.ConfirmPayment(ctx, orderRes.Payment.Reference)

	// Cancel order before any dispatch
	buyerUser, _ := env.Repo.Queries().GetUserByID(ctx, orderRes.Order.BuyerUserID.String)
	buyerUserDb := &db.User{ID: buyerUser.ID, PrimaryRole: buyerUser.PrimaryRole}

	_, err := env.MarketplaceService.CancelOrder(ctx, buyerUserDb, orderRes.Order.ID, "Customer request before dispatch")
	if err != nil {
		t.Fatalf("CancelOrder failed: %v", err)
	}

	// Verify all shipments marked cancelled
	shipments, err := env.MarketplaceService.ListOrderShipments(ctx, admin, orderRes.Order.ID)
	if err != nil {
		t.Fatalf("ListOrderShipments failed: %v", err)
	}
	for _, s := range shipments {
		if s.Shipment.Status != "cancelled" {
			t.Errorf("Expected shipment %s to be cancelled, got %s", s.Shipment.ID, s.Shipment.Status)
		}
	}

	// Verify inventory release logs exist exactly once per product
	countA, err := env.Repo.Queries().CountOrderInventoryReleaseLogs(ctx, db.CountOrderInventoryReleaseLogsParams{
		OrderID:   service.TextFromString(orderRes.Order.ID),
		ProductID: prodA.ID,
	})
	if err != nil || countA != 1 {
		t.Errorf("Expected exactly 1 inventory release log for Product A, got %d (err: %v)", countA, err)
	}

	countB, err := env.Repo.Queries().CountOrderInventoryReleaseLogs(ctx, db.CountOrderInventoryReleaseLogsParams{
		OrderID:   service.TextFromString(orderRes.Order.ID),
		ProductID: prodB.ID,
	})
	if err != nil || countB != 1 {
		t.Errorf("Expected exactly 1 inventory release log for Product B, got %d (err: %v)", countB, err)
	}
}

// 17. Historical shipment migration compatibility
func TestMultiVendor_HistoricalShipmentMigrationCompatibility(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	admin := createRegisteredUser(t, env, fmt.Sprintf("hist_adm_%d", time.Now().UnixNano()), "super_admin")

	// Create raw order and single shipment without items initially
	orderID := security.GenerateUUID()
	orderNum := fmt.Sprintf("HIST-%d", time.Now().UnixNano()%1000000)
	_, err := env.Pool.Exec(ctx, `
		INSERT INTO marketplace_orders (
			id, order_number, buyer_user_id, buyer_name, buyer_email, buyer_phone,
			delivery_type, shipping_address, subtotal_kobo, delivery_fee_kobo, total_kobo,
			order_status, payment_status, fulfilment_status, created_at, updated_at
		) VALUES (
			$1, $2, $3, 'Hist Buyer', 'hist@example.org', '+2348000000000',
			'delivery', 'Hist Address', 50000, 1000, 51000,
			'dispatched', 'paid', 'dispatched', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
		)
	`, orderID, orderNum, admin.ID)
	if err != nil {
		t.Fatalf("Failed to create historical order: %v", err)
	}

	// Insert historical single-vendor shipment
	shipmentID := security.GenerateUUID()
	_, err = env.Pool.Exec(ctx, `
		INSERT INTO marketplace_shipments (
			id, order_id, carrier_name, tracking_number, status, dispatched_at, created_at, updated_at
		) VALUES (
			$1, $2, 'Legacy Carrier', 'LEGACY-001', 'dispatched', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
		)
	`, shipmentID, orderID)
	if err != nil {
		t.Fatalf("Failed to create historical shipment: %v", err)
	}

	// Query historical shipments via service
	shipments, err := env.MarketplaceService.ListOrderShipments(ctx, admin, orderID)
	if err != nil {
		t.Fatalf("ListOrderShipments on historical order failed: %v", err)
	}
	if len(shipments) != 1 {
		t.Fatalf("Expected 1 historical shipment, got %d", len(shipments))
	}
	if shipments[0].Shipment.TrackingNumber != "LEGACY-001" {
		t.Errorf("Expected LEGACY-001, got %s", shipments[0].Shipment.TrackingNumber)
	}
}

// 18. Shipment creation and dispatch transaction rollback
func TestMultiVendor_ShipmentCreationAndDispatchTransactionRollback(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	admin, sellerA, _, prodA, _, orderRes := helperCreateMultiVendorOrder(t, env, ctx)
	_, _ = env.MarketplaceService.ConfirmPayment(ctx, orderRes.Payment.Reference)

	shipments, _ := env.MarketplaceService.ListOrderShipments(ctx, admin, orderRes.Order.ID)
	var shipAId string
	for _, s := range shipments {
		if s.Shipment.BusinessID.String == prodA.BusinessID {
			shipAId = s.Shipment.ID
		}
	}

	// Unauthorized seller dispatch triggers rollback: shipment must remain 'pending'
	rogueSeller := createRegisteredUser(t, env, fmt.Sprintf("rogue_sel_%d", time.Now().UnixNano()), "member")
	_, err := env.MarketplaceService.DispatchShipment(ctx, rogueSeller, shipAId, service.DispatchOrderInput{
		CarrierName:    "Rogue",
		TrackingNumber: "TRK-FAIL",
	})
	if err == nil {
		t.Fatal("Expected unauthorized dispatch to fail, got nil")
	}

	// Verify shipment status was NOT committed as dispatched
	shDetail, err := env.MarketplaceService.GetShipmentByID(ctx, sellerA, shipAId)
	if err != nil {
		t.Fatalf("GetShipmentByID failed: %v", err)
	}
	if shDetail.Shipment.Status != "pending" {
		t.Errorf("Expected shipment status to remain 'pending' after rollback, got '%s'", shDetail.Shipment.Status)
	}

	// Verify order status remained 'processing'
	ord, _, _, _ := env.MarketplaceService.GetOrderByID(ctx, admin, orderRes.Order.ID)
	if ord.OrderStatus != "processing" {
		t.Errorf("Expected order status 'processing', got '%s'", ord.OrderStatus)
	}
}
