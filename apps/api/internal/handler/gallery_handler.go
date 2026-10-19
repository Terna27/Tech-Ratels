package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"ratels-api/internal/db"
	"ratels-api/internal/service"
	"ratels-api/internal/storage"
)

type GalleryHandler struct {
	galleryService *service.GalleryService
}

func NewGalleryHandler(galleryService *service.GalleryService) *GalleryHandler {
	return &GalleryHandler{
		galleryService: galleryService,
	}
}

// -------------------------------------------------------------
// Public Endpoints
// -------------------------------------------------------------

// ListPublicGallery handles public discovery of approved and published event media.
func (h *GalleryHandler) ListPublicGallery(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	var mediaTypePtr, zonePtr, statePtr, lgaPtr, eventPtr *string
	if val := q.Get("type"); val != "" {
		mediaTypePtr = &val
	}
	if val := q.Get("zone_id"); val != "" {
		zonePtr = &val
	}
	if val := q.Get("state_id"); val != "" {
		statePtr = &val
	}
	if val := q.Get("lga_id"); val != "" {
		lgaPtr = &val
	}
	if val := q.Get("event_id"); val != "" {
		eventPtr = &val
	}

	var yearPtr *int32
	if val := q.Get("year"); val != "" {
		if y, err := strconv.Atoi(val); err == nil && y > 1900 && y < 2100 {
			y32 := int32(y)
			yearPtr = &y32
		}
	}

	page := int32(1)
	if val := q.Get("page"); val != "" {
		if p, err := strconv.Atoi(val); err == nil && p > 0 {
			page = int32(p)
		}
	}

	limit := int32(24)
	if val := q.Get("limit"); val != "" {
		if l, err := strconv.Atoi(val); err == nil && l > 0 && l <= 100 {
			limit = int32(l)
		}
	}

	res, err := h.galleryService.ListPublicGallery(r.Context(), service.PublicGalleryFilter{
		MediaType: mediaTypePtr,
		ZoneID:    zonePtr,
		StateID:   statePtr,
		LgaID:     lgaPtr,
		EventID:   eventPtr,
		Year:      yearPtr,
		Page:      page,
		Limit:     limit,
	})
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, res)
}

// GetPublicMediaDetail returns detailed public information for a single approved media item.
func (h *GalleryHandler) GetPublicMediaDetail(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		ErrorJSON(w, http.StatusBadRequest, "MISSING_ID", "Media ID is required", nil)
		return
	}

	media, err := h.galleryService.GetPublicMediaDetail(r.Context(), id)
	if err != nil {
		if errors.Is(err, service.ErrMediaNotFound) {
			ErrorJSON(w, http.StatusNotFound, "NOT_FOUND", "Media not found or unapproved", nil)
			return
		}
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, media)
}

// ListFeatured returns a small selection of recently approved featured media for the homepage.
func (h *GalleryHandler) ListFeatured(w http.ResponseWriter, r *http.Request) {
	limit := int32(6)
	if val := r.URL.Query().Get("limit"); val != "" {
		if l, err := strconv.Atoi(val); err == nil && l > 0 && l <= 20 {
			limit = int32(l)
		}
	}

	items, err := h.galleryService.ListFeaturedGallery(r.Context(), limit)
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}

	if items == nil {
		items = []db.ListFeaturedGalleryMediaRow{}
	}

	JSON(w, http.StatusOK, items)
}

// ListEventGallery returns approved media for a specific cleanup event.
func (h *GalleryHandler) ListEventGallery(w http.ResponseWriter, r *http.Request) {
	eventID := chi.URLParam(r, "eventId")
	if eventID == "" {
		eventID = chi.URLParam(r, "id")
	}
	if eventID == "" {
		ErrorJSON(w, http.StatusBadRequest, "MISSING_ID", "Event ID is required", nil)
		return
	}

	page := int32(1)
	if val := r.URL.Query().Get("page"); val != "" {
		if p, err := strconv.Atoi(val); err == nil && p > 0 {
			page = int32(p)
		}
	}

	limit := int32(12)
	if val := r.URL.Query().Get("limit"); val != "" {
		if l, err := strconv.Atoi(val); err == nil && l > 0 && l <= 50 {
			limit = int32(l)
		}
	}

	items, err := h.galleryService.ListEventGallery(r.Context(), eventID, page, limit)
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}

	if items == nil {
		items = []db.ListEventGalleryMediaRow{}
	}

	JSON(w, http.StatusOK, items)
}

