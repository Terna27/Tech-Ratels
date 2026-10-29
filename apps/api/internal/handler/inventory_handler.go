package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"ratels-api/internal/db"
	"ratels-api/internal/service"
)

type InventoryHandler struct {
	inventoryService *service.InventoryService
}

func NewInventoryHandler(inventoryService *service.InventoryService) *InventoryHandler {
	return &InventoryHandler{inventoryService: inventoryService}
}

// -------------------------------------------------------------
// Inventory Catalogue
// -------------------------------------------------------------

func (h *InventoryHandler) ListItems(w http.ResponseWriter, r *http.Request) {
	var category *string
	if cat := r.URL.Query().Get("category"); cat != "" {
		category = &cat
	}
	var isActive *bool
	if activeStr := r.URL.Query().Get("is_active"); activeStr != "" {
		b := activeStr == "true" || activeStr == "1"
		isActive = &b
	}

	items, err := h.inventoryService.ListItems(r.Context(), category, isActive)
	if err != nil {
		if inventoryAuthorizationError(w, err) {
			return
		}
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}
	if items == nil {
		items = []db.InventoryItem{}
	}
	JSON(w, http.StatusOK, items)
}

func (h *InventoryHandler) CreateItem(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	var req service.CreateItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Malformed JSON body", nil)
		return
	}

	item, err := h.inventoryService.CreateItem(r.Context(), user, req)
	if err != nil {
		if inventoryAuthorizationError(w, err) {
			return
		}
		ErrorJSON(w, http.StatusBadRequest, "CREATION_FAILED", err.Error(), nil)
		return
	}
	JSON(w, http.StatusCreated, item)
}

func (h *InventoryHandler) GetItem(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	item, err := h.inventoryService.GetItem(r.Context(), id)
	if err != nil {
		if inventoryAuthorizationError(w, err) {
			return
		}
		if errors.Is(err, service.ErrItemNotFound) {
			ErrorJSON(w, http.StatusNotFound, "NOT_FOUND", "Inventory item not found", nil)
			return
		}
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}
	JSON(w, http.StatusOK, item)
}

func (h *InventoryHandler) UpdateItem(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	id := chi.URLParam(r, "id")
	var req service.UpdateItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Malformed JSON body", nil)
		return
	}

	item, err := h.inventoryService.UpdateItem(r.Context(), user, id, req)
	if err != nil {
		if inventoryAuthorizationError(w, err) {
			return
		}
		if errors.Is(err, service.ErrItemNotFound) {
			ErrorJSON(w, http.StatusNotFound, "NOT_FOUND", "Inventory item not found", nil)
			return
		}
		ErrorJSON(w, http.StatusBadRequest, "UPDATE_FAILED", err.Error(), nil)
		return
	}
	JSON(w, http.StatusOK, item)
}

// -------------------------------------------------------------
// Storage Locations / Warehouses
// -------------------------------------------------------------

func (h *InventoryHandler) ListLocations(w http.ResponseWriter, r *http.Request) {
	var locType, zoneID, stateID, lgaID *string
	if lt := r.URL.Query().Get("location_type"); lt != "" {
		locType = &lt
	}
	if zid := r.URL.Query().Get("zone_id"); zid != "" {
		zoneID = &zid
	}
	if sid := r.URL.Query().Get("state_id"); sid != "" {
		stateID = &sid
	}
	if lid := r.URL.Query().Get("lga_id"); lid != "" {
		lgaID = &lid
	}
	var isActive *bool
	if activeStr := r.URL.Query().Get("is_active"); activeStr != "" {
		b := activeStr == "true" || activeStr == "1"
		isActive = &b
	}

	locations, err := h.inventoryService.ListLocations(r.Context(), locType, zoneID, stateID, lgaID, isActive)
	if err != nil {
		if inventoryAuthorizationError(w, err) {
			return
		}
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}
	if locations == nil {
		locations = []db.ListStorageLocationsRow{}
	}
	JSON(w, http.StatusOK, locations)
}

func (h *InventoryHandler) CreateLocation(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	var req service.CreateLocationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Malformed JSON body", nil)
		return
	}

	loc, err := h.inventoryService.CreateLocation(r.Context(), user, req)
	if err != nil {
		if inventoryAuthorizationError(w, err) {
			return
		}
		ErrorJSON(w, http.StatusBadRequest, "CREATION_FAILED", err.Error(), nil)
		return
	}
	JSON(w, http.StatusCreated, loc)
}

