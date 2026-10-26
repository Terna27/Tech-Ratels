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

	"github.com/jackc/pgx/v5/pgtype"
	"ratels-api/internal/db"
	"ratels-api/internal/security"
	"ratels-api/internal/service"
	"ratels-api/internal/worker"
)

// 1. Admin dashboard statistics retrieval with real counts and next expected period
func TestAdminDashboard_StatsRealData(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()

	uid := time.Now().UnixNano()
	creator, _ := createHandlerTestUser(t, h, "national_admin", fmt.Sprintf("na_dash_%d@test.com", uid))
	approver, adminToken := createHandlerTestUser(t, h, "super_admin", fmt.Sprintf("sa_dash_%d@test.com", uid))
	period := fmt.Sprintf("202%d-01", uid%3)

	defer func() {
		_, _ = h.Pool.Exec(context.Background(), "DELETE FROM public_report_snapshots WHERE reporting_period = $1", period)
	}()

	createAndPublishTestReport(t, h, creator, approver, "cleanups", period)

	req, _ := http.NewRequest("GET", "/api/v1/admin/reports/dashboard/stats", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	rr := httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from dashboard stats, got %d: %s", rr.Code, rr.Body.String())
	}

	var stats service.AdminDashboardStatsDTO
	if err := json.Unmarshal(rr.Body.Bytes(), &stats); err != nil {
		t.Fatalf("failed unmarshaling dashboard stats: %v", err)
	}

	if stats.TotalSnapshots < 1 {
		t.Errorf("expected at least 1 total snapshot, got %d", stats.TotalSnapshots)
	}
	if stats.PublishedReports < 1 {
		t.Errorf("expected at least 1 published report, got %d", stats.PublishedReports)
	}
	if stats.NextExpectedPeriod.PeriodLabel == "" {
		t.Errorf("expected next expected period label to be populated")
	}
	if stats.NextExpectedPeriod.ExpectedDate.IsZero() {
		t.Errorf("expected next expected date to be set")
	}
}

// 2. Scheduler run history listing and detail endpoint
func TestAdminDashboard_SchedulerRunHistory(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()

	uid := time.Now().UnixNano()
	_, adminToken := createHandlerTestUser(t, h, "super_admin", fmt.Sprintf("sa_hist_%d@test.com", uid))
	period := fmt.Sprintf("202%d-02", uid%3)

	defer func() {
		_, _ = h.Pool.Exec(context.Background(), "DELETE FROM public_report_snapshots WHERE reporting_period = $1", period)
		_, _ = h.Pool.Exec(context.Background(), "DELETE FROM public_report_scheduler_runs WHERE reporting_period = $1", period)
	}()

	// Trigger run
	body, _ := json.Marshal(map[string]string{
		"target_period": period,
	})
	runReq, _ := http.NewRequest("POST", "/api/v1/admin/reports/scheduler/run", bytes.NewReader(body))
	runReq.Header.Set("Content-Type", "application/json")
	runReq.Header.Set("Authorization", "Bearer "+adminToken)
	runRR := httptest.NewRecorder()
	h.Router.ServeHTTP(runRR, runReq)
	if runRR.Code != http.StatusOK {
		t.Fatalf("trigger failed with %d: %s", runRR.Code, runRR.Body.String())
	}

	// List runs
	listReq, _ := http.NewRequest("GET", "/api/v1/admin/reports/scheduler/runs?reporting_period="+period, nil)
	listReq.Header.Set("Authorization", "Bearer "+adminToken)
	listRR := httptest.NewRecorder()
	h.Router.ServeHTTP(listRR, listReq)
	if listRR.Code != http.StatusOK {
		t.Fatalf("expected 200 from listing runs, got %d: %s", listRR.Code, listRR.Body.String())
	}

	var listRes service.PaginatedSchedulerRunsResponse
	if err := json.Unmarshal(listRR.Body.Bytes(), &listRes); err != nil {
		t.Fatalf("failed unmarshaling scheduler runs: %v", err)
	}

	if listRes.TotalCount < 1 || len(listRes.Runs) == 0 {
		t.Fatalf("expected at least 1 scheduler run for period %s", period)
	}

	firstRun := listRes.Runs[0]
	if firstRun.Status != "completed" {
		t.Errorf("expected completed status, got %s", firstRun.Status)
	}
	if firstRun.RunIdentifier == "" {
		t.Errorf("expected non-empty run identifier")
	}

	// Get detail by ID
	detReq, _ := http.NewRequest("GET", "/api/v1/admin/reports/scheduler/runs/"+firstRun.ID, nil)
	detReq.Header.Set("Authorization", "Bearer "+adminToken)
	detRR := httptest.NewRecorder()
	h.Router.ServeHTTP(detRR, detReq)
	if detRR.Code != http.StatusOK {
		t.Fatalf("expected 200 from run detail, got %d: %s", detRR.Code, detRR.Body.String())
	}

	var detail service.SchedulerRunDTO
	if err := json.Unmarshal(detRR.Body.Bytes(), &detail); err != nil {
		t.Fatalf("failed unmarshaling run detail: %v", err)
	}
	if detail.RunIdentifier != firstRun.RunIdentifier {
		t.Errorf("expected matching run identifier %s, got %s", firstRun.RunIdentifier, detail.RunIdentifier)
	}
}

