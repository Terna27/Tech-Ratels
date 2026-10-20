package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"ratels-api/internal/config"
	"ratels-api/internal/db"
	"ratels-api/internal/security"
	"ratels-api/internal/service"
	"ratels-api/internal/storage"
)

type SafeguardingHandler struct {
	svc     *service.SafeguardingService
	storage storage.SafeguardingStorageService
	cfg     *config.Config
}

func NewSafeguardingHandler(svc *service.SafeguardingService, storage storage.SafeguardingStorageService, cfg *config.Config) *SafeguardingHandler {
	return &SafeguardingHandler{
		svc:     svc,
		storage: storage,
		cfg:     cfg,
	}
}

// -------------------------------------------------------------
// Public Endpoints
// -------------------------------------------------------------

// GetPublicConfig returns public safeguarding operational state and emergency guidance.
func (h *SafeguardingHandler) GetPublicConfig(w http.ResponseWriter, r *http.Request) {
	enabled := true
	if h.cfg != nil {
		enabled = h.cfg.SafeguardingReportingEnabled
	}

	guidance := "This form is not monitored continuously and is not an emergency response service. If someone is in immediate danger, contact local emergency services or a trusted safeguarding authority immediately."

	JSON(w, http.StatusOK, map[string]interface{}{
		"is_enabled":               enabled,
		"reporting_enabled":        enabled,
		"emergency_guidance_text": guidance,
		"emergency_banner":         guidance,
	})
}

