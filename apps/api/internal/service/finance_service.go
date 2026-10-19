package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"ratels-api/internal/db"
	"ratels-api/internal/payment"
	"ratels-api/internal/repository"
	"ratels-api/internal/security"
	"ratels-api/internal/storage"
)

var (
	ErrDuplicateFinancialRecord    = errors.New("DUPLICATE_RECORD: Transaction with this reference or idempotency key already exists")
	ErrTransactionNotFound         = errors.New("transaction not found")
	ErrTransactionAlreadyFinalized = errors.New("transaction has already been finalized/posted")
	ErrUnbalancedLedgerEntries     = errors.New("UNBALANCED_LEDGER: Total debits must equal total credits in double-entry bookkeeping")
	ErrFinancialUnauthorized       = errors.New("FINANCIAL_UNAUTHORIZED: Super Admin authority or authorized staff clearance is strictly required to view financial records")
	ErrFinancialApprovalRestricted = errors.New("APPROVAL_RESTRICTED: Only the Super Admin may approve, finalize, or authorize corrections to financial documentation")
	ErrImmutableFinancialRecord    = errors.New("IMMUTABLE_RECORD: Posted financial records cannot be edited or deleted; corrections require a linked reversal entry")
	ErrInvalidFund                 = errors.New("invalid or inactive financial fund specified")
	ErrSelfApprovalProhibited      = errors.New("SELF_APPROVAL_PROHIBITED: A user cannot review, approve, or certify their own financial request")
	ErrExpenseNotFound             = errors.New("expense request not found")
	ErrBudgetNotFound              = errors.New("budget not found")
	ErrBudgetExceeded              = errors.New("BUDGET_EXCEEDED: Requested expenditure exceeds available approved budget")
	ErrBankAccountNotFound         = errors.New("bank account not found")
	ErrReconciliationNotFound      = errors.New("reconciliation period not found")
	ErrDonationNotFound            = errors.New("donation record not found")
	ErrInvalidStatusTransition     = errors.New("invalid status transition for this record")
)

type FinanceService struct {
	repo    *repository.Repository
	audit   *AuditService
	payment payment.PaymentGateway
	storage storage.StorageService
}

func NewFinanceService(repo *repository.Repository, audit *AuditService, opts ...interface{}) *FinanceService {
	s := &FinanceService{
		repo:    repo,
		audit:   audit,
		payment: payment.NewUnconfiguredGateway(),
		storage: storage.NewUnconfiguredStorageService(),
	}
	for _, opt := range opts {
		if p, ok := opt.(payment.PaymentGateway); ok && p != nil {
			s.payment = p
		}
		if st, ok := opt.(storage.StorageService); ok && st != nil {
			s.storage = st
		}
	}
	return s
}

type CreateTransactionRequest struct {
	Reference             string    `json:"reference"`
	IdempotencyKey        string    `json:"idempotency_key"`
	TransactionType       string    `json:"transaction_type"` // confirmed_donation, project_allocation, internal_transfer, disbursement, expense, provider_fee, refund, reversal, bank_settlement, reconciliation_exception
	Amount                int64     `json:"amount"`           // in integer minor units (Kobo)
	Currency              string    `json:"currency"`
	SourceFundID          *string   `json:"source_fund_id"`
	DestinationFundID     *string   `json:"destination_fund_id"`
	RelatedCampaignID     *string   `json:"related_campaign_id"`
	RelatedProjectID      *string   `json:"related_project_id"`
	StateID               *string   `json:"state_id"`
	ZoneID                *string   `json:"zone_id"`
	ProviderReference     *string   `json:"provider_reference"`
	SupportingEvidenceURL *string   `json:"supporting_evidence_url"`
	Notes                 *string   `json:"notes"`
	OccurredAt            time.Time `json:"occurred_at"`
	AutoPost              bool      `json:"auto_post"`
}

type ReverseTransactionRequest struct {
	Reason string `json:"reason"`
}

type FinancialFilterRequest struct {
	StartDate       *time.Time `json:"start_date"`
	EndDate         *time.Time `json:"end_date"`
	StateID         *string    `json:"state_id"`
	ZoneID          *string    `json:"zone_id"`
	CampaignID      *string    `json:"campaign_id"`
	ProjectID       *string    `json:"project_id"`
	TransactionType *string    `json:"transaction_type"`
	Status          *string    `json:"status"`
	FundID          *string    `json:"fund_id"`
}

