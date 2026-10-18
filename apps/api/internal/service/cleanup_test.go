package service_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"ratels-api/internal/calendar"
	"ratels-api/internal/service"
)

func TestCleanupCalendarAndCapacityLocking(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	// 1. Calendar Test: Last Saturday calculation in Africa/Lagos
	// October 2026: last day is 31st (Saturday)
	oct2026 := calendar.CalculateLastSaturdayOfLagos(2026, 10)
	if oct2026.Day() != 31 || oct2026.Weekday() != time.Saturday {
		t.Errorf("Expected October 2026 last Saturday to be Oct 31, got Day %d Weekday %v", oct2026.Day(), oct2026.Weekday())
	}
	if oct2026.Hour() != 7 {
		t.Errorf("Expected 07:00 WAT, got hour %d", oct2026.Hour())
	}

	// November 2026: 30 days. Nov 30 is Monday, Nov 28 is Saturday
	nov2026 := calendar.CalculateLastSaturdayOfLagos(2026, 11)
	if nov2026.Day() != 28 || nov2026.Weekday() != time.Saturday {
		t.Errorf("Expected November 2026 last Saturday to be Nov 28, got Day %d Weekday %v", nov2026.Day(), nov2026.Weekday())
	}

	// 2. Fetch or seed a state and LGA
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

	// 3. Create an event with capacity = 2
	adminUserEmail := fmt.Sprintf("admin_%d@example.com", time.Now().UnixNano())
	regAdmin, err := env.AuthService.Register(ctx, service.RegisterRequest{
		Email:       adminUserEmail,
		Password:    "AdminPassword123!",
		DisplayName: "Admin Event Creator",
	})
	if err != nil {
		t.Fatalf("Failed to create admin user: %v", err)
	}

	event, err := env.CleanupService.CreateEvent(ctx, regAdmin.UserID, "national_admin", nil, service.CreateEventRequest{
		ProgrammeType: "cleanup",
		Title:         "Capacity Controlled Cleanup",
		StateID:       state.ID,
		LGAID:         lga.ID,
		Address:       "123 Test Street",
		Landmark:      "Test Landmark",
		StartTime:     time.Now().Add(24 * time.Hour),
		EndTime:       time.Now().Add(28 * time.Hour),
		Capacity:      2,
		PublicContact: "test@example.com",
		Status:        "published",
	})
	if err != nil {
		t.Fatalf("Failed to create event: %v", err)
	}

	// 4. Create 3 distinct volunteer users
	var userIDs []string
	for i := 1; i <= 3; i++ {
		uEmail := fmt.Sprintf("volunteer_%d_%d@example.com", i, time.Now().UnixNano())
		reg, err := env.AuthService.Register(ctx, service.RegisterRequest{
			Email:       uEmail,
			Password:    "Password123!",
			DisplayName: fmt.Sprintf("Volunteer %d", i),
		})
		if err != nil {
			t.Fatalf("Failed to register volunteer: %v", err)
		}
		userIDs = append(userIDs, reg.UserID)
	}

	// 5. Register User 1: Succeeds
	_, err = env.CleanupService.RegisterForEvent(ctx, userIDs[0], event.ID)
	if err != nil {
		t.Fatalf("User 1 registration failed: %v", err)
	}

	// 6. Double registration check for User 1: Fails with ErrAlreadyRegistered
	_, err = env.CleanupService.RegisterForEvent(ctx, userIDs[0], event.ID)
	if err != service.ErrAlreadyRegistered {
		t.Fatalf("Expected ErrAlreadyRegistered for double registration, got %v", err)
	}

	// 7. Register User 2: Succeeds (Capacity 2/2 reached)
	_, err = env.CleanupService.RegisterForEvent(ctx, userIDs[1], event.ID)
	if err != nil {
		t.Fatalf("User 2 registration failed: %v", err)
	}

	// 8. Register User 3: Fails with ErrEventAtCapacity
	_, err = env.CleanupService.RegisterForEvent(ctx, userIDs[2], event.ID)
	if err != service.ErrEventAtCapacity {
		t.Fatalf("Expected ErrEventAtCapacity when registering beyond capacity, got %v", err)
	}

	// 9. Reschedule event test
	newStart := time.Now().Add(48 * time.Hour)
	newEnd := time.Now().Add(52 * time.Hour)
	rescheduled, err := env.CleanupService.RescheduleEvent(ctx, regAdmin.UserID, "national_admin", nil, event.ID, service.RescheduleEventRequest{
		NewStartTime: newStart,
		NewEndTime:   newEnd,
		Reason:       "Severe weather warning issued by NIMET",
	})
	if err != nil {
		t.Fatalf("Failed to reschedule event: %v", err)
	}
	if !rescheduled.RescheduledFrom.Valid {
		t.Errorf("Expected RescheduledFrom to be populated, got invalid")
	}
}
