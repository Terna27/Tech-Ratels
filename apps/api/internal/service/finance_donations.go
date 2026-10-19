package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"ratels-api/internal/db"
	"ratels-api/internal/payment"
	"ratels-api/internal/security"
)

type InitiateDonationRequest struct {
	EntityType    string  `json:"entity_type"` // ratels, foundation
	CampaignID    *string `json:"campaign_id"`
	DonorName     string  `json:"donor_name"`
	DonorEmail    string  `json:"donor_email"`
	DonorPhone    *string `json:"donor_phone"`
	IsAnonymous   bool    `json:"is_anonymous"`
	AmountKobo    int64   `json:"amount_kobo"`
	Currency      string  `json:"currency"`
	PaymentMethod string  `json:"payment_method"` // paystack
	ZoneID        *string `json:"zone_id"`
	StateID       *string `json:"state_id"`
	CallbackURL   string  `json:"callback_url"`
	Notes         *string `json:"notes"`
}

type InitiateDonationResponse struct {
	DonationID       string `json:"donation_id"`
	Reference        string `json:"reference"`
	PaymentReference string `json:"payment_reference"`
	CheckoutURL      string `json:"checkout_url"`
	Provider         string `json:"provider"`
}

type DonationFilterRequest struct {
	EntityType *string `json:"entity_type"`
	CampaignID *string `json:"campaign_id"`
	Status     *string `json:"status"`
	ZoneID     *string `json:"zone_id"`
	StateID    *string `json:"state_id"`
}

type DonationView struct {
	ID                 string     `json:"id"`
	Reference          string     `json:"reference"`
	EntityType         string     `json:"entity_type"`
	CampaignID         *string    `json:"campaign_id"`
	CampaignTitle      *string    `json:"campaign_title"`
	DonorName          string     `json:"donor_name"`
	DonorEmail         string     `json:"donor_email"`
	DonorPhone         *string    `json:"donor_phone"`
	IsAnonymous        bool       `json:"is_anonymous"`
	AmountKobo         int64      `json:"amount_kobo"`
	Currency           string     `json:"currency"`
	PaymentMethod      string     `json:"payment_method"`
	PaymentReference   *string    `json:"payment_reference"`
	Status             string     `json:"status"`
	ZoneID             *string    `json:"zone_id"`
	ZoneName           *string    `json:"zone_name"`
	StateID            *string    `json:"state_id"`
	StateName          *string    `json:"state_name"`
	VerifiedAt         *time.Time `json:"verified_at"`
	ReceiptNumber      *string    `json:"receipt_number"`
	Notes              *string    `json:"notes"`
	CreatedAt          time.Time  `json:"created_at"`
}

type PublicTransparencySummary struct {
	TotalVerifiedIncomeKobo      int64  `json:"total_verified_income_kobo"`
	TotalVerifiedExpenditureKobo int64  `json:"total_verified_expenditure_kobo"`
	TotalProjectFundingKobo      int64  `json:"total_project_funding_kobo"`
	RatelsDonationsTotalKobo     int64  `json:"ratels_donations_total_kobo"`
	FoundationDonationsTotalKobo int64  `json:"foundation_donations_total_kobo"`
	TotalDonationsCount          int64  `json:"total_donations_count"`
	AvailableReserveBalanceKobo  int64  `json:"available_reserve_balance_kobo"`
	ReportingCurrency            string `json:"reporting_currency"`
	CertifiedBy                  string `json:"certified_by"`
	LastUpdated                  string `json:"last_updated"`
}

