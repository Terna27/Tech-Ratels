package handler_test

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"ratels-api/internal/db"
	"ratels-api/internal/handler"
	"ratels-api/internal/payment"
	"ratels-api/internal/service"
)

func makeTestJPEGBytes(w, h int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := 0; x < w; x++ {
		for y := 0; y < h; y++ {
			img.Set(x, y, color.RGBA{R: 220, G: 80, B: 40, A: 255})
		}
	}
	var buf bytes.Buffer
	_ = jpeg.Encode(&buf, img, nil)
	return buf.Bytes()
}

func createTestUserWithToken(t *testing.T, h *TestHarness, prefix, name, role string) (*db.User, string) {
	t.Helper()
	userID, token := createTestUserWithRole(t, h, prefix, name, role)
	u, err := h.Repo.Queries().GetUserByID(context.Background(), userID)
	if err != nil {
		t.Fatalf("Failed to fetch user %s: %v", userID, err)
	}
	return &u, token
}

func TestMarketplaceHTTP_StorefrontAndProductCatalogue(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()

	// 1. Check checkout status endpoint
	req, _ := http.NewRequest("GET", "/api/v1/marketplace/checkout/status", nil)
	rr := httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK from checkout status, got %d: %s", rr.Code, rr.Body.String())
	}
	var statusResp map[string]interface{}
	_ = json.Unmarshal(rr.Body.Bytes(), &statusResp)
	if statusResp["available"] != true {
		t.Errorf("Expected checkout status available=true, got %v", statusResp["available"])
	}

	// 2. Query products endpoint
	req, _ = http.NewRequest("GET", "/api/v1/marketplace/products", nil)
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK from products list, got %d: %s", rr.Code, rr.Body.String())
	}

	var products []db.ListMarketplaceProductsRow
	if err := json.Unmarshal(rr.Body.Bytes(), &products); err != nil {
		t.Fatalf("Failed to decode products list: %v", err)
	}

	// 3. Query businesses endpoint
	req, _ = http.NewRequest("GET", "/api/v1/marketplace/businesses", nil)
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK from businesses list, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestMarketplaceHTTP_CartValidationAndPriceManipulationProtection(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()
	ctx := context.Background()

	adminUser, _ := createTestUserWithToken(t, h, "mkt_admin", "Admin User", "super_admin")
	sellerUser, _ := createTestUserWithToken(t, h, "mkt_seller", "Seller User", "member")

	// Register & approve business
	biz, err := h.MarketplaceService.RegisterBusiness(ctx, sellerUser, service.RegisterBusinessInput{
		Name:          "Abuja Clean Gear Store",
		Category:      "sanitation_supplies",
		OwnershipType: "member_owned",
		ContactEmail:  "gear@example.org",
		ContactPhone:  "+2348011223344",
	})
	if err != nil {
		t.Fatalf("RegisterBusiness failed: %v", err)
	}
	_, _ = h.MarketplaceService.ApproveBusiness(ctx, adminUser, biz.ID, true, "")

	// Create test product with trusted database price: ₦5,000 (500,000 kobo)
	p, err := h.MarketplaceService.CreateProduct(ctx, sellerUser, service.CreateProductInput{
		BusinessID:    biz.ID,
		Title:         "High Visibility Safety Vest",
		Category:      "safety_gear",
		SKU:           fmt.Sprintf("SKU-VEST-%d", time.Now().UnixNano()),
		PriceKobo:     500000,
		StockQuantity: 25,
		Status:        "published",
	})
	if err != nil {
		t.Fatalf("CreateProduct failed: %v", err)
	}

	// 1. Post Cart Validation: Client cannot alter price! Server recalculates from database
	cartPayload := map[string]interface{}{
		"items": []map[string]interface{}{
			{
				"product_id": p.ID,
				"quantity":   3,
			},
		},
	}
	cartBytes, _ := json.Marshal(cartPayload)
	req, _ := http.NewRequest("POST", "/api/v1/marketplace/cart/validate", bytes.NewBuffer(cartBytes))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK from cart validation, got %d: %s", rr.Code, rr.Body.String())
	}

	var cartResult service.CartValidationResult
	if err := json.Unmarshal(rr.Body.Bytes(), &cartResult); err != nil {
		t.Fatalf("Failed to decode cart result: %v", err)
	}

	expectedSubtotal := int64(1500000) // 3 * 500,000 kobo
	if cartResult.SubtotalKobo != expectedSubtotal {
		t.Errorf("Subtotal mismatch! Expected %d, got %d", expectedSubtotal, cartResult.SubtotalKobo)
	}

	// 2. Reject zero or negative quantity in cart validation: returns IsValid=false
	badCart := map[string]interface{}{
		"items": []map[string]interface{}{
			{"product_id": p.ID, "quantity": 0},
		},
	}
	badBytes, _ := json.Marshal(badCart)
	req, _ = http.NewRequest("POST", "/api/v1/marketplace/cart/validate", bytes.NewBuffer(badBytes))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK from cart validation, got %d", rr.Code)
	}
	var invalidCartResult service.CartValidationResult
	_ = json.Unmarshal(rr.Body.Bytes(), &invalidCartResult)
	if invalidCartResult.IsValid {
		t.Errorf("Expected invalidCartResult.IsValid to be false for zero quantity")
	}

	// 3. Reject zero or negative quantity on checkout endpoint: returns 400 Bad Request
	badCheckout := map[string]interface{}{
		"items": []map[string]interface{}{
			{"product_id": p.ID, "quantity": 0},
		},
		"buyer_name":    "Test",
		"buyer_email":   "test@example.org",
		"buyer_phone":   "+2348000000000",
		"delivery_type": "pickup",
	}
	chkBytes, _ := json.Marshal(badCheckout)
	req, _ = http.NewRequest("POST", "/api/v1/marketplace/checkout", bytes.NewBuffer(chkBytes))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("Expected 400 Bad Request from checkout with zero quantity, got %d", rr.Code)
	}
}

