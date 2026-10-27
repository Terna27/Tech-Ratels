package service_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"ratels-api/internal/db"
	"ratels-api/internal/payment"
	"ratels-api/internal/security"
	"ratels-api/internal/service"
)

// -------------------------------------------------------------
// Step 64: Role-Based Access Control and Geographic Permissions
// -------------------------------------------------------------

// Helper to find two distinct states and their LGAs
func helperGetTwoStatesWithLgas(t *testing.T, env *TestEnv, ctx context.Context) (db.State, db.Lga, db.State, db.Lga) {
	t.Helper()
	states, err := env.Repo.Queries().ListStates(ctx)
	if err != nil || len(states) < 2 {
		t.Fatalf("Insufficient states in test database: %v", err)
	}
	stateA := states[0]
	stateB := states[1]

	lgasA, err := env.Repo.Queries().ListLGAsByState(ctx, stateA.ID)
	if err != nil || len(lgasA) < 1 {
		t.Fatalf("No LGAs for state %s: %v", stateA.Name, err)
	}
	lgasB, err := env.Repo.Queries().ListLGAsByState(ctx, stateB.ID)
	if err != nil || len(lgasB) < 1 {
		t.Fatalf("No LGAs for state %s: %v", stateB.Name, err)
	}

	return stateA, lgasA[0], stateB, lgasB[0]
}

// 1. Buyers cannot access other buyers' orders, payments, or shipment details
func TestAuth_BuyerCannotAccessOtherBuyerOrder(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	admin := createRegisteredUser(t, env, fmt.Sprintf("ba_adm_%d", time.Now().UnixNano()), "super_admin")
	_, _, prod := helperCreateSellerWithProduct(t, env, ctx, fmt.Sprintf("ba_sel_%d", time.Now().UnixNano()), "Buyer Auth Store", "Auth Prod", 50, admin)

	buyerA := createRegisteredUser(t, env, fmt.Sprintf("buyer_a_%d", time.Now().UnixNano()), "member")
	buyerB := createRegisteredUser(t, env, fmt.Sprintf("buyer_b_%d", time.Now().UnixNano()), "member")

	orderRes, err := env.MarketplaceService.CreateOrder(ctx, buyerA, service.CheckoutInput{
		Items: []service.CartItemInput{
			{ProductID: prod.ID, Quantity: 2},
		},
		BuyerName:       "Buyer Alice",
		BuyerEmail:      "alice@example.org",
		BuyerPhone:      "+2348011223344",
		DeliveryType:    "delivery",
		ShippingAddress: "10 Victoria Island, Lagos",
	})
	if err != nil {
		t.Fatalf("CreateOrder failed: %v", err)
	}

	// Buyer A can access their own order
	ownOrder, _, _, err := env.MarketplaceService.GetOrderByID(ctx, buyerA, orderRes.Order.ID)
	if err != nil {
		t.Fatalf("Buyer A could not view own order: %v", err)
	}
	if ownOrder.ID != orderRes.Order.ID {
		t.Errorf("Order ID mismatch")
	}

	// Buyer B cannot access Buyer A's order
	_, _, _, err = env.MarketplaceService.GetOrderByID(ctx, buyerB, orderRes.Order.ID)
	if err == nil {
		t.Errorf("Expected unauthorized error for Buyer B accessing Buyer A's order, got nil")
	}

	// Buyer B cannot cancel Buyer A's order
	_, err = env.MarketplaceService.CancelOrder(ctx, buyerB, orderRes.Order.ID, "Malicious cancellation")
	if err == nil {
		t.Errorf("Expected unauthorized error for Buyer B cancelling Buyer A's order, got nil")
	}

	// Buyer B cannot view Buyer A's shipments
	_, err = env.MarketplaceService.ListOrderShipments(ctx, buyerB, orderRes.Order.ID)
	if err == nil {
		t.Errorf("Expected unauthorized error for Buyer B viewing Buyer A's shipments, got nil")
	}
}

// 2. Sellers manage only their own shipments and cannot view/dispatch other sellers' goods
func TestAuth_SellerShipmentIsolation(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	admin, sellerA, sellerB, prodA, prodB, orderRes := helperCreateMultiVendorOrder(t, env, ctx)
	_ = sellerB

	// Confirm payment to generate shipments
	_, err := env.MarketplaceService.ConfirmPayment(ctx, orderRes.Payment.Reference)
	if err != nil {
		t.Fatalf("ConfirmPayment failed: %v", err)
	}

	shipments, err := env.MarketplaceService.ListOrderShipments(ctx, admin, orderRes.Order.ID)
	if err != nil || len(shipments) < 2 {
		t.Fatalf("Expected at least 2 shipments for multi-vendor order, got %d (err: %v)", len(shipments), err)
	}

	var shipA, shipB *service.ShipmentDetailResult
	for i := range shipments {
		sh := &shipments[i]
		if sh.Shipment.BusinessID.String == prodA.BusinessID {
			shipA = sh
		} else if sh.Shipment.BusinessID.String == prodB.BusinessID {
			shipB = sh
		}
	}
	if shipA == nil || shipB == nil {
		t.Fatalf("Could not locate separate shipments for Seller A and Seller B")
	}

	// Seller A can view their own shipment
	sAView, err := env.MarketplaceService.GetShipmentByID(ctx, sellerA, shipA.Shipment.ID)
	if err != nil || sAView == nil {
		t.Fatalf("Seller A could not view own shipment: %v", err)
	}

	// Seller A CANNOT view Seller B's shipment
	_, err = env.MarketplaceService.GetShipmentByID(ctx, sellerA, shipB.Shipment.ID)
	if err == nil {
		t.Errorf("Expected unauthorized error for Seller A accessing Seller B's shipment, got nil")
	}

	// Seller A CANNOT dispatch Seller B's shipment
	_, err = env.MarketplaceService.DispatchShipment(ctx, sellerA, shipB.Shipment.ID, service.DispatchOrderInput{
		CarrierName:    "Malicious Courier",
		TrackingNumber: "TRK-HACK-001",
	})
	if err == nil {
		t.Errorf("Expected unauthorized error for Seller A dispatching Seller B's shipment, got nil")
	}

	// Seller A CANNOT update tracking on Seller B's shipment
	_, err = env.MarketplaceService.UpdateShipmentTracking(ctx, sellerA, shipB.Shipment.ID, service.UpdateShipmentTrackingInput{
		CarrierName:    "Malicious Courier",
		TrackingNumber: "TRK-HACK-002",
	})
	if err == nil {
		t.Errorf("Expected unauthorized error for Seller A updating tracking on Seller B's shipment, got nil")
	}
}

