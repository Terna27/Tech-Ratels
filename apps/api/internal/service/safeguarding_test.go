package service_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"ratels-api/internal/service"
)


func TestSafeguardingService_AnonymousReportSubmission(t *testing.T) {
	env := setupTestEnv(t)
	svc := env.SafeguardingService
	ctx := context.Background()

	res, err := svc.SubmitReport(ctx, service.SubmitReportInput{
		Category:          "harassment",
		Description:       "Incident occurred during community cleanup outreach where volunteer was repeatedly insulted.",
		IsImmediateDanger: false,
		IsMinorInvolved:   false,
		IsAnonymous:       true,
	})
	if err != nil {
		t.Fatalf("unexpected error submitting anonymous report: %v", err)
	}

	if !strings.HasPrefix(res.PublicReference, "CASE-2026-") {
		t.Errorf("expected reference starting with CASE-2026-, got: %s", res.PublicReference)
	}
	if !strings.HasPrefix(res.FollowUpSecret, "sec_") {
		t.Errorf("expected follow-up secret starting with sec_, got: %s", res.FollowUpSecret)
	}
	if res.Status != "submitted" {
		t.Errorf("expected status 'submitted', got: %s", res.Status)
	}
	if res.RiskLevel != "normal" {
		t.Errorf("expected risk level 'normal', got: %s", res.RiskLevel)
	}

	// Verify narrative is encrypted in database
	dbCase, err := env.Repo.Queries().GetSafeguardingCaseByID(ctx, res.CaseID)
	if err != nil {
		t.Fatalf("failed to fetch case from DB: %v", err)
	}
	if !strings.HasPrefix(dbCase.EncryptedDescription, "v1:") {
		t.Errorf("expected encrypted description to start with v1:, got: %s", dbCase.EncryptedDescription)
	}
	if strings.Contains(dbCase.EncryptedDescription, "repeatedly insulted") {
		t.Error("plaintext narrative was found in database encrypted_description column!")
	}
}

func TestSafeguardingService_MinorAndImmediateDangerTriage(t *testing.T) {
	env := setupTestEnv(t)
	svc := env.SafeguardingService
	ctx := context.Background()

	// 1. Immediate danger -> Critical
	dangerRes, err := svc.SubmitReport(ctx, service.SubmitReportInput{
		Category:          "threats_intimidation",
		Description:       "Immediate physical threat made towards participants at local warehouse.",
		IsImmediateDanger: true,
		IsMinorInvolved:   false,
		IsAnonymous:       true,
	})
	if err != nil {
		t.Fatalf("submission failed: %v", err)
	}
	if dangerRes.RiskLevel != "critical" {
		t.Errorf("expected risk level 'critical' for immediate danger, got: %s", dangerRes.RiskLevel)
	}

	// 2. Minor involved -> High
	minorRes, err := svc.SubmitReport(ctx, service.SubmitReportInput{
		Category:          "child_safeguarding",
		Description:       "Concern regarding unaccompanied minors participating without parental consent.",
		IsImmediateDanger: false,
		IsMinorInvolved:   true,
		IsAnonymous:       true,
	})
	if err != nil {
		t.Fatalf("submission failed: %v", err)
	}
	if minorRes.RiskLevel != "high" {
		t.Errorf("expected risk level 'high' for minor involved, got: %s", minorRes.RiskLevel)
	}
}

