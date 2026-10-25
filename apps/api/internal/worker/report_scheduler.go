package worker

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"ratels-api/internal/db"
	"ratels-api/internal/repository"
	"ratels-api/internal/service"
)

var (
	// ErrSchedulerAlreadyRunning indicates another process or cluster node holds the advisory lock.
	ErrSchedulerAlreadyRunning = errors.New("report scheduler cycle is already running on another node or process")
	// ErrNoAdminUserFound indicates no active administrator user exists to attribute the automated draft.
	ErrNoAdminUserFound = errors.New("no active administrator user found for automated report attribution")
)

// DefaultSchedulerLockKey is the 64-bit integer advisory lock key for ratels report scheduler (ASCII "RATELS_R").
const DefaultSchedulerLockKey int64 = 5927161816436154210

// ReportSchedulerConfig holds configuration parameters for the scheduled reports engine.
type ReportSchedulerConfig struct {
	PollInterval time.Duration
	LockKey      int64
	Enabled      bool
}

// DefaultReportSchedulerConfig returns standard defaults for the report scheduler.
func DefaultReportSchedulerConfig() ReportSchedulerConfig {
	return ReportSchedulerConfig{
		PollInterval: 1 * time.Hour,
		LockKey:      DefaultSchedulerLockKey,
		Enabled:      true,
	}
}

// SchedulerRunResult provides structured metrics and audit data from a scheduler execution cycle.
type SchedulerRunResult struct {
	PeriodLabel         string            `json:"period_label"`
	PeriodStart         string            `json:"period_start"`
	PeriodEnd           string            `json:"period_end"`
	ReportsEvaluated    int               `json:"reports_evaluated"`
	ReportsGenerated    int               `json:"reports_generated"`
	ReportsSkipped      int               `json:"reports_skipped"`
	GeneratedReferences []string          `json:"generated_references"`
	SkippedReasons      map[string]string `json:"skipped_reasons"`
	Errors              []string          `json:"errors"`
	LockAcquired        bool              `json:"lock_acquired"`
}

// ScheduledReportSpec defines a target periodic report for automated draft generation.
type ScheduledReportSpec struct {
	ReportType string
	ScopeType  string
	ZoneID     *string
	StateID    *string
	LgaID      *string
	Title      string
}

// ReportScheduler coordinates scheduled report generation with distributed locking and idempotency.
type ReportScheduler struct {
	repo          *repository.Repository
	reportService *service.ReportService
	config        ReportSchedulerConfig
	cancelFunc    context.CancelFunc
	wg            sync.WaitGroup
	running       bool
	mu            sync.Mutex
}

// NewReportScheduler initializes a new ReportScheduler instance.
func NewReportScheduler(
	repo *repository.Repository,
	reportService *service.ReportService,
	cfg *ReportSchedulerConfig,
) *ReportScheduler {
	config := DefaultReportSchedulerConfig()
	if cfg != nil {
		if cfg.PollInterval > 0 {
			config.PollInterval = cfg.PollInterval
		}
		if cfg.LockKey != 0 {
			config.LockKey = cfg.LockKey
		}
		config.Enabled = cfg.Enabled
	}

	return &ReportScheduler{
		repo:          repo,
		reportService: reportService,
		config:        config,
	}
}

// Start initiates the background periodic polling loop (when enabled).
func (s *ReportScheduler) Start(parentCtx context.Context) {
	s.mu.Lock()
	if s.running || !s.config.Enabled {
		s.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(parentCtx)
	s.cancelFunc = cancel
	s.running = true
	s.mu.Unlock()

	log.Printf("[REPORT_SCHEDULER] Started periodic reports scheduler (poll interval: %s)", s.config.PollInterval)

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(s.config.PollInterval)
		defer ticker.Stop()

		// Initial check on startup
		s.safeRunCycle(ctx)

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.safeRunCycle(ctx)
			}
		}
	}()
}

// Stop gracefully stops the background scheduler loop.
func (s *ReportScheduler) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.running = false
	if s.cancelFunc != nil {
		s.cancelFunc()
	}
	s.mu.Unlock()

	s.wg.Wait()
	log.Printf("[REPORT_SCHEDULER] Periodic reports scheduler stopped gracefully")
}

