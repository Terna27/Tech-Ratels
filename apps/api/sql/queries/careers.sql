-- -------------------------------------------------------------
-- Careers Employers
-- -------------------------------------------------------------

-- name: CreateCareersEmployer :one
INSERT INTO careers_employers (
    id, owner_user_id, organization_name, slug, registration_number,
    industry, company_size, description, website_url, contact_email,
    contact_phone, logo_url, state_id, lga_id, address,
    verification_status, verified_by_user_id, verified_at, rejection_reason,
    created_at, updated_at, version
) VALUES (
    $1, $2, $3, $4, $5,
    $6, $7, $8, $9, $10,
    $11, $12, $13, $14, $15,
    $16, $17, $18, $19,
    CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 1
) RETURNING *;

-- name: GetCareersEmployerByID :one
SELECT e.*,
       u.display_name as owner_name,
       u.email as owner_email,
       s.name as state_name,
       l.name as lga_name,
       v.display_name as verified_by_name
FROM careers_employers e
JOIN users u ON e.owner_user_id = u.id
JOIN states s ON e.state_id = s.id
LEFT JOIN lgas l ON e.lga_id = l.id
LEFT JOIN users v ON e.verified_by_user_id = v.id
WHERE e.id = $1;

-- name: GetCareersEmployerBySlug :one
SELECT e.*,
       u.display_name as owner_name,
       u.email as owner_email,
       s.name as state_name,
       l.name as lga_name
FROM careers_employers e
JOIN users u ON e.owner_user_id = u.id
JOIN states s ON e.state_id = s.id
LEFT JOIN lgas l ON e.lga_id = l.id
WHERE e.slug = $1;

-- name: GetCareersEmployerByOwner :one
SELECT * FROM careers_employers
WHERE owner_user_id = $1
LIMIT 1;

-- name: UpdateCareersEmployer :one
UPDATE careers_employers
SET organization_name = $2,
    industry = $3,
    company_size = $4,
    description = $5,
    website_url = $6,
    contact_email = $7,
    contact_phone = $8,
    logo_url = $9,
    state_id = $10,
    lga_id = $11,
    address = $12,
    updated_at = CURRENT_TIMESTAMP,
    version = version + 1
WHERE id = $1
RETURNING *;

-- name: UpdateCareersEmployerVerification :one
UPDATE careers_employers
SET verification_status = $2,
    verified_by_user_id = $3,
    verified_at = $4,
    rejection_reason = $5,
    updated_at = CURRENT_TIMESTAMP,
    version = version + 1
WHERE id = $1
RETURNING *;

-- name: ListCareersEmployers :many
SELECT e.*,
       u.display_name as owner_name,
       u.email as owner_email,
       s.name as state_name,
       COUNT(DISTINCT j.id) as total_jobs
FROM careers_employers e
JOIN users u ON e.owner_user_id = u.id
JOIN states s ON e.state_id = s.id
LEFT JOIN careers_jobs j ON j.employer_id = e.id
WHERE ($1::text IS NULL OR $1 = '' OR e.verification_status = $1)
  AND ($2::text IS NULL OR $2 = '' OR e.industry = $2)
  AND ($3::text IS NULL OR $3 = '' OR e.state_id = $3)
  AND ($4::text IS NULL OR $4 = '' OR e.organization_name ILIKE '%' || $4 || '%')
GROUP BY e.id, u.display_name, u.email, s.name
ORDER BY e.created_at DESC
LIMIT $5 OFFSET $6;

-- -------------------------------------------------------------
-- Recruiters
-- -------------------------------------------------------------

-- name: AddCareersRecruiter :one
INSERT INTO careers_recruiters (
    id, employer_id, user_id, role, is_active, created_at, updated_at
) VALUES (
    $1, $2, $3, $4, $5, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
) RETURNING *;

-- name: GetCareersRecruiter :one
SELECT r.*,
       u.display_name,
       u.email,
       u.phone
FROM careers_recruiters r
JOIN users u ON r.user_id = u.id
WHERE r.employer_id = $1 AND r.user_id = $2;

-- name: ListCareersRecruitersByEmployer :many
SELECT r.*,
       u.display_name,
       u.email,
       u.phone
FROM careers_recruiters r
JOIN users u ON r.user_id = u.id
WHERE r.employer_id = $1
ORDER BY r.created_at ASC;

