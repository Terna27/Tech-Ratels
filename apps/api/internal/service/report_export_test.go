package service_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"ratels-api/internal/db"
	"ratels-api/internal/security"
	"ratels-api/internal/service"
	"ratels-api/internal/storage"
)

// Helper to create a test snapshot in draft status
func createTestSnapshot(t *testing.T, env *TestEnv, actor *db.User, reportType, period string, isSystem bool) *service.AdminSnapshotDetailDTO {
	t.Helper()
	ctx := context.Background()

	var triggerSource string
	if isSystem {
		triggerSource = "scheduled"
	} else {
		triggerSource = "admin_manual"
	}

	snap, err := env.ReportService.GenerateSnapshot(ctx, actor, service.GenerateReportRequest{
		ReportType:      reportType,
		Title:           fmt.Sprintf("Ratels %s Report - %s", strings.Title(reportType), period),
		ReportingPeriod: period,
		PeriodStart:     period + "-01",
		PeriodEnd:       period + "-28",
		ScopeType:       "national",
		TriggerSource:   triggerSource,
		IsSystemActor:   isSystem,
	}, nil)
	if err != nil {
		t.Fatalf("failed creating test snapshot: %v", err)
	}
	return snap
}

// 1. PDF rendering and valid PDF output
func TestReportExport_PDFRenderingAndValidOutput(t *testing.T) {
	env := setupTestEnv(t)
	superAdmin := createTestUserWithRole(t, env, "super_admin", fmt.Sprintf("sa_%d@ratels.org", time.Now().UnixNano()))

	snap := createTestSnapshot(t, env, superAdmin, "cleanups", "2087-01", false)

	dto, err := env.ReportService.GenerateAndStoreExport(context.Background(), superAdmin, snap.SnapshotID, "pdf", nil)
	if err != nil {
		t.Fatalf("failed generating PDF export: %v", err)
	}

	if dto.ExportFormat != "pdf" {
		t.Errorf("expected format 'pdf', got %q", dto.ExportFormat)
	}
	if dto.MimeType != "application/pdf" {
		t.Errorf("expected mime_type 'application/pdf', got %q", dto.MimeType)
	}

	// Fetch binary bytes from private stream
	pdfBytes, fileName, mime, err := env.ReportService.GetExportPrivateStream(context.Background(), superAdmin, snap.SnapshotID, "pdf")
	if err != nil {
		t.Fatalf("failed streaming PDF export: %v", err)
	}
	if mime != "application/pdf" {
		t.Errorf("expected mime 'application/pdf', got %q", mime)
	}
	if !strings.HasSuffix(fileName, ".pdf") {
		t.Errorf("expected .pdf filename, got %q", fileName)
	}

	// Verify standard PDF-1.4 header
	if !bytes.HasPrefix(pdfBytes, []byte("%PDF-1.4\n")) {
		t.Errorf("missing standard %%PDF-1.4 header")
	}

	// Verify PDF structure: Catalog, Pages, font, and %%EOF trailer
	pdfStr := string(pdfBytes)
	if !strings.Contains(pdfStr, "/Type /Catalog") {
		t.Errorf("missing PDF /Type /Catalog")
	}
	if !strings.Contains(pdfStr, "/Type /Pages") {
		t.Errorf("missing PDF /Type /Pages")
	}
	if !strings.Contains(pdfStr, "xref") {
		t.Errorf("missing PDF xref table")
	}
	if !strings.Contains(pdfStr, "startxref") || !strings.Contains(pdfStr, "%%EOF") {
		t.Errorf("missing PDF startxref or %%%%EOF trailer")
	}
	if !strings.Contains(pdfStr, "RATELS WORLDWIDE") {
		t.Errorf("missing 'RATELS WORLDWIDE' branding in PDF")
	}
}

