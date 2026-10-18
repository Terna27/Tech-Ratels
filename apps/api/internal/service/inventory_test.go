package service_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"ratels-api/internal/db"
	"ratels-api/internal/service"
)

func createRegisteredUser(t *testing.T, env *TestEnv, emailPrefix, role string) *db.User {
	t.Helper()
	email := fmt.Sprintf("%s_%d@ratels.example.org", emailPrefix, time.Now().UnixNano())
	reg, err := env.AuthService.Register(context.Background(), service.RegisterRequest{
		Email:       email,
		Password:    "TestPassword123!",
		DisplayName: fmt.Sprintf("Test User %s", emailPrefix),
	})
	if err != nil {
		t.Fatalf("Failed to register test user %s: %v", emailPrefix, err)
	}

	_, err = env.Pool.Exec(context.Background(), "UPDATE users SET primary_role = $1, status = 'active', is_verified = true WHERE id = $2", role, reg.UserID)
	if err != nil {
		t.Fatalf("Failed to promote test user: %v", err)
	}

	u, err := env.Repo.Queries().GetUserByID(context.Background(), reg.UserID)
	if err != nil {
		t.Fatalf("Failed to get promoted user: %v", err)
	}
	return &u
}

func TestInventoryService_CatalogueAndLocations(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	// 1. Create inventory items
	item1, err := env.InventoryService.CreateItem(ctx, nil, service.CreateItemRequest{
		SKU:                  fmt.Sprintf("TEST-ITM-%d", time.Now().UnixNano()),
		Name:                 "Industrial Waste Cleanup Rake",
		Description:          strPtr("Reinforced steel rake for outdoor cleanup"),
		Category:             "tools",
		UnitOfMeasure:        "pcs",
		MinThresholdQuantity: 15,
		IsBatchTracked:       false,
	})
	if err != nil {
		t.Fatalf("Failed to create inventory item: %v", err)
	}
	if item1.ID == "" || item1.MinThresholdQuantity != 15 {
		t.Errorf("Unexpected item created: %+v", item1)
	}

	// 2. Fetch created item
	fetchedItem, err := env.InventoryService.GetItem(ctx, item1.ID)
	if err != nil {
		t.Fatalf("Failed to fetch item: %v", err)
	}
	if fetchedItem.Name != item1.Name {
		t.Errorf("Expected name %s, got %s", item1.Name, fetchedItem.Name)
	}

	// 3. Create storage locations (National and State warehouses)
	wh1, err := env.InventoryService.CreateLocation(ctx, nil, service.CreateLocationRequest{
		Code:         fmt.Sprintf("WH-NAT-%d", time.Now().UnixNano()),
		Name:         "Abuja Central Distribution Hub",
		LocationType: "national",
		Address:      strPtr("Central Area, Abuja, FCT"),
	})
	if err != nil {
		t.Fatalf("Failed to create national warehouse: %v", err)
	}

	wh2, err := env.InventoryService.CreateLocation(ctx, nil, service.CreateLocationRequest{
		Code:         fmt.Sprintf("WH-ST-LAG-%d", time.Now().UnixNano()),
		Name:         "Lagos State Sanitation Depot",
		LocationType: "state",
		Address:      strPtr("Ikeja, Lagos State"),
	})
	if err != nil {
		t.Fatalf("Failed to create state warehouse: %v", err)
	}

	if wh1.ID == wh2.ID {
		t.Errorf("Warehouse IDs must be distinct")
	}
}

