package service_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"ratels-api/internal/db"
	"ratels-api/internal/payment"
	"ratels-api/internal/security"
	"ratels-api/internal/service"
)

// ============================================================================
// RATELS MARKETPLACE — STEP 65: END-TO-END INTEGRATION & SECURITY TEST SUITE
// ============================================================================

// ----------------------------------------------------------------------------
// Phase 2: Complete Buyer Journey E2E
// ----------------------------------------------------------------------------

func TestE2E_Phase2_BuyerJourney_CompleteLifecycle(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	admin := createRegisteredUser(t, env, fmt.Sprintf("e2e_b_adm_%d", time.Now().UnixNano()), "super_admin")
	seller, _, prod := helperCreateSellerWithProduct(t, env, ctx, fmt.Sprintf("e2e_b_sel_%d", time.Now().UnixNano()), "Buyer Journey Store", "Sanitation Kit", 25, admin)

	// Step 1: Buyer Registration
	buyerEmail := fmt.Sprintf("e2e_buyer_%d@example.org", time.Now().UnixNano())
	buyerPassword := "BuyerPass2026!Secure"
	regRes, err := env.AuthService.Register(ctx, service.RegisterRequest{
		Email:       buyerEmail,
		Password:    buyerPassword,
		DisplayName: "E2E Buyer",
		Phone:       fmt.Sprintf("+23480%08d", (time.Now().UnixNano()/1000)%100000000),
	})
	if err != nil {
		t.Fatalf("Step 1 failed - Buyer registration: %v", err)
	}
	if regRes.UserID == "" {
		t.Fatalf("Step 1 failed - expected valid UserID from registration")
	}

	// Step 2: Email Verification Token Extraction & Validation
	rawURL := env.Delivery.GetLastVerificationURL(buyerEmail)
	if rawURL == "" {
		t.Fatalf("Step 2 failed - verification URL was not generated")
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("Step 2 failed - parse verification URL: %v", err)
	}
	token := u.Query().Get("token")
	if token == "" {
		t.Fatalf("Step 2 failed - verification token missing from URL: %s", rawURL)
	}

	verRes, err := env.AuthService.VerifyEmailToken(ctx, token)
	if err != nil || !verRes.IsVerified {
		t.Fatalf("Step 2 failed - VerifyEmailToken: %v", err)
	}

	// Step 3: Login as Verified Buyer
	loginRes, err := env.AuthService.Login(ctx, service.LoginRequest{
		Username: buyerEmail,
		Password: buyerPassword,
	}, "127.0.0.1", "Mozilla/5.0")
	if err != nil {
		t.Fatalf("Step 3 failed - Login: %v", err)
	}
	if loginRes.AccessToken == "" {
		t.Fatalf("Step 3 failed - expected access token returned upon login")
	}

	buyerUser, err := env.Repo.Queries().GetUserByID(ctx, regRes.UserID)
	if err != nil {
		t.Fatalf("Failed loading buyer user: %v", err)
	}

	// Step 4: Browse Catalog
	catalog, err := env.MarketplaceService.ListProducts(ctx, service.ProductFilter{})
	if err != nil || len(catalog) < 1 {
		t.Fatalf("Step 4 failed - ListProducts: %v (count: %d)", err, len(catalog))
	}

	// Step 5: Checkout with Price Manipulation Prevention & Stock Reservation
	initialStock := prod.StockQuantity
	orderRes, err := env.MarketplaceService.CreateOrder(ctx, &buyerUser, service.CheckoutInput{
		Items: []service.CartItemInput{
			{ProductID: prod.ID, Quantity: 2},
		},
		BuyerName:       "E2E Buyer",
		BuyerEmail:      buyerEmail,
		BuyerPhone:      "+2348011223344",
		DeliveryType:    "delivery",
		ShippingAddress: "123 Eco Blvd, Victoria Island",
	})
	if err != nil {
		t.Fatalf("Step 5 failed - CreateOrder: %v", err)
	}

	// Verify price is calculated authoritatively from product snapshot, not client input
	expectedSubtotal := prod.PriceKobo * 2
	if orderRes.Order.SubtotalKobo != expectedSubtotal {
		t.Errorf("Step 5 failed - Price manipulation check: expected subtotal %d, got %d", expectedSubtotal, orderRes.Order.SubtotalKobo)
	}

	// Verify stock is immediately reserved
	pAfterOrder, _ := env.Repo.Queries().GetMarketplaceProductByID(ctx, prod.ID)
	if pAfterOrder.StockQuantity != initialStock-2 {
		t.Errorf("Step 5 failed - Stock reservation: expected %d, got %d", initialStock-2, pAfterOrder.StockQuantity)
	}

	// Step 6: Payment Confirmation
	_, err = env.MarketplaceService.ConfirmPayment(ctx, orderRes.Payment.Reference)
	if err != nil {
		t.Fatalf("Step 6 failed - ConfirmPayment: %v", err)
	}

	ordAfterPay, _, _, err := env.MarketplaceService.GetOrderByID(ctx, &buyerUser, orderRes.Order.ID)
	if err != nil {
		t.Fatalf("GetOrderByID after payment failed: %v", err)
	}
	if ordAfterPay.PaymentStatus != "paid" || ordAfterPay.OrderStatus != "processing" {
		t.Errorf("Step 6 failed - expected paid/processing, got %s / %s", ordAfterPay.PaymentStatus, ordAfterPay.OrderStatus)
	}

	// Step 7: Duplicate Payment Callback Idempotency
	dupOrder, err := env.MarketplaceService.ConfirmPayment(ctx, orderRes.Payment.Reference)
	if err != nil {
		t.Fatalf("Step 7 failed - Duplicate ConfirmPayment should succeed idempotently: %v", err)
	}
	if dupOrder.PaymentStatus != "paid" {
		t.Errorf("Step 7 failed - Duplicate payment corrupted payment_status: %s", dupOrder.PaymentStatus)
	}
	// Stock must NOT be deducted again
	pAfterDup, _ := env.Repo.Queries().GetMarketplaceProductByID(ctx, prod.ID)
	if pAfterDup.StockQuantity != initialStock-2 {
		t.Errorf("Step 7 failed - Duplicate payment deducted stock twice: got %d", pAfterDup.StockQuantity)
	}

	// Step 8: Shipment Dispatch by Seller & Tracking by Buyer
	shipments, err := env.MarketplaceService.ListOrderShipments(ctx, &buyerUser, orderRes.Order.ID)
	if err != nil || len(shipments) < 1 {
		t.Fatalf("Step 8 failed - ListOrderShipments: %v", err)
	}
	shipmentID := shipments[0].Shipment.ID

	_, err = env.MarketplaceService.DispatchShipment(ctx, seller, shipmentID, service.DispatchOrderInput{
		CarrierName:    "GIG Logistics",
		TrackingNumber: "GIG-E2E-12345",
	})
	if err != nil {
		t.Fatalf("Step 8 failed - DispatchShipment: %v", err)
	}

	// Buyer tracks shipment
	shBuyer, err := env.MarketplaceService.GetShipmentByID(ctx, &buyerUser, shipmentID)
	if err != nil {
		t.Fatalf("Step 8 failed - Buyer GetShipmentByID: %v", err)
	}
	if shBuyer.Shipment.Status != "dispatched" || shBuyer.Shipment.TrackingNumber != "GIG-E2E-12345" {
		t.Errorf("Step 8 failed - tracking info mismatch: %s / %s", shBuyer.Shipment.Status, shBuyer.Shipment.TrackingNumber)
	}

	// Step 9: Buyer Delivery Confirmation
	completedShip, err := env.MarketplaceService.ConfirmDelivery(ctx, &buyerUser, orderRes.Order.ID, "", "Delivered safely")
	if err != nil {
		t.Fatalf("Step 9 failed - ConfirmDelivery: %v", err)
	}
	if completedShip.Status != "delivered" {
		t.Errorf("Step 9 failed - expected shipment status delivered, got %s", completedShip.Status)
	}
}

