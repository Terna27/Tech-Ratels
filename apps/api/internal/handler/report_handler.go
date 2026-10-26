package handler

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"ratels-api/internal/security"
	"ratels-api/internal/service"
	"ratels-api/internal/worker"
)

// ReportSchedulerRunner defines the interface for triggering scheduled report cycles.
type ReportSchedulerRunner interface {
	RunScheduledCycle(ctx context.Context, params worker.RunSchedulerParams) (*worker.SchedulerRunResult, error)
}

type ReportHandler struct {
	reportService *service.ReportService
	scheduler     ReportSchedulerRunner
	cronSecret    string
	rateLimiter   *security.DownloadRateLimiter
}

func NewReportHandler(reportService *service.ReportService, rateLimiter ...*security.DownloadRateLimiter) *ReportHandler {
	var rl *security.DownloadRateLimiter
	if len(rateLimiter) > 0 && rateLimiter[0] != nil {
		rl = rateLimiter[0]
	} else {
		rl = security.NewDownloadRateLimiter(30, time.Minute)
	}
	return &ReportHandler{
		reportService: reportService,
		rateLimiter:   rl,
	}
}

// SetRateLimiter configures a custom download rate limiter (e.g. for testing).
func (h *ReportHandler) SetRateLimiter(rl *security.DownloadRateLimiter) {
	h.rateLimiter = rl
}

// SetScheduler attaches a report scheduler engine and cron authentication secret.
func (h *ReportHandler) SetScheduler(scheduler ReportSchedulerRunner, cronSecret string) {
	h.scheduler = scheduler
	h.cronSecret = cronSecret
}

// -----------------------------------------------------------------------
// Public Endpoints (Accessible without login)
// -----------------------------------------------------------------------

func (h *ReportHandler) GetOverview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	scope := r.URL.Query().Get("scope")
	zoneID := r.URL.Query().Get("zone_id")
	stateID := r.URL.Query().Get("state_id")
	lgaID := r.URL.Query().Get("lga_id")
	period := r.URL.Query().Get("period")

	var zPtr, sPtr, lPtr *string
	if zoneID != "" {
		zPtr = &zoneID
	}
	if stateID != "" {
		sPtr = &stateID
	}
	if lgaID != "" {
		lPtr = &lgaID
	}

	data, err := h.reportService.GetPublicOverview(
		ctx, &scope, zPtr, sPtr, lPtr, period,
	)
	if err != nil {
		if errors.Is(err, service.ErrReportPublicationNotFound) {
			ErrorJSON(w, http.StatusNotFound, "REPORT_NOT_PUBLISHED",
				"No overview report has been published for this period and location", "")
			return
		}
		if errors.Is(err, service.ErrInvalidReportScope) ||
			errors.Is(err, service.ErrInvalidGeographicFilter) ||
			strings.Contains(err.Error(), "unsupported") ||
			strings.Contains(err.Error(), "cannot include") ||
			strings.Contains(err.Error(), "requires") {
			ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error(), nil)
			return
		}

		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR",
			"Failed to retrieve public overview report", err.Error())
		return
	}

	JSON(w, http.StatusOK, data)
}

func (h *ReportHandler) GetCleanups(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	scope := r.URL.Query().Get("scope")
	zoneID := r.URL.Query().Get("zone_id")
	stateID := r.URL.Query().Get("state_id")
	lgaID := r.URL.Query().Get("lga_id")
	period := r.URL.Query().Get("period")

	var zPtr, sPtr, lPtr *string
	if zoneID != "" {
		zPtr = &zoneID
	}
	if stateID != "" {
		sPtr = &stateID
	}
	if lgaID != "" {
		lPtr = &lgaID
	}

	data, err := h.reportService.GetPublicCleanups(ctx, &scope, zPtr, sPtr, lPtr, period)
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve cleanup impact report", err.Error())
		return
	}

	JSON(w, http.StatusOK, data)
}

func (h *ReportHandler) GetParticipation(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	scope := r.URL.Query().Get("scope")
	zoneID := r.URL.Query().Get("zone_id")
	stateID := r.URL.Query().Get("state_id")
	lgaID := r.URL.Query().Get("lga_id")
	period := r.URL.Query().Get("period")

	var zPtr, sPtr, lPtr *string
	if zoneID != "" {
		zPtr = &zoneID
	}
	if stateID != "" {
		sPtr = &stateID
	}
	if lgaID != "" {
		lPtr = &lgaID
	}

	data, err := h.reportService.GetPublicParticipation(ctx, &scope, zPtr, sPtr, lPtr, period)
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve participation report", err.Error())
		return
	}

	JSON(w, http.StatusOK, data)
}

