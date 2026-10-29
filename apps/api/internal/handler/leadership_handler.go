package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"ratels-api/internal/service"
)

type LeadershipHandler struct {
	leadService *service.LeadershipService
}

func NewLeadershipHandler(leadService *service.LeadershipService) *LeadershipHandler {
	return &LeadershipHandler{leadService: leadService}
}

func (h *LeadershipHandler) ListDirectory(w http.ResponseWriter, r *http.Request) {
	directory, err := h.leadService.ListPublicDirectory(r.Context())
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}
	JSON(w, http.StatusOK, directory)
}

func (h *LeadershipHandler) ListAppointments(w http.ResponseWriter, r *http.Request) {
	appointments, err := h.leadService.ListAppointments(r.Context())
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}
	JSON(w, http.StatusOK, appointments)
}

func (h *LeadershipHandler) CreateAppointment(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	actorRole := GetUserEffectiveRole(r.Context())

	var req service.CreateAppointmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request payload", nil)
		return
	}

	app, err := h.leadService.CreateAppointment(r.Context(), user.ID, actorRole, req)
	if err != nil {
		if err == service.ErrSuperAdminRequired || err == service.ErrSelfModificationProhibited {
			ErrorJSON(w, http.StatusForbidden, "FORBIDDEN", err.Error(), nil)
			return
		}
		if err == service.ErrDuplicateActiveAppointment {
			ErrorJSON(w, http.StatusConflict, "CONFLICT", err.Error(), nil)
			return
		}
		if err == service.ErrInvalidJurisdiction {
			ErrorJSON(w, http.StatusBadRequest, "INVALID_JURISDICTION", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusBadRequest, "APPOINTMENT_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusCreated, app)
}

func (h *LeadershipHandler) ReplaceAppointment(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	appointmentID := chi.URLParam(r, "id")
	if appointmentID == "" {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_ID", "Appointment ID required", nil)
		return
	}

	actorRole := GetUserEffectiveRole(r.Context())

	var req service.ReplaceAppointmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request payload", nil)
		return
	}

	newApp, err := h.leadService.ReplaceAppointment(r.Context(), user.ID, actorRole, appointmentID, req)
	if err != nil {
		if err == service.ErrSuperAdminRequired || err == service.ErrSelfModificationProhibited {
			ErrorJSON(w, http.StatusForbidden, "FORBIDDEN", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusBadRequest, "REPLACE_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusCreated, newApp)
}

func (h *LeadershipHandler) UpdateScopeDates(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	appointmentID := chi.URLParam(r, "id")
	if appointmentID == "" {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_ID", "Appointment ID required", nil)
		return
	}

	actorRole := GetUserEffectiveRole(r.Context())

	var req service.UpdateScopeDatesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request payload", nil)
		return
	}

	updated, err := h.leadService.UpdateAppointmentScopeAndDates(r.Context(), user.ID, actorRole, appointmentID, req)
	if err != nil {
		if err == service.ErrSuperAdminRequired || err == service.ErrSelfModificationProhibited {
			ErrorJSON(w, http.StatusForbidden, "FORBIDDEN", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusBadRequest, "UPDATE_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, updated)
}

type RevokeAppointmentPayload struct {
	Reason string `json:"reason"`
}

type ReactivateAppointmentPayload struct {
	Reason string `json:"reason"`
}

func (h *LeadershipHandler) RevokeAppointment(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	appointmentID := chi.URLParam(r, "id")
	if appointmentID == "" {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_ID", "Appointment ID required", nil)
		return
	}

	actorRole := GetUserEffectiveRole(r.Context())

	var payload RevokeAppointmentPayload
	_ = json.NewDecoder(r.Body).Decode(&payload)
	if payload.Reason == "" {
		payload.Reason = "Revoked by administrator"
	}

	err := h.leadService.RevokeAppointment(r.Context(), user.ID, actorRole, appointmentID, payload.Reason)
	if err != nil {
		if err == service.ErrSuperAdminRequired || err == service.ErrSelfModificationProhibited {
			ErrorJSON(w, http.StatusForbidden, "FORBIDDEN", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusBadRequest, "REVOKE_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, map[string]string{
		"message": "Appointment revoked successfully",
	})
}

func (h *LeadershipHandler) SuspendAppointment(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	appointmentID := chi.URLParam(r, "id")
	if appointmentID == "" {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_ID", "Appointment ID required", nil)
		return
	}

	actorRole := GetUserEffectiveRole(r.Context())

	var payload RevokeAppointmentPayload
	_ = json.NewDecoder(r.Body).Decode(&payload)
	if payload.Reason == "" {
		payload.Reason = "Suspended by administrator"
	}

	err := h.leadService.SuspendAppointment(r.Context(), user.ID, actorRole, appointmentID, payload.Reason)
	if err != nil {
		if err == service.ErrSuperAdminRequired || err == service.ErrSelfModificationProhibited {
			ErrorJSON(w, http.StatusForbidden, "FORBIDDEN", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusBadRequest, "SUSPEND_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, map[string]string{
		"message": "Appointment suspended successfully",
	})
}

func (h *LeadershipHandler) ReactivateAppointment(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	appointmentID := chi.URLParam(r, "id")
	if appointmentID == "" {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_ID", "Appointment ID required", nil)
		return
	}

	actorRole := GetUserEffectiveRole(r.Context())

	var payload ReactivateAppointmentPayload
	_ = json.NewDecoder(r.Body).Decode(&payload)
	if payload.Reason == "" {
		payload.Reason = "Reactivated by administrator"
	}

	app, err := h.leadService.ReactivateAppointment(r.Context(), user.ID, actorRole, appointmentID, payload.Reason)
	if err != nil {
		if err == service.ErrSuperAdminRequired || err == service.ErrSelfModificationProhibited {
			ErrorJSON(w, http.StatusForbidden, "FORBIDDEN", err.Error(), nil)
			return
		}
		if err == service.ErrDuplicateActiveAppointment {
			ErrorJSON(w, http.StatusConflict, "CONFLICT", err.Error(), nil)
			return
		}
		if err == service.ErrInvalidStatusTransition {
			ErrorJSON(w, http.StatusBadRequest, "INVALID_TRANSITION", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusBadRequest, "REACTIVATE_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, app)
}

func (h *LeadershipHandler) ListAppointmentHistory(w http.ResponseWriter, r *http.Request) {
	appointmentID := chi.URLParam(r, "id")
	if appointmentID == "" {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_ID", "Appointment ID required", nil)
		return
	}

	history, err := h.leadService.ListAppointmentHistory(r.Context(), appointmentID)
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, history)
}

// Appointment changes retain a specific denial code used by existing clients.
// The service also reloads current Super Admin authority before every mutation.
func RequireAppointmentAuthority(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u := GetCurrentUser(r.Context())
		allowed := u != nil && u.PrimaryRole == "super_admin"
		for _, a := range GetCurrentAppointments(r.Context()) {
			if a.Status == "active" && a.RoleType == "super_admin" && a.JurisdictionType == "national" {
				allowed = true
			}
		}
		if !allowed {
			ErrorJSON(w, 403, "ERR_SUPER_ADMIN_REQUIRED", "Only Super Admin can manage appointments", nil)
			return
		}
		next.ServeHTTP(w, r)
	})
}
