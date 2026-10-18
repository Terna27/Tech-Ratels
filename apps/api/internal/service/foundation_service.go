package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"ratels-api/internal/config"
	"ratels-api/internal/db"
	"ratels-api/internal/repository"
	"ratels-api/internal/security"
)

type FoundationService struct {
	repo  *repository.Repository
	cfg   *config.Config
	audit *AuditService
}

func NewFoundationService(repo *repository.Repository, cfg *config.Config, audit *AuditService) *FoundationService {
	return &FoundationService{repo: repo, cfg: cfg, audit: audit}
}

type PublicFoundationResponse struct {
	ProvisionalName     string `json:"provisional_name"`
	RegisteredLegalName string `json:"registered_legal_name,omitempty"`
	RegistrationNumber  string `json:"registration_number,omitempty"`
	IsVerified          bool   `json:"is_verified"`
	MissionStatement    string `json:"mission_statement"`
	Disclaimer          string `json:"disclaimer"`
	LiveDonationsActive bool   `json:"live_donations_active"`
	BankAccountsNotice  string `json:"bank_accounts_notice"`
}

type CreateBankAccountRequest struct {
	BankName      string `json:"bank_name"`
	AccountName   string `json:"account_name"`
	AccountNumber string `json:"account_number"`
	Currency      string `json:"currency"`
}

type VerifyBankAccountRequest struct {
	OfficerName string  `json:"officer_name"`
	Notes       *string `json:"notes"`
}

func (s *FoundationService) GetPublicInfo(ctx context.Context) (*PublicFoundationResponse, error) {
	entity, err := s.repo.Queries().GetFoundationEntity(ctx)
	if err != nil || entity.ID == "" {
		return &PublicFoundationResponse{
			ProvisionalName:     s.cfg.FoundationProvisionalName,
			IsVerified:          false,
			MissionStatement:    "Empowering grassroots Nigerian communities through hygiene, transparency, and humanitarian initiatives.",
			Disclaimer:          "The Foundation's official name, legal registration details, and bank account credentials remain pending CAC document verification. No live payments or donations are currently accepted.",
			LiveDonationsActive: false,
			BankAccountsNotice:  "Official donation details will be published only after independent verification and dual-officer executive sign-off.",
		}, nil
	}

	return &PublicFoundationResponse{
		ProvisionalName:     entity.ProvisionalName,
		RegisteredLegalName: StringVal(entity.RegisteredLegalName),
		RegistrationNumber:  StringVal(entity.RegistrationNumber),
		IsVerified:          entity.IsVerified,
		MissionStatement:    entity.MissionStatement,
		Disclaimer:          entity.Disclaimer,
		LiveDonationsActive: false, // Phase 1 strictly false
		BankAccountsNotice:  "Official donation details will be published only after independent verification and dual-officer executive sign-off.",
	}, nil
}

func (s *FoundationService) ListBankAccountsAdmin(ctx context.Context) ([]db.VerifiedBankAccountVersion, error) {
	return s.repo.Queries().ListVerifiedBankAccountVersions(ctx)
}

func (s *FoundationService) CreateBankAccount(ctx context.Context, actorID, actorName string, req CreateBankAccountRequest) (*db.VerifiedBankAccountVersion, error) {
	if req.Currency == "" {
		req.Currency = "NGN"
	}

	accID := security.GenerateUUID()
	created, err := s.repo.Queries().CreateBankAccountVersion(ctx, db.CreateBankAccountVersionParams{
		ID:                 accID,
		BankName:           req.BankName,
		AccountName:        req.AccountName,
		AccountNumber:      req.AccountNumber,
		Currency:           req.Currency,
		Status:             "pending_verification",
		SubmittedByUserID:  TextFromString(actorID),
		SubmittedByOfficer: TextFromString(actorName),
		VerifiedByOfficer1: Text(nil),
		VerifiedByOfficer2: Text(nil),
		EffectiveDate:      Timestamptz(nil),
		VerificationNotes:  Text(nil),
	})
	if err != nil {
		return nil, err
	}

	details := fmt.Sprintf("Submitted bank account draft: %s (%s)", req.AccountName, req.BankName)
	_ = s.audit.Log(ctx, &actorID, nil, "submit_bank_account", "bank_account", accID, nil, nil, &details, nil)

	return &created, nil
}

func (s *FoundationService) VerifyBankAccount(ctx context.Context, actorID string, accountID string, req VerifyBankAccountRequest) (*db.VerifiedBankAccountVersion, error) {
	account, err := s.repo.Queries().GetBankAccountVersionByID(ctx, accountID)
	if err != nil {
		return nil, errors.New("bank account not found")
	}

	// Anti-Self-Approval: The officer who created/submitted the bank account cannot approve their own changes!
	if (account.SubmittedByUserID.Valid && account.SubmittedByUserID.String == actorID) ||
		(account.SubmittedByOfficer.Valid && account.SubmittedByOfficer.String == req.OfficerName) {
		return nil, ErrSelfApprovalNotAllowed
	}

	var officer1 = account.VerifiedByOfficer1
	var officer2 = account.VerifiedByOfficer2
	var newStatus = account.Status
	now := time.Now()
	var effectiveDate = account.EffectiveDate

	if !officer1.Valid || officer1.String == "" {
		officer1 = TextFromString(req.OfficerName)
		newStatus = "pending_second_signoff"
	} else if !officer2.Valid || officer2.String == "" {
		if officer1.String == req.OfficerName {
			return nil, errors.New("dual-officer verification requires two different authorized signers")
		}
		officer2 = TextFromString(req.OfficerName)
		newStatus = "approved"
		effectiveDate = TimestamptzFromTime(now)
	}

	updated, err := s.repo.Queries().UpdateBankAccountVerification(ctx, db.UpdateBankAccountVerificationParams{
		ID:                 accountID,
		Status:             newStatus,
		VerifiedByOfficer1: officer1,
		VerifiedByOfficer2: officer2,
		EffectiveDate:      effectiveDate,
		VerificationNotes:  Text(req.Notes),
	})
	if err != nil {
		return nil, err
	}

	details := fmt.Sprintf("Bank account verification signed by %s. New status: %s", req.OfficerName, newStatus)
	_ = s.audit.Log(ctx, &actorID, nil, "verify_bank_account", "bank_account", accountID, nil, nil, &details, nil)

	return &updated, nil
}