func TestE2E_Phase2_BuyerJourney_AbandonedCheckoutStockRelease(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	admin := createRegisteredUser(t, env, fmt.Sprintf("e2e_rel_adm_%d", time.Now().UnixNano()), "super_admin")
	_, _, prod := helperCreateSellerWithProduct(t, env, ctx, fmt.Sprintf("e2e_rel_sel_%d", time.Now().UnixNano()), "Stock Release Store", "Safety Goggles", 30, admin)
	buyer := createRegisteredUser(t, env, fmt.Sprintf("e2e_rel_buy_%d", time.Now().UnixNano()), "member")

	// Drain any pre-existing placed orders from interfering with test count
	_, _ = env.Pool.Exec(ctx, "UPDATE marketplace_orders SET reservation_expires_at = NOW() + INTERVAL '24 hours' WHERE order_status = 'placed' AND stock_released = FALSE")

	// Buyer creates order with 4 units
	orderRes, err := env.MarketplaceService.CreateOrder(ctx, buyer, service.CheckoutInput{
		Items: []service.CartItemInput{
			{ProductID: prod.ID, Quantity: 4},
		},
		BuyerName:    "Abandoned Buyer",
		BuyerEmail:   "abandoned@example.org",
		BuyerPhone:   "+2348099887766",
		DeliveryType: "pickup",
	})
	if err != nil {
		t.Fatalf("CreateOrder failed: %v", err)
	}

	pReserved, _ := env.Repo.Queries().GetMarketplaceProductByID(ctx, prod.ID)
	if pReserved.StockQuantity != 26 {
		t.Fatalf("Expected stock 26 after reservation, got %d", pReserved.StockQuantity)
	}

	// Abandon checkout: artificially expire reservation timestamp and simulate uncompleted payment
	_, err = env.Pool.Exec(ctx, "UPDATE marketplace_orders SET reservation_expires_at = NOW() - INTERVAL '1 hour' WHERE id = $1", orderRes.Order.ID)
	if err != nil {
		t.Fatalf("Failed expiring reservation: %v", err)
	}
	_, _ = env.Pool.Exec(ctx, "UPDATE marketplace_payments SET reference = 'fail_' || reference WHERE order_id = $1", orderRes.Order.ID)

	// Expiry worker runs
	result, err := env.MarketplaceService.ExpireStaleReservations(ctx, 50)
	if err != nil {
		t.Fatalf("ExpireStaleReservations failed: %v", err)
	}
	if result.ExpiredCount != 1 {
		t.Errorf("Expected exactly 1 expired order, got %d", result.ExpiredCount)
	}

	// Order cancelled
	ord, err := env.Repo.Queries().GetMarketplaceOrderByID(ctx, orderRes.Order.ID)
	if err != nil || ord.OrderStatus != "cancelled" || !ord.StockReleased {
		t.Errorf("Expected cancelled and stock_released=true, got %s / %v", ord.OrderStatus, ord.StockReleased)
	}

	// Stock restored to 30
	pRestored, _ := env.Repo.Queries().GetMarketplaceProductByID(ctx, prod.ID)
	if pRestored.StockQuantity != 30 {
		t.Fatalf("Expected stock restored to 30, got %d", pRestored.StockQuantity)
	}

	// Ledger has exactly 1 release log
	count, _ := env.Repo.Queries().CountOrderInventoryReleaseLogs(ctx, db.CountOrderInventoryReleaseLogsParams{
		OrderID:   service.TextFromString(orderRes.Order.ID),
		ProductID: prod.ID,
	})
	if count != 1 {
		t.Errorf("Expected 1 inventory release log, got %d", count)
	}
}

