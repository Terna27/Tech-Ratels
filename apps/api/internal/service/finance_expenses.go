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

type CreateExpenseRequest struct {
	Title                      string  `json:"title"`
	Category                   string  `json:"category"`
	Description                string  `json:"description"`
	AmountKobo                 int64   `json:"amount_kobo"`
	Currency                   string  `json:"currency"`
	BeneficiaryName            string  `json:"beneficiary_name"`
	BeneficiaryBankName        *string `json:"beneficiary_bank_name"`
	BeneficiaryAccountNumber   *string `json:"beneficiary_account_number"`
	ZoneID                     *string `json:"zone_id"`
	StateID                    *string `json:"state_id"`
	LgaID                      *string `json:"lga_id"`
	RelatedProjectID           *string `json:"related_project_id"`
	RelatedCampaignID          *string `json:"related_campaign_id"`
	BudgetID                   *string `json:"budget_id"`
	FundID                     *string `json:"fund_id"`
	SupportingDocumentURL      *string `json:"supporting_document_url"`
}

type ReviewExpenseRequest struct {
	Decision        string  `json:"decision"` // under_review, approved, rejected
	RejectionReason *string `json:"rejection_reason"`
}

type PayExpenseRequest struct {
	PaymentReference   string  `json:"payment_reference"`
	ReceivingAccountID *string `json:"receiving_account_id"`
	ReceiptURL         *string `json:"receipt_url"`
}

type ExpenseFilterRequest struct {
	Category          *string `json:"category"`
	Status            *string `json:"status"`
	ZoneID            *string `json:"zone_id"`
	StateID           *string `json:"state_id"`
	LgaID             *string `json:"lga_id"`
	RelatedProjectID  *string `json:"related_project_id"`
	BudgetID          *string `json:"budget_id"`
	RequestedByUserID *string `json:"requested_by_user_id"`
}

// CreateExpense submits a new structured expenditure request.
func (s *FinanceService) CreateExpense(ctx context.Context, actorID, actorRole string, req CreateExpenseRequest) (*db.FinancialExpense, error) {
	if req.AmountKobo <= 0 {
		return nil, errors.New("expense amount must be greater than zero")
	}

	if req.Title == "" || req.Description == "" || req.BeneficiaryName == "" {
		return nil, errors.New("title, description, and beneficiary name are required")
	}

	validCategories := map[string]bool{
		"cleanup_materials":          true,
		"waste_collection_logistics": true,
		"transportation":             true,
		"community_projects":         true,
		"administrative_operations":  true,
		"equipment":                  true,
		"event_expenses":             true,
		"emergency":                  true,
		"other":                      true,
	}
	if !validCategories[req.Category] {
		req.Category = "other"
	}

	if req.Currency == "" {
		req.Currency = "NGN"
	}

	fundID := "general_operations_fund"
	if req.FundID != nil && *req.FundID != "" {
		fundID = *req.FundID
	} else if req.Category == "community_projects" {
		fundID = "project_fund"
	}

	// If budget is specified, check available budget
	if req.BudgetID != nil && *req.BudgetID != "" {
		budget, err := s.repo.Queries().GetBudgetByID(ctx, *req.BudgetID)
		if err == nil {
			available := budget.ApprovedAmountKobo - budget.SpentAmountKobo - budget.CommittedAmountKobo
			if available < req.AmountKobo {
				return nil, ErrBudgetExceeded
			}
		}
	}

	expenseID := security.GenerateUUID()
	ref := fmt.Sprintf("EXP-%d-%s", time.Now().Year(), strings.ToUpper(security.GenerateUUID()[:8]))

	created, err := s.repo.Queries().CreateExpense(ctx, db.CreateExpenseParams{
		ID:                         expenseID,
		Reference:                  ref,
		Title:                      req.Title,
		Category:                   req.Category,
		Description:                req.Description,
		AmountKobo:                 req.AmountKobo,
		Currency:                   req.Currency,
		BeneficiaryName:            req.BeneficiaryName,
		BeneficiaryBankName:        Text(req.BeneficiaryBankName),
		BeneficiaryAccountNumber:   Text(req.BeneficiaryAccountNumber),
		ZoneID:                     Text(req.ZoneID),
		StateID:                    Text(req.StateID),
		LgaID:                      Text(req.LgaID),
		RelatedProjectID:           Text(req.RelatedProjectID),
		RelatedCampaignID:          Text(req.RelatedCampaignID),
		SupportingDocumentUrl:      Text(req.SupportingDocumentURL),
		ReceiptUrl:                 Text(nil),
		RequestedByUserID:          actorID,
		BudgetID:                   Text(req.BudgetID),
		FundID:                     TextFromString(fundID),
		Status:                     "submitted",
	})
	if err != nil {
		return nil, err
	}

	// Update budget committed amount if linked
	if req.BudgetID != nil && *req.BudgetID != "" {
		_, _ = s.repo.Queries().UpdateBudgetAmounts(ctx, db.UpdateBudgetAmountsParams{
			ID:                  *req.BudgetID,
			AllocatedAmountKobo: 0,
			CommittedAmountKobo: req.AmountKobo,
			SpentAmountKobo:     0,
		})
	}

	details := fmt.Sprintf("Submitted expense request %s: %s (%d kobo) for %s", ref, req.Title, req.AmountKobo, req.BeneficiaryName)
	_ = s.audit.Log(ctx, &actorID, &actorRole, "create_expense", "financial_expense", expenseID, nil, nil, &details, nil)

	return &created, nil
}

