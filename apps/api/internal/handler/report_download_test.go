package handler_test

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"ratels-api/internal/db"
	"ratels-api/internal/security"
	"ratels-api/internal/service"
	"ratels-api/internal/storage"
)

// Helper to create, approve, and publish a test report with separated duties
func createAndPublishTestReport(t *testing.T, h *TestHarness, creator, approver *db.User, reportType, period string) (*service.AdminSnapshotDetailDTO, *service.PublicPublicationDTO) {
	t.Helper()
	ctx := context.Background()

	snap, err := h.ReportService.GenerateSnapshot(ctx, creator, service.GenerateReportRequest{
		ReportType:      reportType,
		Title:           fmt.Sprintf("Official Certified %s Report %s", strings.Title(reportType), period),
		ReportingPeriod: period,
		PeriodStart:     period + "-01",
		PeriodEnd:       period + "-28",
		ScopeType:       "national",
		TriggerSource:   "admin_manual",
	}, nil)
	if err != nil {
		t.Fatalf("failed generating snapshot: %v", err)
	}

	_, err = h.ReportService.SubmitForReview(ctx, creator, snap.SnapshotID, nil)
	if err != nil {
		t.Fatalf("failed submitting for review: %v", err)
	}

	_, err = h.ReportService.ApproveSnapshot(ctx, approver, snap.SnapshotID, "National Governance Signoff", nil)
	if err != nil {
		t.Fatalf("failed approving snapshot: %v", err)
	}

	pub, err := h.ReportService.PublishSnapshot(ctx, approver, snap.SnapshotID, nil)
	if err != nil {
		t.Fatalf("failed publishing snapshot: %v", err)
	}

	return snap, pub
}

// 1. Successful PDF download
func TestReportDownload_PDF_Success(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()

	uid := time.Now().UnixNano()
	creator, _ := createHandlerTestUser(t, h, "national_admin", fmt.Sprintf("na_dl_%d@test.com", uid))
	approver, _ := createHandlerTestUser(t, h, "super_admin", fmt.Sprintf("sa_dl_%d@test.com", uid))
	period := fmt.Sprintf("208%d-01", uid%9)
	defer func() {
		_, _ = h.Pool.Exec(context.Background(), "DELETE FROM public_report_snapshots WHERE reporting_period = $1", period)
	}()

	_, pub := createAndPublishTestReport(t, h, creator, approver, "cleanups", period)

	req, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/public/reports/publications/%s/pdf", pub.PublicReference), nil)
	req.Header.Set("CF-Connecting-IP", "102.89.44.1")
	rr := httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rr.Code, rr.Body.String())
	}

	// Verify headers
	if ct := rr.Header().Get("Content-Type"); ct != "application/pdf" {
		t.Errorf("expected Content-Type application/pdf, got %s", ct)
	}
	cd := rr.Header().Get("Content-Disposition")
	if !strings.HasPrefix(cd, "attachment; filename=") || !strings.HasSuffix(cd, `.pdf"`) {
		t.Errorf("expected Content-Disposition attachment with .pdf filename, got %s", cd)
	}
	if cc := rr.Header().Get("Cache-Control"); cc != "private, no-cache, no-store, must-revalidate" {
		t.Errorf("expected strict private cache control, got %s", cc)
	}
	if nosniff := rr.Header().Get("X-Content-Type-Options"); nosniff != "nosniff" {
		t.Errorf("expected X-Content-Type-Options: nosniff, got %s", nosniff)
	}

	// Verify PDF magic bytes (%PDF-)
	bodyBytes := rr.Body.Bytes()
	if len(bodyBytes) < 5 || string(bodyBytes[:5]) != "%PDF-" {
		t.Errorf("expected PDF payload to begin with %%PDF-, got %q", string(bodyBytes[:min(5, len(bodyBytes))]))
	}
}

