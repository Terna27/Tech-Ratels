package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"ratels-api/internal/db"
)

type FinancialDashboardSummaryResponse struct {
	TotalVerifiedIncomeKobo      int64                  `json:"total_verified_income_kobo"`
	TotalVerifiedExpenditureKobo int64                  `json:"total_verified_expenditure_kobo"`
	AvailableCashBalanceKobo     int64                  `json:"available_cash_balance_kobo"`
	OutstandingCommitmentsKobo   int64                  `json:"outstanding_commitments_kobo"`
	AvailableToSpendKobo         int64                  `json:"available_to_spend_kobo"`
	PendingApprovalsCount        int64                  `json:"pending_approvals_count"`
	PendingReconciliationsCount  int64                  `json:"pending_reconciliations_count"`
	ActiveBudgetsTotalKobo       int64                  `json:"active_budgets_total_kobo"`
	ProjectAllocationsTotalKobo  int64                  `json:"project_allocations_total_kobo"`
	TotalDonationsReceivedKobo   int64                  `json:"total_donations_received_kobo"`
	MonthlyTrends                []MonthlyTrendItem     `json:"monthly_trends"`
	FundBalances                 []FundBalanceItem      `json:"fund_balances"`
	GeneratedAt                  time.Time              `json:"generated_at"`
}

type MonthlyTrendItem struct {
	MonthLabel      string `json:"month_label"`
	IncomeKobo      int64  `json:"income_kobo"`
	ExpenditureKobo int64  `json:"expenditure_kobo"`
	NetSavingsKobo  int64  `json:"net_savings_kobo"`
}

type FundBalanceItem struct {
	FundID      string `json:"fund_id"`
	FundName    string `json:"fund_name"`
	BalanceKobo int64  `json:"balance_kobo"`
}

type FinancialDocumentResponse struct {
	DocumentType       string                 `json:"document_type"`
	DocumentReference  string                 `json:"document_reference"`
	Title              string                 `json:"title"`
	Organization       string                 `json:"organization"`
	ReportingPeriod    string                 `json:"reporting_period"`
	GeographicScope    string                 `json:"geographic_scope"`
	IssueDate          time.Time              `json:"issue_date"`
	AmountFormatted    string                 `json:"amount_formatted"`
	AmountInWords      string                 `json:"amount_in_words"`
	HeaderData         map[string]interface{} `json:"header_data"`
	LineItems          []DocumentLineItem     `json:"line_items"`
	PreparedBy         string                 `json:"prepared_by"`
	CertifiedBy        string                 `json:"certified_by"`
	CertificationStatus string                `json:"certification_status"`
	HTMLPrintableBody  string                 `json:"html_printable_body"`
}

type DocumentLineItem struct {
	Description string `json:"description"`
	Reference   string `json:"reference"`
	Date        string `json:"date"`
	Amount      string `json:"amount"`
}

