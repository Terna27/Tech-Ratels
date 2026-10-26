package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"ratels-api/internal/db"
	"ratels-api/internal/payment"
	"ratels-api/internal/service"
	"ratels-api/internal/storage"
)

type MarketplaceHandler struct {
	marketplaceService *service.MarketplaceService
}

func NewMarketplaceHandler(marketplaceService *service.MarketplaceService) *MarketplaceHandler {
	return &MarketplaceHandler{marketplaceService: marketplaceService}
}

// -------------------------------------------------------------
// Public Storefront Endpoints
// -------------------------------------------------------------

func (h *MarketplaceHandler) ListPublicProducts(w http.ResponseWriter, r *http.Request) {
	var catPtr, searchPtr *string
	if c := r.URL.Query().Get("category"); c != "" {
		catPtr = &c
	}
	if q := r.URL.Query().Get("q"); q != "" {
		searchPtr = &q
	}
	var bIDPtr *string
	if b := r.URL.Query().Get("business_id"); b != "" {
		bIDPtr = &b
	}

	status := "published"
	approvedStatus := "approved"
	products, err := h.marketplaceService.ListProducts(r.Context(), service.ProductFilter{
		BusinessID:     bIDPtr,
		Category:       catPtr,
		Status:         &status,
		BusinessStatus: &approvedStatus,
		SearchQuery:    searchPtr,
	})
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}
	if products == nil {
		products = []db.ListMarketplaceProductsRow{}
	}
	JSON(w, http.StatusOK, products)
}

func (h *MarketplaceHandler) GetProduct(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	p, err := h.marketplaceService.GetProductByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, service.ErrProductNotFound) {
			ErrorJSON(w, http.StatusNotFound, "NOT_FOUND", "Product not found", nil)
			return
		}
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}
	JSON(w, http.StatusOK, p)
}

func (h *MarketplaceHandler) GetProductBySlug(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	p, err := h.marketplaceService.GetProductBySlug(r.Context(), slug)
	if err != nil {
		if errors.Is(err, service.ErrProductNotFound) {
			ErrorJSON(w, http.StatusNotFound, "NOT_FOUND", "Product not found", nil)
			return
		}
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}
	JSON(w, http.StatusOK, p)
}

func (h *MarketplaceHandler) ListBusinesses(w http.ResponseWriter, r *http.Request) {
	var catPtr *string
	if c := r.URL.Query().Get("category"); c != "" {
		catPtr = &c
	}
	status := "approved"
	businesses, err := h.marketplaceService.ListBusinesses(r.Context(), &status, catPtr, nil, nil)
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}
	if businesses == nil {
		businesses = []db.ListMarketplaceBusinessesRow{}
	}
	JSON(w, http.StatusOK, businesses)
}

func (h *MarketplaceHandler) GetBusiness(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	b, err := h.marketplaceService.GetBusinessByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, service.ErrBusinessNotFound) {
			ErrorJSON(w, http.StatusNotFound, "NOT_FOUND", "Business not found", nil)
			return
		}
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}
	JSON(w, http.StatusOK, b)
}

func (h *MarketplaceHandler) ListBusinessBranches(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	branches, err := h.marketplaceService.ListBranches(r.Context(), id)
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}
	if branches == nil {
		branches = []db.ListBusinessBranchesByBusinessRow{}
	}
	JSON(w, http.StatusOK, branches)
}

