-- +goose Up
-- Migration 20261019000014: Safeguarding & Confidential Incident Reporting System

-- 1. Safeguarding Officers (Designated personnel with explicit safeguarding capabilities)
CREATE TABLE IF NOT EXISTS safeguarding_officers (
    id VARCHAR(36) PRIMARY KEY,
    user_id VARCHAR(36) NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    role_title VARCHAR(100) NOT NULL,
    is_lead BOOLEAN NOT NULL DEFAULT false,
    can_triage BOOLEAN NOT NULL DEFAULT true,
    can_assign BOOLEAN NOT NULL DEFAULT false,
    can_investigate BOOLEAN NOT NULL DEFAULT true,
    can_escalate BOOLEAN NOT NULL DEFAULT true,
    can_close BOOLEAN NOT NULL DEFAULT false,
    can_view_audit BOOLEAN NOT NULL DEFAULT false,
    status VARCHAR(20) NOT NULL DEFAULT 'active', -- 'active', 'suspended', 'revoked'
    appointed_by VARCHAR(36) REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_safeguarding_officers_user ON safeguarding_officers(user_id);
CREATE INDEX IF NOT EXISTS idx_safeguarding_officers_status ON safeguarding_officers(status);

-- 2. Safeguarding Cases
CREATE TABLE IF NOT EXISTS safeguarding_cases (
    id VARCHAR(36) PRIMARY KEY,
    public_reference VARCHAR(50) NOT NULL UNIQUE,
    category VARCHAR(50) NOT NULL,
    encrypted_description TEXT NOT NULL,
    incident_date TIMESTAMPTZ,
    state_id VARCHAR(36) REFERENCES states(id),
    lga_id VARCHAR(36) REFERENCES lgas(id),
    related_activity TEXT,
    person_involved TEXT,
    is_immediate_danger BOOLEAN NOT NULL DEFAULT false,
    is_minor_involved BOOLEAN NOT NULL DEFAULT false,
    risk_level VARCHAR(20) NOT NULL DEFAULT 'normal', -- 'low', 'normal', 'high', 'critical'
    status VARCHAR(30) NOT NULL DEFAULT 'submitted', -- 'submitted', 'triage', 'assigned', 'under_review', 'awaiting_information', 'resolved', 'closed', 'reopened', 'escalated', 'archived'
    is_escalated BOOLEAN NOT NULL DEFAULT false,
    escalated_to VARCHAR(100),
    escalation_reason TEXT,
    resolution_summary TEXT,
    closed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_safeguarding_cases_ref ON safeguarding_cases(public_reference);
CREATE INDEX IF NOT EXISTS idx_safeguarding_cases_status ON safeguarding_cases(status);
CREATE INDEX IF NOT EXISTS idx_safeguarding_cases_risk ON safeguarding_cases(risk_level);
CREATE INDEX IF NOT EXISTS idx_safeguarding_cases_category ON safeguarding_cases(category);
CREATE INDEX IF NOT EXISTS idx_safeguarding_cases_state ON safeguarding_cases(state_id);
CREATE INDEX IF NOT EXISTS idx_safeguarding_cases_created ON safeguarding_cases(created_at DESC);

-- 3. Safeguarding Reporters (Confidential or Anonymous tracking)
CREATE TABLE IF NOT EXISTS safeguarding_reporters (
    id VARCHAR(36) PRIMARY KEY,
    case_id VARCHAR(36) NOT NULL UNIQUE REFERENCES safeguarding_cases(id) ON DELETE CASCADE,
    is_anonymous BOOLEAN NOT NULL DEFAULT true,
    reporter_user_id VARCHAR(36) REFERENCES users(id) ON DELETE SET NULL,
    encrypted_name TEXT,
    encrypted_email TEXT,
    encrypted_phone TEXT,
    contact_preference VARCHAR(20) NOT NULL DEFAULT 'none', -- 'email', 'sms', 'in_app', 'none'
    follow_up_secret_hash VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_safeguarding_reporters_case ON safeguarding_reporters(case_id);
CREATE INDEX IF NOT EXISTS idx_safeguarding_reporters_user ON safeguarding_reporters(reporter_user_id);

-- 4. Safeguarding Evidence (Private cloud storage files)
CREATE TABLE IF NOT EXISTS safeguarding_evidence (
    id VARCHAR(36) PRIMARY KEY,
    case_id VARCHAR(36) NOT NULL REFERENCES safeguarding_cases(id) ON DELETE CASCADE,
    uploader_type VARCHAR(20) NOT NULL DEFAULT 'reporter', -- 'reporter', 'officer'
    uploader_id VARCHAR(36) REFERENCES users(id) ON DELETE SET NULL,
    storage_key VARCHAR(500) NOT NULL,
    file_name VARCHAR(255) NOT NULL,
    mime_type VARCHAR(100) NOT NULL,
    file_size_bytes BIGINT NOT NULL,
    sha256_hash VARCHAR(64) NOT NULL,
    is_quarantined BOOLEAN NOT NULL DEFAULT false,
    retention_classification VARCHAR(30) NOT NULL DEFAULT 'standard',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_safeguarding_evidence_case ON safeguarding_evidence(case_id);

-- 5. Safeguarding Assignments (Case to Officer linking)
CREATE TABLE IF NOT EXISTS safeguarding_assignments (
    id VARCHAR(36) PRIMARY KEY,
    case_id VARCHAR(36) NOT NULL REFERENCES safeguarding_cases(id) ON DELETE CASCADE,
    officer_id VARCHAR(36) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    assigned_by VARCHAR(36) NOT NULL REFERENCES users(id),
    status VARCHAR(20) NOT NULL DEFAULT 'active', -- 'active', 'reassigned', 'recused', 'revoked'
    assignment_reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_safeguarding_assignments_case ON safeguarding_assignments(case_id);
CREATE INDEX IF NOT EXISTS idx_safeguarding_assignments_officer ON safeguarding_assignments(officer_id);

-- 6. Safeguarding Conflicts of Interest (Enforces strict access denial)
CREATE TABLE IF NOT EXISTS safeguarding_conflicts (
    id VARCHAR(36) PRIMARY KEY,
    case_id VARCHAR(36) NOT NULL REFERENCES safeguarding_cases(id) ON DELETE CASCADE,
    officer_id VARCHAR(36) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    conflict_type VARCHAR(30) NOT NULL, -- 'named_in_complaint', 'self_declared', 'administrative_override'
    declared_by VARCHAR(36) NOT NULL REFERENCES users(id),
    reason TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(case_id, officer_id)
);

CREATE INDEX IF NOT EXISTS idx_safeguarding_conflicts_lookup ON safeguarding_conflicts(case_id, officer_id);

-- 7. Safeguarding Messages (Reporter-visible updates vs Internal notes)
CREATE TABLE IF NOT EXISTS safeguarding_messages (
    id VARCHAR(36) PRIMARY KEY,
    case_id VARCHAR(36) NOT NULL REFERENCES safeguarding_cases(id) ON DELETE CASCADE,
    author_type VARCHAR(20) NOT NULL, -- 'reporter', 'officer', 'system'
    author_id VARCHAR(36) REFERENCES users(id) ON DELETE SET NULL,
    visibility VARCHAR(20) NOT NULL DEFAULT 'reporter', -- 'reporter', 'internal'
    content TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_safeguarding_messages_case ON safeguarding_messages(case_id);
CREATE INDEX IF NOT EXISTS idx_safeguarding_messages_created ON safeguarding_messages(case_id, created_at ASC);

-- 8. Safeguarding Case Actions (Lifecycle & decision audit trail)
CREATE TABLE IF NOT EXISTS safeguarding_case_actions (
    id VARCHAR(36) PRIMARY KEY,
    case_id VARCHAR(36) NOT NULL REFERENCES safeguarding_cases(id) ON DELETE CASCADE,
    actor_id VARCHAR(36) REFERENCES users(id) ON DELETE SET NULL,
    action_type VARCHAR(50) NOT NULL,
    previous_status VARCHAR(30),
    new_status VARCHAR(30),
    reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_safeguarding_actions_case ON safeguarding_case_actions(case_id);

-- 9. Safeguarding Access Logs (Immutable record of case inspections)
CREATE TABLE IF NOT EXISTS safeguarding_access_logs (
    id VARCHAR(36) PRIMARY KEY,
    case_id VARCHAR(36) NOT NULL REFERENCES safeguarding_cases(id) ON DELETE CASCADE,
    actor_id VARCHAR(36) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    access_type VARCHAR(50) NOT NULL, -- 'view_case', 'view_description', 'view_reporter_identity', 'download_evidence', 'view_internal_notes'
    justification TEXT,
    ip_address VARCHAR(45),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_safeguarding_access_case ON safeguarding_access_logs(case_id);
CREATE INDEX IF NOT EXISTS idx_safeguarding_access_actor ON safeguarding_access_logs(actor_id);
CREATE INDEX IF NOT EXISTS idx_safeguarding_access_created ON safeguarding_access_logs(created_at DESC);

-- 10. Safeguarding Retention Records (Statutory retention and legal hold management)
CREATE TABLE IF NOT EXISTS safeguarding_retention_records (
    id VARCHAR(36) PRIMARY KEY,
    case_id VARCHAR(36) NOT NULL UNIQUE REFERENCES safeguarding_cases(id) ON DELETE CASCADE,
    retention_category VARCHAR(50) NOT NULL DEFAULT 'standard', -- 'standard', 'minors_safeguarding', 'serious_misconduct', 'statutory_hold'
    review_date TIMESTAMPTZ NOT NULL,
    is_legal_hold BOOLEAN NOT NULL DEFAULT false,
    legal_hold_reason TEXT,
    disposal_authorized_by VARCHAR(36) REFERENCES users(id),
    disposed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_safeguarding_retention_review ON safeguarding_retention_records(review_date);

-- 11. Safeguarding Settings (Configurable thresholds and emergency guidance)
CREATE TABLE IF NOT EXISTS safeguarding_settings (
    key VARCHAR(50) PRIMARY KEY,
    value TEXT NOT NULL,
    description TEXT,
    updated_by VARCHAR(36) REFERENCES users(id),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Seed baseline safeguarding governance settings
INSERT INTO safeguarding_settings (key, value, description)
VALUES
  ('emergency_guidance_text', 'This form is not monitored continuously and is not an emergency response service. If someone is in immediate danger, contact local emergency services or a trusted safeguarding authority immediately.', 'Standard notice displayed prominently on the public reporting form'),
  ('independent_escalation_authority', 'Board Governance & Ethics Oversight Panel', 'Independent external body for complaints involving executive leadership'),
  ('minors_retention_years', '7', 'Configurable retention period for minor safeguarding records following resolution'),
  ('standard_retention_years', '5', 'Standard retention period for closed safeguarding records')
ON CONFLICT (key) DO NOTHING;

-- +goose Down
DROP TABLE IF EXISTS safeguarding_settings;
DROP TABLE IF EXISTS safeguarding_retention_records;
DROP TABLE IF EXISTS safeguarding_access_logs;
DROP TABLE IF EXISTS safeguarding_case_actions;
DROP TABLE IF EXISTS safeguarding_messages;
DROP TABLE IF EXISTS safeguarding_conflicts;
DROP TABLE IF EXISTS safeguarding_assignments;
DROP TABLE IF EXISTS safeguarding_evidence;
DROP TABLE IF EXISTS safeguarding_reporters;
DROP TABLE IF EXISTS safeguarding_cases;
DROP TABLE IF EXISTS safeguarding_officers;