// 3. Manual monthly generation
func TestAdminDashboard_ManualMonthlyGeneration(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()

	uid := time.Now().UnixNano()
	_, adminToken := createHandlerTestUser(t, h, "national_admin", fmt.Sprintf("na_month_%d@test.com", uid))
	period := fmt.Sprintf("202%d-03", uid%3)

	defer func() {
		_, _ = h.Pool.Exec(context.Background(), "DELETE FROM public_report_snapshots WHERE reporting_period = $1", period)
	}()

	body, _ := json.Marshal(map[string]string{
		"target_period": period,
		"period_type":   "monthly",
	})
	req, _ := http.NewRequest("POST", "/api/v1/admin/reports/scheduler/run", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminToken)
	rr := httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for monthly manual generation, got %d: %s", rr.Code, rr.Body.String())
	}

	var res worker.SchedulerRunResult
	_ = json.Unmarshal(rr.Body.Bytes(), &res)
	if res.ReportsGenerated != 6 {
		t.Errorf("expected 6 reports generated for monthly cycle, got %d", res.ReportsGenerated)
	}
}

// 4. Manual quarterly generation
func TestAdminDashboard_ManualQuarterlyGeneration(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()

	uid := time.Now().UnixNano()
	_, adminToken := createHandlerTestUser(t, h, "super_admin", fmt.Sprintf("sa_quart_%d@test.com", uid))
	period := fmt.Sprintf("202%d-Q1", uid%3)

	defer func() {
		_, _ = h.Pool.Exec(context.Background(), "DELETE FROM public_report_snapshots WHERE reporting_period = $1", period)
	}()

	body, _ := json.Marshal(map[string]string{
		"target_period": period,
		"period_type":   "quarterly",
	})
	req, _ := http.NewRequest("POST", "/api/v1/admin/reports/scheduler/run", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminToken)
	rr := httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for quarterly manual generation, got %d: %s", rr.Code, rr.Body.String())
	}

	var res worker.SchedulerRunResult
	_ = json.Unmarshal(rr.Body.Bytes(), &res)
	if res.ReportsGenerated != 6 {
		t.Errorf("expected 6 reports generated for quarterly cycle, got %d", res.ReportsGenerated)
	}
}

// 5. Invalid reporting period format
func TestAdminDashboard_InvalidPeriodFormat(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()

	uid := time.Now().UnixNano()
	_, adminToken := createHandlerTestUser(t, h, "national_admin", fmt.Sprintf("na_inv_%d@test.com", uid))

	body, _ := json.Marshal(map[string]string{
		"target_period": "2026-99-INVALID",
	})
	req, _ := http.NewRequest("POST", "/api/v1/admin/reports/scheduler/run", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminToken)
	rr := httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid period format, got %d", rr.Code)
	}
}

