package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"ratels-api/internal/db"
	"ratels-api/internal/repository"
	"ratels-api/internal/security"
)

var (
	ErrItemNotFound                 = errors.New("inventory item not found")
	ErrLocationNotFound             = errors.New("storage location not found")
	ErrRequestNotFound              = errors.New("material request not found")
	ErrTransferNotFound             = errors.New("material transfer not found")
	ErrDiscrepancyNotFound          = errors.New("transfer discrepancy not found")
	ErrInsufficientStock            = errors.New("INSUFFICIENT_STOCK: Available quantity is less than requested/dispatched quantity")
	ErrAntiSelfApproval             = errors.New("ANTI_SELF_APPROVAL: Requester cannot approve, reduce, or reject their own material request")
	ErrJurisdictionMismatch         = errors.New("JURISDICTION_MISMATCH: Official lacks authority to approve requests outside their appointed jurisdiction")
	ErrInvalidRequestStatus         = errors.New("invalid material request status for this action")
	ErrInvalidTransferStatus        = errors.New("invalid material transfer status for this action")
	ErrDuplicateTransfer            = errors.New("transfer with this idempotency key or reference already exists")
	ErrTransferAlreadyReceived      = errors.New("transfer has already been received")
	ErrSameLocationTransfer         = errors.New("source and destination storage locations cannot be the same")
	ErrInvalidDiscrepancyResolution = errors.New("invalid discrepancy resolution status")
	ErrUnauthorizedInventoryAction  = errors.New("unauthorized inventory action")
)

type InventoryService struct {
	repo  *repository.Repository
	audit *AuditService
}

func NewInventoryService(repo *repository.Repository, audit *AuditService) *InventoryService {
	return &InventoryService{repo: repo, audit: audit}
}

// Request / Input DTOs

type CreateItemRequest struct {
	SKU                  string  `json:"sku"`
	Name                 string  `json:"name"`
	Description          *string `json:"description"`
	Category             string  `json:"category"`
	UnitOfMeasure        string  `json:"unit_of_measure"`
	MinThresholdQuantity int64   `json:"min_threshold_quantity"`
	IsBatchTracked       bool    `json:"is_batch_tracked"`
}

type UpdateItemRequest struct {
	Name                 string  `json:"name"`
	Description          *string `json:"description"`
	Category             string  `json:"category"`
	UnitOfMeasure        string  `json:"unit_of_measure"`
	MinThresholdQuantity int64   `json:"min_threshold_quantity"`
	IsBatchTracked       bool    `json:"is_batch_tracked"`
	IsActive             bool    `json:"is_active"`
}

type CreateLocationRequest struct {
	Code            string  `json:"code"`
	Name            string  `json:"name"`
	LocationType    string  `json:"location_type"` // national, zonal, state, lga
	ZoneID          *string `json:"zone_id"`
	StateID         *string `json:"state_id"`
	LGAID           *string `json:"lga_id"`
	Address         *string `json:"address"`
	CustodianUserID *string `json:"custodian_user_id"`
	ContactPhone    *string `json:"contact_phone"`
}

type UpdateLocationRequest struct {
	Name            string  `json:"name"`
	LocationType    string  `json:"location_type"`
	ZoneID          *string `json:"zone_id"`
	StateID         *string `json:"state_id"`
	LGAID           *string `json:"lga_id"`
	Address         *string `json:"address"`
	CustodianUserID *string `json:"custodian_user_id"`
	ContactPhone    *string `json:"contact_phone"`
	IsActive        bool    `json:"is_active"`
}

type ReceiveStockRequest struct {
	LocationID   string     `json:"location_id"`
	ItemID       string     `json:"item_id"`
	Quantity     int64      `json:"quantity"`
	BatchNumber  string     `json:"batch_number"`
	ExpiryDate   *time.Time `json:"expiry_date"`
	MovementType string     `json:"movement_type"` // initial_receipt, donation_in_kind, return
	Notes        *string    `json:"notes"`
}

type MaterialRequestItemInput struct {
	ItemID            string `json:"item_id"`
	RequestedQuantity int64  `json:"requested_quantity"`
	UnitOfMeasure     string `json:"unit_of_measure"`
}

type CreateMaterialRequestInput struct {
	JurisdictionType      string                     `json:"jurisdiction_type"` // national, zonal, state, lga
	JurisdictionID        *string                    `json:"jurisdiction_id"`
	DestinationLocationID string                     `json:"destination_location_id"`
	IntendedUse           string                     `json:"intended_use"`
	BeneficiaryCategory   *string                    `json:"beneficiary_category"`
	RequiredDate          time.Time                  `json:"required_date"`
	Notes                 *string                    `json:"notes"`
	Items                 []MaterialRequestItemInput `json:"items"`
}

type ReviewMaterialRequestInput struct {
	Action                 string           `json:"action"` // approve, reduce, reject
	RejectionReason        *string          `json:"rejection_reason"`
	ReductionReason        *string          `json:"reduction_reason"`
	ItemApprovedQuantities map[string]int64 `json:"item_approved_quantities"` // item_id -> approved_quantity
}

type CreateTransferItemInput struct {
	ItemID             string `json:"item_id"`
	BatchNumber        string `json:"batch_number"`
	DispatchedQuantity int64  `json:"dispatched_quantity"`
	UnitOfMeasure      string `json:"unit_of_measure"`
}

type CreateTransferInput struct {
	Reference             *string                   `json:"reference"`
	IdempotencyKey        *string                   `json:"idempotency_key"`
	RequestID             *string                   `json:"request_id"`
	SourceLocationID      string                    `json:"source_location_id"`
	DestinationLocationID string                    `json:"destination_location_id"`
	CarrierName           *string                   `json:"carrier_name"`
	CarrierPhone          *string                   `json:"carrier_phone"`
	TrackingNumber        *string                   `json:"tracking_number"`
	DispatchNotes         *string                   `json:"dispatch_notes"`
	Items                 []CreateTransferItemInput `json:"items"`
	AutoDispatch          bool                      `json:"auto_dispatch"`
}

type DispatchTransferInput struct {
	CarrierName    *string `json:"carrier_name"`
	CarrierPhone   *string `json:"carrier_phone"`
	TrackingNumber *string `json:"tracking_number"`
	DispatchNotes  *string `json:"dispatch_notes"`
}

type ReceiptItemInput struct {
	ItemID           string `json:"item_id"`
	ReceivedQuantity int64  `json:"received_quantity"`
}

type ConfirmReceiptInput struct {
	IdempotencyKey        *string            `json:"idempotency_key"`
	Items                 []ReceiptItemInput `json:"items"`
	DiscrepancyReason     *string            `json:"discrepancy_reason"` // missing_in_transit, damaged, expired, wrong_item
	SupportingEvidenceURL *string            `json:"supporting_evidence_url"`
}

type ResolveDiscrepancyInput struct {
	Resolution string `json:"resolution"` // write_off, returned
	Notes      string `json:"notes"`
}

type InventorySummary struct {
	TotalItems          int64 `json:"total_items"`
	TotalLocations      int64 `json:"total_locations"`
	InTransitTransfers  int64 `json:"in_transit_transfers"`
	OpenDiscrepancies   int64 `json:"open_discrepancies"`
	LowStockAlertsCount int64 `json:"low_stock_alerts_count"`
}

// -------------------------------------------------------------
// 1. Inventory Items Catalogue
// -------------------------------------------------------------