// InitiateDonation registers a pending donation and generates a payment gateway checkout session.
func (s *FinanceService) InitiateDonation(ctx context.Context, req InitiateDonationRequest) (*InitiateDonationResponse, error) {
	if req.AmountKobo <= 0 {
		return nil, errors.New("donation amount must be greater than zero")
	}
	if req.DonorName == "" || req.DonorEmail == "" {
		return nil, errors.New("donor name and email are required")
	}

	if req.EntityType != "foundation" {
		req.EntityType = "ratels"
	}

	if req.Currency == "" {
		req.Currency = "NGN"
	}

	destFund := "general_operations_fund"
	if req.EntityType == "foundation" {
		destFund = "foundation_donation_fund"
	}

	donationID := security.GenerateUUID()
	ref := fmt.Sprintf("DON-%d-%s", time.Now().Year(), strings.ToUpper(security.GenerateUUID()[:8]))
	payRef := fmt.Sprintf("PAY-%s", ref)

	// Call payment gateway to initialize payment
	initOut, err := s.payment.InitializePayment(ctx, payment.InitializePaymentInput{
		OrderReference: payRef,
		AmountKobo:     req.AmountKobo,
		Currency:       req.Currency,
		CustomerEmail:  req.DonorEmail,
		CustomerName:   req.DonorName,
		CallbackURL:    req.CallbackURL,
		Metadata: map[string]interface{}{
			"donation_id":  donationID,
			"reference":    ref,
			"entity_type":  req.EntityType,
			"is_anonymous": req.IsAnonymous,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize donation checkout: %w", err)
	}

	// Persist pending donation
	_, err = s.repo.Queries().CreateDonation(ctx, db.CreateDonationParams{
		ID:                donationID,
		Reference:         ref,
		EntityType:        req.EntityType,
		CampaignID:        Text(req.CampaignID),
		DonorName:         req.DonorName,
		DonorEmail:        req.DonorEmail,
		DonorPhone:        Text(req.DonorPhone),
		IsAnonymous:       req.IsAnonymous,
		AmountKobo:        req.AmountKobo,
		Currency:          req.Currency,
		PaymentMethod:     s.payment.ProviderName(),
		PaymentReference:  TextFromString(initOut.PaymentReference),
		ProviderChannel:   TextFromString("gateway"),
		Status:            "pending",
		ZoneID:            Text(req.ZoneID),
		StateID:           Text(req.StateID),
		DestinationFundID: destFund,
		TransactionID:     Text(nil),
		VerifiedAt:        Timestamptz(nil),
		ReceiptNumber:     Text(nil),
		Notes:             Text(req.Notes),
	})
	if err != nil {
		return nil, err
	}

	return &InitiateDonationResponse{
		DonationID:       donationID,
		Reference:        ref,
		PaymentReference: initOut.PaymentReference,
		CheckoutURL:      initOut.CheckoutURL,
		Provider:         initOut.Provider,
	}, nil
}

// VerifyDonation verifies payment with the payment gateway and records posted ledger entries upon confirmation.
func (s *FinanceService) VerifyDonation(ctx context.Context, referenceOrPaymentRef string) (*db.FinancialDonation, error) {
	donation, err := s.repo.Queries().GetDonationByPaymentReference(ctx, TextFromString(referenceOrPaymentRef))
	if err != nil {
		donation, err = s.repo.Queries().GetDonationByReference(ctx, referenceOrPaymentRef)
		if err != nil {
			return nil, ErrDonationNotFound
		}
	}

	if donation.Status == "verified" {
		// Idempotent return of already verified donation
		return &donation, nil
	}

	payRef := referenceOrPaymentRef
	if donation.PaymentReference.Valid && donation.PaymentReference.String != "" {
		payRef = donation.PaymentReference.String
	}

	// Verify with payment provider
	verifyOut, err := s.payment.VerifyPayment(ctx, payRef)
	if err != nil {
		return nil, fmt.Errorf("provider payment verification failed: %w", err)
	}

	if verifyOut.Status != "successful" {
		return nil, fmt.Errorf("payment provider returned status: %s", verifyOut.Status)
	}

	if verifyOut.AmountKobo < donation.AmountKobo {
		return nil, errors.New("verified payment amount is less than expected donation amount")
	}

	now := time.Now()
	receiptNo := fmt.Sprintf("REC-DON-%d-%s", time.Now().Year(), strings.ToUpper(security.GenerateUUID()[:8]))
	txID := security.GenerateUUID()
	txRef := fmt.Sprintf("TXN-%s", donation.Reference)

	var verifiedDonation db.FinancialDonation

	err = s.repo.WithTx(ctx, func(q *db.Queries) error {
		// 1. Create authoritative posted financial transaction (is_external_income = true)
		_, err := q.CreateFinancialTransactionWithIncome(ctx, db.CreateFinancialTransactionWithIncomeParams{
			ID:                     txID,
			Reference:              txRef,
			IdempotencyKey:         TextFromString(fmt.Sprintf("don-tx-%s", donation.ID)),
			TransactionType:        "confirmed_donation",
			Status:                 "posted",
			Amount:                 donation.AmountKobo,
			Currency:               donation.Currency,
			SourceFundID:           Text(nil),
			DestinationFundID:      TextFromString(donation.DestinationFundID),
			RelatedCampaignID:      donation.CampaignID,
			RelatedProjectID:       Text(nil),
			StateID:                donation.StateID,
			ZoneID:                 donation.ZoneID,
			LgaID:                  Text(nil),
			ProviderReference:      TextFromString(payRef),
			SupportingEvidenceUrl:  Text(nil),
			Notes:                  TextFromString(fmt.Sprintf("Online donation by %s (%s)", donation.DonorName, donation.EntityType)),
			InitiatingActorID:      Text(nil),
			InitiatingActorType:    "system_payment_webhook",
			ApprovedByUserID:       Text(nil),
			ApprovedAt:             TimestamptzFromTime(now),
			IsExternalIncome:       true,
			OccurredAt:             TimestamptzFromTime(now),
			ReversalOfTransactionID: Text(nil),
			IsReversal:             false,
			ReversalReason:         Text(nil),
			IncomeCategory:         TextFromString("donation"),
			PaymentMethod:          TextFromString(donation.PaymentMethod),
			FundingSource:          TextFromString(donation.DonorName),
			ReceivingAccountID:     Text(nil),
			VerificationStatus:     "verified",
			VerifiedByUserID:       Text(nil),
			VerifiedAt:             TimestamptzFromTime(now),
		})
		if err != nil {
			return err
		}

		// 2. Write balanced double-entry ledger records
		// Debit: Cash Asset in destination fund
		_, err = q.CreateLedgerEntry(ctx, db.CreateLedgerEntryParams{
			ID:            security.GenerateUUID(),
			TransactionID: txID,
			FundID:        donation.DestinationFundID,
			EntryType:     "debit",
			Amount:        donation.AmountKobo,
			Currency:      donation.Currency,
			Description:   TextFromString(fmt.Sprintf("Cash receipt from online donation %s", donation.Reference)),
		})
		if err != nil {
			return err
		}
		// Credit: Recognized Donation Revenue in destination fund
		_, err = q.CreateLedgerEntry(ctx, db.CreateLedgerEntryParams{
			ID:            security.GenerateUUID(),
			TransactionID: txID,
			FundID:        donation.DestinationFundID,
			EntryType:     "credit",
			Amount:        donation.AmountKobo,
			Currency:      donation.Currency,
			Description:   TextFromString(fmt.Sprintf("Recognized donation income for %s (%s)", donation.Reference, donation.EntityType)),
		})
		if err != nil {
			return err
		}

		// 3. Mark donation as verified with receipt number
		up, err := q.UpdateDonationStatus(ctx, db.UpdateDonationStatusParams{
			ID:            donation.ID,
			Status:        "verified",
			VerifiedAt:    TimestamptzFromTime(now),
			TransactionID: TextFromString(txID),
			ReceiptNumber: TextFromString(receiptNo),
		})
		if err != nil {
			return err
		}
		verifiedDonation = up

		return nil
	})

	if err != nil {
		return nil, err
	}

	details := fmt.Sprintf("Verified donation %s (%d kobo) for %s. Receipt: %s", donation.Reference, donation.AmountKobo, donation.EntityType, receiptNo)
	_ = s.audit.Log(ctx, nil, nil, "verify_donation", "financial_donation", donation.ID, nil, nil, &details, nil)

	return &verifiedDonation, nil
}

// ListDonations returns filtered donations with privacy protection for anonymous donors.
func (s *FinanceService) ListDonations(ctx context.Context, actorID, actorRole string, filter DonationFilterRequest) ([]DonationView, error) {
	hasAccess, _, _ := s.CheckFinancialAccess(ctx, actorID, actorRole)
	canViewPrivate := actorRole == "super_admin" || actorRole == "national_admin" || hasAccess

	rows, err := s.repo.Queries().ListDonationsFiltered(ctx, db.ListDonationsFilteredParams{
		EntityType: Text(filter.EntityType),
		CampaignID: Text(filter.CampaignID),
		Status:     Text(filter.Status),
		ZoneID:     Text(filter.ZoneID),
		StateID:    Text(filter.StateID),
	})
	if err != nil {
		return nil, err
	}

	views := make([]DonationView, len(rows))
	for i, r := range rows {
		donorName := r.DonorName
		donorEmail := r.DonorEmail
		var donorPhone *string
		if r.DonorPhone.Valid {
			donorPhone = &r.DonorPhone.String
		}

		// Mask if donor chose privacy and viewer is not authorized staff
		if r.IsAnonymous && !canViewPrivate {
			donorName = "Anonymous Supporter"
			donorEmail = "supporter@ratels.org.ng"
			donorPhone = nil
		}

		var verifiedAt *time.Time
		if r.VerifiedAt.Valid {
			verifiedAt = &r.VerifiedAt.Time
		}

		views[i] = DonationView{
			ID:               r.ID,
			Reference:        r.Reference,
			EntityType:       r.EntityType,
			CampaignID:       StringPtrFromText(r.CampaignID),
			CampaignTitle:    StringPtrFromText(r.CampaignTitle),
			DonorName:        donorName,
			DonorEmail:       donorEmail,
			DonorPhone:       donorPhone,
			IsAnonymous:      r.IsAnonymous,
			AmountKobo:       r.AmountKobo,
			Currency:         r.Currency,
			PaymentMethod:    r.PaymentMethod,
			PaymentReference: StringPtrFromText(r.PaymentReference),
			Status:           r.Status,
			ZoneID:           StringPtrFromText(r.ZoneID),
			ZoneName:         StringPtrFromText(r.ZoneName),
			StateID:          StringPtrFromText(r.StateID),
			StateName:        StringPtrFromText(r.StateName),
			VerifiedAt:       verifiedAt,
			ReceiptNumber:    StringPtrFromText(r.ReceiptNumber),
			Notes:            StringPtrFromText(r.Notes),
			CreatedAt:        r.CreatedAt.Time,
		}
	}

	return views, nil
}

// GetDonation returns single donation details.
func (s *FinanceService) GetDonation(ctx context.Context, donationID string) (*db.GetDonationByIDRow, error) {
	don, err := s.repo.Queries().GetDonationByID(ctx, donationID)
	if err != nil {
		return nil, ErrDonationNotFound
	}
	return &don, nil
}

// GetPublicTransparencySummary returns aggregate verified figures for public accountability.
func (s *FinanceService) GetPublicTransparencySummary(ctx context.Context) (*PublicTransparencySummary, error) {
	summary, err := s.repo.Queries().GetFinancialDashboardSummary(ctx)
	if err != nil {
		return nil, err
	}

	// Calculate Ratels vs Foundation donation split
	ratelsDons, _ := s.repo.Queries().ListDonationsFiltered(ctx, db.ListDonationsFilteredParams{
		EntityType: TextFromString("ratels"),
		Status:     TextFromString("verified"),
	})
	var ratelsTotal int64 = 0
	for _, d := range ratelsDons {
		ratelsTotal += d.AmountKobo
	}

	foundDons, _ := s.repo.Queries().ListDonationsFiltered(ctx, db.ListDonationsFilteredParams{
		EntityType: TextFromString("foundation"),
		Status:     TextFromString("verified"),
	})
	var foundTotal int64 = 0
	for _, d := range foundDons {
		foundTotal += d.AmountKobo
	}

	reserveBalance := summary.TotalVerifiedIncome - summary.TotalVerifiedExpenditure
	if reserveBalance < 0 {
		reserveBalance = 0
	}

	return &PublicTransparencySummary{
		TotalVerifiedIncomeKobo:      summary.TotalVerifiedIncome,
		TotalVerifiedExpenditureKobo: summary.TotalVerifiedExpenditure,
		TotalProjectFundingKobo:      summary.ProjectAllocationsTotal,
		RatelsDonationsTotalKobo:     ratelsTotal,
		FoundationDonationsTotalKobo: foundTotal,
		TotalDonationsCount:          int64(len(ratelsDons) + len(foundDons)),
		AvailableReserveBalanceKobo:  reserveBalance,
		ReportingCurrency:            "NGN",
		CertifiedBy:                  "Nigerian Ratels National Financial Oversight Committee",
		LastUpdated:                  time.Now().Format(time.RFC3339),
	}, nil
}