// 6. Future-period rejection (cannot generate reports for future or unclosed periods)
func TestAdminDashboard_FuturePeriodRejection(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()

	uid := time.Now().UnixNano()
	_, adminToken := createHandlerTestUser(t, h, "super_admin", fmt.Sprintf("sa_fut_%d@test.com", uid))

	futurePeriods := []string{"2099-01", "2099-Q3"}

	for _, fut := range futurePeriods {
		body, _ := json.Marshal(map[string]string{
			"target_period": fut,
		})
		req, _ := http.NewRequest("POST", "/api/v1/admin/reports/scheduler/run", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+adminToken)
		rr := httptest.NewRecorder()
		h.Router.ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for future period %s, got %d: %s", fut, rr.Code, rr.Body.String())
		}
		if !strings.Contains(rr.Body.String(), "FUTURE_PERIOD_REJECTED") {
			t.Errorf("expected FUTURE_PERIOD_REJECTED code in response body, got: %s", rr.Body.String())
		}
	}
}

// 7. Concurrent manual requests return lock conflict
func TestAdminDashboard_ConcurrentManualRequests(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()

	uid := time.Now().UnixNano()
	_, adminToken := createHandlerTestUser(t, h, "super_admin", fmt.Sprintf("sa_concur_%d@test.com", uid))
	period := fmt.Sprintf("202%d-04", uid%3)

	defer func() {
		_, _ = h.Pool.Exec(context.Background(), "DELETE FROM public_report_snapshots WHERE reporting_period = $1", period)
	}()

	// Hold advisory lock directly in a transaction to simulate concurrent cycle
	ctx := context.Background()
	tx, err := h.Pool.Begin(ctx)
	if err != nil {
		t.Fatalf("failed starting tx: %v", err)
	}
	defer tx.Rollback(ctx)

	var acquired bool
	err = tx.QueryRow(ctx, "SELECT pg_try_advisory_xact_lock($1)", worker.DefaultSchedulerLockKey+100).Scan(&acquired)
	if err != nil || !acquired {
		t.Fatalf("failed acquiring advisory lock for test: %v", err)
	}

	body, _ := json.Marshal(map[string]string{
		"target_period": period,
	})
	req, _ := http.NewRequest("POST", "/api/v1/admin/reports/scheduler/run", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminToken)
	rr := httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409 Conflict when concurrent cycle is active, got %d: %s", rr.Code, rr.Body.String())
	}
}

// 8. Role-based authorization: member forbidden, unauthenticated 401
func TestAdminDashboard_RoleBasedAuthorization(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()

	uid := time.Now().UnixNano()
	_, memberToken := createHandlerTestUser(t, h, "member", fmt.Sprintf("mem_auth_%d@test.com", uid))

	endpoints := []struct {
		method string
		path   string
	}{
		{"GET", "/api/v1/admin/reports/dashboard/stats"},
		{"GET", "/api/v1/admin/reports/scheduler/runs"},
		{"GET", "/api/v1/admin/reports/exports"},
		{"POST", "/api/v1/admin/reports/scheduler/run"},
	}

	for _, ep := range endpoints {
		// 1. Unauthenticated -> 401
		reqUnauth, _ := http.NewRequest(ep.method, ep.path, nil)
		rrUnauth := httptest.NewRecorder()
		h.Router.ServeHTTP(rrUnauth, reqUnauth)
		if rrUnauth.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 for unauth %s %s, got %d", ep.method, ep.path, rrUnauth.Code)
		}

		// 2. Member -> 403
		reqMember, _ := http.NewRequest(ep.method, ep.path, nil)
		reqMember.Header.Set("Authorization", "Bearer "+memberToken)
		rrMember := httptest.NewRecorder()
		h.Router.ServeHTTP(rrMember, reqMember)
		if rrMember.Code != http.StatusForbidden {
			t.Errorf("expected 403 for member %s %s, got %d", ep.method, ep.path, rrMember.Code)
		}
	}
}

