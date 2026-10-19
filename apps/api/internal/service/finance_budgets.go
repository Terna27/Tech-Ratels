package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"ratels-api/internal/db"
	"ratels-api/internal/security"
)

type CreateBudgetRequest struct {
	Title              string    `json:"title"`
	ScopeType          string    `json:"scope_type"` // national, zone, state, lga, community_project, cleanup_campaign, foundation
	ScopeID            *string   `json:"scope_id"`
	ZoneID             *string   `json:"zone_id"`
	StateID            *string   `json:"state_id"`
	LgaID              *string   `json:"lga_id"`
	PeriodType         string    `json:"period_type"` // quarterly, annual, project_based, event_based
	PeriodLabel        string    `json:"period_label"`
	PeriodStart        time.Time `json:"period_start"`
	PeriodEnd          time.Time `json:"period_end"`
	ProposedAmountKobo int64     `json:"proposed_amount_kobo"`
	Notes              *string   `json:"notes"`
}

type BudgetRevisionRequest struct {
	RevisedAmountKobo int64  `json:"revised_amount_kobo"`
	Reason            string `json:"reason"`
}

type BudgetFilterRequest struct {
	ScopeType   *string `json:"scope_type"`
	ScopeID     *string `json:"scope_id"`
	PeriodLabel *string `json:"period_label"`
	Status      *string `json:"status"`
	ZoneID      *string `json:"zone_id"`
	StateID     *string `json:"state_id"`
}

// CreateBudget creates a new budget proposal.
func (s *FinanceService) CreateBudget(ctx context.Context, actorID, actorRole string, req CreateBudgetRequest) (*db.FinancialBudget, error) {
	if req.Title == "" || req.PeriodLabel == "" {
		return nil, errors.New("title and period label are required")
	}

	if req.ProposedAmountKobo <= 0 {
		return nil, errors.New("proposed amount must be greater than zero")
	}

	if req.PeriodStart.IsZero() {
		req.PeriodStart = time.Now()
	}
	if req.PeriodEnd.IsZero() {
		req.PeriodEnd = req.PeriodStart.AddDate(1, 0, 0)
	}

	if req.PeriodType == "" {
		req.PeriodType = "annual"
	}

	validScopes := map[string]bool{
		"national":           true,
		"zone":               true,
		"state":              true,
		"lga":                true,
		"community_project":  true,
		"cleanup_campaign":   true,
		"foundation":         true,
	}
	if !validScopes[req.ScopeType] {
		req.ScopeType = "national"
	}

	status := "submitted"
	var approvedBy pgtype.Text
	var approvedAt pgtype.Timestamptz
	var approvedAmt int64 = 0

	if actorRole == "super_admin" {
		status = "approved"
		approvedBy = TextFromString(actorID)
		approvedAt = TimestamptzFromTime(time.Now())
		approvedAmt = req.ProposedAmountKobo
	}

	budgetID := security.GenerateUUID()
	ref := fmt.Sprintf("BUD-%d-%s", time.Now().Year(), strings.ToUpper(security.GenerateUUID()[:8]))

	budget, err := s.repo.Queries().CreateBudget(ctx, db.CreateBudgetParams{
		ID:                 budgetID,
		Reference:          ref,
		Title:              req.Title,
		ScopeType:          req.ScopeType,
		ScopeID:            Text(req.ScopeID),
		ZoneID:             Text(req.ZoneID),
		StateID:            Text(req.StateID),
		LgaID:              Text(req.LgaID),
		PeriodType:         req.PeriodType,
		PeriodLabel:        req.PeriodLabel,
		PeriodStart:        DateFromTime(&req.PeriodStart),
		PeriodEnd:          DateFromTime(&req.PeriodEnd),
		ProposedAmountKobo: req.ProposedAmountKobo,
		ApprovedAmountKobo: approvedAmt,
		AllocatedAmountKobo: 0,
		CommittedAmountKobo: 0,
		SpentAmountKobo:     0,
		Currency:           "NGN",
		Status:             status,
		CreatedByUserID:    actorID,
		ApprovedByUserID:   approvedBy,
		ApprovedAt:         approvedAt,
		Notes:              Text(req.Notes),
	})
	if err != nil {
		return nil, err
	}

	details := fmt.Sprintf("Created budget %s (%s, %d kobo) Status: %s", ref, req.Title, req.ProposedAmountKobo, status)
	_ = s.audit.Log(ctx, &actorID, &actorRole, "create_budget", "financial_budget", budgetID, nil, nil, &details, nil)

	return &budget, nil
}

// GetBudget retrieves a budget along with its revision history.
func (s *FinanceService) GetBudget(ctx context.Context, budgetID string) (*db.GetBudgetByIDRow, []db.ListBudgetRevisionsRow, error) {
	budget, err := s.repo.Queries().GetBudgetByID(ctx, budgetID)
	if err != nil {
		return nil, nil, ErrBudgetNotFound
	}

	revisions, err := s.repo.Queries().ListBudgetRevisions(ctx, budgetID)
	if err != nil {
		revisions = []db.ListBudgetRevisionsRow{}
	}

	return &budget, revisions, nil
}

// ListBudgets retrieves filtered budgets.
func (s *FinanceService) ListBudgets(ctx context.Context, filter BudgetFilterRequest) ([]db.ListBudgetsFilteredRow, error) {
	return s.repo.Queries().ListBudgetsFiltered(ctx, db.ListBudgetsFilteredParams{
		ScopeType:   Text(filter.ScopeType),
		ScopeID:     Text(filter.ScopeID),
		PeriodLabel: Text(filter.PeriodLabel),
		Status:      Text(filter.Status),
		ZoneID:      Text(filter.ZoneID),
		StateID:     Text(filter.StateID),
	})
}