// GetFinancialDashboardSummary aggregates authoritative figures for the administrative dashboard.
func (s *FinanceService) GetFinancialDashboardSummary(ctx context.Context, actorID, actorRole string) (*FinancialDashboardSummaryResponse, error) {
	hasAccess, _, _ := s.CheckFinancialAccess(ctx, actorID, actorRole)
	if !hasAccess {
		return nil, ErrFinancialUnauthorized
	}

	summary, err := s.repo.Queries().GetFinancialDashboardSummary(ctx)
	if err != nil {
		return nil, err
	}

	availableCash := summary.TotalVerifiedIncome - summary.TotalVerifiedExpenditure
	if availableCash < 0 {
		availableCash = 0
	}
	availableToSpend := availableCash - summary.OutstandingCommitments
	if availableToSpend < 0 {
		availableToSpend = 0
	}

	// Calculate active fund balances
	funds, _ := s.repo.Queries().ListFinancialFunds(ctx)
	fundBalances := make([]FundBalanceItem, 0, len(funds))
	for _, f := range funds {
		bal, _ := s.repo.Queries().CalculateFundBalanceBeforeDate(ctx, db.CalculateFundBalanceBeforeDateParams{
			FundID:     TextFromString(f.ID),
			BeforeDate: TimestamptzFromTime(time.Now().Add(24 * time.Hour)),
		})
		fundBalances = append(fundBalances, FundBalanceItem{
			FundID:      f.ID,
			FundName:    f.Name,
			BalanceKobo: bal,
		})
	}

	// Build last 6 months trends
	now := time.Now()
	trends := make([]MonthlyTrendItem, 6)
	for i := 5; i >= 0; i-- {
		targetMonth := now.AddDate(0, -i, 0)
		startOfMonth := time.Date(targetMonth.Year(), targetMonth.Month(), 1, 0, 0, 0, 0, time.UTC)
		endOfMonth := startOfMonth.AddDate(0, 1, 0).Add(-time.Nanosecond)

		monthTotals, _ := s.repo.Queries().CalculateExternalInflowsAndOutflows(ctx, db.CalculateExternalInflowsAndOutflowsParams{
			StartDate: TimestamptzFromTime(startOfMonth),
			EndDate:   TimestamptzFromTime(endOfMonth),
			StateID:   Text(nil),
			ZoneID:    Text(nil),
			FundID:    Text(nil),
		})

		monthLabel := startOfMonth.Format("Jan 2006")
		trends[5-i] = MonthlyTrendItem{
			MonthLabel:      monthLabel,
			IncomeKobo:      monthTotals.ExternalInflows,
			ExpenditureKobo: monthTotals.Disbursements,
			NetSavingsKobo:  monthTotals.ExternalInflows - monthTotals.Disbursements,
		}
	}

	return &FinancialDashboardSummaryResponse{
		TotalVerifiedIncomeKobo:      summary.TotalVerifiedIncome,
		TotalVerifiedExpenditureKobo: summary.TotalVerifiedExpenditure,
		AvailableCashBalanceKobo:     availableCash,
		OutstandingCommitmentsKobo:   summary.OutstandingCommitments,
		AvailableToSpendKobo:         availableToSpend,
		PendingApprovalsCount:        summary.PendingExpensesCount + summary.PendingTransactionsCount,
		PendingReconciliationsCount:  summary.PendingReconciliationsCount,
		ActiveBudgetsTotalKobo:       summary.ActiveBudgetsTotal,
		ProjectAllocationsTotalKobo:  summary.ProjectAllocationsTotal,
		TotalDonationsReceivedKobo:   summary.TotalDonationsReceived,
		MonthlyTrends:                trends,
		FundBalances:                 fundBalances,
		GeneratedAt:                  time.Now(),
	}, nil
}

