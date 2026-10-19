package service

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"ratels-api/internal/db"
	"ratels-api/internal/security"
)

type RegisterBankAccountRequest struct {
	BankName      string  `json:"bank_name"`
	AccountName   string  `json:"account_name"`
	AccountNumber string  `json:"account_number"`
	Currency      string  `json:"currency"`
	OrgType       string  `json:"org_type"` // ratels, foundation
	IsPrimary     bool    `json:"is_primary"`
	Notes         *string `json:"notes"`
}

type BankAccountView struct {
	ID                  string    `json:"id"`
	BankName            string    `json:"bank_name"`
	AccountName         string    `json:"account_name"`
	MaskedAccountNumber string    `json:"account_number"`
	Currency            string    `json:"currency"`
	OrgType             string    `json:"org_type"`
	Status              string    `json:"status"`
	IsPrimary           bool      `json:"is_primary"`
	Notes               *string   `json:"notes"`
	CreatedAt           time.Time `json:"created_at"`
}

type CreateReconciliationPeriodRequest struct {
	BankAccountID        string    `json:"bank_account_id"`
	PeriodStart          time.Time `json:"period_start"`
	PeriodEnd            time.Time `json:"period_end"`
	OpeningBalanceKobo   int64     `json:"opening_balance_kobo"`
	ClosingBalanceKobo   int64     `json:"closing_balance_kobo"`
	StatementClosingKobo int64     `json:"statement_closing_balance_kobo"`
	Notes                *string   `json:"notes"`
}

// CreateBankAccount registers a financial bank account.
func (s *FinanceService) CreateBankAccount(ctx context.Context, actorID, actorRole string, req RegisterBankAccountRequest) (*db.FinancialBankAccount, error) {
	if req.BankName == "" || req.AccountName == "" || req.AccountNumber == "" {
		return nil, errors.New("bank name, account name, and account number are required")
	}

	if req.Currency == "" {
		req.Currency = "NGN"
	}
	if req.OrgType != "foundation" {
		req.OrgType = "ratels"
	}

	status := "pending_verification"
	var activatedBy pgtype.Text
	if actorRole == "super_admin" {
		status = "active"
		activatedBy = TextFromString(actorID)
	}

	accountID := security.GenerateUUID()
	account, err := s.repo.Queries().CreateBankAccount(ctx, db.CreateBankAccountParams{
		ID:                accountID,
		BankName:          req.BankName,
		AccountName:       req.AccountName,
		AccountNumber:     req.AccountNumber,
		Currency:          req.Currency,
		OrgType:           req.OrgType,
		Status:            status,
		IsPrimary:         req.IsPrimary,
		Notes:             Text(req.Notes),
		CreatedByUserID:   TextFromString(actorID),
		ActivatedByUserID: activatedBy,
	})
	if err != nil {
		return nil, err
	}

	details := fmt.Sprintf("Registered bank account %s (%s, %s) Status: %s", req.AccountName, req.BankName, req.OrgType, status)
	_ = s.audit.Log(ctx, &actorID, &actorRole, "create_bank_account", "financial_bank_account", accountID, nil, nil, &details, nil)

	return &account, nil
}

// ActivateBankAccount activates an official bank account, strictly reserved for Super Admin.
func (s *FinanceService) ActivateBankAccount(ctx context.Context, superAdminID, superAdminRole, accountID string) (*db.FinancialBankAccount, error) {
	if superAdminRole != "super_admin" {
		return nil, ErrFinancialApprovalRestricted
	}

	account, err := s.repo.Queries().ActivateBankAccount(ctx, db.ActivateBankAccountParams{
		ID:                accountID,
		ActivatedByUserID: TextFromString(superAdminID),
	})
	if err != nil {
		return nil, err
	}

	details := fmt.Sprintf("Activated bank account %s (%s)", account.AccountName, account.BankName)
	_ = s.audit.Log(ctx, &superAdminID, &superAdminRole, "activate_bank_account", "financial_bank_account", accountID, nil, nil, &details, nil)

	return &account, nil
}

