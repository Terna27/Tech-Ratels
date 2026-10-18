package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"ratels-api/internal/service"
)

func createTestUserWithRole(t *testing.T, h *TestHarness, prefix, name, role string) (userID, token string) {
	t.Helper()
	ctx := context.Background()
	userID, email, _, _, _ := registerAndVerifyMember(t, h, prefix, name)

	// Promote in DB to desired primary_role and active status
	_, err := h.Pool.Exec(ctx, "UPDATE users SET primary_role = $1, status = 'active' WHERE id = $2", role, userID)
	if err != nil {
		t.Fatalf("Failed to update user role to %s: %v", role, err)
	}

	// Login again to obtain JWT with updated role claims
	loginBytes, _ := json.Marshal(map[string]string{
		"username": email,
		"password": "TestPass123!",
	})
	req, _ := http.NewRequest("POST", "/api/v1/auth/login", bytes.NewBuffer(loginBytes))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("Re-login failed for %s: %d", email, rr.Code)
	}

	var loginResp map[string]interface{}
	_ = json.Unmarshal(rr.Body.Bytes(), &loginResp)
	token = loginResp["access_token"].(string)

	return userID, token
}

func TestSuperAdminAuthority_CoordinatorBoundaries(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()
	ctx := context.Background()

	superAdminID, superAdminToken := createTestUserWithRole(t, h, "sa_coord_test", "Super Admin", "super_admin")
	_, natAdminToken := createTestUserWithRole(t, h, "nat_admin_test", "National Admin", "national_admin")
	targetUserID, _ := createTestUserWithRole(t, h, "target_user", "Target User", "member")
	replacementUserID, _ := createTestUserWithRole(t, h, "repl_user", "Replacement User", "member")

	// Get a valid state (Lagos)
	states, err := h.GeoService.ListStates(ctx)
	if err != nil || len(states) == 0 {
		t.Fatalf("Failed to list states: %v", err)
	}
	stateID := states[0].ID

	// 1. Ordinary National Admin attempts to appoint a state coordinator -> 403 Forbidden
	apptPayload := map[string]interface{}{
		"user_id":           targetUserID,
		"role_title":        "State Coordinator",
		"role_type":         "state_coordinator",
		"jurisdiction_type": "state",
		"jurisdiction_id":   stateID,
		"start_date":        time.Now().Format(time.RFC3339),
	}
	payloadBytes, _ := json.Marshal(apptPayload)

	req, _ := http.NewRequest("POST", "/api/v1/leadership/appointments", bytes.NewBuffer(payloadBytes))
	req.Header.Set("Authorization", "Bearer "+natAdminToken)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("Expected 403 Forbidden for National Admin appointing State Coordinator, got %d: %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "ERR_SUPER_ADMIN_REQUIRED") && !strings.Contains(rr.Body.String(), "Super Admin") {
		t.Errorf("Expected ERR_SUPER_ADMIN_REQUIRED error code, got %s", rr.Body.String())
	}

	// 2. Super Admin appoints State Coordinator -> 201 Created
	req, _ = http.NewRequest("POST", "/api/v1/leadership/appointments", bytes.NewBuffer(payloadBytes))
	req.Header.Set("Authorization", "Bearer "+superAdminToken)
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("Expected 201 Created for Super Admin appointing State Coordinator, got %d: %s", rr.Code, rr.Body.String())
	}

	var apptResp map[string]interface{}
	_ = json.Unmarshal(rr.Body.Bytes(), &apptResp)
	appointmentID := apptResp["id"].(string)

	// 3. Ordinary National Admin attempts to replace State Coordinator -> 403 Forbidden
	replacePayload := map[string]interface{}{
		"new_user_id":    replacementUserID,
		"effective_date": time.Now().Format(time.RFC3339),
		"reason":         "Unauthorized replacement attempt by national admin",
	}
	replaceBytes, _ := json.Marshal(replacePayload)
	req, _ = http.NewRequest("POST", fmt.Sprintf("/api/v1/leadership/appointments/%s/replace", appointmentID), bytes.NewBuffer(replaceBytes))
	req.Header.Set("Authorization", "Bearer "+natAdminToken)
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("Expected 403 Forbidden for National Admin replacing coordinator, got %d", rr.Code)
	}

	// 4. Super Admin replaces State Coordinator -> 201 Created
	req, _ = http.NewRequest("POST", fmt.Sprintf("/api/v1/leadership/appointments/%s/replace", appointmentID), bytes.NewBuffer(replaceBytes))
	req.Header.Set("Authorization", "Bearer "+superAdminToken)
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("Expected 201 Created for Super Admin replacing coordinator, got %d: %s", rr.Code, rr.Body.String())
	}

	// 5. Verify Appointment History audit records
	req, _ = http.NewRequest("GET", fmt.Sprintf("/api/v1/leadership/appointments/%s/history", appointmentID), nil)
	req.Header.Set("Authorization", "Bearer "+superAdminToken)
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for fetching appointment history, got %d: %s", rr.Code, rr.Body.String())
	}

	var historyList []map[string]interface{}
	_ = json.Unmarshal(rr.Body.Bytes(), &historyList)
	if len(historyList) == 0 {
		t.Fatalf("Expected at least 1 history record, got %d", len(historyList))
	}

	foundReplacement := false
	for _, hRec := range historyList {
		if hRec["action"] == "replaced" {
			foundReplacement = true
			if hRec["previous_user_id"] != targetUserID {
				t.Errorf("Expected previous_user_id %s, got %v", targetUserID, hRec["previous_user_id"])
			}
			if hRec["new_user_id"] != replacementUserID {
				t.Errorf("Expected new_user_id %s, got %v", replacementUserID, hRec["new_user_id"])
			}
			if hRec["changed_by_user_id"] != superAdminID {
				t.Errorf("Expected changed_by_user_id %s, got %v", superAdminID, hRec["changed_by_user_id"])
			}
		}
	}
	if !foundReplacement {
		t.Errorf("Expected replacement history record with action='replaced'")
	}

	// 6. Ordinary National Admin attempts to appoint a Zonal Coordinator -> 403 Forbidden
	zones, err := h.GeoService.ListZones(ctx)
	if err == nil && len(zones) > 0 {
		zonalPayload, _ := json.Marshal(map[string]interface{}{
			"user_id":           replacementUserID,
			"role_title":        "Zonal Coordinator - North Central",
			"role_type":         "zonal_coordinator",
			"jurisdiction_type": "zone",
			"jurisdiction_id":   zones[0].ID,
			"start_date":        time.Now().Format(time.RFC3339),
		})
		req, _ = http.NewRequest("POST", "/api/v1/leadership/appointments", bytes.NewBuffer(zonalPayload))
		req.Header.Set("Authorization", "Bearer "+natAdminToken)
		req.Header.Set("Content-Type", "application/json")
		rr = httptest.NewRecorder()
		h.Router.ServeHTTP(rr, req)
		if rr.Code != http.StatusForbidden {
			t.Errorf("Expected 403 Forbidden for National Admin appointing Zonal Coordinator, got %d: %s", rr.Code, rr.Body.String())
		}
	}

	// 7. Ordinary National Admin attempts to modify scope and dates -> 403 Forbidden
	scopePayload, _ := json.Marshal(map[string]interface{}{
		"jurisdiction_type": "state",
		"jurisdiction_id":   stateID,
		"start_date":        time.Now().Format(time.RFC3339),
		"reason":            "Unauthorized scope change attempt",
	})
	req, _ = http.NewRequest("PUT", fmt.Sprintf("/api/v1/leadership/appointments/%s/scope-dates", appointmentID), bytes.NewBuffer(scopePayload))
	req.Header.Set("Authorization", "Bearer "+natAdminToken)
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("Expected 403 Forbidden for National Admin modifying scope and dates, got %d", rr.Code)
	}

	// 8. Ordinary National Admin attempts to revoke coordinator appointment -> 403 Forbidden
	revokePayload, _ := json.Marshal(map[string]interface{}{
		"reason": "Unauthorized revocation attempt",
	})
	req, _ = http.NewRequest("POST", fmt.Sprintf("/api/v1/leadership/appointments/%s/revoke", appointmentID), bytes.NewBuffer(revokePayload))
	req.Header.Set("Authorization", "Bearer "+natAdminToken)
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("Expected 403 Forbidden for National Admin revoking coordinator, got %d", rr.Code)
	}

	// 9. Coordinator self-modification / self-promotion prevention -> 403 Forbidden
	_, targetUserToken := createTestUserWithRole(t, h, "target_token_test", "Target Token User", "state_coordinator")
	selfReplacePayload, _ := json.Marshal(map[string]interface{}{
		"new_user_id":    replacementUserID,
		"effective_date": time.Now().Format(time.RFC3339),
		"reason":         "Self-initiated handover attempt",
	})
	req, _ = http.NewRequest("POST", fmt.Sprintf("/api/v1/leadership/appointments/%s/replace", appointmentID), bytes.NewBuffer(selfReplacePayload))
	req.Header.Set("Authorization", "Bearer "+targetUserToken)
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("Expected 403 Forbidden for Coordinator attempting self-modification, got %d", rr.Code)
	}
}

