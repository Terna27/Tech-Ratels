package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"ratels-api/internal/db"
	"ratels-api/internal/payment"
	"ratels-api/internal/repository"
	"ratels-api/internal/security"
	"ratels-api/internal/storage"
)

var (
	ErrBusinessNotFound         = errors.New("marketplace business not found")
	ErrBusinessAlreadyExists     = errors.New("a business profile already exists for this member")
	ErrBusinessNotApproved       = errors.New("business is not approved for public trading")
	ErrProductNotFound          = errors.New("marketplace product not found")
	ErrBranchNotFound           = errors.New("business branch not found")
	ErrOrderNotFound            = errors.New("marketplace order not found")
	ErrDisputeNotFound          = errors.New("marketplace dispute not found")
	ErrUnauthorizedSellerAction = errors.New("unauthorized: you do not own this business or product")
	ErrUnauthorizedBuyerAction  = errors.New("unauthorized: you are not the buyer of this order")
	ErrDuplicateOrderSubmission = errors.New("duplicate order submission detected")
	ErrEmptyCart                = errors.New("cannot create order with an empty cart")
	ErrInvalidPrice             = errors.New("invalid price: amount must be a positive integer in kobo")
	ErrPaymentNotFound          = errors.New("marketplace payment not found")
	ErrOrderNotPaid             = errors.New("cannot dispatch an unpaid order")
	ErrOrderAlreadyDelivered    = errors.New("order has already been delivered")
	ErrOrderAlreadyDispatched   = errors.New("order has already been dispatched")
	ErrOrderAlreadyCancelled    = errors.New("order is already cancelled")
	ErrOrderAlreadyRefunded     = errors.New("cannot dispatch a refunded order")
	ErrCrossSellerDispatch         = errors.New("sellers can dispatch only items belonging to their businesses")
	ErrOrderReconciliationRequired = errors.New("cannot dispatch order requiring administrative reconciliation")
	ErrOrderStockAlreadyReleased   = errors.New("cannot dispatch order whose inventory reservation has been released")
	ErrShipmentNotFound            = errors.New("marketplace shipment not found")
	ErrShipmentAlreadyDispatched   = errors.New("shipment has already been dispatched")
	ErrShipmentAlreadyDelivered    = errors.New("shipment has already been delivered")
	ErrIdempotencyKeyRequired      = errors.New("idempotency_key is required")
	ErrIdempotencyKeyTooLong       = errors.New("idempotency_key cannot exceed 128 characters")
	ErrIdempotencyConflict         = errors.New("idempotency key conflict: key already used with different payload")
)

type MarketplaceService struct {
	repo           *repository.Repository
	audit          *AuditService
	paymentGateway payment.PaymentGateway
	storage        storage.StorageService
}

func NewMarketplaceService(repo *repository.Repository, audit *AuditService, gateway payment.PaymentGateway, stor ...storage.StorageService) *MarketplaceService {
	var s storage.StorageService = storage.NewDevSandboxStorageService("")
	if len(stor) > 0 && stor[0] != nil {
		s = stor[0]
	}
	return &MarketplaceService{
		repo:           repo,
		audit:          audit,
		paymentGateway: gateway,
		storage:        s,
	}
}

func (s *MarketplaceService) SetStorage(stor storage.StorageService) {
	if stor != nil {
		s.storage = stor
	}
}

func (s *MarketplaceService) Storage() storage.StorageService {
	return s.storage
}

type PresignedUploadResult struct {
	UploadURL        string `json:"upload_url"`
	ObjectKey        string `json:"object_key"`
	PublicURL        string `json:"public_url"`
	ExpiresInSeconds int64  `json:"expires_in_seconds"`
}

type UploadedImageResult struct {
	ObjectKey   string `json:"object_key"`
	PublicURL   string `json:"public_url"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	Format      string `json:"format"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
}

// -------------------------------------------------------------
// DTOs
// -------------------------------------------------------------

type RegisterBusinessInput struct {
	Name                   string  `json:"name"`
	Description            string  `json:"description"`
	Category               string  `json:"category"`
	OwnershipType          string  `json:"ownership_type"` // 'movement_owned', 'member_owned', 'approved_partner'
	ContactEmail           string  `json:"contact_email"`
	ContactPhone           string  `json:"contact_phone"`
	LogoURL                *string `json:"logo_url"`
	BannerURL              *string `json:"banner_url"`
	SettlementBankName     *string `json:"settlement_bank_name"`
	SettlementAccountNumber *string `json:"settlement_account_number"`
	SettlementAccountName   *string `json:"settlement_account_name"`
}

type UpdateBusinessInput struct {
	Name                   string  `json:"name"`
	Description            string  `json:"description"`
	Category               string  `json:"category"`
	ContactEmail           string  `json:"contact_email"`
	ContactPhone           string  `json:"contact_phone"`
	LogoURL                *string `json:"logo_url"`
	BannerURL              *string `json:"banner_url"`
	SettlementBankName     *string `json:"settlement_bank_name"`
	SettlementAccountNumber *string `json:"settlement_account_number"`
	SettlementAccountName   *string `json:"settlement_account_name"`
}

type CreateBranchInput struct {
	BranchName     string  `json:"branch_name"`
	StateID        string  `json:"state_id"`
	LGAID          *string `json:"lga_id"`
	Address        string  `json:"address"`
	ContactPhone   string  `json:"contact_phone"`
	ManagerName    *string `json:"manager_name"`
	IsHeadquarters bool    `json:"is_headquarters"`
	IsActive       bool    `json:"is_active"`
}

type CreateProductInput struct {
	BusinessID           string   `json:"business_id"`
	BranchID             *string  `json:"branch_id"`
	Title                string   `json:"title"`
	Description          string   `json:"description"`
	Category             string   `json:"category"`
	SKU                  string   `json:"sku"`
	PriceKobo            int64    `json:"price_kobo"`
	CompareAtPriceKobo   *int64   `json:"compare_at_price_kobo"`
	StockQuantity        int32    `json:"stock_quantity"`
	LowStockThreshold    int32    `json:"low_stock_threshold"`
	Images               []string `json:"images"`
	WeightGrams          *int32   `json:"weight_grams"`
	DeliveryRestrictions *string  `json:"delivery_restrictions"`
	ReturnPolicy         *string  `json:"return_policy"`
	Status               string   `json:"status"` // 'draft', 'published', 'archived'
	IsFeatured           bool     `json:"is_featured"`
}

type UpdateProductInput struct {
	BranchID             *string  `json:"branch_id"`
	Title                string   `json:"title"`
	Description          string   `json:"description"`
	Category             string   `json:"category"`
	SKU                  string   `json:"sku"`
	PriceKobo            int64    `json:"price_kobo"`
	CompareAtPriceKobo   *int64   `json:"compare_at_price_kobo"`
	StockQuantity        int32    `json:"stock_quantity"`
	LowStockThreshold    int32    `json:"low_stock_threshold"`
	Images               []string `json:"images"`
	WeightGrams          *int32   `json:"weight_grams"`
	DeliveryRestrictions *string  `json:"delivery_restrictions"`
	ReturnPolicy         *string  `json:"return_policy"`
	Status               string   `json:"status"`
	IsFeatured           bool     `json:"is_featured"`
}

type CartItemInput struct {
	ProductID string `json:"product_id"`
	Quantity  int32  `json:"quantity"`
}

type ValidatedCartItem struct {
	ProductID     string `json:"product_id"`
	BusinessID    string `json:"business_id"`
	BusinessName  string `json:"business_name"`
	Title         string `json:"title"`
	SKU           string `json:"sku"`
	PriceKobo     int64  `json:"price_kobo"`
	Quantity      int32  `json:"quantity"`
	SubtotalKobo  int64  `json:"subtotal_kobo"`
	StockQuantity int32  `json:"stock_quantity"`
	ImageURL      string `json:"image_url"`
	IsAvailable   bool   `json:"is_available"`
}

type CartValidationResult struct {
	Items           []ValidatedCartItem `json:"items"`
	SubtotalKobo    int64               `json:"subtotal_kobo"`
	DeliveryFeeKobo int64               `json:"delivery_fee_kobo"`
	TotalKobo       int64               `json:"total_kobo"`
	IsValid         bool                `json:"is_valid"`
	ErrorMessage    *string             `json:"error_message,omitempty"`
}

type CheckoutInput struct {
	Items           []CartItemInput `json:"items"`
	BuyerName       string          `json:"buyer_name"`
	BuyerEmail      string          `json:"buyer_email"`
	BuyerPhone      string          `json:"buyer_phone"`
	DeliveryType    string          `json:"delivery_type"` // 'delivery' or 'pickup'
	ShippingAddress string          `json:"shipping_address"`
	ShippingStateID *string         `json:"shipping_state_id"`
	ShippingLGAID   *string         `json:"shipping_lga_id"`
	PickupBranchID  *string         `json:"pickup_branch_id"`
	IdempotencyKey  string          `json:"idempotency_key"`
	Notes           string          `json:"notes"`
	CallbackURL     string          `json:"callback_url"`
}

type CreateOrderResult struct {
	Order       *db.GetMarketplaceOrderByIDRow `json:"order"`
	Items       []db.ListMarketplaceOrderItemsByOrderRow `json:"items"`
	Payment     *db.MarketplacePayment         `json:"payment"`
	CheckoutURL string                         `json:"checkout_url"`
}

// -------------------------------------------------------------
// Vendor & Business Methods
// -------------------------------------------------------------