func (h *ReportHandler) GetFinance(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	scope := r.URL.Query().Get("scope")
	stateID := r.URL.Query().Get("state_id")
	period := r.URL.Query().Get("period")

	var sPtr *string
	if stateID != "" {
		sPtr = &stateID
	}

	data, err := h.reportService.GetPublicFinance(ctx, &scope, sPtr, period)
	if err != nil {
		if errors.Is(err, service.ErrReportPublicationNotFound) {
			ErrorJSON(w, http.StatusNotFound, "REPORT_NOT_PUBLISHED",
				"No financial report has been published for this period and location", "")
			return
		}
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR",
			"Failed to retrieve financial transparency report", err.Error())
		return
	}

	JSON(w, http.StatusOK, data)
}

func (h *ReportHandler) GetFoundation(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	period := r.URL.Query().Get("period")

	data, err := h.reportService.GetPublicFoundation(ctx, period)
	if err != nil {
		if errors.Is(err, service.ErrReportPublicationNotFound) {
			ErrorJSON(w, http.StatusNotFound, "REPORT_NOT_PUBLISHED",
				"No foundation report has been published for this period", "")
			return
		}

		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR",
			"Failed to retrieve foundation report", err.Error())
		return
	}

	JSON(w, http.StatusOK, data)
}

func (h *ReportHandler) GetCommunityProjects(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	stateID := r.URL.Query().Get("state_id")
	lgaID := r.URL.Query().Get("lga_id")
	period := r.URL.Query().Get("period")

	var sPtr, lPtr *string
	if stateID != "" {
		sPtr = &stateID
	}
	if lgaID != "" {
		lPtr = &lgaID
	}

	data, err := h.reportService.GetPublicCommunityProjects(ctx, sPtr, lPtr, period)
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve community projects report", err.Error())
		return
	}

	JSON(w, http.StatusOK, data)
}

func (h *ReportHandler) ListPublications(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	reportType := r.URL.Query().Get("report_type")
	scopeType := r.URL.Query().Get("scope_type")
	stateID := r.URL.Query().Get("state_id")
	period := r.URL.Query().Get("period")

	page := int32(1)
	pageSize := int32(20)
	if pStr := r.URL.Query().Get("page"); pStr != "" {
		if pVal, err := strconv.Atoi(pStr); err == nil && pVal > 0 {
			page = int32(pVal)
		}
	}
	if psStr := r.URL.Query().Get("page_size"); psStr != "" {
		if psVal, err := strconv.Atoi(psStr); err == nil && psVal > 0 && psVal <= 100 {
			pageSize = int32(psVal)
		}
	}

	res, err := h.reportService.ListPublicPublications(ctx, reportType, scopeType, stateID, period, page, pageSize)
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list published reports", err.Error())
		return
	}

	JSON(w, http.StatusOK, res)
}

func (h *ReportHandler) GetPublicationByID(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chi.URLParam(r, "id")
	if id == "" {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Report ID or reference is required", nil)
		return
	}

	res, err := h.reportService.GetPublicPublicationByID(ctx, id)
	if err != nil {
		if err == service.ErrReportPublicationNotFound {
			ErrorJSON(w, http.StatusNotFound, "NOT_FOUND", "Published report not found", nil)
			return
		}
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve report", err.Error())
		return
	}

	JSON(w, http.StatusOK, res)
}

func (h *ReportHandler) DownloadPDF(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	clientIP := security.GetClientIP(r)

	// Rate limiting enforcement
	if h.rateLimiter != nil {
		allowed, retryAfter := h.rateLimiter.Allow(clientIP)
		if !allowed {
			secs := int(retryAfter.Seconds())
			if secs < 1 {
				secs = 1
			}
			w.Header().Set("Retry-After", strconv.Itoa(secs))
			h.reportService.RecordDownloadAudit(ctx, nil, "report_download_throttled", clientIP, map[string]interface{}{
				"format": "pdf",
				"reason": "rate_limit_exceeded",
			})
			ErrorJSON(w, http.StatusTooManyRequests, "RATE_LIMIT_EXCEEDED", "Download rate limit exceeded. Please wait a moment before trying again.", nil)
			return
		}
	}

	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Report ID or reference is required", nil)
		return
	}

	pdfBytes, filename, err := h.reportService.ExportReportPDF(ctx, id)
	if err != nil {
		if errors.Is(err, service.ErrReportPublicationNotFound) ||
			errors.Is(err, service.ErrReportSnapshotNotFound) ||
			errors.Is(err, service.ErrReportWithdrawn) ||
			errors.Is(err, service.ErrExportRevoked) ||
			errors.Is(err, service.ErrUnauthorizedExportAccess) ||
			errors.Is(err, service.ErrReportNotAvailable) {
			ErrorJSON(w, http.StatusNotFound, "NOT_FOUND", "The requested publication is not available for public download", nil)
			return
		}

		h.reportService.RecordDownloadAudit(ctx, nil, "report_download_failed", clientIP, map[string]interface{}{
			"publication_id": id,
			"format":         "pdf",
			"error":          "storage_read_error",
		})
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed generating report PDF", nil)
		return
	}

	h.reportService.RecordDownloadAudit(ctx, nil, "report_downloaded", clientIP, map[string]interface{}{
		"publication_id":  id,
		"format":          "pdf",
		"file_name":       filename,
		"file_size_bytes": len(pdfBytes),
		"status":          "success",
	})

	safeName := security.SanitizeFilename(filename)
	if safeName == "" {
		safeName = "report.pdf"
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, safeName))
	w.Header().Set("Content-Length", strconv.Itoa(len(pdfBytes)))
	w.Header().Set("Cache-Control", "private, no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, bytes.NewReader(pdfBytes))
}