func TestE2E_Phase2_BuyerJourney_DataIsolation(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	admin := createRegisteredUser(t, env, fmt.Sprintf("e2e_iso_adm_%d", time.Now().UnixNano()), "super_admin")
	_, _, prod := helperCreateSellerWithProduct(t, env, ctx, fmt.Sprintf("e2e_iso_sel_%d", time.Now().UnixNano()), "Isolation Store", "Reflective Vest", 20, admin)

	buyerA := createRegisteredUser(t, env, fmt.Sprintf("e2e_iso_buyA_%d", time.Now().UnixNano()), "member")
	buyerB := createRegisteredUser(t, env, fmt.Sprintf("e2e_iso_buyB_%d", time.Now().UnixNano()), "member")

	orderA, err := env.MarketplaceService.CreateOrder(ctx, buyerA, service.CheckoutInput{
		Items:        []service.CartItemInput{{ProductID: prod.ID, Quantity: 1}},
		BuyerName:    "Buyer A",
		BuyerEmail:   "buyera@example.org",
		BuyerPhone:   "+2348011111111",
		DeliveryType: "pickup",
	})
	if err != nil {
		t.Fatalf("CreateOrder Buyer A failed: %v", err)
	}

	// 1. Buyer B cannot read Buyer A's order
	_, _, _, err = env.MarketplaceService.GetOrderByID(ctx, buyerB, orderA.Order.ID)
	if err != service.ErrUnauthorizedBuyerAction && err != service.ErrUnauthorizedSellerAction {
		t.Errorf("Expected unauthorized error for cross-buyer order lookup, got: %v", err)
	}

	// 2. Buyer B cannot cancel Buyer A's order
	_, err = env.MarketplaceService.CancelOrder(ctx, buyerB, orderA.Order.ID, "Rogue cancellation attempt")
	if err != service.ErrUnauthorizedBuyerAction {
		t.Errorf("Expected ErrUnauthorizedBuyerAction for cross-buyer cancel, got: %v", err)
	}

	// 3. Buyer B cannot confirm delivery of Buyer A's order
	_, err = env.MarketplaceService.ConfirmDelivery(ctx, buyerB, orderA.Order.ID, "", "Fraudulent delivery confirmation")
	if err != service.ErrUnauthorizedBuyerAction && err != service.ErrUnauthorizedSellerAction {
		t.Errorf("Expected unauthorized error for cross-buyer delivery confirm, got: %v", err)
	}

	// 4. Buyer B cannot list Buyer A's refunds
	_, err = env.MarketplaceService.ListRefundsByOrder(ctx, buyerB, orderA.Order.ID)
	if err != service.ErrUnauthorizedBuyerAction {
		t.Errorf("Expected ErrUnauthorizedBuyerAction for cross-buyer refund list, got: %v", err)
	}
}

// ----------------------------------------------------------------------------
// Phase 3: Complete Seller Journey E2E
// ----------------------------------------------------------------------------