func (h *MarketplaceHandler) ValidateCart(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Items []service.CartItemInput `json:"items"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Malformed JSON payload", nil)
		return
	}

	result, err := h.marketplaceService.ValidateCart(r.Context(), req.Items)
	if err != nil {
		ErrorJSON(w, http.StatusBadRequest, "VALIDATION_FAILED", err.Error(), nil)
		return
	}
	JSON(w, http.StatusOK, result)
}

func (h *MarketplaceHandler) GetCheckoutStatus(w http.ResponseWriter, r *http.Request) {
	status := h.marketplaceService.GetCheckoutStatus(r.Context())
	JSON(w, http.StatusOK, status)
}

func (h *MarketplaceHandler) Checkout(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	var req service.CheckoutInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Malformed JSON payload", nil)
		return
	}

	result, err := h.marketplaceService.CreateOrder(r.Context(), user, req)
	if err != nil {
		if errors.Is(err, payment.ErrPaymentProcessingUnavailable) {
			ErrorJSON(w, http.StatusServiceUnavailable, "PAYMENT_PROCESSING_UNAVAILABLE", "Payment processing is currently undergoing scheduled maintenance. Please check back shortly.", nil)
			return
		}
		if errors.Is(err, service.ErrInsufficientStock) {
			ErrorJSON(w, http.StatusConflict, "INSUFFICIENT_STOCK", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusBadRequest, "CHECKOUT_FAILED", err.Error(), nil)
		return
	}
	JSON(w, http.StatusCreated, result)
}

func (h *MarketplaceHandler) LookupOrder(w http.ResponseWriter, r *http.Request) {
	orderNumber := chi.URLParam(r, "orderNumber")
	order, items, err := h.marketplaceService.GetOrderByOrderNumber(r.Context(), orderNumber)
	if err != nil {
		if errors.Is(err, service.ErrOrderNotFound) {
			ErrorJSON(w, http.StatusNotFound, "NOT_FOUND", "Order not found", nil)
			return
		}
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}
	JSON(w, http.StatusOK, map[string]interface{}{
		"order": order,
		"items": items,
	})
}

func (h *MarketplaceHandler) VerifyPayment(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Reference string `json:"reference"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Reference == "" {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Payment reference is required", nil)
		return
	}

	order, err := h.marketplaceService.ConfirmPayment(r.Context(), req.Reference)
	if err != nil {
		if errors.Is(err, payment.ErrPaymentProcessingUnavailable) {
			ErrorJSON(w, http.StatusServiceUnavailable, "PAYMENT_PROCESSING_UNAVAILABLE", "Payment processing is currently unavailable.", nil)
			return
		}
		if errors.Is(err, payment.ErrAmountMismatch) || errors.Is(err, payment.ErrCurrencyMismatch) || errors.Is(err, payment.ErrReferenceMismatch) || errors.Is(err, payment.ErrProviderMismatch) {
			ErrorJSON(w, http.StatusPaymentRequired, "PAYMENT_MISMATCH", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusBadRequest, "VERIFICATION_FAILED", err.Error(), nil)
		return
	}
	JSON(w, http.StatusOK, map[string]interface{}{
		"status": "paid",
		"order":  order,
	})
}

func (h *MarketplaceHandler) PaystackWebhook(w http.ResponseWriter, r *http.Request) {
	signature := r.Header.Get("x-paystack-signature")
	body, err := io.ReadAll(r.Body)
	if err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Cannot read request body", nil)
		return
	}

	if err := h.marketplaceService.HandleWebhook(r.Context(), signature, body); err != nil {
		// Log warning, return 400 for bad signature, 200 for idempotent handling
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// -------------------------------------------------------------
// Buyer Order Endpoints (Authenticated)
// -------------------------------------------------------------

func (h *MarketplaceHandler) ListMyOrders(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	orders, err := h.marketplaceService.ListOrdersByBuyer(r.Context(), user)
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}
	if orders == nil {
		orders = []db.ListMarketplaceOrdersByBuyerRow{}
	}
	JSON(w, http.StatusOK, orders)
}

func (h *MarketplaceHandler) GetOrder(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	id := chi.URLParam(r, "id")
	order, items, shipment, err := h.marketplaceService.GetOrderByID(r.Context(), user, id)
	if err != nil {
		if errors.Is(err, service.ErrOrderNotFound) {
			ErrorJSON(w, http.StatusNotFound, "NOT_FOUND", "Order not found", nil)
			return
		}
		if errors.Is(err, service.ErrUnauthorizedBuyerAction) {
			ErrorJSON(w, http.StatusForbidden, "FORBIDDEN", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}
	JSON(w, http.StatusOK, map[string]interface{}{
		"order":    order,
		"items":    items,
		"shipment": shipment,
	})
}

func (h *MarketplaceHandler) RaiseDispute(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	orderID := chi.URLParam(r, "id")

	var req service.CreateDisputeInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Malformed JSON payload", nil)
		return
	}

	dispute, err := h.marketplaceService.RaiseDispute(r.Context(), user, orderID, req)
	if err != nil {
		ErrorJSON(w, http.StatusBadRequest, "DISPUTE_FAILED", err.Error(), nil)
		return
	}
	JSON(w, http.StatusCreated, dispute)
}

func (h *MarketplaceHandler) CancelOrder(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	id := chi.URLParam(r, "id")
	var req struct {
		Reason string `json:"reason"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.Reason == "" {
		req.Reason = "Cancelled by user"
	}

	order, err := h.marketplaceService.CancelOrder(r.Context(), user, id, req.Reason)
	if err != nil {
		if errors.Is(err, service.ErrOrderNotFound) {
			ErrorJSON(w, http.StatusNotFound, "NOT_FOUND", "Order not found", nil)
			return
		}
		if errors.Is(err, service.ErrUnauthorizedBuyerAction) {
			ErrorJSON(w, http.StatusForbidden, "FORBIDDEN", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusBadRequest, "CANCEL_FAILED", err.Error(), nil)
		return
	}
	JSON(w, http.StatusOK, order)
}

// -------------------------------------------------------------
// Seller / Vendor Portal Endpoints (Authenticated)
// -------------------------------------------------------------

func (h *MarketplaceHandler) RegisterBusiness(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	var req service.RegisterBusinessInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Malformed JSON payload", nil)
		return
	}

	b, err := h.marketplaceService.RegisterBusiness(r.Context(), user, req)
	if err != nil {
		if errors.Is(err, service.ErrBusinessAlreadyExists) {
			ErrorJSON(w, http.StatusConflict, "CONFLICT", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusBadRequest, "REGISTRATION_FAILED", err.Error(), nil)
		return
	}
	JSON(w, http.StatusCreated, b)
}

func (h *MarketplaceHandler) GetMyBusiness(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	b, err := h.marketplaceService.GetMyBusiness(r.Context(), user)
	if err != nil {
		if errors.Is(err, service.ErrBusinessNotFound) {
			ErrorJSON(w, http.StatusNotFound, "NOT_FOUND", "No business profile registered", nil)
			return
		}
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}
	JSON(w, http.StatusOK, b)
}

func (h *MarketplaceHandler) UpdateMyBusiness(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	var req service.UpdateBusinessInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Malformed JSON payload", nil)
		return
	}

	myB, err := h.marketplaceService.GetMyBusiness(r.Context(), user)
	if err != nil {
		ErrorJSON(w, http.StatusNotFound, "NOT_FOUND", "No business profile registered", nil)
		return
	}

	b, err := h.marketplaceService.UpdateBusiness(r.Context(), user, myB.ID, req)
	if err != nil {
		ErrorJSON(w, http.StatusBadRequest, "UPDATE_FAILED", err.Error(), nil)
		return
	}
	JSON(w, http.StatusOK, b)
}

func (h *MarketplaceHandler) CreateBranch(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	myB, err := h.marketplaceService.GetMyBusiness(r.Context(), user)
	if err != nil {
		ErrorJSON(w, http.StatusNotFound, "NOT_FOUND", "No business profile registered", nil)
		return
	}

	var req service.CreateBranchInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Malformed JSON payload", nil)
		return
	}

	br, err := h.marketplaceService.CreateBranch(r.Context(), user, myB.ID, req)
	if err != nil {
		ErrorJSON(w, http.StatusBadRequest, "CREATE_BRANCH_FAILED", err.Error(), nil)
		return
	}
	JSON(w, http.StatusCreated, br)
}

func (h *MarketplaceHandler) ListMyProducts(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	myB, err := h.marketplaceService.GetMyBusiness(r.Context(), user)
	if err != nil {
		ErrorJSON(w, http.StatusNotFound, "NOT_FOUND", "No business profile registered", nil)
		return
	}

	products, err := h.marketplaceService.ListProducts(r.Context(), service.ProductFilter{
		BusinessID: &myB.ID,
	})
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}
	if products == nil {
		products = []db.ListMarketplaceProductsRow{}
	}
	JSON(w, http.StatusOK, products)
}

func (h *MarketplaceHandler) CreateProduct(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	myB, err := h.marketplaceService.GetMyBusiness(r.Context(), user)
	if err != nil {
		ErrorJSON(w, http.StatusNotFound, "NOT_FOUND", "No business profile registered", nil)
		return
	}

	var req service.CreateProductInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Malformed JSON payload", nil)
		return
	}
	req.BusinessID = myB.ID

	p, err := h.marketplaceService.CreateProduct(r.Context(), user, req)
	if err != nil {
		ErrorJSON(w, http.StatusBadRequest, "CREATE_PRODUCT_FAILED", err.Error(), nil)
		return
	}
	JSON(w, http.StatusCreated, p)
}

func (h *MarketplaceHandler) UpdateProduct(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	id := chi.URLParam(r, "id")

	var req service.UpdateProductInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Malformed JSON payload", nil)
		return
	}

	p, err := h.marketplaceService.UpdateProduct(r.Context(), user, id, req)
	if err != nil {
		if errors.Is(err, service.ErrUnauthorizedSellerAction) {
			ErrorJSON(w, http.StatusForbidden, "FORBIDDEN", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusBadRequest, "UPDATE_PRODUCT_FAILED", err.Error(), nil)
		return
	}
	JSON(w, http.StatusOK, p)
}

func (h *MarketplaceHandler) DeleteProduct(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	id := chi.URLParam(r, "id")

	if err := h.marketplaceService.DeleteProduct(r.Context(), user, id); err != nil {
		if errors.Is(err, service.ErrUnauthorizedSellerAction) {
			ErrorJSON(w, http.StatusForbidden, "FORBIDDEN", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusBadRequest, "DELETE_FAILED", err.Error(), nil)
		return
	}
	JSON(w, http.StatusOK, map[string]string{"message": "Product deleted successfully"})
}

// -------------------------------------------------------------
// Product Image Upload & Media Management Endpoints
// -------------------------------------------------------------

type AuthorizeImageUploadRequest struct {
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	FileSize    int64  `json:"file_size"`
}

func (h *MarketplaceHandler) AuthorizeImageUpload(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	var req AuthorizeImageUploadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Malformed JSON payload", nil)
		return
	}

	res, err := h.marketplaceService.AuthorizeProductImageUpload(r.Context(), user, req.Filename, req.ContentType, req.FileSize)
	if err != nil {
		if errors.Is(err, service.ErrBusinessNotFound) {
			ErrorJSON(w, http.StatusNotFound, "BUSINESS_NOT_FOUND", "No active business profile found for this seller account", nil)
			return
		}
		if errors.Is(err, service.ErrBusinessNotApproved) {
			ErrorJSON(w, http.StatusForbidden, "BUSINESS_NOT_APPROVED", "Business account must be active or pending approval to upload product media", nil)
			return
		}
		if errors.Is(err, service.ErrUnauthorizedSellerAction) {
			ErrorJSON(w, http.StatusForbidden, "FORBIDDEN", err.Error(), nil)
			return
		}
		if errors.Is(err, storage.ErrStorageUnconfigured) {
			ErrorJSON(w, http.StatusServiceUnavailable, "STORAGE_UNCONFIGURED", "Product image storage service is currently unconfigured in this environment.", nil)
			return
		}
		if errors.Is(err, storage.ErrImageEmpty) || errors.Is(err, storage.ErrImageTooLarge) || errors.Is(err, storage.ErrInvalidImageType) {
			ErrorJSON(w, http.StatusBadRequest, "INVALID_IMAGE", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusInternalServerError, "UPLOAD_AUTHORIZATION_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, res)
}

func (h *MarketplaceHandler) UploadProductImage(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	// Limit request body to 6MB (5MB file + multipart form metadata)
	r.Body = http.MaxBytesReader(w, r.Body, 6*1024*1024)
	if err := r.ParseMultipartForm(5 * 1024 * 1024); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "PAYLOAD_TOO_LARGE", "Upload exceeds maximum allowed limit of 5 MB", nil)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		// Also accept "image" field
		file, header, err = r.FormFile("image")
		if err != nil {
			ErrorJSON(w, http.StatusBadRequest, "MISSING_FILE", "A multipart file field named 'file' or 'image' is required", nil)
			return
		}
	}
	defer file.Close()

	res, err := h.marketplaceService.UploadProductImageDirect(r.Context(), user, header.Filename, file, header.Size)
	if err != nil {
		if errors.Is(err, service.ErrBusinessNotFound) {
			ErrorJSON(w, http.StatusNotFound, "BUSINESS_NOT_FOUND", "No active business profile found for this seller account", nil)
			return
		}
		if errors.Is(err, service.ErrBusinessNotApproved) {
			ErrorJSON(w, http.StatusForbidden, "BUSINESS_NOT_APPROVED", "Business account must be active or pending approval to upload product media", nil)
			return
		}
		if errors.Is(err, service.ErrUnauthorizedSellerAction) {
			ErrorJSON(w, http.StatusForbidden, "FORBIDDEN", err.Error(), nil)
			return
		}
		if errors.Is(err, storage.ErrStorageUnconfigured) {
			ErrorJSON(w, http.StatusServiceUnavailable, "STORAGE_UNCONFIGURED", "Product image storage service is currently unconfigured in this environment.", nil)
			return
		}
		if errors.Is(err, storage.ErrImageEmpty) || errors.Is(err, storage.ErrImageTooLarge) ||
			errors.Is(err, storage.ErrInvalidImageType) || errors.Is(err, storage.ErrCorruptImage) ||
			errors.Is(err, storage.ErrInvalidDimensions) {
			ErrorJSON(w, http.StatusBadRequest, "INVALID_IMAGE", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusInternalServerError, "UPLOAD_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusCreated, res)
}

func (h *MarketplaceHandler) DeleteProductImage(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	var objectKey string
	if r.Header.Get("Content-Type") == "application/json" {
		var req struct {
			ObjectKey string `json:"object_key"`
			URL       string `json:"url"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
			objectKey = req.ObjectKey
			if objectKey == "" && req.URL != "" {
				parts := strings.Split(req.URL, storage.MarketplaceImagePrefix)
				if len(parts) > 1 {
					objectKey = storage.MarketplaceImagePrefix + parts[1]
				}
			}
		}
	}
	if objectKey == "" {
		objectKey = r.URL.Query().Get("key")
	}
	if objectKey == "" {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "object_key or url is required", nil)
		return
	}

	if err := h.marketplaceService.DeleteProductImage(r.Context(), user, objectKey); err != nil {
		if errors.Is(err, service.ErrBusinessNotFound) {
			ErrorJSON(w, http.StatusNotFound, "BUSINESS_NOT_FOUND", "Business not found", nil)
			return
		}
		if errors.Is(err, storage.ErrUnauthorizedKey) {
			ErrorJSON(w, http.StatusForbidden, "FORBIDDEN", "Unauthorized: this image does not belong to your business", nil)
			return
		}
		if errors.Is(err, storage.ErrStorageUnconfigured) {
			ErrorJSON(w, http.StatusServiceUnavailable, "STORAGE_UNCONFIGURED", "Storage service is unconfigured", nil)
			return
		}
		ErrorJSON(w, http.StatusBadRequest, "DELETE_IMAGE_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Product image deleted successfully",
	})
}

