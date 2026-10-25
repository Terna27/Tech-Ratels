package service

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// PDFReportData holds the normalized structure to render a certified PDF report.
type PDFReportData struct {
	Title             string
	PublicReference   string
	ReportType        string
	ReportingPeriod   string
	Scope             string
	PublicationDate   time.Time
	Version           int
	DataHash          string
	Metrics           []PDFMetricItem
	Tables            []PDFTable
	MethodologyNotes  []string
	CertificationNote string
}

type PDFMetricItem struct {
	Label string
	Value string
}

type PDFTable struct {
	Title   string
	Headers []string
	Rows    [][]string
}

func escapePDFText(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString("\\\\")
		case '(':
			b.WriteString("\\(")
		case ')':
			b.WriteString("\\)")
		case '\r', '\n', '\t':
			b.WriteByte(' ')
		default:
			if r >= 32 && r <= 126 {
				b.WriteRune(r)
			} else if r == '₦' {
				b.WriteString("NGN ")
			} else {
				// Map common unicode or omit
				if r == '•' {
					b.WriteString("-")
				} else {
					b.WriteByte('?')
				}
			}
		}
	}
	return b.String()
}

// GeneratePDF generates a pure standard PDF-1.4 binary document.
func GeneratePDF(data PDFReportData) ([]byte, error) {
	var stream bytes.Buffer

	// Page coordinates: A4 is 595.28 x 841.89 points
	// Top banner (Emerald green: #064e3b -> rgb(6/255, 78/255, 59/255) = 0.024, 0.306, 0.231)
	stream.WriteString("0.024 0.306 0.231 rg 0 770 595.28 72 re f\n")
	// Gold accent line (#d97706 -> 0.851, 0.467, 0.024)
	stream.WriteString("0.851 0.467 0.024 rg 0 766 595.28 4 re f\n")

	// Header Text
	stream.WriteString("BT /F2 15 Tf 1 1 1 rg 36 810 Td (NIGERIAN RATELS MANAGEMENT SYSTEM) Tj ET\n")
	stream.WriteString("BT /F1 9 Tf 0.85 0.95 0.9 rg 36 795 Td (PUBLIC TRANSPARENCY, ACCOUNTABILITY & IMPACT PORTAL) Tj ET\n")
	stream.WriteString("BT /F2 8 Tf 1 1 1 rg 420 810 Td (CERTIFIED DISCLOSURE) Tj ET\n")
	stream.WriteString(fmt.Sprintf("BT /F1 7.5 Tf 0.85 0.95 0.9 rg 420 795 Td (Ref: %s) Tj ET\n", escapePDFText(data.PublicReference)))

	// Report Title & Sub-header
	stream.WriteString(fmt.Sprintf("BT /F2 14 Tf 0.07 0.1 0.15 rg 36 740 Td (%s) Tj ET\n", escapePDFText(data.Title)))
	metaStr := fmt.Sprintf("Period: %s   |   Scope: %s   |   Version: v%d   |   Date: %s",
		data.ReportingPeriod, data.Scope, data.Version, data.PublicationDate.Format("02 Jan 2006"))
	stream.WriteString(fmt.Sprintf("BT /F1 8.5 Tf 0.35 0.4 0.45 rg 36 726 Td (%s) Tj ET\n", escapePDFText(metaStr)))

	// Divider line
	stream.WriteString("0.85 0.88 0.9 rg 36 718 523.28 1 re f\n")

	curY := 700.0

	// Render Metrics Summary Grid (up to 4 boxes)
	if len(data.Metrics) > 0 {
		count := len(data.Metrics)
		if count > 4 {
			count = 4
		}
		cardWidth := (523.28 - float64(count-1)*10) / float64(count)
		cardHeight := 48.0

		for i := 0; i < count; i++ {
			m := data.Metrics[i]
			x := 36.0 + float64(i)*(cardWidth+10.0)

			// Box background (#f8fafc)
			stream.WriteString(fmt.Sprintf("0.97 0.98 0.99 rg %.2f %.2f %.2f %.2f re f\n", x, curY-cardHeight, cardWidth, cardHeight))
			// Box border (#e2e8f0)
			stream.WriteString(fmt.Sprintf("0.88 0.91 0.94 RG 0.75 w %.2f %.2f %.2f %.2f re s\n", x, curY-cardHeight, cardWidth, cardHeight))

			// Metric Value
			stream.WriteString(fmt.Sprintf("BT /F2 12 Tf 0.024 0.306 0.231 rg %.2f %.2f Td (%s) Tj ET\n", x+8, curY-20, escapePDFText(m.Value)))
			// Metric Label
			stream.WriteString(fmt.Sprintf("BT /F1 7.5 Tf 0.4 0.45 0.5 rg %.2f %.2f Td (%s) Tj ET\n", x+8, curY-36, escapePDFText(m.Label)))
		}
		curY -= (cardHeight + 20.0)
	}

	// Render Tables
	for _, tbl := range data.Tables {
		if curY < 180 {
			break // Keep single/clean page boundary
		}

		if tbl.Title != "" {
			stream.WriteString(fmt.Sprintf("BT /F2 10 Tf 0.07 0.1 0.15 rg 36 %.2f Td (%s) Tj ET\n", curY, escapePDFText(tbl.Title)))
			curY -= 14.0
		}

		// Table Header Row
		colCount := len(tbl.Headers)
		if colCount == 0 {
			continue
		}
		colWidth := 523.28 / float64(colCount)
		headerH := 18.0

		// Header background
		stream.WriteString(fmt.Sprintf("0.93 0.95 0.96 rg 36 %.2f 523.28 %.2f re f\n", curY-headerH, headerH))
		stream.WriteString(fmt.Sprintf("0.8 0.85 0.88 RG 0.5 w 36 %.2f 523.28 %.2f re s\n", curY-headerH, headerH))

		for ci, h := range tbl.Headers {
			hx := 36.0 + float64(ci)*colWidth + 6.0
			stream.WriteString(fmt.Sprintf("BT /F2 8 Tf 0.15 0.2 0.25 rg %.2f %.2f Td (%s) Tj ET\n", hx, curY-13, escapePDFText(h)))
		}
		curY -= headerH

		// Table Data Rows
		rowH := 16.0
		for ri, row := range tbl.Rows {
			if curY < 140 {
				break
			}
			// Alternate row fill
			if ri%2 == 1 {
				stream.WriteString(fmt.Sprintf("0.98 0.99 0.99 rg 36 %.2f 523.28 %.2f re f\n", curY-rowH, rowH))
			}
			stream.WriteString(fmt.Sprintf("0.9 0.92 0.94 RG 0.5 w 36 %.2f 523.28 %.2f re s\n", curY-rowH, rowH))

			for ci, cell := range row {
				if ci >= colCount {
					break
				}
				cx := 36.0 + float64(ci)*colWidth + 6.0
				stream.WriteString(fmt.Sprintf("BT /F1 7.5 Tf 0.2 0.25 0.3 rg %.2f %.2f Td (%s) Tj ET\n", cx, curY-11, escapePDFText(cell)))
			}
			curY -= rowH
		}
		curY -= 14.0
	}

	// Governance & Digital Certification Stamp
	if curY > 110 {
		boxH := 46.0
		stream.WriteString(fmt.Sprintf("0.96 0.98 0.96 rg 36 %.2f 523.28 %.2f re f\n", curY-boxH, boxH))
		stream.WriteString(fmt.Sprintf("0.7 0.82 0.7 RG 0.75 w 36 %.2f 523.28 %.2f re s\n", curY-boxH, boxH))

		certNote := data.CertificationNote
		if certNote == "" {
			certNote = "Certified official publication authorized by Nigerian Ratels National Governance & Transparency Office."
		}
		stream.WriteString(fmt.Sprintf("BT /F2 8 Tf 0.024 0.306 0.231 rg 46 %.2f Td (GOVERNANCE CERTIFICATION & DATA INTEGRITY SEAL) Tj ET\n", curY-16))
		stream.WriteString(fmt.Sprintf("BT /F1 7.5 Tf 0.2 0.25 0.3 rg 46 %.2f Td (%s) Tj ET\n", curY-28, escapePDFText(certNote)))
		stream.WriteString(fmt.Sprintf("BT /F1 6.5 Tf 0.4 0.45 0.5 rg 46 %.2f Td (Cryptographic SHA-256 Hash: %s) Tj ET\n", curY-38, escapePDFText(data.DataHash)))
		curY -= (boxH + 10.0)
	}

	// Bottom Footer
	stream.WriteString("0.85 0.88 0.9 rg 36 50 523.28 1 re f\n")
	stream.WriteString("BT /F1 7 Tf 0.45 0.5 0.55 rg 36 38 Td (Nigerian Ratels Management System - Official Public Transparency Portal - https://ratels.onrender.com/reports) Tj ET\n")
	stream.WriteString("BT /F2 7 Tf 0.024 0.306 0.231 rg 470 38 Td (STRICT CONFIDENTIALITY PRESERVED) Tj ET\n")

	contentBytes := stream.Bytes()

	// Assemble PDF objects and xref
	var pdf bytes.Buffer
	pdf.WriteString("%PDF-1.4\n%\xe2\xe3\xcf\xd3\n")

	var offsets []int
	addObject := func(body string) {
		offsets = append(offsets, pdf.Len())
		pdf.WriteString(body)
	}

	// 1: Catalog
	addObject("1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n")

	// 2: Pages
	addObject("2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n")

	// 3: Page
	addObject("3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595.28 841.89] /Resources << /Font << /F1 4 0 R /F2 5 0 R >> >> /Contents 6 0 R >>\nendobj\n")

	// 4: Font Helvetica
	addObject("4 0 obj\n<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>\nendobj\n")

	// 5: Font Helvetica-Bold
	addObject("5 0 obj\n<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica-Bold /Encoding /WinAnsiEncoding >>\nendobj\n")

	// 6: Contents stream
	streamHeader := fmt.Sprintf("6 0 obj\n<< /Length %d >>\nstream\n", len(contentBytes))
	offsets = append(offsets, pdf.Len())
	pdf.WriteString(streamHeader)
	pdf.Write(contentBytes)
	pdf.WriteString("\nendstream\nendobj\n")

	// Cross-reference table
	xrefOffset := pdf.Len()
	pdf.WriteString(fmt.Sprintf("xref\n0 %d\n", len(offsets)+1))
	pdf.WriteString("0000000000 65535 f \n")
	for _, offset := range offsets {
		pdf.WriteString(fmt.Sprintf("%010d 00000 n \n", offset))
	}

	pdf.WriteString(fmt.Sprintf("trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(offsets)+1, xrefOffset))

	return pdf.Bytes(), nil
}

// ComputeDataHash generates an authoritative SHA-256 hash of report data.
func ComputeDataHash(payload []byte) string {
	h := sha256.Sum256(payload)
	return hex.EncodeToString(h[:])
}