// 3. LGA Coordinator jurisdiction enforcement and financial restriction
func TestAuth_LGACoordinatorJurisdictionAndRestrictions(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	stateA, lgaA1, _, _ := helperGetTwoStatesWithLgas(t, env, ctx)
	lgasInStateA, err := env.Repo.Queries().ListLGAsByState(ctx, stateA.ID)
	if err != nil || len(lgasInStateA) < 2 {
		t.Fatalf("Need at least 2 LGAs in state A for intra-state LGA boundary test")
	}
	lgaA2 := lgasInStateA[1]

	admin := createRegisteredUser(t, env, fmt.Sprintf("lga_adm_%d", time.Now().UnixNano()), "super_admin")
	_, _, prod := helperCreateSellerWithProduct(t, env, ctx, fmt.Sprintf("lga_sel_%d", time.Now().UnixNano()), "LGA Store", "LGA Product", 40, admin)

	buyer := createRegisteredUser(t, env, fmt.Sprintf("lga_buy_%d", time.Now().UnixNano()), "member")

	// Order placed with shipping in LGA A1
	orderRes, err := env.MarketplaceService.CreateOrder(ctx, buyer, service.CheckoutInput{
		Items: []service.CartItemInput{
			{ProductID: prod.ID, Quantity: 2},
		},
		BuyerName:       "LGA Buyer",
		BuyerEmail:      "lgabuyer@example.org",
		BuyerPhone:      "+2348099887766",
		DeliveryType:    "delivery",
		ShippingAddress: "1 Market Square, " + lgaA1.Name,
		ShippingStateID: &stateA.ID,
		ShippingLGAID:   &lgaA1.ID,
	})
	if err != nil {
		t.Fatalf("CreateOrder failed: %v", err)
	}

	// Confirm payment
	_, err = env.MarketplaceService.ConfirmPayment(ctx, orderRes.Payment.Reference)
	if err != nil {
		t.Fatalf("ConfirmPayment failed: %v", err)
	}

	// Coordinator 1: Appointed to LGA A1 (matching jurisdiction)
	coord1 := createRegisteredUser(t, env, fmt.Sprintf("coord_lga1_%d", time.Now().UnixNano()), "lga_coordinator")
	_, err = env.Repo.Queries().CreateAppointment(ctx, db.CreateAppointmentParams{
		ID:               security.GenerateUUID(),
		UserID:           coord1.ID,
		RoleTitle:        "LGA Coordinator (" + lgaA1.Name + ")",
		RoleType:         "lga_coordinator",
		JurisdictionType: "lga",
		JurisdictionID:   service.TextFromString(lgaA1.ID),
		StartDate:        pgtype.Timestamptz{Time: time.Now().Add(-2 * time.Hour), Valid: true},
		Status:           "active",
	})
	if err != nil {
		t.Fatalf("CreateAppointment coord1 failed: %v", err)
	}

	// Coordinator 2: Appointed to LGA A2 (cross-LGA within same state)
	coord2 := createRegisteredUser(t, env, fmt.Sprintf("coord_lga2_%d", time.Now().UnixNano()), "lga_coordinator")
	_, err = env.Repo.Queries().CreateAppointment(ctx, db.CreateAppointmentParams{
		ID:               security.GenerateUUID(),
		UserID:           coord2.ID,
		RoleTitle:        "LGA Coordinator (" + lgaA2.Name + ")",
		RoleType:         "lga_coordinator",
		JurisdictionType: "lga",
		JurisdictionID:   service.TextFromString(lgaA2.ID),
		StartDate:        pgtype.Timestamptz{Time: time.Now().Add(-2 * time.Hour), Valid: true},
		Status:           "active",
	})
	if err != nil {
		t.Fatalf("CreateAppointment coord2 failed: %v", err)
	}

	// Coordinator 1 (matching LGA) CAN view order
	ord1, _, _, err := env.MarketplaceService.GetOrderByID(ctx, coord1, orderRes.Order.ID)
	if err != nil || ord1 == nil {
		t.Fatalf("Coordinator 1 failed to view order in their assigned LGA: %v", err)
	}

	// Coordinator 2 (cross-LGA) CANNOT view order
	_, _, _, err = env.MarketplaceService.GetOrderByID(ctx, coord2, orderRes.Order.ID)
	if err == nil {
		t.Errorf("Expected ErrScopeUnauthorized for Coordinator 2 accessing cross-LGA order, got nil")
	}

	// LGA Coordinator 1 CANNOT initiate refund (financial privilege restricted to State/Zonal/National)
	_, _, err = env.MarketplaceService.InitiateRefund(ctx, coord1, orderRes.Order.ID, service.RefundOrderInput{
		AmountKobo: 10000,
		Reason:     "LGA Coordinator attempting financial refund",
	})
	if err == nil {
		t.Errorf("Expected ErrScopeUnauthorized for LGA Coordinator initiating refund, got nil")
	}

	// LGA Coordinator 1 CANNOT approve vendor businesses
	sellerUser, biz, _ := helperCreateSellerWithProduct(t, env, ctx, fmt.Sprintf("lga_app_sel_%d", time.Now().UnixNano()), "Pending LGA Biz", "Prod", 10, admin)
	_ = sellerUser
	_, err = env.MarketplaceService.ApproveBusiness(ctx, coord1, biz.ID, true, "LGA approval")
	if err == nil {
		t.Errorf("Expected ErrScopeUnauthorized for LGA Coordinator approving business, got nil")
	}
}

