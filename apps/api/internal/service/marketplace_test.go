package service_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"ratels-api/internal/service"
)

func TestMarketplaceService_VendorRegistrationAndApproval(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	sellerUser := createRegisteredUser(t, env, "vendor_reg", "member")
	adminUser := createRegisteredUser(t, env, "admin_approver", "super_admin")

	// 1. Register business profile
	biz, err := env.MarketplaceService.RegisterBusiness(ctx, sellerUser, service.RegisterBusinessInput{
		Name:          "Kano Agro Clean Supplies",
		Description:   "Community-driven organic sanitation supplies and tools",
		Category:      "tools_hardware",
		OwnershipType: "member_owned",
		ContactEmail:  "kano_agro@example.org",
		ContactPhone:  "+2348033445566",
	})
	if err != nil {
		t.Fatalf("Failed to register business: %v", err)
	}
	if biz.Status != "pending_review" {
		t.Errorf("Expected pending_review status, got %s", biz.Status)
	}

	// 2. Reject duplicate registration by same user
	_, err = env.MarketplaceService.RegisterBusiness(ctx, sellerUser, service.RegisterBusinessInput{
		Name:         "Duplicate Store",
		ContactEmail: "dup@example.org",
		ContactPhone: "+2348000000000",
	})
	if !errors.Is(err, service.ErrBusinessAlreadyExists) {
		t.Fatalf("Expected ErrBusinessAlreadyExists, got %v", err)
	}

	// 3. Admin approves business
	approvedBiz, err := env.MarketplaceService.ApproveBusiness(ctx, adminUser, biz.ID, true, "")
	if err != nil {
		t.Fatalf("Admin failed to approve business: %v", err)
	}
	if approvedBiz.Status != "approved" || !approvedBiz.IsVerified {
		t.Errorf("Expected approved & verified, got status=%s, verified=%v", approvedBiz.Status, approvedBiz.IsVerified)
	}
}