func (h *ReportHandler) DownloadCSV(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	clientIP := security.GetClientIP(r)

	// Rate limiting enforcement
	if h.rateLimiter != nil {
		allowed, retryAfter := h.rateLimiter.Allow(clientIP)
		if !allowed {
			secs := int(retryAfter.Seconds())
			if secs < 1 {
				secs = 1
			}
			w.Header().Set("Retry-After", strconv.Itoa(secs))
			h.reportService.RecordDownloadAudit(ctx, nil, "report_download_throttled", clientIP, map[string]interface{}{
				"format": "csv",
				"reason": "rate_limit_exceeded",
			})
			ErrorJSON(w, http.StatusTooManyRequests, "RATE_LIMIT_EXCEEDED", "Download rate limit exceeded. Please wait a moment before trying again.", nil)
			return
		}
	}

	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Report ID or reference is required", nil)
		return
	}

	csvBytes, filename, err := h.reportService.ExportReportCSV(ctx, id)
	if err != nil {
		if errors.Is(err, service.ErrReportPublicationNotFound) ||
			errors.Is(err, service.ErrReportSnapshotNotFound) ||
			errors.Is(err, service.ErrReportWithdrawn) ||
			errors.Is(err, service.ErrExportRevoked) ||
			errors.Is(err, service.ErrUnauthorizedExportAccess) ||
			errors.Is(err, service.ErrReportNotAvailable) {
			ErrorJSON(w, http.StatusNotFound, "NOT_FOUND", "The requested publication is not available for public download", nil)
			return
		}

		h.reportService.RecordDownloadAudit(ctx, nil, "report_download_failed", clientIP, map[string]interface{}{
			"publication_id": id,
			"format":         "csv",
			"error":          "storage_read_error",
		})
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed generating report CSV", nil)
		return
	}

	h.reportService.RecordDownloadAudit(ctx, nil, "report_downloaded", clientIP, map[string]interface{}{
		"publication_id":  id,
		"format":          "csv",
		"file_name":       filename,
		"file_size_bytes": len(csvBytes),
		"status":          "success",
	})

	safeName := security.SanitizeFilename(filename)
	if safeName == "" {
		safeName = "report.csv"
	}

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, safeName))
	w.Header().Set("Content-Length", strconv.Itoa(len(csvBytes)))
	w.Header().Set("Cache-Control", "private, no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, bytes.NewReader(csvBytes))
}

func (h *ReportHandler) GetMethodology(w http.ResponseWriter, r *http.Request) {
	methodology := map[string]interface{}{
		"system":             "Ratels Worldwide Transparency, Public Reports & Accountability Platform",
		"governance_version": "Phase D Baseline 1.0 (Constitution 1999 & NBS Reference Geography)",
		"currency_standard":  "Nigerian Naira (NGN), calculated internally in integer kobo to eliminate floating-point rounding errors",
		"definitions": map[string]string{
			"completed_cleanup":      "A scheduled community environmental cleanup activity on the last Saturday of a month whose venue was inspected, published, executed, and signed off by the responsible State/LGA Coordinator.",
			"verified_participation": "An attendance claim submitted with geographic and visual evidence that has been independently verified and approved by an authorized coordinator (anti-self-approval enforced).",
			"unique_participant":     "A deduplicated count of individual registered members who verified attendance across one or more cleanup events during the reporting window.",
			"approved_project":       "A community development project formally approved under the Community Development & Sanitation charter with allocated funding and verified physical location.",
			"completed_project":      "A project whose physical milestones have reached 100% independent inspection sign-off, regardless of financial disbursement schedule.",
			"approved_income":        "Funds received into official bank accounts that have been verified by the Financial Administration and reconciled against bank records.",
			"approved_expenditure":   "Disbursements authorized by Super Admin / Financial Governance, supported by payment vouchers and reconciled against bank statements.",
			"foundation_segregation": "Charitable contributions to the Martinsvincent Ortse Foundation are held in dedicated restricted accounts and are strictly separated from Ratels Worldwide operational funds.",
			"reporting_period":       "The defined calendar boundary (Annual, Quarterly, or Monthly) for which aggregate data was extracted and frozen.",
			"data_revision":          "When an approved publication requires factual amendment, the previous version is marked superseded/withdrawn, and a new numbered revision (e.g. v2) is created with explicit public correction notices.",
		},
		"privacy_protections": []string{
			"Individual participant names, emails, and phone numbers are never disclosed in public reports or exports.",
			"Minimum aggregation thresholds (< 5) suppress small-cell geographic counts to prevent deanonymization.",
			"Confidential safeguarding cases, whistleblower identities, and internal notes are architecturally isolated from the reporting system.",
			"Internal bank account numbers, payee bank details, and payment vendor credentials are fully redacted.",
		},
		"verification_cadence": "Snapshots are frozen upon Super Admin approval and published with an immutable cryptographic SHA-256 data hash.",
	}

	JSON(w, http.StatusOK, methodology)
}

