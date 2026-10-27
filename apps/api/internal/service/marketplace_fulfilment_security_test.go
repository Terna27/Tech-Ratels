package service_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"ratels-api/internal/db"
	"ratels-api/internal/security"
	"ratels-api/internal/service"
)

// Helper to create a business and product for a given user
func helperCreateSellerWithProduct(t *testing.T, env *TestEnv, ctx context.Context, username, bizName, prodName string, stock int32, adminUser *db.User) (*db.User, *db.MarketplaceBusiness, *db.MarketplaceProduct) {
	t.Helper()
	seller := createRegisteredUser(t, env, username, "member")
	biz, err := env.MarketplaceService.RegisterBusiness(ctx, seller, service.RegisterBusinessInput{
		Name:          bizName,
		Category:      "tools_hardware",
		OwnershipType: "member_owned",
		ContactEmail:  username + "@example.org",
		ContactPhone:  "+2348000000001",
	})
	if err != nil {
		t.Fatalf("Failed to register business %s: %v", bizName, err)
	}
	_, err = env.MarketplaceService.ApproveBusiness(ctx, adminUser, biz.ID, true, "")
	if err != nil {
		t.Fatalf("Failed to approve business %s: %v", bizName, err)
	}

	prod, err := env.MarketplaceService.CreateProduct(ctx, seller, service.CreateProductInput{
		BusinessID:    biz.ID,
		Title:         prodName,
		Category:      "safety_gear",
		SKU:           fmt.Sprintf("SKU-%s-%d", username, time.Now().UnixNano()),
		PriceKobo:     250000,
		StockQuantity: stock,
	})
	if err != nil {
		t.Fatalf("Failed to create product %s: %v", prodName, err)
	}

	return seller, biz, prod
}

// 1. Pending-payment dispatch rejection
func TestFulfilmentSecurity_PendingPaymentDispatchRejection(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	admin := createRegisteredUser(t, env, "sec_admin_1", "super_admin")
	seller, _, prod := helperCreateSellerWithProduct(t, env, ctx, "sec_seller_1", "Seller Store 1", "Work Gloves", 10, admin)
	buyer := createRegisteredUser(t, env, "sec_buyer_1", "member")

	orderRes, err := env.MarketplaceService.CreateOrder(ctx, buyer, service.CheckoutInput{
		Items:        []service.CartItemInput{{ProductID: prod.ID, Quantity: 2}},
		BuyerName:    "Pending Buyer",
		BuyerEmail:   "pending@example.org",
		BuyerPhone:   "+2348011112222",
		DeliveryType: "delivery",
		ShippingAddress: "123 Test St",
	})
	if err != nil {
		t.Fatalf("CreateOrder failed: %v", err)
	}

	// Attempt to dispatch while payment is pending
	_, err = env.MarketplaceService.DispatchOrder(ctx, seller, orderRes.Order.ID, service.DispatchOrderInput{
		CarrierName:    "GIG Logistics",
		TrackingNumber: "TRK-PENDING-001",
	})
	if err == nil {
		t.Fatal("Expected error when dispatching pending payment order, got nil")
	}
	if !errors.Is(err, service.ErrOrderNotPaid) && !strings.Contains(err.Error(), "unpaid") {
		t.Fatalf("Expected ErrOrderNotPaid, got %v", err)
	}

	// Verify order status unchanged
	order, _, _, err := env.MarketplaceService.GetOrderByID(ctx, admin, orderRes.Order.ID)
	if err != nil {
		t.Fatalf("GetOrderByID failed: %v", err)
	}
	if order.FulfilmentStatus != "unfulfilled" {
		t.Errorf("Expected fulfilment status 'unfulfilled', got '%s'", order.FulfilmentStatus)
	}
}

// 2. Failed-payment dispatch rejection
func TestFulfilmentSecurity_FailedPaymentDispatchRejection(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	admin := createRegisteredUser(t, env, "sec_admin_2", "super_admin")
	seller, _, prod := helperCreateSellerWithProduct(t, env, ctx, "sec_seller_2", "Seller Store 2", "Reflective Vest", 10, admin)
	buyer := createRegisteredUser(t, env, "sec_buyer_2", "member")

	orderRes, err := env.MarketplaceService.CreateOrder(ctx, buyer, service.CheckoutInput{
		Items:        []service.CartItemInput{{ProductID: prod.ID, Quantity: 1}},
		BuyerName:    "Failed Buyer",
		BuyerEmail:   "failed@example.org",
		BuyerPhone:   "+2348011112222",
		DeliveryType: "delivery",
		ShippingAddress: "456 Test Ave",
	})
	if err != nil {
		t.Fatalf("CreateOrder failed: %v", err)
	}

	// Mark payment as failed in database
	_, _ = env.Repo.Queries().UpdateMarketplaceOrderPaymentStatus(ctx, db.UpdateMarketplaceOrderPaymentStatusParams{
		ID:            orderRes.Order.ID,
		PaymentStatus: "failed",
	})

	// Attempt dispatch
	_, err = env.MarketplaceService.DispatchOrder(ctx, seller, orderRes.Order.ID, service.DispatchOrderInput{
		CarrierName:    "DHL",
		TrackingNumber: "TRK-FAILED-001",
	})
	if err == nil {
		t.Fatal("Expected error when dispatching failed payment order, got nil")
	}
	if !errors.Is(err, service.ErrOrderNotPaid) && !strings.Contains(err.Error(), "unpaid") {
		t.Fatalf("Expected ErrOrderNotPaid, got %v", err)
	}
}

