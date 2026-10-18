package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"ratels-api/internal/service"
	"ratels-api/internal/storage"
)

type CommunityProjectHandler struct {
	projectService *service.CommunityProjectService
}

func NewCommunityProjectHandler(projectService *service.CommunityProjectService) *CommunityProjectHandler {
	return &CommunityProjectHandler{projectService: projectService}
}

// -------------------------------------------------------------
// Helper to extract actor details
// -------------------------------------------------------------

func getActorDetails(r *http.Request) (string, string, *string) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		return "", "", nil
	}
	actorRole := user.PrimaryRole
	var actorJurisdictionID *string

	apps := GetCurrentAppointments(r.Context())
	for _, a := range apps {
		if a.Status == "active" {
			actorRole = a.RoleType
			if a.JurisdictionID.Valid {
				jid := a.JurisdictionID.String
				actorJurisdictionID = &jid
			}
			break
		}
	}
	return user.ID, actorRole, actorJurisdictionID
}

// -------------------------------------------------------------
// Public Endpoints
// -------------------------------------------------------------

func (h *CommunityProjectHandler) GetCategories(w http.ResponseWriter, r *http.Request) {
	type CategoryItem struct {
		Key   string `json:"key"`
		Label string `json:"label"`
	}
	var items []CategoryItem
	for k, v := range service.ValidCommunityCategories {
		items = append(items, CategoryItem{Key: k, Label: v})
	}
	JSON(w, http.StatusOK, map[string]interface{}{
		"categories": items,
	})
}

func (h *CommunityProjectHandler) GetOverviewStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.projectService.GetOverviewStats(r.Context())
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}
	JSON(w, http.StatusOK, stats)
}

func (h *CommunityProjectHandler) ListPublicProjects(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	limit, _ := strconv.Atoi(q.Get("limit"))

	var stateID *string
	if s := q.Get("state_id"); s != "" {
		stateID = &s
	}
	var lgaID *string
	if l := q.Get("lga_id"); l != "" {
		lgaID = &l
	}
	var category *string
	if c := q.Get("category"); c != "" {
		category = &c
	}
	var status *string
	if st := q.Get("execution_status"); st != "" {
		status = &st
	}

	filter := service.PublicProjectFilter{
		StateID:         stateID,
		LGAID:           lgaID,
		Category:        category,
		ExecutionStatus: status,
		Page:            int32(page),
		Limit:           int32(limit),
	}

	items, total, err := h.projectService.ListPublicProjects(r.Context(), filter)
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, map[string]interface{}{
		"projects": items,
		"total":    total,
		"page":     page,
		"limit":    limit,
	})
}

func (h *CommunityProjectHandler) GetPublicProject(w http.ResponseWriter, r *http.Request) {
	idOrRef := chi.URLParam(r, "id")
	if idOrRef == "" {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_ID", "Project ID or reference required", nil)
		return
	}

	res, err := h.projectService.GetPublicProject(r.Context(), idOrRef)
	if err != nil {
		if err == service.ErrCommunityProjectNotFound {
			ErrorJSON(w, http.StatusNotFound, "NOT_FOUND", "Community project not found", nil)
			return
		}
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, res)
}

// -------------------------------------------------------------
// Member Community Request Endpoints
// -------------------------------------------------------------

func (h *CommunityProjectHandler) CreateCommunityRequest(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	var req service.CreateCommunityRequestInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body", nil)
		return
	}

	created, err := h.projectService.CreateRequest(r.Context(), user.ID, req)
	if err != nil {
		ErrorJSON(w, http.StatusBadRequest, "CREATE_REQUEST_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusCreated, created)
}

func (h *CommunityProjectHandler) ListMyCommunityRequests(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	items, err := h.projectService.ListMemberRequests(r.Context(), user.ID)
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, map[string]interface{}{
		"requests": items,
	})
}

