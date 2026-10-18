-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied

-- 1. Employer Organizations
CREATE TABLE IF NOT EXISTS careers_employers (
    id VARCHAR(36) PRIMARY KEY,
    owner_user_id VARCHAR(36) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    organization_name VARCHAR(255) NOT NULL,
    slug VARCHAR(255) UNIQUE NOT NULL,
    registration_number VARCHAR(100) NOT NULL,
    industry VARCHAR(100) NOT NULL,
    company_size VARCHAR(50) NOT NULL DEFAULT '1-10',
    description TEXT NOT NULL DEFAULT '',
    website_url TEXT DEFAULT '',
    contact_email VARCHAR(255) NOT NULL,
    contact_phone VARCHAR(50) NOT NULL,
    logo_url TEXT,
    state_id VARCHAR(36) NOT NULL REFERENCES states(id),
    lga_id VARCHAR(36) REFERENCES lgas(id) ON DELETE SET NULL,
    address TEXT NOT NULL DEFAULT '',
    verification_status VARCHAR(50) NOT NULL DEFAULT 'pending' CHECK (verification_status IN ('pending', 'verified', 'rejected', 'suspended')),
    verified_by_user_id VARCHAR(36) REFERENCES users(id) ON DELETE SET NULL,
    verified_at TIMESTAMPTZ,
    rejection_reason TEXT DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    version INT NOT NULL DEFAULT 1
);
CREATE INDEX IF NOT EXISTS idx_careers_employers_owner ON careers_employers(owner_user_id);
CREATE INDEX IF NOT EXISTS idx_careers_employers_status ON careers_employers(verification_status);
CREATE INDEX IF NOT EXISTS idx_careers_employers_slug ON careers_employers(slug);
CREATE INDEX IF NOT EXISTS idx_careers_employers_state ON careers_employers(state_id);

-- 2. Authorized Recruiters (Linked to Employers)
CREATE TABLE IF NOT EXISTS careers_recruiters (
    id VARCHAR(36) PRIMARY KEY,
    employer_id VARCHAR(36) NOT NULL REFERENCES careers_employers(id) ON DELETE CASCADE,
    user_id VARCHAR(36) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role VARCHAR(50) NOT NULL DEFAULT 'recruiter' CHECK (role IN ('admin', 'recruiter', 'hiring_manager')),
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_careers_recruiter UNIQUE (employer_id, user_id)
);
CREATE INDEX IF NOT EXISTS idx_careers_recruiters_emp ON careers_recruiters(employer_id);
CREATE INDEX IF NOT EXISTS idx_careers_recruiters_user ON careers_recruiters(user_id);