// 3. Successful paid-order dispatch
func TestFulfilmentSecurity_SuccessfulPaidOrderDispatch(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	admin := createRegisteredUser(t, env, "sec_admin_3", "super_admin")
	seller, _, prod := helperCreateSellerWithProduct(t, env, ctx, "sec_seller_3", "Seller Store 3", "Safety Helmet", 10, admin)
	buyer := createRegisteredUser(t, env, "sec_buyer_3", "member")

	orderRes, err := env.MarketplaceService.CreateOrder(ctx, buyer, service.CheckoutInput{
		Items:        []service.CartItemInput{{ProductID: prod.ID, Quantity: 1}},
		BuyerName:    "Paid Buyer",
		BuyerEmail:   "paid@example.org",
		BuyerPhone:   "+2348011112222",
		DeliveryType: "delivery",
		ShippingAddress: "789 Test Rd",
	})
	if err != nil {
		t.Fatalf("CreateOrder failed: %v", err)
	}

	// Confirm payment
	_, err = env.MarketplaceService.ConfirmPayment(ctx, orderRes.Payment.Reference)
	if err != nil {
		t.Fatalf("ConfirmPayment failed: %v", err)
	}

	// Dispatch order
	sh, err := env.MarketplaceService.DispatchOrder(ctx, seller, orderRes.Order.ID, service.DispatchOrderInput{
		CarrierName:    "Fez Delivery",
		TrackingNumber: "FEZ-12345",
		TrackingURL:    "https://track.fez.delivery/FEZ-12345",
	})
	if err != nil {
		t.Fatalf("DispatchOrder failed: %v", err)
	}
	if sh == nil || sh.Status != "dispatched" {
		t.Fatalf("Expected shipment status 'dispatched', got %v", sh)
	}

	// Verify order status
	order, _, shipment, err := env.MarketplaceService.GetOrderByID(ctx, admin, orderRes.Order.ID)
	if err != nil {
		t.Fatalf("GetOrderByID failed: %v", err)
	}
	if order.FulfilmentStatus != "dispatched" || order.OrderStatus != "dispatched" {
		t.Errorf("Expected fulfilment & order status 'dispatched', got fulfilment=%s, order=%s", order.FulfilmentStatus, order.OrderStatus)
	}
	if shipment == nil || shipment.TrackingNumber != "FEZ-12345" {
		t.Errorf("Expected shipment tracking FEZ-12345, got %v", shipment)
	}
}

// 4. Duplicate dispatch
func TestFulfilmentSecurity_DuplicateDispatchRejection(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	admin := createRegisteredUser(t, env, "sec_admin_4", "super_admin")
	seller, _, prod := helperCreateSellerWithProduct(t, env, ctx, "sec_seller_4", "Seller Store 4", "Steel Boots", 10, admin)
	buyer := createRegisteredUser(t, env, "sec_buyer_4", "member")

	orderRes, err := env.MarketplaceService.CreateOrder(ctx, buyer, service.CheckoutInput{
		Items:        []service.CartItemInput{{ProductID: prod.ID, Quantity: 1}},
		BuyerName:    "Dup Buyer",
		BuyerEmail:   "dup@example.org",
		BuyerPhone:   "+2348011112222",
		DeliveryType: "delivery",
		ShippingAddress: "321 Test Blvd",
	})
	if err != nil {
		t.Fatalf("CreateOrder failed: %v", err)
	}

	_, err = env.MarketplaceService.ConfirmPayment(ctx, orderRes.Payment.Reference)
	if err != nil {
		t.Fatalf("ConfirmPayment failed: %v", err)
	}

	// First dispatch succeeds
	_, err = env.MarketplaceService.DispatchOrder(ctx, seller, orderRes.Order.ID, service.DispatchOrderInput{
		CarrierName:    "GIG Logistics",
		TrackingNumber: "GIG-ORIGINAL",
	})
	if err != nil {
		t.Fatalf("First dispatch failed: %v", err)
	}

	// Second dispatch must fail
	_, err = env.MarketplaceService.DispatchOrder(ctx, seller, orderRes.Order.ID, service.DispatchOrderInput{
		CarrierName:    "GIG Logistics",
		TrackingNumber: "GIG-DUPLICATE",
	})
	if err == nil {
		t.Fatal("Expected error on duplicate dispatch, got nil")
	}
	if !errors.Is(err, service.ErrOrderAlreadyDispatched) && !strings.Contains(err.Error(), "already been dispatched") {
		t.Fatalf("Expected ErrOrderAlreadyDispatched, got %v", err)
	}
}