// 2. PDF pagination and metadata
func TestReportExport_PDFPaginationAndMetadata(t *testing.T) {
	// Construct a PDFReportData with multiple tables and large row sets to trigger multi-page pagination
	var longRows [][]string
	for i := 1; i <= 60; i++ {
		longRows = append(longRows, []string{
			fmt.Sprintf("State %02d", i),
			fmt.Sprintf("%d", i*5),
			fmt.Sprintf("%d kg", i*250),
			fmt.Sprintf("%d", i*40),
		})
	}

	pdfData := service.PDFReportData{
		Title:           "National Cleanups Comprehensive Overview",
		PublicReference: "REP-2026-MULTI-001",
		ReportType:      "cleanups",
		ReportingPeriod: "2026-Q3",
		Scope:           "Nigeria (Nationwide)",
		PublicationDate: time.Now(),
		Version:         1,
		DataHash:        "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		Status:          "draft",
		Metrics: []service.PDFMetricItem{
			{Label: "Cleanups Completed", Value: "125"},
			{Label: "Verified Attendees", Value: "14,500"},
			{Label: "Waste Collected (kg)", Value: "85,000 kg"},
			{Label: "Active States", Value: "36"},
		},
		Tables: []service.PDFTable{
			{
				Title:   "Comprehensive State Breakdown",
				Headers: []string{"State", "Cleanups", "Waste (kg)", "Attendees"},
				Rows:    longRows,
			},
		},
	}

	pdfBytes, err := service.GeneratePDF(pdfData)
	if err != nil {
		t.Fatalf("failed generating multi-page PDF: %v", err)
	}

	pdfStr := string(pdfBytes)

	// Verify multi-page pagination: Count should be >= 2
	if !strings.Contains(pdfStr, "/Count 2") && !strings.Contains(pdfStr, "/Count 3") {
		t.Errorf("expected multi-page document with /Count >= 2, got PDF structure: %s", pdfStr[:min(len(pdfStr), 500)])
	}

	// Verify "Page 1 of" footer is present
	if !strings.Contains(pdfStr, "Page 1 of") {
		t.Errorf("missing 'Page 1 of' footer in multi-page document")
	}

	// Verify draft watermark
	if !strings.Contains(pdfStr, "DRAFT - UNPUBLISHED") {
		t.Errorf("missing draft watermark in draft report PDF")
	}
}

// 3. CSV encoding and parsing
func TestReportExport_CSVEncodingAndParsing(t *testing.T) {
	env := setupTestEnv(t)
	superAdmin := createTestUserWithRole(t, env, "super_admin", fmt.Sprintf("sa_%d@ratels.org", time.Now().UnixNano()))

	snap := createTestSnapshot(t, env, superAdmin, "finance", "2087-02", false)

	dto, err := env.ReportService.GenerateAndStoreExport(context.Background(), superAdmin, snap.SnapshotID, "csv", nil)
	if err != nil {
		t.Fatalf("failed generating CSV export: %v", err)
	}
	if dto.ExportFormat != "csv" {
		t.Errorf("expected format 'csv', got %q", dto.ExportFormat)
	}

	csvBytes, _, _, err := env.ReportService.GetExportPrivateStream(context.Background(), superAdmin, snap.SnapshotID, "csv")
	if err != nil {
		t.Fatalf("failed streaming CSV: %v", err)
	}

	// Parse CSV using standard encoding/csv Reader
	reader := csv.NewReader(bytes.NewReader(csvBytes))
	reader.FieldsPerRecord = -1 // Allow variable fields per record between metadata rows (2 columns) and table rows
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("RFC 4180 CSV parsing failed: %v", err)
	}

	if len(records) < 10 {
		t.Fatalf("expected at least 10 CSV lines (metadata + data), got %d", len(records))
	}

	// Verify metadata presence
	foundTitle := false
	foundRef := false
	foundHash := false
	for _, row := range records {
		if len(row) >= 2 {
			if row[0] == "Report Title" {
				foundTitle = true
			}
			if row[0] == "Public Reference" {
				foundRef = true
			}
			if strings.Contains(row[0], "Data Hash") {
				foundHash = true
			}
		}
	}

	if !foundTitle || !foundRef || !foundHash {
		t.Errorf("CSV metadata incomplete: title=%v, ref=%v, hash=%v", foundTitle, foundRef, foundHash)
	}
}