func TestMarketplaceService_ProductCreationAndMultiSellerIsolation(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	seller1 := createRegisteredUser(t, env, "seller_one", "member")
	seller2 := createRegisteredUser(t, env, "seller_two", "member")
	admin := createRegisteredUser(t, env, "admin_moderator", "super_admin")

	// Register & approve business 1
	b1, err := env.MarketplaceService.RegisterBusiness(ctx, seller1, service.RegisterBusinessInput{
		Name:          "Abuja Green Wares",
		Category:      "sanitation_supplies",
		OwnershipType: "member_owned",
		ContactEmail:  "green1@example.org",
		ContactPhone:  "+2348011111111",
	})
	if err != nil {
		t.Fatalf("Failed to create biz 1: %v", err)
	}
	_, _ = env.MarketplaceService.ApproveBusiness(ctx, admin, b1.ID, true, "")

	// Register & approve business 2
	b2, err := env.MarketplaceService.RegisterBusiness(ctx, seller2, service.RegisterBusinessInput{
		Name:          "Lagos Safety Wear",
		Category:      "safety_gear",
		OwnershipType: "member_owned",
		ContactEmail:  "safety2@example.org",
		ContactPhone:  "+2348022222222",
	})
	if err != nil {
		t.Fatalf("Failed to create biz 2: %v", err)
	}
	_, _ = env.MarketplaceService.ApproveBusiness(ctx, admin, b2.ID, true, "")

	// Seller 1 creates Product 1
	p1, err := env.MarketplaceService.CreateProduct(ctx, seller1, service.CreateProductInput{
		BusinessID:    b1.ID,
		Title:         "Heavy Duty Sanitation Shovel",
		Description:   "Hardened steel blade",
		Category:      "tools_hardware",
		SKU:           fmt.Sprintf("SKU-SHOV-%d", time.Now().UnixNano()),
		PriceKobo:     450000, // ₦4,500.00
		StockQuantity: 50,
		Status:        "published",
	})
	if err != nil {
		t.Fatalf("Failed to create product 1: %v", err)
	}
	_ = p1

	// Seller 2 creates Product 2
	p2, err := env.MarketplaceService.CreateProduct(ctx, seller2, service.CreateProductInput{
		BusinessID:    b2.ID,
		Title:         "High Visibility Reflective Vest",
		Description:   "Class 2 reflective safety vest",
		Category:      "safety_gear",
		SKU:           fmt.Sprintf("SKU-VEST-%d", time.Now().UnixNano()),
		PriceKobo:     300000, // ₦3,000.00
		StockQuantity: 30,
		Status:        "published",
	})
	if err != nil {
		t.Fatalf("Failed to create product 2: %v", err)
	}

	// Security Test: Seller 1 attempts to modify Seller 2's product -> MUST FAIL
	_, err = env.MarketplaceService.UpdateProduct(ctx, seller1, p2.ID, service.UpdateProductInput{
		Title:         "Hacked Product Title",
		Category:      p2.Category,
		SKU:           p2.Sku,
		PriceKobo:     100000,
		StockQuantity: 10,
		Status:        "published",
	})
	if !errors.Is(err, service.ErrUnauthorizedSellerAction) {
		t.Fatalf("Multi-seller isolation violated! Expected ErrUnauthorizedSellerAction, got %v", err)
	}

	// Security Test: Seller 1 attempts to delete Seller 2's product -> MUST FAIL
	err = env.MarketplaceService.DeleteProduct(ctx, seller1, p2.ID)
	if !errors.Is(err, service.ErrUnauthorizedSellerAction) {
		t.Fatalf("Multi-seller isolation violated! Expected ErrUnauthorizedSellerAction on delete, got %v", err)
	}

	// Legitimate update: Seller 2 updates Product 2 -> MUST SUCCEED
	updatedP2, err := env.MarketplaceService.UpdateProduct(ctx, seller2, p2.ID, service.UpdateProductInput{
		Title:         "High Visibility Reflective Vest - XL",
		Category:      p2.Category,
		SKU:           p2.Sku,
		PriceKobo:     350000, // ₦3,500.00
		StockQuantity: 25,
		Status:        "published",
	})
	if err != nil {
		t.Fatalf("Legitimate product update failed: %v", err)
	}
	if updatedP2.PriceKobo != 350000 || updatedP2.Title != "High Visibility Reflective Vest - XL" {
		t.Errorf("Unexpected updated product values: %+v", updatedP2)
	}
}

func TestMarketplaceService_CartCalculationsAndIntegrity(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	seller := createRegisteredUser(t, env, "cart_seller", "member")
	admin := createRegisteredUser(t, env, "cart_admin", "super_admin")

	b, _ := env.MarketplaceService.RegisterBusiness(ctx, seller, service.RegisterBusinessInput{
		Name:          "Kaduna Tools Depot",
		Category:      "tools_hardware",
		OwnershipType: "member_owned",
		ContactEmail:  "tools_kd@example.org",
		ContactPhone:  "+2348012345678",
	})
	_, _ = env.MarketplaceService.ApproveBusiness(ctx, admin, b.ID, true, "")

	p1, _ := env.MarketplaceService.CreateProduct(ctx, seller, service.CreateProductInput{
		BusinessID:    b.ID,
		Title:         "Safety Goggles",
		Category:      "safety_gear",
		SKU:           fmt.Sprintf("SKU-GOGG-%d", time.Now().UnixNano()),
		PriceKobo:     120000, // ₦1,200
		StockQuantity: 10,
		Status:        "published",
	})

	p2, _ := env.MarketplaceService.CreateProduct(ctx, seller, service.CreateProductInput{
		BusinessID:    b.ID,
		Title:         "Rubber Work Gloves",
		Category:      "safety_gear",
		SKU:           fmt.Sprintf("SKU-GLOV-%d", time.Now().UnixNano()),
		PriceKobo:     80000, // ₦800
		StockQuantity: 5,
		Status:        "published",
	})

	// Validate cart with 2 goggles (2 * 120000 = 240000) and 3 gloves (3 * 80000 = 240000)
	// Subtotal = 480000 kobo (₦4,800)
	// Standard delivery fee = 150000 kobo (₦1,500)
	// Total = 630000 kobo (₦6,300)
	cartRes, err := env.MarketplaceService.ValidateCart(ctx, []service.CartItemInput{
		{ProductID: p1.ID, Quantity: 2},
		{ProductID: p2.ID, Quantity: 3},
	})
	if err != nil {
		t.Fatalf("Cart validation failed: %v", err)
	}
	if !cartRes.IsValid {
		t.Fatalf("Expected cart to be valid, got false")
	}
	if cartRes.SubtotalKobo != 480000 {
		t.Errorf("Expected subtotal 480000 kobo, got %d", cartRes.SubtotalKobo)
	}
	if cartRes.TotalKobo != 630000 {
		t.Errorf("Expected total 630000 kobo, got %d", cartRes.TotalKobo)
	}

	// Requesting more than available stock should fail validation
	invalidCartRes, err := env.MarketplaceService.ValidateCart(ctx, []service.CartItemInput{
		{ProductID: p2.ID, Quantity: 10}, // Only 5 available
	})
	if err != nil {
		t.Fatalf("Validation error: %v", err)
	}
	if invalidCartRes.IsValid {
		t.Errorf("Expected cart to be invalid due to stock deficit")
	}
}