// ReportMedia records a public or authenticated visitor complaint against a media item.
func (h *GalleryHandler) ReportMedia(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		ErrorJSON(w, http.StatusBadRequest, "MISSING_ID", "Media ID is required", nil)
		return
	}

	user := GetCurrentUser(r.Context())

	var req service.CreateMediaReportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Malformed JSON payload", nil)
		return
	}

	report, err := h.galleryService.ReportMedia(r.Context(), user, id, req)
	if err != nil {
		if errors.Is(err, service.ErrMediaNotFound) {
			ErrorJSON(w, http.StatusNotFound, "NOT_FOUND", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusBadRequest, "REPORT_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusCreated, report)
}

// SubmitTakedown submits a formal media takedown request.
func (h *GalleryHandler) SubmitTakedown(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		ErrorJSON(w, http.StatusBadRequest, "MISSING_ID", "Media ID is required", nil)
		return
	}

	user := GetCurrentUser(r.Context())

	var req service.CreateMediaTakedownRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Malformed JSON payload", nil)
		return
	}

	takedown, err := h.galleryService.SubmitTakedownRequest(r.Context(), user, id, req)
	if err != nil {
		if errors.Is(err, service.ErrMediaNotFound) {
			ErrorJSON(w, http.StatusNotFound, "NOT_FOUND", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusBadRequest, "TAKEDOWN_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusCreated, takedown)
}

// -------------------------------------------------------------
// Authenticated Member Endpoints
// -------------------------------------------------------------

// AuthorizeUpload generates a presigned R2 upload URL for member uploads.
func (h *GalleryHandler) AuthorizeUpload(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	var req service.AuthorizeGalleryUploadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Malformed JSON payload", nil)
		return
	}

	res, err := h.galleryService.AuthorizeUpload(r.Context(), user, req)
	if err != nil {
		if errors.Is(err, service.ErrEventNotFound) {
			ErrorJSON(w, http.StatusNotFound, "EVENT_NOT_FOUND", err.Error(), nil)
			return
		}
		if errors.Is(err, service.ErrEventNotEligible) {
			ErrorJSON(w, http.StatusBadRequest, "EVENT_NOT_ELIGIBLE", err.Error(), nil)
			return
		}
		if errors.Is(err, storage.ErrStorageUnconfigured) {
			ErrorJSON(w, http.StatusServiceUnavailable, "STORAGE_UNCONFIGURED", "Media storage is temporarily unconfigured.", nil)
			return
		}
		ErrorJSON(w, http.StatusBadRequest, "AUTHORIZE_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, res)
}

// SubmitMedia records an uploaded media item into the pending moderation queue.
func (h *GalleryHandler) SubmitMedia(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	var req service.SubmitGalleryMediaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Malformed JSON payload", nil)
		return
	}

	media, err := h.galleryService.SubmitMedia(r.Context(), user, req)
	if err != nil {
		if errors.Is(err, service.ErrEventNotFound) {
			ErrorJSON(w, http.StatusNotFound, "EVENT_NOT_FOUND", err.Error(), nil)
			return
		}
		if errors.Is(err, service.ErrConsentRequired) || errors.Is(err, service.ErrGuardianConsentRequired) {
			ErrorJSON(w, http.StatusBadRequest, "CONSENT_REQUIRED", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusBadRequest, "SUBMIT_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusCreated, media)
}

// SubmitDirectUpload handles multipart form upload directly to the backend.
func (h *GalleryHandler) SubmitDirectUpload(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	// 100 MB max memory limit
	if err := r.ParseMultipartForm(100 * 1024 * 1024); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_MULTIPART", "Failed to parse multipart form data: "+err.Error(), nil)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		ErrorJSON(w, http.StatusBadRequest, "MISSING_FILE", "A media file is required", nil)
		return
	}
	defer file.Close()

	eventID := r.FormValue("event_id")
	mediaType := r.FormValue("media_type")
	caption := r.FormValue("caption")
	desc := r.FormValue("description")
	var descPtr *string
	if desc != "" {
		descPtr = &desc
	}
	allowAttribution := r.FormValue("allow_public_attribution") == "true"

	consentNotes := r.FormValue("consent_notes")
	var notesPtr *string
	if consentNotes != "" {
		notesPtr = &consentNotes
	}

	consent := service.GalleryConsentInput{
		HasOwnerPermission:       r.FormValue("has_owner_permission") == "true",
		UnderstandsPublicDisplay: r.FormValue("understands_public_display") == "true",
		HasIdentifiableConsent:   r.FormValue("has_identifiable_consent") == "true",
		HasNoConfidentialInfo:    r.FormValue("has_no_confidential_info") == "true",
		HasMinors:                r.FormValue("has_minors") == "true",
		HasGuardianConsent:       r.FormValue("has_guardian_consent") == "true",
		ConsentNotes:             notesPtr,
	}

	media, err := h.galleryService.SubmitDirectMedia(
		r.Context(),
		user,
		eventID,
		mediaType,
		caption,
		descPtr,
		allowAttribution,
		consent,
		file,
		header.Filename,
		header.Header.Get("Content-Type"),
		header.Size,
	)
	if err != nil {
		if errors.Is(err, service.ErrEventNotFound) {
			ErrorJSON(w, http.StatusNotFound, "EVENT_NOT_FOUND", err.Error(), nil)
			return
		}
		if errors.Is(err, service.ErrConsentRequired) || errors.Is(err, service.ErrGuardianConsentRequired) {
			ErrorJSON(w, http.StatusBadRequest, "CONSENT_REQUIRED", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusBadRequest, "UPLOAD_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusCreated, media)
}

// ListMySubmissions returns media uploaded by the currently authenticated user.
func (h *GalleryHandler) ListMySubmissions(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	page := int32(1)
	if val := r.URL.Query().Get("page"); val != "" {
		if p, err := strconv.Atoi(val); err == nil && p > 0 {
			page = int32(p)
		}
	}

	limit := int32(20)
	if val := r.URL.Query().Get("limit"); val != "" {
		if l, err := strconv.Atoi(val); err == nil && l > 0 && l <= 100 {
			limit = int32(l)
		}
	}

	res, err := h.galleryService.ListMySubmissions(r.Context(), user, page, limit)
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, res)
}

// WithdrawSubmission allows an uploader to withdraw their own pending/revision media.
func (h *GalleryHandler) WithdrawSubmission(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		ErrorJSON(w, http.StatusBadRequest, "MISSING_ID", "Media ID is required", nil)
		return
	}

	var req struct {
		Reason string `json:"reason"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	updated, err := h.galleryService.WithdrawSubmission(r.Context(), user, id, req.Reason)
	if err != nil {
		if errors.Is(err, service.ErrMediaNotFound) {
			ErrorJSON(w, http.StatusNotFound, "NOT_FOUND", err.Error(), nil)
			return
		}
		if errors.Is(err, service.ErrNotMediaUploader) {
			ErrorJSON(w, http.StatusForbidden, "FORBIDDEN", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusBadRequest, "WITHDRAW_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, updated)
}

// -------------------------------------------------------------
// Admin Moderation Endpoints
// -------------------------------------------------------------

// GetAdminOverview returns gallery moderation statistics.
func (h *GalleryHandler) GetAdminOverview(w http.ResponseWriter, r *http.Request) {
	stats, err := h.galleryService.GetAdminOverview(r.Context())
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}
	JSON(w, http.StatusOK, stats)
}

// ListAdminGallery lists media submissions for moderation with geographic filtering.
func (h *GalleryHandler) ListAdminGallery(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}
	apps := GetCurrentAppointments(r.Context())

	q := r.URL.Query()
	var statusPtr, zonePtr, statePtr, lgaPtr, eventPtr *string
	if val := q.Get("status"); val != "" {
		statusPtr = &val
	}
	if val := q.Get("zone_id"); val != "" {
		zonePtr = &val
	}
	if val := q.Get("state_id"); val != "" {
		statePtr = &val
	}
	if val := q.Get("lga_id"); val != "" {
		lgaPtr = &val
	}
	if val := q.Get("event_id"); val != "" {
		eventPtr = &val
	}

	page := int32(1)
	if val := q.Get("page"); val != "" {
		if p, err := strconv.Atoi(val); err == nil && p > 0 {
			page = int32(p)
		}
	}

	limit := int32(24)
	if val := q.Get("limit"); val != "" {
		if l, err := strconv.Atoi(val); err == nil && l > 0 && l <= 100 {
			limit = int32(l)
		}
	}

	res, err := h.galleryService.ListAdminGallery(r.Context(), user, apps, service.AdminGalleryFilter{
		Status:  statusPtr,
		ZoneID:  zonePtr,
		StateID: statePtr,
		LgaID:   lgaPtr,
		EventID: eventPtr,
		Page:    page,
		Limit:   limit,
	})
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, res)
}

// GetAdminMediaDetail returns detailed media and consent records for review.
func (h *GalleryHandler) GetAdminMediaDetail(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}
	apps := GetCurrentAppointments(r.Context())

	id := chi.URLParam(r, "id")
	if id == "" {
		ErrorJSON(w, http.StatusBadRequest, "MISSING_ID", "Media ID is required", nil)
		return
	}

	media, consent, actions, err := h.galleryService.GetAdminMediaDetail(r.Context(), user, apps, id)
	if err != nil {
		if errors.Is(err, service.ErrMediaNotFound) {
			ErrorJSON(w, http.StatusNotFound, "NOT_FOUND", err.Error(), nil)
			return
		}
		if errors.Is(err, service.ErrUnauthorizedReviewer) {
			ErrorJSON(w, http.StatusForbidden, "FORBIDDEN", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, map[string]interface{}{
		"media":   media,
		"consent": consent,
		"actions": actions,
	})
}

// ApproveMedia approves media, performs metadata sanitization, and publishes derivative.
func (h *GalleryHandler) ApproveMedia(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}
	apps := GetCurrentAppointments(r.Context())

	id := chi.URLParam(r, "id")
	if id == "" {
		ErrorJSON(w, http.StatusBadRequest, "MISSING_ID", "Media ID is required", nil)
		return
	}

	var req struct {
		Notes *string `json:"notes"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	approved, err := h.galleryService.ApproveMedia(r.Context(), user, apps, id, req.Notes)
	if err != nil {
		if errors.Is(err, service.ErrMediaNotFound) {
			ErrorJSON(w, http.StatusNotFound, "NOT_FOUND", err.Error(), nil)
			return
		}
		if errors.Is(err, service.ErrSelfApprovalNotAllowed) {
			ErrorJSON(w, http.StatusForbidden, "SELF_APPROVAL_FORBIDDEN", err.Error(), nil)
			return
		}
		if errors.Is(err, service.ErrUnauthorizedReviewer) {
			ErrorJSON(w, http.StatusForbidden, "UNAUTHORIZED_REVIEWER", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusBadRequest, "APPROVE_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, approved)
}

// RejectMedia rejects media with a mandatory documented reason.
func (h *GalleryHandler) RejectMedia(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}
	apps := GetCurrentAppointments(r.Context())

	id := chi.URLParam(r, "id")
	if id == "" {
		ErrorJSON(w, http.StatusBadRequest, "MISSING_ID", "Media ID is required", nil)
		return
	}

	var req struct {
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Malformed JSON payload", nil)
		return
	}

	rejected, err := h.galleryService.RejectMedia(r.Context(), user, apps, id, req.Reason)
	if err != nil {
		if errors.Is(err, service.ErrMediaNotFound) {
			ErrorJSON(w, http.StatusNotFound, "NOT_FOUND", err.Error(), nil)
			return
		}
		if errors.Is(err, service.ErrSelfApprovalNotAllowed) {
			ErrorJSON(w, http.StatusForbidden, "SELF_APPROVAL_FORBIDDEN", err.Error(), nil)
			return
		}
		if errors.Is(err, service.ErrUnauthorizedReviewer) {
			ErrorJSON(w, http.StatusForbidden, "UNAUTHORIZED_REVIEWER", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusBadRequest, "REJECT_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, rejected)
}

// RequestRevision requests revision from uploader.
func (h *GalleryHandler) RequestRevision(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}
	apps := GetCurrentAppointments(r.Context())

	id := chi.URLParam(r, "id")
	if id == "" {
		ErrorJSON(w, http.StatusBadRequest, "MISSING_ID", "Media ID is required", nil)
		return
	}

	var req struct {
		Notes string `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Malformed JSON payload", nil)
		return
	}

	updated, err := h.galleryService.RequestRevisionMedia(r.Context(), user, apps, id, req.Notes)
	if err != nil {
		if errors.Is(err, service.ErrMediaNotFound) {
			ErrorJSON(w, http.StatusNotFound, "NOT_FOUND", err.Error(), nil)
			return
		}
		if errors.Is(err, service.ErrSelfApprovalNotAllowed) {
			ErrorJSON(w, http.StatusForbidden, "SELF_APPROVAL_FORBIDDEN", err.Error(), nil)
			return
		}
		if errors.Is(err, service.ErrUnauthorizedReviewer) {
			ErrorJSON(w, http.StatusForbidden, "UNAUTHORIZED_REVIEWER", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusBadRequest, "REVISION_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, updated)
}

// RemoveMedia unpublishes media, hides it, and deletes the public object from R2.
func (h *GalleryHandler) RemoveMedia(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}
	apps := GetCurrentAppointments(r.Context())

	id := chi.URLParam(r, "id")
	if id == "" {
		ErrorJSON(w, http.StatusBadRequest, "MISSING_ID", "Media ID is required", nil)
		return
	}

	var req struct {
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Malformed JSON payload", nil)
		return
	}

	removed, err := h.galleryService.RemoveMedia(r.Context(), user, apps, id, req.Reason)
	if err != nil {
		if errors.Is(err, service.ErrMediaNotFound) {
			ErrorJSON(w, http.StatusNotFound, "NOT_FOUND", err.Error(), nil)
			return
		}
		if errors.Is(err, service.ErrUnauthorizedReviewer) {
			ErrorJSON(w, http.StatusForbidden, "UNAUTHORIZED_REVIEWER", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusBadRequest, "REMOVE_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, removed)
}

// ToggleHideMedia temporarily hides media during active investigation.
func (h *GalleryHandler) ToggleHideMedia(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}
	apps := GetCurrentAppointments(r.Context())

	id := chi.URLParam(r, "id")
	if id == "" {
		ErrorJSON(w, http.StatusBadRequest, "MISSING_ID", "Media ID is required", nil)
		return
	}

	var req struct {
		IsHidden bool   `json:"is_hidden"`
		Reason   string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Malformed JSON payload", nil)
		return
	}

	updated, err := h.galleryService.ToggleHideMedia(r.Context(), user, apps, id, req.IsHidden, req.Reason)
	if err != nil {
		if errors.Is(err, service.ErrMediaNotFound) {
			ErrorJSON(w, http.StatusNotFound, "NOT_FOUND", err.Error(), nil)
			return
		}
		if errors.Is(err, service.ErrUnauthorizedReviewer) {
			ErrorJSON(w, http.StatusForbidden, "UNAUTHORIZED_REVIEWER", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusBadRequest, "TOGGLE_HIDE_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, updated)
}

// ListReports lists media complaints.
func (h *GalleryHandler) ListReports(w http.ResponseWriter, r *http.Request) {
	var statusPtr *string
	if val := r.URL.Query().Get("status"); val != "" {
		statusPtr = &val
	}

	page := int32(1)
	if val := r.URL.Query().Get("page"); val != "" {
		if p, err := strconv.Atoi(val); err == nil && p > 0 {
			page = int32(p)
		}
	}

	limit := int32(20)
	if val := r.URL.Query().Get("limit"); val != "" {
		if l, err := strconv.Atoi(val); err == nil && l > 0 && l <= 100 {
			limit = int32(l)
		}
	}

	items, total, err := h.galleryService.ListReports(r.Context(), statusPtr, page, limit)
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, map[string]interface{}{
		"items": items,
		"total": total,
		"page":  page,
		"limit": limit,
	})
}

// ResolveReport marks a complaint investigated/resolved/dismissed.
func (h *GalleryHandler) ResolveReport(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		ErrorJSON(w, http.StatusBadRequest, "MISSING_ID", "Report ID is required", nil)
		return
	}

	var req struct {
		Status string `json:"status"`
		Notes  string `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Malformed JSON payload", nil)
		return
	}

	resolved, err := h.galleryService.ResolveReport(r.Context(), user, id, req.Status, req.Notes)
	if err != nil {
		ErrorJSON(w, http.StatusBadRequest, "RESOLVE_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, resolved)
}

// ListTakedowns lists formal takedown requests.
func (h *GalleryHandler) ListTakedowns(w http.ResponseWriter, r *http.Request) {
	var statusPtr *string
	if val := r.URL.Query().Get("status"); val != "" {
		statusPtr = &val
	}

	page := int32(1)
	if val := r.URL.Query().Get("page"); val != "" {
		if p, err := strconv.Atoi(val); err == nil && p > 0 {
			page = int32(p)
		}
	}

	limit := int32(20)
	if val := r.URL.Query().Get("limit"); val != "" {
		if l, err := strconv.Atoi(val); err == nil && l > 0 && l <= 100 {
			limit = int32(l)
		}
	}

	items, total, err := h.galleryService.ListTakedowns(r.Context(), statusPtr, page, limit)
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, map[string]interface{}{
		"items": items,
		"total": total,
		"page":  page,
		"limit": limit,
	})
}

// ResolveTakedown decides on a formal takedown request.
func (h *GalleryHandler) ResolveTakedown(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}
	apps := GetCurrentAppointments(r.Context())

	id := chi.URLParam(r, "id")
	if id == "" {
		ErrorJSON(w, http.StatusBadRequest, "MISSING_ID", "Takedown ID is required", nil)
		return
	}

	var req struct {
		Decision string `json:"decision"`
		Reason   string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Malformed JSON payload", nil)
		return
	}

	resolved, err := h.galleryService.ResolveTakedown(r.Context(), user, apps, id, req.Decision, req.Reason)
	if err != nil {
		if errors.Is(err, service.ErrTakedownNotFound) {
			ErrorJSON(w, http.StatusNotFound, "NOT_FOUND", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusBadRequest, "RESOLVE_TAKEDOWN_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, resolved)
}
