package service_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"ratels-api/internal/service"
)

func TestFoundationAndBankAccountVerification(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	// 1. Public Foundation information check
	info, err := env.FoundService.GetPublicInfo(ctx)
	if err != nil {
		t.Fatalf("GetPublicInfo failed: %v", err)
	}

	if info.ProvisionalName != "Martins Vincent Otse Foundation" {
		t.Errorf("Expected provisional name 'Martins Vincent Otse Foundation', got %s", info.ProvisionalName)
	}
	if info.IsVerified {
		t.Errorf("Expected IsVerified=false pending CAC documentation, got true")
	}
	if info.LiveDonationsActive {
		t.Errorf("Expected LiveDonationsActive=false in Phase 1, got true")
	}

	// 2. Submit bank account draft by an account editor
	editorEmail := fmt.Sprintf("editor_%d@example.org", time.Now().UnixNano())
	regEditor, err := env.AuthService.Register(ctx, service.RegisterRequest{
		Email:       editorEmail,
		Password:    "EditorPass123!",
		DisplayName: "Chief Accountant Bello",
	})
	if err != nil {
		t.Fatalf("Register editor failed: %v", err)
	}
	editorUserID := regEditor.UserID
	editorName := "Chief Accountant Bello"

	account, err := env.FoundService.CreateBankAccount(ctx, editorUserID, editorName, service.CreateBankAccountRequest{
		BankName:      "First Bank of Nigeria",
		AccountName:   "Martins Vincent Otse Foundation Designated Operations",
		AccountNumber: "0123456789",
		Currency:      "NGN",
	})
	if err != nil {
		t.Fatalf("CreateBankAccount failed: %v", err)
	}
	if account.Status != "pending_verification" {
		t.Errorf("Expected initial status 'pending_verification', got %s", account.Status)
	}

	// 3. Anti-Self-Approval: Account editor attempts to verify their own submission -> MUST FAIL
	_, err = env.FoundService.VerifyBankAccount(ctx, editorUserID, account.ID, service.VerifyBankAccountRequest{
		OfficerName: editorName,
	})
	if err != service.ErrSelfApprovalNotAllowed {
		t.Fatalf("Expected ErrSelfApprovalNotAllowed when creator attempts verification, got: %v", err)
	}

	// 4. First independent officer sign-off
	officer1ID := "officer_user_001"
	firstSignoff, err := env.FoundService.VerifyBankAccount(ctx, officer1ID, account.ID, service.VerifyBankAccountRequest{
		OfficerName: "Barrister Aliyu Ibrahim",
	})
	if err != nil {
		t.Fatalf("First officer verification failed: %v", err)
	}
	if firstSignoff.Status != "pending_second_signoff" {
		t.Errorf("Expected status 'pending_second_signoff' after 1st sign-off, got %s", firstSignoff.Status)
	}

	// 5. Duplicate sign-off attempt by SAME officer: Must fail!
	_, err = env.FoundService.VerifyBankAccount(ctx, officer1ID, account.ID, service.VerifyBankAccountRequest{
		OfficerName: "Barrister Aliyu Ibrahim", // Same signer!
	})
	if err == nil {
		t.Fatalf("Expected error when same officer signs twice, but got nil")
	}

	// 6. Second independent officer signs off: Status becomes 'approved'
	officer2ID := "officer_user_002"
	secondSignoff, err := env.FoundService.VerifyBankAccount(ctx, officer2ID, account.ID, service.VerifyBankAccountRequest{
		OfficerName: "Dr. Ngozi Eze", // Independent second signer
	})
	if err != nil {
		t.Fatalf("Second officer verification failed: %v", err)
	}
	if secondSignoff.Status != "approved" {
		t.Errorf("Expected status 'approved' after 2nd distinct sign-off, got %s", secondSignoff.Status)
	}
}
