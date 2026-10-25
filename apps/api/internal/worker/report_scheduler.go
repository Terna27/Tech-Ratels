package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"ratels-api/internal/db"
	"ratels-api/internal/repository"
	"ratels-api/internal/security"
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

// PeriodRunSummary details outcomes for a specific reporting period window.
type PeriodRunSummary struct {
	PeriodLabel         string   `json:"period_label"`
	PeriodType          string   `json:"period_type"` // "monthly", "quarterly"
	PeriodStart         string   `json:"period_start"`
	PeriodEnd           string   `json:"period_end"`
	ReportsEvaluated    int      `json:"reports_evaluated"`
	ReportsGenerated    int      `json:"reports_generated"`
	ReportsSkipped      int      `json:"reports_skipped"`
	GeneratedReferences []string `json:"generated_references"`
}

// SchedulerRunResult provides structured metrics and audit data from a scheduler execution cycle.
type SchedulerRunResult struct {
	PeriodLabel         string                      `json:"period_label"`
	PeriodStart         string                      `json:"period_start"`
	PeriodEnd           string                      `json:"period_end"`
	PeriodsEvaluated    []string                    `json:"periods_evaluated,omitempty"`
	PeriodSummaries     map[string]PeriodRunSummary `json:"period_summaries,omitempty"`
	ReportsEvaluated    int                         `json:"reports_evaluated"`
	ReportsGenerated    int                         `json:"reports_generated"`
	ReportsSkipped      int                         `json:"reports_skipped"`
	GeneratedReferences []string                    `json:"generated_references"`
	SkippedReasons      map[string]string           `json:"skipped_reasons"`
	Errors              []string                    `json:"errors"`
	LockAcquired        bool                        `json:"lock_acquired"`
}

// RunSchedulerParams encapsulates execution parameters for automated or manually-triggered report cycles.
type RunSchedulerParams struct {
	TargetPeriod  *string    `json:"target_period,omitempty"`  // e.g. "2026-09" or "2026-Q3"
	TargetMonth   *time.Time `json:"target_month,omitempty"`   // convenient time.Time parameter
	PeriodType    string     `json:"period_type,omitempty"`    // "monthly", "quarterly", or "both"
	TriggerSource string     `json:"trigger_source,omitempty"` // "scheduled", "external_cron", "admin_manual"
	ActorUser     *db.User   `json:"actor_user,omitempty"`     // nil for system attribution
	ReportType    string     `json:"report_type,omitempty"`    // optional single category filter
	ScopeType     string     `json:"scope_type,omitempty"`     // optional: "national", "zone", "state", "lga" (defaults to "national")
	ZoneID        *string    `json:"zone_id,omitempty"`
	StateID       *string    `json:"state_id,omitempty"`
	LgaID         *string    `json:"lga_id,omitempty"`
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

// ReportScheduler coordinates scheduled report generation with distributed locking, idempotency, and audit tracking.
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
	res, err := s.RunScheduledCycle(ctx, RunSchedulerParams{
		TriggerSource: "scheduled",
		PeriodType:    "both",
	})
	if err != nil {
		if errors.Is(err, ErrSchedulerAlreadyRunning) {
			log.Printf("[REPORT_SCHEDULER] Cycle skipped: lock held by another process")
		} else {
			log.Printf("[REPORT_SCHEDULER] Cycle error: %v", err)
		}
		return
	}
	if res != nil && res.ReportsGenerated > 0 {
		log.Printf("[REPORT_SCHEDULER] Cycle complete: generated %d draft reports across %v",
			res.ReportsGenerated, res.PeriodsEvaluated)
	}
}

// RunScheduledCycleLegacy maintains backwards compatibility for callers passing (ctx, targetMonth, actorUser).
func (s *ReportScheduler) RunScheduledCycleLegacy(
	ctx context.Context,
	targetMonth *time.Time,
	actorUser *db.User,
) (*SchedulerRunResult, error) {
	triggerSource := "scheduled"
	if actorUser != nil {
		triggerSource = "admin_manual"
	}
	return s.RunScheduledCycle(ctx, RunSchedulerParams{
		TargetMonth:   targetMonth,
		PeriodType:    "monthly",
		TriggerSource: triggerSource,
		ActorUser:     actorUser,
	})
}