func TestMarketplaceHTTP_CheckoutAndIdempotency(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()
	ctx := context.Background()

	adminUser, _ := createTestUserWithToken(t, h, "chk_admin", "Admin User", "super_admin")
	sellerUser, _ := createTestUserWithToken(t, h, "chk_seller", "Seller User", "member")
	_, buyerToken := createTestUserWithToken(t, h, "chk_buyer", "Buyer User", "member")

	biz, _ := h.MarketplaceService.RegisterBusiness(ctx, sellerUser, service.RegisterBusinessInput{
		Name:          "Lagos Sanitation Hardware",
		Category:      "tools_hardware",
		OwnershipType: "member_owned",
		ContactEmail:  "hardware@example.org",
		ContactPhone:  "+2348022334455",
	})
	_, _ = h.MarketplaceService.ApproveBusiness(ctx, adminUser, biz.ID, true, "")

	p, _ := h.MarketplaceService.CreateProduct(ctx, sellerUser, service.CreateProductInput{
		BusinessID:    biz.ID,
		Title:         "Sanitation Rake",
		Category:      "tools_hardware",
		SKU:           fmt.Sprintf("SKU-RAKE-%d", time.Now().UnixNano()),
		PriceKobo:     300000, // ₦3,000
		StockQuantity: 10,
		Status:        "published",
	})

	idempotencyKey := fmt.Sprintf("idemp-http-chk-%d", time.Now().UnixNano())
	checkoutPayload := map[string]interface{}{
		"items": []map[string]interface{}{
			{"product_id": p.ID, "quantity": 2},
		},
		"buyer_name":       "Aliyu Danladi",
		"buyer_email":      "aliyu@example.org",
		"buyer_phone":      "+2348033221100",
		"delivery_type":    "delivery",
		"shipping_address": "Plot 404 Herbert Macaulay Way, Yaba, Lagos",
		"idempotency_key":  idempotencyKey,
	}
	bodyBytes, _ := json.Marshal(checkoutPayload)

	// 1. Initial checkout request
	req, _ := http.NewRequest("POST", "/api/v1/marketplace/checkout", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Authorization", "Bearer "+buyerToken)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("Expected 201 Created from checkout, got %d: %s", rr.Code, rr.Body.String())
	}

	var firstResult map[string]interface{}
	_ = json.Unmarshal(rr.Body.Bytes(), &firstResult)
	firstOrder := firstResult["order"].(map[string]interface{})
	firstOrderNum := firstOrder["order_number"].(string)

	// 2. Duplicate checkout request with exact same idempotency key must NOT duplicate order or decrement stock again
	req2, _ := http.NewRequest("POST", "/api/v1/marketplace/checkout", bytes.NewBuffer(bodyBytes))
	req2.Header.Set("Authorization", "Bearer "+buyerToken)
	req2.Header.Set("Content-Type", "application/json")
	rr2 := httptest.NewRecorder()
	h.Router.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusOK && rr2.Code != http.StatusCreated {
		t.Fatalf("Expected 200/201 from duplicate idempotency checkout, got %d: %s", rr2.Code, rr2.Body.String())
	}
	var secondResult map[string]interface{}
	_ = json.Unmarshal(rr2.Body.Bytes(), &secondResult)
	secondOrder := secondResult["order"].(map[string]interface{})
	if secondOrder["order_number"] != firstOrderNum {
		t.Errorf("Idempotency failed! Expected order %s, got %v", firstOrderNum, secondOrder["order_number"])
	}

	// Stock should have decremented exactly once from 10 to 8
	pCheck, _ := h.MarketplaceService.GetProductByID(ctx, p.ID)
	if pCheck.StockQuantity != 8 {
		t.Errorf("Expected stock quantity 8, got %d", pCheck.StockQuantity)
	}
}