// 2. Successful CSV download
func TestReportDownload_CSV_Success(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()

	uid := time.Now().UnixNano()
	creator, _ := createHandlerTestUser(t, h, "national_admin", fmt.Sprintf("na_dlcsv_%d@test.com", uid))
	approver, _ := createHandlerTestUser(t, h, "super_admin", fmt.Sprintf("sa_dlcsv_%d@test.com", uid))
	period := fmt.Sprintf("208%d-02", uid%9)
	defer func() {
		_, _ = h.Pool.Exec(context.Background(), "DELETE FROM public_report_snapshots WHERE reporting_period = $1", period)
	}()

	_, pub := createAndPublishTestReport(t, h, creator, approver, "cleanups", period)

	req, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/public/reports/publications/%s/csv", pub.PublicReference), nil)
	req.Header.Set("CF-Connecting-IP", "102.89.44.2")
	rr := httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rr.Code, rr.Body.String())
	}

	if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/csv") {
		t.Errorf("expected Content-Type text/csv, got %s", ct)
	}
	cd := rr.Header().Get("Content-Disposition")
	if !strings.HasPrefix(cd, "attachment; filename=") || !strings.HasSuffix(cd, `.csv"`) {
		t.Errorf("expected Content-Disposition attachment with .csv filename, got %s", cd)
	}

	// Parse CSV
	reader := csv.NewReader(rr.Body)
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("failed reading returned CSV: %v", err)
	}
	if len(records) < 2 {
		t.Fatalf("expected at least 2 rows in CSV, got %d", len(records))
	}
}

// 3. Draft report rejection (must return non-revealing 404)
func TestReportDownload_Draft_Rejection(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()

	uid := time.Now().UnixNano()
	admin, _ := createHandlerTestUser(t, h, "super_admin", fmt.Sprintf("sa_draft_%d@test.com", uid))
	period := fmt.Sprintf("208%d-03", uid%9)
	defer func() {
		_, _ = h.Pool.Exec(context.Background(), "DELETE FROM public_report_snapshots WHERE reporting_period = $1", period)
	}()

	snap, err := h.ReportService.GenerateSnapshot(context.Background(), admin, service.GenerateReportRequest{
		ReportType:      "finance",
		Title:           "Draft Finance Report",
		ReportingPeriod: period,
		PeriodStart:     period + "-01",
		PeriodEnd:       period + "-28",
		ScopeType:       "national",
		TriggerSource:   "admin_manual",
	}, nil)
	if err != nil {
		t.Fatalf("failed generating snapshot: %v", err)
	}

	req, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/public/reports/publications/%s/pdf", snap.SnapshotID), nil)
	rr := httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404 Not Found for draft report download, got %d: %s", rr.Code, rr.Body.String())
	}

	var env struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &env)
	if env.Error.Code != "NOT_FOUND" {
		t.Errorf("expected error code NOT_FOUND, got %v", env.Error.Code)
	}
	if !strings.Contains(env.Error.Message, "not available") {
		t.Errorf("expected non-revealing error message, got %v", env.Error.Message)
	}
}

// 4. Pending-review rejection (404 non-revealing)
func TestReportDownload_PendingReview_Rejection(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()

	uid := time.Now().UnixNano()
	admin, _ := createHandlerTestUser(t, h, "super_admin", fmt.Sprintf("sa_pr_%d@test.com", uid))
	period := fmt.Sprintf("208%d-04", uid%9)
	defer func() {
		_, _ = h.Pool.Exec(context.Background(), "DELETE FROM public_report_snapshots WHERE reporting_period = $1", period)
	}()

	snap, _ := h.ReportService.GenerateSnapshot(context.Background(), admin, service.GenerateReportRequest{
		ReportType:      "cleanups",
		Title:           "Pending Review Cleanups",
		ReportingPeriod: period,
		PeriodStart:     period + "-01",
		PeriodEnd:       period + "-28",
		ScopeType:       "national",
		TriggerSource:   "admin_manual",
	}, nil)
	_, _ = h.ReportService.SubmitForReview(context.Background(), admin, snap.SnapshotID, nil)

	req, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/public/reports/publications/%s/pdf", snap.SnapshotID), nil)
	rr := httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for pending_review snapshot, got %d", rr.Code)
	}
}