func TestInventoryService_StockReceiptAndLedger(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	saUser := createRegisteredUser(t, env, "sa_rcp", "super_admin")

	wh, err := env.InventoryService.CreateLocation(ctx, saUser, service.CreateLocationRequest{
		Code:         fmt.Sprintf("WH-RCP-%d", time.Now().UnixNano()),
		Name:         "Receipt Test Warehouse",
		LocationType: "national",
	})
	if err != nil {
		t.Fatalf("Failed to create warehouse: %v", err)
	}

	item, err := env.InventoryService.CreateItem(ctx, saUser, service.CreateItemRequest{
		SKU:                  fmt.Sprintf("ITM-RCP-%d", time.Now().UnixNano()),
		Name:                 "Protective Boots",
		Category:             "protective_equipment",
		UnitOfMeasure:        "pairs",
		MinThresholdQuantity: 10,
	})
	if err != nil {
		t.Fatalf("Failed to create item: %v", err)
	}

	// Receive 50 pairs of boots into warehouse
	bal, err := env.InventoryService.ReceiveStock(ctx, saUser, service.ReceiveStockRequest{
		LocationID:   wh.ID,
		ItemID:       item.ID,
		Quantity:     50,
		MovementType: "initial_receipt",
		Notes:        strPtr("Bulk delivery from manufacturer"),
	})
	if err != nil {
		t.Fatalf("Failed to receive stock: %v", err)
	}

	if bal.OnHandQuantity != 50 {
		t.Errorf("Expected on hand 50, got %d", bal.OnHandQuantity)
	}
	if bal.ReservedQuantity != 0 {
		t.Errorf("Expected reserved 0, got %d", bal.ReservedQuantity)
	}

	// Verify stock ledger has recorded the movement
	entries, total, err := env.InventoryService.ListStockLedger(ctx, &wh.ID, &item.ID, nil, nil, nil, 10, 0)
	if err != nil {
		t.Fatalf("Failed to list ledger: %v", err)
	}
	if total < 1 || len(entries) < 1 {
		t.Fatalf("Expected at least 1 ledger entry, got %d", total)
	}
	if entries[0].Quantity != 50 {
		t.Errorf("Expected ledger quantity 50, got %d", entries[0].Quantity)
	}
	if entries[0].BalanceAfter != 50 {
		t.Errorf("Expected balance after 50, got %d", entries[0].BalanceAfter)
	}
}