func TestSafeguardingService_FollowUpStatusAndMessaging(t *testing.T) {
	env := setupTestEnv(t)
	svc := env.SafeguardingService
	ctx := context.Background()

	res, err := svc.SubmitReport(ctx, service.SubmitReportInput{
		Category:    "bullying",
		Description: "Workplace bullying by squad leader in zonal logistics center.",
		IsAnonymous: true,
	})
	if err != nil {
		t.Fatalf("submission failed: %v", err)
	}

	// Valid lookup
	statusView, err := svc.GetReportStatus(ctx, res.PublicReference, res.FollowUpSecret)
	if err != nil {
		t.Fatalf("status lookup failed: %v", err)
	}
	if statusView.PublicReference != res.PublicReference {
		t.Errorf("expected ref %s, got %s", res.PublicReference, statusView.PublicReference)
	}
	if statusView.Status != "submitted" {
		t.Errorf("expected status 'submitted', got %s", statusView.Status)
	}

	// Invalid secret lookup
	_, err = svc.GetReportStatus(ctx, res.PublicReference, "sec_invalid_secret_12345")
	if err != service.ErrInvalidFollowUpCredentials {
		t.Errorf("expected ErrInvalidFollowUpCredentials, got: %v", err)
	}

	// Post message from reporter
	err = svc.PostReporterMessage(ctx, res.PublicReference, res.FollowUpSecret, "I have additional witness details to share.")
	if err != nil {
		t.Fatalf("failed to post reporter message: %v", err)
	}

	// Verify message in status view
	statusView2, err := svc.GetReportStatus(ctx, res.PublicReference, res.FollowUpSecret)
	if err != nil {
		t.Fatalf("status lookup failed: %v", err)
	}
	if len(statusView2.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(statusView2.Messages))
	}
	if statusView2.Messages[0].Content != "I have additional witness details to share." {
		t.Errorf("unexpected message content: %s", statusView2.Messages[0].Content)
	}
}

func TestSafeguardingService_RBACAndConflictOfInterest(t *testing.T) {
	env := setupTestEnv(t)
	svc := env.SafeguardingService
	ctx := context.Background()

	adminUser := createTestUser(t, env, fmt.Sprintf("sa_%d@example.com", time.Now().UnixNano()), "super_admin")
	regularUser := createTestUser(t, env, fmt.Sprintf("reg_user_%d@example.com", time.Now().UnixNano()), "member")

	// 2. Create a designated safeguarding officer user
	officerUser := createTestUser(t, env, fmt.Sprintf("officer_%d@example.com", time.Now().UnixNano()), "member")
	if err := svc.AppointOfficer(ctx, adminUser, officerUser.ID, "National Safeguarding Lead", true, true, true, true); err != nil {
		t.Fatalf("failed to appoint officer: %v", err)
	}

	// Submit a report
	subRes, err := svc.SubmitReport(ctx, service.SubmitReportInput{
		Category:      "official_misconduct",
		Description:   "Allegations of fund diversion and verbal abuse by coordinator.",
		IsAnonymous:   false,
		ReporterName:  strPtr("Anonymous Comrade"),
		ReporterEmail: strPtr("whistleblower@example.com"),
	})
	if err != nil {
		t.Fatalf("submission failed: %v", err)
	}

	// Regular user attempt to access case details must be denied
	_, err = svc.GetCaseDetails(ctx, regularUser, subRes.CaseID, "Investigation curiosity", "127.0.0.1")
	if err != service.ErrUnauthorizedSafeguardingAccess {
		t.Errorf("expected ErrUnauthorizedSafeguardingAccess for regular user, got: %v", err)
	}

	// Authorized officer can access details
	details, err := svc.GetCaseDetails(ctx, officerUser, subRes.CaseID, "Initial triage inspection", "127.0.0.1")
	if err != nil {
		t.Fatalf("expected authorized officer to view details, got: %v", err)
	}
	if details.Category != "official_misconduct" {
		t.Errorf("unexpected category: %s", details.Category)
	}
	if details.DecryptedDescription != "Allegations of fund diversion and verbal abuse by coordinator." {
		t.Errorf("decrypted narrative mismatch: %s", details.DecryptedDescription)
	}

	// 4. Declare conflict of interest for officerUser on this case
	err = svc.DeclareConflict(ctx, adminUser, subRes.CaseID, officerUser.ID, "named_in_complaint", "Officer is acquainted with implicated party")
	if err != nil {
		t.Fatalf("failed to declare conflict: %v", err)
	}

	// Once conflict is declared, officerUser MUST BE BLOCKED
	_, err = svc.GetCaseDetails(ctx, officerUser, subRes.CaseID, "Attempted review", "127.0.0.1")
	if err != service.ErrConflictOfInterest {
		t.Errorf("expected ErrConflictOfInterest for conflicted officer, got: %v", err)
	}

	// Officer cannot be assigned to this case
	err = svc.AssignOfficer(ctx, adminUser, subRes.CaseID, officerUser.ID, "Routine assignment")
	if err != service.ErrOfficerHasConflict {
		t.Errorf("expected ErrOfficerHasConflict, got: %v", err)
	}
}