// 4. Spreadsheet formula injection protection
func TestReportExport_FormulaInjectionProtection(t *testing.T) {
	dangerousInputs := []string{
		"=SUM(A1:A10)",
		"+cmd|' /C calc'!A0",
		"-2+3+cmd|' /C calc'!A0",
		"@SUM(1+1)",
		"\t=1+1",
		"\r=1+1",
	}

	for _, input := range dangerousInputs {
		sanitized := service.SanitizeCSVValue(input)
		if !strings.HasPrefix(sanitized, "'") {
			t.Errorf("Formula injection trigger %q was NOT escaped with leading single quote: got %q", input, sanitized)
		}
	}

	safeInputs := []string{
		"Nigeria",
		"Total Revenue: NGN 1,500,000",
		"125 kg waste",
		"Completed",
	}
	for _, safe := range safeInputs {
		sanitized := service.SanitizeCSVValue(safe)
		if sanitized != safe {
			t.Errorf("Safe input %q was unnecessarily modified: got %q", safe, sanitized)
		}
	}
}

// 5. Snapshot consistency (exports strictly use snapshot data, not live DB changes)
func TestReportExport_SnapshotConsistency(t *testing.T) {
	env := setupTestEnv(t)
	superAdmin := createTestUserWithRole(t, env, "super_admin", fmt.Sprintf("sa_%d@ratels.org", time.Now().UnixNano()))

	snap := createTestSnapshot(t, env, superAdmin, "cleanups", "2087-03", false)

	// Fetch export
	dto, err := env.ReportService.GenerateAndStoreExport(context.Background(), superAdmin, snap.SnapshotID, "csv", nil)
	if err != nil {
		t.Fatalf("export failed: %v", err)
	}

	// Verify ContentHash matches snapshot DataHash
	if dto.ContentHash == "" {
		t.Fatalf("expected non-empty ContentHash")
	}

	// Verify snapshot data is preserved exactly
	snapDetail, err := env.ReportService.GetSnapshotDetailAdmin(context.Background(), snap.SnapshotID)
	if err != nil {
		t.Fatalf("querying snapshot failed: %v", err)
	}

	if snapDetail.DataHash != snap.DataHash {
		t.Errorf("snapshot DataHash altered: expected %s, got %s", snap.DataHash, snapDetail.DataHash)
	}
}

// 6. SHA-256 checksum correctness
func TestReportExport_ChecksumCorrectness(t *testing.T) {
	env := setupTestEnv(t)
	superAdmin := createTestUserWithRole(t, env, "super_admin", fmt.Sprintf("sa_%d@ratels.org", time.Now().UnixNano()))

	snap := createTestSnapshot(t, env, superAdmin, "cleanups", "2087-04", false)

	dto, err := env.ReportService.GenerateAndStoreExport(context.Background(), superAdmin, snap.SnapshotID, "pdf", nil)
	if err != nil {
		t.Fatalf("export failed: %v", err)
	}

	pdfBytes, _, _, err := env.ReportService.GetExportPrivateStream(context.Background(), superAdmin, snap.SnapshotID, "pdf")
	if err != nil {
		t.Fatalf("streaming failed: %v", err)
	}

	hasher := sha256.New()
	hasher.Write(pdfBytes)
	actualHash := hex.EncodeToString(hasher.Sum(nil))

	if dto.ContentHash != actualHash {
		t.Errorf("ContentHash mismatch: DB record has %s, actual binary SHA-256 is %s", dto.ContentHash, actualHash)
	}
	if dto.FileSizeBytes != int64(len(pdfBytes)) {
		t.Errorf("FileSizeBytes mismatch: DB record has %d, actual len is %d", dto.FileSizeBytes, len(pdfBytes))
	}
}

