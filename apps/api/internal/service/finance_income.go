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

type RecordIncomeRequest struct {
	Reference             string    `json:"reference"`
	IdempotencyKey        string    `json:"idempotency_key"`
	IncomeCategory        string    `json:"income_category"` // member_contribution, donation, approved_grant, sponsorship, fundraising_campaign, other
	AmountKobo            int64     `json:"amount_kobo"`
	Currency              string    `json:"currency"`
	DateReceived          time.Time `json:"date_received"`
	PaymentMethod         string    `json:"payment_method"` // bank_transfer, paystack, cash_deposit, cheque, other
	ReceivingAccountID    *string   `json:"receiving_account_id"`
	FundingSource         string    `json:"funding_source"`
	DestinationFundID     *string   `json:"destination_fund_id"`
	ZoneID                *string   `json:"zone_id"`
	StateID               *string   `json:"state_id"`
	LgaID                 *string   `json:"lga_id"`
	RelatedCampaignID     *string   `json:"related_campaign_id"`
	RelatedProjectID      *string   `json:"related_project_id"`
	SupportingDocumentURL *string   `json:"supporting_document_url"`
	Notes                 *string   `json:"notes"`
	ProviderReference     *string   `json:"provider_reference"`
	AutoPost              bool      `json:"auto_post"`
}

type VerifyIncomeRequest struct {
	Decision string `json:"decision"` // verified, rejected
	Notes    string `json:"notes"`
}