func TestMarketplaceHTTP_PaymentVerificationAndWebhook(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()
	ctx := context.Background()

	adminUser, _ := createTestUserWithToken(t, h, "pay_admin", "Admin User", "super_admin")
	sellerUser, _ := createTestUserWithToken(t, h, "pay_seller", "Seller User", "member")
	buyerUser, buyerToken := createTestUserWithToken(t, h, "pay_buyer", "Buyer User", "member")

	biz, _ := h.MarketplaceService.RegisterBusiness(ctx, sellerUser, service.RegisterBusinessInput{
		Name:          "Kaduna Safety Store",
		Category:      "safety_gear",
		OwnershipType: "member_owned",
		ContactEmail:  "safety_kd@example.org",
		ContactPhone:  "+2348033445566",
	})
	_, _ = h.MarketplaceService.ApproveBusiness(ctx, adminUser, biz.ID, true, "")

	p, _ := h.MarketplaceService.CreateProduct(ctx, sellerUser, service.CreateProductInput{
		BusinessID:    biz.ID,
		Title:         "Sanitation Gloves",
		Category:      "sanitation_supplies",
		SKU:           fmt.Sprintf("SKU-GLOV-%d", time.Now().UnixNano()),
		PriceKobo:     200000, // ₦2,000
		StockQuantity: 15,
		Status:        "published",
	})

	orderRes, err := h.MarketplaceService.CreateOrder(ctx, buyerUser, service.CheckoutInput{
		Items:        []service.CartItemInput{{ProductID: p.ID, Quantity: 1}},
		BuyerName:    "Buyer Payment Test",
		BuyerEmail:   "paytest@example.org",
		BuyerPhone:   "+2348099887766",
		DeliveryType: "pickup",
	})
	if err != nil {
		t.Fatalf("CreateOrder failed: %v", err)
	}

	paymentRef := orderRes.Payment.Reference

	// 1. Post verification request
	verifyPayload := map[string]string{"reference": paymentRef}
	verifyBytes, _ := json.Marshal(verifyPayload)
	req, _ := http.NewRequest("POST", "/api/v1/marketplace/payments/verify", bytes.NewBuffer(verifyBytes))
	req.Header.Set("Authorization", "Bearer "+buyerToken)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK from payment verify, got %d: %s", rr.Code, rr.Body.String())
	}

	var verifyRes map[string]interface{}
	_ = json.Unmarshal(rr.Body.Bytes(), &verifyRes)
	if verifyRes["status"] != "paid" {
		t.Errorf("Expected status 'paid', got %v", verifyRes["status"])
	}

	// 2. Reject Paystack webhook with invalid signature
	webhookPayload := map[string]interface{}{
		"event": "charge.success",
		"data": map[string]interface{}{
			"reference": paymentRef,
			"status":    "success",
			"amount":    orderRes.Order.TotalKobo,
			"currency":  "NGN",
		},
	}
	whBytes, _ := json.Marshal(webhookPayload)
	reqWh, _ := http.NewRequest("POST", "/api/v1/marketplace/webhooks/paystack", bytes.NewBuffer(whBytes))
	reqWh.Header.Set("Content-Type", "application/json")
	// Empty signature header
	rrWh := httptest.NewRecorder()
	h.Router.ServeHTTP(rrWh, reqWh)
	if rrWh.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request for webhook without valid signature, got %d", rrWh.Code)
	}
}