// SubmitReport handles confidential and anonymous incident report submissions.
func (h *SafeguardingHandler) SubmitReport(w http.ResponseWriter, r *http.Request) {
	if h.cfg != nil && !h.cfg.SafeguardingReportingEnabled {
		ErrorJSON(w, http.StatusServiceUnavailable, "SAFEGUARDING_UNAVAILABLE",
			"Safeguarding reporting is currently undergoing administrative readiness review and is unavailable.", nil)
		return
	}

	var input service.SubmitReportInput

	contentType := r.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "multipart/form-data") {
		// Parse multipart with 25MB max
		if err := r.ParseMultipartForm(storage.MaxEvidenceFileSize); err != nil {
			ErrorJSON(w, http.StatusBadRequest, "PAYLOAD_TOO_LARGE", "Request payload exceeds size limit", nil)
			return
		}

		input.Category = r.FormValue("category")
		input.Description = r.FormValue("description")
		if input.Description == "" {
			input.Description = r.FormValue("narrative")
		}
		input.IsImmediateDanger = r.FormValue("is_immediate_danger") == "true" || r.FormValue("immediate_danger") == "true"
		input.IsMinorInvolved = r.FormValue("is_minor_involved") == "true" || r.FormValue("minor_involved") == "true"
		input.IsAnonymous = r.FormValue("is_anonymous") == "true" || r.FormValue("is_anonymous") == "1"

		if dt := r.FormValue("incident_date"); dt != "" {
			if parsed, err := time.Parse(time.RFC3339, dt); err == nil {
				input.IncidentDate = &parsed
			} else if parsedDate, err := time.Parse("2006-01-02", dt); err == nil {
				input.IncidentDate = &parsedDate
			}
		}

		if st := r.FormValue("state_id"); st != "" {
			input.StateID = &st
		}
		if lg := r.FormValue("lga_id"); lg != "" {
			input.LGAID = &lg
		}
		if act := r.FormValue("related_activity"); act != "" {
			input.RelatedActivity = &act
		} else if loc := r.FormValue("location_description"); loc != "" {
			input.RelatedActivity = &loc
		}
		if pi := r.FormValue("person_involved"); pi != "" {
			input.PersonInvolved = &pi
		}

		if !input.IsAnonymous {
			if nm := r.FormValue("reporter_name"); nm != "" {
				input.ReporterName = &nm
			}
			if em := r.FormValue("reporter_email"); em != "" {
				input.ReporterEmail = &em
			}
			if ph := r.FormValue("reporter_phone"); ph != "" {
				input.ReporterPhone = &ph
			}
			input.ContactPreference = r.FormValue("contact_preference")
		}

		// Check optional evidence file
		file, header, err := r.FormFile("evidence")
		if err == nil && file != nil {
			defer file.Close()
			input.EvidenceFileName = header.Filename
			input.EvidenceContentType = header.Header.Get("Content-Type")
			input.EvidenceReader = file
			input.EvidenceSize = header.Size
		}
	} else {
		// JSON payload
		var req struct {
			Category          string  `json:"category"`
			Description       string  `json:"description"`
			Narrative         string  `json:"narrative"`
			IncidentDate      *string `json:"incident_date,omitempty"`
			StateID           *string `json:"state_id,omitempty"`
			LGAID             *string `json:"lga_id,omitempty"`
			RelatedActivity   *string `json:"related_activity,omitempty"`
			LocationDesc      *string `json:"location_description,omitempty"`
			PersonInvolved    *string `json:"person_involved,omitempty"`
			IsImmediateDanger bool    `json:"is_immediate_danger"`
			ImmediateDanger   bool    `json:"immediate_danger"`
			IsMinorInvolved   bool    `json:"is_minor_involved"`
			MinorInvolved     bool    `json:"minor_involved"`
			IsAnonymous       bool    `json:"is_anonymous"`
			ReporterName      *string `json:"reporter_name,omitempty"`
			ReporterEmail     *string `json:"reporter_email,omitempty"`
			ReporterPhone     *string `json:"reporter_phone,omitempty"`
			ContactPreference string  `json:"contact_preference"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			ErrorJSON(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON request body", nil)
			return
		}

		input.Category = req.Category
		input.Description = req.Description
		if input.Description == "" {
			input.Description = req.Narrative
		}
		input.IsImmediateDanger = req.IsImmediateDanger || req.ImmediateDanger
		input.IsMinorInvolved = req.IsMinorInvolved || req.MinorInvolved
		input.IsAnonymous = req.IsAnonymous
		input.StateID = req.StateID
		input.LGAID = req.LGAID
		input.RelatedActivity = req.RelatedActivity
		if input.RelatedActivity == nil {
			input.RelatedActivity = req.LocationDesc
		}
		input.PersonInvolved = req.PersonInvolved
		input.ReporterName = req.ReporterName
		input.ReporterEmail = req.ReporterEmail
		input.ReporterPhone = req.ReporterPhone
		input.ContactPreference = req.ContactPreference

		if req.IncidentDate != nil && *req.IncidentDate != "" {
			if parsed, err := time.Parse(time.RFC3339, *req.IncidentDate); err == nil {
				input.IncidentDate = &parsed
			} else if parsedDate, err := time.Parse("2006-01-02", *req.IncidentDate); err == nil {
				input.IncidentDate = &parsedDate
			}
		}
	}

	// If authenticated user and not anonymous, record user ID
	if user := GetCurrentUser(r.Context()); user != nil && !input.IsAnonymous {
		input.ReporterUserID = &user.ID
		if input.ReporterName == nil || *input.ReporterName == "" {
			input.ReporterName = &user.DisplayName
		}
	}

	result, err := h.svc.SubmitReport(r.Context(), input)
	if err != nil {
		if errors.Is(err, service.ErrSafeguardingDisabled) {
			ErrorJSON(w, http.StatusServiceUnavailable, "SAFEGUARDING_UNAVAILABLE", err.Error(), nil)
			return
		}
		if errors.Is(err, service.ErrInvalidIncidentCategory) || errors.Is(err, service.ErrDescriptionRequired) {
			ErrorJSON(w, http.StatusBadRequest, "VALIDATION_FAILED", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to submit safeguarding report", nil)
		return
	}

	JSON(w, http.StatusCreated, result)
}

// GetReportStatus retrieves safe status for a reporter using their secret.
func (h *SafeguardingHandler) GetReportStatus(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PublicReference string `json:"public_reference"`
		ReferenceNumber string `json:"reference_number"`
		FollowUpSecret  string `json:"follow_up_secret"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON body", nil)
		return
	}

	ref := req.PublicReference
	if ref == "" {
		ref = req.ReferenceNumber
	}

	if ref == "" || req.FollowUpSecret == "" {
		ErrorJSON(w, http.StatusBadRequest, "MISSING_CREDENTIALS", "Public reference and follow-up secret are required", nil)
		return
	}

	statusView, err := h.svc.GetReportStatus(r.Context(), ref, req.FollowUpSecret)
	if err != nil {
		if errors.Is(err, service.ErrCaseNotFound) || errors.Is(err, service.ErrInvalidFollowUpCredentials) {
			// Uniform error to prevent enumeration
			ErrorJSON(w, http.StatusNotFound, "NOT_FOUND", "No matching report found for provided reference and secret", nil)
			return
		}
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve status", nil)
		return
	}

	JSON(w, http.StatusOK, statusView)
}

