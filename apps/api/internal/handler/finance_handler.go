package handler

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"ratels-api/internal/service"
)

type FinanceHandler struct {
	financeService *service.FinanceService
}

func NewFinanceHandler(financeService *service.FinanceService) *FinanceHandler {
	return &FinanceHandler{financeService: financeService}
}

func (h *FinanceHandler) checkAccess(r *http.Request) (*dbUserWithRole, error) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		return nil, service.ErrFinancialUnauthorized
	}

	role := GetUserEffectiveRole(r.Context())
	hasAccess, _, err := h.financeService.CheckFinancialAccess(r.Context(), user.ID, role)
	if err != nil || !hasAccess {
		return nil, service.ErrFinancialUnauthorized
	}

	return &dbUserWithRole{User: *user, EffectiveRole: role}, nil
}

type dbUserWithRole struct {
	User          interface{}
	EffectiveRole string
}

func (h *FinanceHandler) ListFunds(w http.ResponseWriter, r *http.Request) {
	funds, err := h.financeService.ListFunds(r.Context())
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}
	JSON(w, http.StatusOK, funds)
}

func (h *FinanceHandler) ListTransactions(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	role := GetUserEffectiveRole(r.Context())
	hasAccess, _, _ := h.financeService.CheckFinancialAccess(r.Context(), user.ID, role)
	if !hasAccess {
		ErrorJSON(w, http.StatusForbidden, "FINANCIAL_UNAUTHORIZED", "Super Admin authority or authorized staff clearance required", nil)
		return
	}

	q := r.URL.Query()
	req := service.FinancialFilterRequest{}

	if startStr := q.Get("start_date"); startStr != "" {
		if t, err := time.Parse(time.RFC3339, startStr); err == nil {
			req.StartDate = &t
		}
	}
	if endStr := q.Get("end_date"); endStr != "" {
		if t, err := time.Parse(time.RFC3339, endStr); err == nil {
			req.EndDate = &t
		}
	}
	if stateID := q.Get("state_id"); stateID != "" {
		req.StateID = &stateID
	}
	if zoneID := q.Get("zone_id"); zoneID != "" {
		req.ZoneID = &zoneID
	}
	if campaignID := q.Get("campaign_id"); campaignID != "" {
		req.CampaignID = &campaignID
	}
	if projectID := q.Get("project_id"); projectID != "" {
		req.ProjectID = &projectID
	}
	if txType := q.Get("transaction_type"); txType != "" {
		req.TransactionType = &txType
	}
	if status := q.Get("status"); status != "" {
		req.Status = &status
	}
	if fundID := q.Get("fund_id"); fundID != "" {
		req.FundID = &fundID
	}

	transactions, err := h.financeService.ListTransactions(r.Context(), req)
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, transactions)
}