func (s *InventoryService) CreateItem(ctx context.Context, actor *db.User, req CreateItemRequest) (*db.InventoryItem, error) {

	if err := s.authorizeCatalogue(ctx, actor); err != nil {
		return nil, err
	}
	uom := req.UnitOfMeasure
	if uom == "" {
		uom = "pcs"
	}
	threshold := req.MinThresholdQuantity
	if threshold <= 0 {
		threshold = 10
	}

	item, err := s.repo.Queries().CreateInventoryItem(ctx, db.CreateInventoryItemParams{
		ID:                   security.GenerateUUID(),
		Sku:                  strings.TrimSpace(req.SKU),
		Name:                 strings.TrimSpace(req.Name),
		Description:          Text(req.Description),
		Category:             strings.TrimSpace(req.Category),
		UnitOfMeasure:        uom,
		MinThresholdQuantity: threshold,
		IsBatchTracked:       req.IsBatchTracked,
		IsActive:             true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create inventory item: %w", err)
	}

	actorID := ""
	if actor != nil {
		actorID = actor.ID
	}
	_ = s.audit.Log(ctx, &actorID, nil, "create_inventory_item", "inventory_item", item.ID, nil, nil, nil, nil)

	return &item, nil
}

func (s *InventoryService) GetItem(ctx context.Context, id string) (*db.InventoryItem, error) {
	item, err := s.repo.Queries().GetInventoryItemByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrItemNotFound
		}
		return nil, err
	}
	return &item, nil
}

func (s *InventoryService) ListItems(ctx context.Context, category *string, isActive *bool) ([]db.InventoryItem, error) {
	catText := ""
	if category != nil {
		catText = *category
	}
	activeVal := pgtype.Bool{Bool: true, Valid: true}
	if isActive != nil {
		activeVal = pgtype.Bool{Bool: *isActive, Valid: true}
	}

	return s.repo.Queries().ListInventoryItems(ctx, db.ListInventoryItemsParams{
		Column1: catText,
		Column2: activeVal.Bool,
	})
}

func (s *InventoryService) UpdateItem(ctx context.Context, actor *db.User, id string, req UpdateItemRequest) (*db.InventoryItem, error) {

	if err := s.authorizeCatalogue(ctx, actor); err != nil {
		return nil, err
	}
	uom := req.UnitOfMeasure
	if uom == "" {
		uom = "pcs"
	}
	threshold := req.MinThresholdQuantity
	if threshold <= 0 {
		threshold = 10
	}

	item, err := s.repo.Queries().UpdateInventoryItem(ctx, db.UpdateInventoryItemParams{
		ID:                   id,
		Name:                 strings.TrimSpace(req.Name),
		Description:          Text(req.Description),
		Category:             strings.TrimSpace(req.Category),
		UnitOfMeasure:        uom,
		MinThresholdQuantity: threshold,
		IsBatchTracked:       req.IsBatchTracked,
		IsActive:             req.IsActive,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrItemNotFound
		}
		return nil, err
	}

	actorID := ""
	if actor != nil {
		actorID = actor.ID
	}
	_ = s.audit.Log(ctx, &actorID, nil, "update_inventory_item", "inventory_item", item.ID, nil, nil, nil, nil)

	return &item, nil
}

// -------------------------------------------------------------
// 2. Storage Locations (Warehouses)
// -------------------------------------------------------------

func (s *InventoryService) CreateLocation(ctx context.Context, actor *db.User, req CreateLocationRequest) (*db.GetStorageLocationByIDRow, error) {

	if err := s.authorizeCatalogue(ctx, actor); err != nil {
		return nil, err
	}
	// Validate supplied geographic parents independently of an actor's privilege.
	targetState := valueOrEmpty(req.StateID)
	if valueOrEmpty(req.LGAID) != "" {
		lga, e := s.repo.Queries().GetLGAByID(ctx, *req.LGAID)
		if e != nil || (targetState != "" && lga.StateID != targetState) {
			return nil, ErrInvalidJurisdiction
		}
		targetState = lga.StateID
	}
	if targetState != "" {
		state, e := s.repo.Queries().GetStateByID(ctx, targetState)
		if e != nil || (valueOrEmpty(req.ZoneID) != "" && *req.ZoneID != state.ZoneID) {
			return nil, ErrInvalidJurisdiction
		}
	} else if valueOrEmpty(req.ZoneID) != "" {
		if _, e := s.repo.Queries().GetZoneByID(ctx, *req.ZoneID); e != nil {
			return nil, ErrInvalidJurisdiction
		}
	}

	scope, err := operationalScopeForActor(ctx, s.repo, actor)
	if err != nil {
		return nil, err
	}
	if !scope.Covers(valueOrEmpty(req.StateID), valueOrEmpty(req.LGAID), valueOrEmpty(req.ZoneID)) {
		return nil, ErrJurisdictionMismatch
	}
	locID := security.GenerateUUID()
	loc, err := s.repo.Queries().CreateStorageLocation(ctx, db.CreateStorageLocationParams{
		ID:              locID,
		Code:            strings.ToUpper(strings.TrimSpace(req.Code)),
		Name:            strings.TrimSpace(req.Name),
		LocationType:    strings.TrimSpace(req.LocationType),
		ZoneID:          Text(req.ZoneID),
		StateID:         Text(req.StateID),
		LgaID:           Text(req.LGAID),
		Address:         Text(req.Address),
		CustodianUserID: Text(req.CustodianUserID),
		ContactPhone:    Text(req.ContactPhone),
		IsActive:        true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create storage location: %w", err)
	}

	actorID := ""
	if actor != nil {
		actorID = actor.ID
	}
	_ = s.audit.Log(ctx, &actorID, nil, "create_storage_location", "storage_location", loc.ID, nil, nil, nil, nil)

	return s.GetLocation(ctx, loc.ID)
}

func (s *InventoryService) GetLocation(ctx context.Context, id string) (*db.GetStorageLocationByIDRow, error) {

	if operationalActor(ctx) != nil {
		if err := s.authorizeLocation(ctx, operationalActor(ctx), id); err != nil {
			return nil, err
		}
	}
	loc, err := s.repo.Queries().GetStorageLocationByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrLocationNotFound
		}
		return nil, err
	}
	return &loc, nil
}

func (s *InventoryService) ListLocations(ctx context.Context, locType, zoneID, stateID, lgaID *string, isActive *bool) ([]db.ListStorageLocationsRow, error) {
	var lt, zid, sid, lid string
	if locType != nil {
		lt = *locType
	}
	if zoneID != nil {
		zid = *zoneID
	}
	if stateID != nil {
		sid = *stateID
	}
	if lgaID != nil {
		lid = *lgaID
	}
	activeVal := pgtype.Bool{Bool: true, Valid: true}
	if isActive != nil {
		activeVal = pgtype.Bool{Bool: *isActive, Valid: true}
	}

	rows, err := s.repo.Queries().ListStorageLocations(ctx, db.ListStorageLocationsParams{
		Column1: lt,
		Column2: zid,
		Column3: sid,
		Column4: lid,
		Column5: activeVal.Bool,
	})
	if err != nil {
		return nil, err
	}
	return filterInventoryRows(ctx, s.repo, rows, func(r db.ListStorageLocationsRow) []string { return []string{r.ID} }, nil)

}

// -------------------------------------------------------------
// 3. Stock Balances & Initial Receipts / Stock Inflow
// -------------------------------------------------------------