-- name: DeleteCareersRecruiter :exec
DELETE FROM careers_recruiters
WHERE employer_id = $1 AND user_id = $2;

-- -------------------------------------------------------------
-- Jobs
-- -------------------------------------------------------------

-- name: CreateCareersJob :one
INSERT INTO careers_jobs (
    id, employer_id, title, slug, category, employment_type,
    workplace_type, state_id, lga_id, location_summary,
    salary_min_kobo, salary_max_kobo, currency, is_salary_negotiable,
    description, responsibilities, requirements, skills_required,
    experience_level, deadline_at, status, created_at, updated_at, version
) VALUES (
    $1, $2, $3, $4, $5, $6,
    $7, $8, $9, $10,
    $11, $12, $13, $14,
    $15, $16, $17, $18,
    $19, $20, $21, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 1
) RETURNING *;

-- name: GetCareersJobByID :one
SELECT j.*,
       e.organization_name as employer_name,
       e.slug as employer_slug,
       e.logo_url as employer_logo,
       e.verification_status as employer_verification_status,
       e.industry as employer_industry,
       e.website_url as employer_website,
       s.name as state_name,
       l.name as lga_name
FROM careers_jobs j
JOIN careers_employers e ON j.employer_id = e.id
LEFT JOIN states s ON j.state_id = s.id
LEFT JOIN lgas l ON j.lga_id = l.id
WHERE j.id = $1;

-- name: GetCareersJobBySlug :one
SELECT j.*,
       e.organization_name as employer_name,
       e.slug as employer_slug,
       e.logo_url as employer_logo,
       e.verification_status as employer_verification_status,
       e.industry as employer_industry,
       e.website_url as employer_website,
       s.name as state_name,
       l.name as lga_name
FROM careers_jobs j
JOIN careers_employers e ON j.employer_id = e.id
LEFT JOIN states s ON j.state_id = s.id
LEFT JOIN lgas l ON j.lga_id = l.id
WHERE j.slug = $1;

-- name: UpdateCareersJob :one
UPDATE careers_jobs
SET title = $2,
    category = $3,
    employment_type = $4,
    workplace_type = $5,
    state_id = $6,
    lga_id = $7,
    location_summary = $8,
    salary_min_kobo = $9,
    salary_max_kobo = $10,
    is_salary_negotiable = $11,
    description = $12,
    responsibilities = $13,
    requirements = $14,
    skills_required = $15,
    experience_level = $16,
    deadline_at = $17,
    updated_at = CURRENT_TIMESTAMP,
    version = version + 1
WHERE id = $1
RETURNING *;

-- name: UpdateCareersJobStatus :one
UPDATE careers_jobs
SET status = $2,
    moderated_by_user_id = $3,
    moderated_at = $4,
    moderation_notes = $5,
    updated_at = CURRENT_TIMESTAMP,
    version = version + 1
WHERE id = $1
RETURNING *;

-- name: IncrementCareersJobViews :exec
UPDATE careers_jobs
SET views_count = views_count + 1
WHERE id = $1;

-- name: IncrementCareersJobApplications :exec
UPDATE careers_jobs
SET applications_count = applications_count + 1
WHERE id = $1;

-- name: DecrementCareersJobApplications :exec
UPDATE careers_jobs
SET applications_count = GREATEST(0, applications_count - 1)
WHERE id = $1;

-- name: ListPublicCareersJobs :many
SELECT j.*,
       e.organization_name as employer_name,
       e.slug as employer_slug,
       e.logo_url as employer_logo,
       e.verification_status as employer_verification_status,
       s.name as state_name,
       l.name as lga_name
FROM careers_jobs j
JOIN careers_employers e ON j.employer_id = e.id
LEFT JOIN states s ON j.state_id = s.id
LEFT JOIN lgas l ON j.lga_id = l.id
WHERE j.status = 'published'
  AND (j.deadline_at IS NULL OR j.deadline_at > CURRENT_TIMESTAMP)
  AND ($1::text IS NULL OR $1 = '' OR j.category = $1)
  AND ($2::text IS NULL OR $2 = '' OR j.employment_type = $2)
  AND ($3::text IS NULL OR $3 = '' OR j.workplace_type = $3)
  AND ($4::text IS NULL OR $4 = '' OR j.state_id = $4)
  AND ($5::text IS NULL OR $5 = '' OR j.experience_level = $5)
  AND ($6::text IS NULL OR $6 = '' OR (
      j.title ILIKE '%' || $6 || '%' OR
      j.description ILIKE '%' || $6 || '%' OR
      e.organization_name ILIKE '%' || $6 || '%'
  ))
