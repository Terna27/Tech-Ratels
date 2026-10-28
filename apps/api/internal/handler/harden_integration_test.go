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

func registerAndVerifyMember(t *testing.T, h *TestHarness, prefix, name string) (userID, email, token, cookieHeader, csrfToken string) {
	t.Helper()
	email = fmt.Sprintf("%s_%d@ratels.example.org", prefix, time.Now().UnixNano())
	password := "TestPass123!"

	// 1. Register
	regBytes, _ := json.Marshal(map[string]string{
		"email":        email,
		"password":     password,
		"display_name": name,
	})
	req, _ := http.NewRequest("POST", "/api/v1/auth/register", bytes.NewBuffer(regBytes))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("Register failed for %s: code %d, body %s", email, rr.Code, rr.Body.String())
	}
	var regResp map[string]interface{}
	_ = json.Unmarshal(rr.Body.Bytes(), &regResp)
	userID = regResp["user_id"].(string)

	// 2. Verify contact OTP
	otp := h.Delivery.GetLastCode(email)
	verBytes, _ := json.Marshal(map[string]string{
		"user_id":    userID,
		"identifier": email,
		"code":       otp,
	})
	req, _ = http.NewRequest("POST", "/api/v1/auth/verify-contact", bytes.NewBuffer(verBytes))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("Verify contact failed for %s: code %d", email, rr.Code)
	}

	// 3. Login
	loginBytes, _ := json.Marshal(map[string]string{
		"username": email,
		"password": password,
	})
	req, _ = http.NewRequest("POST", "/api/v1/auth/login", bytes.NewBuffer(loginBytes))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("Login failed for %s: code %d", email, rr.Code)
	}

	var loginResp map[string]interface{}
	_ = json.Unmarshal(rr.Body.Bytes(), &loginResp)
	token = loginResp["access_token"].(string)

	var sessionCookie, csrfCookie string
	for _, c := range rr.Result().Cookies() {
		if c.Name == "ratels_session" {
			sessionCookie = c.Value
		}
		if c.Name == "ratels_csrf" {
			csrfCookie = c.Value
		}
	}
	cookieHeader = fmt.Sprintf("ratels_session=%s; ratels_csrf=%s", sessionCookie, csrfCookie)
	csrfToken = csrfCookie

	return userID, email, token, cookieHeader, csrfToken
}

func TestCookieSessionAndCSRFProtection(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()

	_, email, _, _, _ := registerAndVerifyMember(t, h, "csrf_user", "CSRF Volunteer")

	// Perform explicit Login to examine response cookies in detail
	loginBytes, _ := json.Marshal(map[string]string{
		"username": email,
		"password": "TestPass123!",
	})
	req, _ := http.NewRequest("POST", "/api/v1/auth/login", bytes.NewBuffer(loginBytes))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Login failed: %d", rr.Code)
	}

	var sessionCookie, csrfCookie *http.Cookie
	for _, c := range rr.Result().Cookies() {
		if c.Name == "ratels_session" {
			sessionCookie = c
		}
		if c.Name == "ratels_csrf" {
			csrfCookie = c
		}
	}

	if sessionCookie == nil {
		t.Fatalf("Expected ratels_session cookie in login response, found none")
	}
	if !sessionCookie.HttpOnly {
		t.Errorf("Expected ratels_session to be HttpOnly=true, got false")
	}
	if sessionCookie.SameSite != http.SameSiteLaxMode {
		t.Errorf("Expected ratels_session SameSite=Lax, got %v", sessionCookie.SameSite)
	}
	if sessionCookie.Path != "/" {
		t.Errorf("Expected ratels_session Path=/, got %s", sessionCookie.Path)
	}

	if csrfCookie == nil {
		t.Fatalf("Expected ratels_csrf cookie in login response, found none")
	}
	if csrfCookie.HttpOnly {
		t.Errorf("Expected ratels_csrf to be readable by JS client (HttpOnly=false), got true")
	}

	// 1. Mutating request with cookie but WITHOUT X-CSRF-Token -> Must return 403 Forbidden
	req, _ = http.NewRequest("POST", "/api/v1/auth/logout", nil)
	req.AddCookie(sessionCookie)
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("Expected 403 Forbidden for cookie mutation without X-CSRF-Token, got %d", rr.Code)
	}

	// 2. Mutating request with cookie and INVALID X-CSRF-Token -> Must return 403 Forbidden
	req, _ = http.NewRequest("POST", "/api/v1/auth/logout", nil)
	req.AddCookie(sessionCookie)
	req.AddCookie(csrfCookie)
	req.Header.Set("X-CSRF-Token", "invalid_token_12345")
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("Expected 403 Forbidden for mismatched X-CSRF-Token, got %d", rr.Code)
	}

	// 3. Mutating request with cookie and VALID X-CSRF-Token -> Succeeds
	req, _ = http.NewRequest("POST", "/api/v1/auth/logout", nil)
	req.AddCookie(sessionCookie)
	req.AddCookie(csrfCookie)
	req.Header.Set("X-CSRF-Token", csrfCookie.Value)
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for valid CSRF token on logout, got %d: %s", rr.Code, rr.Body.String())
	}

	// 4. Verify logout cleared the session cookie (Max-Age: -1)
	for _, c := range rr.Result().Cookies() {
		if c.Name == "ratels_session" && c.MaxAge >= 0 {
			t.Errorf("Expected ratels_session cookie to be cleared (MaxAge < 0), got MaxAge %d", c.MaxAge)
		}
	}

	// 5. Subsequent request using the revoked session cookie -> Must return 401 Unauthorized
	req, _ = http.NewRequest("GET", "/api/v1/members/me", nil)
	req.AddCookie(sessionCookie)
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("Expected 401 Unauthorized for revoked session, got %d", rr.Code)
	}
}

