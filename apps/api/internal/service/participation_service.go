package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"ratels-api/internal/db"
	"ratels-api/internal/repository"
	"ratels-api/internal/security"
)

var (
	ErrClaimNotFound          = errors.New("participation claim not found")
	ErrSelfApprovalNotAllowed = errors.New("ANTI_SELF_APPROVAL: Coordinators cannot approve their own participation claims")
	ErrDuplicateClaim         = errors.New("participation claim already submitted for this event")
	ErrAppealNotAllowed       = errors.New("appeals can only be submitted for rejected claims")
)

type ParticipationService struct {
	repo  *repository.Repository
	audit *AuditService
	notif *NotificationService
}

func NewParticipationService(repo *repository.Repository, audit *AuditService) *ParticipationService {
	return &ParticipationService{repo: repo, audit: audit}
}

func (s *ParticipationService) SetNotificationService(notif *NotificationService) {
	s.notif = notif
}

type SubmitClaimRequest struct {
	EventID       string  `json:"event_id"`
	Description   string  `json:"description"`
	EvidenceNotes *string `json:"evidence_notes"`
	EvidenceURL   *string `json:"evidence_url"`
	EvidenceType  *string `json:"evidence_type"`
}

type ReviewClaimRequest struct {
	Decision string `json:"decision"` // approved, rejected
	Reason   string `json:"reason"`
}

type SubmitAppealRequest struct {
	Reason string `json:"reason"`
}

func (s *ParticipationService) SubmitClaim(ctx context.Context, userID string, req SubmitClaimRequest) (*db.ParticipationClaim, error) {
	// Verify event exists
	_, err := s.repo.Queries().GetEventInstanceByID(ctx, req.EventID)
	if err != nil {
		return nil, ErrEventNotFound
	}

	// Check if already submitted and pending or verified
	existing, err := s.repo.Queries().GetParticipationClaimByEventAndUser(ctx, db.GetParticipationClaimByEventAndUserParams{
		EventID: req.EventID,
		UserID:  userID,
	})
	if err == nil && existing.ID != "" {
		if existing.Status == "verified" || existing.Status == "approved" {
			return nil, ErrAlreadyVerified
		}
		if existing.Status == "pending" {
			return nil, ErrDuplicateClaim
		}
	}

	claimID := security.GenerateUUID()
	claim, err := s.repo.Queries().SubmitEvidenceCorrection(ctx, db.SubmitEvidenceCorrectionParams{
		ID:            claimID,
		EventID:       req.EventID,
		UserID:        userID,
		Description:   req.Description,
		EvidenceNotes: Text(req.EvidenceNotes),
		EvidenceUrl:   Text(req.EvidenceURL),
		EvidenceType:  Text(req.EvidenceType),
	})
	if err != nil {
		return nil, err
	}

	return &claim, nil
}

func (s *ParticipationService) ListReviewQueue(ctx context.Context, actorID string, actorRole string, actorJurisdictionID *string) (interface{}, error) {
	// 1. Super Admin has nationwide oversight
	u, err := s.repo.Queries().GetUserByID(ctx, actorID)
	if err == nil && (u.PrimaryRole == "super_admin") {
		return s.repo.Queries().ListPendingClaims(ctx)
	}

	// 2. Fetch active approved appointments from DB
	apps, err := s.repo.Queries().ListActiveApprovedAppointmentsByUser(ctx, actorID)
	if err != nil || len(apps) == 0 {
		if err == nil && u.PrimaryRole == "national_admin" {
			return s.repo.Queries().ListPendingClaims(ctx)
		}
		return nil, ErrCoordinatorUnauthorized
	}

	// 3. Filter review queue according to coordinator appointment jurisdiction
	for _, app := range apps {
		switch app.RoleType {
		case "super_admin", "national_admin":
			return s.repo.Queries().ListPendingClaims(ctx)
		case "zonal_coordinator":
			if app.JurisdictionID.Valid {
				return s.repo.Queries().ListPendingClaimsByZone(ctx, app.JurisdictionID.String)
			}
		case "state_coordinator":
			if app.JurisdictionID.Valid {
				return s.repo.Queries().ListPendingClaimsByState(ctx, app.JurisdictionID.String)
			}
		case "lga_coordinator", "lga_representative":
			if app.JurisdictionID.Valid {
				return s.repo.Queries().ListPendingClaimsByLGA(ctx, app.JurisdictionID.String)
			}
		}
	}

	return nil, ErrScopeUnauthorized
}