func (h *MarketplaceHandler) MockDevUpload(w http.ResponseWriter, r *http.Request) {
	// Accept mock upload payload in dev/sandbox mode
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"uploaded"}`))
}

func (h *MarketplaceHandler) ListSellerOrders(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	myB, err := h.marketplaceService.GetMyBusiness(r.Context(), user)
	if err != nil {
		ErrorJSON(w, http.StatusNotFound, "NOT_FOUND", "No business profile registered", nil)
		return
	}

	var statusPtr *string
	if s := r.URL.Query().Get("status"); s != "" {
		statusPtr = &s
	}

	orders, err := h.marketplaceService.ListOrdersByBusiness(r.Context(), user, myB.ID, statusPtr)
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}
	if orders == nil {
		orders = []db.ListMarketplaceOrdersByBusinessRow{}
	}
	JSON(w, http.StatusOK, orders)
}

func (h *MarketplaceHandler) DispatchOrder(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	orderID := chi.URLParam(r, "id")

	var req service.DispatchOrderInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Malformed JSON payload", nil)
		return
	}

	sh, err := h.marketplaceService.DispatchOrder(r.Context(), user, orderID, req)
	if err != nil {
		ErrorJSON(w, http.StatusBadRequest, "DISPATCH_FAILED", err.Error(), nil)
		return
	}
	JSON(w, http.StatusOK, sh)
}

func (h *MarketplaceHandler) ConfirmDelivery(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	orderID := chi.URLParam(r, "id")

	var req struct {
		ProofURL string `json:"proof_url"`
		Notes    string `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Malformed JSON payload", nil)
		return
	}

	sh, err := h.marketplaceService.ConfirmDelivery(r.Context(), user, orderID, req.ProofURL, req.Notes)
	if err != nil {
		ErrorJSON(w, http.StatusBadRequest, "DELIVERY_CONFIRM_FAILED", err.Error(), nil)
		return
	}
	JSON(w, http.StatusOK, sh)
}

