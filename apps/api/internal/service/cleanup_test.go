package service_test

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgtype"
	"ratels-api/internal/db"
	"ratels-api/internal/security"
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

	// Operational appointments require an active, verified account.
	if _, err := env.Pool.Exec(ctx, `UPDATE users SET status='active',is_verified=true WHERE id=$1`, regAdmin.UserID); err != nil {
		t.Fatal(err)
	}

	_, err = env.Repo.Queries().CreateAppointment(ctx, db.CreateAppointmentParams{
		ID:               security.GenerateUUID(),
		UserID:           regAdmin.UserID,
		RoleTitle:        "Test Event Coordinator",
		RoleType:         "state_coordinator",
		JurisdictionType: "state",
		JurisdictionID:   service.TextFromString(state.ID),
		StartDate: pgtype.Timestamptz{
			Time:  time.Now().Add(-time.Hour),
			Valid: true,
		},
		Status: "active",
	})
	if err != nil {
		t.Fatalf("Failed to appoint event coordinator: %v", err)
	}

	event, err := env.CleanupService.CreateEvent(ctx, regAdmin.UserID, "state_coordinator", nil, service.CreateEventRequest{
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

	// Phase 68.9: cancellation and re-registration regression.
	original, err := env.CleanupService.GetMyRegistration(ctx, userIDs[0], event.ID)
	if err != nil {
		t.Fatalf("Failed to retrieve original registration: %v", err)
	}
	if !original.CheckinCode.Valid || original.CheckinCode.String == "" {
		t.Fatal("Original registration has no check-in code")
	}
	oldCode := original.CheckinCode.String

	if err := env.CleanupService.CancelRegistration(ctx, userIDs[0], event.ID); err != nil {
		t.Fatalf("Cancellation failed: %v", err)
	}

	reactivated, err := env.CleanupService.RegisterForEvent(ctx, userIDs[0], event.ID)
	if err != nil {
		t.Fatalf("Re-registration failed: %v", err)
	}
	if !reactivated.CheckinCode.Valid || reactivated.CheckinCode.String == "" {
		t.Fatal("Reactivated registration has no check-in code")
	}
	if reactivated.CheckinCode.String == oldCode {
		t.Fatal("Re-registration reused the old check-in code")
	}

	persisted, err := env.CleanupService.GetMyRegistration(ctx, userIDs[0], event.ID)
	if err != nil {
		t.Fatalf("Failed to retrieve reactivated registration: %v", err)
	}
	if persisted.CheckinCode.String != reactivated.CheckinCode.String {
		t.Fatal("New check-in code was not persisted")
	}

	// Stale check-in code must not verify attendance.
	_, staleErr := env.Repo.Queries().VerifyEventAttendanceByCode(
		ctx,
		db.VerifyEventAttendanceByCodeParams{
			EventID:            event.ID,
			UserID:             userIDs[0],
			VerifiedByUserID:   service.TextFromString(userIDs[1]),
			VerificationMethod: service.TextFromString("manual_code"),
			CheckinCode:        service.TextFromString(oldCode),
		},
	)
	if staleErr == nil {
		t.Fatal("Old check-in code unexpectedly verified attendance")
	}

	eventAfter, err := env.Repo.Queries().GetEventInstanceByID(ctx, event.ID)
	if err != nil {
		t.Fatalf("Failed to retrieve event capacity: %v", err)
	}
	if eventAfter.RegisteredCount != 2 {
		t.Fatalf("Expected 2 registered members, got %d", eventAfter.RegisteredCount)
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
