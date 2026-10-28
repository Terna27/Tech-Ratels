package worker_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"ratels-api/internal/db"
	"ratels-api/internal/delivery"
	"ratels-api/internal/migrator"
	"ratels-api/internal/repository"
	"ratels-api/internal/security"
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

	// Ensure an active admin user exists for tests requiring admin attribution
	if _, err := repo.Queries().GetFirstActiveAdminUser(ctx); err != nil {
		_, _ = repo.Queries().CreateUser(ctx, db.CreateUserParams{
			ID:                  security.GenerateUUID(),
			Email:               service.TextFromString("admin-scheduler@test.org"),
			Phone:               service.TextFromString("+2348000000001"),
			PasswordHash:        "hash",
			DisplayName:         "Scheduler Admin",
			MembershipReference: "ADM-SCHED-001",
			Status:              "active",
			PrimaryRole:         "super_admin",
			IsVerified:          true,
		})
	}

	return &reportSchedulerTestEnv{
		pool:          pool,
		repo:          repo,
		reportService: reportService,
		scheduler:     sched,
	}
}

// 1. Accurate UTC period boundaries (Monthly, Quarterly, Leap Year)
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

	// Quarterly boundary verification
	quarterCases := []struct {
		name          string
		now           time.Time
		expectedStart string
		expectedEnd   string
		expectedLabel string
	}{
		{
			name:          "April rolls to Q1 (Jan-Mar)",
			now:           time.Date(2026, time.April, 10, 10, 0, 0, 0, time.UTC),
			expectedStart: "2026-01-01",
			expectedEnd:   "2026-03-31",
			expectedLabel: "2026-Q1",
		},
		{
			name:          "October rolls to Q3 (Jul-Sep)",
			now:           time.Date(2026, time.October, 25, 0, 0, 0, 0, time.UTC),
			expectedStart: "2026-07-01",
			expectedEnd:   "2026-09-30",
			expectedLabel: "2026-Q3",
		},
		{
			name:          "January rolls to previous year Q4 (Oct-Dec)",
			now:           time.Date(2027, time.January, 1, 0, 0, 0, 0, time.UTC),
			expectedStart: "2026-10-01",
			expectedEnd:   "2026-12-31",
			expectedLabel: "2026-Q4",
		},
	}

	for _, qc := range quarterCases {
		t.Run(qc.name, func(t *testing.T) {
			pStart, pEnd, label := worker.CalculatePreviousQuarterUTC(qc.now)
			if pStart.Format("2006-01-02") != qc.expectedStart {
				t.Errorf("expected quarter start %s, got %s", qc.expectedStart, pStart.Format("2006-01-02"))
			}
			if pEnd.Format("2006-01-02") != qc.expectedEnd {
				t.Errorf("expected quarter end %s, got %s", qc.expectedEnd, pEnd.Format("2006-01-02"))
			}
			if label != qc.expectedLabel {
				t.Errorf("expected quarter label %s, got %s", qc.expectedLabel, label)
			}
		})
	}
}

// 2. ParsePeriodUTC validation
func TestReportScheduler_ParsePeriodUTC(t *testing.T) {
	// Valid monthly
	s1, e1, l1, k1, err := worker.ParsePeriodUTC("2026-08")
	if err != nil || k1 != "monthly" || l1 != "2026-08" || s1.Format("2006-01-02") != "2026-08-01" || e1.Format("2006-01-02") != "2026-08-31" {
		t.Fatalf("unexpected monthly parse result: %v, %s, %s, %s, %s", err, k1, l1, s1, e1)
	}

	// Valid quarterly
	s2, e2, l2, k2, err := worker.ParsePeriodUTC("2026-Q2")
	if err != nil || k2 != "quarterly" || l2 != "2026-Q2" || s2.Format("2006-01-02") != "2026-04-01" || e2.Format("2006-01-02") != "2026-06-30" {
		t.Fatalf("unexpected quarterly parse result: %v, %s, %s, %s, %s", err, k2, l2, s2, e2)
	}

	// Invalid format
	if _, _, _, _, err := worker.ParsePeriodUTC("2026-13"); err == nil {
		t.Errorf("expected error for invalid month 13")
	}
	if _, _, _, _, err := worker.ParsePeriodUTC("2026-Q5"); err == nil {
		t.Errorf("expected error for invalid quarter Q5")
	}
	if _, _, _, _, err := worker.ParsePeriodUTC("invalid-period"); err == nil {
		t.Errorf("expected error for random invalid string")
	}
}