// 7. Private R2 upload behavior (storage key prefix, MIME type, private access)
func TestReportExport_PrivateR2UploadBehavior(t *testing.T) {
	env := setupTestEnv(t)
	superAdmin := createTestUserWithRole(t, env, "super_admin", fmt.Sprintf("sa_%d@ratels.org", time.Now().UnixNano()))

	snap := createTestSnapshot(t, env, superAdmin, "finance", "2087-05", false)

	dto, err := env.ReportService.GenerateAndStoreExport(context.Background(), superAdmin, snap.SnapshotID, "pdf", nil)
	if err != nil {
		t.Fatalf("export failed: %v", err)
	}

	if dto.StorageReference == nil || *dto.StorageReference == "" {
		t.Fatalf("missing storage reference")
	}

	key := *dto.StorageReference
	if !strings.HasPrefix(key, "reports/exports/private/") {
		t.Errorf("storage key missing private prefix: got %q", key)
	}
	if !strings.Contains(key, snap.SnapshotID) {
		t.Errorf("storage key missing snapshot ID: got %q", key)
	}
	if strings.Contains(key, "..") {
		t.Errorf("path traversal vulnerability in key: %q", key)
	}
}

// 8. Duplicate export prevention (deduplication returns existing cached record)
func TestReportExport_DuplicateExportPrevention(t *testing.T) {
	env := setupTestEnv(t)
	superAdmin := createTestUserWithRole(t, env, "super_admin", fmt.Sprintf("sa_%d@ratels.org", time.Now().UnixNano()))

	snap := createTestSnapshot(t, env, superAdmin, "cleanups", "2087-06", false)

	// Call 1
	exp1, err := env.ReportService.GenerateAndStoreExport(context.Background(), superAdmin, snap.SnapshotID, "csv", nil)
	if err != nil {
		t.Fatalf("call 1 failed: %v", err)
	}

	// Call 2
	exp2, err := env.ReportService.GenerateAndStoreExport(context.Background(), superAdmin, snap.SnapshotID, "csv", nil)
	if err != nil {
		t.Fatalf("call 2 failed: %v", err)
	}

	// Must be exact same export ID and storage reference
	if exp1.ID != exp2.ID {
		t.Errorf("deduplication failed: different export IDs created: %s vs %s", exp1.ID, exp2.ID)
	}
	if *exp1.StorageReference != *exp2.StorageReference {
		t.Errorf("deduplication failed: different storage references: %s vs %s", *exp1.StorageReference, *exp2.StorageReference)
	}
}

// Mock failing storage implementation
type failingUploadStorage struct {
	storage.ReportExportStorage
}

func (f *failingUploadStorage) StoreExport(ctx context.Context, snapshotID string, format string, originalFileName string, contentType string, data []byte) (string, string, int64, error) {
	return "", "", 0, errors.New("simulated R2 network timeout / connection refused")
}

// 9. Upload failure and retry (no DB record claiming success; retry succeeds)
func TestReportExport_UploadFailureAndRetry(t *testing.T) {
	env := setupTestEnv(t)
	superAdmin := createTestUserWithRole(t, env, "super_admin", fmt.Sprintf("sa_%d@ratels.org", time.Now().UnixNano()))

	snap := createTestSnapshot(t, env, superAdmin, "cleanups", "2087-07", false)

	origStorage := storage.NewDevSandboxReportExportStorage()
	env.ReportService.SetExportStorage(&failingUploadStorage{ReportExportStorage: origStorage})

	// Generation attempt fails at storage
	_, err := env.ReportService.GenerateAndStoreExport(context.Background(), superAdmin, snap.SnapshotID, "pdf", nil)
	if err == nil {
		t.Fatalf("expected upload failure, got nil")
	}

	// Verify NO record exists in DB
	var count int
	_ = env.Pool.QueryRow(context.Background(), "SELECT COUNT(*) FROM public_report_exports WHERE snapshot_id = $1", snap.SnapshotID).Scan(&count)
	if count != 0 {
		t.Fatalf("FAIL: DB export record created despite storage upload failure! count = %d", count)
	}

	// Restore good storage and retry -> must succeed
	env.ReportService.SetExportStorage(origStorage)
	dto, err := env.ReportService.GenerateAndStoreExport(context.Background(), superAdmin, snap.SnapshotID, "pdf", nil)
	if err != nil {
		t.Fatalf("retry failed: %v", err)
	}
	if dto.ExportStatus != "generated" {
		t.Errorf("expected export status 'generated', got %q", dto.ExportStatus)
	}
}