// 9. Export metadata retrieval without exposing internal storage credentials or keys
func TestAdminDashboard_ExportMetadataRetrieval(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()

	uid := time.Now().UnixNano()
	creator, _ := createHandlerTestUser(t, h, "national_admin", fmt.Sprintf("na_exp_%d@test.com", uid))
	approver, adminToken := createHandlerTestUser(t, h, "super_admin", fmt.Sprintf("sa_exp_%d@test.com", uid))
	period := fmt.Sprintf("202%d-05", uid%3)

	defer func() {
		_, _ = h.Pool.Exec(context.Background(), "DELETE FROM public_report_snapshots WHERE reporting_period = $1", period)
	}()

	snap, _ := createAndPublishTestReport(t, h, creator, approver, "cleanups", period)

	// Generate PDF export
	_, err := h.ReportService.GenerateAndStoreExport(context.Background(), approver, snap.SnapshotID, "pdf", nil)
	if err != nil {
		t.Fatalf("failed generating export: %v", err)
	}

	req, _ := http.NewRequest("GET", "/api/v1/admin/reports/exports?snapshot_id="+snap.SnapshotID, nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	rr := httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 from exports list, got %d: %s", rr.Code, rr.Body.String())
	}

	rawBody := rr.Body.String()
	// Security Assertion: Internal storage keys or secret references MUST NOT be leaked in client response
	if strings.Contains(rawBody, "storage_reference") {
		t.Errorf("raw storage_reference must not be exposed to client: %s", rawBody)
	}

	var res service.PaginatedAdminExportsResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed unmarshaling exports response: %v", err)
	}
	if res.TotalCount < 1 || len(res.Exports) == 0 {
		t.Fatalf("expected at least 1 export")
	}

	exp := res.Exports[0]
	if exp.ContentHash == "" {
		t.Errorf("expected non-empty SHA-256 checksum")
	}
	if !exp.StorageVerified {
		t.Errorf("expected storage_verified = true")
	}
}

// 10. Export regeneration permissions
func TestAdminDashboard_ExportRegenerationPermissions(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()

	uid := time.Now().UnixNano()
	creator, memberToken := createHandlerTestUser(t, h, "national_admin", fmt.Sprintf("na_regen_%d@test.com", uid))
	approver, adminToken := createHandlerTestUser(t, h, "super_admin", fmt.Sprintf("sa_regen_%d@test.com", uid))
	period := fmt.Sprintf("202%d-06", uid%3)

	defer func() {
		_, _ = h.Pool.Exec(context.Background(), "DELETE FROM public_report_snapshots WHERE reporting_period = $1", period)
	}()

	snap, _ := createAndPublishTestReport(t, h, creator, approver, "cleanups", period)

	// National Admin / Super Admin can regenerate
	reqRegen, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/admin/reports/%s/exports/pdf/regenerate", snap.SnapshotID), nil)
	reqRegen.Header.Set("Authorization", "Bearer "+adminToken)
	rrRegen := httptest.NewRecorder()
	h.Router.ServeHTTP(rrRegen, reqRegen)
	if rrRegen.Code != http.StatusOK {
		t.Fatalf("expected 200 for authorized export regeneration, got %d: %s", rrRegen.Code, rrRegen.Body.String())
	}

	// Unauthorized role (member) cannot regenerate
	_, ordinaryMemberToken := createHandlerTestUser(t, h, "member", fmt.Sprintf("ord_mem_%d@test.com", uid))
	reqMem, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/admin/reports/%s/exports/pdf/regenerate", snap.SnapshotID), nil)
	reqMem.Header.Set("Authorization", "Bearer "+ordinaryMemberToken)
	rrMem := httptest.NewRecorder()
	h.Router.ServeHTTP(rrMem, reqMem)
	if rrMem.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for unauthorized export regeneration, got %d", rrMem.Code)
	}
	_ = memberToken
}

// 11. Audit log filtering and pagination
func TestAdminDashboard_AuditLogFilteringAndPagination(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()

	uid := time.Now().UnixNano()
	creator, _ := createHandlerTestUser(t, h, "national_admin", fmt.Sprintf("na_audit_%d@test.com", uid))
	approver, adminToken := createHandlerTestUser(t, h, "super_admin", fmt.Sprintf("sa_audit_%d@test.com", uid))
	period := fmt.Sprintf("202%d-07", uid%3)

	defer func() {
		_, _ = h.Pool.Exec(context.Background(), "DELETE FROM public_report_snapshots WHERE reporting_period = $1", period)
	}()

	snap, _ := createAndPublishTestReport(t, h, creator, approver, "cleanups", period)

	// Filter by action "published" and snapshot ID
	req, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/admin/reports/audit?snapshot_id=%s&action=published&page=1&page_size=10", snap.SnapshotID), nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	rr := httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 from audit list, got %d: %s", rr.Code, rr.Body.String())
	}

	var res service.PaginatedAuditsResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed unmarshaling audits: %v", err)
	}

	if res.TotalCount < 1 || len(res.Audits) == 0 {
		t.Fatalf("expected at least 1 audit item for action 'published'")
	}
	if res.Audits[0].Action != "published" {
		t.Errorf("expected action 'published', got %s", res.Audits[0].Action)
	}
}