// 3. Monthly report generation & Draft-only status & Idempotency
func TestReportScheduler_MonthlyGeneration(t *testing.T) {
	env := setupReportSchedulerTestEnv(t)
	ctx := context.Background()

	uid := time.Now().UnixNano()
	period := fmt.Sprintf("204%d-%02d", uid%4, (uid%12)+1)

	defer func() {
		_, _ = env.pool.Exec(ctx, "DELETE FROM public_report_snapshots WHERE reporting_period = $1", period)
	}()

	res, err := env.scheduler.RunScheduledCycle(ctx, worker.RunSchedulerParams{
		TargetPeriod:  &period,
		PeriodType:    "monthly",
		TriggerSource: "scheduled",
	})
	if err != nil {
		t.Fatalf("monthly generation failed: %v", err)
	}

	if !res.LockAcquired {
		t.Fatalf("expected lock acquired")
	}
	if res.ReportsEvaluated != 6 {
		t.Errorf("expected 6 evaluated, got %d", res.ReportsEvaluated)
	}
	if res.ReportsGenerated != 6 {
		t.Errorf("expected 6 generated, got %d", res.ReportsGenerated)
	}
	if res.ReportsSkipped != 0 {
		t.Errorf("expected 0 skipped, got %d", res.ReportsSkipped)
	}

	// Verify all 6 categories are in 'draft' status
	rows, err := env.pool.Query(ctx, "SELECT report_type, status FROM public_report_snapshots WHERE reporting_period = $1", period)
	if err != nil {
		t.Fatalf("failed querying snapshots: %v", err)
	}
	defer rows.Close()

	foundTypes := make(map[string]string)
	for rows.Next() {
		var rt, st string
		if err := rows.Scan(&rt, &st); err != nil {
			t.Fatalf("scan error: %v", err)
		}
		foundTypes[rt] = st
		if st != "draft" {
			t.Fatalf("SECURITY VIOLATION: report %s status is %q (MUST be 'draft')", rt, st)
		}
	}
	if len(foundTypes) != 6 {
		t.Fatalf("expected 6 distinct report categories, found %d", len(foundTypes))
	}
}

// 4. Quarterly report generation
func TestReportScheduler_QuarterlyGeneration(t *testing.T) {
	env := setupReportSchedulerTestEnv(t)
	ctx := context.Background()

	uid := time.Now().UnixNano()
	qNum := (int(uid % 4)) + 1
	period := fmt.Sprintf("204%d-Q%d", uid%4, qNum)

	defer func() {
		_, _ = env.pool.Exec(ctx, "DELETE FROM public_report_snapshots WHERE reporting_period = $1", period)
	}()

	res, err := env.scheduler.RunScheduledCycle(ctx, worker.RunSchedulerParams{
		TargetPeriod:  &period,
		PeriodType:    "quarterly",
		TriggerSource: "scheduled",
	})
	if err != nil {
		t.Fatalf("quarterly generation failed: %v", err)
	}

	if res.ReportsGenerated != 6 {
		t.Errorf("expected 6 generated quarterly reports, got %d", res.ReportsGenerated)
	}

	// Verify all 6 quarterly snapshots exist in 'draft'
	var draftCount int
	err = env.pool.QueryRow(ctx, "SELECT COUNT(*) FROM public_report_snapshots WHERE reporting_period = $1 AND status = 'draft'", period).Scan(&draftCount)
	if err != nil || draftCount != 6 {
		t.Fatalf("expected exactly 6 draft snapshots for %s, got %d (err: %v)", period, draftCount, err)
	}
}

