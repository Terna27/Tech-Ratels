package service_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"ratels-api/internal/service"
)

func TestFinanceService_DoubleEntryAndIdempotency(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	// 1. Setup a test super admin and member
	saEmail := fmt.Sprintf("fin_superadmin_%d@ratels.example.org", time.Now().UnixNano())
	saReg, err := env.AuthService.Register(ctx, service.RegisterRequest{
		Email:       saEmail,
		Password:    "SuperAdminPass123!",
		DisplayName: "Finance Super Admin",
	})
	if err != nil {
		t.Fatalf("Failed to create super admin: %v", err)
	}

	// 2. Record a donation with balanced ledger entries
	idempotencyKey := fmt.Sprintf("idem-donation-%d", time.Now().UnixNano())
	amount := int64(500000) // 5,000 NGN (500,000 Kobo)
	destFund := "foundation_donation_fund"

	txReq := service.CreateTransactionRequest{
		IdempotencyKey:    idempotencyKey,
		TransactionType:   "confirmed_donation",
		Amount:            amount,
		Currency:          "NGN",
		DestinationFundID: &destFund,
		Notes:             strPtr("Clean water donation"),
		ProviderReference: strPtr("PAY-REF-998811"),
		AutoPost:          true,
	}

	tx1, err := env.FinanceService.RecordTransaction(ctx, saReg.UserID, "super_admin", txReq)
	if err != nil {
		t.Fatalf("Failed to record transaction: %v", err)
	}

	if tx1.Amount != amount {
		t.Errorf("Expected amount %d, got %d", amount, tx1.Amount)
	}
	if tx1.Status != "posted" {
		t.Errorf("Expected status 'posted', got '%s'", tx1.Status)
	}

	// 3. Test Idempotency: retry with exact same idempotency key
	tx2, err := env.FinanceService.RecordTransaction(ctx, saReg.UserID, "super_admin", txReq)
	if err != nil {
		t.Fatalf("Failed on idempotent retry: %v", err)
	}
	if tx2.ID != tx1.ID {
		t.Errorf("Expected same transaction ID %s on idempotent retry, got %s", tx1.ID, tx2.ID)
	}

	// 4. Verify balanced ledger entries: sum(debit) == sum(credit) == amount
	entries, err := env.Repo.Queries().ListLedgerEntriesByTransaction(ctx, tx1.ID)
	if err != nil {
		t.Fatalf("Failed to list ledger entries: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("Expected 2 ledger entries, got %d", len(entries))
	}

	var totalDebit, totalCredit int64
	for _, e := range entries {
		if e.EntryType == "debit" {
			totalDebit += e.Amount
		} else if e.EntryType == "credit" {
			totalCredit += e.Amount
		}
	}
	if totalDebit != totalCredit {
		t.Errorf("Unbalanced ledger! Debit %d != Credit %d", totalDebit, totalCredit)
	}
	if totalDebit != amount {
		t.Errorf("Expected total debit/credit %d, got %d", amount, totalDebit)
	}

	// 5. Test Immutable Reversal: create reversal
	revTx, err := env.FinanceService.ReverseTransaction(ctx, saReg.UserID, "super_admin", tx1.ID, "Donor requested refund due to duplicate entry")
	if err != nil {
		t.Fatalf("Failed to reverse transaction: %v", err)
	}

	if revTx.TransactionType != "reversal" {
		t.Errorf("Expected reversal transaction type, got %s", revTx.TransactionType)
	}
	if revTx.Amount != amount {
		t.Errorf("Expected reversal amount %d, got %d", amount, revTx.Amount)
	}

	// Verify reversal ledger entries are balanced
	revEntries, err := env.Repo.Queries().ListLedgerEntriesByTransaction(ctx, revTx.ID)
	if err != nil {
		t.Fatalf("Failed to list reversal ledger entries: %v", err)
	}
	var revDebit, revCredit int64
	for _, e := range revEntries {
		if e.EntryType == "debit" {
			revDebit += e.Amount
		} else if e.EntryType == "credit" {
			revCredit += e.Amount
		}
	}
	if revDebit != revCredit {
		t.Errorf("Unbalanced reversal ledger! Debit %d != Credit %d", revDebit, revCredit)
	}
}

func TestFinanceService_ExternalIncomeSegregation(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	saEmail := fmt.Sprintf("fin_segregation_%d@ratels.example.org", time.Now().UnixNano())
	saReg, err := env.AuthService.Register(ctx, service.RegisterRequest{
		Email:       saEmail,
		Password:    "SuperAdminPass123!",
		DisplayName: "Segregation Super Admin",
	})
	if err != nil {
		t.Fatalf("Failed to create super admin: %v", err)
	}

	// 1. External Donation: 1,000,000 Kobo (10,000 NGN)
	destFund := "foundation_donation_fund"
	donationReq := service.CreateTransactionRequest{
		TransactionType:   "confirmed_donation",
		Amount:            1000000,
		Currency:          "NGN",
		DestinationFundID: &destFund,
		Notes:             strPtr("Real external donation"),
		AutoPost:          true,
	}
	donTx, err := env.FinanceService.RecordTransaction(ctx, saReg.UserID, "super_admin", donationReq)
	if err != nil {
		t.Fatalf("Failed to record donation: %v", err)
	}
	if !donTx.IsExternalIncome {
		t.Errorf("External donation should have IsExternalIncome = true")
	}

	// 2. Internal Transfer (Project Allocation): 400,000 Kobo
	projFund := "project_fund"
	transferReq := service.CreateTransactionRequest{
		TransactionType:   "internal_transfer",
		Amount:            400000,
		Currency:          "NGN",
		SourceFundID:      &destFund,
		DestinationFundID: &projFund,
		Notes:             strPtr("Allocation to sanitation project"),
		AutoPost:          true,
	}
	transferTx, err := env.FinanceService.RecordTransaction(ctx, saReg.UserID, "super_admin", transferReq)
	if err != nil {
		t.Fatalf("Failed to record internal transfer: %v", err)
	}
	if transferTx.IsExternalIncome {
		t.Errorf("Internal transfer MUST NOT be counted as external income (must be false)")
	}

	// 3. Bank Settlement: 500,000 Kobo
	clearingFund := "settlement_clearing_account"
	opsFund := "general_operations_fund"
	settlementReq := service.CreateTransactionRequest{
		TransactionType:   "bank_settlement",
		Amount:            500000,
		Currency:          "NGN",
		SourceFundID:      &clearingFund,
		DestinationFundID: &opsFund,
		Notes:             strPtr("Payment gateway batch settlement"),
		AutoPost:          true,
	}
	settleTx, err := env.FinanceService.RecordTransaction(ctx, saReg.UserID, "super_admin", settlementReq)
	if err != nil {
		t.Fatalf("Failed to record bank settlement: %v", err)
	}
	if settleTx.IsExternalIncome {
		t.Errorf("Bank settlement MUST NOT be counted as additional external income (must be false)")
	}
}

func TestFinanceService_FinancialAuthorization(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	saEmail := fmt.Sprintf("fin_auth_sa_%d@ratels.example.org", time.Now().UnixNano())
	saReg, _ := env.AuthService.Register(ctx, service.RegisterRequest{
		Email:       saEmail,
		Password:    "SuperAdminPass123!",
		DisplayName: "Financial Auth SA",
	})

	staffEmail := fmt.Sprintf("accountant_%d@ratels.example.org", time.Now().UnixNano())
	staffReg, _ := env.AuthService.Register(ctx, service.RegisterRequest{
		Email:       staffEmail,
		Password:    "AccountantPass123!",
		DisplayName: "Staff Accountant",
	})

	// 1. Initially, staffUser is NOT authorized
	hasAuth, _, err := env.FinanceService.CheckFinancialAccess(ctx, staffReg.UserID, "member")
	if err != nil {
		t.Fatalf("CheckFinancialAccess error: %v", err)
	}
	if hasAuth {
		t.Errorf("Staff member should not have financial authorization before grant")
	}

	// 2. Super Admin IS automatically authorized
	saAuth, _, err := env.FinanceService.CheckFinancialAccess(ctx, saReg.UserID, "super_admin")
	if err != nil || !saAuth {
		t.Errorf("Super Admin must have authorization: auth=%v, err=%v", saAuth, err)
	}

	// 3. Super Admin authorizes staffUser
	auth, err := env.FinanceService.AuthorizeStaffAccess(ctx, saReg.UserID, "super_admin", staffReg.UserID, "viewer", "Authorized for Q4 financial audit")
	if err != nil {
		t.Fatalf("Failed to authorize staff: %v", err)
	}
	if auth.Status != "active" {
		t.Errorf("Expected active authorization, got %s", auth.Status)
	}

	// 4. Staff member now has authorization
	hasAuth, level, err := env.FinanceService.CheckFinancialAccess(ctx, staffReg.UserID, "member")
	if err != nil || !hasAuth || level != "viewer" {
		t.Errorf("Staff member should now have authorization: auth=%v, level=%s, err=%v", hasAuth, level, err)
	}

	// 5. Super Admin revokes staffUser's authorization
	err = env.FinanceService.RevokeStaffAccess(ctx, saReg.UserID, "super_admin", auth.ID)
	if err != nil {
		t.Fatalf("Failed to revoke authorization: %v", err)
	}

	// 6. Staff member no longer has authorization
	hasAuth, _, err = env.FinanceService.CheckFinancialAccess(ctx, staffReg.UserID, "member")
	if err != nil || hasAuth {
		t.Errorf("Staff member should no longer have authorization after revocation: auth=%v", hasAuth)
	}
}

func strPtr(s string) *string {
	return &s
}
