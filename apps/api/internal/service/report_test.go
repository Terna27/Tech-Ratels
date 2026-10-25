package service_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"ratels-api/internal/db"
	"ratels-api/internal/security"
	"ratels-api/internal/service"
)

func TestReportService_EndToEndLifecycle(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()

	uid := time.Now().UnixNano()
	superAdmin := createTestUserWithRole(t, env, "super_admin", fmt.Sprintf("superadmin_rep_%d@test.com", uid))
	nationalAdmin := createTestUserWithRole(t, env, "national_admin", fmt.Sprintf("natadmin_rep_%d@test.com", uid))
	volunteer := createTestUserWithRole(t, env, "member", fmt.Sprintf("volunteer_rep_%d@test.com", uid))

	// Get a state for testing
	states, err := env.Repo.Queries().ListStates(ctx)
	if err != nil || len(states) == 0 {
		t.Fatalf("States reference missing: %v", err)
	}
	testState := states[0]

	lgas, err := env.Repo.Queries().ListLGAsByState(ctx, testState.ID)
	if err != nil || len(lgas) == 0 {
		t.Fatalf("LGAs reference missing: %v", err)
	}
	testLGA := lgas[0]

	// 1. Seed verified operational data
	// A. Cleanup Campaign & Event
	now := time.Now()
	var campaignID string
	err = env.Pool.QueryRow(ctx, `SELECT id FROM cleanup_campaigns WHERE year = $1 AND month = $2`, now.Year(), int(now.Month())).Scan(&campaignID)
	if err != nil {
		campaignID = security.GenerateUUID()
		_, err = env.Pool.Exec(ctx, `
			INSERT INTO cleanup_campaigns (id, year, month, scheduled_date, title, status)
			VALUES ($1, $2, $3, $4, 'National Clean City Campaign', 'scheduled')
		`, campaignID, now.Year(), int(now.Month()), now)
		if err != nil {
			t.Fatalf("Failed creating campaign: %v", err)
		}
	}

	eventID := security.GenerateUUID()
	_, err = env.Repo.Queries().CreateEventInstance(ctx, db.CreateEventInstanceParams{
		ID:                 eventID,
		CampaignID:         pgtype.Text{String: campaignID, Valid: true},
		ProgrammeType:      "cleanup",
		Title:              "Central Market Sanitation",
		StateID:            testState.ID,
		LgaID:              testLGA.ID,
		Address:            "Market Square Road",
		Landmark:           "Post Office",
		StartTime:          pgtype.Timestamptz{Time: now.Add(-2 * time.Hour), Valid: true},
		EndTime:            pgtype.Timestamptz{Time: now, Valid: true},
		Capacity:           200,
		CoordinatorID:      nationalAdmin.ID,
		PublicContact:      "coord@ratels.org",
		AccessibilityNotes: pgtype.Text{Valid: false},
		SafetyNotes:        pgtype.Text{Valid: false},
	})
	if err != nil {
		t.Fatalf("Failed creating event: %v", err)
	}

	// Update event status to completed with environmental indicators
	_, err = env.Pool.Exec(ctx, `UPDATE event_instances SET status = 'completed', waste_collected_kg = 850, waste_bags_count = 42, cleanup_duration_hours = 3.5 WHERE id = $1`, eventID)
	if err != nil {
		t.Fatalf("Failed updating event environmental metrics: %v", err)
	}

	// Participation claim: approved
	claimID := security.GenerateUUID()
	_, err = env.Repo.Queries().CreateParticipationClaim(ctx, db.CreateParticipationClaimParams{
		ID:            claimID,
		EventID:       eventID,
		UserID:        volunteer.ID,
		Description:   "Cleared organic waste and sorted plastics",
		EvidenceNotes: pgtype.Text{String: "Badge photo uploaded", Valid: true},
	})
	if err != nil {
		t.Fatalf("Failed creating participation claim: %v", err)
	}

	_, err = env.Repo.Queries().ReviewParticipationClaim(ctx, db.ReviewParticipationClaimParams{
		ID:               claimID,
		Status:           "approved",
		ReviewedByUserID: pgtype.Text{String: nationalAdmin.ID, Valid: true},
		ReviewDecision:   pgtype.Text{String: "approved", Valid: true},
		ReviewReason:     pgtype.Text{String: "Photo verified", Valid: true},
	})
	if err != nil {
		t.Fatalf("Failed approving participation: %v", err)
	}

	// B. Seed community project
	projectID := security.GenerateUUID()
	_, err = env.Pool.Exec(ctx, `
		INSERT INTO community_projects (
			id, reference, title, description, category, state_id, lga_id,
			community_ward, location_details, approved_budget, recorded_expenditure,
			execution_status, progress_percentage, is_public, created_by_user_id
		) VALUES (
			$1, $2, 'Borehole Rehabilitation', 'Clean drinking water access',
			'water_sanitation', $3, $4, 'Ward 1', 'Central Village', 150000000, 120000000,
			'completed', 100, true, $5
		)
	`, projectID, fmt.Sprintf("PRJ-2026-TEST-%d", uid), testState.ID, testLGA.ID, superAdmin.ID)
	if err != nil {
		t.Fatalf("Failed inserting test project: %v", err)
	}

	// C. Seed financial ledger records
	txID := security.GenerateUUID()
	_, err = env.Pool.Exec(ctx, `
		INSERT INTO financial_transactions (
			id, reference, transaction_type, amount, currency,
			notes, verification_status, occurred_at, recorded_at
		) VALUES (
			$1, $2, 'income', 250000000, 'NGN',
			'Verified public grant contribution', 'verified', NOW(), NOW()
		)
	`, txID, fmt.Sprintf("TX-2026-INCOME-%d", uid))
	if err != nil {
		t.Fatalf("Failed inserting test financial transaction: %v", err)
	}

	// Seed foundation donation (segregated)
	fndDonationID := security.GenerateUUID()
	_, err = env.Pool.Exec(ctx, `
		INSERT INTO financial_donations (
			id, reference, entity_type, donor_name, donor_email,
			amount_kobo, currency, status, destination_fund_id
		) VALUES (
			$1, $2, 'foundation', 'Dr. Anonymous Philanthropist', 'donor@private.com',
			50000000, 'NGN', 'verified', 'foundation_donation_fund'
		)
	`, fndDonationID, fmt.Sprintf("DON-2026-FND-%d", uid))
	if err != nil {
		t.Fatalf("Failed inserting foundation donation: %v", err)
	}

	// 2. Draft Generation by National Admin
	ip := "127.0.0.1"
	draft, err := env.ReportService.GenerateSnapshot(ctx, nationalAdmin, service.GenerateReportRequest{
		ReportType:      "national_overview",
		Title:           "National Overview Q4 2026",
		ReportingPeriod: "2026",
		PeriodStart:     time.Now().AddDate(-1, 0, 0).Format("2006-01-02"),
		PeriodEnd:       time.Now().AddDate(0, 1, 0).Format("2006-01-02"),
		ScopeType:       "national",
	}, &ip)
	if err != nil {
		t.Fatalf("Failed generating draft snapshot: %v", err)
	}

	if draft.Status != "draft" {
		t.Errorf("Expected draft status, got %s", draft.Status)
	}
	if draft.DataHash == "" {
		t.Error("Expected non-empty cryptographic data hash")
	}

	// Verify aggregated values in draft
	var overview service.PublicOverviewData
	if err := json.Unmarshal(draft.AggregatedData, &overview); err != nil {
		t.Fatalf("Failed parsing aggregated data: %v", err)
	}
	if overview.CompletedCleanupsCount < 1 {
		t.Errorf("Expected >= 1 completed cleanup, got %d", overview.CompletedCleanupsCount)
	}
	if overview.VerifiedAttendeesCount < 1 {
		t.Errorf("Expected >= 1 verified attendee, got %d", overview.VerifiedAttendeesCount)
	}
	if overview.CompletedProjectsCount < 1 {
		t.Errorf("Expected >= 1 completed project, got %d", overview.CompletedProjectsCount)
	}
	if overview.ApprovedIncomeKobo < 250000000 {
		t.Errorf("Expected >= 250,000,000 kobo income, got %d", overview.ApprovedIncomeKobo)
	}
	if overview.FoundationDonationsKobo < 50000000 {
		t.Errorf("Expected >= 50,000,000 kobo foundation donations, got %d", overview.FoundationDonationsKobo)
	}

	// 3. Submit for Review
	pendingReview, err := env.ReportService.SubmitForReview(ctx, nationalAdmin, draft.SnapshotID, &ip)
	if err != nil {
		t.Fatalf("Failed submitting report for review: %v", err)
	}
	if pendingReview.Status != "pending_review" {
		t.Errorf("Expected pending_review status, got %s", pendingReview.Status)
	}

	// 4. Governance Rule: Anti-Self-Approval
	// nationalAdmin created the report, so nationalAdmin MUST NOT be allowed to self-approve!
	_, err = env.ReportService.ApproveSnapshot(ctx, nationalAdmin, draft.SnapshotID, "Self approving my work", &ip)
	if err == nil {
		t.Fatal("Expected error on self-approval attempt, got nil")
	}
	if err != service.ErrSelfApprovalProhibited {
		t.Errorf("Expected ErrSelfApprovalProhibited, got %v", err)
	}

	// 5. Governance Rule: Super Admin Approval for Financial Reports
	// Approve properly by Super Admin (different user)
	approved, err := env.ReportService.ApproveSnapshot(ctx, superAdmin, draft.SnapshotID, "Governance verification complete and certified", &ip)
	if err != nil {
		t.Fatalf("Failed approving snapshot by Super Admin: %v", err)
	}
	if approved.Status != "approved" {
		t.Errorf("Expected approved status, got %s", approved.Status)
	}

	// Verify approval history was stored
	if len(approved.Approvals) == 0 {
		t.Error("Expected at least 1 approval record")
	}

	// 6. Publication by Super Admin
	pub, err := env.ReportService.PublishSnapshot(ctx, superAdmin, draft.SnapshotID, &ip)
	if err != nil {
		t.Fatalf("Failed publishing snapshot: %v", err)
	}
	if pub.PublicationID == "" {
		t.Error("Expected valid publication ID")
	}
	if pub.PublicReference != draft.PublicReference {
		t.Errorf("Expected ref %s, got %s", draft.PublicReference, pub.PublicReference)
	}

	// 7. Verify Public Availability via Allowlisted API
	pubDetail, err := env.ReportService.GetPublicPublicationByID(ctx, pub.PublicReference)
	if err != nil {
		t.Fatalf("Failed retrieving public publication: %v", err)
	}
	if pubDetail.PublicReference != draft.PublicReference {
		t.Errorf("Mismatch in public ref: %s vs %s", pubDetail.PublicReference, draft.PublicReference)
	}

	// 8. PDF Export Verification
	pdfBytes, pdfFilename, err := env.ReportService.ExportReportPDF(ctx, pub.PublicReference)
	if err != nil {
		t.Fatalf("Failed exporting PDF: %v", err)
	}
	if len(pdfBytes) < 500 {
		t.Errorf("PDF output is suspiciously small: %d bytes", len(pdfBytes))
	}
	if !strings.HasPrefix(string(pdfBytes), "%PDF-1.4") {
		t.Error("PDF output does not have standard %PDF-1.4 header")
	}
	if !strings.Contains(pdfFilename, ".pdf") {
		t.Errorf("Expected .pdf extension in filename %s", pdfFilename)
	}

	// 9. CSV Export Verification & Formula Injection Defense
	csvBytes, csvFilename, err := env.ReportService.ExportReportCSV(ctx, pub.PublicReference)
	if err != nil {
		t.Fatalf("Failed exporting CSV: %v", err)
	}
	csvStr := string(csvBytes)
	if !strings.Contains(csvStr, "Ratels Worldwide Platform") && !strings.Contains(csvStr, "Nigerian Ratels") {
		t.Error("Expected Ratels branding in CSV export")
	}
	if !strings.Contains(csvFilename, ".csv") {
		t.Errorf("Expected .csv extension in filename %s", csvFilename)
	}

	// 10. Withdrawal & Correction Revision Workflow
	withdrawn, err := env.ReportService.WithdrawPublication(ctx, superAdmin, draft.SnapshotID, "Periodic data reconciliation audit required", &ip)
	if err != nil {
		t.Fatalf("Failed withdrawing publication: %v", err)
	}
	if withdrawn.Status != "withdrawn" {
		t.Errorf("Expected withdrawn status, got %s", withdrawn.Status)
	}

	// Public lookup should now fail for withdrawn publication
	_, err = env.ReportService.GetPublicPublicationByID(ctx, draft.PublicReference)
	if err == nil {
		t.Error("Expected error fetching withdrawn report via public lookup, got nil")
	}

	// Create revised version
	revised, err := env.ReportService.ReviseSnapshot(ctx, superAdmin, draft.SnapshotID, "Updated with latest audited figures", &ip)
	if err != nil {
		t.Fatalf("Failed creating revised snapshot: %v", err)
	}
	if revised.PublicationVersion != 2 {
		t.Errorf("Expected publication version 2, got %d", revised.PublicationVersion)
	}
	if !strings.Contains(revised.PublicReference, "-R2") {
		t.Errorf("Expected revised reference suffix -R2, got %s", revised.PublicReference)
	}
	if revised.Status != "draft" {
		t.Errorf("Expected revised snapshot to start in draft, got %s", revised.Status)
	}
}