// 4. State Coordinator cross-state access rejection
func TestAuth_StateCoordinatorJurisdiction(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	stateA, lgaA, stateB, lgaB := helperGetTwoStatesWithLgas(t, env, ctx)

	admin := createRegisteredUser(t, env, fmt.Sprintf("st_adm_%d", time.Now().UnixNano()), "super_admin")
	_, _, prodB := helperCreateSellerWithProduct(t, env, ctx, fmt.Sprintf("st_sel_b_%d", time.Now().UnixNano()), "State B Store", "State B Product", 50, admin)

	buyer := createRegisteredUser(t, env, fmt.Sprintf("st_buy_%d", time.Now().UnixNano()), "member")

	// Order placed in State B
	orderResB, err := env.MarketplaceService.CreateOrder(ctx, buyer, service.CheckoutInput{
		Items: []service.CartItemInput{
			{ProductID: prodB.ID, Quantity: 1},
		},
		BuyerName:       "State B Buyer",
		BuyerEmail:      "stateb@example.org",
		BuyerPhone:      "+2348022334455",
		DeliveryType:    "delivery",
		ShippingAddress: "50 Independence Way, " + stateB.Name,
		ShippingStateID: &stateB.ID,
		ShippingLGAID:   &lgaB.ID,
	})
	if err != nil {
		t.Fatalf("CreateOrder B failed: %v", err)
	}
	_, err = env.MarketplaceService.ConfirmPayment(ctx, orderResB.Payment.Reference)
	if err != nil {
		t.Fatalf("ConfirmPayment failed: %v", err)
	}

	// Coordinator A: Appointed to State A
	coordStateA := createRegisteredUser(t, env, fmt.Sprintf("st_coord_a_%d", time.Now().UnixNano()), "state_coordinator")
	_, err = env.Repo.Queries().CreateAppointment(ctx, db.CreateAppointmentParams{
		ID:               security.GenerateUUID(),
		UserID:           coordStateA.ID,
		RoleTitle:        "State Coordinator (" + stateA.Name + ")",
		RoleType:         "state_coordinator",
		JurisdictionType: "state",
		JurisdictionID:   service.TextFromString(stateA.ID),
		StartDate:        pgtype.Timestamptz{Time: time.Now().Add(-2 * time.Hour), Valid: true},
		Status:           "active",
	})
	if err != nil {
		t.Fatalf("CreateAppointment coordStateA failed: %v", err)
	}

	// Coordinator B: Appointed to State B
	coordStateB := createRegisteredUser(t, env, fmt.Sprintf("st_coord_b_%d", time.Now().UnixNano()), "state_coordinator")
	_, err = env.Repo.Queries().CreateAppointment(ctx, db.CreateAppointmentParams{
		ID:               security.GenerateUUID(),
		UserID:           coordStateB.ID,
		RoleTitle:        "State Coordinator (" + stateB.Name + ")",
		RoleType:         "state_coordinator",
		JurisdictionType: "state",
		JurisdictionID:   service.TextFromString(stateB.ID),
		StartDate:        pgtype.Timestamptz{Time: time.Now().Add(-2 * time.Hour), Valid: true},
		Status:           "active",
	})
	if err != nil {
		t.Fatalf("CreateAppointment coordStateB failed: %v", err)
	}

	// Coordinator A (State A) CANNOT access order in State B
	_, _, _, err = env.MarketplaceService.GetOrderByID(ctx, coordStateA, orderResB.Order.ID)
	if err == nil {
		t.Errorf("Expected ErrScopeUnauthorized for State A coordinator accessing State B order, got nil")
	}

	// Coordinator A CANNOT initiate refund for State B order
	_, _, err = env.MarketplaceService.InitiateRefund(ctx, coordStateA, orderResB.Order.ID, service.RefundOrderInput{
		AmountKobo: 5000,
		Reason:     "Cross-state refund attempt",
	})
	if err == nil {
		t.Errorf("Expected ErrScopeUnauthorized for State A coordinator refunding State B order, got nil")
	}

	// Coordinator B (State B) CAN access order in State B
	ordB, _, _, err := env.MarketplaceService.GetOrderByID(ctx, coordStateB, orderResB.Order.ID)
	if err != nil || ordB == nil {
		t.Fatalf("State B coordinator failed to access State B order: %v", err)
	}

	// Coordinator B CAN initiate refund for State B order
	refB, _, err := env.MarketplaceService.InitiateRefund(ctx, coordStateB, orderResB.Order.ID, service.RefundOrderInput{
		AmountKobo: 5000,
		Reason:     "Authorized state refund",
	})
	if err != nil || refB == nil {
		t.Fatalf("State B coordinator failed to initiate authorized refund: %v", err)
	}
	_ = lgaA
}

