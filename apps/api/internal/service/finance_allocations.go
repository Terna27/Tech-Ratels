package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"ratels-api/internal/db"
	"ratels-api/internal/security"
)

type CreateAllocationRequest struct {
	SourceType        string  `json:"source_type"` // national, zone, state
	SourceID          *string `json:"source_id"`
	DestinationType   string  `json:"destination_type"` // zone, state, lga, community_project
	DestinationID     *string `json:"destination_id"`
	SourceFundID      string  `json:"source_fund_id"`
	DestinationFundID string  `json:"destination_fund_id"`
	AmountKobo        int64   `json:"amount_kobo"`
	Currency          string  `json:"currency"`
	Purpose           string  `json:"purpose"`
	ApprovalReference string  `json:"approval_reference"`
}

type AllocationFilterRequest struct {
	SourceType      *string `json:"source_type"`
	DestinationType *string `json:"destination_type"`
	Status          *string `json:"status"`
}

// CreateAllocation executes a controlled fund allocation between organizational tiers or projects.
func (s *FinanceService) CreateAllocation(ctx context.Context, actorID, actorRole string, req CreateAllocationRequest) (*db.FinancialAllocation, error) {
	hasAccess, accessLevel, _ := s.CheckFinancialAccess(ctx, actorID, actorRole)
	if !hasAccess || (accessLevel != "super_admin" && accessLevel != "finance_officer" && actorRole != "national_admin") {
		return nil, ErrFinancialUnauthorized
	}

	if req.AmountKobo <= 0 {
		return nil, errors.New("allocation amount must be greater than zero")
	}

	if req.Purpose == "" {
		return nil, errors.New("purpose is mandatory for fund allocations")
	}

	if req.Currency == "" {
		req.Currency = "NGN"
	}

	if req.SourceFundID == "" {
		req.SourceFundID = "general_operations_fund"
	}
	if req.DestinationFundID == "" {
		if req.DestinationType == "community_project" {
			req.DestinationFundID = "project_fund"
		} else {
			req.DestinationFundID = "general_operations_fund"
		}
	}

	if req.ApprovalReference == "" {
		req.ApprovalReference = fmt.Sprintf("ALC-APPR-%d-%s", time.Now().Year(), strings.ToUpper(security.GenerateUUID()[:6]))
	}

	allocID := security.GenerateUUID()
	ref := fmt.Sprintf("ALC-%d-%s", time.Now().Year(), strings.ToUpper(security.GenerateUUID()[:8]))
	txID := security.GenerateUUID()
	txRef := fmt.Sprintf("TXN-%s", ref)
	now := time.Now()

	var createdAlloc db.FinancialAllocation

	err := s.repo.WithTx(ctx, func(q *db.Queries) error {
		// 1. Post internal transfer transaction (is_external_income is FALSE: does not inflate revenue)
		txType := "internal_transfer"
		if req.DestinationType == "community_project" {
			txType = "project_allocation"
		}

		_, err := q.CreateFinancialTransactionWithIncome(ctx, db.CreateFinancialTransactionWithIncomeParams{
			ID:                     txID,
			Reference:              txRef,
			IdempotencyKey:         TextFromString(fmt.Sprintf("alloc-%s", allocID)),
			TransactionType:        txType,
			Status:                 "posted",
			Amount:                 req.AmountKobo,
			Currency:               req.Currency,
			SourceFundID:           TextFromString(req.SourceFundID),
			DestinationFundID:      TextFromString(req.DestinationFundID),
			RelatedCampaignID:      Text(nil),
			RelatedProjectID:       Text(req.DestinationID),
			StateID:                Text(nil),
			ZoneID:                 Text(nil),
			LgaID:                  Text(nil),
			ProviderReference:      TextFromString(req.ApprovalReference),
			SupportingEvidenceUrl:  Text(nil),
			Notes:                  TextFromString(fmt.Sprintf("Allocation: %s -> %s (%s)", req.SourceType, req.DestinationType, req.Purpose)),
			InitiatingActorID:      TextFromString(actorID),
			InitiatingActorType:    "user",
			ApprovedByUserID:       TextFromString(actorID),
			ApprovedAt:             TimestamptzFromTime(now),
			IsExternalIncome:       false, // CRITICAL: internal movement
			OccurredAt:             TimestamptzFromTime(now),
			ReversalOfTransactionID: Text(nil),
			IsReversal:             false,
			ReversalReason:         Text(nil),
			IncomeCategory:         Text(nil),
			PaymentMethod:          TextFromString("internal_ledger_allocation"),
			FundingSource:          TextFromString(req.SourceType),
			ReceivingAccountID:     Text(nil),
			VerificationStatus:     "verified",
			VerifiedByUserID:       TextFromString(actorID),
			VerifiedAt:             TimestamptzFromTime(now),
		})
		if err != nil {
			return err
		}

		// 2. Balanced Double-Entry Postings
		// Debit: Receiving fund
		_, err = q.CreateLedgerEntry(ctx, db.CreateLedgerEntryParams{
			ID:            security.GenerateUUID(),
			TransactionID: txID,
			FundID:        req.DestinationFundID,
			EntryType:     "debit",
			Amount:        req.AmountKobo,
			Currency:      req.Currency,
			Description:   TextFromString(fmt.Sprintf("Allocation received from %s", req.SourceType)),
		})
		if err != nil {
			return err
		}
		// Credit: Disbursing fund
		_, err = q.CreateLedgerEntry(ctx, db.CreateLedgerEntryParams{
			ID:            security.GenerateUUID(),
			TransactionID: txID,
			FundID:        req.SourceFundID,
			EntryType:     "credit",
			Amount:        req.AmountKobo,
			Currency:      req.Currency,
			Description:   TextFromString(fmt.Sprintf("Allocation disbursed to %s", req.DestinationType)),
		})
		if err != nil {
			return err
		}

		// 3. Create allocation record
		alloc, err := q.CreateAllocation(ctx, db.CreateAllocationParams{
			ID:                allocID,
			Reference:         ref,
			SourceType:        req.SourceType,
			SourceID:          Text(req.SourceID),
			DestinationType:   req.DestinationType,
			DestinationID:     Text(req.DestinationID),
			SourceFundID:      req.SourceFundID,
			DestinationFundID: req.DestinationFundID,
			AmountKobo:        req.AmountKobo,
			Currency:          req.Currency,
			Purpose:           req.Purpose,
			ApprovalReference: TextFromString(req.ApprovalReference),
			TransactionID:     TextFromString(txID),
			AllocatedByUserID: actorID,
			Status:            "completed",
		})
		if err != nil {
			return err
		}
		createdAlloc = alloc

		// 4. If destination is community project, atomically increment project's allocated_funding
		if req.DestinationType == "community_project" && req.DestinationID != nil && *req.DestinationID != "" {
			proj, err := q.GetCommunityProjectByID(ctx, *req.DestinationID)
			if err == nil {
				newAlloc := proj.AllocatedFunding + req.AmountKobo
				fStatus := proj.FundingStatus
				if newAlloc >= proj.ApprovedBudget && proj.ApprovedBudget > 0 {
					fStatus = "fully_funded"
				} else if newAlloc > 0 {
					fStatus = "partially_funded"
				}
				_, _ = q.UpdateCommunityProjectFinances(ctx, db.UpdateCommunityProjectFinancesParams{
					ID:                     proj.ID,
					ProposedBudget:         proj.ProposedBudget,
					ApprovedBudget:         proj.ApprovedBudget,
					AllocatedFunding:       newAlloc,
					RecordedExpenditure:    proj.RecordedExpenditure,
					OutstandingCommitments: proj.OutstandingCommitments,
					FundingStatus:          fStatus,
				})
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	details := fmt.Sprintf("Created fund allocation %s: %s -> %s (%d kobo) for %s", ref, req.SourceType, req.DestinationType, req.AmountKobo, req.Purpose)
	_ = s.audit.Log(ctx, &actorID, &actorRole, "create_allocation", "financial_allocation", allocID, nil, nil, &details, nil)

	return &createdAlloc, nil
}

// ListAllocations lists all recorded fund allocations.
func (s *FinanceService) ListAllocations(ctx context.Context, filter AllocationFilterRequest) ([]db.ListAllocationsFilteredRow, error) {
	return s.repo.Queries().ListAllocationsFiltered(ctx, db.ListAllocationsFilteredParams{
		SourceType:      Text(filter.SourceType),
		DestinationType: Text(filter.DestinationType),
		Status:          Text(filter.Status),
	})
}