// 12. Publication governance and anti-self-approval
func TestAdminDashboard_PublicationGovernance(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()

	uid := time.Now().UnixNano()
	creator, creatorToken := createHandlerTestUser(t, h, "national_admin", fmt.Sprintf("na_gov_%d@test.com", uid))
	period := fmt.Sprintf("202%d-08", uid%3)

	defer func() {
		_, _ = h.Pool.Exec(context.Background(), "DELETE FROM public_report_snapshots WHERE reporting_period = $1", period)
	}()

	snap, err := h.ReportService.GenerateSnapshot(context.Background(), creator, service.GenerateReportRequest{
		ReportType:      "cleanups",
		Title:           "Cleanup Report " + period,
		ReportingPeriod: period,
		PeriodStart:     period + "-01",
		PeriodEnd:       period + "-28",
		ScopeType:       "national",
		TriggerSource:   "admin_manual",
	}, nil)
	if err != nil {
		t.Fatalf("failed creating snapshot: %v", err)
	}

	// Submit review
	_, err = h.ReportService.SubmitForReview(context.Background(), creator, snap.SnapshotID, nil)
	if err != nil {
		t.Fatalf("failed submitting review: %v", err)
	}

	// Self-approval prohibition: creator cannot approve own report -> 403 Forbidden
	approveReq, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/admin/reports/%s/approve", snap.SnapshotID), bytes.NewReader([]byte(`{"decision_reason":"Self approval"}`)))
	approveReq.Header.Set("Content-Type", "application/json")
	approveReq.Header.Set("Authorization", "Bearer "+creatorToken)
	approveRR := httptest.NewRecorder()
	h.Router.ServeHTTP(approveRR, approveReq)

	if approveRR.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for self-approval attempt, got %d: %s", approveRR.Code, approveRR.Body.String())
	}
	if !strings.Contains(approveRR.Body.String(), "SELF_APPROVAL_PROHIBITED") {
		t.Errorf("expected SELF_APPROVAL_PROHIBITED error code, got %s", approveRR.Body.String())
	}
}

// 13. Withdrawn report handling: cannot export while withdrawn; can create corrected revision draft
func TestAdminDashboard_WithdrawnReportHandling(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()

	uid := time.Now().UnixNano()
	creator, _ := createHandlerTestUser(t, h, "national_admin", fmt.Sprintf("na_with_%d@test.com", uid))
	approver, adminToken := createHandlerTestUser(t, h, "super_admin", fmt.Sprintf("sa_with_%d@test.com", uid))
	period := fmt.Sprintf("202%d-09", uid%3)

	defer func() {
		_, _ = h.Pool.Exec(context.Background(), "DELETE FROM public_report_snapshots WHERE reporting_period = $1", period)
	}()

	snap, _ := createAndPublishTestReport(t, h, creator, approver, "cleanups", period)

	// Withdraw publication
	withdrawReq, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/admin/reports/%s/withdraw", snap.SnapshotID), bytes.NewReader([]byte(`{"reason":"Audit discrepancy found"}`)))
	withdrawReq.Header.Set("Content-Type", "application/json")
	withdrawReq.Header.Set("Authorization", "Bearer "+adminToken)
	withdrawRR := httptest.NewRecorder()
	h.Router.ServeHTTP(withdrawRR, withdrawReq)
	if withdrawRR.Code != http.StatusOK {
		t.Fatalf("expected 200 for withdraw, got %d: %s", withdrawRR.Code, withdrawRR.Body.String())
	}

	// Cannot regenerate export while withdrawn
	regenReq, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/admin/reports/%s/exports/pdf/regenerate", snap.SnapshotID), nil)
	regenReq.Header.Set("Authorization", "Bearer "+adminToken)
	regenRR := httptest.NewRecorder()
	h.Router.ServeHTTP(regenRR, regenReq)
	if regenRR.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when regenerating withdrawn export, got %d", regenRR.Code)
	}

	// Can revise: generate corrected version draft -> 201 Created
	reviseReq, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/admin/reports/%s/revise", snap.SnapshotID), bytes.NewReader([]byte(`{"correction_notes":"Reconciled with ground survey slips"}`)))
	reviseReq.Header.Set("Content-Type", "application/json")
	reviseReq.Header.Set("Authorization", "Bearer "+adminToken)
	reviseRR := httptest.NewRecorder()
	h.Router.ServeHTTP(reviseRR, reviseReq)
	if reviseRR.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for revise action, got %d: %s", reviseRR.Code, reviseRR.Body.String())
	}

	var revisedSnap service.AdminSnapshotDetailDTO
	_ = json.Unmarshal(reviseRR.Body.Bytes(), &revisedSnap)
	if revisedSnap.Status != "draft" {
		t.Errorf("revised snapshot must be in 'draft' status, got %s", revisedSnap.Status)
	}
	if revisedSnap.PublicationVersion != 2 {
		t.Errorf("expected version 2, got %d", revisedSnap.PublicationVersion)
	}
}