// RecordIncome registers a new income record with full geographic attribution and verification controls.
func (s *FinanceService) RecordIncome(ctx context.Context, actorID, actorRole string, req RecordIncomeRequest) (*db.FinancialTransaction, error) {
	if req.AmountKobo <= 0 {
		return nil, errors.New("income amount must be greater than zero")
	}

	if req.IncomeCategory == "" {
		req.IncomeCategory = "other"
	}

	if req.Currency == "" {
		req.Currency = "NGN"
	}

	if req.DateReceived.IsZero() {
		req.DateReceived = time.Now()
	}

	if req.Reference == "" {
		req.Reference = fmt.Sprintf("INC-%d-%s", time.Now().Year(), strings.ToUpper(security.GenerateUUID()[:8]))
	}

	destFund := "general_operations_fund"
	if req.DestinationFundID != nil && *req.DestinationFundID != "" {
		destFund = *req.DestinationFundID
	} else if req.IncomeCategory == "donation" {
		destFund = "foundation_donation_fund"
	}

	// Check idempotency key to prevent double recordings
	if req.IdempotencyKey != "" {
		existing, err := s.repo.Queries().GetFinancialTransactionByIdempotencyKey(ctx, TextFromString(req.IdempotencyKey))
		if err == nil && existing.ID != "" {
			return &existing, nil
		}
	}

	txID := security.GenerateUUID()
	txStatus := "pending"
	verStatus := "unverified"
	var verifiedBy pgtype.Text
	var verifiedAt pgtype.Timestamptz
	var approvedBy pgtype.Text
	var approvedAt pgtype.Timestamptz

	// Super Admin can choose to auto-verify and auto-post immediate income entries
	if req.AutoPost && (actorRole == "super_admin" || actorRole == "system") {
		txStatus = "posted"
		verStatus = "verified"
		now := time.Now()
		verifiedBy = TextFromString(actorID)
		verifiedAt = TimestamptzFromTime(now)
		approvedBy = TextFromString(actorID)
		approvedAt = TimestamptzFromTime(now)
	}

	var createdTx db.FinancialTransaction
	err := s.repo.WithTx(ctx, func(q *db.Queries) error {
		tx, err := q.CreateFinancialTransactionWithIncome(ctx, db.CreateFinancialTransactionWithIncomeParams{
			ID:                     txID,
			Reference:              req.Reference,
			IdempotencyKey:         TextFromString(req.IdempotencyKey),
			TransactionType:        "income_" + req.IncomeCategory,
			Status:                 txStatus,
			Amount:                 req.AmountKobo,
			Currency:               req.Currency,
			SourceFundID:           Text(nil),
			DestinationFundID:      TextFromString(destFund),
			RelatedCampaignID:      Text(req.RelatedCampaignID),
			RelatedProjectID:       Text(req.RelatedProjectID),
			StateID:                Text(req.StateID),
			ZoneID:                 Text(req.ZoneID),
			LgaID:                  Text(req.LgaID),
			ProviderReference:      Text(req.ProviderReference),
			SupportingEvidenceUrl:  Text(req.SupportingDocumentURL),
			Notes:                  Text(req.Notes),
			InitiatingActorID:      TextFromString(actorID),
			InitiatingActorType:    "user",
			ApprovedByUserID:       approvedBy,
			ApprovedAt:             approvedAt,
			IsExternalIncome:       true,
			OccurredAt:             TimestamptzFromTime(req.DateReceived),
			ReversalOfTransactionID: Text(nil),
			IsReversal:             false,
			ReversalReason:         Text(nil),
			IncomeCategory:         TextFromString(req.IncomeCategory),
			PaymentMethod:          TextFromString(req.PaymentMethod),
			FundingSource:          TextFromString(req.FundingSource),
			ReceivingAccountID:     Text(req.ReceivingAccountID),
			VerificationStatus:     verStatus,
			VerifiedByUserID:       verifiedBy,
			VerifiedAt:             verifiedAt,
		})
		if err != nil {
			if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique constraint") {
				return ErrDuplicateFinancialRecord
			}
			return err
		}
		createdTx = tx

		// If posted, write balanced double-entry entries
		if txStatus == "posted" {
			// Debit: Cash Asset in destination fund
			_, err = q.CreateLedgerEntry(ctx, db.CreateLedgerEntryParams{
				ID:            security.GenerateUUID(),
				TransactionID: txID,
				FundID:        destFund,
				EntryType:     "debit",
				Amount:        req.AmountKobo,
				Currency:      req.Currency,
				Description:   TextFromString(fmt.Sprintf("Cash receipt from %s (%s)", req.FundingSource, req.IncomeCategory)),
			})
			if err != nil {
				return err
			}
			// Credit: Recognized Income in destination fund
			_, err = q.CreateLedgerEntry(ctx, db.CreateLedgerEntryParams{
				ID:            security.GenerateUUID(),
				TransactionID: txID,
				FundID:        destFund,
				EntryType:     "credit",
				Amount:        req.AmountKobo,
				Currency:      req.Currency,
				Description:   TextFromString(fmt.Sprintf("Recognized income from %s (%s)", req.FundingSource, req.IncomeCategory)),
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

	details := fmt.Sprintf("Recorded income %s (%s, %d kobo) from %s with status %s", req.Reference, req.IncomeCategory, req.AmountKobo, req.FundingSource, txStatus)
	_ = s.audit.Log(ctx, &actorID, &actorRole, "record_income", "financial_transaction", txID, nil, nil, &details, nil)

	return &createdTx, nil
}

// VerifyIncome validates manual income entries, strictly prohibiting unauthorized self-verification.
func (s *FinanceService) VerifyIncome(ctx context.Context, verifierID, verifierRole, transactionID string, req VerifyIncomeRequest) (*db.FinancialTransaction, error) {
	hasAccess, accessLevel, _ := s.CheckFinancialAccess(ctx, verifierID, verifierRole)
	if !hasAccess || (accessLevel != "super_admin" && accessLevel != "finance_officer" && verifierRole != "national_admin") {
		return nil, ErrFinancialUnauthorized
	}

	tx, err := s.repo.Queries().GetFinancialTransactionByID(ctx, transactionID)
	if err != nil {
		return nil, ErrTransactionNotFound
	}

	if tx.VerificationStatus == "verified" || tx.Status == "posted" {
		return nil, ErrTransactionAlreadyFinalized
	}

	// Anti-self-approval rule: A submitter cannot verify their own submitted income
	if tx.InitiatingActorID.Valid && tx.InitiatingActorID.String == verifierID && verifierRole != "super_admin" {
		return nil, ErrSelfApprovalProhibited
	}

	now := time.Now()
	newStatus := "rejected"
	newVerStatus := "rejected"
	if req.Decision == "verified" || req.Decision == "approved" {
		newStatus = "posted"
		newVerStatus = "verified"
	}

	var updatedTx db.FinancialTransaction
	err = s.repo.WithTx(ctx, func(q *db.Queries) error {
		up, err := q.VerifyFinancialIncomeTransaction(ctx, db.VerifyFinancialIncomeTransactionParams{
			ID:                 transactionID,
			VerificationStatus: newVerStatus,
			Status:             newStatus,
			VerifiedByUserID:   TextFromString(verifierID),
			VerifiedAt:         TimestamptzFromTime(now),
		})
		if err != nil {
			return err
		}
		updatedTx = up

		// If verified and posted, write balanced double-entry ledger records
		if newStatus == "posted" {
			destFund := "general_operations_fund"
			if tx.DestinationFundID.Valid && tx.DestinationFundID.String != "" {
				destFund = tx.DestinationFundID.String
			}

			// Debit: Cash Asset in destination fund
			_, err = q.CreateLedgerEntry(ctx, db.CreateLedgerEntryParams{
				ID:            security.GenerateUUID(),
				TransactionID: transactionID,
				FundID:        destFund,
				EntryType:     "debit",
				Amount:        tx.Amount,
				Currency:      tx.Currency,
				Description:   TextFromString(fmt.Sprintf("Verified income deposit receipt for %s", tx.Reference)),
			})
			if err != nil {
				return err
			}
			// Credit: Recognized Revenue in destination fund
			_, err = q.CreateLedgerEntry(ctx, db.CreateLedgerEntryParams{
				ID:            security.GenerateUUID(),
				TransactionID: transactionID,
				FundID:        destFund,
				EntryType:     "credit",
				Amount:        tx.Amount,
				Currency:      tx.Currency,
				Description:   TextFromString(fmt.Sprintf("Verified revenue credit for %s", tx.Reference)),
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

	details := fmt.Sprintf("Income verification decision '%s' on %s by %s. Notes: %s", newVerStatus, tx.Reference, verifierID, req.Notes)
	_ = s.audit.Log(ctx, &verifierID, &verifierRole, "verify_income", "financial_transaction", transactionID, nil, nil, &details, nil)

	return &updatedTx, nil
}
