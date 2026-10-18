package main

import (
	"context"
	"crypto/rand"
	"flag"
	"fmt"
	"log"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"ratels-api/internal/config"
	"ratels-api/internal/db"
	"ratels-api/internal/security"
)

func generateRandomPassword(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*()-_=+"
	bytes := make([]byte, length)
	for i := range bytes {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			bytes[i] = charset[i%len(charset)]
		} else {
			bytes[i] = charset[num.Int64()]
		}
	}
	return string(bytes)
}

func text(s string) pgtype.Text {
	return pgtype.Text{String: s, Valid: s != ""}
}

func timestamptz(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

func main() {
	emailFlag := flag.String("email", "", "Administrator email address (required)")
	phoneFlag := flag.String("phone", "", "Administrator phone number (optional)")
	nameFlag := flag.String("name", "Initial Administrator", "Administrator display name")
	passFlag := flag.String("password", "", "Administrator password (generated randomly if omitted)")
	superAdminFlag := flag.Bool("super-admin", false, "Bootstrap or establish as Super Administrator (supreme authority)")
	promoteFlag := flag.Bool("promote", false, "Promote an existing user to Super Administrator without resetting credentials")
	dbURLFlag := flag.String("db", "", "Database connection URL (optional; defaults to DATABASE_URL)")
	flag.Parse()

	if *emailFlag == "" {
		fmt.Fprintf(os.Stderr, "Error: --email is required\n")
		fmt.Fprintf(os.Stderr, "Usage: go run ./cmd/admin --email admin@ratels.org.ng [--super-admin] [--promote] [--name \"First Super Admin\"]\n")
		os.Exit(1)
	}

	cfg := config.Load()
	dbURL := cfg.DatabaseURL
	if *dbURLFlag != "" {
		dbURL = *dbURLFlag
		cfg.DatabaseURL = dbURL
	}
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Fatal configuration error: %v\n", err)
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("Database connection failed: %v\n", security.RedactError(err, dbURL))
	}
	defer pool.Close()

	queries := db.New(pool)
	email := strings.TrimSpace(strings.ToLower(*emailFlag))

	role := "national_admin"
	roleTitle := "National Administrator"
	if *superAdminFlag {
		role = "super_admin"
		roleTitle = "Super Administrator"
	}

	// Check if user already exists
	existing, _ := queries.GetUserByEmail(ctx, text(email))
	if existing.ID != "" {
		if *promoteFlag && *superAdminFlag {
			// Migration / Promotion procedure: Promote existing user to Super Admin without resetting password
			now := time.Now()
			_, err := pool.Exec(ctx, "UPDATE users SET primary_role = 'super_admin', updated_at = CURRENT_TIMESTAMP WHERE id = $1", existing.ID)
			if err != nil {
				log.Fatalf("Failed to promote user: %v\n", err)
			}

			appID := security.GenerateUUID()
			_, err = queries.CreateAppointment(ctx, db.CreateAppointmentParams{
				ID:                       appID,
				UserID:                   existing.ID,
				RoleTitle:                "Super Administrator",
				RoleType:                 "super_admin",
				JurisdictionType:         "national",
				JurisdictionID:           pgtype.Text{Valid: false},
				ReportingToAppointmentID: pgtype.Text{Valid: false},
				StartDate:                timestamptz(now),
				EndDate:                  pgtype.Timestamptz{Valid: false},
				Status:                   "active",
				ApprovalReference:        "CLI-SUPER-ADMIN-PROMOTION",
				PublicContact:            text(email),
			})
			if err != nil {
				log.Fatalf("Failed to create super_admin appointment: %v\n", err)
			}

			// Record in appointment history
			_, _ = queries.CreateAppointmentHistory(ctx, db.CreateAppointmentHistoryParams{
				ID:              security.GenerateUUID(),
				AppointmentID:   appID,
				Action:          "promoted_to_super_admin",
				ChangedByUserID: text(existing.ID),
				PreviousStatus:  text("active"),
				NewStatus:       "active",
				Reason:          text("CLI Bootstrap Super Administrator Promotion"),
				PreviousUserID:  text(existing.ID),
				NewUserID:       text(existing.ID),
				EffectiveDate:   timestamptz(now),
			})

			fmt.Println("==========================================================")
			fmt.Println(" Existing User Promoted to Super Administrator Successfully")
			fmt.Println("==========================================================")
			fmt.Printf("User ID:              %s\n", existing.ID)
			fmt.Printf("Display Name:         %s\n", existing.DisplayName)
			fmt.Printf("Email:                %s\n", email)
			fmt.Printf("Role:                 super_admin (Active)\n")
			fmt.Println("Existing password and security credentials were preserved.")
			fmt.Println("==========================================================")
			return
		}

		fmt.Fprintf(os.Stderr, "Error: User with email %s already exists (ID: %s). Existing credentials are preserved and was not modified. (Use --promote --super-admin to promote).\n", email, existing.ID)
		os.Exit(1)
	}

	password := *passFlag
	generated := false
	if password == "" {
		password = generateRandomPassword(24)
		generated = true
	}

	pwHash, err := security.HashPassword(password)
	if err != nil {
		log.Fatalf("Failed to hash password: %v\n", err)
	}

	adminID := security.GenerateUUID()
	memRef := security.GenerateMembershipReference()

	adminUser, err := queries.CreateUser(ctx, db.CreateUserParams{
		ID:                  adminID,
		Email:               text(email),
		Phone:               text(*phoneFlag),
		PasswordHash:        pwHash,
		DisplayName:         *nameFlag,
		MembershipReference: memRef,
		Status:              "active",
		PrimaryRole:         role,
		IsVerified:          true,
	})
	if err != nil {
		log.Fatalf("Failed to create administrator user: %v\n", err)
	}

	// Create profile
	_, _ = queries.CreateMemberProfile(ctx, db.CreateMemberProfileParams{
		ID:             security.GenerateUUID(),
		UserID:         adminID,
		HomeStateID:    pgtype.Text{Valid: false},
		HomeLgaID:      pgtype.Text{Valid: false},
		Address:        text("National Secretariat"),
		Bio:            text("Root Platform " + roleTitle),
		IsPublicBadges: true,
	})

	// Create appointment
	appID := security.GenerateUUID()
	_, err = queries.CreateAppointment(ctx, db.CreateAppointmentParams{
		ID:                       appID,
		UserID:                   adminID,
		RoleTitle:                roleTitle,
		RoleType:                 role,
		JurisdictionType:         "national",
		JurisdictionID:           pgtype.Text{Valid: false},
		ReportingToAppointmentID: pgtype.Text{Valid: false},
		StartDate:                timestamptz(time.Now()),
		EndDate:                  pgtype.Timestamptz{Valid: false},
		Status:                   "active",
		ApprovalReference:        "CLI-BOOTSTRAP-APPT",
		PublicContact:            text(email),
	})
	if err != nil {
		log.Fatalf("Failed to create administrator appointment: %v\n", err)
	}

	// Record initial appointment history
	_, _ = queries.CreateAppointmentHistory(ctx, db.CreateAppointmentHistoryParams{
		ID:              security.GenerateUUID(),
		AppointmentID:   appID,
		Action:          "created",
		ChangedByUserID: text(adminID),
		PreviousStatus:  text(""),
		NewStatus:       "active",
		Reason:          text("CLI Bootstrap Initial Appointment"),
		PreviousUserID:  text(""),
		NewUserID:       text(adminID),
		EffectiveDate:   timestamptz(time.Now()),
	})

	fmt.Println("==========================================================")
	if *superAdminFlag {
		fmt.Println(" Super Administrator Successfully Created")
	} else {
		fmt.Println(" Administrator Successfully Created")
	}
	fmt.Println("==========================================================")
	fmt.Printf("User ID:              %s\n", adminUser.ID)
	fmt.Printf("Display Name:         %s\n", adminUser.DisplayName)
	fmt.Printf("Email:                %s\n", email)
	fmt.Printf("Membership Reference: %s\n", adminUser.MembershipReference)
	fmt.Printf("Role:                 %s (Active)\n", role)
	if generated {
		fmt.Printf("Generated Password:   %s\n", password)
		fmt.Println("NOTE: Store this password securely. It will NOT be shown again.")
	} else {
		fmt.Println("Password:             [Configured via CLI flag]")
	}
	fmt.Println("==========================================================")
}
