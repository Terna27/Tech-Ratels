package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"ratels-api/internal/db"
	"ratels-api/internal/repository"
	"ratels-api/internal/security"
)

var (
	ErrReportSnapshotNotFound      = errors.New("report snapshot not found")
	ErrReportPublicationNotFound   = errors.New("report publication not found")
	ErrInvalidReportState          = errors.New("invalid report state for requested transition")
	ErrFinancialGovernanceRole     = errors.New("only Super Admin or authorized financial governance officer can approve financial disclosure reports")
	ErrGeographicScopeMismatch     = errors.New("administrator lacks authority over the requested geographic scope")
	ErrReportAlreadyPublished      = errors.New("report snapshot is already published")
	ErrReportNotApproved           = errors.New("report snapshot must be approved before publication")
	ErrInvalidReportScope          = errors.New("invalid or unsupported report scope")
	ErrInvalidGeographicFilter     = errors.New("invalid geographic filter parameters for scope")
	ErrUnsupportedScopeForType     = errors.New("unsupported scope for report type")
	ErrGeographicHierarchyMismatch = errors.New("geographic hierarchy mismatch")
)

// Request / Response DTOs

type GenerateReportRequest struct {
	ReportType      string  `json:"report_type"` // national_overview, cleanups, participation, finance, foundation, community_projects, annual_impact
	Title           string  `json:"title"`
	ReportingPeriod string  `json:"reporting_period"` // e.g., "2026", "2026-10", "All Time"
	PeriodStart     string  `json:"period_start"`     // YYYY-MM-DD
	PeriodEnd       string  `json:"period_end"`       // YYYY-MM-DD
	ScopeType       string  `json:"scope_type"`       // national, zone, state, lga
	ZoneID          *string `json:"zone_id"`
	StateID         *string `json:"state_id"`
	LGAID           *string `json:"lga_id"`
}

type ReviewReportRequest struct {
	DecisionReason string `json:"decision_reason"`
}

type RejectReportRequest struct {
	Reason string `json:"reason"`
}

type WithdrawReportRequest struct {
	Reason string `json:"reason"`
}

type ReviseReportRequest struct {
	CorrectionNotes string `json:"correction_notes"`
}

type PublicOverviewData struct {
	CompletedCleanupsCount       int64  `json:"completed_cleanups_count"`
	TotalRegisteredVolunteers    int64  `json:"total_registered_volunteers"`
	VerifiedAttendeesCount       int64  `json:"verified_attendees_count"`
	UniqueParticipantsCount      int64  `json:"unique_participants_count"`
	ActiveStatesCount            int64  `json:"active_states_count"`
	ActiveLGAsCount              int64  `json:"active_lgas_count"`
	ApprovedProjectsCount        int64  `json:"approved_projects_count"`
	CompletedProjectsCount       int64  `json:"completed_projects_count"`
	ApprovedIncomeKobo           int64  `json:"approved_income_kobo"`
	ApprovedIncomeFormatted      string `json:"approved_income_formatted"`
	ApprovedExpenditureKobo      int64  `json:"approved_expenditure_kobo"`
	ApprovedExpenditureFormatted string `json:"approved_expenditure_formatted"`
	FoundationDonationsKobo      int64  `json:"foundation_donations_kobo"`
	FoundationDonationsFormatted string `json:"foundation_donations_formatted"`
	TotalWasteCollectedKg        int64  `json:"total_waste_collected_kg"`
	ReportingPeriod              string `json:"reporting_period"`
	Scope                        string `json:"scope"`
}

type PublicCleanupsData struct {
	CompletedCleanupsCount  int64             `json:"completed_cleanups_count"`
	ActiveStatesCount       int64             `json:"active_states_count"`
	ActiveLGAsCount         int64             `json:"active_lgas_count"`
	TotalWasteCollectedKg   int64             `json:"total_waste_collected_kg"`
	TotalWasteBags          int64             `json:"total_waste_bags"`
	TotalDurationHours      float64           `json:"total_duration_hours"`
	VerifiedAttendeesCount  int64             `json:"verified_attendees_count"`
	UniqueParticipantsCount int64             `json:"unique_participants_count"`
	StateBreakdown          []CleanupStateRow `json:"state_breakdown"`
	ReportingPeriod         string            `json:"reporting_period"`
}

type CleanupStateRow struct {
	StateID           string `json:"state_id"`
	StateName         string `json:"state_name"`
	CleanupsCount     int64  `json:"cleanups_count"`
	WasteCollectedKg  int64  `json:"waste_collected_kg"`
	VerifiedAttendees int64  `json:"verified_attendees"`
}

type PublicParticipationData struct {
	TotalClaimsSubmitted    int64   `json:"total_claims_submitted"`
	ApprovedClaimsCount     int64   `json:"approved_claims_count"`
	RejectedClaimsCount     int64   `json:"rejected_claims_count"`
	ApprovalRatePercent     float64 `json:"approval_rate_percent"`
	UniqueVerifiedMembers   int64   `json:"unique_verified_members"`
	UniqueMembersSuppressed string  `json:"unique_members_suppressed,omitempty"`
	ReportingPeriod         string  `json:"reporting_period"`
}

type PublicFinanceData struct {
	ApprovedIncomeKobo           int64                 `json:"approved_income_kobo"`
	ApprovedIncomeFormatted      string                `json:"approved_income_formatted"`
	ApprovedExpenditureKobo      int64                 `json:"approved_expenditure_kobo"`
	ApprovedExpenditureFormatted string                `json:"approved_expenditure_formatted"`
	NetSurplusDeficitKobo        int64                 `json:"net_surplus_deficit_kobo"`
	NetSurplusDeficitFormatted   string                `json:"net_surplus_deficit_formatted"`
	TotalApprovedBudgetKobo      int64                 `json:"total_approved_budget_kobo"`
	TotalApprovedBudgetFormatted string                `json:"total_approved_budget_formatted"`
	BudgetVarianceKobo           int64                 `json:"budget_variance_kobo"`
	BudgetVarianceFormatted      string                `json:"budget_variance_formatted"`
	SpendingCategories           []PublicCategorySpend `json:"spending_categories"`
	ReportingPeriod              string                `json:"reporting_period"`
}

type PublicCategorySpend struct {
	Category        string `json:"category"`
	CategoryLabel   string `json:"category_label"`
	ExpenseCount    int64  `json:"expense_count"`
	TotalKobo       int64  `json:"total_kobo"`
	AmountFormatted string `json:"amount_formatted"`
}

type PublicFoundationData struct {
	VerifiedDonationsCount     int64  `json:"verified_donations_count"`
	TotalDonationsKobo         int64  `json:"total_donations_kobo"`
	TotalDonationsFormatted    string `json:"total_donations_formatted"`
	ProvisionalEntityNotice    string `json:"provisional_entity_notice"`
	StrictSegregationNotice    string `json:"strict_segregation_notice"`
	PaymentSafetySwitchEnabled bool   `json:"payment_safety_switch_enabled"`
	ReportingPeriod            string `json:"reporting_period"`
}

type PublicCommunityProjectsData struct {
	TotalProjectsCount                int64                      `json:"total_projects_count"`
	CompletedProjectsCount            int64                      `json:"completed_projects_count"`
	InProgressProjectsCount           int64                      `json:"in_progress_projects_count"`
	PlannedProjectsCount              int64                      `json:"planned_projects_count"`
	TotalApprovedBudgetKobo           int64                      `json:"total_approved_budget_kobo"`
	TotalApprovedBudgetFormatted      string                     `json:"total_approved_budget_formatted"`
	TotalRecordedExpenditureKobo      int64                      `json:"total_recorded_expenditure_kobo"`
	TotalRecordedExpenditureFormatted string                     `json:"total_recorded_expenditure_formatted"`
	Projects                          []PublicProjectSummaryItem `json:"projects"`
	ReportingPeriod                   string                     `json:"reporting_period"`
}

type PublicProjectSummaryItem struct {
	ID                           string `json:"id"`
	Reference                    string `json:"reference"`
	Title                        string `json:"title"`
	Description                  string `json:"description"`
	Category                     string `json:"category"`
	StateName                    string `json:"state_name"`
	LGAName                      string `json:"lga_name"`
	CommunityWard                string `json:"community_ward"`
	LocationDetails              string `json:"location_details"`
	FundingStatus                string `json:"funding_status"`
	ExecutionStatus              string `json:"execution_status"`
	ProgressPercentage           int32  `json:"progress_percentage"`
	ApprovedBudgetKobo           int64  `json:"approved_budget_kobo"`
	ApprovedBudgetFormatted      string `json:"approved_budget_formatted"`
	RecordedExpenditureKobo      int64  `json:"recorded_expenditure_kobo"`
	RecordedExpenditureFormatted string `json:"recorded_expenditure_formatted"`
}

type PublicPublicationDTO struct {
	PublicationID      string          `json:"publication_id"`
	SnapshotID         string          `json:"snapshot_id"`
	PublicReference    string          `json:"public_reference"`
	ReportType         string          `json:"report_type"`
	Title              string          `json:"title"`
	ReportingPeriod    string          `json:"reporting_period"`
	PeriodStart        string          `json:"period_start"`
	PeriodEnd          string          `json:"period_end"`
	ScopeType          string          `json:"scope_type"`
	ZoneName           *string         `json:"zone_name,omitempty"`
	StateName          *string         `json:"state_name,omitempty"`
	LGAName            *string         `json:"lga_name,omitempty"`
	PublicationVersion int             `json:"publication_version"`
	DataHash           string          `json:"data_hash"`
	PublishedAt        time.Time       `json:"published_at"`
	PublisherName      string          `json:"publisher_name"`
	AggregatedData     json.RawMessage `json:"aggregated_data"`
	CorrectionNotice   *string         `json:"correction_notice,omitempty"`
}

type AdminSnapshotDetailDTO struct {
	SnapshotID         string               `json:"snapshot_id"`
	PublicReference    string               `json:"public_reference"`
	DefinitionID       *string              `json:"definition_id,omitempty"`
	ReportType         string               `json:"report_type"`
	Title              string               `json:"title"`
	ReportingPeriod    string               `json:"reporting_period"`
	PeriodStart        string               `json:"period_start"`
	PeriodEnd          string               `json:"period_end"`
	ScopeType          string               `json:"scope_type"`
	ZoneID             *string              `json:"zone_id,omitempty"`
	ZoneName           *string              `json:"zone_name,omitempty"`
	StateID            *string              `json:"state_id,omitempty"`
	StateName          *string              `json:"state_name,omitempty"`
	LGAID              *string              `json:"lga_id,omitempty"`
	LGAName            *string              `json:"lga_name,omitempty"`
	Status             string               `json:"status"`
	PublicationVersion int                  `json:"publication_version"`
	DataHash           string               `json:"data_hash"`
	CreatedByUserID    string               `json:"created_by_user_id"`
	CreatorName        string               `json:"creator_name"`
	CorrectionNotes    *string              `json:"correction_notes,omitempty"`
	AggregatedData     json.RawMessage      `json:"aggregated_data"`
	CreatedAt          time.Time            `json:"created_at"`
	UpdatedAt          time.Time            `json:"updated_at"`
	Approvals          []ReportApprovalItem `json:"approvals"`
	AuditLogs          []ReportAuditItem    `json:"audit_logs"`
}