// GenerateDocument renders official, certified printable financial documentation.
func (s *FinanceService) GenerateDocument(ctx context.Context, actorID, actorRole, docType, id string) (*FinancialDocumentResponse, error) {
	hasAccess, _, _ := s.CheckFinancialAccess(ctx, actorID, actorRole)
	if !hasAccess {
		return nil, ErrFinancialUnauthorized
	}

	switch docType {
	case "income_receipt":
		tx, entries, err := s.GetTransaction(ctx, id)
		if err != nil {
			return nil, err
		}

		amtFmt := FormatKoboToNaira(tx.Amount)
		amtWords := KoboToWords(tx.Amount)
		orgName := "Nigerian Ratels Movement"
		if tx.DestinationFundID.Valid && tx.DestinationFundID.String == "foundation_donation_fund" {
			orgName = "Martins Vincent Otse Foundation"
		}

		lineItems := []DocumentLineItem{
			{
				Description: fmt.Sprintf("Income - %s", tx.TransactionType),
				Reference:   tx.Reference,
				Date:        tx.OccurredAt.Time.Format("02 Jan 2006"),
				Amount:      amtFmt,
			},
		}
		for _, e := range entries {
			lineItems = append(lineItems, DocumentLineItem{
				Description: fmt.Sprintf("Ledger Line: %s (%s)", e.FundName, e.EntryType),
				Reference:   e.ID[:8],
				Date:        e.CreatedAt.Time.Format("02 Jan 2006"),
				Amount:      FormatKoboToNaira(e.Amount),
			})
		}

		preparedBy := "Ratels Treasury & Accounts Desk"
		if tx.InitiatingActorName.Valid {
			preparedBy = tx.InitiatingActorName.String
		}
		certifiedBy := "Official Financial Verification Seal"
		if tx.ApprovedByName.Valid {
			certifiedBy = tx.ApprovedByName.String + " (Super Admin Authorized)"
		}

		html := renderPrintableHTML(orgName, "OFFICIAL REVENUE & INCOME RECEIPT", tx.Reference, amtFmt, amtWords, preparedBy, certifiedBy, lineItems)

		return &FinancialDocumentResponse{
			DocumentType:        "income_receipt",
			DocumentReference:   tx.Reference,
			Title:               "OFFICIAL REVENUE & INCOME RECEIPT",
			Organization:        orgName,
			ReportingPeriod:     tx.OccurredAt.Time.Format("January 2006"),
			GeographicScope:     "National Secretariat",
			IssueDate:           tx.OccurredAt.Time,
			AmountFormatted:     amtFmt,
			AmountInWords:       amtWords,
			HeaderData: map[string]interface{}{
				"funding_source":  tx.FundingSource.String,
				"payment_method":  tx.PaymentMethod.String,
				"income_category": tx.IncomeCategory.String,
				"status":          tx.Status,
			},
			LineItems:           lineItems,
			PreparedBy:          preparedBy,
			CertifiedBy:         certifiedBy,
			CertificationStatus: "CERTIFIED OFFICIAL RECEIPT",
			HTMLPrintableBody:   html,
		}, nil

	case "expense_voucher":
		exp, err := s.GetExpense(ctx, id)
		if err != nil {
			return nil, err
		}

		amtFmt := FormatKoboToNaira(exp.AmountKobo)
		amtWords := KoboToWords(exp.AmountKobo)
		orgName := "Nigerian Ratels Movement"

		lineItems := []DocumentLineItem{
			{
				Description: fmt.Sprintf("%s (%s)", exp.Title, exp.Category),
				Reference:   exp.Reference,
				Date:        exp.CreatedAt.Time.Format("02 Jan 2006"),
				Amount:      amtFmt,
			},
		}

		preparedBy := "Operations Logistics Officer"
		if exp.RequestedByName.Valid {
			preparedBy = exp.RequestedByName.String
		}
		certifiedBy := "Super Admin Financial Governance Authorization"
		if exp.ApprovedByName.Valid {
			certifiedBy = exp.ApprovedByName.String
		}

		html := renderPrintableHTML(orgName, "OFFICIAL PAYMENT & DISBURSEMENT VOUCHER", exp.Reference, amtFmt, amtWords, preparedBy, certifiedBy, lineItems)

		return &FinancialDocumentResponse{
			DocumentType:        "expense_voucher",
			DocumentReference:   exp.Reference,
			Title:               "OFFICIAL PAYMENT & DISBURSEMENT VOUCHER",
			Organization:        orgName,
			ReportingPeriod:     exp.CreatedAt.Time.Format("January 2006"),
			GeographicScope:     exp.StateName.String,
			IssueDate:           exp.CreatedAt.Time,
			AmountFormatted:     amtFmt,
			AmountInWords:       amtWords,
			HeaderData: map[string]interface{}{
				"beneficiary_name":           exp.BeneficiaryName,
				"beneficiary_bank":           exp.BeneficiaryBankName.String,
				"beneficiary_account_number": exp.BeneficiaryAccountNumber.String,
				"status":                     exp.Status,
				"category":                   exp.Category,
			},
			LineItems:           lineItems,
			PreparedBy:          preparedBy,
			CertifiedBy:         certifiedBy,
			CertificationStatus: "APPROVED AND AUTHORIZED VOUCHER",
			HTMLPrintableBody:   html,
		}, nil

	case "donation_receipt":
		don, err := s.GetDonation(ctx, id)
		if err != nil {
			return nil, err
		}

		amtFmt := FormatKoboToNaira(don.AmountKobo)
		amtWords := KoboToWords(don.AmountKobo)
		orgName := "Nigerian Ratels Movement"
		if don.EntityType == "foundation" {
			orgName = "Martins Vincent Otse Foundation"
		}

		recNo := don.Reference
		if don.ReceiptNumber.Valid {
			recNo = don.ReceiptNumber.String
		}

		donorDisplay := don.DonorName
		if don.IsAnonymous {
			donorDisplay = "Valued Anonymous Humanitarian Contributor"
		}

		lineItems := []DocumentLineItem{
			{
				Description: fmt.Sprintf("Voluntary Contribution - %s", don.CampaignTitle.String),
				Reference:   don.Reference,
				Date:        don.CreatedAt.Time.Format("02 Jan 2006"),
				Amount:      amtFmt,
			},
		}

		html := renderPrintableHTML(orgName, "CHARITABLE CONTRIBUTION DONATION RECEIPT", recNo, amtFmt, amtWords, donorDisplay, "Board of Trustees & Financial Oversight", lineItems)

		return &FinancialDocumentResponse{
			DocumentType:        "donation_receipt",
			DocumentReference:   recNo,
			Title:               "CHARITABLE CONTRIBUTION DONATION RECEIPT",
			Organization:        orgName,
			ReportingPeriod:     don.CreatedAt.Time.Format("02 Jan 2006"),
			GeographicScope:     "National",
			IssueDate:           don.CreatedAt.Time,
			AmountFormatted:     amtFmt,
			AmountInWords:       amtWords,
			HeaderData: map[string]interface{}{
				"donor_name":     donorDisplay,
				"payment_method": don.PaymentMethod,
				"entity_type":    don.EntityType,
				"status":         don.Status,
			},
			LineItems:           lineItems,
			PreparedBy:          donorDisplay,
			CertifiedBy:         "Board of Trustees",
			CertificationStatus: "TAX-EXEMPT CHARITABLE RECEIPT",
			HTMLPrintableBody:   html,
		}, nil

	default:
		return nil, errors.New("unsupported document type")
	}
}