// -----------------------------------------------------------------------
// Administrative Endpoints (RBAC and Scoped Authority)
// -----------------------------------------------------------------------

func (h *ReportHandler) ListSnapshotsAdmin(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	status := r.URL.Query().Get("status")
	reportType := r.URL.Query().Get("report_type")
	scopeType := r.URL.Query().Get("scope_type")
	stateID := r.URL.Query().Get("state_id")

	var stPtr, rtPtr, scPtr, sidPtr *string
	if status != "" {
		stPtr = &status
	}
	if reportType != "" {
		rtPtr = &reportType
	}
	if scopeType != "" {
		scPtr = &scopeType
	}
	if stateID != "" {
		sidPtr = &stateID
	}

	page := int32(1)
	pageSize := int32(20)
	if pStr := r.URL.Query().Get("page"); pStr != "" {
		if pVal, err := strconv.Atoi(pStr); err == nil && pVal > 0 {
			page = int32(pVal)
		}
	}
	if psStr := r.URL.Query().Get("page_size"); psStr != "" {
		if psVal, err := strconv.Atoi(psStr); err == nil && psVal > 0 && psVal <= 100 {
			pageSize = int32(psVal)
		}
	}

	res, err := h.reportService.ListSnapshotsAdmin(ctx, stPtr, rtPtr, scPtr, sidPtr, page, pageSize)
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list report snapshots", err.Error())
		return
	}

	JSON(w, http.StatusOK, res)
}

func (h *ReportHandler) GenerateSnapshotAdmin(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := GetCurrentUser(ctx)
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	var req service.GenerateReportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Failed to parse generation request payload", err.Error())
		return
	}

	if req.ReportType == "" || req.Title == "" || req.ReportingPeriod == "" {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "report_type, title, and reporting_period are required", nil)
		return
	}

	ip := r.RemoteAddr

	res, err := h.reportService.GenerateSnapshot(ctx, user, req, &ip)
	if err != nil {
		if errors.Is(err, service.ErrInvalidReportScope) ||
			errors.Is(err, service.ErrInvalidGeographicFilter) ||
			errors.Is(err, service.ErrUnsupportedScopeForType) ||
			errors.Is(err, service.ErrGeographicHierarchyMismatch) {
			ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed generating report snapshot", err.Error())
		return
	}

	JSON(w, http.StatusCreated, res)
}

func (h *ReportHandler) GetSnapshotDetailAdmin(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chi.URLParam(r, "id")
	if id == "" {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Snapshot ID is required", nil)
		return
	}

	res, err := h.reportService.GetSnapshotDetailAdmin(ctx, id)
	if err != nil {
		if err == service.ErrReportSnapshotNotFound {
			ErrorJSON(w, http.StatusNotFound, "NOT_FOUND", "Snapshot not found", nil)
			return
		}
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed retrieving snapshot detail", err.Error())
		return
	}

	JSON(w, http.StatusOK, res)
}

func (h *ReportHandler) SubmitReviewAdmin(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := GetCurrentUser(ctx)
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Snapshot ID is required", nil)
		return
	}

	ip := r.RemoteAddr
	res, err := h.reportService.SubmitForReview(ctx, user, id, &ip)
	if err != nil {
		if err == service.ErrReportSnapshotNotFound {
			ErrorJSON(w, http.StatusNotFound, "NOT_FOUND", "Snapshot not found", nil)
			return
		}
		if err == service.ErrInvalidReportState {
			ErrorJSON(w, http.StatusConflict, "INVALID_STATE", "Report must be in draft status to submit for review", nil)
			return
		}
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed submitting report for review", err.Error())
		return
	}

	JSON(w, http.StatusOK, res)
}

