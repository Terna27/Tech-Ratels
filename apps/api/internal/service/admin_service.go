package service

import (
	"context"
	"fmt"
	"time"

	"ratels-api/internal/db"
	"ratels-api/internal/repository"
	"ratels-api/internal/seeder"
)

type AdminService struct {
	repo  *repository.Repository
	audit *AuditService
}

func NewAdminService(repo *repository.Repository, audit ...*AuditService) *AdminService {
	var a *AuditService
	if len(audit) > 0 {
		a = audit[0]
	}
	return &AdminService{repo: repo, audit: a}
}

type AdminOverviewResponse struct {
	CurrentRole                   string `json:"current_role"`
	CurrentJurisdictionType       string `json:"current_jurisdiction_type"`
	CurrentJurisdictionName       string `json:"current_jurisdiction_name"`
	TotalMembers                  int64  `json:"total_members"`
	PendingMemberVerifications    int64  `json:"pending_member_verifications"`
	ActiveCoordinators            int64  `json:"active_coordinators"`
	PublishedActivities           int64  `json:"published_activities"`
	DraftVenuesReviewQueue        int64  `json:"draft_venues_review_queue"`
	UnverifiedParticipationsQueue int64  `json:"unverified_participations_queue"`
	PendingAppealsQueue           int64  `json:"pending_appeals_queue"`
	TotalRegistrations            int64  `json:"total_registrations"`
	// Backward compatibility fields
	ActiveAppointments int64 `json:"active_appointments"`
	PublishedCleanups  int64 `json:"published_cleanups"`
	PendingReviews     int64 `json:"pending_reviews"`
}

type PaginatedAuditLogsResponse struct {
	Items []db.AuditEvent `json:"items"`
	Total int64           `json:"total"`
	Page  int32           `json:"page"`
	Limit int32           `json:"limit"`
}

type SuperAdminDashboardStats struct {
	// National Member Counts
	TotalMembers               int64 `json:"total_members"`
	VerifiedMembers            int64 `json:"verified_members"`
	PendingMemberVerifications int64 `json:"pending_member_verifications"`
	TotalRegistrations         int64 `json:"total_registrations"`

	// Active Coordinators by Jurisdiction
	ActiveCoordinators     int64 `json:"active_coordinators"`
	ZonalCoordinatorsCount int64 `json:"zonal_coordinators_count"`
	StateCoordinatorsCount int64 `json:"state_coordinators_count"`
	LGACoordinatorsCount   int64 `json:"lga_coordinators_count"`
	NationalAdminsCount    int64 `json:"national_admins_count"`

	// Geographic Coverage Metrics
	ActiveZones   int64 `json:"active_zones"`
	TotalZones    int64 `json:"total_zones"` // 6
	CoveredStates int64 `json:"covered_states"`
	TotalStates   int64 `json:"total_states"` // 37
	CoveredLGAs   int64 `json:"covered_lgas"`
	TotalLGAs     int64 `json:"total_lgas"` // 774

	// Environmental Cleanup Oversight
	UpcomingCleanupsCount         int64 `json:"upcoming_cleanups_count"`
	PublishedActivities           int64 `json:"published_activities"`
	DraftVenuesReviewQueue        int64 `json:"draft_venues_review_queue"`
	UnverifiedParticipationsQueue int64 `json:"unverified_participations_queue"`
	PendingAppealsQueue           int64 `json:"pending_appeals_queue"`

	// Real Operational Lists
	UpcomingActivities []db.ListUpcomingCleanupActivitiesRow `json:"upcoming_activities"`
	RecentAuditActions []db.ListRecentAuditLogsWithActorsRow `json:"recent_audit_actions"`
	ZoneCoverage       []db.ListZoneCoverageRow              `json:"zone_coverage"`
}