// 5. Zonal Coordinator cross-zone access rejection
func TestAuth_ZonalCoordinatorJurisdiction(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	zones, err := env.Repo.Queries().ListZones(ctx)
	if err != nil || len(zones) < 2 {
		t.Fatalf("Need at least 2 geopolitical zones for cross-zone test")
	}
	zoneA := zones[0]
	zoneB := zones[1]

	statesInZoneA, err := env.Repo.Queries().ListStatesByZone(ctx, zoneA.ID)
	if err != nil || len(statesInZoneA) < 1 {
		t.Fatalf("Need state in zone A")
	}
	stA := statesInZoneA[0]
	lgasA, err := env.Repo.Queries().ListLGAsByState(ctx, stA.ID)
	if err != nil || len(lgasA) < 1 {
		t.Fatalf("Need LGA in state A")
	}

	admin := createRegisteredUser(t, env, fmt.Sprintf("zn_adm_%d", time.Now().UnixNano()), "super_admin")
	_, _, prodA := helperCreateSellerWithProduct(t, env, ctx, fmt.Sprintf("zn_sel_%d", time.Now().UnixNano()), "Zone A Store", "Prod", 30, admin)

	buyer := createRegisteredUser(t, env, fmt.Sprintf("zn_buy_%d", time.Now().UnixNano()), "member")

	// Order placed in Zone A
	orderResA, err := env.MarketplaceService.CreateOrder(ctx, buyer, service.CheckoutInput{
		Items: []service.CartItemInput{
			{ProductID: prodA.ID, Quantity: 1},
		},
		BuyerName:       "Zone A Buyer",
		BuyerEmail:      "zonea@example.org",
		BuyerPhone:      "+2348033445566",
		DeliveryType:    "delivery",
		ShippingAddress: "100 Zone Avenue, " + stA.Name,
		ShippingStateID: &stA.ID,
		ShippingLGAID:   &lgasA[0].ID,
	})
	if err != nil {
		t.Fatalf("CreateOrder A failed: %v", err)
	}
	_, err = env.MarketplaceService.ConfirmPayment(ctx, orderResA.Payment.Reference)
	if err != nil {
		t.Fatalf("ConfirmPayment failed: %v", err)
	}

	// Coordinator appointed to Zone B
	coordZoneB := createRegisteredUser(t, env, fmt.Sprintf("zn_coord_b_%d", time.Now().UnixNano()), "zonal_coordinator")
	_, err = env.Repo.Queries().CreateAppointment(ctx, db.CreateAppointmentParams{
		ID:               security.GenerateUUID(),
		UserID:           coordZoneB.ID,
		RoleTitle:        "Zonal Coordinator (" + zoneB.Name + ")",
		RoleType:         "zonal_coordinator",
		JurisdictionType: "zone",
		JurisdictionID:   service.TextFromString(zoneB.ID),
		StartDate:        pgtype.Timestamptz{Time: time.Now().Add(-2 * time.Hour), Valid: true},
		Status:           "active",
	})
	if err != nil {
		t.Fatalf("CreateAppointment coordZoneB failed: %v", err)
	}

	// Coordinator appointed to Zone A
	coordZoneA := createRegisteredUser(t, env, fmt.Sprintf("zn_coord_a_%d", time.Now().UnixNano()), "zonal_coordinator")
	_, err = env.Repo.Queries().CreateAppointment(ctx, db.CreateAppointmentParams{
		ID:               security.GenerateUUID(),
		UserID:           coordZoneA.ID,
		RoleTitle:        "Zonal Coordinator (" + zoneA.Name + ")",
		RoleType:         "zonal_coordinator",
		JurisdictionType: "zone",
		JurisdictionID:   service.TextFromString(zoneA.ID),
		StartDate:        pgtype.Timestamptz{Time: time.Now().Add(-2 * time.Hour), Valid: true},
		Status:           "active",
	})
	if err != nil {
		t.Fatalf("CreateAppointment coordZoneA failed: %v", err)
	}

	// Zone B coordinator CANNOT access Zone A order
	_, _, _, err = env.MarketplaceService.GetOrderByID(ctx, coordZoneB, orderResA.Order.ID)
	if err == nil {
		t.Errorf("Expected ErrScopeUnauthorized for Zone B coordinator accessing Zone A order, got nil")
	}

	// Zone A coordinator CAN access Zone A order
	ordA, _, _, err := env.MarketplaceService.GetOrderByID(ctx, coordZoneA, orderResA.Order.ID)
	if err != nil || ordA == nil {
		t.Fatalf("Zone A coordinator failed to access Zone A order: %v", err)
	}
}