func (h *MarketplaceHandler) GetSellerMetrics(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	metrics, err := h.marketplaceService.GetSellerOverviewMetrics(r.Context(), user)
	if err != nil {
		if errors.Is(err, service.ErrBusinessNotFound) {
			ErrorJSON(w, http.StatusNotFound, "NOT_FOUND", "No business profile registered", nil)
			return
		}
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}
	JSON(w, http.StatusOK, metrics)
}

// -------------------------------------------------------------
// Admin Workspace Endpoints
// -------------------------------------------------------------

func (h *MarketplaceHandler) GetAdminOverview(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	overview, err := h.marketplaceService.GetAdminOverviewMetrics(r.Context(), user)
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}
	JSON(w, http.StatusOK, overview)
}

func (h *MarketplaceHandler) ListAllBusinessesAdmin(w http.ResponseWriter, r *http.Request) {
	var statusPtr, catPtr *string
	if s := r.URL.Query().Get("status"); s != "" {
		statusPtr = &s
	}
	if c := r.URL.Query().Get("category"); c != "" {
		catPtr = &c
	}
	businesses, err := h.marketplaceService.ListBusinesses(r.Context(), statusPtr, catPtr, nil, nil)
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}
	if businesses == nil {
		businesses = []db.ListMarketplaceBusinessesRow{}
	}
	JSON(w, http.StatusOK, businesses)
}