func (h *ReportHandler) ApproveSnapshotAdmin(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := GetCurrentUser(ctx)
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Snapshot ID is required", nil)
		return
	}

	var req service.ReviewReportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req.DecisionReason = "Approved by authorized governance reviewer"
	}
	if req.DecisionReason == "" {
		req.DecisionReason = "Approved by authorized governance reviewer"
	}

	ip := r.RemoteAddr
	res, err := h.reportService.ApproveSnapshot(ctx, user, id, req.DecisionReason, &ip)
	if err != nil {
		if err == service.ErrReportSnapshotNotFound {
			ErrorJSON(w, http.StatusNotFound, "NOT_FOUND", "Snapshot not found", nil)
			return
		}
		if err == service.ErrSelfApprovalProhibited {
			ErrorJSON(w, http.StatusForbidden, "GOVERNANCE_VIOLATION", err.Error(), nil)
			return
		}
		if err == service.ErrFinancialGovernanceRole {
			ErrorJSON(w, http.StatusForbidden, "INSUFFICIENT_ROLE", err.Error(), nil)
			return
		}
		if err == service.ErrInvalidReportState {
			ErrorJSON(w, http.StatusConflict, "INVALID_STATE", "Report must be in pending_review status to approve", nil)
			return
		}
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed approving report", err.Error())
		return
	}

	JSON(w, http.StatusOK, res)
}

func (h *ReportHandler) RejectSnapshotAdmin(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := GetCurrentUser(ctx)
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Snapshot ID is required", nil)
		return
	}

	var req service.RejectReportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Reason == "" {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Rejection reason is required", nil)
		return
	}

	ip := r.RemoteAddr
	res, err := h.reportService.RejectSnapshot(ctx, user, id, req.Reason, &ip)
	if err != nil {
		if err == service.ErrReportSnapshotNotFound {
			ErrorJSON(w, http.StatusNotFound, "NOT_FOUND", "Snapshot not found", nil)
			return
		}
		if err == service.ErrInvalidReportState {
			ErrorJSON(w, http.StatusConflict, "INVALID_STATE", "Report must be in pending_review status to reject", nil)
			return
		}
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed rejecting report", err.Error())
		return
	}

	JSON(w, http.StatusOK, res)
}

func (h *ReportHandler) PublishSnapshotAdmin(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := GetCurrentUser(ctx)
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Snapshot ID is required", nil)
		return
	}

	ip := r.RemoteAddr
	res, err := h.reportService.PublishSnapshot(ctx, user, id, &ip)
	if err != nil {
		if err == service.ErrFinancialGovernanceRole {
			ErrorJSON(w, http.StatusForbidden, "INSUFFICIENT_ROLE", err.Error(), nil)
			return
		}

		if err == service.ErrReportSnapshotNotFound {
			ErrorJSON(w, http.StatusNotFound, "NOT_FOUND", "Snapshot not found", nil)
			return
		}
		if err == service.ErrReportNotApproved {
			ErrorJSON(w, http.StatusConflict, "REPORT_NOT_APPROVED", "Report snapshot must be approved before publication", nil)
			return
		}
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed publishing report", err.Error())
		return
	}

	JSON(w, http.StatusOK, res)
}

func (h *ReportHandler) WithdrawSnapshotAdmin(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := GetCurrentUser(ctx)
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Snapshot ID is required", nil)
		return
	}

	var req service.WithdrawReportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Reason == "" {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Withdrawal reason is required", nil)
		return
	}

	ip := r.RemoteAddr
	res, err := h.reportService.WithdrawPublication(ctx, user, id, req.Reason, &ip)
	if err != nil {
		if err == service.ErrFinancialGovernanceRole {
			ErrorJSON(w, http.StatusForbidden, "INSUFFICIENT_ROLE", err.Error(), nil)
			return
		}

		if err == service.ErrReportSnapshotNotFound {
			ErrorJSON(w, http.StatusNotFound, "NOT_FOUND", "Snapshot not found", nil)
			return
		}
		if err == service.ErrInvalidReportState {
			ErrorJSON(w, http.StatusConflict, "INVALID_STATE", "Report must be in published status to withdraw", nil)
			return
		}
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed withdrawing report", err.Error())
		return
	}

	JSON(w, http.StatusOK, res)
}

func (h *ReportHandler) ReviseSnapshotAdmin(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := GetCurrentUser(ctx)
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Snapshot ID is required", nil)
		return
	}

	var req service.ReviseReportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.CorrectionNotes == "" {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Correction notes are required to revise a report", nil)
		return
	}

	ip := r.RemoteAddr
	res, err := h.reportService.ReviseSnapshot(ctx, user, id, req.CorrectionNotes, &ip)
	if err != nil {
		if err == service.ErrReportSnapshotNotFound {
			ErrorJSON(w, http.StatusNotFound, "NOT_FOUND", "Snapshot not found", nil)
			return
		}
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed creating revised snapshot", err.Error())
		return
	}

	JSON(w, http.StatusCreated, res)
}