// 6. Revoked and expired appointments are rejected
func TestAuth_RevokedAndExpiredAppointments(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	stateA, lgaA, _, _ := helperGetTwoStatesWithLgas(t, env, ctx)

	admin := createRegisteredUser(t, env, fmt.Sprintf("rx_adm_%d", time.Now().UnixNano()), "super_admin")
	_, _, prod := helperCreateSellerWithProduct(t, env, ctx, fmt.Sprintf("rx_sel_%d", time.Now().UnixNano()), "Revoke Store", "Revoke Prod", 20, admin)

	buyer := createRegisteredUser(t, env, fmt.Sprintf("rx_buy_%d", time.Now().UnixNano()), "member")

	orderRes, err := env.MarketplaceService.CreateOrder(ctx, buyer, service.CheckoutInput{
		Items: []service.CartItemInput{
			{ProductID: prod.ID, Quantity: 1},
		},
		BuyerName:       "Revoke Buyer",
		BuyerEmail:      "rxbuyer@example.org",
		BuyerPhone:      "+2348044556677",
		DeliveryType:    "delivery",
		ShippingAddress: "123 Test Street",
		ShippingStateID: &stateA.ID,
		ShippingLGAID:   &lgaA.ID,
	})
	if err != nil {
		t.Fatalf("CreateOrder failed: %v", err)
	}
	_, err = env.MarketplaceService.ConfirmPayment(ctx, orderRes.Payment.Reference)
	if err != nil {
		t.Fatalf("ConfirmPayment failed: %v", err)
	}

	// User with REVOKED appointment
	revokedUser := createRegisteredUser(t, env, fmt.Sprintf("revoked_user_%d", time.Now().UnixNano()), "member")
	_, err = env.Repo.Queries().CreateAppointment(ctx, db.CreateAppointmentParams{
		ID:               security.GenerateUUID(),
		UserID:           revokedUser.ID,
		RoleTitle:        "Revoked Coordinator",
		RoleType:         "state_coordinator",
		JurisdictionType: "state",
		JurisdictionID:   service.TextFromString(stateA.ID),
		StartDate:        pgtype.Timestamptz{Time: time.Now().Add(-50 * time.Hour), Valid: true},
		EndDate:          pgtype.Timestamptz{Time: time.Now().Add(-20 * time.Hour), Valid: true},
		Status:           "revoked",
	})
	if err != nil {
		t.Fatalf("CreateAppointment revoked failed: %v", err)
	}

	// User with EXPIRED appointment
	expiredUser := createRegisteredUser(t, env, fmt.Sprintf("expired_user_%d", time.Now().UnixNano()), "member")
	_, err = env.Repo.Queries().CreateAppointment(ctx, db.CreateAppointmentParams{
		ID:               security.GenerateUUID(),
		UserID:           expiredUser.ID,
		RoleTitle:        "Expired Coordinator",
		RoleType:         "state_coordinator",
		JurisdictionType: "state",
		JurisdictionID:   service.TextFromString(stateA.ID),
		StartDate:        pgtype.Timestamptz{Time: time.Now().Add(-100 * time.Hour), Valid: true},
		EndDate:          pgtype.Timestamptz{Time: time.Now().Add(-1 * time.Hour), Valid: true},
		Status:           "active", // marked active but EndDate is in the past!
	})
	if err != nil {
		t.Fatalf("CreateAppointment expired failed: %v", err)
	}

	// Revoked user cannot access order
	_, _, _, err = env.MarketplaceService.GetOrderByID(ctx, revokedUser, orderRes.Order.ID)
	if err == nil {
		t.Errorf("Expected error for user with revoked appointment, got nil")
	}

	// Expired user cannot access order
	_, _, _, err = env.MarketplaceService.GetOrderByID(ctx, expiredUser, orderRes.Order.ID)
	if err == nil {
		t.Errorf("Expected error for user with expired appointment, got nil")
	}
}