func TestSafeguardingService_InvestigationLifecycle(t *testing.T) {
	env := setupTestEnv(t)
	svc := env.SafeguardingService
	ctx := context.Background()

	adminUser := createTestUser(t, env, fmt.Sprintf("sa_%d@example.com", time.Now().UnixNano()), "super_admin")
	investigator := createTestUser(t, env, fmt.Sprintf("investigator_%d@example.com", time.Now().UnixNano()), "member")
	if err := svc.AppointOfficer(ctx, adminUser, investigator.ID, "Lead Investigator", false, true, true, false); err != nil {
		t.Fatalf("failed to appoint investigator: %v", err)
	}

	subRes, err := svc.SubmitReport(ctx, service.SubmitReportInput{
		Category:    "privacy_violation",
		Description: "Member phone numbers shared without consent in public group chat.",
		IsAnonymous: true,
	})
	if err != nil {
		t.Fatalf("submission failed: %v", err)
	}

	// 1. Update triage
	err = svc.UpdateTriage(ctx, adminUser, subRes.CaseID, "high", "under_review", "Elevated due to bulk contact disclosure")
	if err != nil {
		t.Fatalf("triage update failed: %v", err)
	}

	// 2. Assign investigator
	err = svc.AssignOfficer(ctx, adminUser, subRes.CaseID, investigator.ID, "Assigned to digital privacy lead")
	if err != nil {
		t.Fatalf("assignment failed: %v", err)
	}

	// 3. Add internal note
	err = svc.AddInternalNote(ctx, investigator, subRes.CaseID, "Contacted group admin to delete the offending message.")
	if err != nil {
		t.Fatalf("internal note failed: %v", err)
	}

	// 4. Send reporter update
	err = svc.SendReporterUpdate(ctx, investigator, subRes.CaseID, "We are actively investigating this report and containment measures are underway.")
	if err != nil {
		t.Fatalf("reporter update failed: %v", err)
	}

	// 5. Verify reporter can see reporter update but NOT internal note
	repStatus, err := svc.GetReportStatus(ctx, subRes.PublicReference, subRes.FollowUpSecret)
	if err != nil {
		t.Fatalf("reporter status failed: %v", err)
	}
	if len(repStatus.Messages) != 1 {
		t.Fatalf("expected 1 reporter-visible message, got %d", len(repStatus.Messages))
	}
	if !strings.Contains(repStatus.Messages[0].Content, "containment measures") {
		t.Errorf("unexpected message content: %s", repStatus.Messages[0].Content)
	}

	// 6. Resolve case
	err = svc.ResolveCase(ctx, investigator, subRes.CaseID, "Group admin disciplined; member numbers removed; privacy warning issued.")
	if err != nil {
		t.Fatalf("resolution failed: %v", err)
	}

	// 7. Close case
	err = svc.CloseCase(ctx, adminUser, subRes.CaseID)
	if err != nil {
		t.Fatalf("case closure failed: %v", err)
	}

	// 8. Place legal hold
	err = svc.SetLegalHold(ctx, adminUser, subRes.CaseID, true, "Retained pending compliance review by legal counsel")
	if err != nil {
		t.Fatalf("legal hold failed: %v", err)
	}

	// Verify final details
	details, err := svc.GetCaseDetails(ctx, adminUser, subRes.CaseID, "Final audit", "127.0.0.1")
	if err != nil {
		t.Fatalf("failed to get final details: %v", err)
	}
	if details.Status != "closed" {
		t.Errorf("expected status 'closed', got: %s", details.Status)
	}
	if details.Retention == nil || !details.Retention.IsLegalHold {
		t.Error("expected legal hold to be true")
	}
}

func TestSafeguardingService_ReportingDisabledFailSafe(t *testing.T) {
	env := setupTestEnv(t)
	svc := env.SafeguardingService
	ctx := context.Background()

	// Temporarily disable reporting feature flag
	env.Config.SafeguardingReportingEnabled = false

	_, err := svc.SubmitReport(ctx, service.SubmitReportInput{
		Category:    "harassment",
		Description: "Should be rejected because reporting is disabled",
		IsAnonymous: true,
	})
	if err != service.ErrSafeguardingDisabled {
		t.Errorf("expected ErrSafeguardingDisabled when flag is false, got: %v", err)
	}
}
