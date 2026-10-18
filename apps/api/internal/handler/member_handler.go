package handler

import (
	"encoding/json"
	"net/http"

	"ratels-api/internal/service"
)

type MemberHandler struct {
	authService *service.AuthService
}

func NewMemberHandler(authService *service.AuthService) *MemberHandler {
	return &MemberHandler{authService: authService}
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

type UpdateProfilePayload struct {
	HomeStateID *string `json:"home_state_id"`
	HomeLGAID   *string `json:"home_lga_id"`
	Address     *string `json:"address"`
	Bio         *string `json:"bio"`
	AvatarURL   *string `json:"avatar_url"`
}

func (h *MemberHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Not authenticated", nil)
		return
	}

	var req UpdateProfilePayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request payload", nil)
		return
	}

	profile, err := h.authService.UpdateProfile(r.Context(), user.ID, req.HomeStateID, req.HomeLGAID, req.Address, req.Bio, req.AvatarURL)
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "UPDATE_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, profile)
}
