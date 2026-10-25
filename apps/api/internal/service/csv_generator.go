package service

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"strings"
	"time"
)

// CSVReportData represents tabular data to export to CSV.
type CSVReportData struct {
	Title           string
	PublicReference string
	ReportType      string
	ReportingPeriod string
	Scope           string
	PublicationDate time.Time
	Status          string // "draft", "published", "withdrawn"
	DataHash        string
	Headers         []string
	Rows            [][]string
}

// SanitizeCSVValue prevents spreadsheet formula injection (CSV Injection).
// Prepends a single quote if the first character is an active trigger (=, +, -, @, tab, CR).
func SanitizeCSVValue(val string) string {
	if len(val) > 0 {
		first := val[0]
		if first == '=' || first == '+' || first == '-' || first == '@' || first == '\t' || first == '\r' {
			return "'" + val
		}
	}
	return val
}

// GenerateCSV produces a safe, RFC 4180 compliant CSV byte stream with injection protection.
func GenerateCSV(data CSVReportData) ([]byte, error) {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	statusStr := strings.ToUpper(strings.TrimSpace(data.Status))
	if statusStr == "" {
		statusStr = "PUBLISHED"
	}

	// Metadata header
	metaRows := [][]string{
		{"# Platform", "Ratels Worldwide Platform - Official Report Export"},
		{"Report Title", SanitizeCSVValue(data.Title)},
		{"Public Reference", SanitizeCSVValue(data.PublicReference)},
		{"Report Type", SanitizeCSVValue(data.ReportType)},
		{"Reporting Period", SanitizeCSVValue(data.ReportingPeriod)},
		{"Geographic Scope", SanitizeCSVValue(data.Scope)},
		{"Publication Status", SanitizeCSVValue(statusStr)},
		{"Publication Date", data.PublicationDate.Format(time.RFC3339)},
		{"Data Hash (SHA-256)", SanitizeCSVValue(data.DataHash)},
		{"Confidentiality Note", "Strict privacy protection applied. Internal ledgers and personal PII excluded."},
		{}, // Blank line separator
	}

	for _, mr := range metaRows {
		if err := writer.Write(mr); err != nil {
			return nil, fmt.Errorf("failed writing csv metadata: %w", err)
		}
	}

	// Column Headers
	sanitizedHeaders := make([]string, len(data.Headers))
	for i, h := range data.Headers {
		sanitizedHeaders[i] = SanitizeCSVValue(h)
	}
	if err := writer.Write(sanitizedHeaders); err != nil {
		return nil, fmt.Errorf("failed writing csv headers: %w", err)
	}

	// Data Rows
	for _, row := range data.Rows {
		sanitizedRow := make([]string, len(row))
		for i, cell := range row {
			sanitizedRow[i] = SanitizeCSVValue(cell)
		}
		if err := writer.Write(sanitizedRow); err != nil {
			return nil, fmt.Errorf("failed writing csv row: %w", err)
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, fmt.Errorf("csv flush error: %w", err)
	}

	return buf.Bytes(), nil
}