func (h *ReportHandler) ListAuditsAdmin(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var snapPtr, actPtr *string
	if snapshotID := r.URL.Query().Get("snapshot_id"); snapshotID != "" {
		snapPtr = &snapshotID
	}
	if action := r.URL.Query().Get("action"); action != "" {
		actPtr = &action
	}

	var fromDate, toDate *time.Time
	if fd := r.URL.Query().Get("from_date"); fd != "" {
		if t, err := time.Parse(time.RFC3339, fd); err == nil {
			fromDate = &t
		} else if t, err := time.Parse("2006-01-02", fd); err == nil {
			fromDate = &t
		}
	}
	if td := r.URL.Query().Get("to_date"); td != "" {
		if t, err := time.Parse(time.RFC3339, td); err == nil {
			toDate = &t
		} else if t, err := time.Parse("2006-01-02", td); err == nil {
			t = t.Add(24*time.Hour - time.Nanosecond)
			toDate = &t
		}
	}

	page := int32(1)
	pageSize := int32(50)
	if pStr := r.URL.Query().Get("page"); pStr != "" {
		if pVal, err := strconv.Atoi(pStr); err == nil && pVal > 0 {
			page = int32(pVal)
		}
	}
	if psStr := r.URL.Query().Get("page_size"); psStr != "" {
		if psVal, err := strconv.Atoi(psStr); err == nil && psVal > 0 {
			pageSize = int32(psVal)
		}
	}

	res, err := h.reportService.ListReportAuditsEnhanced(ctx, snapPtr, actPtr, fromDate, toDate, page, pageSize)
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed listing report audit history", err.Error())
		return
	}

	JSON(w, http.StatusOK, res)
}

func (h *ReportHandler) GetDashboardStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	stats, err := h.reportService.GetAdminDashboardStats(ctx)
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed retrieving admin dashboard statistics", err.Error())
		return
	}
	JSON(w, http.StatusOK, stats)
}

func (h *ReportHandler) ListSchedulerRuns(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var stPtr, repPtr, trgPtr *string
	if st := r.URL.Query().Get("status"); st != "" {
		stPtr = &st
	}
	if rep := r.URL.Query().Get("reporting_period"); rep != "" {
		repPtr = &rep
	}
	if trg := r.URL.Query().Get("trigger_source"); trg != "" {
		trgPtr = &trg
	}

	var fromDate, toDate *time.Time
	if fd := r.URL.Query().Get("from_date"); fd != "" {
		if t, err := time.Parse(time.RFC3339, fd); err == nil {
			fromDate = &t
		} else if t, err := time.Parse("2006-01-02", fd); err == nil {
			fromDate = &t
		}
	}
	if td := r.URL.Query().Get("to_date"); td != "" {
		if t, err := time.Parse(time.RFC3339, td); err == nil {
			toDate = &t
		} else if t, err := time.Parse("2006-01-02", td); err == nil {
			t = t.Add(24*time.Hour - time.Nanosecond)
			toDate = &t
		}
	}

	page := int32(1)
	pageSize := int32(20)
	if pStr := r.URL.Query().Get("page"); pStr != "" {
		if pVal, err := strconv.Atoi(pStr); err == nil && pVal > 0 {
			page = int32(pVal)
		}
	}
	if psStr := r.URL.Query().Get("page_size"); psStr != "" {
		if psVal, err := strconv.Atoi(psStr); err == nil && psVal > 0 {
			pageSize = int32(psVal)
		}
	}

	res, err := h.reportService.ListSchedulerRuns(ctx, stPtr, repPtr, trgPtr, fromDate, toDate, page, pageSize)
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed listing scheduler runs", err.Error())
		return
	}
	JSON(w, http.StatusOK, res)
}

func (h *ReportHandler) GetSchedulerRun(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chi.URLParam(r, "id")
	if id == "" {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_INPUT", "run ID is required", nil)
		return
	}

	run, err := h.reportService.GetSchedulerRunDetail(ctx, id)
	if err != nil {
		ErrorJSON(w, http.StatusNotFound, "NOT_FOUND", "Scheduler run not found", err.Error())
		return
	}
	JSON(w, http.StatusOK, run)
}

