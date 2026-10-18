package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"ratels-api/internal/service"
)

type FoundationHandler struct {
	foundService *service.FoundationService
}

func NewFoundationHandler(foundService *service.FoundationService) *FoundationHandler {
	return &FoundationHandler{foundService: foundService}
}

func (h *FoundationHandler) GetPublicInfo(w http.ResponseWriter, r *http.Request) {
	info, err := h.foundService.GetPublicInfo(r.Context())
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}
	JSON(w, http.StatusOK, info)
}

func (h *FoundationHandler) ListBankAccounts(w http.ResponseWriter, r *http.Request) {
	accounts, err := h.foundService.ListBankAccountsAdmin(r.Context())
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}
	JSON(w, http.StatusOK, accounts)
}

func (h *FoundationHandler) CreateBankAccount(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	var req service.CreateBankAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request payload", nil)
		return
	}

	created, err := h.foundService.CreateBankAccount(r.Context(), user.ID, user.DisplayName, req)
	if err != nil {
		ErrorJSON(w, http.StatusBadRequest, "CREATION_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusCreated, created)
}

func (h *FoundationHandler) VerifyBankAccount(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	accountID := chi.URLParam(r, "id")
	if accountID == "" {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_ID", "Account ID required", nil)
		return
	}

	var req service.VerifyBankAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request payload", nil)
		return
	}

	if req.OfficerName == "" {
		req.OfficerName = user.DisplayName
	}

	updated, err := h.foundService.VerifyBankAccount(r.Context(), user.ID, accountID, req)
	if err != nil {
		ErrorJSON(w, http.StatusBadRequest, "VERIFICATION_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, updated)
}