func TestFinancialAccess_ClearanceAndSuperAdminRules(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()

	superAdminID, superAdminToken := createTestUserWithRole(t, h, "fin_sa", "Super Admin", "super_admin")
	_, natAdminToken := createTestUserWithRole(t, h, "fin_nat", "National Admin", "national_admin")
	staffID, staffToken := createTestUserWithRole(t, h, "fin_staff", "Finance Staff", "member")
	_, memberToken := createTestUserWithRole(t, h, "fin_mem", "Ordinary Member", "member")

	// 1. Ordinary Member attempts to access transactions -> 403 Forbidden
	req, _ := http.NewRequest("GET", "/api/v1/finance/transactions", nil)
	req.Header.Set("Authorization", "Bearer "+memberToken)
	rr := httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("Expected 403 Forbidden for member accessing finance, got %d", rr.Code)
	}

	// 2. National Admin without explicit financial authorization -> 403 Forbidden
	req, _ = http.NewRequest("GET", "/api/v1/finance/transactions", nil)
	req.Header.Set("Authorization", "Bearer "+natAdminToken)
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("Expected 403 Forbidden for un-cleared national admin, got %d", rr.Code)
	}

	// 3. Super Admin accesses transactions -> 200 OK
	req, _ = http.NewRequest("GET", "/api/v1/finance/transactions", nil)
	req.Header.Set("Authorization", "Bearer "+superAdminToken)
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for Super Admin accessing finance, got %d: %s", rr.Code, rr.Body.String())
	}

	// 4. Super Admin authorizes staff member with 'viewer' access
	authPayload := map[string]interface{}{
		"user_id":      staffID,
		"access_level": "viewer",
		"notes":        "Authorized financial audit assistant",
	}
	authBytes, _ := json.Marshal(authPayload)
	req, _ = http.NewRequest("POST", "/api/v1/finance/authorizations", bytes.NewBuffer(authBytes))
	req.Header.Set("Authorization", "Bearer "+superAdminToken)
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("Expected 201 Created authorizing staff member, got %d: %s", rr.Code, rr.Body.String())
	}
	var authResp map[string]interface{}
	_ = json.Unmarshal(rr.Body.Bytes(), &authResp)
	authID := authResp["id"].(string)

	// 5. Staff member can now access transactions and statement -> 200 OK
	req, _ = http.NewRequest("GET", "/api/v1/finance/transactions", nil)
	req.Header.Set("Authorization", "Bearer "+staffToken)
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for authorized staff accessing finance, got %d: %s", rr.Code, rr.Body.String())
	}

	req, _ = http.NewRequest("GET", "/api/v1/finance/statement", nil)
	req.Header.Set("Authorization", "Bearer "+staffToken)
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for authorized staff accessing statement, got %d: %s", rr.Code, rr.Body.String())
	}

	// 6. Super Admin revokes staff member's financial authorization
	req, _ = http.NewRequest("DELETE", fmt.Sprintf("/api/v1/finance/authorizations/%s", authID), nil)
	req.Header.Set("Authorization", "Bearer "+superAdminToken)
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK revoking staff authorization, got %d: %s", rr.Code, rr.Body.String())
	}

	// 7. Staff member immediately loses financial access -> 403 Forbidden
	req, _ = http.NewRequest("GET", "/api/v1/finance/transactions", nil)
	req.Header.Set("Authorization", "Bearer "+staffToken)
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("Expected 403 Forbidden after financial clearance revocation, got %d", rr.Code)
	}

	// Avoid unused variable warning
	_ = superAdminID
}