// PostReporterMessage adds a follow-up message from an anonymous or identified reporter.
func (h *SafeguardingHandler) PostReporterMessage(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PublicReference string `json:"public_reference"`
		ReferenceNumber string `json:"reference_number"`
		FollowUpSecret  string `json:"follow_up_secret"`
		Content         string `json:"content"`
		Message         string `json:"message"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON body", nil)
		return
	}

	ref := req.PublicReference
	if ref == "" {
		ref = req.ReferenceNumber
	}

	msg := req.Content
	if msg == "" {
		msg = req.Message
	}

	if ref == "" || req.FollowUpSecret == "" || strings.TrimSpace(msg) == "" {
		ErrorJSON(w, http.StatusBadRequest, "VALIDATION_FAILED", "Reference, secret, and message content are required", nil)
		return
	}

	err := h.svc.PostReporterMessage(r.Context(), ref, req.FollowUpSecret, msg)
	if err != nil {
		if errors.Is(err, service.ErrCaseNotFound) || errors.Is(err, service.ErrInvalidFollowUpCredentials) {
			ErrorJSON(w, http.StatusNotFound, "NOT_FOUND", "No matching report found", nil)
			return
		}
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to send message", nil)
		return
	}

	JSON(w, http.StatusOK, map[string]interface{}{
		"status": "success",
		"message": map[string]interface{}{
			"id":          security.GenerateUUID(),
			"author_type": "reporter",
			"sender_role": "reporter",
			"content":     msg,
			"message":     msg,
			"created_at":  time.Now().UTC().Format(time.RFC3339),
		},
	})
}

// -------------------------------------------------------------
// Restricted Administration Endpoints
// -------------------------------------------------------------

func (h *SafeguardingHandler) getAuthedOfficer(r *http.Request) (*db.User, error) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		return nil, service.ErrUnauthorizedSafeguardingAccess
	}
	if !h.svc.CanAccessSafeguarding(r.Context(), *user) {
		return nil, service.ErrUnauthorizedSafeguardingAccess
	}
	return user, nil
}

// ListCases returns paginated safeguarding cases.
func (h *SafeguardingHandler) ListCases(w http.ResponseWriter, r *http.Request) {
	user, err := h.getAuthedOfficer(r)
	if err != nil {
		ErrorJSON(w, http.StatusForbidden, "FORBIDDEN", err.Error(), nil)
		return
	}

	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	limit, _ := strconv.Atoi(q.Get("limit"))

	filter := service.CaseFilter{
		Status:    strings.TrimSpace(q.Get("status")),
		RiskLevel: strings.TrimSpace(q.Get("risk_level")),
		Category:  strings.TrimSpace(q.Get("category")),
		Page:      int32(page),
		Limit:     int32(limit),
	}

	if q.Get("is_minor_involved") != "" {
		val := q.Get("is_minor_involved") == "true"
		filter.IsMinorInvolved = &val
	}
	if q.Get("is_immediate_danger") != "" {
		val := q.Get("is_immediate_danger") == "true"
		filter.IsImmediateDanger = &val
	}

	result, err := h.svc.ListCases(r.Context(), *user, filter)
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, result)
}

// GetCaseDetails returns complete decrypted case details and logs access.
func (h *SafeguardingHandler) GetCaseDetails(w http.ResponseWriter, r *http.Request) {
	user, err := h.getAuthedOfficer(r)
	if err != nil {
		ErrorJSON(w, http.StatusForbidden, "FORBIDDEN", err.Error(), nil)
		return
	}

	caseID := chi.URLParam(r, "id")
	justification := r.URL.Query().Get("justification")
	clientIP := r.RemoteAddr

	detail, err := h.svc.GetCaseDetails(r.Context(), *user, caseID, justification, clientIP)
	if err != nil {
		if errors.Is(err, service.ErrConflictOfInterest) {
			ErrorJSON(w, http.StatusForbidden, "CONFLICT_OF_INTEREST", err.Error(), nil)
			return
		}
		if errors.Is(err, service.ErrCaseNotFound) {
			ErrorJSON(w, http.StatusNotFound, "NOT_FOUND", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, detail)
}

// UpdateTriage adjusts risk level and case status.
func (h *SafeguardingHandler) UpdateTriage(w http.ResponseWriter, r *http.Request) {
	user, err := h.getAuthedOfficer(r)
	if err != nil {
		ErrorJSON(w, http.StatusForbidden, "FORBIDDEN", err.Error(), nil)
		return
	}

	caseID := chi.URLParam(r, "id")

	var req struct {
		RiskLevel   string `json:"risk_level"`
		Urgency     string `json:"urgency"`
		Status      string `json:"status"`
		Reason      string `json:"reason"`
		TriageNotes string `json:"triage_notes"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON", nil)
		return
	}

	riskLevel := req.RiskLevel
	if riskLevel == "" {
		riskLevel = req.Urgency
	}
	reason := req.Reason
	if reason == "" {
		reason = req.TriageNotes
	}

	err = h.svc.UpdateTriage(r.Context(), *user, caseID, riskLevel, req.Status, reason)
	if err != nil {
		if errors.Is(err, service.ErrConflictOfInterest) {
			ErrorJSON(w, http.StatusForbidden, "CONFLICT_OF_INTEREST", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, map[string]string{"message": "Triage updated successfully"})
}

// AssignOfficer assigns a case to an active safeguarding officer.
func (h *SafeguardingHandler) AssignOfficer(w http.ResponseWriter, r *http.Request) {
	user, err := h.getAuthedOfficer(r)
	if err != nil {
		ErrorJSON(w, http.StatusForbidden, "FORBIDDEN", err.Error(), nil)
		return
	}

	caseID := chi.URLParam(r, "id")

	var req struct {
		OfficerID string `json:"officer_id"`
		Reason    string `json:"reason"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON", nil)
		return
	}

	err = h.svc.AssignOfficer(r.Context(), *user, caseID, req.OfficerID, req.Reason)
	if err != nil {
		if errors.Is(err, service.ErrConflictOfInterest) || errors.Is(err, service.ErrOfficerHasConflict) {
			ErrorJSON(w, http.StatusForbidden, "CONFLICT_OF_INTEREST", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, map[string]string{"message": "Officer assigned successfully"})
}

// DeclareConflict logs an officer conflict on a case.
func (h *SafeguardingHandler) DeclareConflict(w http.ResponseWriter, r *http.Request) {
	user, err := h.getAuthedOfficer(r)
	if err != nil {
		ErrorJSON(w, http.StatusForbidden, "FORBIDDEN", err.Error(), nil)
		return
	}

	caseID := chi.URLParam(r, "id")

	var req struct {
		OfficerID    string `json:"officer_id"`
		ConflictType string `json:"conflict_type"`
		Reason       string `json:"reason"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON", nil)
		return
	}

	targetOfficer := req.OfficerID
	if targetOfficer == "" {
		targetOfficer = user.ID
	}

	err = h.svc.DeclareConflict(r.Context(), *user, caseID, targetOfficer, req.ConflictType, req.Reason)
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, map[string]string{"message": "Conflict of interest declared successfully"})
}