func TestReportService_CSVFormulaInjectionSanitization(t *testing.T) {
	dangerousInputs := []struct {
		input    string
		expected string
	}{
		{"=SUM(A1:A10)", "'=SUM(A1:A10)"},
		{"+cmd|' /C calc'!A0", "'+cmd|' /C calc'!A0"},
		{"-2+3*cmd|' /C calc'!A0", "'-2+3*cmd|' /C calc'!A0"},
		{"@SUM(1,2)", "'@SUM(1,2)"},
		{"\tcmd", "'\tcmd"},
		{"Normal Text", "Normal Text"},
		{"Benue State", "Benue State"},
	}

	for _, tc := range dangerousInputs {
		sanitized := service.SanitizeCSVValue(tc.input)
		if sanitized != tc.expected {
			t.Errorf("SanitizeCSVValue(%q) = %q; expected %q", tc.input, sanitized, tc.expected)
		}
	}
}

func TestReportService_PrivacySuppression(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()

	partData, err := env.ReportService.GetPublicParticipation(ctx, nil, nil, nil, nil, "2026")
	if err != nil {
		t.Fatalf("Failed fetching participation: %v", err)
	}

	if partData.UniqueVerifiedMembers > 0 && partData.UniqueVerifiedMembers < 5 {
		if partData.UniqueMembersSuppressed == "" {
			t.Errorf("Expected small-cell suppression warning for count %d, got empty", partData.UniqueVerifiedMembers)
		}
	}
}

