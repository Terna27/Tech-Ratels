package main

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"ratels-api/internal/db"
	"ratels-api/internal/geodata"
	"ratels-api/internal/security"
)

func getTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	testDBURL := os.Getenv("TEST_DATABASE_URL")
	if testDBURL == "" {
		testDBURL = "postgres://postgres@127.0.0.1:5432/ratels_test?sslmode=disable"
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, testDBURL)
	if err != nil {
		t.Fatalf("Failed to connect to test database %s: %v", testDBURL, err)
	}
	return pool
}

func TestSeedReferenceData_CompleteAndIdempotent(t *testing.T) {
	pool := getTestPool(t)
	defer pool.Close()
	ctx := context.Background()
	q := db.New(pool)

	// 1. Initial Seeding Run
	if err := SeedReferenceData(ctx, pool, q); err != nil {
		t.Fatalf("SeedReferenceData failed on initial execution: %v", err)
	}

	// 2. Validate Zone Counts
	zoneCount, err := q.CountZones(ctx)
	if err != nil {
		t.Fatalf("CountZones failed: %v", err)
	}
	if zoneCount != int64(geodata.ExpectedZonesCount) {
		t.Errorf("expected %d zones, got %d", geodata.ExpectedZonesCount, zoneCount)
	}

	// 3. Validate State Counts
	stateCount, err := q.CountStates(ctx)
	if err != nil {
		t.Fatalf("CountStates failed: %v", err)
	}
	if stateCount != int64(geodata.ExpectedStatesCount) {
		t.Errorf("expected %d states, got %d", geodata.ExpectedStatesCount, stateCount)
	}

	// 4. Validate LGA Counts
	lgaCount, err := q.CountLGAs(ctx)
	if err != nil {
		t.Fatalf("CountLGAs failed: %v", err)
	}
	if lgaCount != int64(geodata.ExpectedTotalLGAs) {
		t.Errorf("expected %d total LGAs, got %d", geodata.ExpectedTotalLGAs, lgaCount)
	}

	// 5. Validate FCT Area Councils vs State LGAs
	fctState, err := q.GetStateByCode(ctx, "FC")
	if err != nil {
		t.Fatalf("GetStateByCode FC failed: %v", err)
	}
	if !fctState.IsFct {
		t.Errorf("expected FCT state to have IsFct=true, got false")
	}

	fctLGAs, err := q.ListLGAsByState(ctx, fctState.ID)
	if err != nil {
		t.Fatalf("ListLGAsByState for FCT failed: %v", err)
	}
	if len(fctLGAs) != geodata.ExpectedAreaCouncils {
		t.Errorf("expected %d area councils for FCT, got %d", geodata.ExpectedAreaCouncils, len(fctLGAs))
	}
	for _, a := range fctLGAs {
		if !a.IsAreaCouncil {
			t.Errorf("expected FCT area council %s to have IsAreaCouncil=true", a.Name)
		}
	}

	// Validate Benue State: 23 LGAs, Makurdi present, Makurdi Central absent
	benueState, err := q.GetStateByCode(ctx, "BE")
	if err != nil {
		t.Fatalf("GetStateByCode BE failed: %v", err)
	}
	benueLGAs, err := q.ListLGAsByState(ctx, benueState.ID)
	if err != nil {
		t.Fatalf("ListLGAsByState for Benue failed: %v", err)
	}
	if len(benueLGAs) != 23 {
		t.Errorf("expected 23 LGAs for Benue, got %d", len(benueLGAs))
	}
	hasMakurdi := false
	for _, l := range benueLGAs {
		if l.Name == "Makurdi" {
			hasMakurdi = true
		}
		if l.Name == "Makurdi Central" {
			t.Errorf("fabricated placeholder 'Makurdi Central' found in Benue LGAs!")
		}
		if l.IsAreaCouncil {
			t.Errorf("state LGA %s in Benue incorrectly marked as Area Council", l.Name)
		}
	}
	if !hasMakurdi {
		t.Errorf("canonical LGA 'Makurdi' not found in Benue state")
	}

	// Validate State Capitals
	states, err := q.ListStates(ctx)
	if err != nil {
		t.Fatalf("ListStates failed: %v", err)
	}
	for _, s := range states {
		capCity, err := q.GetCapitalCityByState(ctx, s.ID)
		if err != nil {
			t.Errorf("state %s (%s) missing capital city: %v", s.Name, s.Code, err)
		}
		if capCity.Name == "" {
			t.Errorf("state %s (%s) has empty capital city name", s.Name, s.Code)
		}
	}

	// 6. IDEMPOTENCY TEST: Run Second Execution
	if err := SeedReferenceData(ctx, pool, q); err != nil {
		t.Fatalf("SeedReferenceData failed on second (idempotency) execution: %v", err)
	}

	// Verify no duplication occurred
	zoneCountAfter, _ := q.CountZones(ctx)
	if zoneCountAfter != zoneCount {
		t.Errorf("zone count changed after second seed: was %d, now %d", zoneCount, zoneCountAfter)
	}

	stateCountAfter, _ := q.CountStates(ctx)
	if stateCountAfter != stateCount {
		t.Errorf("state count changed after second seed: was %d, now %d", stateCount, stateCountAfter)
	}

	lgaCountAfter, _ := q.CountLGAs(ctx)
	if lgaCountAfter != lgaCount {
		t.Errorf("LGA count changed after second seed: was %d, now %d", lgaCount, lgaCountAfter)
	}
}

