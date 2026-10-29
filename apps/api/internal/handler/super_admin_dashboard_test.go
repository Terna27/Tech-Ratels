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

func TestSuperAdminDashboard_AuthorizationEnforcement(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()

	_, memberToken := createTestUserWithRole(t, h, "sa_test_mbr", "Standard Member", "member")
	_, natAdminToken := createTestUserWithRole(t, h, "sa_test_nat", "National Admin", "national_admin")
	_, stateCoordToken := createTestUserWithRole(t, h, "sa_test_st", "State Coordinator", "state_coordinator")
	_, zonalCoordToken := createTestUserWithRole(t, h, "sa_test_zn", "Zonal Coordinator", "zonal_coordinator")
	_, lgaRepToken := createTestUserWithRole(t, h, "sa_test_lga", "LGA Rep", "lga_representative")
	_, superAdminToken := createTestUserWithRole(t, h, "sa_test_root", "Super Admin", "super_admin")

	endpoints := []struct {
		method string
		path   string
	}{
		{"GET", "/api/v1/admin/super/dashboard"},
		{"GET", "/api/v1/admin/super/coverage/zones"},
		{"GET", "/api/v1/admin/super/coverage/states"},
		{"GET", "/api/v1/admin/super/coverage/lgas"},
		{"GET", "/api/v1/admin/audit-logs"},
		{"GET", "/api/v1/admin/audit"},
		{"POST", "/api/v1/admin/geography/seed"},
		{"GET", "/api/v1/admin/settings"},
		{"PUT", "/api/v1/admin/settings/movement_motto"},
	}

	for _, ep := range endpoints {
		var reqBody *bytes.Buffer
		if ep.method == "POST" || ep.method == "PUT" {
			reqBody = bytes.NewBufferString(`{"value":"test"}`)
		}

		// 1. Unauthenticated -> 401 Unauthorized
		var req *http.Request
		if reqBody != nil {
			req, _ = http.NewRequest(ep.method, ep.path, bytes.NewBufferString(`{"value":"test"}`))
			req.Header.Set("Content-Type", "application/json")
		} else {
			req, _ = http.NewRequest(ep.method, ep.path, nil)
		}
		rr := httptest.NewRecorder()
		h.Router.ServeHTTP(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Errorf("[%s %s] Expected 401 for unauthenticated request, got %d", ep.method, ep.path, rr.Code)
		}

		// 2. Standard Member -> 403 Forbidden
		if reqBody != nil {
			req, _ = http.NewRequest(ep.method, ep.path, bytes.NewBufferString(`{"value":"test"}`))
			req.Header.Set("Content-Type", "application/json")
		} else {
			req, _ = http.NewRequest(ep.method, ep.path, nil)
		}
		req.Header.Set("Authorization", "Bearer "+memberToken)
		rr = httptest.NewRecorder()
		h.Router.ServeHTTP(rr, req)
		if rr.Code != http.StatusForbidden {
			t.Errorf("[%s %s] Expected 403 for member, got %d", ep.method, ep.path, rr.Code)
		}

		// 3. National Admin -> 403 Forbidden (prevent privilege escalation)
		if reqBody != nil {
			req, _ = http.NewRequest(ep.method, ep.path, bytes.NewBufferString(`{"value":"test"}`))
			req.Header.Set("Content-Type", "application/json")
		} else {
			req, _ = http.NewRequest(ep.method, ep.path, nil)
		}
		req.Header.Set("Authorization", "Bearer "+natAdminToken)
		rr = httptest.NewRecorder()
		h.Router.ServeHTTP(rr, req)
		if rr.Code != http.StatusForbidden {
			t.Errorf("[%s %s] Expected 403 for national_admin, got %d", ep.method, ep.path, rr.Code)
		}

		// 4. State Coordinator -> 403 Forbidden
		if reqBody != nil {
			req, _ = http.NewRequest(ep.method, ep.path, bytes.NewBufferString(`{"value":"test"}`))
			req.Header.Set("Content-Type", "application/json")
		} else {
			req, _ = http.NewRequest(ep.method, ep.path, nil)
		}
		req.Header.Set("Authorization", "Bearer "+stateCoordToken)
		rr = httptest.NewRecorder()
		h.Router.ServeHTTP(rr, req)
		if rr.Code != http.StatusForbidden {
			t.Errorf("[%s %s] Expected 403 for state_coordinator, got %d", ep.method, ep.path, rr.Code)
		}

		// 5. Zonal Coordinator -> 403 Forbidden
		if reqBody != nil {
			req, _ = http.NewRequest(ep.method, ep.path, bytes.NewBufferString(`{"value":"test"}`))
			req.Header.Set("Content-Type", "application/json")
		} else {
			req, _ = http.NewRequest(ep.method, ep.path, nil)
		}
		req.Header.Set("Authorization", "Bearer "+zonalCoordToken)
		rr = httptest.NewRecorder()
		h.Router.ServeHTTP(rr, req)
		if rr.Code != http.StatusForbidden {
			t.Errorf("[%s %s] Expected 403 for zonal_coordinator, got %d", ep.method, ep.path, rr.Code)
		}

		// 6. LGA Representative -> 403 Forbidden
		if reqBody != nil {
			req, _ = http.NewRequest(ep.method, ep.path, bytes.NewBufferString(`{"value":"test"}`))
			req.Header.Set("Content-Type", "application/json")
		} else {
			req, _ = http.NewRequest(ep.method, ep.path, nil)
		}
		req.Header.Set("Authorization", "Bearer "+lgaRepToken)
		rr = httptest.NewRecorder()
		h.Router.ServeHTTP(rr, req)
		if rr.Code != http.StatusForbidden {
			t.Errorf("[%s %s] Expected 403 for lga_representative, got %d", ep.method, ep.path, rr.Code)
		}

		// 7. Super Admin -> Authorized (not 401 or 403)
		if reqBody != nil {
			req, _ = http.NewRequest(ep.method, ep.path, bytes.NewBufferString(`{"value":"test"}`))
			req.Header.Set("Content-Type", "application/json")
		} else {
			req, _ = http.NewRequest(ep.method, ep.path, nil)
		}
		req.Header.Set("Authorization", "Bearer "+superAdminToken)
		rr = httptest.NewRecorder()
		h.Router.ServeHTTP(rr, req)
		if rr.Code == http.StatusUnauthorized || rr.Code == http.StatusForbidden {
			t.Errorf("[%s %s] Expected Super Admin to be authorized, got %d", ep.method, ep.path, rr.Code)
		}
	}
}