func TestInventoryService_The100BagDispatched95BagReceivedScenario(t *testing.T) {
	// Acceptance test:
	// A warehouse dispatches 100 bags.
	// The destination confirms receipt of 95 bags.
	// 5 bags remain disputed in transit.
	// Destination on-hand increases by 95 only.
	// Retry is idempotent and does not credit again.
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	// 1. Setup Actor: Super Admin user
	saUser := createRegisteredUser(t, env, "sa_100bag", "super_admin")

	// 2. Setup Warehouses: Source (Warehouse A) and Destination (Warehouse B)
	whSource, err := env.InventoryService.CreateLocation(ctx, saUser, service.CreateLocationRequest{
		Code:         fmt.Sprintf("WH-SRC-100-%d", time.Now().UnixNano()),
		Name:         "Central Supply Depot",
		LocationType: "national",
	})
	if err != nil {
		t.Fatalf("Failed to create source warehouse: %v", err)
	}

	whDest, err := env.InventoryService.CreateLocation(ctx, saUser, service.CreateLocationRequest{
		Code:         fmt.Sprintf("WH-DST-100-%d", time.Now().UnixNano()),
		Name:         "Regional Field Depot",
		LocationType: "state",
	})
	if err != nil {
		t.Fatalf("Failed to create destination warehouse: %v", err)
	}

	// 3. Setup Item: Heavy-Duty Trash Bags
	item, err := env.InventoryService.CreateItem(ctx, saUser, service.CreateItemRequest{
		SKU:                  fmt.Sprintf("BAG-100-%d", time.Now().UnixNano()),
		Name:                 "Heavy-Duty Sanitation Trash Bags",
		Category:             "tools",
		UnitOfMeasure:        "bags",
		MinThresholdQuantity: 20,
	})
	if err != nil {
		t.Fatalf("Failed to create item: %v", err)
	}

	// 4. Source Warehouse receives initial stock of 100 bags
	_, err = env.InventoryService.ReceiveStock(ctx, saUser, service.ReceiveStockRequest{
		LocationID:   whSource.ID,
		ItemID:       item.ID,
		Quantity:     100,
		MovementType: "initial_receipt",
		Notes:        strPtr("Initial batch of 100 bags"),
	})
	if err != nil {
		t.Fatalf("Failed to receive 100 bags at source: %v", err)
	}

	// 5. Create transfer of 100 bags from Source to Destination
	idempotencyKey := fmt.Sprintf("idem-trf-%d", time.Now().UnixNano())
	transfer, err := env.InventoryService.CreateTransfer(ctx, saUser, service.CreateTransferInput{
		IdempotencyKey:        &idempotencyKey,
		SourceLocationID:      whSource.ID,
		DestinationLocationID: whDest.ID,
		CarrierName:           strPtr("Express Ratels Logistics"),
		TrackingNumber:        strPtr("TRK-BAG-9900"),
		Items: []service.CreateTransferItemInput{
			{
				ItemID:             item.ID,
				DispatchedQuantity: 100,
				UnitOfMeasure:      "bags",
			},
		},
	})
	if err != nil {
		t.Fatalf("Failed to create transfer: %v", err)
	}
	if transfer.Status != "draft" {
		t.Errorf("Expected transfer status 'draft', got '%s'", transfer.Status)
	}

	// 6. Dispatch Transfer of 100 bags
	dispatched, err := env.InventoryService.DispatchTransfer(ctx, saUser, transfer.ID, service.DispatchTransferInput{
		CarrierName: strPtr("Express Ratels Logistics"),
	})
	if err != nil {
		t.Fatalf("Failed to dispatch transfer: %v", err)
	}
	if dispatched.Status != "dispatched" {
		t.Errorf("Expected status 'dispatched', got '%s'", dispatched.Status)
	}

	// Verify Source warehouse stock is immediately deducted to 0
	srcBalances, err := env.InventoryService.ListStockBalances(ctx, &whSource.ID, &item.ID, nil)
	if err != nil {
		t.Fatalf("Failed to check source balance: %v", err)
	}
	if len(srcBalances) != 1 || srcBalances[0].OnHandQuantity != 0 {
		t.Fatalf("Expected source warehouse on hand 0, got %d", srcBalances[0].OnHandQuantity)
	}

	// CRITICAL: Verify Destination warehouse has NOT been credited yet (remains 0 in transit)
	dstBalancesBefore, err := env.InventoryService.ListStockBalances(ctx, &whDest.ID, &item.ID, nil)
	if err != nil {
		t.Fatalf("Failed to check destination balance: %v", err)
	}
	if len(dstBalancesBefore) > 0 && dstBalancesBefore[0].OnHandQuantity > 0 {
		t.Fatalf("Destination warehouse must NOT receive stock before receipt confirmation! Found %d", dstBalancesBefore[0].OnHandQuantity)
	}

	// 7. Destination confirms receipt of 95 bags (5 bags missing in transit)
	receiptIdemKey := fmt.Sprintf("idem-recv-%d", time.Now().UnixNano())
	received, err := env.InventoryService.ConfirmTransferReceipt(ctx, saUser, transfer.ID, service.ConfirmReceiptInput{
		IdempotencyKey: &receiptIdemKey,
		Items: []service.ReceiptItemInput{
			{
				ItemID:           item.ID,
				ReceivedQuantity: 95,
			},
		},
		DiscrepancyReason: strPtr("missing_in_transit"),
	})
	if err != nil {
		t.Fatalf("Failed to confirm receipt of 95 bags: %v", err)
	}

	// Transfer status must be 'disputed' due to missing 5 bags
	if received.Status != "disputed" {
		t.Errorf("Expected transfer status 'disputed', got '%s'", received.Status)
	}

	// Destination warehouse stock must now be EXACTLY 95 bags
	dstBalancesAfter, err := env.InventoryService.ListStockBalances(ctx, &whDest.ID, &item.ID, nil)
	if err != nil {
		t.Fatalf("Failed to check destination balance after receipt: %v", err)
	}
	if len(dstBalancesAfter) != 1 || dstBalancesAfter[0].OnHandQuantity != 95 {
		t.Fatalf("Expected destination warehouse on hand 95, got %d", dstBalancesAfter[0].OnHandQuantity)
	}

	// Discrepancy record must be generated with difference = 5
	discrepancies, err := env.InventoryService.ListDiscrepancies(ctx, nil, &transfer.ID)
	if err != nil {
		t.Fatalf("Failed to list discrepancies: %v", err)
	}
	if len(discrepancies) != 1 {
		t.Fatalf("Expected exactly 1 discrepancy record, got %d", len(discrepancies))
	}
	disc := discrepancies[0]
	if disc.ExpectedQuantity != 100 {
		t.Errorf("Expected discrepancy expected_qty 100, got %d", disc.ExpectedQuantity)
	}
	if disc.ReceivedQuantity != 95 {
		t.Errorf("Expected discrepancy received_qty 95, got %d", disc.ReceivedQuantity)
	}
	if disc.DifferenceQuantity != 5 {
		t.Errorf("Expected discrepancy difference_qty 5, got %d", disc.DifferenceQuantity)
	}
	if disc.Reason != "missing_in_transit" {
		t.Errorf("Expected reason 'missing_in_transit', got '%s'", disc.Reason)
	}
	if disc.Status != "open" {
		t.Errorf("Expected status 'open', got '%s'", disc.Status)
	}

	// 8. Idempotency test: retry the exact receipt call
	retryReceived, err := env.InventoryService.ConfirmTransferReceipt(ctx, saUser, transfer.ID, service.ConfirmReceiptInput{
		IdempotencyKey: &receiptIdemKey,
		Items: []service.ReceiptItemInput{
			{
				ItemID:           item.ID,
				ReceivedQuantity: 95,
			},
		},
	})
	if err != nil {
		t.Fatalf("Idempotent retry failed: %v", err)
	}
	if retryReceived.ID != received.ID {
		t.Errorf("Expected same transfer ID on retry")
	}

	// Destination warehouse stock must STILL be exactly 95 (no duplicate credit!)
	dstBalancesRetry, err := env.InventoryService.ListStockBalances(ctx, &whDest.ID, &item.ID, nil)
	if err != nil {
		t.Fatalf("Failed to check balance on retry: %v", err)
	}
	if dstBalancesRetry[0].OnHandQuantity != 95 {
		t.Fatalf("Idempotency violation! Stock was credited again: expected 95, got %d", dstBalancesRetry[0].OnHandQuantity)
	}

	// 9. Resolve Discrepancy via loss write-off
	resolvedDisc, err := env.InventoryService.ResolveDiscrepancy(ctx, saUser, disc.ID, service.ResolveDiscrepancyInput{
		Resolution: "write_off",
		Notes:      "5 bags torn and lost during inter-state haulage; approved loss write-off",
	})
	if err != nil {
		t.Fatalf("Failed to resolve discrepancy: %v", err)
	}
	if resolvedDisc.Status != "resolved_writeoff" {
		t.Errorf("Expected status 'resolved_writeoff', got '%s'", resolvedDisc.Status)
	}
}

