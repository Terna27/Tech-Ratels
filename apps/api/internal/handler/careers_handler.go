package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"ratels-api/internal/db"
	"ratels-api/internal/service"
)

type CareersHandler struct {
	careersService *service.CareersService
}

func NewCareersHandler(careersService *service.CareersService) *CareersHandler {
	return &CareersHandler{careersService: careersService}
}

// -------------------------------------------------------------
// Public Endpoints
// -------------------------------------------------------------

func (h *CareersHandler) ListPublicJobs(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))

	filter := service.PublicJobsFilter{
		Category:        q.Get("category"),
		EmploymentType:  q.Get("employment_type"),
		WorkplaceType:   q.Get("workplace_type"),
		StateID:         q.Get("state_id"),
		ExperienceLevel: q.Get("experience_level"),
		Query:           q.Get("q"),
		Limit:           int32(limit),
		Offset:          int32(offset),
	}

	jobs, err := h.careersService.ListPublicJobs(r.Context(), filter)
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}
	if jobs == nil {
		jobs = []db.ListPublicCareersJobsRow{}
	}
	JSON(w, http.StatusOK, jobs)
}

func (h *CareersHandler) GetPublicJob(w http.ResponseWriter, r *http.Request) {
	idOrSlug := chi.URLParam(r, "id")
	job, err := h.careersService.GetPublicJob(r.Context(), idOrSlug)
	if err != nil {
		if errors.Is(err, service.ErrJobNotFound) {
			ErrorJSON(w, http.StatusNotFound, "NOT_FOUND", "Job vacancy not found", nil)
			return
		}
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}
	JSON(w, http.StatusOK, job)
}

func (h *CareersHandler) GetEmployer(w http.ResponseWriter, r *http.Request) {
	idOrSlug := chi.URLParam(r, "id")
	emp, err := h.careersService.GetEmployer(r.Context(), idOrSlug)
	if err != nil {
		if errors.Is(err, service.ErrEmployerNotFound) {
			ErrorJSON(w, http.StatusNotFound, "NOT_FOUND", "Employer organization not found", nil)
			return
		}
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}
	JSON(w, http.StatusOK, emp)
}

// -------------------------------------------------------------
// Candidate Portal Endpoints
// -------------------------------------------------------------

func (h *CareersHandler) GetMyProfile(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	profile, err := h.careersService.GetMyCandidateProfile(r.Context(), user)
	if err != nil {
		if errors.Is(err, service.ErrCandidateProfileNotFound) {
			// Return default empty profile wrapper
			JSON(w, http.StatusOK, map[string]interface{}{
				"user_id":              user.ID,
				"display_name":         user.DisplayName,
				"email":                user.Email,
				"professional_headline": "",
				"summary":              "",
				"skills":               []string{},
				"education":            []interface{}{},
				"experience":           []interface{}{},
				"certifications":       []interface{}{},
				"preferred_categories": []string{},
				"preferred_workplace_type": "any",
				"is_profile_created":   false,
			})
			return
		}
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}
	JSON(w, http.StatusOK, profile)
}

func (h *CareersHandler) UpsertMyProfile(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	var input service.CandidateProfileInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid request body", nil)
		return
	}

	profile, err := h.careersService.UpsertCandidateProfile(r.Context(), user, input)
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}
	JSON(w, http.StatusOK, profile)
}

func (h *CareersHandler) ApplyForJob(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	jobID := chi.URLParam(r, "id")
	var input service.SubmitApplicationInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil && err != io.EOF {
		ErrorJSON(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid request body", nil)
		return
	}

	app, err := h.careersService.ApplyForJob(r.Context(), user, jobID, input)
	if err != nil {
		if errors.Is(err, service.ErrDuplicateApplication) {
			ErrorJSON(w, http.StatusConflict, "DUPLICATE_APPLICATION", err.Error(), nil)
			return
		}
		if errors.Is(err, service.ErrJobClosedOrExpired) || errors.Is(err, service.ErrJobNotPublished) {
			ErrorJSON(w, http.StatusBadRequest, "JOB_NOT_AVAILABLE", err.Error(), nil)
			return
		}
		if errors.Is(err, service.ErrJobNotFound) {
			ErrorJSON(w, http.StatusNotFound, "NOT_FOUND", "Job opportunity not found", nil)
			return
		}
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}
	JSON(w, http.StatusCreated, app)
}

func (h *CareersHandler) ListMyApplications(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	apps, err := h.careersService.ListMyApplications(r.Context(), user)
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}
	if apps == nil {
		apps = []db.ListCareersApplicationsByCandidateRow{}
	}
	JSON(w, http.StatusOK, apps)
}

