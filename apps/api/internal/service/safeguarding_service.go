package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"ratels-api/internal/config"
	"ratels-api/internal/db"
	"ratels-api/internal/repository"
	"ratels-api/internal/security"
	"ratels-api/internal/storage"
)

var (
	ErrSafeguardingDisabled          = errors.New("safeguarding and confidential incident reporting is currently undergoing administrative readiness review and is unavailable")
	ErrUnauthorizedSafeguardingAccess = errors.New("unauthorized: safeguarding permissions required")
	ErrConflictOfInterest            = errors.New("access denied: you have a recorded conflict of interest or are named in this case")
	ErrCaseNotFound                  = errors.New("safeguarding case not found")
	ErrInvalidFollowUpCredentials    = errors.New("invalid case reference or follow-up secret")
	ErrInvalidIncidentCategory       = errors.New("unrecognized incident category")
	ErrDescriptionRequired           = errors.New("incident description is required")
	ErrOfficerHasConflict            = errors.New("cannot assign case to an officer who is implicated or has a conflict of interest")
	ErrOfficerNotFound               = errors.New("safeguarding officer not found")
)

var validCategories = map[string]bool{
	"harassment":                true,
	"bullying":                  true,
	"bullying_and_intimidation": true,
	"discrimination":            true,
	"threats_intimidation":      true,
	"physical_safety":           true,
	"physical_assault":          true,
	"safety_hazard":             true,
	"sexual_misconduct":         true,
	"sexual_harassment":         true,
	"abuse_exploitation":        true,
	"child_safeguarding":        true,
	"minor_safeguarding":        true,
	"official_misconduct":       true,
	"financial_misconduct":      true,
	"leadership_misconduct":     true,
	"retaliation":               true,
	"general_grievance":         true,
	"privacy_violation":         true,
	"other":                     true,
}

func normalizeCategory(cat string) string {
	switch strings.TrimSpace(strings.ToLower(cat)) {
	case "minor_safeguarding":
		return "child_safeguarding"
	case "sexual_harassment":
		return "sexual_misconduct"
	case "bullying_and_intimidation":
		return "bullying"
	case "physical_assault", "safety_hazard":
		return "physical_safety"
	case "financial_misconduct", "leadership_misconduct":
		return "official_misconduct"
	case "retaliation":
		return "threats_intimidation"
	case "general_grievance":
		return "other"
	default:
		return strings.TrimSpace(strings.ToLower(cat))
	}
}

// SafeguardingService encapsulates all business logic for confidential incident reporting and case management.
type SafeguardingService struct {
	repo     *repository.Repository
	cipher   *security.SafeguardingCipher
	storage  storage.SafeguardingStorageService
	notifSvc *NotificationService
	cfg      *config.Config
}

func NewSafeguardingService(
	repo *repository.Repository,
	cipher *security.SafeguardingCipher,
	storage storage.SafeguardingStorageService,
	notifSvc *NotificationService,
	cfg *config.Config,
) *SafeguardingService {
	return &SafeguardingService{
		repo:     repo,
		cipher:   cipher,
		storage:  storage,
		notifSvc: notifSvc,
		cfg:      cfg,
	}
}

// -------------------------------------------------------------
// DTOs
// -------------------------------------------------------------

type SubmitReportInput struct {
	Category          string
	Description       string
	IncidentDate      *time.Time
	StateID           *string
	LGAID             *string
	RelatedActivity   *string
	PersonInvolved    *string
	IsImmediateDanger bool
	IsMinorInvolved   bool
	IsAnonymous       bool
	ReporterUserID    *string
	ReporterName      *string
	ReporterEmail     *string
	ReporterPhone     *string
	ContactPreference string // "email", "sms", "in_app", "none"

	// Optional evidence attachment
	EvidenceFileName    string
	EvidenceContentType string
	EvidenceReader      io.Reader
	EvidenceSize        int64
}

type SubmitReportResult struct {
	CaseID            string `json:"case_id"`
	PublicReference   string `json:"public_reference"`
	FollowUpSecret    string `json:"follow_up_secret"`
	Status            string `json:"status"`
	Category          string `json:"category"`
	RiskLevel         string `json:"risk_level"`
	IsImmediateDanger bool   `json:"is_immediate_danger"`
	IsMinorInvolved   bool   `json:"is_minor_involved"`
	CreatedAt         string `json:"created_at"`
	AdvisoryNotice    string `json:"advisory_notice"`
}

type ReporterCaseView struct {
	PublicReference   string                `json:"public_reference"`
	Category          string                `json:"category"`
	Status            string                `json:"status"`
	RiskLevel         string                `json:"risk_level"`
	IsImmediateDanger bool                  `json:"is_immediate_danger"`
	CreatedAt         string                `json:"created_at"`
	ResolutionSummary *string               `json:"resolution_summary,omitempty"`
	Messages          []ReporterMessageDTO  `json:"messages"`
}

type ReporterMessageDTO struct {
	ID         string    `json:"id"`
	AuthorType string    `json:"author_type"` // "reporter", "officer", "system"
	Content    string    `json:"content"`
	CreatedAt  time.Time `json:"created_at"`
}

type CaseFilter struct {
	Status            string
	RiskLevel         string
	Category          string
	IsMinorInvolved   *bool
	IsImmediateDanger *bool
	Page              int32
	Limit             int32
}

