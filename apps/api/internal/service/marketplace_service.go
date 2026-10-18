package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"ratels-api/internal/db"
	"ratels-api/internal/payment"
	"ratels-api/internal/repository"
	"ratels-api/internal/security"
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
)

type MarketplaceService struct {
	repo           *repository.Repository
	audit          *AuditService
	paymentGateway payment.PaymentGateway
}

func NewMarketplaceService(repo *repository.Repository, audit *AuditService, gateway payment.PaymentGateway) *MarketplaceService {
	return &MarketplaceService{
		repo:           repo,
		audit:          audit,
		paymentGateway: gateway,
	}
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

	// Idempotency: if already paid, return early to prevent duplicate processing
	if payRec.Status == "successful" && order.PaymentStatus == "paid" {
		return &order, nil
	}

	// Verify payment with authoritative provider
	verifyOut, err := s.paymentGateway.VerifyPayment(ctx, reference)
	if err != nil {
		return nil, fmt.Errorf("payment verification failed: %w", err)
	}

	if verifyOut.Status != "successful" {
		// Out-of-order event safeguard: Never downgrade an already-paid order to failed
		if order.PaymentStatus != "paid" {
			_, _ = s.repo.Queries().UpdateMarketplacePaymentTransactionStatus(ctx, db.UpdateMarketplacePaymentTransactionStatusParams{
				Reference:         reference,
				Status:            "failed",
				ProviderReference: TextFromString(verifyOut.ProviderReference),
				Channel:           TextFromString(verifyOut.Channel),
				PaidAt:            pgtype.Timestamptz{Valid: false},
				RawWebhookPayload: verifyOut.RawResponse,
				ErrorMessage:      TextFromString("Payment status returned: " + verifyOut.Status),
			})
			_, _ = s.repo.Queries().UpdateMarketplaceOrderPaymentStatus(ctx, db.UpdateMarketplaceOrderPaymentStatusParams{
				ID:            order.ID,
				PaymentStatus: "failed",
			})
		}
		return nil, fmt.Errorf("payment was not successful (status: %s)", verifyOut.Status)
	}

	// Server-side amount validation: Never mark paid without verifying full amount!
	if verifyOut.AmountKobo < order.TotalKobo {
		return nil, fmt.Errorf("%w: expected %d kobo, got %d kobo", payment.ErrAmountMismatch, order.TotalKobo, verifyOut.AmountKobo)
	}

	// Server-side currency validation: Provider currency must match order currency
	if verifyOut.Currency != "" && strings.ToUpper(verifyOut.Currency) != strings.ToUpper(order.Currency) {
		return nil, fmt.Errorf("%w: expected %s, got %s", payment.ErrCurrencyMismatch, order.Currency, verifyOut.Currency)
	}

	now := time.Now()
	paidAt := TimestamptzFromTime(now)
	if verifyOut.PaidAt != nil {
		paidAt = TimestamptzFromTime(*verifyOut.PaidAt)
	}

	// Update payment record
	_, err = s.repo.Queries().UpdateMarketplacePaymentTransactionStatus(ctx, db.UpdateMarketplacePaymentTransactionStatusParams{
		Reference:         reference,
		Status:            "successful",
		ProviderReference: TextFromString(verifyOut.ProviderReference),
		Channel:           TextFromString(verifyOut.Channel),
		PaidAt:            paidAt,
		RawWebhookPayload: verifyOut.RawResponse,
		ErrorMessage:      TextFromString(""),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to update payment record: %w", err)
	}

	// Update order payment and processing status
	_, err = s.repo.Queries().UpdateMarketplaceOrderPaymentStatus(ctx, db.UpdateMarketplaceOrderPaymentStatusParams{
		ID:            order.ID,
		PaymentStatus: "paid",
	})
	if err != nil {
		return nil, err
	}

	_, err = s.repo.Queries().UpdateMarketplaceOrderStatus(ctx, db.UpdateMarketplaceOrderStatusParams{
		ID:          order.ID,
		OrderStatus: "processing",
	})
	if err != nil {
		return nil, err
	}

	_ = s.audit.Log(ctx, nil, nil, "confirm_marketplace_payment", "marketplace_payment", reference, nil, nil, nil, nil)
	refreshedOrder, _ := s.repo.Queries().GetMarketplaceOrderByID(ctx, order.ID)
	return &refreshedOrder, nil
}

func (s *MarketplaceService) HandleWebhook(ctx context.Context, signatureHeader string, payload []byte) error {
	if !s.paymentGateway.VerifyWebhookSignature(signatureHeader, payload) {
		return payment.ErrInvalidSignature
	}

	var event struct {
		Event string `json:"event"`
		Data  struct {
			Reference string `json:"reference"`
			Status    string `json:"status"`
			Amount    int64  `json:"amount"`
			Currency  string `json:"currency"`
		} `json:"data"`
	}

	if err := json.Unmarshal(payload, &event); err != nil {
		return fmt.Errorf("failed to decode webhook event: %w", err)
	}

	ref := event.Data.Reference
	if ref == "" {
		return errors.New("webhook event missing reference")
	}

	// Confirm payment idempotently on charge.success or successful status
	if event.Event == "charge.success" || event.Data.Status == "success" || event.Data.Status == "successful" {
		_, err := s.ConfirmPayment(ctx, ref)
		return err
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

	// Ownership check if authenticated user is buyer or seller
	if !isAdmin(user) && !isBuyer {
		// Seller check: does user own the business of any item in this order?
		rawItems, _ := s.repo.Queries().ListMarketplaceOrderItemsByOrder(ctx, id)
		isSeller := false
		for _, item := range rawItems {
			b, _ := s.repo.Queries().GetMarketplaceBusinessByID(ctx, item.BusinessID)
			if b.OwnerUserID == user.ID {
				isSeller = true
				break
			}
		}
		if !isSeller {
			return nil, nil, nil, ErrUnauthorizedBuyerAction
		}
	}

	items, err := s.repo.Queries().ListMarketplaceOrderItemsByOrder(ctx, id)
	if err != nil {
		return nil, nil, nil, err
	}
	if items == nil {
		items = []db.ListMarketplaceOrderItemsByOrderRow{}
	}

	// Multi-tenant isolation: A seller must only see items belonging to their own business
	if !isAdmin(user) && !isBuyer {
		var filtered []db.ListMarketplaceOrderItemsByOrderRow
		for _, item := range items {
			b, err := s.repo.Queries().GetMarketplaceBusinessByID(ctx, item.BusinessID)
			if err == nil && b.OwnerUserID == user.ID {
				filtered = append(filtered, item)
			}
		}
		items = filtered
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
	b, err := s.repo.Queries().GetMarketplaceBusinessByID(ctx, businessID)
	if err != nil {
		return nil, ErrBusinessNotFound
	}
	if b.OwnerUserID != user.ID && !isAdmin(user) {
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
	return list, nil
}

func (s *MarketplaceService) ListAllOrders(ctx context.Context, admin *db.User, orderStatus, paymentStatus, fulfilmentStatus *string) ([]db.ListAllMarketplaceOrdersRow, error) {
	if !isAdmin(admin) {
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
	return list, nil
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

	order, err := s.repo.Queries().GetMarketplaceOrderByID(ctx, orderID)
	if err != nil {
		return nil, ErrOrderNotFound
	}

	if order.PaymentStatus != "paid" {
		return nil, errors.New("cannot dispatch an unpaid order")
	}

	if order.OrderStatus == "cancelled" || order.OrderStatus == "refunded" {
		return nil, fmt.Errorf("cannot dispatch an order with status '%s'", order.OrderStatus)
	}
	if order.OrderStatus == "delivered" || order.FulfilmentStatus == "delivered" {
		return nil, errors.New("order has already been delivered")
	}
	if order.FulfilmentStatus == "dispatched" {
		return nil, errors.New("order has already been dispatched")
	}

	// Verify user is seller of an item in this order or admin
	items, err := s.repo.Queries().ListMarketplaceOrderItemsByOrder(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if !isAdmin(user) {
		isSeller := false
		for _, itm := range items {
			b, _ := s.repo.Queries().GetMarketplaceBusinessByID(ctx, itm.BusinessID)
			if b.OwnerUserID == user.ID {
				isSeller = true
				break
			}
		}
		if !isSeller {
			return nil, ErrUnauthorizedSellerAction
		}
	}

	shipmentID := security.GenerateUUID()
	sh, err := s.repo.Queries().CreateMarketplaceShipment(ctx, db.CreateMarketplaceShipmentParams{
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
		return nil, fmt.Errorf("failed to create shipment record: %w", err)
	}

	// Update order statuses
	_, _ = s.repo.Queries().UpdateMarketplaceFulfilmentStatus(ctx, db.UpdateMarketplaceFulfilmentStatusParams{
		ID:               orderID,
		FulfilmentStatus: "dispatched",
	})
	_, _ = s.repo.Queries().UpdateMarketplaceOrderStatus(ctx, db.UpdateMarketplaceOrderStatusParams{
		ID:          orderID,
		OrderStatus: "dispatched",
	})

	_ = s.audit.Log(ctx, &user.ID, &user.PrimaryRole, "dispatch_marketplace_order", "marketplace_shipment", sh.ID, nil, nil, nil, nil)
	return &sh, nil
}

func (s *MarketplaceService) ConfirmDelivery(ctx context.Context, user *db.User, orderID string, proofURL, notes string) (*db.MarketplaceShipment, error) {
	if user == nil {
		return nil, errors.New("authentication required")
	}

	order, err := s.repo.Queries().GetMarketplaceOrderByID(ctx, orderID)
	if err != nil {
		return nil, ErrOrderNotFound
	}

	// Verify caller is buyer, seller of an item, or admin
	isBuyer := order.BuyerUserID.Valid && order.BuyerUserID.String == user.ID
	if !isBuyer && !isAdmin(user) {
		items, _ := s.repo.Queries().ListMarketplaceOrderItemsByOrder(ctx, orderID)
		isSeller := false
		for _, itm := range items {
			b, _ := s.repo.Queries().GetMarketplaceBusinessByID(ctx, itm.BusinessID)
			if b.OwnerUserID == user.ID {
				isSeller = true
				break
			}
		}
		if !isSeller {
			return nil, ErrUnauthorizedBuyerAction
		}
	}

	if order.OrderStatus == "cancelled" || order.OrderStatus == "refunded" {
		return nil, fmt.Errorf("cannot confirm delivery for an order with status '%s'", order.OrderStatus)
	}

	sh, err := s.repo.Queries().GetMarketplaceShipmentByOrder(ctx, orderID)
	if err != nil {
		return nil, errors.New("shipment record not found for this order")
	}

	now := time.Now()
	updatedSh, err := s.repo.Queries().UpdateMarketplaceShipmentStatus(ctx, db.UpdateMarketplaceShipmentStatusParams{
		ID:                 sh.ID,
		Status:             "delivered",
		DeliveredAt:        TimestamptzFromTime(now),
		ProofOfDeliveryUrl: TextFromString(proofURL),
		DeliveryNotes:      TextFromString(notes),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to update delivery: %w", err)
	}

	_, _ = s.repo.Queries().UpdateMarketplaceFulfilmentStatus(ctx, db.UpdateMarketplaceFulfilmentStatusParams{
		ID:               orderID,
		FulfilmentStatus: "delivered",
	})
	_, _ = s.repo.Queries().UpdateMarketplaceOrderStatus(ctx, db.UpdateMarketplaceOrderStatusParams{
		ID:          orderID,
		OrderStatus: "delivered",
	})

	_ = s.audit.Log(ctx, &user.ID, &user.PrimaryRole, "confirm_marketplace_delivery", "marketplace_shipment", sh.ID, nil, nil, nil, nil)
	return &updatedSh, nil
}

func (s *MarketplaceService) CancelOrder(ctx context.Context, user *db.User, orderID string, reason string) (*db.GetMarketplaceOrderByIDRow, error) {
	if user == nil {
		return nil, errors.New("authentication required")
	}

	order, err := s.repo.Queries().GetMarketplaceOrderByID(ctx, orderID)
	if err != nil {
		return nil, ErrOrderNotFound
	}

	isBuyer := order.BuyerUserID.Valid && order.BuyerUserID.String == user.ID
	if !isBuyer && !isAdmin(user) {
		return nil, ErrUnauthorizedBuyerAction
	}

	if order.OrderStatus == "cancelled" {
		return nil, errors.New("order is already cancelled")
	}
	if order.OrderStatus == "delivered" || order.FulfilmentStatus == "delivered" {
		return nil, errors.New("cannot cancel an order that has already been delivered")
	}
	if order.FulfilmentStatus == "dispatched" {
		return nil, errors.New("cannot cancel an order that has already been dispatched; please request a return or dispute")
	}

	// Atomic cancellation and stock restoration
	err = s.repo.WithTx(ctx, func(q *db.Queries) error {
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

		if order.PaymentStatus == "paid" {
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
		return nil, fmt.Errorf("failed to cancel order: %w", err)
	}

	_ = s.audit.Log(ctx, &user.ID, &user.PrimaryRole, "cancel_marketplace_order", "marketplace_order", orderID, nil, nil, &reason, nil)

	updated, err := s.repo.Queries().GetMarketplaceOrderByID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	return &updated, nil
}

type RefundOrderInput struct {
	AmountKobo int64  `json:"amount_kobo"`
	Reason     string `json:"reason"`
}

func (s *MarketplaceService) InitiateRefund(ctx context.Context, admin *db.User, orderID string, input RefundOrderInput) (*db.GetMarketplaceOrderByIDRow, error) {
	if !isAdmin(admin) {
		return nil, errors.New("unauthorized: administrative permissions required")
	}

	order, err := s.repo.Queries().GetMarketplaceOrderByID(ctx, orderID)
	if err != nil {
		return nil, ErrOrderNotFound
	}

	if order.PaymentStatus != "paid" {
		return nil, fmt.Errorf("cannot refund an order with payment status '%s'", order.PaymentStatus)
	}

	refundAmount := input.AmountKobo
	if refundAmount <= 0 {
		refundAmount = order.TotalKobo
	}
	if refundAmount > order.TotalKobo {
		return nil, fmt.Errorf("refund amount %d kobo exceeds total order amount %d kobo", refundAmount, order.TotalKobo)
	}

	isFullRefund := refundAmount == order.TotalKobo

	err = s.repo.WithTx(ctx, func(q *db.Queries) error {
		if isFullRefund {
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
				OrderStatus: "refunded",
			})
			if err != nil {
				return err
			}
			_, err = q.UpdateMarketplaceOrderPaymentStatus(ctx, db.UpdateMarketplaceOrderPaymentStatusParams{
				ID:            orderID,
				PaymentStatus: "refunded",
			})
			if err != nil {
				return err
			}
		} else {
			_, err = q.UpdateMarketplaceOrderPaymentStatus(ctx, db.UpdateMarketplaceOrderPaymentStatusParams{
				ID:            orderID,
				PaymentStatus: "partially_refunded",
			})
			if err != nil {
				return err
			}
		}

		pay, err := q.GetMarketplacePaymentByOrder(ctx, orderID)
		if err == nil {
			payStatus := "refunded"
			if !isFullRefund {
				payStatus = "partially_refunded"
			}
			_, _ = q.UpdateMarketplacePaymentTransactionStatus(ctx, db.UpdateMarketplacePaymentTransactionStatusParams{
				Reference:         pay.Reference,
				Status:            payStatus,
				ProviderReference: pay.ProviderReference,
				Channel:           pay.Channel,
				PaidAt:            pay.PaidAt,
				RawWebhookPayload: pay.RawWebhookPayload,
				ErrorMessage:      TextFromString(fmt.Sprintf("Refund of %d kobo processed by admin (%s): %s", refundAmount, admin.Email.String, input.Reason)),
			})
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to process refund: %w", err)
	}

	_ = s.audit.Log(ctx, &admin.ID, &admin.PrimaryRole, "refund_marketplace_order", "marketplace_order", orderID, nil, nil, &input.Reason, nil)

	updated, err := s.repo.Queries().GetMarketplaceOrderByID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	return &updated, nil
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