func TestSuperAdminDashboard_SuperAdminAccessAndRealStatistics(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()

	_, superAdminToken := createTestUserWithRole(t, h, "sa_stats_root", "Super Admin", "super_admin")

	// 1. Fetch Super Admin Dashboard Statistics
	req, _ := http.NewRequest("GET", "/api/v1/admin/super/dashboard", nil)
	req.Header.Set("Authorization", "Bearer "+superAdminToken)
	rr := httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for super admin dashboard, got %d: %s", rr.Code, rr.Body.String())
	}

	var stats service.SuperAdminDashboardStats
	if err := json.Unmarshal(rr.Body.Bytes(), &stats); err != nil {
		t.Fatalf("Failed to decode SuperAdminDashboardStats JSON: %v", err)
	}

	// Verify authoritative metrics
	if stats.TotalMembers <= 0 {
		t.Errorf("Expected TotalMembers > 0, got %d", stats.TotalMembers)
	}
	if stats.TotalZones != 6 {
		t.Errorf("Expected TotalZones == 6, got %d", stats.TotalZones)
	}
	if stats.TotalStates != 37 {
		t.Errorf("Expected TotalStates == 37, got %d", stats.TotalStates)
	}
	if stats.TotalLGAs != 774 {
		t.Errorf("Expected TotalLGAs == 774, got %d", stats.TotalLGAs)
	}
	if stats.ZoneCoverage == nil || len(stats.ZoneCoverage) != 6 {
		t.Errorf("Expected ZoneCoverage slice of length 6, got %d", len(stats.ZoneCoverage))
	}
	if stats.UpcomingActivities == nil {
		t.Errorf("Expected UpcomingActivities to be non-nil slice, got nil")
	}
	if stats.RecentAuditActions == nil {
		t.Errorf("Expected RecentAuditActions to be non-nil slice, got nil")
	}

	// 2. Fetch Zone Coverage List
	req, _ = http.NewRequest("GET", "/api/v1/admin/super/coverage/zones", nil)
	req.Header.Set("Authorization", "Bearer "+superAdminToken)
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for zone coverage, got %d: %s", rr.Code, rr.Body.String())
	}

	var zoneRows []map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &zoneRows); err != nil {
		t.Fatalf("Failed to decode zone coverage JSON: %v", err)
	}
	if len(zoneRows) != 6 {
		t.Errorf("Expected 6 zones, got %d", len(zoneRows))
	}

	// 3. Fetch State Coverage List
	req, _ = http.NewRequest("GET", "/api/v1/admin/super/coverage/states", nil)
	req.Header.Set("Authorization", "Bearer "+superAdminToken)
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for state coverage, got %d: %s", rr.Code, rr.Body.String())
	}

	var stateRows []map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &stateRows); err != nil {
		t.Fatalf("Failed to decode state coverage JSON: %v", err)
	}
	if len(stateRows) != 37 {
		t.Errorf("Expected 37 states, got %d", len(stateRows))
	}

	firstStateID := stateRows[0]["state_id"].(string)

	// 4. Fetch LGA Coverage List (all LGAs)
	req, _ = http.NewRequest("GET", "/api/v1/admin/super/coverage/lgas", nil)
	req.Header.Set("Authorization", "Bearer "+superAdminToken)
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for lga coverage, got %d: %s", rr.Code, rr.Body.String())
	}

	var allLGAs []map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &allLGAs); err != nil {
		t.Fatalf("Failed to decode lga coverage JSON: %v", err)
	}
	if len(allLGAs) != 774 {
		t.Errorf("Expected 774 LGAs, got %d", len(allLGAs))
	}

	// 5. Fetch LGA Coverage List filtered by state
	req, _ = http.NewRequest("GET", "/api/v1/admin/super/coverage/lgas?state_id="+firstStateID, nil)
	req.Header.Set("Authorization", "Bearer "+superAdminToken)
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for state-filtered lga coverage, got %d: %s", rr.Code, rr.Body.String())
	}

	var stateLGAs []map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &stateLGAs); err != nil {
		t.Fatalf("Failed to decode filtered lga coverage JSON: %v", err)
	}
	if len(stateLGAs) == 0 || len(stateLGAs) >= 774 {
		t.Errorf("Expected filtered LGAs count between 1 and 773, got %d", len(stateLGAs))
	}
}