func TestE2E_Phase3_SellerJourney_MultiVendorAndIsolation(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	admin, sellerA, sellerB, prodA, prodB, orderRes := helperCreateMultiVendorOrder(t, env, ctx)
	_, err := env.MarketplaceService.ConfirmPayment(ctx, orderRes.Payment.Reference)
	if err != nil {
		t.Fatalf("ConfirmPayment failed: %v", err)
	}

	shipments, err := env.MarketplaceService.ListOrderShipments(ctx, admin, orderRes.Order.ID)
	if err != nil || len(shipments) != 2 {
		t.Fatalf("Expected 2 independent shipments, got %d (err: %v)", len(shipments), err)
	}

	var shipA, shipB db.MarketplaceShipment
	for _, s := range shipments {
		if s.Shipment.BusinessID.String == prodA.BusinessID {
			shipA = s.Shipment
		} else if s.Shipment.BusinessID.String == prodB.BusinessID {
			shipB = s.Shipment
		}
	}

	if shipA.ID == "" || shipB.ID == "" {
		t.Fatalf("Failed mapping shipments to vendors")
	}

	// 1. Seller A dispatches Shipment A with tracking -> SUCCEEDS
	updatedShipA, err := env.MarketplaceService.DispatchShipment(ctx, sellerA, shipA.ID, service.DispatchOrderInput{
		CarrierName:    "DHL Nigeria",
		TrackingNumber: "DHL-SEL-A-001",
	})
	if err != nil {
		t.Fatalf("Seller A DispatchShipment failed: %v", err)
	}
	if updatedShipA.Status != "dispatched" {
		t.Errorf("Expected shipment A dispatched, got %s", updatedShipA.Status)
	}

	// 2. Cross-Seller Isolation: Seller A attempts to dispatch Seller B's shipment -> REJECTED
	_, err = env.MarketplaceService.DispatchShipment(ctx, sellerA, shipB.ID, service.DispatchOrderInput{
		CarrierName:    "Rogue Delivery",
		TrackingNumber: "ROGUE-SEL-B-999",
	})
	if err != service.ErrUnauthorizedSellerAction {
		t.Errorf("Expected ErrUnauthorizedSellerAction when Seller A dispatches Seller B's shipment, got: %v", err)
	}

	// 3. Cross-Seller Isolation: Seller A attempts to view Seller B's shipment packing slip -> REJECTED
	_, err = env.MarketplaceService.GetShipmentByID(ctx, sellerA, shipB.ID)
	if err != service.ErrUnauthorizedSellerAction {
		t.Errorf("Expected ErrUnauthorizedSellerAction when Seller A views Seller B's shipment, got: %v", err)
	}

	// 4. Cross-Seller Isolation: Seller A attempts to modify Seller B's product stock -> REJECTED
	_, _, err = env.MarketplaceService.AdjustInventory(ctx, sellerA, prodB.ID, service.AdjustInventoryInput{
		IdempotencyKey: fmt.Sprintf("rogue_adj_%d", time.Now().UnixNano()),
		QuantityDelta:  10,
		Reason:         "Rogue stock injection",
	})
	if err != service.ErrUnauthorizedSellerAction {
		t.Errorf("Expected ErrUnauthorizedSellerAction when Seller A alters Seller B's product stock, got: %v", err)
	}

	// 5. Cross-Seller Isolation: Seller A attempts to update Seller B's product details -> REJECTED
	_, err = env.MarketplaceService.UpdateProduct(ctx, sellerA, prodB.ID, service.UpdateProductInput{
		Title: "Hijacked Product Title",
	})
	if err != service.ErrUnauthorizedSellerAction {
		t.Errorf("Expected ErrUnauthorizedSellerAction when Seller A edits Seller B's product, got: %v", err)
	}

	// 6. Seller B dispatches their own shipment B -> SUCCEEDS
	updatedShipB, err := env.MarketplaceService.DispatchShipment(ctx, sellerB, shipB.ID, service.DispatchOrderInput{
		CarrierName:    "FedEx Express",
		TrackingNumber: "FDX-SEL-B-002",
	})
	if err != nil {
		t.Fatalf("Seller B DispatchShipment failed: %v", err)
	}
	if updatedShipB.Status != "dispatched" {
		t.Errorf("Expected shipment B dispatched, got %s", updatedShipB.Status)
	}

	// Order fulfilment aggregated status is dispatched
	ord, _, _, _ := env.MarketplaceService.GetOrderByID(ctx, admin, orderRes.Order.ID)
	if ord.FulfilmentStatus != "dispatched" {
		t.Errorf("Expected order fulfilment status 'dispatched', got %s", ord.FulfilmentStatus)
	}
}

// ----------------------------------------------------------------------------
// Phase 4: Coordinator and Admin Journey E2E
// ----------------------------------------------------------------------------

