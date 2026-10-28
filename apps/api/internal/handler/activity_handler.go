package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"ratels-api/internal/service"
)

type ActivityHandler struct {
	cleanupService *service.CleanupService
}

func NewActivityHandler(cleanupService *service.CleanupService) *ActivityHandler {
	return &ActivityHandler{cleanupService: cleanupService}
}

func (h *ActivityHandler) GetCurrentCampaign(w http.ResponseWriter, r *http.Request) {
	campaign, err := h.cleanupService.GetCurrentCampaign(r.Context())
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}
	JSON(w, http.StatusOK, campaign)
}

func (h *ActivityHandler) ListEvents(w http.ResponseWriter, r *http.Request) {
	stateFilter := r.URL.Query().Get("state_id")
	publishedOnly := r.URL.Query().Get("published_only") != "false"

	var pState *string
	if stateFilter != "" {
		pState = &stateFilter
	}

	events, err := h.cleanupService.ListEvents(r.Context(), pState, publishedOnly)
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}
	JSON(w, http.StatusOK, events)
}

func (h *ActivityHandler) GetEvent(w http.ResponseWriter, r *http.Request) {
	eventID := chi.URLParam(r, "id")
	if eventID == "" {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_ID", "Event ID required", nil)
		return
	}

	event, err := h.cleanupService.GetEvent(r.Context(), eventID)
	if err != nil {
		ErrorJSON(w, http.StatusNotFound, "NOT_FOUND", err.Error(), nil)
		return
	}
	JSON(w, http.StatusOK, event)
}

