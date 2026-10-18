package service_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"ratels-api/internal/service"
)

func TestCommunityProjectsAndRequests_EndToEnd(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	// 1. Get States and LGAs
	states, err := env.GeoService.ListStates(ctx)
	if err != nil || len(states) < 2 {
		t.Fatalf("Expected at least 2 states in test database, got %v", err)
	}
	stateA := states[0] // e.g. Lagos
	stateB := states[1] // e.g. Kano

	lgasA, err := env.GeoService.ListLGAsByState(ctx, stateA.ID)
	if err != nil || len(lgasA) == 0 {
		t.Fatalf("Expected LGAs for stateA, got %v", err)
	}
	lgaA := lgasA[0]

	// 2. Create Users:
	// - Member A (requester)
	// - Member B (another regular member)
	// - State Coordinator A (for stateA)
	// - State Coordinator B (for stateB)
	// - Super Admin
	timeSuffix := time.Now().UnixNano()

	memberAUser, err := env.AuthService.Register(ctx, service.RegisterRequest{
		Email:       fmt.Sprintf("member_a_%d@example.org", timeSuffix),
		Password:    "Password123!",
		DisplayName: "Community Volunteer Musa",
	})
	if err != nil {
		t.Fatalf("Failed to create member A: %v", err)
	}

	memberBUser, err := env.AuthService.Register(ctx, service.RegisterRequest{
		Email:       fmt.Sprintf("member_b_%d@example.org", timeSuffix),
		Password:    "Password123!",
		DisplayName: "Other Member Ngozi",
	})
	if err != nil {
		t.Fatalf("Failed to create member B: %v", err)
	}

	coordAUser, err := env.AuthService.Register(ctx, service.RegisterRequest{
		Email:       fmt.Sprintf("coord_a_%d@example.org", timeSuffix),
		Password:    "Password123!",
		DisplayName: "State Coordinator Adebayo",
	})
	if err != nil {
		t.Fatalf("Failed to create coordinator A: %v", err)
	}

	coordBUser, err := env.AuthService.Register(ctx, service.RegisterRequest{
		Email:       fmt.Sprintf("coord_b_%d@example.org", timeSuffix),
		Password:    "Password123!",
		DisplayName: "State Coordinator Danjuma",
	})
	if err != nil {
		t.Fatalf("Failed to create coordinator B: %v", err)
	}

	superAdminUser, err := env.AuthService.Register(ctx, service.RegisterRequest{
		Email:       fmt.Sprintf("super_admin_%d@example.org", timeSuffix),
		Password:    "Password123!",
		DisplayName: "National Overseer Martins",
	})
	if err != nil {
		t.Fatalf("Failed to create super admin: %v", err)
	}

	// -------------------------------------------------------------
	// Test Step 1: Member Creates Community Request
	// -------------------------------------------------------------
	reqInput := service.CreateCommunityRequestInput{
		Title:             "Urgent Desilting and Drainage Repair at Oshodi Market",
		Description:       "Flooding occurs every rainfall leading to environmental health risks. Community youth mobilize to assist Ratels sanitation unit.",
		Category:          "drainage_and_flooding",
		StateID:           stateA.ID,
		LGAID:             lgaA.ID,
		CommunityWard:     "Ward 04 - Central Market",
		LocationDetails:   "Along Oshodi Expressway junction by Market Gate 2",
		EstimatedUrgency:  "high",
		EvidenceImageURLs: []string{"https://pub-82990f82f02446b5a11817fdce96912d.r2.dev/community/requests/mock1.jpg"},
	}

	createdReq, err := env.CommunityProjectService.CreateRequest(ctx, memberAUser.UserID, reqInput)
	if err != nil {
		t.Fatalf("Member A failed to create request: %v", err)
	}
	if createdReq.Reference == "" || createdReq.Status != "submitted" {
		t.Fatalf("Expected status 'submitted' and valid reference, got %s and %s", createdReq.Status, createdReq.Reference)
	}

	// Member A can fetch their request
	myReqs, err := env.CommunityProjectService.ListMemberRequests(ctx, memberAUser.UserID)
	if err != nil || len(myReqs) == 0 {
		t.Fatalf("Member A should see their submitted request, got %v", err)
	}

	// Member B CANNOT view Member A's private request
	_, err = env.CommunityProjectService.GetRequestByID(ctx, memberBUser.UserID, "member", nil, createdReq.ID)
	if err != service.ErrCommunityRequestNotFound {
		t.Fatalf("Expected ErrCommunityRequestNotFound when Member B accesses Member A's request, got %v", err)
	}

	// -------------------------------------------------------------
	// Test Step 2: Anti-Self-Approval Enforcement
	// -------------------------------------------------------------
	// If Member A is somehow in a review context and tries to review their own request:
	_, err = env.CommunityProjectService.ReviewRequest(ctx, memberAUser.UserID, "national_admin", nil, createdReq.ID, service.ReviewRequestInput{
		Decision: "approve",
	})
	if err != service.ErrSelfApprovalNotAllowed {
		t.Fatalf("Expected ErrSelfApprovalNotAllowed when requester attempts to approve their own request, got %v", err)
	}

	// -------------------------------------------------------------
	// Test Step 3: Geographic Scoping Enforcement
	// -------------------------------------------------------------
	// Coordinator B (assigned to stateB) tries to review stateA request
	_, err = env.CommunityProjectService.ReviewRequest(ctx, coordBUser.UserID, "state_coordinator", &stateB.ID, createdReq.ID, service.ReviewRequestInput{
		Decision: "approve",
	})
	if err != service.ErrScopeUnauthorized {
		t.Fatalf("Expected ErrScopeUnauthorized when Coordinator B reviews request in state A, got %v", err)
	}

	// Coordinator A requests more information
	notes := "Please clarify the length of the drain channel in meters and add another photo."
	reviewedReq, err := env.CommunityProjectService.ReviewRequest(ctx, coordAUser.UserID, "state_coordinator", &stateA.ID, createdReq.ID, service.ReviewRequestInput{
		Decision:         "request_info",
		InfoRequestNotes: &notes,
	})
	if err != nil {
		t.Fatalf("Coordinator A review failed: %v", err)
	}
	if reviewedReq.Status != "info_requested" {
		t.Fatalf("Expected status 'info_requested', got %s", reviewedReq.Status)
	}

	// Member A resubmits with additional details
	resubmitted, err := env.CommunityProjectService.ResubmitRequest(ctx, memberAUser.UserID, createdReq.ID, service.ResubmitRequestInput{
		Description:       "Drainage channel measures approximately 250 meters. Full community clearance planned.",
		LocationDetails:   "Oshodi Market Gate 2, extending to Railway crossing",
		EvidenceImageURLs: []string{"https://pub-82990f82f02446b5a11817fdce96912d.r2.dev/community/requests/mock1.jpg", "https://pub-82990f82f02446b5a11817fdce96912d.r2.dev/community/requests/mock2.jpg"},
	})
	if err != nil {
		t.Fatalf("Resubmission failed: %v", err)
	}
	if resubmitted.Status != "submitted" {
		t.Fatalf("Expected status 'submitted' after resubmission, got %s", resubmitted.Status)
	}

	// Coordinator A approves request
	approvedNotes := "Verified on ground with Oshodi market leadership. Urgent drainage clearing approved."
	approvedReq, err := env.CommunityProjectService.ReviewRequest(ctx, coordAUser.UserID, "state_coordinator", &stateA.ID, createdReq.ID, service.ReviewRequestInput{
		Decision:    "approve",
		ReviewNotes: &approvedNotes,
	})
	if err != nil {
		t.Fatalf("Approval failed: %v", err)
	}
	if approvedReq.Status != "approved" {
		t.Fatalf("Expected status 'approved', got %s", approvedReq.Status)
	}

	// -------------------------------------------------------------
	// Test Step 4: Community Project Conversion & Duplicate Prevention
	// -------------------------------------------------------------
	startDate := time.Now().Add(24 * time.Hour)
	targetDate := time.Now().Add(14 * 24 * time.Hour)
	projectInput := service.CreateProjectInput{
		CommunityRequestID:    &approvedReq.ID,
		Title:                 "Oshodi Market 250m Drainage Desilting Project",
		Description:           "Clear silt and blockages across 250m channel to prevent seasonal market flooding.",
		Category:              approvedReq.Category,
		StateID:               stateA.ID,
		LGAID:                 lgaA.ID,
		CommunityWard:         approvedReq.CommunityWard,
		LocationDetails:       approvedReq.LocationDetails,
		ResponsibleOfficerID:  &coordAUser.UserID,
		ResponsibleDepartment: "Sanitation & Flood Prevention",
		StartDate:             &startDate,
		TargetCompletionDate:  &targetDate,
		ProposedBudgetKobo:    150000000, // 1,500,000 NGN in Kobo
		ApprovedBudgetKobo:    120000000, // 1,200,000 NGN in Kobo
		IsPublic:              true,
	}

	project, err := env.CommunityProjectService.CreateProject(ctx, coordAUser.UserID, "state_coordinator", &stateA.ID, projectInput)
	if err != nil {
		t.Fatalf("Failed to convert request to project: %v", err)
	}
	if project.Reference == "" || project.ExecutionStatus != "planned" {
		t.Fatalf("Expected reference and status 'planned', got %s, %s", project.Reference, project.ExecutionStatus)
	}

	// Originating request converted_to_project_id must point to project
	updatedReq, err := env.CommunityProjectService.GetRequestByID(ctx, coordAUser.UserID, "state_coordinator", &stateA.ID, approvedReq.ID)
	if err != nil || !updatedReq.ConvertedToProjectID.Valid || updatedReq.ConvertedToProjectID.String != project.ID {
		t.Fatalf("Expected request to link to project ID %s, got %+v", project.ID, updatedReq)
	}

	// STRICT DUPLICATE CONVERSION PREVENTION: Attempting to convert again MUST fail
	_, err = env.CommunityProjectService.CreateProject(ctx, coordAUser.UserID, "state_coordinator", &stateA.ID, projectInput)
	if err != service.ErrDuplicateProjectConversion {
		t.Fatalf("Expected ErrDuplicateProjectConversion when converting the same request twice, got %v", err)
	}

	// -------------------------------------------------------------
	// Test Step 5: Milestones & Strict Completion Gate
	// -------------------------------------------------------------
	coordAName := "State Coordinator Adebayo"
	m1Date := time.Now().Add(5 * 24 * time.Hour)
	milestone, err := env.CommunityProjectService.AddMilestone(ctx, coordAUser.UserID, "state_coordinator", &stateA.ID, project.ID, service.AddMilestoneInput{
		SequenceOrder:      1,
		Title:              "Initial Excavation and Trash Clearance",
		Description:        "Clear refuse and plastic waste from first 100 meters of the drainage ditch.",
		TargetDate:         m1Date,
		AssignedPersonName: &coordAName,
		AssignedPersonID:   &coordAUser.UserID,
		Status:             "in_progress",
	})
	if err != nil {
		t.Fatalf("Failed to add milestone: %v", err)
	}

	// Mark milestone as completed by assigned officer
	now := time.Now()
	notesProgress := "First 100m channel excavated and trash bagged."
	_, err = env.CommunityProjectService.UpdateMilestone(ctx, coordAUser.UserID, "state_coordinator", &stateA.ID, milestone.ID, service.UpdateMilestoneInput{
		SequenceOrder:      1,
		Title:              milestone.Title,
		Description:        milestone.Description,
		TargetDate:         m1Date,
		CompletionDate:     &now,
		AssignedPersonName: &coordAName,
		AssignedPersonID:   &coordAUser.UserID,
		Status:             "completed",
		ProgressNotes:      &notesProgress,
		EvidencePhotoURLs:  []string{"https://pub-82990f82f02446b5a11817fdce96912d.r2.dev/community/projects/ev1.jpg"},
	})
	if err != nil {
		t.Fatalf("Failed to update milestone progress: %v", err)
	}

	// STRICT ANTI-SELF-APPROVAL: Officer assigned to milestone cannot verify their own milestone!
	_, err = env.CommunityProjectService.VerifyMilestone(ctx, coordAUser.UserID, "state_coordinator", &stateA.ID, milestone.ID, service.VerifyMilestoneInput{
		Decision:          "verified",
		VerificationNotes: "I inspected my own work.",
	})
	if err != service.ErrSelfApprovalNotAllowed {
		t.Fatalf("Expected ErrSelfApprovalNotAllowed when assigned officer verifies own milestone, got %v", err)
	}

	// STRICT UNVERIFIED COMPLETION GATE:
	// Trying to mark project as "completed" while milestone is unverified MUST fail!
	compNotes := "Finished project."
	_, err = env.CommunityProjectService.UpdateProjectExecution(ctx, superAdminUser.UserID, "super_admin", nil, project.ID, service.UpdateProjectExecutionInput{
		ExecutionStatus:   "completed",
		VerificationNotes: &compNotes,
	})
	if err == nil || !contains(err.Error(), "UNVERIFIED_MILESTONES") {
		t.Fatalf("Expected UNVERIFIED_MILESTONES error when project completion is attempted with unverified milestones, got %v", err)
	}

	// Independent Super Admin verifies milestone
	verifiedM, err := env.CommunityProjectService.VerifyMilestone(ctx, superAdminUser.UserID, "super_admin", nil, milestone.ID, service.VerifyMilestoneInput{
		Decision:          "verified",
		VerificationNotes: "Inspected and confirmed clearance of first 100 meters. Proper bag disposal verified.",
	})
	if err != nil {
		t.Fatalf("Independent milestone verification failed: %v", err)
	}
	if verifiedM.VerificationStatus != "verified" {
		t.Fatalf("Expected verification status 'verified', got %s", verifiedM.VerificationStatus)
	}

	// Now project completion succeeds!
	completedProj, err := env.CommunityProjectService.UpdateProjectExecution(ctx, superAdminUser.UserID, "super_admin", nil, project.ID, service.UpdateProjectExecutionInput{
		ExecutionStatus:    "completed",
		VerificationNotes:  &compNotes,
		CompletionEvidence: []string{"https://pub-82990f82f02446b5a11817fdce96912d.r2.dev/community/projects/done.jpg"},
	})
	if err != nil {
		t.Fatalf("Project completion verification failed: %v", err)
	}
	if completedProj.ExecutionStatus != "completed" || completedProj.ProgressPercentage != 100 {
		t.Fatalf("Expected project status 'completed' and 100%%, got %s and %d", completedProj.ExecutionStatus, completedProj.ProgressPercentage)
	}

	// -------------------------------------------------------------
	// Test Step 6: Financial Allocation & Expenditure Integration
	// -------------------------------------------------------------
	// Allocate 800,000 NGN (80,000,000 Kobo) to project
	fundedProject, err := env.CommunityProjectService.RecordProjectFunding(ctx, superAdminUser.UserID, "super_admin", project.ID, service.ProjectFundingInput{
		Action:     "allocate",
		AmountKobo: 80000000,
	})
	if err != nil {
		t.Fatalf("Failed to allocate funding: %v", err)
	}
	if fundedProject.AllocatedFunding != 80000000 || fundedProject.FundingStatus != "partially_funded" {
		t.Fatalf("Expected 80,000,000 Kobo allocated and partially_funded, got %d and %s", fundedProject.AllocatedFunding, fundedProject.FundingStatus)
	}

	// Disburse 350,000 NGN (35,000,000 Kobo) for equipment & labor
	evidenceURL := "https://pub-82990f82f02446b5a11817fdce96912d.r2.dev/finance/receipts/oshodi_equipment.jpg"
	notesExp := "Equipment hire and volunteer protective gear purchase"
	expendedProject, err := env.CommunityProjectService.RecordProjectFunding(ctx, superAdminUser.UserID, "super_admin", project.ID, service.ProjectFundingInput{
		Action:      "expense",
		AmountKobo:  35000000,
		EvidenceURL: &evidenceURL,
		Notes:       &notesExp,
	})
	if err != nil {
		t.Fatalf("Failed to record project expenditure: %v", err)
	}
	if expendedProject.RecordedExpenditure != 35000000 {
		t.Fatalf("Expected 35,000,000 Kobo expenditure, got %d", expendedProject.RecordedExpenditure)
	}

	// -------------------------------------------------------------
	// Test Step 7: Public Transparency & Information Protection
	// -------------------------------------------------------------
	publicProject, err := env.CommunityProjectService.GetPublicProject(ctx, project.ID)
	if err != nil {
		t.Fatalf("Failed to fetch public project: %v", err)
	}

	// Check financial summary
	if publicProject.FinancialSummary.ApprovedBudgetKobo != 120000000 {
		t.Errorf("Expected approved budget 120,000,000, got %d", publicProject.FinancialSummary.ApprovedBudgetKobo)
	}
	if publicProject.FinancialSummary.AllocatedFundingKobo != 80000000 {
		t.Errorf("Expected allocated funding 80,000,000, got %d", publicProject.FinancialSummary.AllocatedFundingKobo)
	}
	if publicProject.FinancialSummary.RecordedExpenditureKobo != 35000000 {
		t.Errorf("Expected recorded expenditure 35,000,000, got %d", publicProject.FinancialSummary.RecordedExpenditureKobo)
	}
	expectedBalance := int64(80000000 - 35000000) // 45,000,000 Kobo
	if publicProject.FinancialSummary.AvailableBalanceKobo != expectedBalance {
		t.Errorf("Expected available balance %d, got %d", expectedBalance, publicProject.FinancialSummary.AvailableBalanceKobo)
	}

	// Overview statistics
	stats, err := env.CommunityProjectService.GetOverviewStats(ctx)
	if err != nil {
		t.Fatalf("Failed to fetch overview stats: %v", err)
	}
	if stats.TotalRequests <= 0 || stats.TotalAllocatedFunding <= 0 {
		t.Errorf("Expected positive overview statistics, got %+v", stats)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || (len(s) > 0 && stringContains(s, substr)))
}

func stringContains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
