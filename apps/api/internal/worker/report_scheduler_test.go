package worker_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"ratels-api/internal/delivery"
	"ratels-api/internal/migrator"
	"ratels-api/internal/repository"
	"ratels-api/internal/service"
	"ratels-api/internal/worker"
)

type reportSchedulerTestEnv struct {
	pool          *pgxpool.Pool
	repo          *repository.Repository
	reportService *service.ReportService
	scheduler     *worker.ReportScheduler
}

func setupReportSchedulerTestEnv(t *testing.T) *reportSchedulerTestEnv {
	t.Helper()
	testDBURL := os.Getenv("TEST_DATABASE_URL")
	if testDBURL == "" {
		testDBURL = "postgres://postgres@127.0.0.1:5432/ratels_test?sslmode=disable"
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, testDBURL)
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}

	_ = migrator.RunMigrations(ctx, pool)
	repo := repository.NewRepository(pool)
	auditSvc := service.NewAuditService(repo)
	emailSender := delivery.NewDevSafeEmailSender()
	smsSender := delivery.NewDevSafeSMSSender()
	notifSvc := service.NewNotificationService(repo, auditSvc, emailSender, smsSender, "http://localhost:3000")
	reportService := service.NewReportService(repo, notifSvc, auditSvc)

	cfg := worker.DefaultReportSchedulerConfig()
	sched := worker.NewReportScheduler(repo, reportService, &cfg)

	return &reportSchedulerTestEnv{
		pool:          pool,
		repo:          repo,
		reportService: reportService,
		scheduler:     sched,
	}
}

func TestReportScheduler_PeriodBoundaryUTC(t *testing.T) {
	testCases := []struct {
		name          string
		now           time.Time
		expectedStart string
		expectedEnd   string
		expectedLabel string
	}{
		{
			name:          "October mid-month rolls to September",
			now:           time.Date(2026, time.October, 25, 14, 30, 0, 0, time.UTC),
			expectedStart: "2026-09-01",
			expectedEnd:   "2026-09-30",
			expectedLabel: "2026-09",
		},
		{
			name:          "January mid-month rolls to previous year December",
			now:           time.Date(2027, time.January, 15, 8, 0, 0, 0, time.UTC),
			expectedStart: "2026-12-01",
			expectedEnd:   "2026-12-31",
			expectedLabel: "2026-12",
		},
		{
			name:          "March in leap year 2028 rolls to 29-day February",
			now:           time.Date(2028, time.March, 10, 12, 0, 0, 0, time.UTC),
			expectedStart: "2028-02-01",
			expectedEnd:   "2028-02-29",
			expectedLabel: "2028-02",
		},
		{
			name:          "March in non-leap year 2027 rolls to 28-day February",
			now:           time.Date(2027, time.March, 1, 0, 0, 0, 0, time.UTC),
			expectedStart: "2027-02-01",
			expectedEnd:   "2027-02-28",
			expectedLabel: "2027-02",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pStart, pEnd, label := worker.CalculatePreviousMonthUTC(tc.now)

			if pStart.Location() != time.UTC {
				t.Fatalf("expected pStart in UTC, got %v", pStart.Location())
			}
			if pEnd.Location() != time.UTC {
				t.Fatalf("expected pEnd in UTC, got %v", pEnd.Location())
			}
			if pStart.Format("2006-01-02") != tc.expectedStart {
				t.Errorf("expected start %s, got %s", tc.expectedStart, pStart.Format("2006-01-02"))
			}
			if pEnd.Format("2006-01-02") != tc.expectedEnd {
				t.Errorf("expected end %s, got %s", tc.expectedEnd, pEnd.Format("2006-01-02"))
			}
			if pEnd.Hour() != 23 || pEnd.Minute() != 59 || pEnd.Second() != 59 {
				t.Errorf("expected inclusive end-of-day boundary (23:59:59), got %02d:%02d:%02d",
					pEnd.Hour(), pEnd.Minute(), pEnd.Second())
			}
			if label != tc.expectedLabel {
				t.Errorf("expected label %s, got %s", tc.expectedLabel, label)
			}
		})
	}
}

