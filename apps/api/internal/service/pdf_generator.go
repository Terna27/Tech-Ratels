package service

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// PDFReportData holds the normalized structure to render a certified or draft PDF report.
type PDFReportData struct {
	Title             string
	PublicReference   string
	ReportType        string
	ReportingPeriod   string
	Scope             string
	PublicationDate   time.Time
	Version           int
	DataHash          string
	Status            string // "draft", "published", "withdrawn"
	Metrics           []PDFMetricItem
	Tables            []PDFTable
	MethodologyNotes  []string
	CertificationNote string
	DataSources       string
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
			} else if r == '•' {
				b.WriteString("-")
			} else {
				b.WriteByte('?')
			}
		}
	}
	return b.String()
}

// GeneratePDF generates a pure standard PDF-1.4 binary document supporting multiple pages and draft watermarks.
func GeneratePDF(data PDFReportData) ([]byte, error) {
	status := strings.ToLower(strings.TrimSpace(data.Status))
	if status == "" {
		status = "published"
	}
	isDraft := status == "draft"
	isWithdrawn := status == "withdrawn"

	// Plan pagination: collect page content streams
	var pageStreams []bytes.Buffer

	// Page geometry (A4: 595.28 x 841.89 points)
	const (
		pageWidth   = 595.28
		pageHeight  = 841.89
		leftMargin  = 36.0
		contentW    = 523.28 // 595.28 - 2*36
		topBannerH  = 72.0
		footerLimit = 65.0
	)

	renderHeader := func(p *bytes.Buffer, pageNum int) float64 {
		// Top banner (Emerald green: #064e3b -> 0.024, 0.306, 0.231)
		p.WriteString("0.024 0.306 0.231 rg 0 770 595.28 72 re f\n")
		// Gold accent line (#d97706 -> 0.851, 0.467, 0.024)
		p.WriteString("0.851 0.467 0.024 rg 0 766 595.28 4 re f\n")

		// Top Banner Text
		p.WriteString("BT /F2 15 Tf 1 1 1 rg 36 810 Td (RATELS WORLDWIDE) Tj ET\n")
		p.WriteString("BT /F1 9 Tf 0.85 0.95 0.9 rg 36 795 Td (PUBLIC TRANSPARENCY, ACCOUNTABILITY & IMPACT PORTAL) Tj ET\n")

		// Right Badge (Certified vs Draft vs Withdrawn)
		if isDraft {
			p.WriteString("0.851 0.467 0.024 rg 410 790 150 24 re f\n")
			p.WriteString("BT /F2 9 Tf 1 1 1 rg 422 798 Td (DRAFT - UNPUBLISHED) Tj ET\n")
		} else if isWithdrawn {
			p.WriteString("0.8 0.1 0.1 rg 410 790 150 24 re f\n")
			p.WriteString("BT /F2 9 Tf 1 1 1 rg 425 798 Td (WITHDRAWN - VOID) Tj ET\n")
		} else {
			p.WriteString("0.08 0.45 0.35 rg 410 790 150 24 re f\n")
			p.WriteString("BT /F2 8.5 Tf 1 1 1 rg 418 800 Td (CERTIFIED DISCLOSURE) Tj ET\n")
			p.WriteString(fmt.Sprintf("BT /F1 7.5 Tf 0.9 0.95 0.95 rg 418 792 Td (Ref: %s) Tj ET\n", escapePDFText(data.PublicReference)))
		}

		curY := 744.0
		// Title & Subtitle
		titleStr := data.Title
		if pageNum > 1 {
			titleStr = fmt.Sprintf("%s (Continued)", data.Title)
		}
		p.WriteString(fmt.Sprintf("BT /F2 13 Tf 0.07 0.1 0.15 rg 36 %.2f Td (%s) Tj ET\n", curY, escapePDFText(titleStr)))
		curY -= 14.0

		statusLabel := strings.ToUpper(status)
		metaStr := fmt.Sprintf("Period: %s   |   Scope: %s   |   Status: %s   |   Version: v%d   |   Date: %s",
			data.ReportingPeriod, data.Scope, statusLabel, data.Version, data.PublicationDate.Format("02 Jan 2006"))
		p.WriteString(fmt.Sprintf("BT /F1 8 Tf 0.35 0.4 0.45 rg 36 %.2f Td (%s) Tj ET\n", curY, escapePDFText(metaStr)))
		curY -= 8.0

		// Divider line
		p.WriteString(fmt.Sprintf("0.85 0.88 0.9 rg 36 %.2f 523.28 1 re f\n", curY))
		curY -= 12.0

		// Prominent Draft Notice on Page
		if isDraft {
			p.WriteString(fmt.Sprintf("0.99 0.96 0.9 rg 36 %.2f 523.28 16 re f\n", curY-16))
			p.WriteString(fmt.Sprintf("0.85 0.5 0.1 RG 0.5 w 36 %.2f 523.28 16 re s\n", curY-16))
			p.WriteString(fmt.Sprintf("BT /F2 8 Tf 0.6 0.25 0 rg 46 %.2f Td (*** OFFICIAL DRAFT: PENDING GOVERNANCE REVIEW AND CERTIFICATION - NOT FOR PUBLIC RELEASE ***) Tj ET\n", curY-12))
			curY -= 22.0
		} else if isWithdrawn {
			p.WriteString(fmt.Sprintf("0.99 0.92 0.92 rg 36 %.2f 523.28 16 re f\n", curY-16))
			p.WriteString(fmt.Sprintf("0.8 0.2 0.2 RG 0.5 w 36 %.2f 523.28 16 re s\n", curY-16))
			p.WriteString(fmt.Sprintf("BT /F2 8 Tf 0.7 0.1 0.1 rg 46 %.2f Td (*** WITHDRAWN PUBLICATION: THIS REPORT HAS BEEN REVOKED AND IS NO LONGER VALID ***) Tj ET\n", curY-12))
			curY -= 22.0
		}

		return curY
	}

	renderFooter := func(p *bytes.Buffer, pageNum, totalPages int) {
		p.WriteString("0.85 0.88 0.9 rg 36 50 523.28 1 re f\n")
		p.WriteString("BT /F1 7 Tf 0.45 0.5 0.55 rg 36 38 Td (Ratels Worldwide Platform - Official Public Transparency Portal - https://ratels-web.onrender.com/reports) Tj ET\n")
		p.WriteString(fmt.Sprintf("BT /F1 7 Tf 0.45 0.5 0.55 rg 275 38 Td (Page %d of %d) Tj ET\n", pageNum, totalPages))
		if isDraft {
			p.WriteString("BT /F2 7 Tf 0.7 0.3 0 rg 480 38 Td (UNAPPROVED DRAFT) Tj ET\n")
		} else {
			p.WriteString("BT /F2 7 Tf 0.024 0.306 0.231 rg 470 38 Td (OFFICIAL RECORD) Tj ET\n")
		}
	}

	// First pass: Build page 1
	var curPage bytes.Buffer
	curPageNum := 1
	curY := renderHeader(&curPage, curPageNum)

	// Render Metrics Summary Grid (on page 1)
	if len(data.Metrics) > 0 {
		count := len(data.Metrics)
		if count > 4 {
			count = 4
		}
		cardWidth := (contentW - float64(count-1)*10.0) / float64(count)
		cardHeight := 46.0

		for i := 0; i < count; i++ {
			m := data.Metrics[i]
			x := leftMargin + float64(i)*(cardWidth+10.0)

			// Box background (#f8fafc)
			curPage.WriteString(fmt.Sprintf("0.97 0.98 0.99 rg %.2f %.2f %.2f %.2f re f\n", x, curY-cardHeight, cardWidth, cardHeight))
			// Box border (#e2e8f0)
			curPage.WriteString(fmt.Sprintf("0.88 0.91 0.94 RG 0.75 w %.2f %.2f %.2f %.2f re s\n", x, curY-cardHeight, cardWidth, cardHeight))

			// Metric Value
			curPage.WriteString(fmt.Sprintf("BT /F2 11.5 Tf 0.024 0.306 0.231 rg %.2f %.2f Td (%s) Tj ET\n", x+8, curY-18, escapePDFText(m.Value)))
			// Metric Label
			curPage.WriteString(fmt.Sprintf("BT /F1 7.5 Tf 0.4 0.45 0.5 rg %.2f %.2f Td (%s) Tj ET\n", x+8, curY-34, escapePDFText(m.Label)))
		}
		curY -= (cardHeight + 16.0)
	}

	// Render Tables with proper pagination across pages
	for _, tbl := range data.Tables {
		colCount := len(tbl.Headers)
		if colCount == 0 {
			continue
		}
		colWidth := contentW / float64(colCount)
		headerH := 18.0
		rowH := 15.0

		// Check if we need a new page for table title + header
		if curY < footerLimit+headerH+rowH*2+40 {
			pageStreams = append(pageStreams, curPage)
			curPage.Reset()
			curPageNum++
			curY = renderHeader(&curPage, curPageNum)
		}

		if tbl.Title != "" {
			curPage.WriteString(fmt.Sprintf("BT /F2 9.5 Tf 0.07 0.1 0.15 rg 36 %.2f Td (%s) Tj ET\n", curY, escapePDFText(tbl.Title)))
			curY -= 14.0
		}

		// Table Header
		curPage.WriteString(fmt.Sprintf("0.93 0.95 0.96 rg 36 %.2f 523.28 %.2f re f\n", curY-headerH, headerH))
		curPage.WriteString(fmt.Sprintf("0.8 0.85 0.88 RG 0.5 w 36 %.2f 523.28 %.2f re s\n", curY-headerH, headerH))
		for ci, h := range tbl.Headers {
			hx := leftMargin + float64(ci)*colWidth + 6.0
			curPage.WriteString(fmt.Sprintf("BT /F2 8 Tf 0.15 0.2 0.25 rg %.2f %.2f Td (%s) Tj ET\n", hx, curY-13, escapePDFText(h)))
		}
		curY -= headerH

		// Table Rows
		for ri, row := range tbl.Rows {
			// Check if row fits on current page
			if curY < footerLimit+rowH+50 {
				// Advance to new page
				pageStreams = append(pageStreams, curPage)
				curPage.Reset()
				curPageNum++
				curY = renderHeader(&curPage, curPageNum)

				// Repeat Table Header
				if tbl.Title != "" {
					curPage.WriteString(fmt.Sprintf("BT /F2 9.5 Tf 0.07 0.1 0.15 rg 36 %.2f Td (%s - Continued) Tj ET\n", curY, escapePDFText(tbl.Title)))
					curY -= 14.0
				}
				curPage.WriteString(fmt.Sprintf("0.93 0.95 0.96 rg 36 %.2f 523.28 %.2f re f\n", curY-headerH, headerH))
				curPage.WriteString(fmt.Sprintf("0.8 0.85 0.88 RG 0.5 w 36 %.2f 523.28 %.2f re s\n", curY-headerH, headerH))
				for ci, h := range tbl.Headers {
					hx := leftMargin + float64(ci)*colWidth + 6.0
					curPage.WriteString(fmt.Sprintf("BT /F2 8 Tf 0.15 0.2 0.25 rg %.2f %.2f Td (%s) Tj ET\n", hx, curY-13, escapePDFText(h)))
				}
				curY -= headerH
			}

			// Alternate row fill
			if ri%2 == 1 {
				curPage.WriteString(fmt.Sprintf("0.98 0.99 0.99 rg 36 %.2f 523.28 %.2f re f\n", curY-rowH, rowH))
			}
			curPage.WriteString(fmt.Sprintf("0.9 0.92 0.94 RG 0.5 w 36 %.2f 523.28 %.2f re s\n", curY-rowH, rowH))

			for ci, cell := range row {
				if ci >= colCount {
					break
				}
				cx := leftMargin + float64(ci)*colWidth + 6.0
				curPage.WriteString(fmt.Sprintf("BT /F1 7.5 Tf 0.2 0.25 0.3 rg %.2f %.2f Td (%s) Tj ET\n", cx, curY-11, escapePDFText(cell)))
			}
			curY -= rowH
		}
		curY -= 12.0
	}

	// Data Sources and Methodology block
	dsH := 32.0
	if curY < footerLimit+dsH+60 {
		pageStreams = append(pageStreams, curPage)
		curPage.Reset()
		curPageNum++
		curY = renderHeader(&curPage, curPageNum)
	}

	curPage.WriteString(fmt.Sprintf("0.98 0.98 0.99 rg 36 %.2f 523.28 %.2f re f\n", curY-dsH, dsH))
	curPage.WriteString(fmt.Sprintf("0.85 0.88 0.92 RG 0.5 w 36 %.2f 523.28 %.2f re s\n", curY-dsH, dsH))

	dsText := data.DataSources
	if dsText == "" {
		dsText = "Ratels Worldwide verified field monitoring records, verified attendance registers, and audited accounts."
	}
	curPage.WriteString(fmt.Sprintf("BT /F2 7.5 Tf 0.2 0.25 0.35 rg 42 %.2f Td (DATA SOURCES & METHODOLOGY:) Tj ET\n", curY-12))
	curPage.WriteString(fmt.Sprintf("BT /F1 7 Tf 0.35 0.4 0.45 rg 42 %.2f Td (%s) Tj ET\n", curY-23, escapePDFText(dsText)))
	curY -= (dsH + 10.0)

	// Governance & Digital Certification Stamp
	certBoxH := 46.0
	if curY < footerLimit+certBoxH {
		pageStreams = append(pageStreams, curPage)
		curPage.Reset()
		curPageNum++
		curY = renderHeader(&curPage, curPageNum)
	}

	if isDraft {
		curPage.WriteString(fmt.Sprintf("0.99 0.98 0.94 rg 36 %.2f 523.28 %.2f re f\n", curY-certBoxH, certBoxH))
		curPage.WriteString(fmt.Sprintf("0.85 0.6 0.1 RG 0.75 w 36 %.2f 523.28 %.2f re s\n", curY-certBoxH, certBoxH))
		curPage.WriteString(fmt.Sprintf("BT /F2 8 Tf 0.7 0.35 0 rg 46 %.2f Td (PROVISIONAL DRAFT SEAL - PENDING GOVERNANCE REVIEW & CERTIFICATION) Tj ET\n", curY-16))
		curPage.WriteString(fmt.Sprintf("BT /F1 7.5 Tf 0.3 0.3 0.35 rg 46 %.2f Td (This draft is an uncertified preliminary aggregation for internal administrative audit only.) Tj ET\n", curY-28))
		curPage.WriteString(fmt.Sprintf("BT /F1 6.5 Tf 0.4 0.45 0.5 rg 46 %.2f Td (Cryptographic SHA-256 Digest: %s) Tj ET\n", curY-38, escapePDFText(data.DataHash)))
	} else if isWithdrawn {
		curPage.WriteString(fmt.Sprintf("0.99 0.94 0.94 rg 36 %.2f 523.28 %.2f re f\n", curY-certBoxH, certBoxH))
		curPage.WriteString(fmt.Sprintf("0.8 0.2 0.2 RG 0.75 w 36 %.2f 523.28 %.2f re s\n", curY-certBoxH, certBoxH))
		curPage.WriteString(fmt.Sprintf("BT /F2 8 Tf 0.8 0.1 0.1 rg 46 %.2f Td (REVOCATION & WITHDRAWAL NOTICE) Tj ET\n", curY-16))
		curPage.WriteString(fmt.Sprintf("BT /F1 7.5 Tf 0.3 0.3 0.35 rg 46 %.2f Td (This disclosure was formally withdrawn by governance authority and is no longer certified.) Tj ET\n", curY-28))
		curPage.WriteString(fmt.Sprintf("BT /F1 6.5 Tf 0.4 0.45 0.5 rg 46 %.2f Td (Cryptographic SHA-256 Digest: %s) Tj ET\n", curY-38, escapePDFText(data.DataHash)))
	} else {
		curPage.WriteString(fmt.Sprintf("0.96 0.98 0.96 rg 36 %.2f 523.28 %.2f re f\n", curY-certBoxH, certBoxH))
		curPage.WriteString(fmt.Sprintf("0.7 0.82 0.7 RG 0.75 w 36 %.2f 523.28 %.2f re s\n", curY-certBoxH, certBoxH))

		certNote := data.CertificationNote
		if certNote == "" {
			certNote = "Certified official publication authorized by Ratels Worldwide Governance & Transparency Office."
		}
		curPage.WriteString(fmt.Sprintf("BT /F2 8 Tf 0.024 0.306 0.231 rg 46 %.2f Td (GOVERNANCE CERTIFICATION & DATA INTEGRITY SEAL) Tj ET\n", curY-16))
		curPage.WriteString(fmt.Sprintf("BT /F1 7.5 Tf 0.2 0.25 0.3 rg 46 %.2f Td (%s) Tj ET\n", curY-28, escapePDFText(certNote)))
		curPage.WriteString(fmt.Sprintf("BT /F1 6.5 Tf 0.4 0.45 0.5 rg 46 %.2f Td (Cryptographic SHA-256 Digest: %s) Tj ET\n", curY-38, escapePDFText(data.DataHash)))
	}

	pageStreams = append(pageStreams, curPage)
	totalPages := len(pageStreams)

	// Now append footers to each page stream with actual totalPages count
	for i := 0; i < totalPages; i++ {
		renderFooter(&pageStreams[i], i+1, totalPages)
	}

	// Assemble final PDF document
	var pdf bytes.Buffer
	pdf.WriteString("%PDF-1.4\n%\xe2\xe3\xcf\xd3\n")

	var offsets []int
	addObject := func(body string) {
		offsets = append(offsets, pdf.Len())
		pdf.WriteString(body)
	}

	// 1: Catalog
	addObject("1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n")

	// Collect page object IDs
	// Pages obj is 2. Font 1 is 4, Font 2 is 5.
	// For each page i (0 to totalPages-1):
	//   pageObjID = 6 + i*2
	//   streamObjID = 7 + i*2
	kids := make([]string, totalPages)
	for i := 0; i < totalPages; i++ {
		kids[i] = fmt.Sprintf("%d 0 R", 6+i*2)
	}

	// 2: Pages
	addObject(fmt.Sprintf("2 0 obj\n<< /Type /Pages /Kids [%s] /Count %d >>\nendobj\n", strings.Join(kids, " "), totalPages))

	// 4: Font Helvetica
	addObject("4 0 obj\n<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>\nendobj\n")

	// 5: Font Helvetica-Bold
	addObject("5 0 obj\n<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica-Bold /Encoding /WinAnsiEncoding >>\nendobj\n")

	// Add Page and Stream objects for each page
	for i := 0; i < totalPages; i++ {
		pageObjID := 6 + i*2
		streamObjID := 7 + i*2
		contentBytes := pageStreams[i].Bytes()

		// Page object
		addObject(fmt.Sprintf("%d 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595.28 841.89] /Resources << /Font << /F1 4 0 R /F2 5 0 R >> >> /Contents %d 0 R >>\nendobj\n", pageObjID, streamObjID))

		// Stream object
		streamHeader := fmt.Sprintf("%d 0 obj\n<< /Length %d >>\nstream\n", streamObjID, len(contentBytes))
		offsets = append(offsets, pdf.Len())
		pdf.WriteString(streamHeader)
		pdf.Write(contentBytes)
		pdf.WriteString("\nendstream\nendobj\n")
	}

	// Cross-reference table
	xrefOffset := pdf.Len()
	// Map offset by object number
	objOffsets := make(map[int]int)
	objOffsets[1] = offsets[0]
	objOffsets[2] = offsets[1]
	objOffsets[4] = offsets[2]
	objOffsets[5] = offsets[3]
	for i := 0; i < totalPages; i++ {
		objOffsets[6+i*2] = offsets[4+i*2]
		objOffsets[7+i*2] = offsets[4+i*2+1]
	}

	maxObjID := 5 + totalPages*2
	pdf.WriteString(fmt.Sprintf("xref\n0 %d\n", maxObjID+1))
	pdf.WriteString("0000000000 65535 f \n")
	for id := 1; id <= maxObjID; id++ {
		if off, ok := objOffsets[id]; ok {
			pdf.WriteString(fmt.Sprintf("%010d 00000 n \n", off))
		} else {
			pdf.WriteString("0000000000 65535 f \n")
		}
	}

	pdf.WriteString(fmt.Sprintf("trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", maxObjID+1, xrefOffset))

	return pdf.Bytes(), nil
}

// ComputeDataHash generates an authoritative SHA-256 hash of report data.
func ComputeDataHash(payload []byte) string {
	h := sha256.Sum256(payload)
	return hex.EncodeToString(h[:])
}