// ApproveBudget approves a budget, strictly reserved for Super Admin authority.
func (s *FinanceService) ApproveBudget(ctx context.Context, superAdminID, superAdminRole, budgetID string) (*db.FinancialBudget, error) {
	if superAdminRole != "super_admin" {
		return nil, ErrFinancialApprovalRestricted
	}

	budget, err := s.repo.Queries().GetBudgetByID(ctx, budgetID)
	if err != nil {
		return nil, ErrBudgetNotFound
	}

	now := time.Now()
	approved, err := s.repo.Queries().UpdateBudgetStatus(ctx, db.UpdateBudgetStatusParams{
		ID:               budgetID,
		Status:           "approved",
		ApprovedByUserID: TextFromString(superAdminID),
		ApprovedAt:       TimestamptzFromTime(now),
	})
	if err != nil {
		return nil, err
	}

	details := fmt.Sprintf("Approved budget %s (%s, %d kobo)", budget.Reference, budget.Title, approved.ApprovedAmountKobo)
	_ = s.audit.Log(ctx, &superAdminID, &superAdminRole, "approve_budget", "financial_budget", budgetID, nil, nil, &details, nil)

	return &approved, nil
}

// RequestBudgetRevision logs an amendment request without overwriting previous amounts.
func (s *FinanceService) RequestBudgetRevision(ctx context.Context, actorID, actorRole, budgetID string, req BudgetRevisionRequest) (*db.FinancialBudgetRevision, error) {
	if req.RevisedAmountKobo <= 0 {
		return nil, errors.New("revised amount must be greater than zero")
	}
	if strings.TrimSpace(req.Reason) == "" {
		return nil, errors.New("revision reason is mandatory")
	}

	budget, err := s.repo.Queries().GetBudgetByID(ctx, budgetID)
	if err != nil {
		return nil, ErrBudgetNotFound
	}

	revisions, _ := s.repo.Queries().ListBudgetRevisions(ctx, budgetID)
	nextRevNum := int32(len(revisions) + 1)

	revID := security.GenerateUUID()
	status := "pending"
	var approvedBy pgtype.Text
	var approvedAt pgtype.Timestamptz

	// Super Admin can immediately approve their own revision
	if actorRole == "super_admin" {
		status = "approved"
		approvedBy = TextFromString(actorID)
		approvedAt = TimestamptzFromTime(time.Now())
	}

	revision, err := s.repo.Queries().CreateBudgetRevision(ctx, db.CreateBudgetRevisionParams{
		ID:                  revID,
		BudgetID:            budgetID,
		RevisionNumber:      nextRevNum,
		PreviousAmountKobo:  budget.ApprovedAmountKobo,
		RevisedAmountKobo:   req.RevisedAmountKobo,
		Reason:              req.Reason,
		RequestedByUserID:   actorID,
		ApprovedByUserID:    approvedBy,
		ApprovedAt:          approvedAt,
		Status:              status,
	})
	if err != nil {
		return nil, err
	}

	// If immediately approved, update budget
	if status == "approved" {
		_, _ = s.repo.Queries().UpdateBudgetStatus(ctx, db.UpdateBudgetStatusParams{
			ID:               budgetID,
			Status:           "approved",
			ApprovedByUserID: TextFromString(actorID),
			ApprovedAt:       TimestamptzFromTime(time.Now()),
		})
	}

	details := fmt.Sprintf("Requested budget revision #%d for %s: %d -> %d kobo. Reason: %s", nextRevNum, budget.Reference, budget.ApprovedAmountKobo, req.RevisedAmountKobo, req.Reason)
	_ = s.audit.Log(ctx, &actorID, &actorRole, "request_budget_revision", "financial_budget_revision", revID, nil, nil, &details, nil)

	return &revision, nil
}

// ApproveBudgetRevision commits a budget amendment, strictly reserved for Super Admin.
func (s *FinanceService) ApproveBudgetRevision(ctx context.Context, superAdminID, superAdminRole, revisionID string) (*db.FinancialBudgetRevision, error) {
	if superAdminRole != "super_admin" {
		return nil, ErrFinancialApprovalRestricted
	}

	now := time.Now()
	var updatedRev db.FinancialBudgetRevision

	err := s.repo.WithTx(ctx, func(q *db.Queries) error {
		rev, err := q.UpdateBudgetRevisionStatus(ctx, db.UpdateBudgetRevisionStatusParams{
			ID:               revisionID,
			Status:           "approved",
			ApprovedByUserID: TextFromString(superAdminID),
			ApprovedAt:       TimestamptzFromTime(now),
		})
		if err != nil {
			return err
		}
		updatedRev = rev

		// Update budget approved amount
		_, err = q.UpdateBudgetApprovedAmount(ctx, db.UpdateBudgetApprovedAmountParams{
			ID:                 rev.BudgetID,
			ApprovedAmountKobo: rev.RevisedAmountKobo,
			ApprovedByUserID:   TextFromString(superAdminID),
			ApprovedAt:         TimestamptzFromTime(now),
		})
		return err
	})

	if err != nil {
		return nil, err
	}

	details := fmt.Sprintf("Approved budget revision %s: revised amount %d kobo", revisionID, updatedRev.RevisedAmountKobo)
	_ = s.audit.Log(ctx, &superAdminID, &superAdminRole, "approve_budget_revision", "financial_budget_revision", revisionID, nil, nil, &details, nil)

	return &updatedRev, nil
}