func TestReportService_FinancePublicationSecurity(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()

	ctx := context.Background()
	uid := time.Now().UnixNano()

	superAdmin := createTestUserWithRole(
		t, env, "super_admin",
		fmt.Sprintf("finance_sa_%d@test.com", uid),
	)
	nationalAdmin := createTestUserWithRole(
		t, env, "national_admin",
		fmt.Sprintf("finance_admin_%d@test.com", uid),
	)

	ip := "127.0.0.1"
	period := fmt.Sprintf("finance-security-%d", uid)

	request := service.GenerateReportRequest{
		ReportType:      "finance",
		Title:           "Finance Publication Security Test",
		ReportingPeriod: period,
		PeriodStart:     "2026-01-01",
		PeriodEnd:       "2026-12-31",
		ScopeType:       "national",
	}

	draft, err := env.ReportService.GenerateSnapshot(
		ctx, nationalAdmin, request, &ip,
	)
	if err != nil {
		t.Fatalf("generate finance snapshot: %v", err)
	}

	assertNotPublished := func(stage string) {
		t.Helper()
		_, err := env.ReportService.GetPublicFinance(
			ctx, nil, nil, period,
		)
		if err != service.ErrReportPublicationNotFound {
			t.Fatalf(
				"%s: expected unpublished finance error, got %v",
				stage, err,
			)
		}
	}

	assertNotPublished("draft")

	_, err = env.ReportService.SubmitForReview(
		ctx, nationalAdmin, draft.SnapshotID, &ip,
	)
	if err != nil {
		t.Fatalf("submit finance report: %v", err)
	}

	_, err = env.ReportService.ApproveSnapshot(
		ctx, superAdmin, draft.SnapshotID,
		"Finance security test approval", &ip,
	)
	if err != nil {
		t.Fatalf("approve finance report: %v", err)
	}

	assertNotPublished("approved but unpublished")

	_, err = env.ReportService.PublishSnapshot(
		ctx, nationalAdmin, draft.SnapshotID, &ip,
	)
	if err != service.ErrFinancialGovernanceRole {
		t.Fatalf(
			"expected unauthorized publisher rejection, got %v",
			err,
		)
	}

	assertNotPublished("unauthorized publication attempt")

	_, err = env.ReportService.PublishSnapshot(
		ctx, superAdmin, draft.SnapshotID, &ip,
	)
	if err != nil {
		t.Fatalf("super admin publication: %v", err)
	}

	finance, err := env.ReportService.GetPublicFinance(
		ctx, nil, nil, period,
	)
	if err != nil {
		t.Fatalf("published finance report inaccessible: %v", err)
	}
	if finance == nil {
		t.Fatal("published finance response is nil")
	}
	if finance.ReportingPeriod != period {
		t.Fatalf(
			"wrong reporting period: got %q, want %q",
			finance.ReportingPeriod, period,
		)
	}

	_, err = env.ReportService.GetPublicFinance(
		ctx, nil, nil, period+"-other",
	)
	if err != service.ErrReportPublicationNotFound {
		t.Fatalf(
			"different period must not return published data: %v",
			err,
		)
	}

	_, err = env.ReportService.WithdrawPublication(
		ctx, superAdmin, draft.SnapshotID,
		"Finance publication security regression test", &ip,
	)
	if err != nil {
		t.Fatalf("withdraw finance publication: %v", err)
	}

	assertNotPublished("withdrawn")
}

func TestReportService_FinanceStateIsolation(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()

	ctx := context.Background()
	uid := time.Now().UnixNano()

	states, err := env.Repo.Queries().ListStates(ctx)
	if err != nil {
		t.Fatalf("list states: %v", err)
	}
	if len(states) < 2 {
		t.Fatal("at least two states are required for isolation testing")
	}

	stateA := states[0]
	stateB := states[1]

	if stateA.ID == stateB.ID {
		t.Fatal("test states must have different IDs")
	}

	superAdmin := createTestUserWithRole(
		t, env, "super_admin",
		fmt.Sprintf("state_finance_sa_%d@test.com", uid),
	)
	nationalAdmin := createTestUserWithRole(
		t, env, "national_admin",
		fmt.Sprintf("state_finance_admin_%d@test.com", uid),
	)

	ip := "127.0.0.1"
	period := fmt.Sprintf("state-finance-security-%d", uid)
	scope := "state"

	request := service.GenerateReportRequest{
		ReportType:      "finance",
		Title:           "State Financial Isolation Security Test",
		ReportingPeriod: period,
		PeriodStart:     "2026-01-01",
		PeriodEnd:       "2026-12-31",
		ScopeType:       scope,
		StateID:         &stateA.ID,
	}

	draft, err := env.ReportService.GenerateSnapshot(
		ctx, nationalAdmin, request, &ip,
	)
	if err != nil {
		t.Fatalf("generate state finance snapshot: %v", err)
	}

	_, err = env.ReportService.GetPublicFinance(
		ctx, &scope, &stateA.ID, period,
	)
	if err != service.ErrReportPublicationNotFound {
		t.Fatalf("draft must remain private: %v", err)
	}

	_, err = env.ReportService.SubmitForReview(
		ctx, nationalAdmin, draft.SnapshotID, &ip,
	)
	if err != nil {
		t.Fatalf("submit snapshot: %v", err)
	}

	_, err = env.ReportService.ApproveSnapshot(
		ctx, superAdmin, draft.SnapshotID,
		"State financial security approval", &ip,
	)
	if err != nil {
		t.Fatalf("approve snapshot: %v", err)
	}

	_, err = env.ReportService.PublishSnapshot(
		ctx, superAdmin, draft.SnapshotID, &ip,
	)
	if err != nil {
		t.Fatalf("publish snapshot: %v", err)
	}

	// The correct state must be able to retrieve its report.
	finance, err := env.ReportService.GetPublicFinance(
		ctx, &scope, &stateA.ID, period,
	)
	if err != nil {
		t.Fatalf("correct state could not retrieve report: %v", err)
	}
	if finance == nil || finance.ReportingPeriod != period {
		t.Fatalf("unexpected finance response: %+v", finance)
	}

	// A different state must not retrieve State A's report.
	_, err = env.ReportService.GetPublicFinance(
		ctx, &scope, &stateB.ID, period,
	)
	if err != service.ErrReportPublicationNotFound {
		t.Fatalf(
			"cross-state financial disclosure: expected not found, got %v",
			err,
		)
	}

	// A national request must not retrieve a state report.
	_, err = env.ReportService.GetPublicFinance(
		ctx, nil, nil, period,
	)
	if err != service.ErrReportPublicationNotFound {
		t.Fatalf(
			"state report leaked into national scope: %v",
			err,
		)
	}

	_, err = env.ReportService.WithdrawPublication(
		ctx, superAdmin, draft.SnapshotID,
		"State financial isolation regression test", &ip,
	)
	if err != nil {
		t.Fatalf("withdraw state report: %v", err)
	}

	_, err = env.ReportService.GetPublicFinance(
		ctx, &scope, &stateA.ID, period,
	)
	if err != service.ErrReportPublicationNotFound {
		t.Fatalf(
			"withdrawn state report remains accessible: %v",
			err,
		)
	}
}

