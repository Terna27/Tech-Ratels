package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"ratels-api/internal/db"
	"ratels-api/internal/service"
)

func strPtr(s string) *string {
	return &s
}

func cleanupTestAppointments(h *TestHarness) {
	_, _ = h.Pool.Exec(context.Background(), `
		DELETE FROM appointment_history WHERE appointment_id IN (
			SELECT id FROM appointments WHERE approval_reference LIKE 'APPT-%' OR approval_reference LIKE 'REPLACE-%'
		);
		DELETE FROM appointments WHERE approval_reference LIKE 'APPT-%' OR approval_reference LIKE 'REPLACE-%';
	`)
}

func getVacantStateID(t *testing.T, h *TestHarness) string {
	t.Helper()
	var stateID string
	err := h.Pool.QueryRow(context.Background(), `
		SELECT s.id 
		FROM states s 
		WHERE NOT EXISTS (
			SELECT 1 FROM appointments a 
			WHERE a.jurisdiction_id = s.id 
			  AND a.role_type = 'state_coordinator' 
			  AND a.status = 'active'
		) 
		ORDER BY s.code
		LIMIT 1
	`).Scan(&stateID)
	if err != nil {
		t.Fatalf("Failed to query vacant state: %v", err)
	}
	return stateID
}

func getVacantZoneCode(t *testing.T, h *TestHarness) string {
	t.Helper()
	var zoneCode string
	err := h.Pool.QueryRow(context.Background(), `
		SELECT z.code 
		FROM zones z 
		WHERE NOT EXISTS (
			SELECT 1 FROM appointments a 
			WHERE (a.jurisdiction_id = z.id OR a.jurisdiction_id = z.code) 
			  AND a.role_type = 'zonal_coordinator' 
			  AND a.status = 'active'
		) 
		ORDER BY z.code
		LIMIT 1
	`).Scan(&zoneCode)
	if err != nil {
		t.Fatalf("Failed to query vacant zone: %v", err)
	}
	return zoneCode
}

func TestNationwideCoordinator_SuccessfulAppointments(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()
	cleanupTestAppointments(h)
	defer cleanupTestAppointments(h)

	_, superAdminToken := createTestUserWithRole(t, h, "lead_root_app", "Super Administrator", "super_admin")
	candUserID, _ := createTestUserWithRole(t, h, "lead_cand_1", "Candidate One", "member")
	candUser2ID, _ := createTestUserWithRole(t, h, "lead_cand_2", "Candidate Two", "member")

	stateID := getVacantStateID(t, h)

	// 1. Appoint State Coordinator
	stateAppPayload, _ := json.Marshal(service.CreateAppointmentRequest{
		UserID:            candUserID,
		RoleTitle:         "State Coordinator - Test State",
		RoleType:          "state_coordinator",
		JurisdictionType:  "state",
		JurisdictionID:    strPtr(stateID),
		StartDate:         time.Now(),
		ApprovalReference: "APPT-2026-HQ-TEST-001",
	})
	req, _ := http.NewRequest("POST", "/api/v1/leadership/appointments", bytes.NewBuffer(stateAppPayload))
	req.Header.Set("Authorization", "Bearer "+superAdminToken)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("Expected 201 Created for state coordinator appointment, got %d: %s", rr.Code, rr.Body.String())
	}

	var createdStateApp db.Appointment
	_ = json.Unmarshal(rr.Body.Bytes(), &createdStateApp)
	if createdStateApp.RoleType != "state_coordinator" || createdStateApp.Status != "active" {
		t.Fatalf("Unexpected appointment state: %+v", createdStateApp)
	}

	zoneCode := getVacantZoneCode(t, h)

	// 2. Appoint Zonal Coordinator
	zonalAppPayload, _ := json.Marshal(service.CreateAppointmentRequest{
		UserID:            candUser2ID,
		RoleTitle:         "Zonal Coordinator - Vacant Zone",
		RoleType:          "zonal_coordinator",
		JurisdictionType:  "zone",
		JurisdictionID:    strPtr(zoneCode),
		StartDate:         time.Now(),
		ApprovalReference: "APPT-2026-HQ-TEST-002",
	})
	req2, _ := http.NewRequest("POST", "/api/v1/leadership/appointments", bytes.NewBuffer(zonalAppPayload))
	req2.Header.Set("Authorization", "Bearer "+superAdminToken)
	req2.Header.Set("Content-Type", "application/json")
	rr2 := httptest.NewRecorder()
	h.Router.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusCreated {
		t.Fatalf("Expected 201 Created for zonal coordinator appointment, got %d: %s", rr2.Code, rr2.Body.String())
	}
}