// 5. Concurrent dispatch and cancellation
func TestFulfilmentSecurity_ConcurrentDispatchAndCancellation(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	admin := createRegisteredUser(t, env, "sec_admin_5", "super_admin")
	seller, _, prod := helperCreateSellerWithProduct(t, env, ctx, "sec_seller_5", "Seller Store 5", "Ear Protectors", 10, admin)
	buyer := createRegisteredUser(t, env, "sec_buyer_5", "member")

	orderRes, err := env.MarketplaceService.CreateOrder(ctx, buyer, service.CheckoutInput{
		Items:        []service.CartItemInput{{ProductID: prod.ID, Quantity: 2}},
		BuyerName:    "Race Buyer",
		BuyerEmail:   "race@example.org",
		BuyerPhone:   "+2348011112222",
		DeliveryType: "delivery",
		ShippingAddress: "555 Race St",
	})
	if err != nil {
		t.Fatalf("CreateOrder failed: %v", err)
	}

	_, err = env.MarketplaceService.ConfirmPayment(ctx, orderRes.Payment.Reference)
	if err != nil {
		t.Fatalf("ConfirmPayment failed: %v", err)
	}

	var wg sync.WaitGroup
	var dispatchErr, cancelErr error

	wg.Add(2)
	go func() {
		defer wg.Done()
		_, dispatchErr = env.MarketplaceService.DispatchOrder(ctx, seller, orderRes.Order.ID, service.DispatchOrderInput{
			CarrierName:    "DHL Express",
			TrackingNumber: "RACE-TRK-001",
		})
	}()

	go func() {
		defer wg.Done()
		_, cancelErr = env.MarketplaceService.CancelOrder(ctx, buyer, orderRes.Order.ID, "Customer cancellation race")
	}()

	wg.Wait()

	// Invariant: One operation MUST succeed and the other MUST fail safely.
	// Both cannot succeed simultaneously!
	dispatchedSuccess := dispatchErr == nil
	cancelledSuccess := cancelErr == nil

	if dispatchedSuccess && cancelledSuccess {
		t.Fatal("CRITICAL INCONSISTENCY: Both dispatch and cancellation succeeded concurrently!")
	}
	if !dispatchedSuccess && !cancelledSuccess {
		t.Fatalf("Unexpected: Neither operation succeeded: dispatchErr=%v, cancelErr=%v", dispatchErr, cancelErr)
	}

	// Verify terminal state
	finalOrder, _, finalShipment, err := env.MarketplaceService.GetOrderByID(ctx, admin, orderRes.Order.ID)
	if err != nil {
		t.Fatalf("GetOrderByID failed: %v", err)
	}

	if dispatchedSuccess {
		if finalOrder.FulfilmentStatus != "dispatched" || finalOrder.OrderStatus != "dispatched" {
			t.Errorf("Expected dispatched state, got fulfilment=%s, order=%s", finalOrder.FulfilmentStatus, finalOrder.OrderStatus)
		}
		if finalShipment == nil {
			t.Error("Expected shipment record to exist")
		}
	} else {
		if finalOrder.OrderStatus != "cancelled" {
			t.Errorf("Expected cancelled state, got %s", finalOrder.OrderStatus)
		}
		if finalShipment != nil {
			t.Errorf("Shipment should not exist on cancelled order: %v", finalShipment)
		}
	}
}