// GetExpense returns an expense with full relational names.
func (s *FinanceService) GetExpense(ctx context.Context, expenseID string) (*db.GetExpenseByIDRow, error) {
	exp, err := s.repo.Queries().GetExpenseByID(ctx, expenseID)
	if err != nil {
		return nil, ErrExpenseNotFound
	}
	return &exp, nil
}

// ListExpenses retrieves filtered expense records.
func (s *FinanceService) ListExpenses(ctx context.Context, filter ExpenseFilterRequest) ([]db.ListExpensesFilteredRow, error) {
	return s.repo.Queries().ListExpensesFiltered(ctx, db.ListExpensesFilteredParams{
		Category:          Text(filter.Category),
		Status:            Text(filter.Status),
		ZoneID:            Text(filter.ZoneID),
		StateID:           Text(filter.StateID),
		LgaID:             Text(filter.LgaID),
		RelatedProjectID:  Text(filter.RelatedProjectID),
		BudgetID:          Text(filter.BudgetID),
		RequestedByUserID: Text(filter.RequestedByUserID),
	})
}

// ReviewExpense reviews an expense, strictly enforcing anti-self-approval.
func (s *FinanceService) ReviewExpense(ctx context.Context, reviewerID, reviewerRole, expenseID string, req ReviewExpenseRequest) (*db.FinancialExpense, error) {
	exp, err := s.repo.Queries().GetExpenseByID(ctx, expenseID)
	if err != nil {
		return nil, ErrExpenseNotFound
	}

	// Anti-self-approval rule: A user cannot approve their own expenditure
	if exp.RequestedByUserID == reviewerID {
		return nil, ErrSelfApprovalProhibited
	}

	hasAccess, accessLevel, _ := s.CheckFinancialAccess(ctx, reviewerID, reviewerRole)
	if !hasAccess || (accessLevel != "super_admin" && accessLevel != "finance_officer" && reviewerRole != "national_admin" && reviewerRole != "state_coordinator") {
		return nil, ErrFinancialUnauthorized
	}

	now := time.Now()
	var updated db.FinancialExpense

	switch req.Decision {
	case "under_review":
		updated, err = s.repo.Queries().UpdateExpenseReview(ctx, db.UpdateExpenseReviewParams{
			ID:              expenseID,
			Status:          "under_review",
			ReviewerID:      TextFromString(reviewerID),
			ReviewedAt:      TimestamptzFromTime(now),
			RejectionReason: Text(nil),
		})
	case "approved":
		updated, err = s.repo.Queries().UpdateExpenseApproval(ctx, db.UpdateExpenseApprovalParams{
			ID:               expenseID,
			Status:           "approved",
			ApprovedByUserID: TextFromString(reviewerID),
			ApprovedAt:       TimestamptzFromTime(now),
			RejectionReason:  Text(nil),
		})
	case "rejected":
		if req.RejectionReason == nil || strings.TrimSpace(*req.RejectionReason) == "" {
			return nil, errors.New("rejection reason is strictly required")
		}
		updated, err = s.repo.Queries().UpdateExpenseApproval(ctx, db.UpdateExpenseApprovalParams{
			ID:               expenseID,
			Status:           "rejected",
			ApprovedByUserID: TextFromString(reviewerID),
			ApprovedAt:       TimestamptzFromTime(now),
			RejectionReason:  Text(req.RejectionReason),
		})
		// Release commitment from budget if rejected
		if exp.BudgetID.Valid && exp.BudgetID.String != "" {
			_, _ = s.repo.Queries().UpdateBudgetAmounts(ctx, db.UpdateBudgetAmountsParams{
				ID:                  exp.BudgetID.String,
				AllocatedAmountKobo: 0,
				CommittedAmountKobo: -exp.AmountKobo,
				SpentAmountKobo:     0,
			})
		}
	default:
		return nil, errors.New("invalid decision; must be 'under_review', 'approved', or 'rejected'")
	}

	if err != nil {
		return nil, err
	}

	details := fmt.Sprintf("Reviewed expense %s: decision '%s' by %s", exp.Reference, req.Decision, reviewerID)
	_ = s.audit.Log(ctx, &reviewerID, &reviewerRole, "review_expense", "financial_expense", expenseID, nil, nil, &details, nil)

	return &updated, nil
}

