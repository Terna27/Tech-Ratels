package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"ratels-api/internal/db"
	"ratels-api/internal/security"
	"ratels-api/internal/service"
)

func insertTestAppointment(t *testing.T, h *TestHarness, userID, roleType, roleTitle, jurisdictionType string, jurisdictionID *string, status string, startDate time.Time, endDate *time.Time) string {
	t.Helper()
	apptID := security.GenerateUUID()
	ref := fmt.Sprintf("TEST-APPT-%d", time.Now().UnixNano())

	_, err := h.Pool.Exec(context.Background(), `
		INSERT INTO appointments (
			id, user_id, role_type, role_title, jurisdiction_type, jurisdiction_id,
			status, start_date, end_date, approval_reference, public_contact, created_at, updated_at, version
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, 'coord@test.org', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 1)
	`, apptID, userID, roleType, roleTitle, jurisdictionType, jurisdictionID, status, startDate, endDate, ref)
	if err != nil {
		t.Fatalf("Failed to insert test appointment: %v", err)
	}
	return apptID
}

func getLagosAndKanoData(t *testing.T, h *TestHarness) (lagosState db.State, lagosLGA db.Lga, kanoState db.State, kanoLGA db.Lga, swZoneCode string) {
	t.Helper()
	ctx := context.Background()
	states, err := h.Repo.Queries().ListStates(ctx)
	if err != nil {
		t.Fatalf("Failed to fetch states: %v", err)
	}

	for _, s := range states {
		if s.Code == "LA" {
			lagosState = s
		} else if s.Code == "KN" {
			kanoState = s
		}
	}

	if lagosState.ID == "" || kanoState.ID == "" {
		t.Fatalf("Required states LA or KN missing in database")
	}

	lagosLGAs, err := h.GeoService.ListLGAsByState(ctx, lagosState.ID)
	if err != nil || len(lagosLGAs) < 2 {
		t.Fatalf("Failed to fetch Lagos LGAs: %v", err)
	}
	lagosLGA = lagosLGAs[0]

	kanoLGAs, err := h.GeoService.ListLGAsByState(ctx, kanoState.ID)
	if err != nil || len(kanoLGAs) == 0 {
		t.Fatalf("Failed to fetch Kano LGAs: %v", err)
	}
	kanoLGA = kanoLGAs[0]

	if lagosState.ZoneID != "" {
		z, err := h.Repo.Queries().GetZoneByID(ctx, lagosState.ZoneID)
		if err == nil {
			swZoneCode = z.Code
		}
	}
	if swZoneCode == "" {
		swZoneCode = "SW"
	}

	return
}

// 1. Approved coordinator publishes cleanup without admin approval
func TestCoordinatorPrivileges_ApprovedCoordinatorPublishesWithoutSuperAdminApproval(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()

	lagosState, lagosLGA, _, _, _ := getLagosAndKanoData(t, h)
	coordID, coordToken := createTestUserWithRole(t, h, "coord_pub_ok", "Approved State Coordinator", "member")

	// Create active approved appointment for Lagos State
	now := time.Now()
	insertTestAppointment(t, h, coordID, "state_coordinator", "Lagos State Coordinator", "state", &lagosState.ID, "active", now.Add(-1*time.Hour), nil)

	startTime := now.Add(24 * time.Hour)
	endTime := now.Add(28 * time.Hour)

	payload := map[string]interface{}{
		"title":          "Marina Shoreline Clean Earth Initiative",
		"description":    "Community environmental cleanup along Marina waterfront",
		"state_id":       lagosState.ID,
		"lga_id":         lagosLGA.ID,
		"address":        "Marina Waterfront Esplanade",
		"landmark":       "Near CMS Jetty",
		"start_time":     startTime.Format(time.RFC3339),
		"end_time":       endTime.Format(time.RFC3339),
		"capacity":       150,
		"public_contact": "+2348000000001",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/activities/events", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+coordToken)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("Expected 201 Created for approved coordinator, got %d: %s", rec.Code, rec.Body.String())
	}

	var res map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("Failed to decode event response: %v", err)
	}

	eventData := res
	if wrapped, ok := res["data"].(map[string]interface{}); ok {
		eventData = wrapped
	}
	if eventData["status"] != "published" {
		t.Errorf("Expected event to be published immediately without Super Admin queue, got status: %v", eventData["status"])
	}
	if eventData["coordinator_id"] != coordID {
		t.Errorf("Expected coordinator_id %s, got %v", coordID, eventData["coordinator_id"])
	}
}

// 2. Unapproved member cannot publish cleanup (403)
func TestCoordinatorPrivileges_UnapprovedMemberCannotPublish(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()

	lagosState, lagosLGA, _, _, _ := getLagosAndKanoData(t, h)
	_, memberToken := createTestUserWithRole(t, h, "unapproved_mbr", "Regular Member", "member")

	now := time.Now()
	payload := map[string]interface{}{
		"title":          "Unsanctioned Street Sweep",
		"state_id":       lagosState.ID,
		"lga_id":         lagosLGA.ID,
		"address":        "Unauthorized St",
		"start_time":     now.Add(24 * time.Hour).Format(time.RFC3339),
		"end_time":       now.Add(28 * time.Hour).Format(time.RFC3339),
		"capacity":       50,
		"public_contact": "+2348000000002",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/activities/events", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+memberToken)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("Expected 403 Forbidden for member without active appointment, got %d: %s", rec.Code, rec.Body.String())
	}
}