func (s *AdminService) GetOverview(ctx context.Context, actorRole, actorJurisdictionType, actorJurisdictionName string) (*AdminOverviewResponse, error) {
	totalMembers, err := s.repo.Queries().CountUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to count total users: %w", err)
	}
	pendingVerif, err := s.repo.Queries().CountPendingVerificationUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to count pending verification users: %w", err)
	}
	activeApps, err := s.repo.Queries().CountActiveAppointments(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to count active appointments: %w", err)
	}
	pubCleanups, err := s.repo.Queries().CountPublishedEvents(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to count published events: %w", err)
	}
	draftEvents, err := s.repo.Queries().CountDraftEvents(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to count draft events: %w", err)
	}
	pendingReviews, err := s.repo.Queries().CountPendingClaims(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to count pending claims: %w", err)
	}
	pendingAppeals, err := s.repo.Queries().CountPendingAppeals(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to count pending appeals: %w", err)
	}
	totalRegs, err := s.repo.Queries().CountTotalRegistrations(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to count total registrations: %w", err)
	}

	if actorRole == "" {
		actorRole = "super_admin"
	}
	if actorJurisdictionType == "" {
		actorJurisdictionType = "national"
	}
	if actorJurisdictionName == "" {
		actorJurisdictionName = "National Command (All Nigeria)"
	}

	return &AdminOverviewResponse{
		CurrentRole:                   actorRole,
		CurrentJurisdictionType:       actorJurisdictionType,
		CurrentJurisdictionName:       actorJurisdictionName,
		TotalMembers:                  totalMembers,
		PendingMemberVerifications:    pendingVerif,
		ActiveCoordinators:            activeApps,
		PublishedActivities:           pubCleanups,
		DraftVenuesReviewQueue:        draftEvents,
		UnverifiedParticipationsQueue: pendingReviews,
		PendingAppealsQueue:           pendingAppeals,
		TotalRegistrations:            totalRegs,
		ActiveAppointments:            activeApps,
		PublishedCleanups:             pubCleanups,
		PendingReviews:                pendingReviews,
	}, nil
}

func (s *AdminService) GetSuperAdminDashboardStats(ctx context.Context) (*SuperAdminDashboardStats, error) {
	totalMembers, err := s.repo.Queries().CountUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to count users: %w", err)
	}

	verifiedMembers, err := s.repo.Queries().CountVerifiedUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to count verified users: %w", err)
	}

	pendingVerif, err := s.repo.Queries().CountPendingVerificationUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to count pending verification users: %w", err)
	}

	totalRegs, err := s.repo.Queries().CountTotalRegistrations(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to count total registrations: %w", err)
	}

	activeCoordinators, err := s.repo.Queries().CountTotalCoordinators(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to count active coordinators: %w", err)
	}

	coordByJurisdiction, err := s.repo.Queries().CountActiveCoordinatorsByJurisdictionType(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to count coordinators by jurisdiction: %w", err)
	}

	var zonalCount, stateCount, lgaCount, nationalCount int64
	for _, row := range coordByJurisdiction {
		switch row.JurisdictionType {
		case "zone":
			zonalCount += row.CoordinatorCount
		case "state":
			stateCount += row.CoordinatorCount
		case "lga":
			lgaCount += row.CoordinatorCount
		case "national":
			nationalCount += row.CoordinatorCount
		}
	}

	coveredZones, err := s.repo.Queries().CountCoveredZones(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to count covered zones: %w", err)
	}

	totalZones, err := s.repo.Queries().CountZones(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to count total zones: %w", err)
	}

	coveredStates, err := s.repo.Queries().CountCoveredStates(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to count covered states: %w", err)
	}

	totalStates, err := s.repo.Queries().CountStates(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to count total states: %w", err)
	}

	coveredLGAs, err := s.repo.Queries().CountCoveredLGAs(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to count covered LGAs: %w", err)
	}

	totalLGAs, err := s.repo.Queries().CountLGAs(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to count total LGAs: %w", err)
	}

	upcomingCleanups, err := s.repo.Queries().CountUpcomingCleanupActivities(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to count upcoming cleanups: %w", err)
	}

	pubCleanups, err := s.repo.Queries().CountPublishedEvents(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to count published events: %w", err)
	}

	draftEvents, err := s.repo.Queries().CountDraftEvents(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to count draft events: %w", err)
	}

	pendingReviews, err := s.repo.Queries().CountPendingClaims(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to count pending claims: %w", err)
	}

	pendingAppeals, err := s.repo.Queries().CountPendingAppeals(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to count pending appeals: %w", err)
	}

	upcomingActivities, err := s.repo.Queries().ListUpcomingCleanupActivities(ctx, 10)
	if err != nil {
		return nil, fmt.Errorf("failed to list upcoming cleanups: %w", err)
	}
	if upcomingActivities == nil {
		upcomingActivities = []db.ListUpcomingCleanupActivitiesRow{}
	}

	recentAudit, err := s.repo.Queries().ListRecentAuditLogsWithActors(ctx, 15)
	if err != nil {
		return nil, fmt.Errorf("failed to list recent audit actions: %w", err)
	}
	if recentAudit == nil {
		recentAudit = []db.ListRecentAuditLogsWithActorsRow{}
	}

	zoneCoverage, err := s.repo.Queries().ListZoneCoverage(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list zone coverage: %w", err)
	}
	if zoneCoverage == nil {
		zoneCoverage = []db.ListZoneCoverageRow{}
	}

	return &SuperAdminDashboardStats{
		TotalMembers:                  totalMembers,
		VerifiedMembers:               verifiedMembers,
		PendingMemberVerifications:    pendingVerif,
		TotalRegistrations:            totalRegs,
		ActiveCoordinators:            activeCoordinators,
		ZonalCoordinatorsCount:        zonalCount,
		StateCoordinatorsCount:        stateCount,
		LGACoordinatorsCount:          lgaCount,
		NationalAdminsCount:           nationalCount,
		ActiveZones:                   coveredZones,
		TotalZones:                    totalZones,
		CoveredStates:                 coveredStates,
		TotalStates:                   totalStates,
		CoveredLGAs:                   coveredLGAs,
		TotalLGAs:                     totalLGAs,
		UpcomingCleanupsCount:         upcomingCleanups,
		PublishedActivities:           pubCleanups,
		DraftVenuesReviewQueue:        draftEvents,
		UnverifiedParticipationsQueue: pendingReviews,
		PendingAppealsQueue:           pendingAppeals,
		UpcomingActivities:            upcomingActivities,
		RecentAuditActions:            recentAudit,
		ZoneCoverage:                  zoneCoverage,
	}, nil
}