// AuthorizeExpense moves an approved expense into the authorized for payment state.
func (s *FinanceService) AuthorizeExpense(ctx context.Context, authorizerID, authorizerRole, expenseID, notes string) (*db.FinancialExpense, error) {
	exp, err := s.repo.Queries().GetExpenseByID(ctx, expenseID)
	if err != nil {
		return nil, ErrExpenseNotFound
	}

	if exp.Status != "approved" {
		return nil, errors.New("only approved expenses can be authorized for payment")
	}

	// Super Admin or authorized Finance Officer can authorize payment
	hasAccess, accessLevel, _ := s.CheckFinancialAccess(ctx, authorizerID, authorizerRole)
	if !hasAccess || (accessLevel != "super_admin" && accessLevel != "finance_officer") {
		return nil, ErrFinancialApprovalRestricted
	}

	now := time.Now()
	updated, err := s.repo.Queries().UpdateExpenseAuthorization(ctx, db.UpdateExpenseAuthorizationParams{
		ID:                 expenseID,
		AuthorizedByUserID: TextFromString(authorizerID),
		AuthorizedAt:       TimestamptzFromTime(now),
	})
	if err != nil {
		return nil, err
	}

	details := fmt.Sprintf("Authorized expense %s for disbursement by %s", exp.Reference, authorizerID)
	_ = s.audit.Log(ctx, &authorizerID, &authorizerRole, "authorize_expense", "financial_expense", expenseID, nil, nil, &details, nil)

	return &updated, nil
}