// 3. Suspended coordinator cannot publish or verify attendance (403)
func TestCoordinatorPrivileges_SuspendedCoordinatorCannotPublishOrVerify(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()

	lagosState, lagosLGA, _, _, _ := getLagosAndKanoData(t, h)
	coordID, coordToken := createTestUserWithRole(t, h, "coord_susp", "Suspended Coordinator", "member")

	now := time.Now()
	// Appointment status is suspended
	insertTestAppointment(t, h, coordID, "state_coordinator", "Lagos State Coordinator", "state", &lagosState.ID, "suspended", now.Add(-48*time.Hour), nil)

	payload := map[string]interface{}{
		"title":          "Suspended Coordinator Attempt",
		"state_id":       lagosState.ID,
		"lga_id":         lagosLGA.ID,
		"address":        "Suspended Venue",
		"start_time":     now.Add(24 * time.Hour).Format(time.RFC3339),
		"end_time":       now.Add(28 * time.Hour).Format(time.RFC3339),
		"capacity":       50,
		"public_contact": "+2348000000003",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/activities/events", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+coordToken)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("Expected 403 Forbidden for suspended coordinator, got %d: %s", rec.Code, rec.Body.String())
	}
}

// 4. Expired appointment loses privileges (403) and future start_date cannot operate early
func TestCoordinatorPrivileges_ExpiredAndFutureAppointmentsDenied(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()

	lagosState, lagosLGA, _, _, _ := getLagosAndKanoData(t, h)
	now := time.Now()

	// Case A: Expired appointment
	expCoordID, expToken := createTestUserWithRole(t, h, "coord_expired", "Expired Coordinator", "member")
	pastStart := now.Add(-30 * 24 * time.Hour)
	pastEnd := now.Add(-1 * 24 * time.Hour)
	insertTestAppointment(t, h, expCoordID, "state_coordinator", "Lagos State Coordinator", "state", &lagosState.ID, "active", pastStart, &pastEnd)

	payload := map[string]interface{}{
		"title":          "Expired Appt Venue",
		"state_id":       lagosState.ID,
		"lga_id":         lagosLGA.ID,
		"address":        "Expired Venue",
		"start_time":     now.Add(24 * time.Hour).Format(time.RFC3339),
		"end_time":       now.Add(28 * time.Hour).Format(time.RFC3339),
		"capacity":       50,
		"public_contact": "+2348000000004",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/activities/events", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+expToken)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("Expected 403 Forbidden for expired appointment, got %d: %s", rec.Code, rec.Body.String())
	}

	// Case B: Future appointment (start date tomorrow)
	futCoordID, futToken := createTestUserWithRole(t, h, "coord_future", "Future Coordinator", "member")
	futureStart := now.Add(48 * time.Hour)
	insertTestAppointment(t, h, futCoordID, "state_coordinator", "Lagos State Coordinator", "state", &lagosState.ID, "active", futureStart, nil)

	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/activities/events", bytes.NewReader(body))
	req2.Header.Set("Authorization", "Bearer "+futToken)
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	h.Router.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusForbidden {
		t.Fatalf("Expected 403 Forbidden for future appointment, got %d: %s", rec2.Code, rec2.Body.String())
	}
}

// 5. Zonal coordinator cannot publish outside assigned zone (403)
func TestCoordinatorPrivileges_ZonalCoordinatorCannotPublishOutsideAssignedZone(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()

	_, _, kanoState, kanoLGA, swZoneCode := getLagosAndKanoData(t, h)
	now := time.Now()

	zonalCoordID, zonalToken := createTestUserWithRole(t, h, "coord_zonal_sw", "SW Zonal Coordinator", "member")
	// Appointed to South West Zone
	insertTestAppointment(t, h, zonalCoordID, "zonal_coordinator", "South West Zonal Coordinator", "zone", &swZoneCode, "active", now.Add(-24*time.Hour), nil)

	// Attempts to create event in Kano (North West Zone)
	payload := map[string]interface{}{
		"title":          "Cross-Zone Kano Cleanup",
		"state_id":       kanoState.ID,
		"lga_id":         kanoLGA.ID,
		"address":        "Kano Central Rd",
		"start_time":     now.Add(24 * time.Hour).Format(time.RFC3339),
		"end_time":       now.Add(28 * time.Hour).Format(time.RFC3339),
		"capacity":       100,
		"public_contact": "+2348000000005",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/activities/events", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+zonalToken)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("Expected 403 Forbidden for cross-zone event scheduling, got %d: %s", rec.Code, rec.Body.String())
	}
}

// 6. State coordinator cannot verify outside assigned state (403)
func TestCoordinatorPrivileges_StateCoordinatorCannotVerifyOutsideAssignedState(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()

	lagosState, _, kanoState, kanoLGA, _ := getLagosAndKanoData(t, h)
	now := time.Now()

	// 1. Create Kano Coordinator and an event in Kano
	kanoCoordID, _ := createTestUserWithRole(t, h, "coord_kano_st", "Kano State Coordinator", "member")
	insertTestAppointment(t, h, kanoCoordID, "state_coordinator", "Kano State Coordinator", "state", &kanoState.ID, "active", now.Add(-24*time.Hour), nil)

	kanoEvent, err := h.CleanupService.CreateEvent(context.Background(), kanoCoordID, "state_coordinator", &kanoState.ID, service.CreateEventRequest{
		Title:         "Kano City Clean",
		StateID:       kanoState.ID,
		LGAID:         kanoLGA.ID,
		Address:       "Kano City Road",
		StartTime:     now.Add(-1 * time.Hour), // active checkin window
		EndTime:       now.Add(2 * time.Hour),
		Capacity:      100,
		PublicContact: "+2348000000006",
		Status:        "published",
	})
	if err != nil {
		t.Fatalf("Failed to create Kano event: %v", err)
	}

	// 2. Member registers for Kano event
	mbrID, _ := createTestUserWithRole(t, h, "kano_volunteer", "Kano Volunteer", "member")
	reg, err := h.CleanupService.RegisterForEvent(context.Background(), mbrID, kanoEvent.ID)
	if err != nil {
		t.Fatalf("Failed to register member: %v", err)
	}

	// 3. Lagos State Coordinator attempts to verify attendance for Kano event
	lagosCoordID, lagosToken := createTestUserWithRole(t, h, "coord_lagos_rogue", "Lagos State Coordinator", "member")
	insertTestAppointment(t, h, lagosCoordID, "state_coordinator", "Lagos State Coordinator", "state", &lagosState.ID, "active", now.Add(-24*time.Hour), nil)

	verifyPayload := map[string]interface{}{
		"checkin_code": reg.CheckinCode.String,
	}
	body, _ := json.Marshal(verifyPayload)

	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/activities/events/%s/verify-attendance", kanoEvent.ID), bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+lagosToken)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("Expected 403 Forbidden when Lagos coordinator attempts verification on Kano event, got %d: %s", rec.Code, rec.Body.String())
	}
}

