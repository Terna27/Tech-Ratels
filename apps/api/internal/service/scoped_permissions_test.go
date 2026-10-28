package service_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"ratels-api/internal/db"
	"ratels-api/internal/security"
	"ratels-api/internal/service"
)

func TestScopedPermissions(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	// 1. Fetch Lagos (LA) and Kano (KN)
	states, err := env.GeoService.ListStates(ctx)
	if err != nil {
		t.Fatalf("Failed to fetch states: %v", err)
	}

	var lagosID, kanoID string
	for _, s := range states {
		if s.Code == "LA" {
			lagosID = s.ID
		} else if s.Code == "KN" {
			kanoID = s.ID
		}
	}
	if lagosID == "" || kanoID == "" {
		t.Fatalf("Lagos or Kano state not found in test database")
	}

	kanoLGAs, err := env.GeoService.ListLGAsByState(ctx, kanoID)
	if err != nil || len(kanoLGAs) == 0 {
		t.Fatalf("Failed to fetch Kano LGAs: %v", err)
	}
	kanoLGA := kanoLGAs[0]

	// 2. Create Lagos State Coordinator user
	lagosCoordEmail := fmt.Sprintf("lagos_coord_%d@example.com", time.Now().UnixNano())
	regCoord, err := env.AuthService.Register(ctx, service.RegisterRequest{
		Email:       lagosCoordEmail,
		Password:    "CoordPassword123!",
		DisplayName: "Lagos State Coordinator",
	})
	if err != nil {
		t.Fatalf("Failed to register Lagos coordinator: %v", err)
	}

	// An active appointment is required for coordinator authorization.
	_, err = env.Repo.Queries().CreateAppointment(ctx, db.CreateAppointmentParams{
		ID:               security.GenerateUUID(),
		UserID:           regCoord.UserID,
		RoleTitle:        "Lagos State Coordinator",
		RoleType:         "state_coordinator",
		JurisdictionType: "state",
		JurisdictionID:   service.TextFromString(lagosID),
		StartDate: pgtype.Timestamptz{
			Time:  time.Now().Add(-time.Hour),
			Valid: true,
		},
		Status: "active",
	})
	if err != nil {
		t.Fatalf("Failed to create Lagos coordinator appointment: %v", err)
	}

	// 3. Attempt cross-state creation: Lagos Coordinator tries to create event in Kano State
	_, err = env.CleanupService.CreateEvent(ctx, regCoord.UserID, "state_coordinator", &lagosID, service.CreateEventRequest{
		Title:         "Unauthorized Cross-State Kano Venue",
		StateID:       kanoID, // Kano State (Not Lagos!)
		LGAID:         kanoLGA.ID,
		Address:       "Kano City Road",
		Landmark:      "Kano Central",
		StartTime:     time.Now().Add(24 * time.Hour),
		EndTime:       time.Now().Add(28 * time.Hour),
		Capacity:      100,
		PublicContact: "coord@example.com",
		Status:        "published",
	})

	if err != service.ErrScopeUnauthorized {
		t.Fatalf("Expected ErrScopeUnauthorized when Lagos coordinator creates event in Kano, got %v", err)
	}

	// 4. The active appointment determines the coordinator's jurisdiction.
	// The review queue should be restricted to Lagos.
	_, err = env.PartService.ListReviewQueue(ctx, regCoord.UserID, "state_coordinator", nil)
	if err != nil {
		t.Fatalf("Expected Lagos coordinator to access scoped review queue, got %v", err)
	}
}