// targetPeriodWindow represents an evaluated reporting time frame in UTC.
type targetPeriodWindow struct {
	pStart      time.Time
	pEnd        time.Time
	periodLabel string
	periodKind  string // "monthly" or "quarterly"
}

// RunScheduledCycle executes an idempotent automated snapshot generation cycle with full audit tracking.
// Uses PostgreSQL transaction advisory lock to guarantee mutual exclusion across instances.
func (s *ReportScheduler) RunScheduledCycle(
	ctx context.Context,
	params RunSchedulerParams,
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

	// 2. Resolve Trigger Source & Attribution
	triggerSource := strings.TrimSpace(params.TriggerSource)
	if triggerSource == "" {
		if params.ActorUser != nil {
			triggerSource = "admin_manual"
		} else {
			triggerSource = "scheduled"
		}
	}
	isSystemActor := (params.ActorUser == nil)

	var auditUserPg pgtype.Text
	var actorType, actorIdentity string
	var snapshotAdminUser *db.User

	if isSystemActor {
		actorType = "system"
		actorIdentity = "system:" + triggerSource
		auditUserPg = pgtype.Text{Valid: false}
		snapshotAdminUser = nil
	} else {
		actorType = "administrator"
		actorIdentity = params.ActorUser.DisplayName
		if params.ActorUser.Email.Valid && params.ActorUser.Email.String != "" {
			actorIdentity = params.ActorUser.Email.String
		}
		auditUserPg = pgtype.Text{String: params.ActorUser.ID, Valid: true}
		snapshotAdminUser = params.ActorUser
	}

	// 3. Determine target reporting period windows in UTC with inclusive end-of-day boundaries
	var windows []targetPeriodWindow

	if params.TargetMonth != nil {
		pStart, pEnd, label := CalculateMonthUTC(*params.TargetMonth)
		windows = append(windows, targetPeriodWindow{
			pStart:      pStart,
			pEnd:        pEnd,
			periodLabel: label,
			periodKind:  "monthly",
		})
	} else if params.TargetPeriod != nil && strings.TrimSpace(*params.TargetPeriod) != "" {
		pStart, pEnd, label, kind, err := ParsePeriodUTC(*params.TargetPeriod)
		if err != nil {
			return nil, fmt.Errorf("invalid target period: %w", err)
		}
		windows = append(windows, targetPeriodWindow{
			pStart:      pStart,
			pEnd:        pEnd,
			periodLabel: label,
			periodKind:  kind,
		})
	} else {
		pType := strings.ToLower(strings.TrimSpace(params.PeriodType))
		now := time.Now().UTC()

		if pType == "" || pType == "monthly" || pType == "both" {
			pStartM, pEndM, labelM := CalculatePreviousMonthUTC(now)
			windows = append(windows, targetPeriodWindow{
				pStart:      pStartM,
				pEnd:        pEndM,
				periodLabel: labelM,
				periodKind:  "monthly",
			})
		}
		if pType == "" || pType == "quarterly" || pType == "both" {
			pStartQ, pEndQ, labelQ := CalculatePreviousQuarterUTC(now)
			windows = append(windows, targetPeriodWindow{
				pStart:      pStartQ,
				pEnd:        pEndQ,
				periodLabel: labelQ,
				periodKind:  "quarterly",
			})
		}
	}

	result := &SchedulerRunResult{
		PeriodsEvaluated:    make([]string, 0, len(windows)),
		PeriodSummaries:     make(map[string]PeriodRunSummary),
		GeneratedReferences: make([]string, 0),
		SkippedReasons:      make(map[string]string),
		Errors:              make([]string, 0),
		LockAcquired:        true,
	}

	if len(windows) == 0 {
		_ = tx.Commit(ctx)
		return result, nil
	}

	// Populate top-level period fields for backward compatibility
	result.PeriodLabel = windows[0].periodLabel
	result.PeriodStart = windows[0].pStart.Format("2006-01-02")
	result.PeriodEnd = windows[len(windows)-1].pEnd.Format("2006-01-02")
	if len(windows) > 1 {
		labels := make([]string, len(windows))
		for i, w := range windows {
			labels[i] = w.periodLabel
		}
		result.PeriodLabel = strings.Join(labels, ", ")
	}

	for _, w := range windows {
		result.PeriodsEvaluated = append(result.PeriodsEvaluated, w.periodLabel)
		result.PeriodSummaries[w.periodLabel] = PeriodRunSummary{
			PeriodLabel:         w.periodLabel,
			PeriodType:          w.periodKind,
			PeriodStart:         w.pStart.Format("2006-01-02"),
			PeriodEnd:           w.pEnd.Format("2006-01-02"),
			GeneratedReferences: make([]string, 0),
		}
	}

	// 4. Determine Geographic Scope and Specifications
	scopeType := strings.TrimSpace(params.ScopeType)
	if scopeType == "" {
		scopeType = "national"
	}

	baseSpecs := []struct {
		reportType string
		titleTmpl  string
	}{
		{"national_overview", "National Overview & Impact Summary - %s"},
		{"cleanups", "Environmental Cleanup & Impact Report - %s"},
		{"participation", "Verified Civic Participation Report - %s"},
		{"finance", "Financial Accountability & Ledger Disclosure - %s"},
		{"foundation", "Martins Vincent Otse Foundation Humanitarian Report - %s"},
		{"community_projects", "Community Development Projects Progress - %s"},
	}

	// Filter single report category if requested
	if params.ReportType != "" {
		filtered := make([]struct {
			reportType string
			titleTmpl  string
		}, 0)
		for _, b := range baseSpecs {
			if b.reportType == params.ReportType {
				filtered = append(filtered, b)
				break
			}
		}
		if len(filtered) == 0 {
			return nil, fmt.Errorf("%w: %q", service.ErrInvalidReportScope, params.ReportType)
		}
		baseSpecs = filtered
	}

	// 5. Evaluate and generate drafts idempotently with audit tracking
	for _, w := range windows {
		pSummary := result.PeriodSummaries[w.periodLabel]

		for _, b := range baseSpecs {
			// LGA scope restriction on foundation reports
			if b.reportType == "foundation" && scopeType == "lga" {
				errReason := "foundation reports do not support lga scope"
				result.Errors = append(result.Errors, fmt.Sprintf("%s: %s", b.reportType, errReason))
				s.logSchedulerAudit(ctx, nil, "generation_failed", auditUserPg, map[string]interface{}{
					"report_category":      b.reportType,
					"reporting_period":     w.periodLabel,
					"scope_type":           scopeType,
					"trigger_source":       triggerSource,
					"actor_type":           actorType,
					"actor_identity":       actorIdentity,
					"outcome":              "failed",
					"error_classification": "scope_error",
					"error_message":        errReason,
					"period_start":         w.pStart.Format("2006-01-02"),
					"period_end":           w.pEnd.Format("2006-01-02"),
				})
				continue
			}

			spec := ScheduledReportSpec{
				ReportType: b.reportType,
				ScopeType:  scopeType,
				ZoneID:     params.ZoneID,
				StateID:    params.StateID,
				LgaID:      params.LgaID,
				Title:      fmt.Sprintf(b.titleTmpl, w.periodLabel),
			}

			result.ReportsEvaluated++
			pSummary.ReportsEvaluated++

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
				ReportingPeriod: w.periodLabel,
				ScopeType:       spec.ScopeType,
				ZoneID:          zonePg,
				StateID:         statePg,
				LgaID:           lgaPg,
			})
			if err != nil {
				errMsg := fmt.Sprintf("%s: failed checking existing snapshot: %v", spec.ReportType, err)
				result.Errors = append(result.Errors, errMsg)
				s.logSchedulerAudit(ctx, nil, "generation_failed", auditUserPg, map[string]interface{}{
					"report_category":      spec.ReportType,
					"reporting_period":     w.periodLabel,
					"scope_type":           spec.ScopeType,
					"trigger_source":       triggerSource,
					"actor_type":           actorType,
					"actor_identity":       actorIdentity,
					"outcome":              "failed",
					"error_classification": "database_error",
					"error_message":        sanitizeErrorMessage(err.Error()),
					"period_start":         w.pStart.Format("2006-01-02"),
					"period_end":           w.pEnd.Format("2006-01-02"),
				})
				continue
			}

			if exists > 0 {
				result.ReportsSkipped++
				pSummary.ReportsSkipped++
				skipKey := fmt.Sprintf("%s_%s", spec.ReportType, w.periodLabel)
				reason := "active snapshot already exists for period and scope"
				result.SkippedReasons[skipKey] = reason

				// Audit skipped duplicate
				s.logSchedulerAudit(ctx, nil, "generation_skipped", auditUserPg, map[string]interface{}{
					"report_category":  spec.ReportType,
					"reporting_period": w.periodLabel,
					"scope_type":       spec.ScopeType,
					"trigger_source":   triggerSource,
					"actor_type":       actorType,
					"actor_identity":   actorIdentity,
					"outcome":          "skipped",
					"reason":           reason,
					"period_start":     w.pStart.Format("2006-01-02"),
					"period_end":       w.pEnd.Format("2006-01-02"),
				})
				continue
			}

			// Mandatory Governance Rule: Automated generation creates snapshots in 'draft' status ONLY.
			snap, err := s.reportService.GenerateSnapshot(ctx, snapshotAdminUser, service.GenerateReportRequest{
				ReportType:      spec.ReportType,
				Title:           spec.Title,
				ReportingPeriod: w.periodLabel,
				PeriodStart:     w.pStart.Format("2006-01-02"),
				PeriodEnd:       w.pEnd.Format("2006-01-02"),
				ScopeType:       spec.ScopeType,
				ZoneID:          spec.ZoneID,
				StateID:         spec.StateID,
				LGAID:           spec.LgaID,
				TriggerSource:   triggerSource,
				IsSystemActor:   isSystemActor,
			}, nil)
			if err != nil {
				errMsg := fmt.Sprintf("%s: generation failed: %v", spec.ReportType, err)
				result.Errors = append(result.Errors, errMsg)
				s.logSchedulerAudit(ctx, nil, "generation_failed", auditUserPg, map[string]interface{}{
					"report_category":      spec.ReportType,
					"reporting_period":     w.periodLabel,
					"scope_type":           spec.ScopeType,
					"trigger_source":       triggerSource,
					"actor_type":           actorType,
					"actor_identity":       actorIdentity,
					"outcome":              "failed",
					"error_classification": classifyError(err),
					"error_message":        sanitizeErrorMessage(err.Error()),
					"period_start":         w.pStart.Format("2006-01-02"),
					"period_end":           w.pEnd.Format("2006-01-02"),
				})
				continue
			}

			// Security assertion: status must be strictly 'draft'
			if snap.Status != "draft" {
				errMsg := fmt.Sprintf("%s: automated report was generated with non-draft status '%s'", spec.ReportType, snap.Status)
				result.Errors = append(result.Errors, errMsg)
				return result, errors.New(errMsg)
			}

			result.ReportsGenerated++
			pSummary.ReportsGenerated++
			result.GeneratedReferences = append(result.GeneratedReferences, snap.PublicReference)
			pSummary.GeneratedReferences = append(pSummary.GeneratedReferences, snap.PublicReference)
		}

		result.PeriodSummaries[w.periodLabel] = pSummary
	}

	// Commit advisory lock transaction (releasing the advisory lock cleanly)
	_ = tx.Commit(ctx)
	return result, nil
}