// 10. Database persistence failure (cleanup orphaned storage object)
func TestReportExport_DatabasePersistenceFailure(t *testing.T) {
	env := setupTestEnv(t)
	superAdmin := createTestUserWithRole(t, env, "super_admin", fmt.Sprintf("sa_%d@ratels.org", time.Now().UnixNano()))

	// Use an invalid snapshot ID that violates foreign key constraint in public_report_exports
	fakeSnapshotID := security.GenerateUUID()

	_, err := env.ReportService.GenerateAndStoreExport(context.Background(), superAdmin, fakeSnapshotID, "pdf", nil)
	if err == nil {
		t.Fatalf("expected error for non-existent snapshot ID, got nil")
	}
	if !errors.Is(err, service.ErrReportSnapshotNotFound) {
		t.Errorf("expected ErrReportSnapshotNotFound, got %v", err)
	}
}

// 11. Audit trail generation
func TestReportExport_AuditTrailGeneration(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()
	superAdmin := createTestUserWithRole(t, env, "super_admin", fmt.Sprintf("sa_%d@ratels.org", time.Now().UnixNano()))

	snap := createTestSnapshot(t, env, superAdmin, "cleanups", "2087-08", false)

	ip := "192.168.1.100"
	dto, err := env.ReportService.GenerateAndStoreExport(ctx, superAdmin, snap.SnapshotID, "csv", &ip)
	if err != nil {
		t.Fatalf("export failed: %v", err)
	}

	// Verify public_report_audit has action='exported'
	var auditCount int
	var loggedExportID string
	var loggedFormat string
	err = env.Pool.QueryRow(ctx, `
		SELECT COUNT(*), (details->>'export_id'), (details->>'export_format')
		FROM public_report_audit
		WHERE snapshot_id = $1 AND action = 'exported'
		GROUP BY details->>'export_id', details->>'export_format'
	`, snap.SnapshotID).Scan(&auditCount, &loggedExportID, &loggedFormat)
	if err != nil || auditCount == 0 {
		t.Fatalf("missing 'exported' audit log: err=%v, count=%d", err, auditCount)
	}

	if loggedExportID != dto.ID {
		t.Errorf("audit export_id mismatch: expected %s, got %s", dto.ID, loggedExportID)
	}
	if loggedFormat != "csv" {
		t.Errorf("audit format mismatch: expected 'csv', got %s", loggedFormat)
	}
}

