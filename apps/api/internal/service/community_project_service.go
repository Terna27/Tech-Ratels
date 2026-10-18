package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"ratels-api/internal/db"
	"ratels-api/internal/repository"
	"ratels-api/internal/security"
	"ratels-api/internal/storage"
)

var (
	ErrCommunityRequestNotFound       = errors.New("community request not found")
	ErrCommunityProjectNotFound       = errors.New("community project not found")
	ErrProjectMilestoneNotFound       = errors.New("project milestone not found")
	ErrDuplicateProjectConversion     = errors.New("DUPLICATE_CONVERSION: This community request has already been converted into a managed project")
	ErrInvalidRequestStatusTransition = errors.New("invalid community request status transition")
	ErrInvalidProjectExecutionStatus  = errors.New("invalid community project execution status")
	ErrUnverifiedMilestonesRemaining  = errors.New("UNVERIFIED_MILESTONES: Cannot mark project completed until all completed milestones have been verified by an independent officer")
	ErrMissingJustification           = errors.New("a documented reason or justification is strictly required for this decision")
	ErrInvalidCategory                = errors.New("invalid or unsupported community category specified")
	ErrInvalidUrgency                 = errors.New("invalid urgency level specified")
)

var ValidCommunityCategories = map[string]string{
	"environmental_sanitation": "Environmental Sanitation & Cleanliness",
	"water_and_boreholes":      "Water Supply & Boreholes",
	"waste_management":         "Waste Management & Disposal",
	"drainage_and_flooding":    "Drainage & Flood Prevention",
	"public_infrastructure":    "Public Infrastructure & Roads",
	"community_health_hygiene": "Community Health & Hygiene",
	"education_and_awareness":  "Civic Education & Awareness",
	"other":                    "Other Community Needs",
}

var ValidUrgencyLevels = map[string]bool{
	"low":      true,
	"medium":   true,
	"high":     true,
	"critical": true,
}

type CommunityProjectService struct {
	repo    *repository.Repository
	audit   *AuditService
	storage storage.StorageService
	finance *FinanceService
}

func NewCommunityProjectService(
	repo *repository.Repository,
	audit *AuditService,
	storage storage.StorageService,
	finance *FinanceService,
) *CommunityProjectService {
	return &CommunityProjectService{
		repo:    repo,
		audit:   audit,
		storage: storage,
		finance: finance,
	}
}

// -------------------------------------------------------------
// Request DTOs
// -------------------------------------------------------------

type CreateCommunityRequestInput struct {
	Title             string   `json:"title"`
	Description       string   `json:"description"`
	Category          string   `json:"category"`
	StateID           string   `json:"state_id"`
	LGAID             string   `json:"lga_id"`
	CommunityWard     string   `json:"community_ward"`
	LocationDetails   string   `json:"location_details"`
	EstimatedUrgency  string   `json:"estimated_urgency"`
	EvidenceImageURLs []string `json:"evidence_image_urls"`
}

type ReviewRequestInput struct {
	Decision         string  `json:"decision"` // verify, approve, reject, request_info, escalate
	ReviewNotes      *string `json:"review_notes"`
	RejectionReason  *string `json:"rejection_reason"`
	InfoRequestNotes *string `json:"info_request_notes"`
	EscalationLevel  *string `json:"escalation_level"` // lga, state, zonal, national
}

type ResubmitRequestInput struct {
	Description       string   `json:"description"`
	LocationDetails   string   `json:"location_details"`
	EvidenceImageURLs []string `json:"evidence_image_urls"`
}

type AdminRequestFilter struct {
	StateID  *string `json:"state_id"`
	LGAID    *string `json:"lga_id"`
	Category *string `json:"category"`
	Status   *string `json:"status"`
	Urgency  *string `json:"urgency"`
	Page     int32   `json:"page"`
	Limit    int32   `json:"limit"`
}

// -------------------------------------------------------------
// Project DTOs
// -------------------------------------------------------------

type CreateProjectInput struct {
	CommunityRequestID    *string    `json:"community_request_id"`
	Title                 string     `json:"title"`
	Description           string     `json:"description"`
	Category              string     `json:"category"`
	StateID               string     `json:"state_id"`
	LGAID                 string     `json:"lga_id"`
	CommunityWard         string     `json:"community_ward"`
	LocationDetails       string     `json:"location_details"`
	ResponsibleOfficerID  *string    `json:"responsible_officer_id"`
	ResponsibleDepartment string     `json:"responsible_department"`
	StartDate             *time.Time `json:"start_date"`
	TargetCompletionDate  *time.Time `json:"target_completion_date"`
	ProposedBudgetKobo    int64      `json:"proposed_budget"`
	ApprovedBudgetKobo    int64      `json:"approved_budget"`
	IsPublic              bool       `json:"is_public"`
}