// 5. Unapproved report rejection (404 non-revealing)
func TestReportDownload_Unapproved_Rejection(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()

	uid := time.Now().UnixNano()
	creator, _ := createHandlerTestUser(t, h, "national_admin", fmt.Sprintf("na_unapp_%d@test.com", uid))
	reviewer, _ := createHandlerTestUser(t, h, "super_admin", fmt.Sprintf("sa_unapp_%d@test.com", uid))
	period := fmt.Sprintf("208%d-05", uid%9)
	defer func() {
		_, _ = h.Pool.Exec(context.Background(), "DELETE FROM public_report_snapshots WHERE reporting_period = $1", period)
	}()

	snap, _ := h.ReportService.GenerateSnapshot(context.Background(), creator, service.GenerateReportRequest{
		ReportType:      "participation",
		Title:           "Rejected Participation",
		ReportingPeriod: period,
		PeriodStart:     period + "-01",
		PeriodEnd:       period + "-28",
		ScopeType:       "national",
		TriggerSource:   "admin_manual",
	}, nil)
	_, _ = h.ReportService.SubmitForReview(context.Background(), creator, snap.SnapshotID, nil)
	_, _ = h.ReportService.RejectSnapshot(context.Background(), reviewer, snap.SnapshotID, "Data validation failed", nil)

	req, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/public/reports/publications/%s/pdf", snap.SnapshotID), nil)
	rr := httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for rejected snapshot, got %d", rr.Code)
	}
}

// 6. Withdrawn report rejection (404 non-revealing)
func TestReportDownload_Withdrawn_Rejection(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()

	uid := time.Now().UnixNano()
	creator, _ := createHandlerTestUser(t, h, "national_admin", fmt.Sprintf("na_withdrawn_%d@test.com", uid))
	approver, _ := createHandlerTestUser(t, h, "super_admin", fmt.Sprintf("sa_withdrawn_%d@test.com", uid))
	period := fmt.Sprintf("208%d-06", uid%9)
	defer func() {
		_, _ = h.Pool.Exec(context.Background(), "DELETE FROM public_report_snapshots WHERE reporting_period = $1", period)
	}()

	snap, pub := createAndPublishTestReport(t, h, creator, approver, "cleanups", period)

	// Now withdraw the publication using snapshotID
	_, err := h.ReportService.WithdrawPublication(context.Background(), approver, snap.SnapshotID, "Data correction notice issued", nil)
	if err != nil {
		t.Fatalf("failed withdrawing publication: %v", err)
	}

	// Public PDF download should now be rejected with 404
	req, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/public/reports/publications/%s/pdf", pub.PublicationID), nil)
	rr := httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for withdrawn report, got %d: %s", rr.Code, rr.Body.String())
	}
}

// 7. Revoked export rejection (404 non-revealing)
func TestReportDownload_RevokedExport_Rejection(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()

	uid := time.Now().UnixNano()
	creator, _ := createHandlerTestUser(t, h, "national_admin", fmt.Sprintf("na_revoked_%d@test.com", uid))
	approver, _ := createHandlerTestUser(t, h, "super_admin", fmt.Sprintf("sa_revoked_%d@test.com", uid))
	period := fmt.Sprintf("208%d-07", uid%9)
	defer func() {
		_, _ = h.Pool.Exec(context.Background(), "DELETE FROM public_report_snapshots WHERE reporting_period = $1", period)
	}()

	snap, pub := createAndPublishTestReport(t, h, creator, approver, "cleanups", period)

	// Pre-generate export, then mark it 'revoked' in the database
	_, err := h.ReportService.GenerateAndStoreExport(context.Background(), approver, snap.SnapshotID, "pdf", nil)
	if err != nil {
		t.Fatalf("failed generating export: %v", err)
	}

	_, err = h.Pool.Exec(context.Background(), `
		UPDATE public_report_exports
		SET export_status = 'revoked'
		WHERE snapshot_id = $1 AND export_format = 'pdf'
	`, snap.SnapshotID)
	if err != nil {
		t.Fatalf("failed marking export as revoked: %v", err)
	}

	req, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/public/reports/publications/%s/pdf", pub.PublicationID), nil)
	rr := httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for revoked export, got %d: %s", rr.Code, rr.Body.String())
	}
}

// 8. Unknown publication ID (404 non-revealing)
func TestReportDownload_UnknownID_Rejection(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()

	req, _ := http.NewRequest("GET", "/api/v1/public/reports/publications/00000000-0000-0000-0000-000000000000/pdf", nil)
	rr := httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown publication ID, got %d", rr.Code)
	}
}

// 9. Export-to-publication mismatch
func TestReportDownload_ExportMismatch_Rejection(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()

	req, _ := http.NewRequest("GET", "/api/v1/public/reports/publications/NON_EXISTENT_REF/csv", nil)
	rr := httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for non-existent report reference, got %d", rr.Code)
	}
}