func (h *ReportHandler) ListExportsAdmin(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var fmtPtr, stPtr, snapPtr *string
	if f := r.URL.Query().Get("format"); f != "" {
		fmtPtr = &f
	}
	if st := r.URL.Query().Get("status"); st != "" {
		stPtr = &st
	}
	if snap := r.URL.Query().Get("snapshot_id"); snap != "" {
		snapPtr = &snap
	}

	page := int32(1)
	pageSize := int32(20)
	if pStr := r.URL.Query().Get("page"); pStr != "" {
		if pVal, err := strconv.Atoi(pStr); err == nil && pVal > 0 {
			page = int32(pVal)
		}
	}
	if psStr := r.URL.Query().Get("page_size"); psStr != "" {
		if psVal, err := strconv.Atoi(psStr); err == nil && psVal > 0 {
			pageSize = int32(psVal)
		}
	}

	res, err := h.reportService.ListReportExportsAdmin(ctx, fmtPtr, stPtr, snapPtr, page, pageSize)
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed listing report exports", err.Error())
		return
	}
	JSON(w, http.StatusOK, res)
}

func (h *ReportHandler) RegenerateExportAdmin(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := GetCurrentUser(ctx)
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}
	if user.PrimaryRole != "super_admin" && user.PrimaryRole != "national_admin" {
		ErrorJSON(w, http.StatusForbidden, "FORBIDDEN", "Only national administrators and super admins may regenerate exports", nil)
		return
	}

	snapshotID := chi.URLParam(r, "id")
	format := chi.URLParam(r, "format")
	if snapshotID == "" || format == "" {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_INPUT", "snapshot ID and format are required", nil)
		return
	}

	ip := security.GetClientIP(r)
	dto, err := h.reportService.RegenerateReportExport(ctx, user, snapshotID, format, &ip)
	if err != nil {
		if errors.Is(err, service.ErrReportSnapshotNotFound) {
			ErrorJSON(w, http.StatusNotFound, "NOT_FOUND", "Report snapshot not found", nil)
			return
		}
		if errors.Is(err, service.ErrReportWithdrawn) || errors.Is(err, service.ErrReportRejected) {
			ErrorJSON(w, http.StatusBadRequest, "INVALID_STATE", "Cannot regenerate export for withdrawn or rejected report", nil)
			return
		}
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed regenerating report export", err.Error())
		return
	}

	JSON(w, http.StatusOK, dto)
}