func (s *ReportScheduler) safeRunCycle(ctx context.Context) {
	res, err := s.RunScheduledCycle(ctx, nil, nil)
	if err != nil {
		if errors.Is(err, ErrSchedulerAlreadyRunning) {
			log.Printf("[REPORT_SCHEDULER] Cycle skipped: lock held by another process")
		} else {
			log.Printf("[REPORT_SCHEDULER] Cycle error: %v", err)
		}
		return
	}
	if res != nil && res.ReportsGenerated > 0 {
		log.Printf("[REPORT_SCHEDULER] Cycle complete: generated %d draft reports for period %s",
			res.ReportsGenerated, res.PeriodLabel)
	}
}

// RunScheduledCycle executes an idempotent automated snapshot generation cycle.
// If targetMonth is nil, defaults to the previous closed calendar month in UTC.
// Uses PostgreSQL transaction advisory lock to guarantee mutual exclusion across instances.
func (s *ReportScheduler) RunScheduledCycle(
	ctx context.Context,
	targetMonth *time.Time,
	actorUser *db.User,
) (*SchedulerRunResult, error) {
	// 1. Acquire distributed PostgreSQL transaction advisory lock
	tx, err := s.repo.Pool().Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin advisory lock transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var acquired bool
	err = tx.QueryRow(ctx, "SELECT pg_try_advisory_xact_lock($1)", s.config.LockKey).Scan(&acquired)
	if err != nil {
		return nil, fmt.Errorf("failed querying advisory lock: %w", err)
	}
	if !acquired {
		return &SchedulerRunResult{
			LockAcquired: false,
			Errors:       []string{ErrSchedulerAlreadyRunning.Error()},
		}, ErrSchedulerAlreadyRunning
	}

	// 2. Determine target reporting period in UTC with inclusive end-of-day boundary
	var pStart, pEnd time.Time
	var periodLabel string

	if targetMonth == nil {
		pStart, pEnd, periodLabel = CalculatePreviousMonthUTC(time.Now())
	} else {
		pStart, pEnd, periodLabel = CalculateMonthUTC(*targetMonth)
	}

	result := &SchedulerRunResult{
		PeriodLabel:         periodLabel,
		PeriodStart:         pStart.Format("2006-01-02"),
		PeriodEnd:           pEnd.Format("2006-01-02"),
		GeneratedReferences: make([]string, 0),
		SkippedReasons:      make(map[string]string),
		Errors:              make([]string, 0),
		LockAcquired:        true,
	}

	// 3. Resolve active system administrator user for attribution
	if actorUser == nil {
		adminUser, err := s.repo.Queries().GetFirstActiveAdminUser(ctx)
		if err != nil {
			return result, fmt.Errorf("%w: %v", ErrNoAdminUserFound, err)
		}
		actorUser = &adminUser
	}

	// 4. Standard national periodic report specifications
	specs := []ScheduledReportSpec{
		{
			ReportType: "national_overview",
			ScopeType:  "national",
			Title:      fmt.Sprintf("National Overview & Impact Summary - %s", periodLabel),
		},
		{
			ReportType: "cleanups",
			ScopeType:  "national",
			Title:      fmt.Sprintf("Environmental Cleanup & Impact Report - %s", periodLabel),
		},
		{
			ReportType: "participation",
			ScopeType:  "national",
			Title:      fmt.Sprintf("Verified Civic Participation Report - %s", periodLabel),
		},
		{
			ReportType: "finance",
			ScopeType:  "national",
			Title:      fmt.Sprintf("Financial Accountability & Ledger Disclosure - %s", periodLabel),
		},
		{
			ReportType: "foundation",
			ScopeType:  "national",
			Title:      fmt.Sprintf("Martins Vincent Otse Foundation Humanitarian Report - %s", periodLabel),
		},
		{
			ReportType: "community_projects",
			ScopeType:  "national",
			Title:      fmt.Sprintf("Community Development Projects Progress - %s", periodLabel),
		},
	}

	result.ReportsEvaluated = len(specs)

	// 5. Evaluate and generate drafts idempotently
	for _, spec := range specs {
		var zonePg, statePg, lgaPg pgtype.Text
		if spec.ZoneID != nil && *spec.ZoneID != "" {
			zonePg = pgtype.Text{String: *spec.ZoneID, Valid: true}
		}
		if spec.StateID != nil && *spec.StateID != "" {
			statePg = pgtype.Text{String: *spec.StateID, Valid: true}
		}
		if spec.LgaID != nil && *spec.LgaID != "" {
			lgaPg = pgtype.Text{String: *spec.LgaID, Valid: true}
		}

		// Idempotency check: verify whether an active snapshot already exists
		exists, err := s.repo.Queries().CheckReportSnapshotExists(ctx, db.CheckReportSnapshotExistsParams{
			ReportType:      spec.ReportType,
			ReportingPeriod: periodLabel,
			ScopeType:       spec.ScopeType,
			ZoneID:          zonePg,
			StateID:         statePg,
			LgaID:           lgaPg,
		})
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: failed checking existing snapshot: %v", spec.ReportType, err))
			continue
		}

		if exists > 0 {
			result.ReportsSkipped++
			result.SkippedReasons[spec.ReportType] = "active snapshot already exists for period and scope"
			continue
		}

		// Mandatory Governance Rule: Automated generation creates snapshots in 'draft' status ONLY.
		snap, err := s.reportService.GenerateSnapshot(ctx, actorUser, service.GenerateReportRequest{
			ReportType:      spec.ReportType,
			Title:           spec.Title,
			ReportingPeriod: periodLabel,
			PeriodStart:     pStart.Format("2006-01-02"),
			PeriodEnd:       pEnd.Format("2006-01-02"),
			ScopeType:       spec.ScopeType,
			ZoneID:          spec.ZoneID,
			StateID:         spec.StateID,
			LGAID:           spec.LgaID,
		}, nil)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: generation failed: %v", spec.ReportType, err))
			continue
		}

		// Security assertion: status must be 'draft'
		if snap.Status != "draft" {
			errMsg := fmt.Sprintf("%s: automated report was generated with non-draft status '%s'", spec.ReportType, snap.Status)
			result.Errors = append(result.Errors, errMsg)
			return result, errors.New(errMsg)
		}

		result.ReportsGenerated++
		result.GeneratedReferences = append(result.GeneratedReferences, snap.PublicReference)
	}

	// Commit transaction (releasing the advisory lock cleanly)
	_ = tx.Commit(ctx)
	return result, nil
}

