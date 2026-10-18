-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied

-- 1. Community Development & Sanitation Requests Table
CREATE TABLE IF NOT EXISTS community_requests (
    id VARCHAR(36) PRIMARY KEY,
    reference VARCHAR(50) UNIQUE NOT NULL,
    title VARCHAR(255) NOT NULL,
    description TEXT NOT NULL,
    category VARCHAR(50) NOT NULL,
    state_id VARCHAR(36) NOT NULL REFERENCES states(id),
    lga_id VARCHAR(36) NOT NULL REFERENCES lgas(id),
    community_ward VARCHAR(255) NOT NULL,
    location_details TEXT NOT NULL,
    estimated_urgency VARCHAR(20) NOT NULL DEFAULT 'medium' CHECK (estimated_urgency IN ('low', 'medium', 'high', 'critical')),
    evidence_image_urls JSONB NOT NULL DEFAULT '[]'::jsonb,
    requester_id VARCHAR(36) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status VARCHAR(30) NOT NULL DEFAULT 'submitted' CHECK (status IN ('submitted', 'under_review', 'info_requested', 'verified', 'approved', 'rejected')),
    reviewed_by_user_id VARCHAR(36) REFERENCES users(id) ON DELETE SET NULL,
    reviewed_at TIMESTAMPTZ,
    review_notes TEXT,
    rejection_reason TEXT,
    info_request_notes TEXT,
    escalation_level VARCHAR(50) DEFAULT 'state' CHECK (escalation_level IS NULL OR escalation_level IN ('lga', 'state', 'zonal', 'national')),
    converted_to_project_id VARCHAR(36),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    version INT NOT NULL DEFAULT 1
);

CREATE INDEX IF NOT EXISTS idx_community_requests_ref ON community_requests(reference);
CREATE INDEX IF NOT EXISTS idx_community_requests_status ON community_requests(status);
CREATE INDEX IF NOT EXISTS idx_community_requests_requester ON community_requests(requester_id);
CREATE INDEX IF NOT EXISTS idx_community_requests_state ON community_requests(state_id);
CREATE INDEX IF NOT EXISTS idx_community_requests_lga ON community_requests(lga_id);
CREATE INDEX IF NOT EXISTS idx_community_requests_category ON community_requests(category);
CREATE INDEX IF NOT EXISTS idx_community_requests_created ON community_requests(created_at DESC);

-- 2. Community Managed Projects Table
CREATE TABLE IF NOT EXISTS community_projects (
    id VARCHAR(36) PRIMARY KEY,
    reference VARCHAR(50) UNIQUE NOT NULL,
    title VARCHAR(255) NOT NULL,
    description TEXT NOT NULL,
    community_request_id VARCHAR(36) UNIQUE REFERENCES community_requests(id) ON DELETE SET NULL,
    category VARCHAR(50) NOT NULL,
    state_id VARCHAR(36) NOT NULL REFERENCES states(id),
    lga_id VARCHAR(36) NOT NULL REFERENCES lgas(id),
    community_ward VARCHAR(255) NOT NULL,
    location_details TEXT NOT NULL,
    responsible_officer_id VARCHAR(36) REFERENCES users(id) ON DELETE SET NULL,
    responsible_department VARCHAR(100) NOT NULL DEFAULT 'Community Development & Sanitation',
    start_date DATE,
    target_completion_date DATE,
    actual_completion_date DATE,
    proposed_budget BIGINT NOT NULL DEFAULT 0 CHECK (proposed_budget >= 0),
    approved_budget BIGINT NOT NULL DEFAULT 0 CHECK (approved_budget >= 0),
    allocated_funding BIGINT NOT NULL DEFAULT 0 CHECK (allocated_funding >= 0),
    recorded_expenditure BIGINT NOT NULL DEFAULT 0 CHECK (recorded_expenditure >= 0),
    outstanding_commitments BIGINT NOT NULL DEFAULT 0 CHECK (outstanding_commitments >= 0),
    funding_status VARCHAR(30) NOT NULL DEFAULT 'unfunded' CHECK (funding_status IN ('unfunded', 'partially_funded', 'fully_funded')),
    execution_status VARCHAR(30) NOT NULL DEFAULT 'planned' CHECK (execution_status IN ('planned', 'in_progress', 'awaiting_verification', 'completed', 'paused', 'cancelled')),
    progress_percentage INT NOT NULL DEFAULT 0 CHECK (progress_percentage >= 0 AND progress_percentage <= 100),
    pause_or_cancel_reason TEXT,
    supporting_documents JSONB NOT NULL DEFAULT '[]'::jsonb,
    completion_evidence_urls JSONB NOT NULL DEFAULT '[]'::jsonb,
    verified_by_user_id VARCHAR(36) REFERENCES users(id) ON DELETE SET NULL,
    verified_at TIMESTAMPTZ,
    verification_notes TEXT,
    is_public BOOLEAN NOT NULL DEFAULT TRUE,
    created_by_user_id VARCHAR(36) NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    version INT NOT NULL DEFAULT 1
);