func (h *FinanceHandler) GetTransaction(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	role := GetUserEffectiveRole(r.Context())
	hasAccess, _, _ := h.financeService.CheckFinancialAccess(r.Context(), user.ID, role)
	if !hasAccess {
		ErrorJSON(w, http.StatusForbidden, "FINANCIAL_UNAUTHORIZED", "Super Admin authority or authorized staff clearance required", nil)
		return
	}

	txID := chi.URLParam(r, "id")
	if txID == "" {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_ID", "Transaction ID required", nil)
		return
	}

	tx, entries, err := h.financeService.GetTransaction(r.Context(), txID)
	if err != nil {
		ErrorJSON(w, http.StatusNotFound, "NOT_FOUND", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, map[string]interface{}{
		"transaction":    tx,
		"ledger_entries": entries,
	})
}

func (h *FinanceHandler) RecordTransaction(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	role := GetUserEffectiveRole(r.Context())
	hasAccess, accessLevel, _ := h.financeService.CheckFinancialAccess(r.Context(), user.ID, role)
	if !hasAccess {
		ErrorJSON(w, http.StatusForbidden, "FINANCIAL_UNAUTHORIZED", "Clearance required to submit financial records", nil)
		return
	}

	var req service.CreateTransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request payload", nil)
		return
	}

	// Staff submitters can only submit pending drafts; only Super Admin can auto-post
	if accessLevel != "super_admin" {
		req.AutoPost = false
	}

	tx, err := h.financeService.RecordTransaction(r.Context(), user.ID, role, req)
	if err != nil {
		if err == service.ErrDuplicateFinancialRecord {
			ErrorJSON(w, http.StatusConflict, "DUPLICATE_RECORD", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusBadRequest, "TRANSACTION_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusCreated, tx)
}

func (h *FinanceHandler) ApproveTransaction(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	role := GetUserEffectiveRole(r.Context())
	if role != "super_admin" {
		ErrorJSON(w, http.StatusForbidden, "FORBIDDEN", "Only Super Admin may approve or finalize financial documentation", nil)
		return
	}

	txID := chi.URLParam(r, "id")
	if txID == "" {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_ID", "Transaction ID required", nil)
		return
	}

	approved, err := h.financeService.ApproveTransaction(r.Context(), user.ID, role, txID)
	if err != nil {
		ErrorJSON(w, http.StatusBadRequest, "APPROVAL_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, approved)
}

func (h *FinanceHandler) ReverseTransaction(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	role := GetUserEffectiveRole(r.Context())
	if role != "super_admin" {
		ErrorJSON(w, http.StatusForbidden, "FORBIDDEN", "Only Super Admin may authorize financial corrections and reversals", nil)
		return
	}

	txID := chi.URLParam(r, "id")
	if txID == "" {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_ID", "Transaction ID required", nil)
		return
	}

	var req service.ReverseTransactionRequest
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.Reason == "" {
		req.Reason = "Corrective reversal authorized by Super Admin"
	}

	reversal, err := h.financeService.ReverseTransaction(r.Context(), user.ID, role, txID, req.Reason)
	if err != nil {
		ErrorJSON(w, http.StatusBadRequest, "REVERSAL_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, reversal)
}

func (h *FinanceHandler) GenerateStatement(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	role := GetUserEffectiveRole(r.Context())
	hasAccess, _, _ := h.financeService.CheckFinancialAccess(r.Context(), user.ID, role)
	if !hasAccess {
		ErrorJSON(w, http.StatusForbidden, "FINANCIAL_UNAUTHORIZED", "Super Admin authority or authorized staff clearance required", nil)
		return
	}

	q := r.URL.Query()
	req := service.FinancialFilterRequest{}

	if startStr := q.Get("start_date"); startStr != "" {
		if t, err := time.Parse(time.RFC3339, startStr); err == nil {
			req.StartDate = &t
		}
	}
	if endStr := q.Get("end_date"); endStr != "" {
		if t, err := time.Parse(time.RFC3339, endStr); err == nil {
			req.EndDate = &t
		}
	}
	if stateID := q.Get("state_id"); stateID != "" {
		req.StateID = &stateID
	}
	if zoneID := q.Get("zone_id"); zoneID != "" {
		req.ZoneID = &zoneID
	}
	if fundID := q.Get("fund_id"); fundID != "" {
		req.FundID = &fundID
	}

	statement, err := h.financeService.GenerateFinancialStatement(r.Context(), user.ID, user.DisplayName, req)
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "STATEMENT_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, statement)
}

func (h *FinanceHandler) ExportCSV(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	role := GetUserEffectiveRole(r.Context())
	hasAccess, _, _ := h.financeService.CheckFinancialAccess(r.Context(), user.ID, role)
	if !hasAccess {
		ErrorJSON(w, http.StatusForbidden, "FINANCIAL_UNAUTHORIZED", "Super Admin authority or authorized staff clearance required", nil)
		return
	}

	q := r.URL.Query()
	req := service.FinancialFilterRequest{}
	if startStr := q.Get("start_date"); startStr != "" {
		if t, err := time.Parse(time.RFC3339, startStr); err == nil {
			req.StartDate = &t
		}
	}
	if endStr := q.Get("end_date"); endStr != "" {
		if t, err := time.Parse(time.RFC3339, endStr); err == nil {
			req.EndDate = &t
		}
	}
	if fundID := q.Get("fund_id"); fundID != "" {
		req.FundID = &fundID
	}

	transactions, err := h.financeService.ListTransactions(r.Context(), req)
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "EXPORT_FAILED", err.Error(), nil)
		return
	}

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=ratels-financial-records-%d.csv", time.Now().Unix()))

	writer := csv.NewWriter(w)
	defer writer.Flush()

	// Write CSV Header
	_ = writer.Write([]string{
		"Reference",
		"Type",
		"Status",
		"Amount_Kobo",
		"Amount_Formatted",
		"Currency",
		"Source_Fund",
		"Destination_Fund",
		"State",
		"Zone",
		"Provider_Ref",
		"Occurred_At",
		"Recorded_At",
		"Approved_By",
		"Notes",
	})

	for _, tx := range transactions {
		amountFormatted := fmt.Sprintf("%.2f", float64(tx.Amount)/100.0)
		sourceFund := ""
		if tx.SourceFundName.Valid {
			sourceFund = tx.SourceFundName.String
		}
		destFund := ""
		if tx.DestinationFundName.Valid {
			destFund = tx.DestinationFundName.String
		}
		state := ""
		if tx.StateName.Valid {
			state = tx.StateName.String
		}
		zone := ""
		if tx.ZoneName.Valid {
			zone = tx.ZoneName.String
		}
		provRef := ""
		if tx.ProviderReference.Valid {
			provRef = tx.ProviderReference.String
		}
		apprBy := ""
		if tx.ApprovedByName.Valid {
			apprBy = tx.ApprovedByName.String
		}
		notes := ""
		if tx.Notes.Valid {
			notes = tx.Notes.String
		}

		_ = writer.Write([]string{
			tx.Reference,
			tx.TransactionType,
			tx.Status,
			fmt.Sprintf("%d", tx.Amount),
			amountFormatted,
			tx.Currency,
			sourceFund,
			destFund,
			state,
			zone,
			provRef,
			tx.OccurredAt.Time.Format(time.RFC3339),
			tx.RecordedAt.Time.Format(time.RFC3339),
			apprBy,
			notes,
		})
	}
}

type AuthorizeStaffPayload struct {
	UserID      string `json:"user_id"`
	AccessLevel string `json:"access_level"`
	Notes       string `json:"notes"`
}

func (h *FinanceHandler) AuthorizeStaff(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	role := GetUserEffectiveRole(r.Context())
	if role != "super_admin" {
		ErrorJSON(w, http.StatusForbidden, "FORBIDDEN", "Only Super Admin may authorize financial access", nil)
		return
	}

	var req AuthorizeStaffPayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request payload", nil)
		return
	}

	auth, err := h.financeService.AuthorizeStaffAccess(r.Context(), user.ID, role, req.UserID, req.AccessLevel, req.Notes)
	if err != nil {
		ErrorJSON(w, http.StatusBadRequest, "AUTHORIZATION_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusCreated, auth)
}

func (h *FinanceHandler) RevokeStaff(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	role := GetUserEffectiveRole(r.Context())
	if role != "super_admin" {
		ErrorJSON(w, http.StatusForbidden, "FORBIDDEN", "Only Super Admin may revoke financial access", nil)
		return
	}

	authID := chi.URLParam(r, "id")
	if authID == "" {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_ID", "Authorization ID required", nil)
		return
	}

	err := h.financeService.RevokeStaffAccess(r.Context(), user.ID, role, authID)
	if err != nil {
		ErrorJSON(w, http.StatusBadRequest, "REVOCATION_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, map[string]string{
		"message": "Financial authorization revoked successfully",
	})
}

func (h *FinanceHandler) ListAuthorizations(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	role := GetUserEffectiveRole(r.Context())
	if role != "super_admin" {
		ErrorJSON(w, http.StatusForbidden, "FORBIDDEN", "Only Super Admin may view financial authorizations", nil)
		return
	}

	authorizations, err := h.financeService.ListAuthorizations(r.Context())
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, authorizations)
}
