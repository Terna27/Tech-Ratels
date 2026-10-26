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
	ErrCrossSellerDispatch      = errors.New("sellers can dispatch only items belonging to their businesses")
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
			productID   string
			businessID  string
			title       string
			sku         string
			priceKobo   int64
			quantity    int32
			subtotal    int64
			imageURL    string
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
				productID:  p.ID,
				businessID: p.BusinessID,
				title:      p.Title,
				sku:        p.Sku,
				priceKobo:  p.PriceKobo,
				quantity:   itm.Quantity,
				subtotal:   itemSub,
				imageURL:   img,
			})
		}

		totalKobo := subtotalKobo + deliveryFeeKobo

		var buyerUserID pgtype.Text
		if buyerUser != nil {
			buyerUserID = TextFromString(buyerUser.ID)
		}

		// Create Order
		ord, err := q.CreateMarketplaceOrder(ctx, db.CreateMarketplaceOrderParams{
			ID:              orderID,
			OrderNumber:     orderNumber,
			BuyerUserID:     buyerUserID,
			BuyerName:       input.BuyerName,
			BuyerEmail:      input.BuyerEmail,
			BuyerPhone:      input.BuyerPhone,
			DeliveryType:    deliveryType,
			ShippingAddress: input.ShippingAddress,
			ShippingStateID: Text(input.ShippingStateID),
			ShippingLgaID:   Text(input.ShippingLGAID),
			PickupBranchID:  Text(input.PickupBranchID),
			SubtotalKobo:    subtotalKobo,
			DeliveryFeeKobo: deliveryFeeKobo,
			TotalKobo:       totalKobo,
			Currency:        "NGN",
			OrderStatus:     "placed",
			PaymentStatus:   "pending",
			FulfilmentStatus: "unfulfilled",
			IdempotencyKey:  TextFromString(input.IdempotencyKey),
			Notes:           TextFromString(input.Notes),
		})
		if err != nil {
			return fmt.Errorf("failed to insert order: %w", err)
		}
		createdOrder = ord

		// Create Order Items
		for _, pi := range preparedItems {
			_, err := q.CreateMarketplaceOrderItem(ctx, db.CreateMarketplaceOrderItemParams{
				ID:                security.GenerateUUID(),
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
			for _, itm := range input.Items {
				if itm.Quantity > 0 {
					_, _ = q.IncrementProductStock(ctx, db.IncrementProductStockParams{
						ID:            itm.ProductID,
						StockQuantity: itm.Quantity,
					})
				}
			}
			_, _ = q.UpdateMarketplaceOrderStatus(ctx, db.UpdateMarketplaceOrderStatusParams{
				ID:          createdOrder.ID,
				OrderStatus: "cancelled",
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
		// Out-of-order event safeguard: Row lock and never downgrade an already-paid or refunded order
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

		now := time.Now()
		paidAt := TimestamptzFromTime(now)
		if verifyOut.PaidAt != nil {
			paidAt = TimestamptzFromTime(*verifyOut.PaidAt)
		}
		rawPayload := verifyOut.RawResponse
		if len(rawPayload) == 0 {
			rawPayload = []byte("{}")
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

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to atomically confirm payment: %w", err)
	}

	_ = s.audit.Log(ctx, nil, nil, "confirm_marketplace_payment", "marketplace_payment", reference, nil, nil, nil, nil)
	refreshedOrder, err := s.repo.Queries().GetMarketplaceOrderByID(ctx, payRec.OrderID)
	if err != nil {
		return nil, fmt.Errorf("failed to reload order: %w", err)
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
	orderStateID, orderZoneID := s.resolveOrderGeographyRow(ctx, s.repo.Queries(), o.ShippingStateID, o.PickupBranchID)
	isNationwide := s.isNationwideAdmin(user, apps)
	hasJurisdiction := s.hasCoordinatorJurisdiction(user, apps, orderStateID, orderZoneID)
	hasCoordRole := user.PrimaryRole == "state_coordinator" || user.PrimaryRole == "zonal_coordinator" || s.hasAnyCoordinatorAppointment(apps)

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
	sh, err := s.repo.Queries().GetMarketplaceShipmentByOrder(ctx, id)
	if err == nil && sh.ID != "" {
		shipmentPtr = &sh
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
	hasCoord := admin.PrimaryRole == "state_coordinator" || admin.PrimaryRole == "zonal_coordinator" || s.hasAnyCoordinatorAppointment(apps)

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

	// Filter by coordinator jurisdiction
	var filtered []db.ListAllMarketplaceOrdersRow
	for _, row := range list {
		stateID, zoneID := s.resolveOrderGeographyRow(ctx, s.repo.Queries(), row.ShippingStateID, row.PickupBranchID)
		if s.hasCoordinatorJurisdiction(admin, apps, stateID, zoneID) {
			filtered = append(filtered, row)
		}
	}
	if filtered == nil {
		filtered = []db.ListAllMarketplaceOrdersRow{}
	}
	return filtered, nil
}

// -------------------------------------------------------------
// Fulfilment & Delivery
// -------------------------------------------------------------

type DispatchOrderInput struct {
	CarrierName            string     `json:"carrier_name"`
	TrackingNumber         string     `json:"tracking_number"`
	TrackingURL            string     `json:"tracking_url"`
	EstimatedDeliveryDate  *time.Time `json:"estimated_delivery_date"`
	DeliveryNotes          string     `json:"delivery_notes"`
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

		// 1. Payment status guard: require confirmed payment before dispatch
		if lockedOrder.PaymentStatus != "paid" {
			return ErrOrderNotPaid
		}

		// 2. Order status guards
		if lockedOrder.OrderStatus == "cancelled" {
			return fmt.Errorf("cannot dispatch an order with status '%s'", lockedOrder.OrderStatus)
		}
		if lockedOrder.OrderStatus == "refunded" {
			return fmt.Errorf("cannot dispatch an order with status '%s'", lockedOrder.OrderStatus)
		}
		if lockedOrder.OrderStatus == "delivered" || lockedOrder.FulfilmentStatus == "delivered" {
			return ErrOrderAlreadyDelivered
		}
		if lockedOrder.FulfilmentStatus == "dispatched" || lockedOrder.OrderStatus == "dispatched" {
			return ErrOrderAlreadyDispatched
		}

		// 3. Fulfilment state transition guard
		if lockedOrder.FulfilmentStatus != "unfulfilled" && lockedOrder.FulfilmentStatus != "preparing" {
			return fmt.Errorf("invalid fulfilment state transition from '%s' to 'dispatched'", lockedOrder.FulfilmentStatus)
		}

		// 4. Duplicate shipment check
		existingSh, err := q.GetMarketplaceShipmentByOrder(ctx, orderID)
		if err == nil && existingSh.ID != "" {
			return ErrOrderAlreadyDispatched
		}

		// 5. Authorization:
		items, err := q.ListMarketplaceOrderItemsByOrder(ctx, orderID)
		if err != nil {
			return err
		}
		if len(items) == 0 {
			return errors.New("cannot dispatch order with no items")
		}

		apps := s.getActiveAppointments(ctx, user.ID)
		orderStateID, orderZoneID, _ := s.resolveOrderGeography(ctx, q, &lockedOrder)
		isNationwide := s.isNationwideAdmin(user, apps)
		hasJurisdiction := s.hasCoordinatorJurisdiction(user, apps, orderStateID, orderZoneID)
		hasCoordRole := user.PrimaryRole == "state_coordinator" || user.PrimaryRole == "zonal_coordinator" || s.hasAnyCoordinatorAppointment(apps)

		if hasCoordRole && !hasJurisdiction {
			return ErrScopeUnauthorized
		}

		if !isNationwide && !hasJurisdiction {
			// Seller authorization:
			// Confirm that sellers can dispatch only items belonging to their businesses.
			// Do not allow a seller to dispatch another seller's products.
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

		shipmentID := security.GenerateUUID()
		sh, err := q.CreateMarketplaceShipment(ctx, db.CreateMarketplaceShipmentParams{
			ID:                  shipmentID,
			OrderID:             orderID,
			CarrierName:         input.CarrierName,
			TrackingNumber:      input.TrackingNumber,
			TrackingUrl:         TextFromString(input.TrackingURL),
			Status:              "dispatched",
			DispatchedAt:        TimestamptzFromTime(time.Now()),
			EstimatedDeliveryAt: Timestamptz(input.EstimatedDeliveryDate),
			DeliveredAt:         pgtype.Timestamptz{Valid: false},
			DeliveryNotes:       TextFromString(input.DeliveryNotes),
			ProofOfDeliveryUrl:  TextFromString(""),
		})
		if err != nil {
			return fmt.Errorf("failed to create shipment record: %w", err)
		}

		// Update order statuses
		_, err = q.UpdateMarketplaceFulfilmentStatus(ctx, db.UpdateMarketplaceFulfilmentStatusParams{
			ID:               orderID,
			FulfilmentStatus: "dispatched",
		})
		if err != nil {
			return fmt.Errorf("failed to update fulfilment status: %w", err)
		}

		_, err = q.UpdateMarketplaceOrderStatus(ctx, db.UpdateMarketplaceOrderStatusParams{
			ID:          orderID,
			OrderStatus: "dispatched",
		})
		if err != nil {
			return fmt.Errorf("failed to update order status: %w", err)
		}

		createdShipment = sh
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
		orderStateID, orderZoneID, _ := s.resolveOrderGeography(ctx, q, &lockedOrder)
		isNationwide := s.isNationwideAdmin(user, apps)
		hasJurisdiction := s.hasCoordinatorJurisdiction(user, apps, orderStateID, orderZoneID)
		hasCoordRole := user.PrimaryRole == "state_coordinator" || user.PrimaryRole == "zonal_coordinator" || s.hasAnyCoordinatorAppointment(apps)

		if hasCoordRole && !hasJurisdiction && !isBuyer {
			return ErrScopeUnauthorized
		}

		if !isBuyer && !isNationwide && !hasJurisdiction {
			// Seller check: must own all items in this order
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
		if lockedOrder.FulfilmentStatus != "dispatched" {
			return errors.New("only dispatched orders can be confirmed as delivered")
		}

		sh, err := q.GetMarketplaceShipmentByOrder(ctx, orderID)
		if err != nil {
			return errors.New("shipment record not found for dispatched order")
		}

		now := time.Now()
		upSh, err := q.UpdateMarketplaceShipmentStatus(ctx, db.UpdateMarketplaceShipmentStatusParams{
			ID:                 sh.ID,
			Status:             "delivered",
			DeliveredAt:        TimestamptzFromTime(now),
			ProofOfDeliveryUrl: TextFromString(proofURL),
			DeliveryNotes:      TextFromString(notes),
		})
		if err != nil {
			return fmt.Errorf("failed to update delivery: %w", err)
		}

		_, err = q.UpdateMarketplaceFulfilmentStatus(ctx, db.UpdateMarketplaceFulfilmentStatusParams{
			ID:               orderID,
			FulfilmentStatus: "delivered",
		})
		if err != nil {
			return fmt.Errorf("failed to update fulfilment status: %w", err)
		}

		_, err = q.UpdateMarketplaceOrderStatus(ctx, db.UpdateMarketplaceOrderStatusParams{
			ID:          orderID,
			OrderStatus: "delivered",
		})
		if err != nil {
			return fmt.Errorf("failed to update order status: %w", err)
		}

		updatedSh = upSh
		return nil
	})

	if err != nil {
		return nil, err
	}

	_ = s.audit.Log(ctx, &user.ID, &user.PrimaryRole, "confirm_marketplace_delivery", "marketplace_shipment", updatedSh.ID, nil, nil, nil, nil)
	return &updatedSh, nil
}

func (s *MarketplaceService) CancelOrder(ctx context.Context, user *db.User, orderID string, reason string) (*db.GetMarketplaceOrderByIDRow, error) {
	if user == nil {
		return nil, errors.New("authentication required")
	}

	err := s.repo.WithTx(ctx, func(q *db.Queries) error {
		lockedOrder, err := q.GetMarketplaceOrderForUpdate(ctx, orderID)
		if err != nil {
			return ErrOrderNotFound
		}

		isBuyer := lockedOrder.BuyerUserID.Valid && lockedOrder.BuyerUserID.String == user.ID
		apps := s.getActiveAppointments(ctx, user.ID)
		orderStateID, orderZoneID, _ := s.resolveOrderGeography(ctx, q, &lockedOrder)
		isNationwide := s.isNationwideAdmin(user, apps)
		hasJurisdiction := s.hasCoordinatorJurisdiction(user, apps, orderStateID, orderZoneID)
		hasCoordRole := user.PrimaryRole == "state_coordinator" || user.PrimaryRole == "zonal_coordinator" || s.hasAnyCoordinatorAppointment(apps)

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
		if lockedOrder.FulfilmentStatus == "dispatched" || lockedOrder.OrderStatus == "dispatched" {
			return errors.New("cannot cancel an order that has already been dispatched; please request a return or dispute")
		}
		if lockedOrder.OrderStatus == "refunded" {
			return errors.New("cannot cancel a refunded order")
		}

		// Atomic cancellation and stock restoration
		items, err := q.ListMarketplaceOrderItemsByOrder(ctx, orderID)
		if err != nil {
			return err
		}
		for _, itm := range items {
			if itm.ProductID.Valid {
				_, err = q.IncrementProductStock(ctx, db.IncrementProductStockParams{
					ID:            itm.ProductID.String,
					StockQuantity: itm.Quantity,
				})
				if err != nil {
					return err
				}
			}
		}

		_, err = q.UpdateMarketplaceOrderStatus(ctx, db.UpdateMarketplaceOrderStatusParams{
			ID:          orderID,
			OrderStatus: "cancelled",
		})
		if err != nil {
			return err
		}

		if lockedOrder.PaymentStatus == "paid" {
			_, _ = q.UpdateMarketplaceOrderPaymentStatus(ctx, db.UpdateMarketplaceOrderPaymentStatusParams{
				ID:            orderID,
				PaymentStatus: "refunded",
			})
			pay, err := q.GetMarketplacePaymentByOrder(ctx, orderID)
			if err == nil {
				_, _ = q.UpdateMarketplacePaymentTransactionStatus(ctx, db.UpdateMarketplacePaymentTransactionStatusParams{
					Reference:         pay.Reference,
					Status:            "refunded",
					ProviderReference: pay.ProviderReference,
					Channel:           pay.Channel,
					PaidAt:            pay.PaidAt,
					RawWebhookPayload: pay.RawWebhookPayload,
					ErrorMessage:      TextFromString("Cancelled by buyer/admin: " + reason),
				})
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
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
	hasCoordRole := admin.PrimaryRole == "state_coordinator" || admin.PrimaryRole == "zonal_coordinator" || s.hasAnyCoordinatorAppointment(apps)

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

	orderStateID, orderZoneID := s.resolveOrderGeographyRow(ctx, s.repo.Queries(), order.ShippingStateID, order.PickupBranchID)
	if !isNationwide && !s.hasCoordinatorJurisdiction(admin, apps, orderStateID, orderZoneID) {
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

		stID, znID, _ := s.resolveOrderGeography(ctx, q, &lockedOrder)
		if !isNationwide && !s.hasCoordinatorJurisdiction(admin, apps, stID, znID) {
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
		hasCoordRole := admin.PrimaryRole == "state_coordinator" || admin.PrimaryRole == "zonal_coordinator" || s.hasAnyCoordinatorAppointment(apps)
		if !isNationwide && !hasCoordRole {
			return nil, nil, errors.New("unauthorized: administrative permissions required")
		}
		order, err := s.repo.Queries().GetMarketplaceOrderByID(ctx, lockedRefund.OrderID)
		if err == nil {
			stID, znID := s.resolveOrderGeographyRow(ctx, s.repo.Queries(), order.ShippingStateID, order.PickupBranchID)
			if !isNationwide && !s.hasCoordinatorJurisdiction(admin, apps, stID, znID) {
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
				stID, znID, _ := s.resolveOrderGeography(ctx, q, &lockedOrder)
				if !s.hasCoordinatorJurisdiction(admin, apps, stID, znID) {
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
		// 1. Full refund
		// 2. Order was NOT already cancelled or refunded (which already restored stock)
		// 3. Order has NOT been dispatched or delivered
		shouldRestoreStock := isFullRefund &&
			lockedOrder.OrderStatus != "cancelled" &&
			lockedOrder.OrderStatus != "refunded" &&
			lockedOrder.FulfilmentStatus != "dispatched" &&
			lockedOrder.FulfilmentStatus != "delivered"

		if shouldRestoreStock {
			items, err := q.ListMarketplaceOrderItemsByOrder(ctx, lockedOrder.ID)
			if err != nil {
				return err
			}
			for _, itm := range items {
				if itm.ProductID.Valid {
					_, err = q.IncrementProductStock(ctx, db.IncrementProductStockParams{
						ID:            itm.ProductID.String,
						StockQuantity: itm.Quantity,
					})
					if err != nil {
						return err
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
			strings.ToLower(a.JurisdictionType) == "state" || strings.ToLower(a.JurisdictionType) == "zone" || strings.ToLower(a.JurisdictionType) == "zonal" {
			return true
		}
	}
	return false
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