// RecordExpensePayment marks the expense as paid and creates the authoritative double-entry transaction.
func (s *FinanceService) RecordExpensePayment(ctx context.Context, payerID, payerRole, expenseID string, req PayExpenseRequest) (*db.FinancialExpense, error) {
	exp, err := s.repo.Queries().GetExpenseByID(ctx, expenseID)
	if err != nil {
		return nil, ErrExpenseNotFound
	}

	if exp.Status != "authorized_for_payment" && exp.Status != "approved" {
		return nil, errors.New("expense must be approved or authorized for payment before recording payment")
	}

	hasAccess, accessLevel, _ := s.CheckFinancialAccess(ctx, payerID, payerRole)
	if !hasAccess || (accessLevel != "super_admin" && accessLevel != "finance_officer") {
		return nil, ErrFinancialUnauthorized
	}

	if req.PaymentReference == "" {
		req.PaymentReference = fmt.Sprintf("PAY-%s-%d", exp.Reference, time.Now().Unix())
	}

	txID := security.GenerateUUID()
	now := time.Now()
	var updatedExp db.FinancialExpense

	err = s.repo.WithTx(ctx, func(q *db.Queries) error {
		fundID := "general_operations_fund"
		if exp.FundID.Valid && exp.FundID.String != "" {
			fundID = exp.FundID.String
		}

		// 1. Create authoritative posted financial transaction
		_, err := q.CreateFinancialTransactionWithIncome(ctx, db.CreateFinancialTransactionWithIncomeParams{
			ID:                     txID,
			Reference:              req.PaymentReference,
			IdempotencyKey:         TextFromString(fmt.Sprintf("pay-exp-%s", exp.ID)),
			TransactionType:        "expense",
			Status:                 "posted",
			Amount:                 exp.AmountKobo,
			Currency:               exp.Currency,
			SourceFundID:           TextFromString(fundID),
			DestinationFundID:      Text(nil),
			RelatedCampaignID:      exp.RelatedCampaignID,
			RelatedProjectID:       exp.RelatedProjectID,
			StateID:                exp.StateID,
			ZoneID:                 exp.ZoneID,
			LgaID:                  exp.LgaID,
			ProviderReference:      TextFromString(req.PaymentReference),
			SupportingEvidenceUrl:  exp.SupportingDocumentUrl,
			Notes:                  TextFromString(fmt.Sprintf("Payment for %s to %s", exp.Title, exp.BeneficiaryName)),
			InitiatingActorID:      TextFromString(payerID),
			InitiatingActorType:    "user",
			ApprovedByUserID:       TextFromString(payerID),
			ApprovedAt:             TimestamptzFromTime(now),
			IsExternalIncome:       false,
			OccurredAt:             TimestamptzFromTime(now),
			ReversalOfTransactionID: Text(nil),
			IsReversal:             false,
			ReversalReason:         Text(nil),
			IncomeCategory:         Text(nil),
			PaymentMethod:          TextFromString("bank_transfer"),
			FundingSource:          TextFromString(exp.BeneficiaryName),
			ReceivingAccountID:     Text(req.ReceivingAccountID),
			VerificationStatus:     "verified",
			VerifiedByUserID:       TextFromString(payerID),
			VerifiedAt:             TimestamptzFromTime(now),
		})
		if err != nil {
			return err
		}

		// 2. Balanced Double-Entry Ledger Posting
		// Debit: Expense debit line
		_, err = q.CreateLedgerEntry(ctx, db.CreateLedgerEntryParams{
			ID:            security.GenerateUUID(),
			TransactionID: txID,
			FundID:        fundID,
			EntryType:     "debit",
			Amount:        exp.AmountKobo,
			Currency:      exp.Currency,
			Description:   TextFromString(fmt.Sprintf("Expense debit for %s (%s)", exp.Reference, exp.Category)),
		})
		if err != nil {
			return err
		}
		// Credit: Outflow credit line
		_, err = q.CreateLedgerEntry(ctx, db.CreateLedgerEntryParams{
			ID:            security.GenerateUUID(),
			TransactionID: txID,
			FundID:        fundID,
			EntryType:     "credit",
			Amount:        exp.AmountKobo,
			Currency:      exp.Currency,
			Description:   TextFromString(fmt.Sprintf("Cash disbursement for %s", exp.Reference)),
		})
		if err != nil {
			return err
		}

		// 3. Update expense status to paid
		up, err := q.UpdateExpensePayment(ctx, db.UpdateExpensePaymentParams{
			ID:               expenseID,
			PaidByUserID:     TextFromString(payerID),
			PaidAt:           TimestamptzFromTime(now),
			PaymentReference: TextFromString(req.PaymentReference),
			TransactionID:    TextFromString(txID),
		})
		if err != nil {
			return err
		}
		updatedExp = up

		// 4. Update Budget spent/committed amounts
		if exp.BudgetID.Valid && exp.BudgetID.String != "" {
			_, err = q.UpdateBudgetAmounts(ctx, db.UpdateBudgetAmountsParams{
				ID:                  exp.BudgetID.String,
				AllocatedAmountKobo: 0,
				CommittedAmountKobo: -exp.AmountKobo, // release commitment
				SpentAmountKobo:     exp.AmountKobo,  // record actual spent
			})
			if err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	details := fmt.Sprintf("Recorded disbursement for expense %s (Ref: %s, %d kobo)", exp.Reference, req.PaymentReference, exp.AmountKobo)
	_ = s.audit.Log(ctx, &payerID, &payerRole, "pay_expense", "financial_expense", expenseID, nil, nil, &details, nil)

	return &updatedExp, nil
}