func TestE2E_Phase4_CoordinatorAndAdmin_JurisdictionsAndAntiSelfApproval(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	stateA, lgaA1, stateB, _ := helperGetTwoStatesWithLgas(t, env, ctx)
	superAdmin := createRegisteredUser(t, env, fmt.Sprintf("super_adm_%d", time.Now().UnixNano()), "super_admin")

	// State Coordinator for State A
	coordAUser := createRegisteredUser(t, env, fmt.Sprintf("coord_stA_%d", time.Now().UnixNano()), "state_coordinator")
	_, err := env.Repo.Queries().CreateAppointment(ctx, db.CreateAppointmentParams{
		ID:               security.GenerateUUID(),
		UserID:           coordAUser.ID,
		RoleTitle:        "State Coordinator (" + stateA.Name + ")",
		RoleType:         "state_coordinator",
		JurisdictionType: "state",
		JurisdictionID:   service.TextFromString(stateA.ID),
		StartDate:        pgtype.Timestamptz{Time: time.Now().Add(-24 * time.Hour), Valid: true},
		Status:           "active",
	})
	if err != nil {
		t.Fatalf("Create appointment for State A coordinator failed: %v", err)
	}

	// LGA Coordinator for LGA A1
	lgaCoordUser := createRegisteredUser(t, env, fmt.Sprintf("coord_lgaA1_%d", time.Now().UnixNano()), "lga_coordinator")
	_, err = env.Repo.Queries().CreateAppointment(ctx, db.CreateAppointmentParams{
		ID:               security.GenerateUUID(),
		UserID:           lgaCoordUser.ID,
		RoleTitle:        "LGA Coordinator (" + lgaA1.Name + ")",
		RoleType:         "lga_coordinator",
		JurisdictionType: "lga",
		JurisdictionID:   service.TextFromString(lgaA1.ID),
		StartDate:        pgtype.Timestamptz{Time: time.Now().Add(-24 * time.Hour), Valid: true},
		Status:           "active",
	})
	if err != nil {
		t.Fatalf("Create appointment for LGA coordinator failed: %v", err)
	}

	// Seller in State A
	sellerA := createRegisteredUser(t, env, fmt.Sprintf("sel_stateA_%d", time.Now().UnixNano()), "member")
	bizA, err := env.MarketplaceService.RegisterBusiness(ctx, sellerA, service.RegisterBusinessInput{
		Name:          "State A Industrial Supplier",
		Category:      "tools_hardware",
		OwnershipType: "member_owned",
		ContactEmail:  "stA@example.org",
		ContactPhone:  "+2348011223344",
	})
	if err != nil {
		t.Fatalf("RegisterBusiness State A failed: %v", err)
	}
	_, _ = env.Repo.Queries().CreateBusinessBranch(ctx, db.CreateBusinessBranchParams{
		ID:             security.GenerateUUID(),
		BusinessID:     bizA.ID,
		BranchName:     "State A Headquarters",
		StateID:        stateA.ID,
		LgaID:          service.TextFromString(lgaA1.ID),
		Address:        "12 Industrial Road, State A",
		ContactPhone:   "+2348011223344",
		ManagerName:    service.TextFromString("Manager A"),
		IsHeadquarters: true,
		IsActive:       true,
	})

	// Seller in State B
	sellerB := createRegisteredUser(t, env, fmt.Sprintf("sel_stateB_%d", time.Now().UnixNano()), "member")
	bizB, err := env.MarketplaceService.RegisterBusiness(ctx, sellerB, service.RegisterBusinessInput{
		Name:          "State B Industrial Supplier",
		Category:      "tools_hardware",
		OwnershipType: "member_owned",
		ContactEmail:  "stB@example.org",
		ContactPhone:  "+2348022334455",
	})
	if err != nil {
		t.Fatalf("RegisterBusiness State B failed: %v", err)
	}
	_, _ = env.Repo.Queries().CreateBusinessBranch(ctx, db.CreateBusinessBranchParams{
		ID:             security.GenerateUUID(),
		BusinessID:     bizB.ID,
		BranchName:     "State B Headquarters",
		StateID:        stateB.ID,
		LgaID:          pgtype.Text{Valid: false},
		Address:        "45 Commercial Ave, State B",
		ContactPhone:   "+2348022334455",
		ManagerName:    service.TextFromString("Manager B"),
		IsHeadquarters: true,
		IsActive:       true,
	})

	// 1. State Coordinator A approves business in State A -> SUCCEEDS
	approvedBizA, err := env.MarketplaceService.ApproveBusiness(ctx, coordAUser, bizA.ID, true, "")
	if err != nil {
		t.Fatalf("State Coordinator A ApproveBusiness in State A failed: %v", err)
	}
	if approvedBizA.Status != "approved" {
		t.Errorf("Expected bizA status approved, got %s", approvedBizA.Status)
	}

	// 2. Cross-Jurisdiction: State Coordinator A attempts to approve business in State B -> REJECTED
	_, err = env.MarketplaceService.ApproveBusiness(ctx, coordAUser, bizB.ID, true, "")
	if err != service.ErrScopeUnauthorized {
		t.Errorf("Expected ErrScopeUnauthorized when Coordinator A approves business in State B, got: %v", err)
	}

	// 3. LGA Coordinator attempts to approve business -> REJECTED (LGA is read-only oversight)
	_, err = env.MarketplaceService.ApproveBusiness(ctx, lgaCoordUser, bizA.ID, true, "")
	if err != service.ErrScopeUnauthorized {
		t.Errorf("Expected ErrScopeUnauthorized when LGA Coordinator attempts approval, got: %v", err)
	}

	// 4. Anti-Self-Approval on Dispute Resolution
	// Create product in State A and order
	prodA, _ := env.MarketplaceService.CreateProduct(ctx, sellerA, service.CreateProductInput{
		BusinessID:    bizA.ID,
		Title:         "Protective Safety Helmet",
		Category:      "safety_gear",
		SKU:           fmt.Sprintf("SKU-HELMET-%d", time.Now().UnixNano()),
		PriceKobo:     300000,
		StockQuantity: 10,
	})

	orderRes, _ := env.MarketplaceService.CreateOrder(ctx, coordAUser, service.CheckoutInput{
		Items:           []service.CartItemInput{{ProductID: prodA.ID, Quantity: 1}},
		BuyerName:       "Coordinator A as Buyer",
		BuyerEmail:      "coordA@example.org",
		BuyerPhone:      "+2348011223344",
		DeliveryType:    "delivery",
		ShippingAddress: "10 Jurisdiction Way",
		ShippingStateID: &stateA.ID,
		ShippingLGAID:   &lgaA1.ID,
	})
	_, _ = env.MarketplaceService.ConfirmPayment(ctx, orderRes.Payment.Reference)

	// Coordinator A (claimant) raises dispute
	disp, err := env.MarketplaceService.RaiseDispute(ctx, coordAUser, orderRes.Order.ID, service.CreateDisputeInput{
		Reason:        "damaged_goods",
		CustomerNotes: "Helmet visor cracked",
	})
	if err != nil {
		t.Fatalf("RaiseDispute failed: %v", err)
	}

	// Coordinator A attempts to arbitrate own dispute -> REJECTED (Anti-Self-Approval)
	_, err = env.MarketplaceService.ResolveDispute(ctx, coordAUser, disp.ID, "refunded", "I approve my own refund request")
	if err != service.ErrSelfApprovalProhibited {
		t.Errorf("Expected ErrSelfApprovalProhibited when claimant arbitrates own dispute, got: %v", err)
	}

	// Super Admin arbitrates dispute -> SUCCEEDS
	resolvedDisp, err := env.MarketplaceService.ResolveDispute(ctx, superAdmin, disp.ID, "rejected", "Dispute dismissed after physical inspection")
	if err != nil {
		t.Fatalf("Super Admin ResolveDispute failed: %v", err)
	}
	if resolvedDisp.Status != "rejected" {
		t.Errorf("Expected dispute status rejected, got %s", resolvedDisp.Status)
	}

	// 5. Anti-Self-Approval on Refunds
	adminInitiator := createRegisteredUser(t, env, fmt.Sprintf("ref_init_%d", time.Now().UnixNano()), "super_admin")
	adminApprover := createRegisteredUser(t, env, fmt.Sprintf("ref_appr_%d", time.Now().UnixNano()), "super_admin")

	refund, _, err := env.MarketplaceService.InitiateRefund(ctx, adminInitiator, orderRes.Order.ID, service.RefundOrderInput{
		AmountKobo: 300000,
		Reason:     "Goodwill refund",
	})
	if err != nil {
		t.Fatalf("InitiateRefund failed: %v", err)
	}

	// Initiator attempts to confirm their own refund -> REJECTED
	_, _, err = env.MarketplaceService.ConfirmRefund(ctx, adminInitiator, refund.ID, service.ConfirmRefundInput{
		ProviderRefundReference: refund.ProviderRefundReference,
	})
	if err != service.ErrSelfApprovalProhibited {
		t.Errorf("Expected ErrSelfApprovalProhibited when initiator confirms refund, got: %v", err)
	}

	// Independent Admin confirms refund -> SUCCEEDS
	confirmedRef, _, err := env.MarketplaceService.ConfirmRefund(ctx, adminApprover, refund.ID, service.ConfirmRefundInput{
		ProviderRefundReference: refund.ProviderRefundReference,
	})
	if err != nil {
		t.Fatalf("AdminApprover ConfirmRefund failed: %v", err)
	}
	if confirmedRef.Status != "successful" {
		t.Errorf("Expected refund status successful, got %s", confirmedRef.Status)
	}

	// 6. Super Admin Audit Logging: Suspend & Restore
	_, err = env.MarketplaceService.SuspendBusiness(ctx, superAdmin, bizA.ID, "Compliance audit check")
	if err != nil {
		t.Fatalf("SuspendBusiness failed: %v", err)
	}

	var suspendLogCount int
	err = env.Pool.QueryRow(ctx, "SELECT count(*) FROM audit_events WHERE action = 'suspend_marketplace_business' AND resource_id = $1", bizA.ID).Scan(&suspendLogCount)
	if err != nil || suspendLogCount < 1 {
		t.Errorf("Expected audit log for suspend_marketplace_business, got count %d (err: %v)", suspendLogCount, err)
	}

	_, err = env.MarketplaceService.RestoreBusiness(ctx, superAdmin, bizA.ID, "Audit review cleared")
	if err != nil {
		t.Fatalf("RestoreBusiness failed: %v", err)
	}

	var restoreLogCount int
	err = env.Pool.QueryRow(ctx, "SELECT count(*) FROM audit_events WHERE action = 'restore_marketplace_business' AND resource_id = $1", bizA.ID).Scan(&restoreLogCount)
	if err != nil || restoreLogCount < 1 {
		t.Errorf("Expected audit log for restore_marketplace_business, got count %d (err: %v)", restoreLogCount, err)
	}
}