func TestSeedReferenceData_PlaceholderRemapping(t *testing.T) {
	pool := getTestPool(t)
	defer pool.Close()
	ctx := context.Background()
	q := db.New(pool)

	// Ensure reference data is seeded
	if err := SeedReferenceData(ctx, pool, q); err != nil {
		t.Fatalf("initial seed failed: %v", err)
	}

	benueState, err := q.GetStateByCode(ctx, "BE")
	if err != nil {
		t.Fatalf("failed to get Benue state: %v", err)
	}

	// 1. Artificially insert a fabricated placeholder "Makurdi Central"
	placeholderID := security.GenerateUUID()
	_, err = q.CreateLGA(ctx, db.CreateLGAParams{
		ID:              placeholderID,
		StateID:         benueState.ID,
		Name:            "Makurdi Central",
		IsAreaCouncil:   false,
		SourceReference: "Legacy Test Placeholder",
		RevisionDate:    "2026-10-17",
	})
	if err != nil {
		t.Fatalf("failed to insert placeholder LGA: %v", err)
	}

	// 2. Create a test user with a member profile pointing to the placeholder LGA
	testUserID := security.GenerateUUID()
	pwHash, _ := security.HashPassword("TestPass123!")
	_, err = q.CreateUser(ctx, db.CreateUserParams{
		ID:                  testUserID,
		Email:               text("placeholder_test_" + testUserID[:8] + "@ratels.example.org"),
		Phone:               text("+2348000" + testUserID[:8]),
		PasswordHash:        pwHash,
		DisplayName:         "Placeholder Test User",
		MembershipReference: "RATEL-TEST-" + testUserID[:8],
		Status:              "active",
		PrimaryRole:         "member",
		IsVerified:          true,
	})
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	profileID := security.GenerateUUID()
	_, err = q.CreateMemberProfile(ctx, db.CreateMemberProfileParams{
		ID:             profileID,
		UserID:         testUserID,
		HomeStateID:    text(benueState.ID),
		HomeLgaID:      text(placeholderID),
		Address:        text("123 Makurdi Way"),
		Bio:            text("Test bio"),
		IsPublicBadges: true,
	})
	if err != nil {
		t.Fatalf("failed to create member profile pointing to placeholder: %v", err)
	}

	// 3. Run SeedReferenceData - should detect "Makurdi Central", remap foreign key to "Makurdi", and delete placeholder
	if err := SeedReferenceData(ctx, pool, q); err != nil {
		t.Fatalf("SeedReferenceData failed to clean up and remap placeholder: %v", err)
	}

	// 4. Verify placeholder LGA is gone
	_, err = q.GetLGAByID(ctx, placeholderID)
	if err == nil {
		t.Errorf("expected placeholder LGA %s to be deleted, but it still exists", placeholderID)
	}

	// 5. Verify member profile is preserved and remapped to canonical "Makurdi" LGA
	makurdiLGA, err := q.GetLGAByStateAndName(ctx, db.GetLGAByStateAndNameParams{
		StateID: benueState.ID,
		Lower:   strings.ToLower("Makurdi"),
	})
	if err != nil {
		t.Fatalf("failed to query canonical Makurdi LGA: %v", err)
	}

	// Query member profile directly
	var updatedHomeLgaID pgtype.Text
	err = pool.QueryRow(ctx, "SELECT home_lga_id FROM member_profiles WHERE id = $1", profileID).Scan(&updatedHomeLgaID)
	if err != nil {
		t.Fatalf("failed to query updated member profile: %v", err)
	}
	if !updatedHomeLgaID.Valid || updatedHomeLgaID.String != makurdiLGA.ID {
		t.Errorf("expected member profile to be remapped to Makurdi LGA (%s), got: %v", makurdiLGA.ID, updatedHomeLgaID.String)
	}
}

func TestSeedDemoAccounts_Idempotent(t *testing.T) {
	pool := getTestPool(t)
	defer pool.Close()
	ctx := context.Background()
	q := db.New(pool)

	// Ensure reference data is present first
	if err := SeedReferenceData(ctx, pool, q); err != nil {
		t.Fatalf("SeedReferenceData failed: %v", err)
	}

	// 1. Initial demo accounts seed
	if err := SeedDemoAccounts(ctx, pool, q); err != nil {
		t.Fatalf("SeedDemoAccounts failed on initial run: %v", err)
	}

	adminUser, err := q.GetUserByEmail(ctx, text("admin@ratels.example.org"))
	if err != nil || adminUser.ID == "" {
		t.Fatalf("admin@ratels.example.org not found after seed: %v", err)
	}

	// 2. Second demo accounts seed (Idempotency)
	if err := SeedDemoAccounts(ctx, pool, q); err != nil {
		t.Fatalf("SeedDemoAccounts failed on second run: %v", err)
	}

	adminUserSecond, err := q.GetUserByEmail(ctx, text("admin@ratels.example.org"))
	if err != nil {
		t.Fatalf("failed to query admin user after second seed: %v", err)
	}
	if adminUser.ID != adminUserSecond.ID {
		t.Errorf("admin user ID changed across seeds! Initial: %s, Second: %s", adminUser.ID, adminUserSecond.ID)
	}
}