func (s *AdminService) ListZoneCoverage(ctx context.Context) ([]db.ListZoneCoverageRow, error) {
	rows, err := s.repo.Queries().ListZoneCoverage(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list zone coverage: %w", err)
	}
	if rows == nil {
		rows = []db.ListZoneCoverageRow{}
	}
	return rows, nil
}

func (s *AdminService) ListStateCoverage(ctx context.Context) ([]db.ListStateCoverageRow, error) {
	rows, err := s.repo.Queries().ListStateCoverage(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list state coverage: %w", err)
	}
	if rows == nil {
		rows = []db.ListStateCoverageRow{}
	}
	return rows, nil
}

type LGACoverageRow struct {
	LgaID             string     `json:"lga_id"`
	LgaName           string     `json:"lga_name"`
	IsAreaCouncil     bool       `json:"is_area_council"`
	StateID           string     `json:"state_id"`
	AppointmentID     *string    `json:"appointment_id,omitempty"`
	UserID            *string    `json:"user_id,omitempty"`
	CoordinatorName   *string    `json:"coordinator_name,omitempty"`
	CoordinatorEmail  *string    `json:"coordinator_email,omitempty"`
	CoordinatorPhone  *string    `json:"coordinator_phone,omitempty"`
	StartDate         *time.Time `json:"start_date,omitempty"`
	AppointmentStatus *string    `json:"appointment_status,omitempty"`
}