func TestNationwideCoordinator_DuplicateAppointmentRejection(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()
	cleanupTestAppointments(h)
	defer cleanupTestAppointments(h)

	_, superAdminToken := createTestUserWithRole(t, h, "lead_root_dup", "Super Administrator", "super_admin")
	userAID, _ := createTestUserWithRole(t, h, "lead_dup_a", "Candidate Alpha", "member")
	userBID, _ := createTestUserWithRole(t, h, "lead_dup_b", "Candidate Beta", "member")

	stateID := getVacantStateID(t, h)

	// First appointment succeeds
	p1, _ := json.Marshal(service.CreateAppointmentRequest{
		UserID:            userAID,
		RoleTitle:         "State Coordinator - Lagos",
		RoleType:          "state_coordinator",
		JurisdictionType:  "state",
		JurisdictionID:    strPtr(stateID),
		StartDate:         time.Now(),
		ApprovalReference: "APPT-DUP-01",
	})
	req1, _ := http.NewRequest("POST", "/api/v1/leadership/appointments", bytes.NewBuffer(p1))
	req1.Header.Set("Authorization", "Bearer "+superAdminToken)
	req1.Header.Set("Content-Type", "application/json")
	rr1 := httptest.NewRecorder()
	h.Router.ServeHTTP(rr1, req1)
	if rr1.Code != http.StatusCreated {
		t.Fatalf("First appointment failed: %d: %s", rr1.Code, rr1.Body.String())
	}

	// Second appointment to SAME jurisdiction must be REJECTED with 409 Conflict
	p2, _ := json.Marshal(service.CreateAppointmentRequest{
		UserID:            userBID,
		RoleTitle:         "State Coordinator - Lagos (Duplicate Attempt)",
		RoleType:          "state_coordinator",
		JurisdictionType:  "state",
		JurisdictionID:    strPtr(stateID),
		StartDate:         time.Now(),
		ApprovalReference: "APPT-DUP-02",
	})
	req2, _ := http.NewRequest("POST", "/api/v1/leadership/appointments", bytes.NewBuffer(p2))
	req2.Header.Set("Authorization", "Bearer "+superAdminToken)
	req2.Header.Set("Content-Type", "application/json")
	rr2 := httptest.NewRecorder()
	h.Router.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusConflict {
		t.Fatalf("Expected 409 Conflict for duplicate active appointment, got %d: %s", rr2.Code, rr2.Body.String())
	}
	if !bytes.Contains(rr2.Body.Bytes(), []byte("DUPLICATE_ACTIVE_APPOINTMENT")) {
		t.Fatalf("Expected DUPLICATE_ACTIVE_APPOINTMENT error message, got: %s", rr2.Body.String())
	}
}

func TestNationwideCoordinator_InvalidJurisdictionCombinations(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()

	_, superAdminToken := createTestUserWithRole(t, h, "lead_root_inv", "Super Administrator", "super_admin")
	userAID, _ := createTestUserWithRole(t, h, "lead_inv_user", "Candidate User", "member")

	cases := []struct {
		name             string
		roleType         string
		jurisdictionType string
		jurisdictionID   *string
	}{
		{
			name:             "State coordinator with zone jurisdiction",
			roleType:         "state_coordinator",
			jurisdictionType: "zone",
			jurisdictionID:   strPtr("SW"),
		},
		{
			name:             "Zonal coordinator with national jurisdiction",
			roleType:         "zonal_coordinator",
			jurisdictionType: "national",
			jurisdictionID:   nil,
		},
		{
			name:             "State coordinator with non-existent state ID",
			roleType:         "state_coordinator",
			jurisdictionType: "state",
			jurisdictionID:   strPtr("non-existent-state-uuid-9999"),
		},
		{
			name:             "National admin with zone jurisdiction ID",
			roleType:         "national_admin",
			jurisdictionType: "national",
			jurisdictionID:   strPtr("NC"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			payload, _ := json.Marshal(service.CreateAppointmentRequest{
				UserID:            userAID,
				RoleTitle:         "Invalid Test Role",
				RoleType:          tc.roleType,
				JurisdictionType:  tc.jurisdictionType,
				JurisdictionID:    tc.jurisdictionID,
				StartDate:         time.Now(),
				ApprovalReference: "APPT-INV-TEST",
			})
			req, _ := http.NewRequest("POST", "/api/v1/leadership/appointments", bytes.NewBuffer(payload))
			req.Header.Set("Authorization", "Bearer "+superAdminToken)
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()
			h.Router.ServeHTTP(rr, req)

			if rr.Code != http.StatusBadRequest {
				t.Errorf("[%s] Expected 400 Bad Request, got %d: %s", tc.name, rr.Code, rr.Body.String())
			}
		})
	}
}