// ListBankAccounts retrieves accounts, masking full account numbers for unauthorized roles.
func (s *FinanceService) ListBankAccounts(ctx context.Context, actorRole string, orgType *string) ([]BankAccountView, error) {
	accounts, err := s.repo.Queries().ListBankAccounts(ctx, db.ListBankAccountsParams{
		OrgType: Text(orgType),
		Status:  Text(nil),
	})
	if err != nil {
		return nil, err
	}

	canSeeFull := actorRole == "super_admin"

	views := make([]BankAccountView, len(accounts))
	for i, a := range accounts {
		acctNum := a.AccountNumber
		if !canSeeFull {
			if len(acctNum) > 4 {
				acctNum = strings.Repeat("*", len(acctNum)-4) + acctNum[len(acctNum)-4:]
			} else {
				acctNum = "****"
			}
		}

		views[i] = BankAccountView{
			ID:                  a.ID,
			BankName:            a.BankName,
			AccountName:         a.AccountName,
			MaskedAccountNumber: acctNum,
			Currency:            a.Currency,
			OrgType:             a.OrgType,
			Status:              a.Status,
			IsPrimary:           a.IsPrimary,
			Notes:               StringPtrFromText(a.Notes),
			CreatedAt:           a.CreatedAt.Time,
		}
	}

	return views, nil
}

// CreateReconciliationPeriod starts a new reconciliation period for a bank account.
func (s *FinanceService) CreateReconciliationPeriod(ctx context.Context, actorID, actorRole string, req CreateReconciliationPeriodRequest) (*db.FinancialReconciliationPeriod, error) {
	hasAccess, accessLevel, _ := s.CheckFinancialAccess(ctx, actorID, actorRole)
	if !hasAccess || (accessLevel != "super_admin" && accessLevel != "finance_officer") {
		return nil, ErrFinancialUnauthorized
	}

	if req.PeriodStart.IsZero() || req.PeriodEnd.IsZero() {
		return nil, errors.New("period start and end dates are required")
	}

	periodID := security.GenerateUUID()
	period, err := s.repo.Queries().CreateReconciliationPeriod(ctx, db.CreateReconciliationPeriodParams{
		ID:                          periodID,
		BankAccountID:               req.BankAccountID,
		PeriodStart:                 DateFromTime(&req.PeriodStart),
		PeriodEnd:                   DateFromTime(&req.PeriodEnd),
		OpeningBalanceKobo:          req.OpeningBalanceKobo,
		ClosingBalanceKobo:          req.ClosingBalanceKobo,
		StatementClosingBalanceKobo: req.StatementClosingKobo,
		Status:                      "draft",
		Notes:                       Text(req.Notes),
	})
	if err != nil {
		return nil, err
	}

	details := fmt.Sprintf("Created reconciliation period %s for account %s", periodID, req.BankAccountID)
	_ = s.audit.Log(ctx, &actorID, &actorRole, "create_reconciliation_period", "financial_reconciliation_period", periodID, nil, nil, &details, nil)

	return &period, nil
}

// GetReconciliationPeriod retrieves a period with its associated statement line items.
func (s *FinanceService) GetReconciliationPeriod(ctx context.Context, periodID string) (*db.GetReconciliationPeriodByIDRow, []db.ListStatementItemsByPeriodRow, error) {
	p, err := s.repo.Queries().GetReconciliationPeriodByID(ctx, periodID)
	if err != nil {
		return nil, nil, ErrReconciliationNotFound
	}

	items, err := s.repo.Queries().ListStatementItemsByPeriod(ctx, db.ListStatementItemsByPeriodParams{
		ReconciliationPeriodID: periodID,
		Status:                 Text(nil),
	})
	if err != nil {
		items = []db.ListStatementItemsByPeriodRow{}
	}

	return &p, items, nil
}

// ListReconciliationPeriods lists periods for an account.
func (s *FinanceService) ListReconciliationPeriods(ctx context.Context, accountID, status *string) ([]db.ListReconciliationPeriodsByAccountRow, error) {
	return s.repo.Queries().ListReconciliationPeriodsByAccount(ctx, db.ListReconciliationPeriodsByAccountParams{
		BankAccountID: Text(accountID),
		Status:        Text(status),
	})
}

