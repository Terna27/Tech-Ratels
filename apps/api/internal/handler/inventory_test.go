package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestInventoryAPI_EndpointsAndPermissions(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()
	ctx := context.Background()

	// 1. Setup users
	_, saToken := createTestUserWithRole(t, h, "inv_sa", "Inventory Super Admin", "super_admin")
	_, memberToken := createTestUserWithRole(t, h, "inv_mem", "Regular Member", "member")

	// 2. Unauthenticated GET /api/v1/inventory/items -> 401
	unauthReq, _ := http.NewRequest("GET", "/api/v1/inventory/items", nil)
	rr := httptest.NewRecorder()
	h.Router.ServeHTTP(rr, unauthReq)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("Expected 401 for unauthenticated GET /items, got %d", rr.Code)
	}

	// 3. Regular member attempting POST /api/v1/inventory/items -> 403 Forbidden
	itemPayload, _ := json.Marshal(map[string]interface{}{
		"sku":                    fmt.Sprintf("HTTP-ITM-%d", time.Now().UnixNano()),
		"name":                   "Safety Helmets",
		"category":               "protective_equipment",
		"unit_of_measure":        "pcs",
		"min_threshold_quantity": 20,
	})
	forbiddenReq, _ := http.NewRequest("POST", "/api/v1/inventory/items", bytes.NewBuffer(itemPayload))
	forbiddenReq.Header.Set("Authorization", "Bearer "+memberToken)
	forbiddenReq.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, forbiddenReq)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("Expected 403 for member POST /items, got %d", rr.Code)
	}

	// 4. Super Admin creating item -> 201 Created
	createItemReq, _ := http.NewRequest("POST", "/api/v1/inventory/items", bytes.NewBuffer(itemPayload))
	createItemReq.Header.Set("Authorization", "Bearer "+saToken)
	createItemReq.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, createItemReq)
	if rr.Code != http.StatusCreated {
		t.Fatalf("Expected 201 for super_admin POST /items, got %d: %s", rr.Code, rr.Body.String())
	}

	var createdItem map[string]interface{}
	_ = json.Unmarshal(rr.Body.Bytes(), &createdItem)
	itemID := createdItem["id"].(string)

	// 5. Super Admin creating warehouses (National & State)
	locPayload1, _ := json.Marshal(map[string]interface{}{
		"code":          fmt.Sprintf("WH-API-A-%d", time.Now().UnixNano()),
		"name":          "HTTP Test Warehouse Alpha",
		"location_type": "national",
		"address":       "Abuja, FCT",
	})
	locReq1, _ := http.NewRequest("POST", "/api/v1/inventory/locations", bytes.NewBuffer(locPayload1))
	locReq1.Header.Set("Authorization", "Bearer "+saToken)
	locReq1.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, locReq1)
	if rr.Code != http.StatusCreated {
		t.Fatalf("Expected 201 for POST /locations, got %d", rr.Code)
	}
	var wh1 map[string]interface{}
	_ = json.Unmarshal(rr.Body.Bytes(), &wh1)
	wh1ID := wh1["id"].(string)

	locPayload2, _ := json.Marshal(map[string]interface{}{
		"code":          fmt.Sprintf("WH-API-B-%d", time.Now().UnixNano()),
		"name":          "HTTP Test Warehouse Beta",
		"location_type": "state",
		"address":       "Port Harcourt, Rivers",
	})
	locReq2, _ := http.NewRequest("POST", "/api/v1/inventory/locations", bytes.NewBuffer(locPayload2))
	locReq2.Header.Set("Authorization", "Bearer "+saToken)
	locReq2.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, locReq2)
	if rr.Code != http.StatusCreated {
		t.Fatalf("Expected 201 for POST /locations, got %d", rr.Code)
	}
	var wh2 map[string]interface{}
	_ = json.Unmarshal(rr.Body.Bytes(), &wh2)
	wh2ID := wh2["id"].(string)

	// 6. Receive stock into Warehouse Alpha
	rcvPayload, _ := json.Marshal(map[string]interface{}{
		"location_id":   wh1ID,
		"item_id":       itemID,
		"quantity":      100,
		"movement_type": "initial_receipt",
		"notes":         "Initial stock for HTTP test",
	})
	rcvReq, _ := http.NewRequest("POST", "/api/v1/inventory/balances/receive", bytes.NewBuffer(rcvPayload))
	rcvReq.Header.Set("Authorization", "Bearer "+saToken)
	rcvReq.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, rcvReq)
	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 for POST /balances/receive, got %d: %s", rr.Code, rr.Body.String())
	}

	// 7. Member submitting material request -> 201 Created
	matReqPayload, _ := json.Marshal(map[string]interface{}{
		"destination_location_id": wh2ID,
		"intended_use":           "Community Clean Water Project",
		"jurisdiction_type":       "national",
		"items": []map[string]interface{}{
			{
				"item_id":            itemID,
				"requested_quantity": 25,
				"unit_of_measure":    "pcs",
			},
		},
	})
	submitReq, _ := http.NewRequest("POST", "/api/v1/inventory/requests", bytes.NewBuffer(matReqPayload))
	submitReq.Header.Set("Authorization", "Bearer "+memberToken)
	submitReq.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, submitReq)
	if rr.Code != http.StatusCreated {
		t.Fatalf("Expected 201 for POST /requests by member, got %d: %s", rr.Code, rr.Body.String())
	}
	var createdReq map[string]interface{}
	_ = json.Unmarshal(rr.Body.Bytes(), &createdReq)
	requestID := createdReq["id"].(string)

	// 8. Requester attempting self-approval -> 403 Forbidden (Anti-Self-Approval)
	selfApprPayload, _ := json.Marshal(map[string]interface{}{
		"action": "approve",
	})
	selfApprReq, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/inventory/requests/%s/review", requestID), bytes.NewBuffer(selfApprPayload))
	selfApprReq.Header.Set("Authorization", "Bearer "+memberToken)
	selfApprReq.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, selfApprReq)
	// Member has no role for reviewing, so gets 403
	if rr.Code != http.StatusForbidden {
		t.Fatalf("Expected 403 for self review, got %d", rr.Code)
	}

	// 9. Super Admin approving request -> 200 OK
	saApprReq, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/inventory/requests/%s/review", requestID), bytes.NewBuffer(selfApprPayload))
	saApprReq.Header.Set("Authorization", "Bearer "+saToken)
	saApprReq.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, saApprReq)
	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 for review approval by super_admin, got %d: %s", rr.Code, rr.Body.String())
	}

	// 10. Allocate stock from Warehouse Alpha
	allocPayload, _ := json.Marshal(map[string]interface{}{
		"supplying_location_id": wh1ID,
	})
	allocReq, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/inventory/requests/%s/allocate", requestID), bytes.NewBuffer(allocPayload))
	allocReq.Header.Set("Authorization", "Bearer "+saToken)
	allocReq.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, allocReq)
	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 for allocate request, got %d: %s", rr.Code, rr.Body.String())
	}

	// 11. Create and Dispatch transfer of 25 items from WH1 to WH2
	trfPayload, _ := json.Marshal(map[string]interface{}{
		"request_id":             requestID,
		"source_location_id":      wh1ID,
		"destination_location_id": wh2ID,
		"carrier_name":           "SafeWay Logistics",
		"tracking_number":        "TRK-HTTP-100",
		"items": []map[string]interface{}{
			{
				"item_id":             itemID,
				"dispatched_quantity": 25,
				"unit_of_measure":     "pcs",
			},
		},
		"auto_dispatch": true,
	})
	trfReq, _ := http.NewRequest("POST", "/api/v1/inventory/transfers", bytes.NewBuffer(trfPayload))
	trfReq.Header.Set("Authorization", "Bearer "+saToken)
	trfReq.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, trfReq)
	if rr.Code != http.StatusOK && rr.Code != http.StatusCreated {
		t.Fatalf("Expected 200/201 for auto_dispatch transfer, got %d: %s", rr.Code, rr.Body.String())
	}
	var createdTrf map[string]interface{}
	_ = json.Unmarshal(rr.Body.Bytes(), &createdTrf)
	transferID := createdTrf["id"].(string)

	// 12. Confirm Receipt of 23 items (2 missing in transit)
	rcptPayload, _ := json.Marshal(map[string]interface{}{
		"items": []map[string]interface{}{
			{
				"item_id":           itemID,
				"received_quantity": 23,
			},
		},
		"discrepancy_reason": "missing_in_transit",
	})
	rcptReq, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/inventory/transfers/%s/receive", transferID), bytes.NewBuffer(rcptPayload))
	rcptReq.Header.Set("Authorization", "Bearer "+saToken)
	rcptReq.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, rcptReq)
	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 for confirm receipt, got %d: %s", rr.Code, rr.Body.String())
	}
	var rcptResp map[string]interface{}
	_ = json.Unmarshal(rr.Body.Bytes(), &rcptResp)
	if rcptResp["status"] != "disputed" {
		t.Errorf("Expected transfer status 'disputed' due to 2 missing items, got %s", rcptResp["status"])
	}

	// 13. Check Summary KPI
	sumReq, _ := http.NewRequest("GET", "/api/v1/inventory/summary", nil)
	sumReq.Header.Set("Authorization", "Bearer "+saToken)
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, sumReq)
	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 for GET /summary, got %d", rr.Code)
	}

	_ = ctx
}