func (h *ReportHandler) DownloadExportAdmin(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := GetCurrentUser(ctx)
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	snapshotID := chi.URLParam(r, "id")
	format := chi.URLParam(r, "format")
	if snapshotID == "" || format == "" {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_INPUT", "snapshot ID and format are required", nil)
		return
	}

	exportBytes, fileName, mimeType, err := h.reportService.GetExportPrivateStream(ctx, user, snapshotID, format)
	if err != nil {
		if errors.Is(err, service.ErrReportSnapshotNotFound) {
			ErrorJSON(w, http.StatusNotFound, "NOT_FOUND", "Report snapshot not found", nil)
			return
		}
		if errors.Is(err, service.ErrReportWithdrawn) || errors.Is(err, service.ErrExportRevoked) {
			ErrorJSON(w, http.StatusGone, "EXPORT_WITHDRAWN", "Export has been withdrawn or revoked", nil)
			return
		}
		if errors.Is(err, service.ErrUnauthorizedExportAccess) {
			ErrorJSON(w, http.StatusForbidden, "FORBIDDEN", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed retrieving export payload", err.Error())
		return
	}

	safeName := security.SanitizeFilename(fileName)
	w.Header().Set("Content-Type", mimeType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, safeName))
	w.Header().Set("Content-Length", strconv.Itoa(len(exportBytes)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(exportBytes)
}

func (h *ReportHandler) GetSettingsAdmin(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defs, err := h.reportService.GetSettingsDefinitions(ctx)
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed retrieving report definitions", err.Error())
		return
	}

	JSON(w, http.StatusOK, defs)
}

// -----------------------------------------------------------------------
// Scheduled Generation Triggers (Admin & Render Free Cron Trigger)
// -----------------------------------------------------------------------

type RunSchedulerRequest struct {
	TargetPeriod string  `json:"target_period"` // Optional YYYY-MM or YYYY-Q[1-4]
	PeriodType   string  `json:"period_type"`   // Optional "monthly", "quarterly", or "both"
	ReportType   string  `json:"report_type"`   // Optional single category filter
	ScopeType    string  `json:"scope_type"`    // Optional "national", "zone", "state", "lga"
	ZoneID       *string `json:"zone_id"`
	StateID      *string `json:"state_id"`
	LgaID        *string `json:"lga_id"`
}

// RunSchedulerAdmin allows Super Admins and National Admins to trigger a scheduled report cycle.
func (h *ReportHandler) RunSchedulerAdmin(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := GetCurrentUser(ctx)
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	if h.scheduler == nil {
		ErrorJSON(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Report scheduler engine is not configured", nil)
		return
	}

	var req RunSchedulerRequest
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	var targetPeriodPtr *string
	if strings.TrimSpace(req.TargetPeriod) != "" {
		cleaned := strings.TrimSpace(req.TargetPeriod)
		_, pEnd, _, _, err := worker.ParsePeriodUTC(cleaned)
		if err != nil {
			ErrorJSON(w, http.StatusBadRequest, "INVALID_INPUT", "target_period must be in YYYY-MM or YYYY-Q[1-4] format", err.Error())
			return
		}
		if pEnd.After(time.Now().UTC()) {
			ErrorJSON(w, http.StatusBadRequest, "FUTURE_PERIOD_REJECTED", "Reporting period is incomplete or in the future; reports can only be generated for concluded historical periods", nil)
			return
		}
		targetPeriodPtr = &cleaned
	}

	params := worker.RunSchedulerParams{
		TargetPeriod:     targetPeriodPtr,
		PeriodType:       req.PeriodType,
		TriggerSource:    "admin_manual",
		ActorUser:        user,
		StrictHistorical: true,
		ReportType:       req.ReportType,
		ScopeType:        req.ScopeType,
		ZoneID:           req.ZoneID,
		StateID:          req.StateID,
		LgaID:            req.LgaID,
	}

	res, err := h.scheduler.RunScheduledCycle(ctx, params)
	if err != nil {
		if errors.Is(err, worker.ErrSchedulerAlreadyRunning) {
			ErrorJSON(w, http.StatusConflict, "LOCK_CONFLICT", "Scheduler cycle already running on another node", nil)
			return
		}
		if errors.Is(err, worker.ErrFutureReportingPeriod) {
			ErrorJSON(w, http.StatusBadRequest, "FUTURE_PERIOD_REJECTED", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed executing scheduled reports cycle", err.Error())
		return
	}

	JSON(w, http.StatusOK, res)
}

// RunSchedulerCron allows external cron callers (Render Cron, GitHub Actions, monitoring) to trigger scheduled generation.
func (h *ReportHandler) RunSchedulerCron(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Authenticate cron trigger via X-Cron-Secret, X-Report-Cron-Secret, or Bearer token
	cronSecret := r.Header.Get("X-Cron-Secret")
	if cronSecret == "" {
		cronSecret = r.Header.Get("X-Report-Cron-Secret")
	}
	if cronSecret == "" {
		authHeader := r.Header.Get("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			cronSecret = strings.TrimPrefix(authHeader, "Bearer ")
		}
	}

	if h.cronSecret == "" || subtle.ConstantTimeCompare([]byte(cronSecret), []byte(h.cronSecret)) != 1 {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid or missing cron secret", nil)
		return
	}

	if h.scheduler == nil {
		ErrorJSON(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Report scheduler engine is not configured", nil)
		return
	}

	var req RunSchedulerRequest
	targetParam := r.URL.Query().Get("target_period")
	pTypeParam := r.URL.Query().Get("period_type")
	repTypeParam := r.URL.Query().Get("report_type")

	if targetParam != "" {
		req.TargetPeriod = targetParam
	}
	if pTypeParam != "" {
		req.PeriodType = pTypeParam
	}
	if repTypeParam != "" {
		req.ReportType = repTypeParam
	}

	if r.Body != nil && r.ContentLength > 0 {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	var targetPeriodPtr *string
	if strings.TrimSpace(req.TargetPeriod) != "" {
		cleaned := strings.TrimSpace(req.TargetPeriod)
		_, pEnd, _, _, err := worker.ParsePeriodUTC(cleaned)
		if err != nil {
			ErrorJSON(w, http.StatusBadRequest, "INVALID_INPUT", "target_period must be in YYYY-MM or YYYY-Q[1-4] format", err.Error())
			return
		}
		if pEnd.After(time.Now().UTC()) {
			ErrorJSON(w, http.StatusBadRequest, "FUTURE_PERIOD_REJECTED", "Reporting period is incomplete or in the future; reports can only be generated for concluded historical periods", nil)
			return
		}
		targetPeriodPtr = &cleaned
	}

	params := worker.RunSchedulerParams{
		TargetPeriod:     targetPeriodPtr,
		PeriodType:       req.PeriodType,
		TriggerSource:    "external_cron",
		ActorUser:        nil,
		StrictHistorical: true,
		ReportType:       req.ReportType,
	}

	res, err := h.scheduler.RunScheduledCycle(ctx, params)
	if err != nil {
		if errors.Is(err, worker.ErrSchedulerAlreadyRunning) {
			ErrorJSON(w, http.StatusConflict, "LOCK_CONFLICT", "Scheduler cycle already running on another node", nil)
			return
		}
		if errors.Is(err, worker.ErrFutureReportingPeriod) {
			ErrorJSON(w, http.StatusBadRequest, "FUTURE_PERIOD_REJECTED", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed executing scheduled reports cycle", err.Error())
		return
	}

	JSON(w, http.StatusOK, res)
}