// EscalateCase escalates a case to independent oversight.
func (h *SafeguardingHandler) EscalateCase(w http.ResponseWriter, r *http.Request) {
	user, err := h.getAuthedOfficer(r)
	if err != nil {
		ErrorJSON(w, http.StatusForbidden, "FORBIDDEN", err.Error(), nil)
		return
	}

	caseID := chi.URLParam(r, "id")

	var req struct {
		EscalatedTo string `json:"escalated_to"`
		Reason      string `json:"reason"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON", nil)
		return
	}

	err = h.svc.EscalateCase(r.Context(), *user, caseID, req.EscalatedTo, req.Reason)
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, map[string]string{"message": "Case escalated to independent oversight"})
}

// ResolveCase resolves a case with summary.
func (h *SafeguardingHandler) ResolveCase(w http.ResponseWriter, r *http.Request) {
	user, err := h.getAuthedOfficer(r)
	if err != nil {
		ErrorJSON(w, http.StatusForbidden, "FORBIDDEN", err.Error(), nil)
		return
	}

	caseID := chi.URLParam(r, "id")

	var req struct {
		Summary string `json:"summary"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON", nil)
		return
	}

	err = h.svc.ResolveCase(r.Context(), *user, caseID, req.Summary)
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, map[string]string{"message": "Case marked as resolved"})
}