func TestReportService_FoundationPublicationSecurity(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()

	ctx := context.Background()
	uid := time.Now().UnixNano()

	superAdmin := createTestUserWithRole(
		t, env, "super_admin",
		fmt.Sprintf("foundation_sa_%d@test.com", uid),
	)
	nationalAdmin := createTestUserWithRole(
		t, env, "national_admin",
		fmt.Sprintf("foundation_admin_%d@test.com", uid),
	)

	ip := "127.0.0.1"
	period := fmt.Sprintf("foundation-security-%d", uid)

	request := service.GenerateReportRequest{
		ReportType:      "foundation",
		Title:           "Foundation Publication Security Test",
		ReportingPeriod: period,
		PeriodStart:     "2026-01-01",
		PeriodEnd:       "2026-12-31",
		ScopeType:       "national",
	}

	draft, err := env.ReportService.GenerateSnapshot(
		ctx, nationalAdmin, request, &ip,
	)
	if err != nil {
		t.Fatalf("generate foundation snapshot: %v", err)
	}

	assertNotPublished := func(stage string) {
		t.Helper()
		_, err := env.ReportService.GetPublicFoundation(ctx, period)
		if err != service.ErrReportPublicationNotFound {
			t.Fatalf(
				"%s: expected unpublished foundation error, got %v",
				stage, err,
			)
		}
	}

	assertNotPublished("draft")

	_, err = env.ReportService.SubmitForReview(
		ctx, nationalAdmin, draft.SnapshotID, &ip,
	)
	if err != nil {
		t.Fatalf("submit foundation report: %v", err)
	}

	_, err = env.ReportService.ApproveSnapshot(
		ctx, superAdmin, draft.SnapshotID,
		"Foundation security test approval", &ip,
	)
	if err != nil {
		t.Fatalf("approve foundation report: %v", err)
	}

	assertNotPublished("approved but unpublished")

	_, err = env.ReportService.PublishSnapshot(
		ctx, nationalAdmin, draft.SnapshotID, &ip,
	)
	if err != service.ErrFinancialGovernanceRole {
		t.Fatalf(
			"expected unauthorized publisher rejection, got %v",
			err,
		)
	}

	assertNotPublished("unauthorized publication attempt")

	_, err = env.ReportService.PublishSnapshot(
		ctx, superAdmin, draft.SnapshotID, &ip,
	)
	if err != nil {
		t.Fatalf("super admin publication: %v", err)
	}

	foundation, err := env.ReportService.GetPublicFoundation(
		ctx, period,
	)
	if err != nil {
		t.Fatalf("published foundation report inaccessible: %v", err)
	}
	if foundation == nil {
		t.Fatal("published foundation response is nil")
	}
	if foundation.ReportingPeriod != period {
		t.Fatalf(
			"wrong reporting period: got %q, want %q",
			foundation.ReportingPeriod, period,
		)
	}

	_, err = env.ReportService.GetPublicFoundation(
		ctx, period+"-other",
	)
	if err != service.ErrReportPublicationNotFound {
		t.Fatalf(
			"different period must not return published foundation data: %v",
			err,
		)
	}

	_, err = env.ReportService.WithdrawPublication(
		ctx, superAdmin, draft.SnapshotID,
		"Foundation publication security regression test", &ip,
	)
	if err != nil {
		t.Fatalf("withdraw foundation publication: %v", err)
	}

	assertNotPublished("withdrawn")
}

func TestReportService_OverviewPublicationSecurity(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()

	ctx := context.Background()
	uid := time.Now().UnixNano()

	superAdmin := createTestUserWithRole(
		t, env, "super_admin",
		fmt.Sprintf("overview_sa_%d@test.com", uid),
	)
	nationalAdmin := createTestUserWithRole(
		t, env, "national_admin",
		fmt.Sprintf("overview_admin_%d@test.com", uid),
	)

	ip := "127.0.0.1"
	period := fmt.Sprintf("overview-security-%d", uid)

	request := service.GenerateReportRequest{
		ReportType:      "national_overview",
		Title:           "Overview Publication Security Test",
		ReportingPeriod: period,
		PeriodStart:     "2026-01-01",
		PeriodEnd:       "2026-12-31",
		ScopeType:       "national",
	}

	draft, err := env.ReportService.GenerateSnapshot(
		ctx, nationalAdmin, request, &ip,
	)
	if err != nil {
		t.Fatalf("generate overview snapshot: %v", err)
	}

	assertNotPublished := func(stage string) {
		t.Helper()

		_, err := env.ReportService.GetPublicOverview(
			ctx, nil, nil, nil, nil, period,
		)
		if err != service.ErrReportPublicationNotFound {
			t.Fatalf(
				"%s: expected unpublished overview error, got %v",
				stage, err,
			)
		}
	}

	assertNotPublished("draft")

	_, err = env.ReportService.SubmitForReview(
		ctx, nationalAdmin, draft.SnapshotID, &ip,
	)
	if err != nil {
		t.Fatalf("submit overview report: %v", err)
	}

	_, err = env.ReportService.ApproveSnapshot(
		ctx, superAdmin, draft.SnapshotID,
		"Overview security test approval", &ip,
	)
	if err != nil {
		t.Fatalf("approve overview report: %v", err)
	}

	assertNotPublished("approved but unpublished")

	_, err = env.ReportService.PublishSnapshot(
		ctx, nationalAdmin, draft.SnapshotID, &ip,
	)
	if err != service.ErrFinancialGovernanceRole {
		t.Fatalf(
			"expected unauthorized publisher rejection, got %v",
			err,
		)
	}

	assertNotPublished("unauthorized publication attempt")

	_, err = env.ReportService.PublishSnapshot(
		ctx, superAdmin, draft.SnapshotID, &ip,
	)
	if err != nil {
		t.Fatalf("super admin publication: %v", err)
	}

	overview, err := env.ReportService.GetPublicOverview(
		ctx, nil, nil, nil, nil, period,
	)
	if err != nil {
		t.Fatalf("published overview inaccessible: %v", err)
	}
	if overview == nil {
		t.Fatal("published overview response is nil")
	}
	if overview.ReportingPeriod != period {
		t.Fatalf(
			"wrong reporting period: got %q, want %q",
			overview.ReportingPeriod, period,
		)
	}

	_, err = env.ReportService.GetPublicOverview(
		ctx, nil, nil, nil, nil, period+"-other",
	)
	if err != service.ErrReportPublicationNotFound {
		t.Fatalf(
			"different period must not return overview data: %v",
			err,
		)
	}

	_, err = env.ReportService.WithdrawPublication(
		ctx, superAdmin, draft.SnapshotID,
		"Overview publication security regression test", &ip,
	)
	if err != nil {
		t.Fatalf("withdraw overview publication: %v", err)
	}

	assertNotPublished("withdrawn")
}

