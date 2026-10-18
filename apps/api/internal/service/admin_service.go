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
	TotalMembers       int64 `json:"total_members"`
	ActiveAppointments int64 `json:"active_appointments"`
	PublishedCleanups  int64 `json:"published_cleanups"`
	PendingReviews     int64 `json:"pending_reviews"`
	TotalRegistrations int64 `json:"total_registrations"`
}

type PaginatedAuditLogsResponse struct {
	Items []db.AuditEvent `json:"items"`
	Total int64           `json:"total"`
	Page  int32           `json:"page"`
	Limit int32           `json:"limit"`
}

func (s *AdminService) GetOverview(ctx context.Context, actorRole string, actorJurisdictionID *string) (*AdminOverviewResponse, error) {
	totalMembers, _ := s.repo.Queries().CountUsers(ctx)
	activeApps, _ := s.repo.Queries().CountActiveAppointments(ctx)
	pubCleanups, _ := s.repo.Queries().CountPublishedEvents(ctx)
	pendingReviews, _ := s.repo.Queries().CountPendingClaims(ctx)
	totalRegs, _ := s.repo.Queries().CountTotalRegistrations(ctx)

	return &AdminOverviewResponse{
		TotalMembers:       totalMembers,
		ActiveAppointments: activeApps,
		PublishedCleanups:  pubCleanups,
		PendingReviews:     pendingReviews,
		TotalRegistrations: totalRegs,
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