func (h *InventoryHandler) GetLocation(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	loc, err := h.inventoryService.GetLocation(r.Context(), id)
	if err != nil {
		if inventoryAuthorizationError(w, err) {
			return
		}
		if errors.Is(err, service.ErrLocationNotFound) {
			ErrorJSON(w, http.StatusNotFound, "NOT_FOUND", "Storage location not found", nil)
			return
		}
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}
	JSON(w, http.StatusOK, loc)
}

// -------------------------------------------------------------
// Stock Balances & Initial Receipts
// -------------------------------------------------------------

func (h *InventoryHandler) ListBalances(w http.ResponseWriter, r *http.Request) {
	var locID, itemID, category *string
	if lid := r.URL.Query().Get("location_id"); lid != "" {
		locID = &lid
	}
	if itm := r.URL.Query().Get("item_id"); itm != "" {
		itemID = &itm
	}
	if cat := r.URL.Query().Get("category"); cat != "" {
		category = &cat
	}

	balances, err := h.inventoryService.ListStockBalances(r.Context(), locID, itemID, category)
	if err != nil {
		if inventoryAuthorizationError(w, err) {
			return
		}
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}
	if balances == nil {
		balances = []db.ListAllStockBalancesRow{}
	}
	JSON(w, http.StatusOK, balances)
}

func (h *InventoryHandler) ReceiveStock(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	var req service.ReceiveStockRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Malformed JSON body", nil)
		return
	}

	bal, err := h.inventoryService.ReceiveStock(r.Context(), user, req)
	if err != nil {
		if inventoryAuthorizationError(w, err) {
			return
		}
		if errors.Is(err, service.ErrLocationNotFound) || errors.Is(err, service.ErrItemNotFound) {
			ErrorJSON(w, http.StatusNotFound, "NOT_FOUND", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusBadRequest, "RECEIVE_STOCK_FAILED", err.Error(), nil)
		return
	}
	JSON(w, http.StatusOK, bal)
}

func (h *InventoryHandler) ListLowStockAlerts(w http.ResponseWriter, r *http.Request) {
	alerts, err := h.inventoryService.ListLowStockAlerts(r.Context())
	if err != nil {
		if inventoryAuthorizationError(w, err) {
			return
		}
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}
	if alerts == nil {
		alerts = []db.ListLowStockAlertsRow{}
	}
	JSON(w, http.StatusOK, alerts)
}

// -------------------------------------------------------------
// Material Requests
// -------------------------------------------------------------

func (h *InventoryHandler) ListRequests(w http.ResponseWriter, r *http.Request) {
	var status, destLoc, userParam, jurType *string
	if st := r.URL.Query().Get("status"); st != "" {
		status = &st
	}
	if dl := r.URL.Query().Get("destination_location_id"); dl != "" {
		destLoc = &dl
	}
	if u := r.URL.Query().Get("requesting_user_id"); u != "" {
		userParam = &u
	}
	if jt := r.URL.Query().Get("jurisdiction_type"); jt != "" {
		jurType = &jt
	}

	requests, err := h.inventoryService.ListMaterialRequests(r.Context(), status, destLoc, userParam, jurType)
	if err != nil {
		if inventoryAuthorizationError(w, err) {
			return
		}
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}
	if requests == nil {
		requests = []db.ListMaterialRequestsRow{}
	}
	JSON(w, http.StatusOK, requests)
}

func (h *InventoryHandler) CreateRequest(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	var req service.CreateMaterialRequestInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Malformed JSON body", nil)
		return
	}

	created, err := h.inventoryService.CreateMaterialRequest(r.Context(), user, req)
	if err != nil {
		if inventoryAuthorizationError(w, err) {
			return
		}
		if errors.Is(err, service.ErrLocationNotFound) {
			ErrorJSON(w, http.StatusNotFound, "NOT_FOUND", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusBadRequest, "REQUEST_CREATION_FAILED", err.Error(), nil)
		return
	}
	JSON(w, http.StatusCreated, created)
}