func TestReportService_OverviewStateIsolation(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()

	ctx := context.Background()
	uid := time.Now().UnixNano()

	states, err := env.Repo.Queries().ListStates(ctx)
	if err != nil {
		t.Fatalf("list states: %v", err)
	}
	if len(states) < 2 {
		t.Fatal("at least two states are required")
	}

	stateA := states[0]
	stateB := states[1]

	if stateA.ID == stateB.ID {
		t.Fatal("test states must have different IDs")
	}

	superAdmin := createTestUserWithRole(
		t, env, "super_admin",
		fmt.Sprintf("overview_state_sa_%d@test.com", uid),
	)
	nationalAdmin := createTestUserWithRole(
		t, env, "national_admin",
		fmt.Sprintf("overview_state_admin_%d@test.com", uid),
	)

	ip := "127.0.0.1"
	period := fmt.Sprintf("overview-state-security-%d", uid)
	scope := "state"

	request := service.GenerateReportRequest{
		ReportType:      "national_overview",
		Title:           "Overview State Isolation Test",
		ReportingPeriod: period,
		PeriodStart:     "2026-01-01",
		PeriodEnd:       "2026-12-31",
		ScopeType:       scope,
		StateID:         &stateA.ID,
	}

	draft, err := env.ReportService.GenerateSnapshot(
		ctx, nationalAdmin, request, &ip,
	)
	if err != nil {
		t.Fatalf("generate state overview: %v", err)
	}

	_, err = env.ReportService.GetPublicOverview(
		ctx, &scope, nil, &stateA.ID, nil, period,
	)
	if err != service.ErrReportPublicationNotFound {
		t.Fatalf("draft overview must remain private: %v", err)
	}

	_, err = env.ReportService.SubmitForReview(
		ctx, nationalAdmin, draft.SnapshotID, &ip,
	)
	if err != nil {
		t.Fatalf("submit overview: %v", err)
	}

	_, err = env.ReportService.ApproveSnapshot(
		ctx, superAdmin, draft.SnapshotID,
		"Overview state isolation approval", &ip,
	)
	if err != nil {
		t.Fatalf("approve overview: %v", err)
	}

	_, err = env.ReportService.PublishSnapshot(
		ctx, superAdmin, draft.SnapshotID, &ip,
	)
	if err != nil {
		t.Fatalf("publish overview: %v", err)
	}

	overview, err := env.ReportService.GetPublicOverview(
		ctx, &scope, nil, &stateA.ID, nil, period,
	)
	if err != nil {
		t.Fatalf("correct state cannot retrieve overview: %v", err)
	}
	if overview == nil || overview.ReportingPeriod != period {
		t.Fatalf("unexpected overview response: %+v", overview)
	}

	_, err = env.ReportService.GetPublicOverview(
		ctx, &scope, nil, &stateB.ID, nil, period,
	)
	if err != service.ErrReportPublicationNotFound {
		t.Fatalf("cross-state overview disclosure: %v", err)
	}

	_, err = env.ReportService.GetPublicOverview(
		ctx, nil, nil, nil, nil, period,
	)
	if err != service.ErrReportPublicationNotFound {
		t.Fatalf("state overview leaked into national scope: %v", err)
	}

	_, err = env.ReportService.WithdrawPublication(
		ctx, superAdmin, draft.SnapshotID,
		"Overview state isolation regression test", &ip,
	)
	if err != nil {
		t.Fatalf("withdraw overview: %v", err)
	}

	_, err = env.ReportService.GetPublicOverview(
		ctx, &scope, nil, &stateA.ID, nil, period,
	)
	if err != service.ErrReportPublicationNotFound {
		t.Fatalf("withdrawn overview remains accessible: %v", err)
	}
}

