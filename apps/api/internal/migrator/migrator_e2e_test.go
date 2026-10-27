package migrator_test

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"ratels-api/internal/migrator"
)

func TestMigrations_Migration22ForwardBackwardAndDataPreservation(t *testing.T) {
	testDBURL := os.Getenv("TEST_DATABASE_URL")
	if testDBURL == "" {
		testDBURL = "postgres://postgres@127.0.0.1:5432/ratels_migration_audit_db?sslmode=disable"
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, testDBURL)
	if err != nil {
		t.Fatalf("failed connecting to migration test database: %v", err)
	}
	defer pool.Close()

	// 1. Ensure all migrations up to 22 are applied
	if err := migrator.RunMigrations(ctx, pool); err != nil {
		t.Fatalf("migrator.RunMigrations failed: %v", err)
	}

	// 2. Validate columns created by Migration 22 on marketplace_refunds
	expectedRefundCols := []string{"shipment_id", "business_id", "order_item_id"}
	for _, col := range expectedRefundCols {
		var exists bool
		err := pool.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM information_schema.columns
				WHERE table_name = 'marketplace_refunds' AND column_name = $1
			)
		`, col).Scan(&exists)
		if err != nil || !exists {
			t.Errorf("expected column %s to exist on marketplace_refunds", col)
		}
	}

	// 3. Validate columns created by Migration 22 on marketplace_shipment_items
	expectedShipItemCols := []string{"refunded_quantity", "refund_id"}
	for _, col := range expectedShipItemCols {
		var exists bool
		err := pool.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM information_schema.columns
				WHERE table_name = 'marketplace_shipment_items' AND column_name = $1
			)
		`, col).Scan(&exists)
		if err != nil || !exists {
			t.Errorf("expected column %s to exist on marketplace_shipment_items", col)
		}
	}

	// 4. Validate unique index on marketplace_shipments
	var indexExists bool
	err = pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM pg_indexes
			WHERE tablename = 'marketplace_shipments' AND indexname = 'uq_idx_mkt_shipments_order_business'
		)
	`).Scan(&indexExists)
	if err != nil || !indexExists {
		t.Errorf("expected index uq_idx_mkt_shipments_order_business to exist")
	}

	// 5. Test Rollback of Migration 22
	downSQL22 := `
		DROP INDEX IF EXISTS uq_idx_mkt_shipments_order_business;
		CREATE UNIQUE INDEX IF NOT EXISTS uq_idx_mkt_shipments_order_business
			ON marketplace_shipments(order_id, business_id);

		DROP INDEX IF EXISTS idx_mkt_shipment_items_refund;
		ALTER TABLE marketplace_shipment_items DROP COLUMN IF EXISTS refund_id;
		ALTER TABLE marketplace_shipment_items DROP COLUMN IF EXISTS refunded_quantity;

		DROP INDEX IF EXISTS idx_marketplace_refunds_order_item;
		DROP INDEX IF EXISTS idx_marketplace_refunds_business;
		DROP INDEX IF EXISTS idx_marketplace_refunds_shipment;

		ALTER TABLE marketplace_refunds DROP COLUMN IF EXISTS order_item_id;
		ALTER TABLE marketplace_refunds DROP COLUMN IF EXISTS business_id;
		ALTER TABLE marketplace_refunds DROP COLUMN IF EXISTS shipment_id;
	`
	if _, err := pool.Exec(ctx, downSQL22); err != nil {
		t.Fatalf("failed rolling back migration 22: %v", err)
	}
	if _, err := pool.Exec(ctx, "DELETE FROM goose_db_version WHERE version_id = 20261027000022;"); err != nil {
		t.Fatalf("failed removing version 22 from goose_db_version: %v", err)
	}

	// Verify columns are dropped
	var hasCol bool
	_ = pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_name = 'marketplace_refunds' AND column_name = 'order_item_id'
		)
	`).Scan(&hasCol)
	if hasCol {
		t.Errorf("expected order_item_id column to be dropped after rollback")
	}

	// 6. Test Forward Re-Application of Migration 22
	if err := migrator.RunMigrations(ctx, pool); err != nil {
		t.Fatalf("re-applying migration 22 failed: %v", err)
	}

	// Verify columns restored
	_ = pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_name = 'marketplace_refunds' AND column_name = 'order_item_id'
		)
	`).Scan(&hasCol)
	if !hasCol {
		t.Errorf("expected order_item_id column to exist after re-applying migration 22")
	}
}

func TestMigrations_HistoricalDataForwardApplicationAndLedgerConsistency(t *testing.T) {
	testDBURL := os.Getenv("TEST_DATABASE_URL")
	if testDBURL == "" {
		testDBURL = "postgres://postgres@127.0.0.1:5432/ratels_migration_audit_db?sslmode=disable"
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, testDBURL)
	if err != nil {
		t.Fatalf("failed connecting to migration test database: %v", err)
	}
	defer pool.Close()

	if err := migrator.RunMigrations(ctx, pool); err != nil {
		t.Fatalf("migrator.RunMigrations failed: %v", err)
	}

	// Verify inventory ledger invariants:
	// 1. balance_after must never be negative
	var negativeCount int
	err = pool.QueryRow(ctx, "SELECT count(*) FROM marketplace_inventory_logs WHERE balance_after < 0").Scan(&negativeCount)
	if err != nil || negativeCount > 0 {
		t.Errorf("Expected 0 negative ledger balances, got %d (err: %v)", negativeCount, err)
	}

	// 2. All products must have an opening balance ledger entry
	var unseededProducts int
	err = pool.QueryRow(ctx, `
		SELECT count(*) FROM marketplace_products p
		WHERE NOT EXISTS (
			SELECT 1 FROM marketplace_inventory_logs l WHERE l.product_id = p.id
		)
	`).Scan(&unseededProducts)
	if err != nil || unseededProducts > 0 {
		t.Errorf("Expected all products to have opening balance ledger logs, unseeded count: %d (err: %v)", unseededProducts, err)
	}

	// 3. Historical cancelled/refunded orders must have stock_released = TRUE
	var unreleasedCount int
	err = pool.QueryRow(ctx, `
		SELECT count(*) FROM marketplace_orders
		WHERE order_status IN ('cancelled', 'refunded') AND stock_released = FALSE
	`).Scan(&unreleasedCount)
	if err != nil || unreleasedCount > 0 {
		t.Errorf("Expected 0 historical cancelled/refunded orders with stock_released=FALSE, got %d", unreleasedCount)
	}

	// 4. Verify composite uniqueness on marketplace_shipments(order_id, business_id) NULLS NOT DISTINCT
	var duplicateShipments int
	err = pool.QueryRow(ctx, `
		SELECT count(*) FROM (
			SELECT order_id, business_id, count(*)
			FROM marketplace_shipments
			GROUP BY order_id, business_id
			HAVING count(*) > 1
		) dupes
	`).Scan(&duplicateShipments)
	if err != nil || duplicateShipments > 0 {
		t.Errorf("Expected 0 duplicate shipments per order-business pair, got %d", duplicateShipments)
	}
}