func TestNationwideCoordinator_UnauthorizedAppointments(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()
	cleanupTestAppointments(h)
	defer cleanupTestAppointments(h)

	_, memberToken := createTestUserWithRole(t, h, "lead_unauth_mbr", "Member", "member")
	_, natAdminToken := createTestUserWithRole(t, h, "lead_unauth_nat", "National Admin", "national_admin")
	_, stateCoordToken := createTestUserWithRole(t, h, "lead_unauth_st", "State Coordinator", "state_coordinator")
	candUserID, _ := createTestUserWithRole(t, h, "lead_cand_target", "Candidate", "member")

	stateID := getVacantStateID(t, h)

	// 1. Member cannot appoint -> 403 Forbidden
	payload, _ := json.Marshal(service.CreateAppointmentRequest{
		UserID:            candUserID,
		RoleTitle:         "State Coordinator - Lagos",
		RoleType:          "state_coordinator",
		JurisdictionType:  "state",
		JurisdictionID:    strPtr(stateID),
		StartDate:         time.Now(),
		ApprovalReference: "APPT-UNAUTH",
	})
	req, _ := http.NewRequest("POST", "/api/v1/leadership/appointments", bytes.NewBuffer(payload))
	req.Header.Set("Authorization", "Bearer "+memberToken)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("Expected 403 for member appointment attempt, got %d", rr.Code)
	}

	// 2. State Coordinator cannot appoint state coordinator -> 403 Forbidden
	req2, _ := http.NewRequest("POST", "/api/v1/leadership/appointments", bytes.NewBuffer(payload))
	req2.Header.Set("Authorization", "Bearer "+stateCoordToken)
	req2.Header.Set("Content-Type", "application/json")
	rr2 := httptest.NewRecorder()
	h.Router.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusForbidden {
		t.Errorf("Expected 403 for state coordinator appointing state coordinator, got %d", rr2.Code)
	}

	// 3. National Admin cannot appoint state coordinator (Super Admin exclusive authority) -> 403 Forbidden
	req3, _ := http.NewRequest("POST", "/api/v1/leadership/appointments", bytes.NewBuffer(payload))
	req3.Header.Set("Authorization", "Bearer "+natAdminToken)
	req3.Header.Set("Content-Type", "application/json")
	rr3 := httptest.NewRecorder()
	h.Router.ServeHTTP(rr3, req3)
	if rr3.Code != http.StatusForbidden {
		t.Errorf("Expected 403 for national admin appointing state coordinator, got %d: %s", rr3.Code, rr3.Body.String())
	}
}

