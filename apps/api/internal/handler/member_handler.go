package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"ratels-api/internal/security"
	"ratels-api/internal/service"
)

type MemberHandler struct {
	authService         *service.AuthService
	verificationLimiter *security.DownloadRateLimiter
}

func NewMemberHandler(authService *service.AuthService) *MemberHandler {
	return &MemberHandler{authService: authService, verificationLimiter: security.NewDownloadRateLimiter(60, time.Minute)}
}

func (h *MemberHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Not authenticated", nil)
		return
	}

	summary, err := h.authService.GetMe(r.Context(), user.ID)
	if err != nil {
		ErrorJSON(w, http.StatusNotFound, "NOT_FOUND", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, summary)
}

func (h *MemberHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Not authenticated", nil)
		return
	}

	var req service.ProfileUpdate
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request payload", nil)
		return
	}

	profile, err := h.authService.EditMemberProfile(r.Context(), user.ID, req)
	if err != nil {
		ErrorJSON(w, http.StatusBadRequest, "UPDATE_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, profile)
}

func (h *MemberHandler) ListMembers(w http.ResponseWriter, r *http.Request) {
	limit := int32(50)
	if lStr := r.URL.Query().Get("limit"); lStr != "" {
		if val, err := strconv.Atoi(lStr); err == nil && val > 0 {
			limit = int32(val)
		}
	}

	page := int32(1)
	if pStr := r.URL.Query().Get("page"); pStr != "" {
		if val, err := strconv.Atoi(pStr); err == nil && val > 0 {
			page = int32(val)
		}
	}

	search := r.URL.Query().Get("search")
	if search == "" {
		search = r.URL.Query().Get("q")
	}

	var members []service.UserSummary
	var err error
	if search != "" {
		members, err = h.authService.SearchMembers(r.Context(), search, page, limit)
	} else {
		members, err = h.authService.ListMembers(r.Context(), page, limit)
	}
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, members)
}