type CaseSummaryDTO struct {
	ID                  string     `json:"id"`
	PublicReference     string     `json:"public_reference"`
	Category            string     `json:"category"`
	IncidentDate        *time.Time `json:"incident_date,omitempty"`
	StateName           *string    `json:"state_name,omitempty"`
	LGAName             *string    `json:"lga_name,omitempty"`
	RelatedActivity     *string    `json:"related_activity,omitempty"`
	PersonInvolved      *string    `json:"person_involved,omitempty"`
	IsImmediateDanger   bool       `json:"is_immediate_danger"`
	IsMinorInvolved     bool       `json:"is_minor_involved"`
	RiskLevel           string     `json:"risk_level"`
	Status              string     `json:"status"`
	IsEscalated         bool       `json:"is_escalated"`
	EscalatedTo         *string    `json:"escalated_to,omitempty"`
	IsAnonymous         bool       `json:"is_anonymous"`
	AssignedOfficerID   *string    `json:"assigned_officer_id,omitempty"`
	AssignedOfficerName *string    `json:"assigned_officer_name,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

type CaseListResult struct {
	Items      []CaseSummaryDTO `json:"items"`
	Total      int64            `json:"total"`
	Page       int32            `json:"page"`
	Limit      int32            `json:"limit"`
	TotalPages int32            `json:"total_pages"`
}

type AdminCaseDetail struct {
	CaseSummaryDTO
	DecryptedDescription string                   `json:"decrypted_description"`
	Reporter             *AdminReporterDTO        `json:"reporter,omitempty"`
	Evidence             []AdminEvidenceDTO       `json:"evidence"`
	Assignments          []AdminAssignmentDTO     `json:"assignments"`
	Conflicts            []AdminConflictDTO       `json:"conflicts"`
	Messages             []AdminMessageDTO        `json:"messages"`
	Actions              []AdminCaseActionDTO     `json:"actions"`
	Retention            *AdminRetentionDTO       `json:"retention,omitempty"`
}

type AdminReporterDTO struct {
	IsAnonymous       bool    `json:"is_anonymous"`
	ReporterUserID    *string `json:"reporter_user_id,omitempty"`
	DecryptedName     *string `json:"name,omitempty"`
	DecryptedEmail    *string `json:"email,omitempty"`
	DecryptedPhone    *string `json:"phone,omitempty"`
	ContactPreference string  `json:"contact_preference"`
}

type AdminEvidenceDTO struct {
	ID                      string    `json:"id"`
	FileName                string    `json:"file_name"`
	MimeType                string    `json:"mime_type"`
	FileSizeBytes           int64     `json:"file_size_bytes"`
	Sha256Hash              string    `json:"sha256_hash"`
	RetentionClassification string    `json:"retention_classification"`
	CreatedAt               time.Time `json:"created_at"`
}

type AdminAssignmentDTO struct {
	ID               string    `json:"id"`
	OfficerID        string    `json:"officer_id"`
	OfficerName      string    `json:"officer_name"`
	OfficerEmail     string    `json:"officer_email"`
	Status           string    `json:"status"`
	AssignmentReason *string   `json:"assignment_reason,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
}

type AdminConflictDTO struct {
	ID           string    `json:"id"`
	OfficerID    string    `json:"officer_id"`
	OfficerName  string    `json:"officer_name"`
	ConflictType string    `json:"conflict_type"`
	Reason       string    `json:"reason"`
	CreatedAt    time.Time `json:"created_at"`
}

type AdminMessageDTO struct {
	ID         string    `json:"id"`
	AuthorType string    `json:"author_type"`
	AuthorName *string   `json:"author_name,omitempty"`
	Visibility string    `json:"visibility"`
	Content    string    `json:"content"`
	CreatedAt  time.Time `json:"created_at"`
}