ORDER BY j.created_at DESC
LIMIT $7 OFFSET $8;

-- name: ListCareersJobsByEmployer :many
SELECT j.*,
       s.name as state_name,
       l.name as lga_name
FROM careers_jobs j
LEFT JOIN states s ON j.state_id = s.id
LEFT JOIN lgas l ON j.lga_id = l.id
WHERE j.employer_id = $1
  AND ($2::text IS NULL OR $2 = '' OR j.status = $2)
ORDER BY j.created_at DESC
LIMIT $3 OFFSET $4;

-- name: ListAllCareersJobsAdmin :many
SELECT j.*,
       e.organization_name as employer_name,
       e.slug as employer_slug,
       s.name as state_name
FROM careers_jobs j
JOIN careers_employers e ON j.employer_id = e.id
LEFT JOIN states s ON j.state_id = s.id
WHERE ($1::text IS NULL OR $1 = '' OR j.status = $1)
  AND ($2::text IS NULL OR $2 = '' OR j.category = $2)
  AND ($3::text IS NULL OR $3 = '' OR j.employer_id = $3)
  AND ($4::text IS NULL OR $4 = '' OR j.state_id = $4)
  AND ($5::text IS NULL OR $5 = '' OR (
      j.title ILIKE '%' || $5 || '%' OR
      e.organization_name ILIKE '%' || $5 || '%'
  ))
ORDER BY j.created_at DESC
LIMIT $6 OFFSET $7;

-- -------------------------------------------------------------
-- Candidate Professional Profiles
-- -------------------------------------------------------------

-- name: UpsertCandidateProfile :one
INSERT INTO careers_candidate_profiles (
    id, user_id, professional_headline, summary, skills,
    education, experience, certifications, preferred_categories,
    preferred_workplace_type, preferred_state_id,
    resume_url, resume_filename, resume_uploaded_at,
    is_searchable_by_verified_employers,
    created_at, updated_at, version
) VALUES (
    $1, $2, $3, $4, $5,
    $6, $7, $8, $9,
    $10, $11,
    $12, $13, $14,
    $15,
    CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 1
)
ON CONFLICT (user_id) DO UPDATE SET
    professional_headline = EXCLUDED.professional_headline,
    summary = EXCLUDED.summary,
    skills = EXCLUDED.skills,
    education = EXCLUDED.education,
    experience = EXCLUDED.experience,
    certifications = EXCLUDED.certifications,
    preferred_categories = EXCLUDED.preferred_categories,
    preferred_workplace_type = EXCLUDED.preferred_workplace_type,
    preferred_state_id = EXCLUDED.preferred_state_id,
    resume_url = CASE WHEN EXCLUDED.resume_url <> '' THEN EXCLUDED.resume_url ELSE careers_candidate_profiles.resume_url END,
    resume_filename = CASE WHEN EXCLUDED.resume_filename <> '' THEN EXCLUDED.resume_filename ELSE careers_candidate_profiles.resume_filename END,
    resume_uploaded_at = CASE WHEN EXCLUDED.resume_url <> '' THEN CURRENT_TIMESTAMP ELSE careers_candidate_profiles.resume_uploaded_at END,
    is_searchable_by_verified_employers = EXCLUDED.is_searchable_by_verified_employers,
    updated_at = CURRENT_TIMESTAMP,
    version = careers_candidate_profiles.version + 1
RETURNING *;

-- name: GetCandidateProfileByUserID :one
SELECT p.*,
       u.display_name,
       u.email,
       u.phone,
       u.membership_reference,
       s.name as preferred_state_name
FROM careers_candidate_profiles p
JOIN users u ON p.user_id = u.id
LEFT JOIN states s ON p.preferred_state_id = s.id
WHERE p.user_id = $1;

-- -------------------------------------------------------------
-- Job Applications
-- -------------------------------------------------------------

-- name: CreateCareersApplication :one
INSERT INTO careers_applications (
    id, job_id, candidate_user_id, employer_id, cover_letter,
    qualifications_summary, resume_url, resume_filename,
    status, match_score, match_factors,
    created_at, updated_at, version
) VALUES (
    $1, $2, $3, $4, $5,
    $6, $7, $8,
    'submitted', $9, $10,
    CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 1
) RETURNING *;

