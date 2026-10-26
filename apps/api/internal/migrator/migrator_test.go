package migrator_test

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"ratels-api/internal/migrator"
)

func TestMigrations_LifecycleRollbackAndRecovery(t *testing.T) {
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

	// 1. Apply migrations 1 through 17
	if err := migrator.RunMigrations(ctx, pool); err != nil {
		t.Fatalf("initial migration application failed: %v", err)
	}

	// 2. Validate Migration 16 effects
	var isNullable string
	err = pool.QueryRow(ctx, `
		SELECT is_nullable 
		FROM information_schema.columns 
		WHERE table_name = 'public_report_snapshots' AND column_name = 'created_by_user_id'
	`).Scan(&isNullable)
	if err != nil {
		t.Fatalf("failed querying column info for created_by_user_id: %v", err)
	}
	if isNullable != "YES" {
		t.Errorf("expected created_by_user_id to be nullable (YES), got %s", isNullable)
	}

	var hasCreatedByType bool
	err = pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns 
			WHERE table_name = 'public_report_snapshots' AND column_name = 'created_by_type'
		)
	`).Scan(&hasCreatedByType)
	if err != nil || !hasCreatedByType {
		t.Errorf("expected created_by_type column to exist on public_report_snapshots")
	}

	// 3. Validate Migration 17 effects
	var hasSchedRunsTable bool
	err = pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables 
			WHERE table_name = 'public_report_scheduler_runs'
		)
	`).Scan(&hasSchedRunsTable)
	if err != nil || !hasSchedRunsTable {
		t.Errorf("expected public_report_scheduler_runs table to exist")
	}

	// 4. Test Rollback of Migration 17
	_, err = pool.Exec(ctx, "DROP TABLE IF EXISTS public_report_scheduler_runs;")
	if err != nil {
		t.Fatalf("failed rolling back migration 17: %v", err)
	}
	_, err = pool.Exec(ctx, "DELETE FROM goose_db_version WHERE version_id = 20261026000017;")
	if err != nil {
		t.Fatalf("failed updating goose_db_version for 17 rollback: %v", err)
	}

	// 5. Test Rollback of Migration 16
	downSQL16 := `
		DROP INDEX IF EXISTS idx_report_exports_snapshot_format;
		CREATE INDEX IF NOT EXISTS idx_report_exports_snapshot ON public_report_exports(snapshot_id, export_format);
		ALTER TABLE public_report_exports DROP COLUMN IF EXISTS updated_at;
		ALTER TABLE public_report_exports DROP COLUMN IF EXISTS checksum_algorithm;
		ALTER TABLE public_report_exports DROP COLUMN IF EXISTS mime_type;
		ALTER TABLE public_report_exports DROP COLUMN IF EXISTS file_size_bytes;
		DROP INDEX IF EXISTS idx_report_snapshots_created_by_type;
		ALTER TABLE public_report_snapshots DROP COLUMN IF EXISTS created_by_type;
	`
	if _, err := pool.Exec(ctx, downSQL16); err != nil {
		t.Fatalf("failed rolling back migration 16: %v", err)
	}
	if _, err := pool.Exec(ctx, "DELETE FROM goose_db_version WHERE version_id = 20261025000016;"); err != nil {
		t.Fatalf("failed updating goose_db_version for 16 rollback: %v", err)
	}

	// Verify columns are gone
	var colCount int
	err = pool.QueryRow(ctx, `
		SELECT count(*) 
		FROM information_schema.columns 
		WHERE table_name = 'public_report_exports' AND column_name IN ('file_size_bytes', 'mime_type', 'checksum_algorithm')
	`).Scan(&colCount)
	if err != nil || colCount != 0 {
		t.Errorf("expected 0 added columns after rollback, got %d", colCount)
	}

	// 6. Test Re-application of Migrations 16 and 17 (Recovery)
	if err := migrator.RunMigrations(ctx, pool); err != nil {
		t.Fatalf("re-applying migrations failed: %v", err)
	}

	// 7. Test Idempotency
	if err := migrator.RunMigrations(ctx, pool); err != nil {
		t.Fatalf("second run of migrator (idempotency check) failed: %v", err)
	}
}