func (s *AdminService) ListLGACoverage(ctx context.Context, stateID string) ([]LGACoverageRow, error) {
	if stateID != "" {
		rows, err := s.repo.Queries().ListLGACoverageByState(ctx, stateID)
		if err != nil {
			return nil, fmt.Errorf("failed to list LGA coverage: %w", err)
		}
		result := make([]LGACoverageRow, 0, len(rows))
		for _, r := range rows {
			result = append(result, LGACoverageRow{
				LgaID:             r.LgaID,
				LgaName:           r.LgaName,
				IsAreaCouncil:     r.IsAreaCouncil,
				StateID:           r.StateID,
				AppointmentID:     StringPtrFromText(r.AppointmentID),
				UserID:            StringPtrFromText(r.UserID),
				CoordinatorName:   StringPtrFromText(r.CoordinatorName),
				CoordinatorEmail:  StringPtrFromText(r.CoordinatorEmail),
				CoordinatorPhone:  StringPtrFromText(r.CoordinatorPhone),
				StartDate:         TimeFromTimestamptz(r.StartDate),
				AppointmentStatus: StringPtrFromText(r.AppointmentStatus),
			})
		}
		return result, nil
	}

	rows, err := s.repo.Queries().ListAllLGACoverage(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list all LGA coverage: %w", err)
	}
	result := make([]LGACoverageRow, 0, len(rows))
	for _, r := range rows {
		result = append(result, LGACoverageRow{
			LgaID:             r.LgaID,
			LgaName:           r.LgaName,
			IsAreaCouncil:     r.IsAreaCouncil,
			StateID:           r.StateID,
			AppointmentID:     StringPtrFromText(r.AppointmentID),
			UserID:            StringPtrFromText(r.UserID),
			CoordinatorName:   StringPtrFromText(r.CoordinatorName),
			CoordinatorEmail:  StringPtrFromText(r.CoordinatorEmail),
			CoordinatorPhone:  StringPtrFromText(r.CoordinatorPhone),
			StartDate:         TimeFromTimestamptz(r.StartDate),
			AppointmentStatus: StringPtrFromText(r.AppointmentStatus),
		})
	}
	return result, nil
}

func (s *AdminService) ListLGACoverageByState(ctx context.Context, stateID string) ([]db.ListLGACoverageByStateRow, error) {
	rows, err := s.repo.Queries().ListLGACoverageByState(ctx, stateID)
	if err != nil {
		return nil, fmt.Errorf("failed to list LGA coverage: %w", err)
	}
	if rows == nil {
		rows = []db.ListLGACoverageByStateRow{}
	}
	return rows, nil
}

func (s *AdminService) ListAuditLogs(ctx context.Context, page, limit int32) (*PaginatedAuditLogsResponse, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	logs, err := s.repo.Queries().ListAuditEvents(ctx, db.ListAuditEventsParams{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list audit events: %w", err)
	}
	if logs == nil {
		logs = []db.AuditEvent{}
	}

	total, err := s.repo.Queries().CountAuditEvents(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to count audit events: %w", err)
	}

	return &PaginatedAuditLogsResponse{
		Items: logs,
		Total: total,
		Page:  page,
		Limit: limit,
	}, nil
}

type SeedGeographyResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Zones   int64  `json:"zones"`
	States  int64  `json:"states"`
	LGAs    int64  `json:"lgas"`
}

func (s *AdminService) SeedGeography(ctx context.Context, actorID *string, actorRole *string, ipAddress *string) (*SeedGeographyResponse, error) {
	if err := seeder.SeedReferenceData(ctx, s.repo.Pool(), s.repo.Queries()); err != nil {
		return nil, err
	}
	zoneCount, err := s.repo.Queries().CountZones(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to count zones after seeding: %w", err)
	}
	stateCount, err := s.repo.Queries().CountStates(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to count states after seeding: %w", err)
	}
	lgaCount, err := s.repo.Queries().CountLGAs(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to count LGAs after seeding: %w", err)
	}

	if s.audit != nil {
		details := fmt.Sprintf("Authoritative reference geography verified: %d zones, %d states, %d LGAs synced", zoneCount, stateCount, lgaCount)
		scopeType := "national"
		_ = s.audit.Log(ctx, actorID, actorRole, "seed_geography", "geography", "national", &scopeType, nil, &details, ipAddress)
	}

	return &SeedGeographyResponse{
		Status:  "ok",
		Message: "Authoritative reference geography verified and seeded successfully",
		Zones:   zoneCount,
		States:  stateCount,
		LGAs:    lgaCount,
	}, nil
}