// 14. Spoofed proxy headers rejection in rate limiter
func TestAdminDashboard_SpoofedProxyHeaders(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()

	uid := time.Now().UnixNano()
	creator, _ := createHandlerTestUser(t, h, "national_admin", fmt.Sprintf("na_sp_%d@test.com", uid))
	approver, _ := createHandlerTestUser(t, h, "super_admin", fmt.Sprintf("sa_sp_%d@test.com", uid))
	period := fmt.Sprintf("202%d-10", uid%3)

	defer func() {
		_, _ = h.Pool.Exec(context.Background(), "DELETE FROM public_report_snapshots WHERE reporting_period = $1", period)
	}()

	_, pub := createAndPublishTestReport(t, h, creator, approver, "cleanups", period)

	// Set tight rate limit of 2 requests
	limiter := security.NewDownloadRateLimiter(2, time.Minute)
	h.ReportHandler.SetRateLimiter(limiter)

	// Untrusted direct client sends spoofed CF-Connecting-IP
	untrustedDirectIP := "203.0.113.88:12345"

	// Request 1 from untrusted host spoofing IP 1.1.1.1
	req1, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/public/reports/publications/%s/pdf", pub.PublicationID), nil)
	req1.RemoteAddr = untrustedDirectIP
	req1.Header.Set("CF-Connecting-IP", "1.1.1.1")
	rr1 := httptest.NewRecorder()
	h.Router.ServeHTTP(rr1, req1)
	if rr1.Code != http.StatusOK {
		t.Fatalf("expected 200 for 1st request, got %d", rr1.Code)
	}

	// Request 2 from untrusted host spoofing IP 2.2.2.2 (trying to avoid rate limiter)
	req2, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/public/reports/publications/%s/pdf", pub.PublicationID), nil)
	req2.RemoteAddr = untrustedDirectIP
	req2.Header.Set("CF-Connecting-IP", "2.2.2.2")
	rr2 := httptest.NewRecorder()
	h.Router.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusOK {
		t.Fatalf("expected 200 for 2nd request, got %d", rr2.Code)
	}

	// Request 3 from same untrusted host spoofing IP 3.3.3.3 -> MUST be blocked with 429
	// because rate limiter uses socket IP 203.0.113.88, NOT spoofed headers!
	req3, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/public/reports/publications/%s/pdf", pub.PublicationID), nil)
	req3.RemoteAddr = untrustedDirectIP
	req3.Header.Set("CF-Connecting-IP", "3.3.3.3")
	rr3 := httptest.NewRecorder()
	h.Router.ServeHTTP(rr3, req3)

	if rr3.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 Too Many Requests: spoofed headers must NOT bypass rate limiter on untrusted connection! Got %d", rr3.Code)
	}
}

