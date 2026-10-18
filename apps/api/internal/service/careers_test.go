package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"ratels-api/internal/service"
)

func TestCareersService_EmployerRegistrationAndVerification(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	state, err := env.Repo.Queries().GetStateByCode(ctx, "LA")
	if err != nil {
		t.Fatalf("Failed to fetch state: %v", err)
	}

	employerUser := createRegisteredUser(t, env, "emp_owner_1", "member")
	adminUser := createRegisteredUser(t, env, "careers_adm_1", "super_admin")

	// 1. Register Employer Organization
	emp, err := env.CareersService.RegisterEmployer(ctx, employerUser, service.RegisterEmployerInput{
		OrganizationName:   "Green Cities Waste Solutions Ltd",
		RegistrationNumber: "RC-998877",
		Industry:           "Sanitation & Recycling",
		CompanySize:        "11-50",
		Description:        "Urban sanitation engineering and recycling enterprise",
		ContactEmail:       "careers@greencities.example.org",
		ContactPhone:       "+2348011223344",
		StateID:            state.ID,
		Address:            "12 Marina Road, Lagos Island",
	})
	if err != nil {
		t.Fatalf("Failed to register employer: %v", err)
	}
	if emp.VerificationStatus != "pending" {
		t.Errorf("Expected verification_status 'pending', got '%s'", emp.VerificationStatus)
	}

	// 2. Reject Duplicate Employer by Same User
	_, err = env.CareersService.RegisterEmployer(ctx, employerUser, service.RegisterEmployerInput{
		OrganizationName:   "Another Firm",
		RegistrationNumber: "RC-111111",
		Industry:           "Consulting",
		ContactEmail:       "dup@example.org",
		ContactPhone:       "+2348000000000",
		StateID:            state.ID,
	})
	if !errors.Is(err, service.ErrEmployerAlreadyExists) {
		t.Fatalf("Expected ErrEmployerAlreadyExists, got %v", err)
	}

	// 3. Admin Verifies Employer
	verifiedEmp, err := env.CareersService.VerifyEmployer(ctx, adminUser, emp.ID, true, "CAC documents verified")
	if err != nil {
		t.Fatalf("Admin verification failed: %v", err)
	}
	if verifiedEmp.VerificationStatus != "verified" {
		t.Errorf("Expected 'verified', got '%s'", verifiedEmp.VerificationStatus)
	}
}