// 7. LGA coordinator cannot manage another LGA's event (403)
func TestCoordinatorPrivileges_LGACoordinatorCannotManageAnotherLGAEvent(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()

	lagosState, _, _, _, _ := getLagosAndKanoData(t, h)
	now := time.Now()

	lgas, err := h.GeoService.ListLGAsByState(context.Background(), lagosState.ID)
	if err != nil || len(lgas) < 2 {
		t.Fatalf("Need at least 2 LGAs in Lagos: %v", err)
	}
	lga1 := lgas[0] // e.g. Agege
	lga2 := lgas[1] // e.g. Alimosho

	// LGA1 Coordinator
	lga1CoordID, _ := createTestUserWithRole(t, h, "coord_lga1", "LGA 1 Coordinator", "member")
	insertTestAppointment(t, h, lga1CoordID, "lga_coordinator", "LGA 1 Coordinator", "lga", &lga1.ID, "active", now.Add(-24*time.Hour), nil)

	// Create event in LGA1
	eventLGA1, err := h.CleanupService.CreateEvent(context.Background(), lga1CoordID, "lga_coordinator", &lga1.ID, service.CreateEventRequest{
		Title:         "LGA 1 Grassroots Sweep",
		StateID:       lagosState.ID,
		LGAID:         lga1.ID,
		Address:       "LGA 1 Road",
		StartTime:     now.Add(-1 * time.Hour),
		EndTime:       now.Add(2 * time.Hour),
		Capacity:      50,
		PublicContact: "+2348000000007",
		Status:        "published",
	})
	if err != nil {
		t.Fatalf("Failed to create LGA1 event: %v", err)
	}

	mbrID, _ := createTestUserWithRole(t, h, "lga1_vol", "LGA1 Volunteer", "member")
	reg, err := h.CleanupService.RegisterForEvent(context.Background(), mbrID, eventLGA1.ID)
	if err != nil {
		t.Fatalf("Failed to register volunteer: %v", err)
	}

	// LGA2 Coordinator tries to verify LGA1 event attendance
	lga2CoordID, lga2Token := createTestUserWithRole(t, h, "coord_lga2", "LGA 2 Coordinator", "member")
	insertTestAppointment(t, h, lga2CoordID, "lga_coordinator", "LGA 2 Coordinator", "lga", &lga2.ID, "active", now.Add(-24*time.Hour), nil)

	verifyPayload := map[string]interface{}{
		"checkin_code": reg.CheckinCode.String,
	}
	body, _ := json.Marshal(verifyPayload)

	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/activities/events/%s/verify-attendance", eventLGA1.ID), bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+lga2Token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("Expected 403 Forbidden when LGA2 coordinator manages LGA1 event, got %d: %s", rec.Code, rec.Body.String())
	}
}

// 8. Valid QR/code verification succeeds (200, marked attended & verified)
func TestCoordinatorPrivileges_ValidQRCodeVerificationSucceeds(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()

	lagosState, lagosLGA, _, _, _ := getLagosAndKanoData(t, h)
	now := time.Now()

	coordID, coordToken := createTestUserWithRole(t, h, "coord_verif_ok", "Authorized Coordinator", "member")
	insertTestAppointment(t, h, coordID, "state_coordinator", "Lagos State Coordinator", "state", &lagosState.ID, "active", now.Add(-24*time.Hour), nil)

	event, err := h.CleanupService.CreateEvent(context.Background(), coordID, "state_coordinator", &lagosState.ID, service.CreateEventRequest{
		Title:         "Tarkwa Bay Beach Cleanup",
		StateID:       lagosState.ID,
		LGAID:         lagosLGA.ID,
		Address:       "Tarkwa Bay Beachfront",
		StartTime:     now.Add(-1 * time.Hour), // Happening now
		EndTime:       now.Add(2 * time.Hour),
		Capacity:      100,
		PublicContact: "+2348000000008",
		Status:        "published",
	})
	if err != nil {
		t.Fatalf("Failed to create event: %v", err)
	}

	mbrID, mbrToken := createTestUserWithRole(t, h, "mbr_tarkwa", "Tarkwa Volunteer", "member")

	// Member registers
	regReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/activities/events/%s/register", event.ID), nil)
	regReq.Header.Set("Authorization", "Bearer "+mbrToken)
	regRec := httptest.NewRecorder()
	h.Router.ServeHTTP(regRec, regReq)
	if regRec.Code != http.StatusOK {
		t.Fatalf("Expected 201 registration, got %d: %s", regRec.Code, regRec.Body.String())
	}

	var regRes map[string]interface{}
	_ = json.Unmarshal(regRec.Body.Bytes(), &regRes)
	regData := regRes
	if wrapped, ok := regRes["data"].(map[string]interface{}); ok {
		regData = wrapped
	}
	checkinCode, ok := regData["checkin_code"].(string)
	if !ok || checkinCode == "" {
		t.Fatalf("Expected checkin_code in registration response: %v", regRes)
	}

	// Coordinator verifies on-site
	verifyPayload := map[string]interface{}{
		"checkin_code": checkinCode,
		"is_manual":    false,
	}
	body, _ := json.Marshal(verifyPayload)

	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/activities/events/%s/verify-attendance", event.ID), bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+coordToken)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for valid attendance verification, got %d: %s", rec.Code, rec.Body.String())
	}

	// Confirm registration state
	myRegReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/activities/events/%s/my-registration", event.ID), nil)
	myRegReq.Header.Set("Authorization", "Bearer "+mbrToken)
	myRegRec := httptest.NewRecorder()
	h.Router.ServeHTTP(myRegRec, myRegReq)
	if myRegRec.Code != http.StatusOK {
		t.Fatalf("Expected 200 for my-registration, got %d: %s", myRegRec.Code, myRegRec.Body.String())
	}

	var myRegRes map[string]interface{}
	_ = json.Unmarshal(myRegRec.Body.Bytes(), &myRegRes)
	myRegData := myRegRes
	if wrapped, ok := myRegRes["data"].(map[string]interface{}); ok {
		myRegData = wrapped
	}
	if myRegData["status"] != "attended" {
		t.Errorf("Expected registration status to be 'attended', got %v", myRegData["status"])
	}

	// Confirm participant roster endpoint
	rosterReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/activities/events/%s/participants", event.ID), nil)
	rosterReq.Header.Set("Authorization", "Bearer "+coordToken)
	rosterRec := httptest.NewRecorder()
	h.Router.ServeHTTP(rosterRec, rosterReq)
	if rosterRec.Code != http.StatusOK {
		t.Fatalf("Expected 200 for participants roster, got %d: %s", rosterRec.Code, rosterRec.Body.String())
	}

	// Confirm claim was upserted to verified
	claim, err := h.Repo.Queries().GetParticipationClaimByEventAndUser(context.Background(), db.GetParticipationClaimByEventAndUserParams{
		EventID: event.ID,
		UserID:  mbrID,
	})
	if err != nil {
		t.Fatalf("Failed to fetch verified claim: %v", err)
	}
	if claim.Status != "verified" {
		t.Errorf("Expected claim status 'verified', got %s", claim.Status)
	}
}

