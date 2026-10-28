package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"ratels-api/internal/service"
)

type ParticipationHandler struct {
	partService *service.ParticipationService
}

func NewParticipationHandler(partService *service.ParticipationService) *ParticipationHandler {
	return &ParticipationHandler{partService: partService}
}

func (h *ParticipationHandler) SubmitClaim(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	var req service.SubmitClaimRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request payload", nil)
		return
	}

	claim, err := h.partService.SubmitClaim(r.Context(), user.ID, req)
	if err != nil {
		if err == service.ErrDuplicateClaim {
			ErrorJSON(w, http.StatusConflict, "DUPLICATE_CLAIM", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusBadRequest, "CLAIM_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusCreated, claim)
}

func (h *ParticipationHandler) ListReviewQueue(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
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

	queue, err := h.partService.ListReviewQueue(r.Context(), user.ID, actorRole, actorJurisdictionID)
	if err != nil {
		if err == service.ErrScopeUnauthorized {
			ErrorJSON(w, http.StatusForbidden, "SCOPE_UNAUTHORIZED", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, queue)
}

func (h *ParticipationHandler) ReviewClaim(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	claimID := chi.URLParam(r, "id")
	if claimID == "" {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_ID", "Claim ID required", nil)
		return
	}

	var req service.ReviewClaimRequest
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

	reviewed, err := h.partService.ReviewClaim(r.Context(), user.ID, actorRole, actorJurisdictionID, claimID, req)
	if err != nil {
		if err == service.ErrSelfApprovalNotAllowed {
			ErrorJSON(w, http.StatusForbidden, "ANTI_SELF_APPROVAL", err.Error(), nil)
			return
		}
		if err == service.ErrScopeUnauthorized {
			ErrorJSON(w, http.StatusForbidden, "SCOPE_UNAUTHORIZED", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusBadRequest, "REVIEW_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, reviewed)
}

func (h *ParticipationHandler) SubmitAppeal(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	claimID := chi.URLParam(r, "id")
	if claimID == "" {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_ID", "Claim ID required", nil)
		return
	}

	var req service.SubmitAppealRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request payload", nil)
		return
	}

	appeal, err := h.partService.SubmitAppeal(r.Context(), user.ID, claimID, req)
	if err != nil {
		ErrorJSON(w, http.StatusBadRequest, "APPEAL_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusCreated, appeal)
}

func (h *ParticipationHandler) ListMyBadges(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	badges, err := h.partService.ListUserBadges(r.Context(), user.ID)
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, badges)
}

func (h *ParticipationHandler) ReverseClaim(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	claimID := chi.URLParam(r, "id")
	if claimID == "" {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_ID", "Claim ID required", nil)
		return
	}

	var req struct {
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request payload", nil)
		return
	}

	actorRole := user.PrimaryRole
	reversed, err := h.partService.ReverseClaim(r.Context(), user.ID, actorRole, claimID, req.Reason)
	if err != nil {
		if err == service.ErrScopeUnauthorized {
			ErrorJSON(w, http.StatusForbidden, "SCOPE_UNAUTHORIZED", err.Error(), nil)
			return
		}
		if err == service.ErrClaimAlreadyReversed || err == service.ErrClaimNotApproved {
			ErrorJSON(w, http.StatusBadRequest, "INVALID_STATE", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusBadRequest, "REVERSAL_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, reversed)
}

func (h *ParticipationHandler) ListMyClaims(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	claims, err := h.partService.ListClaimsByUser(r.Context(), user.ID)
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, claims)
}