func TestFinancialStatement_ReconciliationAndCSV(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()

	_, superAdminToken := createTestUserWithRole(t, h, "fin_stmt_sa", "Statement Super Admin", "super_admin")

	// 1. Record a donation (Inflow: 2,500,000 Kobo = 25,000 NGN)
	destFund := "foundation_donation_fund"
	tx1Payload := map[string]interface{}{
		"idempotency_key":     fmt.Sprintf("stmt-tx-1-%d", time.Now().UnixNano()),
		"transaction_type":    "confirmed_donation",
		"amount":              2500000,
		"currency":            "NGN",
		"destination_fund_id": destFund,
		"notes":               "Generous public donor contribution",
		"auto_post":           true,
	}
	tx1Bytes, _ := json.Marshal(tx1Payload)
	req, _ := http.NewRequest("POST", "/api/v1/finance/transactions", bytes.NewBuffer(tx1Bytes))
	req.Header.Set("Authorization", "Bearer "+superAdminToken)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("Failed to create tx1: %d: %s", rr.Code, rr.Body.String())
	}

	// 2. Record a disbursement (Outflow: 500,000 Kobo = 5,000 NGN)
	tx2Payload := map[string]interface{}{
		"idempotency_key":   fmt.Sprintf("stmt-tx-2-%d", time.Now().UnixNano()),
		"transaction_type":  "disbursement",
		"amount":            500000,
		"currency":          "NGN",
		"source_fund_id":    destFund,
		"notes":             "Purchase of cleanup gloves and sacks",
		"auto_post":         true,
	}
	tx2Bytes, _ := json.Marshal(tx2Payload)
	req, _ = http.NewRequest("POST", "/api/v1/finance/transactions", bytes.NewBuffer(tx2Bytes))
	req.Header.Set("Authorization", "Bearer "+superAdminToken)
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("Failed to create tx2: %d: %s", rr.Code, rr.Body.String())
	}

	// 3. Test Statement Generation and Balance Reconciliation
	req, _ = http.NewRequest("GET", fmt.Sprintf("/api/v1/finance/statement?fund_id=%s", destFund), nil)
	req.Header.Set("Authorization", "Bearer "+superAdminToken)
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for financial statement, got %d: %s", rr.Code, rr.Body.String())
	}

	var stmt service.FinancialStatementResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &stmt); err != nil {
		t.Fatalf("Failed to decode statement: %v", err)
	}

	if stmt.ReconciliationStatus != "balanced_and_verified" {
		t.Errorf("Expected reconciliation_status 'balanced_and_verified', got '%s'", stmt.ReconciliationStatus)
	}
	if stmt.ConfirmedInflows < 2500000 {
		t.Errorf("Expected at least 2,500,000 in inflows, got %d", stmt.ConfirmedInflows)
	}
	if stmt.ConfirmedOutflows < 500000 {
		t.Errorf("Expected at least 500,000 in outflows, got %d", stmt.ConfirmedOutflows)
	}
	expectedClosing := stmt.OpeningBalance + stmt.ConfirmedInflows - stmt.ConfirmedOutflows - stmt.ProviderFees - stmt.RefundsAdjustments
	if stmt.ClosingBalance != expectedClosing {
		t.Errorf("Balance reconciliation mismatch! Expected %d, got %d", expectedClosing, stmt.ClosingBalance)
	}
	if stmt.AvailableToSpend != stmt.CashBalance-stmt.Commitments {
		t.Errorf("AvailableToSpend mismatch! Cash (%d) - Commitments (%d) != Available (%d)", stmt.CashBalance, stmt.Commitments, stmt.AvailableToSpend)
	}

	// 4. Test CSV Export Endpoint
	req, _ = http.NewRequest("GET", fmt.Sprintf("/api/v1/finance/export?fund_id=%s", destFund), nil)
	req.Header.Set("Authorization", "Bearer "+superAdminToken)
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for CSV export, got %d: %s", rr.Code, rr.Body.String())
	}
	contentType := rr.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/csv") {
		t.Errorf("Expected text/csv content-type, got '%s'", contentType)
	}
	csvBody := rr.Body.String()
	if !strings.HasPrefix(csvBody, "Reference,Type,Status,Amount_Kobo") {
		t.Errorf("CSV header mismatch! Got: %s", csvBody[:min(80, len(csvBody))])
	}
}