func (h *MarketplaceHandler) ApproveBusiness(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	id := chi.URLParam(r, "id")

	var req struct {
		Approve bool   `json:"approve"`
		Reason  string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Malformed JSON payload", nil)
		return
	}

	b, err := h.marketplaceService.ApproveBusiness(r.Context(), user, id, req.Approve, req.Reason)
	if err != nil {
		ErrorJSON(w, http.StatusBadRequest, "APPROVAL_FAILED", err.Error(), nil)
		return
	}
	JSON(w, http.StatusOK, b)
}

func (h *MarketplaceHandler) ListAllProductsAdmin(w http.ResponseWriter, r *http.Request) {
	var catPtr, statusPtr, searchPtr *string
	if c := r.URL.Query().Get("category"); c != "" {
		catPtr = &c
	}
	if s := r.URL.Query().Get("status"); s != "" {
		statusPtr = &s
	}
	if q := r.URL.Query().Get("q"); q != "" {
		searchPtr = &q
	}

	products, err := h.marketplaceService.ListProducts(r.Context(), service.ProductFilter{
		Category:    catPtr,
		Status:      statusPtr,
		SearchQuery: searchPtr,
	})
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}
	if products == nil {
		products = []db.ListMarketplaceProductsRow{}
	}
	JSON(w, http.StatusOK, products)
}