func TestMarketplaceHTTP_SellerIsolationAndProductManagement(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()
	ctx := context.Background()

	adminUser, _ := createTestUserWithToken(t, h, "iso_admin", "Admin", "super_admin")
	seller1, token1 := createTestUserWithToken(t, h, "iso_seller1", "Seller One", "member")
	seller2, token2 := createTestUserWithToken(t, h, "iso_seller2", "Seller Two", "member")

	// Seller 1 creates business and product
	b1, _ := h.MarketplaceService.RegisterBusiness(ctx, seller1, service.RegisterBusinessInput{
		Name:          "Store Alpha",
		Category:      "tools_hardware",
		OwnershipType: "member_owned",
		ContactEmail:  "alpha@example.org",
		ContactPhone:  "+2348011111111",
	})
	_, _ = h.MarketplaceService.ApproveBusiness(ctx, adminUser, b1.ID, true, "")

	b2, _ := h.MarketplaceService.RegisterBusiness(ctx, seller2, service.RegisterBusinessInput{
		Name:          "Store Beta",
		Category:      "tools_hardware",
		OwnershipType: "member_owned",
		ContactEmail:  "beta@example.org",
		ContactPhone:  "+2348022222222",
	})
	_, _ = h.MarketplaceService.ApproveBusiness(ctx, adminUser, b2.ID, true, "")

	p1, _ := h.MarketplaceService.CreateProduct(ctx, seller1, service.CreateProductInput{
		BusinessID:    b1.ID,
		Title:         "Alpha Shovel",
		Category:      "tools_hardware",
		SKU:           fmt.Sprintf("SKU-ALPHA-%d", time.Now().UnixNano()),
		PriceKobo:     400000,
		StockQuantity: 10,
		Status:        "published",
	})

	// 1. Seller 2 attempts to UPDATE Seller 1's product via API -> Must be 403 Forbidden
	updatePayload := map[string]interface{}{
		"title":          "Tampered Title by Competitor",
		"category":       "tools_hardware",
		"sku":            p1.Sku,
		"price_kobo":     1000, // Attempted price manipulation
		"stock_quantity": 99,
		"status":         "published",
	}
	upBytes, _ := json.Marshal(updatePayload)
	req, _ := http.NewRequest("PUT", fmt.Sprintf("/api/v1/marketplace/seller/products/%s", p1.ID), bytes.NewBuffer(upBytes))
	req.Header.Set("Authorization", "Bearer "+token2)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("Expected 403 Forbidden when Seller 2 updates Seller 1 product, got %d: %s", rr.Code, rr.Body.String())
	}

	// 2. Seller 2 attempts to DELETE Seller 1's product -> Must be 403 Forbidden
	req, _ = http.NewRequest("DELETE", fmt.Sprintf("/api/v1/marketplace/seller/products/%s", p1.ID), nil)
	req.Header.Set("Authorization", "Bearer "+token2)
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("Expected 403 Forbidden when Seller 2 deletes Seller 1 product, got %d: %s", rr.Code, rr.Body.String())
	}

	// 3. Seller 1 successfully updates own product
	req, _ = http.NewRequest("PUT", fmt.Sprintf("/api/v1/marketplace/seller/products/%s", p1.ID), bytes.NewBuffer(upBytes))
	req.Header.Set("Authorization", "Bearer "+token1)
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK when Seller 1 updates own product, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestMarketplaceHTTP_AdminAuthorizationAndRefunds(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()
	ctx := context.Background()

	adminUser, adminToken := createTestUserWithToken(t, h, "adm_auth_sa", "Super Admin", "super_admin")
	sellerUser, _ := createTestUserWithToken(t, h, "adm_seller", "Seller", "member")
	buyerUser, _ := createTestUserWithToken(t, h, "adm_buyer", "Buyer", "member")
	_, ordinaryMemberToken := createTestUserWithToken(t, h, "adm_ordinary", "Ordinary Member", "member")

	// 1. Unprivileged ordinary member cannot access /api/v1/marketplace/admin/overview
	req, _ := http.NewRequest("GET", "/api/v1/marketplace/admin/overview", nil)
	req.Header.Set("Authorization", "Bearer "+ordinaryMemberToken)
	rr := httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("Expected 403 Forbidden for non-admin on admin overview, got %d", rr.Code)
	}

	// 2. Super Admin can access admin overview
	req, _ = http.NewRequest("GET", "/api/v1/marketplace/admin/overview", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for super admin on admin overview, got %d: %s", rr.Code, rr.Body.String())
	}

	// 3. Test Partial and Full Refunds
	biz, _ := h.MarketplaceService.RegisterBusiness(ctx, sellerUser, service.RegisterBusinessInput{
		Name:          "Refund Store Hub",
		Category:      "tools_hardware",
		OwnershipType: "member_owned",
		ContactEmail:  "ref_hub@example.org",
		ContactPhone:  "+2348011223399",
	})
	_, _ = h.MarketplaceService.ApproveBusiness(ctx, adminUser, biz.ID, true, "")

	p, _ := h.MarketplaceService.CreateProduct(ctx, sellerUser, service.CreateProductInput{
		BusinessID:    biz.ID,
		Title:         "Sanitation Gloves Pack",
		Category:      "sanitation_supplies",
		SKU:           fmt.Sprintf("SKU-REF-%d", time.Now().UnixNano()),
		PriceKobo:     500000, // ₦5,000
		StockQuantity: 10,
		Status:        "published",
	})

	orderRes, _ := h.MarketplaceService.CreateOrder(ctx, buyerUser, service.CheckoutInput{
		Items:        []service.CartItemInput{{ProductID: p.ID, Quantity: 2}},
		BuyerName:    "Buyer Refund HTTP",
		BuyerEmail:   "ref_buyer@example.org",
		BuyerPhone:   "+2348000000000",
		DeliveryType: "pickup",
	})
	_, _ = h.MarketplaceService.ConfirmPayment(ctx, orderRes.Payment.Reference)

	// Overpayment refund rejected (1,500,000 > 1,000,000 total)
	overRefundPayload := map[string]interface{}{
		"amount_kobo": 1500000,
		"reason":      "Too much",
	}
	orBytes, _ := json.Marshal(overRefundPayload)
	req, _ = http.NewRequest("POST", fmt.Sprintf("/api/v1/marketplace/admin/orders/%s/refund", orderRes.Order.ID), bytes.NewBuffer(orBytes))
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("Expected 400 Bad Request for overpayment refund, got %d: %s", rr.Code, rr.Body.String())
	}

	// Partial refund (400,000 kobo of 1,000,000 kobo)
	partialRefundPayload := map[string]interface{}{
		"amount_kobo": 400000,
		"reason":      "Partial item compensation",
	}
	prBytes, _ := json.Marshal(partialRefundPayload)
	req, _ = http.NewRequest("POST", fmt.Sprintf("/api/v1/marketplace/admin/orders/%s/refund", orderRes.Order.ID), bytes.NewBuffer(prBytes))
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for partial refund, got %d: %s", rr.Code, rr.Body.String())
	}

	// Full refund (remaining amount)
	fullRefundPayload := map[string]interface{}{
		"amount_kobo": 1000000,
		"reason":      "Full order cancellation and refund",
	}
	frBytes, _ := json.Marshal(fullRefundPayload)
	req, _ = http.NewRequest("POST", fmt.Sprintf("/api/v1/marketplace/admin/orders/%s/refund", orderRes.Order.ID), bytes.NewBuffer(frBytes))
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for full refund, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestMarketplaceHTTP_UnconfiguredGatewaySafety(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()
	ctx := context.Background()

	// Create service with UnconfiguredGateway
	unconfiguredGw := payment.NewUnconfiguredGateway()
	unconfiguredSvc := service.NewMarketplaceService(h.Repo, service.NewAuditService(h.Repo), unconfiguredGw)
	unconfH := handler.NewMarketplaceHandler(unconfiguredSvc)

	// 1. Checkout status must report available=false
	req, _ := http.NewRequest("GET", "/api/v1/marketplace/checkout/status", nil)
	rr := httptest.NewRecorder()
	unconfH.GetCheckoutStatus(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d", rr.Code)
	}
	var res map[string]interface{}
	_ = json.Unmarshal(rr.Body.Bytes(), &res)
	if res["available"] != false {
		t.Errorf("Expected available=false for unconfigured gateway, got %v", res["available"])
	}

	// 2. Checkout submission must be rejected with 503 Service Unavailable
	chkPayload := map[string]interface{}{
		"items": []map[string]interface{}{
			{"product_id": "test-prod", "quantity": 1},
		},
		"buyer_name":    "Test Buyer",
		"buyer_email":   "buyer@example.org",
		"buyer_phone":   "+2348000000000",
		"delivery_type": "pickup",
	}
	bBytes, _ := json.Marshal(chkPayload)
	req, _ = http.NewRequest("POST", "/api/v1/marketplace/checkout", bytes.NewBuffer(bBytes))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	unconfH.Checkout(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("Expected 503 Service Unavailable when gateway is unconfigured, got %d: %s", rr.Code, rr.Body.String())
	}

	_ = ctx
}