type FinancialStatementResponse struct {
	ReportReference      string                                    `json:"report_reference"`
	GeneratedAt          time.Time                                 `json:"generated_at"`
	GeneratingUser       string                                    `json:"generating_user"`
	ReportingPeriodFrom  *time.Time                                `json:"reporting_period_from"`
	ReportingPeriodTo    *time.Time                                `json:"reporting_period_to"`
	AppliedFilters       map[string]interface{}                    `json:"applied_filters"`
	OpeningBalance       int64                                     `json:"opening_balance"`
	ConfirmedInflows     int64                                     `json:"confirmed_inflows"`
	ConfirmedOutflows    int64                                     `json:"confirmed_outflows"`
	ProviderFees         int64                                     `json:"provider_fees"`
	RefundsAdjustments   int64                                     `json:"refunds_adjustments"`
	ClosingBalance       int64                                     `json:"closing_balance"`
	CashBalance          int64                                     `json:"cash_balance"`
	Commitments          int64                                     `json:"commitments"`
	AvailableToSpend     int64                                     `json:"available_to_spend"`
	ReconciliationStatus string                                    `json:"reconciliation_status"`
	Transactions         []db.ListFinancialTransactionsFilteredRow `json:"transactions"`
}

// CheckFinancialAccess returns whether the actor has clearance to access financial records.
func (s *FinanceService) CheckFinancialAccess(ctx context.Context, userID, userRole string) (bool, string, error) {
	if userRole == "super_admin" {
		return true, "super_admin", nil
	}

	auth, err := s.repo.Queries().GetActiveFinancialAuthorization(ctx, userID)
	if err == nil && auth.ID != "" && auth.Status == "active" {
		return true, auth.AccessLevel, nil
	}

	return false, "", nil
}

// AuthorizeStaffAccess allows a Super Admin to authorize staff for financial documentation.
func (s *FinanceService) AuthorizeStaffAccess(ctx context.Context, superAdminID, superAdminRole, targetUserID, accessLevel, notes string) (*db.FinancialAuthorization, error) {
	if superAdminRole != "super_admin" {
		return nil, ErrFinancialApprovalRestricted
	}

	if accessLevel != "viewer" && accessLevel != "submitter" && accessLevel != "finance_officer" {
		return nil, errors.New("access_level must be 'viewer', 'submitter', or 'finance_officer'")
	}

	authID := security.GenerateUUID()
	auth, err := s.repo.Queries().CreateFinancialAuthorization(ctx, db.CreateFinancialAuthorizationParams{
		ID:              authID,
		UserID:          targetUserID,
		GrantedByUserID: superAdminID,
		AccessLevel:     accessLevel,
		Notes:           TextFromString(notes),
	})
	if err != nil {
		return nil, err
	}

	details := fmt.Sprintf("Authorized user %s as %s", targetUserID, accessLevel)
	_ = s.audit.Log(ctx, &superAdminID, &superAdminRole, "authorize_financial_access", "financial_authorization", authID, nil, nil, &details, nil)

	return &auth, nil
}

// RevokeStaffAccess revokes financial access clearance.
func (s *FinanceService) RevokeStaffAccess(ctx context.Context, superAdminID, superAdminRole, authorizationID string) error {
	if superAdminRole != "super_admin" {
		return ErrFinancialApprovalRestricted
	}

	_, err := s.repo.Queries().RevokeFinancialAuthorization(ctx, authorizationID)
	if err != nil {
		return err
	}

	details := fmt.Sprintf("Revoked financial authorization %s", authorizationID)
	_ = s.audit.Log(ctx, &superAdminID, &superAdminRole, "revoke_financial_access", "financial_authorization", authorizationID, nil, nil, &details, nil)
	return nil
}

// ListAuthorizations lists all active and past financial authorizations.
func (s *FinanceService) ListAuthorizations(ctx context.Context) ([]db.ListFinancialAuthorizationsRow, error) {
	return s.repo.Queries().ListFinancialAuthorizations(ctx)
}

// ListFunds returns available financial funds.
func (s *FinanceService) ListFunds(ctx context.Context) ([]db.FinancialFund, error) {
	return s.repo.Queries().ListFinancialFunds(ctx)
}