// 10. R2 Storage failure handling
type failingExportStorage struct {
	storage.ReportExportStorage
}

func (s *failingExportStorage) ReadExport(ctx context.Context, key string) ([]byte, error) {
	return nil, errors.New("simulated R2 network timeout / storage read failure")
}

func TestReportDownload_StorageFailure(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()

	uid := time.Now().UnixNano()
	creator, _ := createHandlerTestUser(t, h, "national_admin", fmt.Sprintf("na_stgfail_%d@test.com", uid))
	approver, _ := createHandlerTestUser(t, h, "super_admin", fmt.Sprintf("sa_stgfail_%d@test.com", uid))
	period := fmt.Sprintf("208%d-10", uid%9)
	defer func() {
		_, _ = h.Pool.Exec(context.Background(), "DELETE FROM public_report_snapshots WHERE reporting_period = $1", period)
	}()

	snap, pub := createAndPublishTestReport(t, h, creator, approver, "cleanups", period)

	// Pre-generate export in mock storage
	_, err := h.ReportService.GenerateAndStoreExport(context.Background(), approver, snap.SnapshotID, "pdf", nil)
	if err != nil {
		t.Fatalf("failed generating export: %v", err)
	}

	// Substitute storage with a failing one
	h.ReportService.SetExportStorage(&failingExportStorage{ReportExportStorage: storage.NewDevSandboxReportExportStorage()})

	req, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/public/reports/publications/%s/pdf", pub.PublicationID), nil)
	rr := httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 for storage read failure, got %d: %s", rr.Code, rr.Body.String())
	}
}

// 11. Content-Disposition & Content-Type header checks
func TestReportDownload_HeadersIntegrity(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()

	uid := time.Now().UnixNano()
	creator, _ := createHandlerTestUser(t, h, "national_admin", fmt.Sprintf("na_hdrs_%d@test.com", uid))
	approver, _ := createHandlerTestUser(t, h, "super_admin", fmt.Sprintf("sa_hdrs_%d@test.com", uid))
	period := fmt.Sprintf("208%d-11", uid%9)
	defer func() {
		_, _ = h.Pool.Exec(context.Background(), "DELETE FROM public_report_snapshots WHERE reporting_period = $1", period)
	}()

	_, pub := createAndPublishTestReport(t, h, creator, approver, "cleanups", period)

	req, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/public/reports/publications/%s/csv", pub.PublicReference), nil)
	rr := httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	if !strings.Contains(rr.Header().Get("Content-Disposition"), "attachment; filename=") {
		t.Errorf("missing attachment Content-Disposition: %s", rr.Header().Get("Content-Disposition"))
	}
	if rr.Header().Get("Cache-Control") != "private, no-cache, no-store, must-revalidate" {
		t.Errorf("expected no-cache Cache-Control, got %s", rr.Header().Get("Cache-Control"))
	}
	if rr.Header().Get("Pragma") != "no-cache" {
		t.Errorf("expected Pragma: no-cache, got %s", rr.Header().Get("Pragma"))
	}
	if rr.Header().Get("Expires") != "0" {
		t.Errorf("expected Expires: 0, got %s", rr.Header().Get("Expires"))
	}
}