type ReportApprovalItem struct {
	ID              string    `json:"id"`
	ReviewerID      string    `json:"reviewer_id"`
	ReviewerName    string    `json:"reviewer_name"`
	ReviewerEmail   string    `json:"reviewer_email"`
	Decision        string    `json:"decision"`
	DecisionReason  string    `json:"decision_reason"`
	GovernanceScope string    `json:"governance_scope"`
	CreatedAt       time.Time `json:"created_at"`
}

type ReportAuditItem struct {
	ID            string          `json:"id"`
	SnapshotID    *string         `json:"snapshot_id,omitempty"`
	Action        string          `json:"action"`
	PerformedByID *string         `json:"performed_by_id,omitempty"`
	ActorName     *string         `json:"actor_name,omitempty"`
	ActorEmail    *string         `json:"actor_email,omitempty"`
	Details       json.RawMessage `json:"details"`
	IPAddress     *string         `json:"ip_address,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
}

type PaginatedSnapshotsResponse struct {
	Snapshots  []AdminSnapshotDetailDTO `json:"snapshots"`
	TotalCount int64                    `json:"total_count"`
	Page       int32                    `json:"page"`
	PageSize   int32                    `json:"page_size"`
}

type PaginatedPublicationsResponse struct {
	Publications []PublicPublicationDTO `json:"publications"`
	TotalCount   int64                  `json:"total_count"`
	Page         int32                  `json:"page"`
	PageSize     int32                  `json:"page_size"`
}

// Service definition
type ReportService struct {
	repo         *repository.Repository
	notifService *NotificationService
	auditService *AuditService
}

func NewReportService(repo *repository.Repository, notifService *NotificationService, auditService *AuditService) *ReportService {
	return &ReportService{
		repo:         repo,
		notifService: notifService,
		auditService: auditService,
	}
}

// -----------------------------------------------------------------------
// Aggregation Engine: Real-time deterministic aggregation from source DB
// -----------------------------------------------------------------------

func (s *ReportService) AggregateSourceData(ctx context.Context, reportType, scopeType string, zoneID, stateID, lgaID *string, pStart, pEnd time.Time, periodLabel string) (json.RawMessage, error) {
	q := s.repo.Queries()

	var zonePg pgtype.Text
	if zoneID != nil && *zoneID != "" {
		zonePg = pgtype.Text{String: *zoneID, Valid: true}
	}
	var statePg pgtype.Text
	if stateID != nil && *stateID != "" {
		statePg = pgtype.Text{String: *stateID, Valid: true}
	}
	var lgaPg pgtype.Text
	if lgaID != nil && *lgaID != "" {
		lgaPg = pgtype.Text{String: *lgaID, Valid: true}
	}

	pStartPg := pgtype.Timestamptz{Time: pStart, Valid: true}
	pEndPg := pgtype.Timestamptz{Time: pEnd, Valid: true}

	scopeDesc := "Nigeria Nationwide"
	switch scopeType {
	case "zone":
		if zoneID != nil && *zoneID != "" {
			z, err := q.GetZoneByID(ctx, *zoneID)
			if err == nil && z.Name != "" {
				scopeDesc = z.Name + " Geopolitical Zone"
			}
		}
	case "state":
		if stateID != nil && *stateID != "" {
			st, err := q.GetStateByID(ctx, *stateID)
			if err == nil && st.Name != "" {
				scopeDesc = st.Name + " State"
			}
		}
	case "lga":
		if lgaID != nil && *lgaID != "" {
			lg, err := q.GetLGAByID(ctx, *lgaID)
			if err == nil && lg.Name != "" {
				st, errSt := q.GetStateByID(ctx, lg.StateID)
				if errSt == nil && st.Name != "" {
					scopeDesc = lg.Name + " LGA, " + st.Name + " State"
				} else {
					scopeDesc = lg.Name + " LGA"
				}
			}
		}
	}

	switch reportType {
	case "cleanups":
		cMetrics, err := q.AggregateCleanupsMetrics(ctx, db.AggregateCleanupsMetricsParams{
			StartTime:   pStartPg,
			StartTime_2: pEndPg,
			ZoneID:      zonePg,
			StateID:     statePg,
			LgaID:       lgaPg,
		})
		if err != nil {
			return nil, fmt.Errorf("failed cleanups metrics aggregation: %w", err)
		}
		cPart, err := q.AggregateCleanupsParticipation(ctx, db.AggregateCleanupsParticipationParams{
			StartTime:   pStartPg,
			StartTime_2: pEndPg,
			ZoneID:      zonePg,
			StateID:     statePg,
			LgaID:       lgaPg,
		})
		if err != nil {
			return nil, fmt.Errorf("failed cleanups participation aggregation: %w", err)
		}
		stateRows, err := q.ListCleanupsBreakdownByState(ctx, db.ListCleanupsBreakdownByStateParams{
			StartTime:   pStartPg,
			StartTime_2: pEndPg,
			ZoneID:      zonePg,
			StateID:     statePg,
			LgaID:       lgaPg,
		})
		if err != nil {
			return nil, fmt.Errorf("failed cleanups state breakdown: %w", err)
		}

		var breakdown []CleanupStateRow
		for _, sr := range stateRows {
			if sr.CleanupsCount > 0 {
				breakdown = append(breakdown, CleanupStateRow{
					StateID:           sr.StateID,
					StateName:         sr.StateName,
					CleanupsCount:     sr.CleanupsCount,
					WasteCollectedKg:  sr.WasteKg,
					VerifiedAttendees: sr.VerifiedAttendees,
				})
			}
		}

		durFloat, _ := cMetrics.TotalDurationHours.Float64Value()

		data := PublicCleanupsData{
			CompletedCleanupsCount:  cMetrics.CompletedCleanups,
			ActiveStatesCount:       cMetrics.ActiveStates,
			ActiveLGAsCount:         cMetrics.ActiveLgas,
			TotalWasteCollectedKg:   cMetrics.TotalWasteKg,
			TotalWasteBags:          int64(cMetrics.TotalWasteBags),
			TotalDurationHours:      durFloat.Float64,
			VerifiedAttendeesCount:  cPart.VerifiedAttendees,
			UniqueParticipantsCount: cPart.UniqueParticipants,
			StateBreakdown:          breakdown,
			ReportingPeriod:         periodLabel,
		}
		return json.Marshal(data)

	case "participation":
		pMetrics, err := q.AggregateParticipationMetrics(ctx, db.AggregateParticipationMetricsParams{
			StartTime:   pStartPg,
			StartTime_2: pEndPg,
			ZoneID:      zonePg,
			StateID:     statePg,
			LgaID:       lgaPg,
		})
		if err != nil {
			return nil, fmt.Errorf("failed participation metrics aggregation: %w", err)
		}

		appRate := 0.0
		if pMetrics.TotalClaims > 0 {
			appRate = float64(pMetrics.ApprovedClaims) * 100.0 / float64(pMetrics.TotalClaims)
		}

		data := PublicParticipationData{
			TotalClaimsSubmitted:  pMetrics.TotalClaims,
			ApprovedClaimsCount:   pMetrics.ApprovedClaims,
			RejectedClaimsCount:   pMetrics.RejectedClaims,
			ApprovalRatePercent:   appRate,
			UniqueVerifiedMembers: pMetrics.UniqueVerifiedMembers,
			ReportingPeriod:       periodLabel,
		}
		// Privacy suppression rule: if count > 0 and < 5, mark as suppressed
		if data.UniqueVerifiedMembers > 0 && data.UniqueVerifiedMembers < 5 {
			data.UniqueMembersSuppressed = "< 5 (Protected for privacy)"
		}

		return json.Marshal(data)

	case "finance":
		fMetrics, err := q.AggregateFinanceMetrics(ctx, db.AggregateFinanceMetricsParams{
			RecordedAt:   pStartPg,
			RecordedAt_2: pEndPg,
			ZoneID:       zonePg,
			StateID:      statePg,
			LgaID:        lgaPg,
		})
		if err != nil {
			return nil, fmt.Errorf("failed finance metrics aggregation: %w", err)
		}
		categories, err := q.AggregateExpenseCategories(ctx, db.AggregateExpenseCategoriesParams{
			CreatedAt:   pStartPg,
			CreatedAt_2: pEndPg,
			ZoneID:      zonePg,
			StateID:     statePg,
			LgaID:       lgaPg,
		})
		if err != nil {
			return nil, fmt.Errorf("failed finance categories aggregation: %w", err)
		}
		pStartDatePg := pgtype.Date{Time: pStart, Valid: true}
		pEndDatePg := pgtype.Date{Time: pEnd, Valid: true}
		bMetrics, err := q.AggregateBudgetsMetrics(ctx, db.AggregateBudgetsMetricsParams{
			PeriodStart: pStartDatePg,
			PeriodEnd:   pEndDatePg,
			ZoneID:      zonePg,
			StateID:     statePg,
			LgaID:       lgaPg,
		})
		if err != nil {
			return nil, fmt.Errorf("failed finance budget aggregation: %w", err)
		}

		var catList []PublicCategorySpend
		for _, cat := range categories {
			catLabel := strings.Title(strings.ReplaceAll(cat.Category, "_", " "))
			catList = append(catList, PublicCategorySpend{
				Category:        cat.Category,
				CategoryLabel:   catLabel,
				ExpenseCount:    cat.ExpenseCount,
				TotalKobo:       cat.TotalKobo,
				AmountFormatted: "₦" + FormatKoboToNaira(cat.TotalKobo),
			})
		}

		netSurplusKobo := fMetrics.VerifiedIncomeKobo - fMetrics.VerifiedExpenseKobo
		varianceKobo := bMetrics.TotalApprovedBudgetKobo - fMetrics.VerifiedExpenseKobo

		data := PublicFinanceData{
			ApprovedIncomeKobo:           fMetrics.VerifiedIncomeKobo,
			ApprovedIncomeFormatted:      "₦" + FormatKoboToNaira(fMetrics.VerifiedIncomeKobo),
			ApprovedExpenditureKobo:      fMetrics.VerifiedExpenseKobo,
			ApprovedExpenditureFormatted: "₦" + FormatKoboToNaira(fMetrics.VerifiedExpenseKobo),
			NetSurplusDeficitKobo:        netSurplusKobo,
			NetSurplusDeficitFormatted:   "₦" + FormatKoboToNaira(netSurplusKobo),
			TotalApprovedBudgetKobo:      bMetrics.TotalApprovedBudgetKobo,
			TotalApprovedBudgetFormatted: "₦" + FormatKoboToNaira(bMetrics.TotalApprovedBudgetKobo),
			BudgetVarianceKobo:           varianceKobo,
			BudgetVarianceFormatted:      "₦" + FormatKoboToNaira(varianceKobo),
			SpendingCategories:           catList,
			ReportingPeriod:              periodLabel,
		}
		return json.Marshal(data)

	case "foundation":
		if scopeType == "lga" {
			return nil, fmt.Errorf("%w: foundation reports do not support lga scope", ErrUnsupportedScopeForType)
		}
		fdMetrics, err := q.AggregateFoundationDonations(ctx, db.AggregateFoundationDonationsParams{
			CreatedAt:   pStartPg,
			CreatedAt_2: pEndPg,
			ZoneID:      zonePg,
			StateID:     statePg,
		})
		if err != nil {
			return nil, fmt.Errorf("failed foundation aggregation: %w", err)
		}

		data := PublicFoundationData{
			VerifiedDonationsCount:     fdMetrics.VerifiedDonationsCount,
			TotalDonationsKobo:         fdMetrics.TotalDonationsKobo,
			TotalDonationsFormatted:    "₦" + FormatKoboToNaira(fdMetrics.TotalDonationsKobo),
			ProvisionalEntityNotice:    "Martinsvincent Ortse Foundation registration is provisional pending final CAC review. Dedicated charitable accounts operate with dual-signatory verification.",
			StrictSegregationNotice:    "Foundation humanitarian funds are strictly separated from Nigerian Ratels operational funds in double-entry ledgers.",
			PaymentSafetySwitchEnabled: true,
			ReportingPeriod:            periodLabel,
		}
		return json.Marshal(data)

	case "community_projects":
		cpMetrics, err := q.AggregateCommunityProjectsMetrics(ctx, db.AggregateCommunityProjectsMetricsParams{
			ZoneID:  zonePg,
			StateID: statePg,
			LgaID:   lgaPg,
		})
		if err != nil {
			return nil, fmt.Errorf("failed community projects metrics aggregation: %w", err)
		}
		projectsList, err := q.ListCommunityProjectsPublicReport(ctx, db.ListCommunityProjectsPublicReportParams{
			ZoneID:      zonePg,
			StateID:     statePg,
			LgaID:       lgaPg,
			LimitCount:  100,
			OffsetCount: 0,
		})
		if err != nil {
			return nil, fmt.Errorf("failed community projects listing: %w", err)
		}

		var items []PublicProjectSummaryItem
		for _, p := range projectsList {
			items = append(items, PublicProjectSummaryItem{
				ID:                           p.ID,
				Reference:                    p.Reference,
				Title:                        p.Title,
				Description:                  p.Description,
				Category:                     p.Category,
				StateName:                    p.StateName,
				LGAName:                      p.LgaName,
				CommunityWard:                p.CommunityWard,
				LocationDetails:              p.LocationDetails,
				FundingStatus:                p.FundingStatus,
				ExecutionStatus:              p.ExecutionStatus,
				ProgressPercentage:           p.ProgressPercentage,
				ApprovedBudgetKobo:           p.ApprovedBudget,
				ApprovedBudgetFormatted:      "₦" + FormatKoboToNaira(p.ApprovedBudget),
				RecordedExpenditureKobo:      p.RecordedExpenditure,
				RecordedExpenditureFormatted: "₦" + FormatKoboToNaira(p.RecordedExpenditure),
			})
		}

		data := PublicCommunityProjectsData{
			TotalProjectsCount:                cpMetrics.TotalProjects,
			CompletedProjectsCount:            cpMetrics.CompletedProjects,
			InProgressProjectsCount:           cpMetrics.InProgressProjects,
			PlannedProjectsCount:              cpMetrics.PlannedProjects,
			TotalApprovedBudgetKobo:           cpMetrics.TotalApprovedBudgetKobo,
			TotalApprovedBudgetFormatted:      "₦" + FormatKoboToNaira(cpMetrics.TotalApprovedBudgetKobo),
			TotalRecordedExpenditureKobo:      cpMetrics.TotalRecordedExpenditureKobo,
			TotalRecordedExpenditureFormatted: "₦" + FormatKoboToNaira(cpMetrics.TotalRecordedExpenditureKobo),
			Projects:                          items,
			ReportingPeriod:                   periodLabel,
		}
		return json.Marshal(data)

	case "national_overview", "annual_impact":
		fallthrough
	default:
		// National Overview rollup combines verified stats across modules
		cMetrics, err := q.AggregateCleanupsMetrics(ctx, db.AggregateCleanupsMetricsParams{
			StartTime:   pStartPg,
			StartTime_2: pEndPg,
			ZoneID:      zonePg,
			StateID:     statePg,
			LgaID:       lgaPg,
		})
		if err != nil {
			return nil, fmt.Errorf("failed cleanups metrics aggregation: %w", err)
		}
		cPart, err := q.AggregateCleanupsParticipation(ctx, db.AggregateCleanupsParticipationParams{
			StartTime:   pStartPg,
			StartTime_2: pEndPg,
			ZoneID:      zonePg,
			StateID:     statePg,
			LgaID:       lgaPg,
		})
		if err != nil {
			return nil, fmt.Errorf("failed cleanups participation aggregation: %w", err)
		}
		fMetrics, err := q.AggregateFinanceMetrics(ctx, db.AggregateFinanceMetricsParams{
			RecordedAt:   pStartPg,
			RecordedAt_2: pEndPg,
			ZoneID:       zonePg,
			StateID:      statePg,
			LgaID:        lgaPg,
		})
		if err != nil {
			return nil, fmt.Errorf("failed finance metrics aggregation: %w", err)
		}

		var fdDonationsKobo int64
		if scopeType != "lga" {
			fdMetrics, err := q.AggregateFoundationDonations(ctx, db.AggregateFoundationDonationsParams{
				CreatedAt:   pStartPg,
				CreatedAt_2: pEndPg,
				ZoneID:      zonePg,
				StateID:     statePg,
			})
			if err != nil {
				return nil, fmt.Errorf("failed foundation aggregation: %w", err)
			}
			fdDonationsKobo = fdMetrics.TotalDonationsKobo
		}

		cpMetrics, err := q.AggregateCommunityProjectsMetrics(ctx, db.AggregateCommunityProjectsMetricsParams{
			ZoneID:  zonePg,
			StateID: statePg,
			LgaID:   lgaPg,
		})
		if err != nil {
			return nil, fmt.Errorf("failed community projects metrics aggregation: %w", err)
		}

		data := PublicOverviewData{
			CompletedCleanupsCount:       cMetrics.CompletedCleanups,
			TotalRegisteredVolunteers:    cMetrics.TotalRegistrations,
			VerifiedAttendeesCount:       cPart.VerifiedAttendees,
			UniqueParticipantsCount:      cPart.UniqueParticipants,
			ActiveStatesCount:            cMetrics.ActiveStates,
			ActiveLGAsCount:              cMetrics.ActiveLgas,
			ApprovedProjectsCount:        cpMetrics.TotalProjects,
			CompletedProjectsCount:       cpMetrics.CompletedProjects,
			ApprovedIncomeKobo:           fMetrics.VerifiedIncomeKobo,
			ApprovedIncomeFormatted:      "₦" + FormatKoboToNaira(fMetrics.VerifiedIncomeKobo),
			ApprovedExpenditureKobo:      fMetrics.VerifiedExpenseKobo,
			ApprovedExpenditureFormatted: "₦" + FormatKoboToNaira(fMetrics.VerifiedExpenseKobo),
			FoundationDonationsKobo:      fdDonationsKobo,
			FoundationDonationsFormatted: "₦" + FormatKoboToNaira(fdDonationsKobo),
			TotalWasteCollectedKg:        cMetrics.TotalWasteKg,
			ReportingPeriod:              periodLabel,
			Scope:                        scopeDesc,
		}
		return json.Marshal(data)
	}
}

// -----------------------------------------------------------------------
// Administrative Snapshot Operations
// -----------------------------------------------------------------------

func (s *ReportService) GenerateSnapshot(ctx context.Context, adminUser *db.User, req GenerateReportRequest, ipAddress *string) (*AdminSnapshotDetailDTO, error) {
	// 1. Report Type validation
	switch req.ReportType {
	case "national_overview", "cleanups", "participation", "finance", "foundation", "community_projects", "annual_impact":
	default:
		return nil, fmt.Errorf("%w: %q", ErrInvalidReportScope, req.ReportType)
	}

	// Parse dates in UTC with inclusive end-of-day boundary
	pStart, err := time.Parse("2006-01-02", req.PeriodStart)
	if err != nil {
		pStart = time.Date(time.Now().UTC().Year(), 1, 1, 0, 0, 0, 0, time.UTC)
	} else {
		pStart = time.Date(pStart.Year(), pStart.Month(), pStart.Day(), 0, 0, 0, 0, time.UTC)
	}
	pEnd, err := time.Parse("2006-01-02", req.PeriodEnd)
	if err != nil {
		pEnd = time.Now().UTC()
	} else {
		pEnd = time.Date(pEnd.Year(), pEnd.Month(), pEnd.Day(), 23, 59, 59, 999999999, time.UTC)
	}

	scopeType := req.ScopeType
	if scopeType == "" {
		scopeType = "national"
	}

	// 2. Scope Type validation
	switch scopeType {
	case "national", "zone", "state", "lga":
	default:
		return nil, fmt.Errorf("%w: %q", ErrInvalidReportScope, scopeType)
	}

	// 3. Module Scope capability validation
	if req.ReportType == "foundation" && scopeType == "lga" {
		return nil, fmt.Errorf("%w: foundation reports do not support lga scope", ErrUnsupportedScopeForType)
	}

	hasID := func(id *string) bool {
		return id != nil && strings.TrimSpace(*id) != ""
	}

	q := s.repo.Queries()

	var resolvedZoneID, resolvedStateID, resolvedLGAID *string

	switch scopeType {
	case "national":
		if hasID(req.ZoneID) || hasID(req.StateID) || hasID(req.LGAID) {
			return nil, fmt.Errorf("%w: national scope must not specify zone_id, state_id, or lga_id", ErrInvalidGeographicFilter)
		}

	case "zone":
		if !hasID(req.ZoneID) {
			return nil, fmt.Errorf("%w: zone_id is required for zone scope", ErrInvalidGeographicFilter)
		}
		if hasID(req.StateID) || hasID(req.LGAID) {
			return nil, fmt.Errorf("%w: zone scope must not specify state_id or lga_id", ErrInvalidGeographicFilter)
		}
		zone, err := q.GetZoneByID(ctx, *req.ZoneID)
		if err != nil {
			return nil, fmt.Errorf("%w: zone not found: %s", ErrInvalidGeographicFilter, *req.ZoneID)
		}
		resolvedZoneID = &zone.ID

	case "state":
		if !hasID(req.StateID) {
			return nil, fmt.Errorf("%w: state_id is required for state scope", ErrInvalidGeographicFilter)
		}
		if hasID(req.LGAID) {
			return nil, fmt.Errorf("%w: state scope must not specify lga_id", ErrInvalidGeographicFilter)
		}
		state, err := q.GetStateByID(ctx, *req.StateID)
		if err != nil {
			return nil, fmt.Errorf("%w: state not found: %s", ErrInvalidGeographicFilter, *req.StateID)
		}
		resolvedStateID = &state.ID
		if hasID(req.ZoneID) {
			if *req.ZoneID != state.ZoneID {
				return nil, fmt.Errorf("%w: state %s does not belong to zone %s", ErrGeographicHierarchyMismatch, state.ID, *req.ZoneID)
			}
			resolvedZoneID = req.ZoneID
		}

	case "lga":
		if !hasID(req.LGAID) {
			return nil, fmt.Errorf("%w: lga_id is required for lga scope", ErrInvalidGeographicFilter)
		}
		lga, err := q.GetLGAByID(ctx, *req.LGAID)
		if err != nil {
			return nil, fmt.Errorf("%w: lga not found: %s", ErrInvalidGeographicFilter, *req.LGAID)
		}
		resolvedLGAID = &lga.ID
		if hasID(req.StateID) {
			if *req.StateID != lga.StateID {
				return nil, fmt.Errorf("%w: lga %s does not belong to state %s", ErrGeographicHierarchyMismatch, lga.ID, *req.StateID)
			}
			resolvedStateID = req.StateID
		}
		state, err := q.GetStateByID(ctx, lga.StateID)
		if err != nil {
			return nil, fmt.Errorf("%w: state for lga not found: %s", ErrInvalidGeographicFilter, lga.StateID)
		}
		if hasID(req.ZoneID) {
			if *req.ZoneID != state.ZoneID {
				return nil, fmt.Errorf("%w: lga's state %s does not belong to zone %s", ErrGeographicHierarchyMismatch, state.ID, *req.ZoneID)
			}
			resolvedZoneID = req.ZoneID
		}
	}

	// Aggregate live verified source data
	aggregatedBytes, err := s.AggregateSourceData(ctx, req.ReportType, scopeType, resolvedZoneID, resolvedStateID, resolvedLGAID, pStart, pEnd, req.ReportingPeriod)
	if err != nil {
		return nil, fmt.Errorf("failed to aggregate source data: %w", err)
	}

	dataHash := ComputeDataHash(aggregatedBytes)

	// Generate public reference
	prefix := "REP"
	typeCode := "NAT"
	switch req.ReportType {
	case "cleanups":
		typeCode = "CLN"
	case "participation":
		typeCode = "PRT"
	case "finance":
		typeCode = "FIN"
	case "foundation":
		typeCode = "FDN"
	case "community_projects":
		typeCode = "PRJ"
	case "annual_impact":
		typeCode = "ANN"
	}
	publicRef := fmt.Sprintf("%s-%d-%s-%s", prefix, time.Now().Year(), typeCode, strings.ToUpper(security.GenerateUUID()[:8]))

	snapshotID := security.GenerateUUID()

	var definitionID *string
	def, err := s.repo.Queries().GetReportDefinitionByType(ctx, req.ReportType)
	if err == nil {
		definitionID = &def.ID
	}

	var zonePg, statePg, lgaPg pgtype.Text
	switch scopeType {
	case "zone":
		zonePg = pgtype.Text{String: *resolvedZoneID, Valid: true}
	case "state":
		statePg = pgtype.Text{String: *resolvedStateID, Valid: true}
		if resolvedZoneID != nil {
			zonePg = pgtype.Text{String: *resolvedZoneID, Valid: true}
		}
	case "lga":
		lgaPg = pgtype.Text{String: *resolvedLGAID, Valid: true}
		if resolvedStateID != nil {
			statePg = pgtype.Text{String: *resolvedStateID, Valid: true}
		}
		if resolvedZoneID != nil {
			zonePg = pgtype.Text{String: *resolvedZoneID, Valid: true}
		}
	}

	var defPg pgtype.Text
	if definitionID != nil {
		defPg = pgtype.Text{String: *definitionID, Valid: true}
	}

	row, err := s.repo.Queries().CreateReportSnapshot(ctx, db.CreateReportSnapshotParams{
		ID:                 snapshotID,
		PublicReference:    publicRef,
		DefinitionID:       defPg,
		ReportType:         req.ReportType,
		Title:              req.Title,
		ReportingPeriod:    req.ReportingPeriod,
		PeriodStart:        pgtype.Date{Time: pStart, Valid: true},
		PeriodEnd:          pgtype.Date{Time: pEnd, Valid: true},
		ScopeType:          scopeType,
		ZoneID:             zonePg,
		StateID:            statePg,
		LgaID:              lgaPg,
		Status:             "draft",
		AggregatedData:     aggregatedBytes,
		DataHash:           dataHash,
		PublicationVersion: 1,
		CorrectionNotes:    pgtype.Text{Valid: false},
		CreatedByUserID:    adminUser.ID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed creating report snapshot: %w", err)
	}

	// Log audit
	detailsJSON, _ := json.Marshal(map[string]interface{}{
		"report_type": req.ReportType,
		"public_ref":  publicRef,
		"scope_type":  scopeType,
		"data_hash":   dataHash,
	})
	var ipPg pgtype.Text
	if ipAddress != nil {
		ipPg = pgtype.Text{String: *ipAddress, Valid: true}
	}
	_, _ = s.repo.Queries().CreateReportAudit(ctx, db.CreateReportAuditParams{
		ID:                security.GenerateUUID(),
		SnapshotID:        pgtype.Text{String: snapshotID, Valid: true},
		Action:            "draft_created",
		PerformedByUserID: pgtype.Text{String: adminUser.ID, Valid: true},
		Details:           detailsJSON,
		IpAddress:         ipPg,
	})

	return s.GetSnapshotDetailAdmin(ctx, row.ID)
}

func (s *ReportService) SubmitForReview(ctx context.Context, adminUser *db.User, snapshotID string, ipAddress *string) (*AdminSnapshotDetailDTO, error) {
	snap, err := s.repo.Queries().GetReportSnapshotByID(ctx, snapshotID)
	if err != nil {
		return nil, ErrReportSnapshotNotFound
	}
	if snap.Status != "draft" && snap.Status != "rejected" {
		return nil, ErrInvalidReportState
	}

	updated, err := s.repo.Queries().UpdateReportSnapshotStatus(ctx, db.UpdateReportSnapshotStatusParams{
		ID:     snapshotID,
		Status: "pending_review",
	})
	if err != nil {
		return nil, err
	}

	detailsJSON, _ := json.Marshal(map[string]interface{}{
		"previous_status": snap.Status,
		"new_status":      "pending_review",
	})
	var ipPg pgtype.Text
	if ipAddress != nil {
		ipPg = pgtype.Text{String: *ipAddress, Valid: true}
	}
	_, _ = s.repo.Queries().CreateReportAudit(ctx, db.CreateReportAuditParams{
		ID:                security.GenerateUUID(),
		SnapshotID:        pgtype.Text{String: snapshotID, Valid: true},
		Action:            "submitted_for_review",
		PerformedByUserID: pgtype.Text{String: adminUser.ID, Valid: true},
		Details:           detailsJSON,
		IpAddress:         ipPg,
	})

	// Notify national admins
	if s.notifService != nil {
		s.notifService.OnReportSubmittedForReview(ctx, s.repo.Queries(), adminUser.ID, snapshotID, snap.Title, snap.PublicReference)
	}

	return s.GetSnapshotDetailAdmin(ctx, updated.ID)
}

func (s *ReportService) ApproveSnapshot(ctx context.Context, reviewer *db.User, snapshotID string, reason string, ipAddress *string) (*AdminSnapshotDetailDTO, error) {
	snap, err := s.repo.Queries().GetReportSnapshotByID(ctx, snapshotID)
	if err != nil {
		return nil, ErrReportSnapshotNotFound
	}
	if snap.Status != "pending_review" {
		return nil, ErrInvalidReportState
	}

	// 1. Separation of duties: Anti-Self-Approval
	if snap.CreatedByUserID == reviewer.ID {
		return nil, ErrSelfApprovalProhibited
	}

	// 2. Financial Disclosure Authorization Guard: Only Super Admin can authorize finance reports
	if (snap.ReportType == "finance" || snap.ReportType == "annual_impact") && reviewer.PrimaryRole != "super_admin" {
		return nil, ErrFinancialGovernanceRole
	}

	// Record approval
	_, err = s.repo.Queries().CreateReportApproval(ctx, db.CreateReportApprovalParams{
		ID:              security.GenerateUUID(),
		SnapshotID:      snapshotID,
		ReviewerID:      reviewer.ID,
		Decision:        "approved",
		DecisionReason:  reason,
		GovernanceScope: snap.ScopeType,
	})
	if err != nil {
		return nil, fmt.Errorf("failed creating approval record: %w", err)
	}

	updated, err := s.repo.Queries().UpdateReportSnapshotStatus(ctx, db.UpdateReportSnapshotStatusParams{
		ID:     snapshotID,
		Status: "approved",
	})
	if err != nil {
		return nil, err
	}

	detailsJSON, _ := json.Marshal(map[string]interface{}{
		"decision":        "approved",
		"decision_reason": reason,
	})
	var ipPg pgtype.Text
	if ipAddress != nil {
		ipPg = pgtype.Text{String: *ipAddress, Valid: true}
	}
	_, _ = s.repo.Queries().CreateReportAudit(ctx, db.CreateReportAuditParams{
		ID:                security.GenerateUUID(),
		SnapshotID:        pgtype.Text{String: snapshotID, Valid: true},
		Action:            "approved",
		PerformedByUserID: pgtype.Text{String: reviewer.ID, Valid: true},
		Details:           detailsJSON,
		IpAddress:         ipPg,
	})

	if s.notifService != nil {
		s.notifService.OnReportApproved(ctx, s.repo.Queries(), snap.CreatedByUserID, snapshotID, snap.Title, snap.PublicReference)
	}

	return s.GetSnapshotDetailAdmin(ctx, updated.ID)
}

func (s *ReportService) RejectSnapshot(ctx context.Context, reviewer *db.User, snapshotID string, reason string, ipAddress *string) (*AdminSnapshotDetailDTO, error) {
	snap, err := s.repo.Queries().GetReportSnapshotByID(ctx, snapshotID)
	if err != nil {
		return nil, ErrReportSnapshotNotFound
	}
	if snap.Status != "pending_review" {
		return nil, ErrInvalidReportState
	}

	_, err = s.repo.Queries().CreateReportApproval(ctx, db.CreateReportApprovalParams{
		ID:              security.GenerateUUID(),
		SnapshotID:      snapshotID,
		ReviewerID:      reviewer.ID,
		Decision:        "rejected",
		DecisionReason:  reason,
		GovernanceScope: snap.ScopeType,
	})
	if err != nil {
		return nil, err
	}

	updated, err := s.repo.Queries().UpdateReportSnapshotStatus(ctx, db.UpdateReportSnapshotStatusParams{
		ID:     snapshotID,
		Status: "rejected",
	})
	if err != nil {
		return nil, err
	}

	detailsJSON, _ := json.Marshal(map[string]interface{}{
		"decision": "rejected",
		"reason":   reason,
	})
	var ipPg pgtype.Text
	if ipAddress != nil {
		ipPg = pgtype.Text{String: *ipAddress, Valid: true}
	}
	_, _ = s.repo.Queries().CreateReportAudit(ctx, db.CreateReportAuditParams{
		ID:                security.GenerateUUID(),
		SnapshotID:        pgtype.Text{String: snapshotID, Valid: true},
		Action:            "rejected",
		PerformedByUserID: pgtype.Text{String: reviewer.ID, Valid: true},
		Details:           detailsJSON,
		IpAddress:         ipPg,
	})

	if s.notifService != nil {
		s.notifService.OnReportRejected(ctx, s.repo.Queries(), snap.CreatedByUserID, snapshotID, snap.Title, snap.PublicReference, reason)
	}

	return s.GetSnapshotDetailAdmin(ctx, updated.ID)
}

func (s *ReportService) PublishSnapshot(ctx context.Context, publisher *db.User, snapshotID string, ipAddress *string) (*PublicPublicationDTO, error) {
	if publisher == nil || publisher.PrimaryRole != "super_admin" {
		return nil, ErrFinancialGovernanceRole
	}

	snap, err := s.repo.Queries().GetReportSnapshotByID(ctx, snapshotID)
	if err != nil {
		return nil, ErrReportSnapshotNotFound
	}
	if snap.Status != "approved" {
		return nil, ErrReportNotApproved
	}

	// Transactional state transition: snapshot -> published, public_report_publications -> published
	var pub db.PublicReportPublication
	err = s.repo.WithTx(ctx, func(qtx *db.Queries) error {
		_, err := qtx.UpdateReportSnapshotStatus(ctx, db.UpdateReportSnapshotStatusParams{
			ID:     snapshotID,
			Status: "published",
		})
		if err != nil {
			return err
		}

		pubID := security.GenerateUUID()
		pub, err = qtx.CreateReportPublication(ctx, db.CreateReportPublicationParams{
			ID:                pubID,
			SnapshotID:        snapshotID,
			PublishedByUserID: publisher.ID,
		})
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("failed publishing report: %w", err)
	}

	detailsJSON, _ := json.Marshal(map[string]interface{}{
		"publication_id": pub.ID,
		"snapshot_id":    snapshotID,
		"public_ref":     snap.PublicReference,
	})
	var ipPg pgtype.Text
	if ipAddress != nil {
		ipPg = pgtype.Text{String: *ipAddress, Valid: true}
	}
	_, _ = s.repo.Queries().CreateReportAudit(ctx, db.CreateReportAuditParams{
		ID:                security.GenerateUUID(),
		SnapshotID:        pgtype.Text{String: snapshotID, Valid: true},
		Action:            "published",
		PerformedByUserID: pgtype.Text{String: publisher.ID, Valid: true},
		Details:           detailsJSON,
		IpAddress:         ipPg,
	})

	if s.notifService != nil {
		s.notifService.OnReportPublished(ctx, s.repo.Queries(), snap.CreatedByUserID, snapshotID, snap.Title, snap.PublicReference)
	}

	return s.GetPublicPublicationByID(ctx, snapshotID)
}

func (s *ReportService) WithdrawPublication(ctx context.Context, user *db.User, snapshotID string, reason string, ipAddress *string) (*AdminSnapshotDetailDTO, error) {
	if user == nil || user.PrimaryRole != "super_admin" {
		return nil, ErrFinancialGovernanceRole
	}

	snap, err := s.repo.Queries().GetReportSnapshotByID(ctx, snapshotID)
	if err != nil {
		return nil, ErrReportSnapshotNotFound
	}
	if snap.Status != "published" {
		return nil, ErrInvalidReportState
	}

	err = s.repo.WithTx(ctx, func(qtx *db.Queries) error {
		_, err := qtx.WithdrawReportPublication(ctx, db.WithdrawReportPublicationParams{
			SnapshotID:        snapshotID,
			WithdrawnByUserID: pgtype.Text{String: user.ID, Valid: true},
			WithdrawalReason:  pgtype.Text{String: reason, Valid: true},
		})
		if err != nil {
			return err
		}

		_, err = qtx.UpdateReportSnapshotStatus(ctx, db.UpdateReportSnapshotStatusParams{
			ID:     snapshotID,
			Status: "withdrawn",
		})
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("failed withdrawing report: %w", err)
	}

	detailsJSON, _ := json.Marshal(map[string]interface{}{
		"action": "withdrawn",
		"reason": reason,
	})
	var ipPg pgtype.Text
	if ipAddress != nil {
		ipPg = pgtype.Text{String: *ipAddress, Valid: true}
	}
	_, _ = s.repo.Queries().CreateReportAudit(ctx, db.CreateReportAuditParams{
		ID:                security.GenerateUUID(),
		SnapshotID:        pgtype.Text{String: snapshotID, Valid: true},
		Action:            "withdrawn",
		PerformedByUserID: pgtype.Text{String: user.ID, Valid: true},
		Details:           detailsJSON,
		IpAddress:         ipPg,
	})

	if s.notifService != nil {
		s.notifService.OnReportWithdrawn(ctx, s.repo.Queries(), snap.CreatedByUserID, snapshotID, snap.Title, snap.PublicReference, reason)
	}

	return s.GetSnapshotDetailAdmin(ctx, snapshotID)
}

func (s *ReportService) ReviseSnapshot(ctx context.Context, user *db.User, snapshotID string, correctionNotes string, ipAddress *string) (*AdminSnapshotDetailDTO, error) {
	snap, err := s.repo.Queries().GetReportSnapshotByID(ctx, snapshotID)
	if err != nil {
		return nil, ErrReportSnapshotNotFound
	}

	// Generate refreshed aggregation
	pStart := snap.PeriodStart.Time
	pEnd := snap.PeriodEnd.Time

	var zoneID, stateID, lgaID *string
	if snap.ZoneID.Valid {
		zoneID = &snap.ZoneID.String
	}
	if snap.StateID.Valid {
		stateID = &snap.StateID.String
	}
	if snap.LgaID.Valid {
		lgaID = &snap.LgaID.String
	}

	freshData, err := s.AggregateSourceData(ctx, snap.ReportType, snap.ScopeType, zoneID, stateID, lgaID, pStart, pEnd, snap.ReportingPeriod)
	if err != nil {
		return nil, fmt.Errorf("failed regenerating data: %w", err)
	}
	freshHash := ComputeDataHash(freshData)

	newVersion := snap.PublicationVersion + 1
	revisedRef := fmt.Sprintf("%s-R%d", snap.PublicReference, newVersion)
	newSnapshotID := security.GenerateUUID()

	var defIDPg pgtype.Text
	if snap.DefinitionID.Valid {
		defIDPg = snap.DefinitionID
	}

	newSnap, err := s.repo.Queries().CreateReportSnapshot(ctx, db.CreateReportSnapshotParams{
		ID:                 newSnapshotID,
		PublicReference:    revisedRef,
		DefinitionID:       defIDPg,
		ReportType:         snap.ReportType,
		Title:              fmt.Sprintf("%s (Revision %d)", snap.Title, newVersion),
		ReportingPeriod:    snap.ReportingPeriod,
		PeriodStart:        snap.PeriodStart,
		PeriodEnd:          snap.PeriodEnd,
		ScopeType:          snap.ScopeType,
		ZoneID:             snap.ZoneID,
		StateID:            snap.StateID,
		LgaID:              snap.LgaID,
		Status:             "draft",
		AggregatedData:     freshData,
		DataHash:           freshHash,
		PublicationVersion: newVersion,
		CorrectionNotes:    pgtype.Text{String: correctionNotes, Valid: true},
		CreatedByUserID:    user.ID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed creating revised snapshot: %w", err)
	}

	detailsJSON, _ := json.Marshal(map[string]interface{}{
		"superseded_snapshot_id": snapshotID,
		"new_snapshot_id":        newSnapshotID,
		"new_version":            newVersion,
		"correction_notes":       correctionNotes,
	})
	var ipPg pgtype.Text
	if ipAddress != nil {
		ipPg = pgtype.Text{String: *ipAddress, Valid: true}
	}
	_, _ = s.repo.Queries().CreateReportAudit(ctx, db.CreateReportAuditParams{
		ID:                security.GenerateUUID(),
		SnapshotID:        pgtype.Text{String: newSnapshotID, Valid: true},
		Action:            "revised",
		PerformedByUserID: pgtype.Text{String: user.ID, Valid: true},
		Details:           detailsJSON,
		IpAddress:         ipPg,
	})

	return s.GetSnapshotDetailAdmin(ctx, newSnap.ID)
}

// -----------------------------------------------------------------------
// Public Queries (Allowlisted, Published Only)
// -----------------------------------------------------------------------

func (s *ReportService) ListPublicPublications(ctx context.Context, reportType, scopeType, stateID, period string, page, pageSize int32) (*PaginatedPublicationsResponse, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	var rtPg, stPg, sidPg, perPg pgtype.Text
	if reportType != "" {
		rtPg = pgtype.Text{String: reportType, Valid: true}
	}
	if scopeType != "" {
		stPg = pgtype.Text{String: scopeType, Valid: true}
	}
	if stateID != "" {
		sidPg = pgtype.Text{String: stateID, Valid: true}
	}
	if period != "" {
		perPg = pgtype.Text{String: period, Valid: true}
	}

	rows, err := s.repo.Queries().ListPublicPublications(ctx, db.ListPublicPublicationsParams{
		ReportType:      rtPg,
		ScopeType:       stPg,
		StateID:         sidPg,
		ReportingPeriod: perPg,
		LimitCount:      pageSize,
		OffsetCount:     offset,
	})
	if err != nil {
		return nil, err
	}

	totalCount, err := s.repo.Queries().CountPublicPublications(ctx, db.CountPublicPublicationsParams{
		ReportType:      rtPg,
		ScopeType:       stPg,
		StateID:         sidPg,
		ReportingPeriod: perPg,
	})
	if err != nil {
		return nil, err
	}

	var pubs []PublicPublicationDTO
	for _, r := range rows {
		p := PublicPublicationDTO{
			PublicationID:      r.PublicationID,
			SnapshotID:         r.SnapshotID,
			PublicReference:    r.PublicReference,
			ReportType:         r.ReportType,
			Title:              r.Title,
			ReportingPeriod:    r.ReportingPeriod,
			PeriodStart:        r.PeriodStart.Time.Format("2006-01-02"),
			PeriodEnd:          r.PeriodEnd.Time.Format("2006-01-02"),
			ScopeType:          r.ScopeType,
			PublicationVersion: int(r.PublicationVersion),
			DataHash:           r.DataHash,
			PublishedAt:        r.PublishedAt.Time,
			PublisherName:      r.PublisherName,
			AggregatedData:     r.AggregatedData,
		}
		if r.ZoneName.Valid {
			p.ZoneName = &r.ZoneName.String
		}
		if r.StateName.Valid {
			p.StateName = &r.StateName.String
		}
		if r.LgaName.Valid {
			p.LGAName = &r.LgaName.String
		}
		if r.CorrectionNotice.Valid {
			p.CorrectionNotice = &r.CorrectionNotice.String
		}
		pubs = append(pubs, p)
	}

	return &PaginatedPublicationsResponse{
		Publications: pubs,
		TotalCount:   totalCount,
		Page:         page,
		PageSize:     pageSize,
	}, nil
}

func (s *ReportService) GetPublicPublicationByID(ctx context.Context, idOrRef string) (*PublicPublicationDTO, error) {
	r, err := s.repo.Queries().GetPublishedReportByID(ctx, idOrRef)
	if err != nil {
		return nil, ErrReportPublicationNotFound
	}

	p := &PublicPublicationDTO{
		PublicationID:      r.PublicationID,
		SnapshotID:         r.SnapshotID,
		PublicReference:    r.PublicReference,
		ReportType:         r.ReportType,
		Title:              r.Title,
		ReportingPeriod:    r.ReportingPeriod,
		PeriodStart:        r.PeriodStart.Time.Format("2006-01-02"),
		PeriodEnd:          r.PeriodEnd.Time.Format("2006-01-02"),
		ScopeType:          r.ScopeType,
		PublicationVersion: int(r.PublicationVersion),
		DataHash:           r.DataHash,
		PublishedAt:        r.PublishedAt.Time,
		PublisherName:      r.PublisherName,
		AggregatedData:     r.AggregatedData,
	}
	if r.ZoneName.Valid {
		p.ZoneName = &r.ZoneName.String
	}
	if r.StateName.Valid {
		p.StateName = &r.StateName.String
	}
	if r.LgaName.Valid {
		p.LGAName = &r.LgaName.String
	}
	if r.CorrectionNotice.Valid {
		p.CorrectionNotice = &r.CorrectionNotice.String
	}

	return p, nil
}

// -----------------------------------------------------------------------
// Exports: PDF and CSV Generation
// -----------------------------------------------------------------------

func (s *ReportService) ExportReportPDF(ctx context.Context, idOrRef string) ([]byte, string, error) {
	pub, err := s.GetPublicPublicationByID(ctx, idOrRef)
	if err != nil {
		return nil, "", err
	}

	scopeStr := "Nigeria (Nationwide)"
	if pub.StateName != nil {
		scopeStr = *pub.StateName + " State"
	}

	pdfData := PDFReportData{
		Title:             pub.Title,
		PublicReference:   pub.PublicReference,
		ReportType:        pub.ReportType,
		ReportingPeriod:   pub.ReportingPeriod,
		Scope:             scopeStr,
		PublicationDate:   pub.PublishedAt,
		Version:           pub.PublicationVersion,
		DataHash:          pub.DataHash,
		CertificationNote: "This document is a certified public transparency report issued by the Nigerian Ratels Management System.",
	}

	// Decode aggregated data to construct metrics and tables
	switch pub.ReportType {
	case "cleanups":
		var cData PublicCleanupsData
		_ = json.Unmarshal(pub.AggregatedData, &cData)
		pdfData.Metrics = []PDFMetricItem{
			{Label: "Cleanups Completed", Value: fmt.Sprintf("%d", cData.CompletedCleanupsCount)},
			{Label: "Verified Attendees", Value: fmt.Sprintf("%d", cData.VerifiedAttendeesCount)},
			{Label: "Waste Collected (kg)", Value: fmt.Sprintf("%d kg", cData.TotalWasteCollectedKg)},
			{Label: "Active States", Value: fmt.Sprintf("%d", cData.ActiveStatesCount)},
		}
		if len(cData.StateBreakdown) > 0 {
			tbl := PDFTable{
				Title:   "Cleanup Activities by State",
				Headers: []string{"State", "Cleanups", "Waste (kg)", "Attendees"},
			}
			for _, sb := range cData.StateBreakdown {
				tbl.Rows = append(tbl.Rows, []string{
					sb.StateName,
					fmt.Sprintf("%d", sb.CleanupsCount),
					fmt.Sprintf("%d kg", sb.WasteCollectedKg),
					fmt.Sprintf("%d", sb.VerifiedAttendees),
				})
			}
			pdfData.Tables = append(pdfData.Tables, tbl)
		}

	case "finance":
		var fData PublicFinanceData
		_ = json.Unmarshal(pub.AggregatedData, &fData)
		pdfData.Metrics = []PDFMetricItem{
			{Label: "Approved Income", Value: fData.ApprovedIncomeFormatted},
			{Label: "Approved Expenditure", Value: fData.ApprovedExpenditureFormatted},
			{Label: "Net Surplus / Deficit", Value: fData.NetSurplusDeficitFormatted},
			{Label: "Approved Budget", Value: fData.TotalApprovedBudgetFormatted},
		}
		if len(fData.SpendingCategories) > 0 {
			tbl := PDFTable{
				Title:   "Expenditure Breakdown by Governance Category",
				Headers: []string{"Category", "Expense Count", "Total (NGN)"},
			}
			for _, cat := range fData.SpendingCategories {
				tbl.Rows = append(tbl.Rows, []string{
					cat.CategoryLabel,
					fmt.Sprintf("%d", cat.ExpenseCount),
					cat.AmountFormatted,
				})
			}
			pdfData.Tables = append(pdfData.Tables, tbl)
		}

	case "community_projects":
		var pData PublicCommunityProjectsData
		_ = json.Unmarshal(pub.AggregatedData, &pData)
		pdfData.Metrics = []PDFMetricItem{
			{Label: "Total Projects", Value: fmt.Sprintf("%d", pData.TotalProjectsCount)},
			{Label: "Completed", Value: fmt.Sprintf("%d", pData.CompletedProjectsCount)},
			{Label: "In Progress", Value: fmt.Sprintf("%d", pData.InProgressProjectsCount)},
			{Label: "Approved Budget", Value: pData.TotalApprovedBudgetFormatted},
		}
		if len(pData.Projects) > 0 {
			tbl := PDFTable{
				Title:   "Community Projects Progress",
				Headers: []string{"Ref", "Project Title", "State", "Status", "Progress"},
			}
			for _, pr := range pData.Projects {
				tbl.Rows = append(tbl.Rows, []string{
					pr.Reference,
					pr.Title,
					pr.StateName,
					strings.Title(strings.ReplaceAll(pr.ExecutionStatus, "_", " ")),
					fmt.Sprintf("%d%%", pr.ProgressPercentage),
				})
			}
			pdfData.Tables = append(pdfData.Tables, tbl)
		}

	case "foundation":
		var fndData PublicFoundationData
		_ = json.Unmarshal(pub.AggregatedData, &fndData)
		pdfData.Metrics = []PDFMetricItem{
			{Label: "Verified Donations", Value: fmt.Sprintf("%d", fndData.VerifiedDonationsCount)},
			{Label: "Total Receipts", Value: fndData.TotalDonationsFormatted},
			{Label: "Entity Status", Value: "Provisional"},
			{Label: "Live Checkout Switch", Value: "Safe Mode"},
		}

	default: // national_overview
		var oData PublicOverviewData
		_ = json.Unmarshal(pub.AggregatedData, &oData)
		pdfData.Metrics = []PDFMetricItem{
			{Label: "Cleanups Completed", Value: fmt.Sprintf("%d", oData.CompletedCleanupsCount)},
			{Label: "Verified Attendees", Value: fmt.Sprintf("%d", oData.VerifiedAttendeesCount)},
			{Label: "Community Projects", Value: fmt.Sprintf("%d", oData.ApprovedProjectsCount)},
			{Label: "Reconciled Income", Value: oData.ApprovedIncomeFormatted},
		}
	}

	pdfBytes, err := GeneratePDF(pdfData)
	if err != nil {
		return nil, "", fmt.Errorf("failed generating pdf: %w", err)
	}

	filename := fmt.Sprintf("Ratels-Report-%s.pdf", pub.PublicReference)
	return pdfBytes, filename, nil
}

func (s *ReportService) ExportReportCSV(ctx context.Context, idOrRef string) ([]byte, string, error) {
	pub, err := s.GetPublicPublicationByID(ctx, idOrRef)
	if err != nil {
		return nil, "", err
	}

	scopeStr := "Nigeria (Nationwide)"
	if pub.StateName != nil {
		scopeStr = *pub.StateName + " State"
	}

	csvData := CSVReportData{
		Title:           pub.Title,
		PublicReference: pub.PublicReference,
		ReportType:      pub.ReportType,
		ReportingPeriod: pub.ReportingPeriod,
		Scope:           scopeStr,
		PublicationDate: pub.PublishedAt,
		DataHash:        pub.DataHash,
	}

	switch pub.ReportType {
	case "cleanups":
		var cData PublicCleanupsData
		_ = json.Unmarshal(pub.AggregatedData, &cData)
		csvData.Headers = []string{"State", "Cleanups Completed", "Waste Collected (kg)", "Verified Attendees"}
		for _, sb := range cData.StateBreakdown {
			csvData.Rows = append(csvData.Rows, []string{
				sb.StateName,
				fmt.Sprintf("%d", sb.CleanupsCount),
				fmt.Sprintf("%d", sb.WasteCollectedKg),
				fmt.Sprintf("%d", sb.VerifiedAttendees),
			})
		}
		if len(csvData.Rows) == 0 {
			csvData.Rows = append(csvData.Rows, []string{
				"All Nigeria",
				fmt.Sprintf("%d", cData.CompletedCleanupsCount),
				fmt.Sprintf("%d", cData.TotalWasteCollectedKg),
				fmt.Sprintf("%d", cData.VerifiedAttendeesCount),
			})
		}

	case "finance":
		var fData PublicFinanceData
		_ = json.Unmarshal(pub.AggregatedData, &fData)
		csvData.Headers = []string{"Metric / Category", "Amount (NGN)", "Expense Count", "Notes"}
		csvData.Rows = append(csvData.Rows,
			[]string{"Total Approved Income", fData.ApprovedIncomeFormatted, "-", "Reconciled ledger income"},
			[]string{"Total Approved Expenditure", fData.ApprovedExpenditureFormatted, "-", "Reconciled ledger expenditure"},
			[]string{"Net Surplus / Deficit", fData.NetSurplusDeficitFormatted, "-", "Net cash movement"},
			[]string{"Total Approved Budget", fData.TotalApprovedBudgetFormatted, "-", "Approved budget allocations"},
		)
		for _, cat := range fData.SpendingCategories {
			csvData.Rows = append(csvData.Rows, []string{
				cat.CategoryLabel,
				cat.AmountFormatted,
				fmt.Sprintf("%d", cat.ExpenseCount),
				"Approved operational category",
			})
		}

	case "community_projects":
		var pData PublicCommunityProjectsData
		_ = json.Unmarshal(pub.AggregatedData, &pData)
		csvData.Headers = []string{"Reference", "Title", "Category", "State", "LGA", "Status", "Approved Budget", "Recorded Expenditure", "Progress (%)"}
		for _, pr := range pData.Projects {
			csvData.Rows = append(csvData.Rows, []string{
				pr.Reference,
				pr.Title,
				pr.Category,
				pr.StateName,
				pr.LGAName,
				pr.ExecutionStatus,
				pr.ApprovedBudgetFormatted,
				pr.RecordedExpenditureFormatted,
				fmt.Sprintf("%d%%", pr.ProgressPercentage),
			})
		}

	case "foundation":
		var fndData PublicFoundationData
		_ = json.Unmarshal(pub.AggregatedData, &fndData)
		csvData.Headers = []string{"Metric", "Value", "Status", "Notes"}
		csvData.Rows = append(csvData.Rows,
			[]string{"Total Verified Donations", fndData.TotalDonationsFormatted, "Verified", "Charitable donations"},
			[]string{"Donations Count", fmt.Sprintf("%d", fndData.VerifiedDonationsCount), "Verified", "Completed transactions"},
			[]string{"Legal Registration", "Provisional", "Pending CAC", fndData.ProvisionalEntityNotice},
			[]string{"Fund Segregation", "Active", "Verified", fndData.StrictSegregationNotice},
		)

	default: // national_overview
		var oData PublicOverviewData
		_ = json.Unmarshal(pub.AggregatedData, &oData)
		csvData.Headers = []string{"Indicator", "Verified Total", "Unit", "Verification Status"}
		csvData.Rows = append(csvData.Rows,
			[]string{"Completed Cleanups", fmt.Sprintf("%d", oData.CompletedCleanupsCount), "Activities", "Verified"},
			[]string{"Verified Attendees", fmt.Sprintf("%d", oData.VerifiedAttendeesCount), "Attendees", "Verified"},
			[]string{"Unique Participants", fmt.Sprintf("%d", oData.UniqueParticipantsCount), "Members", "Verified"},
			[]string{"Active States", fmt.Sprintf("%d", oData.ActiveStatesCount), "States", "Verified"},
			[]string{"Active LGAs", fmt.Sprintf("%d", oData.ActiveLGAsCount), "LGAs", "Verified"},
			[]string{"Approved Community Projects", fmt.Sprintf("%d", oData.ApprovedProjectsCount), "Projects", "Verified"},
			[]string{"Completed Community Projects", fmt.Sprintf("%d", oData.CompletedProjectsCount), "Projects", "Verified"},
			[]string{"Approved Financial Income", oData.ApprovedIncomeFormatted, "NGN", "Approved Ledger"},
			[]string{"Approved Financial Expenditure", oData.ApprovedExpenditureFormatted, "NGN", "Approved Ledger"},
			[]string{"Foundation Donations", oData.FoundationDonationsFormatted, "NGN", "Approved Ledger (Segregated)"},
			[]string{"Waste Collected", fmt.Sprintf("%d", oData.TotalWasteCollectedKg), "kg", "Verified Field Data"},
		)
	}

	csvBytes, err := GenerateCSV(csvData)
	if err != nil {
		return nil, "", fmt.Errorf("failed generating csv: %w", err)
	}

	filename := fmt.Sprintf("Ratels-Report-%s.csv", pub.PublicReference)
	return csvBytes, filename, nil
}

// -----------------------------------------------------------------------
// Admin Queries (Snapshots, Approvals, Audit)
// -----------------------------------------------------------------------

func (s *ReportService) ListSnapshotsAdmin(ctx context.Context, status, reportType, scopeType, stateID *string, page, pageSize int32) (*PaginatedSnapshotsResponse, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	var stPg, rtPg, scPg, sidPg pgtype.Text
	if status != nil && *status != "" {
		stPg = pgtype.Text{String: *status, Valid: true}
	}
	if reportType != nil && *reportType != "" {
		rtPg = pgtype.Text{String: *reportType, Valid: true}
	}
	if scopeType != nil && *scopeType != "" {
		scPg = pgtype.Text{String: *scopeType, Valid: true}
	}
	if stateID != nil && *stateID != "" {
		sidPg = pgtype.Text{String: *stateID, Valid: true}
	}

	rows, err := s.repo.Queries().ListReportSnapshotsAdmin(ctx, db.ListReportSnapshotsAdminParams{
		Status:      stPg,
		ReportType:  rtPg,
		ScopeType:   scPg,
		StateID:     sidPg,
		LimitCount:  pageSize,
		OffsetCount: offset,
	})
	if err != nil {
		return nil, err
	}

	totalCount, err := s.repo.Queries().CountReportSnapshotsAdmin(ctx, db.CountReportSnapshotsAdminParams{
		Status:     stPg,
		ReportType: rtPg,
		ScopeType:  scPg,
		StateID:    sidPg,
	})
	if err != nil {
		return nil, err
	}

	var dtos []AdminSnapshotDetailDTO
	for _, r := range rows {
		item := AdminSnapshotDetailDTO{
			SnapshotID:         r.ID,
			PublicReference:    r.PublicReference,
			ReportType:         r.ReportType,
			Title:              r.Title,
			ReportingPeriod:    r.ReportingPeriod,
			PeriodStart:        r.PeriodStart.Time.Format("2006-01-02"),
			PeriodEnd:          r.PeriodEnd.Time.Format("2006-01-02"),
			ScopeType:          r.ScopeType,
			Status:             r.Status,
			PublicationVersion: int(r.PublicationVersion),
			DataHash:           r.DataHash,
			CreatedByUserID:    r.CreatedByUserID,
			CreatorName:        r.CreatorName,
			AggregatedData:     r.AggregatedData,
			CreatedAt:          r.CreatedAt.Time,
			UpdatedAt:          r.UpdatedAt.Time,
		}
		if r.DefinitionID.Valid {
			item.DefinitionID = &r.DefinitionID.String
		}
		if r.ZoneID.Valid {
			item.ZoneID = &r.ZoneID.String
		}
		if r.ZoneName.Valid {
			item.ZoneName = &r.ZoneName.String
		}
		if r.StateID.Valid {
			item.StateID = &r.StateID.String
		}
		if r.StateName.Valid {
			item.StateName = &r.StateName.String
		}
		if r.LgaID.Valid {
			item.LGAID = &r.LgaID.String
		}
		if r.LgaName.Valid {
			item.LGAName = &r.LgaName.String
		}
		if r.CorrectionNotes.Valid {
			item.CorrectionNotes = &r.CorrectionNotes.String
		}
		dtos = append(dtos, item)
	}

	return &PaginatedSnapshotsResponse{
		Snapshots:  dtos,
		TotalCount: totalCount,
		Page:       page,
		PageSize:   pageSize,
	}, nil
}

func (s *ReportService) GetSnapshotDetailAdmin(ctx context.Context, snapshotID string) (*AdminSnapshotDetailDTO, error) {
	r, err := s.repo.Queries().GetReportSnapshotByID(ctx, snapshotID)
	if err != nil {
		return nil, ErrReportSnapshotNotFound
	}

	approvals, _ := s.repo.Queries().ListReportApprovalsBySnapshot(ctx, snapshotID)
	var approvalItems []ReportApprovalItem
	for _, ap := range approvals {
		approvalItems = append(approvalItems, ReportApprovalItem{
			ID:              ap.ID,
			ReviewerID:      ap.ReviewerID,
			ReviewerName:    ap.ReviewerName,
			ReviewerEmail:   ap.ReviewerEmail.String,
			Decision:        ap.Decision,
			DecisionReason:  ap.DecisionReason,
			GovernanceScope: ap.GovernanceScope,
			CreatedAt:       ap.CreatedAt.Time,
		})
	}

	audits, _ := s.repo.Queries().ListReportAudits(ctx, db.ListReportAuditsParams{
		SnapshotID:  pgtype.Text{String: snapshotID, Valid: true},
		LimitCount:  50,
		OffsetCount: 0,
	})
	var auditItems []ReportAuditItem
	for _, a := range audits {
		ai := ReportAuditItem{
			ID:        a.ID,
			Action:    a.Action,
			Details:   a.Details,
			CreatedAt: a.CreatedAt.Time,
		}
		if a.SnapshotID.Valid {
			ai.SnapshotID = &a.SnapshotID.String
		}
		if a.PerformedByUserID.Valid {
			ai.PerformedByID = &a.PerformedByUserID.String
		}
		if a.ActorName.Valid {
			ai.ActorName = &a.ActorName.String
		}
		if a.ActorEmail.Valid {
			ai.ActorEmail = &a.ActorEmail.String
		}
		if a.IpAddress.Valid {
			ai.IPAddress = &a.IpAddress.String
		}
		auditItems = append(auditItems, ai)
	}

	dto := &AdminSnapshotDetailDTO{
		SnapshotID:         r.ID,
		PublicReference:    r.PublicReference,
		ReportType:         r.ReportType,
		Title:              r.Title,
		ReportingPeriod:    r.ReportingPeriod,
		PeriodStart:        r.PeriodStart.Time.Format("2006-01-02"),
		PeriodEnd:          r.PeriodEnd.Time.Format("2006-01-02"),
		ScopeType:          r.ScopeType,
		Status:             r.Status,
		PublicationVersion: int(r.PublicationVersion),
		DataHash:           r.DataHash,
		CreatedByUserID:    r.CreatedByUserID,
		CreatorName:        r.CreatorName,
		AggregatedData:     r.AggregatedData,
		CreatedAt:          r.CreatedAt.Time,
		UpdatedAt:          r.UpdatedAt.Time,
		Approvals:          approvalItems,
		AuditLogs:          auditItems,
	}
	if r.DefinitionID.Valid {
		dto.DefinitionID = &r.DefinitionID.String
	}
	if r.ZoneID.Valid {
		dto.ZoneID = &r.ZoneID.String
	}
	if r.ZoneName.Valid {
		dto.ZoneName = &r.ZoneName.String
	}
	if r.StateID.Valid {
		dto.StateID = &r.StateID.String
	}
	if r.StateName.Valid {
		dto.StateName = &r.StateName.String
	}
	if r.LgaID.Valid {
		dto.LGAID = &r.LgaID.String
	}
	if r.LgaName.Valid {
		dto.LGAName = &r.LgaName.String
	}
	if r.CorrectionNotes.Valid {
		dto.CorrectionNotes = &r.CorrectionNotes.String
	}

	return dto, nil
}

func (s *ReportService) ListReportAudits(ctx context.Context, snapshotID *string, page, pageSize int32) ([]ReportAuditItem, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 50
	}
	offset := (page - 1) * pageSize

	var snapPg pgtype.Text
	if snapshotID != nil && *snapshotID != "" {
		snapPg = pgtype.Text{String: *snapshotID, Valid: true}
	}

	rows, err := s.repo.Queries().ListReportAudits(ctx, db.ListReportAuditsParams{
		SnapshotID:  snapPg,
		LimitCount:  pageSize,
		OffsetCount: offset,
	})
	if err != nil {
		return nil, err
	}

	var items []ReportAuditItem
	for _, a := range rows {
		ai := ReportAuditItem{
			ID:        a.ID,
			Action:    a.Action,
			Details:   a.Details,
			CreatedAt: a.CreatedAt.Time,
		}
		if a.SnapshotID.Valid {
			ai.SnapshotID = &a.SnapshotID.String
		}
		if a.PerformedByUserID.Valid {
			ai.PerformedByID = &a.PerformedByUserID.String
		}
		if a.ActorName.Valid {
			ai.ActorName = &a.ActorName.String
		}
		if a.ActorEmail.Valid {
			ai.ActorEmail = &a.ActorEmail.String
		}
		if a.IpAddress.Valid {
			ai.IPAddress = &a.IpAddress.String
		}
		items = append(items, ai)
	}

	return items, nil
}

// -----------------------------------------------------------------------
// Direct Public Category Aggregation Handlers
// -----------------------------------------------------------------------

func parsePeriodDates(period string) (time.Time, time.Time, string) {
	now := time.Now()
	pLabel := period
	if pLabel == "" || pLabel == "all" || pLabel == "all_time" {
		pLabel = "All Time"
		return time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), now, pLabel
	}

	// Try Year e.g. "2026"
	if len(period) == 4 {
		year, err := time.Parse("2006", period)
		if err == nil {
			start := time.Date(year.Year(), 1, 1, 0, 0, 0, 0, time.UTC)
			end := time.Date(year.Year(), 12, 31, 23, 59, 59, 0, time.UTC)
			return start, end, period
		}
	}

	// Try Year-Month e.g. "2026-10"
	if len(period) == 7 {
		ym, err := time.Parse("2006-01", period)
		if err == nil {
			start := time.Date(ym.Year(), ym.Month(), 1, 0, 0, 0, 0, time.UTC)
			end := start.AddDate(0, 1, -1).Add(23*time.Hour + 59*time.Minute + 59*time.Second)
			return start, end, period
		}
	}

	return time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), now, pLabel
}

func (s *ReportService) GetPublicOverview(ctx context.Context, scopeType, zoneID, stateID, lgaID *string, period string) (*PublicOverviewData, error) {
	scope := "national"
	if scopeType != nil && *scopeType != "" {
		scope = *scopeType
	}

	requestedPeriod := period
	if requestedPeriod == "" || requestedPeriod == "all" || requestedPeriod == "all_time" {
		requestedPeriod = "All Time"
	}

	hasID := func(id *string) bool {
		return id != nil && *id != ""
	}

	switch scope {
	case "national":
		if hasID(zoneID) || hasID(stateID) || hasID(lgaID) {
			return nil, errors.New("national overview cannot include geographic IDs")
		}
	case "zone":
		if !hasID(zoneID) || hasID(stateID) || hasID(lgaID) {
			return nil, errors.New("zone overview requires only zone_id")
		}
	case "state":
		if !hasID(stateID) || hasID(lgaID) {
			return nil, errors.New("state overview requires state_id")
		}
	case "lga":
		if !hasID(lgaID) {
			return nil, errors.New("lga overview requires lga_id")
		}
	default:
		return nil, fmt.Errorf("unsupported overview scope: %s", scope)
	}

	toNullableText := func(id *string) pgtype.Text {
		if !hasID(id) {
			return pgtype.Text{}
		}
		return pgtype.Text{
			String: *id,
			Valid:  true,
		}
	}

	snapshot, err := s.repo.Queries().GetPublishedOverviewByScope(
		ctx,
		db.GetPublishedOverviewByScopeParams{
			ScopeType:       scope,
			ReportingPeriod: requestedPeriod,
			ZoneID:          toNullableText(zoneID),
			StateID:         toNullableText(stateID),
			LgaID:           toNullableText(lgaID),
		},
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrReportPublicationNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve published overview: %w", err)
	}

	var data PublicOverviewData
	if err := json.Unmarshal(snapshot, &data); err != nil {
		return nil, fmt.Errorf("invalid published overview snapshot: %w", err)
	}

	return &data, nil
}

func (s *ReportService) GetPublicCleanups(ctx context.Context, scopeType, zoneID, stateID, lgaID *string, period string) (*PublicCleanupsData, error) {
	pStart, pEnd, pLabel := parsePeriodDates(period)
	scType := "national"
	if scopeType != nil && *scopeType != "" {
		scType = *scopeType
	}

	dataBytes, err := s.AggregateSourceData(ctx, "cleanups", scType, zoneID, stateID, lgaID, pStart, pEnd, pLabel)
	if err != nil {
		return nil, err
	}

	var data PublicCleanupsData
	if err := json.Unmarshal(dataBytes, &data); err != nil {
		return nil, err
	}
	return &data, nil
}

func (s *ReportService) GetPublicParticipation(ctx context.Context, scopeType, zoneID, stateID, lgaID *string, period string) (*PublicParticipationData, error) {
	pStart, pEnd, pLabel := parsePeriodDates(period)
	scType := "national"
	if scopeType != nil && *scopeType != "" {
		scType = *scopeType
	}

	dataBytes, err := s.AggregateSourceData(ctx, "participation", scType, zoneID, stateID, lgaID, pStart, pEnd, pLabel)
	if err != nil {
		return nil, err
	}

	var data PublicParticipationData
	if err := json.Unmarshal(dataBytes, &data); err != nil {
		return nil, err
	}
	return &data, nil
}

func (s *ReportService) GetPublicFinance(ctx context.Context, scopeType, stateID *string, period string) (*PublicFinanceData, error) {
	scope := "national"
	if scopeType != nil && *scopeType != "" {
		scope = *scopeType
	}

	if stateID != nil && *stateID != "" {
		if scope != "national" && scope != "state" {
			return nil, fmt.Errorf("unsupported financial report scope: %s", scope)
		}
		scope = "state"
	} else if scope != "national" {
		return nil, errors.New("state_id is required for state financial reports")
	}

	requestedPeriod := period
	if requestedPeriod == "" || requestedPeriod == "all" || requestedPeriod == "all_time" {
		requestedPeriod = "All Time"
	}

	// Filter by exact state ID at the database level.
	// National reports must not inherit a state filter.
	filterStateID := ""
	if scope == "state" && stateID != nil {
		filterStateID = *stateID
	}

	publications, err := s.ListPublicPublications(
		ctx, "finance", scope, filterStateID, requestedPeriod, 1, 100,
	)
	if err != nil {
		return nil, err
	}

	for _, publication := range publications.Publications {
		// ListPublicPublications already filters to published records.
		// Require exact geographical scope, including state identity.
		if publication.ScopeType != scope {
			continue
		}

		if scope == "state" {
			if stateID == nil || publication.StateName == nil {
				continue
			}

			// Resolve the snapshot's exact state ID before accepting it.
			row, err := s.repo.Queries().GetPublishedReportByID(ctx, publication.PublicationID)
			if err != nil || !row.StateID.Valid || row.StateID.String != *stateID {
				continue
			}
		} else {
			row, err := s.repo.Queries().GetPublishedReportByID(ctx, publication.PublicationID)
			if err != nil || row.StateID.Valid || row.ZoneID.Valid || row.LgaID.Valid {
				continue
			}
		}

		if publication.ReportingPeriod != requestedPeriod {
			continue
		}

		var data PublicFinanceData
		if err := json.Unmarshal(publication.AggregatedData, &data); err != nil {
			return nil, fmt.Errorf("invalid published finance snapshot: %w", err)
		}

		return &data, nil
	}

	return nil, ErrReportPublicationNotFound
}

func (s *ReportService) GetPublicFoundation(ctx context.Context, period string) (*PublicFoundationData, error) {
	requestedPeriod := period
	if requestedPeriod == "" || requestedPeriod == "all" || requestedPeriod == "all_time" {
		requestedPeriod = "All Time"
	}

	publications, err := s.ListPublicPublications(
		ctx, "foundation", "national", "", requestedPeriod, 1, 100,
	)
	if err != nil {
		return nil, err
	}

	for _, publication := range publications.Publications {
		if publication.ScopeType != "national" ||
			publication.ReportingPeriod != requestedPeriod {
			continue
		}

		// Confirm the publication belongs to the national scope,
		// without zone, state or LGA restrictions.
		row, err := s.repo.Queries().GetPublishedReportByID(
			ctx, publication.PublicationID,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to verify published foundation report: %w", err)
		}

		if row.ZoneID.Valid || row.StateID.Valid || row.LgaID.Valid {
			continue
		}

		var data PublicFoundationData
		if err := json.Unmarshal(publication.AggregatedData, &data); err != nil {
			return nil, fmt.Errorf("invalid published foundation snapshot: %w", err)
		}

		return &data, nil
	}

	return nil, ErrReportPublicationNotFound
}

func (s *ReportService) GetPublicCommunityProjects(ctx context.Context, stateID, lgaID *string, period string) (*PublicCommunityProjectsData, error) {
	pStart, pEnd, pLabel := parsePeriodDates(period)

	scope := "national"
	if lgaID != nil && *lgaID != "" {
		scope = "lga"
	} else if stateID != nil && *stateID != "" {
		scope = "state"
	}

	dataBytes, err := s.AggregateSourceData(ctx, "community_projects", scope, nil, stateID, lgaID, pStart, pEnd, pLabel)
	if err != nil {
		return nil, err
	}

	var data PublicCommunityProjectsData
	if err := json.Unmarshal(dataBytes, &data); err != nil {
		return nil, err
	}
	return &data, nil
}

func (s *ReportService) GetSettingsDefinitions(ctx context.Context) ([]db.PublicReportDefinition, error) {
	return s.repo.Queries().ListReportDefinitions(ctx)
}