func TestReportService_OverviewZoneAndLGAIsolation(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()

	ctx := context.Background()
	uid := time.Now().UnixNano()

	zones, err := env.Repo.Queries().ListZones(ctx)
	if err != nil || len(zones) < 2 {
		t.Fatalf("at least two zones required: %v", err)
	}

	states, err := env.Repo.Queries().ListStates(ctx)
	if err != nil || len(states) == 0 {
		t.Fatalf("states required: %v", err)
	}

	var lgas []db.Lga
	for _, state := range states {
		candidates, err := env.Repo.Queries().ListLGAsByState(ctx, state.ID)
		if err != nil {
			t.Fatalf("list LGAs: %v", err)
		}
		if len(candidates) >= 2 {
			lgas = candidates
			break
		}
	}
	if len(lgas) < 2 {
		t.Fatal("at least two LGAs in one state required")
	}

	superAdmin := createTestUserWithRole(
		t, env, "super_admin",
		fmt.Sprintf("overview_geo_sa_%d@test.com", uid),
	)
	nationalAdmin := createTestUserWithRole(
		t, env, "national_admin",
		fmt.Sprintf("overview_geo_admin_%d@test.com", uid),
	)

	ip := "127.0.0.1"

	cases := []struct {
		name      string
		scope     string
		zoneID    *string
		stateID   *string
		lgaID     *string
		otherZone *string
		otherLGA  *string
	}{
		{
			name:      "zone",
			scope:     "zone",
			zoneID:    &zones[0].ID,
			otherZone: &zones[1].ID,
		},
		{
			name:     "lga",
			scope:    "lga",
			lgaID:    &lgas[0].ID,
			otherLGA: &lgas[1].ID,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			period := fmt.Sprintf(
				"overview-%s-isolation-%d", tc.name, uid,
			)

			request := service.GenerateReportRequest{
				ReportType:      "national_overview",
				Title:           "Overview Geographic Isolation Test",
				ReportingPeriod: period,
				PeriodStart:     "2026-01-01",
				PeriodEnd:       "2026-12-31",
				ScopeType:       tc.scope,
				ZoneID:          tc.zoneID,
				StateID:         tc.stateID,
				LGAID:           tc.lgaID,
			}

			draft, err := env.ReportService.GenerateSnapshot(
				ctx, nationalAdmin, request, &ip,
			)
			if err != nil {
				t.Fatalf("generate snapshot: %v", err)
			}

			_, err = env.ReportService.SubmitForReview(
				ctx, nationalAdmin, draft.SnapshotID, &ip,
			)
			if err != nil {
				t.Fatalf("submit snapshot: %v", err)
			}

			_, err = env.ReportService.ApproveSnapshot(
				ctx, superAdmin, draft.SnapshotID,
				"Geographic isolation approval", &ip,
			)
			if err != nil {
				t.Fatalf("approve snapshot: %v", err)
			}

			_, err = env.ReportService.PublishSnapshot(
				ctx, superAdmin, draft.SnapshotID, &ip,
			)
			if err != nil {
				t.Fatalf("publish snapshot: %v", err)
			}

			overview, err := env.ReportService.GetPublicOverview(
				ctx, &tc.scope,
				tc.zoneID, tc.stateID, tc.lgaID, period,
			)
			if err != nil || overview == nil {
				t.Fatalf("correct geography inaccessible: %v", err)
			}

			if tc.otherZone != nil {
				_, err = env.ReportService.GetPublicOverview(
					ctx, &tc.scope,
					tc.otherZone, nil, nil, period,
				)
				if err != service.ErrReportPublicationNotFound {
					t.Fatalf("cross-zone disclosure: %v", err)
				}
			}

			if tc.otherLGA != nil {
				_, err = env.ReportService.GetPublicOverview(
					ctx, &tc.scope,
					nil, nil, tc.otherLGA, period,
				)
				if err != service.ErrReportPublicationNotFound {
					t.Fatalf("cross-LGA disclosure: %v", err)
				}
			}

			_, err = env.ReportService.GetPublicOverview(
				ctx, nil, nil, nil, nil, period,
			)
			if err != service.ErrReportPublicationNotFound {
				t.Fatalf("geographic report leaked nationally: %v", err)
			}

			_, err = env.ReportService.WithdrawPublication(
				ctx, superAdmin, draft.SnapshotID,
				"Geographic isolation regression test", &ip,
			)
			if err != nil {
				t.Fatalf("withdraw publication: %v", err)
			}

			_, err = env.ReportService.GetPublicOverview(
				ctx, &tc.scope,
				tc.zoneID, tc.stateID, tc.lgaID, period,
			)
			if err != service.ErrReportPublicationNotFound {
				t.Fatalf("withdrawn report accessible: %v", err)
			}
		})
	}
}

func TestReportService_GeographicScopeValidation(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()

	ctx := context.Background()
	uid := time.Now().UnixNano()
	nationalAdmin := createTestUserWithRole(t, env, "national_admin", fmt.Sprintf("geo_val_admin_%d@test.com", uid))
	ip := "127.0.0.1"

	zones, err := env.Repo.Queries().ListZones(ctx)
	if err != nil || len(zones) < 2 {
		t.Fatalf("at least 2 zones required: %v", err)
	}

	states, err := env.Repo.Queries().ListStates(ctx)
	if err != nil || len(states) < 2 {
		t.Fatalf("at least 2 states required: %v", err)
	}

	var stateZoneA, stateZoneB db.State
	var zoneA, zoneB db.Zone
	for _, sA := range states {
		for _, sB := range states {
			if sA.ZoneID != sB.ZoneID {
				stateZoneA = sA
				stateZoneB = sB
				break
			}
		}
		if stateZoneA.ID != "" {
			break
		}
	}
	if stateZoneA.ID == "" {
		t.Fatal("need two states in different zones for hierarchy test")
	}
	for _, z := range zones {
		if z.ID == stateZoneA.ZoneID {
			zoneA = z
		}
		if z.ID == stateZoneB.ZoneID {
			zoneB = z
		}
	}

	lgasA, err := env.Repo.Queries().ListLGAsByState(ctx, stateZoneA.ID)
	if err != nil || len(lgasA) == 0 {
		t.Fatalf("lgas for stateA missing: %v", err)
	}
	lgaA := lgasA[0]

	baseReq := service.GenerateReportRequest{
		ReportType:      "national_overview",
		Title:           "Scope Validation Test",
		ReportingPeriod: "2026",
		PeriodStart:     "2026-01-01",
		PeriodEnd:       "2026-12-31",
	}

	tests := []struct {
		name      string
		mutate    func(r *service.GenerateReportRequest)
		targetErr error
	}{
		{
			name: "Unsupported scope type",
			mutate: func(r *service.GenerateReportRequest) {
				r.ScopeType = "continent"
			},
			targetErr: service.ErrInvalidReportScope,
		},
		{
			name: "National scope with zone_id",
			mutate: func(r *service.GenerateReportRequest) {
				r.ScopeType = "national"
				r.ZoneID = &zoneA.ID
			},
			targetErr: service.ErrInvalidGeographicFilter,
		},
		{
			name: "National scope with state_id",
			mutate: func(r *service.GenerateReportRequest) {
				r.ScopeType = "national"
				r.StateID = &stateZoneA.ID
			},
			targetErr: service.ErrInvalidGeographicFilter,
		},
		{
			name: "National scope with lga_id",
			mutate: func(r *service.GenerateReportRequest) {
				r.ScopeType = "national"
				r.LGAID = &lgaA.ID
			},
			targetErr: service.ErrInvalidGeographicFilter,
		},
		{
			name: "Zone scope missing zone_id",
			mutate: func(r *service.GenerateReportRequest) {
				r.ScopeType = "zone"
				r.ZoneID = nil
			},
			targetErr: service.ErrInvalidGeographicFilter,
		},
		{
			name: "Zone scope with state_id",
			mutate: func(r *service.GenerateReportRequest) {
				r.ScopeType = "zone"
				r.ZoneID = &zoneA.ID
				r.StateID = &stateZoneA.ID
			},
			targetErr: service.ErrInvalidGeographicFilter,
		},
		{
			name: "Zone scope with lga_id",
			mutate: func(r *service.GenerateReportRequest) {
				r.ScopeType = "zone"
				r.ZoneID = &zoneA.ID
				r.LGAID = &lgaA.ID
			},
			targetErr: service.ErrInvalidGeographicFilter,
		},
		{
			name: "Zone scope non-existent zone_id",
			mutate: func(r *service.GenerateReportRequest) {
				r.ScopeType = "zone"
				badZone := "non-existent-zone-uuid"
				r.ZoneID = &badZone
			},
			targetErr: service.ErrInvalidGeographicFilter,
		},
		{
			name: "State scope missing state_id",
			mutate: func(r *service.GenerateReportRequest) {
				r.ScopeType = "state"
				r.StateID = nil
			},
			targetErr: service.ErrInvalidGeographicFilter,
		},
		{
			name: "State scope with lga_id",
			mutate: func(r *service.GenerateReportRequest) {
				r.ScopeType = "state"
				r.StateID = &stateZoneA.ID
				r.LGAID = &lgaA.ID
			},
			targetErr: service.ErrInvalidGeographicFilter,
		},
		{
			name: "State scope non-existent state_id",
			mutate: func(r *service.GenerateReportRequest) {
				r.ScopeType = "state"
				badState := "non-existent-state-uuid"
				r.StateID = &badState
			},
			targetErr: service.ErrInvalidGeographicFilter,
		},
		{
			name: "State hierarchy mismatch: state does not belong to specified zone",
			mutate: func(r *service.GenerateReportRequest) {
				r.ScopeType = "state"
				r.StateID = &stateZoneA.ID
				r.ZoneID = &zoneB.ID
			},
			targetErr: service.ErrGeographicHierarchyMismatch,
		},
		{
			name: "LGA scope missing lga_id",
			mutate: func(r *service.GenerateReportRequest) {
				r.ScopeType = "lga"
				r.LGAID = nil
			},
			targetErr: service.ErrInvalidGeographicFilter,
		},
		{
			name: "LGA scope non-existent lga_id",
			mutate: func(r *service.GenerateReportRequest) {
				r.ScopeType = "lga"
				badLGA := "non-existent-lga-uuid"
				r.LGAID = &badLGA
			},
			targetErr: service.ErrInvalidGeographicFilter,
		},
		{
			name: "LGA hierarchy mismatch: lga does not belong to specified state",
			mutate: func(r *service.GenerateReportRequest) {
				r.ScopeType = "lga"
				r.LGAID = &lgaA.ID
				r.StateID = &stateZoneB.ID
			},
			targetErr: service.ErrGeographicHierarchyMismatch,
		},
		{
			name: "Foundation report rejects LGA scope",
			mutate: func(r *service.GenerateReportRequest) {
				r.ReportType = "foundation"
				r.ScopeType = "lga"
				r.LGAID = &lgaA.ID
			},
			targetErr: service.ErrUnsupportedScopeForType,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := baseReq
			tc.mutate(&req)
			_, err := env.ReportService.GenerateSnapshot(ctx, nationalAdmin, req, &ip)
			if err == nil {
				t.Fatalf("expected error wrapping %v, got nil", tc.targetErr)
			}
			if !errors.Is(err, tc.targetErr) {
				t.Fatalf("expected error wrapping %v, got: %v", tc.targetErr, err)
			}
		})
	}
}

