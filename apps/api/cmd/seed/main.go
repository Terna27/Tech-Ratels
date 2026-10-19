package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"ratels-api/internal/calendar"
	"ratels-api/internal/config"
	"ratels-api/internal/db"
	"ratels-api/internal/seeder"
	"ratels-api/internal/security"
)

func text(s string) pgtype.Text {
	return pgtype.Text{String: s, Valid: s != ""}
}

func timestamptz(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

func main() {
	demoFlag := flag.Bool("demo-accounts", false, "Seed demonstration test accounts (development only)")
	flag.Parse()

	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Fatal configuration error: %v\n", err)
	}
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v\n", security.RedactError(err, cfg.DatabaseURL))
	}
	defer pool.Close()

	queries := db.New(pool)

	// 1. Seed Core Reference Data (Safe and idempotent in all environments)
	if err := SeedReferenceData(ctx, pool, queries); err != nil {
		log.Fatalf("Reference data seeding failed: %v\n", security.RedactError(err, cfg.DatabaseURL))
	}

	// 2. Demonstration Accounts Gating
	seedDemos := *demoFlag || os.Getenv("SEED_DEMO_ACCOUNTS") == "true"
	if seedDemos {
		if strings.ToLower(cfg.Environment) == "production" || strings.ToLower(os.Getenv("APP_ENV")) == "production" {
			log.Fatalf("FATAL: Demonstration accounts are strictly prohibited in production (APP_ENV=production)")
		}
		if err := SeedDemoAccounts(ctx, pool, queries); err != nil {
			log.Fatalf("Demo accounts seeding failed: %v\n", security.RedactError(err, cfg.DatabaseURL))
		}
	} else {
		log.Println("Notice: Demonstration accounts were skipped (pass --demo-accounts or SEED_DEMO_ACCOUNTS=true in development to enable).")
	}

	fmt.Println("Seeding execution completed successfully!")
}

// SeedReferenceData seeds or updates the complete 6 Geopolitical Zones, 36 States + FCT,
// all 774 Local Government Areas / Area Councils, State Capitals, Badges, Foundation Entity,
// and scheduled campaigns in a single atomic transaction.
func SeedReferenceData(ctx context.Context, pool *pgxpool.Pool, q *db.Queries) error {
	return seeder.SeedReferenceData(ctx, pool, q)
}