// 15. Task G: Snapshot detail inspection, aggregated data return, integrity hash, and exports
func TestAdminDashboard_SnapshotDetailRetrievalAndIntegrity(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()

	uid := time.Now().UnixNano()
	creator, _ := createHandlerTestUser(t, h, "national_admin", fmt.Sprintf("na_dtl_%d@test.com", uid))
	approver, adminToken := createHandlerTestUser(t, h, "super_admin", fmt.Sprintf("sa_dtl_%d@test.com", uid))
	period := fmt.Sprintf("202%d-11", uid%3)

	defer func() {
		_, _ = h.Pool.Exec(context.Background(), "DELETE FROM public_report_snapshots WHERE reporting_period = $1", period)
	}()

	snap, _ := createAndPublishTestReport(t, h, creator, approver, "cleanups", period)

	// Explicitly generate a PDF export
	_, err := h.ReportService.GenerateAndStoreExport(context.Background(), approver, snap.SnapshotID, "pdf", nil)
	if err != nil {
		t.Fatalf("failed generating test export: %v", err)
	}

	// Fetch detail as Super Admin
	req, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/admin/reports/%s", snap.SnapshotID), nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	rr := httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for snapshot detail, got %d: %s", rr.Code, rr.Body.String())
	}

	var detail service.AdminSnapshotDetailDTO
	if err := json.Unmarshal(rr.Body.Bytes(), &detail); err != nil {
		t.Fatalf("failed unmarshaling snapshot detail: %v", err)
	}

	// 1. Verify identification and status
	if detail.SnapshotID != snap.SnapshotID {
		t.Errorf("expected snapshot ID %s, got %s", snap.SnapshotID, detail.SnapshotID)
	}
	if detail.Status != "published" {
		t.Errorf("expected status published, got %s", detail.Status)
	}
	if detail.ReportType != "cleanups" {
		t.Errorf("expected cleanups report type, got %s", detail.ReportType)
	}

	// 2. Verify cryptographic hash
	if len(detail.DataHash) != 64 {
		t.Errorf("expected 64-char SHA-256 data hash, got %q", detail.DataHash)
	}

	// 3. Verify aggregated data
	if len(detail.AggregatedData) == 0 {
		t.Errorf("expected non-empty aggregated data")
	}
	var cleanupsData service.PublicCleanupsData
	if err := json.Unmarshal(detail.AggregatedData, &cleanupsData); err != nil {
		t.Errorf("failed parsing aggregated cleanups data: %v", err)
	}
	if cleanupsData.ReportingPeriod != period {
		t.Errorf("expected period %s, got %s", period, cleanupsData.ReportingPeriod)
	}

	// 4. Verify approvals history
	if len(detail.Approvals) == 0 {
		t.Errorf("expected approvals history to be populated")
	} else if detail.Approvals[0].Decision != "approved" {
		t.Errorf("expected approved decision, got %s", detail.Approvals[0].Decision)
	}

	// 5. Verify audit logs & IP address visibility for Super Admin
	if len(detail.AuditLogs) == 0 {
		t.Errorf("expected audit logs to be populated")
	}

	// 6. Verify exports list
	if len(detail.Exports) == 0 {
		t.Errorf("expected export artifacts to be populated for published report")
	}
}