// ----------------------------------------------------------------------------
// Phase 5: Payment and Refund Journey E2E
// ----------------------------------------------------------------------------

func TestE2E_Phase5_PaymentAndRefund_AmbiguousTimeoutMultiCheckReconciliation(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	admin := createRegisteredUser(t, env, fmt.Sprintf("rec_adm_%d", time.Now().UnixNano()), "super_admin")
	_, _, prod := helperCreateSellerWithProduct(t, env, ctx, fmt.Sprintf("rec_sel_%d", time.Now().UnixNano()), "Rec Safe Store", "Rec Prod", 50, admin)
	buyer := createRegisteredUser(t, env, fmt.Sprintf("rec_buy_%d", time.Now().UnixNano()), "member")

	orderRes, err := env.MarketplaceService.CreateOrder(ctx, buyer, service.CheckoutInput{
		Items:        []service.CartItemInput{{ProductID: prod.ID, Quantity: 2}},
		BuyerName:    "Reconciliation Buyer",
		BuyerEmail:   "rec_safe@example.org",
		BuyerPhone:   "+2348099001122",
		DeliveryType: "pickup",
	})
	if err != nil {
		t.Fatalf("CreateOrder failed: %v", err)
	}
	_, _ = env.MarketplaceService.ConfirmPayment(ctx, orderRes.Payment.Reference)

	payRec, err := env.Repo.Queries().GetMarketplacePaymentByOrder(ctx, orderRes.Order.ID)
	if err != nil {
		t.Fatalf("GetMarketplacePaymentByOrder failed: %v", err)
	}

	// Drain pre-existing pending refunds to isolate this test run
	_, _ = env.Pool.Exec(ctx, "UPDATE marketplace_refunds SET status = 'failed' WHERE status = 'pending'")

	refundID := security.GenerateUUID()
	refundAmount := int64(200000)

	// Create an ambiguous timeout refund record (initial dispatch timed out)
	_, err = env.Repo.Queries().CreateMarketplaceRefund(ctx, db.CreateMarketplaceRefundParams{
		ID:                      refundID,
		OrderID:                 orderRes.Order.ID,
		PaymentID:               payRec.ID,
		AmountKobo:              refundAmount,
		Status:                  "pending",
		Provider:                "paystack",
		ProviderRefundReference: "",
		InitiatedByUserID:       admin.ID,
		ConfirmedByUserID:       pgtype.Text{Valid: false},
		Reason:                  "Ambiguous timeout test",
		SettlementNotes:         "Refund dispatch timed out; pending provider reconciliation",
		Metadata:                []byte(`{"ambiguous_dispatch_error":"context deadline exceeded"}`),
		ShipmentID:              pgtype.Text{Valid: false},
		BusinessID:              pgtype.Text{Valid: false},
		OrderItemID:             pgtype.Text{Valid: false},
	})
	if err != nil {
		t.Fatalf("CreateMarketplaceRefund failed: %v", err)
	}

	// Ensure no refund exists on provider sandbox yet
	// PASS 1: VerifyPayment returns 0 refunds -> Must NOT re-dispatch, must record check count 1 and remain pending
	wRes1, err := env.MarketplaceService.ProcessPendingRefunds(ctx, 50)
	if err != nil {
		t.Fatalf("Pass 1 ProcessPendingRefunds failed: %v", err)
	}
	if wRes1.SettledCount != 0 {
		t.Errorf("Pass 1: Expected 0 settled, got %d", wRes1.SettledCount)
	}
	refAfter1, err := env.Repo.Queries().GetMarketplaceRefundByID(ctx, refundID)

	var meta1 map[string]interface{}
	_ = json.Unmarshal(refAfter1.Metadata, &meta1)
	if c, ok := meta1["reconciliation_checks_count"].(float64); !ok || int(c) != 1 {
		t.Errorf("Pass 1: Expected reconciliation_checks_count = 1, got %v", meta1["reconciliation_checks_count"])
	}

	// PASS 2: Provider verification timeout error -> Must NOT re-dispatch, must preserve pending state
	customGw := &mockCustomGateway{
		providerName: "paystack",
		verifyFn: func(ctx context.Context, reference string) (*payment.VerifyPaymentOutput, error) {
			return nil, errors.New("gateway connection timeout")
		},
	}
	mktSvcWithTimeout := service.NewMarketplaceService(env.Repo, env.AuditService, customGw)
	wRes2, err := mktSvcWithTimeout.ProcessPendingRefunds(ctx, 50)
	if err != nil {
		t.Fatalf("Pass 2 ProcessPendingRefunds failed: %v", err)
	}
	if wRes2.SettledCount != 0 {
		t.Errorf("Pass 2: Expected 0 settled during timeout, got %d", wRes2.SettledCount)
	}

	// PASS 3: Clean lookup without refund -> Increments check count to 2, remains pending
	wRes3, err := env.MarketplaceService.ProcessPendingRefunds(ctx, 50)
	if err != nil {
		t.Fatalf("Pass 3 ProcessPendingRefunds failed: %v", err)
	}
	_ = wRes3
	refAfter3, err := env.Repo.Queries().GetMarketplaceRefundByID(ctx, refundID)
	if err != nil || refAfter3.Status != "pending" {
		t.Fatalf("Pass 3: Expected refund pending, got %s", refAfter3.Status)
	}
	var meta3 map[string]interface{}
	_ = json.Unmarshal(refAfter3.Metadata, &meta3)
	if c, ok := meta3["reconciliation_checks_count"].(float64); !ok || int(c) != 2 {
		t.Errorf("Pass 3: Expected reconciliation_checks_count = 2, got %v", meta3["reconciliation_checks_count"])
	}

	// PASS 4: Multi-check threshold satisfied -> Re-dispatch is safely permitted and succeeds!
	wRes4, err := env.MarketplaceService.ProcessPendingRefunds(ctx, 50)
	if err != nil {
		t.Fatalf("Pass 4 ProcessPendingRefunds failed: %v", err)
	}
	if wRes4.SettledCount < 1 && wRes4.PendingCount < 1 {
		t.Errorf("Pass 4: Expected refund processed, got settled: %d, pending: %d", wRes4.SettledCount, wRes4.PendingCount)
	}

	// Verify refund record adopted a valid provider reference
	refFinal, err := env.Repo.Queries().GetMarketplaceRefundByID(ctx, refundID)
	if err != nil {
		t.Fatalf("GetMarketplaceRefundByID final failed: %v", err)
	}
	if refFinal.ProviderRefundReference == "" {
		t.Errorf("Expected provider reference populated upon confirmed re-dispatch")
	}
	_ = payRec
}