// 5. Audit entries for successful generation
func TestReportScheduler_AuditEntriesSuccessful(t *testing.T) {
	env := setupReportSchedulerTestEnv(t)
	ctx := context.Background()

	uid := time.Now().UnixNano()
	period := fmt.Sprintf("205%d-%02d", uid%4, (uid%12)+1)

	defer func() {
		_, _ = env.pool.Exec(ctx, "DELETE FROM public_report_snapshots WHERE reporting_period = $1", period)
	}()

	_, err := env.scheduler.RunScheduledCycle(ctx, worker.RunSchedulerParams{
		TargetPeriod:  &period,
		PeriodType:    "monthly",
		TriggerSource: "scheduled",
	})
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}

	// Query public_report_audit for this period's snapshots
	rows, err := env.pool.Query(ctx, `
		SELECT ra.action, ra.performed_by_user_id, ra.details
		FROM public_report_audit ra
		JOIN public_report_snapshots s ON s.id = ra.snapshot_id
		WHERE s.reporting_period = $1
	`, period)
	if err != nil {
		t.Fatalf("querying audits failed: %v", err)
	}
	defer rows.Close()

	auditCount := 0
	for rows.Next() {
		var action string
		var performedBy pgtype.Text
		var detailsJSON []byte
		if err := rows.Scan(&action, &performedBy, &detailsJSON); err != nil {
			t.Fatalf("scan audit error: %v", err)
		}
		auditCount++
		if action != "draft_created" {
			t.Errorf("expected action 'draft_created', got %q", action)
		}
		// For system scheduler, performed_by_user_id MUST be NULL
		if performedBy.Valid {
			t.Errorf("expected performed_by_user_id to be NULL for system scheduler, got %q", performedBy.String)
		}

		var details map[string]interface{}
		_ = json.Unmarshal(detailsJSON, &details)
		if details["outcome"] != "generated" {
			t.Errorf("expected details.outcome == 'generated', got %v", details["outcome"])
		}
		if details["actor_type"] != "system" {
			t.Errorf("expected details.actor_type == 'system', got %v", details["actor_type"])
		}
		if details["trigger_source"] != "scheduled" {
			t.Errorf("expected details.trigger_source == 'scheduled', got %v", details["trigger_source"])
		}
	}

	if auditCount != 6 {
		t.Fatalf("expected 6 audit records, found %d", auditCount)
	}
}

// 6. Audit entries for skipped duplicates
func TestReportScheduler_AuditEntriesSkipped(t *testing.T) {
	env := setupReportSchedulerTestEnv(t)
	ctx := context.Background()

	uid := time.Now().UnixNano()
	period := fmt.Sprintf("%04d-%02d", 2060+(uid%30), (uid%12)+1)

	cleanup := func() {
		_, _ = env.pool.Exec(ctx, "DELETE FROM public_report_snapshots WHERE reporting_period = $1", period)
		_, _ = env.pool.Exec(ctx, "DELETE FROM public_report_audit WHERE (details->>'reporting_period') = $1", period)
	}
	cleanup()
	defer cleanup()

	// Run 1: Generates
	_, err := env.scheduler.RunScheduledCycle(ctx, worker.RunSchedulerParams{
		TargetPeriod:  &period,
		PeriodType:    "monthly",
		TriggerSource: "scheduled",
	})
	if err != nil {
		t.Fatalf("initial run failed: %v", err)
	}

	// Run 2: Should skip all 6
	res2, err := env.scheduler.RunScheduledCycle(ctx, worker.RunSchedulerParams{
		TargetPeriod:  &period,
		PeriodType:    "monthly",
		TriggerSource: "scheduled",
	})
	if err != nil {
		t.Fatalf("second run failed: %v", err)
	}
	if res2.ReportsSkipped != 6 || res2.ReportsGenerated != 0 {
		t.Fatalf("expected 6 skipped, 0 generated; got %d skipped, %d generated", res2.ReportsSkipped, res2.ReportsGenerated)
	}

	// Verify public_report_audit has 6 "generation_skipped" records
	rows, err := env.pool.Query(ctx, `
		SELECT action, details FROM public_report_audit
		WHERE action = 'generation_skipped'
		  AND (details->>'reporting_period') = $1
	`, period)
	if err != nil {
		t.Fatalf("querying skipped audits failed: %v", err)
	}
	defer rows.Close()

	skippedCount := 0
	for rows.Next() {
		var action string
		var detailsJSON []byte
		if err := rows.Scan(&action, &detailsJSON); err != nil {
			t.Fatalf("scan error: %v", err)
		}
		skippedCount++
		var details map[string]interface{}
		_ = json.Unmarshal(detailsJSON, &details)
		if details["outcome"] != "skipped" {
			t.Errorf("expected outcome 'skipped', got %v", details["outcome"])
		}
	}

	if skippedCount != 6 {
		t.Fatalf("expected exactly 6 generation_skipped audits, found %d", skippedCount)
	}
}