func TestMarketplaceService_CheckoutStockReservationAndConcurrency(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	seller := createRegisteredUser(t, env, "conc_seller", "member")
	admin := createRegisteredUser(t, env, "conc_admin", "super_admin")
	buyer1 := createRegisteredUser(t, env, "conc_buyer1", "member")
	buyer2 := createRegisteredUser(t, env, "conc_buyer2", "member")

	b, _ := env.MarketplaceService.RegisterBusiness(ctx, seller, service.RegisterBusinessInput{
		Name:          "Enugu Sanitation Store",
		Category:      "tools_hardware",
		OwnershipType: "member_owned",
		ContactEmail:  "enugu@example.org",
		ContactPhone:  "+2348055667788",
	})
	_, _ = env.MarketplaceService.ApproveBusiness(ctx, admin, b.ID, true, "")

	// Exactly 10 units in stock
	p, err := env.MarketplaceService.CreateProduct(ctx, seller, service.CreateProductInput{
		BusinessID:    b.ID,
		Title:         "High-Capacity Plastic Dustbin",
		Category:      "tools_hardware",
		SKU:           fmt.Sprintf("SKU-BIN-%d", time.Now().UnixNano()),
		PriceKobo:     500000, // ₦5,000
		StockQuantity: 10,
		Status:        "published",
	})
	if err != nil {
		t.Fatalf("Failed to create product: %v", err)
	}

	// First buyer checks out 7 units -> stock should become 3
	order1, err := env.MarketplaceService.CreateOrder(ctx, buyer1, service.CheckoutInput{
		Items:           []service.CartItemInput{{ProductID: p.ID, Quantity: 7}},
		BuyerName:       "Buyer One",
		BuyerEmail:      "buyer1@example.org",
		BuyerPhone:      "+2348011112222",
		DeliveryType:    "delivery",
		ShippingAddress: "12 Independence Layout, Enugu",
		IdempotencyKey:  fmt.Sprintf("IDEMP-ORD-1-%d", time.Now().UnixNano()),
	})
	if err != nil {
		t.Fatalf("Order 1 creation failed: %v", err)
	}
	if order1.Order.SubtotalKobo != 3500000 { // 7 * 500,000
		t.Errorf("Expected subtotal 3500000, got %d", order1.Order.SubtotalKobo)
	}

	// Verify remaining stock is 3
	refreshedP, _ := env.MarketplaceService.GetProductByID(ctx, p.ID)
	if refreshedP.StockQuantity != 3 {
		t.Fatalf("Expected remaining stock 3, got %d", refreshedP.StockQuantity)
	}

	// Second buyer attempts to checkout 5 units (only 3 available) -> MUST FAIL
	_, err = env.MarketplaceService.CreateOrder(ctx, buyer2, service.CheckoutInput{
		Items:           []service.CartItemInput{{ProductID: p.ID, Quantity: 5}},
		BuyerName:       "Buyer Two",
		BuyerEmail:      "buyer2@example.org",
		BuyerPhone:      "+2348033334444",
		DeliveryType:    "delivery",
		ShippingAddress: "45 Ogui Road, Enugu",
		IdempotencyKey:  fmt.Sprintf("IDEMP-ORD-2-%d", time.Now().UnixNano()),
	})
	if !errors.Is(err, service.ErrInsufficientStock) {
		t.Fatalf("Expected ErrInsufficientStock, got %v", err)
	}

	// Second buyer checks out remaining 3 units -> MUST SUCCEED
	order2, err := env.MarketplaceService.CreateOrder(ctx, buyer2, service.CheckoutInput{
		Items:           []service.CartItemInput{{ProductID: p.ID, Quantity: 3}},
		BuyerName:       "Buyer Two",
		BuyerEmail:      "buyer2@example.org",
		BuyerPhone:      "+2348033334444",
		DeliveryType:    "pickup",
		ShippingAddress: "",
		IdempotencyKey:  fmt.Sprintf("IDEMP-ORD-3-%d", time.Now().UnixNano()),
	})
	if err != nil {
		t.Fatalf("Order 2 creation failed: %v", err)
	}
	if order2.Order.DeliveryFeeKobo != 0 {
		t.Errorf("Expected 0 delivery fee for pickup, got %d", order2.Order.DeliveryFeeKobo)
	}

	// Final stock must be exactly 0
	finalP, _ := env.MarketplaceService.GetProductByID(ctx, p.ID)
	if finalP.StockQuantity != 0 {
		t.Fatalf("Expected stock 0, got %d", finalP.StockQuantity)
	}
}