func (s *InventoryService) ReceiveStock(ctx context.Context, actor *db.User, req ReceiveStockRequest) (*db.StockBalance, error) {

	if err := s.authorizeLocation(ctx, actor, req.LocationID); err != nil {
		return nil, err
	}
	if req.Quantity <= 0 {
		return nil, errors.New("received quantity must be greater than zero")
	}

	var updatedBal db.StockBalance
	err := s.repo.WithTx(ctx, func(q *db.Queries) error {
		// Verify location and item exist
		if _, err := q.GetStorageLocationByID(ctx, req.LocationID); err != nil {
			return ErrLocationNotFound
		}
		if _, err := q.GetInventoryItemByID(ctx, req.ItemID); err != nil {
			return ErrItemNotFound
		}

		batch := strings.TrimSpace(req.BatchNumber)
		mType := req.MovementType
		if mType == "" {
			mType = "initial_receipt"
		}

		// Check if stock balance row exists with row lock
		bal, err := q.GetStockBalanceForUpdate(ctx, db.GetStockBalanceForUpdateParams{
			LocationID:  req.LocationID,
			ItemID:      req.ItemID,
			BatchNumber: batch,
		})

		var newOnHand int64
		var newReserved int64
		if errors.Is(err, pgx.ErrNoRows) {
			// First time receipt for this location, item, batch
			newOnHand = req.Quantity
			newReserved = 0
			var exp pgtype.Timestamptz
			if req.ExpiryDate != nil {
				exp = pgtype.Timestamptz{Time: *req.ExpiryDate, Valid: true}
			}
			inserted, err := q.UpsertStockBalance(ctx, db.UpsertStockBalanceParams{
				ID:               security.GenerateUUID(),
				LocationID:       req.LocationID,
				ItemID:           req.ItemID,
				OnHandQuantity:   newOnHand,
				ReservedQuantity: newReserved,
				BatchNumber:      batch,
				ExpiryDate:       exp,
			})
			if err != nil {
				return fmt.Errorf("failed to insert initial stock balance: %w", err)
			}
			updatedBal = inserted
		} else if err != nil {
			return err
		} else {
			// Increment on_hand_quantity
			newOnHand = bal.OnHandQuantity + req.Quantity
			newReserved = bal.ReservedQuantity
			updated, err := q.SetStockBalanceQuantities(ctx, db.SetStockBalanceQuantitiesParams{
				LocationID:       req.LocationID,
				ItemID:           req.ItemID,
				BatchNumber:      batch,
				OnHandQuantity:   newOnHand,
				ReservedQuantity: newReserved,
			})
			if err != nil {
				return fmt.Errorf("failed to update stock balance: %w", err)
			}
			updatedBal = updated
		}

		// Append to append-only stock ledger
		ledgerRef := fmt.Sprintf("LED-%s-%s", time.Now().Format("20060102"), strings.ToUpper(security.GenerateUUID()[:8]))
		actorID := ""
		if actor != nil {
			actorID = actor.ID
		}
		_, err = q.CreateStockLedgerEntry(ctx, db.CreateStockLedgerEntryParams{
			ID:                   security.GenerateUUID(),
			Reference:            ledgerRef,
			LocationID:           req.LocationID,
			ItemID:               req.ItemID,
			MovementType:         mType,
			Quantity:             req.Quantity, // positive inflow
			BalanceAfter:         newOnHand,
			BatchNumber:          batch,
			RelatedTransferID:    pgtype.Text{Valid: false},
			RelatedRequestID:     pgtype.Text{Valid: false},
			RelatedDiscrepancyID: pgtype.Text{Valid: false},
			Notes:                Text(req.Notes),
			ActorUserID:          TextFromString(actorID),
			OccurredAt:           pgtype.Timestamptz{Time: time.Now(), Valid: true},
		})
		if err != nil {
			return fmt.Errorf("failed to append to stock ledger: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return &updatedBal, nil
}

func (s *InventoryService) ListStockBalances(ctx context.Context, locationID, itemID, category *string) ([]db.ListAllStockBalancesRow, error) {
	var loc, itm, cat string
	if locationID != nil {
		loc = *locationID
	}
	if itemID != nil {
		itm = *itemID
	}
	if category != nil {
		cat = *category
	}
	rows, err := s.repo.Queries().ListAllStockBalances(ctx, db.ListAllStockBalancesParams{
		Column1: loc,
		Column2: itm,
		Column3: cat,
	})
	if err != nil {
		return nil, err
	}
	return filterInventoryRows(ctx, s.repo, rows, func(r db.ListAllStockBalancesRow) []string { return []string{r.LocationID} }, nil)

}

func (s *InventoryService) ListLowStockAlerts(ctx context.Context) ([]db.ListLowStockAlertsRow, error) {
	rows, err := s.repo.Queries().ListLowStockAlerts(ctx)
	if err != nil {
		return nil, err
	}
	return filterInventoryRows(ctx, s.repo, rows, func(r db.ListLowStockAlertsRow) []string { return []string{r.LocationID} }, nil)
}

// -------------------------------------------------------------
// 4. Material Requests & Multi-Tier Approvals
// -------------------------------------------------------------

func (s *InventoryService) CreateMaterialRequest(ctx context.Context, requester *db.User, req CreateMaterialRequestInput) (*db.GetMaterialRequestByIDRow, error) {
	if requester == nil {
		return nil, errors.New("authenticated requester required")
	}
	if len(req.Items) == 0 {
		return nil, errors.New("material request must contain at least one item")
	}
	if req.IntendedUse == "" {
		return nil, errors.New("intended use is required")
	}

	ref := fmt.Sprintf("REQ-%s-%s", time.Now().Format("20060102"), strings.ToUpper(security.GenerateUUID()[:6]))
	reqID := security.GenerateUUID()

	err := s.repo.WithTx(ctx, func(q *db.Queries) error {
		// Verify destination location exists
		if _, err := q.GetStorageLocationByID(ctx, req.DestinationLocationID); err != nil {
			return ErrLocationNotFound
		}

		jurisdictionType := strings.ToLower(strings.TrimSpace(req.JurisdictionType))
		if jurisdictionType == "" {
			jurisdictionType = "national"
		}

		reqDate := req.RequiredDate
		if reqDate.IsZero() {
			reqDate = time.Now().AddDate(0, 0, 7)
		}

		_, err := q.CreateMaterialRequest(ctx, db.CreateMaterialRequestParams{
			ID:                    reqID,
			Reference:             ref,
			RequestingUserID:      requester.ID,
			JurisdictionType:      jurisdictionType,
			JurisdictionID:        Text(req.JurisdictionID),
			DestinationLocationID: req.DestinationLocationID,
			Status:                "submitted",
			IntendedUse:           strings.TrimSpace(req.IntendedUse),
			BeneficiaryCategory:   Text(req.BeneficiaryCategory),
			RequiredDate:          pgtype.Timestamptz{Time: reqDate, Valid: true},
			Notes:                 Text(req.Notes),
		})
		if err != nil {
			return fmt.Errorf("failed to create material request: %w", err)
		}

		for _, itm := range req.Items {
			if itm.RequestedQuantity <= 0 {
				return errors.New("requested quantity must be greater than zero")
			}
			uom := itm.UnitOfMeasure
			if uom == "" {
				uom = "pcs"
			}
			_, err := q.CreateMaterialRequestItem(ctx, db.CreateMaterialRequestItemParams{
				ID:                security.GenerateUUID(),
				RequestID:         reqID,
				ItemID:            itm.ItemID,
				RequestedQuantity: itm.RequestedQuantity,
				ApprovedQuantity:  0,
				UnitOfMeasure:     uom,
			})
			if err != nil {
				return fmt.Errorf("failed to create request item: %w", err)
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	_ = s.audit.Log(ctx, &requester.ID, nil, "create_material_request", "material_request", reqID, nil, nil, nil, nil)

	return s.GetMaterialRequest(ctx, reqID)
}

func (s *InventoryService) GetMaterialRequest(ctx context.Context, id string) (*db.GetMaterialRequestByIDRow, error) {

	if operationalActor(ctx) != nil {
		if err := s.authorizeRequestRead(ctx, id); err != nil {
			return nil, err
		}
	}
	req, err := s.repo.Queries().GetMaterialRequestByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRequestNotFound
		}
		return nil, err
	}
	return &req, nil
}

func (s *InventoryService) ListMaterialRequests(ctx context.Context, status, destinationLocationID, requestingUserID, jurisdictionType *string) ([]db.ListMaterialRequestsRow, error) {
	var st, dest, usr, jur string
	if status != nil {
		st = *status
	}
	if destinationLocationID != nil {
		dest = *destinationLocationID
	}
	if requestingUserID != nil {
		usr = *requestingUserID
	}
	if jurisdictionType != nil {
		jur = *jurisdictionType
	}

	rows, err := s.repo.Queries().ListMaterialRequests(ctx, db.ListMaterialRequestsParams{
		Column1: st,
		Column2: dest,
		Column3: usr,
		Column4: jur,
	})
	if err != nil {
		return nil, err
	}
	return filterInventoryRows(ctx, s.repo, rows, func(r db.ListMaterialRequestsRow) []string { return []string{r.DestinationLocationID} }, func(r db.ListMaterialRequestsRow) string { return r.RequestingUserID })

}

func (s *InventoryService) ListMaterialRequestItems(ctx context.Context, requestID string) ([]db.ListMaterialRequestItemsByRequestRow, error) {

	if operationalActor(ctx) != nil {
		if err := s.authorizeRequestRead(ctx, requestID); err != nil {
			return nil, err
		}
	}
	return s.repo.Queries().ListMaterialRequestItemsByRequest(ctx, requestID)
}

// ReviewMaterialRequest reviews (approves, reduces, or rejects) a material request with anti-self-approval and jurisdiction checks.
func (s *InventoryService) ReviewMaterialRequest(ctx context.Context, reviewer *db.User, appointments []db.Appointment, requestID string, input ReviewMaterialRequestInput) (*db.GetMaterialRequestByIDRow, error) {
	if reviewer == nil {
		return nil, errors.New("authenticated reviewer required")
	}

	req, err := s.repo.Queries().GetMaterialRequestByID(ctx, requestID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRequestNotFound
		}
		return nil, err
	}

	// 1. Anti-Self-Approval: The requester cannot approve, reduce, or reject their own request!
	if req.RequestingUserID == reviewer.ID {
		return nil, ErrAntiSelfApproval
	}

	// 2. Jurisdiction authority check
	if s.authorizeLocation(ctx, reviewer, req.DestinationLocationID) != nil {
		return nil, ErrJurisdictionMismatch
	}

	// 3. State check: Must be in 'submitted' status
	if req.Status != "submitted" {
		return nil, fmt.Errorf("%w: cannot review request in status '%s'", ErrInvalidRequestStatus, req.Status)
	}

	action := strings.ToLower(strings.TrimSpace(input.Action))
	var targetStatus string
	var rejReason, redReason *string

	switch action {
	case "approve":
		targetStatus = "approved"
	case "reduce":
		targetStatus = "reduced"
		if input.ReductionReason == nil || strings.TrimSpace(*input.ReductionReason) == "" {
			return nil, errors.New("reduction reason is mandatory when reducing a material request")
		}
		redReason = input.ReductionReason
	case "reject":
		targetStatus = "rejected"
		if input.RejectionReason == nil || strings.TrimSpace(*input.RejectionReason) == "" {
			return nil, errors.New("rejection reason is mandatory when rejecting a material request")
		}
		rejReason = input.RejectionReason
	default:
		return nil, fmt.Errorf("invalid review action: '%s'. Allowed actions: approve, reduce, reject", action)
	}

	err = s.withOperationalAudit(ctx, reviewer, "review_material_request", "material_request", requestID, func(q *db.Queries) error {
		if err := q.LockMaterialRequest(ctx, requestID); err != nil {
			return err
		}
		locked, err := q.GetMaterialRequestByID(ctx, requestID)
		if err != nil {
			return err
		}
		if locked.Status != "submitted" {
			return ErrInvalidRequestStatus
		}
		// Fetch request items to set approved quantities
		items, err := q.ListMaterialRequestItemsByRequest(ctx, requestID)
		if err != nil {
			return err
		}

		for _, itm := range items {
			var appQty int64
			if targetStatus == "rejected" {
				appQty = 0
			} else if targetStatus == "approve" || targetStatus == "approved" {
				appQty = itm.RequestedQuantity
			} else if targetStatus == "reduced" {
				if customQty, ok := input.ItemApprovedQuantities[itm.ItemID]; ok {
					if customQty > itm.RequestedQuantity {
						return fmt.Errorf("approved quantity (%d) cannot exceed requested quantity (%d) for item %s", customQty, itm.RequestedQuantity, itm.ItemName)
					}
					if customQty < 0 {
						return errors.New("approved quantity cannot be negative")
					}
					appQty = customQty
				} else {
					appQty = itm.RequestedQuantity
				}
			}

			_, err := q.UpdateMaterialRequestItemApprovedQty(ctx, db.UpdateMaterialRequestItemApprovedQtyParams{
				ID:               itm.ID,
				ApprovedQuantity: appQty,
			})
			if err != nil {
				return fmt.Errorf("failed to update item approved quantity: %w", err)
			}
		}

		_, err = q.UpdateMaterialRequestReview(ctx, db.UpdateMaterialRequestReviewParams{
			ID:               requestID,
			Status:           targetStatus,
			RejectionReason:  Text(rejReason),
			ReductionReason:  Text(redReason),
			ReviewedByUserID: TextFromString(reviewer.ID),
		})
		return err
	})

	if err != nil {
		return nil, err
	}

	return s.GetMaterialRequest(ctx, requestID)
}

// -------------------------------------------------------------
// 5. Stock Allocation & Transactional Reservations
// -------------------------------------------------------------

// AllocateAndReserveStock allocates and reserves stock from a supplying warehouse for an approved/reduced request.
// It acquires row locks (SELECT ... FOR UPDATE) to prevent race conditions and concurrent over-allocation.
func (s *InventoryService) AllocateAndReserveStock(ctx context.Context, actor *db.User, requestID, supplyingLocationID string) error {

	if err := s.authorizeLocation(ctx, actor, supplyingLocationID); err != nil {
		return err
	}
	return s.withOperationalAudit(ctx, actor, "allocate_material_stock", "material_request", requestID, func(q *db.Queries) error {
		if err := q.LockMaterialRequest(ctx, requestID); err != nil {
			return err
		}
		req, err := q.GetMaterialRequestByID(ctx, requestID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrRequestNotFound
			}
			return err
		}

		if req.Status != "approved" && req.Status != "reduced" {
			return fmt.Errorf("%w: request must be approved or reduced to allocate stock (current status: %s)", ErrInvalidRequestStatus, req.Status)
		}

		items, err := q.ListMaterialRequestItemsByRequest(ctx, requestID)
		if err != nil {
			return err
		}

		for _, itm := range items {
			if itm.ApprovedQuantity <= 0 {
				continue
			}

			// Acquire row lock FOR UPDATE on supplying location's stock balance
			bal, err := q.GetStockBalanceForUpdate(ctx, db.GetStockBalanceForUpdateParams{
				LocationID:  supplyingLocationID,
				ItemID:      itm.ItemID,
				BatchNumber: "",
			})
			if err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return fmt.Errorf("%w: no stock balance for item %s at location %s", ErrInsufficientStock, itm.ItemName, supplyingLocationID)
				}
				return err
			}

			available := bal.OnHandQuantity - bal.ReservedQuantity
			if available < itm.ApprovedQuantity {
				return fmt.Errorf("%w: item '%s' available stock is %d, but approved allocation requires %d", ErrInsufficientStock, itm.ItemName, available, itm.ApprovedQuantity)
			}

			// Increment reservation
			newReserved := bal.ReservedQuantity + itm.ApprovedQuantity
			_, err = q.SetStockBalanceQuantities(ctx, db.SetStockBalanceQuantitiesParams{
				LocationID:       supplyingLocationID,
				ItemID:           itm.ItemID,
				BatchNumber:      "",
				OnHandQuantity:   bal.OnHandQuantity,
				ReservedQuantity: newReserved,
			})
			if err != nil {
				return fmt.Errorf("failed to update reservation on stock balance: %w", err)
			}

			// Create reservation record
			_, err = q.CreateStockReservation(ctx, db.CreateStockReservationParams{
				ID:            security.GenerateUUID(),
				RequestID:     requestID,
				RequestItemID: TextFromString(itm.ID),
				LocationID:    supplyingLocationID,
				ItemID:        itm.ItemID,
				BatchNumber:   "",
				Quantity:      itm.ApprovedQuantity,
				Status:        "active",
				ExpiresAt:     pgtype.Timestamptz{Time: time.Now().AddDate(0, 0, 14), Valid: true},
			})
			if err != nil {
				return fmt.Errorf("failed to create stock reservation record: %w", err)
			}
		}

		// Update request status to 'allocated'
		_, err = q.UpdateMaterialRequestReview(ctx, db.UpdateMaterialRequestReviewParams{
			ID:               requestID,
			Status:           "allocated",
			RejectionReason:  req.RejectionReason,
			ReductionReason:  req.ReductionReason,
			ReviewedByUserID: req.ReviewedByUserID,
		})
		return err
	})
}

// -------------------------------------------------------------
// 6. Material Transfers & Dispatches
// -------------------------------------------------------------

func (s *InventoryService) CreateTransfer(ctx context.Context, actor *db.User, input CreateTransferInput) (*db.GetMaterialTransferByIDRow, error) {

	if err := s.authorizeLocation(ctx, actor, input.SourceLocationID); err != nil {
		return nil, err
	}
	if actor == nil {
		return nil, errors.New("authenticated actor required")
	}
	if input.SourceLocationID == input.DestinationLocationID {
		return nil, ErrSameLocationTransfer
	}
	if len(input.Items) == 0 {
		return nil, errors.New("transfer must contain at least one item")
	}

	// Check idempotency if key provided
	if input.IdempotencyKey != nil && *input.IdempotencyKey != "" {
		existing, err := s.repo.Queries().GetMaterialTransferByIdempotencyKey(ctx, Text(input.IdempotencyKey))
		if err == nil && existing.ID != "" {
			if existing.SourceLocationID != input.SourceLocationID || existing.DestinationLocationID != input.DestinationLocationID {
				return nil, ErrDuplicateTransfer
			}
			return s.GetTransfer(ctx, existing.ID)
		}
	}

	transferID := security.GenerateUUID()
	ref := ""
	if input.Reference != nil && *input.Reference != "" {
		ref = strings.TrimSpace(*input.Reference)
	} else {
		ref = fmt.Sprintf("TRF-%s-%s", time.Now().Format("20060102"), strings.ToUpper(security.GenerateUUID()[:6]))
	}

	err := s.withOperationalAudit(ctx, actor, "create_material_transfer", "material_transfer", transferID, func(q *db.Queries) error {
		// Check locations
		if _, err := q.GetStorageLocationByID(ctx, input.SourceLocationID); err != nil {
			return fmt.Errorf("source %w", ErrLocationNotFound)
		}
		if _, err := q.GetStorageLocationByID(ctx, input.DestinationLocationID); err != nil {
			return fmt.Errorf("destination %w", ErrLocationNotFound)
		}

		_, err := q.CreateMaterialTransfer(ctx, db.CreateMaterialTransferParams{
			ID:                    transferID,
			Reference:             ref,
			IdempotencyKey:        Text(input.IdempotencyKey),
			RequestID:             Text(input.RequestID),
			SourceLocationID:      input.SourceLocationID,
			DestinationLocationID: input.DestinationLocationID,
			Status:                "draft",
			CarrierName:           Text(input.CarrierName),
			CarrierPhone:          Text(input.CarrierPhone),
			TrackingNumber:        Text(input.TrackingNumber),
			DispatchNotes:         Text(input.DispatchNotes),
		})
		if err != nil {
			return fmt.Errorf("failed to create material transfer: %w", err)
		}

		for _, itm := range input.Items {
			if itm.DispatchedQuantity <= 0 {
				return errors.New("dispatched quantity must be greater than zero")
			}
			uom := itm.UnitOfMeasure
			if uom == "" {
				uom = "pcs"
			}
			_, err := q.CreateMaterialTransferItem(ctx, db.CreateMaterialTransferItemParams{
				ID:                  security.GenerateUUID(),
				TransferID:          transferID,
				ItemID:              itm.ItemID,
				BatchNumber:         strings.TrimSpace(itm.BatchNumber),
				DispatchedQuantity:  itm.DispatchedQuantity,
				ReceivedQuantity:    0,
				DiscrepancyQuantity: 0,
				UnitOfMeasure:       uom,
			})
			if err != nil {
				return fmt.Errorf("failed to create transfer item: %w", err)
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	if input.AutoDispatch {
		return s.DispatchTransfer(ctx, actor, transferID, DispatchTransferInput{
			CarrierName:    input.CarrierName,
			CarrierPhone:   input.CarrierPhone,
			TrackingNumber: input.TrackingNumber,
			DispatchNotes:  input.DispatchNotes,
		})
	}

	return s.GetTransfer(ctx, transferID)
}

func (s *InventoryService) GetTransfer(ctx context.Context, id string) (*db.GetMaterialTransferByIDRow, error) {

	if operationalActor(ctx) != nil {
		if err := s.authorizeTransferRead(ctx, id); err != nil {
			return nil, err
		}
	}
	t, err := s.repo.Queries().GetMaterialTransferByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrTransferNotFound
		}
		return nil, err
	}
	return &t, nil
}

func (s *InventoryService) ListTransfers(ctx context.Context, status, sourceLocationID, destinationLocationID *string) ([]db.ListMaterialTransfersRow, error) {
	var st, src, dst string
	if status != nil {
		st = *status
	}
	if sourceLocationID != nil {
		src = *sourceLocationID
	}
	if destinationLocationID != nil {
		dst = *destinationLocationID
	}
	rows, err := s.repo.Queries().ListMaterialTransfers(ctx, db.ListMaterialTransfersParams{
		Column1: st,
		Column2: src,
		Column3: dst,
	})
	if err != nil {
		return nil, err
	}
	return filterInventoryRows(ctx, s.repo, rows, func(r db.ListMaterialTransfersRow) []string {
		return []string{r.SourceLocationID, r.DestinationLocationID}
	}, nil)

}

func (s *InventoryService) ListTransferItems(ctx context.Context, transferID string) ([]db.ListMaterialTransferItemsByTransferRow, error) {

	if operationalActor(ctx) != nil {
		if err := s.authorizeTransferRead(ctx, transferID); err != nil {
			return nil, err
		}
	}
	return s.repo.Queries().ListMaterialTransferItemsByTransfer(ctx, transferID)
}

// DispatchTransfer transitions transfer to 'dispatched'.
// Crucially:
// 1. Source warehouse on-hand stock is decremented immediately.
// 2. Active reservations are released/fulfilled.
// 3. Stock ledger logs transfer_dispatch (-quantity) at source warehouse.
// 4. DESTINATION WAREHOUSE STOCK IS NOT CREDITED (remains in transit).
func (s *InventoryService) DispatchTransfer(ctx context.Context, actor *db.User, transferID string, req DispatchTransferInput) (*db.GetMaterialTransferByIDRow, error) {
	if actor == nil {
		return nil, errors.New("authenticated actor required")
	}

	target, authErr := s.repo.Queries().GetMaterialTransferByID(ctx, transferID)
	if authErr != nil {
		return nil, ErrTransferNotFound
	}
	if authErr = s.authorizeLocation(ctx, actor, target.SourceLocationID); authErr != nil {
		return nil, authErr
	}

	err := s.withOperationalAudit(ctx, actor, "dispatch_material_transfer", "material_transfer", transferID, func(q *db.Queries) error {
		if err := q.LockMaterialTransfer(ctx, transferID); err != nil {
			return err
		}
		t, err := q.GetMaterialTransferByID(ctx, transferID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrTransferNotFound
			}
			return err
		}

		if t.Status != "draft" {
			return fmt.Errorf("%w: transfer is in status '%s', expected 'draft'", ErrInvalidTransferStatus, t.Status)
		}

		items, err := q.ListMaterialTransferItemsByTransfer(ctx, transferID)
		if err != nil {
			return err
		}

		for _, itm := range items {
			// Acquire row lock FOR UPDATE on source location's stock balance
			bal, err := q.GetStockBalanceForUpdate(ctx, db.GetStockBalanceForUpdateParams{
				LocationID:  t.SourceLocationID,
				ItemID:      itm.ItemID,
				BatchNumber: itm.BatchNumber,
			})
			if err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return fmt.Errorf("%w: no stock balance for item %s at source warehouse", ErrInsufficientStock, itm.ItemName)
				}
				return err
			}

			if bal.OnHandQuantity < itm.DispatchedQuantity {
				return fmt.Errorf("%w: source warehouse has %d on hand, but dispatch requires %d for item %s", ErrInsufficientStock, bal.OnHandQuantity, itm.DispatchedQuantity, itm.ItemName)
			}

			// Decrement on_hand at source warehouse
			newOnHand := bal.OnHandQuantity - itm.DispatchedQuantity
			// If there was a reserved amount, reduce it accordingly
			newReserved := bal.ReservedQuantity
			if newReserved >= itm.DispatchedQuantity {
				newReserved -= itm.DispatchedQuantity
			} else {
				newReserved = 0
			}

			_, err = q.SetStockBalanceQuantities(ctx, db.SetStockBalanceQuantitiesParams{
				LocationID:       t.SourceLocationID,
				ItemID:           itm.ItemID,
				BatchNumber:      itm.BatchNumber,
				OnHandQuantity:   newOnHand,
				ReservedQuantity: newReserved,
			})
			if err != nil {
				return fmt.Errorf("failed to deduct dispatched stock: %w", err)
			}

			// Stock Ledger Entry for source dispatch
			ledgerRef := fmt.Sprintf("LED-%s-%s", time.Now().Format("20060102"), strings.ToUpper(security.GenerateUUID()[:8]))
			notes := fmt.Sprintf("Dispatched to %s (Ref: %s)", t.DestinationLocationName, t.Reference)
			_, err = q.CreateStockLedgerEntry(ctx, db.CreateStockLedgerEntryParams{
				ID:                   security.GenerateUUID(),
				Reference:            ledgerRef,
				LocationID:           t.SourceLocationID,
				ItemID:               itm.ItemID,
				MovementType:         "transfer_dispatch",
				Quantity:             -itm.DispatchedQuantity, // negative outflow
				BalanceAfter:         newOnHand,
				BatchNumber:          itm.BatchNumber,
				RelatedTransferID:    TextFromString(t.ID),
				RelatedRequestID:     t.RequestID,
				RelatedDiscrepancyID: pgtype.Text{Valid: false},
				Notes:                TextFromString(notes),
				ActorUserID:          TextFromString(actor.ID),
				OccurredAt:           pgtype.Timestamptz{Time: time.Now(), Valid: true},
			})
			if err != nil {
				return fmt.Errorf("failed to record dispatch stock ledger entry: %w", err)
			}
		}

		// Update transfer status to 'dispatched'
		carrierName := ""
		if req.CarrierName != nil {
			carrierName = *req.CarrierName
		} else if t.CarrierName.Valid {
			carrierName = t.CarrierName.String
		}
		carrierPhone := ""
		if req.CarrierPhone != nil {
			carrierPhone = *req.CarrierPhone
		} else if t.CarrierPhone.Valid {
			carrierPhone = t.CarrierPhone.String
		}
		trackingNo := ""
		if req.TrackingNumber != nil {
			trackingNo = *req.TrackingNumber
		} else if t.TrackingNumber.Valid {
			trackingNo = t.TrackingNumber.String
		}
		notes := ""
		if req.DispatchNotes != nil {
			notes = *req.DispatchNotes
		} else if t.DispatchNotes.Valid {
			notes = t.DispatchNotes.String
		}

		_, err = q.MarkMaterialTransferDispatched(ctx, db.MarkMaterialTransferDispatchedParams{
			ID:                 transferID,
			CarrierName:        TextFromString(carrierName),
			CarrierPhone:       TextFromString(carrierPhone),
			TrackingNumber:     TextFromString(trackingNo),
			DispatchNotes:      TextFromString(notes),
			DispatchedByUserID: TextFromString(actor.ID),
		})
		return err
	})

	if err != nil {
		return nil, err
	}

	return s.GetTransfer(ctx, transferID)
}

// -------------------------------------------------------------
// 7. Confirmed Receipts, Discrepancies & Idempotency
// -------------------------------------------------------------

// ConfirmTransferReceipt processes shipment arrival at destination.
// Requirements:
// 1. Destination stock credited ONLY by confirmed received_quantity.
// 2. Discrepancy (if received_quantity < dispatched_quantity) recorded in transfer_discrepancies.
// 3. Status set to 'disputed' if discrepancy exists, or 'received' if full receipt.
// 4. Stock ledger logs transfer_receipt (+received_quantity) at destination warehouse.
// 5. Idempotent: repeating the call returns existing record without duplicate crediting.
func (s *InventoryService) ConfirmTransferReceipt(ctx context.Context, actor *db.User, transferID string, req ConfirmReceiptInput) (*db.GetMaterialTransferByIDRow, error) {
	if actor == nil {
		return nil, errors.New("authenticated actor required")
	}

	target, authErr := s.repo.Queries().GetMaterialTransferByID(ctx, transferID)
	if authErr != nil {
		return nil, ErrTransferNotFound
	}
	if authErr = s.authorizeLocation(ctx, actor, target.DestinationLocationID); authErr != nil {
		return nil, authErr
	}

	err := s.withOperationalAudit(ctx, actor, "confirm_transfer_receipt", "material_transfer", transferID, func(q *db.Queries) error {
		if err := q.LockMaterialTransfer(ctx, transferID); err != nil {
			return err
		}
		t, err := q.GetMaterialTransferByID(ctx, transferID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrTransferNotFound
			}
			return err
		}

		// Idempotency: if already received or disputed, return safely without double crediting

		if t.Status == "received" || t.Status == "disputed" || t.Status == "partially_received" {
			return nil
		}

		if t.Status != "dispatched" {
			return fmt.Errorf("%w: transfer is in status '%s', expected 'dispatched'", ErrInvalidTransferStatus, t.Status)
		}

		items, err := q.ListMaterialTransferItemsByTransfer(ctx, transferID)
		if err != nil {
			return err
		}

		// Map received quantities by item ID
		receivedMap := make(map[string]int64)
		for _, ri := range req.Items {
			receivedMap[ri.ItemID] = ri.ReceivedQuantity
		}

		hasDiscrepancy := false

		for _, itm := range items {
			recvQty, ok := receivedMap[itm.ItemID]
			if !ok {
				// If not explicitly specified in receipt items, default to dispatched quantity
				recvQty = itm.DispatchedQuantity
			}

			if recvQty < 0 {
				return errors.New("received quantity cannot be negative")
			}
			if recvQty > itm.DispatchedQuantity {
				return fmt.Errorf("received quantity (%d) cannot exceed dispatched quantity (%d) for item %s", recvQty, itm.DispatchedQuantity, itm.ItemName)
			}

			discrepancyQty := itm.DispatchedQuantity - recvQty

			// Update transfer item with actual received & discrepancy quantities
			_, err = q.UpdateMaterialTransferItemReceipt(ctx, db.UpdateMaterialTransferItemReceiptParams{
				ID:                  itm.ID,
				ReceivedQuantity:    recvQty,
				DiscrepancyQuantity: discrepancyQty,
			})
			if err != nil {
				return fmt.Errorf("failed to update transfer item receipt: %w", err)
			}

			// If any goods received at destination (> 0), credit destination storage location
			if recvQty > 0 {
				// Lock/fetch destination balance
				destBal, err := q.GetStockBalanceForUpdate(ctx, db.GetStockBalanceForUpdateParams{
					LocationID:  t.DestinationLocationID,
					ItemID:      itm.ItemID,
					BatchNumber: itm.BatchNumber,
				})

				var destOnHand int64
				if errors.Is(err, pgx.ErrNoRows) {
					// Create new balance row for destination
					destOnHand = recvQty
					inserted, err := q.UpsertStockBalance(ctx, db.UpsertStockBalanceParams{
						ID:               security.GenerateUUID(),
						LocationID:       t.DestinationLocationID,
						ItemID:           itm.ItemID,
						OnHandQuantity:   destOnHand,
						ReservedQuantity: 0,
						BatchNumber:      itm.BatchNumber,
						ExpiryDate:       pgtype.Timestamptz{Valid: false},
					})
					if err != nil {
						return fmt.Errorf("failed to insert destination stock balance: %w", err)
					}
					destOnHand = inserted.OnHandQuantity
				} else if err != nil {
					return err
				} else {
					destOnHand = destBal.OnHandQuantity + recvQty
					_, err = q.SetStockBalanceQuantities(ctx, db.SetStockBalanceQuantitiesParams{
						LocationID:       t.DestinationLocationID,
						ItemID:           itm.ItemID,
						BatchNumber:      itm.BatchNumber,
						OnHandQuantity:   destOnHand,
						ReservedQuantity: destBal.ReservedQuantity,
					})
					if err != nil {
						return fmt.Errorf("failed to increment destination stock balance: %w", err)
					}
				}

				// Stock Ledger Entry for destination receipt
				ledgerRef := fmt.Sprintf("LED-%s-%s", time.Now().Format("20060102"), strings.ToUpper(security.GenerateUUID()[:8]))
				notes := fmt.Sprintf("Received from %s (Ref: %s)", t.SourceLocationName, t.Reference)
				_, err = q.CreateStockLedgerEntry(ctx, db.CreateStockLedgerEntryParams{
					ID:                   security.GenerateUUID(),
					Reference:            ledgerRef,
					LocationID:           t.DestinationLocationID,
					ItemID:               itm.ItemID,
					MovementType:         "transfer_receipt",
					Quantity:             recvQty, // positive inflow
					BalanceAfter:         destOnHand,
					BatchNumber:          itm.BatchNumber,
					RelatedTransferID:    TextFromString(t.ID),
					RelatedRequestID:     t.RequestID,
					RelatedDiscrepancyID: pgtype.Text{Valid: false},
					Notes:                TextFromString(notes),
					ActorUserID:          TextFromString(actor.ID),
					OccurredAt:           pgtype.Timestamptz{Time: time.Now(), Valid: true},
				})
				if err != nil {
					return fmt.Errorf("failed to record receipt stock ledger entry: %w", err)
				}
			}

			// If discrepancy quantity > 0, log a transfer_discrepancies entry
			if discrepancyQty > 0 {
				hasDiscrepancy = true
				reason := "missing_in_transit"
				if req.DiscrepancyReason != nil && *req.DiscrepancyReason != "" {
					reason = *req.DiscrepancyReason
				}

				discID := security.GenerateUUID()
				_, err = q.CreateTransferDiscrepancy(ctx, db.CreateTransferDiscrepancyParams{
					ID:                    discID,
					TransferID:            t.ID,
					TransferItemID:        itm.ID,
					ExpectedQuantity:      itm.DispatchedQuantity,
					ReceivedQuantity:      recvQty,
					DifferenceQuantity:    discrepancyQty,
					Reason:                reason,
					SupportingEvidenceUrl: Text(req.SupportingEvidenceURL),
					Status:                "open",
				})
				if err != nil {
					return fmt.Errorf("failed to create transfer discrepancy: %w", err)
				}
			}
		}

		targetStatus := "received"
		if hasDiscrepancy {
			targetStatus = "disputed"
		}

		_, err = q.MarkMaterialTransferReceived(ctx, db.MarkMaterialTransferReceivedParams{
			ID:               transferID,
			Status:           targetStatus,
			ReceivedByUserID: TextFromString(actor.ID),
		})
		if err != nil {
			return fmt.Errorf("failed to mark transfer received: %w", err)
		}

		// If this transfer originated from a material request, update request to 'fulfilled'
		if t.RequestID.Valid && t.RequestID.String != "" {
			reqID := t.RequestID.String
			matReq, err := q.GetMaterialRequestByID(ctx, reqID)
			if err == nil {
				_, _ = q.UpdateMaterialRequestReview(ctx, db.UpdateMaterialRequestReviewParams{
					ID:               reqID,
					Status:           "fulfilled",
					RejectionReason:  matReq.RejectionReason,
					ReductionReason:  matReq.ReductionReason,
					ReviewedByUserID: matReq.ReviewedByUserID,
				})
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return s.GetTransfer(ctx, transferID)
}

// -------------------------------------------------------------
// 8. Discrepancy Resolution
// -------------------------------------------------------------

func (s *InventoryService) ListDiscrepancies(ctx context.Context, status, transferID *string) ([]db.ListTransferDiscrepanciesRow, error) {
	var st, tx string
	if status != nil {
		st = *status
	}
	if transferID != nil {
		tx = *transferID
	}
	rows, err := s.repo.Queries().ListTransferDiscrepancies(ctx, db.ListTransferDiscrepanciesParams{
		Column1: st,
		Column2: tx,
	})
	if err != nil {
		return nil, err
	}
	if operationalActor(ctx) == nil {
		return rows, nil
	}
	out := []db.ListTransferDiscrepanciesRow{}
	for _, row := range rows {
		if s.authorizeTransferRead(ctx, row.TransferID) == nil {
			out = append(out, row)
		}
	}
	return out, nil

}

func (s *InventoryService) GetDiscrepancy(ctx context.Context, id string) (*db.GetTransferDiscrepancyByIDRow, error) {
	d, err := s.repo.Queries().GetTransferDiscrepancyByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrDiscrepancyNotFound
		}
		return nil, err
	}
	if err := s.authorizeTransferRead(ctx, d.TransferID); err != nil {
		return nil, err
	}
	return &d, nil
}

func (s *InventoryService) ResolveDiscrepancy(ctx context.Context, actor *db.User, discrepancyID string, input ResolveDiscrepancyInput) (*db.GetTransferDiscrepancyByIDRow, error) {
	if actor == nil {
		return nil, errors.New("authenticated actor required")
	}

	d, err := s.repo.Queries().GetTransferDiscrepancyByID(ctx, discrepancyID)
	if err != nil {
		return nil, ErrDiscrepancyNotFound
	}
	transfer, err := s.repo.Queries().GetMaterialTransferByID(ctx, d.TransferID)
	if err != nil {
		return nil, err
	}
	if s.authorizeLocation(ctx, actor, transfer.SourceLocationID) != nil && s.authorizeLocation(ctx, actor, transfer.DestinationLocationID) != nil {
		return nil, ErrJurisdictionMismatch
	}

	resolution := strings.ToLower(strings.TrimSpace(input.Resolution))
	var targetStatus string
	switch resolution {
	case "write_off", "writeoff":
		targetStatus = "resolved_writeoff"
	case "returned", "return":
		targetStatus = "resolved_returned"
	default:
		return nil, fmt.Errorf("%w: '%s'. Allowed: write_off, returned", ErrInvalidDiscrepancyResolution, resolution)
	}

	if input.Notes == "" {
		return nil, errors.New("resolution notes are mandatory")
	}

	err = s.withOperationalAudit(ctx, actor, "resolve_transfer_discrepancy", "transfer_discrepancy", discrepancyID, func(q *db.Queries) error {
		if err := q.LockTransferDiscrepancy(ctx, discrepancyID); err != nil {
			return err
		}
		d, err := q.GetTransferDiscrepancyByID(ctx, discrepancyID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrDiscrepancyNotFound
			}
			return err
		}

		if d.Status != "open" && d.Status != "investigating" {
			return fmt.Errorf("cannot resolve discrepancy with status '%s'", d.Status)
		}

		_, err = q.ResolveTransferDiscrepancy(ctx, db.ResolveTransferDiscrepancyParams{
			ID:                 discrepancyID,
			Status:             targetStatus,
			AssignedReviewerID: TextFromString(actor.ID),
			ResolutionNotes:    TextFromString(input.Notes),
		})
		if err != nil {
			return err
		}

		// If write-off: append compensating ledger entry
		if targetStatus == "resolved_writeoff" {
			ledgerRef := fmt.Sprintf("LED-%s-%s", time.Now().Format("20060102"), strings.ToUpper(security.GenerateUUID()[:8]))
			notes := fmt.Sprintf("Loss write-off for discrepancy (%s): %s", d.TransferReference, input.Notes)
			_, err = q.CreateStockLedgerEntry(ctx, db.CreateStockLedgerEntryParams{
				ID:                   security.GenerateUUID(),
				Reference:            ledgerRef,
				LocationID:           d.DestinationLocationID,
				ItemID:               d.ItemID,
				MovementType:         "write_off_loss",
				Quantity:             -d.DifferenceQuantity,
				BalanceAfter:         0,
				BatchNumber:          "",
				RelatedTransferID:    TextFromString(d.TransferID),
				RelatedRequestID:     pgtype.Text{Valid: false},
				RelatedDiscrepancyID: TextFromString(d.ID),
				Notes:                TextFromString(notes),
				ActorUserID:          TextFromString(actor.ID),
				OccurredAt:           pgtype.Timestamptz{Time: time.Now(), Valid: true},
			})
			if err != nil {
				return fmt.Errorf("failed to append discrepancy write-off ledger entry: %w", err)
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return s.GetDiscrepancy(ctx, discrepancyID)
}

// -------------------------------------------------------------
// 9. Immutable Stock Ledger & System Summary
// -------------------------------------------------------------

func (s *InventoryService) ListStockLedger(ctx context.Context, locationID, itemID, movementType *string, startDate, endDate *time.Time, limit, offset int32) ([]db.ListStockLedgerEntriesRow, int64, error) {
	if limit <= 0 {
		limit = 50
	}
	var loc, itm, mtype string
	if locationID != nil {
		loc = *locationID
	}
	if itemID != nil {
		itm = *itemID
	}
	if movementType != nil {
		mtype = *movementType
	}
	var sDate, eDate pgtype.Timestamptz
	if startDate != nil {
		sDate = pgtype.Timestamptz{Time: *startDate, Valid: true}
	}
	if endDate != nil {
		eDate = pgtype.Timestamptz{Time: *endDate, Valid: true}
	}

	if operationalActor(ctx) != nil {
		ids, err := s.scopedLocationIDs(ctx)
		if err != nil {
			return nil, 0, err
		}
		entries, err := s.repo.Queries().ListScopedStockLedgerEntries(ctx, db.ListStockLedgerEntriesParams{Column1: loc, Column2: itm, Column3: mtype, Column4: sDate, Column5: eDate, Limit: limit, Offset: offset}, ids)
		if err != nil {
			return nil, 0, err
		}
		total, err := s.repo.Queries().CountScopedStockLedgerEntries(ctx, db.CountStockLedgerEntriesParams{Column1: loc, Column2: itm, Column3: mtype, Column4: sDate, Column5: eDate}, ids)
		return entries, total, err
	}

	entries, err := s.repo.Queries().ListStockLedgerEntries(ctx, db.ListStockLedgerEntriesParams{
		Column1: loc,
		Column2: itm,
		Column3: mtype,
		Column4: sDate,
		Column5: eDate,
		Limit:   limit,
		Offset:  offset,
	})
	if err != nil {
		return nil, 0, err
	}

	total, err := s.repo.Queries().CountStockLedgerEntries(ctx, db.CountStockLedgerEntriesParams{
		Column1: loc,
		Column2: itm,
		Column3: mtype,
		Column4: sDate,
		Column5: eDate,
	})
	if err != nil {
		total = int64(len(entries))
	}

	return entries, total, nil
}

func (s *InventoryService) GetInventorySummary(ctx context.Context) (*InventorySummary, error) {
	items, err := s.repo.Queries().ListInventoryItems(ctx, db.ListInventoryItemsParams{
		Column1: "",
		Column2: true,
	})
	if err != nil {
		return nil, err
	}

	locations, err := s.repo.Queries().ListStorageLocations(ctx, db.ListStorageLocationsParams{
		Column1: "",
		Column2: "",
		Column3: "",
		Column4: "",
		Column5: true,
	})
	if err != nil {
		return nil, err
	}

	transfers, err := s.repo.Queries().ListMaterialTransfers(ctx, db.ListMaterialTransfersParams{
		Column1: "dispatched",
		Column2: "",
		Column3: "",
	})
	if err != nil {
		return nil, err
	}

	discrepancies, err := s.repo.Queries().ListTransferDiscrepancies(ctx, db.ListTransferDiscrepanciesParams{
		Column1: "open",
		Column2: "",
	})
	if err != nil {
		return nil, err
	}

	lowStock, err := s.repo.Queries().ListLowStockAlerts(ctx)
	if err != nil {
		return nil, err
	}

	if operationalActor(ctx) != nil {
		locations, err = s.ListLocations(ctx, nil, nil, nil, nil, nil)
		if err != nil {
			return nil, err
		}
		dispatched := "dispatched"
		transfers, err = s.ListTransfers(ctx, &dispatched, nil, nil)
		if err != nil {
			return nil, err
		}
		open := "open"
		discrepancies, err = s.ListDiscrepancies(ctx, &open, nil)
		if err != nil {
			return nil, err
		}
		lowStock, err = s.ListLowStockAlerts(ctx)
		if err != nil {
			return nil, err
		}
	}

	return &InventorySummary{
		TotalItems:          int64(len(items)),
		TotalLocations:      int64(len(locations)),
		InTransitTransfers:  int64(len(transfers)),
		OpenDiscrepancies:   int64(len(discrepancies)),
		LowStockAlertsCount: int64(len(lowStock)),
	}, nil
}

// -------------------------------------------------------------
// Helper: Geographic Jurisdiction Authority
// -------------------------------------------------------------

func (s *InventoryService) hasJurisdictionAuthority(user *db.User, appointments []db.Appointment, targetJurisdictionType string, targetJurisdictionID *string) bool {
	if user == nil {
		return false
	}

	// Super Admin and National Admin have national jurisdiction covering all operations
	if user.PrimaryRole == "super_admin" || user.PrimaryRole == "national_admin" {
		return true
	}

	for _, a := range appointments {
		if a.Status != "active" {
			continue
		}
		if a.RoleType == "super_admin" || a.RoleType == "national_admin" {
			return true
		}
	}

	// Normalize jurisdiction
	targetType := strings.ToLower(targetJurisdictionType)
	targetID := ""
	if targetJurisdictionID != nil {
		targetID = *targetJurisdictionID
	}

	for _, a := range appointments {
		if a.Status != "active" {
			continue
		}

		appJurisdictionType := strings.ToLower(a.JurisdictionType)
		appJurisdictionID := ""
		if a.JurisdictionID.Valid {
			appJurisdictionID = a.JurisdictionID.String
		}

		// National Coordinator
		if appJurisdictionType == "national" {
			return true
		}

		// Zonal Coordinator
		if (appJurisdictionType == "zone" || appJurisdictionType == "zonal") && (a.RoleType == "zonal_coordinator") {
			if (targetType == "zone" || targetType == "zonal") && appJurisdictionID == targetID {
				return true
			}
			// If target is state, check if state belongs to this zone
			if targetType == "state" && targetID != "" {
				st, err := s.repo.Queries().GetStateByID(context.Background(), targetID)
				if err == nil && st.ZoneID == appJurisdictionID {
					return true
				}
			}
		}

		// State Coordinator
		if appJurisdictionType == "state" && (a.RoleType == "state_coordinator") {
			if targetType == "state" && appJurisdictionID == targetID {
				return true
			}
			// If target is LGA, check if LGA belongs to this state
			if targetType == "lga" && targetID != "" {
				lga, err := s.repo.Queries().GetLGAByID(context.Background(), targetID)
				if err == nil && lga.StateID == appJurisdictionID {
					return true
				}
			}
		}

		// LGA Representative / Coordinator
		if appJurisdictionType == "lga" && (a.RoleType == "lga_representative" || a.RoleType == "lga_coordinator") {
			if targetType == "lga" && appJurisdictionID == targetID {
				return true
			}
		}
	}

	return false
}

func valueOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