func TestSuperAdminFinancialDocumentation_AuthorityAndUnauthorizedBoundaries(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()

	_, superAdminToken := createTestUserWithRole(t, h, "doc_sa", "Super Admin", "super_admin")
	_, natAdminToken := createTestUserWithRole(t, h, "doc_nat", "National Admin", "national_admin")
	staffID, staffToken := createTestUserWithRole(t, h, "doc_staff", "Finance Staff", "member")
	_, memberToken := createTestUserWithRole(t, h, "doc_mem", "Ordinary Member", "member")

	// Super Admin authorizes staff member with 'submitter' access
	authPayload := map[string]interface{}{
		"user_id":      staffID,
		"access_level": "submitter",
		"notes":        "Authorized financial submitter",
	}
	authBytes, _ := json.Marshal(authPayload)
	req, _ := http.NewRequest("POST", "/api/v1/finance/authorizations", bytes.NewBuffer(authBytes))
	req.Header.Set("Authorization", "Bearer "+superAdminToken)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("Failed to authorize staff: %d", rr.Code)
	}

	// 1. Create a pending transaction
	txPayload := map[string]interface{}{
		"idempotency_key":     fmt.Sprintf("doc-auth-tx-%d", time.Now().UnixNano()),
		"transaction_type":    "confirmed_donation",
		"amount":              5000000,
		"currency":            "NGN",
		"destination_fund_id": "foundation_donation_fund",
		"notes":               "Test pending donation for approval authorization boundaries",
		"auto_post":           false,
	}
	txBytes, _ := json.Marshal(txPayload)
	req, _ = http.NewRequest("POST", "/api/v1/finance/transactions", bytes.NewBuffer(txBytes))
	req.Header.Set("Authorization", "Bearer "+superAdminToken)
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("Failed to create pending transaction: %d: %s", rr.Code, rr.Body.String())
	}
	var createdTx map[string]interface{}
	_ = json.Unmarshal(rr.Body.Bytes(), &createdTx)
	txID := createdTx["id"].(string)

	// --- SECTION A: APPROVAL BOUNDARIES (Only Super Admin can approve/finalize) ---
	// A1. Member attempts to approve transaction -> 403 Forbidden
	req, _ = http.NewRequest("POST", fmt.Sprintf("/api/v1/finance/transactions/%s/approve", txID), nil)
	req.Header.Set("Authorization", "Bearer "+memberToken)
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("Expected 403 Forbidden for Member approving transaction, got %d", rr.Code)
	}

	// A2. National Admin attempts to approve transaction -> 403 Forbidden
	req, _ = http.NewRequest("POST", fmt.Sprintf("/api/v1/finance/transactions/%s/approve", txID), nil)
	req.Header.Set("Authorization", "Bearer "+natAdminToken)
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("Expected 403 Forbidden for National Admin approving transaction, got %d", rr.Code)
	}

	// A3. Submitter/Staff member attempts to approve transaction -> 403 Forbidden
	req, _ = http.NewRequest("POST", fmt.Sprintf("/api/v1/finance/transactions/%s/approve", txID), nil)
	req.Header.Set("Authorization", "Bearer "+staffToken)
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("Expected 403 Forbidden for Finance Staff approving transaction, got %d", rr.Code)
	}

	// A4. Super Admin approves transaction -> 200 OK
	req, _ = http.NewRequest("POST", fmt.Sprintf("/api/v1/finance/transactions/%s/approve", txID), nil)
	req.Header.Set("Authorization", "Bearer "+superAdminToken)
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for Super Admin approving transaction, got %d: %s", rr.Code, rr.Body.String())
	}

	// --- SECTION B: REVERSAL / CORRECTION BOUNDARIES (Only Super Admin can reverse) ---
	// B1. Member attempts to reverse transaction -> 403 Forbidden
	revBody := bytes.NewBufferString(`{"reason":"Unauthorized reversal attempt"}`)
	req, _ = http.NewRequest("POST", fmt.Sprintf("/api/v1/finance/transactions/%s/reverse", txID), revBody)
	req.Header.Set("Authorization", "Bearer "+memberToken)
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("Expected 403 Forbidden for Member reversing transaction, got %d", rr.Code)
	}

	// B2. National Admin attempts to reverse transaction -> 403 Forbidden
	revBody = bytes.NewBufferString(`{"reason":"National admin reversal attempt"}`)
	req, _ = http.NewRequest("POST", fmt.Sprintf("/api/v1/finance/transactions/%s/reverse", txID), revBody)
	req.Header.Set("Authorization", "Bearer "+natAdminToken)
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("Expected 403 Forbidden for National Admin reversing transaction, got %d", rr.Code)
	}

	// B3. Finance Staff attempts to reverse transaction -> 403 Forbidden
	revBody = bytes.NewBufferString(`{"reason":"Staff reversal attempt"}`)
	req, _ = http.NewRequest("POST", fmt.Sprintf("/api/v1/finance/transactions/%s/reverse", txID), revBody)
	req.Header.Set("Authorization", "Bearer "+staffToken)
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("Expected 403 Forbidden for Finance Staff reversing transaction, got %d", rr.Code)
	}

	// B4. Super Admin reverses transaction -> 200 OK
	revBody = bytes.NewBufferString(`{"reason":"Legitimate corrective reversal by Super Admin"}`)
	req, _ = http.NewRequest("POST", fmt.Sprintf("/api/v1/finance/transactions/%s/reverse", txID), revBody)
	req.Header.Set("Authorization", "Bearer "+superAdminToken)
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for Super Admin reversing transaction, got %d: %s", rr.Code, rr.Body.String())
	}

	// --- SECTION C: UNAUTHORIZED FINANCIAL RECORD, STATEMENT & EXPORT ACCESS ---
	// C1. Member attempts to access statement -> 403 Forbidden
	req, _ = http.NewRequest("GET", "/api/v1/finance/statement", nil)
	req.Header.Set("Authorization", "Bearer "+memberToken)
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("Expected 403 Forbidden for Member accessing statement, got %d", rr.Code)
	}

	// C2. Un-cleared National Admin attempts to access statement -> 403 Forbidden
	req, _ = http.NewRequest("GET", "/api/v1/finance/statement", nil)
	req.Header.Set("Authorization", "Bearer "+natAdminToken)
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("Expected 403 Forbidden for un-cleared National Admin accessing statement, got %d", rr.Code)
	}

	// C3. Member attempts CSV export -> 403 Forbidden
	req, _ = http.NewRequest("GET", "/api/v1/finance/export", nil)
	req.Header.Set("Authorization", "Bearer "+memberToken)
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("Expected 403 Forbidden for Member accessing export, got %d", rr.Code)
	}

	// C4. Member attempts CSV export path /export/csv -> 403 Forbidden
	req, _ = http.NewRequest("GET", "/api/v1/finance/export/csv", nil)
	req.Header.Set("Authorization", "Bearer "+memberToken)
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("Expected 403 Forbidden for Member accessing export/csv, got %d", rr.Code)
	}

	// C5. Un-cleared National Admin attempts CSV export -> 403 Forbidden
	req, _ = http.NewRequest("GET", "/api/v1/finance/export", nil)
	req.Header.Set("Authorization", "Bearer "+natAdminToken)
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("Expected 403 Forbidden for un-cleared National Admin accessing export, got %d", rr.Code)
	}

	// C6. Member attempts to access transaction voucher detail -> 403 Forbidden
	req, _ = http.NewRequest("GET", fmt.Sprintf("/api/v1/finance/transactions/%s", txID), nil)
	req.Header.Set("Authorization", "Bearer "+memberToken)
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("Expected 403 Forbidden for Member accessing transaction detail, got %d", rr.Code)
	}

	// C7. Un-cleared National Admin attempts to access transaction voucher detail -> 403 Forbidden
	req, _ = http.NewRequest("GET", fmt.Sprintf("/api/v1/finance/transactions/%s", txID), nil)
	req.Header.Set("Authorization", "Bearer "+natAdminToken)
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("Expected 403 Forbidden for un-cleared National Admin accessing transaction detail, got %d", rr.Code)
	}

	// --- SECTION D: UNAUTHORIZED STAFF CLEARANCE GRANTS ---
	// D1. National Admin attempts to grant financial authorization -> 403 Forbidden
	req, _ = http.NewRequest("POST", "/api/v1/finance/authorizations", bytes.NewBuffer(authBytes))
	req.Header.Set("Authorization", "Bearer "+natAdminToken)
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("Expected 403 Forbidden for National Admin granting financial authorizations, got %d", rr.Code)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
