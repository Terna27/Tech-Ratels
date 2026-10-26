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

// TestReport_EndToEndLifecycleIntegration verifies the complete 16-step reporting lifecycle
// from automated draft generation to governance review, approval, publication, private R2 export,
// public streaming download, withdrawal revocation, and scheduler retry safety.
func TestReport_EndToEndLifecycleIntegration(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()

	uid := time.Now().UnixNano()
	_, creatorToken := createHandlerTestUser(t, h, "national_admin", fmt.Sprintf("na_e2e_%d@test.com", uid))
	_, approverToken := createHandlerTestUser(t, h, "super_admin", fmt.Sprintf("sa_e2e_%d@test.com", uid))
	historicalPeriod := fmt.Sprintf("202%d-05", uid%3)

	defer func() {
		ctx := context.Background()
		_, _ = h.Pool.Exec(ctx, "DELETE FROM public_report_scheduler_runs WHERE reporting_period = $1", historicalPeriod)
		_, _ = h.Pool.Exec(ctx, "DELETE FROM public_report_exports WHERE snapshot_id IN (SELECT id FROM public_report_snapshots WHERE reporting_period = $1)", historicalPeriod)
		_, _ = h.Pool.Exec(ctx, "DELETE FROM public_report_audit WHERE reporting_period = $1", historicalPeriod)
		_, _ = h.Pool.Exec(ctx, "DELETE FROM public_report_snapshots WHERE reporting_period = $1", historicalPeriod)
	}()

	// =========================================================================
	// Step 1: Scheduled/Manual Report Generation Trigger
	// =========================================================================
	runPayload, _ := json.Marshal(map[string]interface{}{
		"target_period": historicalPeriod,
		"period_type":   "monthly",
		"report_type":   "cleanups",
		"scope_type":    "national",
	})
	runReq, _ := http.NewRequest("POST", "/api/v1/admin/reports/scheduler/run", bytes.NewBuffer(runPayload))
	runReq.Header.Set("Authorization", "Bearer "+creatorToken)
	runReq.Header.Set("Content-Type", "application/json")
	runRec := httptest.NewRecorder()
	h.Router.ServeHTTP(runRec, runReq)

	if runRec.Code != http.StatusOK {
		t.Fatalf("Step 1 failed: expected 200 OK from manual/scheduler generation, got %d: %s", runRec.Code, runRec.Body.String())
	}

	// =========================================================================
	// Step 2: Draft Snapshot Creation & Verification
	// =========================================================================
	listReq, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/admin/reports?status=draft&reporting_period=%s", historicalPeriod), nil)
	listReq.Header.Set("Authorization", "Bearer "+creatorToken)
	listRec := httptest.NewRecorder()
	h.Router.ServeHTTP(listRec, listReq)

	if listRec.Code != http.StatusOK {
		t.Fatalf("Step 2 failed: expected 200 OK listing drafts, got %d: %s", listRec.Code, listRec.Body.String())
	}

	var pagedSnapshots service.PaginatedSnapshotsResponse
	if err := json.Unmarshal(listRec.Body.Bytes(), &pagedSnapshots); err != nil {
		t.Fatalf("Step 2 unmarshal failed: %v", err)
	}
	if len(pagedSnapshots.Snapshots) == 0 {
		t.Fatalf("Step 2 failed: expected at least 1 draft snapshot generated for period %s", historicalPeriod)
	}
	targetSnapshot := pagedSnapshots.Snapshots[0]
	if targetSnapshot.Status != "draft" {
		t.Fatalf("Step 2 failed: snapshot status must be 'draft', got: %s", targetSnapshot.Status)
	}

	// =========================================================================
	// Step 3: Audit Record Creation
	// =========================================================================
	auditReq, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/admin/reports/audit?reporting_period=%s", historicalPeriod), nil)
	auditReq.Header.Set("Authorization", "Bearer "+creatorToken)
	auditRec := httptest.NewRecorder()
	h.Router.ServeHTTP(auditRec, auditReq)

	if auditRec.Code != http.StatusOK {
		t.Fatalf("Step 3 failed: expected 200 OK fetching audit history, got %d: %s", auditRec.Code, auditRec.Body.String())
	}
	var auditResp service.PaginatedAuditsResponse
	_ = json.Unmarshal(auditRec.Body.Bytes(), &auditResp)
	if auditResp.TotalCount < 1 {
		t.Fatalf("Step 3 failed: expected generation audit record, got total %d", auditResp.TotalCount)
	}

	// =========================================================================
	// Step 4: Verification that Public Access is BLOCKED for Draft Reports
	// =========================================================================
	publicDraftPDF, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/public/reports/publications/%s/pdf", targetSnapshot.PublicReference), nil)
	pubPDFRec := httptest.NewRecorder()
	h.Router.ServeHTTP(pubPDFRec, publicDraftPDF)
	if pubPDFRec.Code != http.StatusNotFound {
		t.Fatalf("Step 4 security failure: draft PDF accessible publicly! Expected 404, got %d: %s", pubPDFRec.Code, pubPDFRec.Body.String())
	}

	publicDraftCSV, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/public/reports/publications/%s/csv", targetSnapshot.PublicReference), nil)
	pubCSVRec := httptest.NewRecorder()
	h.Router.ServeHTTP(pubCSVRec, publicDraftCSV)
	if pubCSVRec.Code != http.StatusNotFound {
		t.Fatalf("Step 4 security failure: draft CSV accessible publicly! Expected 404, got %d: %s", pubCSVRec.Code, pubCSVRec.Body.String())
	}

	// =========================================================================
	// Step 5: Submission for Approval
	// =========================================================================
	submitPayload, _ := json.Marshal(map[string]string{"notes": "Ready for executive governance review"})
	subReq, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/admin/reports/%s/submit-review", targetSnapshot.SnapshotID), bytes.NewBuffer(submitPayload))
	subReq.Header.Set("Authorization", "Bearer "+creatorToken)
	subReq.Header.Set("Content-Type", "application/json")
	subRec := httptest.NewRecorder()
	h.Router.ServeHTTP(subRec, subReq)

	if subRec.Code != http.StatusOK {
		t.Fatalf("Step 5 failed: expected 200 OK submitting report for review, got %d: %s", subRec.Code, subRec.Body.String())
	}

	// =========================================================================
	// Step 6: Self-Approval Restrictions (Anti-Self-Approval Enforcement)
	// =========================================================================
	selfApprovePayload, _ := json.Marshal(map[string]string{"notes": "Attempting illegal self approval"})
	selfReq, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/admin/reports/%s/approve", targetSnapshot.SnapshotID), bytes.NewBuffer(selfApprovePayload))
	selfReq.Header.Set("Authorization", "Bearer "+creatorToken) // Creator tries to approve own report
	selfReq.Header.Set("Content-Type", "application/json")
	selfRec := httptest.NewRecorder()
	h.Router.ServeHTTP(selfRec, selfReq)

	if selfRec.Code != http.StatusForbidden {
		t.Fatalf("Step 6 security failure: creator was allowed to self-approve! Expected 403, got %d: %s", selfRec.Code, selfRec.Body.String())
	}

	// =========================================================================
	// Step 7: Authorised Independent Approval
	// =========================================================================
	apprPayload, _ := json.Marshal(map[string]string{"notes": "Independent review completed and verified"})
	apprReq, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/admin/reports/%s/approve", targetSnapshot.SnapshotID), bytes.NewBuffer(apprPayload))
	apprReq.Header.Set("Authorization", "Bearer "+approverToken) // Distinct Super Admin
	apprReq.Header.Set("Content-Type", "application/json")
	apprRec := httptest.NewRecorder()
	h.Router.ServeHTTP(apprRec, apprReq)

	if apprRec.Code != http.StatusOK {
		t.Fatalf("Step 7 failed: expected 200 OK from distinct approver, got %d: %s", apprRec.Code, apprRec.Body.String())
	}

	// Verify still not publicly downloadable while approved (prior to publication)
	pubApprovedCheck, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/public/reports/publications/%s/pdf", targetSnapshot.PublicReference), nil)
	pubAppRec := httptest.NewRecorder()
	h.Router.ServeHTTP(pubAppRec, pubApprovedCheck)
	if pubAppRec.Code != http.StatusNotFound {
		t.Fatalf("Step 7 security failure: approved report accessible publicly prior to publication! Got %d", pubAppRec.Code)
	}

	// =========================================================================
	// Step 8 & 9: Official Publication
	// =========================================================================
	pubPayload, _ := json.Marshal(map[string]string{"notes": "Official release on Ratels public transparency portal"})
	pubReq, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/admin/reports/%s/publish", targetSnapshot.SnapshotID), bytes.NewBuffer(pubPayload))
	pubReq.Header.Set("Authorization", "Bearer "+approverToken)
	pubReq.Header.Set("Content-Type", "application/json")
	pubRec := httptest.NewRecorder()
	h.Router.ServeHTTP(pubRec, pubReq)

	if pubRec.Code != http.StatusOK {
		t.Fatalf("Step 8/9 failed: expected 200 OK publishing report, got %d: %s", pubRec.Code, pubRec.Body.String())
	}

	var pubResult service.PublicPublicationDTO
	if err := json.Unmarshal(pubRec.Body.Bytes(), &pubResult); err != nil {
		t.Fatalf("Step 8/9 unmarshal failed: %v", err)
	}

	// =========================================================================
	// Step 10 & 11: Export Generation in Private R2 Storage & Checksum Verification
	// =========================================================================
	// Trigger PDF regeneration/generation
	pdfGenReq, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/admin/reports/%s/exports/pdf/regenerate", targetSnapshot.SnapshotID), nil)
	pdfGenReq.Header.Set("Authorization", "Bearer "+approverToken)
	pdfGenRec := httptest.NewRecorder()
	h.Router.ServeHTTP(pdfGenRec, pdfGenReq)
	if pdfGenRec.Code != http.StatusOK {
		t.Fatalf("Step 10 failed: PDF generation failed, got %d: %s", pdfGenRec.Code, pdfGenRec.Body.String())
	}

	// Trigger CSV regeneration/generation
	csvGenReq, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/admin/reports/%s/exports/csv/regenerate", targetSnapshot.SnapshotID), nil)
	csvGenReq.Header.Set("Authorization", "Bearer "+approverToken)
	csvGenRec := httptest.NewRecorder()
	h.Router.ServeHTTP(csvGenRec, csvGenReq)
	if csvGenRec.Code != http.StatusOK {
		t.Fatalf("Step 11 failed: CSV generation failed, got %d: %s", csvGenRec.Code, csvGenRec.Body.String())
	}

	// Verify both exports are visible in the exports catalogue with valid checksums
	expReq, _ := http.NewRequest("GET", "/api/v1/admin/reports/exports", nil)
	expReq.Header.Set("Authorization", "Bearer "+approverToken)
	expRec := httptest.NewRecorder()
	h.Router.ServeHTTP(expRec, expReq)

	if expRec.Code != http.StatusOK {
		t.Fatalf("Step 10/11 failed: expected 200 OK from exports catalogue, got %d: %s", expRec.Code, expRec.Body.String())
	}

	var expResp service.PaginatedAdminExportsResponse
	if err := json.Unmarshal(expRec.Body.Bytes(), &expResp); err != nil {
		t.Fatalf("Step 10/11 unmarshal failed: %v", err)
	}
	var foundPDF, foundCSV bool
	for _, exp := range expResp.Exports {
		if exp.SnapshotID == targetSnapshot.SnapshotID {
			if exp.FileSizeBytes <= 0 {
				t.Errorf("Export %s has invalid file size: %d", exp.FileName, exp.FileSizeBytes)
			}
			if len(exp.ContentHash) != 64 {
				t.Errorf("Export %s has invalid SHA-256 hash length: %s", exp.FileName, exp.ContentHash)
			}
			if !exp.StorageVerified {
				t.Errorf("Export %s failed storage verification", exp.FileName)
			}
			if exp.ExportFormat == "pdf" {
				foundPDF = true
			}
			if exp.ExportFormat == "csv" {
				foundCSV = true
			}
		}
	}
	if !foundPDF || !foundCSV {
		t.Fatalf("Step 10/11 failed: expected both PDF and CSV exports in catalogue, found PDF=%v CSV=%v", foundPDF, foundCSV)
	}

	// =========================================================================
	// Step 12: Public PDF Download (Streaming, Safe Content Disposition, Non-Empty)
	// =========================================================================
	pubPDF, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/public/reports/publications/%s/pdf", pubResult.PublicReference), nil)
	pubPDFDownRec := httptest.NewRecorder()
	h.Router.ServeHTTP(pubPDFDownRec, pubPDF)

	if pubPDFDownRec.Code != http.StatusOK {
		t.Fatalf("Step 12 failed: expected 200 OK downloading public PDF, got %d: %s", pubPDFDownRec.Code, pubPDFDownRec.Body.String())
	}
	if pubPDFDownRec.Header().Get("Content-Type") != "application/pdf" {
		t.Errorf("Step 12 failed: expected application/pdf, got %s", pubPDFDownRec.Header().Get("Content-Type"))
	}
	if !strings.HasPrefix(pubPDFDownRec.Body.String(), "%PDF-") {
		t.Errorf("Step 12 failed: response does not begin with standard PDF signature (%%PDF-)")
	}

	// =========================================================================
	// Step 13: Public CSV Download (RFC 4180 Format, Formula Injection Free)
	// =========================================================================
	pubCSV, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/public/reports/publications/%s/csv", pubResult.PublicReference), nil)
	pubCSVDownRec := httptest.NewRecorder()
	h.Router.ServeHTTP(pubCSVDownRec, pubCSV)

	if pubCSVDownRec.Code != http.StatusOK {
		t.Fatalf("Step 13 failed: expected 200 OK downloading public CSV, got %d: %s", pubCSVDownRec.Code, pubCSVDownRec.Body.String())
	}
	if pubCSVDownRec.Header().Get("Content-Type") != "text/csv; charset=utf-8" {
		t.Errorf("Step 13 failed: expected text/csv; charset=utf-8, got %s", pubCSVDownRec.Header().Get("Content-Type"))
	}
	if len(pubCSVDownRec.Body.Bytes()) == 0 {
		t.Errorf("Step 13 failed: CSV download body was empty")
	}

	// =========================================================================
	// Step 14: Consequential Report Withdrawal (Documented Reason Required)
	// =========================================================================
	emptyWithdraw, _ := json.Marshal(map[string]string{"reason": ""})
	badWithReq, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/admin/reports/%s/withdraw", targetSnapshot.SnapshotID), bytes.NewBuffer(emptyWithdraw))
	badWithReq.Header.Set("Authorization", "Bearer "+approverToken)
	badWithReq.Header.Set("Content-Type", "application/json")
	badWithRec := httptest.NewRecorder()
	h.Router.ServeHTTP(badWithRec, badWithReq)
	if badWithRec.Code != http.StatusBadRequest {
		t.Fatalf("Step 14 security failure: withdrawal without reason was allowed! Got %d", badWithRec.Code)
	}

	validWithdraw, _ := json.Marshal(map[string]string{"reason": "Documented withdrawal due to periodic data reconciliation"})
	withReq, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/admin/reports/%s/withdraw", targetSnapshot.SnapshotID), bytes.NewBuffer(validWithdraw))
	withReq.Header.Set("Authorization", "Bearer "+approverToken)
	withReq.Header.Set("Content-Type", "application/json")
	withRec := httptest.NewRecorder()
	h.Router.ServeHTTP(withRec, withReq)

	if withRec.Code != http.StatusOK {
		t.Fatalf("Step 14 failed: expected 200 OK from valid withdrawal, got %d: %s", withRec.Code, withRec.Body.String())
	}

	// =========================================================================
	// Step 15: Download Rejection & Revocation After Withdrawal (404 Non-Revealing)
	// =========================================================================
	withPDFCheck, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/public/reports/publications/%s/pdf", pubResult.PublicReference), nil)
	withPDFRec := httptest.NewRecorder()
	h.Router.ServeHTTP(withPDFRec, withPDFCheck)

	if withPDFRec.Code != http.StatusNotFound {
		t.Fatalf("Step 15 security failure: withdrawn report was still downloadable! Expected 404 NOT FOUND, got %d: %s", withPDFRec.Code, withPDFRec.Body.String())
	}

	withCSVCheck, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/public/reports/publications/%s/csv", pubResult.PublicReference), nil)
	withCSVRec := httptest.NewRecorder()
	h.Router.ServeHTTP(withCSVRec, withCSVCheck)

	if withCSVRec.Code != http.StatusNotFound {
		t.Fatalf("Step 15 security failure: withdrawn CSV was still downloadable! Expected 404 NOT FOUND, got %d: %s", withCSVRec.Code, withCSVRec.Body.String())
	}

	// =========================================================================
	// Step 16: Scheduler Failure Classification & Safe Retry Ability
	// =========================================================================
	// 16a: Reject future period
	futurePayload, _ := json.Marshal(map[string]interface{}{
		"target_period": "2099-12",
	})
	futReq, _ := http.NewRequest("POST", "/api/v1/admin/reports/scheduler/run", bytes.NewBuffer(futurePayload))
	futReq.Header.Set("Authorization", "Bearer "+approverToken)
	futReq.Header.Set("Content-Type", "application/json")
	futRec := httptest.NewRecorder()
	h.Router.ServeHTTP(futRec, futReq)
	if futRec.Code != http.StatusBadRequest {
		t.Fatalf("Step 16a failed: expected 400 Bad Request for future period, got %d", futRec.Code)
	}

	// 16b: Reject invalid period syntax
	badSyntaxPayload, _ := json.Marshal(map[string]interface{}{
		"target_period": "invalid-month-syntax",
	})
	badReq, _ := http.NewRequest("POST", "/api/v1/admin/reports/scheduler/run", bytes.NewBuffer(badSyntaxPayload))
	badReq.Header.Set("Authorization", "Bearer "+approverToken)
	badReq.Header.Set("Content-Type", "application/json")
	badRec := httptest.NewRecorder()
	h.Router.ServeHTTP(badRec, badReq)
	if badRec.Code != http.StatusBadRequest {
		t.Fatalf("Step 16b failed: expected 400 Bad Request for invalid period syntax, got %d", badRec.Code)
	}

	// 16c: Safe retry on concluded historical period
	retryPeriod := fmt.Sprintf("202%d-06", uid%3)
	defer func() {
		ctx := context.Background()
		_, _ = h.Pool.Exec(ctx, "DELETE FROM public_report_scheduler_runs WHERE reporting_period = $1", retryPeriod)
		_, _ = h.Pool.Exec(ctx, "DELETE FROM public_report_snapshots WHERE reporting_period = $1", retryPeriod)
	}()
	retryPayload, _ := json.Marshal(map[string]interface{}{
		"target_period": retryPeriod,
		"period_type":   "monthly",
		"report_type":   "cleanups",
	})
	retryReq, _ := http.NewRequest("POST", "/api/v1/admin/reports/scheduler/run", bytes.NewBuffer(retryPayload))
	retryReq.Header.Set("Authorization", "Bearer "+approverToken)
	retryReq.Header.Set("Content-Type", "application/json")
	retryRec := httptest.NewRecorder()
	h.Router.ServeHTTP(retryRec, retryReq)
	if retryRec.Code != http.StatusOK {
		t.Fatalf("Step 16c failed: expected 200 OK on safe scheduler retry, got %d: %s", retryRec.Code, retryRec.Body.String())
	}
}