// ImportBankStatementCSV parses a bank statement CSV and imports statement line items with duplicate detection.
func (s *FinanceService) ImportBankStatementCSV(ctx context.Context, actorID, actorRole, periodID string, reader io.Reader) (int, error) {
	hasAccess, accessLevel, _ := s.CheckFinancialAccess(ctx, actorID, actorRole)
	if !hasAccess || (accessLevel != "super_admin" && accessLevel != "finance_officer") {
		return 0, ErrFinancialUnauthorized
	}

	period, err := s.repo.Queries().GetReconciliationPeriodByID(ctx, periodID)
	if err != nil {
		return 0, ErrReconciliationNotFound
	}
	if period.Status == "certified" {
		return 0, errors.New("cannot import statements into a certified reconciliation period")
	}

	csvReader := csv.NewReader(reader)
	csvReader.FieldsPerRecord = -1 // Allow variable columns
	csvReader.TrimLeadingSpace = true

	records, err := csvReader.ReadAll()
	if err != nil {
		return 0, fmt.Errorf("failed to parse CSV: %w", err)
	}

	if len(records) < 2 {
		return 0, errors.New("CSV contains no transaction records")
	}

	// Detect column indices from header
	headers := records[0]
	dateIdx := -1
	refIdx := -1
	descIdx := -1
	amtIdx := -1
	typeIdx := -1
	debitIdx := -1
	creditIdx := -1

	for i, h := range headers {
		clean := strings.ToLower(strings.TrimSpace(h))
		switch {
		case strings.Contains(clean, "date"):
			if dateIdx == -1 {
				dateIdx = i
			}
		case strings.Contains(clean, "ref") || strings.Contains(clean, "txn") || strings.Contains(clean, "chq") || strings.Contains(clean, "number"):
			if refIdx == -1 {
				refIdx = i
			}
		case strings.Contains(clean, "desc") || strings.Contains(clean, "narration") || strings.Contains(clean, "particulars") || strings.Contains(clean, "detail"):
			if descIdx == -1 {
				descIdx = i
			}
		case clean == "debit" || strings.Contains(clean, "withdrawal"):
			debitIdx = i
		case clean == "credit" || strings.Contains(clean, "deposit"):
			creditIdx = i
		case strings.Contains(clean, "amount"):
			amtIdx = i
		case strings.Contains(clean, "type"):
			typeIdx = i
		}
	}

	if dateIdx == -1 || (amtIdx == -1 && (debitIdx == -1 || creditIdx == -1)) {
		return 0, errors.New("CSV header must contain at least Date and Amount (or Debit/Credit) columns")
	}

	importedCount := 0

	for rowIdx, row := range records[1:] {
		if len(row) <= dateIdx {
			continue
		}

		dateStr := strings.TrimSpace(row[dateIdx])
		if dateStr == "" {
			continue
		}

		txDate, err := parseFlexibleDate(dateStr)
		if err != nil {
			continue
		}

		ref := fmt.Sprintf("STMT-%s-%d", periodID[:8], rowIdx+1)
		if refIdx != -1 && len(row) > refIdx && strings.TrimSpace(row[refIdx]) != "" {
			ref = strings.TrimSpace(row[refIdx])
		}

		desc := "Bank Transaction"
		if descIdx != -1 && len(row) > descIdx && strings.TrimSpace(row[descIdx]) != "" {
			desc = strings.TrimSpace(row[descIdx])
		}

		var amountKobo int64 = 0
		entryType := "credit"

		if debitIdx != -1 && creditIdx != -1 && len(row) > debitIdx && len(row) > creditIdx {
			debitStr := cleanAmountString(row[debitIdx])
			creditStr := cleanAmountString(row[creditIdx])

			if debitStr != "" && debitStr != "0" && debitStr != "0.00" {
				if dVal, err := strconv.ParseFloat(debitStr, 64); err == nil && dVal > 0 {
					amountKobo = int64(dVal * 100)
					entryType = "debit"
				}
			} else if creditStr != "" && creditStr != "0" && creditStr != "0.00" {
				if cVal, err := strconv.ParseFloat(creditStr, 64); err == nil && cVal > 0 {
					amountKobo = int64(cVal * 100)
					entryType = "credit"
				}
			}
		} else if amtIdx != -1 && len(row) > amtIdx {
			amtStr := cleanAmountString(row[amtIdx])
			if aVal, err := strconv.ParseFloat(amtStr, 64); err == nil {
				if aVal < 0 {
					amountKobo = int64(-aVal * 100)
					entryType = "debit"
				} else {
					amountKobo = int64(aVal * 100)
					entryType = "credit"
				}
			}

			if typeIdx != -1 && len(row) > typeIdx {
				tStr := strings.ToLower(strings.TrimSpace(row[typeIdx]))
				if strings.Contains(tStr, "dr") || strings.Contains(tStr, "debit") || strings.Contains(tStr, "out") {
					entryType = "debit"
				} else if strings.Contains(tStr, "cr") || strings.Contains(tStr, "credit") || strings.Contains(tStr, "in") {
					entryType = "credit"
				}
			}
		}

		if amountKobo <= 0 {
			continue
		}

		itemID := security.GenerateUUID()
		_, err = s.repo.Queries().CreateStatementItem(ctx, db.CreateStatementItemParams{
			ID:                     itemID,
			ReconciliationPeriodID: periodID,
			StatementReference:     ref,
			TransactionDate:        DateFromTime(&txDate),
			ValueDate:              DateFromTime(nil),
			Description:            desc,
			AmountKobo:             amountKobo,
			EntryType:              entryType,
			MatchedTransactionID:   Text(nil),
			Status:                 "unmatched",
			MatchNotes:             Text(nil),
		})
		if err == nil {
			importedCount++
		}
	}

	if importedCount == 0 {
		return 0, errors.New("no new statement items imported; all transactions were duplicates or invalid")
	}

	details := fmt.Sprintf("Imported %d statement items into reconciliation period %s", importedCount, periodID)
	_ = s.audit.Log(ctx, &actorID, &actorRole, "import_bank_statement", "financial_reconciliation_period", periodID, nil, nil, &details, nil)

	return importedCount, nil
}