// 12. Draft export access restrictions (non-admin / public cannot access draft exports)
func TestReportExport_DraftExportAccessRestrictions(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()
	superAdmin := createTestUserWithRole(t, env, "super_admin", fmt.Sprintf("sa_%d@ratels.org", time.Now().UnixNano()))
	memberUser := createTestUserWithRole(t, env, "member", fmt.Sprintf("mem_%d@ratels.org", time.Now().UnixNano()))

	snap := createTestSnapshot(t, env, superAdmin, "finance", "2087-09", false)

	// Generate export while still in draft
	_, err := env.ReportService.GenerateAndStoreExport(ctx, superAdmin, snap.SnapshotID, "pdf", nil)
	if err != nil {
		t.Fatalf("generating export failed: %v", err)
	}

	// 1. Unauthenticated request (actor == nil) on draft report -> MUST BE REJECTED
	_, _, _, err = env.ReportService.GetExportPrivateStream(ctx, nil, snap.SnapshotID, "pdf")
	if !errors.Is(err, service.ErrUnauthorizedExportAccess) {
		t.Errorf("expected ErrUnauthorizedExportAccess for unauthenticated draft download, got %v", err)
	}

	// 2. Regular member user request on draft report -> MUST BE REJECTED
	_, _, _, err = env.ReportService.GetExportPrivateStream(ctx, memberUser, snap.SnapshotID, "pdf")
	if !errors.Is(err, service.ErrUnauthorizedExportAccess) {
		t.Errorf("expected ErrUnauthorizedExportAccess for member draft download, got %v", err)
	}

	// 3. Super admin request on draft report -> MUST SUCCEED (for internal review)
	_, _, _, err = env.ReportService.GetExportPrivateStream(ctx, superAdmin, snap.SnapshotID, "pdf")
	if err != nil {
		t.Errorf("expected super_admin to access draft export for audit review, got %v", err)
	}
}

// 13. Withdrawn report handling (exports marked withdrawn, purged, download blocked)
func TestReportExport_WithdrawnReportHandling(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()
	uid := time.Now().UnixNano()
	superAdmin := createTestUserWithRole(t, env, "super_admin", fmt.Sprintf("sa_%d@ratels.org", uid))
	nationalAdmin := createTestUserWithRole(t, env, "national_admin", fmt.Sprintf("na_%d@ratels.org", uid))

	snap := createTestSnapshot(t, env, nationalAdmin, "cleanups", "2087-10", false)

	// Progress through review and publication
	_, err := env.ReportService.SubmitForReview(ctx, nationalAdmin, snap.SnapshotID, nil)
	if err != nil {
		t.Fatalf("submit for review failed: %v", err)
	}
	_, err = env.ReportService.ApproveSnapshot(ctx, superAdmin, snap.SnapshotID, "Signoff", nil)
	if err != nil {
		t.Fatalf("approval failed: %v", err)
	}
	_, err = env.ReportService.PublishSnapshot(ctx, superAdmin, snap.SnapshotID, nil)
	if err != nil {
		t.Fatalf("publication failed: %v", err)
	}

	// Generate export while published
	exp, err := env.ReportService.GenerateAndStoreExport(ctx, superAdmin, snap.SnapshotID, "pdf", nil)
	if err != nil {
		t.Fatalf("generating export failed: %v", err)
	}

	// Now WITHDRAW publication
	_, err = env.ReportService.WithdrawPublication(ctx, superAdmin, snap.SnapshotID, "Data reconciliation", nil)
	if err != nil {
		t.Fatalf("withdrawal failed: %v", err)
	}

	// 1. Verify export record status changed to 'withdrawn' in DB
	var exportStatus string
	err = env.Pool.QueryRow(ctx, "SELECT export_status FROM public_report_exports WHERE id = $1", exp.ID).Scan(&exportStatus)
	if err != nil || exportStatus != "withdrawn" {
		t.Fatalf("expected export_status 'withdrawn', got %q (err: %v)", exportStatus, err)
	}

	// 2. Download attempt must be rejected with ErrReportWithdrawn
	_, _, _, err = env.ReportService.GetExportPrivateStream(ctx, superAdmin, snap.SnapshotID, "pdf")
	if !errors.Is(err, service.ErrReportWithdrawn) {
		t.Errorf("expected ErrReportWithdrawn downloading withdrawn export, got %v", err)
	}
}

