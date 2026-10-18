package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"ratels-api/internal/calendar"
	"ratels-api/internal/config"
	"ratels-api/internal/db"
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

	// 1. Seed Core Reference Data (Safe in all environments)
	if err := SeedReferenceData(ctx, queries); err != nil {
		log.Fatalf("Reference data seeding failed: %v\n", err)
	}

	// 2. Demonstration Accounts Gating
	seedDemos := *demoFlag || os.Getenv("SEED_DEMO_ACCOUNTS") == "true"
	if seedDemos {
		if strings.ToLower(cfg.Environment) == "production" || strings.ToLower(os.Getenv("APP_ENV")) == "production" {
			log.Fatalf("FATAL: Demonstration accounts are strictly prohibited in production (APP_ENV=production)")
		}
		if err := SeedDemoAccounts(ctx, queries); err != nil {
			log.Fatalf("Demo accounts seeding failed: %v\n", err)
		}
	} else {
		log.Println("Notice: Demonstration accounts were skipped (pass --demo-accounts or SEED_DEMO_ACCOUNTS=true in development to enable).")
	}

	fmt.Println("Seeding execution completed successfully!")
}

func SeedReferenceData(ctx context.Context, q *db.Queries) error {
	log.Println("Checking and applying reference geographic and institutional data...")

	// 1. Zones
	zonesData := []struct {
		Code string
		Name string
	}{
		{"NC", "North Central"},
		{"NE", "North East"},
		{"NW", "North West"},
		{"SE", "South East"},
		{"SS", "South South"},
		{"SW", "South West"},
	}

	zoneMap := make(map[string]string)
	for _, z := range zonesData {
		existing, _ := q.ListZones(ctx)
		found := false
		for _, ez := range existing {
			if ez.Code == z.Code {
				zoneMap[z.Code] = ez.ID
				found = true
				break
			}
		}
		if !found {
			id := security.GenerateUUID()
			created, err := q.CreateZone(ctx, db.CreateZoneParams{
				ID:   id,
				Code: z.Code,
				Name: z.Name,
			})
			if err != nil {
				return fmt.Errorf("failed to create zone %s: %w", z.Code, err)
			}
			zoneMap[z.Code] = created.ID
		}
	}

	// 2. States & FCT
	statesData := []struct {
		ZoneCode    string
		Code        string
		Name        string
		IsFCT       bool
		CapitalCity string
		LGAs        []string
	}{
		{
			ZoneCode:    "SW",
			Code:        "LA",
			Name:        "Lagos",
			IsFCT:       false,
			CapitalCity: "Ikeja",
			LGAs:        []string{"Ikeja", "Alimosho", "Lagos Island", "Eti-Osa", "Surulere", "Ikorodu", "Badagry", "Epe", "Kosofe", "Oshodi-Isolo"},
		},
		{
			ZoneCode:    "NC",
			Code:        "FC",
			Name:        "Federal Capital Territory",
			IsFCT:       true, // Territory, not a 37th state
			CapitalCity: "Abuja",
			LGAs:        []string{"Abuja Municipal", "Bwari", "Gwagwalada", "Kuje", "Kwali", "Abaji"},
		},
		{
			ZoneCode:    "SW",
			Code:        "OY",
			Name:        "Oyo",
			IsFCT:       false,
			CapitalCity: "Ibadan",
			LGAs:        []string{"Ibadan North", "Ibadan South-West", "Ogbomosho North", "Oyo East"},
		},
		{
			ZoneCode:    "NW",
			Code:        "KN",
			Name:        "Kano",
			IsFCT:       false,
			CapitalCity: "Kano",
			LGAs:        []string{"Kano Municipal", "Dala", "Fagge", "Nassarawa", "Gwale"},
		},
		{
			ZoneCode:    "SS",
			Code:        "RI",
			Name:        "Rivers",
			IsFCT:       false,
			CapitalCity: "Port Harcourt",
			LGAs:        []string{"Port Harcourt", "Obio-Akpor", "Eleme", "Bonny"},
		},
		{
			ZoneCode:    "SE",
			Code:        "EN",
			Name:        "Enugu",
			IsFCT:       false,
			CapitalCity: "Enugu",
			LGAs:        []string{"Enugu North", "Enugu South", "Enugu East", "Nsukka"},
		},
		{
			ZoneCode:    "NE",
			Code:        "BO",
			Name:        "Borno",
			IsFCT:       false,
			CapitalCity: "Maiduguri",
			LGAs:        []string{"Maiduguri", "Jere", "Bama", "Biu"},
		},
	}

	otherStates := []struct {
		ZoneCode string
		Code     string
		Name     string
		CapCity  string
	}{
		{"NE", "AD", "Adamawa", "Yola"},
		{"SS", "AK", "Akwa Ibom", "Uyo"},
		{"SE", "AN", "Anambra", "Awka"},
		{"NE", "BA", "Bauchi", "Bauchi"},
		{"SS", "BY", "Bayelsa", "Yenagoa"},
		{"NC", "BE", "Benue", "Makurdi"},
		{"SS", "CR", "Cross River", "Calabar"},
		{"SS", "DE", "Delta", "Asaba"},
		{"SE", "EB", "Ebonyi", "Abakaliki"},
		{"SS", "ED", "Edo", "Benin City"},
		{"SW", "EK", "Ekiti", "Ado-Ekiti"},
		{"NE", "GO", "Gombe", "Gombe"},
		{"SE", "IM", "Imo", "Owerri"},
		{"NW", "JI", "Jigawa", "Dutse"},
		{"NW", "KD", "Kaduna", "Kaduna"},
		{"NW", "KT", "Katsina", "Katsina"},
		{"NW", "KE", "Kebbi", "Birnin Kebbi"},
		{"NC", "KO", "Kogi", "Lokoja"},
		{"NC", "KW", "Kwara", "Ilorin"},
		{"NC", "NA", "Nasarawa", "Lafia"},
		{"NC", "NI", "Niger", "Minna"},
		{"SW", "OG", "Ogun", "Abeokuta"},
		{"SW", "ON", "Ondo", "Akure"},
		{"SW", "OS", "Osun", "Osogbo"},
		{"NC", "PL", "Plateau", "Jos"},
		{"NW", "SO", "Sokoto", "Sokoto"},
		{"NE", "TA", "Taraba", "Jalingo"},
		{"NE", "YO", "Yobe", "Damaturu"},
		{"NW", "ZA", "Zamfara", "Gusau"},
		{"SE", "AB", "Abia", "Umuahia"},
	}

	for _, sData := range statesData {
		zoneID := zoneMap[sData.ZoneCode]
		st, err := q.GetStateByCode(ctx, sData.Code)
		if err != nil || st.ID == "" {
			stID := security.GenerateUUID()
			created, err := q.CreateState(ctx, db.CreateStateParams{
				ID:     stID,
				ZoneID: zoneID,
				Code:   sData.Code,
				Name:   sData.Name,
				IsFct:  sData.IsFCT,
			})
			if err != nil {
				return err
			}
			st = created
		}

		capCity, _ := q.GetCapitalCityByState(ctx, st.ID)
		if capCity.ID == "" {
			_, _ = q.CreateCapitalCity(ctx, db.CreateCapitalCityParams{
				ID:      security.GenerateUUID(),
				StateID: st.ID,
				Name:    sData.CapitalCity,
			})
		}

		existingLGAs, _ := q.ListLGAsByState(ctx, st.ID)
		lgaNames := make(map[string]bool)
		for _, el := range existingLGAs {
			lgaNames[el.Name] = true
		}

		for _, lName := range sData.LGAs {
			if !lgaNames[lName] {
				_, err = q.CreateLGA(ctx, db.CreateLGAParams{
					ID:              security.GenerateUUID(),
					StateID:         st.ID,
					Name:            lName,
					IsAreaCouncil:   sData.IsFCT,
					SourceReference: "National Reference / NBS 2018",
					RevisionDate:    "2026-10-17",
				})
				if err != nil {
					return err
				}
			}
		}
	}

	for _, ost := range otherStates {
		zoneID := zoneMap[ost.ZoneCode]
		st, err := q.GetStateByCode(ctx, ost.Code)
		if err != nil || st.ID == "" {
			stID := security.GenerateUUID()
			created, err := q.CreateState(ctx, db.CreateStateParams{
				ID:     stID,
				ZoneID: zoneID,
				Code:   ost.Code,
				Name:   ost.Name,
				IsFct:  false,
			})
			if err == nil {
				_, _ = q.CreateCapitalCity(ctx, db.CreateCapitalCityParams{
					ID:      security.GenerateUUID(),
					StateID: created.ID,
					Name:    ost.CapCity,
				})
				_, _ = q.CreateLGA(ctx, db.CreateLGAParams{
					ID:              security.GenerateUUID(),
					StateID:         created.ID,
					Name:            ost.CapCity + " Central",
					IsAreaCouncil:   false,
					SourceReference: "National Reference / NBS 2018",
					RevisionDate:    "2026-10-17",
				})
			}
		}
	}

	// 3. Badges
	badge, _ := q.GetBadgeByCode(ctx, "first-cleanup-ratel")
	if badge.ID == "" {
		_, _ = q.CreateBadge(ctx, db.CreateBadgeParams{
			ID:          security.GenerateUUID(),
			Code:        "first-cleanup-ratel",
			Name:        "First Participation",
			Description: "Awarded to members upon successful verification of their first community cleanup participation.",
			RuleVersion: "1.0",
		})
	}

	// 4. Foundation Entity
	foundation, _ := q.GetFoundationEntity(ctx)
	if foundation.ID == "" {
		_, _ = q.CreateFoundationEntity(ctx, db.CreateFoundationEntityParams{
			ID:                  security.GenerateUUID(),
			ProvisionalName:     "Martins Vincent Otse Foundation",
			RegisteredLegalName: text(""),
			RegistrationNumber:  text(""),
			IsVerified:          false,
			MissionStatement:    "Empowering grassroots Nigerian communities through hygiene, transparency, and humanitarian initiatives.",
			Disclaimer:          "The Foundation's official name, legal registration details, and bank account credentials remain pending CAC document verification. No live payments or donations are currently accepted.",
		})
	}

	// 5. Cleanup Campaign for October 2026
	schedDate := calendar.CalculateLastSaturdayOfLagos(2026, 10)
	campaign, err := q.GetCleanupCampaignByYearMonth(ctx, db.GetCleanupCampaignByYearMonthParams{
		Year:  2026,
		Month: 10,
	})
	if err != nil || campaign.ID == "" {
		desc := "October 2026 National Clean-up Campaign on the last Saturday of the month across all 36 States and the FCT."
		_, _ = q.CreateCleanupCampaign(ctx, db.CreateCleanupCampaignParams{
			ID:            security.GenerateUUID(),
			Year:          2026,
			Month:         10,
			ScheduledDate: timestamptz(schedDate),
			Title:         "October 2026 National Clean-up Campaign",
			Description:   text(desc),
			Status:        "scheduled",
		})
	}

	return nil
}