func computePaystackSignature(secret string, body []byte) string {
	mac := hmac.New(sha512.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func TestMarketplaceHandler_ProductImageUpload_PresignEndpoint(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Pool.Close()
	ctx := context.Background()

	seller, token := createTestUserWithToken(t, h, "h_img_seller", "H Seller", "member")
	admin, _ := createTestUserWithToken(t, h, "h_img_admin", "H Admin", "super_admin")

	biz, err := h.MarketplaceService.RegisterBusiness(ctx, seller, service.RegisterBusinessInput{
		Name:          "H Image Store",
		Description:   "Testing Image Handlers",
		Category:      "apparel",
		OwnershipType: "member_owned",
		ContactEmail:  "himg@example.org",
		ContactPhone:  "+2348008889999",
	})
	if err != nil {
		t.Fatalf("RegisterBusiness failed: %v", err)
	}
	_, _ = h.MarketplaceService.ApproveBusiness(ctx, admin, biz.ID, true, "Approved for testing")

	// 1. Authorized presign request
	reqBody, _ := json.Marshal(map[string]interface{}{
		"filename":     "product.jpg",
		"content_type": "image/jpeg",
		"file_size":    200000,
	})
	req, _ := http.NewRequest("POST", "/api/v1/marketplace/seller/products/images/presign", bytes.NewBuffer(reqBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for presign request, got %d: %s", rr.Code, rr.Body.String())
	}

	var res map[string]interface{}
	_ = json.Unmarshal(rr.Body.Bytes(), &res)
	if res["upload_url"] == "" || res["object_key"] == "" || res["public_url"] == "" {
		t.Fatalf("Expected upload_url, object_key, public_url in response: %v", res)
	}

	// 2. Reject request exceeding 5MB
	hugeBody, _ := json.Marshal(map[string]interface{}{
		"filename":     "huge.jpg",
		"content_type": "image/jpeg",
		"file_size":    6 * 1024 * 1024,
	})
	req, _ = http.NewRequest("POST", "/api/v1/marketplace/seller/products/images/presign", bytes.NewBuffer(hugeBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("Expected 400 Bad Request for file > 5MB, got %d", rr.Code)
	}

	// 3. Reject unauthenticated request
	req, _ = http.NewRequest("POST", "/api/v1/marketplace/seller/products/images/presign", bytes.NewBuffer(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("Expected 401 Unauthorized for missing auth token, got %d", rr.Code)
	}
}

func TestMarketplaceHandler_ProductImageUpload_DirectMultipartEndpoint(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Pool.Close()
	ctx := context.Background()

	seller, token := createTestUserWithToken(t, h, "h_direct_seller", "Direct Seller", "member")
	admin, _ := createTestUserWithToken(t, h, "h_direct_admin", "Direct Admin", "super_admin")

	biz, _ := h.MarketplaceService.RegisterBusiness(ctx, seller, service.RegisterBusinessInput{
		Name:          "Direct Multipart Store",
		Description:   "Testing Multipart",
		Category:      "apparel",
		OwnershipType: "member_owned",
		ContactEmail:  "direct_mp@example.org",
		ContactPhone:  "+2348007776666",
	})
	_, _ = h.MarketplaceService.ApproveBusiness(ctx, admin, biz.ID, true, "Approved for testing")

	// 1. Valid multipart upload of JPEG
	jpegBytes := makeTestJPEGBytes(300, 200)
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, _ := writer.CreateFormFile("file", "cap.jpg")
	_, _ = part.Write(jpegBytes)
	_ = writer.Close()

	req, _ := http.NewRequest("POST", "/api/v1/marketplace/seller/products/images/upload", &body)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rr := httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("Expected 201 Created from direct multipart upload, got %d: %s", rr.Code, rr.Body.String())
	}

	var res map[string]interface{}
	_ = json.Unmarshal(rr.Body.Bytes(), &res)
	if res["object_key"] == "" || res["public_url"] == "" {
		t.Fatalf("Expected object_key and public_url: %v", res)
	}
	if fmt.Sprintf("%v", res["width"]) != "300" || fmt.Sprintf("%v", res["height"]) != "200" {
		t.Fatalf("Expected 300x200 dimensions, got %v x %v", res["width"], res["height"])
	}

	// 2. Reject corrupt image upload
	var badBody bytes.Buffer
	badWriter := multipart.NewWriter(&badBody)
	badPart, _ := badWriter.CreateFormFile("file", "corrupt.jpg")
	_, _ = badPart.Write([]byte("not an image at all"))
	_ = badWriter.Close()

	req, _ = http.NewRequest("POST", "/api/v1/marketplace/seller/products/images/upload", &badBody)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", badWriter.FormDataContentType())
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("Expected 400 Bad Request for corrupted image payload, got %d", rr.Code)
	}
}

func TestMarketplaceHandler_ProductImageUpload_DeleteEndpoint(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Pool.Close()
	ctx := context.Background()

	sellerA, tokenA := createTestUserWithToken(t, h, "del_seller_a", "Del Seller A", "member")
	sellerB, tokenB := createTestUserWithToken(t, h, "del_seller_b", "Del Seller B", "member")
	admin, _ := createTestUserWithToken(t, h, "del_admin", "Del Admin", "super_admin")

	bizA, _ := h.MarketplaceService.RegisterBusiness(ctx, sellerA, service.RegisterBusinessInput{
		Name:          "Delete Store A",
		Description:   "Store A",
		Category:      "apparel",
		OwnershipType: "member_owned",
		ContactEmail:  "dela@example.org",
		ContactPhone:  "+2348005551111",
	})
	_, _ = h.MarketplaceService.ApproveBusiness(ctx, admin, bizA.ID, true, "Approved for testing")

	bizB, _ := h.MarketplaceService.RegisterBusiness(ctx, sellerB, service.RegisterBusinessInput{
		Name:          "Delete Store B",
		Description:   "Store B",
		Category:      "apparel",
		OwnershipType: "member_owned",
		ContactEmail:  "delb@example.org",
		ContactPhone:  "+2348005552222",
	})
	_, _ = h.MarketplaceService.ApproveBusiness(ctx, admin, bizB.ID, true, "Approved for testing")

	// Upload image for Seller A
	upResA, err := h.MarketplaceService.UploadProductImageDirect(ctx, sellerA, "a.jpg", bytes.NewReader(makeTestJPEGBytes(100, 100)), int64(len(makeTestJPEGBytes(100, 100))))
	if err != nil {
		t.Fatalf("UploadProductImageDirect failed: %v", err)
	}

	// 1. Seller B attempts to delete Seller A's image -> 403 Forbidden
	delBody, _ := json.Marshal(map[string]interface{}{"object_key": upResA.ObjectKey})
	req, _ := http.NewRequest("DELETE", "/api/v1/marketplace/seller/products/images", bytes.NewBuffer(delBody))
	req.Header.Set("Authorization", "Bearer "+tokenB)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("Expected 403 Forbidden when deleting other seller's image, got %d: %s", rr.Code, rr.Body.String())
	}

	// 2. Seller A deletes their own image -> 200 OK
	req, _ = http.NewRequest("DELETE", "/api/v1/marketplace/seller/products/images", bytes.NewBuffer(delBody))
	req.Header.Set("Authorization", "Bearer "+tokenA)
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK when seller deletes their own image, got %d: %s", rr.Code, rr.Body.String())
	}
}
