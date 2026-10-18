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
	"ratels-api/internal/geodata"
	"ratels-api/internal/security"
)

func text(s string) pgtype.Text {
	return pgtype.Text{String: s, Valid: s != ""}
}

func timestamptz(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

// Known placeholder remaps for previously fabricated "<Capital> Central" entries
var placeholderLgaRemaps = map[string]string{
	"Makurdi Central":      "Makurdi",
	"Yola Central":         "Yola North",
	"Uyo Central":          "Uyo",
	"Awka Central":         "Awka South",
	"Bauchi Central":       "Bauchi",
	"Yenagoa Central":      "Yenagoa",
	"Calabar Central":      "Calabar Municipal",
	"Asaba Central":        "Oshimili South",
	"Abakaliki Central":    "Abakaliki",
	"Benin City Central":   "Oredo",
	"Ado-Ekiti Central":    "Ado-Ekiti",
	"Gombe Central":        "Gombe",
	"Owerri Central":       "Owerri Municipal",
	"Dutse Central":        "Dutse",
	"Kaduna Central":       "Kaduna North",
	"Birnin Kebbi Central": "Birnin Kebbi",
	"Lokoja Central":       "Lokoja",
	"Katsina Central":      "Katsina",
	"Ilorin Central":       "Ilorin West",
	"Lafia Central":        "Lafia",
	"Minna Central":        "Chanchaga",
	"Abeokuta Central":     "Abeokuta South",
	"Akure Central":        "Akure South",
	"Osogbo Central":       "Osogbo",
	"Jos Central":          "Jos North",
	"Sokoto Central":       "Sokoto North",
	"Jalingo Central":      "Jalingo",
	"Damaturu Central":     "Damaturu",
	"Gusau Central":        "Gusau",
	"Umuahia Central":      "Umuahia North",
	"Nassarawa":            "Nasarawa",
	"Obio-Akpor":           "Obio/Akpor",
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
	log.Println("Checking and applying complete Nigerian reference geographic and institutional data...")

	var tx pgx.Tx
	var qtx *db.Queries
	if pool != nil {
		var err error
		tx, err = pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("failed to begin reference data transaction: %w", err)
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

	// 1. Geopolitical Zones (6 Zones)
	zoneMap := make(map[string]string) // Code -> ID
	for _, z := range geodata.Zones() {
		existing, err := qtx.GetZoneByCode(ctx, z.Code)
		if err == nil {
			if existing.Name != z.Name {
				updated, err := qtx.UpdateZone(ctx, db.UpdateZoneParams{
					ID:   existing.ID,
					Name: z.Name,
				})
				if err != nil {
					return fmt.Errorf("failed to update zone %s: %w", z.Code, err)
				}
				zoneMap[z.Code] = updated.ID
			} else {
				zoneMap[z.Code] = existing.ID
			}
		} else if errors.Is(err, pgx.ErrNoRows) {
			created, err := qtx.CreateZone(ctx, db.CreateZoneParams{
				ID:   security.GenerateUUID(),
				Code: z.Code,
				Name: z.Name,
			})
			if err != nil {
				return fmt.Errorf("failed to create zone %s: %w", z.Code, err)
			}
			zoneMap[z.Code] = created.ID
		} else {
			return fmt.Errorf("failed to query zone %s: %w", z.Code, err)
		}
	}

	// 2. States, Capitals, and Complete 774 LGAs / Area Councils
	for _, sData := range geodata.States() {
		zoneID, ok := zoneMap[sData.ZoneCode]
		if !ok {
			return fmt.Errorf("zone %s not found for state %s", sData.ZoneCode, sData.Code)
		}

		st, err := qtx.GetStateByCode(ctx, sData.Code)
		if err == nil {
			if st.ZoneID != zoneID || st.Name != sData.Name || st.IsFct != sData.IsFCT {
				st, err = qtx.UpdateState(ctx, db.UpdateStateParams{
					ID:     st.ID,
					ZoneID: zoneID,
					Name:   sData.Name,
					IsFct:  sData.IsFCT,
				})
				if err != nil {
					return fmt.Errorf("failed to update state %s (%s): %w", sData.Code, sData.Name, err)
				}
			}
		} else if errors.Is(err, pgx.ErrNoRows) {
			st, err = qtx.CreateState(ctx, db.CreateStateParams{
				ID:     security.GenerateUUID(),
				ZoneID: zoneID,
				Code:   sData.Code,
				Name:   sData.Name,
				IsFct:  sData.IsFCT,
			})
			if err != nil {
				return fmt.Errorf("failed to create state %s (%s): %w", sData.Code, sData.Name, err)
			}
		} else {
			return fmt.Errorf("failed to query state %s: %w", sData.Code, err)
		}

		// Capital City
		capCity, err := qtx.GetCapitalCityByState(ctx, st.ID)
		if err == nil {
			if capCity.Name != sData.CapitalCity {
				_, err = qtx.UpdateCapitalCity(ctx, db.UpdateCapitalCityParams{
					ID:   capCity.ID,
					Name: sData.CapitalCity,
				})
				if err != nil {
					return fmt.Errorf("failed to update capital city for state %s: %w", sData.Code, err)
				}
			}
		} else if errors.Is(err, pgx.ErrNoRows) {
			_, err = qtx.CreateCapitalCity(ctx, db.CreateCapitalCityParams{
				ID:      security.GenerateUUID(),
				StateID: st.ID,
				Name:    sData.CapitalCity,
			})
			if err != nil {
				return fmt.Errorf("failed to create capital city for state %s: %w", sData.Code, err)
			}
		} else {
			return fmt.Errorf("failed to query capital city for state %s: %w", sData.Code, err)
		}

		// Existing LGAs in this state
		existingLGAs, err := qtx.ListLGAsByState(ctx, st.ID)
		if err != nil {
			return fmt.Errorf("failed to list LGAs for state %s: %w", sData.Code, err)
		}

		existingMap := make(map[string]db.Lga)
		for _, el := range existingLGAs {
			existingMap[strings.ToLower(strings.TrimSpace(el.Name))] = el
		}

		canonicalMap := make(map[string]string)
		for _, lName := range sData.LGAs {
			canonicalMap[strings.ToLower(strings.TrimSpace(lName))] = lName
		}

		// Ensure all canonical LGAs exist
		for _, lName := range sData.LGAs {
			lower := strings.ToLower(strings.TrimSpace(lName))
			if existing, found := existingMap[lower]; found {
				// Update metadata if needed
				if existing.Name != lName || existing.IsAreaCouncil != sData.IsFCT ||
					existing.SourceReference != geodata.AuthoritativeSource ||
					existing.RevisionDate != geodata.VerificationDate {
					updated, err := qtx.UpdateLGA(ctx, db.UpdateLGAParams{
						ID:              existing.ID,
						Name:            lName,
						IsAreaCouncil:   sData.IsFCT,
						SourceReference: geodata.AuthoritativeSource,
						RevisionDate:    geodata.VerificationDate,
					})
					if err != nil {
						return fmt.Errorf("failed to update LGA %s in state %s: %w", lName, sData.Code, err)
					}
					existingMap[lower] = updated
				}
			} else {
				created, err := qtx.CreateLGA(ctx, db.CreateLGAParams{
					ID:              security.GenerateUUID(),
					StateID:         st.ID,
					Name:            lName,
					IsAreaCouncil:   sData.IsFCT,
					SourceReference: geodata.AuthoritativeSource,
					RevisionDate:    geodata.VerificationDate,
				})
				if err != nil {
					return fmt.Errorf("failed to create LGA %s in state %s: %w", lName, sData.Code, err)
				}
				existingMap[lower] = created
			}
		}

		// Detect and remove fabricated placeholders (e.g. "Makurdi Central")
		for _, el := range existingLGAs {
			lower := strings.ToLower(strings.TrimSpace(el.Name))
			if _, isCanonical := canonicalMap[lower]; !isCanonical {
				// Find target canonical LGA to remap any references
				targetName := ""
				if remapName, hasRemap := placeholderLgaRemaps[el.Name]; hasRemap {
					targetName = remapName
				} else if strings.HasSuffix(strings.ToLower(el.Name), " central") {
					stripped := strings.TrimSpace(el.Name[:len(el.Name)-8])
					if _, ok := canonicalMap[strings.ToLower(stripped)]; ok {
						targetName = canonicalMap[strings.ToLower(stripped)]
					}
				} else {
					alt1 := strings.ReplaceAll(el.Name, "-", "/")
					alt2 := strings.ReplaceAll(el.Name, "/", "-")
					if _, ok := canonicalMap[strings.ToLower(alt1)]; ok {
						targetName = canonicalMap[strings.ToLower(alt1)]
					} else if _, ok := canonicalMap[strings.ToLower(alt2)]; ok {
						targetName = canonicalMap[strings.ToLower(alt2)]
					}
				}

				if targetName == "" {
					// Fall back to first canonical LGA in this state
					if len(sData.LGAs) > 0 {
						targetName = sData.LGAs[0]
					}
				}

				if targetLga, ok := existingMap[strings.ToLower(targetName)]; ok {
					// Remap foreign keys in member_profiles
					if err := qtx.RemapMemberProfileLGA(ctx, db.RemapMemberProfileLGAParams{
						NewLgaID: pgtype.Text{String: targetLga.ID, Valid: true},
						OldLgaID: pgtype.Text{String: el.ID, Valid: true},
					}); err != nil {
						return fmt.Errorf("failed to remap member profiles from placeholder LGA %s: %w", el.Name, err)
					}

					// Remap foreign keys in event_instances
					if err := qtx.RemapEventInstanceLGA(ctx, db.RemapEventInstanceLGAParams{
						NewLgaID: targetLga.ID,
						OldLgaID: el.ID,
					}); err != nil {
						return fmt.Errorf("failed to remap events from placeholder LGA %s: %w", el.Name, err)
					}
				}

				// Safe to delete placeholder record
				if err := qtx.DeleteLGA(ctx, el.ID); err != nil {
					return fmt.Errorf("failed to delete placeholder LGA %s: %w", el.Name, err)
				}
				log.Printf("Removed placeholder LGA %q in %s (remapped references to %q)", el.Name, sData.Name, targetName)
			}
		}
	}

	// 3. Badges
	badge, err := qtx.GetBadgeByCode(ctx, "first-cleanup-ratel")
	if errors.Is(err, pgx.ErrNoRows) {
		_, err = qtx.CreateBadge(ctx, db.CreateBadgeParams{
			ID:          security.GenerateUUID(),
			Code:        "first-cleanup-ratel",
			Name:        "First Participation",
			Description: "Awarded to members upon successful verification of their first community cleanup participation.",
			RuleVersion: "1.0",
		})
		if err != nil {
			return fmt.Errorf("failed to create badge 'first-cleanup-ratel': %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("failed to query badge 'first-cleanup-ratel': %w", err)
	} else {
		_ = badge // Preserved without duplicate
	}

	// 4. Foundation Entity
	foundation, err := qtx.GetFoundationEntity(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		_, err = qtx.CreateFoundationEntity(ctx, db.CreateFoundationEntityParams{
			ID:                  security.GenerateUUID(),
			ProvisionalName:     "Martins Vincent Otse Foundation",
			RegisteredLegalName: text(""),
			RegistrationNumber:  text(""),
			IsVerified:          false,
			MissionStatement:    "Empowering grassroots Nigerian communities through hygiene, transparency, and humanitarian initiatives.",
			Disclaimer:          "The Foundation's official name, legal registration details, and bank account credentials remain pending CAC document verification. No live payments or donations are currently accepted.",
		})
		if err != nil {
			return fmt.Errorf("failed to create foundation entity: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("failed to query foundation entity: %w", err)
	} else {
		_ = foundation // Preserved without duplicate
	}

	// 5. Cleanup Campaign for October 2026
	campaign, err := qtx.GetCleanupCampaignByYearMonth(ctx, db.GetCleanupCampaignByYearMonthParams{
		Year:  2026,
		Month: 10,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		schedDate := calendar.CalculateLastSaturdayOfLagos(2026, 10)
		desc := "October 2026 National Clean-up Campaign on the last Saturday of the month across all 36 States and the FCT."
		_, err = qtx.CreateCleanupCampaign(ctx, db.CreateCleanupCampaignParams{
			ID:            security.GenerateUUID(),
			Year:          2026,
			Month:         10,
			ScheduledDate: timestamptz(schedDate),
			Title:         "October 2026 National Clean-up Campaign",
			Description:   text(desc),
			Status:        "scheduled",
		})
		if err != nil {
			return fmt.Errorf("failed to create cleanup campaign for 2026-10: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("failed to query cleanup campaign for 2026-10: %w", err)
	} else {
		_ = campaign // Preserved without duplicate
	}

	// Verification check of total administrative counts
	zoneCount, err := qtx.CountZones(ctx)
	if err != nil {
		return fmt.Errorf("failed to count zones: %w", err)
	}
	stateCount, err := qtx.CountStates(ctx)
	if err != nil {
		return fmt.Errorf("failed to count states: %w", err)
	}
	lgaCount, err := qtx.CountLGAs(ctx)
	if err != nil {
		return fmt.Errorf("failed to count LGAs: %w", err)
	}

	if lgaCount != int64(geodata.ExpectedTotalLGAs) {
		return fmt.Errorf("LGA count mismatch after seeding: got %d, expected %d", lgaCount, geodata.ExpectedTotalLGAs)
	}

	log.Printf("Geographic reference data verified: %d zones, %d states/territories, %d LGAs / Area Councils.", zoneCount, stateCount, lgaCount)

	if tx != nil {
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("failed to commit reference data transaction: %w", err)
		}
		tx = nil
	}

	return nil
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
	adminUser, err := qtx.GetUserByEmail(ctx, text(adminEmail))
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
	coordUser, err := qtx.GetUserByEmail(ctx, text(coordEmail))
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
	memberUser, err := qtx.GetUserByEmail(ctx, text(memberEmail))
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