// 16. Task G: Geographic scope enforcement on snapshot inspection
func TestAdminDashboard_SnapshotGeographicAuthorityRestriction(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()
	ctx := context.Background()

	uid := time.Now().UnixNano()
	superAdmin, _ := createHandlerTestUser(t, h, "super_admin", fmt.Sprintf("sa_geo_%d@test.com", uid))
	states, err := h.GeoService.ListStates(ctx)
	if err != nil || len(states) < 2 {
		t.Fatalf("failed fetching test states: %v", err)
	}
	lagosState := states[0]
	kanoState := states[1]

	// Create a state coordinator user for Lagos
	lagosCoord, lagosToken := createHandlerTestUser(t, h, "state_coordinator", fmt.Sprintf("sc_lagos_%d@test.com", uid))
	_, err = h.Repo.Queries().CreateAppointment(ctx, db.CreateAppointmentParams{
		ID:               security.GenerateUUID(),
		UserID:           lagosCoord.ID,
		RoleTitle:        "State Coordinator (Lagos)",
		RoleType:         "state_coordinator",
		JurisdictionType: "state",
		JurisdictionID:   pgtype.Text{String: lagosState.ID, Valid: true},
		StartDate:        pgtype.Timestamptz{Time: time.Now().Add(-1 * time.Hour), Valid: true},
		Status:           "active",
	})
	if err != nil {
		t.Fatalf("failed appointing lagos coordinator: %v", err)
	}

	period := fmt.Sprintf("202%d-12", uid%3)
	defer func() {
		_, _ = h.Pool.Exec(ctx, "DELETE FROM public_report_snapshots WHERE reporting_period = $1", period)
	}()

	// 1. Create a draft snapshot for Lagos State
	lagosSnap, err := h.ReportService.GenerateSnapshot(ctx, superAdmin, service.GenerateReportRequest{
		ReportType:      "cleanups",
		Title:           "Lagos Environmental Cleanup Report - " + period,
		ReportingPeriod: period,
		PeriodStart:     period + "-01",
		PeriodEnd:       period + "-28",
		ScopeType:       "state",
		StateID:         &lagosState.ID,
		TriggerSource:   "admin_manual",
	}, nil)
	if err != nil {
		t.Fatalf("failed generating lagos draft snapshot: %v", err)
	}

	// 2. Create a draft snapshot for Kano State
	kanoSnap, err := h.ReportService.GenerateSnapshot(ctx, superAdmin, service.GenerateReportRequest{
		ReportType:      "cleanups",
		Title:           "Kano Environmental Cleanup Report - " + period,
		ReportingPeriod: period,
		PeriodStart:     period + "-01",
		PeriodEnd:       period + "-28",
		ScopeType:       "state",
		StateID:         &kanoState.ID,
		TriggerSource:   "admin_manual",
	}, nil)
	if err != nil {
		t.Fatalf("failed generating kano draft snapshot: %v", err)
	}

	// 3. Lagos coordinator accesses Lagos draft -> 200 OK
	reqLagos, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/admin/reports/%s", lagosSnap.SnapshotID), nil)
	reqLagos.Header.Set("Authorization", "Bearer "+lagosToken)
	rrLagos := httptest.NewRecorder()
	h.Router.ServeHTTP(rrLagos, reqLagos)
	if rrLagos.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for authorized Lagos coordinator inspecting Lagos draft, got %d: %s", rrLagos.Code, rrLagos.Body.String())
	}

	// Non-super-admin audit logs must omit raw IP address for privacy
	var lagosDetail service.AdminSnapshotDetailDTO
	_ = json.Unmarshal(rrLagos.Body.Bytes(), &lagosDetail)
	for _, audit := range lagosDetail.AuditLogs {
		if audit.IPAddress != nil {
			t.Errorf("IP address must be redacted/omitted for non-super-admin inspection, got: %s", *audit.IPAddress)
		}
	}

	// 4. Lagos coordinator accesses Kano draft -> 403 Forbidden (SCOPE_UNAUTHORIZED)
	reqKano, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/admin/reports/%s", kanoSnap.SnapshotID), nil)
	reqKano.Header.Set("Authorization", "Bearer "+lagosToken)
	rrKano := httptest.NewRecorder()
	h.Router.ServeHTTP(rrKano, reqKano)
	if rrKano.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for Lagos coordinator inspecting Kano draft, got %d: %s", rrKano.Code, rrKano.Body.String())
	}
	if !strings.Contains(rrKano.Body.String(), "SCOPE_UNAUTHORIZED") {
		t.Errorf("expected SCOPE_UNAUTHORIZED error code, got: %s", rrKano.Body.String())
	}

	// 5. Unauthenticated request to draft -> 401 Unauthorized
	reqUnauth, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/admin/reports/%s", lagosSnap.SnapshotID), nil)
	rrUnauth := httptest.NewRecorder()
	h.Router.ServeHTTP(rrUnauth, reqUnauth)
	if rrUnauth.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unauthenticated inspection, got %d", rrUnauth.Code)
	}

	// 6. Public endpoint cannot view draft snapshot -> 404 Not Found
	reqPub, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/public/reports/publications/%s", lagosSnap.SnapshotID), nil)
	rrPub := httptest.NewRecorder()
	h.Router.ServeHTTP(rrPub, reqPub)
	if rrPub.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for public access to unpublished draft, got %d", rrPub.Code)
	}
}