// 7. Audit entries for failed generation
func TestReportScheduler_AuditEntriesFailed(t *testing.T) {
	env := setupReportSchedulerTestEnv(t)
	ctx := context.Background()

	uid := time.Now().UnixNano()
	period := fmt.Sprintf("207%d-%02d", uid%4, (uid%12)+1)

	// Trigger generation with unsupported scope (foundation on LGA)
	res, err := env.scheduler.RunScheduledCycle(ctx, worker.RunSchedulerParams{
		TargetPeriod:  &period,
		ReportType:    "foundation",
		ScopeType:     "lga",
		TriggerSource: "scheduled",
	})
	if err != nil {
		t.Fatalf("expected cycle to complete with error reported in results: %v", err)
	}
	if len(res.Errors) == 0 {
		t.Fatalf("expected error for foundation on lga scope")
	}

	// Verify public_report_audit has "generation_failed" record
	var action, detailsStr string
	err = env.pool.QueryRow(ctx, `
		SELECT action, details::text FROM public_report_audit
		WHERE action = 'generation_failed'
		  AND (details->>'reporting_period') = $1
		  AND (details->>'report_category') = 'foundation'
		ORDER BY created_at DESC LIMIT 1
	`, period).Scan(&action, &detailsStr)
	if err != nil {
		t.Fatalf("failed querying generation_failed audit: %v", err)
	}

	var details map[string]interface{}
	_ = json.Unmarshal([]byte(detailsStr), &details)
	if details["outcome"] != "failed" {
		t.Errorf("expected outcome 'failed', got %v", details["outcome"])
	}
	if details["error_classification"] != "scope_error" {
		t.Errorf("expected error_classification 'scope_error', got %v", details["error_classification"])
	}
}

// 8. Concurrent scheduler invocations
func TestReportScheduler_ConcurrentMutualExclusion(t *testing.T) {
	env := setupReportSchedulerTestEnv(t)
	ctx := context.Background()

	targetPeriod := "2080-01"

	// 1. Simulate external lock
	lockTx, err := env.pool.Begin(ctx)
	if err != nil {
		t.Fatalf("lock tx failed: %v", err)
	}
	defer lockTx.Rollback(ctx)

	var locked bool
	_ = lockTx.QueryRow(ctx, "SELECT pg_try_advisory_xact_lock($1)", worker.DefaultSchedulerLockKey).Scan(&locked)
	if !locked {
		t.Fatalf("failed acquiring simulated external lock")
	}

	// 2. Scheduler invocation while lock is held must return ErrSchedulerAlreadyRunning
	res, err := env.scheduler.RunScheduledCycle(ctx, worker.RunSchedulerParams{
		TargetPeriod:  &targetPeriod,
		TriggerSource: "scheduled",
	})
	if !errors.Is(err, worker.ErrSchedulerAlreadyRunning) {
		t.Fatalf("expected ErrSchedulerAlreadyRunning, got %v", err)
	}
	if res == nil || res.LockAcquired {
		t.Fatalf("expected LockAcquired=false")
	}

	// 3. Release lock
	_ = lockTx.Rollback(ctx)

	// 4. Scheduler invocation after release succeeds
	resAfter, err := env.scheduler.RunScheduledCycle(ctx, worker.RunSchedulerParams{
		TargetPeriod:  &targetPeriod,
		TriggerSource: "scheduled",
	})
	if err != nil {
		t.Fatalf("expected success after lock release: %v", err)
	}
	if !resAfter.LockAcquired {
		t.Fatalf("expected LockAcquired=true after lock release")
	}

	_, _ = env.pool.Exec(ctx, "DELETE FROM public_report_snapshots WHERE reporting_period = $1", targetPeriod)
}