func TestExpiredAppointmentRevokesAccessHTTP(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()
	ctx := context.Background()

	// Register user
	userID, _, token, _, _ := registerAndVerifyMember(t, h, "expired_coord", "Expired Coordinator")

	states, err := h.GeoService.ListStates(ctx)
	if err != nil || len(states) == 0 {
		t.Fatalf("Failed to fetch states: %v", err)
	}
	lagos := states[0]

	// Create an EXPIRED appointment for this user (end_date was 1 day ago)
	yesterday := time.Now().Add(-24 * time.Hour)
	twoDaysAgo := time.Now().Add(-48 * time.Hour)

	_, err = h.Repo.Queries().CreateAppointment(ctx, db.CreateAppointmentParams{
		ID:                       security.GenerateUUID(),
		UserID:                   userID,
		RoleTitle:                "State Coordinator",
		RoleType:                 "state_coordinator",
		JurisdictionType:         "state",
		JurisdictionID:           pgtype.Text{String: lagos.ID, Valid: true},
		ReportingToAppointmentID: pgtype.Text{Valid: false},
		StartDate:                pgtype.Timestamptz{Time: twoDaysAgo, Valid: true},
		EndDate:                  pgtype.Timestamptz{Time: yesterday, Valid: true},
		Status:                   "active", // marked active in column, but end_date has expired!
		ApprovalReference:        "APPT-EXP-001",
		PublicContact:            pgtype.Text{Valid: false},
	})
	if err != nil {
		t.Fatalf("Failed to insert appointment: %v", err)
	}

	// Make request to endpoint requiring state_coordinator role: POST /api/v1/activities/events
	eventBytes, _ := json.Marshal(map[string]interface{}{
		"title":      "Expired Access Attempt",
		"state_id":   lagos.ID,
		"address":    "Test Address",
		"start_time": time.Now().Add(48 * time.Hour).Format(time.RFC3339),
		"end_time":   time.Now().Add(52 * time.Hour).Format(time.RFC3339),
		"capacity":   50,
	})
	req, _ := http.NewRequest("POST", "/api/v1/activities/events", bytes.NewBuffer(eventBytes))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)

	// Since appointment has expired, user has only "member" role -> Must be 403 Forbidden!
	if rr.Code != http.StatusForbidden {
		t.Fatalf("Expected 403 Forbidden for expired appointment holder, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestPermissionsAndCrossStateIsolationHTTP(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()
	ctx := context.Background()

	// 1. Ordinary member attempts restricted actions
	_, _, memberToken, _, _ := registerAndVerifyMember(t, h, "ord_member", "Ordinary Member")

	// Member attempts creating appointment -> 403 Forbidden
	apptBytes, _ := json.Marshal(map[string]interface{}{
		"user_id":           "some-user-id",
		"role_title":        "National Administrator",
		"role_type":         "national_admin",
		"jurisdiction_type": "national",
	})
	req, _ := http.NewRequest("POST", "/api/v1/leadership/appointments", bytes.NewBuffer(apptBytes))
	req.Header.Set("Authorization", "Bearer "+memberToken)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("Expected 403 Forbidden for member creating appointment, got %d", rr.Code)
	}

	// Member attempts reviewing claim -> 403 Forbidden
	revBytes, _ := json.Marshal(map[string]interface{}{
		"decision": "approved",
		"reason":   "Self review",
	})
	req, _ = http.NewRequest("POST", "/api/v1/participations/claims/dummy-claim-id/review", bytes.NewBuffer(revBytes))
	req.Header.Set("Authorization", "Bearer "+memberToken)
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("Expected 403 Forbidden for member reviewing claim, got %d", rr.Code)
	}

	// 2. Cross-state geographic isolation: Lagos Coordinator vs Kano State
	states, err := h.GeoService.ListStates(ctx)
	if err != nil || len(states) < 2 {
		t.Fatalf("Need at least 2 states, found %d", len(states))
	}
	var lagosState, kanoState service.StateDetailResponse
	for _, s := range states {
		if s.Code == "LA" {
			lagosState = s
		}
		if s.Code == "KN" {
			kanoState = s
		}
	}
	if lagosState.ID == "" || kanoState.ID == "" {
		lagosState = states[0]
		kanoState = states[1]
	}

	lagosLGAs, _ := h.GeoService.ListLGAsByState(ctx, lagosState.ID)
	kanoLGAs, _ := h.GeoService.ListLGAsByState(ctx, kanoState.ID)

	// Register Lagos Coordinator
	lagosUserID, _, lagosToken, _, _ := registerAndVerifyMember(t, h, "lagos_coord", "Lagos Coordinator")
	_, err = h.Repo.Queries().CreateAppointment(ctx, db.CreateAppointmentParams{
		ID:                       security.GenerateUUID(),
		UserID:                   lagosUserID,
		RoleTitle:                "State Coordinator",
		RoleType:                 "state_coordinator",
		JurisdictionType:         "state",
		JurisdictionID:           pgtype.Text{String: lagosState.ID, Valid: true},
		ReportingToAppointmentID: pgtype.Text{Valid: false},
		StartDate:                pgtype.Timestamptz{Time: time.Now().Add(-1 * time.Hour), Valid: true},
		EndDate:                  pgtype.Timestamptz{Valid: false},
		Status:                   "active",
		ApprovalReference:        "APPT-LA-001",
		PublicContact:            pgtype.Text{Valid: false},
	})
	if err != nil {
		t.Fatalf("Failed to appoint Lagos coordinator: %v", err)
	}

	// A. Lagos Coordinator creates event IN LAGOS -> 201 Created
	lagosEventBytes, _ := json.Marshal(map[string]interface{}{
		"title":      "Lagos Community Cleanup",
		"state_id":   lagosState.ID,
		"lga_id":     lagosLGAs[0].ID,
		"address":    "Marina Waterfront",
		"start_time": time.Now().Add(48 * time.Hour).Format(time.RFC3339),
		"end_time":   time.Now().Add(52 * time.Hour).Format(time.RFC3339),
		"capacity":   100,
	})
	req, _ = http.NewRequest("POST", "/api/v1/activities/events", bytes.NewBuffer(lagosEventBytes))
	req.Header.Set("Authorization", "Bearer "+lagosToken)
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("Expected 201 Created for Lagos coordinator creating event in Lagos, got %d: %s", rr.Code, rr.Body.String())
	}

	// B. Lagos Coordinator attempts to create event IN KANO -> 403 Forbidden!
	kanoEventBytes, _ := json.Marshal(map[string]interface{}{
		"title":      "Unauthorized Kano Event",
		"state_id":   kanoState.ID,
		"lga_id":     kanoLGAs[0].ID,
		"address":    "Kano City Plaza",
		"start_time": time.Now().Add(48 * time.Hour).Format(time.RFC3339),
		"end_time":   time.Now().Add(52 * time.Hour).Format(time.RFC3339),
		"capacity":   100,
	})
	req, _ = http.NewRequest("POST", "/api/v1/activities/events", bytes.NewBuffer(kanoEventBytes))
	req.Header.Set("Authorization", "Bearer "+lagosToken)
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("Expected 403 Forbidden for Lagos coordinator trying to create event in Kano, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestCompleteUserJourneyHTTP(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()
	ctx := context.Background()

	// 1. Setup National Admin
	adminUserID, _, adminToken, _, _ := registerAndVerifyMember(t, h, "e2e_admin", "E2E Administrator")
	_, err := h.Repo.Queries().CreateAppointment(ctx, db.CreateAppointmentParams{
		ID:                       security.GenerateUUID(),
		UserID:                   adminUserID,
		RoleTitle:                "National Administrator",
		RoleType:                 "national_admin",
		JurisdictionType:         "national",
		JurisdictionID:           pgtype.Text{Valid: false},
		ReportingToAppointmentID: pgtype.Text{Valid: false},
		StartDate:                pgtype.Timestamptz{Time: time.Now().Add(-1 * time.Hour), Valid: true},
		EndDate:                  pgtype.Timestamptz{Valid: false},
		Status:                   "active",
		ApprovalReference:        "APPT-NAT-001",
		PublicContact:            pgtype.Text{Valid: false},
	})
	if err != nil {
		t.Fatalf("Failed to appoint national admin: %v", err)
	}

	states, _ := h.GeoService.ListStates(ctx)
	state := states[0]
	lgas, _ := h.GeoService.ListLGAsByState(ctx, state.ID)
	lga := lgas[0]

	// 2. Admin creates activity draft with capacity=2
	createEvtBytes, _ := json.Marshal(map[string]interface{}{
		"title":      "Phase 1 Demonstration Cleanup",
		"state_id":   state.ID,
		"lga_id":     lga.ID,
		"address":    "12 Freedom Way",
		"start_time": time.Now().Add(24 * time.Hour).Format(time.RFC3339),
		"end_time":   time.Now().Add(28 * time.Hour).Format(time.RFC3339),
		"capacity":   2, // Low capacity for testing limits
		"status":     "draft",
	})
	req, _ := http.NewRequest("POST", "/api/v1/activities/events", bytes.NewBuffer(createEvtBytes))
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("Admin create event failed: code %d, body %s", rr.Code, rr.Body.String())
	}
	var createdEvt map[string]interface{}
	_ = json.Unmarshal(rr.Body.Bytes(), &createdEvt)
	eventID := createdEvt["id"].(string)

	// 3. Admin publishes the activity
	req, _ = http.NewRequest("POST", fmt.Sprintf("/api/v1/activities/events/%s/publish", eventID), nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("Publish event failed: code %d, body %s", rr.Code, rr.Body.String())
	}

	// 4. Member 1 registers for event
	member1ID, _, member1Token, _, _ := registerAndVerifyMember(t, h, "e2e_vol1", "E2E Volunteer 1")
	req, _ = http.NewRequest("POST", fmt.Sprintf("/api/v1/activities/events/%s/register", eventID), nil)
	req.Header.Set("Authorization", "Bearer "+member1Token)
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("Member 1 register failed: code %d, body %s", rr.Code, rr.Body.String())
	}

	// 5. Member 1 attempts duplicate registration -> 409 Conflict
	req, _ = http.NewRequest("POST", fmt.Sprintf("/api/v1/activities/events/%s/register", eventID), nil)
	req.Header.Set("Authorization", "Bearer "+member1Token)
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusConflict {
		t.Fatalf("Expected 409 Conflict for duplicate registration, got %d", rr.Code)
	}

	// 6. Member 2 registers -> Reaches capacity 2/2
	_, _, member2Token, _, _ := registerAndVerifyMember(t, h, "e2e_vol2", "E2E Volunteer 2")
	req, _ = http.NewRequest("POST", fmt.Sprintf("/api/v1/activities/events/%s/register", eventID), nil)
	req.Header.Set("Authorization", "Bearer "+member2Token)
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("Member 2 register failed: code %d", rr.Code)
	}

	// 7. Member 3 attempts registration -> Capacity limit exceeded -> 409 Conflict
	_, _, member3Token, _, _ := registerAndVerifyMember(t, h, "e2e_vol3", "E2E Volunteer 3")
	req, _ = http.NewRequest("POST", fmt.Sprintf("/api/v1/activities/events/%s/register", eventID), nil)
	req.Header.Set("Authorization", "Bearer "+member3Token)
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusConflict {
		t.Fatalf("Expected 409 Conflict when event is full, got %d: %s", rr.Code, rr.Body.String())
	}

	// 8. Member 1 submits participation evidence claim
	evidenceNotes := "Photographic evidence submitted to coordinator."
	claimPayload, _ := json.Marshal(map[string]interface{}{
		"event_id":       eventID,
		"description":    "Participated in street cleanup and waste sorting.",
		"evidence_notes": evidenceNotes,
	})
	req, _ = http.NewRequest("POST", "/api/v1/participations/claims", bytes.NewBuffer(claimPayload))
	req.Header.Set("Authorization", "Bearer "+member1Token)
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("Submit claim failed: code %d, body %s", rr.Code, rr.Body.String())
	}
	var createdClaim map[string]interface{}
	_ = json.Unmarshal(rr.Body.Bytes(), &createdClaim)
	claimID := createdClaim["id"].(string)

	// 9. Member 1 attempts self-review -> 403 Forbidden
	reviewSelfPayload, _ := json.Marshal(map[string]interface{}{
		"decision": "approved",
		"reason":   "Self review attempt",
	})
	req, _ = http.NewRequest("POST", fmt.Sprintf("/api/v1/participations/claims/%s/review", claimID), bytes.NewBuffer(reviewSelfPayload))
	req.Header.Set("Authorization", "Bearer "+member1Token)
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("Expected 403 Forbidden for member self-review attempt, got %d", rr.Code)
	}

	// 10. Independent Admin reviews and approves claim
	adminReviewPayload, _ := json.Marshal(map[string]interface{}{
		"decision": "approved",
		"reason":   "Verified photographic evidence and timestamps",
	})
	req, _ = http.NewRequest("POST", fmt.Sprintf("/api/v1/participations/claims/%s/review", claimID), bytes.NewBuffer(adminReviewPayload))
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("Admin review claim failed: code %d, body %s", rr.Code, rr.Body.String())
	}

	// 11. Member 1 checks badges via HTTP -> Awarded badge is present!
	req, _ = http.NewRequest("GET", "/api/v1/participations/my-badges", nil)
	req.Header.Set("Authorization", "Bearer "+member1Token)
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("Get my badges failed: code %d, body %s", rr.Code, rr.Body.String())
	}
	var myBadges []map[string]interface{}
	_ = json.Unmarshal(rr.Body.Bytes(), &myBadges)
	if len(myBadges) == 0 {
		t.Fatalf("Expected at least 1 awarded badge for Member 1, got 0")
	}

	badgeName, ok := myBadges[0]["badge_name"].(string)
	if !ok || badgeName == "" {
		t.Errorf("Expected valid badge_name in response, got %v", myBadges[0]["badge_name"])
	}

	// 12. Direct DB Persistence Check: Verify registration and claim records exist persistently in database
	dbReg, err := h.Repo.Queries().GetEventRegistration(ctx, db.GetEventRegistrationParams{
		EventID: eventID,
		UserID:  member1ID,
	})
	if err != nil {
		t.Fatalf("Failed to fetch persistent registration record from DB: %v", err)
	}
	if dbReg.Status != "attended" {
        t.Errorf("Expected registration record Status='attended', got %s", dbReg.Status)
    }

	dbClaim, err := h.Repo.Queries().GetParticipationClaimByID(ctx, claimID)
	if err != nil {
		t.Fatalf("Failed to fetch persistent claim record from DB: %v", err)
	}
	if dbClaim.Status != "verified" {
		t.Errorf("Expected claim record Status='verified', got %s", dbClaim.Status)
	}
}