// CloseCase closes a case.
func (h *SafeguardingHandler) CloseCase(w http.ResponseWriter, r *http.Request) {
	user, err := h.getAuthedOfficer(r)
	if err != nil {
		ErrorJSON(w, http.StatusForbidden, "FORBIDDEN", err.Error(), nil)
		return
	}

	caseID := chi.URLParam(r, "id")

	err = h.svc.CloseCase(r.Context(), *user, caseID)
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, map[string]string{"message": "Case closed successfully"})
}

// AddInternalNote adds an internal note.
func (h *SafeguardingHandler) AddInternalNote(w http.ResponseWriter, r *http.Request) {
	user, err := h.getAuthedOfficer(r)
	if err != nil {
		ErrorJSON(w, http.StatusForbidden, "FORBIDDEN", err.Error(), nil)
		return
	}

	caseID := chi.URLParam(r, "id")

	var req struct {
		Content string `json:"content"`
		Note    string `json:"note"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON", nil)
		return
	}

	content := req.Content
	if content == "" {
		content = req.Note
	}

	err = h.svc.AddInternalNote(r.Context(), *user, caseID, content)
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, map[string]string{"message": "Internal note recorded"})
}

// SendReporterUpdate sends a reporter-visible update.
func (h *SafeguardingHandler) SendReporterUpdate(w http.ResponseWriter, r *http.Request) {
	user, err := h.getAuthedOfficer(r)
	if err != nil {
		ErrorJSON(w, http.StatusForbidden, "FORBIDDEN", err.Error(), nil)
		return
	}

	caseID := chi.URLParam(r, "id")

	var req struct {
		Content string `json:"content"`
		Message string `json:"message"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON", nil)
		return
	}

	content := req.Content
	if content == "" {
		content = req.Message
	}

	err = h.svc.SendReporterUpdate(r.Context(), *user, caseID, content)
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, map[string]string{"message": "Reporter update dispatched"})
}