func TestReportScheduler_DraftStatusAndIdempotency(t *testing.T) {
	env := setupReportSchedulerTestEnv(t)
	ctx := context.Background()

	// Isolated dynamic test period
	targetYear := 2035 + int(time.Now().UnixNano()%5)
	targetMonthNum := 1 + int(time.Now().UnixNano()%12)
	targetDate := time.Date(targetYear, time.Month(targetMonthNum), 15, 0, 0, 0, 0, time.UTC)
	expectedPeriod := targetDate.Format("2006-01")

	// Ensure clean slate for test period
	_, _ = env.pool.Exec(ctx, "DELETE FROM public_report_snapshots WHERE reporting_period = $1", expectedPeriod)

	// Cycle 1: Should evaluate 6 reports and generate all 6 in 'draft' status
	res1, err := env.scheduler.RunScheduledCycle(ctx, &targetDate, nil)
	if err != nil {
		t.Fatalf("Cycle 1 failed unexpectedly: %v", err)
	}

	if !res1.LockAcquired {
		t.Fatalf("expected lock to be acquired in cycle 1")
	}
	if res1.PeriodLabel != expectedPeriod {
		t.Fatalf("expected period label %s, got %s", expectedPeriod, res1.PeriodLabel)
	}
	if res1.ReportsEvaluated != 6 {
		t.Errorf("expected 6 evaluated reports, got %d", res1.ReportsEvaluated)
	}
	if res1.ReportsGenerated != 6 {
		t.Errorf("expected 6 generated reports, got %d", res1.ReportsGenerated)
	}
	if res1.ReportsSkipped != 0 {
		t.Errorf("expected 0 skipped reports in cycle 1, got %d", res1.ReportsSkipped)
	}
	if len(res1.GeneratedReferences) != 6 {
		t.Errorf("expected 6 references, got %d", len(res1.GeneratedReferences))
	}

	// Verify all created snapshots are strictly in 'draft' status
	rows, err := env.pool.Query(ctx, "SELECT id, report_type, status FROM public_report_snapshots WHERE reporting_period = $1", expectedPeriod)
	if err != nil {
		t.Fatalf("failed querying generated snapshots: %v", err)
	}
	defer rows.Close()

	snapshotCount := 0
	for rows.Next() {
		var id, reportType, status string
		if err := rows.Scan(&id, &reportType, &status); err != nil {
			t.Fatalf("failed scanning row: %v", err)
		}
		snapshotCount++
		if status != "draft" {
			t.Fatalf("SECURITY VIOLATION: report %s (%s) created with status %q; MUST be 'draft'", id, reportType, status)
		}
	}
	if snapshotCount != 6 {
		t.Fatalf("expected exactly 6 snapshots in database, got %d", snapshotCount)
	}

	// Cycle 2 (Idempotency): Running again for the exact same target month must generate 0 and skip 6
	res2, err := env.scheduler.RunScheduledCycle(ctx, &targetDate, nil)
	if err != nil {
		t.Fatalf("Cycle 2 failed unexpectedly: %v", err)
	}

	if res2.ReportsGenerated != 0 {
		t.Errorf("expected 0 reports generated on rerun (idempotent), got %d", res2.ReportsGenerated)
	}
	if res2.ReportsSkipped != 6 {
		t.Errorf("expected 6 reports skipped on rerun (idempotent), got %d", res2.ReportsSkipped)
	}

	// Clean up test period
	_, _ = env.pool.Exec(ctx, "DELETE FROM public_report_snapshots WHERE reporting_period = $1", expectedPeriod)
}

func TestReportScheduler_AdvisoryLockMutualExclusion(t *testing.T) {
	env := setupReportSchedulerTestEnv(t)
	ctx := context.Background()

	targetDate := time.Date(2039, time.January, 15, 0, 0, 0, 0, time.UTC)
	expectedPeriod := "2039-01"

	// 1. Open an independent transaction holding the advisory lock simulating a concurrent worker
	lockTx, err := env.pool.Begin(ctx)
	if err != nil {
		t.Fatalf("failed to open lock tx: %v", err)
	}
	defer lockTx.Rollback(ctx)

	var locked bool
	err = lockTx.QueryRow(ctx, "SELECT pg_try_advisory_xact_lock($1)", worker.DefaultSchedulerLockKey).Scan(&locked)
	if err != nil || !locked {
		t.Fatalf("failed acquiring simulated external lock: %v (locked=%v)", err, locked)
	}

	// 2. Scheduler attempts to run while external lock is active
	res, err := env.scheduler.RunScheduledCycle(ctx, &targetDate, nil)
	if err == nil {
		t.Fatalf("expected ErrSchedulerAlreadyRunning error, got nil")
	}
	if res == nil || res.LockAcquired {
		t.Fatalf("expected LockAcquired=false, got %v", res)
	}

	// 3. Release external lock
	_ = lockTx.Rollback(ctx)

	// Clean slate
	_, _ = env.pool.Exec(ctx, "DELETE FROM public_report_snapshots WHERE reporting_period = $1", expectedPeriod)

	// 4. Scheduler attempts to run again after lock release
	resAfter, err := env.scheduler.RunScheduledCycle(ctx, &targetDate, nil)
	if err != nil {
		t.Fatalf("expected scheduler to succeed after lock release, got: %v", err)
	}
	if !resAfter.LockAcquired {
		t.Fatalf("expected lock to be acquired after release")
	}
	if resAfter.ReportsGenerated != 6 {
		t.Errorf("expected 6 reports generated after lock release, got %d", resAfter.ReportsGenerated)
	}

	// Clean up test period
	_, _ = env.pool.Exec(ctx, "DELETE FROM public_report_snapshots WHERE reporting_period = $1", expectedPeriod)
}