// AutoMatchStatementItems automatically matches statement items against posted financial ledger records.
func (s *FinanceService) AutoMatchStatementItems(ctx context.Context, actorID, actorRole, periodID string) (int, error) {
	hasAccess, accessLevel, _ := s.CheckFinancialAccess(ctx, actorID, actorRole)
	if !hasAccess || (accessLevel != "super_admin" && accessLevel != "finance_officer") {
		return 0, ErrFinancialUnauthorized
	}

	items, err := s.repo.Queries().ListStatementItemsByPeriod(ctx, db.ListStatementItemsByPeriodParams{
		ReconciliationPeriodID: periodID,
		Status:                 TextFromString("unmatched"),
	})
	if err != nil {
		return 0, err
	}

	// Fetch posted transactions
	txs, err := s.repo.Queries().ListFinancialTransactionsFiltered(ctx, db.ListFinancialTransactionsFilteredParams{
		Status: TextFromString("posted"),
	})
	if err != nil {
		return 0, err
	}

	matchedCount := 0

	for _, item := range items {
		for _, tx := range txs {
			if tx.Amount != item.AmountKobo {
				continue
			}

			// Check matching reference or provider reference
			refMatch := strings.EqualFold(tx.Reference, item.StatementReference)
			if !refMatch && tx.ProviderReference.Valid {
				refMatch = strings.EqualFold(tx.ProviderReference.String, item.StatementReference)
			}

			// Or date match within 4 days
			itemDate := item.TransactionDate.Time
			dateDiff := tx.OccurredAt.Time.Sub(itemDate)
			if dateDiff < 0 {
				dateDiff = -dateDiff
			}
			dateMatch := dateDiff <= 4*24*time.Hour

			if refMatch || dateMatch {
				matchNotes := "Auto-matched by amount and reference/date proximity"
				_, err = s.repo.Queries().UpdateStatementItemMatch(ctx, db.UpdateStatementItemMatchParams{
					ID:                   item.ID,
					Status:               "matched",
					MatchedTransactionID: TextFromString(tx.ID),
					MatchNotes:           TextFromString(matchNotes),
				})
				if err == nil {
					matchedCount++
					break
				}
			}
		}
	}

	details := fmt.Sprintf("Auto-matched %d items in reconciliation period %s", matchedCount, periodID)
	_ = s.audit.Log(ctx, &actorID, &actorRole, "auto_match_reconciliation", "financial_reconciliation_period", periodID, nil, nil, &details, nil)

	return matchedCount, nil
}