func (h *ActivityHandler) CreateEvent(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	var req service.CreateEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request payload", nil)
		return
	}

	// Determine active role & jurisdiction
	actorRole := user.PrimaryRole
	var actorJurisdictionID *string

	apps := GetCurrentAppointments(r.Context())
	for _, a := range apps {
		if a.Status == "active" {
			actorRole = a.RoleType
			if a.JurisdictionID.Valid {
				jid := a.JurisdictionID.String
				actorJurisdictionID = &jid
			}
			break
		}
	}

	created, err := h.cleanupService.CreateEvent(r.Context(), user.ID, actorRole, actorJurisdictionID, req)
	if err != nil {
		if err == service.ErrScopeUnauthorized {
			ErrorJSON(w, http.StatusForbidden, "SCOPE_UNAUTHORIZED", err.Error(), nil)
			return
		}
		if err == service.ErrCoordinatorUnauthorized {
			ErrorJSON(w, http.StatusForbidden, "COORDINATOR_UNAUTHORIZED", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusBadRequest, "CREATE_EVENT_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusCreated, created)
}

func (h *ActivityHandler) PublishEvent(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	eventID := chi.URLParam(r, "id")
	if eventID == "" {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_ID", "Event ID required", nil)
		return
	}

	actorRole := user.PrimaryRole
	var actorJurisdictionID *string

	apps := GetCurrentAppointments(r.Context())
	for _, a := range apps {
		if a.Status == "active" {
			actorRole = a.RoleType
			if a.JurisdictionID.Valid {
				jid := a.JurisdictionID.String
				actorJurisdictionID = &jid
			}
			break
		}
	}

	updated, err := h.cleanupService.PublishEvent(r.Context(), user.ID, actorRole, actorJurisdictionID, eventID)
	if err != nil {
		if err == service.ErrScopeUnauthorized {
			ErrorJSON(w, http.StatusForbidden, "SCOPE_UNAUTHORIZED", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusBadRequest, "PUBLISH_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, updated)
}

func (h *ActivityHandler) RescheduleEvent(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	eventID := chi.URLParam(r, "id")
	if eventID == "" {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_ID", "Event ID required", nil)
		return
	}

	var req service.RescheduleEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request payload", nil)
		return
	}

	actorRole := user.PrimaryRole
	var actorJurisdictionID *string

	apps := GetCurrentAppointments(r.Context())
	for _, a := range apps {
		if a.Status == "active" {
			actorRole = a.RoleType
			if a.JurisdictionID.Valid {
				jid := a.JurisdictionID.String
				actorJurisdictionID = &jid
			}
			break
		}
	}

	updated, err := h.cleanupService.RescheduleEvent(r.Context(), user.ID, actorRole, actorJurisdictionID, eventID, req)
	if err != nil {
		if err == service.ErrScopeUnauthorized {
			ErrorJSON(w, http.StatusForbidden, "SCOPE_UNAUTHORIZED", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusBadRequest, "RESCHEDULE_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, updated)
}

func (h *ActivityHandler) RegisterForEvent(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	eventID := chi.URLParam(r, "id")
	if eventID == "" {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_ID", "Event ID required", nil)
		return
	}

	reg, err := h.cleanupService.RegisterForEvent(r.Context(), user.ID, eventID)
	if err != nil {
		if err == service.ErrAlreadyRegistered {
			ErrorJSON(w, http.StatusConflict, "ALREADY_REGISTERED", err.Error(), nil)
			return
		}
		if err == service.ErrEventAtCapacity {
			ErrorJSON(w, http.StatusConflict, "EVENT_AT_CAPACITY", err.Error(), nil)
			return
		}
		if err == service.ErrEventNotPublished {
			ErrorJSON(w, http.StatusBadRequest, "EVENT_NOT_PUBLISHED", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusBadRequest, "REGISTRATION_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, reg)
}

func (h *ActivityHandler) CancelRegistration(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	eventID := chi.URLParam(r, "id")
	if eventID == "" {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_ID", "Event ID required", nil)
		return
	}

	err := h.cleanupService.CancelRegistration(r.Context(), user.ID, eventID)
	if err != nil {
		ErrorJSON(w, http.StatusBadRequest, "CANCEL_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, map[string]string{
		"message": "Registration cancelled successfully",
	})
}

func (h *ActivityHandler) CancelEvent(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	eventID := chi.URLParam(r, "id")
	if eventID == "" {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_ID", "Event ID required", nil)
		return
	}

	var req service.CancelEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req.Reason = "Cancelled by coordinator"
	}

	actorRole := user.PrimaryRole
	var actorJurisdictionID *string

	apps := GetCurrentAppointments(r.Context())
	for _, a := range apps {
		if a.Status == "active" {
			actorRole = a.RoleType
			if a.JurisdictionID.Valid {
				jid := a.JurisdictionID.String
				actorJurisdictionID = &jid
			}
			break
		}
	}

	updated, err := h.cleanupService.CancelEvent(r.Context(), user.ID, actorRole, actorJurisdictionID, eventID, req)
	if err != nil {
		if err == service.ErrScopeUnauthorized {
			ErrorJSON(w, http.StatusForbidden, "SCOPE_UNAUTHORIZED", err.Error(), nil)
			return
		}
		if err == service.ErrCoordinatorUnauthorized {
			ErrorJSON(w, http.StatusForbidden, "COORDINATOR_UNAUTHORIZED", err.Error(), nil)
			return
		}
		if err == service.ErrEventNotFound {
			ErrorJSON(w, http.StatusNotFound, "NOT_FOUND", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusBadRequest, "CANCEL_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, updated)
}

func (h *ActivityHandler) GetMyRegistration(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	eventID := chi.URLParam(r, "id")
	if eventID == "" {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_ID", "Event ID required", nil)
		return
	}

	reg, err := h.cleanupService.GetMyRegistration(r.Context(), user.ID, eventID)
	if err != nil {
		if err == service.ErrNotRegistered {
			ErrorJSON(w, http.StatusNotFound, "NOT_REGISTERED", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusBadRequest, "FETCH_REGISTRATION_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, reg)
}

func (h *ActivityHandler) ListMyRegistrations(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	regs, err := h.cleanupService.ListMyRegistrations(r.Context(), user.ID)
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, regs)
}

func (h *ActivityHandler) VerifyAttendance(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	eventID := chi.URLParam(r, "id")
	if eventID == "" {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_ID", "Event ID required", nil)
		return
	}

	var req service.VerifyAttendanceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request payload", nil)
		return
	}

	reg, claim, err := h.cleanupService.VerifyAttendance(r.Context(), user.ID, eventID, req)
	if err != nil {
		if err == service.ErrCoordinatorUnauthorized {
			ErrorJSON(w, http.StatusForbidden, "COORDINATOR_UNAUTHORIZED", err.Error(), nil)
			return
		}
		if err == service.ErrScopeUnauthorized {
			ErrorJSON(w, http.StatusForbidden, "SCOPE_UNAUTHORIZED", err.Error(), nil)
			return
		}
		if err == service.ErrSelfVerificationNotAllowed {
			ErrorJSON(w, http.StatusForbidden, "ANTI_SELF_VERIFICATION", err.Error(), nil)
			return
		}
		if err == service.ErrCheckinWindowClosed {
			ErrorJSON(w, http.StatusBadRequest, "CHECKIN_WINDOW_CLOSED", err.Error(), nil)
			return
		}
		if err == service.ErrAlreadyVerified {
			ErrorJSON(w, http.StatusConflict, "ALREADY_VERIFIED", err.Error(), nil)
			return
		}
		if err == service.ErrInvalidCheckinCode {
			ErrorJSON(w, http.StatusNotFound, "INVALID_CHECKIN_CODE", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusBadRequest, "VERIFICATION_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, map[string]any{
		"registration": reg,
		"claim":        claim,
		"message":      "Attendance verified successfully",
	})
}

func (h *ActivityHandler) ListParticipants(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	eventID := chi.URLParam(r, "id")
	if eventID == "" {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_ID", "Event ID required", nil)
		return
	}

	participants, err := h.cleanupService.ListParticipants(r.Context(), user.ID, eventID)
	if err != nil {
		if err == service.ErrScopeUnauthorized {
			ErrorJSON(w, http.StatusForbidden, "SCOPE_UNAUTHORIZED", err.Error(), nil)
			return
		}
		if err == service.ErrCoordinatorUnauthorized {
			ErrorJSON(w, http.StatusForbidden, "COORDINATOR_UNAUTHORIZED", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusBadRequest, "FETCH_PARTICIPANTS_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, participants)
}