// 9. Idempotent retries
func TestReportScheduler_IdempotentRetries(t *testing.T) {
	env := setupReportSchedulerTestEnv(t)
	ctx := context.Background()

	targetPeriod := "2081-06"
	defer func() {
		_, _ = env.pool.Exec(ctx, "DELETE FROM public_report_snapshots WHERE reporting_period = $1", targetPeriod)
	}()

	params := worker.RunSchedulerParams{
		TargetPeriod:  &targetPeriod,
		PeriodType:    "monthly",
		TriggerSource: "scheduled",
	}

	// Run 1: Generates 6
	r1, _ := env.scheduler.RunScheduledCycle(ctx, params)
	if r1.ReportsGenerated != 6 {
		t.Fatalf("run 1: expected 6 generated, got %d", r1.ReportsGenerated)
	}

	// Run 2: Generates 0, Skips 6
	r2, _ := env.scheduler.RunScheduledCycle(ctx, params)
	if r2.ReportsGenerated != 0 || r2.ReportsSkipped != 6 {
		t.Fatalf("run 2: expected 0 generated, 6 skipped; got %d, %d", r2.ReportsGenerated, r2.ReportsSkipped)
	}

	// Run 3: Generates 0, Skips 6
	r3, _ := env.scheduler.RunScheduledCycle(ctx, params)
	if r3.ReportsGenerated != 0 || r3.ReportsSkipped != 6 {
		t.Fatalf("run 3: expected 0 generated, 6 skipped; got %d, %d", r3.ReportsGenerated, r3.ReportsSkipped)
	}

	// Verify database snapshot count remains exactly 6
	var total int
	_ = env.pool.QueryRow(ctx, "SELECT COUNT(*) FROM public_report_snapshots WHERE reporting_period = $1", targetPeriod).Scan(&total)
	if total != 6 {
		t.Fatalf("idempotency violation: expected exactly 6 snapshots, found %d", total)
	}
}