func (s *ParticipationService) ReviewClaim(ctx context.Context, reviewerID, reviewerRole string, reviewerJurisdictionID *string, claimID string, req ReviewClaimRequest) (*db.ParticipationClaim, error) {
	claim, err := s.repo.Queries().GetParticipationClaimByID(ctx, claimID)
	if err != nil {
		return nil, ErrClaimNotFound
	}

	// STRICT ANTI-SELF-APPROVAL INVARIANT
	if claim.UserID == reviewerID {
		return nil, ErrSelfApprovalNotAllowed
	}

	cleanReason := strings.TrimSpace(req.Reason)
	if len(cleanReason) < 5 {
		return nil, errors.New("decision reason must be at least 5 characters")
	}

	// Centralized database-backed coordinator authorization for the claim's event
	event, app, err := AuthorizeCoordinatorForEvent(ctx, s.repo, reviewerID, claim.EventID)
	if err != nil {
		// Fallback for super_admin or national_admin user role
		u, uErr := s.repo.Queries().GetUserByID(ctx, reviewerID)
		if uErr == nil && (u.PrimaryRole == "super_admin" || u.PrimaryRole == "national_admin") {
			ev, eErr := s.repo.Queries().GetEventInstanceByID(ctx, claim.EventID)
			if eErr != nil {
				return nil, ErrEventNotFound
			}
			event = &ev
			app = &db.Appointment{RoleType: u.PrimaryRole, JurisdictionType: "national"}
		} else {
			return nil, err
		}
	}

	decision := strings.TrimSpace(req.Decision)

	if decision != "approved" && decision != "rejected" {
		return nil, errors.New("decision must be approved or rejected")
	}

	status := "verified"
	if decision == "rejected" {
		status = "rejected"
	}

	var updatedClaim db.ParticipationClaim
	var awardedBadge *db.Badge

	err = s.repo.WithTx(ctx, func(q *db.Queries) error {
		p, err := q.ReviewParticipationClaim(ctx, db.ReviewParticipationClaimParams{
			ID:               claimID,
			Status:           status,
			ReviewedByUserID: TextFromString(reviewerID),
			ReviewDecision:   TextFromString(decision),
			ReviewReason:     TextFromString(cleanReason),
		})
		if err != nil {
			return err
		}
		updatedClaim = p

		if status == "verified" {
			// Update event registration if it exists
			_, _ = q.VerifyEventAttendance(ctx, db.VerifyEventAttendanceParams{
				EventID:            claim.EventID,
				UserID:             claim.UserID,
				VerifiedByUserID:   TextFromString(reviewerID),
				VerificationMethod: TextFromString("evidence_correction"),
			})

			// Check total verified participations for user
			verifiedCount, err := q.CountVerifiedParticipationsByUser(ctx, claim.UserID)
			if err != nil {
				return err
			}

			// If this is the user's first approved participation, award First Participation badge
			if verifiedCount == 1 {
				badge, err := q.GetBadgeByCode(ctx, "first-cleanup-ratel")
				if err == nil && badge.ID != "" {
					existingAward, _ := q.GetBadgeAward(ctx, db.GetBadgeAwardParams{
						UserID:  claim.UserID,
						BadgeID: badge.ID,
					})
					if existingAward.ID == "" {
						_, _ = q.CreateBadgeAward(ctx, db.CreateBadgeAwardParams{
							ID:                        security.GenerateUUID(),
							UserID:                    claim.UserID,
							BadgeID:                   badge.ID,
							QualifyingParticipationID: TextFromString(claimID),
							IsPublic:                  true,
						})
						awardedBadge = &badge
					}
				}
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	effectiveRole := app.RoleType
	scopeType := app.JurisdictionType
	details := fmt.Sprintf("Decision: %s. Reason: %s", decision, cleanReason)
	if awardedBadge != nil {
		details += fmt.Sprintf(" [Awarded Badge: %s]", awardedBadge.Name)
	}
	_ = s.audit.Log(ctx, &reviewerID, &effectiveRole, "review_participation", "participation_claim", claimID, &scopeType, &event.StateID, &details, nil)

	if s.notif != nil {
		if status == "verified" {
			s.notif.OnCorrectionApproved(ctx, s.repo.Queries(), claim.UserID, event.Title)
		} else {
			s.notif.OnCorrectionRejected(ctx, s.repo.Queries(), claim.UserID, event.Title, cleanReason)
		}
	}

	return &updatedClaim, nil
}

func (s *ParticipationService) ReverseClaim(ctx context.Context, actorID string, actorRole string, claimID string, reason string) (*db.ParticipationClaim, error) {
	// Strictly Super Admin authority
	u, err := s.repo.Queries().GetUserByID(ctx, actorID)
	if err != nil || (u.PrimaryRole != "super_admin") {
		return nil, ErrSuperAdminRequired
	}

	cleanReason := strings.TrimSpace(reason)
	if len(cleanReason) < 5 {
		return nil, errors.New("reversal reason must be at least 5 characters")
	}

	claim, err := s.repo.Queries().GetParticipationClaimByID(ctx, claimID)
	if err != nil {
		return nil, ErrClaimNotFound
	}

	if claim.Status == "reversed" {
		return nil, ErrClaimAlreadyReversed
	}
	if claim.Status != "verified" && claim.Status != "approved" {
		return nil, ErrClaimNotApproved
	}

	event, err := s.repo.Queries().GetEventInstanceByID(ctx, claim.EventID)
	if err != nil {
		return nil, ErrEventNotFound
	}

	var reversedClaim db.ParticipationClaim

	err = s.repo.WithTx(ctx, func(q *db.Queries) error {
		c, err := q.ReverseParticipationClaim(ctx, db.ReverseParticipationClaimParams{
			ID:               claimID,
			ReversedByUserID: TextFromString(actorID),
			ReversalReason:   TextFromString(cleanReason),
		})
		if err != nil {
			return err
		}
		reversedClaim = c

		// Reset event registration attendance status back to registered
		_, _ = q.ResetEventAttendance(ctx, db.ResetEventAttendanceParams{
			EventID: claim.EventID,
			UserID:  claim.UserID,
		})

		return nil
	})

	if err != nil {
		return nil, err
	}

	effectiveRole := "super_admin"
	scopeType := "national"
	details := fmt.Sprintf("Reversed participation verification for claim %s (Participant %s). Reason: %s", claimID, claim.UserID, cleanReason)
	_ = s.audit.Log(ctx, &actorID, &effectiveRole, "reverse_participation_verification", "participation_claim", claimID, &scopeType, &event.StateID, &details, nil)

	if s.notif != nil {
		s.notif.OnParticipationReversed(ctx, s.repo.Queries(), claim.UserID, event.Title, cleanReason)
	}

	return &reversedClaim, nil
}

func (s *ParticipationService) SubmitAppeal(ctx context.Context, userID, claimID string, req SubmitAppealRequest) (*db.ParticipationAppeal, error) {
	claim, err := s.repo.Queries().GetParticipationClaimByID(ctx, claimID)
	if err != nil {
		return nil, ErrClaimNotFound
	}

	if claim.UserID != userID {
		return nil, errors.New("unauthorized: claim does not belong to user")
	}

	if claim.Status != "rejected" {
		return nil, ErrAppealNotAllowed
	}

	appealID := security.GenerateUUID()
	appeal, err := s.repo.Queries().CreateParticipationAppeal(ctx, db.CreateParticipationAppealParams{
		ID:              appealID,
		ParticipationID: claimID,
		AppealReason:    req.Reason,
	})
	if err != nil {
		return nil, err
	}

	return &appeal, nil
}

func (s *ParticipationService) ListClaimsByUser(ctx context.Context, userID string) ([]db.ListClaimsByUserRow, error) {
	return s.repo.Queries().ListClaimsByUser(ctx, userID)
}

func (s *ParticipationService) ListUserBadges(ctx context.Context, userID string) ([]db.ListBadgeAwardsByUserRow, error) {
	return s.repo.Queries().ListBadgeAwardsByUser(ctx, userID)
}