CREATE INDEX IF NOT EXISTS idx_community_projects_ref ON community_projects(reference);
CREATE INDEX IF NOT EXISTS idx_community_projects_status ON community_projects(execution_status);
CREATE INDEX IF NOT EXISTS idx_community_projects_funding ON community_projects(funding_status);
CREATE INDEX IF NOT EXISTS idx_community_projects_state ON community_projects(state_id);
CREATE INDEX IF NOT EXISTS idx_community_projects_lga ON community_projects(lga_id);
CREATE INDEX IF NOT EXISTS idx_community_projects_category ON community_projects(category);
CREATE INDEX IF NOT EXISTS idx_community_projects_is_public ON community_projects(is_public);
CREATE INDEX IF NOT EXISTS idx_community_projects_created ON community_projects(created_at DESC);

-- Add foreign key constraint back to community_requests for converted_to_project_id
ALTER TABLE community_requests
ADD CONSTRAINT fk_community_requests_project
FOREIGN KEY (converted_to_project_id) REFERENCES community_projects(id) ON DELETE SET NULL;

-- 3. Community Project Milestones Table
CREATE TABLE IF NOT EXISTS community_project_milestones (
    id VARCHAR(36) PRIMARY KEY,
    project_id VARCHAR(36) NOT NULL REFERENCES community_projects(id) ON DELETE CASCADE,
    sequence_order INT NOT NULL DEFAULT 1,
    title VARCHAR(255) NOT NULL,
    description TEXT NOT NULL,
    target_date DATE NOT NULL,
    completion_date DATE,
    assigned_person_name VARCHAR(150),
    assigned_person_id VARCHAR(36) REFERENCES users(id) ON DELETE SET NULL,
    status VARCHAR(30) NOT NULL DEFAULT 'planned' CHECK (status IN ('planned', 'in_progress', 'awaiting_verification', 'completed')),
    progress_notes TEXT,
    evidence_photo_urls JSONB NOT NULL DEFAULT '[]'::jsonb,
    verification_status VARCHAR(30) NOT NULL DEFAULT 'unverified' CHECK (verification_status IN ('unverified', 'verified', 'rejected')),
    verified_by_user_id VARCHAR(36) REFERENCES users(id) ON DELETE SET NULL,
    verified_at TIMESTAMPTZ,
    verification_notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_project_milestones_proj ON community_project_milestones(project_id, sequence_order);
CREATE INDEX IF NOT EXISTS idx_project_milestones_status ON community_project_milestones(status);
CREATE INDEX IF NOT EXISTS idx_project_milestones_ver ON community_project_milestones(verification_status);

-- 4. Community Project Updates & Progress Logs Table
CREATE TABLE IF NOT EXISTS community_project_updates (
    id VARCHAR(36) PRIMARY KEY,
    project_id VARCHAR(36) NOT NULL REFERENCES community_projects(id) ON DELETE CASCADE,
    milestone_id VARCHAR(36) REFERENCES community_project_milestones(id) ON DELETE SET NULL,
    author_id VARCHAR(36) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title VARCHAR(255) NOT NULL,
    content TEXT NOT NULL,
    photo_urls JSONB NOT NULL DEFAULT '[]'::jsonb,
    progress_percentage INT CHECK (progress_percentage IS NULL OR (progress_percentage >= 0 AND progress_percentage <= 100)),
    is_public BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_project_updates_proj ON community_project_updates(project_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_project_updates_public ON community_project_updates(is_public);

-- +goose Down
-- SQL in section 'Down' is executed when this migration is rolled back

DROP TABLE IF EXISTS community_project_updates;
DROP TABLE IF EXISTS community_project_milestones;
ALTER TABLE IF EXISTS community_requests DROP CONSTRAINT IF EXISTS fk_community_requests_project;
DROP TABLE IF EXISTS community_projects;
DROP TABLE IF EXISTS community_requests;