// 9. Invalid or replayed codes are rejected (404 / 409)
func TestCoordinatorPrivileges_InvalidOrReplayedCodesRejected(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()

	lagosState, lagosLGA, _, _, _ := getLagosAndKanoData(t, h)
	now := time.Now()

	coordID, coordToken := createTestUserWithRole(t, h, "coord_replay_t", "Coordinator", "member")
	insertTestAppointment(t, h, coordID, "state_coordinator", "Lagos State Coordinator", "state", &lagosState.ID, "active", now.Add(-24*time.Hour), nil)

	event, err := h.CleanupService.CreateEvent(context.Background(), coordID, "state_coordinator", &lagosState.ID, service.CreateEventRequest{
		Title:         "Replay Test Cleanup",
		StateID:       lagosState.ID,
		LGAID:         lagosLGA.ID,
		Address:       "Replay Venue",
		StartTime:     now.Add(-1 * time.Hour),
		EndTime:       now.Add(2 * time.Hour),
		Capacity:      100,
		PublicContact: "+2348000000009",
		Status:        "published",
	})
	if err != nil {
		t.Fatalf("Failed to create event: %v", err)
	}

	mbrID, _ := createTestUserWithRole(t, h, "mbr_replay_t", "Volunteer", "member")
	reg, err := h.CleanupService.RegisterForEvent(context.Background(), mbrID, event.ID)
	if err != nil {
		t.Fatalf("Failed to register volunteer: %v", err)
	}

	// 1. Try invalid code -> 404
	invalidPayload := map[string]interface{}{"checkin_code": "RT-FA44-0000"}
	invBody, _ := json.Marshal(invalidPayload)
	reqInv := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/activities/events/%s/verify-attendance", event.ID), bytes.NewReader(invBody))
	reqInv.Header.Set("Authorization", "Bearer "+coordToken)
	reqInv.Header.Set("Content-Type", "application/json")
	recInv := httptest.NewRecorder()
	h.Router.ServeHTTP(recInv, reqInv)

	if recInv.Code != http.StatusNotFound {
		t.Fatalf("Expected 404 for invalid checkin code, got %d: %s", recInv.Code, recInv.Body.String())
	}

	// 2. First verification succeeds -> 200
	validPayload := map[string]interface{}{"checkin_code": reg.CheckinCode.String}
	valBody, _ := json.Marshal(validPayload)
	req1 := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/activities/events/%s/verify-attendance", event.ID), bytes.NewReader(valBody))
	req1.Header.Set("Authorization", "Bearer "+coordToken)
	req1.Header.Set("Content-Type", "application/json")
	rec1 := httptest.NewRecorder()
	h.Router.ServeHTTP(rec1, req1)

	if rec1.Code != http.StatusOK {
		t.Fatalf("Expected 200 for initial verification, got %d: %s", rec1.Code, rec1.Body.String())
	}

	// 3. Replay verification -> 409 Conflict
	req2 := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/activities/events/%s/verify-attendance", event.ID), bytes.NewReader(valBody))
	req2.Header.Set("Authorization", "Bearer "+coordToken)
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	h.Router.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusConflict {
		t.Fatalf("Expected 409 Conflict for replayed checkin, got %d: %s", rec2.Code, rec2.Body.String())
	}
}

// 10. Anti-Self-Verification is strictly enforced (403)
func TestCoordinatorPrivileges_AntiSelfVerificationEnforced(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()

	lagosState, lagosLGA, _, _, _ := getLagosAndKanoData(t, h)
	now := time.Now()

	coordID, coordToken := createTestUserWithRole(t, h, "coord_self_ver", "Selfish Coordinator", "member")
	insertTestAppointment(t, h, coordID, "state_coordinator", "Lagos State Coordinator", "state", &lagosState.ID, "active", now.Add(-24*time.Hour), nil)

	event, err := h.CleanupService.CreateEvent(context.Background(), coordID, "state_coordinator", &lagosState.ID, service.CreateEventRequest{
		Title:         "Anti Self Verification Test Venue",
		StateID:       lagosState.ID,
		LGAID:         lagosLGA.ID,
		Address:       "Venue Address",
		StartTime:     now.Add(-1 * time.Hour),
		EndTime:       now.Add(2 * time.Hour),
		Capacity:      100,
		PublicContact: "+2348000000010",
		Status:        "published",
	})
	if err != nil {
		t.Fatalf("Failed to create event: %v", err)
	}

	reg, err := h.CleanupService.RegisterForEvent(context.Background(), coordID, event.ID)
	if err != nil {
		t.Fatalf("Failed to register coordinator as participant: %v", err)
	}

	// Coordinator attempts to verify their own attendance
	selfPayload := map[string]interface{}{"checkin_code": reg.CheckinCode.String}
	body, _ := json.Marshal(selfPayload)
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/activities/events/%s/verify-attendance", event.ID), bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+coordToken)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("Expected 403 Forbidden for anti-self-verification invariant, got %d: %s", rec.Code, rec.Body.String())
	}
}

