package service

import (
	"context"

	"ratels-api/internal/db"
	"ratels-api/internal/repository"
)

type AdminService struct {
	repo *repository.Repository
}

func NewAdminService(repo *repository.Repository) *AdminService {
	return &AdminService{repo: repo}
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

func (s *AdminService) GetOverview(ctx context.Context, actorRole, actorJurisdictionType, actorJurisdictionName string) (*AdminOverviewResponse, error) {
	totalMembers, _ := s.repo.Queries().CountUsers(ctx)
	pendingVerif, _ := s.repo.Queries().CountPendingVerificationUsers(ctx)
	activeApps, _ := s.repo.Queries().CountActiveAppointments(ctx)
	pubCleanups, _ := s.repo.Queries().CountPublishedEvents(ctx)
	draftEvents, _ := s.repo.Queries().CountDraftEvents(ctx)
	pendingReviews, _ := s.repo.Queries().CountPendingClaims(ctx)
	pendingAppeals, _ := s.repo.Queries().CountPendingAppeals(ctx)
	totalRegs, _ := s.repo.Queries().CountTotalRegistrations(ctx)

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
		return nil, err
	}

	total, _ := s.repo.Queries().CountAuditEvents(ctx)

	return &PaginatedAuditLogsResponse{
		Items: logs,
		Total: total,
		Page:  page,
		Limit: limit,
	}, nil
}