// 12. Rate-limit enforcement (HTTP 429)
func TestReportDownload_RateLimitEnforcement(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()

	uid := time.Now().UnixNano()
	creator, _ := createHandlerTestUser(t, h, "national_admin", fmt.Sprintf("na_rate_%d@test.com", uid))
	approver, _ := createHandlerTestUser(t, h, "super_admin", fmt.Sprintf("sa_rate_%d@test.com", uid))
	period := fmt.Sprintf("208%d-12", uid%9)
	defer func() {
		_, _ = h.Pool.Exec(context.Background(), "DELETE FROM public_report_snapshots WHERE reporting_period = $1", period)
	}()

	_, pub := createAndPublishTestReport(t, h, creator, approver, "cleanups", period)

	// Configure a tight rate limiter: 2 requests per minute
	testLimiter := security.NewDownloadRateLimiter(2, time.Minute)
	h.ReportHandler.SetRateLimiter(testLimiter)

	clientIP := "197.210.55.99"

	// Request 1: OK
	req1, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/public/reports/publications/%s/pdf", pub.PublicationID), nil)
	req1.Header.Set("CF-Connecting-IP", clientIP)
	rr1 := httptest.NewRecorder()
	h.Router.ServeHTTP(rr1, req1)
	if rr1.Code != http.StatusOK {
		t.Fatalf("expected 200 for 1st request, got %d", rr1.Code)
	}

	// Request 2: OK
	req2, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/public/reports/publications/%s/pdf", pub.PublicationID), nil)
	req2.Header.Set("CF-Connecting-IP", clientIP)
	rr2 := httptest.NewRecorder()
	h.Router.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusOK {
		t.Fatalf("expected 200 for 2nd request, got %d", rr2.Code)
	}

	// Request 3: Rate Limited (429)
	req3, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/public/reports/publications/%s/pdf", pub.PublicationID), nil)
	req3.Header.Set("CF-Connecting-IP", clientIP)
	rr3 := httptest.NewRecorder()
	h.Router.ServeHTTP(rr3, req3)
	if rr3.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 Too Many Requests for 3rd request, got %d: %s", rr3.Code, rr3.Body.String())
	}
	if retryAfter := rr3.Header().Get("Retry-After"); retryAfter == "" {
		t.Errorf("expected Retry-After header on 429 response")
	}

	// Different IP: Still OK
	reqOther, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/public/reports/publications/%s/pdf", pub.PublicationID), nil)
	reqOther.Header.Set("CF-Connecting-IP", "197.210.55.100")
	rrOther := httptest.NewRecorder()
	h.Router.ServeHTTP(rrOther, reqOther)
	if rrOther.Code != http.StatusOK {
		t.Fatalf("expected 200 for different IP, got %d", rrOther.Code)
	}
}

// 13. Concurrent download requests
func TestReportDownload_ConcurrentStreaming(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()

	uid := time.Now().UnixNano()
	creator, _ := createHandlerTestUser(t, h, "national_admin", fmt.Sprintf("na_conc_%d@test.com", uid))
	approver, _ := createHandlerTestUser(t, h, "super_admin", fmt.Sprintf("sa_conc_%d@test.com", uid))
	period := fmt.Sprintf("208%d-13", uid%9)
	defer func() {
		_, _ = h.Pool.Exec(context.Background(), "DELETE FROM public_report_snapshots WHERE reporting_period = $1", period)
	}()

	_, pub := createAndPublishTestReport(t, h, creator, approver, "cleanups", period)

	// High limit for concurrency test
	h.ReportHandler.SetRateLimiter(security.NewDownloadRateLimiter(100, time.Minute))

	concurrency := 8
	var wg sync.WaitGroup
	errCh := make(chan error, concurrency)

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			req, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/public/reports/publications/%s/pdf", pub.PublicationID), nil)
			req.Header.Set("CF-Connecting-IP", fmt.Sprintf("10.10.10.%d", workerID+1))
			rr := httptest.NewRecorder()
			h.Router.ServeHTTP(rr, req)

			if rr.Code != http.StatusOK {
				errCh <- fmt.Errorf("worker %d got status %d", workerID, rr.Code)
				return
			}
			if !strings.HasPrefix(string(rr.Body.Bytes()), "%PDF-") {
				errCh <- fmt.Errorf("worker %d got invalid PDF bytes", workerID)
				return
			}
		}(i)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Errorf("concurrent download error: %v", err)
	}
}

// 14. Public bucket exposure prevention (no raw R2/S3 URLs exposed)
func TestReportDownload_NoPublicBucketExposure(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()

	uid := time.Now().UnixNano()
	creator, _ := createHandlerTestUser(t, h, "national_admin", fmt.Sprintf("na_expo_%d@test.com", uid))
	approver, _ := createHandlerTestUser(t, h, "super_admin", fmt.Sprintf("sa_expo_%d@test.com", uid))
	period := fmt.Sprintf("208%d-14", uid%9)
	defer func() {
		_, _ = h.Pool.Exec(context.Background(), "DELETE FROM public_report_snapshots WHERE reporting_period = $1", period)
	}()

	_, pub := createAndPublishTestReport(t, h, creator, approver, "cleanups", period)

	req, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/public/reports/publications/%s/pdf", pub.PublicationID), nil)
	rr := httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)

	// Response must NOT contain redirect, r2.dev domain, or s3 endpoint
	if rr.Code == http.StatusFound || rr.Code == http.StatusMovedPermanently || rr.Code == http.StatusTemporaryRedirect {
		t.Fatalf("public download must stream directly and not redirect to external URLs, got %d", rr.Code)
	}

	for k, v := range rr.Header() {
		for _, val := range v {
			if strings.Contains(val, "r2.dev") || strings.Contains(val, "cloudflarestorage.com") {
				t.Errorf("header %s exposed private R2 storage address: %s", k, val)
			}
		}
	}
}