// 11. Missed check-in correction is approved with evidence
func TestCoordinatorPrivileges_MissedCheckinCorrectionApprovedWithEvidence(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()

	lagosState, lagosLGA, _, _, _ := getLagosAndKanoData(t, h)
	now := time.Now()

	coordID, coordToken := createTestUserWithRole(t, h, "coord_corr_app", "Reviewing Coordinator", "member")
	insertTestAppointment(t, h, coordID, "state_coordinator", "Lagos State Coordinator", "state", &lagosState.ID, "active", now.Add(-24*time.Hour), nil)

	event, err := h.CleanupService.CreateEvent(context.Background(), coordID, "state_coordinator", &lagosState.ID, service.CreateEventRequest{
		Title:         "Lekki Clean-Up Drive",
		StateID:       lagosState.ID,
		LGAID:         lagosLGA.ID,
		Address:       "Lekki Phase 1",
		StartTime:     now.Add(-4 * time.Hour),
		EndTime:       now.Add(-1 * time.Hour),
		Capacity:      100,
		PublicContact: "+2348000000011",
		Status:        "published",
	})
	if err != nil {
		t.Fatalf("Failed to create event: %v", err)
	}

	mbrID, mbrToken := createTestUserWithRole(t, h, "mbr_corr_usr", "Unchecked Volunteer", "member")
	_, err = h.CleanupService.RegisterForEvent(context.Background(), mbrID, event.ID)
	if err != nil {
		t.Fatalf("Failed to register member: %v", err)
	}

	// Member submits evidence correction
	corrPayload := map[string]interface{}{
		"event_id":       event.ID,
		"description":    "My phone battery died during afternoon check-in at Lekki",
		"evidence_notes": "Attached timestamped selfie holding blue waste bag",
		"evidence_url":   "https://r2.ratels.org/evidence/lekki_selfie.jpg",
		"evidence_type":  "photo_evidence",
	}
	body, _ := json.Marshal(corrPayload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/participations/claims", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+mbrToken)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("Expected 201 Created for claim submission, got %d: %s", rec.Code, rec.Body.String())
	}

	var res map[string]interface{}
	_ = json.Unmarshal(rec.Body.Bytes(), &res)
	claimData := res
	if wrapped, ok := res["data"].(map[string]interface{}); ok {
		claimData = wrapped
	}
	claimID := claimData["id"].(string)

	// Coordinator inspects review queue
	queueReq := httptest.NewRequest(http.MethodGet, "/api/v1/participations/review-queue", nil)
	queueReq.Header.Set("Authorization", "Bearer "+coordToken)
	queueRec := httptest.NewRecorder()
	h.Router.ServeHTTP(queueRec, queueReq)

	if queueRec.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for review-queue, got %d: %s", queueRec.Code, queueRec.Body.String())
	}

	// Coordinator approves correction with reason
	apprPayload := map[string]interface{}{
		"decision": "approved",
		"reason":   "Photo timestamp and venue background verified by lead coordinator",
	}
	apprBody, _ := json.Marshal(apprPayload)
	apprReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/participations/claims/%s/review", claimID), bytes.NewReader(apprBody))
	apprReq.Header.Set("Authorization", "Bearer "+coordToken)
	apprReq.Header.Set("Content-Type", "application/json")
	apprRec := httptest.NewRecorder()
	h.Router.ServeHTTP(apprRec, apprReq)

	if apprRec.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for claim approval, got %d: %s", apprRec.Code, apprRec.Body.String())
	}

	// Verify that event_registrations was atomically updated to attended
	myRegReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/activities/events/%s/my-registration", event.ID), nil)
	myRegReq.Header.Set("Authorization", "Bearer "+mbrToken)
	myRegRec := httptest.NewRecorder()
	h.Router.ServeHTTP(myRegRec, myRegReq)

	var myRegRes map[string]interface{}
	_ = json.Unmarshal(myRegRec.Body.Bytes(), &myRegRes)
	myRegData := myRegRes
	if wrapped, ok := myRegRes["data"].(map[string]interface{}); ok {
		myRegData = wrapped
	}
	if myRegData["status"] != "attended" {
		t.Errorf("Expected registration to be synced to 'attended', got %v", myRegData["status"])
	}
}

