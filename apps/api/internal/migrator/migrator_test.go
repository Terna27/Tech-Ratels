package migrator_test

import (
	"context"
	"os"
	"sync"
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

func TestMigrations_ConcurrentExecutionSafety(t *testing.T) {
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

	// Concurrently invoke RunMigrations from 5 simultaneous workers
	const numWorkers = 5
	errs := make([]error, numWorkers)
	var wg sync.WaitGroup
	startCh := make(chan struct{})

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerIdx int) {
			defer wg.Done()
			<-startCh // Synchronize start
			errs[workerIdx] = migrator.RunMigrations(ctx, pool)
		}(i)
	}

	// Trigger all workers at once
	close(startCh)
	wg.Wait()

	for i, wErr := range errs {
		if wErr != nil {
			t.Errorf("worker %d encountered error during concurrent RunMigrations: %v", i, wErr)
		}
	}

	// Verify database is consistent and goose_db_version has no duplicate version records
	var duplicateCount int
	err = pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM (
			SELECT version_id, COUNT(*)
			FROM goose_db_version
			WHERE is_applied = true
			GROUP BY version_id
			HAVING COUNT(*) > 1
		) dups
	`).Scan(&duplicateCount)
	if err != nil {
		t.Fatalf("failed checking for duplicate version records: %v", err)
	}
	if duplicateCount > 0 {
		t.Errorf("expected 0 duplicate version records in goose_db_version, found %d", duplicateCount)
	}
}