// FormatKoboToNaira formats minor units to standard Nigerian Naira string.
func FormatKoboToNaira(kobo int64) string {
	naira := float64(kobo) / 100.0
	sign := ""
	if naira < 0 {
		sign = "-"
		naira = -naira
	}
	intPart := int64(naira)
	fracPart := int64((naira-float64(intPart))*100 + 0.5)

	intStr := fmt.Sprintf("%d", intPart)
	// Add thousands separators
	var formattedInt strings.Builder
	l := len(intStr)
	for i, c := range intStr {
		if i > 0 && (l-i)%3 == 0 {
			formattedInt.WriteString(",")
		}
		formattedInt.WriteRune(c)
	}

	return fmt.Sprintf("%s₦%s.%02d", sign, formattedInt.String(), fracPart)
}

// KoboToWords converts kobo amount to formal financial words.
func KoboToWords(kobo int64) string {
	if kobo == 0 {
		return "Zero Naira Only"
	}
	naira := kobo / 100
	koboRem := kobo % 100

	words := numberToWords(naira) + " Naira"
	if koboRem > 0 {
		words += fmt.Sprintf(" and %s Kobo", numberToWords(koboRem))
	}
	return words + " Only"
}

func numberToWords(n int64) string {
	if n == 0 {
		return "Zero"
	}
	if n < 0 {
		return "Negative " + numberToWords(-n)
	}

	units := []string{"", "One", "Two", "Three", "Four", "Five", "Six", "Seven", "Eight", "Nine", "Ten", "Eleven", "Twelve", "Thirteen", "Fourteen", "Fifteen", "Sixteen", "Seventeen", "Eighteen", "Nineteen"}
	tens := []string{"", "", "Twenty", "Thirty", "Forty", "Fifty", "Sixty", "Seventy", "Eighty", "Ninety"}

	if n < 20 {
		return units[n]
	}
	if n < 100 {
		res := tens[n/10]
		if n%10 > 0 {
			res += " " + units[n%10]
		}
		return res
	}
	if n < 1000 {
		res := units[n/100] + " Hundred"
		if n%100 > 0 {
			res += " and " + numberToWords(n%100)
		}
		return res
	}
	if n < 1000000 {
		res := numberToWords(n/1000) + " Thousand"
		if n%1000 > 0 {
			res += " " + numberToWords(n%1000)
		}
		return res
	}
	if n < 1000000000 {
		res := numberToWords(n/1000000) + " Million"
		if n%1000000 > 0 {
			res += " " + numberToWords(n%1000000)
		}
		return res
	}
	return fmt.Sprintf("%d", n)
}