func (h *InventoryHandler) GetRequest(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	req, err := h.inventoryService.GetMaterialRequest(r.Context(), id)
	if err != nil {
		if inventoryAuthorizationError(w, err) {
			return
		}
		if errors.Is(err, service.ErrRequestNotFound) {
			ErrorJSON(w, http.StatusNotFound, "NOT_FOUND", "Material request not found", nil)
			return
		}
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}

	items, _ := h.inventoryService.ListMaterialRequestItems(r.Context(), id)
	response := map[string]interface{}{
		"request": req,
		"items":   items,
	}
	JSON(w, http.StatusOK, response)
}

func (h *InventoryHandler) ReviewRequest(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	apps := GetCurrentAppointments(r.Context())
	id := chi.URLParam(r, "id")

	var input service.ReviewMaterialRequestInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Malformed JSON body", nil)
		return
	}

	reviewed, err := h.inventoryService.ReviewMaterialRequest(r.Context(), user, apps, id, input)
	if err != nil {
		if inventoryAuthorizationError(w, err) {
			return
		}
		if errors.Is(err, service.ErrAntiSelfApproval) {
			ErrorJSON(w, http.StatusForbidden, "ANTI_SELF_APPROVAL", err.Error(), nil)
			return
		}
		if errors.Is(err, service.ErrJurisdictionMismatch) {
			ErrorJSON(w, http.StatusForbidden, "JURISDICTION_MISMATCH", err.Error(), nil)
			return
		}
		if errors.Is(err, service.ErrRequestNotFound) {
			ErrorJSON(w, http.StatusNotFound, "NOT_FOUND", err.Error(), nil)
			return
		}
		if errors.Is(err, service.ErrInvalidRequestStatus) {
			ErrorJSON(w, http.StatusConflict, "INVALID_STATUS", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusBadRequest, "REVIEW_FAILED", err.Error(), nil)
		return
	}
	JSON(w, http.StatusOK, reviewed)
}

type allocateRequestPayload struct {
	SupplyingLocationID string `json:"supplying_location_id"`
}

func (h *InventoryHandler) AllocateRequest(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	id := chi.URLParam(r, "id")

	var payload allocateRequestPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Malformed JSON body", nil)
		return
	}
	if payload.SupplyingLocationID == "" {
		ErrorJSON(w, http.StatusBadRequest, "VALIDATION_ERROR", "supplying_location_id is required", nil)
		return
	}

	err := h.inventoryService.AllocateAndReserveStock(r.Context(), user, id, payload.SupplyingLocationID)
	if err != nil {
		if inventoryAuthorizationError(w, err) {
			return
		}
		if errors.Is(err, service.ErrInsufficientStock) {
			ErrorJSON(w, http.StatusConflict, "INSUFFICIENT_STOCK", err.Error(), nil)
			return
		}
		if errors.Is(err, service.ErrRequestNotFound) {
			ErrorJSON(w, http.StatusNotFound, "NOT_FOUND", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusBadRequest, "ALLOCATION_FAILED", err.Error(), nil)
		return
	}

	updated, err := h.inventoryService.GetMaterialRequest(r.Context(), id)
	if err != nil {
		if inventoryAuthorizationError(w, err) {
			return
		}
		JSON(w, http.StatusOK, map[string]string{"status": "allocated"})
		return
	}
	JSON(w, http.StatusOK, updated)
}

// -------------------------------------------------------------
// Material Transfers & Dispatches
// -------------------------------------------------------------

func (h *InventoryHandler) ListTransfers(w http.ResponseWriter, r *http.Request) {
	var status, src, dst *string
	if st := r.URL.Query().Get("status"); st != "" {
		status = &st
	}
	if s := r.URL.Query().Get("source_location_id"); s != "" {
		src = &s
	}
	if d := r.URL.Query().Get("destination_location_id"); d != "" {
		dst = &d
	}

	transfers, err := h.inventoryService.ListTransfers(r.Context(), status, src, dst)
	if err != nil {
		if inventoryAuthorizationError(w, err) {
			return
		}
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}
	if transfers == nil {
		transfers = []db.ListMaterialTransfersRow{}
	}
	JSON(w, http.StatusOK, transfers)
}