// 15. Audit trail records download and throttle events
func TestReportDownload_AuditObservability(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()

	uid := time.Now().UnixNano()
	creator, _ := createHandlerTestUser(t, h, "national_admin", fmt.Sprintf("na_aud_%d@test.com", uid))
	approver, _ := createHandlerTestUser(t, h, "super_admin", fmt.Sprintf("sa_aud_%d@test.com", uid))
	period := fmt.Sprintf("208%d-15", uid%9)
	defer func() {
		_, _ = h.Pool.Exec(context.Background(), "DELETE FROM public_report_snapshots WHERE reporting_period = $1", period)
	}()

	_, pub := createAndPublishTestReport(t, h, creator, approver, "cleanups", period)

	clientIP := "197.210.99.1"
	req, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/public/reports/publications/%s/pdf", pub.PublicationID), nil)
	req.Header.Set("CF-Connecting-IP", clientIP)
	rr := httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	// Verify that an audit row was recorded in public_report_audit
	var count int
	err := h.Pool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM public_report_audit
		WHERE action = 'report_downloaded' AND ip_address = $1
	`, clientIP).Scan(&count)
	if err != nil {
		t.Fatalf("failed querying audit table: %v", err)
	}
	if count == 0 {
		t.Errorf("expected audit record with action 'report_downloaded' for IP %s", clientIP)
	}
}

// 16. No regression in admin-only export access
func TestReportDownload_AdminAccess_NoRegression(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()

	uid := time.Now().UnixNano()
	admin, _ := createHandlerTestUser(t, h, "super_admin", fmt.Sprintf("sa_adminonly_%d@test.com", uid))
	member, _ := createHandlerTestUser(t, h, "member", fmt.Sprintf("mem_adminonly_%d@test.com", uid))
	period := fmt.Sprintf("208%d-16", uid%9)
	defer func() {
		_, _ = h.Pool.Exec(context.Background(), "DELETE FROM public_report_snapshots WHERE reporting_period = $1", period)
	}()

	// Create a draft snapshot
	snap, err := h.ReportService.GenerateSnapshot(context.Background(), admin, service.GenerateReportRequest{
		ReportType:      "finance",
		Title:           "Admin Review Draft",
		ReportingPeriod: period,
		PeriodStart:     period + "-01",
		PeriodEnd:       period + "-28",
		ScopeType:       "national",
		TriggerSource:   "admin_manual",
	}, nil)
	if err != nil {
		t.Fatalf("failed generating snapshot: %v", err)
	}

	// Public (actor == nil) must be denied
	_, _, _, err = h.ReportService.GetExportPrivateStream(context.Background(), nil, snap.SnapshotID, "pdf")
	if !errors.Is(err, service.ErrUnauthorizedExportAccess) {
		t.Errorf("expected ErrUnauthorizedExportAccess for public access to draft, got %v", err)
	}

	// Member user must be denied
	_, _, _, err = h.ReportService.GetExportPrivateStream(context.Background(), member, snap.SnapshotID, "pdf")
	if !errors.Is(err, service.ErrUnauthorizedExportAccess) {
		t.Errorf("expected ErrUnauthorizedExportAccess for member access to draft, got %v", err)
	}

	// Super Admin must succeed for internal audit review
	pdfBytes, fileName, mime, err := h.ReportService.GetExportPrivateStream(context.Background(), admin, snap.SnapshotID, "pdf")
	if err != nil {
		t.Fatalf("expected super_admin to access draft export, got error: %v", err)
	}
	if len(pdfBytes) == 0 || fileName == "" || mime != "application/pdf" {
		t.Errorf("invalid draft export data returned to admin")
	}
}