// 6. Cross-seller access denial
func TestFulfilmentSecurity_CrossSellerAccessDenial(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	admin := createRegisteredUser(t, env, "sec_admin_6", "super_admin")
	seller1, _, prod1 := helperCreateSellerWithProduct(t, env, ctx, "sec_seller_6a", "Store A", "Product A", 10, admin)
	seller2, _, prod2 := helperCreateSellerWithProduct(t, env, ctx, "sec_seller_6b", "Store B", "Product B", 10, admin)
	buyer := createRegisteredUser(t, env, "sec_buyer_6", "member")

	// Order with only Product A (belonging to Seller 1)
	order1Res, err := env.MarketplaceService.CreateOrder(ctx, buyer, service.CheckoutInput{
		Items:        []service.CartItemInput{{ProductID: prod1.ID, Quantity: 1}},
		BuyerName:    "Buyer 6",
		BuyerEmail:   "buyer6@example.org",
		BuyerPhone:   "+2348011112222",
		DeliveryType: "delivery",
		ShippingAddress: "111 Seller St",
	})
	if err != nil {
		t.Fatalf("CreateOrder failed: %v", err)
	}
	_, _ = env.MarketplaceService.ConfirmPayment(ctx, order1Res.Payment.Reference)

	// Seller 2 attempts to dispatch Seller 1's order -> MUST FAIL
	_, err = env.MarketplaceService.DispatchOrder(ctx, seller2, order1Res.Order.ID, service.DispatchOrderInput{
		CarrierName:    "Courier",
		TrackingNumber: "TRK-ROGUE",
	})
	if err == nil {
		t.Fatal("Expected error when unauthorized seller dispatches another seller's order, got nil")
	}
	if !errors.Is(err, service.ErrUnauthorizedSellerAction) && !strings.Contains(err.Error(), "unauthorized") {
		t.Fatalf("Expected ErrUnauthorizedSellerAction, got %v", err)
	}

	// Multi-vendor cart: order with Product A AND Product B
	multiOrderRes, err := env.MarketplaceService.CreateOrder(ctx, buyer, service.CheckoutInput{
		Items: []service.CartItemInput{
			{ProductID: prod1.ID, Quantity: 1},
			{ProductID: prod2.ID, Quantity: 1},
		},
		BuyerName:    "Buyer Multi",
		BuyerEmail:   "multi@example.org",
		BuyerPhone:   "+2348011112222",
		DeliveryType: "delivery",
		ShippingAddress: "222 Multi St",
	})
	if err != nil {
		t.Fatalf("CreateOrder (multi) failed: %v", err)
	}
	_, _ = env.MarketplaceService.ConfirmPayment(ctx, multiOrderRes.Payment.Reference)

	// Seller 1 attempts to dispatch the multi-seller order:
	// Seller 1 does NOT own Product B, so Seller 1 must not be able to dispatch another seller's products
	_, err = env.MarketplaceService.DispatchOrder(ctx, seller1, multiOrderRes.Order.ID, service.DispatchOrderInput{
		CarrierName:    "Courier",
		TrackingNumber: "TRK-MULTI-A",
	})
	if err == nil {
		t.Fatal("Expected error when seller attempts to dispatch multi-vendor order containing other sellers' products, got nil")
	}
	if !errors.Is(err, service.ErrUnauthorizedSellerAction) && !strings.Contains(err.Error(), "unauthorized") {
		t.Fatalf("Expected ErrUnauthorizedSellerAction, got %v", err)
	}
}

// 7. State coordinator cross-state access denial
func TestFulfilmentSecurity_StateCoordinatorCrossStateAccessDenial(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	admin := createRegisteredUser(t, env, "sec_admin_7", "super_admin")
	seller, _, prod := helperCreateSellerWithProduct(t, env, ctx, "sec_seller_7", "Seller Store 7", "Broom Set", 20, admin)
	_ = seller
	buyer := createRegisteredUser(t, env, "sec_buyer_7", "member")

	states, err := env.Repo.Queries().ListStates(ctx)
	if err != nil || len(states) < 2 {
		t.Fatalf("Need at least 2 states, got %d, err=%v", len(states), err)
	}
	lagosState := states[0]
	kanoState := states[1]

	// Create Lagos State Coordinator with official appointment
	lagosCoord := createRegisteredUser(t, env, "lagos_coord_user", "state_coordinator")
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

	// 1. Create order shipped to Kano State
	kanoOrderRes, err := env.MarketplaceService.CreateOrder(ctx, buyer, service.CheckoutInput{
		Items:           []service.CartItemInput{{ProductID: prod.ID, Quantity: 1}},
		BuyerName:       "Kano Recipient",
		BuyerEmail:      "kano@example.org",
		BuyerPhone:      "+2348011112222",
		DeliveryType:    "delivery",
		ShippingAddress: "10 Bompai Road, Kano",
		ShippingStateID: &kanoState.ID,
	})
	if err != nil {
		t.Fatalf("CreateOrder Kano failed: %v", err)
	}
	_, _ = env.MarketplaceService.ConfirmPayment(ctx, kanoOrderRes.Payment.Reference)

	// 2. Create order shipped to Lagos State
	lagosOrderRes, err := env.MarketplaceService.CreateOrder(ctx, buyer, service.CheckoutInput{
		Items:           []service.CartItemInput{{ProductID: prod.ID, Quantity: 1}},
		BuyerName:       "Lagos Recipient",
		BuyerEmail:      "lagos@example.org",
		BuyerPhone:      "+2348011112222",
		DeliveryType:    "delivery",
		ShippingAddress: "25 Marina, Lagos Island",
		ShippingStateID: &lagosState.ID,
	})
	if err != nil {
		t.Fatalf("CreateOrder Lagos failed: %v", err)
	}
	_, _ = env.MarketplaceService.ConfirmPayment(ctx, lagosOrderRes.Payment.Reference)

	// Lagos coordinator inspecting Kano order -> ErrScopeUnauthorized
	_, _, _, err = env.MarketplaceService.GetOrderByID(ctx, lagosCoord, kanoOrderRes.Order.ID)
	if !errors.Is(err, service.ErrScopeUnauthorized) {
		t.Fatalf("Expected ErrScopeUnauthorized for Lagos coordinator inspecting Kano order, got %v", err)
	}

	// Lagos coordinator cancelling Kano order -> ErrScopeUnauthorized
	_, err = env.MarketplaceService.CancelOrder(ctx, lagosCoord, kanoOrderRes.Order.ID, "Unauthorized cross-state cancellation")
	if !errors.Is(err, service.ErrScopeUnauthorized) {
		t.Fatalf("Expected ErrScopeUnauthorized for Lagos coordinator cancelling Kano order, got %v", err)
	}

	// Lagos coordinator dispatching Kano order -> ErrScopeUnauthorized
	_, err = env.MarketplaceService.DispatchOrder(ctx, lagosCoord, kanoOrderRes.Order.ID, service.DispatchOrderInput{
		CarrierName:    "Rogue Delivery",
		TrackingNumber: "ROGUE-TRK",
	})
	if !errors.Is(err, service.ErrScopeUnauthorized) {
		t.Fatalf("Expected ErrScopeUnauthorized for Lagos coordinator dispatching Kano order, got %v", err)
	}

	// Lagos coordinator listing all orders -> must include Lagos order, exclude Kano order
	orders, err := env.MarketplaceService.ListAllOrders(ctx, lagosCoord, nil, nil, nil)
	if err != nil {
		t.Fatalf("ListAllOrders failed: %v", err)
	}
	foundLagos := false
	for _, o := range orders {
		if o.ID == kanoOrderRes.Order.ID {
			t.Errorf("Lagos coordinator must NOT see Kano order %s in administrative list", kanoOrderRes.Order.ID)
		}
		if o.ID == lagosOrderRes.Order.ID {
			foundLagos = true
		}
	}
	if !foundLagos {
		t.Errorf("Lagos coordinator should see Lagos order %s in list", lagosOrderRes.Order.ID)
	}
}