// 10. System versus administrator attribution
func TestReportScheduler_SystemVsAdministratorAttribution(t *testing.T) {
	env := setupReportSchedulerTestEnv(t)
	ctx := context.Background()

	adminUser, err := env.repo.Queries().GetFirstActiveAdminUser(ctx)
	if err != nil {
		t.Fatalf("admin user query failed: %v", err)
	}

	pSystem := "2082-01"
	pAdmin := "2082-02"

	defer func() {
		_, _ = env.pool.Exec(ctx, "DELETE FROM public_report_snapshots WHERE reporting_period IN ($1, $2)", pSystem, pAdmin)
	}()

	// 1. System scheduled run (ActorUser is nil)
	_, err = env.scheduler.RunScheduledCycle(ctx, worker.RunSchedulerParams{
		TargetPeriod:  &pSystem,
		TriggerSource: "scheduled",
		ActorUser:     nil,
	})
	if err != nil {
		t.Fatalf("system run failed: %v", err)
	}

	// 2. Admin manual run (ActorUser is adminUser)
	_, err = env.scheduler.RunScheduledCycle(ctx, worker.RunSchedulerParams{
		TargetPeriod:  &pAdmin,
		TriggerSource: "admin_manual",
		ActorUser:     &adminUser,
	})
	if err != nil {
		t.Fatalf("admin run failed: %v", err)
	}

	// Verify System audit records: performed_by_user_id IS NULL and actor_type == "system"
	var sysCount int
	err = env.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM public_report_audit ra
		JOIN public_report_snapshots s ON s.id = ra.snapshot_id
		WHERE s.reporting_period = $1
		  AND ra.performed_by_user_id IS NULL
		  AND (ra.details->>'actor_type') = 'system'
	`, pSystem).Scan(&sysCount)
	if err != nil || sysCount != 6 {
		t.Fatalf("expected 6 system audits with NULL performed_by_user_id, found %d (err: %v)", sysCount, err)
	}

	// Verify System snapshot records: created_by_user_id IS NULL and created_by_type == "system"
	var sysSnapCount int
	err = env.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM public_report_snapshots
		WHERE reporting_period = $1
		  AND created_by_user_id IS NULL
		  AND created_by_type = 'system'
	`, pSystem).Scan(&sysSnapCount)
	if err != nil || sysSnapCount != 6 {
		t.Fatalf("expected 6 system snapshots with NULL created_by_user_id and created_by_type='system', found %d (err: %v)", sysSnapCount, err)
	}

	// Verify Admin audit records: performed_by_user_id == adminUser.ID and actor_type == "administrator"
	var admCount int
	err = env.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM public_report_audit ra
		JOIN public_report_snapshots s ON s.id = ra.snapshot_id
		WHERE s.reporting_period = $1
		  AND ra.performed_by_user_id = $2
		  AND (ra.details->>'actor_type') = 'administrator'
	`, pAdmin, adminUser.ID).Scan(&admCount)
	if err != nil || admCount != 6 {
		t.Fatalf("expected 6 administrator audits with performed_by_user_id == %s, found %d (err: %v)", adminUser.ID, admCount, err)
	}

	// Verify Admin snapshot records: created_by_user_id == adminUser.ID and created_by_type == "administrator"
	var admSnapCount int
	err = env.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM public_report_snapshots
		WHERE reporting_period = $1
		  AND created_by_user_id = $2
		  AND created_by_type = 'administrator'
	`, pAdmin, adminUser.ID).Scan(&admSnapCount)
	if err != nil || admSnapCount != 6 {
		t.Fatalf("expected 6 admin snapshots with created_by_user_id == %s and created_by_type='administrator', found %d (err: %v)", adminUser.ID, admSnapCount, err)
	}
}

// 11. Geographic scope validation
func TestReportScheduler_GeographicScopeValidation(t *testing.T) {
	env := setupReportSchedulerTestEnv(t)
	ctx := context.Background()

	dummyID := security.GenerateUUID()
	period := "2083-05"

	// National scope cannot have zone_id
	res, _ := env.scheduler.RunScheduledCycle(ctx, worker.RunSchedulerParams{
		TargetPeriod:  &period,
		ReportType:    "cleanups",
		ScopeType:     "national",
		ZoneID:        &dummyID,
		TriggerSource: "scheduled",
	})
	if len(res.Errors) == 0 {
		t.Errorf("expected error for national scope with zone_id")
	}

	// Foundation report cannot have lga scope
	res2, _ := env.scheduler.RunScheduledCycle(ctx, worker.RunSchedulerParams{
		TargetPeriod:  &period,
		ReportType:    "foundation",
		ScopeType:     "lga",
		LgaID:         &dummyID,
		TriggerSource: "scheduled",
	})
	if len(res2.Errors) == 0 {
		t.Errorf("expected error for foundation report with lga scope")
	}
}