func (c *CreateProjectInput) UnmarshalJSON(data []byte) error {
	type Alias CreateProjectInput
	aux := &struct {
		StartDateRaw            *string `json:"start_date"`
		TargetCompletionDateRaw *string `json:"target_completion_date"`
		IsPublicRaw             *bool   `json:"is_public"`
		*Alias
	}{
		Alias: (*Alias)(c),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	c.IsPublic = true
	if aux.IsPublicRaw != nil {
		c.IsPublic = *aux.IsPublicRaw
	}
	if aux.StartDateRaw != nil && *aux.StartDateRaw != "" {
		t, err := ParseFlexibleDate(*aux.StartDateRaw)
		if err != nil {
			return err
		}
		c.StartDate = t
	}
	if aux.TargetCompletionDateRaw != nil && *aux.TargetCompletionDateRaw != "" {
		t, err := ParseFlexibleDate(*aux.TargetCompletionDateRaw)
		if err != nil {
			return err
		}
		c.TargetCompletionDate = t
	}
	return nil
}

type UpdateProjectDetailsInput struct {
	Title                 string     `json:"title"`
	Description           string     `json:"description"`
	Category              string     `json:"category"`
	StateID               string     `json:"state_id"`
	LGAID                 string     `json:"lga_id"`
	CommunityWard         string     `json:"community_ward"`
	LocationDetails       string     `json:"location_details"`
	ResponsibleOfficerID  *string    `json:"responsible_officer_id"`
	ResponsibleDepartment string     `json:"responsible_department"`
	StartDate             *time.Time `json:"start_date"`
	TargetCompletionDate  *time.Time `json:"target_completion_date"`
	IsPublic              bool       `json:"is_public"`
}

func (u *UpdateProjectDetailsInput) UnmarshalJSON(data []byte) error {
	type Alias UpdateProjectDetailsInput
	aux := &struct {
		StartDateRaw            *string `json:"start_date"`
		TargetCompletionDateRaw *string `json:"target_completion_date"`
		*Alias
	}{
		Alias: (*Alias)(u),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	if aux.StartDateRaw != nil && *aux.StartDateRaw != "" {
		t, err := ParseFlexibleDate(*aux.StartDateRaw)
		if err != nil {
			return err
		}
		u.StartDate = t
	}
	if aux.TargetCompletionDateRaw != nil && *aux.TargetCompletionDateRaw != "" {
		t, err := ParseFlexibleDate(*aux.TargetCompletionDateRaw)
		if err != nil {
			return err
		}
		u.TargetCompletionDate = t
	}
	return nil
}

type UpdateProjectExecutionInput struct {
	ExecutionStatus      string     `json:"execution_status"` // planned, in_progress, awaiting_verification, completed, paused, cancelled
	ProgressPercentage   int32      `json:"progress_percentage"`
	ActualCompletionDate *time.Time `json:"actual_completion_date"`
	PauseOrCancelReason  *string    `json:"pause_or_cancel_reason"`
	VerificationNotes    *string    `json:"verification_notes"`
	CompletionEvidence   []string   `json:"completion_evidence_urls"`
}

func (u *UpdateProjectExecutionInput) UnmarshalJSON(data []byte) error {
	type Alias UpdateProjectExecutionInput
	aux := &struct {
		ActualCompletionDateRaw *string `json:"actual_completion_date"`
		*Alias
	}{
		Alias: (*Alias)(u),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	if aux.ActualCompletionDateRaw != nil && *aux.ActualCompletionDateRaw != "" {
		t, err := ParseFlexibleDate(*aux.ActualCompletionDateRaw)
		if err != nil {
			return err
		}
		u.ActualCompletionDate = t
	}
	return nil
}

type PublicProjectFilter struct {
	StateID         *string `json:"state_id"`
	LGAID           *string `json:"lga_id"`
	Category        *string `json:"category"`
	ExecutionStatus *string `json:"execution_status"`
	Page            int32   `json:"page"`
	Limit           int32   `json:"limit"`
}

type AdminProjectFilter struct {
	StateID         *string `json:"state_id"`
	LGAID           *string `json:"lga_id"`
	Category        *string `json:"category"`
	ExecutionStatus *string `json:"execution_status"`
	FundingStatus   *string `json:"funding_status"`
	Page            int32   `json:"page"`
	Limit           int32   `json:"limit"`
}

// -------------------------------------------------------------
// Milestone DTOs
// -------------------------------------------------------------

type AddMilestoneInput struct {
	SequenceOrder      int32      `json:"sequence_order"`
	Title              string     `json:"title"`
	Description        string     `json:"description"`
	TargetDate         time.Time  `json:"target_date"`
	AssignedPersonName *string    `json:"assigned_person_name"`
	AssignedPersonID   *string    `json:"assigned_person_id"`
	Status             string     `json:"status"` // planned, in_progress, awaiting_verification, completed
	ProgressNotes      *string    `json:"progress_notes"`
	EvidencePhotoURLs  []string   `json:"evidence_photo_urls"`
}

func (m *AddMilestoneInput) UnmarshalJSON(data []byte) error {
	type Alias AddMilestoneInput
	aux := &struct {
		TargetDateRaw string `json:"target_date"`
		*Alias
	}{
		Alias: (*Alias)(m),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	if aux.TargetDateRaw != "" {
		t, err := ParseFlexibleDate(aux.TargetDateRaw)
		if err != nil {
			return err
		}
		if t != nil {
			m.TargetDate = *t
		}
	}
	return nil
}

type UpdateMilestoneInput struct {
	SequenceOrder      int32      `json:"sequence_order"`
	Title              string     `json:"title"`
	Description        string     `json:"description"`
	TargetDate         time.Time  `json:"target_date"`
	CompletionDate     *time.Time `json:"completion_date"`
	AssignedPersonName *string    `json:"assigned_person_name"`
	AssignedPersonID   *string    `json:"assigned_person_id"`
	Status             string     `json:"status"`
	ProgressNotes      *string    `json:"progress_notes"`
	EvidencePhotoURLs  []string   `json:"evidence_photo_urls"`
}

func (m *UpdateMilestoneInput) UnmarshalJSON(data []byte) error {
	type Alias UpdateMilestoneInput
	aux := &struct {
		TargetDateRaw     string  `json:"target_date"`
		CompletionDateRaw *string `json:"completion_date"`
		*Alias
	}{
		Alias: (*Alias)(m),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	if aux.TargetDateRaw != "" {
		t, err := ParseFlexibleDate(aux.TargetDateRaw)
		if err != nil {
			return err
		}
		if t != nil {
			m.TargetDate = *t
		}
	}
	if aux.CompletionDateRaw != nil && *aux.CompletionDateRaw != "" {
		t, err := ParseFlexibleDate(*aux.CompletionDateRaw)
		if err != nil {
			return err
		}
		m.CompletionDate = t
	}
	return nil
}

type VerifyMilestoneInput struct {
	Decision          string  `json:"decision"` // verified, rejected
	VerificationNotes string  `json:"verification_notes"`
}

type ProjectFundingInput struct {
	Action         string  `json:"action"` // allocate, expense, disburse
	AmountKobo     int64   `json:"amount_kobo"`
	Reference      string  `json:"reference"`
	IdempotencyKey string  `json:"idempotency_key"`
	EvidenceURL    *string `json:"evidence_url"`
	Notes          *string `json:"notes"`
}

type PresignedCommunityUploadResponse struct {
	UploadURL        string `json:"upload_url"`
	ObjectKey        string `json:"object_key"`
	PublicURL        string `json:"public_url"`
	ContentType      string `json:"content_type"`
	MaxSizeBytes     int64  `json:"max_size_bytes"`
	ExpiresInSeconds int64  `json:"expires_in_seconds"`
}

// -------------------------------------------------------------
// Community Requests Methods
// -------------------------------------------------------------

func (s *CommunityProjectService) CreateRequest(ctx context.Context, requesterID string, req CreateCommunityRequestInput) (*db.CommunityRequest, error) {
	if strings.TrimSpace(req.Title) == "" {
		return nil, errors.New("request title is required")
	}
	if strings.TrimSpace(req.Description) == "" {
		return nil, errors.New("request description and justification are required")
	}
	if _, ok := ValidCommunityCategories[req.Category]; !ok {
		return nil, fmt.Errorf("%w: %s", ErrInvalidCategory, req.Category)
	}
	if req.EstimatedUrgency == "" {
		req.EstimatedUrgency = "medium"
	}
	if !ValidUrgencyLevels[req.EstimatedUrgency] {
		return nil, fmt.Errorf("%w: %s", ErrInvalidUrgency, req.EstimatedUrgency)
	}

	// Verify State & LGA existence
	_, err := s.repo.Queries().GetStateByID(ctx, req.StateID)
	if err != nil {
		return nil, errors.New("invalid state specified")
	}
	_, err = s.repo.Queries().GetLGAByID(ctx, req.LGAID)
	if err != nil {
		return nil, errors.New("invalid LGA specified")
	}

	evidenceJSON, err := json.Marshal(req.EvidenceImageURLs)
	if err != nil {
		evidenceJSON = []byte("[]")
	}

	ref := fmt.Sprintf("CR-%d-%s", time.Now().Year(), strings.ToUpper(security.GenerateUUID()[:8]))
	id := security.GenerateUUID()

	created, err := s.repo.Queries().CreateCommunityRequest(ctx, db.CreateCommunityRequestParams{
		ID:                id,
		Reference:         ref,
		Title:             strings.TrimSpace(req.Title),
		Description:       strings.TrimSpace(req.Description),
		Category:          req.Category,
		StateID:           req.StateID,
		LgaID:             req.LGAID,
		CommunityWard:     strings.TrimSpace(req.CommunityWard),
		LocationDetails:   strings.TrimSpace(req.LocationDetails),
		EstimatedUrgency:  req.EstimatedUrgency,
		EvidenceImageUrls: evidenceJSON,
		RequesterID:       requesterID,
		Status:            "submitted",
		EscalationLevel:   TextFromString("state"),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create community request: %w", err)
	}

	details := fmt.Sprintf("Created community request %s (%s) for state %s", ref, req.Title, req.StateID)
	_ = s.audit.Log(ctx, &requesterID, nil, "community_request_created", "community_requests", id, StringPtrFromText(TextFromString("state")), &req.StateID, &details, nil)

	return &created, nil
}

func (s *CommunityProjectService) GetRequestByID(ctx context.Context, actorID, actorRole string, actorJurisdictionID *string, requestID string) (*db.GetCommunityRequestByIDRow, error) {
	req, err := s.repo.Queries().GetCommunityRequestByID(ctx, requestID)
	if err != nil {
		return nil, ErrCommunityRequestNotFound
	}

	// Privacy & scoping checks:
	// A regular member can ONLY view their own submitted request
	if actorRole == "member" {
		if req.RequesterID != actorID {
			return nil, ErrCommunityRequestNotFound
		}
		return &req, nil
	}

	// Administrative scoping
	if actorRole == "state_coordinator" {
		if actorJurisdictionID == nil || *actorJurisdictionID != req.StateID {
			return nil, ErrScopeUnauthorized
		}
	} else if actorRole == "lga_representative" {
		if actorJurisdictionID == nil || *actorJurisdictionID != req.LgaID {
			return nil, ErrScopeUnauthorized
		}
	}

	return &req, nil
}

func (s *CommunityProjectService) ListMemberRequests(ctx context.Context, requesterID string) ([]db.ListCommunityRequestsByRequesterRow, error) {
	return s.repo.Queries().ListCommunityRequestsByRequester(ctx, requesterID)
}

func (s *CommunityProjectService) ListAdminRequests(ctx context.Context, actorID, actorRole string, actorJurisdictionID *string, filter AdminRequestFilter) ([]db.ListAdminCommunityRequestsRow, int64, error) {
	// Enforce geographic scoping
	if actorRole == "state_coordinator" {
		filter.StateID = actorJurisdictionID
	} else if actorRole == "lga_representative" {
		filter.LGAID = actorJurisdictionID
	}

	limit := filter.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	offset := filter.Page * limit
	if offset < 0 {
		offset = 0
	}

	items, err := s.repo.Queries().ListAdminCommunityRequests(ctx, db.ListAdminCommunityRequestsParams{
		Limit:    limit,
		Offset:   offset,
		StateID:  Text(filter.StateID),
		LgaID:    Text(filter.LGAID),
		Category: Text(filter.Category),
		Status:   Text(filter.Status),
		Urgency:  Text(filter.Urgency),
	})
	if err != nil {
		return nil, 0, err
	}

	total, err := s.repo.Queries().CountAdminCommunityRequests(ctx, db.CountAdminCommunityRequestsParams{
		StateID:  Text(filter.StateID),
		LgaID:    Text(filter.LGAID),
		Category: Text(filter.Category),
		Status:   Text(filter.Status),
		Urgency:  Text(filter.Urgency),
	})
	if err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

func (s *CommunityProjectService) ReviewRequest(
	ctx context.Context,
	reviewerID, reviewerRole string,
	reviewerJurisdictionID *string,
	requestID string,
	req ReviewRequestInput,
) (*db.CommunityRequest, error) {
	existing, err := s.repo.Queries().GetCommunityRequestByID(ctx, requestID)
	if err != nil {
		return nil, ErrCommunityRequestNotFound
	}

	// 1. STRICT ANTI-SELF-APPROVAL INVARIANT
	if existing.RequesterID == reviewerID {
		return nil, ErrSelfApprovalNotAllowed
	}

	// 2. Geographic scoping check
	if reviewerRole == "state_coordinator" {
		if reviewerJurisdictionID == nil || *reviewerJurisdictionID != existing.StateID {
			return nil, ErrScopeUnauthorized
		}
	} else if reviewerRole == "lga_representative" {
		if reviewerJurisdictionID == nil || *reviewerJurisdictionID != existing.LgaID {
			return nil, ErrScopeUnauthorized
		}
	} else if reviewerRole != "national_admin" && reviewerRole != "super_admin" {
		return nil, errors.New("unauthorized to review community requests")
	}

	// 3. Status transition mapping
	newStatus := existing.Status
	switch req.Decision {
	case "verify":
		newStatus = "verified"
	case "approve":
		newStatus = "approved"
	case "reject":
		newStatus = "rejected"
		if req.RejectionReason == nil || strings.TrimSpace(*req.RejectionReason) == "" {
			return nil, fmt.Errorf("%w: a documented rejection reason is strictly required", ErrMissingJustification)
		}
	case "request_info":
		newStatus = "info_requested"
		if req.InfoRequestNotes == nil || strings.TrimSpace(*req.InfoRequestNotes) == "" {
			return nil, fmt.Errorf("%w: notes detailing the requested corrections/information are required", ErrMissingJustification)
		}
	case "escalate":
		if req.EscalationLevel == nil || *req.EscalationLevel == "" {
			return nil, errors.New("escalation target level is required")
		}
	default:
		return nil, fmt.Errorf("%w: unsupported decision %q", ErrInvalidRequestStatusTransition, req.Decision)
	}

	now := time.Now()
	reviewed, err := s.repo.Queries().ReviewCommunityRequest(ctx, db.ReviewCommunityRequestParams{
		ID:               requestID,
		Status:           newStatus,
		ReviewedByUserID: TextFromString(reviewerID),
		ReviewedAt:       TimestamptzFromTime(now),
		ReviewNotes:      Text(req.ReviewNotes),
		RejectionReason:  Text(req.RejectionReason),
		InfoRequestNotes: Text(req.InfoRequestNotes),
		EscalationLevel:  Text(req.EscalationLevel),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to update review status: %w", err)
	}

	details := fmt.Sprintf("Reviewed community request %s: decision=%s, status=%s", existing.Reference, req.Decision, newStatus)
	_ = s.audit.Log(ctx, &reviewerID, &reviewerRole, "community_request_reviewed", "community_requests", requestID, StringPtrFromText(TextFromString("state")), &existing.StateID, &details, nil)

	return &reviewed, nil
}

func (s *CommunityProjectService) ResubmitRequest(
	ctx context.Context,
	requesterID, requestID string,
	req ResubmitRequestInput,
) (*db.CommunityRequest, error) {
	existing, err := s.repo.Queries().GetCommunityRequestByID(ctx, requestID)
	if err != nil {
		return nil, ErrCommunityRequestNotFound
	}
	if existing.RequesterID != requesterID {
		return nil, ErrCommunityRequestNotFound
	}
	if existing.Status != "info_requested" {
		return nil, errors.New("only requests in 'info_requested' status can be resubmitted")
	}

	evidenceJSON, err := json.Marshal(req.EvidenceImageURLs)
	if err != nil {
		evidenceJSON = []byte("[]")
	}

	resubmitted, err := s.repo.Queries().ResubmitCommunityRequest(ctx, db.ResubmitCommunityRequestParams{
		ID:                requestID,
		Description:       strings.TrimSpace(req.Description),
		LocationDetails:   strings.TrimSpace(req.LocationDetails),
		EvidenceImageUrls: evidenceJSON,
		RequesterID:       requesterID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to resubmit request: %w", err)
	}

	details := fmt.Sprintf("Member resubmitted community request %s following info request", existing.Reference)
	_ = s.audit.Log(ctx, &requesterID, nil, "community_request_resubmitted", "community_requests", requestID, StringPtrFromText(TextFromString("state")), &existing.StateID, &details, nil)

	return &resubmitted, nil
}

// -------------------------------------------------------------
// Community Projects Methods
// -------------------------------------------------------------

func (s *CommunityProjectService) CreateProject(
	ctx context.Context,
	creatorID, creatorRole string,
	creatorJurisdictionID *string,
	req CreateProjectInput,
) (*db.CommunityProject, error) {
	// Role check: Only national_admin, super_admin, or state_coordinator can create projects
	if creatorRole != "super_admin" && creatorRole != "national_admin" && creatorRole != "state_coordinator" {
		return nil, errors.New("unauthorized to initiate or create community projects")
	}

	if creatorRole == "state_coordinator" {
		if creatorJurisdictionID == nil || *creatorJurisdictionID != req.StateID {
			return nil, ErrScopeUnauthorized
		}
	}

	if strings.TrimSpace(req.Title) == "" {
		return nil, errors.New("project title is required")
	}
	if strings.TrimSpace(req.Description) == "" {
		return nil, errors.New("project description is required")
	}
	if _, ok := ValidCommunityCategories[req.Category]; !ok {
		return nil, fmt.Errorf("%w: %s", ErrInvalidCategory, req.Category)
	}

	// If originating from a request, verify and prevent duplicate project conversion
	var requestIDText pgtype.Text
	if req.CommunityRequestID != nil && *req.CommunityRequestID != "" {
		reqData, err := s.repo.Queries().GetCommunityRequestByID(ctx, *req.CommunityRequestID)
		if err != nil {
			return nil, fmt.Errorf("originating community request not found: %w", err)
		}
		if reqData.ConvertedToProjectID.Valid && reqData.ConvertedToProjectID.String != "" {
			return nil, ErrDuplicateProjectConversion
		}
		requestIDText = TextFromString(*req.CommunityRequestID)
	}

	ref := fmt.Sprintf("CP-%d-%s", time.Now().Year(), strings.ToUpper(security.GenerateUUID()[:8]))
	projectID := security.GenerateUUID()

	var createdProject db.CommunityProject

	err := s.repo.WithTx(ctx, func(q *db.Queries) error {
		p, err := q.CreateCommunityProject(ctx, db.CreateCommunityProjectParams{
			ID:                    projectID,
			Reference:             ref,
			Title:                 strings.TrimSpace(req.Title),
			Description:           strings.TrimSpace(req.Description),
			CommunityRequestID:    requestIDText,
			Category:              req.Category,
			StateID:               req.StateID,
			LgaID:                 req.LGAID,
			CommunityWard:         strings.TrimSpace(req.CommunityWard),
			LocationDetails:       strings.TrimSpace(req.LocationDetails),
			ResponsibleOfficerID:  Text(req.ResponsibleOfficerID),
			ResponsibleDepartment: req.ResponsibleDepartment,
			StartDate:             DateFromTime(req.StartDate),
			TargetCompletionDate:  DateFromTime(req.TargetCompletionDate),
			ProposedBudget:        req.ProposedBudgetKobo,
			ApprovedBudget:        req.ApprovedBudgetKobo,
			AllocatedFunding:      0,
			RecordedExpenditure:   0,
			OutstandingCommitments: 0,
			FundingStatus:         "unfunded",
			ExecutionStatus:       "planned",
			ProgressPercentage:    0,
			SupportingDocuments:   []byte("[]"),
			IsPublic:              req.IsPublic,
			CreatedByUserID:       creatorID,
		})
		if err != nil {
			return err
		}
		createdProject = p

		// If created from request, mark request converted atomically
		if req.CommunityRequestID != nil && *req.CommunityRequestID != "" {
			_, err = q.MarkCommunityRequestConverted(ctx, db.MarkCommunityRequestConvertedParams{
				ID:                   *req.CommunityRequestID,
				ConvertedToProjectID: TextFromString(projectID),
			})
			if err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			return nil, ErrDuplicateProjectConversion
		}
		return nil, fmt.Errorf("failed to create community project: %w", err)
	}

	details := fmt.Sprintf("Created community project %s (%s) for state %s", ref, req.Title, req.StateID)
	_ = s.audit.Log(ctx, &creatorID, &creatorRole, "community_project_created", "community_projects", projectID, StringPtrFromText(TextFromString("state")), &req.StateID, &details, nil)

	return &createdProject, nil
}

func (s *CommunityProjectService) GetPublicProject(ctx context.Context, idOrRef string) (*PublicProjectResponse, error) {
	var project db.GetCommunityProjectByIDRow
	var err error

	if strings.HasPrefix(idOrRef, "CP-") {
		refRow, rErr := s.repo.Queries().GetCommunityProjectByReference(ctx, idOrRef)
		if rErr != nil {
			return nil, ErrCommunityProjectNotFound
		}
		project = db.GetCommunityProjectByIDRow(refRow)
	} else {
		project, err = s.repo.Queries().GetCommunityProjectByID(ctx, idOrRef)
		if err != nil {
			return nil, ErrCommunityProjectNotFound
		}
	}

	if !project.IsPublic {
		return nil, ErrCommunityProjectNotFound
	}

	// Fetch verified milestones
	milestones, err := s.repo.Queries().ListMilestonesByProjectID(ctx, project.ID)
	if err != nil || milestones == nil {
		milestones = []db.ListMilestonesByProjectIDRow{}
	}

	// Fetch public updates
	updates, err := s.repo.Queries().ListPublicUpdatesByProjectID(ctx, project.ID)
	if err != nil || updates == nil {
		updates = []db.ListPublicUpdatesByProjectIDRow{}
	}

	// Public financial summary
	availBalance := project.AllocatedFunding - project.RecordedExpenditure
	if availBalance < 0 {
		availBalance = 0
	}

	return &PublicProjectResponse{
		Project: project,
		FinancialSummary: PublicProjectFinancialSummary{
			ApprovedBudgetKobo:     project.ApprovedBudget,
			AllocatedFundingKobo:   project.AllocatedFunding,
			RecordedExpenditureKobo: project.RecordedExpenditure,
			AvailableBalanceKobo:   availBalance,
			FundingStatus:          project.FundingStatus,
		},
		Milestones: milestones,
		Updates:    updates,
	}, nil
}

func (s *CommunityProjectService) GetAdminProject(
	ctx context.Context,
	actorID, actorRole string,
	actorJurisdictionID *string,
	projectID string,
) (*AdminProjectDetailResponse, error) {
	project, err := s.repo.Queries().GetCommunityProjectByID(ctx, projectID)
	if err != nil {
		return nil, ErrCommunityProjectNotFound
	}

	if actorRole == "state_coordinator" {
		if actorJurisdictionID == nil || *actorJurisdictionID != project.StateID {
			return nil, ErrScopeUnauthorized
		}
	} else if actorRole == "lga_representative" {
		if actorJurisdictionID == nil || *actorJurisdictionID != project.LgaID {
			return nil, ErrScopeUnauthorized
		}
	}

	milestones, err := s.repo.Queries().ListMilestonesByProjectID(ctx, projectID)
	if err != nil || milestones == nil {
		milestones = []db.ListMilestonesByProjectIDRow{}
	}

	updates, err := s.repo.Queries().ListAllUpdatesByProjectID(ctx, projectID)
	if err != nil || updates == nil {
		updates = []db.ListAllUpdatesByProjectIDRow{}
	}

	availBalance := project.AllocatedFunding - project.RecordedExpenditure
	if availBalance < 0 {
		availBalance = 0
	}

	return &AdminProjectDetailResponse{
		Project: project,
		FinancialSummary: AdminProjectFinancialSummary{
			ProposedBudgetKobo:     project.ProposedBudget,
			ApprovedBudgetKobo:     project.ApprovedBudget,
			AllocatedFundingKobo:   project.AllocatedFunding,
			RecordedExpenditureKobo: project.RecordedExpenditure,
			OutstandingCommitmentsKobo: project.OutstandingCommitments,
			AvailableBalanceKobo:   availBalance,
			FundingStatus:          project.FundingStatus,
		},
		Milestones: milestones,
		Updates:    updates,
	}, nil
}

func (s *CommunityProjectService) ListPublicProjects(ctx context.Context, filter PublicProjectFilter) ([]db.ListPublicCommunityProjectsRow, int64, error) {
	limit := filter.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	offset := filter.Page * limit
	if offset < 0 {
		offset = 0
	}

	items, err := s.repo.Queries().ListPublicCommunityProjects(ctx, db.ListPublicCommunityProjectsParams{
		Limit:           limit,
		Offset:          offset,
		StateID:         Text(filter.StateID),
		LgaID:           Text(filter.LGAID),
		Category:        Text(filter.Category),
		ExecutionStatus: Text(filter.ExecutionStatus),
	})
	if err != nil {
		return nil, 0, err
	}

	total, err := s.repo.Queries().CountPublicCommunityProjects(ctx, db.CountPublicCommunityProjectsParams{
		StateID:         Text(filter.StateID),
		LgaID:           Text(filter.LGAID),
		Category:        Text(filter.Category),
		ExecutionStatus: Text(filter.ExecutionStatus),
	})
	if err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

func (s *CommunityProjectService) ListAdminProjects(
	ctx context.Context,
	actorID, actorRole string,
	actorJurisdictionID *string,
	filter AdminProjectFilter,
) ([]db.ListAdminCommunityProjectsRow, int64, error) {
	if actorRole == "state_coordinator" {
		filter.StateID = actorJurisdictionID
	} else if actorRole == "lga_representative" {
		filter.LGAID = actorJurisdictionID
	}

	limit := filter.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	offset := filter.Page * limit
	if offset < 0 {
		offset = 0
	}

	items, err := s.repo.Queries().ListAdminCommunityProjects(ctx, db.ListAdminCommunityProjectsParams{
		Limit:           limit,
		Offset:          offset,
		StateID:         Text(filter.StateID),
		LgaID:           Text(filter.LGAID),
		Category:        Text(filter.Category),
		ExecutionStatus: Text(filter.ExecutionStatus),
		FundingStatus:   Text(filter.FundingStatus),
	})
	if err != nil {
		return nil, 0, err
	}

	total, err := s.repo.Queries().CountAdminCommunityProjects(ctx, db.CountAdminCommunityProjectsParams{
		StateID:         Text(filter.StateID),
		LgaID:           Text(filter.LGAID),
		Category:        Text(filter.Category),
		ExecutionStatus: Text(filter.ExecutionStatus),
		FundingStatus:   Text(filter.FundingStatus),
	})
	if err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

func (s *CommunityProjectService) UpdateProjectDetails(
	ctx context.Context,
	actorID, actorRole string,
	actorJurisdictionID *string,
	projectID string,
	req UpdateProjectDetailsInput,
) (*db.CommunityProject, error) {
	existing, err := s.repo.Queries().GetCommunityProjectByID(ctx, projectID)
	if err != nil {
		return nil, ErrCommunityProjectNotFound
	}

	if actorRole == "state_coordinator" {
		if actorJurisdictionID == nil || *actorJurisdictionID != existing.StateID {
			return nil, ErrScopeUnauthorized
		}
	} else if actorRole != "national_admin" && actorRole != "super_admin" {
		return nil, errors.New("unauthorized to edit community projects")
	}

	updated, err := s.repo.Queries().UpdateCommunityProjectDetails(ctx, db.UpdateCommunityProjectDetailsParams{
		ID:                    projectID,
		Title:                 strings.TrimSpace(req.Title),
		Description:           strings.TrimSpace(req.Description),
		Category:              req.Category,
		StateID:               req.StateID,
		LgaID:                 req.LGAID,
		CommunityWard:         strings.TrimSpace(req.CommunityWard),
		LocationDetails:       strings.TrimSpace(req.LocationDetails),
		ResponsibleOfficerID:  Text(req.ResponsibleOfficerID),
		ResponsibleDepartment: req.ResponsibleDepartment,
		StartDate:             DateFromTime(req.StartDate),
		TargetCompletionDate:  DateFromTime(req.TargetCompletionDate),
		IsPublic:              req.IsPublic,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to update project details: %w", err)
	}

	details := fmt.Sprintf("Updated details for community project %s", existing.Reference)
	_ = s.audit.Log(ctx, &actorID, &actorRole, "community_project_updated", "community_projects", projectID, StringPtrFromText(TextFromString("state")), &existing.StateID, &details, nil)

	return &updated, nil
}

func (s *CommunityProjectService) UpdateProjectExecution(
	ctx context.Context,
	actorID, actorRole string,
	actorJurisdictionID *string,
	projectID string,
	req UpdateProjectExecutionInput,
) (*db.CommunityProject, error) {
	existing, err := s.repo.Queries().GetCommunityProjectByID(ctx, projectID)
	if err != nil {
		return nil, ErrCommunityProjectNotFound
	}

	if actorRole == "state_coordinator" {
		if actorJurisdictionID == nil || *actorJurisdictionID != existing.StateID {
			return nil, ErrScopeUnauthorized
		}
	} else if actorRole != "national_admin" && actorRole != "super_admin" {
		return nil, errors.New("unauthorized to update project execution status")
	}

	// Strict Completion Gate:
	// If marking as completed, verify that no unverified completed milestones exist
	if req.ExecutionStatus == "completed" {
		unverifiedCount, cErr := s.repo.Queries().CountUnverifiedCompletedMilestones(ctx, projectID)
		if cErr == nil && unverifiedCount > 0 {
			return nil, fmt.Errorf("%w: %d completed milestone(s) are pending official verification", ErrUnverifiedMilestonesRemaining, unverifiedCount)
		}

		evidenceJSON, _ := json.Marshal(req.CompletionEvidence)
		now := time.Now()
		completed, err := s.repo.Queries().VerifyCommunityProjectCompletion(ctx, db.VerifyCommunityProjectCompletionParams{
			ID:                     projectID,
			ActualCompletionDate:   DateFromTime(&now),
			CompletionEvidenceUrls: evidenceJSON,
			VerifiedByUserID:       TextFromString(actorID),
			VerifiedAt:             TimestamptzFromTime(now),
			VerificationNotes:      Text(req.VerificationNotes),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to complete project: %w", err)
		}

		details := fmt.Sprintf("Verified completion of community project %s", existing.Reference)
		_ = s.audit.Log(ctx, &actorID, &actorRole, "community_project_completed", "community_projects", projectID, StringPtrFromText(TextFromString("state")), &existing.StateID, &details, nil)
		return &completed, nil
	}

	// Paused or Cancelled require a reason
	if req.ExecutionStatus == "paused" || req.ExecutionStatus == "cancelled" {
		if req.PauseOrCancelReason == nil || strings.TrimSpace(*req.PauseOrCancelReason) == "" {
			return nil, fmt.Errorf("%w: a documented reason is strictly required when pausing or cancelling a project", ErrMissingJustification)
		}
	}

	updated, err := s.repo.Queries().UpdateCommunityProjectExecution(ctx, db.UpdateCommunityProjectExecutionParams{
		ID:                  projectID,
		ExecutionStatus:     req.ExecutionStatus,
		ProgressPercentage:  req.ProgressPercentage,
		ActualCompletionDate: DateFromTime(req.ActualCompletionDate),
		PauseOrCancelReason: Text(req.PauseOrCancelReason),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to update project execution: %w", err)
	}

	details := fmt.Sprintf("Updated execution of project %s: status=%s, progress=%d%%", existing.Reference, req.ExecutionStatus, req.ProgressPercentage)
	_ = s.audit.Log(ctx, &actorID, &actorRole, "community_project_progress_updated", "community_projects", projectID, StringPtrFromText(TextFromString("state")), &existing.StateID, &details, nil)

	return &updated, nil
}

// -------------------------------------------------------------
// Milestone Methods
// -------------------------------------------------------------

func (s *CommunityProjectService) AddMilestone(
	ctx context.Context,
	actorID, actorRole string,
	actorJurisdictionID *string,
	projectID string,
	req AddMilestoneInput,
) (*db.CommunityProjectMilestone, error) {
	project, err := s.repo.Queries().GetCommunityProjectByID(ctx, projectID)
	if err != nil {
		return nil, ErrCommunityProjectNotFound
	}

	if actorRole == "state_coordinator" {
		if actorJurisdictionID == nil || *actorJurisdictionID != project.StateID {
			return nil, ErrScopeUnauthorized
		}
	} else if actorRole != "national_admin" && actorRole != "super_admin" {
		return nil, errors.New("unauthorized to manage milestones")
	}

	if strings.TrimSpace(req.Title) == "" {
		return nil, errors.New("milestone title is required")
	}
	if req.TargetDate.IsZero() {
		return nil, errors.New("target date is required")
	}

	evidenceJSON, _ := json.Marshal(req.EvidencePhotoURLs)
	if req.Status == "" {
		req.Status = "planned"
	}
	if req.SequenceOrder <= 0 {
		req.SequenceOrder = 1
	}

	milestoneID := security.GenerateUUID()
	created, err := s.repo.Queries().CreateProjectMilestone(ctx, db.CreateProjectMilestoneParams{
		ID:                 milestoneID,
		ProjectID:          projectID,
		SequenceOrder:      req.SequenceOrder,
		Title:              strings.TrimSpace(req.Title),
		Description:        strings.TrimSpace(req.Description),
		TargetDate:         DateFromTime(&req.TargetDate),
		CompletionDate:     pgtype.Date{Valid: false},
		AssignedPersonName: Text(req.AssignedPersonName),
		AssignedPersonID:   Text(req.AssignedPersonID),
		Status:             req.Status,
		ProgressNotes:      Text(req.ProgressNotes),
		EvidencePhotoUrls:  evidenceJSON,
		VerificationStatus: "unverified",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create milestone: %w", err)
	}

	details := fmt.Sprintf("Added milestone %s to project %s", req.Title, project.Reference)
	_ = s.audit.Log(ctx, &actorID, &actorRole, "project_milestone_added", "community_project_milestones", milestoneID, StringPtrFromText(TextFromString("state")), &project.StateID, &details, nil)

	return &created, nil
}

func (s *CommunityProjectService) UpdateMilestone(
	ctx context.Context,
	actorID, actorRole string,
	actorJurisdictionID *string,
	milestoneID string,
	req UpdateMilestoneInput,
) (*db.CommunityProjectMilestone, error) {
	existing, err := s.repo.Queries().GetProjectMilestoneByID(ctx, milestoneID)
	if err != nil {
		return nil, ErrProjectMilestoneNotFound
	}

	seq := req.SequenceOrder
	if seq <= 0 {
		seq = existing.SequenceOrder
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = existing.Title
	}
	desc := strings.TrimSpace(req.Description)
	if desc == "" {
		desc = existing.Description
	}
	targetDate := DateFromTime(&req.TargetDate)
	if !targetDate.Valid {
		targetDate = existing.TargetDate
	}
	status := strings.TrimSpace(req.Status)
	if status == "" {
		status = existing.Status
	}

	var assignedPersonName pgtype.Text
	if req.AssignedPersonName != nil {
		assignedPersonName = Text(req.AssignedPersonName)
	} else {
		assignedPersonName = existing.AssignedPersonName
	}

	var assignedPersonID pgtype.Text
	if req.AssignedPersonID != nil {
		assignedPersonID = Text(req.AssignedPersonID)
	} else {
		assignedPersonID = existing.AssignedPersonID
	}

	var progressNotes pgtype.Text
	if req.ProgressNotes != nil {
		progressNotes = Text(req.ProgressNotes)
	} else {
		progressNotes = existing.ProgressNotes
	}

	var completionDate pgtype.Date
	if req.CompletionDate != nil {
		completionDate = DateFromTime(req.CompletionDate)
	} else {
		completionDate = existing.CompletionDate
	}

	evidenceJSON := existing.EvidencePhotoUrls
	if len(req.EvidencePhotoURLs) > 0 {
		evidenceJSON, _ = json.Marshal(req.EvidencePhotoURLs)
	}

	updated, err := s.repo.Queries().UpdateProjectMilestone(ctx, db.UpdateProjectMilestoneParams{
		ID:                 milestoneID,
		SequenceOrder:      seq,
		Title:              title,
		Description:        desc,
		TargetDate:         targetDate,
		CompletionDate:     completionDate,
		AssignedPersonName: assignedPersonName,
		AssignedPersonID:   assignedPersonID,
		Status:             status,
		ProgressNotes:      progressNotes,
		EvidencePhotoUrls:  evidenceJSON,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to update milestone: %w", err)
	}

	return &updated, nil
}

func (s *CommunityProjectService) VerifyMilestone(
	ctx context.Context,
	verifierID, verifierRole string,
	verifierJurisdictionID *string,
	milestoneID string,
	req VerifyMilestoneInput,
) (*db.CommunityProjectMilestone, error) {
	milestone, err := s.repo.Queries().GetProjectMilestoneByID(ctx, milestoneID)
	if err != nil {
		return nil, ErrProjectMilestoneNotFound
	}

	// 1. ANTI-SELF-APPROVAL INVARIANT: Assigned officer cannot verify their own milestone
	if milestone.AssignedPersonID.Valid && milestone.AssignedPersonID.String == verifierID {
		return nil, ErrSelfApprovalNotAllowed
	}

	// 2. Role authorization check
	if verifierRole != "super_admin" && verifierRole != "national_admin" && verifierRole != "state_coordinator" {
		return nil, errors.New("unauthorized to verify milestones")
	}

	if strings.TrimSpace(req.VerificationNotes) == "" {
		return nil, fmt.Errorf("%w: verification notes documenting the inspection are required", ErrMissingJustification)
	}

	verificationStatus := "verified"
	milestoneStatus := milestone.Status
	now := time.Now()
	var compDate pgtype.Date

	if req.Decision == "rejected" {
		verificationStatus = "rejected"
	} else {
		milestoneStatus = "completed"
		compDate = DateFromTime(&now)
	}

	verified, err := s.repo.Queries().VerifyProjectMilestone(ctx, db.VerifyProjectMilestoneParams{
		ID:                 milestoneID,
		VerificationStatus: verificationStatus,
		VerifiedByUserID:   TextFromString(verifierID),
		VerifiedAt:         TimestamptzFromTime(now),
		VerificationNotes:  TextFromString(req.VerificationNotes),
		Status:             milestoneStatus,
		CompletionDate:     compDate,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to record milestone verification: %w", err)
	}

	details := fmt.Sprintf("Recorded milestone verification: decision=%s, milestone=%s", req.Decision, milestone.Title)
	_ = s.audit.Log(ctx, &verifierID, &verifierRole, "project_milestone_verified", "community_project_milestones", milestoneID, nil, nil, &details, nil)

	return &verified, nil
}

func (s *CommunityProjectService) DeleteMilestone(
	ctx context.Context,
	actorID, actorRole string,
	actorJurisdictionID *string,
	milestoneID string,
) error {
	milestone, err := s.repo.Queries().GetProjectMilestoneByID(ctx, milestoneID)
	if err != nil {
		return ErrProjectMilestoneNotFound
	}

	if actorRole != "super_admin" && actorRole != "national_admin" {
		return errors.New("only national and super administrators can remove milestones")
	}

	err = s.repo.Queries().DeleteProjectMilestone(ctx, milestoneID)
	if err != nil {
		return fmt.Errorf("failed to delete milestone: %w", err)
	}

	details := fmt.Sprintf("Deleted milestone %s from project %s", milestone.Title, milestone.ProjectID)
	_ = s.audit.Log(ctx, &actorID, &actorRole, "project_milestone_deleted", "community_project_milestones", milestoneID, nil, nil, &details, nil)

	return nil
}

// -------------------------------------------------------------
// Financial Management Integration
// -------------------------------------------------------------

func (s *CommunityProjectService) RecordProjectFunding(
	ctx context.Context,
	actorID, actorRole string,
	projectID string,
	req ProjectFundingInput,
) (*db.CommunityProject, error) {
	project, err := s.repo.Queries().GetCommunityProjectByID(ctx, projectID)
	if err != nil {
		return nil, ErrCommunityProjectNotFound
	}

	// Strictly requires financial clearance
	if actorRole != "super_admin" && actorRole != "national_admin" {
		hasAccess, _, _ := s.finance.CheckFinancialAccess(ctx, actorID, actorRole)
		if !hasAccess {
			return nil, ErrFinancialUnauthorized
		}
	}

	if req.AmountKobo <= 0 {
		return nil, errors.New("monetary amount must be greater than zero")
	}

	var newAllocated = project.AllocatedFunding
	var newExpended = project.RecordedExpenditure

	now := time.Now()
	ref := req.Reference
	if ref == "" {
		ref = fmt.Sprintf("FIN-CP-%d-%s", time.Now().Year(), strings.ToUpper(security.GenerateUUID()[:8]))
	}
	idempotency := req.IdempotencyKey
	if idempotency == "" {
		idempotency = fmt.Sprintf("idem-%s-%d", ref, time.Now().UnixNano())
	}

	projectFundID := "project_fund"
	genFundID := "general_operations_fund"

	switch req.Action {
	case "allocate":
		// Moves funds from general clearing/reserve into project fund
		_, err = s.finance.RecordTransaction(ctx, actorID, actorRole, CreateTransactionRequest{
			Reference:             ref,
			IdempotencyKey:        idempotency,
			TransactionType:       "project_allocation",
			Amount:                req.AmountKobo,
			Currency:              "NGN",
			SourceFundID:          &genFundID,
			DestinationFundID:     &projectFundID,
			RelatedProjectID:      &projectID,
			StateID:               &project.StateID,
			SupportingEvidenceURL: req.EvidenceURL,
			Notes:                 req.Notes,
			OccurredAt:            now,
			AutoPost:              true,
		})
		if err != nil {
			return nil, fmt.Errorf("financial ledger allocation failed: %w", err)
		}
		newAllocated += req.AmountKobo

	case "expense", "disburse":
		// Disburses from project fund
		_, err = s.finance.RecordTransaction(ctx, actorID, actorRole, CreateTransactionRequest{
			Reference:             ref,
			IdempotencyKey:        idempotency,
			TransactionType:       "expense",
			Amount:                req.AmountKobo,
			Currency:              "NGN",
			SourceFundID:          &projectFundID,
			RelatedProjectID:      &projectID,
			StateID:               &project.StateID,
			SupportingEvidenceURL: req.EvidenceURL,
			Notes:                 req.Notes,
			OccurredAt:            now,
			AutoPost:              true,
		})
		if err != nil {
			return nil, fmt.Errorf("financial ledger expenditure record failed: %w", err)
		}
		newExpended += req.AmountKobo

	default:
		return nil, fmt.Errorf("invalid funding action %q: must be 'allocate' or 'expense'", req.Action)
	}

	// Update funding status
	fundingStatus := "unfunded"
	if newAllocated > 0 {
		if project.ApprovedBudget > 0 && newAllocated >= project.ApprovedBudget {
			fundingStatus = "fully_funded"
		} else {
			fundingStatus = "partially_funded"
		}
	}

	updated, err := s.repo.Queries().UpdateCommunityProjectFinances(ctx, db.UpdateCommunityProjectFinancesParams{
		ID:                     projectID,
		ProposedBudget:         project.ProposedBudget,
		ApprovedBudget:         project.ApprovedBudget,
		AllocatedFunding:       newAllocated,
		RecordedExpenditure:    newExpended,
		OutstandingCommitments: project.OutstandingCommitments,
		FundingStatus:          fundingStatus,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to update project financial balances: %w", err)
	}

	details := fmt.Sprintf("Recorded project financial transaction %s (%s) for %d Kobo", ref, req.Action, req.AmountKobo)
	_ = s.audit.Log(ctx, &actorID, &actorRole, "community_project_funding_updated", "community_projects", projectID, StringPtrFromText(TextFromString("state")), &project.StateID, &details, nil)

	return &updated, nil
}

// -------------------------------------------------------------
// Cloudflare R2 Evidence Storage
// -------------------------------------------------------------

func (s *CommunityProjectService) AuthorizeRequestImageUpload(
	ctx context.Context,
	userID string,
	filename, contentType string,
	fileSize int64,
) (*PresignedCommunityUploadResponse, error) {
	canonicalExt, cleanContentType, err := storage.ValidateUploadRequest(filename, contentType, fileSize)
	if err != nil {
		return nil, err
	}

	tempRequestID := security.GenerateUUID()
	objectKey := storage.GenerateCommunityRequestImageKey(tempRequestID, canonicalExt)

	expiry := 15 * time.Minute
	uploadURL, err := s.storage.GeneratePresignedUpload(ctx, objectKey, cleanContentType, storage.MaxProductImageSize, expiry)
	if err != nil {
		return nil, fmt.Errorf("failed to authorize R2 upload: %w", err)
	}

	publicURL := s.storage.GetPublicURL(objectKey)

	return &PresignedCommunityUploadResponse{
		UploadURL:        uploadURL,
		ObjectKey:        objectKey,
		PublicURL:        publicURL,
		ContentType:      cleanContentType,
		MaxSizeBytes:     storage.MaxProductImageSize,
		ExpiresInSeconds: int64(expiry.Seconds()),
	}, nil
}

func (s *CommunityProjectService) UploadRequestImageDirect(
	ctx context.Context,
	userID, requestID string,
	filename string,
	reader io.Reader,
	size int64,
) (string, string, error) {
	if requestID == "" {
		requestID = security.GenerateUUID()
	}

	data, meta, err := storage.InspectAndValidateImage(reader, storage.MaxProductImageSize)
	if err != nil {
		return "", "", err
	}

	ext := ".jpg"
	if strings.Contains(meta.ContentType, "png") {
		ext = ".png"
	} else if strings.Contains(meta.ContentType, "webp") {
		ext = ".webp"
	}

	objectKey := storage.GenerateCommunityRequestImageKey(requestID, ext)
	publicURL, err := s.storage.UploadObject(ctx, objectKey, meta.ContentType, strings.NewReader(string(data)), meta.Size)
	if err != nil {
		return "", "", fmt.Errorf("failed to upload image to storage: %w", err)
	}

	return objectKey, publicURL, nil
}

func (s *CommunityProjectService) AuthorizeProjectImageUpload(
	ctx context.Context,
	userID, userRole, projectID string,
	filename, contentType string,
	fileSize int64,
) (*PresignedCommunityUploadResponse, error) {
	canonicalExt, cleanContentType, err := storage.ValidateUploadRequest(filename, contentType, fileSize)
	if err != nil {
		return nil, err
	}

	if projectID == "" {
		projectID = security.GenerateUUID()
	}
	objectKey := storage.GenerateCommunityProjectImageKey(projectID, canonicalExt)

	expiry := 15 * time.Minute
	uploadURL, err := s.storage.GeneratePresignedUpload(ctx, objectKey, cleanContentType, storage.MaxProductImageSize, expiry)
	if err != nil {
		return nil, fmt.Errorf("failed to authorize R2 upload: %w", err)
	}

	publicURL := s.storage.GetPublicURL(objectKey)

	return &PresignedCommunityUploadResponse{
		UploadURL:        uploadURL,
		ObjectKey:        objectKey,
		PublicURL:        publicURL,
		ContentType:      cleanContentType,
		MaxSizeBytes:     storage.MaxProductImageSize,
		ExpiresInSeconds: int64(expiry.Seconds()),
	}, nil
}

func (s *CommunityProjectService) UploadProjectImageDirect(
	ctx context.Context,
	userID, userRole, projectID string,
	filename string,
	reader io.Reader,
	size int64,
) (string, string, error) {
	if projectID == "" {
		projectID = security.GenerateUUID()
	}

	data, meta, err := storage.InspectAndValidateImage(reader, storage.MaxProductImageSize)
	if err != nil {
		return "", "", err
	}

	ext := ".jpg"
	if strings.Contains(meta.ContentType, "png") {
		ext = ".png"
	} else if strings.Contains(meta.ContentType, "webp") {
		ext = ".webp"
	}

	objectKey := storage.GenerateCommunityProjectImageKey(projectID, ext)
	publicURL, err := s.storage.UploadObject(ctx, objectKey, meta.ContentType, strings.NewReader(string(data)), meta.Size)
	if err != nil {
		return "", "", fmt.Errorf("failed to upload project image: %w", err)
	}

	return objectKey, publicURL, nil
}

// -------------------------------------------------------------
// Project Updates & Overview Stats
// -------------------------------------------------------------

type CreateProjectUpdateInput struct {
	MilestoneID        *string  `json:"milestone_id"`
	Title              string   `json:"title"`
	Content            string   `json:"content"`
	PhotoURLs          []string `json:"photo_urls"`
	ProgressPercentage *int32   `json:"progress_percentage"`
	IsPublic           bool     `json:"is_public"`
}

func (u *CreateProjectUpdateInput) UnmarshalJSON(data []byte) error {
	type Alias CreateProjectUpdateInput
	aux := &struct {
		IsPublicRaw *bool `json:"is_public"`
		*Alias
	}{
		Alias: (*Alias)(u),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	u.IsPublic = true
	if aux.IsPublicRaw != nil {
		u.IsPublic = *aux.IsPublicRaw
	}
	return nil
}

func (s *CommunityProjectService) CreateProjectUpdate(
	ctx context.Context,
	authorID, authorRole string,
	projectID string,
	req CreateProjectUpdateInput,
) (*db.CommunityProjectUpdate, error) {
	project, err := s.repo.Queries().GetCommunityProjectByID(ctx, projectID)
	if err != nil {
		return nil, ErrCommunityProjectNotFound
	}

	if strings.TrimSpace(req.Title) == "" || strings.TrimSpace(req.Content) == "" {
		return nil, errors.New("update title and content are required")
	}

	photosJSON, _ := json.Marshal(req.PhotoURLs)

	var progInt pgtype.Int4
	if req.ProgressPercentage != nil {
		progInt = pgtype.Int4{Int32: *req.ProgressPercentage, Valid: true}
	}

	updateID := security.GenerateUUID()
	created, err := s.repo.Queries().CreateProjectUpdate(ctx, db.CreateProjectUpdateParams{
		ID:                 updateID,
		ProjectID:          projectID,
		MilestoneID:        Text(req.MilestoneID),
		AuthorID:           authorID,
		Title:              strings.TrimSpace(req.Title),
		Content:            strings.TrimSpace(req.Content),
		PhotoUrls:          photosJSON,
		ProgressPercentage: progInt,
		IsPublic:           req.IsPublic,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to record project update: %w", err)
	}

	details := fmt.Sprintf("Posted progress update on project %s: %s", project.Reference, req.Title)
	_ = s.audit.Log(ctx, &authorID, &authorRole, "project_update_posted", "community_project_updates", updateID, StringPtrFromText(TextFromString("state")), &project.StateID, &details, nil)

	return &created, nil
}

func (s *CommunityProjectService) GetOverviewStats(ctx context.Context) (*db.GetCommunityProjectsOverviewStatsRow, error) {
	stats, err := s.repo.Queries().GetCommunityProjectsOverviewStats(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch overview statistics: %w", err)
	}
	return &stats, nil
}

// -------------------------------------------------------------
// Response Models
// -------------------------------------------------------------

type PublicProjectFinancialSummary struct {
	ApprovedBudgetKobo      int64  `json:"approved_budget_kobo"`
	AllocatedFundingKobo    int64  `json:"allocated_funding_kobo"`
	RecordedExpenditureKobo int64  `json:"recorded_expenditure_kobo"`
	AvailableBalanceKobo    int64  `json:"available_balance_kobo"`
	FundingStatus           string `json:"funding_status"`
}

type AdminProjectFinancialSummary struct {
	ProposedBudgetKobo         int64  `json:"proposed_budget_kobo"`
	ApprovedBudgetKobo         int64  `json:"approved_budget_kobo"`
	AllocatedFundingKobo       int64  `json:"allocated_funding_kobo"`
	RecordedExpenditureKobo    int64  `json:"recorded_expenditure_kobo"`
	OutstandingCommitmentsKobo int64  `json:"outstanding_commitments_kobo"`
	AvailableBalanceKobo       int64  `json:"available_balance_kobo"`
	FundingStatus              string `json:"funding_status"`
}

type PublicProjectResponse struct {
	Project          db.GetCommunityProjectByIDRow    `json:"project"`
	FinancialSummary PublicProjectFinancialSummary     `json:"financial_summary"`
	Milestones       []db.ListMilestonesByProjectIDRow `json:"milestones"`
	Updates          []db.ListPublicUpdatesByProjectIDRow `json:"updates"`
}

type AdminProjectDetailResponse struct {
	Project          db.GetCommunityProjectByIDRow    `json:"project"`
	FinancialSummary AdminProjectFinancialSummary      `json:"financial_summary"`
	Milestones       []db.ListMilestonesByProjectIDRow `json:"milestones"`
	Updates          []db.ListAllUpdatesByProjectIDRow `json:"updates"`
}