func TestNationwideCoordinator_CoordinatorReplacement(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()
	cleanupTestAppointments(h)
	defer cleanupTestAppointments(h)

	_, superAdminToken := createTestUserWithRole(t, h, "lead_rep_root", "Super Administrator", "super_admin")
	incumbentID, _ := createTestUserWithRole(t, h, "lead_rep_old", "Old Coordinator", "member")
	successorID, _ := createTestUserWithRole(t, h, "lead_rep_new", "New Coordinator", "member")

	stateID := getVacantStateID(t, h)

	// 1. Initial Appointment
	p1, _ := json.Marshal(service.CreateAppointmentRequest{
		UserID:            incumbentID,
		RoleTitle:         "State Coordinator - Lagos",
		RoleType:          "state_coordinator",
		JurisdictionType:  "state",
		JurisdictionID:    strPtr(stateID),
		StartDate:         time.Now().Add(-30 * 24 * time.Hour),
		ApprovalReference: "APPT-INIT-01",
	})
	req1, _ := http.NewRequest("POST", "/api/v1/leadership/appointments", bytes.NewBuffer(p1))
	req1.Header.Set("Authorization", "Bearer "+superAdminToken)
	req1.Header.Set("Content-Type", "application/json")
	rr1 := httptest.NewRecorder()
	h.Router.ServeHTTP(rr1, req1)
	if rr1.Code != http.StatusCreated {
		t.Fatalf("Initial appointment failed: %d: %s", rr1.Code, rr1.Body.String())
	}

	var initialApp db.Appointment
	_ = json.Unmarshal(rr1.Body.Bytes(), &initialApp)

	// 2. Execute Replacement Workflow
	repPayload, _ := json.Marshal(service.ReplaceAppointmentRequest{
		NewUserID:     successorID,
		EffectiveDate: time.Now(),
		Reason:        "Rotational governance replacement following completion of mandate",
	})
	req2, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/leadership/appointments/%s/replace", initialApp.ID), bytes.NewBuffer(repPayload))
	req2.Header.Set("Authorization", "Bearer "+superAdminToken)
	req2.Header.Set("Content-Type", "application/json")
	rr2 := httptest.NewRecorder()
	h.Router.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusCreated {
		t.Fatalf("Replacement failed: %d: %s", rr2.Code, rr2.Body.String())
	}

	var replacementApp db.Appointment
	_ = json.Unmarshal(rr2.Body.Bytes(), &replacementApp)
	if replacementApp.UserID != successorID || replacementApp.Status != "active" {
		t.Fatalf("Unexpected replacement appointment state: %+v", replacementApp)
	}

	// 3. Verify Old Appointment status is now 'replaced'
	oldApp, err := h.Repo.Queries().GetAppointmentByID(context.Background(), initialApp.ID)
	if err != nil {
		t.Fatalf("Failed to fetch old appointment: %v", err)
	}
	if oldApp.Status != "replaced" {
		t.Fatalf("Expected old appointment status 'replaced', got '%s'", oldApp.Status)
	}

	// 4. Verify History is recorded for old appointment
	hist, err := h.Repo.Queries().ListAppointmentHistory(context.Background(), initialApp.ID)
	if err != nil || len(hist) == 0 {
		t.Fatalf("Expected appointment history for replaced appointment, got err=%v len=%d", err, len(hist))
	}
	hasReplacedAction := false
	for _, rec := range hist {
		if rec.Action == "replaced" {
			hasReplacedAction = true
			break
		}
	}
	if !hasReplacedAction {
		t.Fatalf("Expected 'replaced' action in appointment history: %+v", hist)
	}
}