func TestSuperAdminDashboard_AuditLogsPrivilege(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()

	_, superAdminToken := createTestUserWithRole(t, h, "sa_audit_root", "Super Admin", "super_admin")
	_, natAdminToken := createTestUserWithRole(t, h, "sa_audit_nat", "National Admin", "national_admin")

	// 1. National Admin cannot access audit logs
	req, _ := http.NewRequest("GET", "/api/v1/admin/audit-logs", nil)
	req.Header.Set("Authorization", "Bearer "+natAdminToken)
	rr := httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("Expected 403 Forbidden for national_admin accessing /api/v1/admin/audit-logs, got %d", rr.Code)
	}

	// 2. Super Admin can access audit logs with pagination
	req, _ = http.NewRequest("GET", "/api/v1/admin/audit-logs?page=1&limit=10", nil)
	req.Header.Set("Authorization", "Bearer "+superAdminToken)
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for super admin accessing audit-logs, got %d: %s", rr.Code, rr.Body.String())
	}

	var paginatedResp service.PaginatedAuditLogsResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &paginatedResp); err != nil {
		t.Fatalf("Failed to parse paginated audit logs: %v", err)
	}
	if paginatedResp.Page != 1 {
		t.Errorf("Expected Page 1, got %d", paginatedResp.Page)
	}
	if paginatedResp.Limit != 10 {
		t.Errorf("Expected Limit 10, got %d", paginatedResp.Limit)
	}

	// 3. Super Admin can access /admin/audit alias
	req, _ = http.NewRequest("GET", "/api/v1/admin/audit?page=1&limit=5", nil)
	req.Header.Set("Authorization", "Bearer "+superAdminToken)
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for super admin accessing /admin/audit alias, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestSuperAdminDashboard_PreserveExistingCoordinatorOverview(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()

	_, natAdminToken := createTestUserWithRole(t, h, "sa_coord_nat", "National Admin", "national_admin")
	stateCoordID, stateCoordToken := createTestUserWithRole(t, h, "sa_coord_st", "State Coordinator", "state_coordinator")
	stateID := getVacantStateID(t, h)
	insertTestAppointment(t, h, stateCoordID, "state_coordinator", "State Coordinator", "state", &stateID, "active", time.Now().Add(-time.Hour), nil)

	// Standard coordinators CAN still access standard overview
	for _, tok := range []string{natAdminToken, stateCoordToken} {
		req, _ := http.NewRequest("GET", "/api/v1/admin/overview", nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		rr := httptest.NewRecorder()
		h.Router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected 200 OK for coordinator accessing /api/v1/admin/overview, got %d: %s", rr.Code, rr.Body.String())
		}

		var overview service.AdminOverviewResponse
		if err := json.Unmarshal(rr.Body.Bytes(), &overview); err != nil {
			t.Errorf("Failed to decode AdminOverviewResponse: %v", err)
		}
	}
}