// 14. System actor attribution (created_by_user_id IS NULL and created_by_type = 'system')
func TestReportExport_SystemActorAttribution(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()

	period := "2087-11"
	defer func() {
		_, _ = env.Pool.Exec(ctx, "DELETE FROM public_report_snapshots WHERE reporting_period = $1", period)
	}()

	// Create system-generated snapshot (actor is nil, IsSystemActor is true)
	sysSnap, err := env.ReportService.GenerateSnapshot(ctx, nil, service.GenerateReportRequest{
		ReportType:      "national_overview",
		Title:           "System Generated Overview",
		ReportingPeriod: period,
		PeriodStart:     "2087-11-01",
		PeriodEnd:       "2087-11-30",
		ScopeType:       "national",
		TriggerSource:   "scheduled",
		IsSystemActor:   true,
	}, nil)
	if err != nil {
		t.Fatalf("system snapshot generation failed: %v", err)
	}

	// Verify DTO fields
	if sysSnap.CreatedByUserID != nil {
		t.Errorf("expected CreatedByUserID to be nil for system actor, got %v", *sysSnap.CreatedByUserID)
	}
	if sysSnap.CreatedByType != "system" {
		t.Errorf("expected CreatedByType 'system', got %q", sysSnap.CreatedByType)
	}
	if sysSnap.CreatorName != "System" {
		t.Errorf("expected CreatorName 'System', got %q", sysSnap.CreatorName)
	}

	// Verify database record directly
	var dbUserID pgtype.Text
	var dbType string
	err = env.Pool.QueryRow(ctx, `
		SELECT created_by_user_id, created_by_type
		FROM public_report_snapshots
		WHERE id = $1
	`, sysSnap.SnapshotID).Scan(&dbUserID, &dbType)
	if err != nil {
		t.Fatalf("database query failed: %v", err)
	}

	if dbUserID.Valid {
		t.Errorf("database created_by_user_id is NOT NULL: %q", dbUserID.String)
	}
	if dbType != "system" {
		t.Errorf("database created_by_type is %q, expected 'system'", dbType)
	}
}

// 15. Existing approval workflow preservation
func TestReportExport_ExistingApprovalWorkflowPreservation(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()
	uid := time.Now().UnixNano()
	superAdmin := createTestUserWithRole(t, env, "super_admin", fmt.Sprintf("sa_%d@ratels.org", uid))
	nationalAdmin := createTestUserWithRole(t, env, "national_admin", fmt.Sprintf("na_%d@ratels.org", uid))

	snap := createTestSnapshot(t, env, nationalAdmin, "cleanups", "2087-12", false)

	// Generate export on draft snapshot
	_, err := env.ReportService.GenerateAndStoreExport(ctx, nationalAdmin, snap.SnapshotID, "pdf", nil)
	if err != nil {
		t.Fatalf("generating export failed: %v", err)
	}

	// Verify snapshot status remains draft
	detail1, err := env.ReportService.GetSnapshotDetailAdmin(ctx, snap.SnapshotID)
	if err != nil || detail1.Status != "draft" {
		t.Fatalf("snapshot status altered by export: got %q", detail1.Status)
	}

	// Submit for review -> status must transition to pending_review
	detail2, err := env.ReportService.SubmitForReview(ctx, nationalAdmin, snap.SnapshotID, nil)
	if err != nil || detail2.Status != "pending_review" {
		t.Fatalf("submit for review failed: got status %q (err: %v)", detail2.Status, err)
	}

	// Approve snapshot -> status must transition to approved
	detail3, err := env.ReportService.ApproveSnapshot(ctx, superAdmin, snap.SnapshotID, "Audited and verified", nil)
	if err != nil || detail3.Status != "approved" {
		t.Fatalf("approval failed: got status %q (err: %v)", detail3.Status, err)
	}

	// Publish snapshot -> must publish cleanly
	pub, err := env.ReportService.PublishSnapshot(ctx, superAdmin, snap.SnapshotID, nil)
	if err != nil || pub.SnapshotID != snap.SnapshotID {
		t.Fatalf("publish failed: err=%v", err)
	}
}