func TestNationwideCoordinator_SuspensionAndReactivation(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()
	cleanupTestAppointments(h)
	defer cleanupTestAppointments(h)

	_, superAdminToken := createTestUserWithRole(t, h, "lead_susp_root", "Super Administrator", "super_admin")
	appointeeID, _ := createTestUserWithRole(t, h, "lead_susp_user", "Suspended Appointee", "member")

	stateID := getVacantStateID(t, h)

	// 1. Initial Appointment
	p1, _ := json.Marshal(service.CreateAppointmentRequest{
		UserID:            appointeeID,
		RoleTitle:         "State Coordinator - Lagos",
		RoleType:          "state_coordinator",
		JurisdictionType:  "state",
		JurisdictionID:    strPtr(stateID),
		StartDate:         time.Now(),
		ApprovalReference: "APPT-SUSP-TEST",
	})
	req1, _ := http.NewRequest("POST", "/api/v1/leadership/appointments", bytes.NewBuffer(p1))
	req1.Header.Set("Authorization", "Bearer "+superAdminToken)
	req1.Header.Set("Content-Type", "application/json")
	rr1 := httptest.NewRecorder()
	h.Router.ServeHTTP(rr1, req1)
	if rr1.Code != http.StatusCreated {
		t.Fatalf("Initial appointment failed: %d: %s", rr1.Code, rr1.Body.String())
	}

	var app db.Appointment
	_ = json.Unmarshal(rr1.Body.Bytes(), &app)

	// 2. Suspend Appointment
	suspPayload, _ := json.Marshal(map[string]string{
		"reason": "Temporary suspension pending administrative review",
	})
	req2, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/leadership/appointments/%s/suspend", app.ID), bytes.NewBuffer(suspPayload))
	req2.Header.Set("Authorization", "Bearer "+superAdminToken)
	req2.Header.Set("Content-Type", "application/json")
	rr2 := httptest.NewRecorder()
	h.Router.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusOK {
		t.Fatalf("Suspension failed: %d: %s", rr2.Code, rr2.Body.String())
	}

	suspendedApp, _ := h.Repo.Queries().GetAppointmentByID(context.Background(), app.ID)
	if suspendedApp.Status != "suspended" {
		t.Fatalf("Expected status 'suspended', got '%s'", suspendedApp.Status)
	}

	// 3. Reactivate Appointment
	reactPayload, _ := json.Marshal(map[string]string{
		"reason": "Administrative review concluded with full clearance",
	})
	req3, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/leadership/appointments/%s/reactivate", app.ID), bytes.NewBuffer(reactPayload))
	req3.Header.Set("Authorization", "Bearer "+superAdminToken)
	req3.Header.Set("Content-Type", "application/json")
	rr3 := httptest.NewRecorder()
	h.Router.ServeHTTP(rr3, req3)

	if rr3.Code != http.StatusOK {
		t.Fatalf("Reactivation failed: %d: %s", rr3.Code, rr3.Body.String())
	}

	reactivatedApp, _ := h.Repo.Queries().GetAppointmentByID(context.Background(), app.ID)
	if reactivatedApp.Status != "active" {
		t.Fatalf("Expected status 'active' after reactivation, got '%s'", reactivatedApp.Status)
	}

	// 4. Verify History has both suspended and reactivated events
	hist, err := h.Repo.Queries().ListAppointmentHistory(context.Background(), app.ID)
	if err != nil {
		t.Fatalf("Failed to fetch history: %v", err)
	}
	var hasSusp, hasReact bool
	for _, rec := range hist {
		if rec.Action == "suspended" {
			hasSusp = true
		}
		if rec.Action == "reactivated" {
			hasReact = true
		}
	}
	if !hasSusp || !hasReact {
		t.Fatalf("Expected both suspended and reactivated in history, got: %+v", hist)
	}
}

func TestNationwideCoordinator_AuditLoggingAndNotificationFailsafe(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()
	cleanupTestAppointments(h)
	defer cleanupTestAppointments(h)

	_, superAdminToken := createTestUserWithRole(t, h, "lead_aud_root", "Super Administrator", "super_admin")
	appointeeID, _ := createTestUserWithRole(t, h, "lead_aud_user", "Audited Appointee", "member")

	stateID := getVacantStateID(t, h)

	// Create Appointment
	p1, _ := json.Marshal(service.CreateAppointmentRequest{
		UserID:            appointeeID,
		RoleTitle:         "State Coordinator - Lagos",
		RoleType:          "state_coordinator",
		JurisdictionType:  "state",
		JurisdictionID:    strPtr(stateID),
		StartDate:         time.Now(),
		ApprovalReference: "APPT-AUDIT-LOG",
	})
	req, _ := http.NewRequest("POST", "/api/v1/leadership/appointments", bytes.NewBuffer(p1))
	req.Header.Set("Authorization", "Bearer "+superAdminToken)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("Appointment creation failed: %d", rr.Code)
	}

	// Verify Audit Event was recorded
	events, err := h.Repo.Queries().ListAuditEvents(context.Background(), db.ListAuditEventsParams{
		Limit:  10,
		Offset: 0,
	})
	if err != nil || len(events) == 0 {
		t.Fatalf("Expected audit events recorded, got err=%v len=%d", err, len(events))
	}

	hasAppointAudit := false
	for _, ev := range events {
		if ev.Action == "create_appointment" && ev.ResourceType == "appointment" {
			hasAppointAudit = true
			break
		}
	}
	if !hasAppointAudit {
		t.Fatalf("Audit record for 'create_appointment' not found: %+v", events)
	}
}

