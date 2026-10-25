package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"ratels-api/internal/service"
)

type ReportHandler struct {
	reportService *service.ReportService
}

func NewReportHandler(reportService *service.ReportService) *ReportHandler {
	return &ReportHandler{
		reportService: reportService,
	}
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
	id := chi.URLParam(r, "id")
	if id == "" {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Report ID or reference is required", nil)
		return
	}

	pdfBytes, filename, err := h.reportService.ExportReportPDF(ctx, id)
	if err != nil {
		if err == service.ErrReportPublicationNotFound {
			ErrorJSON(w, http.StatusNotFound, "NOT_FOUND", "Published report not found", nil)
			return
		}
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed generating report PDF", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	w.Header().Set("Content-Length", strconv.Itoa(len(pdfBytes)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(pdfBytes)
}

func (h *ReportHandler) DownloadCSV(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chi.URLParam(r, "id")
	if id == "" {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Report ID or reference is required", nil)
		return
	}

	csvBytes, filename, err := h.reportService.ExportReportCSV(ctx, id)
	if err != nil {
		if err == service.ErrReportPublicationNotFound {
			ErrorJSON(w, http.StatusNotFound, "NOT_FOUND", "Published report not found", nil)
			return
		}
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed generating report CSV", err.Error())
		return
	}

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	w.Header().Set("Content-Length", strconv.Itoa(len(csvBytes)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(csvBytes)
}

func (h *ReportHandler) GetMethodology(w http.ResponseWriter, r *http.Request) {
	methodology := map[string]interface{}{
		"system":             "Nigerian Ratels Transparency, Public Reports & Accountability Platform",
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
			"foundation_segregation": "Charitable contributions to the Martinsvincent Ortse Foundation are held in dedicated restricted accounts and are strictly separated from Nigerian Ratels operational funds.",
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
	snapshotID := r.URL.Query().Get("snapshot_id")

	var snapPtr *string
	if snapshotID != "" {
		snapPtr = &snapshotID
	}

	page := int32(1)
	pageSize := int32(50)
	if pStr := r.URL.Query().Get("page"); pStr != "" {
		if pVal, err := strconv.Atoi(pStr); err == nil && pVal > 0 {
			page = int32(pVal)
		}
	}

	res, err := h.reportService.ListReportAudits(ctx, snapPtr, page, pageSize)
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed listing report audit history", err.Error())
		return
	}

	JSON(w, http.StatusOK, res)
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