// 8. Zonal coordinator cross-zone access denial
func TestFulfilmentSecurity_ZonalCoordinatorCrossZoneAccessDenial(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	admin := createRegisteredUser(t, env, "sec_admin_8", "super_admin")
	seller, _, prod := helperCreateSellerWithProduct(t, env, ctx, "sec_seller_8", "Seller Store 8", "Safety Boots", 20, admin)
	_ = seller
	buyer := createRegisteredUser(t, env, "sec_buyer_8", "member")

	states, err := env.Repo.Queries().ListStates(ctx)
	if err != nil || len(states) < 2 {
		t.Fatalf("Need states, got %v", err)
	}

	// Find two states in different zones
	stateZoneA := states[0]
	var stateZoneB db.State
	foundDiffZone := false
	for _, st := range states[1:] {
		if st.ZoneID != stateZoneA.ZoneID {
			stateZoneB = st
			foundDiffZone = true
			break
		}
	}
	if !foundDiffZone {
		t.Fatal("Could not find two states in different geopolitical zones")
	}

	// Appoint Zonal Coordinator for Zone A
	zonalCoord := createRegisteredUser(t, env, "zonal_coord_user", "zonal_coordinator")
	_, err = env.Repo.Queries().CreateAppointment(ctx, db.CreateAppointmentParams{
		ID:               security.GenerateUUID(),
		UserID:           zonalCoord.ID,
		RoleTitle:        "Zonal Coordinator (Zone A)",
		RoleType:         "zonal_coordinator",
		JurisdictionType: "zone",
		JurisdictionID:   pgtype.Text{String: stateZoneA.ZoneID, Valid: true},
		StartDate:        pgtype.Timestamptz{Time: time.Now().Add(-1 * time.Hour), Valid: true},
		Status:           "active",
	})
	if err != nil {
		t.Fatalf("Failed to create Zonal appointment: %v", err)
	}

	// Order in Zone B (outside coordinator's zone)
	orderZoneB, err := env.MarketplaceService.CreateOrder(ctx, buyer, service.CheckoutInput{
		Items:           []service.CartItemInput{{ProductID: prod.ID, Quantity: 1}},
		BuyerName:       "Zone B Recipient",
		BuyerEmail:      "zoneb@example.org",
		BuyerPhone:      "+2348011112222",
		DeliveryType:    "delivery",
		ShippingAddress: "Zone B Address",
		ShippingStateID: &stateZoneB.ID,
	})
	if err != nil {
		t.Fatalf("CreateOrder Zone B failed: %v", err)
	}
	_, _ = env.MarketplaceService.ConfirmPayment(ctx, orderZoneB.Payment.Reference)

	// Order in Zone A (inside coordinator's zone)
	orderZoneA, err := env.MarketplaceService.CreateOrder(ctx, buyer, service.CheckoutInput{
		Items:           []service.CartItemInput{{ProductID: prod.ID, Quantity: 1}},
		BuyerName:       "Zone A Recipient",
		BuyerEmail:      "zonea@example.org",
		BuyerPhone:      "+2348011112222",
		DeliveryType:    "delivery",
		ShippingAddress: "Zone A Address",
		ShippingStateID: &stateZoneA.ID,
	})
	if err != nil {
		t.Fatalf("CreateOrder Zone A failed: %v", err)
	}
	_, _ = env.MarketplaceService.ConfirmPayment(ctx, orderZoneA.Payment.Reference)

	// Zonal coordinator accessing Zone B order -> ErrScopeUnauthorized
	_, _, _, err = env.MarketplaceService.GetOrderByID(ctx, zonalCoord, orderZoneB.Order.ID)
	if !errors.Is(err, service.ErrScopeUnauthorized) {
		t.Fatalf("Expected ErrScopeUnauthorized for zonal coordinator inspecting Zone B order, got %v", err)
	}

	// Zonal coordinator cancelling Zone B order -> ErrScopeUnauthorized
	_, err = env.MarketplaceService.CancelOrder(ctx, zonalCoord, orderZoneB.Order.ID, "Cross-zone cancel")
	if !errors.Is(err, service.ErrScopeUnauthorized) {
		t.Fatalf("Expected ErrScopeUnauthorized for zonal coordinator cancelling Zone B order, got %v", err)
	}

	// Zonal coordinator listing all orders -> must include Zone A, exclude Zone B
	orders, err := env.MarketplaceService.ListAllOrders(ctx, zonalCoord, nil, nil, nil)
	if err != nil {
		t.Fatalf("ListAllOrders failed: %v", err)
	}
	foundZoneA := false
	for _, o := range orders {
		if o.ID == orderZoneB.Order.ID {
			t.Errorf("Zonal coordinator must NOT see Zone B order %s", orderZoneB.Order.ID)
		}
		if o.ID == orderZoneA.Order.ID {
			foundZoneA = true
		}
	}
	if !foundZoneA {
		t.Errorf("Zonal coordinator should see Zone A order %s", orderZoneA.Order.ID)
	}
}