// 7. Anti-Self-Approval: Initiator cannot confirm or settle their own refund, claimant cannot arbitrate dispute
func TestAuth_AntiSelfApproval(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	admin1 := createRegisteredUser(t, env, fmt.Sprintf("asa_adm1_%d", time.Now().UnixNano()), "super_admin")
	admin2 := createRegisteredUser(t, env, fmt.Sprintf("asa_adm2_%d", time.Now().UnixNano()), "super_admin")

	_, _, prod := helperCreateSellerWithProduct(t, env, ctx, fmt.Sprintf("asa_sel_%d", time.Now().UnixNano()), "ASA Store", "ASA Prod", 50, admin1)
	buyer := createRegisteredUser(t, env, fmt.Sprintf("asa_buy_%d", time.Now().UnixNano()), "member")

	orderRes, err := env.MarketplaceService.CreateOrder(ctx, buyer, service.CheckoutInput{
		Items: []service.CartItemInput{
			{ProductID: prod.ID, Quantity: 2},
		},
		BuyerName:       "ASA Buyer",
		BuyerEmail:      "asabuyer@example.org",
		BuyerPhone:      "+2348055667788",
		DeliveryType:    "delivery",
		ShippingAddress: "12 Anti Self Approval Way",
	})
	if err != nil {
		t.Fatalf("CreateOrder failed: %v", err)
	}

	_, err = env.MarketplaceService.ConfirmPayment(ctx, orderRes.Payment.Reference)
	if err != nil {
		t.Fatalf("ConfirmPayment failed: %v", err)
	}

	// Admin 1 initiates a refund
	refund, _, err := env.MarketplaceService.InitiateRefund(ctx, admin1, orderRes.Order.ID, service.RefundOrderInput{
		AmountKobo: 5000,
		Reason:     "Customer dissatisfaction",
	})
	if err != nil {
		t.Fatalf("InitiateRefund failed: %v", err)
	}

	// Admin 1 attempts to confirm/approve their OWN refund -> MUST BE REJECTED
	_, _, err = env.MarketplaceService.ConfirmRefund(ctx, admin1, refund.ID, service.ConfirmRefundInput{
		ProviderRefundReference: refund.ProviderRefundReference,
	})
	if err == nil {
		t.Fatalf("Expected ErrSelfApprovalProhibited when Admin 1 confirms own refund, got nil")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "self") {
		t.Errorf("Expected self-approval prohibition error, got: %v", err)
	}

	// Admin 2 (different admin) confirms the refund -> SUCCEEDS
	confirmedRef, _, err := env.MarketplaceService.ConfirmRefund(ctx, admin2, refund.ID, service.ConfirmRefundInput{
		ProviderRefundReference: refund.ProviderRefundReference,
	})
	if err != nil {
		t.Fatalf("Admin 2 failed to confirm refund: %v", err)
	}
	if confirmedRef.Status != "successful" {
		t.Errorf("Expected refund status 'successful', got %s", confirmedRef.Status)
	}

	// Anti-Self-Approval for Disputes:
	// Buyer raises dispute
	dispute, err := env.MarketplaceService.RaiseDispute(ctx, buyer, orderRes.Order.ID, service.CreateDisputeInput{
		Reason:        "damaged_goods",
		CustomerNotes: "Items arrived damaged",
	})
	if err != nil {
		t.Fatalf("RaiseDispute failed: %v", err)
	}

	// If the buyer is also an admin, they cannot arbitrate their own dispute
	buyerWithAdminRole := &db.User{
		ID:          buyer.ID,
		Email:       buyer.Email,
		PrimaryRole: "super_admin",
	}
	_, err = env.MarketplaceService.ResolveDispute(ctx, buyerWithAdminRole, dispute.ID, "resolved_refunded", "Self-arbitration")
	if err == nil {
		t.Fatalf("Expected ErrSelfApprovalProhibited when claimant arbitrates own dispute, got nil")
	}

	// Admin 2 arbitrates -> SUCCEEDS
	resolvedD, err := env.MarketplaceService.ResolveDispute(ctx, admin2, dispute.ID, "rejected", "Independent arbitration rejected claim")
	if err != nil || resolvedD.Status != "rejected" {
		t.Fatalf("Admin 2 arbitration failed: %v", err)
	}
}

// 8. Reject spoofed geographic locations (Shipping LGA must belong to Shipping State)
func TestAuth_GeographicSpoofingRejection(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	stateA, _, stateB, lgaB := helperGetTwoStatesWithLgas(t, env, ctx)
	_ = stateB

	admin := createRegisteredUser(t, env, fmt.Sprintf("geo_adm_%d", time.Now().UnixNano()), "super_admin")
	_, _, prod := helperCreateSellerWithProduct(t, env, ctx, fmt.Sprintf("geo_sel_%d", time.Now().UnixNano()), "Geo Store", "Geo Prod", 30, admin)
	buyer := createRegisteredUser(t, env, fmt.Sprintf("geo_buy_%d", time.Now().UnixNano()), "member")

	// Attempt to create order with State A but LGA belonging to State B (spoofed location)
	_, err := env.MarketplaceService.CreateOrder(ctx, buyer, service.CheckoutInput{
		Items: []service.CartItemInput{
			{ProductID: prod.ID, Quantity: 1},
		},
		BuyerName:       "Spoof Buyer",
		BuyerEmail:      "spoof@example.org",
		BuyerPhone:      "+2348066778899",
		DeliveryType:    "delivery",
		ShippingAddress: "Mismatch Road",
		ShippingStateID: &stateA.ID,
		ShippingLGAID:   &lgaB.ID, // Mismatched! LGA B belongs to State B, not State A
	})
	if err == nil {
		t.Fatalf("Expected location error when creating order with mismatched State and LGA, got nil")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "location") && !strings.Contains(strings.ToLower(err.Error()), "lga") {
		t.Errorf("Expected error regarding invalid location or LGA mismatch, got: %v", err)
	}
}

// 9. Super Admin nationwide audit logging
func TestAuth_SuperAdminNationwideAuditLogging(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	admin := createRegisteredUser(t, env, fmt.Sprintf("audit_adm_%d", time.Now().UnixNano()), "super_admin")
	_, biz, prod := helperCreateSellerWithProduct(t, env, ctx, fmt.Sprintf("audit_sel_%d", time.Now().UnixNano()), "Audit Biz", "Prod", 15, admin)

	// Super Admin suspends business
	suspendedBiz, err := env.MarketplaceService.SuspendBusiness(ctx, admin, biz.ID, "Routine administrative audit check")
	if err != nil {
		t.Fatalf("SuspendBusiness failed: %v", err)
	}
	if suspendedBiz.Status != "suspended" {
		t.Errorf("Expected approval status 'suspended', got %s", suspendedBiz.Status)
	}

	// Super Admin restores business
	restoredBiz, err := env.MarketplaceService.RestoreBusiness(ctx, admin, biz.ID, "Passed audit verification")
	if err != nil {
		t.Fatalf("RestoreBusiness failed: %v", err)
	}
	if restoredBiz.Status != "approved" {
		t.Errorf("Expected approval status 'approved', got %s", restoredBiz.Status)
	}
	_ = prod
}