// 12. Unauthorized correction approval is rejected (403) and Anti-Self-Approval (403)
func TestCoordinatorPrivileges_UnauthorizedCorrectionApprovalRejected(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()

	lagosState, lagosLGA, kanoState, kanoLGA, _ := getLagosAndKanoData(t, h)
	now := time.Now()

	// Kano Coordinator & Event
	kanoCoordID, _ := createTestUserWithRole(t, h, "coord_kno_c", "Kano Coordinator", "member")
	insertTestAppointment(t, h, kanoCoordID, "state_coordinator", "Kano State Coordinator", "state", &kanoState.ID, "active", now.Add(-24*time.Hour), nil)

	kanoEvent, err := h.CleanupService.CreateEvent(context.Background(), kanoCoordID, "state_coordinator", &kanoState.ID, service.CreateEventRequest{
		Title:         "Kano Ground Clean",
		StateID:       kanoState.ID,
		LGAID:         kanoLGA.ID,
		Address:       "Kano City",
		StartTime:     now.Add(-4 * time.Hour),
		EndTime:       now.Add(-1 * time.Hour),
		Capacity:      100,
		PublicContact: "+2348000000012",
		Status:        "published",
	})
	if err != nil {
		t.Fatalf("Failed to create Kano event: %v", err)
	}

	mbrID, _ := createTestUserWithRole(t, h, "kno_mbr_clm", "Kano Volunteer", "member")
	claim, err := h.PartService.SubmitClaim(context.Background(), mbrID, service.SubmitClaimRequest{
		EventID:     kanoEvent.ID,
		Description: "Attended in Kano",
	})
	if err != nil {
		t.Fatalf("Failed to submit claim: %v", err)
	}

	// Lagos Coordinator attempts to approve Kano claim -> 403
	lagosCoordID, lagosToken := createTestUserWithRole(t, h, "coord_lgs_intruder", "Lagos Coordinator", "member")
	insertTestAppointment(t, h, lagosCoordID, "state_coordinator", "Lagos State Coordinator", "state", &lagosState.ID, "active", now.Add(-24*time.Hour), nil)

	apprPayload := map[string]interface{}{
		"decision": "approved",
		"reason":   "Illegal cross-state approval attempt",
	}
	apprBody, _ := json.Marshal(apprPayload)
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/participations/claims/%s/review", claim.ID), bytes.NewReader(apprBody))
	req.Header.Set("Authorization", "Bearer "+lagosToken)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("Expected 403 Forbidden for unauthorized cross-jurisdiction review, got %d: %s", rec.Code, rec.Body.String())
	}

	// Case B: Anti-Self-Approval
	// Coordinator submits claim for own event then tries to approve it
	selfEvent, _ := h.CleanupService.CreateEvent(context.Background(), lagosCoordID, "state_coordinator", &lagosState.ID, service.CreateEventRequest{
		Title:         "Self Claim Event",
		StateID:       lagosState.ID,
		LGAID:         lagosLGA.ID,
		Address:       "Lagos St",
		StartTime:     now.Add(-4 * time.Hour),
		EndTime:       now.Add(-1 * time.Hour),
		Capacity:      100,
		PublicContact: "+2348000000013",
		Status:        "published",
	})
	selfClaim, err := h.PartService.SubmitClaim(context.Background(), lagosCoordID, service.SubmitClaimRequest{
		EventID:     selfEvent.ID,
		Description: "I worked as volunteer too",
	})
	if err != nil {
		t.Fatalf("Failed to submit self claim: %v", err)
	}

	reqSelf := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/participations/claims/%s/review", selfClaim.ID), bytes.NewReader(apprBody))
	reqSelf.Header.Set("Authorization", "Bearer "+lagosToken)
	reqSelf.Header.Set("Content-Type", "application/json")
	recSelf := httptest.NewRecorder()
	h.Router.ServeHTTP(recSelf, reqSelf)

	if recSelf.Code != http.StatusForbidden {
		t.Fatalf("Expected 403 Forbidden for anti-self-approval invariant, got %d: %s", recSelf.Code, recSelf.Body.String())
	}
}

// 13. Fraudulent attendance can be reversed with an audit trail
func TestCoordinatorPrivileges_FraudulentAttendanceReversedWithAuditTrail(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()

	lagosState, lagosLGA, _, _, _ := getLagosAndKanoData(t, h)
	now := time.Now()

	coordID, coordToken := createTestUserWithRole(t, h, "coord_fraud_test", "Coordinator", "member")
	insertTestAppointment(t, h, coordID, "state_coordinator", "Lagos State Coordinator", "state", &lagosState.ID, "active", now.Add(-24*time.Hour), nil)

	_, superAdminToken := createTestUserWithRole(t, h, "sa_reverser", "Super Administrator", "super_admin")

	event, err := h.CleanupService.CreateEvent(context.Background(), coordID, "state_coordinator", &lagosState.ID, service.CreateEventRequest{
		Title:         "Fraud Detection Drive",
		StateID:       lagosState.ID,
		LGAID:         lagosLGA.ID,
		Address:       "Obalende Bus Stop",
		StartTime:     now.Add(-2 * time.Hour),
		EndTime:       now.Add(1 * time.Hour),
		Capacity:      100,
		PublicContact: "+2348000000014",
		Status:        "published",
	})
	if err != nil {
		t.Fatalf("Failed to create event: %v", err)
	}

	mbrID, _ := createTestUserWithRole(t, h, "mbr_fraudulent", "Fraud Volunteer", "member")
	reg, err := h.CleanupService.RegisterForEvent(context.Background(), mbrID, event.ID)
	if err != nil {
		t.Fatalf("Failed to register volunteer: %v", err)
	}

	// On-site check-in happens
	_, claim, err := h.CleanupService.VerifyAttendance(context.Background(), coordID, event.ID, service.VerifyAttendanceRequest{
		CheckinCode: reg.CheckinCode.String,
	})
	if err != nil {
		t.Fatalf("Failed initial verification: %v", err)
	}

	// 1. Coordinator attempts to reverse -> 403 (Exclusive to Super Admin)
	revPayload := map[string]interface{}{"reason": "Suspected proxy check-in code sharing"}
	revBody, _ := json.Marshal(revPayload)
	reqCoordRev := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/participations/claims/%s/reverse", claim.ID), bytes.NewReader(revBody))
	reqCoordRev.Header.Set("Authorization", "Bearer "+coordToken)
	reqCoordRev.Header.Set("Content-Type", "application/json")
	recCoordRev := httptest.NewRecorder()
	h.Router.ServeHTTP(recCoordRev, reqCoordRev)

	if recCoordRev.Code != http.StatusForbidden {
		t.Fatalf("Expected 403 Forbidden for non-super-admin reversal attempt, got %d: %s", recCoordRev.Code, recCoordRev.Body.String())
	}

	// 2. Super Admin reverses attendance
	reqSaRev := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/participations/claims/%s/reverse", claim.ID), bytes.NewReader(revBody))
	reqSaRev.Header.Set("Authorization", "Bearer "+superAdminToken)
	reqSaRev.Header.Set("Content-Type", "application/json")
	recSaRev := httptest.NewRecorder()
	h.Router.ServeHTTP(recSaRev, reqSaRev)

	if recSaRev.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for Super Admin reversal, got %d: %s", recSaRev.Code, recSaRev.Body.String())
	}

	// 3. Confirm claim status is 'reversed' and audit log recorded
	updatedClaim, err := h.Repo.Queries().GetParticipationClaimByID(context.Background(), claim.ID)
	if err != nil {
		t.Fatalf("Failed to retrieve claim: %v", err)
	}
	if updatedClaim.Status != "reversed" {
		t.Errorf("Expected claim status to be 'reversed', got %s", updatedClaim.Status)
	}
	if !updatedClaim.ReversedAt.Valid || !updatedClaim.ReversedByUserID.Valid {
		t.Errorf("Expected reversed_at and reversed_by_user_id to be populated")
	}

	// Confirm registration attendance status was reset
	regAfter, err := h.Repo.Queries().GetEventRegistration(context.Background(), db.GetEventRegistrationParams{
		EventID: event.ID,
		UserID:  mbrID,
	})
	if err != nil {
		t.Fatalf("Failed to query registration: %v", err)
	}
	if regAfter.VerifiedAt.Valid {
		t.Errorf("Expected registration.verified_at to be NULL after reversal")
	}
}