func (h *MarketplaceHandler) ModerateProduct(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	id := chi.URLParam(r, "id")

	var req struct {
		Status     string `json:"status"` // 'published', 'archived'
		IsFeatured *bool  `json:"is_featured"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Malformed JSON payload", nil)
		return
	}

	existing, err := h.marketplaceService.GetProductByID(r.Context(), id)
	if err != nil {
		ErrorJSON(w, http.StatusNotFound, "NOT_FOUND", "Product not found", nil)
		return
	}

	isFeat := existing.IsFeatured
	if req.IsFeatured != nil {
		isFeat = *req.IsFeatured
	}
	newStatus := existing.Status
	if req.Status != "" {
		newStatus = req.Status
	}

	var compPrice *int64
	if existing.CompareAtPriceKobo.Valid {
		compPrice = &existing.CompareAtPriceKobo.Int64
	}
	var weight *int32
	if existing.WeightGrams.Valid {
		weight = &existing.WeightGrams.Int32
	}
	var images []string
	_ = json.Unmarshal(existing.Images, &images)

	var branchID *string
	if existing.BranchID.Valid {
		branchID = &existing.BranchID.String
	}
	var delRest *string
	if existing.DeliveryRestrictions.Valid {
		delRest = &existing.DeliveryRestrictions.String
	}
	var retPol *string
	if existing.ReturnPolicy.Valid {
		retPol = &existing.ReturnPolicy.String
	}

	updated, err := h.marketplaceService.UpdateProduct(r.Context(), user, id, service.UpdateProductInput{
		BranchID:             branchID,
		Title:                existing.Title,
		Description:          existing.Description,
		Category:             existing.Category,
		SKU:                  existing.Sku,
		PriceKobo:            existing.PriceKobo,
		CompareAtPriceKobo:   compPrice,
		StockQuantity:        existing.StockQuantity,
		LowStockThreshold:    existing.LowStockThreshold,
		Images:               images,
		WeightGrams:          weight,
		DeliveryRestrictions: delRest,
		ReturnPolicy:         retPol,
		Status:               newStatus,
		IsFeatured:           isFeat,
	})
	if err != nil {
		ErrorJSON(w, http.StatusBadRequest, "MODERATION_FAILED", err.Error(), nil)
		return
	}
	JSON(w, http.StatusOK, updated)
}

func (h *MarketplaceHandler) ListAllOrdersAdmin(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	var ordSt, paySt, fulSt *string
	if o := r.URL.Query().Get("order_status"); o != "" {
		ordSt = &o
	}
	if p := r.URL.Query().Get("payment_status"); p != "" {
		paySt = &p
	}
	if f := r.URL.Query().Get("fulfilment_status"); f != "" {
		fulSt = &f
	}

	orders, err := h.marketplaceService.ListAllOrders(r.Context(), user, ordSt, paySt, fulSt)
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}
	if orders == nil {
		orders = []db.ListAllMarketplaceOrdersRow{}
	}
	JSON(w, http.StatusOK, orders)
}

func (h *MarketplaceHandler) ListPaymentsAdmin(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	var st, pr *string
	if s := r.URL.Query().Get("status"); s != "" {
		st = &s
	}
	if p := r.URL.Query().Get("provider"); p != "" {
		pr = &p
	}

	payments, err := h.marketplaceService.ListPayments(r.Context(), user, st, pr)
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}
	if payments == nil {
		payments = []db.ListMarketplacePaymentsRow{}
	}
	JSON(w, http.StatusOK, payments)
}

func (h *MarketplaceHandler) ListDisputesAdmin(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	var st *string
	if s := r.URL.Query().Get("status"); s != "" {
		st = &s
	}

	disputes, err := h.marketplaceService.ListDisputes(r.Context(), user, st)
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}
	if disputes == nil {
		disputes = []db.ListAllMarketplaceDisputesRow{}
	}
	JSON(w, http.StatusOK, disputes)
}

func (h *MarketplaceHandler) ResolveDispute(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	id := chi.URLParam(r, "id")

	var req struct {
		Resolution string `json:"resolution"` // 'resolved_refunded', 'rejected'
		Notes      string `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Malformed JSON payload", nil)
		return
	}

	d, err := h.marketplaceService.ResolveDispute(r.Context(), user, id, req.Resolution, req.Notes)
	if err != nil {
		ErrorJSON(w, http.StatusBadRequest, "RESOLVE_FAILED", err.Error(), nil)
		return
	}
	JSON(w, http.StatusOK, d)
}