func (h *CommunityProjectHandler) GetCommunityRequest(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	requestID := chi.URLParam(r, "id")
	if requestID == "" {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_ID", "Request ID required", nil)
		return
	}

	actorID, actorRole, actorJurisdictionID := getActorDetails(r)
	req, err := h.projectService.GetRequestByID(r.Context(), actorID, actorRole, actorJurisdictionID, requestID)
	if err != nil {
		if err == service.ErrCommunityRequestNotFound {
			ErrorJSON(w, http.StatusNotFound, "NOT_FOUND", "Request not found", nil)
			return
		}
		if err == service.ErrScopeUnauthorized {
			ErrorJSON(w, http.StatusForbidden, "SCOPE_UNAUTHORIZED", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, req)
}

func (h *CommunityProjectHandler) ResubmitCommunityRequest(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	requestID := chi.URLParam(r, "id")
	if requestID == "" {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_ID", "Request ID required", nil)
		return
	}

	var req service.ResubmitRequestInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body", nil)
		return
	}

	resubmitted, err := h.projectService.ResubmitRequest(r.Context(), user.ID, requestID, req)
	if err != nil {
		ErrorJSON(w, http.StatusBadRequest, "RESUBMIT_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, resubmitted)
}

func (h *CommunityProjectHandler) PresignRequestUpload(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	var req struct {
		Filename    string `json:"filename"`
		ContentType string `json:"content_type"`
		FileSize    int64  `json:"file_size"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body", nil)
		return
	}

	res, err := h.projectService.AuthorizeRequestImageUpload(r.Context(), user.ID, req.Filename, req.ContentType, req.FileSize)
	if err != nil {
		ErrorJSON(w, http.StatusBadRequest, "PRESIGN_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, res)
}

func (h *CommunityProjectHandler) UploadRequestImageDirect(w http.ResponseWriter, r *http.Request) {
	user := GetCurrentUser(r.Context())
	if user == nil {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	err := r.ParseMultipartForm(storage.MaxProductImageSize)
	if err != nil {
		ErrorJSON(w, http.StatusBadRequest, "PARSE_ERROR", "Failed to parse multipart form or image exceeds 5 MB", nil)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		ErrorJSON(w, http.StatusBadRequest, "FILE_REQUIRED", "A valid image file under 'file' key is required", nil)
		return
	}
	defer file.Close()

	requestID := r.FormValue("request_id")
	objectKey, publicURL, err := h.projectService.UploadRequestImageDirect(r.Context(), user.ID, requestID, header.Filename, file, header.Size)
	if err != nil {
		ErrorJSON(w, http.StatusBadRequest, "UPLOAD_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusCreated, map[string]string{
		"object_key": objectKey,
		"public_url": publicURL,
	})
}

// -------------------------------------------------------------
// Admin & Coordinator Endpoints
// -------------------------------------------------------------

func (h *CommunityProjectHandler) ListAdminRequests(w http.ResponseWriter, r *http.Request) {
	actorID, actorRole, actorJurisdictionID := getActorDetails(r)
	if actorID == "" {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	limit, _ := strconv.Atoi(q.Get("limit"))

	var stateID *string
	if s := q.Get("state_id"); s != "" {
		stateID = &s
	}
	var lgaID *string
	if l := q.Get("lga_id"); l != "" {
		lgaID = &l
	}
	var category *string
	if c := q.Get("category"); c != "" {
		category = &c
	}
	var status *string
	if st := q.Get("status"); st != "" {
		status = &st
	}
	var urgency *string
	if u := q.Get("urgency"); u != "" {
		urgency = &u
	}

	filter := service.AdminRequestFilter{
		StateID:  stateID,
		LGAID:    lgaID,
		Category: category,
		Status:   status,
		Urgency:  urgency,
		Page:     int32(page),
		Limit:    int32(limit),
	}

	items, total, err := h.projectService.ListAdminRequests(r.Context(), actorID, actorRole, actorJurisdictionID, filter)
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, map[string]interface{}{
		"requests": items,
		"total":    total,
		"page":     page,
		"limit":    limit,
	})
}

func (h *CommunityProjectHandler) ReviewCommunityRequest(w http.ResponseWriter, r *http.Request) {
	actorID, actorRole, actorJurisdictionID := getActorDetails(r)
	if actorID == "" {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	requestID := chi.URLParam(r, "id")
	if requestID == "" {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_ID", "Request ID required", nil)
		return
	}

	var req service.ReviewRequestInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body", nil)
		return
	}

	reviewed, err := h.projectService.ReviewRequest(r.Context(), actorID, actorRole, actorJurisdictionID, requestID, req)
	if err != nil {
		if err == service.ErrSelfApprovalNotAllowed {
			ErrorJSON(w, http.StatusForbidden, "SELF_APPROVAL_FORBIDDEN", err.Error(), nil)
			return
		}
		if err == service.ErrScopeUnauthorized {
			ErrorJSON(w, http.StatusForbidden, "SCOPE_UNAUTHORIZED", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusBadRequest, "REVIEW_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, reviewed)
}

func (h *CommunityProjectHandler) CreateCommunityProject(w http.ResponseWriter, r *http.Request) {
	actorID, actorRole, actorJurisdictionID := getActorDetails(r)
	if actorID == "" {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	var req service.CreateProjectInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body", nil)
		return
	}

	project, err := h.projectService.CreateProject(r.Context(), actorID, actorRole, actorJurisdictionID, req)
	if err != nil {
		if err == service.ErrDuplicateProjectConversion {
			ErrorJSON(w, http.StatusConflict, "DUPLICATE_CONVERSION", err.Error(), nil)
			return
		}
		if err == service.ErrScopeUnauthorized {
			ErrorJSON(w, http.StatusForbidden, "SCOPE_UNAUTHORIZED", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusBadRequest, "CREATE_PROJECT_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusCreated, project)
}

func (h *CommunityProjectHandler) ConvertCommunityRequest(w http.ResponseWriter, r *http.Request) {
	reqID := chi.URLParam(r, "id")
	actorID, actorRole, actorJurisdictionID := getActorDetails(r)
	if actorID == "" {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	var req service.CreateProjectInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body", nil)
		return
	}
	req.CommunityRequestID = &reqID

	project, err := h.projectService.CreateProject(r.Context(), actorID, actorRole, actorJurisdictionID, req)
	if err != nil {
		if err == service.ErrDuplicateProjectConversion {
			ErrorJSON(w, http.StatusConflict, "DUPLICATE_CONVERSION", err.Error(), nil)
			return
		}
		if err == service.ErrScopeUnauthorized {
			ErrorJSON(w, http.StatusForbidden, "SCOPE_UNAUTHORIZED", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusBadRequest, "CREATE_PROJECT_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusCreated, project)
}

func (h *CommunityProjectHandler) ListAdminProjects(w http.ResponseWriter, r *http.Request) {
	actorID, actorRole, actorJurisdictionID := getActorDetails(r)
	if actorID == "" {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	limit, _ := strconv.Atoi(q.Get("limit"))

	var stateID *string
	if s := q.Get("state_id"); s != "" {
		stateID = &s
	}
	var lgaID *string
	if l := q.Get("lga_id"); l != "" {
		lgaID = &l
	}
	var category *string
	if c := q.Get("category"); c != "" {
		category = &c
	}
	var execStatus *string
	if st := q.Get("execution_status"); st != "" {
		execStatus = &st
	}
	var fundStatus *string
	if fs := q.Get("funding_status"); fs != "" {
		fundStatus = &fs
	}

	filter := service.AdminProjectFilter{
		StateID:         stateID,
		LGAID:           lgaID,
		Category:        category,
		ExecutionStatus: execStatus,
		FundingStatus:   fundStatus,
		Page:            int32(page),
		Limit:           int32(limit),
	}

	items, total, err := h.projectService.ListAdminProjects(r.Context(), actorID, actorRole, actorJurisdictionID, filter)
	if err != nil {
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, map[string]interface{}{
		"projects": items,
		"total":    total,
		"page":     page,
		"limit":    limit,
	})
}

func (h *CommunityProjectHandler) GetAdminProject(w http.ResponseWriter, r *http.Request) {
	actorID, actorRole, actorJurisdictionID := getActorDetails(r)
	if actorID == "" {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	projectID := chi.URLParam(r, "id")
	if projectID == "" {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_ID", "Project ID required", nil)
		return
	}

	res, err := h.projectService.GetAdminProject(r.Context(), actorID, actorRole, actorJurisdictionID, projectID)
	if err != nil {
		if err == service.ErrCommunityProjectNotFound {
			ErrorJSON(w, http.StatusNotFound, "NOT_FOUND", "Community project not found", nil)
			return
		}
		if err == service.ErrScopeUnauthorized {
			ErrorJSON(w, http.StatusForbidden, "SCOPE_UNAUTHORIZED", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, res)
}

func (h *CommunityProjectHandler) UpdateProjectDetails(w http.ResponseWriter, r *http.Request) {
	actorID, actorRole, actorJurisdictionID := getActorDetails(r)
	if actorID == "" {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	projectID := chi.URLParam(r, "id")
	if projectID == "" {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_ID", "Project ID required", nil)
		return
	}

	var req service.UpdateProjectDetailsInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body", nil)
		return
	}

	updated, err := h.projectService.UpdateProjectDetails(r.Context(), actorID, actorRole, actorJurisdictionID, projectID, req)
	if err != nil {
		if err == service.ErrScopeUnauthorized {
			ErrorJSON(w, http.StatusForbidden, "SCOPE_UNAUTHORIZED", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusBadRequest, "UPDATE_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, updated)
}

func (h *CommunityProjectHandler) UpdateProjectExecution(w http.ResponseWriter, r *http.Request) {
	actorID, actorRole, actorJurisdictionID := getActorDetails(r)
	if actorID == "" {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	projectID := chi.URLParam(r, "id")
	if projectID == "" {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_ID", "Project ID required", nil)
		return
	}

	var req service.UpdateProjectExecutionInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body", nil)
		return
	}

	updated, err := h.projectService.UpdateProjectExecution(r.Context(), actorID, actorRole, actorJurisdictionID, projectID, req)
	if err != nil {
		if strings.Contains(err.Error(), "UNVERIFIED_MILESTONES") {
			ErrorJSON(w, http.StatusConflict, "UNVERIFIED_MILESTONES", err.Error(), nil)
			return
		}
		if err == service.ErrScopeUnauthorized {
			ErrorJSON(w, http.StatusForbidden, "SCOPE_UNAUTHORIZED", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusBadRequest, "EXECUTION_UPDATE_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, updated)
}

// -------------------------------------------------------------
// Milestone Handlers
// -------------------------------------------------------------

func (h *CommunityProjectHandler) AddMilestone(w http.ResponseWriter, r *http.Request) {
	actorID, actorRole, actorJurisdictionID := getActorDetails(r)
	if actorID == "" {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	projectID := chi.URLParam(r, "id")
	if projectID == "" {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_ID", "Project ID required", nil)
		return
	}

	var req service.AddMilestoneInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body", nil)
		return
	}

	created, err := h.projectService.AddMilestone(r.Context(), actorID, actorRole, actorJurisdictionID, projectID, req)
	if err != nil {
		if err == service.ErrScopeUnauthorized {
			ErrorJSON(w, http.StatusForbidden, "SCOPE_UNAUTHORIZED", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusBadRequest, "ADD_MILESTONE_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusCreated, created)
}

func (h *CommunityProjectHandler) UpdateMilestone(w http.ResponseWriter, r *http.Request) {
	actorID, actorRole, actorJurisdictionID := getActorDetails(r)
	if actorID == "" {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	milestoneID := chi.URLParam(r, "milestone_id")
	if milestoneID == "" {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_ID", "Milestone ID required", nil)
		return
	}

	var req service.UpdateMilestoneInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body", nil)
		return
	}

	updated, err := h.projectService.UpdateMilestone(r.Context(), actorID, actorRole, actorJurisdictionID, milestoneID, req)
	if err != nil {
		ErrorJSON(w, http.StatusBadRequest, "UPDATE_MILESTONE_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, updated)
}

func (h *CommunityProjectHandler) VerifyMilestone(w http.ResponseWriter, r *http.Request) {
	actorID, actorRole, actorJurisdictionID := getActorDetails(r)
	if actorID == "" {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	milestoneID := chi.URLParam(r, "milestone_id")
	if milestoneID == "" {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_ID", "Milestone ID required", nil)
		return
	}

	var req service.VerifyMilestoneInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body", nil)
		return
	}

	verified, err := h.projectService.VerifyMilestone(r.Context(), actorID, actorRole, actorJurisdictionID, milestoneID, req)
	if err != nil {
		if err == service.ErrSelfApprovalNotAllowed {
			ErrorJSON(w, http.StatusForbidden, "SELF_APPROVAL_FORBIDDEN", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusBadRequest, "VERIFY_MILESTONE_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, verified)
}

func (h *CommunityProjectHandler) DeleteMilestone(w http.ResponseWriter, r *http.Request) {
	actorID, actorRole, actorJurisdictionID := getActorDetails(r)
	if actorID == "" {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	milestoneID := chi.URLParam(r, "milestone_id")
	if milestoneID == "" {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_ID", "Milestone ID required", nil)
		return
	}

	err := h.projectService.DeleteMilestone(r.Context(), actorID, actorRole, actorJurisdictionID, milestoneID)
	if err != nil {
		ErrorJSON(w, http.StatusBadRequest, "DELETE_MILESTONE_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, map[string]string{"message": "Milestone successfully deleted"})
}

// -------------------------------------------------------------
// Financial Allocation & Expenditure Handler
// -------------------------------------------------------------

func (h *CommunityProjectHandler) RecordProjectFunding(w http.ResponseWriter, r *http.Request) {
	actorID, actorRole, _ := getActorDetails(r)
	if actorID == "" {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	projectID := chi.URLParam(r, "id")
	if projectID == "" {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_ID", "Project ID required", nil)
		return
	}

	var req service.ProjectFundingInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body", nil)
		return
	}

	updated, err := h.projectService.RecordProjectFunding(r.Context(), actorID, actorRole, projectID, req)
	if err != nil {
		if err == service.ErrFinancialUnauthorized {
			ErrorJSON(w, http.StatusForbidden, "FINANCIAL_UNAUTHORIZED", err.Error(), nil)
			return
		}
		ErrorJSON(w, http.StatusBadRequest, "RECORD_FUNDING_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, updated)
}

// -------------------------------------------------------------
// Project Updates & Upload Handlers
// -------------------------------------------------------------

func (h *CommunityProjectHandler) CreateProjectUpdate(w http.ResponseWriter, r *http.Request) {
	actorID, actorRole, _ := getActorDetails(r)
	if actorID == "" {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	projectID := chi.URLParam(r, "id")
	if projectID == "" {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_ID", "Project ID required", nil)
		return
	}

	var req service.CreateProjectUpdateInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body", nil)
		return
	}

	created, err := h.projectService.CreateProjectUpdate(r.Context(), actorID, actorRole, projectID, req)
	if err != nil {
		ErrorJSON(w, http.StatusBadRequest, "CREATE_UPDATE_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusCreated, created)
}

func (h *CommunityProjectHandler) PresignProjectUpload(w http.ResponseWriter, r *http.Request) {
	actorID, actorRole, _ := getActorDetails(r)
	if actorID == "" {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	projectID := chi.URLParam(r, "id")

	var req struct {
		Filename    string `json:"filename"`
		ContentType string `json:"content_type"`
		FileSize    int64  `json:"file_size"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body", nil)
		return
	}

	res, err := h.projectService.AuthorizeProjectImageUpload(r.Context(), actorID, actorRole, projectID, req.Filename, req.ContentType, req.FileSize)
	if err != nil {
		ErrorJSON(w, http.StatusBadRequest, "PRESIGN_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusOK, res)
}

func (h *CommunityProjectHandler) UploadProjectImageDirect(w http.ResponseWriter, r *http.Request) {
	actorID, actorRole, _ := getActorDetails(r)
	if actorID == "" {
		ErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	projectID := chi.URLParam(r, "id")

	err := r.ParseMultipartForm(storage.MaxProductImageSize)
	if err != nil {
		ErrorJSON(w, http.StatusBadRequest, "PARSE_ERROR", "Failed to parse multipart form or image exceeds 5 MB", nil)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		ErrorJSON(w, http.StatusBadRequest, "FILE_REQUIRED", "A valid image file under 'file' key is required", nil)
		return
	}
	defer file.Close()

	objectKey, publicURL, err := h.projectService.UploadProjectImageDirect(r.Context(), actorID, actorRole, projectID, header.Filename, file, header.Size)
	if err != nil {
		ErrorJSON(w, http.StatusBadRequest, "UPLOAD_FAILED", err.Error(), nil)
		return
	}

	JSON(w, http.StatusCreated, map[string]string{
		"object_key": objectKey,
		"public_url": publicURL,
	})
}
