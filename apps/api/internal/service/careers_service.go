package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"ratels-api/internal/db"
	"ratels-api/internal/delivery"
	"ratels-api/internal/repository"
	"ratels-api/internal/security"
)

var (
	ErrEmployerNotFound            = errors.New("employer organization not found")
	ErrEmployerAlreadyExists        = errors.New("an employer organization already exists for this account")
	ErrEmployerNotVerified          = errors.New("employer organization is pending verification and cannot publish jobs directly")
	ErrUnauthorizedRecruiterAction  = errors.New("unauthorized: you are not an authorized recruiter for this employer")
	ErrJobNotFound                  = errors.New("job opportunity not found")
	ErrJobClosedOrExpired           = errors.New("this job opportunity is closed or its application deadline has passed")
	ErrJobNotPublished              = errors.New("job opportunity is not currently open for applications")
	ErrCandidateProfileNotFound     = errors.New("candidate profile not found")
	ErrApplicationNotFound          = errors.New("job application not found")
	ErrDuplicateApplication         = errors.New("you have already submitted an active application for this opportunity")
	ErrInvalidApplicationStatus     = errors.New("invalid recruitment status transition")
	ErrUnauthorizedCandidateAction  = errors.New("unauthorized: you do not own this candidate profile or application")
	ErrApplicationCannotBeWithdrawn = errors.New("application cannot be withdrawn in its current status")
	ErrInterviewNotFound            = errors.New("interview schedule not found")
)

type CareersService struct {
	repo     *repository.Repository
	audit    *AuditService
	delivery delivery.DeliveryAdapter
}

func strPtr(s string) *string {
	return &s
}

func NewCareersService(repo *repository.Repository, audit *AuditService, del delivery.DeliveryAdapter) *CareersService {
	return &CareersService{
		repo:     repo,
		audit:    audit,
		delivery: del,
	}
}

// -------------------------------------------------------------
// DTOs
// -------------------------------------------------------------

type RegisterEmployerInput struct {
	OrganizationName   string  `json:"organization_name"`
	RegistrationNumber string  `json:"registration_number"`
	Industry           string  `json:"industry"`
	CompanySize        string  `json:"company_size"`
	Description        string  `json:"description"`
	WebsiteURL         *string `json:"website_url"`
	ContactEmail       string  `json:"contact_email"`
	ContactPhone       string  `json:"contact_phone"`
	LogoURL            *string `json:"logo_url"`
	StateID            string  `json:"state_id"`
	LgaID              *string `json:"lga_id"`
	Address            string  `json:"address"`
}

type UpdateEmployerInput struct {
	OrganizationName string  `json:"organization_name"`
	Industry         string  `json:"industry"`
	CompanySize      string  `json:"company_size"`
	Description      string  `json:"description"`
	WebsiteURL       *string `json:"website_url"`
	ContactEmail     string  `json:"contact_email"`
	ContactPhone     string  `json:"contact_phone"`
	LogoURL          *string `json:"logo_url"`
	StateID          string  `json:"state_id"`
	LgaID            *string `json:"lga_id"`
	Address          string  `json:"address"`
}

type CreateJobInput struct {
	Title              string     `json:"title"`
	Category           string     `json:"category"`
	EmploymentType     string     `json:"employment_type"` // full_time, part_time, contract, internship, volunteer
	WorkplaceType      string     `json:"workplace_type"`  // on_site, remote, hybrid
	StateID            *string    `json:"state_id"`
	LgaID              *string    `json:"lga_id"`
	LocationSummary    string     `json:"location_summary"`
	SalaryMinKobo      *int64     `json:"salary_min_kobo"`
	SalaryMaxKobo      *int64     `json:"salary_max_kobo"`
	Currency           string     `json:"currency"`
	IsSalaryNegotiable bool       `json:"is_salary_negotiable"`
	Description        string     `json:"description"`
	Responsibilities   string     `json:"responsibilities"`
	Requirements       string     `json:"requirements"`
	SkillsRequired     []string   `json:"skills_required"`
	ExperienceLevel    string     `json:"experience_level"` // entry, mid, senior, executive, no_experience
	DeadlineAt         *time.Time `json:"deadline_at"`
	AsDraft            bool       `json:"as_draft"`
}

type CandidateProfileInput struct {
	ProfessionalHeadline            string                   `json:"professional_headline"`
	Summary                         string                   `json:"summary"`
	Skills                          []string                 `json:"skills"`
	Education                       []map[string]interface{} `json:"education"`
	Experience                      []map[string]interface{} `json:"experience"`
	Certifications                  []map[string]interface{} `json:"certifications"`
	PreferredCategories             []string                 `json:"preferred_categories"`
	PreferredWorkplaceType          string                   `json:"preferred_workplace_type"`
	PreferredStateID                *string                  `json:"preferred_state_id"`
	ResumeURL                       string                   `json:"resume_url"`
	ResumeFilename                  string                   `json:"resume_filename"`
	IsSearchableByVerifiedEmployers bool                     `json:"is_searchable_by_verified_employers"`
}

type SubmitApplicationInput struct {
	CoverLetter           string `json:"cover_letter"`
	QualificationsSummary string `json:"qualifications_summary"`
	ResumeURL             string `json:"resume_url"`
	ResumeFilename        string `json:"resume_filename"`
}

type ScheduleInterviewInput struct {
	ScheduledTime  time.Time `json:"scheduled_time"`
	Format         string    `json:"format"` // video, in_person, phone
	LocationOrLink string    `json:"location_or_link"`
	Instructions   string    `json:"instructions"`
}

type PublicJobsFilter struct {
	Category        string
	EmploymentType  string
	WorkplaceType   string
	StateID         string
	ExperienceLevel string
	Query           string
	Limit           int32
	Offset          int32
}