// 9. Super Admin access
func TestFulfilmentSecurity_SuperAdminNationwideAccess(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	superAdmin := createRegisteredUser(t, env, "sec_super_admin_9", "super_admin")
	seller, _, prod := helperCreateSellerWithProduct(t, env, ctx, "sec_seller_9", "Seller Store 9", "Hard Hat", 20, superAdmin)
	_ = seller
	buyer := createRegisteredUser(t, env, "sec_buyer_9", "member")

	states, err := env.Repo.Queries().ListStates(ctx)
	if err != nil || len(states) < 2 {
		t.Fatalf("Need states, got %v", err)
	}

	order1, err := env.MarketplaceService.CreateOrder(ctx, buyer, service.CheckoutInput{
		Items:           []service.CartItemInput{{ProductID: prod.ID, Quantity: 1}},
		BuyerName:       "Buyer State 1",
		BuyerEmail:      "b1@example.org",
		BuyerPhone:      "+2348011112222",
		DeliveryType:    "delivery",
		ShippingAddress: "State 1 Addr",
		ShippingStateID: &states[0].ID,
	})
	if err != nil {
		t.Fatalf("CreateOrder 1 failed: %v", err)
	}
	_, _ = env.MarketplaceService.ConfirmPayment(ctx, order1.Payment.Reference)

	order2, err := env.MarketplaceService.CreateOrder(ctx, buyer, service.CheckoutInput{
		Items:           []service.CartItemInput{{ProductID: prod.ID, Quantity: 1}},
		BuyerName:       "Buyer State 2",
		BuyerEmail:      "b2@example.org",
		BuyerPhone:      "+2348011112222",
		DeliveryType:    "delivery",
		ShippingAddress: "State 2 Addr",
		ShippingStateID: &states[1].ID,
	})
	if err != nil {
		t.Fatalf("CreateOrder 2 failed: %v", err)
	}
	_, _ = env.MarketplaceService.ConfirmPayment(ctx, order2.Payment.Reference)

	// Super Admin can list orders nationwide
	allOrders, err := env.MarketplaceService.ListAllOrders(ctx, superAdmin, nil, nil, nil)
	if err != nil {
		t.Fatalf("ListAllOrders failed: %v", err)
	}
	found1, found2 := false, false
	for _, o := range allOrders {
		if o.ID == order1.Order.ID {
			found1 = true
		}
		if o.ID == order2.Order.ID {
			found2 = true
		}
	}
	if !found1 || !found2 {
		t.Errorf("Super Admin must see all orders nationwide: found1=%v, found2=%v", found1, found2)
	}

	// Super Admin can dispatch order1
	sh, err := env.MarketplaceService.DispatchOrder(ctx, superAdmin, order1.Order.ID, service.DispatchOrderInput{
		CarrierName:    "National Carrier",
		TrackingNumber: "NAT-TRK-001",
	})
	if err != nil {
		t.Fatalf("Super Admin DispatchOrder failed: %v", err)
	}
	if sh == nil || sh.Status != "dispatched" {
		t.Errorf("Expected dispatched status, got %v", sh)
	}

	// Super Admin can cancel order2
	cancelled, err := env.MarketplaceService.CancelOrder(ctx, superAdmin, order2.Order.ID, "Super admin cancellation")
	if err != nil {
		t.Fatalf("Super Admin CancelOrder failed: %v", err)
	}
	if cancelled.OrderStatus != "cancelled" {
		t.Errorf("Expected cancelled order status, got %s", cancelled.OrderStatus)
	}
}