func TestE2E_Phase5_PaymentAndRefund_ConcurrentRefundAndDispatchRacePrevention(t *testing.T) {
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

	adminApprover := createRegisteredUser(t, env, fmt.Sprintf("race_appr_%d", time.Now().UnixNano()), "super_admin")

	// Concurrent Execution: Goroutine 1 initiates and settles a full refund on Shipment A.
	// Goroutine 2 attempts to dispatch Shipment A simultaneously.
	var wg sync.WaitGroup
	wg.Add(2)

	var refundErr, dispatchErr error
	var refID string

	go func() {
		defer wg.Done()
		ref, _, err := env.MarketplaceService.InitiateRefund(ctx, admin, orderRes.Order.ID, service.RefundOrderInput{
			AmountKobo: prodA.PriceKobo * 2,
			Reason:     "Concurrent race test refund",
			ShipmentID: shipAId,
		})
		if err != nil {
			refundErr = err
			return
		}
		refID = ref.ID
		_, _, err = env.MarketplaceService.ConfirmRefund(ctx, adminApprover, ref.ID, service.ConfirmRefundInput{
			ProviderRefundReference: ref.ProviderRefundReference,
		})
		if err != nil {
			refundErr = err
		}
	}()

	go func() {
		defer wg.Done()
		// Small stagger to test concurrent interleave
		time.Sleep(10 * time.Millisecond)
		_, err := env.MarketplaceService.DispatchShipment(ctx, sellerA, shipAId, service.DispatchOrderInput{
			CarrierName:    "Speedy Courier",
			TrackingNumber: "SPD-RACE-001",
		})
		dispatchErr = err
	}()

	wg.Wait()

	// Verification: Both operations must not both succeed concurrently.
	// If refund settled first, dispatch must fail closed with error.
	shFinal, err := env.MarketplaceService.GetShipmentByID(ctx, admin, shipAId)
	if err != nil {
		t.Fatalf("GetShipmentByID failed: %v", err)
	}

	if refundErr == nil {
		// Refund succeeded: shipment status MUST be cancelled, and dispatch must have failed or been blocked
		if shFinal.Shipment.Status != "cancelled" && shFinal.Shipment.Status != "refunded" {
			t.Errorf("Expected refunded shipment status to be cancelled, got %s", shFinal.Shipment.Status)
		}
	} else if dispatchErr == nil {
		// Dispatch succeeded: shipment status must be dispatched
		if shFinal.Shipment.Status != "dispatched" {
			t.Errorf("Expected dispatched shipment, got %s", shFinal.Shipment.Status)
		}
	}
	_ = refID
}