-- 3. Job Vacancies and Opportunities
CREATE TABLE IF NOT EXISTS careers_jobs (
    id VARCHAR(36) PRIMARY KEY,
    employer_id VARCHAR(36) NOT NULL REFERENCES careers_employers(id) ON DELETE CASCADE,
    title VARCHAR(255) NOT NULL,
    slug VARCHAR(255) UNIQUE NOT NULL,
    category VARCHAR(100) NOT NULL,
    employment_type VARCHAR(50) NOT NULL CHECK (employment_type IN ('full_time', 'part_time', 'contract', 'internship', 'volunteer')),
    workplace_type VARCHAR(50) NOT NULL DEFAULT 'on_site' CHECK (workplace_type IN ('on_site', 'remote', 'hybrid')),
    state_id VARCHAR(36) REFERENCES states(id) ON DELETE SET NULL,
    lga_id VARCHAR(36) REFERENCES lgas(id) ON DELETE SET NULL,
    location_summary VARCHAR(255) NOT NULL DEFAULT '',
    salary_min_kobo BIGINT CHECK (salary_min_kobo IS NULL OR salary_min_kobo >= 0),
    salary_max_kobo BIGINT CHECK (salary_max_kobo IS NULL OR salary_max_kobo >= salary_min_kobo),
    currency VARCHAR(10) NOT NULL DEFAULT 'NGN',
    is_salary_negotiable BOOLEAN NOT NULL DEFAULT FALSE,
    description TEXT NOT NULL,
    responsibilities TEXT NOT NULL DEFAULT '',
    requirements TEXT NOT NULL DEFAULT '',
    skills_required JSONB NOT NULL DEFAULT '[]'::jsonb,
    experience_level VARCHAR(50) NOT NULL DEFAULT 'mid' CHECK (experience_level IN ('entry', 'mid', 'senior', 'executive', 'no_experience')),
    deadline_at TIMESTAMPTZ,
    status VARCHAR(50) NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'pending_approval', 'published', 'paused', 'closed', 'expired', 'rejected')),
    moderated_by_user_id VARCHAR(36) REFERENCES users(id) ON DELETE SET NULL,
    moderated_at TIMESTAMPTZ,
    moderation_notes TEXT DEFAULT '',
    views_count INT NOT NULL DEFAULT 0,
    applications_count INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    version INT NOT NULL DEFAULT 1
);
CREATE INDEX IF NOT EXISTS idx_careers_jobs_employer ON careers_jobs(employer_id);
CREATE INDEX IF NOT EXISTS idx_careers_jobs_status ON careers_jobs(status);
CREATE INDEX IF NOT EXISTS idx_careers_jobs_category ON careers_jobs(category);
CREATE INDEX IF NOT EXISTS idx_careers_jobs_state ON careers_jobs(state_id);
CREATE INDEX IF NOT EXISTS idx_careers_jobs_type ON careers_jobs(employment_type);
CREATE INDEX IF NOT EXISTS idx_careers_jobs_workplace ON careers_jobs(workplace_type);
CREATE INDEX IF NOT EXISTS idx_careers_jobs_deadline ON careers_jobs(deadline_at);
CREATE INDEX IF NOT EXISTS idx_careers_jobs_slug ON careers_jobs(slug);

-- 4. Candidate Professional Profiles (Private to Member)
CREATE TABLE IF NOT EXISTS careers_candidate_profiles (
    id VARCHAR(36) PRIMARY KEY,
    user_id VARCHAR(36) UNIQUE NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    professional_headline VARCHAR(255) NOT NULL DEFAULT '',
    summary TEXT NOT NULL DEFAULT '',
    skills JSONB NOT NULL DEFAULT '[]'::jsonb,
    education JSONB NOT NULL DEFAULT '[]'::jsonb,
    experience JSONB NOT NULL DEFAULT '[]'::jsonb,
    certifications JSONB NOT NULL DEFAULT '[]'::jsonb,
    preferred_categories JSONB NOT NULL DEFAULT '[]'::jsonb,
    preferred_workplace_type VARCHAR(50) NOT NULL DEFAULT 'any' CHECK (preferred_workplace_type IN ('any', 'on_site', 'remote', 'hybrid')),
    preferred_state_id VARCHAR(36) REFERENCES states(id) ON DELETE SET NULL,
    resume_url TEXT NOT NULL DEFAULT '',
    resume_filename VARCHAR(255) NOT NULL DEFAULT '',
    resume_uploaded_at TIMESTAMPTZ,
    is_searchable_by_verified_employers BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    version INT NOT NULL DEFAULT 1
);
CREATE INDEX IF NOT EXISTS idx_careers_candidate_profiles_user ON careers_candidate_profiles(user_id);

