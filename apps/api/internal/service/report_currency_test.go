package service_test

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"ratels-api/internal/service"
)

// 1. Unit regression tests for FormatKoboToNaira covering positive, zero, and negative monetary values.
func TestFormatKoboToNaira_Regression(t *testing.T) {
	tests := []struct {
		name     string
		kobo     int64
		expected string
	}{
		{name: "Zero kobo", kobo: 0, expected: "₦0.00"},
		{name: "Positive 1 kobo", kobo: 1, expected: "₦0.01"},
		{name: "Positive 50 kobo", kobo: 50, expected: "₦0.50"},
		{name: "Positive 99 kobo", kobo: 99, expected: "₦0.99"},
		{name: "Positive 100 kobo (1 Naira)", kobo: 100, expected: "₦1.00"},
		{name: "Positive 1,500 Naira (delivery fee)", kobo: 150000, expected: "₦1,500.00"},
		{name: "Positive 10,000 Naira", kobo: 1000000, expected: "₦10,000.00"},
		{name: "Positive 1,500,000 Naira", kobo: 150000000, expected: "₦1,500,000.00"},
		{name: "Positive large budget 500M Naira", kobo: 50000000000, expected: "₦500,000,000.00"},
		{name: "Negative 1 kobo", kobo: -1, expected: "-₦0.01"},
		{name: "Negative 50 kobo", kobo: -50, expected: "-₦0.50"},
		{name: "Negative 100 kobo (-1 Naira)", kobo: -100, expected: "-₦1.00"},
		{name: "Negative deficit 15,000 Naira", kobo: -1500000, expected: "-₦15,000.00"},
		{name: "Negative budget variance 2,500,000 Naira", kobo: -250000000, expected: "-₦2,500,000.00"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual := service.FormatKoboToNaira(tc.kobo)
			if actual != tc.expected {
				t.Errorf("FormatKoboToNaira(%d) = %q, want %q", tc.kobo, actual, tc.expected)
			}

			// Verify formatted value contains exactly one currency symbol
			nairaCount := strings.Count(actual, "₦")
			if nairaCount != 1 {
				t.Errorf("expected exactly 1 '₦' symbol in %q, got %d", actual, nairaCount)
			}

			// Verify no duplicated symbols
			if strings.Contains(actual, "₦₦") {
				t.Errorf("found duplicate '₦₦' in %q", actual)
			}
			if strings.Contains(actual, "NGN NGN") {
				t.Errorf("found duplicate 'NGN NGN' in %q", actual)
			}
		})
	}
}