// GetEvidenceDownloadURL returns a presigned download link for evidence.
func (h *SafeguardingHandler) GetEvidenceDownloadURL(w http.ResponseWriter, r *http.Request) {
	user, err := h.getAuthedOfficer(r)
	if err != nil {
		ErrorJSON(w, http.StatusForbidden, "FORBIDDEN", err.Error(), nil)
		return
	}

	evidenceID := chi.URLParam(r, "evidence_id")
	justification := r.URL.Query().Get("justification")
	clientIP := r.RemoteAddr

	url, err := h.svc.GetEvidenceDownloadURL(r.Context(), *user, evidenceID, justification, clientIP)
	if err != nil {
		if errors.Is(err, service.ErrConflictOfInterest) {
			ErrorJSON(w, http.StatusForbidden, "CONFLICT_OF_INTEREST", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, map[string]string{"download_url": url})
}

// SandboxDownloadEvidence serves local files during dev/test mock storage runs.
func (h *SafeguardingHandler) SandboxDownloadEvidence(w http.ResponseWriter, r *http.Request) {
	user, err := h.getAuthedOfficer(r)
	if err != nil {
		ErrorJSON(w, http.StatusForbidden, "FORBIDDEN", err.Error(), nil)
		return
	}
	_ = user

	storageKey := r.URL.Query().Get("key")
	filename := r.URL.Query().Get("filename")

	data, err := h.storage.ReadEvidence(r.Context(), storageKey)
	if err != nil {
		ErrorJSON(w, http.StatusNotFound, "NOT_FOUND", "Evidence file not found", nil)
		return
	}

	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	w.Header().Set("Cache-Control", "private, no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// SetLegalHold sets or removes legal hold.
func (h *SafeguardingHandler) SetLegalHold(w http.ResponseWriter, r *http.Request) {
	user, err := h.getAuthedOfficer(r)
	if err != nil {
		ErrorJSON(w, http.StatusForbidden, "FORBIDDEN", err.Error(), nil)
		return
	}

	caseID := chi.URLParam(r, "id")

	var req struct {
		IsLegalHold bool   `json:"is_legal_hold"`
		Reason      string `json:"reason"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON", nil)
		return
	}

	err = h.svc.SetLegalHold(r.Context(), *user, caseID, req.IsLegalHold, req.Reason)
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, map[string]string{"message": "Legal hold updated successfully"})
}

// ListAccessLogs lists immutable access audits.
func (h *SafeguardingHandler) ListAccessLogs(w http.ResponseWriter, r *http.Request) {
	user, err := h.getAuthedOfficer(r)
	if err != nil {
		ErrorJSON(w, http.StatusForbidden, "FORBIDDEN", err.Error(), nil)
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

	logs, total, err := h.svc.ListAccessLogs(r.Context(), *user, int32(page), int32(limit))
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, map[string]interface{}{
		"items": logs,
		"total": total,
	})
}

// ListActiveOfficers lists designated safeguarding officers.
func (h *SafeguardingHandler) ListActiveOfficers(w http.ResponseWriter, r *http.Request) {
	user, err := h.getAuthedOfficer(r)
	if err != nil {
		ErrorJSON(w, http.StatusForbidden, "FORBIDDEN", err.Error(), nil)
		return
	}

	officers, err := h.svc.ListActiveOfficers(r.Context(), *user)
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, map[string]interface{}{
		"items": officers,
	})
}

// AppointOfficer designates or updates a safeguarding officer (Super Admin only).
func (h *SafeguardingHandler) AppointOfficer(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil || user.PrimaryRole != "super_admin" {
		ErrorJSON(w, http.StatusForbidden, "FORBIDDEN", "Only Super Admin can appoint safeguarding personnel", nil)
		return
	}

	var req struct {
		UserID    string `json:"user_id"`
		RoleTitle string `json:"role_title"`
		IsLead    bool   `json:"is_lead"`
		CanAssign bool   `json:"can_assign"`
		CanClose  bool   `json:"can_close"`
		CanAudit  bool   `json:"can_audit"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON", nil)
		return
	}

	err := h.svc.AppointOfficer(r.Context(), *user, req.UserID, req.RoleTitle, req.IsLead, req.CanAssign, req.CanClose, req.CanAudit)
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, map[string]string{"message": "Safeguarding officer appointed successfully"})
}