-- name: GetCareersApplicationByID :one
SELECT a.*,
       j.title as job_title,
       j.category as job_category,
       j.employment_type as job_employment_type,
       j.workplace_type as job_workplace_type,
       e.organization_name as employer_name,
       e.slug as employer_slug,
       e.logo_url as employer_logo,
       u.display_name as candidate_name,
       u.email as candidate_email,
       u.phone as candidate_phone,
       u.membership_reference as candidate_membership_reference
FROM careers_applications a
JOIN careers_jobs j ON a.job_id = j.id
JOIN careers_employers e ON a.employer_id = e.id
JOIN users u ON a.candidate_user_id = u.id
WHERE a.id = $1;

-- name: GetCareersApplicationByJobAndCandidate :one
SELECT * FROM careers_applications
WHERE job_id = $1 AND candidate_user_id = $2;

-- name: ListCareersApplicationsByCandidate :many
SELECT a.*,
       j.title as job_title,
       j.category as job_category,
       j.employment_type as job_employment_type,
       j.workplace_type as job_workplace_type,
       j.status as job_status,
       e.organization_name as employer_name,
       e.slug as employer_slug,
       e.logo_url as employer_logo
FROM careers_applications a
JOIN careers_jobs j ON a.job_id = j.id
JOIN careers_employers e ON a.employer_id = e.id
WHERE a.candidate_user_id = $1
ORDER BY a.created_at DESC;

-- name: ListCareersApplicationsByJob :many
SELECT a.*,
       u.display_name as candidate_name,
       u.email as candidate_email,
       u.phone as candidate_phone,
       u.membership_reference as candidate_membership_reference
FROM careers_applications a
JOIN users u ON a.candidate_user_id = u.id
WHERE a.job_id = $1
  AND ($2::text IS NULL OR $2 = '' OR a.status = $2)
ORDER BY a.match_score DESC, a.created_at DESC
LIMIT $3 OFFSET $4;

-- name: ListCareersApplicationsByEmployer :many
SELECT a.*,
       j.title as job_title,
       u.display_name as candidate_name,
       u.email as candidate_email,
       u.phone as candidate_phone
FROM careers_applications a
JOIN careers_jobs j ON a.job_id = j.id
JOIN users u ON a.candidate_user_id = u.id
WHERE a.employer_id = $1
  AND ($2::text IS NULL OR $2 = '' OR a.status = $2)
  AND ($3::text IS NULL OR $3 = '' OR a.job_id = $3)
ORDER BY a.created_at DESC
LIMIT $4 OFFSET $5;

-- name: ListCareersApplicationsAdmin :many
SELECT a.id,
       a.job_id,
       a.employer_id,
       a.status,
       a.match_score,
       a.created_at,
       a.status_updated_at,
       j.title as job_title,
       e.organization_name as employer_name,
       u.display_name as candidate_name
FROM careers_applications a
JOIN careers_jobs j ON a.job_id = j.id
JOIN careers_employers e ON a.employer_id = e.id
JOIN users u ON a.candidate_user_id = u.id
WHERE ($1::text IS NULL OR $1 = '' OR a.status = $1)
  AND ($2::text IS NULL OR $2 = '' OR a.employer_id = $2)
  AND ($3::text IS NULL OR $3 = '' OR a.job_id = $3)
ORDER BY a.created_at DESC
LIMIT $4 OFFSET $5;

-- name: UpdateCareersApplicationStatus :one
UPDATE careers_applications
SET status = $2,
    rejection_reason = $3,
    withdrawal_reason = $4,
    status_updated_at = CURRENT_TIMESTAMP,
    status_updated_by_user_id = $5,
    updated_at = CURRENT_TIMESTAMP,
    version = version + 1
WHERE id = $1
RETURNING *;

-- -------------------------------------------------------------
-- Application History
-- -------------------------------------------------------------

-- name: CreateCareersApplicationHistory :one
INSERT INTO careers_application_history (
    id, application_id, from_status, to_status,
    changed_by_user_id, notes, created_at
) VALUES (
    $1, $2, $3, $4,
    $5, $6, CURRENT_TIMESTAMP
) RETURNING *;

-- name: ListCareersApplicationHistory :many
SELECT h.*,
       u.display_name as changed_by_name
