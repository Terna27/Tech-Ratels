package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"ratels-api/internal/db"
	"ratels-api/internal/service"
)

func TestCareersHTTP_PublicJobsAndDiscovery(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()

	// 1. GET /api/v1/careers/jobs
	req, _ := http.NewRequest("GET", "/api/v1/careers/jobs", nil)
	rr := httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK from public jobs list, got %d: %s", rr.Code, rr.Body.String())
	}

	var jobs []db.ListPublicCareersJobsRow
	if err := json.Unmarshal(rr.Body.Bytes(), &jobs); err != nil {
		t.Fatalf("Failed to decode jobs response: %v", err)
	}
}

func TestCareersHTTP_CandidateProfileAndApplicationFlow(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()
	ctx := context.Background()

	state, err := h.Repo.Queries().GetStateByCode(ctx, "FC")
	if err != nil {
		t.Fatalf("Failed to fetch state: %v", err)
	}

	empOwner, empToken := createTestUserWithToken(t, h, "http_emp_owner", "Employer Owner", "member")
	candidate, candToken := createTestUserWithToken(t, h, "http_candidate", "Job Candidate", "member")
	admin, adminToken := createTestUserWithToken(t, h, "http_admin", "Super Admin", "super_admin")

	// 1. Register Employer
	regBody, _ := json.Marshal(map[string]interface{}{
		"organization_name":   "Civic Environmental Innovations",
		"registration_number": "RC-554433",
		"industry":           "Sanitation & Recycling",
		"company_size":        "11-50",
		"description":        "Civic technology and environmental management",
		"contact_email":       "careers@civicenv.example.org",
		"contact_phone":       "+2348011998877",
		"state_id":            state.ID,
		"address":            "Plot 10 Abuja Tech District",
	})
	req, _ := http.NewRequest("POST", "/api/v1/careers/employer/register", bytes.NewBuffer(regBody))
	req.Header.Set("Authorization", "Bearer "+empToken)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("Expected 201 Created from employer registration, got %d: %s", rr.Code, rr.Body.String())
	}
	var emp db.CareersEmployer
	_ = json.Unmarshal(rr.Body.Bytes(), &emp)

	// 2. Admin Verifies Employer
	verifyBody, _ := json.Marshal(map[string]interface{}{
		"approved": true,
		"reason":   "CAC documents verified via CAC portal",
	})
	req, _ = http.NewRequest("POST", "/api/v1/careers/admin/employers/"+emp.ID+"/verify", bytes.NewBuffer(verifyBody))
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK from employer verification, got %d: %s", rr.Code, rr.Body.String())
	}

	// 3. Employer Creates Published Job
	futureDeadline := time.Now().AddDate(0, 2, 0)
	minSal := int64(18000000)
	maxSal := int64(25000000)
	jobBody, _ := json.Marshal(service.CreateJobInput{
		Title:              "Community Cleanup Logistics Coordinator",
		Category:           "Sanitation & Recycling",
		EmploymentType:     "full_time",
		WorkplaceType:      "on_site",
		StateID:            &state.ID,
		LocationSummary:    "Abuja Municipal Area",
		SalaryMinKobo:      &minSal,
		SalaryMaxKobo:      &maxSal,
		IsSalaryNegotiable: true,
		Description:        "Coordinate volunteer squads, cleanup supplies, and truck dispatch.",
		Responsibilities:   "Manage trucks, supply manifests, and volunteer field safety.",
		Requirements:       "HND or B.Sc with 2 years logistics experience.",
		SkillsRequired:     []string{"Logistics", "Waste Sorting", "First Aid"},
		ExperienceLevel:    "mid",
		DeadlineAt:         &futureDeadline,
		AsDraft:            false,
	})
	req, _ = http.NewRequest("POST", "/api/v1/careers/employer/"+emp.ID+"/jobs", bytes.NewBuffer(jobBody))
	req.Header.Set("Authorization", "Bearer "+empToken)
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("Expected 201 Created from job creation, got %d: %s", rr.Code, rr.Body.String())
	}
	var job db.CareersJob
	_ = json.Unmarshal(rr.Body.Bytes(), &job)

	// 4. Candidate Updates Professional Profile
	profBody, _ := json.Marshal(service.CandidateProfileInput{
		ProfessionalHeadline:   "Logistics & Operations Specialist",
		Summary:                "Experienced supply chain coordinator with field cleanup background.",
		Skills:                 []string{"Logistics", "Waste Sorting", "First Aid", "Inventory Control"},
		PreferredCategories:    []string{"Sanitation & Recycling"},
		PreferredWorkplaceType: "on_site",
		PreferredStateID:       &state.ID,
		ResumeURL:              "/resumes/sample_cv.pdf",
		ResumeFilename:         "sample_cv.pdf",
	})
	req, _ = http.NewRequest("PUT", "/api/v1/careers/candidate/profile", bytes.NewBuffer(profBody))
	req.Header.Set("Authorization", "Bearer "+candToken)
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK from profile update, got %d: %s", rr.Code, rr.Body.String())
	}

	// 5. Candidate Applies for Job
	appBody, _ := json.Marshal(service.SubmitApplicationInput{
		CoverLetter:           "I have extensive experience in logistics and cleanup dispatch in Abuja.",
		QualificationsSummary: "B.Sc Logistics, 3 years operational field experience.",
		ResumeURL:             "/resumes/sample_cv.pdf",
		ResumeFilename:        "sample_cv.pdf",
	})
	req, _ = http.NewRequest("POST", "/api/v1/careers/jobs/"+job.ID+"/apply", bytes.NewBuffer(appBody))
	req.Header.Set("Authorization", "Bearer "+candToken)
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("Expected 201 Created from job application, got %d: %s", rr.Code, rr.Body.String())
	}
	var app db.CareersApplication
	_ = json.Unmarshal(rr.Body.Bytes(), &app)
	if app.Status != "submitted" {
		t.Errorf("Expected status 'submitted', got '%s'", app.Status)
	}

	// 6. Duplicate Application is Rejected (409 Conflict)
	req, _ = http.NewRequest("POST", "/api/v1/careers/jobs/"+job.ID+"/apply", bytes.NewBuffer(appBody))
	req.Header.Set("Authorization", "Bearer "+candToken)
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusConflict {
		t.Fatalf("Expected 409 Conflict for duplicate application, got %d: %s", rr.Code, rr.Body.String())
	}

	// 7. Candidate Views Own Applications
	req, _ = http.NewRequest("GET", "/api/v1/careers/applications", nil)
	req.Header.Set("Authorization", "Bearer "+candToken)
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK from candidate applications list, got %d: %s", rr.Code, rr.Body.String())
	}
	var myApps []db.ListCareersApplicationsByCandidateRow
	_ = json.Unmarshal(rr.Body.Bytes(), &myApps)
	if len(myApps) != 1 {
		t.Errorf("Expected 1 application, got %d", len(myApps))
	}

	// 8. Employer Reviews Applicants for Job
	req, _ = http.NewRequest("GET", "/api/v1/careers/employer/jobs/"+job.ID+"/applications", nil)
	req.Header.Set("Authorization", "Bearer "+empToken)
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK from employer applicants list, got %d: %s", rr.Code, rr.Body.String())
	}
	var jobApps []db.ListCareersApplicationsByJobRow
	_ = json.Unmarshal(rr.Body.Bytes(), &jobApps)
	if len(jobApps) != 1 {
		t.Errorf("Expected 1 job applicant, got %d", len(jobApps))
	}

	// 9. Recruiter from Another Employer CANNOT Access These Applicants (Isolation)
	otherOwner, otherToken := createTestUserWithToken(t, h, "other_emp_owner", "Other Owner", "member")
	_ = otherOwner
	req, _ = http.NewRequest("GET", "/api/v1/careers/employer/jobs/"+job.ID+"/applications", nil)
	req.Header.Set("Authorization", "Bearer "+otherToken)
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("Expected 403 Forbidden for unauthorized recruiter, got %d: %s", rr.Code, rr.Body.String())
	}

	// 10. Employer Advances Candidate: submitted -> under_review -> shortlisted
	advanceBody, _ := json.Marshal(map[string]interface{}{
		"status": "shortlisted",
		"reason": "Strong background matches all requirements",
	})
	req, _ = http.NewRequest("PATCH", "/api/v1/careers/employer/applications/"+app.ID+"/status", bytes.NewBuffer(advanceBody))
	req.Header.Set("Authorization", "Bearer "+empToken)
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK advancing to shortlisted, got %d: %s", rr.Code, rr.Body.String())
	}

	// 11. Employer Schedules Interview
	interviewTime := time.Now().AddDate(0, 0, 7)
	intBody, _ := json.Marshal(service.ScheduleInterviewInput{
		ScheduledTime:  interviewTime,
		Format:         "video",
		LocationOrLink: "https://meet.google.com/ratels-careers-interview",
		Instructions:   "Technical interview on field cleanup operations",
	})
	req, _ = http.NewRequest("POST", "/api/v1/careers/employer/applications/"+app.ID+"/interviews", bytes.NewBuffer(intBody))
	req.Header.Set("Authorization", "Bearer "+empToken)
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("Expected 201 Created from interview scheduling, got %d: %s", rr.Code, rr.Body.String())
	}

	// 12. Candidate Lists Scheduled Interviews
	req, _ = http.NewRequest("GET", "/api/v1/careers/interviews", nil)
	req.Header.Set("Authorization", "Bearer "+candToken)
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK from candidate interviews list, got %d: %s", rr.Code, rr.Body.String())
	}

	// 13. Admin Overview Check (/api/v1/careers/admin/overview)
	req, _ = http.NewRequest("GET", "/api/v1/careers/admin/overview", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK from admin careers overview, got %d: %s", rr.Code, rr.Body.String())
	}

	// Non-admin receives 403 on admin overview
	req, _ = http.NewRequest("GET", "/api/v1/careers/admin/overview", nil)
	req.Header.Set("Authorization", "Bearer "+candToken)
	rr = httptest.NewRecorder()
	h.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("Expected 403 Forbidden for non-admin on admin overview, got %d: %s", rr.Code, rr.Body.String())
	}
	_ = candidate
	_ = empOwner
	_ = admin
}
