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

	"github.com/jackc/pgx/v5/pgtype"
	"ratels-api/internal/db"
	"ratels-api/internal/security"
	"ratels-api/internal/service"
)

func createHandlerTestUser(t *testing.T, h *TestHarness, role, email string) (*db.User, string) {
	t.Helper()
	ctx := context.Background()
	hash, err := security.HashPassword("TestPass123!")
	if err != nil {
		t.Fatalf("failed hashing password: %v", err)
	}

	userID := security.GenerateUUID()
	user, err := h.Repo.Queries().CreateUser(ctx, db.CreateUserParams{
		ID:                  userID,
		Email:               service.Text(&email),
		PasswordHash:        hash,
		DisplayName:         "Test User " + role,
		MembershipReference: fmt.Sprintf("REF-%s-%d", role, time.Now().UnixNano()),
		Status:              "active",
		PrimaryRole:         role,
		IsVerified:          true,
	})
	if err != nil {
		t.Fatalf("failed creating user: %v", err)
	}

	jti := security.GenerateJTI()
	_, _ = h.Repo.Queries().CreateUserSession(ctx, db.CreateUserSessionParams{
		ID:        security.GenerateUUID(),
		UserID:    user.ID,
		TokenJti:  jti,
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(24 * time.Hour), Valid: true},
	})

	token, _, _ := security.GenerateJWT(user.ID, email, role, jti, h.Config.JWTSecret, 24)
	return &user, token
}

func TestPublicReportsEndpoints_UnauthenticatedAccess(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()

	// 1. Overview
	req, _ := http.NewRequest("GET", "/api/v1/public/reports/overview", nil)
	rr := httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected 404 for unpublished for /public/reports/overview, got %d: %s", rr.Code, rr.Body.String())
	}

	// 2. Cleanups
	req, _ = http.NewRequest("GET", "/api/v1/public/reports/cleanups", nil)
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200 for /public/reports/cleanups, got %d", rr.Code)
	}

	// 3. Participation
	req, _ = http.NewRequest("GET", "/api/v1/public/reports/participation", nil)
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200 for /public/reports/participation, got %d", rr.Code)
	}

	// 4. Finance
	req, _ = http.NewRequest("GET", "/api/v1/public/reports/finance", nil)
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected 404 for unpublished /public/reports/finance, got %d: %s", rr.Code, rr.Body.String())
	}

	// 5. Foundation
	req, _ = http.NewRequest("GET", "/api/v1/public/reports/foundation", nil)
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected 404 for unpublished /public/reports/foundation, got %d: %s", rr.Code, rr.Body.String())
	}

	// 6. Projects
	req, _ = http.NewRequest("GET", "/api/v1/public/reports/projects", nil)
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200 for /public/reports/projects, got %d", rr.Code)
	}

	// 7. Methodology
	req, _ = http.NewRequest("GET", "/api/v1/public/reports/methodology", nil)
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200 for /public/reports/methodology, got %d", rr.Code)
	}

	// 8. Publications list
	req, _ = http.NewRequest("GET", "/api/v1/public/reports/publications", nil)
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200 for /public/reports/publications, got %d", rr.Code)
	}

	// 9. Alias route /api/v1/reports/overview
	req, _ = http.NewRequest("GET", "/api/v1/reports/overview", nil)
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected 404 for unpublished for /reports/overview alias, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestAdminReports_RBACGuards(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()

	// 1. Unauthenticated request to /admin/reports is blocked (401 or 403)
	req, _ := http.NewRequest("GET", "/api/v1/admin/reports", nil)
	rr := httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized && rr.Code != http.StatusForbidden {
		t.Errorf("Expected 401 or 403 for unauthenticated access to /admin/reports, got %d", rr.Code)
	}

	// 2. Authenticated as ordinary member is blocked (403 Forbidden)
	uid := time.Now().UnixNano()
	_, memberToken := createHandlerTestUser(t, h, "member", fmt.Sprintf("member_rep_%d@test.com", uid))

	req, _ = http.NewRequest("GET", "/api/v1/admin/reports", nil)
	req.Header.Set("Authorization", "Bearer "+memberToken)
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("Expected 403 Forbidden for member accessing /admin/reports, got %d", rr.Code)
	}

	// 3. Authenticated as super_admin succeeds (200 OK)
	_, superAdminToken := createHandlerTestUser(t, h, "super_admin", fmt.Sprintf("superadmin_hrep_%d@test.com", uid))

	req, _ = http.NewRequest("GET", "/api/v1/admin/reports", nil)
	req.Header.Set("Authorization", "Bearer "+superAdminToken)
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200 OK for super_admin accessing /admin/reports, got %d", rr.Code)
	}
}