func TestMarketplaceService_IdempotencyAndDuplicatePrevention(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	seller := createRegisteredUser(t, env, "idemp_seller", "member")
	admin := createRegisteredUser(t, env, "idemp_admin", "super_admin")
	buyer := createRegisteredUser(t, env, "idemp_buyer", "member")

	b, _ := env.MarketplaceService.RegisterBusiness(ctx, seller, service.RegisterBusinessInput{
		Name:          "Port Harcourt Supplies",
		Category:      "tools_hardware",
		OwnershipType: "member_owned",
		ContactEmail:  "ph@example.org",
		ContactPhone:  "+2348099887766",
	})
	_, _ = env.MarketplaceService.ApproveBusiness(ctx, admin, b.ID, true, "")

	p, _ := env.MarketplaceService.CreateProduct(ctx, seller, service.CreateProductInput{
		BusinessID:    b.ID,
		Title:         "Safety Boots Size 43",
		Category:      "safety_gear",
		SKU:           fmt.Sprintf("SKU-BOOT-%d", time.Now().UnixNano()),
		PriceKobo:     800000,
		StockQuantity: 10,
		Status:        "published",
	})

	idempotencyKey := fmt.Sprintf("IDEMP-REPEAT-%d", time.Now().UnixNano())

	// First execution
	ord1, err := env.MarketplaceService.CreateOrder(ctx, buyer, service.CheckoutInput{
		Items:          []service.CartItemInput{{ProductID: p.ID, Quantity: 2}},
		BuyerName:      "Chidi Okonkwo",
		BuyerEmail:     "chidi@example.org",
		BuyerPhone:     "+2348022223333",
		DeliveryType:   "delivery",
		IdempotencyKey: idempotencyKey,
	})
	if err != nil {
		t.Fatalf("First order attempt failed: %v", err)
	}

	// Second execution with identical idempotency key (e.g. network retry)
	ord2, err := env.MarketplaceService.CreateOrder(ctx, buyer, service.CheckoutInput{
		Items:          []service.CartItemInput{{ProductID: p.ID, Quantity: 2}},
		BuyerName:      "Chidi Okonkwo",
		BuyerEmail:     "chidi@example.org",
		BuyerPhone:     "+2348022223333",
		DeliveryType:   "delivery",
		IdempotencyKey: idempotencyKey,
	})
	if err != nil {
		t.Fatalf("Second order attempt failed: %v", err)
	}

	// Must return the identical order ID
	if ord1.Order.ID != ord2.Order.ID {
		t.Fatalf("Idempotency violation! Generated two distinct orders: %s vs %s", ord1.Order.ID, ord2.Order.ID)
	}

	// Stock must have only been deducted ONCE (10 - 2 = 8, not 6!)
	refreshedP, _ := env.MarketplaceService.GetProductByID(ctx, p.ID)
	if refreshedP.StockQuantity != 8 {
		t.Fatalf("Duplicate stock deduction occurred! Expected 8, got %d", refreshedP.StockQuantity)
	}
}