func (h *CareersHandler) GetApplication(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	appID := chi.URLParam(r, "id")
	app, err := h.careersService.GetApplication(r.Context(), user, appID)
	if err != nil {
		if errors.Is(err, service.ErrApplicationNotFound) {
			ErrorJSON(w, http.StatusNotFound, "NOT_FOUND", "Application not found", nil)
			return
		}
		if errors.Is(err, service.ErrUnauthorizedCandidateAction) {
			ErrorJSON(w, http.StatusForbidden, "FORBIDDEN", "Unauthorized to view this application", nil)
			return
		}
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}
	JSON(w, http.StatusOK, app)
}

func (h *CareersHandler) WithdrawApplication(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	appID := chi.URLParam(r, "id")
	var body struct {
		Reason string `json:"reason"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)

	app, err := h.careersService.WithdrawApplication(r.Context(), user, appID, body.Reason)
	if err != nil {
		if errors.Is(err, service.ErrApplicationNotFound) {
			ErrorJSON(w, http.StatusNotFound, "NOT_FOUND", "Application not found", nil)
			return
		}
		if errors.Is(err, service.ErrApplicationCannotBeWithdrawn) {
			ErrorJSON(w, http.StatusBadRequest, "INVALID_STATE", err.Error(), nil)
			return
		}
		if errors.Is(err, service.ErrUnauthorizedCandidateAction) {
			ErrorJSON(w, http.StatusForbidden, "FORBIDDEN", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}
	JSON(w, http.StatusOK, app)
}

func (h *CareersHandler) GetApplicationHistory(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	appID := chi.URLParam(r, "id")
	history, err := h.careersService.GetApplicationHistory(r.Context(), user, appID)
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}
	if history == nil {
		history = []db.ListCareersApplicationHistoryRow{}
	}
	JSON(w, http.StatusOK, history)
}

func (h *CareersHandler) ListMyInterviews(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	interviews, err := h.careersService.ListInterviewsForCandidate(r.Context(), user)
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}
	if interviews == nil {
		interviews = []db.ListCareersInterviewsByCandidateRow{}
	}
	JSON(w, http.StatusOK, interviews)
}

// -------------------------------------------------------------
// Employer & Recruiter Endpoints
// -------------------------------------------------------------

func (h *CareersHandler) RegisterEmployer(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	var input service.RegisterEmployerInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid request body", nil)
		return
	}

	if input.OrganizationName == "" || input.RegistrationNumber == "" || input.ContactEmail == "" {
		ErrorJSON(w, http.StatusBadRequest, "BAD_REQUEST", "Organization name, registration number, and contact email are required", nil)
		return
	}

	emp, err := h.careersService.RegisterEmployer(r.Context(), user, input)
	if err != nil {
		if errors.Is(err, service.ErrEmployerAlreadyExists) {
			ErrorJSON(w, http.StatusConflict, "EMPLOYER_EXISTS", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}
	JSON(w, http.StatusCreated, emp)
}

func (h *CareersHandler) GetMyEmployer(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	emp, err := h.careersService.GetMyEmployer(r.Context(), user.ID)
	if err != nil {
		if errors.Is(err, service.ErrEmployerNotFound) {
			ErrorJSON(w, http.StatusNotFound, "NOT_FOUND", "No employer organization registered for this account", nil)
			return
		}
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}
	JSON(w, http.StatusOK, emp)
}

func (h *CareersHandler) UpdateEmployer(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	empID := chi.URLParam(r, "id")
	var input service.UpdateEmployerInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid request body", nil)
		return
	}

	emp, err := h.careersService.UpdateEmployer(r.Context(), user, empID, input)
	if err != nil {
		if errors.Is(err, service.ErrUnauthorizedRecruiterAction) {
			ErrorJSON(w, http.StatusForbidden, "FORBIDDEN", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}
	JSON(w, http.StatusOK, emp)
}

func (h *CareersHandler) GetEmployerOverview(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	empID := chi.URLParam(r, "id")
	overview, err := h.careersService.GetEmployerOverview(r.Context(), user, empID)
	if err != nil {
		if errors.Is(err, service.ErrUnauthorizedRecruiterAction) {
			ErrorJSON(w, http.StatusForbidden, "FORBIDDEN", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}
	JSON(w, http.StatusOK, overview)
}

func (h *CareersHandler) ListRecruiters(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	empID := chi.URLParam(r, "id")
	recs, err := h.careersService.ListRecruiters(r.Context(), user, empID)
	if err != nil {
		if errors.Is(err, service.ErrUnauthorizedRecruiterAction) {
			ErrorJSON(w, http.StatusForbidden, "FORBIDDEN", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}
	if recs == nil {
		recs = []db.ListCareersRecruitersByEmployerRow{}
	}
	JSON(w, http.StatusOK, recs)
}

func (h *CareersHandler) AddRecruiter(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	empID := chi.URLParam(r, "id")
	var body struct {
		TargetUserID string `json:"user_id"`
		Role         string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid request body", nil)
		return
	}

	rec, err := h.careersService.AddRecruiter(r.Context(), user, empID, body.TargetUserID, body.Role)
	if err != nil {
		if errors.Is(err, service.ErrUnauthorizedRecruiterAction) {
			ErrorJSON(w, http.StatusForbidden, "FORBIDDEN", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}
	JSON(w, http.StatusCreated, rec)
}

func (h *CareersHandler) CreateJob(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	empID := chi.URLParam(r, "id")
	var input service.CreateJobInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid request body", nil)
		return
	}

	if input.Title == "" || input.Category == "" || input.EmploymentType == "" {
		ErrorJSON(w, http.StatusBadRequest, "BAD_REQUEST", "Job title, category, and employment type are required", nil)
		return
	}

	job, err := h.careersService.CreateJob(r.Context(), user, empID, input)
	if err != nil {
		if errors.Is(err, service.ErrUnauthorizedRecruiterAction) {
			ErrorJSON(w, http.StatusForbidden, "FORBIDDEN", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}
	JSON(w, http.StatusCreated, job)
}

func (h *CareersHandler) UpdateJob(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	jobID := chi.URLParam(r, "jobId")
	var input service.CreateJobInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid request body", nil)
		return
	}

	job, err := h.careersService.UpdateJob(r.Context(), user, jobID, input)
	if err != nil {
		if errors.Is(err, service.ErrUnauthorizedRecruiterAction) {
			ErrorJSON(w, http.StatusForbidden, "FORBIDDEN", err.Error(), nil)
			return
		}
		if errors.Is(err, service.ErrJobNotFound) {
			ErrorJSON(w, http.StatusNotFound, "NOT_FOUND", "Job vacancy not found", nil)
			return
		}
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}
	JSON(w, http.StatusOK, job)
}

func (h *CareersHandler) UpdateJobStatus(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	jobID := chi.URLParam(r, "jobId")
	var body struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid request body", nil)
		return
	}

	job, err := h.careersService.UpdateJobStatus(r.Context(), user, jobID, body.Status)
	if err != nil {
		if errors.Is(err, service.ErrEmployerNotVerified) {
			ErrorJSON(w, http.StatusBadRequest, "EMPLOYER_NOT_VERIFIED", err.Error(), nil)
			return
		}
		if errors.Is(err, service.ErrUnauthorizedRecruiterAction) {
			ErrorJSON(w, http.StatusForbidden, "FORBIDDEN", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}
	JSON(w, http.StatusOK, job)
}

func (h *CareersHandler) ListEmployerJobs(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	empID := chi.URLParam(r, "id")
	status := r.URL.Query().Get("status")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	jobs, err := h.careersService.ListEmployerJobs(r.Context(), user, empID, status, int32(limit), int32(offset))
	if err != nil {
		if errors.Is(err, service.ErrUnauthorizedRecruiterAction) {
			ErrorJSON(w, http.StatusForbidden, "FORBIDDEN", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}
	if jobs == nil {
		jobs = []db.ListCareersJobsByEmployerRow{}
	}
	JSON(w, http.StatusOK, jobs)
}

func (h *CareersHandler) ListJobApplications(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	jobID := chi.URLParam(r, "jobId")
	status := r.URL.Query().Get("status")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	apps, err := h.careersService.ListJobApplications(r.Context(), user, jobID, status, int32(limit), int32(offset))
	if err != nil {
		if errors.Is(err, service.ErrUnauthorizedRecruiterAction) {
			ErrorJSON(w, http.StatusForbidden, "FORBIDDEN", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}
	if apps == nil {
		apps = []db.ListCareersApplicationsByJobRow{}
	}
	JSON(w, http.StatusOK, apps)
}

func (h *CareersHandler) GetApplicantProfile(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	appID := chi.URLParam(r, "appId")
	candidateID := r.URL.Query().Get("candidate_id")

	profile, err := h.careersService.GetCandidateProfileForRecruiter(r.Context(), user, candidateID, appID)
	if err != nil {
		if errors.Is(err, service.ErrUnauthorizedRecruiterAction) {
			ErrorJSON(w, http.StatusForbidden, "FORBIDDEN", err.Error(), nil)
			return
		}
		if errors.Is(err, service.ErrCandidateProfileNotFound) {
			ErrorJSON(w, http.StatusNotFound, "NOT_FOUND", "Candidate profile not found", nil)
			return
		}
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}
	JSON(w, http.StatusOK, profile)
}

func (h *CareersHandler) UpdateApplicationStatus(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	appID := chi.URLParam(r, "appId")
	var body struct {
		Status string `json:"status"`
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid request body", nil)
		return
	}

	app, err := h.careersService.UpdateApplicationStatus(r.Context(), user, appID, body.Status, body.Reason)
	if err != nil {
		if errors.Is(err, service.ErrInvalidApplicationStatus) {
			ErrorJSON(w, http.StatusBadRequest, "INVALID_TRANSITION", err.Error(), nil)
			return
		}
		if errors.Is(err, service.ErrUnauthorizedRecruiterAction) {
			ErrorJSON(w, http.StatusForbidden, "FORBIDDEN", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}
	JSON(w, http.StatusOK, app)
}

func (h *CareersHandler) ScheduleInterview(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	appID := chi.URLParam(r, "appId")
	var input service.ScheduleInterviewInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid request body", nil)
		return
	}

	interview, err := h.careersService.ScheduleInterview(r.Context(), user, appID, input)
	if err != nil {
		if errors.Is(err, service.ErrUnauthorizedRecruiterAction) {
			ErrorJSON(w, http.StatusForbidden, "FORBIDDEN", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}
	JSON(w, http.StatusCreated, interview)
}

func (h *CareersHandler) ListEmployerInterviews(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	empID := chi.URLParam(r, "id")
	interviews, err := h.careersService.ListInterviewsForEmployer(r.Context(), user, empID)
	if err != nil {
		if errors.Is(err, service.ErrUnauthorizedRecruiterAction) {
			ErrorJSON(w, http.StatusForbidden, "FORBIDDEN", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}
	if interviews == nil {
		interviews = []db.ListCareersInterviewsByEmployerRow{}
	}
	JSON(w, http.StatusOK, interviews)
}

// -------------------------------------------------------------
// Admin Endpoints (/admin/careers)
// -------------------------------------------------------------

func (h *CareersHandler) GetAdminOverview(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	overview, err := h.careersService.GetAdminOverview(r.Context(), user)
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}
	JSON(w, http.StatusOK, overview)
}

func (h *CareersHandler) ListEmployersAdmin(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))

	employers, err := h.careersService.ListEmployersAdmin(r.Context(), user, q.Get("status"), q.Get("industry"), q.Get("state_id"), q.Get("search"), int32(limit), int32(offset))
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}
	if employers == nil {
		employers = []db.ListCareersEmployersRow{}
	}
	JSON(w, http.StatusOK, employers)
}

func (h *CareersHandler) VerifyEmployer(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	empID := chi.URLParam(r, "id")
	var body struct {
		Approved bool   `json:"approved"`
		Reason   string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid request body", nil)
		return
	}

	emp, err := h.careersService.VerifyEmployer(r.Context(), user, empID, body.Approved, body.Reason)
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}
	JSON(w, http.StatusOK, emp)
}

func (h *CareersHandler) ListJobsAdmin(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))

	jobs, err := h.careersService.ListJobsAdmin(r.Context(), user, q.Get("status"), q.Get("category"), q.Get("employer_id"), q.Get("state_id"), q.Get("search"), int32(limit), int32(offset))
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}
	if jobs == nil {
		jobs = []db.ListAllCareersJobsAdminRow{}
	}
	JSON(w, http.StatusOK, jobs)
}

func (h *CareersHandler) ModerateJob(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	jobID := chi.URLParam(r, "id")
	var body struct {
		Status string `json:"status"`
		Notes  string `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid request body", nil)
		return
	}

	job, err := h.careersService.ModerateJob(r.Context(), user, jobID, body.Status, body.Notes)
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}
	JSON(w, http.StatusOK, job)
}

func (h *CareersHandler) ListApplicationsAdmin(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))

	apps, err := h.careersService.ListApplicationsAdmin(r.Context(), user, q.Get("status"), q.Get("employer_id"), q.Get("job_id"), int32(limit), int32(offset))
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}
	if apps == nil {
		apps = []db.ListCareersApplicationsAdminRow{}
	}
	JSON(w, http.StatusOK, apps)
}