func TestReportService_GeographicAggregationIntegrity(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()

	ctx := context.Background()
	uid := time.Now().UnixNano()

	coordinator := createTestUserWithRole(t, env, "coordinator", fmt.Sprintf("geo_coord_%d@test.com", uid))

	states, err := env.Repo.Queries().ListStates(ctx)
	if err != nil || len(states) < 2 {
		t.Fatalf("at least 2 states required: %v", err)
	}

	var stateA, stateB db.State
	for _, sA := range states {
		for _, sB := range states {
			if sA.ZoneID != sB.ZoneID {
				stateA = sA
				stateB = sB
				break
			}
		}
		if stateA.ID != "" {
			break
		}
	}
	if stateA.ID == "" {
		t.Fatal("need two states in different zones")
	}

	lgasA, err := env.Repo.Queries().ListLGAsByState(ctx, stateA.ID)
	if err != nil || len(lgasA) < 2 {
		t.Fatalf("at least 2 lgas in stateA required: %v", err)
	}
	lgaA1 := lgasA[0]
	lgaA2 := lgasA[1]

	lgasB, err := env.Repo.Queries().ListLGAsByState(ctx, stateB.ID)
	if err != nil || len(lgasB) == 0 {
		t.Fatalf("lgas in stateB required: %v", err)
	}
	lgaB1 := lgasB[0]

	zoneA := stateA.ZoneID
	zoneB := stateB.ZoneID

	// Create test events in a dedicated unique future window to avoid interference
	baseYear := 2040 + int((uid/1000)%500)
	tStart := time.Date(baseYear, 5, 1, 0, 0, 0, 0, time.UTC)
	tEnd := time.Date(baseYear, 5, 31, 23, 59, 59, 0, time.UTC)
	eventTimeA := time.Date(baseYear, 5, 10, 8, 0, 0, 0, time.UTC)
	eventTimeB := time.Date(baseYear, 5, 20, 8, 0, 0, 0, time.UTC)

	campaignID := security.GenerateUUID()
	_, _ = env.Pool.Exec(ctx, "DELETE FROM event_instances WHERE campaign_id IN (SELECT id FROM cleanup_campaigns WHERE year = $1 AND month = 5)", baseYear)
	_, _ = env.Pool.Exec(ctx, "DELETE FROM cleanup_campaigns WHERE year = $1 AND month = 5", baseYear)
	defer func() {
		_, _ = env.Pool.Exec(ctx, "DELETE FROM event_instances WHERE campaign_id = $1", campaignID)
		_, _ = env.Pool.Exec(ctx, "DELETE FROM cleanup_campaigns WHERE id = $1", campaignID)
	}()

	_, err = env.Pool.Exec(ctx, `
		INSERT INTO cleanup_campaigns (id, year, month, scheduled_date, title, status)
		VALUES ($1, $2, 5, $3, 'Future Clean Campaign', 'scheduled')
	`, campaignID, baseYear, eventTimeA)
	if err != nil {
		t.Fatalf("create campaign: %v", err)
	}

	eventIDA := security.GenerateUUID()
	eventIDB := security.GenerateUUID()

	// Event A in Zone A, State A, LGA A1: 55 kg waste
	_, err = env.Pool.Exec(ctx, `
		INSERT INTO event_instances (
			id, campaign_id, programme_type, title, state_id, lga_id,
			address, landmark, start_time, end_time, capacity, registered_count,
			coordinator_id, public_contact, status, waste_collected_kg, waste_bags_count, cleanup_duration_hours
		) VALUES (
			$1, $2, 'cleanup', 'Zone A Event', $3, $4,
			'Zone A Address', 'Landmark A', $5, $6, 100, 10,
			$7, '08000000000', 'completed', 55, 10, 3.0
		)
	`, eventIDA, campaignID, stateA.ID, lgaA1.ID, eventTimeA, eventTimeA.Add(3*time.Hour), coordinator.ID)
	if err != nil {
		t.Fatalf("create event A: %v", err)
	}

	// Event B in Zone B, State B, LGA B1: 145 kg waste
	_, err = env.Pool.Exec(ctx, `
		INSERT INTO event_instances (
			id, campaign_id, programme_type, title, state_id, lga_id,
			address, landmark, start_time, end_time, capacity, registered_count,
			coordinator_id, public_contact, status, waste_collected_kg, waste_bags_count, cleanup_duration_hours
		) VALUES (
			$1, $2, 'cleanup', 'Zone B Event', $3, $4,
			'Zone B Address', 'Landmark B', $5, $6, 100, 20,
			$7, '08000000001', 'completed', 145, 25, 4.0
		)
	`, eventIDB, campaignID, stateB.ID, lgaB1.ID, eventTimeB, eventTimeB.Add(4*time.Hour), coordinator.ID)
	if err != nil {
		t.Fatalf("create event B: %v", err)
	}

	// 1. Verify Cleanups Aggregation by Zone
	rawZoneA, err := env.ReportService.AggregateSourceData(ctx, "cleanups", "zone", &zoneA, nil, nil, tStart, tEnd, "May 2028")
	if err != nil {
		t.Fatalf("aggregate cleanups zone A: %v", err)
	}
	var dataZoneA service.PublicCleanupsData
	_ = json.Unmarshal(rawZoneA, &dataZoneA)
	if dataZoneA.TotalWasteCollectedKg != 55 {
		t.Errorf("Zone A waste: expected 55 kg, got %d kg", dataZoneA.TotalWasteCollectedKg)
	}
	if dataZoneA.CompletedCleanupsCount != 1 {
		t.Errorf("Zone A cleanups count: expected 1, got %d", dataZoneA.CompletedCleanupsCount)
	}

	rawZoneB, err := env.ReportService.AggregateSourceData(ctx, "cleanups", "zone", &zoneB, nil, nil, tStart, tEnd, "May 2028")
	if err != nil {
		t.Fatalf("aggregate cleanups zone B: %v", err)
	}
	var dataZoneB service.PublicCleanupsData
	_ = json.Unmarshal(rawZoneB, &dataZoneB)
	if dataZoneB.TotalWasteCollectedKg != 145 {
		t.Errorf("Zone B waste: expected 145 kg, got %d kg", dataZoneB.TotalWasteCollectedKg)
	}
	if dataZoneB.CompletedCleanupsCount != 1 {
		t.Errorf("Zone B cleanups count: expected 1, got %d", dataZoneB.CompletedCleanupsCount)
	}

	// 2. Verify Cleanups Aggregation by State
	rawStateA, err := env.ReportService.AggregateSourceData(ctx, "cleanups", "state", nil, &stateA.ID, nil, tStart, tEnd, "May 2028")
	if err != nil {
		t.Fatalf("aggregate cleanups state A: %v", err)
	}
	var dataStateA service.PublicCleanupsData
	_ = json.Unmarshal(rawStateA, &dataStateA)
	if dataStateA.TotalWasteCollectedKg != 55 {
		t.Errorf("State A waste: expected 55 kg, got %d kg", dataStateA.TotalWasteCollectedKg)
	}

	// 3. Verify Cleanups Aggregation by LGA
	rawLgaA1, err := env.ReportService.AggregateSourceData(ctx, "cleanups", "lga", nil, nil, &lgaA1.ID, tStart, tEnd, "May 2028")
	if err != nil {
		t.Fatalf("aggregate cleanups lga A1: %v", err)
	}
	var dataLgaA1 service.PublicCleanupsData
	_ = json.Unmarshal(rawLgaA1, &dataLgaA1)
	if dataLgaA1.TotalWasteCollectedKg != 55 {
		t.Errorf("LGA A1 waste: expected 55 kg, got %d kg", dataLgaA1.TotalWasteCollectedKg)
	}

	// LGA A2 has no events -> must be 0
	rawLgaA2, err := env.ReportService.AggregateSourceData(ctx, "cleanups", "lga", nil, nil, &lgaA2.ID, tStart, tEnd, "May 2028")
	if err != nil {
		t.Fatalf("aggregate cleanups lga A2: %v", err)
	}
	var dataLgaA2 service.PublicCleanupsData
	_ = json.Unmarshal(rawLgaA2, &dataLgaA2)
	if dataLgaA2.TotalWasteCollectedKg != 0 || dataLgaA2.CompletedCleanupsCount != 0 {
		t.Errorf("LGA A2 waste: expected 0, got %d kg, %d cleanups", dataLgaA2.TotalWasteCollectedKg, dataLgaA2.CompletedCleanupsCount)
	}

	// 4. Verify Foundation Donation Aggregation
	// Insert foundation donations: 250,000 kobo in Zone A (state A), 750,000 kobo in Zone B (state B)
	donIDA := security.GenerateUUID()
	donIDB := security.GenerateUUID()
	_, err = env.Pool.Exec(ctx, `
		INSERT INTO financial_donations (
			id, reference, entity_type, donor_name, donor_email, amount_kobo,
			currency, status, zone_id, state_id, destination_fund_id, created_at, verified_at
		) VALUES
		($1, $2, 'foundation', 'Donor Zone A', 'donora@example.com', 250000, 'NGN', 'verified', $3, $4, 'foundation_donation_fund', $5, $5),
		($6, $7, 'foundation', 'Donor Zone B', 'donorb@example.com', 750000, 'NGN', 'verified', $8, $9, 'foundation_donation_fund', $5, $5)
	`, donIDA, "REF-DON-A-"+security.GenerateUUID()[:6], zoneA, stateA.ID, eventTimeA,
		donIDB, "REF-DON-B-"+security.GenerateUUID()[:6], zoneB, stateB.ID)
	if err != nil {
		t.Fatalf("seed donations: %v", err)
	}

	rawFoundZoneA, err := env.ReportService.AggregateSourceData(ctx, "foundation", "zone", &zoneA, nil, nil, tStart, tEnd, "May 2028")
	if err != nil {
		t.Fatalf("aggregate foundation zone A: %v", err)
	}
	var dataFoundA service.PublicFoundationData
	_ = json.Unmarshal(rawFoundZoneA, &dataFoundA)
	if dataFoundA.TotalDonationsKobo != 250000 {
		t.Errorf("Foundation Zone A: expected 250,000 kobo, got %d", dataFoundA.TotalDonationsKobo)
	}

	rawFoundZoneB, err := env.ReportService.AggregateSourceData(ctx, "foundation", "zone", &zoneB, nil, nil, tStart, tEnd, "May 2028")
	if err != nil {
		t.Fatalf("aggregate foundation zone B: %v", err)
	}
	var dataFoundB service.PublicFoundationData
	_ = json.Unmarshal(rawFoundZoneB, &dataFoundB)
	if dataFoundB.TotalDonationsKobo != 750000 {
		t.Errorf("Foundation Zone B: expected 750,000 kobo, got %d", dataFoundB.TotalDonationsKobo)
	}

	// 5. Verify LGA Overview suppresses Foundation Donations (does not leak nationwide totals)
	rawOverviewLGA, err := env.ReportService.AggregateSourceData(ctx, "national_overview", "lga", nil, nil, &lgaA1.ID, tStart, tEnd, "May 2028")
	if err != nil {
		t.Fatalf("aggregate overview lga: %v", err)
	}
	var dataOverviewLGA service.PublicOverviewData
	_ = json.Unmarshal(rawOverviewLGA, &dataOverviewLGA)
	if dataOverviewLGA.FoundationDonationsKobo != 0 {
		t.Errorf("LGA overview should have 0 kobo foundation donations, got %d", dataOverviewLGA.FoundationDonationsKobo)
	}
	if dataOverviewLGA.CompletedCleanupsCount != 1 {
		t.Errorf("LGA overview cleanups count: expected 1, got %d", dataOverviewLGA.CompletedCleanupsCount)
	}
}