func (h *MarketplaceHandler) AdminRefundOrder(w http.ResponseWriter, r *http.Request) {
	admin := GetCurrentUser(r.Context())
	id := chi.URLParam(r, "id")

	var req service.RefundOrderInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Malformed JSON payload", nil)
		return
	}

	refund, order, err := h.marketplaceService.InitiateRefund(r.Context(), admin, id, req)
	if err != nil {
		if errors.Is(err, service.ErrOrderNotFound) {
			ErrorJSON(w, http.StatusNotFound, "NOT_FOUND", "Order not found", nil)
			return
		}
		ErrorJSON(w, http.StatusBadRequest, "REFUND_FAILED", err.Error(), nil)
		return
	}
	JSON(w, http.StatusOK, map[string]interface{}{
		"status": "refund_initiated",
		"refund": refund,
		"order":  order,
	})
}

func (h *MarketplaceHandler) AdminConfirmRefund(w http.ResponseWriter, r *http.Request) {
	admin := GetCurrentUser(r.Context())
	refundID := chi.URLParam(r, "refundId")

	var req service.ConfirmRefundInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// Optional empty body
		req = service.ConfirmRefundInput{}
	}

	refund, order, err := h.marketplaceService.ConfirmRefund(r.Context(), admin, refundID, req)
	if err != nil {
		ErrorJSON(w, http.StatusBadRequest, "CONFIRM_FAILED", err.Error(), nil)
		return
	}
	JSON(w, http.StatusOK, map[string]interface{}{
		"status": "refund_confirmed",
		"refund": refund,
		"order":  order,
	})
}

func (h *MarketplaceHandler) AdminManualSettleRefund(w http.ResponseWriter, r *http.Request) {
	admin := GetCurrentUser(r.Context())
	refundID := chi.URLParam(r, "refundId")

	var req service.ManualSettlementInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Malformed JSON payload", nil)
		return
	}

	refund, order, err := h.marketplaceService.ManualSettleRefund(r.Context(), admin, refundID, req)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unauthorized") {
			ErrorJSON(w, http.StatusForbidden, "UNAUTHORIZED", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusBadRequest, "SETTLEMENT_FAILED", err.Error(), nil)
		return
	}
	JSON(w, http.StatusOK, map[string]interface{}{
		"status": "refund_settled",
		"refund": refund,
		"order":  order,
	})
}