func (s *MarketplaceService) RegisterBusiness(ctx context.Context, user *db.User, input RegisterBusinessInput) (*db.MarketplaceBusiness, error) {
	if strings.TrimSpace(input.Name) == "" {
		return nil, errors.New("business name is required")
	}

	// Check if user already has a business
	existing, err := s.repo.Queries().GetMarketplaceBusinessByOwner(ctx, user.ID)
	if err == nil && existing.ID != "" {
		return nil, ErrBusinessAlreadyExists
	}

	slug := slugify(input.Name)
	// Ensure unique slug
	if _, err := s.repo.Queries().GetMarketplaceBusinessBySlug(ctx, slug); err == nil {
		slug = fmt.Sprintf("%s-%s", slug, strings.ToLower(security.GenerateUUID()[:6]))
	}

	ownershipType := input.OwnershipType
	if ownershipType == "" {
		ownershipType = "member_owned"
	}
	switch ownershipType {
	case "movement_owned", "member_owned", "approved_partner":
	default:
		return nil, fmt.Errorf("invalid ownership type: %s", ownershipType)
	}

	businessID := security.GenerateUUID()
	b, err := s.repo.Queries().CreateMarketplaceBusiness(ctx, db.CreateMarketplaceBusinessParams{
		ID:                      businessID,
		OwnerUserID:             user.ID,
		Name:                    input.Name,
		Slug:                    slug,
		Description:             input.Description,
		Category:                input.Category,
		OwnershipType:           ownershipType,
		Status:                  "pending_review",
		ContactEmail:            input.ContactEmail,
		ContactPhone:            input.ContactPhone,
		LogoUrl:                 Text(input.LogoURL),
		BannerUrl:               Text(input.BannerURL),
		SettlementBankName:      Text(input.SettlementBankName),
		SettlementAccountNumber: Text(input.SettlementAccountNumber),
		SettlementAccountName:   Text(input.SettlementAccountName),
		IsVerified:              false,
		VerifiedByUserID:        pgtype.Text{Valid: false},
		VerifiedAt:              pgtype.Timestamptz{Valid: false},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to register business: %w", err)
	}

	_ = s.audit.Log(ctx, &user.ID, &user.PrimaryRole, "register_marketplace_business", "marketplace_business", b.ID, nil, nil, nil, nil)
	return &b, nil
}

func (s *MarketplaceService) GetBusinessByID(ctx context.Context, id string) (*db.GetMarketplaceBusinessByIDRow, error) {
	b, err := s.repo.Queries().GetMarketplaceBusinessByID(ctx, id)
	if err != nil {
		return nil, ErrBusinessNotFound
	}
	return &b, nil
}

func (s *MarketplaceService) GetBusinessBySlug(ctx context.Context, slug string) (*db.GetMarketplaceBusinessBySlugRow, error) {
	b, err := s.repo.Queries().GetMarketplaceBusinessBySlug(ctx, slug)
	if err != nil {
		return nil, ErrBusinessNotFound
	}
	return &b, nil
}

func (s *MarketplaceService) GetMyBusiness(ctx context.Context, user *db.User) (*db.MarketplaceBusiness, error) {
	b, err := s.repo.Queries().GetMarketplaceBusinessByOwner(ctx, user.ID)
	if err != nil {
		return nil, ErrBusinessNotFound
	}
	return &b, nil
}

func (s *MarketplaceService) ListBusinesses(ctx context.Context, status, category, ownershipType, ownerID *string) ([]db.ListMarketplaceBusinessesRow, error) {
	statusStr := ""
	if status != nil {
		statusStr = *status
	}
	catStr := ""
	if category != nil {
		catStr = *category
	}
	ownStr := ""
	if ownershipType != nil {
		ownStr = *ownershipType
	}
	ownerStr := ""
	if ownerID != nil {
		ownerStr = *ownerID
	}

	list, err := s.repo.Queries().ListMarketplaceBusinesses(ctx, db.ListMarketplaceBusinessesParams{
		Column1: statusStr,
		Column2: catStr,
		Column3: ownStr,
		Column4: ownerStr,
	})
	if err != nil {
		return nil, err
	}
	if list == nil {
		list = []db.ListMarketplaceBusinessesRow{}
	}
	return list, nil
}

func (s *MarketplaceService) UpdateBusiness(ctx context.Context, user *db.User, id string, input UpdateBusinessInput) (*db.MarketplaceBusiness, error) {
	existing, err := s.repo.Queries().GetMarketplaceBusinessByID(ctx, id)
	if err != nil {
		return nil, ErrBusinessNotFound
	}

	// Only owner or national/super admin can update
	if existing.OwnerUserID != user.ID && !isAdmin(user) {
		return nil, ErrUnauthorizedSellerAction
	}

	b, err := s.repo.Queries().UpdateMarketplaceBusiness(ctx, db.UpdateMarketplaceBusinessParams{
		ID:                      id,
		Name:                    input.Name,
		Description:             input.Description,
		Category:                input.Category,
		ContactEmail:            input.ContactEmail,
		ContactPhone:            input.ContactPhone,
		LogoUrl:                 Text(input.LogoURL),
		BannerUrl:               Text(input.BannerURL),
		SettlementBankName:      Text(input.SettlementBankName),
		SettlementAccountNumber: Text(input.SettlementAccountNumber),
		SettlementAccountName:   Text(input.SettlementAccountName),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to update business: %w", err)
	}

	_ = s.audit.Log(ctx, &user.ID, &user.PrimaryRole, "update_marketplace_business", "marketplace_business", id, nil, nil, nil, nil)
	return &b, nil
}

func (s *MarketplaceService) ApproveBusiness(ctx context.Context, admin *db.User, id string, approve bool, reason string) (*db.MarketplaceBusiness, error) {
	if !isAdmin(admin) {
		return nil, errors.New("unauthorized: administrative privilege required to approve businesses")
	}

	newStatus := "approved"
	isVerified := true
	now := time.Now()
	var reasonPtr *string
	if !approve {
		newStatus = "rejected"
		isVerified = false
		if reason != "" {
			reasonPtr = &reason
		}
	}

	b, err := s.repo.Queries().UpdateMarketplaceBusinessStatus(ctx, db.UpdateMarketplaceBusinessStatusParams{
		ID:               id,
		Status:           newStatus,
		IsVerified:       isVerified,
		VerifiedByUserID: TextFromString(admin.ID),
		VerifiedAt:       TimestamptzFromTime(now),
		RejectionReason:  Text(reasonPtr),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to update business approval status: %w", err)
	}

	action := "approve_marketplace_business"
	if !approve {
		action = "reject_marketplace_business"
	}
	_ = s.audit.Log(ctx, &admin.ID, &admin.PrimaryRole, action, "marketplace_business", id, nil, nil, &reason, nil)
	return &b, nil
}

// -------------------------------------------------------------
// Business Branches
// -------------------------------------------------------------

func (s *MarketplaceService) CreateBranch(ctx context.Context, user *db.User, businessID string, input CreateBranchInput) (*db.MarketplaceBusinessBranch, error) {
	b, err := s.repo.Queries().GetMarketplaceBusinessByID(ctx, businessID)
	if err != nil {
		return nil, ErrBusinessNotFound
	}
	if b.OwnerUserID != user.ID && !isAdmin(user) {
		return nil, ErrUnauthorizedSellerAction
	}

	branchID := security.GenerateUUID()
	br, err := s.repo.Queries().CreateBusinessBranch(ctx, db.CreateBusinessBranchParams{
		ID:             branchID,
		BusinessID:     businessID,
		BranchName:     input.BranchName,
		StateID:        input.StateID,
		LgaID:          Text(input.LGAID),
		Address:        input.Address,
		ContactPhone:   input.ContactPhone,
		ManagerName:    Text(input.ManagerName),
		IsHeadquarters: input.IsHeadquarters,
		IsActive:       input.IsActive,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create branch: %w", err)
	}
	return &br, nil
}

func (s *MarketplaceService) ListBranches(ctx context.Context, businessID string) ([]db.ListBusinessBranchesByBusinessRow, error) {
	list, err := s.repo.Queries().ListBusinessBranchesByBusiness(ctx, businessID)
	if err != nil {
		return nil, err
	}
	if list == nil {
		list = []db.ListBusinessBranchesByBusinessRow{}
	}
	return list, nil
}

// -------------------------------------------------------------
// Products
// -------------------------------------------------------------

func (s *MarketplaceService) CreateProduct(ctx context.Context, user *db.User, input CreateProductInput) (*db.MarketplaceProduct, error) {
	if strings.TrimSpace(input.Title) == "" {
		return nil, errors.New("product title is required")
	}
	if input.PriceKobo <= 0 {
		return nil, ErrInvalidPrice
	}

	b, err := s.repo.Queries().GetMarketplaceBusinessByID(ctx, input.BusinessID)
	if err != nil {
		return nil, ErrBusinessNotFound
	}
	if b.OwnerUserID != user.ID && !isAdmin(user) {
		return nil, ErrUnauthorizedSellerAction
	}

	slug := slugify(input.Title)
	if _, err := s.repo.Queries().GetMarketplaceProductBySlug(ctx, slug); err == nil {
		slug = fmt.Sprintf("%s-%s", slug, strings.ToLower(security.GenerateUUID()[:6]))
	}

	if len(input.Images) > storage.MaxImagesPerProduct {
		return nil, storage.ErrTooManyImages
	}
	for _, img := range input.Images {
		if strings.Contains(img, storage.MarketplaceImagePrefix) && !strings.Contains(img, storage.MarketplaceImagePrefix+b.ID+"/") {
			return nil, errors.New("cannot associate product with an image belonging to another business")
		}
	}

	imagesJSON, _ := json.Marshal(input.Images)
	if len(input.Images) == 0 {
		imagesJSON = []byte("[]")
	}

	status := input.Status
	if status == "" {
		status = "published"
	}
	switch status {
	case "draft", "published", "archived":
	default:
		status = "draft"
	}

	productID := security.GenerateUUID()
	var compPrice pgtype.Int8
	if input.CompareAtPriceKobo != nil {
		compPrice = pgtype.Int8{Int64: *input.CompareAtPriceKobo, Valid: true}
	}
	var weight pgtype.Int4
	if input.WeightGrams != nil {
		weight = pgtype.Int4{Int32: *input.WeightGrams, Valid: true}
	}

	p, err := s.repo.Queries().CreateMarketplaceProduct(ctx, db.CreateMarketplaceProductParams{
		ID:                   productID,
		BusinessID:           input.BusinessID,
		BranchID:             Text(input.BranchID),
		Title:                input.Title,
		Slug:                 slug,
		Description:          input.Description,
		Category:             input.Category,
		Sku:                  input.SKU,
		PriceKobo:            input.PriceKobo,
		CompareAtPriceKobo:   compPrice,
		StockQuantity:        input.StockQuantity,
		LowStockThreshold:    input.LowStockThreshold,
		Images:               imagesJSON,
		WeightGrams:          weight,
		DeliveryRestrictions: Text(input.DeliveryRestrictions),
		ReturnPolicy:         Text(input.ReturnPolicy),
		Status:               status,
		IsFeatured:           input.IsFeatured,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create product: %w", err)
	}

	_ = s.audit.Log(ctx, &user.ID, &user.PrimaryRole, "create_marketplace_product", "marketplace_product", p.ID, nil, nil, nil, nil)
	return &p, nil
}

func (s *MarketplaceService) GetProductByID(ctx context.Context, id string) (*db.GetMarketplaceProductByIDRow, error) {
	p, err := s.repo.Queries().GetMarketplaceProductByID(ctx, id)
	if err != nil {
		return nil, ErrProductNotFound
	}
	return &p, nil
}

func (s *MarketplaceService) GetProductBySlug(ctx context.Context, slug string) (*db.GetMarketplaceProductBySlugRow, error) {
	p, err := s.repo.Queries().GetMarketplaceProductBySlug(ctx, slug)
	if err != nil {
		return nil, ErrProductNotFound
	}
	return &p, nil
}

type ProductFilter struct {
	BusinessID     *string
	Category       *string
	Status         *string
	IsFeatured     *bool
	BusinessStatus *string
	SearchQuery    *string
}

func (s *MarketplaceService) ListProducts(ctx context.Context, filter ProductFilter) ([]db.ListMarketplaceProductsRow, error) {
	bID := ""
	if filter.BusinessID != nil {
		bID = *filter.BusinessID
	}
	cat := ""
	if filter.Category != nil {
		cat = *filter.Category
	}
	st := ""
	if filter.Status != nil {
		st = *filter.Status
	}
	bSt := ""
	if filter.BusinessStatus != nil {
		bSt = *filter.BusinessStatus
	}
	q := ""
	if filter.SearchQuery != nil {
		q = *filter.SearchQuery
	}
	featStr := ""
	if filter.IsFeatured != nil {
		if *filter.IsFeatured {
			featStr = "true"
		} else {
			featStr = "false"
		}
	}

	list, err := s.repo.Queries().ListMarketplaceProducts(ctx, db.ListMarketplaceProductsParams{
		Column1: bID,
		Column2: cat,
		Column3: st,
		Column4: featStr,
		Column5: bSt,
		Column6: q,
	})
	if err != nil {
		return nil, err
	}
	if list == nil {
		list = []db.ListMarketplaceProductsRow{}
	}
	return list, nil
}

func (s *MarketplaceService) UpdateProduct(ctx context.Context, user *db.User, id string, input UpdateProductInput) (*db.MarketplaceProduct, error) {
	existing, err := s.repo.Queries().GetMarketplaceProductByID(ctx, id)
	if err != nil {
		return nil, ErrProductNotFound
	}

	// Strictly verify ownership: Seller A cannot update Seller B's product
	if existing.BusinessOwnerID != user.ID && !isAdmin(user) {
		return nil, ErrUnauthorizedSellerAction
	}

	if input.PriceKobo <= 0 {
		return nil, ErrInvalidPrice
	}

	if len(input.Images) > storage.MaxImagesPerProduct {
		return nil, storage.ErrTooManyImages
	}
	for _, img := range input.Images {
		if strings.Contains(img, storage.MarketplaceImagePrefix) && !strings.Contains(img, storage.MarketplaceImagePrefix+existing.BusinessID+"/") {
			return nil, errors.New("cannot associate product with an image belonging to another business")
		}
	}

	imagesJSON, _ := json.Marshal(input.Images)
	if len(input.Images) == 0 {
		imagesJSON = []byte("[]")
	}

	var compPrice pgtype.Int8
	if input.CompareAtPriceKobo != nil {
		compPrice = pgtype.Int8{Int64: *input.CompareAtPriceKobo, Valid: true}
	}
	var weight pgtype.Int4
	if input.WeightGrams != nil {
		weight = pgtype.Int4{Int32: *input.WeightGrams, Valid: true}
	}

	p, err := s.repo.Queries().UpdateMarketplaceProduct(ctx, db.UpdateMarketplaceProductParams{
		ID:                   id,
		BranchID:             Text(input.BranchID),
		Title:                input.Title,
		Description:          input.Description,
		Category:             input.Category,
		Sku:                  input.SKU,
		PriceKobo:            input.PriceKobo,
		CompareAtPriceKobo:   compPrice,
		StockQuantity:        input.StockQuantity,
		LowStockThreshold:    input.LowStockThreshold,
		Images:               imagesJSON,
		WeightGrams:          weight,
		DeliveryRestrictions: Text(input.DeliveryRestrictions),
		ReturnPolicy:         Text(input.ReturnPolicy),
		Status:               input.Status,
		IsFeatured:           input.IsFeatured,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to update product: %w", err)
	}

	_ = s.audit.Log(ctx, &user.ID, &user.PrimaryRole, "update_marketplace_product", "marketplace_product", id, nil, nil, nil, nil)
	return &p, nil
}

func (s *MarketplaceService) DeleteProduct(ctx context.Context, user *db.User, id string) error {
	existing, err := s.repo.Queries().GetMarketplaceProductByID(ctx, id)
	if err != nil {
		return ErrProductNotFound
	}
	if existing.BusinessOwnerID != user.ID && !isAdmin(user) {
		return ErrUnauthorizedSellerAction
	}

	if err := s.repo.Queries().DeleteMarketplaceProduct(ctx, id); err != nil {
		return fmt.Errorf("failed to delete product: %w", err)
	}
	_ = s.audit.Log(ctx, &user.ID, &user.PrimaryRole, "delete_marketplace_product", "marketplace_product", id, nil, nil, nil, nil)
	return nil
}

// -------------------------------------------------------------
// Product Image Upload & Storage Services
// -------------------------------------------------------------

func (s *MarketplaceService) AuthorizeProductImageUpload(ctx context.Context, user *db.User, filename, contentType string, fileSize int64) (*PresignedUploadResult, error) {
	if user == nil {
		return nil, ErrUnauthorizedSellerAction
	}

	b, err := s.repo.Queries().GetMarketplaceBusinessByOwner(ctx, user.ID)
	if err != nil {
		return nil, ErrBusinessNotFound
	}
	if b.Status != "approved" && b.Status != "pending" && !isAdmin(user) {
		return nil, ErrBusinessNotApproved
	}

	canonicalExt, cleanContentType, err := storage.ValidateUploadRequest(filename, contentType, fileSize)
	if err != nil {
		return nil, err
	}

	if !s.storage.IsConfigured() {
		return nil, storage.ErrStorageUnconfigured
	}

	objectKey := storage.GenerateProductImageKey(b.ID, canonicalExt)
	expiry := 15 * time.Minute
	uploadURL, err := s.storage.GeneratePresignedUpload(ctx, objectKey, cleanContentType, fileSize, expiry)
	if err != nil {
		return nil, fmt.Errorf("failed to generate upload authorization: %w", err)
	}

	publicURL := s.storage.GetPublicURL(objectKey)

	_ = s.audit.Log(ctx, &user.ID, &user.PrimaryRole, "authorize_product_image_upload", "marketplace_image", objectKey, nil, nil, nil, nil)

	return &PresignedUploadResult{
		UploadURL:        uploadURL,
		ObjectKey:        objectKey,
		PublicURL:        publicURL,
		ExpiresInSeconds: int64(expiry.Seconds()),
	}, nil
}

func (s *MarketplaceService) UploadProductImageDirect(ctx context.Context, user *db.User, filename string, r io.Reader, size int64) (*UploadedImageResult, error) {
	if user == nil {
		return nil, ErrUnauthorizedSellerAction
	}

	b, err := s.repo.Queries().GetMarketplaceBusinessByOwner(ctx, user.ID)
	if err != nil {
		return nil, ErrBusinessNotFound
	}
	if b.Status != "approved" && b.Status != "pending" && !isAdmin(user) {
		return nil, ErrBusinessNotApproved
	}

	if !s.storage.IsConfigured() {
		return nil, storage.ErrStorageUnconfigured
	}

	imgBytes, meta, err := storage.InspectAndValidateImage(r, size)
	if err != nil {
		return nil, err
	}

	objectKey := storage.GenerateProductImageKey(b.ID, meta.Format)
	publicURL, err := s.storage.UploadObject(ctx, objectKey, meta.ContentType, bytes.NewReader(imgBytes), meta.Size)
	if err != nil {
		return nil, fmt.Errorf("failed to store product image: %w", err)
	}

	_ = s.audit.Log(ctx, &user.ID, &user.PrimaryRole, "upload_product_image_direct", "marketplace_image", objectKey, nil, nil, nil, nil)

	return &UploadedImageResult{
		ObjectKey:   objectKey,
		PublicURL:   publicURL,
		Width:       meta.Width,
		Height:      meta.Height,
		Format:      meta.Format,
		ContentType: meta.ContentType,
		Size:        meta.Size,
	}, nil
}

func (s *MarketplaceService) DeleteProductImage(ctx context.Context, user *db.User, objectKey string) error {
	if user == nil {
		return ErrUnauthorizedSellerAction
	}

	b, err := s.repo.Queries().GetMarketplaceBusinessByOwner(ctx, user.ID)
	if err != nil {
		return ErrBusinessNotFound
	}

	if !storage.ValidateKeyBelongsToBusiness(objectKey, b.ID) && !isAdmin(user) {
		return storage.ErrUnauthorizedKey
	}

	if !s.storage.IsConfigured() {
		return storage.ErrStorageUnconfigured
	}

	if err := s.storage.DeleteObject(ctx, objectKey); err != nil {
		return fmt.Errorf("failed to delete image object: %w", err)
	}

	_ = s.audit.Log(ctx, &user.ID, &user.PrimaryRole, "delete_product_image", "marketplace_image", objectKey, nil, nil, nil, nil)
	return nil
}

// -------------------------------------------------------------
// Cart Validation & Server-Side Calculations
// -------------------------------------------------------------

func (s *MarketplaceService) ValidateCart(ctx context.Context, items []CartItemInput) (*CartValidationResult, error) {
	if len(items) == 0 {
		return &CartValidationResult{
			Items:        []ValidatedCartItem{},
			IsValid:      false,
			ErrorMessage: stringPtr("Cart is empty"),
		}, nil
	}

	var validated []ValidatedCartItem
	var subtotalKobo int64 = 0
	isValid := true
	var errorMsg *string

	for _, itm := range items {
		if itm.Quantity <= 0 {
			isValid = false
			msg := fmt.Sprintf("Invalid quantity for product %s", itm.ProductID)
			errorMsg = &msg
			continue
		}

		p, err := s.repo.Queries().GetMarketplaceProductByID(ctx, itm.ProductID)
		if err != nil {
			isValid = false
			msg := fmt.Sprintf("Product %s no longer exists", itm.ProductID)
			errorMsg = &msg
			continue
		}

		if p.Status != "published" {
			isValid = false
			msg := fmt.Sprintf("Product '%s' is currently unavailable", p.Title)
			errorMsg = &msg
			continue
		}

		isAvailable := p.StockQuantity >= itm.Quantity
		if !isAvailable {
			isValid = false
			msg := fmt.Sprintf("Insufficient stock for '%s' (available: %d, requested: %d)", p.Title, p.StockQuantity, itm.Quantity)
			errorMsg = &msg
		}

		var images []string
		_ = json.Unmarshal(p.Images, &images)
		imgURL := ""
		if len(images) > 0 {
			imgURL = images[0]
		}

		itemSubtotal := p.PriceKobo * int64(itm.Quantity)
		subtotalKobo += itemSubtotal

		validated = append(validated, ValidatedCartItem{
			ProductID:     p.ID,
			BusinessID:    p.BusinessID,
			BusinessName:  p.BusinessName,
			Title:         p.Title,
			SKU:           p.Sku,
			PriceKobo:     p.PriceKobo,
			Quantity:      itm.Quantity,
			SubtotalKobo:  itemSubtotal,
			StockQuantity: p.StockQuantity,
			ImageURL:      imgURL,
			IsAvailable:   isAvailable,
		})
	}

	// Flat standard delivery fee across Nigeria: ₦1,500 = 150000 kobo
	deliveryFeeKobo := int64(150000)
	totalKobo := subtotalKobo + deliveryFeeKobo

	return &CartValidationResult{
		Items:           validated,
		SubtotalKobo:    subtotalKobo,
		DeliveryFeeKobo: deliveryFeeKobo,
		TotalKobo:       totalKobo,
		IsValid:         isValid,
		ErrorMessage:    errorMsg,
	}, nil
}

type CheckoutStatusResponse struct {
	Provider  string `json:"provider"`
	Available bool   `json:"available"`
	Message   string `json:"message"`
}

func (s *MarketplaceService) GetCheckoutStatus(ctx context.Context) CheckoutStatusResponse {
	p := s.paymentGateway.ProviderName()
	if p == "unconfigured" {
		return CheckoutStatusResponse{
			Provider:  p,
			Available: false,
			Message:   "Payment processing is currently undergoing scheduled maintenance. Please check back shortly.",
		}
	}
	return CheckoutStatusResponse{
		Provider:  p,
		Available: true,
		Message:   "Payment processing is online.",
	}
}

// -------------------------------------------------------------
// Transactional Checkout & Order Creation
// -------------------------------------------------------------

func (s *MarketplaceService) CreateOrder(ctx context.Context, buyerUser *db.User, input CheckoutInput) (*CreateOrderResult, error) {
	if s.paymentGateway.ProviderName() == "unconfigured" {
		return nil, payment.ErrPaymentProcessingUnavailable
	}
	if len(input.Items) == 0 {
		return nil, ErrEmptyCart
	}
	if strings.TrimSpace(input.BuyerEmail) == "" || strings.TrimSpace(input.BuyerName) == "" {
		return nil, errors.New("buyer name and email are required")
	}

	// Strictly validate all item quantities: Reject zero or negative quantities
	for _, itm := range input.Items {
		if itm.Quantity <= 0 {
			return nil, fmt.Errorf("invalid quantity %d for product %s; quantity must be at least 1", itm.Quantity, itm.ProductID)
		}
	}

	deliveryType := input.DeliveryType
	if deliveryType != "pickup" {
		deliveryType = "delivery"
	}
	if deliveryType == "delivery" && strings.TrimSpace(input.ShippingAddress) == "" {
		return nil, errors.New("shipping address is required for doorstep delivery")
	}

	// Idempotency protection: Prevent duplicate checkout requests
	if input.IdempotencyKey != "" {
		existingOrder, err := s.repo.Queries().GetMarketplaceOrderByIdempotencyKey(ctx, TextFromString(input.IdempotencyKey))
		if err == nil && existingOrder.ID != "" {
			fullOrder, _ := s.repo.Queries().GetMarketplaceOrderByID(ctx, existingOrder.ID)
			orderItems, _ := s.repo.Queries().ListMarketplaceOrderItemsByOrder(ctx, existingOrder.ID)
			paymentRec, _ := s.repo.Queries().GetMarketplacePaymentByOrder(ctx, existingOrder.ID)
			return &CreateOrderResult{
				Order:       &fullOrder,
				Items:       orderItems,
				Payment:     &paymentRec,
				CheckoutURL: "",
			}, nil
		}
	}

	orderID := security.GenerateUUID()
	orderNumber := fmt.Sprintf("ORD-%s-%s", time.Now().Format("20060102"), strings.ToUpper(security.GenerateUUID()[:8]))

	// Delivery fee in kobo: 0 for pickup, 150,000 (₦1,500) for delivery
	var deliveryFeeKobo int64 = 150000
	if deliveryType == "pickup" {
		deliveryFeeKobo = 0
	}

	var createdOrder db.MarketplaceOrder
	var orderItems []db.ListMarketplaceOrderItemsByOrderRow
	var paymentRec db.MarketplacePayment

	// Execute transactional stock decrement and order creation
	err := s.repo.WithTx(ctx, func(q *db.Queries) error {
		var subtotalKobo int64 = 0

		// Snapshot items and decrement stock
		type itemToCreate struct {
			productID    string
			businessID   string
			title        string
			sku          string
			priceKobo    int64
			quantity     int32
			balanceAfter int32
			subtotal     int64
			imageURL     string
		}
		var preparedItems []itemToCreate

		for _, itm := range input.Items {
			p, err := q.GetMarketplaceProductByID(ctx, itm.ProductID)
			if err != nil {
				return fmt.Errorf("product %s not found: %w", itm.ProductID, err)
			}
			if p.Status != "published" {
				return fmt.Errorf("product '%s' is unavailable", p.Title)
			}

			// Atomic stock decrement with check constraint
			updatedProduct, err := q.DecrementProductStock(ctx, db.DecrementProductStockParams{
				ID:            p.ID,
				StockQuantity: itm.Quantity,
			})
			if err != nil || updatedProduct.StockQuantity < 0 {
				return fmt.Errorf("%w: product '%s' has insufficient stock", ErrInsufficientStock, p.Title)
			}

			itemSub := p.PriceKobo * int64(itm.Quantity)
			subtotalKobo += itemSub

			var images []string
			_ = json.Unmarshal(p.Images, &images)
			img := ""
			if len(images) > 0 {
				img = images[0]
			}

			preparedItems = append(preparedItems, itemToCreate{
				productID:    p.ID,
				businessID:   p.BusinessID,
				title:        p.Title,
				sku:          p.Sku,
				priceKobo:    p.PriceKobo,
				quantity:     itm.Quantity,
				balanceAfter: updatedProduct.StockQuantity,
				subtotal:     itemSub,
				imageURL:     img,
			})
		}

		totalKobo := subtotalKobo + deliveryFeeKobo

		var buyerUserID pgtype.Text
		if buyerUser != nil {
			buyerUserID = TextFromString(buyerUser.ID)
		}

		reservationExpiresAt := time.Now().Add(30 * time.Minute)

		// Create Order
		ord, err := q.CreateMarketplaceOrder(ctx, db.CreateMarketplaceOrderParams{
			ID:                   orderID,
			OrderNumber:          orderNumber,
			BuyerUserID:          buyerUserID,
			BuyerName:            input.BuyerName,
			BuyerEmail:           input.BuyerEmail,
			BuyerPhone:           input.BuyerPhone,
			DeliveryType:         deliveryType,
			ShippingAddress:      input.ShippingAddress,
			ShippingStateID:      Text(input.ShippingStateID),
			ShippingLgaID:        Text(input.ShippingLGAID),
			PickupBranchID:       Text(input.PickupBranchID),
			SubtotalKobo:         subtotalKobo,
			DeliveryFeeKobo:      deliveryFeeKobo,
			TotalKobo:            totalKobo,
			Currency:             "NGN",
			OrderStatus:          "placed",
			PaymentStatus:        "pending",
			FulfilmentStatus:     "unfulfilled",
			IdempotencyKey:       TextFromString(input.IdempotencyKey),
			Notes:                TextFromString(input.Notes),
			ReservationExpiresAt: TimestamptzFromTime(reservationExpiresAt),
		})
		if err != nil {
			return fmt.Errorf("failed to insert order: %w", err)
		}
		createdOrder = ord

		// Create Order Items and Inventory Reservation Logs
		for _, pi := range preparedItems {
			orderItemID := security.GenerateUUID()
			_, err := q.CreateMarketplaceOrderItem(ctx, db.CreateMarketplaceOrderItemParams{
				ID:                orderItemID,
				OrderID:           orderID,
				ProductID:         TextFromString(pi.productID),
				BusinessID:        pi.businessID,
				TitleSnapshot:     pi.title,
				SkuSnapshot:       pi.sku,
				PriceKoboSnapshot: pi.priceKobo,
				Quantity:          pi.quantity,
				SubtotalKobo:      pi.subtotal,
				ImageUrlSnapshot:  TextFromString(pi.imageURL),
			})
			if err != nil {
				return fmt.Errorf("failed to insert order item: %w", err)
			}

			_, err = q.CreateMarketplaceInventoryLog(ctx, db.CreateMarketplaceInventoryLogParams{
				ID:             security.GenerateUUID(),
				ProductID:      pi.productID,
				BusinessID:     pi.businessID,
				ActorUserID:    buyerUserID,
				ActionType:     "reservation",
				QuantityDelta:  -pi.quantity,
				BalanceAfter:   pi.balanceAfter,
				OrderID:        TextFromString(orderID),
				ReferenceID:    orderNumber,
				Reason:         "Order checkout reservation",
				IdempotencyKey: TextFromString(fmt.Sprintf("reserve:%s:%s", orderID, orderItemID)),
			})
			if err != nil {
				return fmt.Errorf("failed to record inventory reservation: %w", err)
			}
		}

		// Initialize Payment Record
		paymentRef := fmt.Sprintf("PAY-%s", orderNumber)
		payRec, err := q.CreateMarketplacePayment(ctx, db.CreateMarketplacePaymentParams{
			ID:                security.GenerateUUID(),
			OrderID:           orderID,
			Reference:         paymentRef,
			Provider:          s.paymentGateway.ProviderName(),
			ProviderReference: TextFromString(""),
			Channel:           TextFromString("card"),
			AmountKobo:        totalKobo,
			Currency:          "NGN",
			Status:            "pending",
			PaidAt:            pgtype.Timestamptz{Valid: false},
			RawWebhookPayload: []byte("{}"),
			ErrorMessage:      TextFromString(""),
		})
		if err != nil {
			return fmt.Errorf("failed to insert payment record: %w", err)
		}
		paymentRec = payRec

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Initialize with payment gateway
	initOut, err := s.paymentGateway.InitializePayment(ctx, payment.InitializePaymentInput{
		OrderReference: paymentRec.Reference,
		AmountKobo:     createdOrder.TotalKobo,
		Currency:       "NGN",
		CustomerEmail:  input.BuyerEmail,
		CustomerName:   input.BuyerName,
		CallbackURL:    input.CallbackURL,
		Metadata: map[string]interface{}{
			"order_id":     createdOrder.ID,
			"order_number": createdOrder.OrderNumber,
		},
	})
	if err != nil {
		// Rollback stock reservation and mark order cancelled to prevent dangling reserved stock
		_ = s.repo.WithTx(ctx, func(q *db.Queries) error {
			prodRollback := make(map[string]int32)
			for _, itm := range input.Items {
				if itm.Quantity > 0 {
					prodRollback[itm.ProductID] += itm.Quantity
				}
			}
			for pID, qty := range prodRollback {
				incProd, incErr := q.IncrementProductStock(ctx, db.IncrementProductStockParams{
					ID:            pID,
					StockQuantity: qty,
				})
				if incErr == nil {
					var prodBusID string
					p, pErr := q.GetMarketplaceProductByID(ctx, pID)
					if pErr == nil {
						prodBusID = p.BusinessID
					}
					var actorID pgtype.Text
					if buyerUser != nil {
						actorID = TextFromString(buyerUser.ID)
					}
					_, logErr := q.CreateMarketplaceInventoryLog(ctx, db.CreateMarketplaceInventoryLogParams{
						ID:             security.GenerateUUID(),
						ProductID:      pID,
						BusinessID:     prodBusID,
						ActorUserID:    actorID,
						ActionType:     "reservation_release",
						QuantityDelta:  qty,
						BalanceAfter:   incProd.StockQuantity,
						OrderID:        TextFromString(createdOrder.ID),
						ReferenceID:    createdOrder.OrderNumber,
						Reason:         "Gateway initialization failure rollback",
						IdempotencyKey: TextFromString(fmt.Sprintf("release:%s:%s", createdOrder.ID, pID)),
					})
					if logErr != nil {
						return fmt.Errorf("failed to record inventory reservation release during rollback: %w", logErr)
					}
				}
			}
			_, _ = q.MarkOrderCancelledWithReason(ctx, db.MarkOrderCancelledWithReasonParams{
				ID:                 createdOrder.ID,
				CancellationReason: "gateway_initialization_failed",
				StockReleased:      true,
			})
			_, _ = q.UpdateMarketplaceOrderPaymentStatus(ctx, db.UpdateMarketplaceOrderPaymentStatusParams{
				ID:            createdOrder.ID,
				PaymentStatus: "failed",
			})
			return nil
		})
		return nil, fmt.Errorf("payment gateway initialization failed: %w", err)
	}

	fullOrder, _ := s.repo.Queries().GetMarketplaceOrderByID(ctx, createdOrder.ID)
	orderItems, _ = s.repo.Queries().ListMarketplaceOrderItemsByOrder(ctx, createdOrder.ID)

	var actorID *string
	if buyerUser != nil {
		actorID = &buyerUser.ID
	}
	_ = s.audit.Log(ctx, actorID, nil, "create_marketplace_order", "marketplace_order", createdOrder.ID, nil, nil, nil, nil)

	return &CreateOrderResult{
		Order:       &fullOrder,
		Items:       orderItems,
		Payment:     &paymentRec,
		CheckoutURL: initOut.CheckoutURL,
	}, nil
}

// -------------------------------------------------------------
// Payment Confirmation & Webhooks
// -------------------------------------------------------------

func (s *MarketplaceService) ConfirmPayment(ctx context.Context, reference string) (*db.GetMarketplaceOrderByIDRow, error) {
	if s.paymentGateway.ProviderName() == "unconfigured" {
		return nil, payment.ErrPaymentProcessingUnavailable
	}

	// Look up payment by reference
	payRec, err := s.repo.Queries().GetMarketplacePaymentByReference(ctx, reference)
	if err != nil {
		return nil, ErrPaymentNotFound
	}

	order, err := s.repo.Queries().GetMarketplaceOrderByID(ctx, payRec.OrderID)
	if err != nil {
		return nil, ErrOrderNotFound
	}

	// Idempotency: if already paid or refunded, return early
	if payRec.Status == "successful" && (order.PaymentStatus == "paid" || order.PaymentStatus == "partially_refunded" || order.PaymentStatus == "refunded") {
		return &order, nil
	}

	// Validate expected payment provider
	if payRec.Provider != s.paymentGateway.ProviderName() {
		return nil, fmt.Errorf("%w: expected provider '%s', got '%s'",
			payment.ErrProviderMismatch, payRec.Provider, s.paymentGateway.ProviderName())
	}

	// Verify payment with authoritative provider (outside any DB transaction)
	verifyOut, err := s.paymentGateway.VerifyPayment(ctx, reference)
	if err != nil {
		return nil, fmt.Errorf("payment verification failed: %w", err)
	}

	if verifyOut.Status != "successful" {
		// Non-terminal status check: Do NOT mark payment as failed if status is pending, processing, queued, etc.
		isTerminalFailure := verifyOut.Status == "failed" || verifyOut.Status == "abandoned" || verifyOut.Status == "reversed"
		if !isTerminalFailure {
			// Status is pending/in-progress: preserve reservation and return clear pending error
			return nil, fmt.Errorf("payment verification pending (status: %s)", verifyOut.Status)
		}

		// Confirmed terminal failure: record failed status safely without overwriting already-paid orders
		_ = s.repo.WithTx(ctx, func(q *db.Queries) error {
			lockedPay, err := q.GetMarketplacePaymentByReferenceForUpdate(ctx, reference)
			if err != nil {
				return err
			}
			lockedOrder, err := q.GetMarketplaceOrderForUpdate(ctx, lockedPay.OrderID)
			if err != nil {
				return err
			}

			// Never overwrite an already-paid, partially refunded, or refunded order or payment
			if lockedOrder.PaymentStatus == "paid" || lockedOrder.PaymentStatus == "partially_refunded" || lockedOrder.PaymentStatus == "refunded" || lockedPay.Status == "successful" {
				return nil
			}

			rawPayload := verifyOut.RawResponse
			if len(rawPayload) == 0 {
				rawPayload = []byte("{}")
			}

			_, err = q.UpdateMarketplacePaymentTransactionStatus(ctx, db.UpdateMarketplacePaymentTransactionStatusParams{
				Reference:         reference,
				Status:            "failed",
				ProviderReference: TextFromString(verifyOut.ProviderReference),
				Channel:           TextFromString(verifyOut.Channel),
				PaidAt:            pgtype.Timestamptz{Valid: false},
				RawWebhookPayload: rawPayload,
				ErrorMessage:      TextFromString("Payment status returned: " + verifyOut.Status),
			})
			if err != nil {
				return err
			}

			if lockedOrder.PaymentStatus == "pending" && lockedOrder.OrderStatus != "cancelled" && lockedOrder.OrderStatus != "refunded" {
				_, err = q.UpdateMarketplaceOrderPaymentStatus(ctx, db.UpdateMarketplaceOrderPaymentStatusParams{
					ID:            lockedOrder.ID,
					PaymentStatus: "failed",
				})
				if err != nil {
					return err
				}
			}
			return nil
		})
		return nil, fmt.Errorf("payment was not successful (status: %s)", verifyOut.Status)
	}

	// Strict Paystack verification:
	// 1. Returned payment reference must match expected reference exactly
	if verifyOut.PaymentReference != reference {
		return nil, fmt.Errorf("%w: expected reference %s, got %s", payment.ErrReferenceMismatch, reference, verifyOut.PaymentReference)
	}

	// 2. Verified amount must equal expected order total exactly (rejects both underpayment and overpayment)
	if verifyOut.AmountKobo != order.TotalKobo {
		return nil, fmt.Errorf("%w: expected %d kobo, got %d kobo", payment.ErrAmountMismatch, order.TotalKobo, verifyOut.AmountKobo)
	}

	// 3. Reject missing or mismatched currencies
	if strings.TrimSpace(verifyOut.Currency) == "" {
		return nil, fmt.Errorf("%w: verified currency is missing", payment.ErrCurrencyMismatch)
	}
	if !strings.EqualFold(verifyOut.Currency, order.Currency) {
		return nil, fmt.Errorf("%w: expected %s, got %s", payment.ErrCurrencyMismatch, order.Currency, verifyOut.Currency)
	}

	// Atomic transaction with row-level locks
	err = s.repo.WithTx(ctx, func(q *db.Queries) error {
		lockedPay, err := q.GetMarketplacePaymentByReferenceForUpdate(ctx, reference)
		if err != nil {
			return fmt.Errorf("failed to lock payment record: %w", err)
		}

		lockedOrder, err := q.GetMarketplaceOrderForUpdate(ctx, lockedPay.OrderID)
		if err != nil {
			return fmt.Errorf("failed to lock order record: %w", err)
		}

		// Concurrency protection: if already marked paid / refunded by a concurrent request, no-op
		if lockedPay.Status == "successful" && (lockedOrder.PaymentStatus == "paid" || lockedOrder.PaymentStatus == "partially_refunded" || lockedOrder.PaymentStatus == "refunded") {
			return nil
		}

		now := time.Now()
		paidAt := TimestamptzFromTime(now)
		if verifyOut.PaidAt != nil {
			paidAt = TimestamptzFromTime(*verifyOut.PaidAt)
		}
		rawPayload := verifyOut.RawResponse
		if len(rawPayload) == 0 {
			rawPayload = []byte("{}")
		}

		// Delayed payment check: If stock was already released (e.g. reservation expired or cancelled)
		if lockedOrder.StockReleased || lockedOrder.OrderStatus == "cancelled" {
			// Mark payment as paid/successful and flag order as requires_reconciliation
			_, err = q.UpdateMarketplacePaymentTransactionStatus(ctx, db.UpdateMarketplacePaymentTransactionStatusParams{
				Reference:         reference,
				Status:            "successful",
				ProviderReference: TextFromString(verifyOut.ProviderReference),
				Channel:           TextFromString(verifyOut.Channel),
				PaidAt:            paidAt,
				RawWebhookPayload: rawPayload,
				ErrorMessage:      TextFromString(""),
			})
			if err != nil {
				return fmt.Errorf("failed to update payment record: %w", err)
			}

			_, err = q.UpdateMarketplaceOrderPaymentStatus(ctx, db.UpdateMarketplaceOrderPaymentStatusParams{
				ID:            lockedOrder.ID,
				PaymentStatus: "paid",
			})
			if err != nil {
				return fmt.Errorf("failed to update order payment status: %w", err)
			}

			notes := "Payment confirmed after stock reservation expired and was released. Administrative action required."
			targetOrderStatus := "requires_reconciliation"
			if lockedOrder.OrderStatus == "cancelled" && lockedOrder.CancellationReason != "payment_timeout" {
				targetOrderStatus = "cancelled"
				notes = "Payment confirmed after order was cancelled. Administrative action required."
			}

			if targetOrderStatus == "requires_reconciliation" {
				_, err = q.MarkOrderRequiresReconciliation(ctx, db.MarkOrderRequiresReconciliationParams{
					ID:                  lockedOrder.ID,
					ReconciliationNotes: notes,
				})
			} else {
				_, err = q.UpdateMarketplaceOrderExpiryFields(ctx, db.UpdateMarketplaceOrderExpiryFieldsParams{
					ID:                     lockedOrder.ID,
					ReservationExpiresAt:   lockedOrder.ReservationExpiresAt,
					CancellationReason:     lockedOrder.CancellationReason,
					StockReleased:          lockedOrder.StockReleased,
					ReconciliationRequired: true,
					ReconciliationNotes:    notes,
				})
			}
			if err != nil {
				return fmt.Errorf("failed to update reconciliation fields on order: %w", err)
			}

			return nil
		}

		// Update order payment status to paid (unless already partially refunded or refunded)
		if lockedOrder.PaymentStatus != "refunded" && lockedOrder.PaymentStatus != "partially_refunded" {
			_, err = q.UpdateMarketplaceOrderPaymentStatus(ctx, db.UpdateMarketplaceOrderPaymentStatusParams{
				ID:            lockedOrder.ID,
				PaymentStatus: "paid",
			})
			if err != nil {
				return fmt.Errorf("failed to update order payment status: %w", err)
			}
		}

		// Order processing status advances ONLY when the transition is valid:
		// Do not automatically reset cancelled, fulfilled, or disputed orders to processing.
		if lockedOrder.OrderStatus == "placed" && lockedOrder.FulfilmentStatus == "unfulfilled" {
			disputes, err := q.ListMarketplaceDisputesByOrder(ctx, lockedOrder.ID)
			hasOpenDispute := false
			if err == nil {
				for _, d := range disputes {
					if d.Status == "open" {
						hasOpenDispute = true
						break
					}
				}
			}
			if !hasOpenDispute {
				_, err = q.UpdateMarketplaceOrderStatus(ctx, db.UpdateMarketplaceOrderStatusParams{
					ID:          lockedOrder.ID,
					OrderStatus: "processing",
				})
				if err != nil {
					return fmt.Errorf("failed to advance order status: %w", err)
				}
			}
		}

		// Record sale confirmation in inventory ledger (quantity delta is 0; stock was reserved at order creation)
		items, err := q.ListMarketplaceOrderItemsByOrder(ctx, lockedOrder.ID)
		if err == nil {
			for _, itm := range items {
				if itm.ProductID.Valid {
					p, pErr := q.GetMarketplaceProductByID(ctx, itm.ProductID.String)
					if pErr == nil {
						idempotencyKey := fmt.Sprintf("sale_confirmed:%s:%s", lockedOrder.ID, itm.ProductID.String)
						_, logErr := q.CreateMarketplaceInventoryLog(ctx, db.CreateMarketplaceInventoryLogParams{
							ID:             security.GenerateUUID(),
							ProductID:      p.ID,
							BusinessID:     p.BusinessID,
							ActorUserID:    lockedOrder.BuyerUserID,
							ActionType:     "sale_confirmed",
							QuantityDelta:  0,
							BalanceAfter:   p.StockQuantity,
							OrderID:        TextFromString(lockedOrder.ID),
							ReferenceID:    reference,
							Reason:         "Payment confirmed",
							IdempotencyKey: TextFromString(idempotencyKey),
						})
						if logErr != nil {
							return fmt.Errorf("failed to record sale confirmation inventory log: %w", logErr)
						}
					}
				}
			}
		}

		_, err = q.UpdateMarketplacePaymentTransactionStatus(ctx, db.UpdateMarketplacePaymentTransactionStatusParams{
			Reference:         reference,
			Status:            "successful",
			ProviderReference: TextFromString(verifyOut.ProviderReference),
			Channel:           TextFromString(verifyOut.Channel),
			PaidAt:            paidAt,
			RawWebhookPayload: rawPayload,
			ErrorMessage:      TextFromString(""),
		})
		if err != nil {
			return fmt.Errorf("failed to update payment record: %w", err)
		}

		// Automatically create seller-specific shipments for eligible paid order
		_, sErr := s.createShipmentsForOrderTx(ctx, q, lockedOrder.ID)
		if sErr != nil {
			return fmt.Errorf("failed to create vendor shipments: %w", sErr)
		}
		_ = s.aggregateOrderStatusTx(ctx, q, lockedOrder.ID)

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to atomically confirm payment: %w", err)
	}

	refreshedOrder, err := s.repo.Queries().GetMarketplaceOrderByID(ctx, payRec.OrderID)
	if err != nil {
		return nil, fmt.Errorf("failed to reload order: %w", err)
	}

	if refreshedOrder.OrderStatus == "requires_reconciliation" {
		_ = s.audit.Log(ctx, nil, nil, "order_delayed_payment_reconciliation_required", "marketplace_order", refreshedOrder.ID, nil, nil, nil, nil)
	} else {
		_ = s.audit.Log(ctx, nil, nil, "confirm_marketplace_payment", "marketplace_payment", reference, nil, nil, nil, nil)
	}

	return &refreshedOrder, nil
}

func (s *MarketplaceService) HandleWebhook(ctx context.Context, signatureHeader string, payload []byte) error {
	if !s.paymentGateway.VerifyWebhookSignature(signatureHeader, payload) {
		return payment.ErrInvalidSignature
	}

	var event struct {
		Event string `json:"event"`
		Data  struct {
			Reference            string `json:"reference"`
			TransactionReference string `json:"transaction_reference"`
			RefundReference      string `json:"refund_reference"`
			Status               string `json:"status"`
			Amount               int64  `json:"amount"`
			Currency             string `json:"currency"`
		} `json:"data"`
	}

	if err := json.Unmarshal(payload, &event); err != nil {
		return fmt.Errorf("failed to decode webhook event: %w", err)
	}

	// 1. Refund Events (Paystack refund.processed, refund.successful, and refund.failed)
	if event.Event == "refund.processed" || event.Event == "refund.successful" {
		refundRef := strings.TrimSpace(event.Data.RefundReference)
		if refundRef == "" {
			refundRef = strings.TrimSpace(event.Data.Reference)
		}
		if refundRef == "" {
			// Handle missing refund identifier safely without changing refund status
			return nil
		}

		// Match refund event strictly to a unique provider refund identifier (no ambiguous fallback)
		refund, err := s.repo.Queries().GetMarketplaceRefundByProviderReference(ctx, refundRef)
		if err != nil {
			// Unknown or mismatched refund identifier: handle safely without changing refund status
			return nil
		}

		// Validate the refund belongs to the expected payment and order
		pay, pErr := s.repo.Queries().GetMarketplacePaymentByOrder(ctx, refund.OrderID)
		if pErr != nil {
			return nil
		}
		if refund.PaymentID != pay.ID {
			return errors.New("webhook refund payment ID does not match order payment")
		}
		txnRef := strings.TrimSpace(event.Data.TransactionReference)
		if txnRef != "" && pay.Reference != "" && txnRef != pay.Reference && (!pay.ProviderReference.Valid || txnRef != pay.ProviderReference.String) {
			return errors.New("webhook transaction reference does not match refund payment")
		}

		// Verify amount and currency when supplied by the provider
		if event.Data.Amount > 0 && event.Data.Amount != refund.AmountKobo {
			return fmt.Errorf("webhook refund amount %d does not match expected refund amount %d", event.Data.Amount, refund.AmountKobo)
		}
		if event.Data.Currency != "" && pay.Currency != "" && !strings.EqualFold(event.Data.Currency, pay.Currency) {
			return fmt.Errorf("webhook refund currency %s does not match expected payment currency %s", event.Data.Currency, pay.Currency)
		}

		// Do not treat a pending, processing, or accepted refund as successfully settled
		provStatus := strings.ToLower(event.Data.Status)
		if provStatus == "pending" || provStatus == "processing" || provStatus == "accepted" {
			return nil
		}

		// State transition and idempotency checks
		if refund.Status == "successful" {
			// Idempotent duplicate: harmless no-op
			return nil
		}
		if refund.Status == "failed" {
			// Failed refunds must not be silently converted to successful
			return errors.New("cannot convert failed refund to successful")
		}

		_, _, err = s.settleRefundSuccess(ctx, nil, refund.ID, refundRef, "Confirmed via Paystack webhook: "+event.Event, payload)
		return err
	}

	if event.Event == "refund.failed" {
		refundRef := strings.TrimSpace(event.Data.RefundReference)
		if refundRef == "" {
			refundRef = strings.TrimSpace(event.Data.Reference)
		}
		if refundRef == "" {
			return nil
		}

		refund, err := s.repo.Queries().GetMarketplaceRefundByProviderReference(ctx, refundRef)
		if err != nil {
			return nil
		}

		// Validate the refund belongs to the expected payment and order
		pay, pErr := s.repo.Queries().GetMarketplacePaymentByOrder(ctx, refund.OrderID)
		if pErr != nil {
			return nil
		}
		if refund.PaymentID != pay.ID {
			return errors.New("webhook refund payment ID does not match order payment")
		}
		txnRef := strings.TrimSpace(event.Data.TransactionReference)
		if txnRef != "" && pay.Reference != "" && txnRef != pay.Reference && (!pay.ProviderReference.Valid || txnRef != pay.ProviderReference.String) {
			return errors.New("webhook transaction reference does not match refund payment")
		}

		// State transition and idempotency checks
		if refund.Status == "failed" {
			// Idempotent duplicate: harmless no-op
			return nil
		}
		if refund.Status == "successful" {
			// Successful refunds must NEVER be downgraded
			return nil
		}

		_, _, err = s.FailRefund(ctx, nil, refund.ID, "Refund failed at provider: "+event.Event)
		return err
	}

	// 2. Charge Events
	if event.Event == "charge.success" || (event.Event == "" && (event.Data.Status == "success" || event.Data.Status == "successful")) {
		ref := event.Data.Reference
		if ref == "" {
			return errors.New("webhook event missing reference")
		}
		_, err := s.ConfirmPayment(ctx, ref)
		return err
	}

	// For failed payment events, record failure authoritatively without overwriting successful payments
	if event.Event == "charge.failed" || (event.Event == "" && event.Data.Status == "failed") {
		ref := event.Data.Reference
		if ref != "" {
			_, _ = s.ConfirmPayment(ctx, ref)
		}
		return nil
	}

	return nil
}

// -------------------------------------------------------------
// Buyer & Seller Order Queries
// -------------------------------------------------------------

func (s *MarketplaceService) GetOrderByID(ctx context.Context, user *db.User, id string) (*db.GetMarketplaceOrderByIDRow, []db.ListMarketplaceOrderItemsByOrderRow, *db.MarketplaceShipment, error) {
	if user == nil {
		return nil, nil, nil, ErrUnauthorizedBuyerAction
	}

	o, err := s.repo.Queries().GetMarketplaceOrderByID(ctx, id)
	if err != nil {
		return nil, nil, nil, ErrOrderNotFound
	}

	isBuyer := o.BuyerUserID.Valid && o.BuyerUserID.String == user.ID

	apps := s.getActiveAppointments(ctx, user.ID)
	orderStateID, orderZoneID, orderLgaID := s.resolveOrderGeographyWithLga(ctx, s.repo.Queries(), &db.MarketplaceOrder{
		ShippingStateID: o.ShippingStateID,
		ShippingLgaID:   o.ShippingLgaID,
		PickupBranchID:  o.PickupBranchID,
	})
	isNationwide := s.isNationwideAdmin(user, apps)
	hasJurisdiction := s.hasCoordinatorJurisdictionWithLga(ctx, user, apps, orderStateID, orderZoneID, orderLgaID)
	hasCoordRole := s.isCoordinatorUser(user, apps)

	if !isBuyer && !isNationwide {
		if hasCoordRole && !hasJurisdiction {
			return nil, nil, nil, ErrScopeUnauthorized
		}
	}

	rawItems, err := s.repo.Queries().ListMarketplaceOrderItemsByOrder(ctx, id)
	if err != nil {
		return nil, nil, nil, err
	}
	if rawItems == nil {
		rawItems = []db.ListMarketplaceOrderItemsByOrderRow{}
	}

	items := rawItems

	// Ownership check if authenticated user is seller (and not buyer, and not authorized admin)
	if !isBuyer && !isNationwide && !hasJurisdiction {
		var filtered []db.ListMarketplaceOrderItemsByOrderRow
		for _, item := range rawItems {
			b, err := s.repo.Queries().GetMarketplaceBusinessByID(ctx, item.BusinessID)
			if err == nil && b.OwnerUserID == user.ID {
				filtered = append(filtered, item)
			}
		}
		if len(filtered) == 0 {
			return nil, nil, nil, ErrUnauthorizedSellerAction
		}
		items = filtered

		// Customer privacy: minimize customer personal info for seller view
		o.BuyerEmail = "[REDACTED]"
		o.BuyerUserID = pgtype.Text{Valid: false}
		if o.DeliveryType == "pickup" {
			o.ShippingAddress = ""
			o.ShippingStateID = pgtype.Text{Valid: false}
			o.ShippingLgaID = pgtype.Text{Valid: false}
		}
	}

	var shipmentPtr *db.MarketplaceShipment
	if o.OrderStatus != "cancelled" {
		sh, err := s.repo.Queries().GetMarketplaceShipmentByOrder(ctx, id)
		if err == nil && sh.ID != "" && sh.Status != "cancelled" {
			shipmentPtr = &sh
		}
	}

	return &o, items, shipmentPtr, nil
}

func (s *MarketplaceService) GetOrderByOrderNumber(ctx context.Context, orderNumber string) (*db.GetMarketplaceOrderByOrderNumberRow, []db.ListMarketplaceOrderItemsByOrderRow, error) {
	o, err := s.repo.Queries().GetMarketplaceOrderByOrderNumber(ctx, orderNumber)
	if err != nil {
		return nil, nil, ErrOrderNotFound
	}
	items, err := s.repo.Queries().ListMarketplaceOrderItemsByOrder(ctx, o.ID)
	if err != nil {
		return nil, nil, err
	}
	if items == nil {
		items = []db.ListMarketplaceOrderItemsByOrderRow{}
	}
	return &o, items, nil
}

func (s *MarketplaceService) ListOrdersByBuyer(ctx context.Context, buyerUser *db.User) ([]db.ListMarketplaceOrdersByBuyerRow, error) {
	list, err := s.repo.Queries().ListMarketplaceOrdersByBuyer(ctx, TextFromString(buyerUser.ID))
	if err != nil {
		return nil, err
	}
	if list == nil {
		list = []db.ListMarketplaceOrdersByBuyerRow{}
	}
	return list, nil
}

func (s *MarketplaceService) ListOrdersByBusiness(ctx context.Context, user *db.User, businessID string, status *string) ([]db.ListMarketplaceOrdersByBusinessRow, error) {
	if user == nil {
		return nil, errors.New("authentication required")
	}
	b, err := s.repo.Queries().GetMarketplaceBusinessByID(ctx, businessID)
	if err != nil {
		return nil, ErrBusinessNotFound
	}
	apps := s.getActiveAppointments(ctx, user.ID)
	isNationwide := s.isNationwideAdmin(user, apps)
	if b.OwnerUserID != user.ID && !isNationwide {
		return nil, ErrUnauthorizedSellerAction
	}

	statusStr := ""
	if status != nil {
		statusStr = *status
	}

	list, err := s.repo.Queries().ListMarketplaceOrdersByBusiness(ctx, db.ListMarketplaceOrdersByBusinessParams{
		BusinessID: businessID,
		Column2:    statusStr,
	})
	if err != nil {
		return nil, err
	}
	if list == nil {
		list = []db.ListMarketplaceOrdersByBusinessRow{}
	}

	// Customer privacy: sanitize PII for seller-facing responses
	if !isNationwide {
		for i := range list {
			list[i].BuyerEmail = "[REDACTED]"
			list[i].BuyerUserID = pgtype.Text{Valid: false}
			if list[i].DeliveryType == "pickup" {
				list[i].ShippingAddress = ""
				list[i].ShippingStateID = pgtype.Text{Valid: false}
				list[i].ShippingLgaID = pgtype.Text{Valid: false}
			}
		}
	}

	return list, nil
}

func (s *MarketplaceService) ListAllOrders(ctx context.Context, admin *db.User, orderStatus, paymentStatus, fulfilmentStatus *string) ([]db.ListAllMarketplaceOrdersRow, error) {
	if admin == nil {
		return nil, errors.New("authentication required")
	}

	apps := s.getActiveAppointments(ctx, admin.ID)
	isNationwide := s.isNationwideAdmin(admin, apps)
	hasCoord := s.isCoordinatorUser(admin, apps)

	if !isNationwide && !hasCoord {
		return nil, errors.New("unauthorized: administrative permissions required")
	}

	ordSt := ""
	if orderStatus != nil {
		ordSt = *orderStatus
	}
	paySt := ""
	if paymentStatus != nil {
		paySt = *paymentStatus
	}
	fulSt := ""
	if fulfilmentStatus != nil {
		fulSt = *fulfilmentStatus
	}

	list, err := s.repo.Queries().ListAllMarketplaceOrders(ctx, db.ListAllMarketplaceOrdersParams{
		Column1: ordSt,
		Column2: paySt,
		Column3: fulSt,
	})
	if err != nil {
		return nil, err
	}
	if list == nil {
		list = []db.ListAllMarketplaceOrdersRow{}
	}

	if isNationwide {
		return list, nil
	}

	// Filter by coordinator jurisdiction with LGA support
	var filtered []db.ListAllMarketplaceOrdersRow
	for _, row := range list {
		orderStateID, orderZoneID, orderLgaID := s.resolveOrderGeographyWithLga(ctx, s.repo.Queries(), &db.MarketplaceOrder{
			ShippingStateID: row.ShippingStateID,
			ShippingLgaID:   row.ShippingLgaID,
			PickupBranchID:  row.PickupBranchID,
		})
		if s.hasCoordinatorJurisdictionWithLga(ctx, admin, apps, orderStateID, orderZoneID, orderLgaID) {
			filtered = append(filtered, row)
		}
	}
	if filtered == nil {
		filtered = []db.ListAllMarketplaceOrdersRow{}
	}
	return filtered, nil
}

// -------------------------------------------------------------
// Fulfilment & Delivery (Multi-Vendor Consignments)
// -------------------------------------------------------------

type DispatchOrderInput struct {
	CarrierName           string     `json:"carrier_name"`
	TrackingNumber        string     `json:"tracking_number"`
	TrackingURL           string     `json:"tracking_url"`
	EstimatedDeliveryDate *time.Time `json:"estimated_delivery_date"`
	DeliveryNotes         string     `json:"delivery_notes"`
	Notes                 string     `json:"notes,omitempty"`
}

type UpdateShipmentTrackingInput struct {
	CarrierName    string `json:"carrier_name"`
	TrackingNumber string `json:"tracking_number"`
	TrackingURL    string `json:"tracking_url"`
	DeliveryNotes  string `json:"delivery_notes"`
}

type ShipmentItemDetail struct {
	ID                string      `json:"id"`
	ShipmentID        string      `json:"shipment_id"`
	OrderItemID       string      `json:"order_item_id"`
	Quantity          int32       `json:"quantity"`
	ProductID         pgtype.Text `json:"product_id"`
	TitleSnapshot     string      `json:"title_snapshot"`
	SkuSnapshot       string      `json:"sku_snapshot"`
	PriceKoboSnapshot int64       `json:"price_kobo_snapshot"`
	ImageUrlSnapshot  pgtype.Text `json:"image_url_snapshot"`
	BusinessID        string      `json:"business_id"`
}

type ShipmentDetailResult struct {
	Shipment db.MarketplaceShipment  `json:"shipment"`
	Order    *db.MarketplaceOrder    `json:"order,omitempty"`
	Business *db.MarketplaceBusiness `json:"business,omitempty"`
	Items    []ShipmentItemDetail    `json:"items"`
}

// createShipmentsForOrderTx idempotently groups order items by seller and creates one shipment per seller
func (s *MarketplaceService) createShipmentsForOrderTx(ctx context.Context, q *db.Queries, orderID string) ([]db.MarketplaceShipment, error) {
	lockedOrder, err := q.GetMarketplaceOrderForUpdate(ctx, orderID)
	if err != nil {
		return nil, err
	}

	// Never create dispatchable shipments for unpaid orders
	if lockedOrder.PaymentStatus != "paid" && lockedOrder.PaymentStatus != "partially_refunded" {
		return nil, ErrOrderNotPaid
	}

	// Block shipment fulfillment for orders requiring reconciliation or whose stock has been released
	if lockedOrder.OrderStatus == "requires_reconciliation" || lockedOrder.ReconciliationRequired {
		return nil, ErrOrderReconciliationRequired
	}
	if lockedOrder.StockReleased {
		return nil, ErrOrderStockAlreadyReleased
	}
	if lockedOrder.OrderStatus == "cancelled" || lockedOrder.OrderStatus == "refunded" {
		return nil, fmt.Errorf("cannot create shipments for %s order", lockedOrder.OrderStatus)
	}

	items, err := q.ListMarketplaceOrderItemsByOrder(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, errors.New("cannot create shipments for order without items")
	}

	// Group order items by business_id
	businessItems := make(map[string][]db.ListMarketplaceOrderItemsByOrderRow)
	for _, itm := range items {
		businessItems[itm.BusinessID] = append(businessItems[itm.BusinessID], itm)
	}

	var createdShipments []db.MarketplaceShipment
	for busID, bItems := range businessItems {
		// Idempotency check: see if shipment already exists for this order and business
		sh, err := q.GetMarketplaceShipmentByOrderAndBusiness(ctx, db.GetMarketplaceShipmentByOrderAndBusinessParams{
			OrderID:    orderID,
			BusinessID: TextFromString(busID),
		})
		if err != nil {
			// Create shipment for this seller
			shID := security.GenerateUUID()
			newSh, cErr := q.CreateMarketplaceShipment(ctx, db.CreateMarketplaceShipmentParams{
				ID:                  shID,
				OrderID:             orderID,
				BusinessID:          TextFromString(busID),
				CarrierName:         "",
				TrackingNumber:      "",
				TrackingUrl:         TextFromString(""),
				Status:              "pending",
				DispatchedAt:        pgtype.Timestamptz{Valid: false},
				EstimatedDeliveryAt: pgtype.Timestamptz{Valid: false},
				DeliveredAt:         pgtype.Timestamptz{Valid: false},
				DeliveryNotes:       TextFromString(""),
				ProofOfDeliveryUrl:  TextFromString(""),
			})
			if cErr != nil {
				return nil, fmt.Errorf("failed to create vendor shipment: %w", cErr)
			}
			sh = newSh
		}

		// Link shipment items
		for _, itm := range bItems {
			_, _ = q.CreateMarketplaceShipmentItem(ctx, db.CreateMarketplaceShipmentItemParams{
				ID:          security.GenerateUUID(),
				ShipmentID:  sh.ID,
				OrderItemID: itm.ID,
				Quantity:    itm.Quantity,
			})
			_ = q.UpdateOrderItemShipmentID(ctx, db.UpdateOrderItemShipmentIDParams{
				ID:         itm.ID,
				ShipmentID: TextFromString(sh.ID),
			})
		}

		createdShipments = append(createdShipments, sh)
	}

	return createdShipments, nil
}

// aggregateOrderStatusTx deterministically calculates order-level status from the combined states of its shipments
func (s *MarketplaceService) aggregateOrderStatusTx(ctx context.Context, q *db.Queries, orderID string) error {
	lockedOrder, err := q.GetMarketplaceOrderForUpdate(ctx, orderID)
	if err != nil {
		return err
	}

	// Never overwrite terminal, reconciliation, or stock released states
	if lockedOrder.OrderStatus == "requires_reconciliation" || lockedOrder.ReconciliationRequired ||
		lockedOrder.StockReleased || lockedOrder.OrderStatus == "cancelled" || lockedOrder.OrderStatus == "refunded" {
		return nil
	}

	shipments, err := q.ListMarketplaceShipmentsByOrder(ctx, orderID)
	if err != nil || len(shipments) == 0 {
		return nil
	}

	totalActive := 0
	pendingCount := 0
	dispatchedCount := 0
	deliveredCount := 0

	for _, sh := range shipments {
		if sh.Status == "cancelled" {
			continue
		}
		totalActive++
		switch sh.Status {
		case "pending", "processing", "ready_for_pickup":
			pendingCount++
		case "dispatched", "in_transit":
			dispatchedCount++
		case "delivered":
			deliveredCount++
		}
	}

	if totalActive == 0 {
		return nil
	}

	var newOrderStatus string
	var newFulfilmentStatus string

	if deliveredCount == totalActive {
		newOrderStatus = "delivered"
		newFulfilmentStatus = "delivered"
	} else if deliveredCount > 0 {
		newOrderStatus = "partially_delivered"
		newFulfilmentStatus = "partially_delivered"
	} else if dispatchedCount == totalActive {
		newOrderStatus = "dispatched"
		newFulfilmentStatus = "dispatched"
	} else if dispatchedCount > 0 {
		newOrderStatus = "partially_dispatched"
		newFulfilmentStatus = "partially_dispatched"
	} else {
		if lockedOrder.PaymentStatus == "paid" {
			newOrderStatus = "processing"
		} else {
			newOrderStatus = "placed"
		}
		newFulfilmentStatus = "unfulfilled"
	}

	if newOrderStatus != lockedOrder.OrderStatus {
		_, err = q.UpdateMarketplaceOrderStatus(ctx, db.UpdateMarketplaceOrderStatusParams{
			ID:          orderID,
			OrderStatus: newOrderStatus,
		})
		if err != nil {
			return fmt.Errorf("failed to update aggregated order status: %w", err)
		}
	}

	if newFulfilmentStatus != lockedOrder.FulfilmentStatus {
		_, err = q.UpdateMarketplaceFulfilmentStatus(ctx, db.UpdateMarketplaceFulfilmentStatusParams{
			ID:               orderID,
			FulfilmentStatus: newFulfilmentStatus,
		})
		if err != nil {
			return fmt.Errorf("failed to update aggregated fulfilment status: %w", err)
		}
	}

	return nil
}

func (s *MarketplaceService) resolveOrderGeographyWithLga(ctx context.Context, q *db.Queries, order *db.MarketplaceOrder) (stateID string, zoneID string, lgaID string) {
	if order == nil {
		return "", "", ""
	}
	if order.ShippingStateID.Valid && strings.TrimSpace(order.ShippingStateID.String) != "" {
		stateID = order.ShippingStateID.String
	}
	if order.ShippingLgaID.Valid && strings.TrimSpace(order.ShippingLgaID.String) != "" {
		lgaID = order.ShippingLgaID.String
	}
	if order.PickupBranchID.Valid && strings.TrimSpace(order.PickupBranchID.String) != "" {
		br, err := q.GetBusinessBranchByID(ctx, order.PickupBranchID.String)
		if err == nil {
			if stateID == "" {
				stateID = br.StateID
			}
			if lgaID == "" && br.LgaID.Valid {
				lgaID = br.LgaID.String
			}
		}
	}
	if stateID != "" {
		st, err := q.GetStateByID(ctx, stateID)
		if err == nil {
			zoneID = st.ZoneID
		}
	}
	return stateID, zoneID, lgaID
}

func (s *MarketplaceService) hasCoordinatorJurisdictionWithLga(ctx context.Context, user *db.User, appointments []db.Appointment, orderStateID, orderZoneID, orderLgaID string) bool {
	if user == nil {
		return false
	}
	if s.isNationwideAdmin(user, appointments) {
		return true
	}

	for _, a := range appointments {
		if a.Status != "active" {
			continue
		}
		jType := strings.ToLower(a.JurisdictionType)
		jID := ""
		if a.JurisdictionID.Valid {
			jID = a.JurisdictionID.String
		}

		// LGA coordinator
		if (a.RoleType == "lga_coordinator" || a.RoleType == "lga_representative" || jType == "lga") && jID != "" {
			if orderLgaID != "" && jID == orderLgaID {
				return true
			}
		}

		// State coordinator
		if (a.RoleType == "state_coordinator" || jType == "state") && jID != "" {
			if orderStateID != "" && jID == orderStateID {
				return true
			}
		}

		// Zonal coordinator
		if (a.RoleType == "zonal_coordinator" || jType == "zone" || jType == "zonal") && jID != "" {
			if orderZoneID != "" && jID == orderZoneID {
				return true
			}
		}
	}

	// Fallback for user.PrimaryRole
	if user.PrimaryRole == "state_coordinator" && orderStateID != "" {
		prof, err := s.repo.Queries().GetMemberProfileByUserID(ctx, user.ID)
		if err == nil && prof.HomeStateID.Valid && prof.HomeStateID.String == orderStateID {
			return true
		}
	}
	if (user.PrimaryRole == "lga_coordinator" || user.PrimaryRole == "lga_representative") && orderLgaID != "" {
		prof, err := s.repo.Queries().GetMemberProfileByUserID(ctx, user.ID)
		if err == nil && prof.HomeLgaID.Valid && prof.HomeLgaID.String == orderLgaID {
			return true
		}
	}

	return false
}

func (s *MarketplaceService) DispatchOrder(ctx context.Context, user *db.User, orderID string, input DispatchOrderInput) (*db.MarketplaceShipment, error) {
	if user == nil {
		return nil, errors.New("authentication required")
	}

	var createdShipment db.MarketplaceShipment
	err := s.repo.WithTx(ctx, func(q *db.Queries) error {
		lockedOrder, err := q.GetMarketplaceOrderForUpdate(ctx, orderID)
		if err != nil {
			return ErrOrderNotFound
		}

		// 1. Payment status guard
		if lockedOrder.PaymentStatus != "paid" && lockedOrder.PaymentStatus != "partially_refunded" {
			return ErrOrderNotPaid
		}

		// 2. Reconciliation & Inventory Release guards
		if lockedOrder.OrderStatus == "requires_reconciliation" || lockedOrder.ReconciliationRequired {
			return ErrOrderReconciliationRequired
		}
		if lockedOrder.StockReleased {
			return ErrOrderStockAlreadyReleased
		}

		// 3. Order status guards
		if lockedOrder.OrderStatus == "cancelled" || lockedOrder.OrderStatus == "refunded" {
			return fmt.Errorf("cannot dispatch an order with status '%s'", lockedOrder.OrderStatus)
		}
		if lockedOrder.OrderStatus == "delivered" || lockedOrder.FulfilmentStatus == "delivered" {
			return ErrOrderAlreadyDelivered
		}
		if lockedOrder.FulfilmentStatus == "dispatched" || lockedOrder.OrderStatus == "dispatched" {
			return ErrOrderAlreadyDispatched
		}

		// 4. Items and Businesses
		items, err := q.ListMarketplaceOrderItemsByOrder(ctx, orderID)
		if err != nil {
			return err
		}
		if len(items) == 0 {
			return errors.New("cannot dispatch order with no items")
		}

		businessMap := make(map[string]bool)
		for _, itm := range items {
			businessMap[itm.BusinessID] = true
		}

		apps := s.getActiveAppointments(ctx, user.ID)
		orderStateID, orderZoneID, orderLgaID := s.resolveOrderGeographyWithLga(ctx, q, &lockedOrder)
		isNationwide := s.isNationwideAdmin(user, apps)
		hasJurisdiction := s.hasCoordinatorJurisdictionWithLga(ctx, user, apps, orderStateID, orderZoneID, orderLgaID)
		hasCoordRole := s.isCoordinatorUser(user, apps)

		if hasCoordRole && !hasJurisdiction {
			return ErrScopeUnauthorized
		}

		if !isNationwide && !hasJurisdiction {
			// Seller authorization:
			// If order has multiple sellers, a seller cannot dispatch the multi-seller order
			if len(businessMap) > 1 {
				return ErrUnauthorizedSellerAction
			}
			for _, itm := range items {
				b, err := q.GetMarketplaceBusinessByID(ctx, itm.BusinessID)
				if err != nil {
					return ErrBusinessNotFound
				}
				if b.OwnerUserID != user.ID {
					return ErrUnauthorizedSellerAction
				}
			}
		}

		// Ensure seller shipments exist
		shipments, sErr := s.createShipmentsForOrderTx(ctx, q, orderID)
		if sErr != nil {
			return sErr
		}

		delNotes := input.DeliveryNotes
		if delNotes == "" {
			delNotes = input.Notes
		}

		// Dispatch shipments
		for _, sh := range shipments {
			if sh.Status == "pending" || sh.Status == "processing" {
				upSh, dErr := q.DispatchMarketplaceShipment(ctx, db.DispatchMarketplaceShipmentParams{
					ID:                  sh.ID,
					CarrierName:         input.CarrierName,
					TrackingNumber:      input.TrackingNumber,
					TrackingUrl:         TextFromString(input.TrackingURL),
					DispatchedAt:        TimestamptzFromTime(time.Now()),
					EstimatedDeliveryAt: Timestamptz(input.EstimatedDeliveryDate),
					DeliveryNotes:       TextFromString(delNotes),
				})
				if dErr != nil {
					return fmt.Errorf("failed to dispatch shipment: %w", dErr)
				}
				createdShipment = upSh
			} else {
				createdShipment = sh
			}
		}

		if err := s.aggregateOrderStatusTx(ctx, q, orderID); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	_ = s.audit.Log(ctx, &user.ID, &user.PrimaryRole, "dispatch_marketplace_order", "marketplace_shipment", createdShipment.ID, nil, nil, nil, nil)
	return &createdShipment, nil
}

func (s *MarketplaceService) ConfirmDelivery(ctx context.Context, user *db.User, orderID string, proofURL, notes string) (*db.MarketplaceShipment, error) {
	if user == nil {
		return nil, errors.New("authentication required")
	}

	var updatedSh db.MarketplaceShipment
	err := s.repo.WithTx(ctx, func(q *db.Queries) error {
		lockedOrder, err := q.GetMarketplaceOrderForUpdate(ctx, orderID)
		if err != nil {
			return ErrOrderNotFound
		}

		isBuyer := lockedOrder.BuyerUserID.Valid && lockedOrder.BuyerUserID.String == user.ID
		apps := s.getActiveAppointments(ctx, user.ID)
		orderStateID, orderZoneID, orderLgaID := s.resolveOrderGeographyWithLga(ctx, q, &lockedOrder)
		isNationwide := s.isNationwideAdmin(user, apps)
		hasJurisdiction := s.hasCoordinatorJurisdictionWithLga(ctx, user, apps, orderStateID, orderZoneID, orderLgaID)
		hasCoordRole := s.isCoordinatorUser(user, apps)

		if hasCoordRole && !hasJurisdiction && !isBuyer {
			return ErrScopeUnauthorized
		}

		if !isBuyer && !isNationwide && !hasJurisdiction {
			rawItems, err := q.ListMarketplaceOrderItemsByOrder(ctx, orderID)
			if err != nil {
				return err
			}
			if len(rawItems) == 0 {
				return ErrUnauthorizedSellerAction
			}
			for _, item := range rawItems {
				b, err := q.GetMarketplaceBusinessByID(ctx, item.BusinessID)
				if err != nil || b.OwnerUserID != user.ID {
					return ErrUnauthorizedSellerAction
				}
			}
		}

		if lockedOrder.FulfilmentStatus == "delivered" || lockedOrder.OrderStatus == "delivered" {
			return errors.New("order is already marked as delivered")
		}
		if lockedOrder.FulfilmentStatus != "dispatched" && lockedOrder.FulfilmentStatus != "partially_dispatched" && lockedOrder.FulfilmentStatus != "partially_delivered" {
			return errors.New("only dispatched orders can be confirmed as delivered")
		}

		shipments, err := q.ListMarketplaceShipmentsByOrder(ctx, orderID)
		if err != nil || len(shipments) == 0 {
			return errors.New("shipment record not found for dispatched order")
		}

		// Prevent bulk delivery confirmation for multi-vendor orders:
		// Require buyers to confirm each vendor's consignment individually
		if len(shipments) > 1 {
			return errors.New("cannot bulk confirm delivery for multi-vendor order; please confirm receipt for each vendor shipment individually")
		}

		now := time.Now()
		deliveredAny := false
		for _, sh := range shipments {
			if sh.Status == "dispatched" || sh.Status == "in_transit" {
				upSh, err := q.ConfirmMarketplaceShipmentDelivery(ctx, db.ConfirmMarketplaceShipmentDeliveryParams{
					ID:                 sh.ID,
					DeliveredAt:        TimestamptzFromTime(now),
					ProofOfDeliveryUrl: TextFromString(proofURL),
					DeliveryNotes:      TextFromString(notes),
				})
				if err != nil {
					return fmt.Errorf("failed to update delivery: %w", err)
				}
				updatedSh = upSh
				deliveredAny = true
			} else if sh.Status == "delivered" {
				updatedSh = sh
			}
		}

		if !deliveredAny && updatedSh.ID == "" {
			return errors.New("no dispatched shipments found to confirm delivery")
		}

		if err := s.aggregateOrderStatusTx(ctx, q, orderID); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	_ = s.audit.Log(ctx, &user.ID, &user.PrimaryRole, "confirm_marketplace_delivery", "marketplace_shipment", updatedSh.ID, nil, nil, nil, nil)
	return &updatedSh, nil
}

// DispatchShipment allows an individual seller, coordinator, or admin to dispatch a specific seller shipment
func (s *MarketplaceService) DispatchShipment(ctx context.Context, user *db.User, shipmentID string, input DispatchOrderInput) (*db.MarketplaceShipment, error) {
	if user == nil {
		return nil, errors.New("authentication required")
	}

	var updatedSh db.MarketplaceShipment
	err := s.repo.WithTx(ctx, func(q *db.Queries) error {
		// Standardized lock hierarchy: acquire Order lock before Shipment lock
		shMeta, err := q.GetMarketplaceShipmentByID(ctx, shipmentID)
		if err != nil {
			return ErrShipmentNotFound
		}

		lockedOrder, err := q.GetMarketplaceOrderForUpdate(ctx, shMeta.OrderID)
		if err != nil {
			return ErrOrderNotFound
		}

		lockedSh, err := q.GetMarketplaceShipmentByIDForUpdate(ctx, shipmentID)
		if err != nil {
			return ErrShipmentNotFound
		}

		// 1. Payment status guard: allow paid or partially_refunded orders
		if lockedOrder.PaymentStatus != "paid" && lockedOrder.PaymentStatus != "partially_refunded" {
			return ErrOrderNotPaid
		}

		// 2. Reconciliation & Inventory Release guards
		if lockedOrder.OrderStatus == "requires_reconciliation" || lockedOrder.ReconciliationRequired {
			return ErrOrderReconciliationRequired
		}
		if lockedOrder.StockReleased {
			return ErrOrderStockAlreadyReleased
		}

		// 3. Terminal state guards
		if lockedOrder.OrderStatus == "cancelled" || lockedOrder.OrderStatus == "refunded" {
			return fmt.Errorf("cannot dispatch shipment for %s order", lockedOrder.OrderStatus)
		}

		// 4. Shipment status and refund guards
		if lockedSh.Status == "dispatched" || lockedSh.Status == "in_transit" {
			return ErrOrderAlreadyDispatched
		}
		if lockedSh.Status == "delivered" {
			return ErrOrderAlreadyDelivered
		}
		if lockedSh.Status == "cancelled" {
			return errors.New("cannot dispatch cancelled shipment")
		}

		// Reject dispatch if this specific consignment has an active or completed refund
		hasActiveRef, rErr := q.HasActiveRefundForShipment(ctx, TextFromString(shipmentID))
		if rErr == nil && hasActiveRef {
			return errors.New("cannot dispatch a refunded consignment")
		}

		// Fail closed if order has unattributed partial refunds requiring manual reconciliation
		if lockedOrder.PaymentStatus == "partially_refunded" {
			hasUnattributed, uErr := q.HasUnattributedSuccessfulOrPendingRefunds(ctx, lockedOrder.ID)
			if uErr == nil && hasUnattributed {
				return errors.New("order has unattributed partial refunds requiring reconciliation before dispatch")
			}
		}

		// 5. Authorization:
		apps := s.getActiveAppointments(ctx, user.ID)
		orderStateID, orderZoneID, orderLgaID := s.resolveOrderGeographyWithLga(ctx, q, &lockedOrder)
		isNationwide := s.isNationwideAdmin(user, apps)
		hasJurisdiction := s.hasCoordinatorJurisdictionWithLga(ctx, user, apps, orderStateID, orderZoneID, orderLgaID)
		hasCoordRole := s.isCoordinatorUser(user, apps)

		if hasCoordRole && !hasJurisdiction {
			return ErrScopeUnauthorized
		}

		if !isNationwide && !hasJurisdiction {
			if !lockedSh.BusinessID.Valid {
				return ErrUnauthorizedSellerAction
			}
			b, err := q.GetMarketplaceBusinessByID(ctx, lockedSh.BusinessID.String)
			if err != nil || b.OwnerUserID != user.ID {
				return ErrUnauthorizedSellerAction
			}
		}

		delNotes := input.DeliveryNotes
		if delNotes == "" {
			delNotes = input.Notes
		}

		// 6. Update shipment
		upSh, err := q.DispatchMarketplaceShipment(ctx, db.DispatchMarketplaceShipmentParams{
			ID:                  shipmentID,
			CarrierName:         input.CarrierName,
			TrackingNumber:      input.TrackingNumber,
			TrackingUrl:         TextFromString(input.TrackingURL),
			DispatchedAt:        TimestamptzFromTime(time.Now()),
			EstimatedDeliveryAt: Timestamptz(input.EstimatedDeliveryDate),
			DeliveryNotes:       TextFromString(delNotes),
		})
		if err != nil {
			return fmt.Errorf("failed to dispatch shipment: %w", err)
		}

		// 7. Recalculate order status aggregation
		if err := s.aggregateOrderStatusTx(ctx, q, lockedSh.OrderID); err != nil {
			return err
		}

		updatedSh = upSh
		return nil
	})

	if err != nil {
		return nil, err
	}

	_ = s.audit.Log(ctx, &user.ID, &user.PrimaryRole, "dispatch_marketplace_shipment", "marketplace_shipment", updatedSh.ID, nil, nil, nil, nil)
	return &updatedSh, nil
}

// ConfirmShipmentDelivery confirms receipt of a specific seller shipment
func (s *MarketplaceService) ConfirmShipmentDelivery(ctx context.Context, user *db.User, shipmentID string, proofURL, notes string) (*db.MarketplaceShipment, error) {
	if user == nil {
		return nil, errors.New("authentication required")
	}

	var updatedSh db.MarketplaceShipment
	err := s.repo.WithTx(ctx, func(q *db.Queries) error {
		// Standardized lock hierarchy: acquire Order lock before Shipment lock
		shMeta, err := q.GetMarketplaceShipmentByID(ctx, shipmentID)
		if err != nil {
			return ErrShipmentNotFound
		}

		lockedOrder, err := q.GetMarketplaceOrderForUpdate(ctx, shMeta.OrderID)
		if err != nil {
			return ErrOrderNotFound
		}

		lockedSh, err := q.GetMarketplaceShipmentByIDForUpdate(ctx, shipmentID)
		if err != nil {
			return ErrShipmentNotFound
		}

		if lockedSh.Status == "delivered" {
			return errors.New("shipment is already marked as delivered")
		}
		if lockedSh.Status != "dispatched" && lockedSh.Status != "in_transit" {
			return errors.New("only dispatched shipments can be confirmed as delivered")
		}

		isBuyer := lockedOrder.BuyerUserID.Valid && lockedOrder.BuyerUserID.String == user.ID
		apps := s.getActiveAppointments(ctx, user.ID)
		orderStateID, orderZoneID, orderLgaID := s.resolveOrderGeographyWithLga(ctx, q, &lockedOrder)
		isNationwide := s.isNationwideAdmin(user, apps)
		hasJurisdiction := s.hasCoordinatorJurisdictionWithLga(ctx, user, apps, orderStateID, orderZoneID, orderLgaID)
		hasCoordRole := s.isCoordinatorUser(user, apps)

		if hasCoordRole && !hasJurisdiction && !isBuyer {
			return ErrScopeUnauthorized
		}

		if !isBuyer && !isNationwide && !hasJurisdiction {
			if !lockedSh.BusinessID.Valid {
				return ErrUnauthorizedSellerAction
			}
			b, err := q.GetMarketplaceBusinessByID(ctx, lockedSh.BusinessID.String)
			if err != nil || b.OwnerUserID != user.ID {
				return ErrUnauthorizedSellerAction
			}
		}

		upSh, err := q.ConfirmMarketplaceShipmentDelivery(ctx, db.ConfirmMarketplaceShipmentDeliveryParams{
			ID:                 shipmentID,
			DeliveredAt:        TimestamptzFromTime(time.Now()),
			ProofOfDeliveryUrl: TextFromString(proofURL),
			DeliveryNotes:      TextFromString(notes),
		})
		if err != nil {
			return fmt.Errorf("failed to confirm delivery: %w", err)
		}

		if err := s.aggregateOrderStatusTx(ctx, q, lockedSh.OrderID); err != nil {
			return err
		}

		updatedSh = upSh
		return nil
	})

	if err != nil {
		return nil, err
	}

	_ = s.audit.Log(ctx, &user.ID, &user.PrimaryRole, "confirm_marketplace_shipment_delivery", "marketplace_shipment", updatedSh.ID, nil, nil, nil, nil)
	return &updatedSh, nil
}

// UpdateShipmentTracking updates carrier, tracking, and notes for a specific shipment
func (s *MarketplaceService) UpdateShipmentTracking(ctx context.Context, user *db.User, shipmentID string, input UpdateShipmentTrackingInput) (*db.MarketplaceShipment, error) {
	if user == nil {
		return nil, errors.New("authentication required")
	}

	var updatedSh db.MarketplaceShipment
	err := s.repo.WithTx(ctx, func(q *db.Queries) error {
		// Standardized lock hierarchy: acquire Order lock before Shipment lock
		shMeta, err := q.GetMarketplaceShipmentByID(ctx, shipmentID)
		if err != nil {
			return ErrShipmentNotFound
		}

		lockedOrder, err := q.GetMarketplaceOrderForUpdate(ctx, shMeta.OrderID)
		if err != nil {
			return ErrOrderNotFound
		}

		lockedSh, err := q.GetMarketplaceShipmentByIDForUpdate(ctx, shipmentID)
		if err != nil {
			return ErrShipmentNotFound
		}

		apps := s.getActiveAppointments(ctx, user.ID)
		orderStateID, orderZoneID, orderLgaID := s.resolveOrderGeographyWithLga(ctx, q, &lockedOrder)
		isNationwide := s.isNationwideAdmin(user, apps)
		hasJurisdiction := s.hasCoordinatorJurisdictionWithLga(ctx, user, apps, orderStateID, orderZoneID, orderLgaID)
		hasCoordRole := s.isCoordinatorUser(user, apps)

		if hasCoordRole && !hasJurisdiction {
			return ErrScopeUnauthorized
		}

		if !isNationwide && !hasJurisdiction {
			if !lockedSh.BusinessID.Valid {
				return ErrUnauthorizedSellerAction
			}
			b, err := q.GetMarketplaceBusinessByID(ctx, lockedSh.BusinessID.String)
			if err != nil || b.OwnerUserID != user.ID {
				return ErrUnauthorizedSellerAction
			}
		}

		upSh, err := q.UpdateMarketplaceShipmentTracking(ctx, db.UpdateMarketplaceShipmentTrackingParams{
			ID:             shipmentID,
			CarrierName:    input.CarrierName,
			TrackingNumber: input.TrackingNumber,
			TrackingUrl:    TextFromString(input.TrackingURL),
			DeliveryNotes:  TextFromString(input.DeliveryNotes),
		})
		if err != nil {
			return fmt.Errorf("failed to update tracking: %w", err)
		}

		updatedSh = upSh
		return nil
	})

	if err != nil {
		return nil, err
	}

	_ = s.audit.Log(ctx, &user.ID, &user.PrimaryRole, "update_marketplace_shipment_tracking", "marketplace_shipment", updatedSh.ID, nil, nil, nil, nil)
	return &updatedSh, nil
}

// GetShipmentByID fetches detailed shipment information including products and order details
func (s *MarketplaceService) GetShipmentByID(ctx context.Context, user *db.User, shipmentID string) (*ShipmentDetailResult, error) {
	if user == nil {
		return nil, errors.New("authentication required")
	}

	sh, err := s.repo.Queries().GetMarketplaceShipmentByID(ctx, shipmentID)
	if err != nil {
		return nil, ErrShipmentNotFound
	}

	order, err := s.repo.Queries().GetMarketplaceOrderByID(ctx, sh.OrderID)
	if err != nil {
		return nil, ErrOrderNotFound
	}

	isBuyer := order.BuyerUserID.Valid && order.BuyerUserID.String == user.ID
	apps := s.getActiveAppointments(ctx, user.ID)
	orderStateID, orderZoneID, orderLgaID := s.resolveOrderGeographyWithLga(ctx, s.repo.Queries(), &db.MarketplaceOrder{
		ShippingStateID: order.ShippingStateID,
		ShippingLgaID:   order.ShippingLgaID,
		PickupBranchID:  order.PickupBranchID,
	})
	isNationwide := s.isNationwideAdmin(user, apps)
	hasJurisdiction := s.hasCoordinatorJurisdictionWithLga(ctx, user, apps, orderStateID, orderZoneID, orderLgaID)
	hasCoordRole := s.isCoordinatorUser(user, apps)

	if hasCoordRole && !hasJurisdiction && !isBuyer {
		return nil, ErrScopeUnauthorized
	}

	if !isBuyer && !isNationwide && !hasJurisdiction {
		if !sh.BusinessID.Valid {
			return nil, ErrUnauthorizedSellerAction
		}
		b, err := s.repo.Queries().GetMarketplaceBusinessByID(ctx, sh.BusinessID.String)
		if err != nil || b.OwnerUserID != user.ID {
			return nil, ErrUnauthorizedSellerAction
		}
	}

	var busPtr *db.MarketplaceBusiness
	if sh.BusinessID.Valid {
		b, bErr := s.repo.Queries().GetMarketplaceBusinessByID(ctx, sh.BusinessID.String)
		if bErr == nil {
			busPtr = &db.MarketplaceBusiness{
				ID:          b.ID,
				OwnerUserID: b.OwnerUserID,
				Name:        b.Name,
				Slug:        b.Slug,
			}
		}
	}

	rawItems, err := s.repo.Queries().ListMarketplaceShipmentItemsByShipment(ctx, shipmentID)
	if err != nil {
		return nil, err
	}
	var items []ShipmentItemDetail
	for _, itm := range rawItems {
		items = append(items, ShipmentItemDetail{
			ID:                itm.ID,
			ShipmentID:        itm.ShipmentID,
			OrderItemID:       itm.OrderItemID,
			Quantity:          itm.Quantity,
			ProductID:         itm.ProductID,
			TitleSnapshot:     itm.TitleSnapshot,
			SkuSnapshot:       itm.SkuSnapshot,
			PriceKoboSnapshot: itm.PriceKoboSnapshot,
			ImageUrlSnapshot:  itm.ImageUrlSnapshot,
			BusinessID:        itm.BusinessID,
		})
	}

	orderModel := &db.MarketplaceOrder{
		ID:               order.ID,
		OrderNumber:      order.OrderNumber,
		BuyerUserID:      order.BuyerUserID,
		BuyerName:        order.BuyerName,
		BuyerEmail:       order.BuyerEmail,
		BuyerPhone:       order.BuyerPhone,
		DeliveryType:     order.DeliveryType,
		ShippingAddress:  order.ShippingAddress,
		ShippingStateID:  order.ShippingStateID,
		ShippingLgaID:    order.ShippingLgaID,
		OrderStatus:      order.OrderStatus,
		PaymentStatus:    order.PaymentStatus,
		FulfilmentStatus: order.FulfilmentStatus,
	}

	if !isBuyer && !isNationwide {
		orderModel.BuyerEmail = "[REDACTED]"
		orderModel.BuyerUserID = pgtype.Text{Valid: false}
	}

	return &ShipmentDetailResult{
		Shipment: sh,
		Order:    orderModel,
		Business: busPtr,
		Items:    items,
	}, nil
}

// ListSellerShipments lists shipments belonging to businesses owned by the authenticated seller
func (s *MarketplaceService) ListSellerShipments(ctx context.Context, user *db.User, status *string) ([]ShipmentDetailResult, error) {
	if user == nil {
		return nil, errors.New("authentication required")
	}

	ownerID := user.ID
	businesses, err := s.repo.Queries().ListMarketplaceBusinesses(ctx, db.ListMarketplaceBusinessesParams{
		Column1: "",
		Column2: "",
		Column3: "",
		Column4: ownerID,
	})
	if err != nil {
		return nil, err
	}
	if len(businesses) == 0 {
		return []ShipmentDetailResult{}, nil
	}

	statusFilter := ""
	if status != nil {
		statusFilter = *status
	}

	var results []ShipmentDetailResult
	for _, b := range businesses {
		shipments, err := s.repo.Queries().ListMarketplaceShipmentsByBusiness(ctx, db.ListMarketplaceShipmentsByBusinessParams{
			BusinessID: pgtype.Text{String: b.ID, Valid: true},
			Column2:    statusFilter,
		})
		if err != nil {
			continue
		}

		for _, shRow := range shipments {
			sh := db.MarketplaceShipment{
				ID:                  shRow.ID,
				OrderID:             shRow.OrderID,
				BusinessID:          shRow.BusinessID,
				CarrierName:         shRow.CarrierName,
				TrackingNumber:      shRow.TrackingNumber,
				TrackingUrl:         shRow.TrackingUrl,
				Status:              shRow.Status,
				DispatchedAt:        shRow.DispatchedAt,
				EstimatedDeliveryAt: shRow.EstimatedDeliveryAt,
				DeliveredAt:         shRow.DeliveredAt,
				DeliveryNotes:       shRow.DeliveryNotes,
				ProofOfDeliveryUrl:  shRow.ProofOfDeliveryUrl,
				CreatedAt:           shRow.CreatedAt,
				UpdatedAt:           shRow.UpdatedAt,
			}

			orderModel := &db.MarketplaceOrder{
				ID:              shRow.OrderID,
				OrderNumber:     shRow.OrderNumber,
				BuyerName:       shRow.BuyerName,
				BuyerEmail:      "[REDACTED]",
				BuyerPhone:      shRow.BuyerPhone,
				ShippingAddress: shRow.ShippingAddress,
				ShippingStateID: shRow.ShippingStateID,
				ShippingLgaID:   shRow.ShippingLgaID,
				OrderStatus:     shRow.OrderStatus,
				PaymentStatus:   shRow.PaymentStatus,
			}

			busModel := &db.MarketplaceBusiness{
				ID:   b.ID,
				Name: b.Name,
				Slug: b.Slug,
			}

			rawItems, _ := s.repo.Queries().ListMarketplaceShipmentItemsByShipment(ctx, sh.ID)
			var items []ShipmentItemDetail
			for _, itm := range rawItems {
				items = append(items, ShipmentItemDetail{
					ID:                itm.ID,
					ShipmentID:        itm.ShipmentID,
					OrderItemID:       itm.OrderItemID,
					Quantity:          itm.Quantity,
					ProductID:         itm.ProductID,
					TitleSnapshot:     itm.TitleSnapshot,
					SkuSnapshot:       itm.SkuSnapshot,
					PriceKoboSnapshot: itm.PriceKoboSnapshot,
					ImageUrlSnapshot:  itm.ImageUrlSnapshot,
					BusinessID:        itm.BusinessID,
				})
			}

			results = append(results, ShipmentDetailResult{
				Shipment: sh,
				Order:    orderModel,
				Business: busModel,
				Items:    items,
			})
		}
	}

	if results == nil {
		results = []ShipmentDetailResult{}
	}
	return results, nil
}

// ListOrderShipments lists all vendor shipments for a specific customer order
func (s *MarketplaceService) ListOrderShipments(ctx context.Context, user *db.User, orderID string) ([]ShipmentDetailResult, error) {
	if user == nil {
		return nil, errors.New("authentication required")
	}

	order, err := s.repo.Queries().GetMarketplaceOrderByID(ctx, orderID)
	if err != nil {
		return nil, ErrOrderNotFound
	}

	isBuyer := order.BuyerUserID.Valid && order.BuyerUserID.String == user.ID
	apps := s.getActiveAppointments(ctx, user.ID)
	orderStateID, orderZoneID, orderLgaID := s.resolveOrderGeographyWithLga(ctx, s.repo.Queries(), &db.MarketplaceOrder{
		ShippingStateID: order.ShippingStateID,
		ShippingLgaID:   order.ShippingLgaID,
		PickupBranchID:  order.PickupBranchID,
	})
	isNationwide := s.isNationwideAdmin(user, apps)
	hasJurisdiction := s.hasCoordinatorJurisdictionWithLga(ctx, user, apps, orderStateID, orderZoneID, orderLgaID)
	hasCoordRole := s.isCoordinatorUser(user, apps)

	if hasCoordRole && !hasJurisdiction && !isBuyer {
		return nil, ErrScopeUnauthorized
	}

	if !isBuyer && !isNationwide && !hasJurisdiction {
		// Verify participating seller ownership
		items, err := s.repo.Queries().ListMarketplaceOrderItemsByOrder(ctx, orderID)
		if err != nil {
			return nil, err
		}
		isParticipatingSeller := false
		for _, itm := range items {
			b, bErr := s.repo.Queries().GetMarketplaceBusinessByID(ctx, itm.BusinessID)
			if bErr == nil && b.OwnerUserID == user.ID {
				isParticipatingSeller = true
				break
			}
		}
		if !isParticipatingSeller {
			// Non-disclosing response: return ErrOrderNotFound to prevent ID enumeration oracle
			return nil, ErrOrderNotFound
		}
	}

	// Idempotently create shipments for paid or partially refunded orders if not yet created
	if (order.PaymentStatus == "paid" || order.PaymentStatus == "partially_refunded") && !order.StockReleased && !order.ReconciliationRequired &&
		order.OrderStatus != "cancelled" && order.OrderStatus != "refunded" && order.OrderStatus != "requires_reconciliation" {
		_ = s.repo.WithTx(ctx, func(q *db.Queries) error {
			_, _ = s.createShipmentsForOrderTx(ctx, q, orderID)
			return nil
		})
	}

	shipments, err := s.repo.Queries().ListMarketplaceShipmentsByOrder(ctx, orderID)
	if err != nil {
		return nil, err
	}

	var results []ShipmentDetailResult
	for _, sh := range shipments {
		// If seller, filter to own business shipment
		if !isBuyer && !isNationwide && !hasJurisdiction {
			if !sh.BusinessID.Valid {
				continue
			}
			b, bErr := s.repo.Queries().GetMarketplaceBusinessByID(ctx, sh.BusinessID.String)
			if bErr != nil || b.OwnerUserID != user.ID {
				continue
			}
		}

		var busModel *db.MarketplaceBusiness
		if sh.BusinessID.Valid {
			b, bErr := s.repo.Queries().GetMarketplaceBusinessByID(ctx, sh.BusinessID.String)
			if bErr == nil {
				busModel = &db.MarketplaceBusiness{
					ID:   b.ID,
					Name: b.Name,
					Slug: b.Slug,
				}
			}
		}

		rawItems, _ := s.repo.Queries().ListMarketplaceShipmentItemsByShipment(ctx, sh.ID)
		var items []ShipmentItemDetail
		for _, itm := range rawItems {
			items = append(items, ShipmentItemDetail{
				ID:                itm.ID,
				ShipmentID:        itm.ShipmentID,
				OrderItemID:       itm.OrderItemID,
				Quantity:          itm.Quantity,
				ProductID:         itm.ProductID,
				TitleSnapshot:     itm.TitleSnapshot,
				SkuSnapshot:       itm.SkuSnapshot,
				PriceKoboSnapshot: itm.PriceKoboSnapshot,
				ImageUrlSnapshot:  itm.ImageUrlSnapshot,
				BusinessID:        itm.BusinessID,
			})
		}

		orderModel := &db.MarketplaceOrder{
			ID:               order.ID,
			OrderNumber:      order.OrderNumber,
			BuyerName:        order.BuyerName,
			BuyerEmail:       order.BuyerEmail,
			BuyerPhone:       order.BuyerPhone,
			DeliveryType:     order.DeliveryType,
			ShippingAddress:  order.ShippingAddress,
			ShippingStateID:  order.ShippingStateID,
			ShippingLgaID:    order.ShippingLgaID,
			OrderStatus:      order.OrderStatus,
			PaymentStatus:    order.PaymentStatus,
			FulfilmentStatus: order.FulfilmentStatus,
		}
		if !isBuyer && !isNationwide {
			orderModel.BuyerEmail = "[REDACTED]"
			orderModel.BuyerUserID = pgtype.Text{Valid: false}
		}

		results = append(results, ShipmentDetailResult{
			Shipment: sh,
			Order:    orderModel,
			Business: busModel,
			Items:    items,
		})
	}

	if results == nil {
		results = []ShipmentDetailResult{}
	}
	return results, nil
}

// ListAllShipmentsAdmin lists shipments nationwide for super admins, or scoped for coordinators
func (s *MarketplaceService) ListAllShipmentsAdmin(ctx context.Context, user *db.User, status *string) ([]ShipmentDetailResult, error) {
	if user == nil {
		return nil, errors.New("authentication required")
	}

	apps := s.getActiveAppointments(ctx, user.ID)
	isNationwide := s.isNationwideAdmin(user, apps)
	hasCoordRole := s.isCoordinatorUser(user, apps)

	if !isNationwide && !hasCoordRole {
		return nil, ErrScopeUnauthorized
	}

	statusFilter := ""
	if status != nil {
		statusFilter = *status
	}

	rows, err := s.repo.Queries().ListAllMarketplaceShipmentsAdmin(ctx, statusFilter)
	if err != nil {
		return nil, err
	}

	var results []ShipmentDetailResult
	for _, r := range rows {
		orderStateID, orderZoneID, orderLgaID := s.resolveOrderGeographyWithLga(ctx, s.repo.Queries(), &db.MarketplaceOrder{
			ShippingStateID: r.ShippingStateID,
			ShippingLgaID:   r.ShippingLgaID,
			PickupBranchID:  r.PickupBranchID,
		})

		if !isNationwide && !s.hasCoordinatorJurisdictionWithLga(ctx, user, apps, orderStateID, orderZoneID, orderLgaID) {
			continue
		}

		sh := db.MarketplaceShipment{
			ID:                  r.ID,
			OrderID:             r.OrderID,
			BusinessID:          r.BusinessID,
			CarrierName:         r.CarrierName,
			TrackingNumber:      r.TrackingNumber,
			TrackingUrl:         r.TrackingUrl,
			Status:              r.Status,
			DispatchedAt:        r.DispatchedAt,
			EstimatedDeliveryAt: r.EstimatedDeliveryAt,
			DeliveredAt:         r.DeliveredAt,
			DeliveryNotes:       r.DeliveryNotes,
			ProofOfDeliveryUrl:  r.ProofOfDeliveryUrl,
			CreatedAt:           r.CreatedAt,
			UpdatedAt:           r.UpdatedAt,
		}

		orderModel := &db.MarketplaceOrder{
			ID:              r.OrderID,
			OrderNumber:     r.OrderNumber,
			BuyerName:       r.BuyerName,
			BuyerEmail:      r.BuyerEmail,
			ShippingAddress: r.ShippingAddress,
			ShippingStateID: r.ShippingStateID,
			ShippingLgaID:   r.ShippingLgaID,
			OrderStatus:     r.OrderStatus,
			PaymentStatus:   r.PaymentStatus,
		}

		busModel := &db.MarketplaceBusiness{
			ID:          r.BusinessID.String,
			Name:        r.BusinessName,
			OwnerUserID: r.SellerUserID,
		}

		rawItems, _ := s.repo.Queries().ListMarketplaceShipmentItemsByShipment(ctx, sh.ID)
		var items []ShipmentItemDetail
		for _, itm := range rawItems {
			items = append(items, ShipmentItemDetail{
				ID:                itm.ID,
				ShipmentID:        itm.ShipmentID,
				OrderItemID:       itm.OrderItemID,
				Quantity:          itm.Quantity,
				ProductID:         itm.ProductID,
				TitleSnapshot:     itm.TitleSnapshot,
				SkuSnapshot:       itm.SkuSnapshot,
				PriceKoboSnapshot: itm.PriceKoboSnapshot,
				ImageUrlSnapshot:  itm.ImageUrlSnapshot,
				BusinessID:        itm.BusinessID,
			})
		}

		results = append(results, ShipmentDetailResult{
			Shipment: sh,
			Order:    orderModel,
			Business: busModel,
			Items:    items,
		})
	}

	if results == nil {
		results = []ShipmentDetailResult{}
	}
	return results, nil
}

// GetOrderWithShipmentsByID fetches order details, items, primary shipment, and all multi-vendor shipments
func (s *MarketplaceService) GetOrderWithShipmentsByID(ctx context.Context, user *db.User, id string) (*db.GetMarketplaceOrderByIDRow, []db.ListMarketplaceOrderItemsByOrderRow, *db.MarketplaceShipment, []ShipmentDetailResult, error) {
	order, items, primaryShipment, err := s.GetOrderByID(ctx, user, id)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	shipments, _ := s.ListOrderShipments(ctx, user, id)
	return order, items, primaryShipment, shipments, nil
}

func (s *MarketplaceService) CancelOrder(ctx context.Context, user *db.User, orderID string, reason string) (*db.GetMarketplaceOrderByIDRow, error) {
	if user == nil {
		return nil, errors.New("authentication required")
	}

	var createdRefund *db.MarketplaceRefund
	var payRec *db.MarketplacePayment

	err := s.repo.WithTx(ctx, func(q *db.Queries) error {
		lockedOrder, err := q.GetMarketplaceOrderForUpdate(ctx, orderID)
		if err != nil {
			return ErrOrderNotFound
		}

		isBuyer := lockedOrder.BuyerUserID.Valid && lockedOrder.BuyerUserID.String == user.ID
		apps := s.getActiveAppointments(ctx, user.ID)
		orderStateID, orderZoneID, orderLgaID := s.resolveOrderGeographyWithLga(ctx, q, &lockedOrder)
		isNationwide := s.isNationwideAdmin(user, apps)
		hasJurisdiction := s.hasCoordinatorJurisdictionWithLga(ctx, user, apps, orderStateID, orderZoneID, orderLgaID)
		hasCoordRole := s.isCoordinatorUser(user, apps)

		if !isBuyer && !isNationwide {
			if hasCoordRole && !hasJurisdiction {
				return ErrScopeUnauthorized
			}
			if !hasJurisdiction {
				return ErrUnauthorizedBuyerAction
			}
		}

		if lockedOrder.OrderStatus == "cancelled" {
			return errors.New("order is already cancelled")
		}
		if lockedOrder.OrderStatus == "delivered" || lockedOrder.FulfilmentStatus == "delivered" {
			return errors.New("cannot cancel an order that has already been delivered")
		}
		if lockedOrder.FulfilmentStatus == "dispatched" || lockedOrder.OrderStatus == "dispatched" ||
			lockedOrder.FulfilmentStatus == "partially_dispatched" || lockedOrder.OrderStatus == "partially_dispatched" ||
			lockedOrder.FulfilmentStatus == "partially_delivered" || lockedOrder.OrderStatus == "partially_delivered" {
			return errors.New("cannot cancel an order where one or more shipments have already been dispatched; please request a return or dispute")
		}

		shipments, sErr := q.ListMarketplaceShipmentsByOrder(ctx, orderID)
		if sErr == nil {
			for _, sh := range shipments {
				if sh.Status == "dispatched" || sh.Status == "in_transit" || sh.Status == "delivered" {
					return errors.New("cannot cancel an order where one or more shipments have already been dispatched; please request a return or dispute")
				}
			}
		}

		if lockedOrder.OrderStatus == "refunded" {
			return errors.New("cannot cancel a refunded order")
		}

		// Atomic cancellation and stock restoration (aggregating quantities by product ID to prevent dropped restocks)
		if !lockedOrder.StockReleased {
			items, err := q.ListMarketplaceOrderItemsByOrder(ctx, orderID)
			if err != nil {
				return err
			}
			prodTotals := make(map[string]int32)
			for _, itm := range items {
				if itm.ProductID.Valid {
					prodTotals[itm.ProductID.String] += itm.Quantity
				}
			}
			for prodID, totalQty := range prodTotals {
				releaseCount, cErr := q.CountOrderInventoryReleaseLogs(ctx, db.CountOrderInventoryReleaseLogsParams{
					OrderID:   TextFromString(orderID),
					ProductID: prodID,
				})
				if cErr == nil && releaseCount == 0 {
					incProd, err := q.IncrementProductStock(ctx, db.IncrementProductStockParams{
						ID:            prodID,
						StockQuantity: totalQty,
					})
					if err != nil {
						return err
					}
					var prodBusID string
					p, pErr := q.GetMarketplaceProductByID(ctx, prodID)
					if pErr == nil {
						prodBusID = p.BusinessID
					}
					var actorID pgtype.Text
					if user != nil {
						actorID = TextFromString(user.ID)
					}
					_, logErr := q.CreateMarketplaceInventoryLog(ctx, db.CreateMarketplaceInventoryLogParams{
						ID:             security.GenerateUUID(),
						ProductID:      prodID,
						BusinessID:     prodBusID,
						ActorUserID:    actorID,
						ActionType:     "reservation_release",
						QuantityDelta:  totalQty,
						BalanceAfter:   incProd.StockQuantity,
						OrderID:        TextFromString(orderID),
						ReferenceID:    lockedOrder.OrderNumber,
						Reason:         "Order cancelled: " + reason,
						IdempotencyKey: TextFromString(fmt.Sprintf("release:%s:%s", orderID, prodID)),
					})
					if logErr != nil {
						return fmt.Errorf("failed to record cancellation inventory release log: %w", logErr)
					}
				}
			}
		}

		_, err = q.MarkOrderCancelledWithReason(ctx, db.MarkOrderCancelledWithReasonParams{
			ID:                 orderID,
			CancellationReason: reason,
			StockReleased:      true,
		})
		if err != nil {
			return err
		}

		_ = q.CancelMarketplaceShipmentsByOrder(ctx, orderID)

		// Create durable refund obligation for paid order without prematurely claiming payment status = refunded
		if lockedOrder.PaymentStatus == "paid" {
			pay, pErr := q.GetMarketplacePaymentByOrderForUpdate(ctx, orderID)
			if pErr == nil {
				payRec = &pay
				existingPending, peErr := q.GetPendingRefundByOrder(ctx, orderID)
				if peErr != nil || existingPending.ID == "" {
					refundID := security.GenerateUUID()
					r, rErr := q.CreateMarketplaceRefund(ctx, db.CreateMarketplaceRefundParams{
						ID:                      refundID,
						OrderID:                 orderID,
						PaymentID:               pay.ID,
						AmountKobo:              lockedOrder.TotalKobo,
						Status:                  "pending",
						Provider:                pay.Provider,
						ProviderRefundReference: "",
						InitiatedByUserID:       user.ID,
						ConfirmedByUserID:       pgtype.Text{Valid: false},
						Reason:                  "Order cancelled: " + reason,
						SettlementNotes:         "Cancellation refund obligation pending provider settlement",
						Metadata:                []byte("{}"),
						ShipmentID:              pgtype.Text{Valid: false},
						BusinessID:              pgtype.Text{Valid: false},
						OrderItemID:             pgtype.Text{Valid: false},
					})
					if rErr == nil {
						createdRefund = &r
					}
				} else {
					createdRefund = &existingPending
				}
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	// Attempt idempotent provider refund initiation outside the database transaction
	if createdRefund != nil && payRec != nil && createdRefund.Provider != "manual" && createdRefund.Provider != "offline" {
		refundOut, pReqErr := s.paymentGateway.RefundPayment(ctx, payment.RefundPaymentInput{
			PaymentReference: payRec.Reference,
			AmountKobo:       createdRefund.AmountKobo,
			Currency:         payRec.Currency,
			CustomerNote:     reason,
			MerchantNote:     fmt.Sprintf("Cancellation refund for order %s initiated by user %s", orderID, user.Email.String),
		})
		if pReqErr == nil && refundOut != nil {
			if refundOut.Status == "successful" {
				_, _, _ = s.settleRefundSuccess(ctx, user, createdRefund.ID, refundOut.RefundReference, "Settled immediately on cancellation by payment provider", refundOut.RawResponse)
			} else if refundOut.RefundReference != "" {
				meta := refundOut.RawResponse
				if len(meta) == 0 {
					meta = []byte("{}")
				}
				_ = s.repo.WithTx(ctx, func(q *db.Queries) error {
					_, err := q.UpdateMarketplaceRefundStatus(ctx, db.UpdateMarketplaceRefundStatusParams{
						ID:                      createdRefund.ID,
						Status:                  "pending",
						ProviderRefundReference: refundOut.RefundReference,
						ConfirmedByUserID:       pgtype.Text{Valid: false},
						SettlementNotes:         "Pending provider settlement",
						Metadata:                meta,
					})
					return err
				})
			}
		}
	}

	_ = s.audit.Log(ctx, &user.ID, &user.PrimaryRole, "cancel_marketplace_order", "marketplace_order", orderID, nil, nil, &reason, nil)

	updated, err := s.repo.Queries().GetMarketplaceOrderByID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	return &updated, nil
}

type RefundOrderInput struct {
	AmountKobo       int64  `json:"amount_kobo"`
	Reason           string `json:"reason"`
	ManualSettlement bool   `json:"manual_settlement,omitempty"`
	SettlementNotes  string `json:"settlement_notes,omitempty"`
	ShipmentID       string `json:"shipment_id,omitempty"`
	OrderItemID      string `json:"order_item_id,omitempty"`
}

type ConfirmRefundInput struct {
	ProviderRefundReference string `json:"provider_refund_reference,omitempty"`
	SettlementNotes         string `json:"settlement_notes,omitempty"`
}

type ManualSettlementInput struct {
	Evidence string `json:"settlement_evidence"`
	Reason   string `json:"reason"`
	Notes    string `json:"notes,omitempty"`
}

func (s *MarketplaceService) InitiateRefund(ctx context.Context, admin *db.User, orderID string, input RefundOrderInput) (*db.MarketplaceRefund, *db.GetMarketplaceOrderByIDRow, error) {
	if admin == nil {
		return nil, nil, errors.New("authentication required")
	}

	apps := s.getActiveAppointments(ctx, admin.ID)
	isNationwide := s.isNationwideAdmin(admin, apps)
	hasCoordRole := s.isCoordinatorUser(admin, apps)

	if !isNationwide && !hasCoordRole {
		return nil, nil, errors.New("unauthorized: administrative permissions required")
	}

	if input.AmountKobo < 0 {
		return nil, nil, errors.New("refund amount must be greater than zero")
	}

	order, err := s.repo.Queries().GetMarketplaceOrderByID(ctx, orderID)
	if err != nil {
		return nil, nil, ErrOrderNotFound
	}

	orderStateID, orderZoneID, orderLgaID := s.resolveOrderGeographyWithLga(ctx, s.repo.Queries(), &db.MarketplaceOrder{
		ShippingStateID: order.ShippingStateID,
		ShippingLgaID:   order.ShippingLgaID,
		PickupBranchID:  order.PickupBranchID,
	})
	if !isNationwide && !s.hasCoordinatorJurisdictionWithLga(ctx, admin, apps, orderStateID, orderZoneID, orderLgaID) {
		return nil, nil, ErrScopeUnauthorized
	}

	if order.PaymentStatus != "paid" && order.PaymentStatus != "partially_refunded" {
		return nil, nil, fmt.Errorf("cannot refund an order with payment status '%s'", order.PaymentStatus)
	}

	var createdRefund db.MarketplaceRefund

	err = s.repo.WithTx(ctx, func(q *db.Queries) error {
		lockedOrder, err := q.GetMarketplaceOrderForUpdate(ctx, orderID)
		if err != nil {
			return fmt.Errorf("failed to lock order for refund: %w", err)
		}

		stID, znID, lgaID := s.resolveOrderGeographyWithLga(ctx, q, &lockedOrder)
		if !isNationwide && !s.hasCoordinatorJurisdictionWithLga(ctx, admin, apps, stID, znID, lgaID) {
			return ErrScopeUnauthorized
		}

		if lockedOrder.PaymentStatus != "paid" && lockedOrder.PaymentStatus != "partially_refunded" {
			return fmt.Errorf("cannot refund an order with payment status '%s'", lockedOrder.PaymentStatus)
		}

		pay, err := q.GetMarketplacePaymentByOrderForUpdate(ctx, orderID)
		if err != nil {
			return fmt.Errorf("failed to locate payment for order: %w", err)
		}

		// Calculate eligible refunds from authoritative refund records and pending reservations
		sums, err := q.GetMarketplaceRefundSumsByOrder(ctx, orderID)
		if err != nil {
			return fmt.Errorf("failed to calculate existing refund sums: %w", err)
		}

		totalCommittedOrReserved := sums.SuccessfulAmountKobo + sums.PendingAmountKobo
		remainingEligible := lockedOrder.TotalKobo - totalCommittedOrReserved

		if remainingEligible <= 0 {
			return errors.New("order is already fully refunded or has pending refund reservations covering the balance")
		}

		actualRefund := input.AmountKobo
		if actualRefund <= 0 || actualRefund == lockedOrder.TotalKobo {
			actualRefund = remainingEligible
		}

		if actualRefund > remainingEligible {
			return fmt.Errorf("refund amount %d kobo exceeds eligible remaining amount %d kobo (total order %d kobo, already refunded %d kobo, pending reservations %d kobo)",
				actualRefund, remainingEligible, lockedOrder.TotalKobo, sums.SuccessfulAmountKobo, sums.PendingAmountKobo)
		}

		var shipmentID pgtype.Text
		var businessID pgtype.Text
		var orderItemID pgtype.Text

		if strings.TrimSpace(input.ShipmentID) != "" {
			shp, err := q.GetMarketplaceShipmentByID(ctx, input.ShipmentID)
			if err != nil {
				return fmt.Errorf("shipment not found: %w", err)
			}
			if shp.OrderID != orderID {
				return errors.New("specified shipment does not belong to this order")
			}
			shipmentID = TextFromString(shp.ID)
			businessID = shp.BusinessID
		}

		if strings.TrimSpace(input.OrderItemID) != "" {
			itm, err := q.GetMarketplaceOrderItemByID(ctx, input.OrderItemID)
			if err != nil {
				return fmt.Errorf("order item not found: %w", err)
			}
			if itm.OrderID != orderID {
				return errors.New("specified order item does not belong to this order")
			}
			orderItemID = TextFromString(itm.ID)
			if !businessID.Valid {
				businessID = TextFromString(itm.BusinessID)
			}
		}

		refundID := security.GenerateUUID()
		refundStatus := "pending"

		r, err := q.CreateMarketplaceRefund(ctx, db.CreateMarketplaceRefundParams{
			ID:                      refundID,
			OrderID:                 orderID,
			PaymentID:               pay.ID,
			AmountKobo:              actualRefund,
			Status:                  refundStatus,
			Provider:                pay.Provider,
			ProviderRefundReference: "",
			InitiatedByUserID:       admin.ID,
			ConfirmedByUserID:       pgtype.Text{Valid: false},
			Reason:                  input.Reason,
			SettlementNotes:         input.SettlementNotes,
			Metadata:                []byte("{}"),
			ShipmentID:              shipmentID,
			BusinessID:              businessID,
			OrderItemID:             orderItemID,
		})
		if err != nil {
			return fmt.Errorf("failed to create refund record: %w", err)
		}
		createdRefund = r
		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	// Attempt provider refund execution outside the database transaction
	if createdRefund.Provider != "manual" && createdRefund.Provider != "offline" {
		payRec, pErr := s.repo.Queries().GetMarketplacePaymentByOrder(ctx, orderID)
		if pErr == nil {
			refundOut, pReqErr := s.paymentGateway.RefundPayment(ctx, payment.RefundPaymentInput{
				PaymentReference: payRec.Reference,
				AmountKobo:       createdRefund.AmountKobo,
				Currency:         payRec.Currency,
				CustomerNote:     input.Reason,
				MerchantNote:     fmt.Sprintf("Refund for order %s initiated by admin %s", orderID, admin.Email.String),
			})
			if pReqErr != nil {
				_, _, _ = s.FailRefund(ctx, admin, createdRefund.ID, fmt.Sprintf("Payment provider rejected refund: %v", pReqErr))
				return nil, nil, fmt.Errorf("payment provider refund initiation failed: %w", pReqErr)
			}
			if refundOut != nil {
				if refundOut.Status == "failed" {
					_, _, _ = s.FailRefund(ctx, admin, createdRefund.ID, "Payment provider returned failed refund status")
					return nil, nil, errors.New("payment provider rejected refund")
				}
				if refundOut.Status == "successful" {
					return s.settleRefundSuccess(ctx, admin, createdRefund.ID, refundOut.RefundReference, "Completed immediately by payment provider", refundOut.RawResponse)
				}
				// If pending, record the provider refund reference
				if refundOut.RefundReference != "" {
					meta := refundOut.RawResponse
					if len(meta) == 0 {
						meta = []byte("{}")
					}
					_ = s.repo.WithTx(ctx, func(q *db.Queries) error {
						_, err := q.UpdateMarketplaceRefundStatus(ctx, db.UpdateMarketplaceRefundStatusParams{
							ID:                      createdRefund.ID,
							Status:                  "pending",
							ProviderRefundReference: refundOut.RefundReference,
							ConfirmedByUserID:       pgtype.Text{Valid: false},
							SettlementNotes:         "Pending provider settlement",
							Metadata:                meta,
						})
						return err
					})
					createdRefund.ProviderRefundReference = refundOut.RefundReference
				}
			}
		}
	}

	_ = s.audit.Log(ctx, &admin.ID, &admin.PrimaryRole, "initiate_marketplace_refund", "marketplace_refund", createdRefund.ID, nil, nil, &input.Reason, nil)

	refreshedOrder, err := s.repo.Queries().GetMarketplaceOrderByID(ctx, orderID)
	if err != nil {
		return nil, nil, err
	}
	return &createdRefund, &refreshedOrder, nil
}

func (s *MarketplaceService) ConfirmRefund(ctx context.Context, admin *db.User, refundID string, input ConfirmRefundInput) (*db.MarketplaceRefund, *db.GetMarketplaceOrderByIDRow, error) {
	lockedRefund, err := s.repo.Queries().GetMarketplaceRefundByID(ctx, refundID)
	if err != nil {
		return nil, nil, errors.New("marketplace refund not found")
	}

	if admin != nil {
		apps := s.getActiveAppointments(ctx, admin.ID)
		isNationwide := s.isNationwideAdmin(admin, apps)
		hasCoordRole := s.isCoordinatorUser(admin, apps)
		if !isNationwide && !hasCoordRole {
			return nil, nil, errors.New("unauthorized: administrative permissions required")
		}
		order, err := s.repo.Queries().GetMarketplaceOrderByID(ctx, lockedRefund.OrderID)
		if err == nil {
			stID, znID, lgaID := s.resolveOrderGeographyWithLga(ctx, s.repo.Queries(), &db.MarketplaceOrder{
				ShippingStateID: order.ShippingStateID,
				ShippingLgaID:   order.ShippingLgaID,
				PickupBranchID:  order.PickupBranchID,
			})
			if !isNationwide && !s.hasCoordinatorJurisdictionWithLga(ctx, admin, apps, stID, znID, lgaID) {
				return nil, nil, ErrScopeUnauthorized
			}
		}
	}

	// Idempotent no-op if already successful
	if lockedRefund.Status == "successful" {
		order, err := s.repo.Queries().GetMarketplaceOrderByID(ctx, lockedRefund.OrderID)
		if err != nil {
			return nil, nil, err
		}
		return &lockedRefund, &order, nil
	}
	if lockedRefund.Status != "pending" {
		return nil, nil, fmt.Errorf("cannot confirm refund with status '%s'", lockedRefund.Status)
	}

	// For Paystack refunds:
	// Do NOT trust administrator-supplied references or notes as proof of settlement.
	// Require authoritative confirmation from Paystack that the exact refund has succeeded.
	if lockedRefund.Provider == "paystack" {
		providerRef := strings.TrimSpace(lockedRefund.ProviderRefundReference)
		if providerRef == "" {
			return nil, nil, errors.New("cannot confirm provider refund without recorded provider reference")
		}

		// Reject forged or mismatched provider reference supplied by caller
		if input.ProviderRefundReference != "" && input.ProviderRefundReference != providerRef {
			return nil, nil, errors.New("provided refund reference does not match authoritative provider refund reference")
		}

		// Authoritatively verify with payment gateway
		verifyOut, err := s.paymentGateway.VerifyRefund(ctx, providerRef)
		if err != nil {
			return nil, nil, fmt.Errorf("reliable provider confirmation is unavailable: %w", err)
		}

		// Check terminal status: do not treat pending/processing as settled
		if verifyOut.Status == "pending" {
			return nil, nil, errors.New("provider refund is still pending or processing; cannot mark as settled")
		}
		if verifyOut.Status == "failed" {
			_, _, _ = s.FailRefund(ctx, admin, refundID, "Provider reported refund failure during verification")
			return nil, nil, errors.New("provider refund has failed")
		}
		if verifyOut.Status != "successful" {
			return nil, nil, fmt.Errorf("provider refund returned unverified status: %s", verifyOut.Status)
		}

		// Verify provider refund identifier
		if verifyOut.RefundReference != "" && verifyOut.RefundReference != providerRef {
			return nil, nil, fmt.Errorf("verified provider refund identifier mismatch: expected %s, got %s", providerRef, verifyOut.RefundReference)
		}

		// Verify original transaction reference
		pay, err := s.repo.Queries().GetMarketplacePaymentByOrder(ctx, lockedRefund.OrderID)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to locate order payment: %w", err)
		}
		if verifyOut.TransactionReference != "" && pay.Reference != "" && verifyOut.TransactionReference != pay.Reference && (!pay.ProviderReference.Valid || verifyOut.TransactionReference != pay.ProviderReference.String) {
			return nil, nil, fmt.Errorf("verified provider transaction reference mismatch: expected %s, got %s", pay.Reference, verifyOut.TransactionReference)
		}

		// Verify amount where available
		if verifyOut.AmountKobo > 0 && verifyOut.AmountKobo != lockedRefund.AmountKobo {
			return nil, nil, fmt.Errorf("verified provider refund amount mismatch: expected %d kobo, got %d kobo", lockedRefund.AmountKobo, verifyOut.AmountKobo)
		}

		// Verify currency where available
		if verifyOut.Currency != "" && pay.Currency != "" && !strings.EqualFold(verifyOut.Currency, pay.Currency) {
			return nil, nil, fmt.Errorf("verified provider refund currency mismatch: expected %s, got %s", pay.Currency, verifyOut.Currency)
		}

		return s.settleRefundSuccess(ctx, admin, refundID, providerRef, "Verified with Paystack provider", verifyOut.RawResponse)
	}

	// For sandbox mode
	if lockedRefund.Provider == "dev_sandbox" || lockedRefund.Provider == "sandbox" {
		providerRef := lockedRefund.ProviderRefundReference
		if input.ProviderRefundReference != "" && input.ProviderRefundReference != providerRef {
			return nil, nil, errors.New("provided refund reference does not match authoritative provider refund reference")
		}
		verifyOut, err := s.paymentGateway.VerifyRefund(ctx, providerRef)
		if err != nil {
			return nil, nil, fmt.Errorf("reliable provider confirmation is unavailable: %w", err)
		}
		if verifyOut.Status == "pending" {
			return nil, nil, errors.New("provider refund is still pending or processing; cannot mark as settled")
		}
		if verifyOut.Status == "failed" {
			_, _, _ = s.FailRefund(ctx, admin, refundID, "Provider reported refund failure during verification")
			return nil, nil, errors.New("provider refund has failed")
		}
		if verifyOut.Status != "successful" {
			return nil, nil, fmt.Errorf("provider refund returned unverified status: %s", verifyOut.Status)
		}
		return s.settleRefundSuccess(ctx, admin, refundID, providerRef, "Verified with sandbox provider", verifyOut.RawResponse)
	}

	return nil, nil, errors.New("unsupported refund provider for automated confirmation; manual settlement required")
}

func (s *MarketplaceService) ManualSettleRefund(ctx context.Context, admin *db.User, refundID string, input ManualSettlementInput) (*db.MarketplaceRefund, *db.GetMarketplaceOrderByIDRow, error) {
	// Restrict this workflow strictly to Super Admin
	if admin == nil || admin.PrimaryRole != "super_admin" {
		return nil, nil, errors.New("unauthorized: super admin authorization required for manual settlement")
	}

	evidence := strings.TrimSpace(input.Evidence)
	if evidence == "" {
		return nil, nil, errors.New("settlement evidence is required for manual settlement")
	}

	reason := strings.TrimSpace(input.Reason)
	if reason == "" {
		return nil, nil, errors.New("settlement reason is required for manual settlement")
	}

	refund, err := s.repo.Queries().GetMarketplaceRefundByID(ctx, refundID)
	if err != nil {
		return nil, nil, errors.New("marketplace refund not found")
	}

	if refund.Status == "successful" {
		order, err := s.repo.Queries().GetMarketplaceOrderByID(ctx, refund.OrderID)
		if err != nil {
			return nil, nil, err
		}
		return &refund, &order, nil
	}
	if refund.Status != "pending" {
		return nil, nil, fmt.Errorf("cannot manually settle refund with status '%s'", refund.Status)
	}

	// Do not let it bypass Paystack refund verification for active electronic Paystack payments
	if refund.Provider == "paystack" {
		return nil, nil, errors.New("cannot bypass provider verification for active electronic Paystack refunds; please use provider confirmation or reconciliation")
	}

	settlementNotes := fmt.Sprintf("Manual settlement authorized by Super Admin (%s): %s | Evidence: %s", admin.Email.String, reason, evidence)
	if input.Notes != "" {
		settlementNotes += " | Notes: " + input.Notes
	}

	meta, _ := json.Marshal(map[string]interface{}{
		"manual_settlement":   true,
		"settlement_evidence": evidence,
		"settlement_reason":   reason,
		"settled_by_email":    admin.Email.String,
		"settled_by_id":       admin.ID,
	})

	resRefund, resOrder, err := s.settleRefundSuccess(ctx, admin, refundID, refund.ProviderRefundReference, settlementNotes, meta)
	if err != nil {
		return nil, nil, err
	}

	_ = s.audit.Log(ctx, &admin.ID, &admin.PrimaryRole, "manual_settle_marketplace_refund", "marketplace_refund", refundID, nil, nil, &reason, nil)

	return resRefund, resOrder, nil
}

func (s *MarketplaceService) settleRefundSuccess(ctx context.Context, admin *db.User, refundID string, providerRef string, notes string, metadata []byte) (*db.MarketplaceRefund, *db.GetMarketplaceOrderByIDRow, error) {
	var updatedRefund db.MarketplaceRefund
	var orderID string

	err := s.repo.WithTx(ctx, func(q *db.Queries) error {
		lockedRefund, err := q.GetMarketplaceRefundByIDForUpdate(ctx, refundID)
		if err != nil {
			return errors.New("marketplace refund not found")
		}
		orderID = lockedRefund.OrderID

		// Idempotent no-op if already successful
		if lockedRefund.Status == "successful" {
			updatedRefund = lockedRefund
			return nil
		}
		if lockedRefund.Status == "failed" {
			return errors.New("cannot settle a failed refund")
		}
		if lockedRefund.Status != "pending" {
			return fmt.Errorf("cannot settle refund with status '%s'", lockedRefund.Status)
		}

		lockedOrder, err := q.GetMarketplaceOrderForUpdate(ctx, lockedRefund.OrderID)
		if err != nil {
			return fmt.Errorf("failed to lock order for refund settlement: %w", err)
		}

		if admin != nil {
			apps := s.getActiveAppointments(ctx, admin.ID)
			if !s.isNationwideAdmin(admin, apps) {
				stID, znID, lgaID := s.resolveOrderGeographyWithLga(ctx, q, &lockedOrder)
				if !s.hasCoordinatorJurisdictionWithLga(ctx, admin, apps, stID, znID, lgaID) {
					return ErrScopeUnauthorized
				}
			}
		}

		pay, err := q.GetMarketplacePaymentByOrderForUpdate(ctx, lockedRefund.OrderID)
		if err != nil {
			return fmt.Errorf("failed to locate payment: %w", err)
		}

		pRef := lockedRefund.ProviderRefundReference
		if providerRef != "" {
			pRef = providerRef
		}

		var confirmedBy pgtype.Text
		if admin != nil {
			confirmedBy = TextFromString(admin.ID)
		}

		settleNotes := lockedRefund.SettlementNotes
		if notes != "" {
			settleNotes = notes
		}

		meta := lockedRefund.Metadata
		if len(metadata) > 0 {
			meta = metadata
		}
		if len(meta) == 0 {
			meta = []byte("{}")
		}

		refRecord, err := q.UpdateMarketplaceRefundStatus(ctx, db.UpdateMarketplaceRefundStatusParams{
			ID:                      refundID,
			Status:                  "successful",
			ProviderRefundReference: pRef,
			ConfirmedByUserID:       confirmedBy,
			SettlementNotes:         settleNotes,
			Metadata:                meta,
		})
		if err != nil {
			return fmt.Errorf("failed to update refund record: %w", err)
		}
		updatedRefund = refRecord

		// Recalculate authoritative successful refund sum
		sums, err := q.GetMarketplaceRefundSumsByOrder(ctx, lockedRefund.OrderID)
		if err != nil {
			return fmt.Errorf("failed to calculate refund sums: %w", err)
		}

		isFullRefund := sums.SuccessfulAmountKobo >= lockedOrder.TotalKobo

		// Stock restoration idempotency check:
		// Only restore stock if:
		// 1. Stock has NOT already been released
		// 2. Order was NOT already cancelled or refunded (which already restored stock)
		canRestoreStock := !lockedOrder.StockReleased &&
			lockedOrder.OrderStatus != "cancelled" &&
			lockedOrder.OrderStatus != "refunded"

		if isFullRefund && canRestoreStock &&
			lockedOrder.FulfilmentStatus != "dispatched" &&
			lockedOrder.FulfilmentStatus != "partially_dispatched" &&
			lockedOrder.FulfilmentStatus != "partially_delivered" &&
			lockedOrder.FulfilmentStatus != "delivered" {

			_ = q.CancelMarketplaceShipmentsByOrder(ctx, lockedOrder.ID)
			items, err := q.ListMarketplaceOrderItemsByOrder(ctx, lockedOrder.ID)
			if err != nil {
				return err
			}

			// Aggregate stock restoration by ProductID to resolve VULN-07
			type restockAgg struct {
				totalQty int32
				busID    string
			}
			prodTotals := make(map[string]*restockAgg)
			for _, itm := range items {
				if itm.ProductID.Valid && strings.TrimSpace(itm.ProductID.String) != "" {
					pID := itm.ProductID.String
					if agg, exists := prodTotals[pID]; exists {
						agg.totalQty += itm.Quantity
					} else {
						prodTotals[pID] = &restockAgg{
							totalQty: itm.Quantity,
							busID:    itm.BusinessID,
						}
					}
				}
			}

			for pID, agg := range prodTotals {
				releaseCount, cErr := q.CountOrderInventoryReleaseLogs(ctx, db.CountOrderInventoryReleaseLogsParams{
					OrderID:   TextFromString(lockedOrder.ID),
					ProductID: pID,
				})
				if cErr == nil && releaseCount == 0 {
					incProd, err := q.IncrementProductStock(ctx, db.IncrementProductStockParams{
						ID:            pID,
						StockQuantity: agg.totalQty,
					})
					if err != nil {
						return err
					}
					var actorID pgtype.Text
					if admin != nil {
						actorID = TextFromString(admin.ID)
					}
					_, logErr := q.CreateMarketplaceInventoryLog(ctx, db.CreateMarketplaceInventoryLogParams{
						ID:             security.GenerateUUID(),
						ProductID:      pID,
						BusinessID:     agg.busID,
						ActorUserID:    actorID,
						ActionType:     "refund_restock",
						QuantityDelta:  agg.totalQty,
						BalanceAfter:   incProd.StockQuantity,
						OrderID:        TextFromString(lockedOrder.ID),
						ReferenceID:    refundID,
						Reason:         fmt.Sprintf("Full refund %s settled", refundID),
						IdempotencyKey: TextFromString(fmt.Sprintf("refund_restock:%s:%s", refundID, pID)),
					})
					if logErr != nil {
						return fmt.Errorf("failed to record refund restock inventory log: %w", logErr)
					}
				}
			}
		} else if !isFullRefund && canRestoreStock && lockedRefund.ShipmentID.Valid {
			// Targeted shipment refund: cancel that specific shipment and restock its items
			shp, err := q.GetMarketplaceShipmentByID(ctx, lockedRefund.ShipmentID.String)
			if err == nil && (shp.Status == "pending" || shp.Status == "processing" || shp.Status == "ready_for_pickup") {
				_, _ = q.CancelMarketplaceShipmentByID(ctx, shp.ID)

				type restockAgg struct {
					totalQty int32
					busID    string
				}
				shipProdTotals := make(map[string]*restockAgg)
				sItems, sErr := q.ListMarketplaceShipmentItemsByShipment(ctx, shp.ID)
				if sErr == nil {
					for _, si := range sItems {
						if si.ProductID.Valid && strings.TrimSpace(si.ProductID.String) != "" {
							pID := si.ProductID.String
							if agg, exists := shipProdTotals[pID]; exists {
								agg.totalQty += si.Quantity
							} else {
								shipProdTotals[pID] = &restockAgg{
									totalQty: si.Quantity,
									busID:    si.BusinessID,
								}
							}
						}
					}

					for pID, agg := range shipProdTotals {
						releaseCount, cErr := q.CountOrderInventoryReleaseLogs(ctx, db.CountOrderInventoryReleaseLogsParams{
							OrderID:   TextFromString(lockedOrder.ID),
							ProductID: pID,
						})
						if cErr == nil && releaseCount == 0 {
							incProd, err := q.IncrementProductStock(ctx, db.IncrementProductStockParams{
								ID:            pID,
								StockQuantity: agg.totalQty,
							})
							if err == nil {
								var actorID pgtype.Text
								if admin != nil {
									actorID = TextFromString(admin.ID)
								}
								_, _ = q.CreateMarketplaceInventoryLog(ctx, db.CreateMarketplaceInventoryLogParams{
									ID:             security.GenerateUUID(),
									ProductID:      pID,
									BusinessID:     agg.busID,
									ActorUserID:    actorID,
									ActionType:     "refund_restock",
									QuantityDelta:  agg.totalQty,
									BalanceAfter:   incProd.StockQuantity,
									OrderID:        TextFromString(lockedOrder.ID),
									ReferenceID:    refundID,
									Reason:         fmt.Sprintf("Partial shipment refund %s settled", refundID),
									IdempotencyKey: TextFromString(fmt.Sprintf("refund_restock:%s:%s", refundID, pID)),
								})
							}
						}
					}
				}
			}
		}

		newOrderStatus := lockedOrder.OrderStatus
		newPaymentStatus := "partially_refunded"
		if isFullRefund {
			newOrderStatus = "refunded"
			newPaymentStatus = "refunded"
		}

		if newOrderStatus != lockedOrder.OrderStatus {
			_, err = q.UpdateMarketplaceOrderStatus(ctx, db.UpdateMarketplaceOrderStatusParams{
				ID:          lockedOrder.ID,
				OrderStatus: newOrderStatus,
			})
			if err != nil {
				return err
			}
		}

		if isFullRefund {
			_, _ = q.UpdateMarketplaceOrderExpiryFields(ctx, db.UpdateMarketplaceOrderExpiryFieldsParams{
				ID:                     lockedOrder.ID,
				ReservationExpiresAt:   lockedOrder.ReservationExpiresAt,
				CancellationReason:     lockedOrder.CancellationReason,
				StockReleased:          true,
				ReconciliationRequired: lockedOrder.ReconciliationRequired,
				ReconciliationNotes:    lockedOrder.ReconciliationNotes,
			})
		} else {
			// Update fulfilment and order status aggregation across active shipments
			_ = s.aggregateOrderStatusTx(ctx, q, lockedOrder.ID)
		}

		_, err = q.UpdateMarketplaceOrderPaymentStatus(ctx, db.UpdateMarketplaceOrderPaymentStatusParams{
			ID:            lockedOrder.ID,
			PaymentStatus: newPaymentStatus,
		})
		if err != nil {
			return err
		}

		_, err = q.UpdateMarketplacePaymentTransactionStatus(ctx, db.UpdateMarketplacePaymentTransactionStatusParams{
			Reference:         pay.Reference,
			Status:            newPaymentStatus,
			ProviderReference: pay.ProviderReference,
			Channel:           pay.Channel,
			PaidAt:            pay.PaidAt,
			RawWebhookPayload: pay.RawWebhookPayload, // Preserve original Paystack payload!
			ErrorMessage:      TextFromString(fmt.Sprintf("Refund %s settled (total refunded: %d kobo)", refundID, sums.SuccessfulAmountKobo)),
		})
		if err != nil {
			return fmt.Errorf("failed to update payment record: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	var actorID, role *string
	if admin != nil {
		actorID = &admin.ID
		role = &admin.PrimaryRole
	}
	_ = s.audit.Log(ctx, actorID, role, "confirm_marketplace_refund", "marketplace_refund", refundID, nil, nil, nil, nil)

	refreshedOrder, err := s.repo.Queries().GetMarketplaceOrderByID(ctx, orderID)
	if err != nil {
		return nil, nil, err
	}
	return &updatedRefund, &refreshedOrder, nil
}

func (s *MarketplaceService) FailRefund(ctx context.Context, admin *db.User, refundID string, failureReason string) (*db.MarketplaceRefund, *db.GetMarketplaceOrderByIDRow, error) {
	if admin != nil && !isAdmin(admin) {
		return nil, nil, errors.New("unauthorized: administrative permissions required")
	}

	var updatedRefund db.MarketplaceRefund
	var orderID string

	err := s.repo.WithTx(ctx, func(q *db.Queries) error {
		lockedRefund, err := q.GetMarketplaceRefundByIDForUpdate(ctx, refundID)
		if err != nil {
			return errors.New("marketplace refund not found")
		}
		orderID = lockedRefund.OrderID

		if lockedRefund.Status == "failed" {
			updatedRefund = lockedRefund
			return nil
		}
		if lockedRefund.Status != "pending" {
			return fmt.Errorf("cannot fail refund with status '%s'", lockedRefund.Status)
		}

		meta := lockedRefund.Metadata
		if len(meta) == 0 {
			meta = []byte("{}")
		}

		refRecord, err := q.UpdateMarketplaceRefundStatus(ctx, db.UpdateMarketplaceRefundStatusParams{
			ID:                      refundID,
			Status:                  "failed",
			ProviderRefundReference: lockedRefund.ProviderRefundReference,
			ConfirmedByUserID:       pgtype.Text{Valid: false},
			SettlementNotes:         failureReason,
			Metadata:                meta,
		})
		if err != nil {
			return fmt.Errorf("failed to update refund record: %w", err)
		}
		updatedRefund = refRecord
		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	var actorID, role *string
	if admin != nil {
		actorID = &admin.ID
		role = &admin.PrimaryRole
	}
	_ = s.audit.Log(ctx, actorID, role, "fail_marketplace_refund", "marketplace_refund", refundID, nil, nil, &failureReason, nil)

	refreshedOrder, err := s.repo.Queries().GetMarketplaceOrderByID(ctx, orderID)
	if err != nil {
		return nil, nil, err
	}
	return &updatedRefund, &refreshedOrder, nil
}

func (s *MarketplaceService) GetRefundByID(ctx context.Context, admin *db.User, refundID string) (*db.MarketplaceRefund, error) {
	if !isAdmin(admin) {
		return nil, errors.New("unauthorized: administrative permissions required")
	}
	r, err := s.repo.Queries().GetMarketplaceRefundByID(ctx, refundID)
	if err != nil {
		return nil, errors.New("marketplace refund not found")
	}
	return &r, nil
}

func (s *MarketplaceService) ListRefundsByOrder(ctx context.Context, user *db.User, orderID string) ([]db.MarketplaceRefund, error) {
	if user == nil {
		return nil, ErrUnauthorizedBuyerAction
	}
	order, err := s.repo.Queries().GetMarketplaceOrderByID(ctx, orderID)
	if err != nil {
		return nil, ErrOrderNotFound
	}

	isBuyer := order.BuyerUserID.Valid && order.BuyerUserID.String == user.ID
	if !isAdmin(user) && !isBuyer {
		return nil, ErrUnauthorizedBuyerAction
	}

	list, err := s.repo.Queries().ListMarketplaceRefundsByOrder(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if list == nil {
		list = []db.MarketplaceRefund{}
	}
	return list, nil
}

// -------------------------------------------------------------
// Disputes & Customer Protection
// -------------------------------------------------------------

type CreateDisputeInput struct {
	Reason        string `json:"reason"`
	CustomerNotes string `json:"customer_notes"`
}

func (s *MarketplaceService) RaiseDispute(ctx context.Context, user *db.User, orderID string, input CreateDisputeInput) (*db.MarketplaceDispute, error) {
	order, err := s.repo.Queries().GetMarketplaceOrderByID(ctx, orderID)
	if err != nil {
		return nil, ErrOrderNotFound
	}

	if order.BuyerUserID.Valid && order.BuyerUserID.String != user.ID && !isAdmin(user) {
		return nil, ErrUnauthorizedBuyerAction
	}

	disputeID := security.GenerateUUID()
	d, err := s.repo.Queries().CreateMarketplaceDispute(ctx, db.CreateMarketplaceDisputeParams{
		ID:            disputeID,
		OrderID:       orderID,
		RaisedByUserID: user.ID,
		Reason:        input.Reason,
		CustomerNotes: input.CustomerNotes,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create dispute: %w", err)
	}

	_ = s.audit.Log(ctx, &user.ID, &user.PrimaryRole, "raise_marketplace_dispute", "marketplace_dispute", d.ID, nil, nil, nil, nil)
	return &d, nil
}

func (s *MarketplaceService) ResolveDispute(ctx context.Context, admin *db.User, disputeID string, resolution, notes string) (*db.MarketplaceDispute, error) {
	if !isAdmin(admin) {
		return nil, errors.New("unauthorized: administrative permissions required")
	}

	d, err := s.repo.Queries().GetMarketplaceDisputeByID(ctx, disputeID)
	if err != nil {
		return nil, ErrDisputeNotFound
	}

	now := time.Now()
	status := "resolved_refunded"
	if resolution == "rejected" {
		status = "rejected"
	}

	updatedD, err := s.repo.Queries().ResolveMarketplaceDispute(ctx, db.ResolveMarketplaceDisputeParams{
		ID:               disputeID,
		Status:           status,
		ResolutionNotes:  TextFromString(notes),
		ResolvedByUserID: TextFromString(admin.ID),
		ResolvedAt:       TimestamptzFromTime(now),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to resolve dispute: %w", err)
	}

	// If resolved with refund, restore stock quantities and set order and payment status to refunded
	if status == "resolved_refunded" {
		_ = s.repo.WithTx(ctx, func(q *db.Queries) error {
			items, _ := q.ListMarketplaceOrderItemsByOrder(ctx, d.OrderID)
			for _, itm := range items {
				if itm.ProductID.Valid {
					_, _ = q.IncrementProductStock(ctx, db.IncrementProductStockParams{
						ID:            itm.ProductID.String,
						StockQuantity: itm.Quantity,
					})
				}
			}
			_, _ = q.UpdateMarketplaceOrderStatus(ctx, db.UpdateMarketplaceOrderStatusParams{
				ID:          d.OrderID,
				OrderStatus: "refunded",
			})
			_, _ = q.UpdateMarketplaceOrderPaymentStatus(ctx, db.UpdateMarketplaceOrderPaymentStatusParams{
				ID:            d.OrderID,
				PaymentStatus: "refunded",
			})
			pay, err := q.GetMarketplacePaymentByOrder(ctx, d.OrderID)
			if err == nil {
				_, _ = q.UpdateMarketplacePaymentTransactionStatus(ctx, db.UpdateMarketplacePaymentTransactionStatusParams{
					Reference:         pay.Reference,
					Status:            "refunded",
					ProviderReference: pay.ProviderReference,
					Channel:           pay.Channel,
					PaidAt:            pay.PaidAt,
					RawWebhookPayload: pay.RawWebhookPayload,
					ErrorMessage:      TextFromString("Refunded via dispute resolution"),
				})
			}
			return nil
		})
	}

	_ = s.audit.Log(ctx, &admin.ID, &admin.PrimaryRole, "resolve_marketplace_dispute", "marketplace_dispute", disputeID, nil, nil, &notes, nil)
	return &updatedD, nil
}

func (s *MarketplaceService) ListDisputes(ctx context.Context, admin *db.User, status *string) ([]db.ListAllMarketplaceDisputesRow, error) {
	if !isAdmin(admin) {
		return nil, errors.New("unauthorized")
	}
	st := ""
	if status != nil {
		st = *status
	}
	list, err := s.repo.Queries().ListAllMarketplaceDisputes(ctx, st)
	if err != nil {
		return nil, err
	}
	if list == nil {
		list = []db.ListAllMarketplaceDisputesRow{}
	}
	return list, nil
}

// -------------------------------------------------------------
// Overview Analytics
// -------------------------------------------------------------

func (s *MarketplaceService) GetAdminOverviewMetrics(ctx context.Context, admin *db.User) (*db.GetMarketplaceAdminOverviewMetricsRow, error) {
	if !isAdmin(admin) {
		return nil, errors.New("unauthorized: administrative permissions required")
	}
	metrics, err := s.repo.Queries().GetMarketplaceAdminOverviewMetrics(ctx)
	if err != nil {
		return nil, err
	}
	return &metrics, nil
}

func (s *MarketplaceService) GetSellerOverviewMetrics(ctx context.Context, user *db.User) (*db.GetMarketplaceSellerOverviewMetricsRow, error) {
	b, err := s.repo.Queries().GetMarketplaceBusinessByOwner(ctx, user.ID)
	if err != nil {
		return nil, ErrBusinessNotFound
	}
	metrics, err := s.repo.Queries().GetMarketplaceSellerOverviewMetrics(ctx, b.ID)
	if err != nil {
		return nil, err
	}
	return &metrics, nil
}

func (s *MarketplaceService) ListPayments(ctx context.Context, admin *db.User, status, provider *string) ([]db.ListMarketplacePaymentsRow, error) {
	if !isAdmin(admin) {
		return nil, errors.New("unauthorized")
	}
	st := ""
	if status != nil {
		st = *status
	}
	pr := ""
	if provider != nil {
		pr = *provider
	}
	list, err := s.repo.Queries().ListMarketplacePayments(ctx, db.ListMarketplacePaymentsParams{
		Column1: st,
		Column2: pr,
	})
	if err != nil {
		return nil, err
	}
	if list == nil {
		list = []db.ListMarketplacePaymentsRow{}
	}
	return list, nil
}

// -------------------------------------------------------------
// Helper functions
// -------------------------------------------------------------

func (s *MarketplaceService) isNationwideAdmin(u *db.User, appointments []db.Appointment) bool {
	if u == nil {
		return false
	}
	if u.PrimaryRole == "super_admin" || u.PrimaryRole == "national_admin" {
		return true
	}
	for _, a := range appointments {
		if a.Status != "active" {
			continue
		}
		if a.RoleType == "super_admin" || a.RoleType == "national_admin" || strings.ToLower(a.JurisdictionType) == "national" {
			return true
		}
	}
	return false
}

func (s *MarketplaceService) hasAnyCoordinatorAppointment(appointments []db.Appointment) bool {
	for _, a := range appointments {
		if a.Status != "active" {
			continue
		}
		if a.RoleType == "state_coordinator" || a.RoleType == "zonal_coordinator" ||
			a.RoleType == "lga_coordinator" || a.RoleType == "lga_representative" ||
			strings.ToLower(a.JurisdictionType) == "state" || strings.ToLower(a.JurisdictionType) == "zone" ||
			strings.ToLower(a.JurisdictionType) == "zonal" || strings.ToLower(a.JurisdictionType) == "lga" {
			return true
		}
	}
	return false
}

func (s *MarketplaceService) isCoordinatorUser(user *db.User, appointments []db.Appointment) bool {
	if user == nil {
		return false
	}
	if user.PrimaryRole == "state_coordinator" || user.PrimaryRole == "zonal_coordinator" ||
		user.PrimaryRole == "lga_coordinator" || user.PrimaryRole == "lga_representative" {
		return true
	}
	return s.hasAnyCoordinatorAppointment(appointments)
}

func (s *MarketplaceService) getActiveAppointments(ctx context.Context, userID string) []db.Appointment {
	if strings.TrimSpace(userID) == "" {
		return nil
	}
	apps, err := s.repo.Queries().ListActiveAppointmentsByUser(ctx, userID)
	if err != nil {
		return nil
	}
	now := time.Now()
	var valid []db.Appointment
	for _, a := range apps {
		if a.Status == "active" {
			if !a.EndDate.Valid || a.EndDate.Time.After(now) {
				valid = append(valid, a)
			}
		}
	}
	return valid
}

func (s *MarketplaceService) resolveOrderGeography(ctx context.Context, q *db.Queries, order *db.MarketplaceOrder) (stateID string, zoneID string, err error) {
	if order == nil {
		return "", "", nil
	}
	stID, znID := s.resolveOrderGeographyRow(ctx, q, order.ShippingStateID, order.PickupBranchID)
	return stID, znID, nil
}

func (s *MarketplaceService) resolveOrderGeographyRow(ctx context.Context, q *db.Queries, shippingStateID, pickupBranchID pgtype.Text) (stateID string, zoneID string) {
	if shippingStateID.Valid && strings.TrimSpace(shippingStateID.String) != "" {
		stateID = shippingStateID.String
	} else if pickupBranchID.Valid && strings.TrimSpace(pickupBranchID.String) != "" {
		br, err := q.GetBusinessBranchByID(ctx, pickupBranchID.String)
		if err == nil {
			stateID = br.StateID
		}
	}

	if stateID != "" {
		st, err := q.GetStateByID(ctx, stateID)
		if err == nil {
			zoneID = st.ZoneID
		}
	}
	return stateID, zoneID
}

func (s *MarketplaceService) hasCoordinatorJurisdiction(user *db.User, appointments []db.Appointment, orderStateID, orderZoneID string) bool {
	if user == nil {
		return false
	}
	if s.isNationwideAdmin(user, appointments) {
		return true
	}

	for _, a := range appointments {
		if a.Status != "active" {
			continue
		}
		jType := strings.ToLower(a.JurisdictionType)
		jID := ""
		if a.JurisdictionID.Valid {
			jID = a.JurisdictionID.String
		}

		// State coordinator
		if (a.RoleType == "state_coordinator" || jType == "state") && jID != "" {
			if orderStateID != "" && jID == orderStateID {
				return true
			}
		}

		// Zonal coordinator
		if (a.RoleType == "zonal_coordinator" || jType == "zone" || jType == "zonal") && jID != "" {
			if orderZoneID != "" && jID == orderZoneID {
				return true
			}
		}
	}

	// Fallback for user.PrimaryRole if appointment is not in appointments table
	if user.PrimaryRole == "state_coordinator" && orderStateID != "" {
		prof, err := s.repo.Queries().GetMemberProfileByUserID(context.Background(), user.ID)
		if err == nil && prof.HomeStateID.Valid && prof.HomeStateID.String == orderStateID {
			return true
		}
	}
	if user.PrimaryRole == "zonal_coordinator" && orderZoneID != "" {
		prof, err := s.repo.Queries().GetMemberProfileByUserID(context.Background(), user.ID)
		if err == nil && prof.HomeStateID.Valid && prof.HomeStateID.String != "" {
			st, err := s.repo.Queries().GetStateByID(context.Background(), prof.HomeStateID.String)
			if err == nil && st.ZoneID == orderZoneID {
				return true
			}
		}
	}

	return false
}

func isAdmin(u *db.User) bool {
	if u == nil {
		return false
	}
	return u.PrimaryRole == "super_admin" || u.PrimaryRole == "national_admin" ||
		u.PrimaryRole == "zonal_coordinator" || u.PrimaryRole == "state_coordinator"
}

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var out []rune
	lastWasDash := false
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			out = append(out, r)
			lastWasDash = false
		} else if r == ' ' || r == '-' || r == '_' {
			if !lastWasDash && len(out) > 0 {
				out = append(out, '-')
				lastWasDash = true
			}
		}
	}
	res := strings.Trim(string(out), "-")
	if res == "" {
		res = "item"
	}
	return res
}

func stringPtr(s string) *string {
	return &s
}

// -------------------------------------------------------------
// Inventory Ledger & Reservation Expiry
// -------------------------------------------------------------

type ExpireReservationsResult struct {
	ProcessedCount      int      `json:"processed_count"`
	ExpiredCount        int      `json:"expired_count"`
	ConfirmedPaidCount  int      `json:"confirmed_paid_count"`
	ReconciliationCount int      `json:"reconciliation_count"`
	Errors              []string `json:"errors,omitempty"`
}

func (s *MarketplaceService) ExpireStaleReservations(ctx context.Context, batchSize int) (*ExpireReservationsResult, error) {
	if batchSize <= 0 {
		batchSize = 50
	}
	if batchSize > 200 {
		batchSize = 200
	}

	now := time.Now()
	expiredOrders, err := s.repo.Queries().ListExpiredPendingMarketplaceOrders(ctx, db.ListExpiredPendingMarketplaceOrdersParams{
		ReservationExpiresAt: TimestamptzFromTime(now),
		Limit:                int32(batchSize),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list expired pending orders: %w", err)
	}

	result := &ExpireReservationsResult{
		ProcessedCount: len(expiredOrders),
	}

	for _, ord := range expiredOrders {
		// 1. Look up payment record
		payRec, pErr := s.repo.Queries().GetMarketplacePaymentByOrder(ctx, ord.ID)

		// 2. Query provider status OUTSIDE any database transaction if gateway configured
		providerPaid := false
		providerInconclusive := false
		var verifyErr error

		if s.paymentGateway.ProviderName() != "unconfigured" && pErr == nil && payRec.Reference != "" {
			verifyOut, err := s.paymentGateway.VerifyPayment(ctx, payRec.Reference)
			if err != nil {
				// Treat ALL provider errors (HTTP 429, 500, 502, timeouts, network, malformed) as inconclusive!
				providerInconclusive = true
				verifyErr = err
			} else if verifyOut.Status == "successful" {
				providerPaid = true
			} else if verifyOut.Status != "failed" && verifyOut.Status != "abandoned" && verifyOut.Status != "reversed" {
				// Status is non-terminal or unknown (e.g. "pending", "processing", "queued", etc.)
				// Never release stock when provider verification is non-terminal or unknown.
				providerInconclusive = true
				verifyErr = fmt.Errorf("inconclusive provider payment status: %s", verifyOut.Status)
			}
		}

		// 3. If provider was inconclusive: flag for administrative reconciliation, do NOT cancel, do NOT release stock!
		if providerInconclusive {
			_ = s.repo.WithTx(ctx, func(q *db.Queries) error {
				lockedOrd, err := q.GetMarketplaceOrderForUpdate(ctx, ord.ID)
				if err != nil {
					return err
				}
				if lockedOrd.StockReleased || lockedOrd.OrderStatus != "placed" {
					return nil
				}
				_, err = q.MarkOrderRequiresReconciliation(ctx, db.MarkOrderRequiresReconciliationParams{
					ID:                  ord.ID,
					ReconciliationNotes: fmt.Sprintf("Payment verification inconclusive during reservation expiry check: %v", verifyErr),
				})
				return err
			})
			result.ReconciliationCount++
			result.Errors = append(result.Errors, fmt.Sprintf("order %s flagged for reconciliation: provider verification inconclusive (%v)", ord.OrderNumber, verifyErr))
			continue
		}

		// 4. If provider verified paid: confirm payment atomically!
		if providerPaid {
			_, err := s.ConfirmPayment(ctx, payRec.Reference)
			if err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("failed to confirm payment for order %s: %v", ord.OrderNumber, err))
			} else {
				result.ConfirmedPaidCount++
			}
			continue
		}

		// 5. Provider confirms unpaid / abandoned, or unconfigured / dev sandbox with no payment:
		// Execute safe atomic expiration with row locks:
		err = s.repo.WithTx(ctx, func(q *db.Queries) error {
			lockedOrd, err := q.GetMarketplaceOrderForUpdate(ctx, ord.ID)
			if err != nil {
				return err
			}

			// Concurrency guard: If already handled, paid, or stock released
			if lockedOrd.StockReleased || lockedOrd.OrderStatus != "placed" || (lockedOrd.PaymentStatus != "pending" && lockedOrd.PaymentStatus != "failed") {
				return nil
			}

			// Release reserved stock for each item if not already released
			items, err := q.ListMarketplaceOrderItemsByOrder(ctx, ord.ID)
			if err != nil {
				return err
			}

			for _, itm := range items {
				if itm.ProductID.Valid {
					releaseCount, cErr := q.CountOrderInventoryReleaseLogs(ctx, db.CountOrderInventoryReleaseLogsParams{
						OrderID:   TextFromString(ord.ID),
						ProductID: itm.ProductID.String,
					})
					if cErr == nil && releaseCount == 0 {
						incProd, err := q.IncrementProductStock(ctx, db.IncrementProductStockParams{
							ID:            itm.ProductID.String,
							StockQuantity: itm.Quantity,
						})
						if err != nil {
							return err
						}
						var prodBusID string
						p, pErr := q.GetMarketplaceProductByID(ctx, itm.ProductID.String)
						if pErr == nil {
							prodBusID = p.BusinessID
						}
						_, logErr := q.CreateMarketplaceInventoryLog(ctx, db.CreateMarketplaceInventoryLogParams{
							ID:             security.GenerateUUID(),
							ProductID:      itm.ProductID.String,
							BusinessID:     prodBusID,
							ActorUserID:    pgtype.Text{Valid: false},
							ActionType:     "reservation_release",
							QuantityDelta:  itm.Quantity,
							BalanceAfter:   incProd.StockQuantity,
							OrderID:        TextFromString(ord.ID),
							ReferenceID:    ord.OrderNumber,
							Reason:         "Reservation expired: payment timeout",
							IdempotencyKey: TextFromString(fmt.Sprintf("release:%s:%s", ord.ID, itm.ProductID.String)),
						})
						if logErr != nil {
							return fmt.Errorf("failed to record expiry inventory release log: %w", logErr)
						}
					}
				}
			}

			// Mark order cancelled with payment_timeout reason and stock_released = true
			_, err = q.MarkOrderCancelledWithReason(ctx, db.MarkOrderCancelledWithReasonParams{
				ID:                 ord.ID,
				CancellationReason: "payment_timeout",
				StockReleased:      true,
			})
			if err != nil {
				return err
			}

			// Update payment to failed
			_, _ = q.UpdateMarketplaceOrderPaymentStatus(ctx, db.UpdateMarketplaceOrderPaymentStatusParams{
				ID:            ord.ID,
				PaymentStatus: "failed",
			})

			lockedPay, pErr := q.GetMarketplacePaymentByOrderForUpdate(ctx, ord.ID)
			if pErr == nil {
				_, _ = q.UpdateMarketplacePaymentTransactionStatus(ctx, db.UpdateMarketplacePaymentTransactionStatusParams{
					Reference:         lockedPay.Reference,
					Status:            "failed",
					ProviderReference: lockedPay.ProviderReference,
					Channel:           lockedPay.Channel,
					PaidAt:            lockedPay.PaidAt,
					RawWebhookPayload: lockedPay.RawWebhookPayload,
					ErrorMessage:      TextFromString("Reservation expired: payment timeout"),
				})
			}

			return nil
		})

		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("failed to expire order %s: %v", ord.OrderNumber, err))
		} else {
			result.ExpiredCount++
			_ = s.audit.Log(ctx, nil, nil, "expire_marketplace_order_reservation", "marketplace_order", ord.ID, nil, nil, nil, nil)
		}
	}

	return result, nil
}

type AdjustInventoryInput struct {
	IdempotencyKey string `json:"idempotency_key"`
	QuantityDelta  int32  `json:"quantity_delta"`
	Reason         string `json:"reason"`
	ReferenceID    string `json:"reference_id,omitempty"`
}

func (s *MarketplaceService) AdjustInventory(ctx context.Context, user *db.User, productID string, input AdjustInventoryInput) (*db.MarketplaceProduct, *db.MarketplaceInventoryLog, error) {
	if user == nil {
		return nil, nil, errors.New("authentication required")
	}
	idempotencyKey := strings.TrimSpace(input.IdempotencyKey)
	if idempotencyKey == "" {
		return nil, nil, ErrIdempotencyKeyRequired
	}
	if len(idempotencyKey) > 128 {
		return nil, nil, ErrIdempotencyKeyTooLong
	}
	reason := strings.TrimSpace(input.Reason)
	if reason == "" {
		return nil, nil, errors.New("adjustment reason is required")
	}
	if input.QuantityDelta == 0 {
		return nil, nil, errors.New("quantity delta cannot be zero")
	}

	var updatedProd db.MarketplaceProduct
	var invLog db.MarketplaceInventoryLog

	err := s.repo.WithTx(ctx, func(q *db.Queries) error {
		lockedProd, err := q.GetMarketplaceProductByIDForUpdate(ctx, productID)
		if err != nil {
			return ErrProductNotFound
		}

		// Authorization: Owner or Admin
		apps := s.getActiveAppointments(ctx, user.ID)
		isNationwide := s.isNationwideAdmin(user, apps)
		isOwner := lockedProd.BusinessOwnerID == user.ID

		if !isOwner && !isNationwide {
			hasCoordRole := user.PrimaryRole == "state_coordinator" || user.PrimaryRole == "zonal_coordinator" || s.hasAnyCoordinatorAppointment(apps)
			if hasCoordRole {
				branches, bErr := q.ListBusinessBranchesByBusiness(ctx, lockedProd.BusinessID)
				inJurisdiction := false
				if bErr == nil {
					for _, br := range branches {
						stID := br.StateID
						znID := ""
						st, sErr := q.GetStateByID(ctx, stID)
						if sErr == nil {
							znID = st.ZoneID
						}
						if s.hasCoordinatorJurisdiction(user, apps, stID, znID) {
							inJurisdiction = true
							break
						}
					}
				}
				if !inJurisdiction {
					return ErrScopeUnauthorized
				}
			} else {
				return ErrUnauthorizedSellerAction
			}
		}

		// Enforce Idempotency in PostgreSQL:
		// Check if a ledger record with this exact idempotency key already exists.
		existingLog, err := q.GetMarketplaceInventoryLogByIdempotencyKey(ctx, TextFromString(idempotencyKey))
		if err == nil && existingLog.ID != "" {
			// Idempotency key already exists. Verify identical payload:
			if existingLog.ProductID == productID &&
				existingLog.QuantityDelta == input.QuantityDelta &&
				existingLog.Reason == reason {
				// Exact duplicate request: return existing product and log without altering inventory
				invLog = existingLog
				updatedProd = db.MarketplaceProduct{
					ID:                   lockedProd.ID,
					BusinessID:           lockedProd.BusinessID,
					BranchID:             lockedProd.BranchID,
					Title:                lockedProd.Title,
					Slug:                 lockedProd.Slug,
					Description:          lockedProd.Description,
					Category:             lockedProd.Category,
					Sku:                  lockedProd.Sku,
					PriceKobo:            lockedProd.PriceKobo,
					CompareAtPriceKobo:   lockedProd.CompareAtPriceKobo,
					StockQuantity:        lockedProd.StockQuantity,
					LowStockThreshold:    lockedProd.LowStockThreshold,
					Images:               lockedProd.Images,
					WeightGrams:          lockedProd.WeightGrams,
					DeliveryRestrictions: lockedProd.DeliveryRestrictions,
					ReturnPolicy:         lockedProd.ReturnPolicy,
					Status:               lockedProd.Status,
					IsFeatured:           lockedProd.IsFeatured,
					CreatedAt:            lockedProd.CreatedAt,
					UpdatedAt:            lockedProd.UpdatedAt,
					Version:              lockedProd.Version,
				}
				return nil
			}
			// Same key used with different payload: reject conflicting reuse
			return ErrIdempotencyConflict
		}

		newStock := lockedProd.StockQuantity + input.QuantityDelta
		if newStock < 0 {
			return fmt.Errorf("insufficient stock: adjustment would result in negative stock balance (%d)", newStock)
		}

		up, err := q.SetProductStockQuantity(ctx, db.SetProductStockQuantityParams{
			ID:            productID,
			StockQuantity: newStock,
		})
		if err != nil {
			return fmt.Errorf("failed to update stock quantity: %w", err)
		}
		updatedProd = up

		refID := input.ReferenceID
		if refID == "" {
			refID = fmt.Sprintf("ADJ-%s", security.GenerateUUID()[:8])
		}

		logEntry, err := q.CreateMarketplaceInventoryLog(ctx, db.CreateMarketplaceInventoryLogParams{
			ID:             security.GenerateUUID(),
			ProductID:      productID,
			BusinessID:     lockedProd.BusinessID,
			ActorUserID:    TextFromString(user.ID),
			ActionType:     "manual_adjustment",
			QuantityDelta:  input.QuantityDelta,
			BalanceAfter:   newStock,
			OrderID:        pgtype.Text{Valid: false},
			ReferenceID:    refID,
			Reason:         reason,
			IdempotencyKey: TextFromString(idempotencyKey),
		})
		if err != nil {
			return fmt.Errorf("failed to create inventory log: %w", err)
		}
		invLog = logEntry
		return nil
	})

	if err != nil {
		return nil, nil, err
	}

	_ = s.audit.Log(ctx, &user.ID, &user.PrimaryRole, "adjust_marketplace_inventory", "marketplace_product", productID, nil, nil, &reason, nil)

	return &updatedProd, &invLog, nil
}

func (s *MarketplaceService) ListProductInventoryLogs(ctx context.Context, user *db.User, productID string) ([]db.MarketplaceInventoryLog, error) {
	if user == nil {
		return nil, errors.New("authentication required")
	}

	prod, err := s.repo.Queries().GetMarketplaceProductByID(ctx, productID)
	if err != nil {
		return nil, ErrProductNotFound
	}

	apps := s.getActiveAppointments(ctx, user.ID)
	isNationwide := s.isNationwideAdmin(user, apps)
	isOwner := prod.BusinessOwnerID == user.ID

	if !isOwner && !isNationwide {
		hasCoordRole := user.PrimaryRole == "state_coordinator" || user.PrimaryRole == "zonal_coordinator" || s.hasAnyCoordinatorAppointment(apps)
		if hasCoordRole {
			branches, bErr := s.repo.Queries().ListBusinessBranchesByBusiness(ctx, prod.BusinessID)
			inJurisdiction := false
			if bErr == nil {
				for _, br := range branches {
					stID := br.StateID
					znID := ""
					st, sErr := s.repo.Queries().GetStateByID(ctx, stID)
					if sErr == nil {
						znID = st.ZoneID
					}
					if s.hasCoordinatorJurisdiction(user, apps, stID, znID) {
						inJurisdiction = true
						break
					}
				}
			}
			if !inJurisdiction {
				return nil, ErrScopeUnauthorized
			}
		} else {
			return nil, ErrUnauthorizedSellerAction
		}
	}

	logs, err := s.repo.Queries().ListMarketplaceInventoryLogsByProduct(ctx, productID)
	if err != nil {
		return nil, err
	}
	if logs == nil {
		logs = []db.MarketplaceInventoryLog{}
	}
	return logs, nil
}

func (s *MarketplaceService) ListBusinessInventoryLogs(ctx context.Context, user *db.User, businessID string) ([]db.MarketplaceInventoryLog, error) {
	if user == nil {
		return nil, errors.New("authentication required")
	}

	b, err := s.repo.Queries().GetMarketplaceBusinessByID(ctx, businessID)
	if err != nil {
		return nil, ErrBusinessNotFound
	}

	apps := s.getActiveAppointments(ctx, user.ID)
	isNationwide := s.isNationwideAdmin(user, apps)
	isOwner := b.OwnerUserID == user.ID

	if !isOwner && !isNationwide {
		hasCoordRole := user.PrimaryRole == "state_coordinator" || user.PrimaryRole == "zonal_coordinator" || s.hasAnyCoordinatorAppointment(apps)
		if hasCoordRole {
			branches, bErr := s.repo.Queries().ListBusinessBranchesByBusiness(ctx, b.ID)
			inJurisdiction := false
			if bErr == nil {
				for _, br := range branches {
					stID := br.StateID
					znID := ""
					st, sErr := s.repo.Queries().GetStateByID(ctx, stID)
					if sErr == nil {
						znID = st.ZoneID
					}
					if s.hasCoordinatorJurisdiction(user, apps, stID, znID) {
						inJurisdiction = true
						break
					}
				}
			}
			if !inJurisdiction {
				return nil, ErrScopeUnauthorized
			}
		} else {
			return nil, ErrUnauthorizedSellerAction
		}
	}

	logs, err := s.repo.Queries().ListMarketplaceInventoryLogsByBusiness(ctx, businessID)
	if err != nil {
		return nil, err
	}
	if logs == nil {
		logs = []db.MarketplaceInventoryLog{}
	}
	return logs, nil
}