func TestCareersService_JobCreationAndRecruiterIsolation(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	state, err := env.Repo.Queries().GetStateByCode(ctx, "FC")
	if err != nil {
		t.Fatalf("Failed to fetch state: %v", err)
	}

	empOwner := createRegisteredUser(t, env, "emp_owner_2", "member")
	recruiter := createRegisteredUser(t, env, "emp_recruiter_2", "member")
	unauthorizedUser := createRegisteredUser(t, env, "emp_outsider", "member")
	adminUser := createRegisteredUser(t, env, "careers_adm_2", "super_admin")

	emp, err := env.CareersService.RegisterEmployer(ctx, empOwner, service.RegisterEmployerInput{
		OrganizationName:   "Abuja Environmental Tech",
		RegistrationNumber: "RC-445566",
		Industry:           "Environmental Engineering",
		CompanySize:        "51-200",
		ContactEmail:       "info@abujaenviro.example.org",
		ContactPhone:       "+2348099887766",
		StateID:            state.ID,
		Address:            "Plot 50 Central Business District, Abuja",
	})
	if err != nil {
		t.Fatalf("RegisterEmployer failed: %v", err)
	}

	// Verify employer so jobs can be published directly
	_, err = env.CareersService.VerifyEmployer(ctx, adminUser, emp.ID, true, "Verified")
	if err != nil {
		t.Fatalf("VerifyEmployer failed: %v", err)
	}

	// Add recruiter
	_, err = env.CareersService.AddRecruiter(ctx, empOwner, emp.ID, recruiter.ID, "recruiter")
	if err != nil {
		t.Fatalf("AddRecruiter failed: %v", err)
	}

	// Recruiter creates published job
	deadline := time.Now().AddDate(0, 1, 0)
	minSal := int64(15000000)
	maxSal := int64(22000000)
	job, err := env.CareersService.CreateJob(ctx, recruiter, emp.ID, service.CreateJobInput{
		Title:              "Recycling Plant Operations Lead",
		Category:           "Sanitation & Recycling",
		EmploymentType:     "full_time",
		WorkplaceType:      "on_site",
		StateID:            &state.ID,
		LocationSummary:    "Abuja Idu Industrial Area",
		SalaryMinKobo:      &minSal,
		SalaryMaxKobo:      &maxSal,
		IsSalaryNegotiable: true,
		Description:        "Lead daily recycling facility sorting, sorting lines, and material logistics.",
		Responsibilities:   "Supervise shift workers, ensure safety protocols, maintain sorting quotas.",
		Requirements:       "HND/B.Sc, 3+ years in industrial facility operations.",
		SkillsRequired:     []string{"Plant Management", "Material Sorting", "HSE Compliance"},
		ExperienceLevel:    "mid",
		DeadlineAt:         &deadline,
		AsDraft:            false,
	})
	if err != nil {
		t.Fatalf("CreateJob failed: %v", err)
	}
	if job.Status != "published" {
		t.Errorf("Expected job status 'published', got '%s'", job.Status)
	}

	// Unauthorized user cannot modify job
	_, err = env.CareersService.UpdateJob(ctx, unauthorizedUser, job.ID, service.CreateJobInput{
		Title: "Hacked Job Title",
	})
	if !errors.Is(err, service.ErrUnauthorizedRecruiterAction) {
		t.Fatalf("Expected ErrUnauthorizedRecruiterAction, got %v", err)
	}

	// Unauthorized user cannot toggle job status
	_, err = env.CareersService.UpdateJobStatus(ctx, unauthorizedUser, job.ID, "paused")
	if !errors.Is(err, service.ErrUnauthorizedRecruiterAction) {
		t.Fatalf("Expected ErrUnauthorizedRecruiterAction on status toggle, got %v", err)
	}

	// Recruiter can pause job
	pausedJob, err := env.CareersService.UpdateJobStatus(ctx, recruiter, job.ID, "paused")
	if err != nil {
		t.Fatalf("Recruiter failed to pause job: %v", err)
	}
	if pausedJob.Status != "paused" {
		t.Errorf("Expected 'paused', got '%s'", pausedJob.Status)
	}
}

func TestCareersService_CandidateProfilePrivacyAndAccess(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	state, err := env.Repo.Queries().GetStateByCode(ctx, "LA")
	if err != nil {
		t.Fatalf("Failed to fetch state: %v", err)
	}

	candidate := createRegisteredUser(t, env, "cand_priv_1", "member")
	intruder := createRegisteredUser(t, env, "intruder_user", "member")

	// 1. Candidate upserts profile
	profile, err := env.CareersService.UpsertCandidateProfile(ctx, candidate, service.CandidateProfileInput{
		ProfessionalHeadline:   "Senior Environmental Sanitation Specialist",
		Summary:                "Experienced community organizer and certified HSE waste management specialist.",
		Skills:                 []string{"Waste Sorting", "First Aid", "HSE Compliance", "Project Leadership"},
		PreferredCategories:    []string{"Sanitation & Recycling", "Environmental & Sanitation"},
		PreferredWorkplaceType: "hybrid",
		PreferredStateID:       &state.ID,
		ResumeURL:              "/resumes/cand_priv_1_resume.pdf",
		ResumeFilename:         "resume_lead.pdf",
	})
	if err != nil {
		t.Fatalf("UpsertCandidateProfile failed: %v", err)
	}
	if profile.ProfessionalHeadline != "Senior Environmental Sanitation Specialist" {
		t.Errorf("Headline mismatch: %s", profile.ProfessionalHeadline)
	}

	// 2. Candidate can fetch own profile
	myProf, err := env.CareersService.GetMyCandidateProfile(ctx, candidate)
	if err != nil {
		t.Fatalf("GetMyCandidateProfile failed: %v", err)
	}
	if myProf.UserID != candidate.ID {
		t.Errorf("User ID mismatch: %s vs %s", myProf.UserID, candidate.ID)
	}

	// 3. Intruder cannot access candidate profile without an application context
	_, err = env.CareersService.GetCandidateProfileForRecruiter(ctx, intruder, candidate.ID, "fake-app-id")
	if err == nil {
		t.Fatal("Expected error when intruder attempts to view candidate profile, got nil")
	}
}

