package service_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"ratels-api/internal/service"
)

func TestFinanceAccountability_IncomeLifecycleAndAntiSelfApproval(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	// 1. Setup users: Submitter (State Coordinator) and Super Admin
	coordEmail := fmt.Sprintf("income_coord_%d@ratels.example.org", time.Now().UnixNano())
	coordReg, err := env.AuthService.Register(ctx, service.RegisterRequest{
		Email:       coordEmail,
		Password:    "CoordinatorPass123!",
		DisplayName: "State Coordinator User",
	})
	if err != nil {
		t.Fatalf("Failed to create coordinator: %v", err)
	}

	saEmail := fmt.Sprintf("income_sa_%d@ratels.example.org", time.Now().UnixNano())
	saReg, err := env.AuthService.Register(ctx, service.RegisterRequest{
		Email:       saEmail,
		Password:    "SuperAdminPass123!",
		DisplayName: "Super Admin User",
	})
	if err != nil {
		t.Fatalf("Failed to create super admin: %v", err)
	}

	states, _ := env.Repo.Queries().ListStates(ctx)
	var validStateID *string
	if len(states) > 0 {
		validStateID = &states[0].ID
	}

	// 2. Submitter records manual income
	idemKey := fmt.Sprintf("income-idem-%d", time.Now().UnixNano())
	incomeReq := service.RecordIncomeRequest{
		IdempotencyKey:  idemKey,
		IncomeCategory:  "member_contribution",
		AmountKobo:      15000000, // 150,000 NGN
		PaymentMethod:   "bank_transfer",
		FundingSource:   "state_membership_dues",
		StateID:         validStateID,
		Notes:           strPtr("Lagos monthly member contribution dues"),
	}

	tx, err := env.FinanceService.RecordIncome(ctx, coordReg.UserID, "state_coordinator", incomeReq)
	if err != nil {
		t.Fatalf("RecordIncome failed: %v", err)
	}
	if tx.VerificationStatus != "unverified" {
		t.Errorf("Expected status unverified, got %s", tx.VerificationStatus)
	}
	if tx.Status != "pending" {
		t.Errorf("Expected tx status pending, got %s", tx.Status)
	}

	// 3. Self-approval prevention: submitter tries to verify their own income
	_, err = env.FinanceService.VerifyIncome(ctx, coordReg.UserID, "state_coordinator", tx.ID, service.VerifyIncomeRequest{
		Decision: "verified",
		Notes:    "Approve myself",
	})
	if err == nil {
		t.Fatalf("Expected self-approval error when coordinator verifies their own income, got nil")
	}

	// 4. Authorized Super Admin verifies income
	verifiedTx, err := env.FinanceService.VerifyIncome(ctx, saReg.UserID, "super_admin", tx.ID, service.VerifyIncomeRequest{
		Decision: "verified",
		Notes:    "Verified with bank statement reference",
	})
	if err != nil {
		t.Fatalf("Super Admin failed to verify income: %v", err)
	}
	if verifiedTx.VerificationStatus != "verified" {
		t.Errorf("Expected verification status verified, got %s", verifiedTx.VerificationStatus)
	}
	if verifiedTx.Status != "posted" {
		t.Errorf("Expected tx status posted after verification, got %s", verifiedTx.Status)
	}

	// 5. Verify ledger balance: debits == credits == 15000000
	entries, err := env.Repo.Queries().ListLedgerEntriesByTransaction(ctx, verifiedTx.ID)
	if err != nil {
		t.Fatalf("Failed to fetch ledger entries: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("Expected 2 ledger entries, got %d", len(entries))
	}
	var debits, credits int64
	for _, e := range entries {
		if e.EntryType == "debit" {
			debits += e.Amount
		} else if e.EntryType == "credit" {
			credits += e.Amount
		}
	}
	if debits != credits || debits != 15000000 {
		t.Errorf("Ledger not balanced! Debits: %d, Credits: %d, Expected: 15000000", debits, credits)
	}
}

func TestFinanceAccountability_ExpenseLifecycleAndAntiSelfApproval(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	// Users: Requester (LGA Coordinator), Reviewer (State Coordinator), Payer (Super Admin)
	reqEmail := fmt.Sprintf("exp_req_%d@ratels.example.org", time.Now().UnixNano())
	reqUser, _ := env.AuthService.Register(ctx, service.RegisterRequest{
		Email:       reqEmail,
		Password:    "Password123!",
		DisplayName: "LGA Officer",
	})

	revEmail := fmt.Sprintf("exp_rev_%d@ratels.example.org", time.Now().UnixNano())
	revUser, _ := env.AuthService.Register(ctx, service.RegisterRequest{
		Email:       revEmail,
		Password:    "Password123!",
		DisplayName: "Reviewing Coordinator",
	})

	saEmail := fmt.Sprintf("exp_sa_%d@ratels.example.org", time.Now().UnixNano())
	saUser, _ := env.AuthService.Register(ctx, service.RegisterRequest{
		Email:       saEmail,
		Password:    "SuperAdminPass123!",
		DisplayName: "Finance Super Admin",
	})

	states, _ := env.Repo.Queries().ListStates(ctx)
	var validStateID *string
	if len(states) > 0 {
		validStateID = &states[0].ID
	}

	// 1. Create Expense
	expReq := service.CreateExpenseRequest{
		Title:                    "Cleanup materials procurement",
		Category:                 "cleanup_materials",
		Description:              "Brooms, rake, and heavy gloves for cleanup",
		AmountKobo:               7500000, // 75,000 NGN
		BeneficiaryName:          "Mama Ngozi Hardware Supplies",
		BeneficiaryAccountNumber: strPtr("0123456789"),
		BeneficiaryBankName:      strPtr("First Bank"),
		StateID:                  validStateID,
		SupportingDocumentURL:    strPtr("https://cdn.ratels.org/receipt-789.pdf"),
	}

	exp, err := env.FinanceService.CreateExpense(ctx, reqUser.UserID, "lga_representative", expReq)
	if err != nil {
		t.Fatalf("CreateExpense failed: %v", err)
	}
	if exp.Status != "submitted" {
		t.Errorf("Expected status submitted, got %s", exp.Status)
	}

	// 2. Anti-Self-Approval: Requester tries to review/approve their own expense
	_, err = env.FinanceService.ReviewExpense(ctx, reqUser.UserID, "lga_representative", exp.ID, service.ReviewExpenseRequest{
		Decision: "approved",
	})
	if err == nil {
		t.Fatalf("Expected self-approval error when requester reviews own expense, got nil")
	}

	// 3. Super Admin authorizes coordinator for financial review duties
	_, err = env.FinanceService.AuthorizeStaffAccess(ctx, saUser.UserID, "super_admin", revUser.UserID, "finance_officer", "Staff clearance to review expenses")
	if err != nil {
		t.Fatalf("AuthorizeStaffAccess failed: %v", err)
	}

	// Authorized Coordinator reviews & approves
	reviewedExp, err := env.FinanceService.ReviewExpense(ctx, revUser.UserID, "state_coordinator", exp.ID, service.ReviewExpenseRequest{
		Decision: "approved",
	})
	if err != nil {
		t.Fatalf("ReviewExpense failed: %v", err)
	}
	if reviewedExp.Status != "approved" {
		t.Errorf("Expected status approved, got %s", reviewedExp.Status)
	}

	// 4. Super Admin authorizes payment
	authExp, err := env.FinanceService.AuthorizeExpense(ctx, saUser.UserID, "super_admin", exp.ID, "Approved for payment")
	if err != nil {
		t.Fatalf("AuthorizeExpense failed: %v", err)
	}
	if authExp.Status != "authorized_for_payment" {
		t.Errorf("Expected status authorized_for_payment, got %s", authExp.Status)
	}

	// 5. Record Payment & Balanced Ledger Posting
	paidExp, err := env.FinanceService.RecordExpensePayment(ctx, saUser.UserID, "super_admin", exp.ID, service.PayExpenseRequest{
		PaymentReference: fmt.Sprintf("PAY-TRANSFER-%d", time.Now().UnixNano()),
	})
	if err != nil {
		t.Fatalf("RecordExpensePayment failed: %v", err)
	}
	if paidExp.Status != "paid" {
		t.Errorf("Expected status paid, got status=%s", paidExp.Status)
	}

	// Verify ledger entries for expense payment
	if !paidExp.TransactionID.Valid || paidExp.TransactionID.String == "" {
		t.Fatalf("Expected transaction ID to be linked on paid expense")
	}
	entries, err := env.Repo.Queries().ListLedgerEntriesByTransaction(ctx, paidExp.TransactionID.String)
	if err != nil {
		t.Fatalf("Failed to fetch ledger entries for expense: %v", err)
	}
	var debits, credits int64
	for _, e := range entries {
		if e.EntryType == "debit" {
			debits += e.Amount
		} else if e.EntryType == "credit" {
			credits += e.Amount
		}
	}
	if debits != credits || debits != 7500000 {
		t.Errorf("Ledger unbalanced for expense payment! Debits: %d, Credits: %d, Amount: 7500000", debits, credits)
	}
}

func TestFinanceAccountability_BudgetLifecycleAndRevisions(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	saEmail := fmt.Sprintf("bgt_sa_%d@ratels.example.org", time.Now().UnixNano())
	saUser, _ := env.AuthService.Register(ctx, service.RegisterRequest{
		Email:       saEmail,
		Password:    "SuperAdminPass123!",
		DisplayName: "Budget Super Admin",
	})

	coordEmail := fmt.Sprintf("bgt_coord_%d@ratels.example.org", time.Now().UnixNano())
	coordUser, _ := env.AuthService.Register(ctx, service.RegisterRequest{
		Email:       coordEmail,
		Password:    "CoordinatorPass123!",
		DisplayName: "Budget Coordinator",
	})

	states, _ := env.Repo.Queries().ListStates(ctx)
	var validStateID *string
	if len(states) > 0 {
		validStateID = &states[0].ID
	}

	// 1. Coordinator creates a proposed budget for Lagos State 2026-Q4
	bgtReq := service.CreateBudgetRequest{
		Title:              "Lagos State Q4 Sanitation Budget",
		ScopeType:          "state",
		StateID:            validStateID,
		PeriodType:         "quarterly",
		PeriodLabel:        "2026-Q4",
		PeriodStart:        time.Now(),
		PeriodEnd:          time.Now().AddDate(0, 3, 0),
		ProposedAmountKobo: 500000000, // 5,000,000 NGN
		Notes:              strPtr("Budget covering Ikeja, Surulere, and Lagos Island cleanups"),
	}

	budget, err := env.FinanceService.CreateBudget(ctx, coordUser.UserID, "state_coordinator", bgtReq)
	if err != nil {
		t.Fatalf("CreateBudget failed: %v", err)
	}
	if budget.Status != "submitted" {
		t.Errorf("Expected status submitted, got %s", budget.Status)
	}

	// 2. Non-Super-Admin cannot approve budget
	_, err = env.FinanceService.ApproveBudget(ctx, coordUser.UserID, "state_coordinator", budget.ID)
	if err == nil {
		t.Fatalf("Expected error when non-superadmin approves budget, got nil")
	}

	// 3. Super Admin approves budget
	appBudget, err := env.FinanceService.ApproveBudget(ctx, saUser.UserID, "super_admin", budget.ID)
	if err != nil {
		t.Fatalf("ApproveBudget failed: %v", err)
	}
	if appBudget.Status != "approved" || appBudget.ApprovedAmountKobo != 500000000 {
		t.Errorf("Expected approved status and 500,000,000 approved amount, got %s, %d", appBudget.Status, appBudget.ApprovedAmountKobo)
	}

	// 4. Request budget revision
	revReq := service.BudgetRevisionRequest{
		RevisedAmountKobo: 600000000,
		Reason:            "Increased logistics costs and storm response requirements",
	}
	revision, err := env.FinanceService.RequestBudgetRevision(ctx, coordUser.UserID, "state_coordinator", budget.ID, revReq)
	if err != nil {
		t.Fatalf("RequestBudgetRevision failed: %v", err)
	}
	if revision.Status != "pending" {
		t.Errorf("Expected revision status pending, got %s", revision.Status)
	}

	// 5. Super Admin approves revision
	_, err = env.FinanceService.ApproveBudgetRevision(ctx, saUser.UserID, "super_admin", revision.ID)
	if err != nil {
		t.Fatalf("ApproveBudgetRevision failed: %v", err)
	}

	revBudget, _, err := env.FinanceService.GetBudget(ctx, budget.ID)
	if err != nil {
		t.Fatalf("GetBudget after revision failed: %v", err)
	}
	if revBudget.ApprovedAmountKobo != 600000000 {
		t.Errorf("Expected updated approved amount 600000000, got %d", revBudget.ApprovedAmountKobo)
	}
}

func TestFinanceAccountability_AllocationsAndLedgerIntegrity(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	saEmail := fmt.Sprintf("alloc_sa_%d@ratels.example.org", time.Now().UnixNano())
	saUser, _ := env.AuthService.Register(ctx, service.RegisterRequest{
		Email:       saEmail,
		Password:    "SuperAdminPass123!",
		DisplayName: "Allocation Super Admin",
	})

	// Create allocation: National -> State (1,000,000 NGN)
	allocReq := service.CreateAllocationRequest{
		SourceType:        "national",
		DestinationType:   "state",
		DestinationID:     strPtr("LA"),
		SourceFundID:      "general_operations_fund",
		DestinationFundID: "general_operations_fund",
		AmountKobo:        100000000,
		Purpose:           "Q4 Operations disbursement to Lagos state account",
		ApprovalReference: fmt.Sprintf("ALLOC-LA-%d", time.Now().UnixNano()),
	}

	alloc, err := env.FinanceService.CreateAllocation(ctx, saUser.UserID, "super_admin", allocReq)
	if err != nil {
		t.Fatalf("CreateAllocation failed: %v", err)
	}
	if alloc.AmountKobo != 100000000 {
		t.Errorf("Expected allocation amount 100000000, got %d", alloc.AmountKobo)
	}

	// Verify underlying transaction: MUST NOT be external income
	if !alloc.TransactionID.Valid || alloc.TransactionID.String == "" {
		t.Fatalf("Expected valid transaction ID for allocation")
	}
	tx, err := env.Repo.Queries().GetFinancialTransactionByID(ctx, alloc.TransactionID.String)
	if err != nil {
		t.Fatalf("Failed to fetch allocation transaction: %v", err)
	}
	if tx.IsExternalIncome {
		t.Errorf("Internal fund allocation MUST NOT be counted as external income!")
	}

	// Verify double-entry ledger balance
	entries, err := env.Repo.Queries().ListLedgerEntriesByTransaction(ctx, alloc.TransactionID.String)
	if err != nil {
		t.Fatalf("Failed to fetch ledger entries for allocation: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("Expected 2 ledger entries, got %d", len(entries))
	}
	var debits, credits int64
	for _, e := range entries {
		if e.EntryType == "debit" {
			debits += e.Amount
		} else if e.EntryType == "credit" {
			credits += e.Amount
		}
	}
	if debits != credits || debits != 100000000 {
		t.Errorf("Allocation ledger entries unbalanced! Debits: %d, Credits: %d", debits, credits)
	}
}

func TestFinanceAccountability_BankAccountsAndReconciliation(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	saEmail := fmt.Sprintf("bank_sa_%d@ratels.example.org", time.Now().UnixNano())
	saUser, _ := env.AuthService.Register(ctx, service.RegisterRequest{
		Email:       saEmail,
		Password:    "SuperAdminPass123!",
		DisplayName: "Bank Super Admin",
	})

	coordEmail := fmt.Sprintf("bank_coord_%d@ratels.example.org", time.Now().UnixNano())
	coordUser, _ := env.AuthService.Register(ctx, service.RegisterRequest{
		Email:       coordEmail,
		Password:    "CoordinatorPass123!",
		DisplayName: "Bank Coordinator",
	})

	// 1. Register bank account
	bankReq := service.RegisterBankAccountRequest{
		BankName:      "Zenith Bank",
		AccountNumber: "1012345678",
		AccountName:   "Nigerian Ratels Operational Account",
		Currency:      "NGN",
		OrgType:       "ratels",
	}

	acc, err := env.FinanceService.CreateBankAccount(ctx, saUser.UserID, "super_admin", bankReq)
	if err != nil {
		t.Fatalf("CreateBankAccount failed: %v", err)
	}
	if acc.Status != "active" { // created by super admin -> active
		t.Errorf("Expected status active upon creation by super admin, got %s", acc.Status)
	}

	// 2. Non-Super-Admin cannot activate bank accounts
	_, err = env.FinanceService.ActivateBankAccount(ctx, coordUser.UserID, "state_coordinator", acc.ID)
	if err == nil {
		t.Fatalf("Expected error when non-superadmin tries to activate bank account, got nil")
	}

	// 3. Masking test: Non-superadmin gets masked account number
	accounts, err := env.FinanceService.ListBankAccounts(ctx, "state_coordinator", nil)
	if err != nil {
		t.Fatalf("ListBankAccounts failed: %v", err)
	}
	var found *service.BankAccountView
	for _, a := range accounts {
		if a.ID == acc.ID {
			found = &a
			break
		}
	}
	if found == nil {
		t.Fatalf("Bank account not found in list")
	}
	if !strings.HasPrefix(found.MaskedAccountNumber, "******") {
		t.Errorf("Expected masked account number for coordinator, got %s", found.MaskedAccountNumber)
	}

	// 4. Create reconciliation period
	periodReq := service.CreateReconciliationPeriodRequest{
		BankAccountID:        acc.ID,
		PeriodStart:          time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
		PeriodEnd:            time.Date(2026, 10, 31, 23, 59, 59, 0, time.UTC),
		OpeningBalanceKobo:   50000000,
		ClosingBalanceKobo:   65000000,
		StatementClosingKobo: 65000000,
	}
	period, err := env.FinanceService.CreateReconciliationPeriod(ctx, saUser.UserID, "super_admin", periodReq)
	if err != nil {
		t.Fatalf("CreateReconciliationPeriod failed: %v", err)
	}

	// 5. Import CSV bank statement
	csvData := `date,reference,amount,type,description
2026-10-05,TXN-99881,150000,credit,Donation from public donor
2026-10-12,TXN-99882,50000,debit,Refuse bags procurement
`
	count, err := env.FinanceService.ImportBankStatementCSV(ctx, saUser.UserID, "super_admin", period.ID, strings.NewReader(csvData))
	if err != nil {
		t.Fatalf("ImportBankStatementCSV failed: %v", err)
	}
	if count != 2 {
		t.Fatalf("Expected 2 imported statement items, got %d", count)
	}

	// 6. Duplicate import detection
	_, err = env.FinanceService.ImportBankStatementCSV(ctx, saUser.UserID, "super_admin", period.ID, strings.NewReader(csvData))
	if err == nil {
		t.Fatalf("Expected duplicate import error on re-uploading same statement, got nil")
	}

	// 7. Super Admin certifies reconciliation
	_, err = env.FinanceService.CertifyReconciliation(ctx, saUser.UserID, "super_admin", period.ID, "All transactions accounted for and matched against bank statement.")
	if err != nil {
		t.Fatalf("CertifyReconciliation failed: %v", err)
	}
}

func TestFinanceAccountability_DonationsAndDonorPrivacy(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	states, _ := env.Repo.Queries().ListStates(ctx)
	var validStateID *string
	if len(states) > 0 {
		validStateID = &states[0].ID
	}

	// 1. Initiate Donation with anonymous preference
	initReq := service.InitiateDonationRequest{
		EntityType:    "ratels",
		DonorName:     "Chief Emeka Adebayo",
		DonorEmail:    "emeka@example.com",
		AmountKobo:    25000000, // 250,000 NGN
		IsAnonymous:   true,
		StateID:       validStateID,
		CallbackURL:   "http://localhost:3000/donate/verify",
	}

	donRes, err := env.FinanceService.InitiateDonation(ctx, initReq)
	if err != nil {
		t.Fatalf("InitiateDonation failed: %v", err)
	}
	if donRes.Reference == "" {
		t.Errorf("Expected reference to be generated")
	}

	// 2. Verify Donation (e.g. via payment gateway callback)
	verifiedDon, err := env.FinanceService.VerifyDonation(ctx, donRes.Reference)
	if err != nil {
		t.Fatalf("VerifyDonation failed: %v", err)
	}
	if verifiedDon.Status != "verified" {
		t.Errorf("Expected status verified, got %s", verifiedDon.Status)
	}

	// 3. Test Donor Privacy in list view
	donations, err := env.FinanceService.ListDonations(ctx, "anon_user", "member", service.DonationFilterRequest{})
	if err != nil {
		t.Fatalf("ListDonations failed: %v", err)
	}
	var foundDon *service.DonationView
	for _, d := range donations {
		if d.ID == verifiedDon.ID {
			foundDon = &d
			break
		}
	}
	if foundDon == nil {
		t.Fatalf("Donation not found in list")
	}
	if !strings.Contains(foundDon.DonorName, "Anonymous") {
		t.Errorf("Expected donor name to be masked as Anonymous, got %s", foundDon.DonorName)
	}

	// 4. Public transparency summary
	transparency, err := env.FinanceService.GetPublicTransparencySummary(ctx)
	if err != nil {
		t.Fatalf("GetPublicTransparencySummary failed: %v", err)
	}
	if transparency.RatelsDonationsTotalKobo < 25000000 {
		t.Errorf("Expected at least 25000000 kobo in transparency summary, got %d", transparency.RatelsDonationsTotalKobo)
	}
}

func TestFinanceAccountability_DocumentGeneration(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	saEmail := fmt.Sprintf("doc_sa_%d@ratels.example.org", time.Now().UnixNano())
	saUser, _ := env.AuthService.Register(ctx, service.RegisterRequest{
		Email:       saEmail,
		Password:    "SuperAdminPass123!",
		DisplayName: "Doc Super Admin",
	})

	staffEmail := fmt.Sprintf("doc_staff_%d@ratels.example.org", time.Now().UnixNano())
	staffUser, _ := env.AuthService.Register(ctx, service.RegisterRequest{
		Email:       staffEmail,
		Password:    "StaffPass123!",
		DisplayName: "Doc Staff User",
	})

	states, _ := env.Repo.Queries().ListStates(ctx)
	var validStateID *string
	if len(states) > 0 {
		validStateID = &states[0].ID
	}

	// 1. Create and pay an expense
	expReq := service.CreateExpenseRequest{
		Title:                    "Borehole maintenance",
		Category:                 "community_projects",
		Description:              "Borehole maintenance spares and pipe fittings",
		AmountKobo:               12500000, // 125,000 NGN
		BeneficiaryName:          "Benue Water Works Engineering Ltd",
		StateID:                  validStateID,
	}
	exp, err := env.FinanceService.CreateExpense(ctx, staffUser.UserID, "member", expReq)
	if err != nil {
		t.Fatalf("CreateExpense failed: %v", err)
	}
	_, err = env.FinanceService.ReviewExpense(ctx, saUser.UserID, "super_admin", exp.ID, service.ReviewExpenseRequest{Decision: "approved"})
	if err != nil {
		t.Fatalf("ReviewExpense failed: %v", err)
	}
	_, err = env.FinanceService.AuthorizeExpense(ctx, saUser.UserID, "super_admin", exp.ID, "Authorized for payment")
	if err != nil {
		t.Fatalf("AuthorizeExpense failed: %v", err)
	}
	paidExp, err := env.FinanceService.RecordExpensePayment(ctx, saUser.UserID, "super_admin", exp.ID, service.PayExpenseRequest{
		PaymentReference: fmt.Sprintf("PAY-BENUE-%d", time.Now().UnixNano()),
	})
	if err != nil {
		t.Fatalf("RecordExpensePayment failed: %v", err)
	}

	// 2. Generate Expense Voucher
	voucherDoc, err := env.FinanceService.GenerateDocument(ctx, saUser.UserID, "super_admin", "expense_voucher", paidExp.ID)
	if err != nil {
		t.Fatalf("GenerateDocument expense_voucher failed: %v", err)
	}
	if !strings.Contains(voucherDoc.HTMLPrintableBody, "OFFICIAL PAYMENT &amp; DISBURSEMENT VOUCHER") && !strings.Contains(voucherDoc.HTMLPrintableBody, "OFFICIAL PAYMENT & DISBURSEMENT VOUCHER") {
		t.Errorf("Expected voucher title in HTML content")
	}
	if !strings.Contains(voucherDoc.HTMLPrintableBody, "Borehole maintenance") {
		t.Errorf("Expected line item description in HTML content")
	}
	if voucherDoc.HeaderData["beneficiary_name"] != "Benue Water Works Engineering Ltd" {
		t.Errorf("Expected beneficiary name in header data")
	}

	// 3. Test KoboToWords helper
	words := service.KoboToWords(12500000)
	if !strings.Contains(strings.ToLower(words), "one hundred and twenty five thousand") {
		t.Errorf("Unexpected words conversion: %s", words)
	}
}