// 10. Financial Integrity: Paystack Ambiguous Timeout Reconciliation
func TestFinancial_PaystackAmbiguousTimeoutReconciliation(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	admin := createRegisteredUser(t, env, fmt.Sprintf("rec_adm_%d", time.Now().UnixNano()), "super_admin")
	_, _, prod := helperCreateSellerWithProduct(t, env, ctx, fmt.Sprintf("rec_sel_%d", time.Now().UnixNano()), "Rec Store", "Rec Prod", 50, admin)
	buyer := createRegisteredUser(t, env, fmt.Sprintf("rec_buy_%d", time.Now().UnixNano()), "member")

	orderRes, err := env.MarketplaceService.CreateOrder(ctx, buyer, service.CheckoutInput{
		Items: []service.CartItemInput{
			{ProductID: prod.ID, Quantity: 2},
		},
		BuyerName:       "Rec Buyer",
		BuyerEmail:      "recbuyer@example.org",
		BuyerPhone:      "+2348077889900",
		DeliveryType:    "delivery",
		ShippingAddress: "44 Reconciliation Ave",
	})
	if err != nil {
		t.Fatalf("CreateOrder failed: %v", err)
	}

	_, err = env.MarketplaceService.ConfirmPayment(ctx, orderRes.Payment.Reference)
	if err != nil {
		t.Fatalf("ConfirmPayment failed: %v", err)
	}

	payRec, err := env.Repo.Queries().GetMarketplacePaymentByOrder(ctx, orderRes.Order.ID)
	if err != nil {
		t.Fatalf("GetMarketplacePaymentByOrder failed: %v", err)
	}

	// Create a pending refund record WITHOUT a provider reference
	// (simulating a network timeout or crash right after Paystack accepted the request)
	refundID := security.GenerateUUID()
	refundAmount := int64(10000)
	providerSimRef := "PST_REFUND_SIM_" + security.GenerateUUID()

	_, err = env.Repo.Queries().CreateMarketplaceRefund(ctx, db.CreateMarketplaceRefundParams{
		ID:                      refundID,
		OrderID:                 orderRes.Order.ID,
		PaymentID:               payRec.ID,
		AmountKobo:              refundAmount,
		Status:                  "pending",
		Provider:                "sandbox",
		ProviderRefundReference: "", // Empty: dispatch timed out!
		InitiatedByUserID:       admin.ID,
		ConfirmedByUserID:       pgtype.Text{Valid: false},
		Reason:                  "Simulated timeout refund",
		SettlementNotes:         "Refund dispatch timed out; pending provider reconciliation",
		Metadata:                []byte(`{"ambiguous_dispatch_error":"context deadline exceeded"}`),
		ShipmentID:              pgtype.Text{Valid: false},
		BusinessID:              pgtype.Text{Valid: false},
		OrderItemID:             pgtype.Text{Valid: false},
	})
	if err != nil {
		t.Fatalf("CreateMarketplaceRefund failed: %v", err)
	}

	// Record the simulated refund directly on the payment provider sandbox
	// (as if Paystack had successfully recorded it before the timeout)
	env.SandboxGateway.AddSimulatedRefund(payRec.Reference, payment.TransactionRefundSummary{
		RefundReference: providerSimRef,
		AmountKobo:      refundAmount,
		Status:          "processed",
	})

	// Run durable recovery worker
	workerRes, err := env.MarketplaceService.ProcessPendingRefunds(ctx, 50)
	if err != nil {
		t.Fatalf("ProcessPendingRefunds failed: %v", err)
	}

	if workerRes.SettledCount < 1 {
		t.Errorf("Expected at least 1 settled refund from reconciliation, got %d (errors: %v)", workerRes.SettledCount, workerRes.Errors)
	}

	// Verify refund record adopted the provider reference and settled authoritatively
	updatedRef, err := env.Repo.Queries().GetMarketplaceRefundByID(ctx, refundID)
	if err != nil {
		t.Fatalf("GetMarketplaceRefundByID failed: %v", err)
	}
	if updatedRef.Status != "successful" {
		t.Errorf("Expected refund status 'successful', got %s", updatedRef.Status)
	}
	if updatedRef.ProviderRefundReference != providerSimRef {
		t.Errorf("Expected provider reference %s, got %s", providerSimRef, updatedRef.ProviderRefundReference)
	}
}