// 10. Seller-facing customer data minimization
func TestFulfilmentSecurity_SellerFacingCustomerDataMinimization(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	admin := createRegisteredUser(t, env, "sec_admin_10", "super_admin")
	seller1, biz1, prod1 := helperCreateSellerWithProduct(t, env, ctx, "sec_seller_10a", "Store 10A", "Mask A", 10, admin)
	_, _, prod2 := helperCreateSellerWithProduct(t, env, ctx, "sec_seller_10b", "Store 10B", "Mask B", 10, admin)
	buyer := createRegisteredUser(t, env, "sec_buyer_10", "member")

	// Order with Pickup delivery
	pickupOrderRes, err := env.MarketplaceService.CreateOrder(ctx, buyer, service.CheckoutInput{
		Items:        []service.CartItemInput{{ProductID: prod1.ID, Quantity: 1}},
		BuyerName:    "Alice Sensitive",
		BuyerEmail:   "alice.sensitive@example.com",
		BuyerPhone:   "+2348099887766",
		DeliveryType: "pickup",
	})
	if err != nil {
		t.Fatalf("CreateOrder pickup failed: %v", err)
	}

	// Order with Multi-seller items
	multiOrderRes, err := env.MarketplaceService.CreateOrder(ctx, buyer, service.CheckoutInput{
		Items: []service.CartItemInput{
			{ProductID: prod1.ID, Quantity: 1},
			{ProductID: prod2.ID, Quantity: 1},
		},
		BuyerName:       "Bob Confidential",
		BuyerEmail:      "bob.confidential@example.com",
		BuyerPhone:      "+2348077665544",
		DeliveryType:    "delivery",
		ShippingAddress: "Secret Home Villa 42, Maitama",
	})
	if err != nil {
		t.Fatalf("CreateOrder multi failed: %v", err)
	}

	// 1. Seller 1 fetches multi-order -> Email must be redacted, other seller's item must be excluded
	sellerOrderView, sellerItems, _, err := env.MarketplaceService.GetOrderByID(ctx, seller1, multiOrderRes.Order.ID)
	if err != nil {
		t.Fatalf("GetOrderByID for seller failed: %v", err)
	}
	if sellerOrderView.BuyerEmail != "[REDACTED]" {
		t.Errorf("Expected BuyerEmail to be [REDACTED] for seller, got: %s", sellerOrderView.BuyerEmail)
	}
	if sellerOrderView.BuyerUserID.Valid {
		t.Errorf("Expected BuyerUserID to be invalid/empty for seller, got: %v", sellerOrderView.BuyerUserID)
	}
	if len(sellerItems) != 1 || (sellerItems[0].ProductID.Valid && sellerItems[0].ProductID.String != prod1.ID) {
		t.Errorf("Seller must only see their own items (expected 1 item for prod1, got %d)", len(sellerItems))
	}

	// 2. Seller 1 fetches pickup order -> Shipping address must be blanked
	pickupSellerView, _, _, err := env.MarketplaceService.GetOrderByID(ctx, seller1, pickupOrderRes.Order.ID)
	if err != nil {
		t.Fatalf("GetOrderByID for pickup seller failed: %v", err)
	}
	if pickupSellerView.ShippingAddress != "" {
		t.Errorf("Expected empty ShippingAddress for pickup order seller view, got: %s", pickupSellerView.ShippingAddress)
	}

	// 3. Seller 1 lists business orders -> All rows must have redacted email and masked details
	bizOrders, err := env.MarketplaceService.ListOrdersByBusiness(ctx, seller1, biz1.ID, nil)
	if err != nil {
		t.Fatalf("ListOrdersByBusiness failed: %v", err)
	}
	if len(bizOrders) == 0 {
		t.Fatal("Expected at least one business order")
	}
	for _, bo := range bizOrders {
		if bo.BuyerEmail != "[REDACTED]" {
			t.Errorf("Expected business order list BuyerEmail to be [REDACTED], got: %s", bo.BuyerEmail)
		}
		if bo.BuyerUserID.Valid {
			t.Errorf("Expected BuyerUserID to be invalid/empty, got: %v", bo.BuyerUserID)
		}
		if bo.DeliveryType == "pickup" && bo.ShippingAddress != "" {
			t.Errorf("Expected empty ShippingAddress for pickup business order, got: %s", bo.ShippingAddress)
		}
	}

	// 4. Buyer tracking MUST NOT be broken -> Buyer fetches their own order and sees their full data
	buyerView, buyerItems, _, err := env.MarketplaceService.GetOrderByID(ctx, buyer, multiOrderRes.Order.ID)
	if err != nil {
		t.Fatalf("GetOrderByID for buyer failed: %v", err)
	}
	if buyerView.BuyerEmail != "bob.confidential@example.com" {
		t.Errorf("Buyer must see their own actual email, got: %s", buyerView.BuyerEmail)
	}
	if buyerView.ShippingAddress != "Secret Home Villa 42, Maitama" {
		t.Errorf("Buyer must see their full shipping address, got: %s", buyerView.ShippingAddress)
	}
	if len(buyerItems) != 2 {
		t.Errorf("Buyer must see all their cart items (expected 2, got %d)", len(buyerItems))
	}
}

