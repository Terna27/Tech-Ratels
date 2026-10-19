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

// -----------------------------------------------------------------------------
// Financial Dashboard & Overview Handlers
// -----------------------------------------------------------------------------

func (h *FinanceHandler) GetDashboardSummary(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}
	role := GetUserEffectiveRole(r.Context())

	summary, err := h.financeService.GetFinancialDashboardSummary(r.Context(), user.ID, role)
	if err != nil {
		if err == service.ErrFinancialUnauthorized {
			ErrorJSON(w, http.StatusForbidden, "FINANCIAL_UNAUTHORIZED", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, summary)
}

// -----------------------------------------------------------------------------
// Income Management Handlers
// -----------------------------------------------------------------------------

func (h *FinanceHandler) RecordIncome(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}
	role := GetUserEffectiveRole(r.Context())

	var req service.RecordIncomeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid income payload", nil)
		return
	}

	tx, err := h.financeService.RecordIncome(r.Context(), user.ID, role, req)
	if err != nil {
		if err == service.ErrDuplicateFinancialRecord {
			ErrorJSON(w, http.StatusConflict, "DUPLICATE_RECORD", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusBadRequest, "INCOME_RECORD_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusCreated, tx)
}

func (h *FinanceHandler) VerifyIncome(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}
	role := GetUserEffectiveRole(r.Context())

	txID := chi.URLParam(r, "id")
	if txID == "" {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_ID", "Transaction ID required", nil)
		return
	}

	var req service.VerifyIncomeRequest
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.Decision == "" {
		req.Decision = "verified"
	}

	tx, err := h.financeService.VerifyIncome(r.Context(), user.ID, role, txID, req)
	if err != nil {
		if err == service.ErrSelfApprovalProhibited {
			ErrorJSON(w, http.StatusForbidden, "SELF_APPROVAL_PROHIBITED", err.Error(), nil)
			return
		}
		if err == service.ErrFinancialUnauthorized {
			ErrorJSON(w, http.StatusForbidden, "FINANCIAL_UNAUTHORIZED", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusBadRequest, "VERIFICATION_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, tx)
}

// -----------------------------------------------------------------------------
// Expense Management Handlers
// -----------------------------------------------------------------------------

func (h *FinanceHandler) CreateExpense(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}
	role := GetUserEffectiveRole(r.Context())

	var req service.CreateExpenseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid expense payload", nil)
		return
	}

	exp, err := h.financeService.CreateExpense(r.Context(), user.ID, role, req)
	if err != nil {
		if err == service.ErrBudgetExceeded {
			ErrorJSON(w, http.StatusBadRequest, "BUDGET_EXCEEDED", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusBadRequest, "EXPENSE_CREATION_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusCreated, exp)
}

func (h *FinanceHandler) GetExpense(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	expID := chi.URLParam(r, "id")
	if expID == "" {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_ID", "Expense ID required", nil)
		return
	}

	exp, err := h.financeService.GetExpense(r.Context(), expID)
	if err != nil {
		ErrorJSON(w, http.StatusNotFound, "NOT_FOUND", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, exp)
}

func (h *FinanceHandler) ListExpenses(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	q := r.URL.Query()
	filter := service.ExpenseFilterRequest{}
	if cat := q.Get("category"); cat != "" {
		filter.Category = &cat
	}
	if st := q.Get("status"); st != "" {
		filter.Status = &st
	}
	if z := q.Get("zone_id"); z != "" {
		filter.ZoneID = &z
	}
	if s := q.Get("state_id"); s != "" {
		filter.StateID = &s
	}
	if l := q.Get("lga_id"); l != "" {
		filter.LgaID = &l
	}
	if p := q.Get("related_project_id"); p != "" {
		filter.RelatedProjectID = &p
	}
	if b := q.Get("budget_id"); b != "" {
		filter.BudgetID = &b
	}
	if reqBy := q.Get("requested_by_user_id"); reqBy != "" {
		filter.RequestedByUserID = &reqBy
	}

	expenses, err := h.financeService.ListExpenses(r.Context(), filter)
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, expenses)
}

func (h *FinanceHandler) ReviewExpense(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}
	role := GetUserEffectiveRole(r.Context())

	expID := chi.URLParam(r, "id")
	if expID == "" {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_ID", "Expense ID required", nil)
		return
	}

	var req service.ReviewExpenseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid review payload", nil)
		return
	}

	updated, err := h.financeService.ReviewExpense(r.Context(), user.ID, role, expID, req)
	if err != nil {
		if err == service.ErrSelfApprovalProhibited {
			ErrorJSON(w, http.StatusForbidden, "SELF_APPROVAL_PROHIBITED", err.Error(), nil)
			return
		}
		if err == service.ErrFinancialUnauthorized {
			ErrorJSON(w, http.StatusForbidden, "FINANCIAL_UNAUTHORIZED", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusBadRequest, "REVIEW_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, updated)
}

func (h *FinanceHandler) AuthorizeExpense(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}
	role := GetUserEffectiveRole(r.Context())

	expID := chi.URLParam(r, "id")
	if expID == "" {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_ID", "Expense ID required", nil)
		return
	}

	type AuthBody struct {
		Notes string `json:"notes"`
	}
	var body AuthBody
	_ = json.NewDecoder(r.Body).Decode(&body)

	updated, err := h.financeService.AuthorizeExpense(r.Context(), user.ID, role, expID, body.Notes)
	if err != nil {
		if err == service.ErrFinancialApprovalRestricted {
			ErrorJSON(w, http.StatusForbidden, "FORBIDDEN", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusBadRequest, "AUTHORIZATION_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, updated)
}

func (h *FinanceHandler) RecordExpensePayment(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}
	role := GetUserEffectiveRole(r.Context())

	expID := chi.URLParam(r, "id")
	if expID == "" {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_ID", "Expense ID required", nil)
		return
	}

	var req service.PayExpenseRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	paid, err := h.financeService.RecordExpensePayment(r.Context(), user.ID, role, expID, req)
	if err != nil {
		if err == service.ErrFinancialUnauthorized {
			ErrorJSON(w, http.StatusForbidden, "FINANCIAL_UNAUTHORIZED", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusBadRequest, "PAYMENT_RECORDING_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, paid)
}

// -----------------------------------------------------------------------------
// Budget Management Handlers
// -----------------------------------------------------------------------------

func (h *FinanceHandler) CreateBudget(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}
	role := GetUserEffectiveRole(r.Context())

	var req service.CreateBudgetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid budget payload", nil)
		return
	}

	budget, err := h.financeService.CreateBudget(r.Context(), user.ID, role, req)
	if err != nil {
		ErrorJSON(w, http.StatusBadRequest, "BUDGET_CREATION_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusCreated, budget)
}

func (h *FinanceHandler) GetBudget(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	budgetID := chi.URLParam(r, "id")
	if budgetID == "" {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_ID", "Budget ID required", nil)
		return
	}

	budget, revisions, err := h.financeService.GetBudget(r.Context(), budgetID)
	if err != nil {
		ErrorJSON(w, http.StatusNotFound, "NOT_FOUND", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, map[string]interface{}{
		"budget":    budget,
		"revisions": revisions,
	})
}

func (h *FinanceHandler) ListBudgets(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	q := r.URL.Query()
	filter := service.BudgetFilterRequest{}
	if st := q.Get("scope_type"); st != "" {
		filter.ScopeType = &st
	}
	if sid := q.Get("scope_id"); sid != "" {
		filter.ScopeID = &sid
	}
	if p := q.Get("period_label"); p != "" {
		filter.PeriodLabel = &p
	}
	if s := q.Get("status"); s != "" {
		filter.Status = &s
	}
	if z := q.Get("zone_id"); z != "" {
		filter.ZoneID = &z
	}
	if stId := q.Get("state_id"); stId != "" {
		filter.StateID = &stId
	}

	budgets, err := h.financeService.ListBudgets(r.Context(), filter)
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, budgets)
}

func (h *FinanceHandler) ApproveBudget(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}
	role := GetUserEffectiveRole(r.Context())

	budgetID := chi.URLParam(r, "id")
	if budgetID == "" {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_ID", "Budget ID required", nil)
		return
	}

	approved, err := h.financeService.ApproveBudget(r.Context(), user.ID, role, budgetID)
	if err != nil {
		if err == service.ErrFinancialApprovalRestricted {
			ErrorJSON(w, http.StatusForbidden, "FORBIDDEN", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusBadRequest, "BUDGET_APPROVAL_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, approved)
}

func (h *FinanceHandler) RequestBudgetRevision(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}
	role := GetUserEffectiveRole(r.Context())

	budgetID := chi.URLParam(r, "id")
	if budgetID == "" {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_ID", "Budget ID required", nil)
		return
	}

	var req service.BudgetRevisionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid revision payload", nil)
		return
	}

	revision, err := h.financeService.RequestBudgetRevision(r.Context(), user.ID, role, budgetID, req)
	if err != nil {
		ErrorJSON(w, http.StatusBadRequest, "REVISION_REQUEST_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusCreated, revision)
}

func (h *FinanceHandler) ApproveBudgetRevision(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}
	role := GetUserEffectiveRole(r.Context())

	revID := chi.URLParam(r, "id")
	if revID == "" {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_ID", "Revision ID required", nil)
		return
	}

	approved, err := h.financeService.ApproveBudgetRevision(r.Context(), user.ID, role, revID)
	if err != nil {
		if err == service.ErrFinancialApprovalRestricted {
			ErrorJSON(w, http.StatusForbidden, "FORBIDDEN", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusBadRequest, "REVISION_APPROVAL_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, approved)
}

// -----------------------------------------------------------------------------
// Fund Allocation Handlers
// -----------------------------------------------------------------------------

func (h *FinanceHandler) CreateAllocation(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}
	role := GetUserEffectiveRole(r.Context())

	var req service.CreateAllocationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid allocation payload", nil)
		return
	}

	alloc, err := h.financeService.CreateAllocation(r.Context(), user.ID, role, req)
	if err != nil {
		if err == service.ErrFinancialUnauthorized {
			ErrorJSON(w, http.StatusForbidden, "FINANCIAL_UNAUTHORIZED", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusBadRequest, "ALLOCATION_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusCreated, alloc)
}

func (h *FinanceHandler) ListAllocations(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	q := r.URL.Query()
	filter := service.AllocationFilterRequest{}
	if st := q.Get("source_type"); st != "" {
		filter.SourceType = &st
	}
	if dt := q.Get("destination_type"); dt != "" {
		filter.DestinationType = &dt
	}
	if s := q.Get("status"); s != "" {
		filter.Status = &s
	}

	allocations, err := h.financeService.ListAllocations(r.Context(), filter)
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, allocations)
}

// -----------------------------------------------------------------------------
// Donations & Public Transparency Handlers
// -----------------------------------------------------------------------------

func (h *FinanceHandler) InitiateDonation(w http.ResponseWriter, r *http.Request) {
	var req service.InitiateDonationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid donation payload", nil)
		return
	}

	resp, err := h.financeService.InitiateDonation(r.Context(), req)
	if err != nil {
		ErrorJSON(w, http.StatusBadRequest, "DONATION_INITIATION_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusCreated, resp)
}

func (h *FinanceHandler) VerifyDonation(w http.ResponseWriter, r *http.Request) {
	type VerifyBody struct {
		PaymentReference string `json:"payment_reference"`
	}
	var body VerifyBody
	_ = json.NewDecoder(r.Body).Decode(&body)

	ref := body.PaymentReference
	if ref == "" {
		ref = r.URL.Query().Get("reference")
	}
	if ref == "" {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REFERENCE", "Payment reference is required", nil)
		return
	}

	verified, err := h.financeService.VerifyDonation(r.Context(), ref)
	if err != nil {
		ErrorJSON(w, http.StatusBadRequest, "DONATION_VERIFICATION_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, verified)
}

func (h *FinanceHandler) ListDonations(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	role := "public"
	userID := ""
	if user != nil {
		role = GetUserEffectiveRole(r.Context())
		userID = user.ID
	}

	q := r.URL.Query()
	filter := service.DonationFilterRequest{}
	if et := q.Get("entity_type"); et != "" {
		filter.EntityType = &et
	}
	if c := q.Get("campaign_id"); c != "" {
		filter.CampaignID = &c
	}
	if s := q.Get("status"); s != "" {
		filter.Status = &s
	}
	if z := q.Get("zone_id"); z != "" {
		filter.ZoneID = &z
	}
	if st := q.Get("state_id"); st != "" {
		filter.StateID = &st
	}

	donations, err := h.financeService.ListDonations(r.Context(), userID, role, filter)
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, donations)
}

func (h *FinanceHandler) GetDonation(w http.ResponseWriter, r *http.Request) {
	donID := chi.URLParam(r, "id")
	if donID == "" {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_ID", "Donation ID required", nil)
		return
	}

	don, err := h.financeService.GetDonation(r.Context(), donID)
	if err != nil {
		ErrorJSON(w, http.StatusNotFound, "NOT_FOUND", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, don)
}

func (h *FinanceHandler) GetPublicTransparencySummary(w http.ResponseWriter, r *http.Request) {
	summary, err := h.financeService.GetPublicTransparencySummary(r.Context())
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, summary)
}

// -----------------------------------------------------------------------------
// Bank Accounts Handlers
// -----------------------------------------------------------------------------

func (h *FinanceHandler) CreateBankAccount(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}
	role := GetUserEffectiveRole(r.Context())

	var req service.RegisterBankAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid bank account payload", nil)
		return
	}

	account, err := h.financeService.CreateBankAccount(r.Context(), user.ID, role, req)
	if err != nil {
		ErrorJSON(w, http.StatusBadRequest, "CREATION_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusCreated, account)
}

func (h *FinanceHandler) ActivateBankAccount(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}
	role := GetUserEffectiveRole(r.Context())

	accountID := chi.URLParam(r, "id")
	if accountID == "" {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_ID", "Bank account ID required", nil)
		return
	}

	activated, err := h.financeService.ActivateBankAccount(r.Context(), user.ID, role, accountID)
	if err != nil {
		if err == service.ErrFinancialApprovalRestricted {
			ErrorJSON(w, http.StatusForbidden, "FORBIDDEN", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusBadRequest, "ACTIVATION_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, activated)
}

func (h *FinanceHandler) ListBankAccounts(w http.ResponseWriter, r *http.Request) {
	role := "public"
	user := GetCurrentUser(r.Context())
	if user != nil {
		role = GetUserEffectiveRole(r.Context())
	}

	var orgType *string
	if o := r.URL.Query().Get("org_type"); o != "" {
		orgType = &o
	}

	accounts, err := h.financeService.ListBankAccounts(r.Context(), role, orgType)
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, accounts)
}

// -----------------------------------------------------------------------------
// Bank Reconciliation Handlers
// -----------------------------------------------------------------------------

func (h *FinanceHandler) CreateReconciliationPeriod(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}
	role := GetUserEffectiveRole(r.Context())

	var req service.CreateReconciliationPeriodRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid period payload", nil)
		return
	}

	period, err := h.financeService.CreateReconciliationPeriod(r.Context(), user.ID, role, req)
	if err != nil {
		if err == service.ErrFinancialUnauthorized {
			ErrorJSON(w, http.StatusForbidden, "FINANCIAL_UNAUTHORIZED", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusBadRequest, "CREATION_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusCreated, period)
}

func (h *FinanceHandler) GetReconciliationPeriod(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	periodID := chi.URLParam(r, "id")
	if periodID == "" {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_ID", "Period ID required", nil)
		return
	}

	period, items, err := h.financeService.GetReconciliationPeriod(r.Context(), periodID)
	if err != nil {
		ErrorJSON(w, http.StatusNotFound, "NOT_FOUND", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, map[string]interface{}{
		"period":          period,
		"statement_items": items,
	})
}

func (h *FinanceHandler) ListReconciliationPeriods(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	q := r.URL.Query()
	var acctID *string
	if a := q.Get("bank_account_id"); a != "" {
		acctID = &a
	}
	var st *string
	if s := q.Get("status"); s != "" {
		st = &s
	}

	periods, err := h.financeService.ListReconciliationPeriods(r.Context(), acctID, st)
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, periods)
}

func (h *FinanceHandler) ImportBankStatementCSV(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}
	role := GetUserEffectiveRole(r.Context())

	periodID := chi.URLParam(r, "id")
	if periodID == "" {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_ID", "Period ID required", nil)
		return
	}

	// Support both multipart file upload and raw CSV body
	var reader = r.Body
	if file, _, err := r.FormFile("file"); err == nil {
		defer file.Close()
		reader = file
	}

	imported, err := h.financeService.ImportBankStatementCSV(r.Context(), user.ID, role, periodID, reader)
	if err != nil {
		if err == service.ErrFinancialUnauthorized {
			ErrorJSON(w, http.StatusForbidden, "FINANCIAL_UNAUTHORIZED", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusBadRequest, "IMPORT_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, map[string]interface{}{
		"message":        fmt.Sprintf("Successfully imported %d statement items", imported),
		"imported_count": imported,
	})
}

func (h *FinanceHandler) AutoMatchStatement(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}
	role := GetUserEffectiveRole(r.Context())

	periodID := chi.URLParam(r, "id")
	if periodID == "" {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_ID", "Period ID required", nil)
		return
	}

	matched, err := h.financeService.AutoMatchStatementItems(r.Context(), user.ID, role, periodID)
	if err != nil {
		ErrorJSON(w, http.StatusBadRequest, "MATCHING_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, map[string]interface{}{
		"message":       fmt.Sprintf("Auto-matched %d statement items", matched),
		"matched_count": matched,
	})
}

func (h *FinanceHandler) ManualMatchStatementItem(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}
	role := GetUserEffectiveRole(r.Context())

	itemID := chi.URLParam(r, "id")
	if itemID == "" {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_ID", "Item ID required", nil)
		return
	}

	type MatchBody struct {
		TransactionID string `json:"transaction_id"`
		Notes         string `json:"notes"`
	}
	var body MatchBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid match payload", nil)
		return
	}

	item, err := h.financeService.ManualMatchStatementItem(r.Context(), user.ID, role, itemID, body.TransactionID, body.Notes)
	if err != nil {
		ErrorJSON(w, http.StatusBadRequest, "MANUAL_MATCH_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, item)
}

func (h *FinanceHandler) CertifyReconciliation(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}
	role := GetUserEffectiveRole(r.Context())

	periodID := chi.URLParam(r, "id")
	if periodID == "" {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_ID", "Period ID required", nil)
		return
	}

	type CertBody struct {
		Notes string `json:"notes"`
	}
	var body CertBody
	_ = json.NewDecoder(r.Body).Decode(&body)

	certified, err := h.financeService.CertifyReconciliation(r.Context(), user.ID, role, periodID, body.Notes)
	if err != nil {
		if err == service.ErrFinancialApprovalRestricted {
			ErrorJSON(w, http.StatusForbidden, "FORBIDDEN", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusBadRequest, "CERTIFICATION_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, certified)
}

// -----------------------------------------------------------------------------
// Financial Documentation & Printable Handlers
// -----------------------------------------------------------------------------

func (h *FinanceHandler) GetFinancialDocument(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}
	role := GetUserEffectiveRole(r.Context())

	docType := chi.URLParam(r, "type")
	docID := chi.URLParam(r, "id")

	doc, err := h.financeService.GenerateDocument(r.Context(), user.ID, role, docType, docID)
	if err != nil {
		ErrorJSON(w, http.StatusBadRequest, "DOCUMENT_GENERATION_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, doc)
}

func (h *FinanceHandler) GetDocumentPrintableHTML(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}
	role := GetUserEffectiveRole(r.Context())

	docType := chi.URLParam(r, "type")
	docID := chi.URLParam(r, "id")

	doc, err := h.financeService.GenerateDocument(r.Context(), user.ID, role, docType, docID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(doc.HTMLPrintableBody))
}