FROM careers_application_history h
LEFT JOIN users u ON h.changed_by_user_id = u.id
WHERE h.application_id = $1
ORDER BY h.created_at ASC;

-- -------------------------------------------------------------
-- Interviews
-- -------------------------------------------------------------

-- name: CreateCareersInterview :one
INSERT INTO careers_interviews (
    id, application_id, job_id, employer_id, candidate_user_id,
    scheduled_time, format, location_or_link, instructions,
    status, created_by_user_id, created_at, updated_at
) VALUES (
    $1, $2, $3, $4, $5,
    $6, $7, $8, $9,
    'scheduled', $10, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
) RETURNING *;

-- name: GetCareersInterviewByID :one
SELECT i.*,
       j.title as job_title,
       e.organization_name as employer_name,
       u.display_name as candidate_name,
       u.email as candidate_email
FROM careers_interviews i
JOIN careers_jobs j ON i.job_id = j.id
JOIN careers_employers e ON i.employer_id = e.id
JOIN users u ON i.candidate_user_id = u.id
WHERE i.id = $1;

-- name: ListCareersInterviewsByApplication :many
SELECT i.*,
       u.display_name as candidate_name
FROM careers_interviews i
JOIN users u ON i.candidate_user_id = u.id
WHERE i.application_id = $1
ORDER BY i.scheduled_time ASC;

-- name: ListCareersInterviewsByCandidate :many
SELECT i.*,
       j.title as job_title,
       e.organization_name as employer_name,
       e.slug as employer_slug
FROM careers_interviews i
JOIN careers_jobs j ON i.job_id = j.id
JOIN careers_employers e ON i.employer_id = e.id
WHERE i.candidate_user_id = $1
ORDER BY i.scheduled_time ASC;

-- name: ListCareersInterviewsByEmployer :many
SELECT i.*,
       j.title as job_title,
       u.display_name as candidate_name,
       u.email as candidate_email,
       u.phone as candidate_phone
FROM careers_interviews i
JOIN careers_jobs j ON i.job_id = j.id
JOIN users u ON i.candidate_user_id = u.id
WHERE i.employer_id = $1
ORDER BY i.scheduled_time ASC;

-- name: UpdateCareersInterviewStatus :one
UPDATE careers_interviews
SET status = $2,
    instructions = CASE WHEN $3::text IS NOT NULL AND $3 <> '' THEN $3 ELSE instructions END,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING *;

-- -------------------------------------------------------------
-- Overview & Metrics
-- -------------------------------------------------------------

-- name: GetCareersAdminOverview :one
SELECT 
    COUNT(DISTINCT e.id) as total_employers,
    COUNT(DISTINCT CASE WHEN e.verification_status = 'verified' THEN e.id END) as verified_employers,
    COUNT(DISTINCT CASE WHEN e.verification_status = 'pending' THEN e.id END) as pending_employers,
    COUNT(DISTINCT j.id) as total_jobs,
    COUNT(DISTINCT CASE WHEN j.status = 'published' THEN j.id END) as active_jobs,
    COUNT(DISTINCT CASE WHEN j.status = 'pending_approval' THEN j.id END) as pending_approval_jobs,
    COUNT(DISTINCT a.id) as total_applications,
    COUNT(DISTINCT CASE WHEN a.status = 'hired' THEN a.id END) as total_hires
FROM careers_employers e
FULL OUTER JOIN careers_jobs j ON true
FULL OUTER JOIN careers_applications a ON true;

-- name: GetCareersEmployerOverview :one
SELECT 
    COUNT(DISTINCT j.id) as total_jobs,
    COUNT(DISTINCT CASE WHEN j.status = 'published' THEN j.id END) as active_jobs,
    COUNT(DISTINCT a.id) as total_applications,
    COUNT(DISTINCT CASE WHEN a.status IN ('submitted', 'under_review') THEN a.id END) as pending_applications,
    COUNT(DISTINCT CASE WHEN a.status = 'shortlisted' THEN a.id END) as shortlisted_applications,
    COUNT(DISTINCT CASE WHEN a.status = 'interview' THEN a.id END) as interview_applications,
    COUNT(DISTINCT CASE WHEN a.status = 'hired' THEN a.id END) as hired_applications
FROM careers_jobs j
LEFT JOIN careers_applications a ON a.job_id = j.id
WHERE j.employer_id = $1;