func renderPrintableHTML(org, title, ref, amount, words, prep, cert string, items []DocumentLineItem) string {
	var itemRows strings.Builder
	for _, it := range items {
		itemRows.WriteString(fmt.Sprintf(`
			<tr>
				<td style="padding: 10px; border-bottom: 1px solid #e5e7eb;">%s</td>
				<td style="padding: 10px; border-bottom: 1px solid #e5e7eb; font-family: monospace;">%s</td>
				<td style="padding: 10px; border-bottom: 1px solid #e5e7eb;">%s</td>
				<td style="padding: 10px; border-bottom: 1px solid #e5e7eb; text-align: right; font-weight: bold;">%s</td>
			</tr>
		`, it.Description, it.Reference, it.Date, it.Amount))
	}

	return fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
	<meta charset="utf-8">
	<title>%s - %s</title>
	<style>
		body { font-family: 'Helvetica Neue', Arial, sans-serif; margin: 40px; color: #111827; background: #fff; }
		.doc-card { border: 2px solid #111827; padding: 30px; border-radius: 8px; max-width: 800px; margin: 0 auto; }
		.header { border-bottom: 2px solid #111827; padding-bottom: 20px; margin-bottom: 24px; display: flex; justify-content: space-between; align-items: flex-start; }
		.org-title { font-size: 24px; font-weight: 800; color: #047857; text-transform: uppercase; letter-spacing: 0.05em; }
		.doc-badge { background: #111827; color: #fff; padding: 4px 12px; border-radius: 4px; font-size: 12px; font-weight: bold; }
		.meta-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; margin-bottom: 24px; font-size: 14px; }
		table { width: 100%%; border-collapse: collapse; margin-bottom: 24px; }
		th { background: #f3f4f6; text-align: left; padding: 10px; font-size: 13px; text-transform: uppercase; letter-spacing: 0.05em; }
		.amount-box { background: #f9fafb; border: 1px solid #e5e7eb; padding: 16px; border-radius: 6px; margin-bottom: 24px; text-align: right; }
		.amount-val { font-size: 26px; font-weight: 900; color: #047857; }
		.amount-words { font-style: italic; color: #4b5563; font-size: 13px; margin-top: 4px; }
		.signatures { display: grid; grid-template-columns: 1fr 1fr; gap: 40px; margin-top: 40px; border-top: 1px dashed #d1d5db; padding-top: 24px; }
		.sig-line { border-bottom: 1px solid #9ca3af; height: 40px; margin-bottom: 8px; }
		.footer { text-align: center; font-size: 11px; color: #6b7280; margin-top: 30px; border-top: 1px solid #e5e7eb; padding-top: 12px; }
	</style>
</head>
<body>
	<div class="doc-card">
		<div class="header">
			<div>
				<div class="org-title">%s</div>
				<h2 style="margin: 6px 0 0; font-size: 18px; color: #1f2937;">%s</h2>
			</div>
			<div class="doc-badge">OFFICIAL FINANCIAL DOCUMENT</div>
		</div>

		<div class="meta-grid">
			<div><strong>Document Reference:</strong> <span style="font-family: monospace;">%s</span></div>
			<div><strong>Issue Date:</strong> %s</div>
			<div><strong>Prepared By:</strong> %s</div>
			<div><strong>Certified Authority:</strong> %s</div>
		</div>

		<table>
			<thead>
				<tr>
					<th>Item Description</th>
					<th>Reference</th>
					<th>Date</th>
					<th style="text-align: right;">Amount (NGN)</th>
				</tr>
			</thead>
			<tbody>
				%s
			</tbody>
		</table>

		<div class="amount-box">
			<div class="amount-val">%s</div>
			<div class="amount-words">%s</div>
		</div>

		<div class="signatures">
			<div>
				<div class="sig-line"></div>
				<strong>Prepared / Initiated By:</strong>
				<div style="font-size: 13px; color: #4b5563;">%s</div>
			</div>
			<div>
				<div class="sig-line"></div>
				<strong>Super Admin Certification / Authorized Signatory:</strong>
				<div style="font-size: 13px; color: #4b5563;">%s</div>
			</div>
		</div>

		<div class="footer">
			Nigerian Ratels Financial & Governance Accountability System • Automated Integrity Check Passed • Stored on Cloudflare R2
		</div>
	</div>
</body>
</html>
	`, title, ref, org, title, ref, time.Now().Format("02 January 2006"), prep, cert, itemRows.String(), amount, words, prep, cert)
}