func TestMarketplaceService_PaymentLifecycleAndWebhook(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	seller := createRegisteredUser(t, env, "pay_seller", "member")
	admin := createRegisteredUser(t, env, "pay_admin", "super_admin")
	buyer := createRegisteredUser(t, env, "pay_buyer", "member")

	b, _ := env.MarketplaceService.RegisterBusiness(ctx, seller, service.RegisterBusinessInput{
		Name:          "Ibadan Equipment Hub",
		Category:      "tools_hardware",
		OwnershipType: "member_owned",
		ContactEmail:  "ibadan@example.org",
		ContactPhone:  "+2348077665544",
	})
	_, _ = env.MarketplaceService.ApproveBusiness(ctx, admin, b.ID, true, "")

	p, _ := env.MarketplaceService.CreateProduct(ctx, seller, service.CreateProductInput{
		BusinessID:    b.ID,
		Title:         "Sanitation Rake Heavy Duty",
		Category:      "tools_hardware",
		SKU:           fmt.Sprintf("SKU-RAKE-%d", time.Now().UnixNano()),
		PriceKobo:     250000, // ₦2,500
		StockQuantity: 20,
		Status:        "published",
	})

	orderRes, err := env.MarketplaceService.CreateOrder(ctx, buyer, service.CheckoutInput{
		Items:          []service.CartItemInput{{ProductID: p.ID, Quantity: 2}}, // 500,000 + 150,000 delivery = 650,000
		BuyerName:      "Adebayo Tunde",
		BuyerEmail:     "adebayo@example.org",
		BuyerPhone:     "+2348088776655",
		DeliveryType:   "delivery",
		IdempotencyKey: fmt.Sprintf("IDEMP-PAY-%d", time.Now().UnixNano()),
	})
	if err != nil {
		t.Fatalf("Order creation failed: %v", err)
	}

	paymentRef := orderRes.Payment.Reference

	// 1. Order initial status must be 'placed' and payment 'pending'
	if orderRes.Order.OrderStatus != "placed" || orderRes.Order.PaymentStatus != "pending" {
		t.Fatalf("Expected placed / pending, got %s / %s", orderRes.Order.OrderStatus, orderRes.Order.PaymentStatus)
	}

	// 2. Process webhook event
	webhookPayload, _ := json.Marshal(map[string]interface{}{
		"event": "charge.success",
		"data": map[string]interface{}{
			"reference": paymentRef,
			"status":    "success",
			"amount":    orderRes.Order.TotalKobo,
		},
	})

	err = env.MarketplaceService.HandleWebhook(ctx, "sandbox-signature-ok", webhookPayload)
	if err != nil {
		t.Fatalf("Webhook processing failed: %v", err)
	}

	// 3. Order status must now be 'processing' and payment 'paid'
	paidOrder, _, _, err := env.MarketplaceService.GetOrderByID(ctx, buyer, orderRes.Order.ID)
	if err != nil {
		t.Fatalf("Failed to retrieve order: %v", err)
	}
	if paidOrder.PaymentStatus != "paid" || paidOrder.OrderStatus != "processing" {
		t.Errorf("Expected paid / processing, got %s / %s", paidOrder.PaymentStatus, paidOrder.OrderStatus)
	}

	// 4. Repeated webhook execution (idempotency check)
	err = env.MarketplaceService.HandleWebhook(ctx, "sandbox-signature-ok", webhookPayload)
	if err != nil {
		t.Fatalf("Repeated webhook returned error: %v", err)
	}
}