// SeedDemoAccounts seeds isolated test accounts and a sample event for development environments.
func SeedDemoAccounts(ctx context.Context, pool *pgxpool.Pool, q *db.Queries) error {
	log.Println("Seeding demonstration accounts using RFC 2606 reserved example domains (.example.org)...")

	var tx pgx.Tx
	var qtx *db.Queries
	if pool != nil {
		var err error
		tx, err = pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("failed to begin demo accounts transaction: %w", err)
		}
		defer func() {
			if tx != nil {
				_ = tx.Rollback(ctx)
			}
		}()
		qtx = q.WithTx(tx)
	} else {
		qtx = q
	}

	adminPass := os.Getenv("DEMO_ADMIN_PASSWORD")
	if adminPass == "" {
		adminPass = "Admin1234!"
	}
	coordPass := os.Getenv("DEMO_COORD_PASSWORD")
	if coordPass == "" {
		coordPass = "Coord1234!"
	}
	memberPass := os.Getenv("DEMO_MEMBER_PASSWORD")
	if memberPass == "" {
		memberPass = "Member1234!"
	}

	// 1. National Admin (Reserved domain)
	adminEmail := "admin@ratels.example.org"
	adminUser, err := qtx.GetUserByEmail(ctx, adminEmail)
	if errors.Is(err, pgx.ErrNoRows) || adminUser.ID == "" {
		pwHash, err := security.HashPassword(adminPass)
		if err != nil {
			return fmt.Errorf("failed to hash demo admin password: %w", err)
		}
		adminID := security.GenerateUUID()
		adminUser, err = qtx.CreateUser(ctx, db.CreateUserParams{
			ID:                  adminID,
			Email:               text(adminEmail),
			Phone:               text("+2348000000001"),
			PasswordHash:        pwHash,
			DisplayName:         "Demo National Administrator",
			MembershipReference: "RATEL-2026-000001",
			Status:              "active",
			PrimaryRole:         "national_admin",
			IsVerified:          true,
		})
		if err != nil {
			return fmt.Errorf("failed to create demo admin user: %w", err)
		}

		_, err = qtx.CreateMemberProfile(ctx, db.CreateMemberProfileParams{
			ID:             security.GenerateUUID(),
			UserID:         adminID,
			HomeStateID:    pgtype.Text{Valid: false},
			HomeLgaID:      pgtype.Text{Valid: false},
			Address:        text("National Secretariat, Abuja"),
			Bio:            text("Demonstration National Administrator"),
			IsPublicBadges: true,
		})
		if err != nil {
			return fmt.Errorf("failed to create demo admin member profile: %w", err)
		}

		_, err = qtx.CreateAppointment(ctx, db.CreateAppointmentParams{
			ID:                security.GenerateUUID(),
			UserID:            adminID,
			RoleTitle:         "National Administrator",
			RoleType:          "national_admin",
			JurisdictionType:  "national",
			JurisdictionID:    pgtype.Text{Valid: false},
			StartDate:         timestamptz(time.Now().AddDate(-1, 0, 0)),
			Status:            "active",
			ApprovalReference: "DEMO-NAT-BOARD-01",
			PublicContact:     text(adminEmail),
		})
		if err != nil {
			return fmt.Errorf("failed to create demo admin appointment: %w", err)
		}
		log.Printf("Demo Admin created: %s\n", adminEmail)
	} else if err != nil {
		return fmt.Errorf("failed to query demo admin user: %w", err)
	} else {
		log.Printf("Demo Admin (%s) exists; preserving existing credentials without modification.\n", adminEmail)
	}

	// 2. Lagos State Coordinator
	lagosState, err := qtx.GetStateByCode(ctx, "LA")
	if err != nil {
		return fmt.Errorf("failed to query Lagos state for demo coordinator: %w", err)
	}
	lagosLGAs, err := qtx.ListLGAsByState(ctx, lagosState.ID)
	if err != nil {
		return fmt.Errorf("failed to query Lagos LGAs: %w", err)
	}
	var ikejaID string
	for _, l := range lagosLGAs {
		if l.Name == "Ikeja" {
			ikejaID = l.ID
			break
		}
	}
	if ikejaID == "" {
		return errors.New("Ikeja LGA not found in Lagos state")
	}

	coordEmail := "lagos_coord@ratels.example.org"
	coordUser, err := qtx.GetUserByEmail(ctx, coordEmail)
	if errors.Is(err, pgx.ErrNoRows) || coordUser.ID == "" {
		pwHash, err := security.HashPassword(coordPass)
		if err != nil {
			return fmt.Errorf("failed to hash demo coordinator password: %w", err)
		}
		coordID := security.GenerateUUID()
		coordUser, err = qtx.CreateUser(ctx, db.CreateUserParams{
			ID:                  coordID,
			Email:               text(coordEmail),
			Phone:               text("+2348000000002"),
			PasswordHash:        pwHash,
			DisplayName:         "Demo Lagos Coordinator",
			MembershipReference: "RATEL-2026-000002",
			Status:              "active",
			PrimaryRole:         "state_coordinator",
			IsVerified:          true,
		})
		if err != nil {
			return fmt.Errorf("failed to create demo coordinator user: %w", err)
		}

		_, err = qtx.CreateMemberProfile(ctx, db.CreateMemberProfileParams{
			ID:             security.GenerateUUID(),
			UserID:         coordID,
			HomeStateID:    text(lagosState.ID),
			HomeLgaID:      text(ikejaID),
			Address:        text("Ikeja, Lagos State"),
			Bio:            text("Demonstration Lagos State Coordinator"),
			IsPublicBadges: true,
		})
		if err != nil {
			return fmt.Errorf("failed to create demo coordinator member profile: %w", err)
		}

		_, err = qtx.CreateAppointment(ctx, db.CreateAppointmentParams{
			ID:                security.GenerateUUID(),
			UserID:            coordID,
			RoleTitle:         "State Coordinator",
			RoleType:          "state_coordinator",
			JurisdictionType:  "state",
			JurisdictionID:    text(lagosState.ID),
			StartDate:         timestamptz(time.Now().AddDate(-1, 0, 0)),
			Status:            "active",
			ApprovalReference: "DEMO-LAG-APPT-01",
			PublicContact:     text(coordEmail),
		})
		if err != nil {
			return fmt.Errorf("failed to create demo coordinator appointment: %w", err)
		}
		log.Printf("Demo Coordinator created: %s\n", coordEmail)
	} else if err != nil {
		return fmt.Errorf("failed to query demo coordinator user: %w", err)
	} else {
		log.Printf("Demo Coordinator (%s) exists; preserving existing credentials without modification.\n", coordEmail)
	}

	// 3. Sample Member
	memberEmail := "member1@ratels.example.org"
	memberUser, err := qtx.GetUserByEmail(ctx, memberEmail)
	if errors.Is(err, pgx.ErrNoRows) || memberUser.ID == "" {
		pwHash, err := security.HashPassword(memberPass)
		if err != nil {
			return fmt.Errorf("failed to hash demo member password: %w", err)
		}
		memID := security.GenerateUUID()
		memberUser, err = qtx.CreateUser(ctx, db.CreateUserParams{
			ID:                  memID,
			Email:               text(memberEmail),
			Phone:               text("+2348000000003"),
			PasswordHash:        pwHash,
			DisplayName:         "Demo Adebayo Chukwuma",
			MembershipReference: "RATEL-2026-000100",
			Status:              "active",
			PrimaryRole:         "member",
			IsVerified:          true,
		})
		if err != nil {
			return fmt.Errorf("failed to create demo member user: %w", err)
		}

		_, err = qtx.CreateMemberProfile(ctx, db.CreateMemberProfileParams{
			ID:             security.GenerateUUID(),
			UserID:         memID,
			HomeStateID:    text(lagosState.ID),
			HomeLgaID:      text(ikejaID),
			Address:        text("Ikeja, Lagos"),
			Bio:            text("Active community cleanup volunteer"),
			IsPublicBadges: true,
		})
		if err != nil {
			return fmt.Errorf("failed to create demo member profile: %w", err)
		}
		log.Printf("Demo Member created: %s\n", memberEmail)
	} else if err != nil {
		return fmt.Errorf("failed to query demo member user: %w", err)
	} else {
		log.Printf("Demo Member (%s) exists; preserving existing credentials without modification.\n", memberEmail)
	}

	// 4. Sample Published Event
	campaign, err := qtx.GetCleanupCampaignByYearMonth(ctx, db.GetCleanupCampaignByYearMonthParams{
		Year:  2026,
		Month: 10,
	})
	if err != nil {
		return fmt.Errorf("failed to query cleanup campaign for demo event: %w", err)
	}

	events, err := qtx.ListPublishedEventsByState(ctx, lagosState.ID)
	if err != nil {
		return fmt.Errorf("failed to list published events for Lagos: %w", err)
	}
	if len(events) == 0 {
		eventStartTime := time.Date(2026, 10, 31, 7, 0, 0, 0, calendar.GetLagosLocation())
		eventEndTime := time.Date(2026, 10, 31, 11, 0, 0, 0, calendar.GetLagosLocation())
		lat := 6.6018
		lon := 3.3515
		safetyNotes := "Gloves, safety vests, boots, and collection bags distributed at 06:45 AM."
		accNotes := "Wheelchair accessible staging area at the market car park."

		_, err = qtx.CreateEventInstance(ctx, db.CreateEventInstanceParams{
			ID:                 security.GenerateUUID(),
			CampaignID:         text(campaign.ID),
			ProgrammeType:      "cleanup",
			Title:              "Ikeja Under-Bridge & Market Mega Sanitation",
			StateID:            lagosState.ID,
			LgaID:              ikejaID,
			Address:            "Ikeja Under-Bridge Bus Stop & Computer Village Perimeter",
			Landmark:           "Opposite Ikeja Central Motor Park",
			Latitude:           pgtype.Float8{Float64: lat, Valid: true},
			Longitude:          pgtype.Float8{Float64: lon, Valid: true},
			StartTime:          timestamptz(eventStartTime),
			EndTime:            timestamptz(eventEndTime),
			Capacity:           250,
			CoordinatorID:      coordUser.ID,
			PublicContact:      "lagos@ratels.example.org",
			AccessibilityNotes: text(accNotes),
			SafetyNotes:        text(safetyNotes),
			Status:             "published",
		})
		if err != nil {
			return fmt.Errorf("failed to create demo event instance: %w", err)
		}
		log.Println("Demo published cleanup event created for Lagos state.")
	}

	if tx != nil {
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("failed to commit demo accounts transaction: %w", err)
		}
		tx = nil
	}

	return nil
}