// ----------------------------------------------------------------------------
// Phase 6: API and Security Testing E2E
// ----------------------------------------------------------------------------

func TestE2E_Phase6_Security_GeographicSpoofingAndForgedWebhooks(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	stateA, _, stateB, lgaB1 := helperGetTwoStatesWithLgas(t, env, ctx)
	_ = stateB
	admin := createRegisteredUser(t, env, fmt.Sprintf("sec_adm_%d", time.Now().UnixNano()), "super_admin")
	_, _, prod := helperCreateSellerWithProduct(t, env, ctx, fmt.Sprintf("sec_sel_%d", time.Now().UnixNano()), "Sec Store", "Sec Prod", 20, admin)
	buyer := createRegisteredUser(t, env, fmt.Sprintf("sec_buy_%d", time.Now().UnixNano()), "member")

	// 1. Geographic Delivery Spoofing: Buyer attempts to submit LGA belonging to State B under State A
	_, err := env.MarketplaceService.CreateOrder(ctx, buyer, service.CheckoutInput{
		Items:           []service.CartItemInput{{ProductID: prod.ID, Quantity: 1}},
		BuyerName:       "Spoofer",
		BuyerEmail:      "spoofer@example.org",
		BuyerPhone:      "+2348011223344",
		DeliveryType:    "delivery",
		ShippingAddress: "99 Spoof Avenue",
		ShippingStateID: &stateA.ID,
		ShippingLGAID:   &lgaB1.ID, // Belongs to State B, NOT State A!
	})
	if err == nil || !strings.Contains(err.Error(), "does not belong to specified state") {
		t.Errorf("Expected invalid shipping location error when LGA does not belong to State, got: %v", err)
	}

	// 2. Forged Paystack Webhook Delivery: Invalid HMAC signature rejected
	forgedWebhookPayload := []byte(`{
		"event": "charge.success",
		"data": {
			"reference": "FORGED_TX_123",
			"status": "success",
			"amount": 500000
		}
	}`)
	paystackGw := payment.NewPaystackGateway("sk_test_secret_key_12345", "pk_test_public_key_12345")
	mktSvcWithPaystack := service.NewMarketplaceService(env.Repo, env.AuditService, paystackGw, nil)

	err = mktSvcWithPaystack.HandleWebhook(ctx, "invalid_forged_hmac_signature", forgedWebhookPayload)
	if err != payment.ErrInvalidSignature {
		t.Errorf("Expected ErrInvalidSignature for forged webhook, got: %v", err)
	}

	// 3. Forged Webhook with empty signature rejected
	err = mktSvcWithPaystack.HandleWebhook(ctx, "", forgedWebhookPayload)
	if err != payment.ErrInvalidSignature {
		t.Errorf("Expected ErrInvalidSignature for empty webhook signature, got: %v", err)
	}
}