type AdminCaseActionDTO struct {
	ID             string    `json:"id"`
	ActionType     string    `json:"action_type"`
	ActorName      *string   `json:"actor_name,omitempty"`
	PreviousStatus *string   `json:"previous_status,omitempty"`
	NewStatus      *string   `json:"new_status,omitempty"`
	Reason         *string   `json:"reason,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

type AdminRetentionDTO struct {
	RetentionCategory string    `json:"retention_category"`
	ReviewDate        time.Time `json:"review_date"`
	IsLegalHold       bool      `json:"is_legal_hold"`
	LegalHoldReason   *string   `json:"legal_hold_reason,omitempty"`
}

// -------------------------------------------------------------
// Public Incident Submission
// -------------------------------------------------------------

func (s *SafeguardingService) SubmitReport(ctx context.Context, input SubmitReportInput) (*SubmitReportResult, error) {
	if s.cfg != nil && !s.cfg.SafeguardingReportingEnabled {
		return nil, ErrSafeguardingDisabled
	}

	rawCat := strings.TrimSpace(strings.ToLower(input.Category))
	if !validCategories[rawCat] {
		return nil, ErrInvalidIncidentCategory
	}
	category := normalizeCategory(rawCat)
	if category == "child_safeguarding" {
		input.IsMinorInvolved = true
	}

	description := strings.TrimSpace(input.Description)
	if description == "" {
		return nil, ErrDescriptionRequired
	}

	// Encrypt description with authenticated encryption
	encryptedDesc, err := s.cipher.Encrypt(description)
	if err != nil {
		return nil, fmt.Errorf("failed to secure incident narrative: %w", err)
	}

	// Encrypt reporter details if provided
	var encName, encEmail, encPhone *string
	if !input.IsAnonymous {
		if input.ReporterName != nil && strings.TrimSpace(*input.ReporterName) != "" {
			enc, err := s.cipher.Encrypt(strings.TrimSpace(*input.ReporterName))
			if err != nil {
				return nil, fmt.Errorf("failed to secure reporter name: %w", err)
			}
			encName = &enc
		}
		if input.ReporterEmail != nil && strings.TrimSpace(*input.ReporterEmail) != "" {
			enc, err := s.cipher.Encrypt(strings.TrimSpace(*input.ReporterEmail))
			if err != nil {
				return nil, fmt.Errorf("failed to secure reporter email: %w", err)
			}
			encEmail = &enc
		}
		if input.ReporterPhone != nil && strings.TrimSpace(*input.ReporterPhone) != "" {
			enc, err := s.cipher.Encrypt(strings.TrimSpace(*input.ReporterPhone))
			if err != nil {
				return nil, fmt.Errorf("failed to secure reporter phone: %w", err)
			}
			encPhone = &enc
		}
	}

	// Determine initial risk level
	riskLevel := "normal"
	if input.IsImmediateDanger {
		riskLevel = "critical"
	} else if input.IsMinorInvolved {
		riskLevel = "high"
	}

	caseID := security.GenerateUUID()
	publicRef := security.GeneratePublicCaseReference()
	secret, err := security.GenerateFollowUpSecret()
	if err != nil {
		return nil, fmt.Errorf("failed to generate follow-up secret: %w", err)
	}
	secretHash := security.HashFollowUpSecret(secret)

	// Persist case
	var incDate pgtype.Timestamptz
	if input.IncidentDate != nil {
		incDate = pgtype.Timestamptz{Time: *input.IncidentDate, Valid: true}
	}

	createdCase, err := s.repo.Queries().CreateSafeguardingCase(ctx, db.CreateSafeguardingCaseParams{
		ID:                   caseID,
		PublicReference:      publicRef,
		Category:             category,
		EncryptedDescription: encryptedDesc,
		IncidentDate:         incDate,
		StateID:              Text(input.StateID),
		LgaID:                Text(input.LGAID),
		RelatedActivity:      Text(input.RelatedActivity),
		PersonInvolved:       Text(input.PersonInvolved),
		IsImmediateDanger:    input.IsImmediateDanger,
		IsMinorInvolved:      input.IsMinorInvolved,
		RiskLevel:            riskLevel,
		Status:               "submitted",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to record safeguarding case: %w", err)
	}

	contactPref := input.ContactPreference
	if contactPref == "" || input.IsAnonymous {
		contactPref = "none"
	}

	// Persist reporter
	_, err = s.repo.Queries().CreateSafeguardingReporter(ctx, db.CreateSafeguardingReporterParams{
		ID:                 security.GenerateUUID(),
		CaseID:             caseID,
		IsAnonymous:        input.IsAnonymous,
		ReporterUserID:     Text(input.ReporterUserID),
		EncryptedName:      Text(encName),
		EncryptedEmail:     Text(encEmail),
		EncryptedPhone:     Text(encPhone),
		ContactPreference:  contactPref,
		FollowUpSecretHash: secretHash,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to record reporter credentials: %w", err)
	}

	// Initial lifecycle audit action
	_, _ = s.repo.Queries().CreateSafeguardingCaseAction(ctx, db.CreateSafeguardingCaseActionParams{
		ID:             security.GenerateUUID(),
		CaseID:         caseID,
		ActorID:        pgtype.Text{Valid: false},
		ActionType:     "created",
		PreviousStatus: pgtype.Text{Valid: false},
		NewStatus:      pgtype.Text{String: "submitted", Valid: true},
		Reason:         Text(strPtr("Case submitted via public reporting portal")),
	})

	// Initial retention record (5 years standard, 7 years for minors)
	retentionYears := 5
	retentionCategory := "standard"
	if input.IsMinorInvolved {
		retentionYears = 7
		retentionCategory = "minors_safeguarding"
	}
	reviewDate := time.Now().AddDate(retentionYears, 0, 0)
	_, _ = s.repo.Queries().CreateSafeguardingRetentionRecord(ctx, db.CreateSafeguardingRetentionRecordParams{
		ID:                security.GenerateUUID(),
		CaseID:            caseID,
		RetentionCategory: retentionCategory,
		ReviewDate:        pgtype.Timestamptz{Time: reviewDate, Valid: true},
		IsLegalHold:       false,
		LegalHoldReason:   pgtype.Text{Valid: false},
	})

	// Evidence attachment if provided
	if input.EvidenceReader != nil && input.EvidenceSize > 0 && input.EvidenceFileName != "" {
		storageKey, sha256Hex, actualSize, err := s.storage.StoreEvidence(
			ctx, caseID, input.EvidenceFileName, input.EvidenceContentType, input.EvidenceReader, input.EvidenceSize,
		)
		if err == nil {
			_, _ = s.repo.Queries().CreateSafeguardingEvidence(ctx, db.CreateSafeguardingEvidenceParams{
				ID:                      security.GenerateUUID(),
				CaseID:                  caseID,
				UploaderType:            "reporter",
				UploaderID:              pgtype.Text{Valid: false},
				StorageKey:              storageKey,
				FileName:                filepathBase(input.EvidenceFileName),
				MimeType:                input.EvidenceContentType,
				FileSizeBytes:           actualSize,
				Sha256Hash:              sha256Hex,
				IsQuarantined:           false,
				RetentionClassification: retentionCategory,
			})
		}
	}

	// Privacy-preserving notification to active safeguarding officers
	s.notifySafeguardingOfficers(ctx, createdCase.PublicReference, riskLevel, input.IsImmediateDanger)

	return &SubmitReportResult{
		CaseID:            caseID,
		PublicReference:   publicRef,
		FollowUpSecret:    secret,
		Status:            createdCase.Status,
		Category:          createdCase.Category,
		RiskLevel:         createdCase.RiskLevel,
		IsImmediateDanger: createdCase.IsImmediateDanger,
		IsMinorInvolved:   createdCase.IsMinorInvolved,
		CreatedAt:         createdCase.CreatedAt.Time.Format(time.RFC3339),
		AdvisoryNotice:    "Store your Case Reference and Follow-up Secret securely. It cannot be recovered if lost.",
	}, nil
}

// -------------------------------------------------------------
// Public Status Lookup & Messaging
// -------------------------------------------------------------

func (s *SafeguardingService) GetReportStatus(ctx context.Context, publicRef, candidateSecret string) (*ReporterCaseView, error) {
	c, err := s.repo.Queries().GetSafeguardingCaseByPublicRef(ctx, strings.TrimSpace(publicRef))
	if err != nil || c.ID == "" {
		return nil, ErrCaseNotFound
	}

	reporter, err := s.repo.Queries().GetSafeguardingReporterByCaseID(ctx, c.ID)
	if err != nil {
		return nil, ErrCaseNotFound
	}

	if !security.VerifyFollowUpSecret(candidateSecret, reporter.FollowUpSecretHash) {
		return nil, ErrInvalidFollowUpCredentials
	}

	msgRows, _ := s.repo.Queries().ListReporterVisibleMessagesByCaseID(ctx, c.ID)
	var messages []ReporterMessageDTO
	for _, m := range msgRows {
		messages = append(messages, ReporterMessageDTO{
			ID:         m.ID,
			AuthorType: m.AuthorType,
			Content:    m.Content,
			CreatedAt:  m.CreatedAt.Time,
		})
	}

	var resSummary *string
	if c.ResolutionSummary.Valid && c.ResolutionSummary.String != "" {
		resSummary = &c.ResolutionSummary.String
	}

	return &ReporterCaseView{
		PublicReference:   c.PublicReference,
		Category:          c.Category,
		Status:            c.Status,
		RiskLevel:         c.RiskLevel,
		IsImmediateDanger: c.IsImmediateDanger,
		CreatedAt:         c.CreatedAt.Time.Format(time.RFC3339),
		ResolutionSummary: resSummary,
		Messages:          messages,
	}, nil
}

func (s *SafeguardingService) PostReporterMessage(ctx context.Context, publicRef, candidateSecret, content string) error {
	cleanContent := strings.TrimSpace(content)
	if cleanContent == "" {
		return errors.New("message content cannot be empty")
	}

	c, err := s.repo.Queries().GetSafeguardingCaseByPublicRef(ctx, strings.TrimSpace(publicRef))
	if err != nil || c.ID == "" {
		return ErrCaseNotFound
	}

	reporter, err := s.repo.Queries().GetSafeguardingReporterByCaseID(ctx, c.ID)
	if err != nil {
		return ErrCaseNotFound
	}

	if !security.VerifyFollowUpSecret(candidateSecret, reporter.FollowUpSecretHash) {
		return ErrInvalidFollowUpCredentials
	}

	_, err = s.repo.Queries().CreateSafeguardingMessage(ctx, db.CreateSafeguardingMessageParams{
		ID:         security.GenerateUUID(),
		CaseID:     c.ID,
		AuthorType: "reporter",
		AuthorID:   pgtype.Text{Valid: false},
		Visibility: "reporter",
		Content:    cleanContent,
	})
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	// Record action
	_, _ = s.repo.Queries().CreateSafeguardingCaseAction(ctx, db.CreateSafeguardingCaseActionParams{
		ID:             security.GenerateUUID(),
		CaseID:         c.ID,
		ActorID:        pgtype.Text{Valid: false},
		ActionType:     "message_sent",
		PreviousStatus: pgtype.Text{String: c.Status, Valid: true},
		NewStatus:      pgtype.Text{String: c.Status, Valid: true},
		Reason:         Text(strPtr("New message received from reporter")),
	})

	return nil
}

// -------------------------------------------------------------
// Restricted Safeguarding Personnel Operations
// -------------------------------------------------------------

// CanAccessSafeguarding checks if user is an active safeguarding officer or super admin.
func (s *SafeguardingService) CanAccessSafeguarding(ctx context.Context, user db.User) bool {
	if user.PrimaryRole == "super_admin" {
		return true
	}
	officer, err := s.repo.Queries().GetSafeguardingOfficerByUserID(ctx, user.ID)
	return err == nil && officer.ID != "" && officer.Status == "active"
}

// CheckCaseConflict checks if the user has a recorded conflict or is implicated in this case.
func (s *SafeguardingService) CheckCaseConflict(ctx context.Context, caseID, userID string) error {
	// 1. Check explicit conflicts
	conflict, err := s.repo.Queries().GetSafeguardingConflict(ctx, db.GetSafeguardingConflictParams{
		CaseID:    caseID,
		OfficerID: userID,
	})
	if err == nil && conflict.ID != "" {
		return ErrConflictOfInterest
	}

	// 2. Check if the user is the reporter
	reporter, err := s.repo.Queries().GetSafeguardingReporterByCaseID(ctx, caseID)
	if err == nil && reporter.ReporterUserID.Valid && reporter.ReporterUserID.String == userID {
		return ErrConflictOfInterest
	}

	return nil
}

// ListCases returns paginated cases for authorized officers, respecting conflicts.
func (s *SafeguardingService) ListCases(ctx context.Context, user db.User, filter CaseFilter) (*CaseListResult, error) {
	if !s.CanAccessSafeguarding(ctx, user) {
		return nil, ErrUnauthorizedSafeguardingAccess
	}

	page := filter.Page
	if page < 1 {
		page = 1
	}
	limit := filter.Limit
	if limit < 1 || limit > 50 {
		limit = 15
	}
	offset := (page - 1) * limit

	var isMinor pgtype.Bool
	if filter.IsMinorInvolved != nil {
		isMinor = pgtype.Bool{Bool: *filter.IsMinorInvolved, Valid: true}
	}

	var isDanger pgtype.Bool
	if filter.IsImmediateDanger != nil {
		isDanger = pgtype.Bool{Bool: *filter.IsImmediateDanger, Valid: true}
	}

	var statusText, riskText, catText pgtype.Text
	if filter.Status != "" {
		statusText = pgtype.Text{String: filter.Status, Valid: true}
	}
	if filter.RiskLevel != "" {
		riskText = pgtype.Text{String: filter.RiskLevel, Valid: true}
	}
	if filter.Category != "" {
		catText = pgtype.Text{String: filter.Category, Valid: true}
	}

	rows, err := s.repo.Queries().ListSafeguardingCases(ctx, db.ListSafeguardingCasesParams{
		Limit:             limit,
		Offset:            offset,
		Status:            statusText,
		RiskLevel:         riskText,
		Category:          catText,
		IsMinorInvolved:   isMinor,
		IsImmediateDanger: isDanger,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list safeguarding cases: %w", err)
	}

	total, err := s.repo.Queries().CountSafeguardingCases(ctx, db.CountSafeguardingCasesParams{
		Status:            statusText,
		RiskLevel:         riskText,
		Category:          catText,
		IsMinorInvolved:   isMinor,
		IsImmediateDanger: isDanger,
	})
	if err != nil {
		total = int64(len(rows))
	}

	totalPages := int32(0)
	if limit > 0 {
		totalPages = int32((total + int64(limit) - 1) / int64(limit))
	}

	var dtos []CaseSummaryDTO
	for _, r := range rows {
		// If user has a conflict on this case, skip or hide from listing
		if err := s.CheckCaseConflict(ctx, r.ID, user.ID); err != nil {
			continue
		}

		var incDate *time.Time
		if r.IncidentDate.Valid {
			incDate = &r.IncidentDate.Time
		}
		var sName, lName, relAct, personInv, escTo, offID, offName *string
		if r.StateName.Valid {
			sName = &r.StateName.String
		}
		if r.LgaName.Valid {
			lName = &r.LgaName.String
		}
		if r.RelatedActivity.Valid {
			relAct = &r.RelatedActivity.String
		}
		if r.PersonInvolved.Valid {
			personInv = &r.PersonInvolved.String
		}
		if r.EscalatedTo.Valid {
			escTo = &r.EscalatedTo.String
		}
		if r.AssignedOfficerID.Valid {
			offID = &r.AssignedOfficerID.String
		}
		if r.AssignedOfficerName.Valid {
			offName = &r.AssignedOfficerName.String
		}

		isAnon := true
		if r.IsAnonymous.Valid {
			isAnon = r.IsAnonymous.Bool
		}

		dtos = append(dtos, CaseSummaryDTO{
			ID:                  r.ID,
			PublicReference:     r.PublicReference,
			Category:            r.Category,
			IncidentDate:        incDate,
			StateName:           sName,
			LGAName:             lName,
			RelatedActivity:     relAct,
			PersonInvolved:      personInv,
			IsImmediateDanger:   r.IsImmediateDanger,
			IsMinorInvolved:     r.IsMinorInvolved,
			RiskLevel:           r.RiskLevel,
			Status:              r.Status,
			IsEscalated:         r.IsEscalated,
			EscalatedTo:         escTo,
			IsAnonymous:         isAnon,
			AssignedOfficerID:   offID,
			AssignedOfficerName: offName,
			CreatedAt:           r.CreatedAt.Time,
			UpdatedAt:           r.UpdatedAt.Time,
		})
	}

	return &CaseListResult{
		Items:      dtos,
		Total:      total,
		Page:       page,
		Limit:      limit,
		TotalPages: totalPages,
	}, nil
}

// GetCaseDetails fetches full case information, logs access, and verifies no conflict of interest.
func (s *SafeguardingService) GetCaseDetails(ctx context.Context, user db.User, caseID, justification, clientIP string) (*AdminCaseDetail, error) {
	if !s.CanAccessSafeguarding(ctx, user) {
		return nil, ErrUnauthorizedSafeguardingAccess
	}

	if err := s.CheckCaseConflict(ctx, caseID, user.ID); err != nil {
		// Log attempted access by conflicted person
		_, _ = s.repo.Queries().CreateSafeguardingAccessLog(ctx, db.CreateSafeguardingAccessLogParams{
			ID:            security.GenerateUUID(),
			CaseID:        caseID,
			ActorID:       user.ID,
			AccessType:    "view_denied_conflict",
			Justification: Text(strPtr("Access denied: User has conflict of interest")),
			IpAddress:     Text(strPtr(clientIP)),
		})
		return nil, ErrConflictOfInterest
	}

	c, err := s.repo.Queries().GetSafeguardingCaseByID(ctx, caseID)
	if err != nil || c.ID == "" {
		return nil, ErrCaseNotFound
	}

	// Immutable access log
	_, _ = s.repo.Queries().CreateSafeguardingAccessLog(ctx, db.CreateSafeguardingAccessLogParams{
		ID:            security.GenerateUUID(),
		CaseID:        caseID,
		ActorID:       user.ID,
		AccessType:    "read_details",
		Justification: Text(strPtr(justification)),
		IpAddress:     Text(strPtr(clientIP)),
	})

	// Decrypt narrative
	decryptedDesc, err := s.cipher.Decrypt(c.EncryptedDescription)
	if err != nil {
		decryptedDesc = "[Decryption unavailable: check key configuration]"
	}

	// Reporter details
	var reporterDTO *AdminReporterDTO
	repRow, err := s.repo.Queries().GetSafeguardingReporterByCaseID(ctx, caseID)
	if err == nil && repRow.ID != "" {
		var repUser *string
		if repRow.ReporterUserID.Valid {
			repUser = &repRow.ReporterUserID.String
		}
		var decName, decEmail, decPhone *string
		if repRow.EncryptedName.Valid && repRow.EncryptedName.String != "" {
			if n, err := s.cipher.Decrypt(repRow.EncryptedName.String); err == nil {
				decName = &n
			}
		}
		if repRow.EncryptedEmail.Valid && repRow.EncryptedEmail.String != "" {
			if e, err := s.cipher.Decrypt(repRow.EncryptedEmail.String); err == nil {
				decEmail = &e
			}
		}
		if repRow.EncryptedPhone.Valid && repRow.EncryptedPhone.String != "" {
			if p, err := s.cipher.Decrypt(repRow.EncryptedPhone.String); err == nil {
				decPhone = &p
			}
		}

		reporterDTO = &AdminReporterDTO{
			IsAnonymous:       repRow.IsAnonymous,
			ReporterUserID:    repUser,
			DecryptedName:     decName,
			DecryptedEmail:    decEmail,
			DecryptedPhone:    decPhone,
			ContactPreference: repRow.ContactPreference,
		}
	}

	// Evidence list
	evRows, _ := s.repo.Queries().ListSafeguardingEvidenceByCaseID(ctx, caseID)
	var evidenceDTOs []AdminEvidenceDTO
	for _, ev := range evRows {
		evidenceDTOs = append(evidenceDTOs, AdminEvidenceDTO{
			ID:                      ev.ID,
			FileName:                ev.FileName,
			MimeType:                ev.MimeType,
			FileSizeBytes:           ev.FileSizeBytes,
			Sha256Hash:              ev.Sha256Hash,
			RetentionClassification: ev.RetentionClassification,
			CreatedAt:               ev.CreatedAt.Time,
		})
	}

	// Assignments list
	asRows, _ := s.repo.Queries().ListAssignmentsByCaseID(ctx, caseID)
	var assignDTOs []AdminAssignmentDTO
	for _, a := range asRows {
		var rsn *string
		if a.AssignmentReason.Valid {
			rsn = &a.AssignmentReason.String
		}
		var offEmail string
		if a.OfficerEmail.Valid {
			offEmail = a.OfficerEmail.String
		}
		assignDTOs = append(assignDTOs, AdminAssignmentDTO{
			ID:               a.ID,
			OfficerID:        a.OfficerID,
			OfficerName:      a.OfficerName,
			OfficerEmail:     offEmail,
			Status:           a.Status,
			AssignmentReason: rsn,
			CreatedAt:        a.CreatedAt.Time,
		})
	}

	// Conflicts list
	conRows, _ := s.repo.Queries().ListSafeguardingConflictsByCaseID(ctx, caseID)
	var conflictDTOs []AdminConflictDTO
	for _, cf := range conRows {
		conflictDTOs = append(conflictDTOs, AdminConflictDTO{
			ID:           cf.ID,
			OfficerID:    cf.OfficerID,
			OfficerName:  cf.OfficerName,
			ConflictType: cf.ConflictType,
			Reason:       cf.Reason,
			CreatedAt:    cf.CreatedAt.Time,
		})
	}

	// Messages list
	msgRows, _ := s.repo.Queries().ListAllMessagesByCaseID(ctx, caseID)
	var msgDTOs []AdminMessageDTO
	for _, m := range msgRows {
		var authName *string
		if m.AuthorName.Valid {
			authName = &m.AuthorName.String
		}
		msgDTOs = append(msgDTOs, AdminMessageDTO{
			ID:         m.ID,
			AuthorType: m.AuthorType,
			AuthorName: authName,
			Visibility: m.Visibility,
			Content:    m.Content,
			CreatedAt:  m.CreatedAt.Time,
		})
	}

	// Actions list
	actRows, _ := s.repo.Queries().ListSafeguardingCaseActionsByCaseID(ctx, caseID)
	var actionDTOs []AdminCaseActionDTO
	for _, act := range actRows {
		var an, ps, ns, rsn *string
		if act.ActorName.Valid {
			an = &act.ActorName.String
		}
		if act.PreviousStatus.Valid {
			ps = &act.PreviousStatus.String
		}
		if act.NewStatus.Valid {
			ns = &act.NewStatus.String
		}
		if act.Reason.Valid {
			rsn = &act.Reason.String
		}
		actionDTOs = append(actionDTOs, AdminCaseActionDTO{
			ID:             act.ID,
			ActionType:     act.ActionType,
			ActorName:      an,
			PreviousStatus: ps,
			NewStatus:      ns,
			Reason:         rsn,
			CreatedAt:      act.CreatedAt.Time,
		})
	}

	// Retention record
	var retentionDTO *AdminRetentionDTO
	retRow, err := s.repo.Queries().GetSafeguardingRetentionRecordByCaseID(ctx, caseID)
	if err == nil && retRow.ID != "" {
		var holdReason *string
		if retRow.LegalHoldReason.Valid {
			holdReason = &retRow.LegalHoldReason.String
		}
		retentionDTO = &AdminRetentionDTO{
			RetentionCategory: retRow.RetentionCategory,
			ReviewDate:        retRow.ReviewDate.Time,
			IsLegalHold:       retRow.IsLegalHold,
			LegalHoldReason:   holdReason,
		}
	}

	var incDate *time.Time
	if c.IncidentDate.Valid {
		incDate = &c.IncidentDate.Time
	}
	var sName, lName, relAct, personInv, escTo *string
	if c.StateName.Valid {
		sName = &c.StateName.String
	}
	if c.LgaName.Valid {
		lName = &c.LgaName.String
	}
	if c.RelatedActivity.Valid {
		relAct = &c.RelatedActivity.String
	}
	if c.PersonInvolved.Valid {
		personInv = &c.PersonInvolved.String
	}
	if c.EscalatedTo.Valid {
		escTo = &c.EscalatedTo.String
	}

	isAnon := true
	if reporterDTO != nil {
		isAnon = reporterDTO.IsAnonymous
	}

	return &AdminCaseDetail{
		CaseSummaryDTO: CaseSummaryDTO{
			ID:                c.ID,
			PublicReference:   c.PublicReference,
			Category:          c.Category,
			IncidentDate:      incDate,
			StateName:         sName,
			LGAName:           lName,
			RelatedActivity:   relAct,
			PersonInvolved:    personInv,
			IsImmediateDanger: c.IsImmediateDanger,
			IsMinorInvolved:   c.IsMinorInvolved,
			RiskLevel:         c.RiskLevel,
			Status:            c.Status,
			IsEscalated:       c.IsEscalated,
			EscalatedTo:       escTo,
			IsAnonymous:       isAnon,
			CreatedAt:         c.CreatedAt.Time,
			UpdatedAt:         c.UpdatedAt.Time,
		},
		DecryptedDescription: decryptedDesc,
		Reporter:             reporterDTO,
		Evidence:             evidenceDTOs,
		Assignments:          assignDTOs,
		Conflicts:            conflictDTOs,
		Messages:             msgDTOs,
		Actions:              actionDTOs,
		Retention:            retentionDTO,
	}, nil
}

// UpdateTriage adjusts case risk level and moves it to under_review or triage state.
func (s *SafeguardingService) UpdateTriage(ctx context.Context, user db.User, caseID, newRisk, newStatus, reason string) error {
	if !s.CanAccessSafeguarding(ctx, user) {
		return ErrUnauthorizedSafeguardingAccess
	}
	if err := s.CheckCaseConflict(ctx, caseID, user.ID); err != nil {
		return ErrConflictOfInterest
	}

	c, err := s.repo.Queries().GetSafeguardingCaseByID(ctx, caseID)
	if err != nil || c.ID == "" {
		return ErrCaseNotFound
	}

	if newRisk == "" {
		newRisk = c.RiskLevel
	}
	if newStatus == "" {
		newStatus = "triaged"
	}

	prevStatus := c.Status
	_, err = s.repo.Queries().UpdateSafeguardingCaseTriage(ctx, db.UpdateSafeguardingCaseTriageParams{
		ID:        caseID,
		RiskLevel: newRisk,
		Status:    newStatus,
	})
	if err != nil {
		return fmt.Errorf("failed to update triage: %w", err)
	}

	_, _ = s.repo.Queries().CreateSafeguardingCaseAction(ctx, db.CreateSafeguardingCaseActionParams{
		ID:             security.GenerateUUID(),
		CaseID:         caseID,
		ActorID:        Text(strPtr(user.ID)),
		ActionType:     "triage_updated",
		PreviousStatus: Text(strPtr(prevStatus)),
		NewStatus:      Text(strPtr(newStatus)),
		Reason:         Text(strPtr(reason)),
	})

	// Also record in independent safeguarding access audit log
	_, _ = s.repo.Queries().CreateSafeguardingAccessLog(ctx, db.CreateSafeguardingAccessLogParams{
		ID:            security.GenerateUUID(),
		CaseID:        caseID,
		ActorID:       user.ID,
		AccessType:    "update_triage",
		Justification: Text(strPtr(reason)),
		IpAddress:     Text(nil),
	})

	return nil
}

// AssignOfficer assigns an active safeguarding officer to investigate a case.
func (s *SafeguardingService) AssignOfficer(ctx context.Context, user db.User, caseID, officerID, reason string) error {
	if !s.CanAccessSafeguarding(ctx, user) {
		return ErrUnauthorizedSafeguardingAccess
	}
	if err := s.CheckCaseConflict(ctx, caseID, user.ID); err != nil {
		return ErrConflictOfInterest
	}

	// Verify officer has NO conflict
	if err := s.CheckCaseConflict(ctx, caseID, officerID); err != nil {
		return ErrOfficerHasConflict
	}

	// Verify officer exists and is active
	officer, err := s.repo.Queries().GetSafeguardingOfficerByUserID(ctx, officerID)
	if err != nil || officer.ID == "" || officer.Status != "active" {
		return ErrOfficerNotFound
	}

	c, err := s.repo.Queries().GetSafeguardingCaseByID(ctx, caseID)
	if err != nil || c.ID == "" {
		return ErrCaseNotFound
	}

	// Deactivate existing assignments
	_ = s.repo.Queries().DeactivateAssignmentsForCase(ctx, caseID)

	// Create new assignment
	_, err = s.repo.Queries().CreateSafeguardingAssignment(ctx, db.CreateSafeguardingAssignmentParams{
		ID:               security.GenerateUUID(),
		CaseID:           caseID,
		OfficerID:        officerID,
		AssignedBy:       user.ID,
		Status:           "active",
		AssignmentReason: Text(strPtr(reason)),
	})
	if err != nil {
		return fmt.Errorf("failed to assign officer: %w", err)
	}

	// Update case status to assigned or under_review
	newStatus := "assigned"
	if c.Status == "submitted" || c.Status == "triage" {
		_, _ = s.repo.Queries().UpdateSafeguardingCaseStatus(ctx, db.UpdateSafeguardingCaseStatusParams{
			ID:     caseID,
			Status: newStatus,
		})
	}

	_, _ = s.repo.Queries().CreateSafeguardingCaseAction(ctx, db.CreateSafeguardingCaseActionParams{
		ID:             security.GenerateUUID(),
		CaseID:         caseID,
		ActorID:        Text(strPtr(user.ID)),
		ActionType:     "assigned",
		PreviousStatus: Text(strPtr(c.Status)),
		NewStatus:      Text(strPtr(newStatus)),
		Reason:         Text(strPtr(fmt.Sprintf("Assigned to officer %s. Reason: %s", officerID, reason))),
	})

	// Dispatch privacy-preserving notification to assigned officer
	if s.notifSvc != nil {
		actionURL := "/admin/safeguarding"
		relType := "safeguarding_case"
		_, _ = s.notifSvc.CreateInAppNotification(ctx, nil, CreateInAppNotificationInput{
			RecipientID:         officerID,
			NotificationType:    "safeguarding_assigned",
			Title:               "Safeguarding Case Assignment",
			Body:                "A confidential safeguarding case has been assigned to you for investigation.",
			ActionURL:           &actionURL,
			Priority:            "high",
			RelatedResourceType: &relType,
			RelatedResourceID:   &caseID,
		})
	}

	return nil
}

// DeclareConflict logs a conflict of interest, recusing or blocking an officer.
func (s *SafeguardingService) DeclareConflict(ctx context.Context, user db.User, caseID, officerID, conflictType, reason string) error {
	if !s.CanAccessSafeguarding(ctx, user) {
		return ErrUnauthorizedSafeguardingAccess
	}

	_, err := s.repo.Queries().CreateSafeguardingConflict(ctx, db.CreateSafeguardingConflictParams{
		ID:           security.GenerateUUID(),
		CaseID:       caseID,
		OfficerID:    officerID,
		ConflictType: conflictType,
		DeclaredBy:   user.ID,
		Reason:       reason,
	})
	if err != nil {
		return fmt.Errorf("failed to record conflict: %w", err)
	}

	// Deactivate any active assignment for this officer
	_ = s.repo.Queries().DeactivateAssignmentsForCase(ctx, caseID)

	_, _ = s.repo.Queries().CreateSafeguardingCaseAction(ctx, db.CreateSafeguardingCaseActionParams{
		ID:             security.GenerateUUID(),
		CaseID:         caseID,
		ActorID:        Text(strPtr(user.ID)),
		ActionType:     "conflict_declared",
		PreviousStatus: pgtype.Text{Valid: false},
		NewStatus:      pgtype.Text{Valid: false},
		Reason:         Text(strPtr(fmt.Sprintf("Conflict declared for officer %s: %s", officerID, reason))),
	})

	return nil
}

// EscalateCase marks a case as escalated to independent oversight.
func (s *SafeguardingService) EscalateCase(ctx context.Context, user db.User, caseID, escalatedTo, reason string) error {
	if !s.CanAccessSafeguarding(ctx, user) {
		return ErrUnauthorizedSafeguardingAccess
	}
	if err := s.CheckCaseConflict(ctx, caseID, user.ID); err != nil {
		return ErrConflictOfInterest
	}

	c, err := s.repo.Queries().GetSafeguardingCaseByID(ctx, caseID)
	if err != nil || c.ID == "" {
		return ErrCaseNotFound
	}

	_, err = s.repo.Queries().EscalateSafeguardingCase(ctx, db.EscalateSafeguardingCaseParams{
		ID:               caseID,
		EscalatedTo:      Text(strPtr(escalatedTo)),
		EscalationReason: Text(strPtr(reason)),
	})
	if err != nil {
		return fmt.Errorf("failed to escalate case: %w", err)
	}

	_, _ = s.repo.Queries().CreateSafeguardingCaseAction(ctx, db.CreateSafeguardingCaseActionParams{
		ID:             security.GenerateUUID(),
		CaseID:         caseID,
		ActorID:        Text(strPtr(user.ID)),
		ActionType:     "escalated",
		PreviousStatus: Text(strPtr(c.Status)),
		NewStatus:      Text(strPtr("escalated")),
		Reason:         Text(strPtr(fmt.Sprintf("Escalated to %s: %s", escalatedTo, reason))),
	})

	return nil
}

// ResolveCase marks a case as resolved with an investigation summary.
func (s *SafeguardingService) ResolveCase(ctx context.Context, user db.User, caseID, summary string) error {
	if !s.CanAccessSafeguarding(ctx, user) {
		return ErrUnauthorizedSafeguardingAccess
	}
	if err := s.CheckCaseConflict(ctx, caseID, user.ID); err != nil {
		return ErrConflictOfInterest
	}

	cleanSummary := strings.TrimSpace(summary)
	if cleanSummary == "" {
		return errors.New("resolution summary is required")
	}

	c, err := s.repo.Queries().GetSafeguardingCaseByID(ctx, caseID)
	if err != nil || c.ID == "" {
		return ErrCaseNotFound
	}

	_, err = s.repo.Queries().ResolveSafeguardingCase(ctx, db.ResolveSafeguardingCaseParams{
		ID:                caseID,
		ResolutionSummary: Text(strPtr(cleanSummary)),
	})
	if err != nil {
		return fmt.Errorf("failed to resolve case: %w", err)
	}

	_, _ = s.repo.Queries().CreateSafeguardingCaseAction(ctx, db.CreateSafeguardingCaseActionParams{
		ID:             security.GenerateUUID(),
		CaseID:         caseID,
		ActorID:        Text(strPtr(user.ID)),
		ActionType:     "resolved",
		PreviousStatus: Text(strPtr(c.Status)),
		NewStatus:      Text(strPtr("resolved")),
		Reason:         Text(strPtr(cleanSummary)),
	})

	return nil
}

// CloseCase marks a case as closed following investigation and resolution.
func (s *SafeguardingService) CloseCase(ctx context.Context, user db.User, caseID string) error {
	if !s.CanAccessSafeguarding(ctx, user) {
		return ErrUnauthorizedSafeguardingAccess
	}
	if err := s.CheckCaseConflict(ctx, caseID, user.ID); err != nil {
		return ErrConflictOfInterest
	}

	c, err := s.repo.Queries().GetSafeguardingCaseByID(ctx, caseID)
	if err != nil || c.ID == "" {
		return ErrCaseNotFound
	}

	_, err = s.repo.Queries().CloseSafeguardingCase(ctx, caseID)
	if err != nil {
		return fmt.Errorf("failed to close case: %w", err)
	}

	_, _ = s.repo.Queries().CreateSafeguardingCaseAction(ctx, db.CreateSafeguardingCaseActionParams{
		ID:             security.GenerateUUID(),
		CaseID:         caseID,
		ActorID:        Text(strPtr(user.ID)),
		ActionType:     "closed",
		PreviousStatus: Text(strPtr(c.Status)),
		NewStatus:      Text(strPtr("closed")),
		Reason:         Text(strPtr("Case closed by authorized safeguarding personnel")),
	})

	return nil
}

// AddInternalNote adds an internal investigation note visible only to safeguarding officers.
func (s *SafeguardingService) AddInternalNote(ctx context.Context, user db.User, caseID, content string) error {
	return s.addOfficerMessage(ctx, user, caseID, content, "internal")
}

// SendReporterUpdate sends a message visible to the reporter on their follow-up page.
func (s *SafeguardingService) SendReporterUpdate(ctx context.Context, user db.User, caseID, content string) error {
	return s.addOfficerMessage(ctx, user, caseID, content, "reporter")
}

func (s *SafeguardingService) addOfficerMessage(ctx context.Context, user db.User, caseID, content, visibility string) error {
	if !s.CanAccessSafeguarding(ctx, user) {
		return ErrUnauthorizedSafeguardingAccess
	}
	if err := s.CheckCaseConflict(ctx, caseID, user.ID); err != nil {
		return ErrConflictOfInterest
	}

	cleanContent := strings.TrimSpace(content)
	if cleanContent == "" {
		return errors.New("message content cannot be empty")
	}

	_, err := s.repo.Queries().CreateSafeguardingMessage(ctx, db.CreateSafeguardingMessageParams{
		ID:         security.GenerateUUID(),
		CaseID:     caseID,
		AuthorType: "officer",
		AuthorID:   Text(strPtr(user.ID)),
		Visibility: visibility,
		Content:    cleanContent,
	})
	if err != nil {
		return fmt.Errorf("failed to add message: %w", err)
	}

	return nil
}

// GetEvidenceDownloadURL returns a short-lived presigned download URL for evidence and logs access.
func (s *SafeguardingService) GetEvidenceDownloadURL(ctx context.Context, user db.User, evidenceID, justification, clientIP string) (string, error) {
	if !s.CanAccessSafeguarding(ctx, user) {
		return "", ErrUnauthorizedSafeguardingAccess
	}

	ev, err := s.repo.Queries().GetSafeguardingEvidenceByID(ctx, evidenceID)
	if err != nil || ev.ID == "" {
		return "", errors.New("evidence not found")
	}

	if err := s.CheckCaseConflict(ctx, ev.CaseID, user.ID); err != nil {
		return "", ErrConflictOfInterest
	}

	// Immutable access log
	_, _ = s.repo.Queries().CreateSafeguardingAccessLog(ctx, db.CreateSafeguardingAccessLogParams{
		ID:            security.GenerateUUID(),
		CaseID:        ev.CaseID,
		ActorID:       user.ID,
		AccessType:    "download_evidence",
		Justification: Text(strPtr(justification)),
		IpAddress:     Text(strPtr(clientIP)),
	})

	return s.storage.GeneratePresignedDownload(ctx, ev.StorageKey, ev.FileName, 5*time.Minute)
}

// SetLegalHold sets or lifts a statutory legal hold on case evidence and records.
func (s *SafeguardingService) SetLegalHold(ctx context.Context, user db.User, caseID string, hold bool, reason string) error {
	if !s.CanAccessSafeguarding(ctx, user) {
		return ErrUnauthorizedSafeguardingAccess
	}
	if err := s.CheckCaseConflict(ctx, caseID, user.ID); err != nil {
		return ErrConflictOfInterest
	}

	_, err := s.repo.Queries().UpdateSafeguardingLegalHold(ctx, db.UpdateSafeguardingLegalHoldParams{
		CaseID:          caseID,
		IsLegalHold:     hold,
		LegalHoldReason: Text(strPtr(reason)),
	})
	if err != nil {
		return fmt.Errorf("failed to update legal hold: %w", err)
	}

	actionDesc := "Legal hold placed on case records"
	if !hold {
		actionDesc = "Legal hold released from case records"
	}
	_, _ = s.repo.Queries().CreateSafeguardingCaseAction(ctx, db.CreateSafeguardingCaseActionParams{
		ID:             security.GenerateUUID(),
		CaseID:         caseID,
		ActorID:        Text(strPtr(user.ID)),
		ActionType:     "legal_hold_set",
		PreviousStatus: pgtype.Text{Valid: false},
		NewStatus:      pgtype.Text{Valid: false},
		Reason:         Text(strPtr(fmt.Sprintf("%s: %s", actionDesc, reason))),
	})

	return nil
}

// ListAccessLogs returns audit trail of case inspections for authorized oversight.
func (s *SafeguardingService) ListAccessLogs(ctx context.Context, user db.User, page, limit int32) ([]db.ListSafeguardingAccessLogsRow, int64, error) {
	if !s.CanAccessSafeguarding(ctx, user) {
		return nil, 0, ErrUnauthorizedSafeguardingAccess
	}

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	logs, err := s.repo.Queries().ListSafeguardingAccessLogs(ctx, db.ListSafeguardingAccessLogsParams{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list access logs: %w", err)
	}

	total, _ := s.repo.Queries().CountSafeguardingAccessLogs(ctx)
	return logs, total, nil
}

// ListActiveOfficers returns active designated safeguarding officers.
func (s *SafeguardingService) ListActiveOfficers(ctx context.Context, user db.User) ([]db.ListActiveSafeguardingOfficersRow, error) {
	if !s.CanAccessSafeguarding(ctx, user) {
		return nil, ErrUnauthorizedSafeguardingAccess
	}
	return s.repo.Queries().ListActiveSafeguardingOfficers(ctx)
}

// AppointOfficer designates or updates a safeguarding officer's permissions (Super Admin only).
func (s *SafeguardingService) AppointOfficer(ctx context.Context, user db.User, targetUserID, roleTitle string, isLead, canAssign, canClose, canAudit bool) error {
	if user.PrimaryRole != "super_admin" {
		return errors.New("only Super Admin can appoint or modify safeguarding officers")
	}

	officerID := security.GenerateUUID()
	_, err := s.repo.Queries().UpsertSafeguardingOfficer(ctx, db.UpsertSafeguardingOfficerParams{
		ID:            officerID,
		UserID:        targetUserID,
		RoleTitle:     roleTitle,
		IsLead:        isLead,
		CanTriage:     true,
		CanAssign:     canAssign,
		CanInvestigate: true,
		CanEscalate:   true,
		CanClose:      canClose,
		CanViewAudit:  canAudit,
		Status:        "active",
		AppointedBy:   Text(strPtr(user.ID)),
	})
	return err
}

func (s *SafeguardingService) notifySafeguardingOfficers(ctx context.Context, publicRef, riskLevel string, isDanger bool) {
	if s.notifSvc == nil {
		return
	}
	officers, err := s.repo.Queries().ListActiveSafeguardingOfficers(ctx)
	if err != nil || len(officers) == 0 {
		return
	}

	priority := "normal"
	if isDanger || riskLevel == "critical" {
		priority = "urgent"
	} else if riskLevel == "high" {
		priority = "high"
	}

	actionURL := "/admin/safeguarding"
	for _, off := range officers {
		_, _ = s.notifSvc.CreateInAppNotification(ctx, nil, CreateInAppNotificationInput{
			RecipientID:      off.UserID,
			NotificationType: "safeguarding_alert",
			Title:            "Confidential Safeguarding Alert",
			Body:             "A confidential safeguarding item requires your attention in the safeguarding workspace.",
			ActionURL:        &actionURL,
			Priority:         priority,
		})
	}
}

func filepathBase(path string) string {
	idx := strings.LastIndexAny(path, "/\\")
	if idx != -1 {
		return path[idx+1:]
	}
	return path
}