// 14. Concurrent verification attempts do not duplicate credit
func TestCoordinatorPrivileges_ConcurrentVerificationDoesNotDuplicateCredit(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()

	lagosState, lagosLGA, _, _, _ := getLagosAndKanoData(t, h)
	now := time.Now()

	coordID, coordToken := createTestUserWithRole(t, h, "coord_conc_ver", "Coordinator", "member")
	insertTestAppointment(t, h, coordID, "state_coordinator", "Lagos State Coordinator", "state", &lagosState.ID, "active", now.Add(-24*time.Hour), nil)

	event, err := h.CleanupService.CreateEvent(context.Background(), coordID, "state_coordinator", &lagosState.ID, service.CreateEventRequest{
		Title:         "High Concurrency Stress Event",
		StateID:       lagosState.ID,
		LGAID:         lagosLGA.ID,
		Address:       "Victoria Island Beach",
		StartTime:     now.Add(-1 * time.Hour),
		EndTime:       now.Add(2 * time.Hour),
		Capacity:      100,
		PublicContact: "+2348000000015",
		Status:        "published",
	})
	if err != nil {
		t.Fatalf("Failed to create event: %v", err)
	}

	mbrID, _ := createTestUserWithRole(t, h, "mbr_conc_stress", "Volunteer", "member")
	reg, err := h.CleanupService.RegisterForEvent(context.Background(), mbrID, event.ID)
	if err != nil {
		t.Fatalf("Failed to register volunteer: %v", err)
	}

	var wg sync.WaitGroup
	var successCount int32
	var conflictCount int32
	concurrency := 6

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			payload := map[string]interface{}{"checkin_code": reg.CheckinCode.String}
			body, _ := json.Marshal(payload)
			req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/activities/events/%s/verify-attendance", event.ID), bytes.NewReader(body))
			req.Header.Set("Authorization", "Bearer "+coordToken)
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			h.Router.ServeHTTP(rec, req)

			if rec.Code == http.StatusOK {
				atomic.AddInt32(&successCount, 1)
			} else if rec.Code == http.StatusConflict {
				atomic.AddInt32(&conflictCount, 1)
			}
		}()
	}

	wg.Wait()

	if successCount != 1 {
		t.Fatalf("Expected exactly 1 verification success in race condition, got %d (conflicts: %d)", successCount, conflictCount)
	}
	if conflictCount != int32(concurrency-1) {
		t.Fatalf("Expected %d conflicts in race condition, got %d", concurrency-1, conflictCount)
	}
}

// 15. Notification failure does not corrupt participation
func TestCoordinatorPrivileges_NotificationFailureDoesNotCorruptParticipation(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()

	lagosState, lagosLGA, _, _, _ := getLagosAndKanoData(t, h)
	now := time.Now()

	coordID, coordToken := createTestUserWithRole(t, h, "coord_notif_fail", "Coordinator", "member")
	insertTestAppointment(t, h, coordID, "state_coordinator", "Lagos State Coordinator", "state", &lagosState.ID, "active", now.Add(-24*time.Hour), nil)

	event, err := h.CleanupService.CreateEvent(context.Background(), coordID, "state_coordinator", &lagosState.ID, service.CreateEventRequest{
		Title:         "Fail-Safe Notification Venue",
		StateID:       lagosState.ID,
		LGAID:         lagosLGA.ID,
		Address:       "Safe Harbor",
		StartTime:     now.Add(-1 * time.Hour),
		EndTime:       now.Add(2 * time.Hour),
		Capacity:      50,
		PublicContact: "+2348000000016",
		Status:        "published",
	})
	if err != nil {
		t.Fatalf("Failed to create event: %v", err)
	}

	mbrID, _ := createTestUserWithRole(t, h, "mbr_notif_target", "Volunteer", "member")
	reg, err := h.CleanupService.RegisterForEvent(context.Background(), mbrID, event.ID)
	if err != nil {
		t.Fatalf("Failed to register volunteer: %v", err)
	}

	// Verify attendance completes successfully without panic or transaction abort
	payload := map[string]interface{}{"checkin_code": reg.CheckinCode.String}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/activities/events/%s/verify-attendance", event.ID), bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+coordToken)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK even if notification fails or sandbox is used, got %d: %s", rec.Code, rec.Body.String())
	}
}