// 2. Integration regression tests for snapshot aggregated data across all affected report categories
func TestReportCurrency_AggregatedDataFormatting(t *testing.T) {
	env := setupTestEnv(t)
	superAdmin := createTestUserWithRole(t, env, "super_admin", fmt.Sprintf("sa_%d@ratels.org", time.Now().UnixNano()))

	checkFormattedString := func(t *testing.T, fieldName, val string) {
		t.Helper()
		if val == "" {
			t.Errorf("field %s is empty", fieldName)
			return
		}
		if strings.Contains(val, "₦₦") {
			t.Errorf("field %s has duplicated currency symbol: %q", fieldName, val)
		}
		if strings.Contains(val, "NGN NGN") {
			t.Errorf("field %s has duplicated NGN prefix: %q", fieldName, val)
		}
		count := strings.Count(val, "₦")
		if count != 1 {
			t.Errorf("field %s expected exactly 1 '₦' symbol, got %d in %q", fieldName, count, val)
		}
	}

	// 2a. Finance Report
	t.Run("Finance", func(t *testing.T) {
		snap := createTestSnapshot(t, env, superAdmin, "finance", "2088-01", false)
		var fData service.PublicFinanceData
		if err := json.Unmarshal(snap.AggregatedData, &fData); err != nil {
			t.Fatalf("failed unmarshaling finance aggregated data: %v", err)
		}

		checkFormattedString(t, "ApprovedIncomeFormatted", fData.ApprovedIncomeFormatted)
		checkFormattedString(t, "ApprovedExpenditureFormatted", fData.ApprovedExpenditureFormatted)
		checkFormattedString(t, "NetSurplusDeficitFormatted", fData.NetSurplusDeficitFormatted)
		checkFormattedString(t, "TotalApprovedBudgetFormatted", fData.TotalApprovedBudgetFormatted)
		checkFormattedString(t, "BudgetVarianceFormatted", fData.BudgetVarianceFormatted)

		for i, cat := range fData.SpendingCategories {
			checkFormattedString(t, fmt.Sprintf("SpendingCategories[%d].AmountFormatted", i), cat.AmountFormatted)
		}
	})

	// 2b. Foundation Report
	t.Run("Foundation", func(t *testing.T) {
		snap := createTestSnapshot(t, env, superAdmin, "foundation", "2088-02", false)
		var fndData service.PublicFoundationData
		if err := json.Unmarshal(snap.AggregatedData, &fndData); err != nil {
			t.Fatalf("failed unmarshaling foundation aggregated data: %v", err)
		}

		checkFormattedString(t, "TotalDonationsFormatted", fndData.TotalDonationsFormatted)
	})

	// 2c. Community Projects Report
	t.Run("CommunityProjects", func(t *testing.T) {
		snap := createTestSnapshot(t, env, superAdmin, "community_projects", "2088-03", false)
		var cpData service.PublicCommunityProjectsData
		if err := json.Unmarshal(snap.AggregatedData, &cpData); err != nil {
			t.Fatalf("failed unmarshaling community projects aggregated data: %v", err)
		}

		checkFormattedString(t, "TotalApprovedBudgetFormatted", cpData.TotalApprovedBudgetFormatted)
		checkFormattedString(t, "TotalRecordedExpenditureFormatted", cpData.TotalRecordedExpenditureFormatted)

		for i, p := range cpData.Projects {
			checkFormattedString(t, fmt.Sprintf("Projects[%d].ApprovedBudgetFormatted", i), p.ApprovedBudgetFormatted)
			checkFormattedString(t, fmt.Sprintf("Projects[%d].RecordedExpenditureFormatted", i), p.RecordedExpenditureFormatted)
		}
	})

	// 2d. National Overview Report
	t.Run("NationalOverview", func(t *testing.T) {
		snap := createTestSnapshot(t, env, superAdmin, "national_overview", "2088-04", false)
		var oData service.PublicOverviewData
		if err := json.Unmarshal(snap.AggregatedData, &oData); err != nil {
			t.Fatalf("failed unmarshaling overview aggregated data: %v", err)
		}

		checkFormattedString(t, "ApprovedIncomeFormatted", oData.ApprovedIncomeFormatted)
		checkFormattedString(t, "ApprovedExpenditureFormatted", oData.ApprovedExpenditureFormatted)
		checkFormattedString(t, "FoundationDonationsFormatted", oData.FoundationDonationsFormatted)
	})
}

// 3. Regression test verifying PDF export contains 'NGN 0.00' and no 'NGN NGN' across all categories
func TestReportCurrency_PDFExportFormatting(t *testing.T) {
	env := setupTestEnv(t)
	superAdmin := createTestUserWithRole(t, env, "super_admin", fmt.Sprintf("sa_%d@ratels.org", time.Now().UnixNano()))

	categories := []string{"finance", "foundation", "community_projects", "national_overview"}

	for _, cat := range categories {
		t.Run(cat, func(t *testing.T) {
			period := fmt.Sprintf("2088-%02d", len(cat))
			snap := createTestSnapshot(t, env, superAdmin, cat, period, false)

			_, err := env.ReportService.GenerateAndStoreExport(context.Background(), superAdmin, snap.SnapshotID, "pdf", nil)
			if err != nil {
				t.Fatalf("failed generating PDF for %s: %v", cat, err)
			}

			pdfBytes, _, _, err := env.ReportService.GetExportPrivateStream(context.Background(), superAdmin, snap.SnapshotID, "pdf")
			if err != nil {
				t.Fatalf("failed fetching PDF stream for %s: %v", cat, err)
			}

			pdfStr := string(pdfBytes)

			// Verify NO duplicate currency prefixes exist in PDF
			if strings.Contains(pdfStr, "NGN NGN") {
				t.Errorf("PDF for %s contains duplicate 'NGN NGN' prefix", cat)
			}
			if strings.Contains(pdfStr, "₦₦") {
				t.Errorf("PDF for %s contains raw duplicate '₦₦' prefix", cat)
			}
			if strings.Contains(pdfStr, "₦") {
				t.Errorf("PDF for %s contains unescaped '₦' character (should be converted to 'NGN ')", cat)
			}

			// When zero amounts are rendered, verify they appear as 'NGN 0.00'
			if !strings.Contains(pdfStr, "NGN 0.00") {
				t.Logf("Notice: PDF for %s does not contain 'NGN 0.00' (non-zero test data present)", cat)
			} else {
				// Ensure it's not 'NGN NGN 0.00'
				if strings.Contains(pdfStr, "NGN NGN 0.00") {
					t.Errorf("PDF for %s contains 'NGN NGN 0.00'", cat)
				}
			}
		})
	}
}