func TestSuperAdminDashboard_PrivilegedActionAuditLogging(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()
	ctx := context.Background()

	superAdminID, superAdminToken := createTestUserWithRole(t, h, "sa_seed_root", "Super Admin", "super_admin")

	// Execute privileged Geography Seeding
	req, _ := http.NewRequest("POST", "/api/v1/admin/geography/seed", bytes.NewBuffer([]byte("{}")))
	req.Header.Set("Authorization", "Bearer "+superAdminToken)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for geography seed, got %d: %s", rr.Code, rr.Body.String())
	}

	// Verify that an audit record was written recording this action
	var auditCount int
	err := h.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM audit_events 
		WHERE actor_id = $1 AND action = 'seed_geography' AND resource_type = 'geography'
	`, superAdminID).Scan(&auditCount)
	if err != nil {
		t.Fatalf("Failed to query audit_events: %v", err)
	}
	if auditCount < 1 {
		t.Errorf("Expected at least 1 audit event for seed_geography by actor %s, found %d", superAdminID, auditCount)
	}
}

func TestSuperAdminDashboard_UpcomingActivitiesAndRealMetrics(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()
	ctx := context.Background()

	superAdminID, superAdminToken := createTestUserWithRole(t, h, "sa_act_root", "Super Admin", "super_admin")

	// Get a state and LGA
	states, err := h.GeoService.ListStates(ctx)
	if err != nil || len(states) == 0 {
		t.Fatalf("Failed to list states: %v", err)
	}
	stateID := states[0].ID
	lgas, err := h.GeoService.ListLGAsByState(ctx, stateID)
	if err != nil || len(lgas) == 0 {
		t.Fatalf("Failed to list lgas: %v", err)
	}
	lgaID := lgas[0].ID

	// Create an upcoming cleanup activity scheduled as the very next event
	startTime := time.Now().Add(1 * time.Minute)
	endTime := startTime.Add(4 * time.Hour)
	eventID := "sa-test-evt-" + time.Now().Format("20060102150405.000000000")
	var activityID string
	err = h.Pool.QueryRow(ctx, `
		INSERT INTO event_instances (
			id, title, programme_type, status,
			state_id, lga_id, address, landmark,
			start_time, end_time, capacity, registered_count,
			coordinator_id, public_contact
		) VALUES (
			$1, 'Lekki Beach Mega Cleanup', 'cleanup', 'published',
			$2, $3, 'Lekki Toll Gate Point', 'Toll Gate',
			$4, $5, 100, 25,
			$6, 'cleanup@ratels.example.org'
		) RETURNING id
	`, eventID, stateID, lgaID, startTime, endTime, superAdminID).Scan(&activityID)
	if err != nil {
		t.Fatalf("Failed to insert cleanup activity: %v", err)
	}

	// Query dashboard stats
	req, _ := http.NewRequest("GET", "/api/v1/admin/super/dashboard", nil)
	req.Header.Set("Authorization", "Bearer "+superAdminToken)
	rr := httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d: %s", rr.Code, rr.Body.String())
	}

	var stats service.SuperAdminDashboardStats
	if err := json.Unmarshal(rr.Body.Bytes(), &stats); err != nil {
		t.Fatalf("Failed to unmarshal stats: %v", err)
	}

	if stats.UpcomingCleanupsCount < 1 {
		t.Errorf("Expected UpcomingCleanupsCount >= 1, got %d", stats.UpcomingCleanupsCount)
	}

	foundActivity := false
	for _, act := range stats.UpcomingActivities {
		if act.Title == "Lekki Beach Mega Cleanup" {
			foundActivity = true
			if act.Status != "published" {
				t.Errorf("Expected status 'published', got %s", act.Status)
			}
		}
	}
	if !foundActivity {
		t.Errorf("Expected 'Lekki Beach Mega Cleanup' in UpcomingActivities")
	}
}

func TestSuperAdminDashboard_InvalidParametersAndRobustness(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()

	_, superAdminToken := createTestUserWithRole(t, h, "sa_robust_root", "Super Admin", "super_admin")

	// 1. Audit logs with negative page & limit clamps to safe defaults (page=1, limit=20)
	req, _ := http.NewRequest("GET", "/api/v1/admin/audit-logs?page=-5&limit=-20", nil)
	req.Header.Set("Authorization", "Bearer "+superAdminToken)
	rr := httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for clamped audit log request, got %d", rr.Code)
	}

	var resp service.PaginatedAuditLogsResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if resp.Page != 1 {
		t.Errorf("Expected page clamped to 1, got %d", resp.Page)
	}
	if resp.Limit != 20 {
		t.Errorf("Expected limit clamped to 20, got %d", resp.Limit)
	}

	// 2. Audit logs with limit > 100 clamps to 20
	req, _ = http.NewRequest("GET", "/api/v1/admin/audit-logs?page=1&limit=5000", nil)
	req.Header.Set("Authorization", "Bearer "+superAdminToken)
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for excessive limit request, got %d", rr.Code)
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if resp.Limit != 20 {
		t.Errorf("Expected limit clamped to 20, got %d", resp.Limit)
	}

	// 3. LGA coverage for non-existent state ID returns empty JSON array, not null or 500
	req, _ = http.NewRequest("GET", "/api/v1/admin/super/coverage/lgas?state_id=00000000-0000-0000-0000-000000000000", nil)
	req.Header.Set("Authorization", "Bearer "+superAdminToken)
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for non-existent state LGA query, got %d", rr.Code)
	}

	var emptyLgas []service.LGACoverageRow
	if err := json.Unmarshal(rr.Body.Bytes(), &emptyLgas); err != nil {
		t.Fatalf("Failed to parse empty LGA coverage: %v", err)
	}
	if len(emptyLgas) != 0 {
		t.Errorf("Expected 0 LGAs for non-existent state, got %d", len(emptyLgas))
	}
}

func TestSuperAdminDashboard_OrganizationSettings(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()
	ctx := context.Background()

	superAdminID, superAdminToken := createTestUserWithRole(t, h, "sa_org_root", "Super Admin", "super_admin")
	_, natAdminToken := createTestUserWithRole(t, h, "sa_org_nat", "National Admin", "national_admin")
	_, memberToken := createTestUserWithRole(t, h, "sa_org_mbr", "Member", "member")

	// 1. Unauthenticated cannot list settings -> 401
	req, _ := http.NewRequest("GET", "/api/v1/admin/settings", nil)
	rr := httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 for unauthenticated settings access, got %d", rr.Code)
	}

	// 2. Member cannot list settings -> 403
	req, _ = http.NewRequest("GET", "/api/v1/admin/settings", nil)
	req.Header.Set("Authorization", "Bearer "+memberToken)
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("Expected 403 for member settings access, got %d", rr.Code)
	}

	// 3. National Admin cannot list settings -> 403
	req, _ = http.NewRequest("GET", "/api/v1/admin/settings", nil)
	req.Header.Set("Authorization", "Bearer "+natAdminToken)
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("Expected 403 for national admin settings access, got %d", rr.Code)
	}

	// 4. Super Admin can list settings -> 200 OK
	req, _ = http.NewRequest("GET", "/api/v1/admin/settings", nil)
	req.Header.Set("Authorization", "Bearer "+superAdminToken)
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for Super Admin listing settings, got %d: %s", rr.Code, rr.Body.String())
	}

	var settings []map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &settings); err != nil {
		t.Fatalf("Failed to decode settings JSON: %v", err)
	}
	if len(settings) == 0 {
		t.Fatalf("Expected seeded organization settings, got empty list")
	}

	// Verify baseline setting exists
	foundMotto := false
	for _, s := range settings {
		if s["key"] == "movement_motto" {
			foundMotto = true
			break
		}
	}
	if !foundMotto {
		t.Errorf("Expected 'movement_motto' in organization settings")
	}

	// 5. National Admin cannot update setting -> 403 Forbidden
	updatePayload := bytes.NewBufferString(`{"value":"Updated Motto by Nat Admin"}`)
	req, _ = http.NewRequest("PUT", "/api/v1/admin/settings/movement_motto", updatePayload)
	req.Header.Set("Authorization", "Bearer "+natAdminToken)
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("Expected 403 Forbidden for National Admin updating setting, got %d", rr.Code)
	}

	// 6. Super Admin updates setting -> 200 OK
	newMotto := "Clean Communities, Accountable Leadership - 2026"
	updatePayload = bytes.NewBufferString(fmt.Sprintf(`{"value":"%s"}`, newMotto))
	req, _ = http.NewRequest("PUT", "/api/v1/admin/settings/movement_motto", updatePayload)
	req.Header.Set("Authorization", "Bearer "+superAdminToken)
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for Super Admin updating setting, got %d: %s", rr.Code, rr.Body.String())
	}

	var updatedSetting map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &updatedSetting); err != nil {
		t.Fatalf("Failed to decode updated setting JSON: %v", err)
	}
	if updatedSetting["value"] != newMotto {
		t.Errorf("Expected setting value %s, got %v", newMotto, updatedSetting["value"])
	}

	// 7. Verify Audit Log was generated for organization setting update
	var auditCount int
	var auditDetails string
	err := h.Pool.QueryRow(ctx, `
		SELECT COUNT(*), COALESCE(MAX(details), '')
		FROM audit_events
		WHERE actor_id = $1 AND action = 'update_organization_setting' AND resource_id = 'movement_motto'
	`, superAdminID).Scan(&auditCount, &auditDetails)
	if err != nil {
		t.Fatalf("Failed to query audit events: %v", err)
	}
	if auditCount < 1 {
		t.Errorf("Expected at least 1 audit event for update_organization_setting, found %d", auditCount)
	}
	if !strings.Contains(auditDetails, newMotto) {
		t.Errorf("Expected audit details to record new value %s, got: %s", newMotto, auditDetails)
	}

	// 8. Updating non-existent setting returns 400 Bad Request
	nonExistentPayload := bytes.NewBufferString(`{"value":"val"}`)
	req, _ = http.NewRequest("PUT", "/api/v1/admin/settings/non_existent_key_12345", nonExistentPayload)
	req.Header.Set("Authorization", "Bearer "+superAdminToken)
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request for non-existent setting update, got %d", rr.Code)
	}
}

func TestSuperAdminDashboard_EmptyDatabaseStates(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()
	ctx := context.Background()

	// Verify that querying with empty event_instances or empty appointments returns clean zero counts and non-nil slices
	_, superAdminToken := createTestUserWithRole(t, h, "sa_empty_root", "Super Admin", "super_admin")

	req, _ := http.NewRequest("GET", "/api/v1/admin/super/dashboard", nil)
	req.Header.Set("Authorization", "Bearer "+superAdminToken)
	rr := httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for empty state query, got %d: %s", rr.Code, rr.Body.String())
	}

	var stats service.SuperAdminDashboardStats
	if err := json.Unmarshal(rr.Body.Bytes(), &stats); err != nil {
		t.Fatalf("Failed to unmarshal dashboard stats: %v", err)
	}

	// Ensure non-nil slices
	if stats.UpcomingActivities == nil {
		t.Errorf("Expected non-nil UpcomingActivities slice")
	}
	if stats.RecentAuditActions == nil {
		t.Errorf("Expected non-nil RecentAuditActions slice")
	}
	if stats.ZoneCoverage == nil {
		t.Errorf("Expected non-nil ZoneCoverage slice")
	}

	_ = ctx
}

func TestSuperAdminDashboard_DatabaseErrorsNotSilenced(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()

	// 1. When context is canceled, service must return an error rather than mock zero stats
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	stats, err := h.AdminService.GetSuperAdminDashboardStats(canceledCtx)
	if err == nil {
		t.Errorf("Expected error when calling GetSuperAdminDashboardStats with canceled context, got nil error with stats: %+v", stats)
	}
	if stats != nil {
		t.Errorf("Expected nil stats on database error, got non-nil: %+v", stats)
	}
}

func TestSuperAdminDashboard_JurisdictionRestrictionsAndHorizontalEscalation(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()
	ctx := context.Background()

	// Setup state coordinator
	stateCoordID, stateCoordToken := createTestUserWithRole(t, h, "sa_esc_st", "State Coord", "state_coordinator")
	stateID := getVacantStateID(t, h)
	insertTestAppointment(t, h, stateCoordID, "state_coordinator", "State Coordinator", "state", &stateID, "active", time.Now().Add(-time.Hour), nil)

	// 1. State coordinator attempts to access nationwide dashboard -> 403 Forbidden
	req, _ := http.NewRequest("GET", "/api/v1/admin/super/dashboard", nil)
	req.Header.Set("Authorization", "Bearer "+stateCoordToken)
	rr := httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("Expected 403 Forbidden for state coordinator accessing nationwide dashboard, got %d", rr.Code)
	}

	// 2. State coordinator attempts to modify organization settings -> 403 Forbidden
	req, _ = http.NewRequest("PUT", "/api/v1/admin/settings/movement_name", bytes.NewBufferString(`{"value":"Rogue Name"}`))
	req.Header.Set("Authorization", "Bearer "+stateCoordToken)
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("Expected 403 Forbidden for state coordinator modifying organization settings, got %d", rr.Code)
	}

	// 3. State coordinator CAN access standard /admin/overview
	req, _ = http.NewRequest("GET", "/api/v1/admin/overview", nil)
	req.Header.Set("Authorization", "Bearer "+stateCoordToken)
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200 OK for state coordinator accessing /admin/overview, got %d: %s", rr.Code, rr.Body.String())
	}

	_ = ctx
}