// -------------------------------------------------------------
// Employer Operations
// -------------------------------------------------------------

func (s *CareersService) RegisterEmployer(ctx context.Context, user *db.User, input RegisterEmployerInput) (*db.CareersEmployer, error) {
	if user == nil {
		return nil, errors.New("unauthenticated")
	}

	// Check if user already owns an employer
	existing, _ := s.repo.Queries().GetCareersEmployerByOwner(ctx, user.ID)
	if existing.ID != "" {
		return nil, ErrEmployerAlreadyExists
	}

	slug := slugify(input.OrganizationName)
	// Append random suffix if needed
	uniqueSlug := fmt.Sprintf("%s-%s", slug, strings.ToLower(security.GenerateUUID()[:6]))

	empID := security.GenerateUUID()
	emp, err := s.repo.Queries().CreateCareersEmployer(ctx, db.CreateCareersEmployerParams{
		ID:                 empID,
		OwnerUserID:        user.ID,
		OrganizationName:   strings.TrimSpace(input.OrganizationName),
		Slug:               uniqueSlug,
		RegistrationNumber: strings.TrimSpace(input.RegistrationNumber),
		Industry:           strings.TrimSpace(input.Industry),
		CompanySize:        input.CompanySize,
		Description:        input.Description,
		WebsiteUrl:         Text(input.WebsiteURL),
		ContactEmail:       strings.TrimSpace(input.ContactEmail),
		ContactPhone:       strings.TrimSpace(input.ContactPhone),
		LogoUrl:            Text(input.LogoURL),
		StateID:            input.StateID,
		LgaID:              Text(input.LgaID),
		Address:            input.Address,
		VerificationStatus: "pending",
		VerifiedByUserID:   pgtype.Text{Valid: false},
		VerifiedAt:         pgtype.Timestamptz{Valid: false},
		RejectionReason:    pgtype.Text{Valid: false},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to register employer organization: %w", err)
	}

	// Add owner as admin recruiter
	recID := security.GenerateUUID()
	_, _ = s.repo.Queries().AddCareersRecruiter(ctx, db.AddCareersRecruiterParams{
		ID:         recID,
		EmployerID: emp.ID,
		UserID:     user.ID,
		Role:       "admin",
		IsActive:   true,
	})

	_ = s.audit.Log(ctx, &user.ID, &user.PrimaryRole, "CAREERS_EMPLOYER_REGISTERED", "careers_employer", emp.ID, nil, nil, strPtr("Employer registered: "+emp.OrganizationName), nil)
	return &emp, nil
}

func (s *CareersService) GetEmployer(ctx context.Context, idOrSlug string) (*db.GetCareersEmployerByIDRow, error) {
	emp, err := s.repo.Queries().GetCareersEmployerByID(ctx, idOrSlug)
	if err == nil && emp.ID != "" {
		return &emp, nil
	}

	bySlug, errSlug := s.repo.Queries().GetCareersEmployerBySlug(ctx, idOrSlug)
	if errSlug != nil {
		return nil, ErrEmployerNotFound
	}

	return &db.GetCareersEmployerByIDRow{
		ID:                 bySlug.ID,
		OwnerUserID:        bySlug.OwnerUserID,
		OrganizationName:   bySlug.OrganizationName,
		Slug:               bySlug.Slug,
		RegistrationNumber: bySlug.RegistrationNumber,
		Industry:           bySlug.Industry,
		CompanySize:        bySlug.CompanySize,
		Description:        bySlug.Description,
		WebsiteUrl:         bySlug.WebsiteUrl,
		ContactEmail:       bySlug.ContactEmail,
		ContactPhone:       bySlug.ContactPhone,
		LogoUrl:            bySlug.LogoUrl,
		StateID:            bySlug.StateID,
		LgaID:              bySlug.LgaID,
		Address:            bySlug.Address,
		VerificationStatus: bySlug.VerificationStatus,
		VerifiedByUserID:   bySlug.VerifiedByUserID,
		VerifiedAt:         bySlug.VerifiedAt,
		RejectionReason:    bySlug.RejectionReason,
		CreatedAt:          bySlug.CreatedAt,
		UpdatedAt:          bySlug.UpdatedAt,
		Version:            bySlug.Version,
		OwnerName:          bySlug.OwnerName,
		OwnerEmail:         bySlug.OwnerEmail,
		StateName:          bySlug.StateName,
		LgaName:            bySlug.LgaName,
	}, nil
}

func (s *CareersService) GetMyEmployer(ctx context.Context, userID string) (*db.GetCareersEmployerByIDRow, error) {
	emp, err := s.repo.Queries().GetCareersEmployerByOwner(ctx, userID)
	if err != nil || emp.ID == "" {
		return nil, ErrEmployerNotFound
	}
	return s.GetEmployer(ctx, emp.ID)
}

func (s *CareersService) UpdateEmployer(ctx context.Context, user *db.User, employerID string, input UpdateEmployerInput) (*db.CareersEmployer, error) {
	if user == nil {
		return nil, errors.New("unauthenticated")
	}

	if !s.isAuthorizedRecruiter(ctx, employerID, user.ID) {
		return nil, ErrUnauthorizedRecruiterAction
	}

	emp, err := s.repo.Queries().UpdateCareersEmployer(ctx, db.UpdateCareersEmployerParams{
		ID:               employerID,
		OrganizationName: strings.TrimSpace(input.OrganizationName),
		Industry:         strings.TrimSpace(input.Industry),
		CompanySize:      input.CompanySize,
		Description:      input.Description,
		WebsiteUrl:       Text(input.WebsiteURL),
		ContactEmail:     strings.TrimSpace(input.ContactEmail),
		ContactPhone:     strings.TrimSpace(input.ContactPhone),
		LogoUrl:          Text(input.LogoURL),
		StateID:          input.StateID,
		LgaID:            Text(input.LgaID),
		Address:          input.Address,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to update employer: %w", err)
	}

	_ = s.audit.Log(ctx, &user.ID, &user.PrimaryRole, "CAREERS_EMPLOYER_UPDATED", "careers_employer", emp.ID, nil, nil, strPtr("Employer updated"), nil)
	return &emp, nil
}

func (s *CareersService) AddRecruiter(ctx context.Context, adminUser *db.User, employerID string, targetUserID string, role string) (*db.CareersRecruiter, error) {
	if !s.isAuthorizedRecruiter(ctx, employerID, adminUser.ID) {
		return nil, ErrUnauthorizedRecruiterAction
	}

	rec, err := s.repo.Queries().AddCareersRecruiter(ctx, db.AddCareersRecruiterParams{
		ID:         security.GenerateUUID(),
		EmployerID: employerID,
		UserID:     targetUserID,
		Role:       role,
		IsActive:   true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to add recruiter: %w", err)
	}
	return &rec, nil
}

func (s *CareersService) ListRecruiters(ctx context.Context, user *db.User, employerID string) ([]db.ListCareersRecruitersByEmployerRow, error) {
	if !s.isAuthorizedRecruiter(ctx, employerID, user.ID) {
		return nil, ErrUnauthorizedRecruiterAction
	}
	return s.repo.Queries().ListCareersRecruitersByEmployer(ctx, employerID)
}

// -------------------------------------------------------------
// Job Vacancies & Listings
// -------------------------------------------------------------

func (s *CareersService) CreateJob(ctx context.Context, user *db.User, employerID string, input CreateJobInput) (*db.CareersJob, error) {
	if user == nil {
		return nil, errors.New("unauthenticated")
	}

	if !s.isAuthorizedRecruiter(ctx, employerID, user.ID) {
		return nil, ErrUnauthorizedRecruiterAction
	}

	emp, err := s.repo.Queries().GetCareersEmployerByID(ctx, employerID)
	if err != nil {
		return nil, ErrEmployerNotFound
	}

	status := "draft"
	if !input.AsDraft {
		if emp.VerificationStatus == "verified" {
			status = "published"
		} else {
			status = "pending_approval"
		}
	}

	skillsJSON, _ := json.Marshal(input.SkillsRequired)
	if skillsJSON == nil {
		skillsJSON = []byte("[]")
	}

	slug := fmt.Sprintf("%s-%s", slugify(input.Title), strings.ToLower(security.GenerateUUID()[:6]))
	jobID := security.GenerateUUID()

	var deadlinePg pgtype.Timestamptz
	if input.DeadlineAt != nil && !input.DeadlineAt.IsZero() {
		deadlinePg = TimestamptzFromTime(*input.DeadlineAt)
	}

	job, err := s.repo.Queries().CreateCareersJob(ctx, db.CreateCareersJobParams{
		ID:                 jobID,
		EmployerID:         employerID,
		Title:              strings.TrimSpace(input.Title),
		Slug:               slug,
		Category:           strings.TrimSpace(input.Category),
		EmploymentType:     input.EmploymentType,
		WorkplaceType:      input.WorkplaceType,
		StateID:            Text(input.StateID),
		LgaID:              Text(input.LgaID),
		LocationSummary:    input.LocationSummary,
		SalaryMinKobo:      Int8(input.SalaryMinKobo),
		SalaryMaxKobo:      Int8(input.SalaryMaxKobo),
		Currency:           "NGN",
		IsSalaryNegotiable: input.IsSalaryNegotiable,
		Description:        input.Description,
		Responsibilities:   input.Responsibilities,
		Requirements:       input.Requirements,
		SkillsRequired:     skillsJSON,
		ExperienceLevel:    input.ExperienceLevel,
		DeadlineAt:         deadlinePg,
		Status:             status,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create job opportunity: %w", err)
	}

	_ = s.audit.Log(ctx, &user.ID, &user.PrimaryRole, "CAREERS_JOB_CREATED", "careers_job", job.ID, nil, nil, strPtr("Job created: "+job.Title), nil)
	return &job, nil
}

func (s *CareersService) UpdateJob(ctx context.Context, user *db.User, jobID string, input CreateJobInput) (*db.CareersJob, error) {
	job, err := s.repo.Queries().GetCareersJobByID(ctx, jobID)
	if err != nil {
		return nil, ErrJobNotFound
	}

	if !s.isAuthorizedRecruiter(ctx, job.EmployerID, user.ID) {
		return nil, ErrUnauthorizedRecruiterAction
	}

	skillsJSON, _ := json.Marshal(input.SkillsRequired)
	if skillsJSON == nil {
		skillsJSON = []byte("[]")
	}

	var deadlinePg pgtype.Timestamptz
	if input.DeadlineAt != nil && !input.DeadlineAt.IsZero() {
		deadlinePg = TimestamptzFromTime(*input.DeadlineAt)
	}

	updated, err := s.repo.Queries().UpdateCareersJob(ctx, db.UpdateCareersJobParams{
		ID:                 jobID,
		Title:              strings.TrimSpace(input.Title),
		Category:           strings.TrimSpace(input.Category),
		EmploymentType:     input.EmploymentType,
		WorkplaceType:      input.WorkplaceType,
		StateID:            Text(input.StateID),
		LgaID:              Text(input.LgaID),
		LocationSummary:    input.LocationSummary,
		SalaryMinKobo:      Int8(input.SalaryMinKobo),
		SalaryMaxKobo:      Int8(input.SalaryMaxKobo),
		IsSalaryNegotiable: input.IsSalaryNegotiable,
		Description:        input.Description,
		Responsibilities:   input.Responsibilities,
		Requirements:       input.Requirements,
		SkillsRequired:     skillsJSON,
		ExperienceLevel:    input.ExperienceLevel,
		DeadlineAt:         deadlinePg,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to update job: %w", err)
	}

	_ = s.audit.Log(ctx, &user.ID, &user.PrimaryRole, "CAREERS_JOB_UPDATED", "careers_job", job.ID, nil, nil, strPtr("Job updated: "+job.Title), nil)
	return &updated, nil
}

func (s *CareersService) UpdateJobStatus(ctx context.Context, user *db.User, jobID string, newStatus string) (*db.CareersJob, error) {
	job, err := s.repo.Queries().GetCareersJobByID(ctx, jobID)
	if err != nil {
		return nil, ErrJobNotFound
	}

	if !s.isAuthorizedRecruiter(ctx, job.EmployerID, user.ID) {
		return nil, ErrUnauthorizedRecruiterAction
	}

	// Recruiter can toggle: draft, published (if employer verified), paused, closed
	if newStatus == "published" && job.EmployerVerificationStatus != "verified" {
		return nil, ErrEmployerNotVerified
	}

	validStatuses := map[string]bool{"draft": true, "published": true, "paused": true, "closed": true}
	if !validStatuses[newStatus] {
		return nil, fmt.Errorf("unsupported recruiter status '%s'", newStatus)
	}

	updated, err := s.repo.Queries().UpdateCareersJobStatus(ctx, db.UpdateCareersJobStatusParams{
		ID:                jobID,
		Status:            newStatus,
		ModeratedByUserID: pgtype.Text{Valid: false},
		ModeratedAt:       pgtype.Timestamptz{Valid: false},
		ModerationNotes:   pgtype.Text{Valid: false},
	})
	if err != nil {
		return nil, err
	}

	_ = s.audit.Log(ctx, &user.ID, &user.PrimaryRole, "CAREERS_JOB_STATUS_CHANGED", "careers_job", job.ID, nil, nil, strPtr("Job status changed to "+newStatus), nil)
	return &updated, nil
}

func (s *CareersService) ListPublicJobs(ctx context.Context, filter PublicJobsFilter) ([]db.ListPublicCareersJobsRow, error) {
	limit := filter.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	return s.repo.Queries().ListPublicCareersJobs(ctx, db.ListPublicCareersJobsParams{
		Column1: filter.Category,
		Column2: filter.EmploymentType,
		Column3: filter.WorkplaceType,
		Column4: filter.StateID,
		Column5: filter.ExperienceLevel,
		Column6: filter.Query,
		Limit:   limit,
		Offset:  filter.Offset,
	})
}

func (s *CareersService) GetPublicJob(ctx context.Context, idOrSlug string) (*db.GetCareersJobByIDRow, error) {
	job, err := s.repo.Queries().GetCareersJobByID(ctx, idOrSlug)
	if err == nil && job.ID != "" {
		_ = s.repo.Queries().IncrementCareersJobViews(ctx, job.ID)
		return &job, nil
	}

	bySlug, errSlug := s.repo.Queries().GetCareersJobBySlug(ctx, idOrSlug)
	if errSlug != nil {
		return nil, ErrJobNotFound
	}

	_ = s.repo.Queries().IncrementCareersJobViews(ctx, bySlug.ID)
	return &db.GetCareersJobByIDRow{
		ID:                         bySlug.ID,
		EmployerID:                 bySlug.EmployerID,
		Title:                      bySlug.Title,
		Slug:                       bySlug.Slug,
		Category:                   bySlug.Category,
		EmploymentType:             bySlug.EmploymentType,
		WorkplaceType:              bySlug.WorkplaceType,
		StateID:                    bySlug.StateID,
		LgaID:                      bySlug.LgaID,
		LocationSummary:            bySlug.LocationSummary,
		SalaryMinKobo:              bySlug.SalaryMinKobo,
		SalaryMaxKobo:              bySlug.SalaryMaxKobo,
		Currency:                   bySlug.Currency,
		IsSalaryNegotiable:         bySlug.IsSalaryNegotiable,
		Description:                bySlug.Description,
		Responsibilities:           bySlug.Responsibilities,
		Requirements:               bySlug.Requirements,
		SkillsRequired:             bySlug.SkillsRequired,
		ExperienceLevel:            bySlug.ExperienceLevel,
		DeadlineAt:                 bySlug.DeadlineAt,
		Status:                     bySlug.Status,
		ModeratedByUserID:          bySlug.ModeratedByUserID,
		ModeratedAt:                bySlug.ModeratedAt,
		ModerationNotes:            bySlug.ModerationNotes,
		ViewsCount:                 bySlug.ViewsCount,
		ApplicationsCount:          bySlug.ApplicationsCount,
		CreatedAt:                  bySlug.CreatedAt,
		UpdatedAt:                  bySlug.UpdatedAt,
		Version:                    bySlug.Version,
		EmployerName:               bySlug.EmployerName,
		EmployerSlug:               bySlug.EmployerSlug,
		EmployerLogo:               bySlug.EmployerLogo,
		EmployerVerificationStatus: bySlug.EmployerVerificationStatus,
		EmployerIndustry:           bySlug.EmployerIndustry,
		EmployerWebsite:            bySlug.EmployerWebsite,
		StateName:                  bySlug.StateName,
		LgaName:                    bySlug.LgaName,
	}, nil
}

func (s *CareersService) ListEmployerJobs(ctx context.Context, user *db.User, employerID string, status string, limit, offset int32) ([]db.ListCareersJobsByEmployerRow, error) {
	if !s.isAuthorizedRecruiter(ctx, employerID, user.ID) {
		return nil, ErrUnauthorizedRecruiterAction
	}

	if limit <= 0 || limit > 100 {
		limit = 50
	}

	return s.repo.Queries().ListCareersJobsByEmployer(ctx, db.ListCareersJobsByEmployerParams{
		EmployerID: employerID,
		Column2:    status,
		Limit:      limit,
		Offset:     offset,
	})
}

// -------------------------------------------------------------
// Candidate Profiles (Private)
// -------------------------------------------------------------

func (s *CareersService) UpsertCandidateProfile(ctx context.Context, user *db.User, input CandidateProfileInput) (*db.CareersCandidateProfile, error) {
	if user == nil {
		return nil, errors.New("unauthenticated")
	}

	skillsJSON, _ := json.Marshal(input.Skills)
	if skillsJSON == nil {
		skillsJSON = []byte("[]")
	}
	eduJSON, _ := json.Marshal(input.Education)
	if eduJSON == nil {
		eduJSON = []byte("[]")
	}
	expJSON, _ := json.Marshal(input.Experience)
	if expJSON == nil {
		expJSON = []byte("[]")
	}
	certJSON, _ := json.Marshal(input.Certifications)
	if certJSON == nil {
		certJSON = []byte("[]")
	}
	prefCatJSON, _ := json.Marshal(input.PreferredCategories)
	if prefCatJSON == nil {
		prefCatJSON = []byte("[]")
	}

	workplaceType := input.PreferredWorkplaceType
	if workplaceType == "" {
		workplaceType = "any"
	}

	profID := security.GenerateUUID()
	profile, err := s.repo.Queries().UpsertCandidateProfile(ctx, db.UpsertCandidateProfileParams{
		ID:                                profID,
		UserID:                            user.ID,
		ProfessionalHeadline:              input.ProfessionalHeadline,
		Summary:                           input.Summary,
		Skills:                            skillsJSON,
		Education:                         eduJSON,
		Experience:                        expJSON,
		Certifications:                    certJSON,
		PreferredCategories:               prefCatJSON,
		PreferredWorkplaceType:            workplaceType,
		PreferredStateID:                  Text(input.PreferredStateID),
		ResumeUrl:                         input.ResumeURL,
		ResumeFilename:                    input.ResumeFilename,
		ResumeUploadedAt:                  TimestamptzFromTime(time.Now()),
		IsSearchableByVerifiedEmployers:   input.IsSearchableByVerifiedEmployers,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to save candidate profile: %w", err)
	}

	return &profile, nil
}

func (s *CareersService) GetMyCandidateProfile(ctx context.Context, user *db.User) (*db.GetCandidateProfileByUserIDRow, error) {
	if user == nil {
		return nil, errors.New("unauthenticated")
	}

	profile, err := s.repo.Queries().GetCandidateProfileByUserID(ctx, user.ID)
	if err != nil {
		return nil, ErrCandidateProfileNotFound
	}
	return &profile, nil
}

func (s *CareersService) GetCandidateProfileForRecruiter(ctx context.Context, user *db.User, candidateUserID string, applicationID string) (*db.GetCandidateProfileByUserIDRow, error) {
	app, err := s.repo.Queries().GetCareersApplicationByID(ctx, applicationID)
	if err != nil {
		return nil, ErrApplicationNotFound
	}

	if app.CandidateUserID != candidateUserID {
		return nil, ErrUnauthorizedCandidateAction
	}

	// Must be authorized recruiter for the employer associated with this application
	if !s.isAuthorizedRecruiter(ctx, app.EmployerID, user.ID) {
		return nil, ErrUnauthorizedRecruiterAction
	}

	profile, err := s.repo.Queries().GetCandidateProfileByUserID(ctx, candidateUserID)
	if err != nil {
		return nil, ErrCandidateProfileNotFound
	}
	return &profile, nil
}

// -------------------------------------------------------------
// Job Applications & Fair Explainable Matching
// -------------------------------------------------------------

func (s *CareersService) ApplyForJob(ctx context.Context, user *db.User, jobID string, input SubmitApplicationInput) (*db.CareersApplication, error) {
	if user == nil {
		return nil, errors.New("unauthenticated")
	}

	job, err := s.repo.Queries().GetCareersJobByID(ctx, jobID)
	if err != nil {
		return nil, ErrJobNotFound
	}

	if job.Status != "published" {
		return nil, ErrJobNotPublished
	}

	if job.DeadlineAt.Valid && job.DeadlineAt.Time.Before(time.Now()) {
		return nil, ErrJobClosedOrExpired
	}

	// Check for duplicate application
	existing, _ := s.repo.Queries().GetCareersApplicationByJobAndCandidate(ctx, db.GetCareersApplicationByJobAndCandidateParams{
		JobID:           jobID,
		CandidateUserID: user.ID,
	})
	if existing.ID != "" && existing.Status != "withdrawn" {
		return nil, ErrDuplicateApplication
	}

	// Fetch candidate profile to compute explainable match score
	candProfile, _ := s.repo.Queries().GetCandidateProfileByUserID(ctx, user.ID)
	matchScore, matchFactors := s.calculateExplainableMatch(candProfile, job)
	matchFactorsJSON, _ := json.Marshal(matchFactors)

	resumeURL := input.ResumeURL
	resumeFilename := input.ResumeFilename
	if resumeURL == "" && candProfile.ID != "" {
		resumeURL = candProfile.ResumeUrl
		resumeFilename = candProfile.ResumeFilename
	}
	if resumeURL == "" {
		resumeURL = "/resumes/placeholder-resume.pdf"
		resumeFilename = "resume.pdf"
	}

	appID := security.GenerateUUID()
	app, err := s.repo.Queries().CreateCareersApplication(ctx, db.CreateCareersApplicationParams{
		ID:                    appID,
		JobID:                 job.ID,
		CandidateUserID:       user.ID,
		EmployerID:            job.EmployerID,
		CoverLetter:           input.CoverLetter,
		QualificationsSummary: input.QualificationsSummary,
		ResumeUrl:             resumeURL,
		ResumeFilename:        resumeFilename,
		MatchScore:            int32(matchScore),
		MatchFactors:          matchFactorsJSON,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to submit application: %w", err)
	}

	// Increment job applications count
	_ = s.repo.Queries().IncrementCareersJobApplications(ctx, job.ID)

	// Record in application history
	histID := security.GenerateUUID()
	_, _ = s.repo.Queries().CreateCareersApplicationHistory(ctx, db.CreateCareersApplicationHistoryParams{
		ID:              histID,
		ApplicationID:   app.ID,
		FromStatus:      "none",
		ToStatus:        "submitted",
		ChangedByUserID: Text(&user.ID),
		Notes:           "Application submitted by candidate",
	})

	// Dispatch notification
	_ = s.delivery.SendVerificationEmail(user.Email.String, user.DisplayName, fmt.Sprintf("https://ratels.onrender.com/careers/applications"))

	return &app, nil
}

func (s *CareersService) calculateExplainableMatch(cand db.GetCandidateProfileByUserIDRow, job db.GetCareersJobByIDRow) (int, map[string]interface{}) {
	score := 0
	var matchedSkills []string

	// 1. Skill Matching (up to 40 points)
	var jobSkills []string
	_ = json.Unmarshal(job.SkillsRequired, &jobSkills)

	var candSkills []string
	if cand.ID != "" {
		_ = json.Unmarshal(cand.Skills, &candSkills)
	}

	skillPoints := 40
	if len(jobSkills) > 0 {
		candSkillMap := make(map[string]bool)
		for _, s := range candSkills {
			candSkillMap[strings.ToLower(strings.TrimSpace(s))] = true
		}

		matchedCount := 0
		for _, req := range jobSkills {
			if candSkillMap[strings.ToLower(strings.TrimSpace(req))] {
				matchedCount++
				matchedSkills = append(matchedSkills, req)
			}
		}
		skillPoints = (matchedCount * 40) / len(jobSkills)
	}
	score += skillPoints

	// 2. Workplace / Location Matching (up to 30 points)
	locPoints := 10
	if job.WorkplaceType == "remote" || cand.PreferredWorkplaceType == "remote" {
		locPoints = 30
	} else if job.StateID.Valid && cand.PreferredStateID.Valid && job.StateID.String == cand.PreferredStateID.String {
		locPoints = 30
	} else if job.WorkplaceType == "hybrid" {
		locPoints = 20
	}
	score += locPoints

	// 3. Category / Industry Alignment (up to 20 points)
	var candCats []string
	if cand.ID != "" {
		_ = json.Unmarshal(cand.PreferredCategories, &candCats)
	}
	catPoints := 10
	for _, c := range candCats {
		if strings.EqualFold(c, job.Category) {
			catPoints = 20
			break
		}
	}
	score += catPoints

	// 4. Experience Record (up to 10 points)
	expPoints := 0
	var candExps []interface{}
	if cand.ID != "" {
		_ = json.Unmarshal(cand.Experience, &candExps)
		if len(candExps) > 0 {
			expPoints = 10
		}
	}
	score += expPoints

	if score > 100 {
		score = 100
	}

	factors := map[string]interface{}{
		"skills_score":         skillPoints,
		"skills_matched":       matchedSkills,
		"location_score":       locPoints,
		"category_score":       catPoints,
		"experience_score":     expPoints,
		"criteria_explanation": "Explainable rules-based candidate matching based strictly on skills, location preference, and experience. No personal demographic data is evaluated.",
	}

	return score, factors
}

func (s *CareersService) WithdrawApplication(ctx context.Context, user *db.User, applicationID string, reason string) (*db.CareersApplication, error) {
	app, err := s.repo.Queries().GetCareersApplicationByID(ctx, applicationID)
	if err != nil {
		return nil, ErrApplicationNotFound
	}

	if app.CandidateUserID != user.ID {
		return nil, ErrUnauthorizedCandidateAction
	}

	if app.Status == "withdrawn" || app.Status == "hired" || app.Status == "rejected" {
		return nil, ErrApplicationCannotBeWithdrawn
	}

	fromStatus := app.Status
	updated, err := s.repo.Queries().UpdateCareersApplicationStatus(ctx, db.UpdateCareersApplicationStatusParams{
		ID:                    applicationID,
		Status:                "withdrawn",
		RejectionReason:       "",
		WithdrawalReason:      reason,
		StatusUpdatedByUserID: Text(&user.ID),
	})
	if err != nil {
		return nil, err
	}

	_ = s.repo.Queries().DecrementCareersJobApplications(ctx, app.JobID)

	// Record history
	histID := security.GenerateUUID()
	_, _ = s.repo.Queries().CreateCareersApplicationHistory(ctx, db.CreateCareersApplicationHistoryParams{
		ID:              histID,
		ApplicationID:   applicationID,
		FromStatus:      fromStatus,
		ToStatus:        "withdrawn",
		ChangedByUserID: Text(&user.ID),
		Notes:           reason,
	})

	return &updated, nil
}

func (s *CareersService) ListMyApplications(ctx context.Context, user *db.User) ([]db.ListCareersApplicationsByCandidateRow, error) {
	if user == nil {
		return nil, errors.New("unauthenticated")
	}
	return s.repo.Queries().ListCareersApplicationsByCandidate(ctx, user.ID)
}

func (s *CareersService) GetApplication(ctx context.Context, user *db.User, applicationID string) (*db.GetCareersApplicationByIDRow, error) {
	app, err := s.repo.Queries().GetCareersApplicationByID(ctx, applicationID)
	if err != nil {
		return nil, ErrApplicationNotFound
	}

	// Candidate or authorized recruiter or admin
	isCandidate := app.CandidateUserID == user.ID
	isRecruiter := s.isAuthorizedRecruiter(ctx, app.EmployerID, user.ID)
	isAdmin := user.PrimaryRole == "super_admin" || user.PrimaryRole == "national_admin"

	if !isCandidate && !isRecruiter && !isAdmin {
		return nil, ErrUnauthorizedCandidateAction
	}

	return &app, nil
}

// -------------------------------------------------------------
// Recruiter Operations (Applicant Management & Interviews)
// -------------------------------------------------------------

func (s *CareersService) ListJobApplications(ctx context.Context, user *db.User, jobID string, status string, limit, offset int32) ([]db.ListCareersApplicationsByJobRow, error) {
	job, err := s.repo.Queries().GetCareersJobByID(ctx, jobID)
	if err != nil {
		return nil, ErrJobNotFound
	}

	if !s.isAuthorizedRecruiter(ctx, job.EmployerID, user.ID) {
		return nil, ErrUnauthorizedRecruiterAction
	}

	if limit <= 0 || limit > 100 {
		limit = 50
	}

	return s.repo.Queries().ListCareersApplicationsByJob(ctx, db.ListCareersApplicationsByJobParams{
		JobID:   jobID,
		Column2: status,
		Limit:   limit,
		Offset:  offset,
	})
}

func (s *CareersService) UpdateApplicationStatus(ctx context.Context, user *db.User, applicationID string, newStatus string, reason string) (*db.CareersApplication, error) {
	app, err := s.repo.Queries().GetCareersApplicationByID(ctx, applicationID)
	if err != nil {
		return nil, ErrApplicationNotFound
	}

	if !s.isAuthorizedRecruiter(ctx, app.EmployerID, user.ID) {
		return nil, ErrUnauthorizedRecruiterAction
	}

	// Validate status transition
	validTransitions := map[string][]string{
		"submitted":    {"under_review", "shortlisted", "rejected"},
		"under_review": {"shortlisted", "rejected"},
		"shortlisted":  {"interview", "rejected"},
		"interview":    {"offered", "rejected"},
		"offered":      {"hired", "rejected"},
		"rejected":     {},
		"withdrawn":    {},
		"hired":        {},
	}

	allowed := false
	for _, target := range validTransitions[app.Status] {
		if target == newStatus {
			allowed = true
			break
		}
	}

	if !allowed {
		return nil, fmt.Errorf("%w: cannot transition from '%s' to '%s'", ErrInvalidApplicationStatus, app.Status, newStatus)
	}

	fromStatus := app.Status
	updated, err := s.repo.Queries().UpdateCareersApplicationStatus(ctx, db.UpdateCareersApplicationStatusParams{
		ID:                    applicationID,
		Status:                newStatus,
		RejectionReason:       reason,
		WithdrawalReason:      "",
		StatusUpdatedByUserID: Text(&user.ID),
	})
	if err != nil {
		return nil, err
	}

	// Record history
	histID := security.GenerateUUID()
	_, _ = s.repo.Queries().CreateCareersApplicationHistory(ctx, db.CreateCareersApplicationHistoryParams{
		ID:              histID,
		ApplicationID:   applicationID,
		FromStatus:      fromStatus,
		ToStatus:        newStatus,
		ChangedByUserID: Text(&user.ID),
		Notes:           reason,
	})

	// Dispatch notification
	if app.CandidateEmail.Valid {
		_ = s.delivery.SendVerificationEmail(app.CandidateEmail.String, app.CandidateName, "https://ratels.onrender.com/careers/applications")
	}

	return &updated, nil
}

func (s *CareersService) ScheduleInterview(ctx context.Context, user *db.User, applicationID string, input ScheduleInterviewInput) (*db.CareersInterview, error) {
	app, err := s.repo.Queries().GetCareersApplicationByID(ctx, applicationID)
	if err != nil {
		return nil, ErrApplicationNotFound
	}

	if !s.isAuthorizedRecruiter(ctx, app.EmployerID, user.ID) {
		return nil, ErrUnauthorizedRecruiterAction
	}

	intID := security.GenerateUUID()
	interview, err := s.repo.Queries().CreateCareersInterview(ctx, db.CreateCareersInterviewParams{
		ID:              intID,
		ApplicationID:   app.ID,
		JobID:           app.JobID,
		EmployerID:      app.EmployerID,
		CandidateUserID: app.CandidateUserID,
		ScheduledTime:   TimestamptzFromTime(input.ScheduledTime),
		Format:          input.Format,
		LocationOrLink:  input.LocationOrLink,
		Instructions:    input.Instructions,
		CreatedByUserID: Text(&user.ID),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to schedule interview: %w", err)
	}

	// If application is shortlisted, advance to interview status
	if app.Status == "shortlisted" {
		_, _ = s.UpdateApplicationStatus(ctx, user, applicationID, "interview", "Interview invitation scheduled")
	}

	return &interview, nil
}

func (s *CareersService) ListInterviewsForCandidate(ctx context.Context, user *db.User) ([]db.ListCareersInterviewsByCandidateRow, error) {
	return s.repo.Queries().ListCareersInterviewsByCandidate(ctx, user.ID)
}

func (s *CareersService) ListInterviewsForEmployer(ctx context.Context, user *db.User, employerID string) ([]db.ListCareersInterviewsByEmployerRow, error) {
	if !s.isAuthorizedRecruiter(ctx, employerID, user.ID) {
		return nil, ErrUnauthorizedRecruiterAction
	}
	return s.repo.Queries().ListCareersInterviewsByEmployer(ctx, employerID)
}

// -------------------------------------------------------------
// Admin Moderation & Oversight (/admin/careers)
// -------------------------------------------------------------

func (s *CareersService) GetAdminOverview(ctx context.Context, admin *db.User) (*db.GetCareersAdminOverviewRow, error) {
	res, err := s.repo.Queries().GetCareersAdminOverview(ctx)
	if err != nil {
		return nil, err
	}
	return &res, nil
}

func (s *CareersService) GetEmployerOverview(ctx context.Context, user *db.User, employerID string) (*db.GetCareersEmployerOverviewRow, error) {
	if !s.isAuthorizedRecruiter(ctx, employerID, user.ID) {
		return nil, ErrUnauthorizedRecruiterAction
	}
	res, err := s.repo.Queries().GetCareersEmployerOverview(ctx, employerID)
	if err != nil {
		return nil, err
	}
	return &res, nil
}

func (s *CareersService) ListEmployersAdmin(ctx context.Context, admin *db.User, status, industry, stateID, search string, limit, offset int32) ([]db.ListCareersEmployersRow, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	return s.repo.Queries().ListCareersEmployers(ctx, db.ListCareersEmployersParams{
		Column1: status,
		Column2: industry,
		Column3: stateID,
		Column4: search,
		Limit:   limit,
		Offset:  offset,
	})
}

func (s *CareersService) VerifyEmployer(ctx context.Context, admin *db.User, employerID string, isApproved bool, reason string) (*db.CareersEmployer, error) {
	status := "verified"
	if !isApproved {
		status = "rejected"
	}

	emp, err := s.repo.Queries().UpdateCareersEmployerVerification(ctx, db.UpdateCareersEmployerVerificationParams{
		ID:                 employerID,
		VerificationStatus: status,
		VerifiedByUserID:   Text(&admin.ID),
		VerifiedAt:         TimestamptzFromTime(time.Now()),
		RejectionReason:    Text(&reason),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to verify employer: %w", err)
	}

	_ = s.audit.Log(ctx, &admin.ID, &admin.PrimaryRole, "CAREERS_EMPLOYER_VERIFIED", "careers_employer", emp.ID, nil, nil, strPtr(fmt.Sprintf("Employer verification set to %s. Notes: %s", status, reason)), nil)
	return &emp, nil
}

func (s *CareersService) ListJobsAdmin(ctx context.Context, admin *db.User, status, category, employerID, stateID, search string, limit, offset int32) ([]db.ListAllCareersJobsAdminRow, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	return s.repo.Queries().ListAllCareersJobsAdmin(ctx, db.ListAllCareersJobsAdminParams{
		Column1: status,
		Column2: category,
		Column3: employerID,
		Column4: stateID,
		Column5: search,
		Limit:   limit,
		Offset:  offset,
	})
}

func (s *CareersService) ModerateJob(ctx context.Context, admin *db.User, jobID string, newStatus string, notes string) (*db.CareersJob, error) {
	job, err := s.repo.Queries().GetCareersJobByID(ctx, jobID)
	if err != nil {
		return nil, ErrJobNotFound
	}

	updated, err := s.repo.Queries().UpdateCareersJobStatus(ctx, db.UpdateCareersJobStatusParams{
		ID:                jobID,
		Status:            newStatus,
		ModeratedByUserID: Text(&admin.ID),
		ModeratedAt:       TimestamptzFromTime(time.Now()),
		ModerationNotes:   Text(&notes),
	})
	if err != nil {
		return nil, err
	}

	_ = s.audit.Log(ctx, &admin.ID, &admin.PrimaryRole, "CAREERS_JOB_MODERATED", "careers_job", job.ID, nil, nil, strPtr(fmt.Sprintf("Job moderated to %s. Notes: %s", newStatus, notes)), nil)
	return &updated, nil
}

func (s *CareersService) ListApplicationsAdmin(ctx context.Context, admin *db.User, status, employerID, jobID string, limit, offset int32) ([]db.ListCareersApplicationsAdminRow, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	return s.repo.Queries().ListCareersApplicationsAdmin(ctx, db.ListCareersApplicationsAdminParams{
		Column1: status,
		Column2: employerID,
		Column3: jobID,
		Limit:   limit,
		Offset:  offset,
	})
}

func (s *CareersService) GetApplicationHistory(ctx context.Context, user *db.User, applicationID string) ([]db.ListCareersApplicationHistoryRow, error) {
	app, err := s.repo.Queries().GetCareersApplicationByID(ctx, applicationID)
	if err != nil {
		return nil, ErrApplicationNotFound
	}

	isCandidate := app.CandidateUserID == user.ID
	isRecruiter := s.isAuthorizedRecruiter(ctx, app.EmployerID, user.ID)
	isAdmin := user.PrimaryRole == "super_admin" || user.PrimaryRole == "national_admin"

	if !isCandidate && !isRecruiter && !isAdmin {
		return nil, ErrUnauthorizedCandidateAction
	}

	return s.repo.Queries().ListCareersApplicationHistory(ctx, applicationID)
}

// -------------------------------------------------------------
// Helper Authorization Checks
// -------------------------------------------------------------

func (s *CareersService) isAuthorizedRecruiter(ctx context.Context, employerID string, userID string) bool {
	emp, err := s.repo.Queries().GetCareersEmployerByID(ctx, employerID)
	if err == nil && emp.OwnerUserID == userID {
		return true
	}

	rec, errRec := s.repo.Queries().GetCareersRecruiter(ctx, db.GetCareersRecruiterParams{
		EmployerID: employerID,
		UserID:     userID,
	})
	if errRec == nil && rec.ID != "" && rec.IsActive {
		return true
	}

	return false
}
