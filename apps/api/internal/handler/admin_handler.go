package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"ratels-api/internal/service"
)

type AdminHandler struct {
	adminService *service.AdminService
}

func NewAdminHandler(adminService *service.AdminService) *AdminHandler {
	return &AdminHandler{adminService: adminService}
}

func (h *AdminHandler) GetOverview(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	overview, err := h.adminService.GetOperationalOverview(r.Context(), user)
	if err != nil {
		coordinatorError(w, err)
		return
	}
	JSON(w, http.StatusOK, overview)
}

func (h *AdminHandler) GetSuperAdminDashboard(w http.ResponseWriter, r *http.Request) {
	stats, err := h.adminService.GetSuperAdminDashboardStats(r.Context())
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, stats)
}

func (h *AdminHandler) ListZoneCoverage(w http.ResponseWriter, r *http.Request) {
	coverage, err := h.adminService.ListZoneCoverage(r.Context())
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, coverage)
}

func (h *AdminHandler) ListStateCoverage(w http.ResponseWriter, r *http.Request) {
	coverage, err := h.adminService.ListStateCoverage(r.Context())
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, coverage)
}

func (h *AdminHandler) ListLGACoverage(w http.ResponseWriter, r *http.Request) {
	stateID := strings.TrimSpace(r.URL.Query().Get("state_id"))
	coverage, err := h.adminService.ListLGACoverage(r.Context(), stateID)
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, coverage)
}

func (h *AdminHandler) ListAuditLogs(w http.ResponseWriter, r *http.Request) {
	page := int32(1)
	if pStr := r.URL.Query().Get("page"); pStr != "" {
		if val, err := strconv.Atoi(pStr); err == nil && val > 0 {
			page = int32(val)
		}
	}

	limit := int32(20)
	if lStr := r.URL.Query().Get("limit"); lStr != "" {
		if val, err := strconv.Atoi(lStr); err == nil && val > 0 {
			limit = int32(val)
		}
	}

	logs, err := h.adminService.ListAuditLogs(r.Context(), page, limit)
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, logs)
}

func (h *AdminHandler) SeedGeography(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	var actorID, actorRole *string
	if user != nil {
		actorID = &user.ID
		actorRole = &user.PrimaryRole
	}
	ip := r.RemoteAddr
	res, err := h.adminService.SeedGeography(r.Context(), actorID, actorRole, &ip)
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "SEED_ERROR", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, res)
}

func (h *AdminHandler) ListOrganizationSettings(w http.ResponseWriter, r *http.Request) {
	actorRole := GetUserEffectiveRole(r.Context())
	settings, err := h.adminService.ListOrganizationSettings(r.Context(), actorRole)
	if err != nil {
		if err == service.ErrSuperAdminRequired {
			ErrorJSON(w, http.StatusForbidden, "FORBIDDEN", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, settings)
}

type UpdateOrganizationSettingPayload struct {
	Value string `json:"value"`
}

func (h *AdminHandler) UpdateOrganizationSetting(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	key := chi.URLParam(r, "key")
	if key == "" {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_KEY", "Setting key is required", nil)
		return
	}

	var req UpdateOrganizationSettingPayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid JSON payload", nil)
		return
	}

	actorRole := GetUserEffectiveRole(r.Context())
	ip := r.RemoteAddr
	updated, err := h.adminService.UpdateOrganizationSetting(r.Context(), user.ID, actorRole, key, req.Value, &ip)
	if err != nil {
		if err == service.ErrSuperAdminRequired {
			ErrorJSON(w, http.StatusForbidden, "FORBIDDEN", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusBadRequest, "UPDATE_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, updated)
}