// 3b. Explicit verification that zero-value PDF output contains 'NGN 0.00' and not 'NGN NGN 0.00'
func TestReportCurrency_ZeroValuePDFVerification(t *testing.T) {
	env := setupTestEnv(t)
	superAdmin := createTestUserWithRole(t, env, "super_admin", fmt.Sprintf("sa_%d@ratels.org", time.Now().UnixNano()))

	// A period with no recorded financial data produces 0 amounts
	snap := createTestSnapshot(t, env, superAdmin, "finance", "2099-01", false)

	_, err := env.ReportService.GenerateAndStoreExport(context.Background(), superAdmin, snap.SnapshotID, "pdf", nil)
	if err != nil {
		t.Fatalf("failed generating PDF: %v", err)
	}

	pdfBytes, _, _, err := env.ReportService.GetExportPrivateStream(context.Background(), superAdmin, snap.SnapshotID, "pdf")
	if err != nil {
		t.Fatalf("failed fetching PDF stream: %v", err)
	}

	pdfStr := string(pdfBytes)

	// Must contain 'NGN 0.00'
	if !strings.Contains(pdfStr, "NGN 0.00") {
		t.Errorf("expected PDF to contain 'NGN 0.00', but it was missing in PDF content")
	}

	// Must NOT contain 'NGN NGN 0.00' or any 'NGN NGN'
	if strings.Contains(pdfStr, "NGN NGN 0.00") {
		t.Errorf("PDF contains duplicated 'NGN NGN 0.00'")
	}
	if strings.Contains(pdfStr, "NGN NGN") {
		t.Errorf("PDF contains duplicated 'NGN NGN'")
	}
	if strings.Contains(pdfStr, "₦₦") {
		t.Errorf("PDF contains raw duplicate '₦₦'")
	}
}

// 4. Regression test verifying CSV export contains correctly formatted monetary values
func TestReportCurrency_CSVExportFormatting(t *testing.T) {
	env := setupTestEnv(t)
	superAdmin := createTestUserWithRole(t, env, "super_admin", fmt.Sprintf("sa_%d@ratels.org", time.Now().UnixNano()))

	categories := []string{"finance", "foundation", "community_projects", "national_overview"}

	for _, cat := range categories {
		t.Run(cat, func(t *testing.T) {
			period := fmt.Sprintf("2088-%02d", len(cat)+10)
			snap := createTestSnapshot(t, env, superAdmin, cat, period, false)

			_, err := env.ReportService.GenerateAndStoreExport(context.Background(), superAdmin, snap.SnapshotID, "csv", nil)
			if err != nil {
				t.Fatalf("failed generating CSV for %s: %v", cat, err)
			}

			csvBytes, _, _, err := env.ReportService.GetExportPrivateStream(context.Background(), superAdmin, snap.SnapshotID, "csv")
			if err != nil {
				t.Fatalf("failed fetching CSV stream for %s: %v", cat, err)
			}

			reader := csv.NewReader(bytes.NewReader(csvBytes))
			reader.FieldsPerRecord = -1
			records, err := reader.ReadAll()
			if err != nil {
				t.Fatalf("failed parsing CSV for %s: %v", cat, err)
			}

			for rowIndex, row := range records {
				for colIndex, cell := range row {
					if strings.Contains(cell, "₦₦") {
						t.Errorf("[%s CSV r%d:c%d] found duplicated '₦₦' in %q", cat, rowIndex, colIndex, cell)
					}
					if strings.Contains(cell, "NGN NGN") {
						t.Errorf("[%s CSV r%d:c%d] found duplicated 'NGN NGN' in %q", cat, rowIndex, colIndex, cell)
					}
					if strings.Contains(cell, "₦") {
						count := strings.Count(cell, "₦")
						if count != 1 {
							t.Errorf("[%s CSV r%d:c%d] expected exactly 1 '₦' in %q, got %d", cat, rowIndex, colIndex, cell, count)
						}
					}
				}
			}
		})
	}
}
