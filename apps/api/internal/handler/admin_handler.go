package handler

import (
	"net/http"
	"strconv"
	"strings"

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
	actorRole := "super_admin"
	actorJurisdictionType := "national"
	actorJurisdictionName := "National Command (All Nigeria)"

	if user != nil {
		actorRole = user.PrimaryRole
		if user.PrimaryRole == "super_admin" {
			actorJurisdictionType = "national"
			actorJurisdictionName = "National Command (All Nigeria)"
		} else if user.PrimaryRole == "national_admin" {
			actorJurisdictionType = "national"
			actorJurisdictionName = "National Secretariat"
		}

		apps := GetCurrentAppointments(r.Context())
		for _, a := range apps {
			if a.Status == "active" {
				actorRole = a.RoleType
				actorJurisdictionType = a.JurisdictionType
				actorJurisdictionName = a.RoleTitle
				break
			}
		}
	}

	overview, err := h.adminService.GetOverview(r.Context(), actorRole, actorJurisdictionType, actorJurisdictionName)
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
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