func SeedDemoAccounts(ctx context.Context, q *db.Queries) error {
	log.Println("Seeding demonstration accounts using RFC 2606 reserved example domains (.example.org)...")

	// Passwords: from env or development fallback
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
	adminUser, _ := q.GetUserByEmail(ctx, text(adminEmail))
	if adminUser.ID == "" {
		pwHash, _ := security.HashPassword(adminPass)
		adminID := security.GenerateUUID()
		adminUser, _ = q.CreateUser(ctx, db.CreateUserParams{
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
		_, _ = q.CreateMemberProfile(ctx, db.CreateMemberProfileParams{
			ID:             security.GenerateUUID(),
			UserID:         adminID,
			HomeStateID:    pgtype.Text{Valid: false},
			HomeLgaID:      pgtype.Text{Valid: false},
			Address:        text("National Secretariat, Abuja"),
			Bio:            text("Demonstration National Administrator"),
			IsPublicBadges: true,
		})
		_, _ = q.CreateAppointment(ctx, db.CreateAppointmentParams{
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
		log.Printf("Demo Admin created: %s\n", adminEmail)
	} else {
		log.Printf("Demo Admin (%s) exists; preserving existing credentials without modification.\n", adminEmail)
	}

	// 2. Lagos State Coordinator
	coordEmail := "lagos_coord@ratels.example.org"
	coordUser, _ := q.GetUserByEmail(ctx, text(coordEmail))
	lagosState, _ := q.GetStateByCode(ctx, "LA")
	lagosLGAs, _ := q.ListLGAsByState(ctx, lagosState.ID)
	var ikejaID string
	for _, l := range lagosLGAs {
		if l.Name == "Ikeja" {
			ikejaID = l.ID
			break
		}
	}

	if coordUser.ID == "" {
		pwHash, _ := security.HashPassword(coordPass)
		coordID := security.GenerateUUID()
		coordUser, _ = q.CreateUser(ctx, db.CreateUserParams{
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
		_, _ = q.CreateMemberProfile(ctx, db.CreateMemberProfileParams{
			ID:             security.GenerateUUID(),
			UserID:         coordID,
			HomeStateID:    text(lagosState.ID),
			HomeLgaID:      text(ikejaID),
			Address:        text("Ikeja, Lagos State"),
			Bio:            text("Demonstration Lagos State Coordinator"),
			IsPublicBadges: true,
		})
		_, _ = q.CreateAppointment(ctx, db.CreateAppointmentParams{
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
		log.Printf("Demo Coordinator created: %s\n", coordEmail)
	} else {
		log.Printf("Demo Coordinator (%s) exists; preserving existing credentials without modification.\n", coordEmail)
	}

	// 3. Sample Member
	memberEmail := "member1@ratels.example.org"
	memberUser, _ := q.GetUserByEmail(ctx, text(memberEmail))
	if memberUser.ID == "" {
		pwHash, _ := security.HashPassword(memberPass)
		memID := security.GenerateUUID()
		memberUser, _ = q.CreateUser(ctx, db.CreateUserParams{
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
		_, _ = q.CreateMemberProfile(ctx, db.CreateMemberProfileParams{
			ID:             security.GenerateUUID(),
			UserID:         memID,
			HomeStateID:    text(lagosState.ID),
			HomeLgaID:      text(ikejaID),
			Address:        text("Ikeja, Lagos"),
			Bio:            text("Active community cleanup volunteer"),
			IsPublicBadges: true,
		})
		log.Printf("Demo Member created: %s\n", memberEmail)
	} else {
		log.Printf("Demo Member (%s) exists; preserving existing credentials without modification.\n", memberEmail)
	}

	// 4. Sample Published Event
	campaign, err := q.GetCleanupCampaignByYearMonth(ctx, db.GetCleanupCampaignByYearMonthParams{
		Year:  2026,
		Month: 10,
	})
	if err == nil && campaign.ID != "" && coordUser.ID != "" && lagosState.ID != "" && ikejaID != "" {
		events, _ := q.ListPublishedEventsByState(ctx, lagosState.ID)
		if len(events) == 0 {
			eventStartTime := time.Date(2026, 10, 31, 7, 0, 0, 0, calendar.GetLagosLocation())
			eventEndTime := time.Date(2026, 10, 31, 11, 0, 0, 0, calendar.GetLagosLocation())
			lat := 6.6018
			lon := 3.3515
			safetyNotes := "Gloves, safety vests, boots, and collection bags distributed at 06:45 AM."
			accNotes := "Wheelchair accessible staging area at the market car park."

			_, _ = q.CreateEventInstance(ctx, db.CreateEventInstanceParams{
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
		}
	}

	return nil
}