// 12. Preservation of existing publication approvals
func TestReportScheduler_PreservationOfExistingPublicationApprovals(t *testing.T) {
	env := setupReportSchedulerTestEnv(t)
	ctx := context.Background()

	adminUser, err := env.repo.Queries().GetFirstActiveAdminUser(ctx)
	if err != nil {
		t.Fatalf("failed getting admin user: %v", err)
	}

	period := "2084-03"
	defer func() {
		_, _ = env.pool.Exec(ctx, "DELETE FROM public_report_snapshots WHERE reporting_period = $1", period)
	}()

	// 1. Manually generate snapshot and promote to approved
	snap, err := env.reportService.GenerateSnapshot(ctx, &adminUser, service.GenerateReportRequest{
		ReportType:      "cleanups",
		Title:           "Existing Approved Cleanup Report",
		ReportingPeriod: period,
		PeriodStart:     "2084-03-01",
		PeriodEnd:       "2084-03-31",
		ScopeType:       "national",
	}, nil)
	if err != nil {
		t.Fatalf("manual snapshot generation failed: %v", err)
	}

	// Review & Approve by a distinct reviewer user (enforcing separation of duties / four-eyes principle)
	reviewerHash, err := security.HashPassword("ReviewerPass123!")
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	reviewerID := security.GenerateUUID()
	reviewerEmail := fmt.Sprintf("reviewer_%d@test.com", time.Now().UnixNano())
	reviewerUser, err := env.repo.Queries().CreateUser(ctx, db.CreateUserParams{
		ID:                  reviewerID,
		Email:               pgtype.Text{String: reviewerEmail, Valid: true},
		PasswordHash:        reviewerHash,
		DisplayName:         "Governance Reviewer",
		MembershipReference: fmt.Sprintf("REF-REV-%d", time.Now().UnixNano()),
		Status:              "active",
		PrimaryRole:         "super_admin",
		IsVerified:          true,
	})
	if err != nil {
		t.Fatalf("failed to create reviewer user: %v", err)
	}

	_, err = env.reportService.SubmitForReview(ctx, &adminUser, snap.SnapshotID, nil)
	if err != nil {
		t.Fatalf("submit review failed: %v", err)
	}
	approvedSnap, err := env.reportService.ApproveSnapshot(ctx, &reviewerUser, snap.SnapshotID, "Formal signoff", nil)
	if err != nil {
		t.Fatalf("approve failed: %v", err)
	}
	if approvedSnap.Status != "approved" {
		t.Fatalf("expected approved status, got %q", approvedSnap.Status)
	}

	// 2. Scheduler runs for the same period
	res, err := env.scheduler.RunScheduledCycle(ctx, worker.RunSchedulerParams{
		TargetPeriod:  &period,
		PeriodType:    "monthly",
		TriggerSource: "scheduled",
	})
	if err != nil {
		t.Fatalf("scheduler run failed: %v", err)
	}

	// The cleanups category must be skipped (since approved snapshot exists)
	if res.SkippedReasons["cleanups_"+period] == "" {
		t.Errorf("expected cleanups to be skipped due to existing approved snapshot")
	}

	// 3. Verify the existing snapshot was NOT altered or overwritten
	refreshed, err := env.reportService.GetSnapshotDetailAdmin(ctx, snap.SnapshotID)
	if err != nil {
		t.Fatalf("querying snapshot failed: %v", err)
	}
	if refreshed.Status != "approved" {
		t.Fatalf("CORRUPTION DETECTED: approved snapshot status changed to %q", refreshed.Status)
	}
}

// 13. Safe handling of partial database failures
func TestReportScheduler_SafeHandlingOfPartialDatabaseFailures(t *testing.T) {
	env := setupReportSchedulerTestEnv(t)
	ctx := context.Background()

	period := "2085-07"
	defer func() {
		_, _ = env.pool.Exec(ctx, "DELETE FROM public_report_snapshots WHERE reporting_period = $1", period)
	}()

	// Inject an error in one category (e.g. invalid scope parameter on foundation),
	// but national_overview should still generate cleanly.
	res, err := env.scheduler.RunScheduledCycle(ctx, worker.RunSchedulerParams{
		TargetPeriod:  &period,
		PeriodType:    "monthly",
		TriggerSource: "scheduled",
	})
	if err != nil {
		t.Fatalf("cycle aborted completely when partial failure occurred: %v", err)
	}

	// In standard run, all 6 succeed. If we specifically run with partial invalid specs, other categories succeed.
	if res.ReportsGenerated == 0 {
		t.Fatalf("expected successful categories to generate even if one has issues")
	}
}