func TestMarketplaceService_FulfilmentAndDisputeResolution(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	seller := createRegisteredUser(t, env, "ful_seller", "member")
	admin := createRegisteredUser(t, env, "ful_admin", "super_admin")
	buyer := createRegisteredUser(t, env, "ful_buyer", "member")

	b, _ := env.MarketplaceService.RegisterBusiness(ctx, seller, service.RegisterBusinessInput{
		Name:          "Benin Logistics Supplies",
		Category:      "tools_hardware",
		OwnershipType: "member_owned",
		ContactEmail:  "benin@example.org",
		ContactPhone:  "+2348012398765",
	})
	_, _ = env.MarketplaceService.ApproveBusiness(ctx, admin, b.ID, true, "")

	p, _ := env.MarketplaceService.CreateProduct(ctx, seller, service.CreateProductInput{
		BusinessID:    b.ID,
		Title:         "Protective Safety Helmet",
		Category:      "safety_gear",
		SKU:           fmt.Sprintf("SKU-HELM-%d", time.Now().UnixNano()),
		PriceKobo:     350000,
		StockQuantity: 15,
		Status:        "published",
	})

	orderRes, _ := env.MarketplaceService.CreateOrder(ctx, buyer, service.CheckoutInput{
		Items:          []service.CartItemInput{{ProductID: p.ID, Quantity: 4}}, // 15 - 4 = 11 stock
		BuyerName:      "Osas Ighodaro",
		BuyerEmail:     "osas@example.org",
		BuyerPhone:     "+2348099112233",
		DeliveryType:   "delivery",
		IdempotencyKey: fmt.Sprintf("IDEMP-FUL-%d", time.Now().UnixNano()),
	})

	// Confirm payment
	_, err := env.MarketplaceService.ConfirmPayment(ctx, orderRes.Payment.Reference)
	if err != nil {
		t.Fatalf("Payment confirmation failed: %v", err)
	}

	// 1. Seller dispatches order
	sh, err := env.MarketplaceService.DispatchOrder(ctx, seller, orderRes.Order.ID, service.DispatchOrderInput{
		CarrierName:    "GIG Logistics",
		TrackingNumber: "GIG-RAT-9912",
		TrackingURL:    "https://giglogistics.example/track/GIG-RAT-9912",
		DeliveryNotes:  "Fragile sanitation consignment",
	})
	if err != nil {
		t.Fatalf("Order dispatch failed: %v", err)
	}
	if sh.CarrierName != "GIG Logistics" {
		t.Errorf("Unexpected carrier: %s", sh.CarrierName)
	}

	// 2. Buyer inspects and raises a dispute (e.g. damaged goods during transit)
	disp, err := env.MarketplaceService.RaiseDispute(ctx, buyer, orderRes.Order.ID, service.CreateDisputeInput{
		Reason:        "damaged_goods",
		CustomerNotes: "Helmet visors cracked upon package opening",
	})
	if err != nil {
		t.Fatalf("Failed to raise dispute: %v", err)
	}
	if disp.Status != "open" {
		t.Errorf("Expected open dispute, got %s", disp.Status)
	}

	// 3. Admin resolves dispute with refund
	// Stock should automatically be restored! (11 + 4 = 15)
	resolvedDisp, err := env.MarketplaceService.ResolveDispute(ctx, admin, disp.ID, "resolved_refunded", "Full refund approved; carrier claim filed")
	if err != nil {
		t.Fatalf("Failed to resolve dispute: %v", err)
	}
	if resolvedDisp.Status != "resolved_refunded" {
		t.Errorf("Expected resolved_refunded, got %s", resolvedDisp.Status)
	}

	// 4. Verify order status is 'refunded' and payment status is 'refunded'
	refreshedOrder, _, _, _ := env.MarketplaceService.GetOrderByID(ctx, buyer, orderRes.Order.ID)
	if refreshedOrder.OrderStatus != "refunded" || refreshedOrder.PaymentStatus != "refunded" {
		t.Errorf("Expected refunded / refunded, got %s / %s", refreshedOrder.OrderStatus, refreshedOrder.PaymentStatus)
	}

	// 5. Verify stock was restored back to 15!
	restoredP, _ := env.MarketplaceService.GetProductByID(ctx, p.ID)
	if restoredP.StockQuantity != 15 {
		t.Fatalf("Stock restoration failed! Expected 15, got %d", restoredP.StockQuantity)
	}
}