-- 5. Job Applications
CREATE TABLE IF NOT EXISTS careers_applications (
    id VARCHAR(36) PRIMARY KEY,
    job_id VARCHAR(36) NOT NULL REFERENCES careers_jobs(id) ON DELETE CASCADE,
    candidate_user_id VARCHAR(36) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    employer_id VARCHAR(36) NOT NULL REFERENCES careers_employers(id) ON DELETE CASCADE,
    cover_letter TEXT NOT NULL DEFAULT '',
    qualifications_summary TEXT NOT NULL DEFAULT '',
    resume_url TEXT NOT NULL,
    resume_filename VARCHAR(255) NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'submitted' CHECK (status IN ('submitted', 'under_review', 'shortlisted', 'interview', 'offered', 'hired', 'rejected', 'withdrawn')),
    rejection_reason TEXT NOT NULL DEFAULT '',
    withdrawal_reason TEXT NOT NULL DEFAULT '',
    status_updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    status_updated_by_user_id VARCHAR(36) REFERENCES users(id) ON DELETE SET NULL,
    match_score INT NOT NULL DEFAULT 0 CHECK (match_score >= 0 AND match_score <= 100),
    match_factors JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    version INT NOT NULL DEFAULT 1,
    CONSTRAINT uq_careers_job_candidate UNIQUE (job_id, candidate_user_id)
);
CREATE INDEX IF NOT EXISTS idx_careers_applications_job ON careers_applications(job_id);
CREATE INDEX IF NOT EXISTS idx_careers_applications_candidate ON careers_applications(candidate_user_id);
CREATE INDEX IF NOT EXISTS idx_careers_applications_employer ON careers_applications(employer_id);
CREATE INDEX IF NOT EXISTS idx_careers_applications_status ON careers_applications(status);

-- 6. Application Status History (Immutable Audit Trail)
CREATE TABLE IF NOT EXISTS careers_application_history (
    id VARCHAR(36) PRIMARY KEY,
    application_id VARCHAR(36) NOT NULL REFERENCES careers_applications(id) ON DELETE CASCADE,
    from_status VARCHAR(50) NOT NULL,
    to_status VARCHAR(50) NOT NULL,
    changed_by_user_id VARCHAR(36) REFERENCES users(id) ON DELETE SET NULL,
    notes TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_careers_app_history_app ON careers_application_history(application_id);

-- 7. Interview Schedules
CREATE TABLE IF NOT EXISTS careers_interviews (
    id VARCHAR(36) PRIMARY KEY,
    application_id VARCHAR(36) NOT NULL REFERENCES careers_applications(id) ON DELETE CASCADE,
    job_id VARCHAR(36) NOT NULL REFERENCES careers_jobs(id) ON DELETE CASCADE,
    employer_id VARCHAR(36) NOT NULL REFERENCES careers_employers(id) ON DELETE CASCADE,
    candidate_user_id VARCHAR(36) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    scheduled_time TIMESTAMPTZ NOT NULL,
    format VARCHAR(50) NOT NULL DEFAULT 'video' CHECK (format IN ('video', 'in_person', 'phone')),
    location_or_link TEXT NOT NULL,
    instructions TEXT NOT NULL DEFAULT '',
    status VARCHAR(50) NOT NULL DEFAULT 'scheduled' CHECK (status IN ('scheduled', 'completed', 'cancelled', 'rescheduled')),
    created_by_user_id VARCHAR(36) REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_careers_interviews_app ON careers_interviews(application_id);
CREATE INDEX IF NOT EXISTS idx_careers_interviews_candidate ON careers_interviews(candidate_user_id);
CREATE INDEX IF NOT EXISTS idx_careers_interviews_job ON careers_interviews(job_id);
CREATE INDEX IF NOT EXISTS idx_careers_interviews_time ON careers_interviews(scheduled_time);

-- +goose Down
-- SQL in section 'Down' is executed when this migration is rolled back

DROP TABLE IF EXISTS careers_interviews;
DROP TABLE IF EXISTS careers_application_history;
DROP TABLE IF EXISTS careers_applications;
DROP TABLE IF EXISTS careers_candidate_profiles;
DROP TABLE IF EXISTS careers_jobs;
DROP TABLE IF EXISTS careers_recruiters;
DROP TABLE IF EXISTS careers_employers;