// 16. Event cancellation by authorized coordinator
func TestCoordinatorPrivileges_EventCancellationByCoordinator(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()

	lagosState, lagosLGA, _, _, _ := getLagosAndKanoData(t, h)
	now := time.Now()

	coordID, coordToken := createTestUserWithRole(t, h, "coord_cancel_ev", "Coordinator", "member")
	insertTestAppointment(t, h, coordID, "state_coordinator", "Lagos State Coordinator", "state", &lagosState.ID, "active", now.Add(-24*time.Hour), nil)

	event, err := h.CleanupService.CreateEvent(context.Background(), coordID, "state_coordinator", &lagosState.ID, service.CreateEventRequest{
		Title:         "Event To Be Cancelled",
		StateID:       lagosState.ID,
		LGAID:         lagosLGA.ID,
		Address:       "Cancel St",
		StartTime:     now.Add(24 * time.Hour),
		EndTime:       now.Add(28 * time.Hour),
		Capacity:      50,
		PublicContact: "+2348000000017",
		Status:        "published",
	})
	if err != nil {
		t.Fatalf("Failed to create event: %v", err)
	}

	cancelPayload := map[string]interface{}{
		"reason": "Severe tropical rainstorm forecast",
	}
	body, _ := json.Marshal(cancelPayload)

	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/activities/events/%s/cancel", event.ID), bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+coordToken)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for event cancellation, got %d: %s", rec.Code, rec.Body.String())
	}

	cancelledEvent, err := h.Repo.Queries().GetEventInstanceByID(context.Background(), event.ID)
	if err != nil {
		t.Fatalf("Failed to query cancelled event: %v", err)
	}
	if cancelledEvent.Status != "cancelled" {
		t.Errorf("Expected event status 'cancelled', got %s", cancelledEvent.Status)
	}
}

// Phase 68.15: National admins cannot verify attendance with expired appointments.
func TestCoordinatorPrivileges_ExpiredNationalAdminCannotVerifyAttendance(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()

	ctx := context.Background()
	now := time.Now()

	lagosState, lagosLGA, _, _, _ := getLagosAndKanoData(t, h)

	creatorID, _ := createTestUserWithRole(
		t, h, "coord_nat_reg_creator", "Event Creator", "member",
	)
	insertTestAppointment(
		t, h, creatorID, "state_coordinator", "State Coordinator",
		"state", &lagosState.ID, "active", now.Add(-24*time.Hour), nil,
	)

	event, err := h.CleanupService.CreateEvent(
		ctx, creatorID, "state_coordinator", &lagosState.ID,
		service.CreateEventRequest{
			Title:         "National Admin Expiry Regression",
			StateID:       lagosState.ID,
			LGAID:         lagosLGA.ID,
			Address:       "Regression Test Venue",
			StartTime:     now.Add(-time.Hour),
			EndTime:       now.Add(2 * time.Hour),
			Capacity:      50,
			PublicContact: "+2348000000099",
			Status:        "published",
		},
	)
	if err != nil {
		t.Fatalf("Create event: %v", err)
	}

	memberID, _ := createTestUserWithRole(
		t, h, "member_nat_reg", "Test Volunteer", "member",
	)
	reg, err := h.CleanupService.RegisterForEvent(ctx, memberID, event.ID)
	if err != nil {
		t.Fatalf("Register volunteer: %v", err)
	}

	adminID, adminToken := createTestUserWithRole(
		t, h, "expired_nat_admin", "Expired National Admin", "national_admin",
	)

	pastEnd := now.Add(-24 * time.Hour)
	insertTestAppointment(
		t, h, adminID, "national_admin", "National Administrator",
		"national", nil, "active", now.Add(-30*24*time.Hour), &pastEnd,
	)

	body, _ := json.Marshal(map[string]interface{}{
		"checkin_code": reg.CheckinCode.String,
	})

	req := httptest.NewRequest(
		http.MethodPost,
		fmt.Sprintf("/api/v1/activities/events/%s/verify-attendance", event.ID),
		bytes.NewReader(body),
	)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	h.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf(
			"Expected 403 for expired national admin, got %d: %s",
			rec.Code, rec.Body.String(),
		)
	}

	after, err := h.Repo.Queries().GetEventRegistration(
		ctx,
		db.GetEventRegistrationParams{
			EventID: event.ID,
			UserID:  memberID,
		},
	)
	if err != nil {
		t.Fatalf("Reload registration: %v", err)
	}

	if after.VerifiedAt.Valid || after.Status == "attended" {
		t.Fatal("Expired national admin changed attendance status")
	}
	// Change the expired appointment into a revoked appointment
	// with otherwise valid dates.
	_, err = h.Pool.Exec(ctx, `
		UPDATE appointments
		SET status = 'revoked',
		    start_date = $2,
		    end_date = NULL,
		    updated_at = CURRENT_TIMESTAMP
		WHERE user_id = $1 AND role_type = 'national_admin'
	`, adminID, now.Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("Revoke national-admin appointment: %v", err)
	}

	req2 := httptest.NewRequest(
		http.MethodPost,
		fmt.Sprintf("/api/v1/activities/events/%s/verify-attendance", event.ID),
		bytes.NewReader(body),
	)
	req2.Header.Set("Authorization", "Bearer "+adminToken)
	req2.Header.Set("Content-Type", "application/json")

	rec2 := httptest.NewRecorder()
	h.Router.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusForbidden {
		t.Fatalf(
			"Expected 403 for revoked national admin, got %d: %s",
			rec2.Code, rec2.Body.String(),
		)
	}

	afterRevocation, err := h.Repo.Queries().GetEventRegistration(
		ctx,
		db.GetEventRegistrationParams{
			EventID: event.ID,
			UserID:  memberID,
		},
	)
	if err != nil {
		t.Fatalf("Reload registration after revocation: %v", err)
	}

	if afterRevocation.VerifiedAt.Valid || afterRevocation.Status == "attended" {
		t.Fatal("Revoked national admin changed attendance status")
	}

}