// 11. Dispatch blocked for reconciliation orders
func TestFulfilmentSecurity_RequiresReconciliationDispatchRejection(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	admin := createRegisteredUser(t, env, "sec_admin_rec_1", "super_admin")
	seller, _, prod := helperCreateSellerWithProduct(t, env, ctx, "sec_seller_rec_1", "Seller Store Rec 1", "Safety Boots", 10, admin)
	buyer := createRegisteredUser(t, env, "sec_buyer_rec_1", "member")

	orderRes, err := env.MarketplaceService.CreateOrder(ctx, buyer, service.CheckoutInput{
		Items:           []service.CartItemInput{{ProductID: prod.ID, Quantity: 2}},
		BuyerName:       "Reconciliation Buyer",
		BuyerEmail:      "rec_buyer@example.org",
		BuyerPhone:      "+2348011112222",
		DeliveryType:    "delivery",
		ShippingAddress: "123 Reconciliation St",
	})
	if err != nil {
		t.Fatalf("CreateOrder failed: %v", err)
	}

	// Order is marked as paid but requiring reconciliation (e.g. late payment after expiry)
	_, err = env.Pool.Exec(ctx, `
		UPDATE marketplace_orders
		SET payment_status = 'paid',
		    order_status = 'requires_reconciliation',
		    reconciliation_required = true,
		    stock_released = true,
		    reconciliation_notes = 'Test late payment reconciliation'
		WHERE id = $1`, orderRes.Order.ID)
	if err != nil {
		t.Fatalf("Failed to update order to requires_reconciliation: %v", err)
	}

	// Attempt to dispatch must fail with ErrOrderReconciliationRequired
	_, err = env.MarketplaceService.DispatchOrder(ctx, seller, orderRes.Order.ID, service.DispatchOrderInput{
		CarrierName:    "DHL",
		TrackingNumber: "DHL-REC-001",
	})
	if err == nil {
		t.Fatal("Expected error when dispatching order in requires_reconciliation state, got nil")
	}
	if !errors.Is(err, service.ErrOrderReconciliationRequired) {
		t.Fatalf("Expected ErrOrderReconciliationRequired, got %v", err)
	}
}

// 12. Dispatch blocked when stock_released is true
func TestFulfilmentSecurity_StockReleasedDispatchRejection(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	admin := createRegisteredUser(t, env, "sec_admin_rel_1", "super_admin")
	seller, _, prod := helperCreateSellerWithProduct(t, env, ctx, "sec_seller_rel_1", "Seller Store Rel 1", "Safety Goggles", 10, admin)
	buyer := createRegisteredUser(t, env, "sec_buyer_rel_1", "member")

	orderRes, err := env.MarketplaceService.CreateOrder(ctx, buyer, service.CheckoutInput{
		Items:           []service.CartItemInput{{ProductID: prod.ID, Quantity: 1}},
		BuyerName:       "Released Stock Buyer",
		BuyerEmail:      "released_buyer@example.org",
		BuyerPhone:      "+2348011112222",
		DeliveryType:    "delivery",
		ShippingAddress: "456 Released Rd",
	})
	if err != nil {
		t.Fatalf("CreateOrder failed: %v", err)
	}

	// Order is marked as paid, but stock was released
	_, err = env.Pool.Exec(ctx, `
		UPDATE marketplace_orders
		SET payment_status = 'paid',
		    order_status = 'processing',
		    stock_released = true
		WHERE id = $1`, orderRes.Order.ID)
	if err != nil {
		t.Fatalf("Failed to update order stock_released: %v", err)
	}

	// Attempt to dispatch must fail with ErrOrderStockAlreadyReleased
	_, err = env.MarketplaceService.DispatchOrder(ctx, seller, orderRes.Order.ID, service.DispatchOrderInput{
		CarrierName:    "FedEx",
		TrackingNumber: "FDX-REL-001",
	})
	if err == nil {
		t.Fatal("Expected error when dispatching order with stock_released = true, got nil")
	}
	if !errors.Is(err, service.ErrOrderStockAlreadyReleased) {
		t.Fatalf("Expected ErrOrderStockAlreadyReleased, got %v", err)
	}
}