// ManualMatchStatementItem manually links a statement item to a specific financial transaction.
func (s *FinanceService) ManualMatchStatementItem(ctx context.Context, actorID, actorRole, itemID, transactionID, notes string) (*db.FinancialStatementItem, error) {
	hasAccess, accessLevel, _ := s.CheckFinancialAccess(ctx, actorID, actorRole)
	if !hasAccess || (accessLevel != "super_admin" && accessLevel != "finance_officer") {
		return nil, ErrFinancialUnauthorized
	}

	matchNotes := fmt.Sprintf("Manually matched by %s: %s", actorID, notes)
	updated, err := s.repo.Queries().UpdateStatementItemMatch(ctx, db.UpdateStatementItemMatchParams{
		ID:                   itemID,
		Status:               "manually_reconciled",
		MatchedTransactionID: TextFromString(transactionID),
		MatchNotes:           TextFromString(matchNotes),
	})
	if err != nil {
		return nil, err
	}

	details := fmt.Sprintf("Manually matched statement item %s to transaction %s", itemID, transactionID)
	_ = s.audit.Log(ctx, &actorID, &actorRole, "manual_match_reconciliation", "financial_statement_item", itemID, nil, nil, &details, nil)

	return &updated, nil
}

// CertifyReconciliation marks a reconciliation period as officially certified, strictly reserved for Super Admin.
func (s *FinanceService) CertifyReconciliation(ctx context.Context, superAdminID, superAdminRole, periodID, notes string) (*db.FinancialReconciliationPeriod, error) {
	if superAdminRole != "super_admin" {
		return nil, ErrFinancialApprovalRestricted
	}

	period, err := s.repo.Queries().GetReconciliationPeriodByID(ctx, periodID)
	if err != nil {
		return nil, ErrReconciliationNotFound
	}

	now := time.Now()
	cert, err := s.repo.Queries().UpdateReconciliationPeriodStatus(ctx, db.UpdateReconciliationPeriodStatusParams{
		ID:                 periodID,
		Status:             "certified",
		ReconciledByUserID: Text(nil),
		ReconciledAt:       Timestamptz(nil),
		CertifiedByUserID:  TextFromString(superAdminID),
		CertifiedAt:        TimestamptzFromTime(now),
	})
	if err != nil {
		return nil, err
	}

	details := fmt.Sprintf("Certified reconciliation period %s for account %s. Notes: %s", periodID, period.BankAccountID, notes)
	_ = s.audit.Log(ctx, &superAdminID, &superAdminRole, "certify_reconciliation", "financial_reconciliation_period", periodID, nil, nil, &details, nil)

	return &cert, nil
}

// Helpers
func parseFlexibleDate(s string) (time.Time, error) {
	formats := []string{
		"2006-01-02",
		"02/01/2006",
		"02-01-2006",
		"01/02/2006",
		"2006/01/02",
		"02-Jan-2006",
		"02-Jan-06",
		"02 Jan 2006",
		time.RFC3339,
	}
	for _, f := range formats {
		if t, err := time.Parse(f, strings.TrimSpace(s)); err == nil {
			return t, nil
		}
	}
	return time.Time{}, errors.New("unrecognized date format")
}

func cleanAmountString(s string) string {
	s = strings.ReplaceAll(s, ",", "")
	s = strings.ReplaceAll(s, "₦", "")
	s = strings.ReplaceAll(s, "NGN", "")
	s = strings.ReplaceAll(s, " ", "")
	return strings.TrimSpace(s)
}