func (h *InventoryHandler) CreateTransfer(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	var input service.CreateTransferInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Malformed JSON body", nil)
		return
	}

	transfer, err := h.inventoryService.CreateTransfer(r.Context(), user, input)
	if err != nil {
		if inventoryAuthorizationError(w, err) {
			return
		}
		if errors.Is(err, service.ErrSameLocationTransfer) {
			ErrorJSON(w, http.StatusBadRequest, "INVALID_LOCATION", err.Error(), nil)
			return
		}
		if errors.Is(err, service.ErrInsufficientStock) {
			ErrorJSON(w, http.StatusConflict, "INSUFFICIENT_STOCK", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusBadRequest, "TRANSFER_FAILED", err.Error(), nil)
		return
	}
	JSON(w, http.StatusCreated, transfer)
}

func (h *InventoryHandler) GetTransfer(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	t, err := h.inventoryService.GetTransfer(r.Context(), id)
	if err != nil {
		if inventoryAuthorizationError(w, err) {
			return
		}
		if errors.Is(err, service.ErrTransferNotFound) {
			ErrorJSON(w, http.StatusNotFound, "NOT_FOUND", "Material transfer not found", nil)
			return
		}
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}

	items, _ := h.inventoryService.ListTransferItems(r.Context(), id)
	discrepancies, _ := h.inventoryService.ListDiscrepancies(r.Context(), nil, &id)

	response := map[string]interface{}{
		"transfer":      t,
		"items":         items,
		"discrepancies": discrepancies,
	}
	JSON(w, http.StatusOK, response)
}

func (h *InventoryHandler) DispatchTransfer(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	id := chi.URLParam(r, "id")
	var req service.DispatchTransferInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// Empty body is acceptable for dispatch
		req = service.DispatchTransferInput{}
	}

	dispatched, err := h.inventoryService.DispatchTransfer(r.Context(), user, id, req)
	if err != nil {
		if inventoryAuthorizationError(w, err) {
			return
		}
		if errors.Is(err, service.ErrTransferNotFound) {
			ErrorJSON(w, http.StatusNotFound, "NOT_FOUND", err.Error(), nil)
			return
		}
		if errors.Is(err, service.ErrInsufficientStock) {
			ErrorJSON(w, http.StatusConflict, "INSUFFICIENT_STOCK", err.Error(), nil)
			return
		}
		if errors.Is(err, service.ErrInvalidTransferStatus) {
			ErrorJSON(w, http.StatusConflict, "INVALID_STATUS", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusBadRequest, "DISPATCH_FAILED", err.Error(), nil)
		return
	}
	JSON(w, http.StatusOK, dispatched)
}

func (h *InventoryHandler) ConfirmReceipt(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	id := chi.URLParam(r, "id")
	var req service.ConfirmReceiptInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Malformed JSON body", nil)
		return
	}

	received, err := h.inventoryService.ConfirmTransferReceipt(r.Context(), user, id, req)
	if err != nil {
		if inventoryAuthorizationError(w, err) {
			return
		}
		if errors.Is(err, service.ErrTransferNotFound) {
			ErrorJSON(w, http.StatusNotFound, "NOT_FOUND", err.Error(), nil)
			return
		}
		if errors.Is(err, service.ErrInvalidTransferStatus) {
			ErrorJSON(w, http.StatusConflict, "INVALID_STATUS", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusBadRequest, "RECEIPT_FAILED", err.Error(), nil)
		return
	}
	JSON(w, http.StatusOK, received)
}

// -------------------------------------------------------------
// Discrepancies
// -------------------------------------------------------------

func (h *InventoryHandler) ListDiscrepancies(w http.ResponseWriter, r *http.Request) {
	var status, transferID *string
	if st := r.URL.Query().Get("status"); st != "" {
		status = &st
	}
	if tx := r.URL.Query().Get("transfer_id"); tx != "" {
		transferID = &tx
	}

	discrepancies, err := h.inventoryService.ListDiscrepancies(r.Context(), status, transferID)
	if err != nil {
		if inventoryAuthorizationError(w, err) {
			return
		}
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}
	if discrepancies == nil {
		discrepancies = []db.ListTransferDiscrepanciesRow{}
	}
	JSON(w, http.StatusOK, discrepancies)
}

