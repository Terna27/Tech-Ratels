-- +goose Up
-- Migration 20261020000015: Public Transparency, Accountability & Reporting Portal

-- 1. Environmental impact metrics on event_instances
ALTER TABLE event_instances
ADD COLUMN IF NOT EXISTS waste_collected_kg BIGINT NOT NULL DEFAULT 0,
ADD COLUMN IF NOT EXISTS waste_bags_count INT NOT NULL DEFAULT 0,
ADD COLUMN IF NOT EXISTS recyclables_kg BIGINT NOT NULL DEFAULT 0,
ADD COLUMN IF NOT EXISTS cleanup_duration_hours NUMERIC(5,2) NOT NULL DEFAULT 0,
ADD COLUMN IF NOT EXISTS environmental_notes TEXT;

-- 2. Report Definitions
CREATE TABLE IF NOT EXISTS public_report_definitions (
    id VARCHAR(36) PRIMARY KEY,
    report_type VARCHAR(50) NOT NULL UNIQUE,
    name VARCHAR(150) NOT NULL,
    description TEXT NOT NULL,
    reporting_scope VARCHAR(30) NOT NULL DEFAULT 'national',
    aggregation_rules JSONB NOT NULL DEFAULT '{}'::jsonb,
    visibility_config JSONB NOT NULL DEFAULT '{}'::jsonb,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_public_report_def_type ON public_report_definitions(report_type);
CREATE INDEX IF NOT EXISTS idx_public_report_def_active ON public_report_definitions(is_active);

-- Seed default public report definitions
INSERT INTO public_report_definitions (id, report_type, name, description, reporting_scope) VALUES
    ('rep-def-nat-overview', 'national_overview', 'National Overview & Impact Summary', 'Comprehensive top-line performance metrics across cleanups, verified attendance, community projects, and reconciled finances nationwide.', 'national'),
    ('rep-def-cleanups', 'cleanups', 'Environmental Cleanup & Impact Reports', 'Detailed monthly cleanup campaign metrics including verified venues, completed activities, waste collected, and attendee counts.', 'national'),
    ('rep-def-participation', 'participation', 'Verified Civic Participation Reports', 'Civic volunteerism trends, verified attendance statistics, unique participants, and geographic engagement across Nigeria.', 'national'),
    ('rep-def-finance', 'finance', 'Approved Financial Accountability & Ledger Disclosures', 'Super Admin certified public financial disclosures including approved income, operational expenses, project grants, and budget execution.', 'national'),
    ('rep-def-foundation', 'foundation', 'Martinsvincent Ortse Foundation Charitable Reports', 'Segregated charitable donation receipts, humanitarian aid disbursements, and community impact summaries under the Foundation.', 'national'),
    ('rep-def-projects', 'community_projects', 'Community Development Projects Progress', 'Status, budget execution, physical milestone inspections, and beneficiary impact of approved public community projects.', 'national'),
    ('rep-def-annual', 'annual_impact', 'Certified Annual Impact & Transparency Report', 'Authoritative annual consolidation of all Nigerian Ratels operations, audited expenditures, environmental tonnage, and volunteer hours.', 'national')
ON CONFLICT (report_type) DO UPDATE SET
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    reporting_scope = EXCLUDED.reporting_scope;

-- 3. Report Snapshots (Immutable once published)
CREATE TABLE IF NOT EXISTS public_report_snapshots (
    id VARCHAR(36) PRIMARY KEY,
    public_reference VARCHAR(50) NOT NULL UNIQUE,
    definition_id VARCHAR(36) REFERENCES public_report_definitions(id),
    report_type VARCHAR(50) NOT NULL,
    title VARCHAR(255) NOT NULL,
    reporting_period VARCHAR(50) NOT NULL,
    period_start DATE NOT NULL,
    period_end DATE NOT NULL,
    scope_type VARCHAR(30) NOT NULL DEFAULT 'national',
    zone_id VARCHAR(36) REFERENCES zones(id),
    state_id VARCHAR(36) REFERENCES states(id),
    lga_id VARCHAR(36) REFERENCES lgas(id),
    status VARCHAR(30) NOT NULL DEFAULT 'draft', -- 'draft', 'pending_review', 'approved', 'published', 'rejected', 'withdrawn', 'superseded'
    aggregated_data JSONB NOT NULL DEFAULT '{}'::jsonb,
    data_hash VARCHAR(128) NOT NULL,
    publication_version INT NOT NULL DEFAULT 1,
    correction_notes TEXT,
    created_by_user_id VARCHAR(36) NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_report_snapshots_ref ON public_report_snapshots(public_reference);
CREATE INDEX IF NOT EXISTS idx_report_snapshots_type ON public_report_snapshots(report_type);
CREATE INDEX IF NOT EXISTS idx_report_snapshots_status ON public_report_snapshots(status);
CREATE INDEX IF NOT EXISTS idx_report_snapshots_scope ON public_report_snapshots(scope_type, state_id, lga_id);
CREATE INDEX IF NOT EXISTS idx_report_snapshots_period ON public_report_snapshots(reporting_period);
CREATE INDEX IF NOT EXISTS idx_report_snapshots_created ON public_report_snapshots(created_at DESC);

-- 4. Report Approvals (Governance and separation of duties)
CREATE TABLE IF NOT EXISTS public_report_approvals (
    id VARCHAR(36) PRIMARY KEY,
    snapshot_id VARCHAR(36) NOT NULL REFERENCES public_report_snapshots(id) ON DELETE CASCADE,
    reviewer_id VARCHAR(36) NOT NULL REFERENCES users(id),
    decision VARCHAR(30) NOT NULL, -- 'approved', 'rejected', 'revision_requested'
    decision_reason TEXT NOT NULL,
    governance_scope VARCHAR(50) NOT NULL DEFAULT 'national',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_report_approvals_snapshot ON public_report_approvals(snapshot_id);
CREATE INDEX IF NOT EXISTS idx_report_approvals_reviewer ON public_report_approvals(reviewer_id);

-- 5. Report Publications (Active and Historical tracking)
CREATE TABLE IF NOT EXISTS public_report_publications (
    id VARCHAR(36) PRIMARY KEY,
    snapshot_id VARCHAR(36) NOT NULL UNIQUE REFERENCES public_report_snapshots(id) ON DELETE CASCADE,
    status VARCHAR(30) NOT NULL DEFAULT 'published', -- 'published', 'withdrawn', 'superseded'
    published_by_user_id VARCHAR(36) NOT NULL REFERENCES users(id),
    published_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    withdrawn_by_user_id VARCHAR(36) REFERENCES users(id),
    withdrawn_at TIMESTAMPTZ,
    withdrawal_reason TEXT,
    superseded_by_snapshot_id VARCHAR(36) REFERENCES public_report_snapshots(id),
    correction_notice TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_report_pub_snapshot ON public_report_publications(snapshot_id);
CREATE INDEX IF NOT EXISTS idx_report_pub_status ON public_report_publications(status);
CREATE INDEX IF NOT EXISTS idx_report_pub_published_at ON public_report_publications(published_at DESC);

-- 6. Report Exports (PDF and CSV cache metadata)
CREATE TABLE IF NOT EXISTS public_report_exports (
    id VARCHAR(36) PRIMARY KEY,
    snapshot_id VARCHAR(36) NOT NULL REFERENCES public_report_snapshots(id) ON DELETE CASCADE,
    export_format VARCHAR(20) NOT NULL, -- 'pdf', 'csv'
    file_name VARCHAR(255) NOT NULL,
    content_hash VARCHAR(128) NOT NULL,
    storage_reference TEXT,
    export_status VARCHAR(30) NOT NULL DEFAULT 'generated',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_report_exports_snapshot ON public_report_exports(snapshot_id, export_format);

-- 7. Report Audit Trail
CREATE TABLE IF NOT EXISTS public_report_audit (
    id VARCHAR(36) PRIMARY KEY,
    snapshot_id VARCHAR(36),
    action VARCHAR(50) NOT NULL, -- 'draft_created', 'calculated', 'submitted_for_review', 'approved', 'rejected', 'published', 'withdrawn', 'revised', 'exported'
    performed_by_user_id VARCHAR(36) REFERENCES users(id),
    details JSONB NOT NULL DEFAULT '{}'::jsonb,
    ip_address VARCHAR(45),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_report_audit_snapshot ON public_report_audit(snapshot_id);
CREATE INDEX IF NOT EXISTS idx_report_audit_action ON public_report_audit(action);
CREATE INDEX IF NOT EXISTS idx_report_audit_created ON public_report_audit(created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS public_report_audit;
DROP TABLE IF EXISTS public_report_exports;
DROP TABLE IF EXISTS public_report_publications;
DROP TABLE IF EXISTS public_report_approvals;
DROP TABLE IF EXISTS public_report_snapshots;
DROP TABLE IF EXISTS public_report_definitions;
ALTER TABLE event_instances
DROP COLUMN IF EXISTS environmental_notes,
DROP COLUMN IF EXISTS cleanup_duration_hours,
DROP COLUMN IF EXISTS recyclables_kg,
DROP COLUMN IF EXISTS waste_bags_count,
DROP COLUMN IF EXISTS waste_collected_kg;