func TestInventoryService_AntiSelfApprovalAndJurisdictionCheck(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	saUser := createRegisteredUser(t, env, "sa_jur", "super_admin")

	wh, err := env.InventoryService.CreateLocation(ctx, saUser, service.CreateLocationRequest{
		Code:         fmt.Sprintf("WH-JUR-%d", time.Now().UnixNano()),
		Name:         "Jurisdiction Warehouse",
		LocationType: "state",
	})
	if err != nil {
		t.Fatalf("Failed to create warehouse: %v", err)
	}

	item, err := env.InventoryService.CreateItem(ctx, saUser, service.CreateItemRequest{
		SKU:                  fmt.Sprintf("ITM-JUR-%d", time.Now().UnixNano()),
		Name:                 "First Aid Kit",
		Category:             "first_aid",
		UnitOfMeasure:        "kits",
		MinThresholdQuantity: 5,
	})
	if err != nil {
		t.Fatalf("Failed to create item: %v", err)
	}

	// Create requesting user
	requester := createRegisteredUser(t, env, "req_usr", "member")

	matReq, err := env.InventoryService.CreateMaterialRequest(ctx, requester, service.CreateMaterialRequestInput{
		JurisdictionType:      "state",
		JurisdictionID:        strPtr("state_lagos"),
		DestinationLocationID: wh.ID,
		IntendedUse:           "Lekki Beach Cleanup Drive",
		RequiredDate:          time.Now().AddDate(0, 0, 5),
		Items: []service.MaterialRequestItemInput{
			{
				ItemID:            item.ID,
				RequestedQuantity: 10,
				UnitOfMeasure:     "kits",
			},
		},
	})
	if err != nil {
		t.Fatalf("Failed to create material request: %v", err)
	}

	// 1. Anti-Self-Approval test: requester attempts to approve their own request
	_, err = env.InventoryService.ReviewMaterialRequest(ctx, requester, nil, matReq.ID, service.ReviewMaterialRequestInput{
		Action: "approve",
	})
	if !errors.Is(err, service.ErrAntiSelfApproval) {
		t.Fatalf("Expected ErrAntiSelfApproval, got %v", err)
	}

	// 2. Jurisdiction mismatch test: State coordinator of Benue attempts to approve Lagos request
	benueCoord := createRegisteredUser(t, env, "benue_coord", "state_coordinator")
	benueAppointments := []db.Appointment{
		{
			ID:               "app_benue_1",
			UserID:           benueCoord.ID,
			RoleType:         "state_coordinator",
			JurisdictionType: "state",
			JurisdictionID:   service.TextFromString("state_benue"),
			Status:           "active",
		},
	}

	_, err = env.InventoryService.ReviewMaterialRequest(ctx, benueCoord, benueAppointments, matReq.ID, service.ReviewMaterialRequestInput{
		Action: "approve",
	})
	if !errors.Is(err, service.ErrJurisdictionMismatch) {
		t.Fatalf("Expected ErrJurisdictionMismatch, got %v", err)
	}

	// 3. Valid Approval by authorized Super Admin
	reviewed, err := env.InventoryService.ReviewMaterialRequest(ctx, saUser, nil, matReq.ID, service.ReviewMaterialRequestInput{
		Action: "approve",
	})
	if err != nil {
		t.Fatalf("Super admin approval failed: %v", err)
	}
	if reviewed.Status != "approved" {
		t.Errorf("Expected status 'approved', got '%s'", reviewed.Status)
	}
}