// CalculatePreviousMonthUTC returns the UTC boundary [start of 1st day, end of last day] and YYYY-MM label for the previous month.
func CalculatePreviousMonthUTC(now time.Time) (time.Time, time.Time, string) {
	now = now.UTC()
	firstOfCurrentMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	prevMonthTarget := firstOfCurrentMonth.AddDate(0, -1, 0)
	pStart := time.Date(prevMonthTarget.Year(), prevMonthTarget.Month(), 1, 0, 0, 0, 0, time.UTC)
	lastDayOfPrev := firstOfCurrentMonth.AddDate(0, 0, -1)
	pEnd := time.Date(lastDayOfPrev.Year(), lastDayOfPrev.Month(), lastDayOfPrev.Day(), 23, 59, 59, 999999999, time.UTC)
	periodLabel := pStart.Format("2006-01")
	return pStart, pEnd, periodLabel
}

// CalculateMonthUTC returns the UTC boundary [start of 1st day, end of last day] and YYYY-MM label for a given month.
func CalculateMonthUTC(target time.Time) (time.Time, time.Time, string) {
	target = target.UTC()
	pStart := time.Date(target.Year(), target.Month(), 1, 0, 0, 0, 0, time.UTC)
	nextMonthFirst := pStart.AddDate(0, 1, 0)
	lastDay := nextMonthFirst.AddDate(0, 0, -1)
	pEnd := time.Date(lastDay.Year(), lastDay.Month(), lastDay.Day(), 23, 59, 59, 999999999, time.UTC)
	periodLabel := pStart.Format("2006-01")
	return pStart, pEnd, periodLabel
}