func TestNationwideCoordinator_ConcurrentAppointments(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()
	cleanupTestAppointments(h)
	defer cleanupTestAppointments(h)

	_, superAdminToken := createTestUserWithRole(t, h, "lead_conc_root", "Super Administrator", "super_admin")
	userAID, _ := createTestUserWithRole(t, h, "lead_conc_a", "Candidate A", "member")
	userBID, _ := createTestUserWithRole(t, h, "lead_conc_b", "Candidate B", "member")

	stateID := getVacantStateID(t, h)

	var wg sync.WaitGroup
	var codeA, codeB int

	wg.Add(2)
	go func() {
		defer wg.Done()
		p, _ := json.Marshal(service.CreateAppointmentRequest{
			UserID:            userAID,
			RoleTitle:         "State Coordinator - Lagos",
			RoleType:          "state_coordinator",
			JurisdictionType:  "state",
			JurisdictionID:    strPtr(stateID),
			StartDate:         time.Now(),
			ApprovalReference: "APPT-CONC-A",
		})
		r, _ := http.NewRequest("POST", "/api/v1/leadership/appointments", bytes.NewBuffer(p))
		r.Header.Set("Authorization", "Bearer "+superAdminToken)
		r.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()
		h.Router.ServeHTTP(recorder, r)
		codeA = recorder.Code
	}()

	go func() {
		defer wg.Done()
		p, _ := json.Marshal(service.CreateAppointmentRequest{
			UserID:            userBID,
			RoleTitle:         "State Coordinator - Lagos",
			RoleType:          "state_coordinator",
			JurisdictionType:  "state",
			JurisdictionID:    strPtr(stateID),
			StartDate:         time.Now(),
			ApprovalReference: "APPT-CONC-B",
		})
		r, _ := http.NewRequest("POST", "/api/v1/leadership/appointments", bytes.NewBuffer(p))
		r.Header.Set("Authorization", "Bearer "+superAdminToken)
		r.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()
		h.Router.ServeHTTP(recorder, r)
		codeB = recorder.Code
	}()

	wg.Wait()

	// Exactly one must succeed (201 Created), and the other must be rejected (409 Conflict)
	hasCreated := (codeA == http.StatusCreated || codeB == http.StatusCreated)
	hasConflict := (codeA == http.StatusConflict || codeB == http.StatusConflict)

	if !hasCreated || !hasConflict {
		t.Fatalf("Expected exactly one 201 Created and one 409 Conflict, got codeA=%d codeB=%d", codeA, codeB)
	}
}

func TestNationwideCoordinator_MemberSearchEndpoint(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()

	_, superAdminToken := createTestUserWithRole(t, h, "lead_srch_root", "Super Administrator", "super_admin")
	targetUserID, _ := createTestUserWithRole(t, h, "chibuike_ratels", "Chibuike Okafor", "member")

	// Search by name
	req, _ := http.NewRequest("GET", "/api/v1/members?search=Chibuike", nil)
	req.Header.Set("Authorization", "Bearer "+superAdminToken)
	rr := httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for member search, got %d: %s", rr.Code, rr.Body.String())
	}

	var results []service.UserSummary
	_ = json.Unmarshal(rr.Body.Bytes(), &results)
	if len(results) == 0 {
		t.Fatalf("Expected at least one matching member for query 'Chibuike', got 0")
	}

	found := false
	for _, m := range results {
		if m.ID == targetUserID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("Target user %s not found in search results: %+v", targetUserID, results)
	}
}

func TestNationwideCoordinator_SelfModificationProhibited(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()
	cleanupTestAppointments(h)
	defer cleanupTestAppointments(h)

	superAdminUserID, superAdminToken := createTestUserWithRole(t, h, "lead_self_root", "Super Administrator", "super_admin")

	stateID := getVacantStateID(t, h)

	// Super Admin cannot appoint themselves as state coordinator
	payload, _ := json.Marshal(service.CreateAppointmentRequest{
		UserID:            superAdminUserID,
		RoleTitle:         "State Coordinator - Self Appointment",
		RoleType:          "state_coordinator",
		JurisdictionType:  "state",
		JurisdictionID:    strPtr(stateID),
		StartDate:         time.Now(),
		ApprovalReference: "APPT-SELF-TEST",
	})
	req, _ := http.NewRequest("POST", "/api/v1/leadership/appointments", bytes.NewBuffer(payload))
	req.Header.Set("Authorization", "Bearer "+superAdminToken)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("Expected 403 Forbidden for self-appointment, got %d: %s", rr.Code, rr.Body.String())
	}
	if !bytes.Contains(rr.Body.Bytes(), []byte("SELF_MODIFICATION_PROHIBITED")) {
		t.Fatalf("Expected SELF_MODIFICATION_PROHIBITED in response, got: %s", rr.Body.String())
	}
}
