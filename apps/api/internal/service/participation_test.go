package service_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"ratels-api/internal/service"
)

func TestParticipationAndAntiSelfApproval(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	// 1. Fetch State and LGA
	states, err := env.GeoService.ListStates(ctx)
	if err != nil || len(states) == 0 {
		t.Fatalf("Failed to fetch states: %v", err)
	}
	state := states[0]
	lgas, err := env.GeoService.ListLGAsByState(ctx, state.ID)
	if err != nil || len(lgas) == 0 {
		t.Fatalf("Failed to fetch LGAs: %v", err)
	}
	lga := lgas[0]

	// 2. Create Coordinator User
	coordEmail := fmt.Sprintf("coord_%d@example.com", time.Now().UnixNano())
	regCoord, err := env.AuthService.Register(ctx, service.RegisterRequest{
		Email:       coordEmail,
		Password:    "Password123!",
		DisplayName: "Review Coordinator",
	})
	if err != nil {
		t.Fatalf("Failed to create coordinator: %v", err)
	}

	// 3. Create Event
	event, err := env.CleanupService.CreateEvent(ctx, regCoord.UserID, "national_admin", nil, service.CreateEventRequest{
		Title:         "Badge Verification Event",
		StateID:       state.ID,
		LGAID:         lga.ID,
		Address:       "Community Center",
		Landmark:      "City Square",
		StartTime:     time.Now().Add(-2 * time.Hour),
		EndTime:       time.Now().Add(-1 * time.Hour),
		Capacity:      100,
		PublicContact: "contact@example.com",
		Status:        "published",
	})
	if err != nil {
		t.Fatalf("Failed to create event: %v", err)
	}

	// 4. Coordinator submits a claim for themselves
	notes := "Participated in clearing the main drainages"
	coordClaim, err := env.PartService.SubmitClaim(ctx, regCoord.UserID, service.SubmitClaimRequest{
		EventID:       event.ID,
		Description:   "Coordinated and actively swept zone A",
		EvidenceNotes: &notes,
	})
	if err != nil {
		t.Fatalf("Failed to submit claim: %v", err)
	}

	// 5. ANTI-SELF-APPROVAL TEST: Coordinator attempts to approve their own claim!
	_, err = env.PartService.ReviewClaim(ctx, regCoord.UserID, "national_admin", nil, coordClaim.ID, service.ReviewClaimRequest{
		Decision: "approved",
		Reason:   "I did great work!",
	})
	if err != service.ErrSelfApprovalNotAllowed {
		t.Fatalf("Expected ErrSelfApprovalNotAllowed when coordinator reviews their own claim, got %v", err)
	}

	// 6. Independent Reviewer tests First Participation badge award
	reviewerEmail := fmt.Sprintf("independent_admin_%d@example.com", time.Now().UnixNano())
	regReviewer, err := env.AuthService.Register(ctx, service.RegisterRequest{
		Email:       reviewerEmail,
		Password:    "ReviewerPassword123!",
		DisplayName: "Independent Review Officer",
	})
	if err != nil {
		t.Fatalf("Failed to create independent reviewer: %v", err)
	}

	// Independent reviewer approves the claim
	reviewed, err := env.PartService.ReviewClaim(ctx, regReviewer.UserID, "national_admin", nil, coordClaim.ID, service.ReviewClaimRequest{
		Decision: "approved",
		Reason:   "Verified via on-ground coordinator logs and attendance sheet",
	})
	if err != nil {
		t.Fatalf("Independent review failed: %v", err)
	}
	if reviewed.Status != "verified" {
		t.Errorf("Expected claim status 'verified', got %s", reviewed.Status)
	}

	// 7. Verify First Participation badge was awarded
	badges, err := env.PartService.ListUserBadges(ctx, regCoord.UserID)
	if err != nil {
		t.Fatalf("Failed to list badges: %v", err)
	}

	var firstBadgeFound bool
	for _, b := range badges {
		if b.BadgeCode == "first-cleanup-ratel" {
			firstBadgeFound = true
			if !b.QualifyingParticipationID.Valid || b.QualifyingParticipationID.String != coordClaim.ID {
				t.Errorf("Expected QualifyingParticipationID to match claim ID %s, got %v", coordClaim.ID, b.QualifyingParticipationID)
			}
			break
		}
	}
	if !firstBadgeFound {
		t.Errorf("Expected First Participation badge to be awarded to user on first approved claim, but was not found")
	}
}
