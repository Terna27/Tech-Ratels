package service

import (
	"context"
	"errors"
	"fmt"

	"ratels-api/internal/db"
	"ratels-api/internal/repository"
	"ratels-api/internal/security"
)

var (
	ErrClaimNotFound           = errors.New("participation claim not found")
	ErrSelfApprovalNotAllowed  = errors.New("ANTI_SELF_APPROVAL: Coordinators cannot approve their own participation claims")
	ErrDuplicateClaim          = errors.New("participation claim already submitted for this event")
	ErrAppealNotAllowed        = errors.New("appeals can only be submitted for rejected claims")
)

type ParticipationService struct {
	repo  *repository.Repository
	audit *AuditService
}

func NewParticipationService(repo *repository.Repository, audit *AuditService) *ParticipationService {
	return &ParticipationService{repo: repo, audit: audit}
}

type SubmitClaimRequest struct {
	EventID       string  `json:"event_id"`
	Description   string  `json:"description"`
	EvidenceNotes *string `json:"evidence_notes"`
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

	// Check duplicate claim
	existing, err := s.repo.Queries().GetParticipationClaimByEventAndUser(ctx, db.GetParticipationClaimByEventAndUserParams{
		EventID: req.EventID,
		UserID:  userID,
	})
	if err == nil && existing.ID != "" {
		return nil, ErrDuplicateClaim
	}

	claimID := security.GenerateUUID()
	claim, err := s.repo.Queries().CreateParticipationClaim(ctx, db.CreateParticipationClaimParams{
		ID:            claimID,
		EventID:       req.EventID,
		UserID:        userID,
		Description:   req.Description,
		EvidenceNotes: Text(req.EvidenceNotes),
	})
	if err != nil {
		return nil, err
	}

	return &claim, nil
}

func (s *ParticipationService) ListReviewQueue(ctx context.Context, actorRole string, actorJurisdictionID *string) (interface{}, error) {
	if actorRole == "state_coordinator" {
		if actorJurisdictionID == nil || *actorJurisdictionID == "" {
			return nil, ErrScopeUnauthorized
		}
		return s.repo.Queries().ListPendingClaimsByState(ctx, *actorJurisdictionID)
	}

	return s.repo.Queries().ListPendingClaims(ctx)
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

	// Scope verification for State Coordinator
	if reviewerRole == "state_coordinator" {
		event, err := s.repo.Queries().GetEventInstanceByID(ctx, claim.EventID)
		if err != nil || reviewerJurisdictionID == nil || *reviewerJurisdictionID != event.StateID {
			return nil, ErrScopeUnauthorized
		}
	} else if reviewerRole != "national_admin" {
		return nil, errors.New("unauthorized to review participation claims")
	}

	status := "verified"
	if req.Decision == "rejected" {
		status = "rejected"
	}

	var updatedClaim db.ParticipationClaim
	var awardedBadge *db.Badge

	err = s.repo.WithTx(ctx, func(q *db.Queries) error {
		p, err := q.ReviewParticipationClaim(ctx, db.ReviewParticipationClaimParams{
			ID:               claimID,
			Status:           status,
			ReviewedByUserID: TextFromString(reviewerID),
			ReviewDecision:   TextFromString(req.Decision),
			ReviewReason:     TextFromString(req.Reason),
		})
		if err != nil {
			return err
		}
		updatedClaim = p

		if status == "verified" {
			// Check total verified participations for user
			verifiedCount, err := q.CountVerifiedParticipationsByUser(ctx, claim.UserID)
			if err != nil {
				return err
			}

			// If this is the user's first approved participation, award First Participation badge!
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

	details := fmt.Sprintf("Decision: %s. Reason: %s", req.Decision, req.Reason)
	if awardedBadge != nil {
		details += fmt.Sprintf(" [Awarded Badge: %s]", awardedBadge.Name)
	}
	_ = s.audit.Log(ctx, &reviewerID, &reviewerRole, "review_participation", "participation_claim", claimID, nil, nil, &details, nil)

	return &updatedClaim, nil
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

func (s *ParticipationService) ListUserBadges(ctx context.Context, userID string) ([]db.ListBadgeAwardsByUserRow, error) {
	return s.repo.Queries().ListBadgeAwardsByUser(ctx, userID)
}