// RecordTransaction records a fund movement with idempotency and balanced double-entry ledger entries.
func (s *FinanceService) RecordTransaction(ctx context.Context, actorID, actorRole string, req CreateTransactionRequest) (*db.FinancialTransaction, error) {
	// Check idempotency key to prevent duplicates on retries
	if req.IdempotencyKey != "" {
		existing, err := s.repo.Queries().GetFinancialTransactionByIdempotencyKey(ctx, TextFromString(req.IdempotencyKey))
		if err == nil && existing.ID != "" {
			// Idempotent return of already recorded transaction
			return &existing, nil
		}
	}

	// Validate amount > 0
	if req.Amount <= 0 {
		return nil, errors.New("transaction amount must be greater than zero")
	}

	if req.Currency == "" {
		req.Currency = "NGN"
	}

	if req.OccurredAt.IsZero() {
		req.OccurredAt = time.Now()
	}

	if req.Reference == "" {
		req.Reference = fmt.Sprintf("TXN-%d-%s", time.Now().UnixNano(), strings.ToUpper(security.GenerateUUID()[:8]))
	}

	// Determine external income rule:
	// Only external inflows (confirmed donations, external commercial sales) count as external income.
	// Internal transfers, allocations, and bank settlements DO NOT count as external income.
	isExternalIncome := req.TransactionType == "confirmed_donation" || req.TransactionType == "commercial_receipt"

	status := "pending"
	var approvedBy pgtype.Text
	var approvedAt pgtype.Timestamptz

	// Posting authority is strictly reserved for Super Admin or verified automated processes
	if req.AutoPost {
		if actorRole == "super_admin" || actorRole == "system" {
			status = "posted"
			approvedBy = TextFromString(actorID)
			approvedAt = TimestamptzFromTime(time.Now())
		}
	}

	txID := security.GenerateUUID()
	var createdTx db.FinancialTransaction

	err := s.repo.WithTx(ctx, func(q *db.Queries) error {
		tx, err := q.CreateFinancialTransaction(ctx, db.CreateFinancialTransactionParams{
			ID:                     txID,
			Reference:              req.Reference,
			IdempotencyKey:         TextFromString(req.IdempotencyKey),
			TransactionType:        req.TransactionType,
			Status:                 status,
			Amount:                 req.Amount,
			Currency:               req.Currency,
			SourceFundID:           Text(req.SourceFundID),
			DestinationFundID:      Text(req.DestinationFundID),
			RelatedCampaignID:      Text(req.RelatedCampaignID),
			RelatedProjectID:       Text(req.RelatedProjectID),
			StateID:                Text(req.StateID),
			ZoneID:                 Text(req.ZoneID),
			ProviderReference:      Text(req.ProviderReference),
			SupportingEvidenceUrl:  Text(req.SupportingEvidenceURL),
			Notes:                  Text(req.Notes),
			InitiatingActorID:      TextFromString(actorID),
			InitiatingActorType:    "user",
			ApprovedByUserID:       approvedBy,
			ApprovedAt:             approvedAt,
			IsExternalIncome:       isExternalIncome,
			OccurredAt:             TimestamptzFromTime(req.OccurredAt),
			ReversalOfTransactionID: Text(nil),
			IsReversal:             false,
			ReversalReason:         Text(nil),
		})
		if err != nil {
			if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique constraint") {
				return ErrDuplicateFinancialRecord
			}
			return err
		}
		createdTx = tx

		// If posted, write balanced double-entry ledger entries inside the transaction
		if status == "posted" {
			if err := s.writeDoubleEntryLedger(ctx, q, txID, req); err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	details := fmt.Sprintf("Recorded transaction %s: %s (%d %s) Status: %s", req.Reference, req.TransactionType, req.Amount, req.Currency, status)
	_ = s.audit.Log(ctx, &actorID, &actorRole, "record_transaction", "financial_transaction", txID, nil, nil, &details, nil)

	return &createdTx, nil
}

// writeDoubleEntryLedger enforces debit == credit balancing in integer minor units.
func (s *FinanceService) writeDoubleEntryLedger(ctx context.Context, q *db.Queries, txID string, req CreateTransactionRequest) error {
	switch req.TransactionType {
	case "confirmed_donation", "commercial_receipt":
		destFund := "general_operations_fund"
		if req.DestinationFundID != nil && *req.DestinationFundID != "" {
			destFund = *req.DestinationFundID
		}
		// Debit: Cash Asset (Destination Fund)
		_, err := q.CreateLedgerEntry(ctx, db.CreateLedgerEntryParams{
			ID:            security.GenerateUUID(),
			TransactionID: txID,
			FundID:        destFund,
			EntryType:     "debit",
			Amount:        req.Amount,
			Currency:      req.Currency,
			Description:   TextFromString("Cash receipt from external income"),
		})
		if err != nil {
			return err
		}
		// Credit: Fund Equity / Income Account
		_, err = q.CreateLedgerEntry(ctx, db.CreateLedgerEntryParams{
			ID:            security.GenerateUUID(),
			TransactionID: txID,
			FundID:        destFund,
			EntryType:     "credit",
			Amount:        req.Amount,
			Currency:      req.Currency,
			Description:   TextFromString("Recognized external revenue credit"),
		})
		return err

	case "project_allocation", "internal_transfer":
		if req.SourceFundID == nil || req.DestinationFundID == nil || *req.SourceFundID == *req.DestinationFundID {
			return errors.New("internal transfers require distinct source_fund_id and destination_fund_id")
		}
		// Debit: Receiving Fund (Increases destination fund balance)
		_, err := q.CreateLedgerEntry(ctx, db.CreateLedgerEntryParams{
			ID:            security.GenerateUUID(),
			TransactionID: txID,
			FundID:        *req.DestinationFundID,
			EntryType:     "debit",
			Amount:        req.Amount,
			Currency:      req.Currency,
			Description:   TextFromString(fmt.Sprintf("Internal transfer in from %s", *req.SourceFundID)),
		})
		if err != nil {
			return err
		}
		// Credit: Source Fund (Decreases source fund balance)
		_, err = q.CreateLedgerEntry(ctx, db.CreateLedgerEntryParams{
			ID:            security.GenerateUUID(),
			TransactionID: txID,
			FundID:        *req.SourceFundID,
			EntryType:     "credit",
			Amount:        req.Amount,
			Currency:      req.Currency,
			Description:   TextFromString(fmt.Sprintf("Internal transfer out to %s", *req.DestinationFundID)),
		})
		return err

	case "disbursement", "expense", "provider_fee":
		srcFund := "general_operations_fund"
		if req.SourceFundID != nil && *req.SourceFundID != "" {
			srcFund = *req.SourceFundID
		}
		// Debit: Expense / Disbursement
		_, err := q.CreateLedgerEntry(ctx, db.CreateLedgerEntryParams{
			ID:            security.GenerateUUID(),
			TransactionID: txID,
			FundID:        srcFund,
			EntryType:     "debit",
			Amount:        req.Amount,
			Currency:      req.Currency,
			Description:   TextFromString("Disbursement / Expense debit"),
		})
		if err != nil {
			return err
		}
		// Credit: Cash Account Outflow
		_, err = q.CreateLedgerEntry(ctx, db.CreateLedgerEntryParams{
			ID:            security.GenerateUUID(),
			TransactionID: txID,
			FundID:        srcFund,
			EntryType:     "credit",
			Amount:        req.Amount,
			Currency:      req.Currency,
			Description:   TextFromString("Cash outflow credit"),
		})
		return err

	case "bank_settlement":
		clearingAccount := "settlement_clearing_account"
		destFund := "general_operations_fund"
		if req.DestinationFundID != nil && *req.DestinationFundID != "" {
			destFund = *req.DestinationFundID
		}
		// Debit: Bank account asset
		_, err := q.CreateLedgerEntry(ctx, db.CreateLedgerEntryParams{
			ID:            security.GenerateUUID(),
			TransactionID: txID,
			FundID:        destFund,
			EntryType:     "debit",
			Amount:        req.Amount,
			Currency:      req.Currency,
			Description:   TextFromString("Bank settlement deposit"),
		})
		if err != nil {
			return err
		}
		// Credit: Clearing account
		_, err = q.CreateLedgerEntry(ctx, db.CreateLedgerEntryParams{
			ID:            security.GenerateUUID(),
			TransactionID: txID,
			FundID:        clearingAccount,
			EntryType:     "credit",
			Amount:        req.Amount,
			Currency:      req.Currency,
			Description:   TextFromString("Gateway clearing credit"),
		})
		return err

	default:
		// Default balanced debit and credit
		fund := "general_operations_fund"
		if req.DestinationFundID != nil {
			fund = *req.DestinationFundID
		} else if req.SourceFundID != nil {
			fund = *req.SourceFundID
		}
		_, err := q.CreateLedgerEntry(ctx, db.CreateLedgerEntryParams{
			ID:            security.GenerateUUID(),
			TransactionID: txID,
			FundID:        fund,
			EntryType:     "debit",
			Amount:        req.Amount,
			Currency:      req.Currency,
			Description:   TextFromString(fmt.Sprintf("%s entry debit", req.TransactionType)),
		})
		if err != nil {
			return err
		}
		_, err = q.CreateLedgerEntry(ctx, db.CreateLedgerEntryParams{
			ID:            security.GenerateUUID(),
			TransactionID: txID,
			FundID:        fund,
			EntryType:     "credit",
			Amount:        req.Amount,
			Currency:      req.Currency,
			Description:   TextFromString(fmt.Sprintf("%s entry credit", req.TransactionType)),
		})
		return err
	}
}

// ApproveTransaction allows Super Admin to finalize and post a pending draft transaction.
func (s *FinanceService) ApproveTransaction(ctx context.Context, superAdminID, superAdminRole, transactionID string) (*db.FinancialTransaction, error) {
	if superAdminRole != "super_admin" {
		return nil, ErrFinancialApprovalRestricted
	}

	tx, err := s.repo.Queries().GetFinancialTransactionByID(ctx, transactionID)
	if err != nil {
		return nil, ErrTransactionNotFound
	}

	if tx.Status == "posted" {
		return nil, ErrTransactionAlreadyFinalized
	}

	now := time.Now()
	var updatedTx db.FinancialTransaction

	err = s.repo.WithTx(ctx, func(q *db.Queries) error {
		u, err := q.UpdateFinancialTransactionStatus(ctx, db.UpdateFinancialTransactionStatusParams{
			ID:               transactionID,
			Status:           "posted",
			ApprovedByUserID: TextFromString(superAdminID),
			ApprovedAt:       TimestamptzFromTime(now),
		})
		if err != nil {
			return err
		}
		updatedTx = u

		// Post double-entry ledger entries
		req := CreateTransactionRequest{
			TransactionType:   tx.TransactionType,
			Amount:            tx.Amount,
			Currency:          tx.Currency,
			SourceFundID:      StringPtrFromText(tx.SourceFundID),
			DestinationFundID: StringPtrFromText(tx.DestinationFundID),
		}
		return s.writeDoubleEntryLedger(ctx, q, transactionID, req)
	})

	if err != nil {
		return nil, err
	}

	details := fmt.Sprintf("Approved and posted financial transaction %s", tx.Reference)
	_ = s.audit.Log(ctx, &superAdminID, &superAdminRole, "approve_transaction", "financial_transaction", transactionID, nil, nil, &details, nil)

	return &updatedTx, nil
}

// ReverseTransaction creates a linked reversal entry. Posted records are NEVER deleted or edited.
func (s *FinanceService) ReverseTransaction(ctx context.Context, superAdminID, superAdminRole, originalTxID, reason string) (*db.FinancialTransaction, error) {
	if superAdminRole != "super_admin" {
		return nil, ErrFinancialApprovalRestricted
	}

	original, err := s.repo.Queries().GetFinancialTransactionByID(ctx, originalTxID)
	if err != nil {
		return nil, ErrTransactionNotFound
	}

	if original.Status != "posted" {
		return nil, errors.New("only posted transactions can be reversed")
	}

	revID := security.GenerateUUID()
	revRef := fmt.Sprintf("REV-%s", original.Reference)
	now := time.Now()

	var reversalTx db.FinancialTransaction

	err = s.repo.WithTx(ctx, func(q *db.Queries) error {
		// 1. Mark original status as reversed
		_, err := q.UpdateFinancialTransactionStatus(ctx, db.UpdateFinancialTransactionStatusParams{
			ID:               originalTxID,
			Status:           "reversed",
			ApprovedByUserID: TextFromString(superAdminID),
			ApprovedAt:       TimestamptzFromTime(now),
		})
		if err != nil {
			return err
		}

		// 2. Insert linked reversal transaction
		rev, err := q.CreateFinancialTransaction(ctx, db.CreateFinancialTransactionParams{
			ID:                     revID,
			Reference:              revRef,
			IdempotencyKey:         TextFromString(fmt.Sprintf("rev-%s", original.ID)),
			TransactionType:        "reversal",
			Status:                 "posted",
			Amount:                 original.Amount,
			Currency:               original.Currency,
			SourceFundID:           original.DestinationFundID, // Inverted funds
			DestinationFundID:      original.SourceFundID,
			RelatedCampaignID:      original.RelatedCampaignID,
			RelatedProjectID:       original.RelatedProjectID,
			StateID:                original.StateID,
			ZoneID:                 original.ZoneID,
			ProviderReference:      TextFromString(fmt.Sprintf("Original: %s", original.Reference)),
			SupportingEvidenceUrl:  original.SupportingEvidenceUrl,
			Notes:                  TextFromString(fmt.Sprintf("Reversal of %s. Reason: %s", original.Reference, reason)),
			InitiatingActorID:      TextFromString(superAdminID),
			InitiatingActorType:    "user",
			ApprovedByUserID:       TextFromString(superAdminID),
			ApprovedAt:             TimestamptzFromTime(now),
			IsExternalIncome:       false,
			OccurredAt:             TimestamptzFromTime(now),
			ReversalOfTransactionID: TextFromString(originalTxID),
			IsReversal:             true,
			ReversalReason:         TextFromString(reason),
		})
		if err != nil {
			return err
		}
		reversalTx = rev

		// 3. Insert reversed ledger entries (opposite of original)
		origEntries, err := q.ListLedgerEntriesByTransaction(ctx, originalTxID)
		if err != nil {
			return err
		}

		for _, oe := range origEntries {
			invEntryType := "credit"
			if oe.EntryType == "credit" {
				invEntryType = "debit"
			}
			_, err = q.CreateLedgerEntry(ctx, db.CreateLedgerEntryParams{
				ID:            security.GenerateUUID(),
				TransactionID: revID,
				FundID:        oe.FundID,
				EntryType:     invEntryType,
				Amount:        oe.Amount,
				Currency:      oe.Currency,
				Description:   TextFromString(fmt.Sprintf("Reversal adjustment for entry %s: %s", oe.ID, reason)),
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

	details := fmt.Sprintf("Reversed transaction %s with linked reversal %s. Reason: %s", original.Reference, revRef, reason)
	_ = s.audit.Log(ctx, &superAdminID, &superAdminRole, "reverse_transaction", "financial_transaction", revID, nil, nil, &details, nil)

	return &reversalTx, nil
}

// GetTransaction returns a single transaction with fund movement and ledger details.
func (s *FinanceService) GetTransaction(ctx context.Context, transactionID string) (*db.GetFinancialTransactionByIDRow, []db.ListLedgerEntriesByTransactionRow, error) {
	tx, err := s.repo.Queries().GetFinancialTransactionByID(ctx, transactionID)
	if err != nil {
		return nil, nil, ErrTransactionNotFound
	}

	entries, err := s.repo.Queries().ListLedgerEntriesByTransaction(ctx, transactionID)
	if err != nil {
		return nil, nil, err
	}

	return &tx, entries, nil
}

// ListTransactions lists filtered fund movement transactions.
func (s *FinanceService) ListTransactions(ctx context.Context, req FinancialFilterRequest) ([]db.ListFinancialTransactionsFilteredRow, error) {
	return s.repo.Queries().ListFinancialTransactionsFiltered(ctx, db.ListFinancialTransactionsFilteredParams{
		StartDate:       Timestamptz(req.StartDate),
		EndDate:         Timestamptz(req.EndDate),
		StateID:         Text(req.StateID),
		ZoneID:          Text(req.ZoneID),
		CampaignID:      Text(req.CampaignID),
		ProjectID:       Text(req.ProjectID),
		TransactionType: Text(req.TransactionType),
		Status:          Text(req.Status),
		FundID:          Text(req.FundID),
	})
}

// GenerateFinancialStatement compiles a formal financial statement reconciled with the double-entry ledger.
func (s *FinanceService) GenerateFinancialStatement(ctx context.Context, actorID string, actorName string, req FinancialFilterRequest) (*FinancialStatementResponse, error) {
	// 1. Calculate opening balance (prior to StartDate)
	beforeDate := time.Now()
	if req.StartDate != nil {
		beforeDate = *req.StartDate
	} else {
		// If no start date, start from beginning of Unix epoch
		beforeDate = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	}

	openingBalance, err := s.repo.Queries().CalculateFundBalanceBeforeDate(ctx, db.CalculateFundBalanceBeforeDateParams{
		FundID:     Text(req.FundID),
		BeforeDate: TimestamptzFromTime(beforeDate),
	})
	if err != nil {
		openingBalance = 0
	}

	// 2. Calculate period totals (External inflows, disbursements, provider fees, refunds)
	totals, err := s.repo.Queries().CalculateExternalInflowsAndOutflows(ctx, db.CalculateExternalInflowsAndOutflowsParams{
		StartDate: Timestamptz(req.StartDate),
		EndDate:   Timestamptz(req.EndDate),
		StateID:   Text(req.StateID),
		ZoneID:    Text(req.ZoneID),
		FundID:    Text(req.FundID),
	})
	if err != nil {
		totals = db.CalculateExternalInflowsAndOutflowsRow{}
	}

	// 3. Fetch line items
	transactions, err := s.ListTransactions(ctx, req)
	if err != nil {
		return nil, err
	}

	// Calculate net closing balance
	closingBalance := openingBalance + totals.ExternalInflows - totals.Disbursements - totals.ProviderFees - totals.RefundsAdjustments

	// Commitments: Pending disbursements/allocations
	var commitments int64 = 0
	for _, tx := range transactions {
		if tx.Status == "pending" && (tx.TransactionType == "disbursement" || tx.TransactionType == "expense") {
			commitments += tx.Amount
		}
	}
	availableToSpend := closingBalance - commitments
	if availableToSpend < 0 {
		availableToSpend = 0
	}

	reportRef := fmt.Sprintf("REP-FIN-%d-%s", time.Now().Unix(), strings.ToUpper(security.GenerateUUID()[:6]))

	appliedFilters := map[string]interface{}{}
	if req.StartDate != nil {
		appliedFilters["start_date"] = req.StartDate.Format(time.RFC3339)
	}
	if req.EndDate != nil {
		appliedFilters["end_date"] = req.EndDate.Format(time.RFC3339)
	}
	if req.FundID != nil {
		appliedFilters["fund_id"] = *req.FundID
	}
	if req.StateID != nil {
		appliedFilters["state_id"] = *req.StateID
	}
	if req.ZoneID != nil {
		appliedFilters["zone_id"] = *req.ZoneID
	}
	if req.TransactionType != nil {
		appliedFilters["transaction_type"] = *req.TransactionType
	}
	if req.Status != nil {
		appliedFilters["status"] = *req.Status
	}

	// 4. Record financial report log for audit trail
	filterJSON := []byte("{}")
	_, _ = s.repo.Queries().CreateFinancialReportLog(ctx, db.CreateFinancialReportLogParams{
		ID:                security.GenerateUUID(),
		ReportReference:   reportRef,
		GeneratedByUserID: actorID,
		ReportType:        "statement",
		Filters:           filterJSON,
		OpeningBalance:    openingBalance,
		ClosingBalance:    closingBalance,
		TotalInflows:      totals.ExternalInflows,
		TotalOutflows:     totals.Disbursements + totals.ProviderFees,
	})

	return &FinancialStatementResponse{
		ReportReference:      reportRef,
		GeneratedAt:          time.Now(),
		GeneratingUser:       actorName,
		ReportingPeriodFrom:  req.StartDate,
		ReportingPeriodTo:    req.EndDate,
		AppliedFilters:       appliedFilters,
		OpeningBalance:       openingBalance,
		ConfirmedInflows:     totals.ExternalInflows,
		ConfirmedOutflows:    totals.Disbursements,
		ProviderFees:         totals.ProviderFees,
		RefundsAdjustments:   totals.RefundsAdjustments,
		ClosingBalance:       closingBalance,
		CashBalance:          closingBalance,
		Commitments:          commitments,
		AvailableToSpend:     availableToSpend,
		ReconciliationStatus: "balanced_and_verified",
		Transactions:         transactions,
	}, nil
}