func TestInventoryService_ConcurrentOverAllocationProtection(t *testing.T) {
	// Concurrency test:
	// A warehouse has 10 units available.
	// Two concurrent allocations attempt to reserve 8 units each (8 + 8 = 16 > 10).
	// Exactly one must succeed and one must fail with ErrInsufficientStock.
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	saUser := createRegisteredUser(t, env, "sa_concur", "super_admin")

	wh, err := env.InventoryService.CreateLocation(ctx, saUser, service.CreateLocationRequest{
		Code:         fmt.Sprintf("WH-CONC-%d", time.Now().UnixNano()),
		Name:         "Concurrent Test Warehouse",
		LocationType: "national",
	})
	if err != nil {
		t.Fatalf("Failed to create warehouse: %v", err)
	}

	item, err := env.InventoryService.CreateItem(ctx, saUser, service.CreateItemRequest{
		SKU:                  fmt.Sprintf("ITM-CONC-%d", time.Now().UnixNano()),
		Name:                 "Wheelbarrow",
		Category:             "tools",
		UnitOfMeasure:        "pcs",
		MinThresholdQuantity: 5,
	})
	if err != nil {
		t.Fatalf("Failed to create item: %v", err)
	}

	// Stock 10 wheelbarrows
	_, err = env.InventoryService.ReceiveStock(ctx, saUser, service.ReceiveStockRequest{
		LocationID: wh.ID,
		ItemID:     item.ID,
		Quantity:   10,
	})
	if err != nil {
		t.Fatalf("Failed to stock 10 items: %v", err)
	}

	// Create Request 1 (8 units) and approve
	reqUser1 := createRegisteredUser(t, env, "req_conc_1", "member")
	req1, err := env.InventoryService.CreateMaterialRequest(ctx, reqUser1, service.CreateMaterialRequestInput{
		DestinationLocationID: wh.ID,
		IntendedUse:           "Cleanup Project Alpha",
		Items:                 []service.MaterialRequestItemInput{{ItemID: item.ID, RequestedQuantity: 8, UnitOfMeasure: "pcs"}},
	})
	if err != nil {
		t.Fatalf("Failed to create req1: %v", err)
	}
	_, _ = env.InventoryService.ReviewMaterialRequest(ctx, saUser, nil, req1.ID, service.ReviewMaterialRequestInput{Action: "approve"})

	// Create Request 2 (8 units) and approve
	reqUser2 := createRegisteredUser(t, env, "req_conc_2", "member")
	req2, err := env.InventoryService.CreateMaterialRequest(ctx, reqUser2, service.CreateMaterialRequestInput{
		DestinationLocationID: wh.ID,
		IntendedUse:           "Cleanup Project Beta",
		Items:                 []service.MaterialRequestItemInput{{ItemID: item.ID, RequestedQuantity: 8, UnitOfMeasure: "pcs"}},
	})
	if err != nil {
		t.Fatalf("Failed to create req2: %v", err)
	}
	_, _ = env.InventoryService.ReviewMaterialRequest(ctx, saUser, nil, req2.ID, service.ReviewMaterialRequestInput{Action: "approve"})

	// Concurrently attempt to allocate both
	var wg sync.WaitGroup
	var err1, err2 error

	wg.Add(2)
	go func() {
		defer wg.Done()
		err1 = env.InventoryService.AllocateAndReserveStock(ctx, saUser, req1.ID, wh.ID)
	}()
	go func() {
		defer wg.Done()
		err2 = env.InventoryService.AllocateAndReserveStock(ctx, saUser, req2.ID, wh.ID)
	}()
	wg.Wait()

	// Exactly one must have succeeded, and one must have failed with insufficient stock
	successCount := 0
	failureCount := 0

	if err1 == nil {
		successCount++
	} else if errors.Is(err1, service.ErrInsufficientStock) {
		failureCount++
	}

	if err2 == nil {
		successCount++
	} else if errors.Is(err2, service.ErrInsufficientStock) {
		failureCount++
	}

	if successCount != 1 || failureCount != 1 {
		t.Fatalf("Race condition protection failed! Success: %d, Failure: %d (Err1: %v, Err2: %v)", successCount, failureCount, err1, err2)
	}
}