func TestCareersService_ApplicationLifecycleAndExplainableMatching(t *testing.T) {
	env := setupTestEnv(t)
	defer env.Pool.Close()
	ctx := context.Background()

	state, err := env.Repo.Queries().GetStateByCode(ctx, "FC")
	if err != nil {
		t.Fatalf("Failed to fetch state: %v", err)
	}

	empOwner := createRegisteredUser(t, env, "emp_owner_3", "member")
	adminUser := createRegisteredUser(t, env, "careers_adm_3", "super_admin")
	candidate := createRegisteredUser(t, env, "cand_app_1", "member")

	emp, _ := env.CareersService.RegisterEmployer(ctx, empOwner, service.RegisterEmployerInput{
		OrganizationName:   "Capital Waste Management",
		RegistrationNumber: "RC-778899",
		Industry:           "Environmental & Sanitation",
		CompanySize:        "201-500",
		ContactEmail:       "hr@capitalwaste.example.org",
		ContactPhone:       "+2348033221100",
		StateID:            state.ID,
		Address:            "Plot 120 Garki, Abuja",
	})
	_, _ = env.CareersService.VerifyEmployer(ctx, adminUser, emp.ID, true, "Verified")

	// Candidate profile with matching skills
	_, _ = env.CareersService.UpsertCandidateProfile(ctx, candidate, service.CandidateProfileInput{
		ProfessionalHeadline:   "Certified HSE Waste Coordinator",
		Summary:                "Specialist in urban cleanup logistics and safety.",
		Skills:                 []string{"Waste Sorting", "HSE Compliance", "Hazardous Waste", "First Aid"},
		PreferredCategories:    []string{"Environmental & Sanitation"},
		PreferredWorkplaceType: "on_site",
		PreferredStateID:       &state.ID,
		ResumeURL:              "/resumes/cand_app_1.pdf",
		ResumeFilename:         "cand_app_1.pdf",
	})

	// Create published job with deadline
	futureDeadline := time.Now().AddDate(0, 2, 0)
	job, err := env.CareersService.CreateJob(ctx, empOwner, emp.ID, service.CreateJobInput{
		Title:           "Field Hazardous Waste Supervisor",
		Category:        "Environmental & Sanitation",
		EmploymentType:  "full_time",
		WorkplaceType:   "on_site",
		StateID:         &state.ID,
		LocationSummary: "Abuja Central",
		Description:     "Oversee safe collection, sorting, and treatment of waste.",
		SkillsRequired:  []string{"Waste Sorting", "HSE Compliance", "Hazardous Waste"},
		ExperienceLevel: "mid",
		DeadlineAt:      &futureDeadline,
		AsDraft:         false,
	})
	if err != nil {
		t.Fatalf("CreateJob failed: %v", err)
	}

	// 1. Submit Application
	app, err := env.CareersService.ApplyForJob(ctx, candidate, job.ID, service.SubmitApplicationInput{
		CoverLetter:           "I have 5 years experience in hazardous waste compliance in Abuja.",
		QualificationsSummary: "B.Sc Microbiology, NEBOSH HSE Level 3.",
		ResumeURL:             "/resumes/cand_app_1.pdf",
		ResumeFilename:        "cand_app_1.pdf",
	})
	if err != nil {
		t.Fatalf("ApplyForJob failed: %v", err)
	}
	if app.Status != "submitted" {
		t.Errorf("Expected initial status 'submitted', got '%s'", app.Status)
	}
	if app.MatchScore < 70 {
		t.Errorf("Expected high explainable match score due to exact skill & location overlap, got %d", app.MatchScore)
	}

	// 2. Reject Duplicate Application
	_, err = env.CareersService.ApplyForJob(ctx, candidate, job.ID, service.SubmitApplicationInput{
		CoverLetter: "Duplicate submission",
	})
	if !errors.Is(err, service.ErrDuplicateApplication) {
		t.Fatalf("Expected ErrDuplicateApplication, got %v", err)
	}

	// 3. Status Transition: submitted -> under_review
	updatedApp, err := env.CareersService.UpdateApplicationStatus(ctx, empOwner, app.ID, "under_review", "Initial resume screening complete")
	if err != nil {
		t.Fatalf("Update to under_review failed: %v", err)
	}
	if updatedApp.Status != "under_review" {
		t.Errorf("Expected 'under_review', got '%s'", updatedApp.Status)
	}

	// 4. Status Transition: under_review -> shortlisted
	shortlistedApp, err := env.CareersService.UpdateApplicationStatus(ctx, empOwner, app.ID, "shortlisted", "Strong qualifications")
	if err != nil {
		t.Fatalf("Update to shortlisted failed: %v", err)
	}
	if shortlistedApp.Status != "shortlisted" {
		t.Errorf("Expected 'shortlisted', got '%s'", shortlistedApp.Status)
	}

	// 5. Schedule Interview (automatically advances to interview status)
	interviewTime := time.Now().AddDate(0, 0, 5)
	interview, err := env.CareersService.ScheduleInterview(ctx, empOwner, app.ID, service.ScheduleInterviewInput{
		ScheduledTime:  interviewTime,
		Format:         "video",
		LocationOrLink: "https://meet.google.com/abc-defg-hij",
		Instructions:   "Please prepare a 5-minute presentation on spill response.",
	})
	if err != nil {
		t.Fatalf("ScheduleInterview failed: %v", err)
	}
	if interview.Format != "video" {
		t.Errorf("Expected video format, got %s", interview.Format)
	}

	// 6. Reject Invalid Transition (e.g. Cannot jump from shortlisted directly to hired without offer)
	_, err = env.CareersService.UpdateApplicationStatus(ctx, empOwner, app.ID, "hired", "Invalid jump")
	if !errors.Is(err, service.ErrInvalidApplicationStatus) {
		t.Fatalf("Expected ErrInvalidApplicationStatus for illegal leap, got %v", err)
	}

	// 7. Offer & Hire
	_, err = env.CareersService.UpdateApplicationStatus(ctx, empOwner, app.ID, "offered", "Formal job offer dispatched")
	if err != nil {
		t.Fatalf("Update to offered failed: %v", err)
	}

	hiredApp, err := env.CareersService.UpdateApplicationStatus(ctx, empOwner, app.ID, "hired", "Candidate accepted offer")
	if err != nil {
		t.Fatalf("Update to hired failed: %v", err)
	}
	if hiredApp.Status != "hired" {
		t.Errorf("Expected 'hired', got '%s'", hiredApp.Status)
	}

	// 8. Verify Application History Audit Trail
	history, err := env.CareersService.GetApplicationHistory(ctx, candidate, app.ID)
	if err != nil {
		t.Fatalf("GetApplicationHistory failed: %v", err)
	}
	if len(history) < 4 {
		t.Errorf("Expected at least 4 history audit entries, got %d", len(history))
	}
}