func TestAdminReports_GenerationAndApprovalFlowHTTP(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()

	uid := time.Now().UnixNano()
	creator, creatorToken := createHandlerTestUser(t, h, "national_admin", fmt.Sprintf("creator_%d@test.com", uid))
	_, reviewerToken := createHandlerTestUser(t, h, "super_admin", fmt.Sprintf("reviewer_%d@test.com", uid))

	// 1. Generate snapshot via POST /api/v1/admin/reports/generate
	genPayload := map[string]interface{}{
		"report_type":      "cleanups",
		"title":            "Q4 Cleanups Report",
		"reporting_period": "2026",
		"period_start":     "2026-01-01",
		"period_end":       "2026-12-31",
		"scope_type":       "national",
	}
	body, _ := json.Marshal(genPayload)
	req, _ := http.NewRequest("POST", "/api/v1/admin/reports/generate", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+creatorToken)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("Expected 201 Created for generation, got %d: %s", rr.Code, rr.Body.String())
	}

	var snapDetail service.AdminSnapshotDetailDTO
	if err := json.Unmarshal(rr.Body.Bytes(), &snapDetail); err != nil {
		t.Fatalf("Failed unmarshaling snapshot detail: %v", err)
	}
	snapshotID := snapDetail.SnapshotID

	// 2. Submit for review via POST /api/v1/admin/reports/{id}/submit-review
	req, _ = http.NewRequest("POST", fmt.Sprintf("/api/v1/admin/reports/%s/submit-review", snapshotID), nil)
	req.Header.Set("Authorization", "Bearer "+creatorToken)
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK submitting review, got %d: %s", rr.Code, rr.Body.String())
	}

	// 3. Creator attempts self-approval -> must return 403 Forbidden
	req, _ = http.NewRequest("POST", fmt.Sprintf("/api/v1/admin/reports/%s/approve", snapshotID), bytes.NewReader([]byte(`{"decision_reason":"Self approving"}`)))
	req.Header.Set("Authorization", "Bearer "+creatorToken)
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("Expected 403 Forbidden on self-approval attempt, got %d", rr.Code)
	}
	_ = creator

	// 4. Reviewer approves -> 200 OK
	req, _ = http.NewRequest("POST", fmt.Sprintf("/api/v1/admin/reports/%s/approve", snapshotID), bytes.NewReader([]byte(`{"decision_reason":"Approved by governance committee"}`)))
	req.Header.Set("Authorization", "Bearer "+reviewerToken)
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK approving snapshot, got %d: %s", rr.Code, rr.Body.String())
	}

	// 5. Publish -> 200 OK
	req, _ = http.NewRequest("POST", fmt.Sprintf("/api/v1/admin/reports/%s/publish", snapshotID), nil)
	req.Header.Set("Authorization", "Bearer "+reviewerToken)
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK publishing snapshot, got %d: %s", rr.Code, rr.Body.String())
	}

	// 6. Test PDF and CSV downloads via public endpoint
	req, _ = http.NewRequest("GET", fmt.Sprintf("/api/v1/public/reports/publications/%s/pdf", snapshotID), nil)
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK downloading PDF, got %d", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/pdf" {
		t.Errorf("Expected application/pdf, got %s", ct)
	}

	req, _ = http.NewRequest("GET", fmt.Sprintf("/api/v1/public/reports/publications/%s/csv", snapshotID), nil)
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK downloading CSV, got %d", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "text/csv; charset=utf-8" {
		t.Errorf("Expected text/csv, got %s", ct)
	}
}

func TestAdminReports_GeographicValidationHTTP400(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()

	uid := time.Now().UnixNano()
	_, adminToken := createHandlerTestUser(t, h, "national_admin", fmt.Sprintf("geo_val_http_%d@test.com", uid))

	// Get a state for testing
	states, err := h.Repo.Queries().ListStates(context.Background())
	if err != nil || len(states) == 0 {
		t.Fatalf("states required: %v", err)
	}
	stateA := states[0]

	lgas, err := h.Repo.Queries().ListLGAsByState(context.Background(), stateA.ID)
	if err != nil || len(lgas) == 0 {
		t.Fatalf("lgas required: %v", err)
	}
	lgaA := lgas[0]

	cases := []struct {
		name    string
		payload map[string]interface{}
	}{
		{
			name: "Unsupported scope type",
			payload: map[string]interface{}{
				"report_type":      "cleanups",
				"title":            "Invalid Scope Report",
				"reporting_period": "2026",
				"scope_type":       "intergalactic",
			},
		},
		{
			name: "State scope without state_id",
			payload: map[string]interface{}{
				"report_type":      "cleanups",
				"title":            "Missing State ID Report",
				"reporting_period": "2026",
				"scope_type":       "state",
			},
		},
		{
			name: "Zone scope without zone_id",
			payload: map[string]interface{}{
				"report_type":      "cleanups",
				"title":            "Missing Zone ID Report",
				"reporting_period": "2026",
				"scope_type":       "zone",
			},
		},
		{
			name: "LGA scope without lga_id",
			payload: map[string]interface{}{
				"report_type":      "cleanups",
				"title":            "Missing LGA ID Report",
				"reporting_period": "2026",
				"scope_type":       "lga",
			},
		},
		{
			name: "Foundation report with LGA scope",
			payload: map[string]interface{}{
				"report_type":      "foundation",
				"title":            "Foundation LGA Report",
				"reporting_period": "2026",
				"scope_type":       "lga",
				"lga_id":           lgaA.ID,
			},
		},
		{
			name: "Conflicting geographic identifiers on national scope",
			payload: map[string]interface{}{
				"report_type":      "cleanups",
				"title":            "Conflicting Scope Report",
				"reporting_period": "2026",
				"scope_type":       "national",
				"state_id":         stateA.ID,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body, _ := json.Marshal(tc.payload)
			req, _ := http.NewRequest("POST", "/api/v1/admin/reports/generate", bytes.NewBuffer(body))
			req.Header.Set("Authorization", "Bearer "+adminToken)
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()
			h.Router.ServeHTTP(rr, req)

			if rr.Code != http.StatusBadRequest {
				t.Fatalf("expected HTTP 400 Bad Request, got HTTP %d: %s", rr.Code, rr.Body.String())
			}

			var resp map[string]interface{}
			_ = json.Unmarshal(rr.Body.Bytes(), &resp)
			errObj, ok := resp["error"].(map[string]interface{})
			if !ok {
				t.Fatalf("expected error response format, got: %s", rr.Body.String())
			}
			if errObj["code"] != "INVALID_REQUEST" {
				t.Errorf("expected error code INVALID_REQUEST, got %v", errObj["code"])
			}
		})
	}
}