// logSchedulerAudit records audit attempts for skipped or failed generation executions.
func (s *ReportScheduler) logSchedulerAudit(
	ctx context.Context,
	snapshotID *string,
	action string,
	actorUserPg pgtype.Text,
	detailsMap map[string]interface{},
) {
	detailsJSON, err := json.Marshal(detailsMap)
	if err != nil {
		detailsJSON = []byte("{}")
	}

	var snapPg pgtype.Text
	if snapshotID != nil && *snapshotID != "" {
		snapPg = pgtype.Text{String: *snapshotID, Valid: true}
	}

	_, _ = s.repo.Queries().CreateReportAudit(ctx, db.CreateReportAuditParams{
		ID:                security.GenerateUUID(),
		SnapshotID:        snapPg,
		Action:            action,
		PerformedByUserID: actorUserPg,
		Details:           detailsJSON,
		IpAddress:         pgtype.Text{Valid: false},
	})
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

// CalculateQuarterUTC returns the UTC boundary [start of 1st day, end of last day] and YYYY-Q# label for a given quarter (1-4).
func CalculateQuarterUTC(year int, quarter int) (time.Time, time.Time, string, error) {
	if quarter < 1 || quarter > 4 {
		return time.Time{}, time.Time{}, "", fmt.Errorf("invalid quarter %d (must be between 1 and 4)", quarter)
	}
	startMonth := time.Month((quarter-1)*3 + 1) // 1 for Q1, 4 for Q2, 7 for Q3, 10 for Q4
	pStart := time.Date(year, startMonth, 1, 0, 0, 0, 0, time.UTC)
	nextQuarterFirst := pStart.AddDate(0, 3, 0)
	lastDay := nextQuarterFirst.AddDate(0, 0, -1)
	pEnd := time.Date(lastDay.Year(), lastDay.Month(), lastDay.Day(), 23, 59, 59, 999999999, time.UTC)
	periodLabel := fmt.Sprintf("%04d-Q%d", year, quarter)
	return pStart, pEnd, periodLabel, nil
}

// CalculatePreviousQuarterUTC returns the UTC boundary and YYYY-Q# label for the most recently completed quarter.
func CalculatePreviousQuarterUTC(now time.Time) (time.Time, time.Time, string) {
	now = now.UTC()
	currentQuarter := int((now.Month()-1)/3) + 1
	targetYear := now.Year()
	targetQuarter := currentQuarter - 1
	if targetQuarter == 0 {
		targetQuarter = 4
		targetYear--
	}
	pStart, pEnd, label, _ := CalculateQuarterUTC(targetYear, targetQuarter)
	return pStart, pEnd, label
}

// ParsePeriodUTC parses either a monthly period ("YYYY-MM") or quarterly period ("YYYY-Q[1-4]") into exact UTC boundaries.
func ParsePeriodUTC(periodStr string) (time.Time, time.Time, string, string, error) {
	periodStr = strings.TrimSpace(strings.ToUpper(periodStr))
	if len(periodStr) == 7 && strings.Contains(periodStr, "-Q") {
		// Quarterly: YYYY-Q#
		parts := strings.Split(periodStr, "-Q")
		if len(parts) != 2 {
			return time.Time{}, time.Time{}, "", "", fmt.Errorf("invalid quarterly format %q (expected YYYY-Q#)", periodStr)
		}
		year, err := strconv.Atoi(parts[0])
		if err != nil || year < 2000 || year > 2100 {
			return time.Time{}, time.Time{}, "", "", fmt.Errorf("invalid year in quarterly period %q", periodStr)
		}
		qNum, err := strconv.Atoi(parts[1])
		if err != nil || qNum < 1 || qNum > 4 {
			return time.Time{}, time.Time{}, "", "", fmt.Errorf("invalid quarter number in period %q (expected 1-4)", periodStr)
		}
		pStart, pEnd, label, err := CalculateQuarterUTC(year, qNum)
		if err != nil {
			return time.Time{}, time.Time{}, "", "", err
		}
		return pStart, pEnd, label, "quarterly", nil
	}

	// Monthly: YYYY-MM
	t, err := time.Parse("2006-01", periodStr)
	if err != nil {
		return time.Time{}, time.Time{}, "", "", fmt.Errorf("invalid period format %q (expected YYYY-MM or YYYY-Q[1-4])", periodStr)
	}
	pStart, pEnd, label := CalculateMonthUTC(t)
	return pStart, pEnd, label, "monthly", nil
}

func classifyError(err error) string {
	if err == nil {
		return ""
	}
	switch {
	case errors.Is(err, ErrSchedulerAlreadyRunning):
		return "lock_conflict"
	case errors.Is(err, service.ErrInvalidReportScope) || errors.Is(err, service.ErrUnsupportedScopeForType):
		return "scope_error"
	case errors.Is(err, service.ErrInvalidGeographicFilter) || errors.Is(err, service.ErrGeographicHierarchyMismatch):
		return "hierarchy_error"
	case errors.Is(err, ErrNoAdminUserFound):
		return "actor_resolution_error"
	case strings.Contains(strings.ToLower(err.Error()), "database") ||
		strings.Contains(strings.ToLower(err.Error()), "sql") ||
		strings.Contains(strings.ToLower(err.Error()), "connection") ||
		strings.Contains(strings.ToLower(err.Error()), "tx"):
		return "database_error"
	default:
		return "generation_failed"
	}
}

func sanitizeErrorMessage(msg string) string {
	cleaned := security.RedactString(msg)
	if len(cleaned) > 250 {
		cleaned = cleaned[:247] + "..."
	}
	return cleaned
}