func (h *InventoryHandler) GetDiscrepancy(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	d, err := h.inventoryService.GetDiscrepancy(r.Context(), id)
	if err != nil {
		if inventoryAuthorizationError(w, err) {
			return
		}
		if errors.Is(err, service.ErrDiscrepancyNotFound) {
			ErrorJSON(w, http.StatusNotFound, "NOT_FOUND", "Discrepancy not found", nil)
			return
		}
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}
	JSON(w, http.StatusOK, d)
}

func (h *InventoryHandler) ResolveDiscrepancy(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	id := chi.URLParam(r, "id")
	var req service.ResolveDiscrepancyInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Malformed JSON body", nil)
		return
	}

	resolved, err := h.inventoryService.ResolveDiscrepancy(r.Context(), user, id, req)
	if err != nil {
		if inventoryAuthorizationError(w, err) {
			return
		}
		if errors.Is(err, service.ErrDiscrepancyNotFound) {
			ErrorJSON(w, http.StatusNotFound, "NOT_FOUND", err.Error(), nil)
			return
		}
		if errors.Is(err, service.ErrInvalidDiscrepancyResolution) {
			ErrorJSON(w, http.StatusBadRequest, "INVALID_RESOLUTION", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusBadRequest, "RESOLUTION_FAILED", err.Error(), nil)
		return
	}
	JSON(w, http.StatusOK, resolved)
}

// -------------------------------------------------------------
// Stock Ledger & Reporting
// -------------------------------------------------------------

func (h *InventoryHandler) ListStockLedger(w http.ResponseWriter, r *http.Request) {
	var locID, itemID, mType *string
	if lid := r.URL.Query().Get("location_id"); lid != "" {
		locID = &lid
	}
	if itm := r.URL.Query().Get("item_id"); itm != "" {
		itemID = &itm
	}
	if mt := r.URL.Query().Get("movement_type"); mt != "" {
		mType = &mt
	}

	var startDate, endDate *time.Time
	if sd := r.URL.Query().Get("start_date"); sd != "" {
		if t, err := time.Parse(time.RFC3339, sd); err == nil {
			startDate = &t
		} else if t, err := time.Parse("2006-01-02", sd); err == nil {
			startDate = &t
		}
	}
	if ed := r.URL.Query().Get("end_date"); ed != "" {
		if t, err := time.Parse(time.RFC3339, ed); err == nil {
			endDate = &t
		} else if t, err := time.Parse("2006-01-02", ed); err == nil {
			endDate = &t
		}
	}

	limit := int32(50)
	offset := int32(0)
	if l := r.URL.Query().Get("limit"); l != "" {
		if val, err := strconv.Atoi(l); err == nil && val > 0 && val <= 500 {
			limit = int32(val)
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if val, err := strconv.Atoi(o); err == nil && val >= 0 {
			offset = int32(val)
		}
	}

	entries, total, err := h.inventoryService.ListStockLedger(r.Context(), locID, itemID, mType, startDate, endDate, limit, offset)
	if err != nil {
		if inventoryAuthorizationError(w, err) {
			return
		}
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}
	if entries == nil {
		entries = []db.ListStockLedgerEntriesRow{}
	}

	response := map[string]interface{}{
		"entries": entries,
		"total":   total,
		"limit":   limit,
		"offset":  offset,
	}
	JSON(w, http.StatusOK, response)
}

func (h *InventoryHandler) GetSummary(w http.ResponseWriter, r *http.Request) {
	summary, err := h.inventoryService.GetInventorySummary(r.Context())
	if err != nil {
		if inventoryAuthorizationError(w, err) {
			return
		}
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}
	JSON(w, http.StatusOK, summary)
}

func inventoryAuthorizationError(w http.ResponseWriter, err error) bool {
	if errors.Is(err, service.ErrUnauthorizedInventoryAction) || errors.Is(err, service.ErrJurisdictionMismatch) || errors.Is(err, service.ErrCoordinatorUnauthorized) || errors.Is(err, service.ErrScopeUnauthorized) {
		ErrorJSON(w, 403, "INVENTORY_SCOPE_UNAUTHORIZED", err.Error(), nil)
		return true
	}
	return false
}

func (h *InventoryHandler) TransferDestinations(w http.ResponseWriter, r *http.Request) {
	rows, err := h.inventoryService.TransferDestinations(r.Context(), GetCurrentUser(r.Context()))
	if err != nil {
		coordinatorError(w, err)
		return
	}
	JSON(w, 200, rows)
}