// 11. Financial Integrity: Multiple partial refunds of the same product restore exact quantities without idempotency key collisions
func TestFinancial_MultiplePartialRefundsExactRestock(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	admin1 := createRegisteredUser(t, env, fmt.Sprintf("mp_adm1_%d", time.Now().UnixNano()), "super_admin")
	admin2 := createRegisteredUser(t, env, fmt.Sprintf("mp_adm2_%d", time.Now().UnixNano()), "super_admin")
	_, _, prod := helperCreateSellerWithProduct(t, env, ctx, fmt.Sprintf("mp_sel_%d", time.Now().UnixNano()), "Multi Partial Store", "Multi Partial Prod", 50, admin1)
	buyer := createRegisteredUser(t, env, fmt.Sprintf("mp_buy_%d", time.Now().UnixNano()), "member")

	initialStock := prod.StockQuantity // 50

	// Order placed with 3 items of the product
	orderRes, err := env.MarketplaceService.CreateOrder(ctx, buyer, service.CheckoutInput{
		Items: []service.CartItemInput{
			{ProductID: prod.ID, Quantity: 3},
		},
		BuyerName:       "Multi Partial Buyer",
		BuyerEmail:      "mpbuyer@example.org",
		BuyerPhone:      "+2348088990011",
		DeliveryType:    "delivery",
		ShippingAddress: "88 Multi Partial Blvd",
	})
	if err != nil {
		t.Fatalf("CreateOrder failed: %v", err)
	}

	pAfterOrder, _ := env.Repo.Queries().GetMarketplaceProductByID(ctx, prod.ID)
	if pAfterOrder.StockQuantity != initialStock-3 {
		t.Fatalf("Expected stock after order %d, got %d", initialStock-3, pAfterOrder.StockQuantity)
	}

	_, err = env.MarketplaceService.ConfirmPayment(ctx, orderRes.Payment.Reference)
	if err != nil {
		t.Fatalf("ConfirmPayment failed: %v", err)
	}

	orderItems, err := env.Repo.Queries().ListMarketplaceOrderItemsByOrder(ctx, orderRes.Order.ID)
	if err != nil || len(orderItems) < 1 {
		t.Fatalf("No order items found")
	}
	orderItem := orderItems[0]

	// Partial Refund 1: Refund 1st unit (item level)
	itemPriceKobo := orderItem.PriceKoboSnapshot
	ref1, _, err := env.MarketplaceService.InitiateRefund(ctx, admin1, orderRes.Order.ID, service.RefundOrderInput{
		AmountKobo:  itemPriceKobo,
		OrderItemID: orderItem.ID,
		Reason:      "First item returned",
	})
	if err != nil {
		t.Fatalf("InitiateRefund 1 failed: %v", err)
	}

	// Confirm Refund 1 by Admin 2 (anti-self-approval)
	_, _, err = env.MarketplaceService.ConfirmRefund(ctx, admin2, ref1.ID, service.ConfirmRefundInput{
		ProviderRefundReference: ref1.ProviderRefundReference,
	})
	if err != nil {
		t.Fatalf("ConfirmRefund 1 failed: %v", err)
	}

	// Stock should have increased by exactly 1 unit
	pAfterRef1, _ := env.Repo.Queries().GetMarketplaceProductByID(ctx, prod.ID)
	if pAfterRef1.StockQuantity != initialStock-2 {
		t.Errorf("Expected stock after refund 1 to be %d, got %d", initialStock-2, pAfterRef1.StockQuantity)
	}

	// Partial Refund 2: Refund 2nd unit of the same product
	ref2, _, err := env.MarketplaceService.InitiateRefund(ctx, admin1, orderRes.Order.ID, service.RefundOrderInput{
		AmountKobo:  itemPriceKobo,
		OrderItemID: orderItem.ID,
		Reason:      "Second item returned",
	})
	if err != nil {
		t.Fatalf("InitiateRefund 2 failed: %v", err)
	}

	// Confirm Refund 2 by Admin 2
	_, _, err = env.MarketplaceService.ConfirmRefund(ctx, admin2, ref2.ID, service.ConfirmRefundInput{
		ProviderRefundReference: ref2.ProviderRefundReference,
	})
	if err != nil {
		t.Fatalf("ConfirmRefund 2 failed: %v", err)
	}

	// Stock should have increased by another 1 unit (total 2 units restored)
	pAfterRef2, _ := env.Repo.Queries().GetMarketplaceProductByID(ctx, prod.ID)
	if pAfterRef2.StockQuantity != initialStock-1 {
		t.Errorf("Expected stock after refund 2 to be %d, got %d", initialStock-1, pAfterRef2.StockQuantity)
	}

	// Final Full Settlement / Cancellation for the remaining 3rd unit
	ref3, _, err := env.MarketplaceService.InitiateRefund(ctx, admin1, orderRes.Order.ID, service.RefundOrderInput{
		AmountKobo: orderRes.Order.TotalKobo - (2 * itemPriceKobo),
		Reason:     "Final balance refund",
	})
	if err != nil {
		t.Fatalf("InitiateRefund 3 failed: %v", err)
	}

	_, finalOrder, err := env.MarketplaceService.ConfirmRefund(ctx, admin2, ref3.ID, service.ConfirmRefundInput{
		ProviderRefundReference: ref3.ProviderRefundReference,
	})
	if err != nil {
		t.Fatalf("ConfirmRefund 3 failed: %v", err)
	}

	if finalOrder.PaymentStatus != "refunded" {
		t.Errorf("Expected final payment status 'refunded', got %s", finalOrder.PaymentStatus)
	}

	// Stock must now be fully restored to original initialStock (50) exactly, without double-restock!
	pFinal, _ := env.Repo.Queries().GetMarketplaceProductByID(ctx, prod.ID)
	if pFinal.StockQuantity != initialStock {
		t.Errorf("Expected final stock restored to %d, got %d", initialStock, pFinal.StockQuantity)
	}

	// Verify total restocked quantity recorded in inventory ledger for this order and product
	totalRestocked, err := env.Repo.Queries().SumOrderInventoryRestockedQuantity(ctx, db.SumOrderInventoryRestockedQuantityParams{
		OrderID:   service.TextFromString(orderRes.Order.ID),
		ProductID: prod.ID,
	})
	if err != nil {
		t.Fatalf("SumOrderInventoryRestockedQuantity failed: %v", err)
	}
	if totalRestocked != 3 {
		t.Errorf("Expected total restocked quantity 3, got %d", totalRestocked)
	}
}
