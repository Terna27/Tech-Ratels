-- +goose Up
-- Event Media Gallery & Moderation System Migration (Phase A)

CREATE TABLE IF NOT EXISTS event_media (
    id VARCHAR(36) PRIMARY KEY,
    event_id VARCHAR(36) NOT NULL REFERENCES event_instances(id) ON DELETE CASCADE,
    uploaded_by_user_id VARCHAR(36) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    media_type VARCHAR(20) NOT NULL, -- 'image', 'video'
    caption VARCHAR(255) NOT NULL,
    description TEXT,
    storage_key VARCHAR(500) NOT NULL,
    public_url VARCHAR(500),
    thumbnail_url VARCHAR(500),
    mime_type VARCHAR(100) NOT NULL,
    file_size_bytes BIGINT NOT NULL,
    width INT,
    height INT,
    duration_seconds NUMERIC(10, 2),
    moderation_status VARCHAR(30) NOT NULL DEFAULT 'pending_review', -- 'draft', 'pending_review', 'approved', 'rejected', 'revision_requested', 'withdrawn', 'removed'
    rejection_reason TEXT,
    reviewed_by_user_id VARCHAR(36) REFERENCES users(id),
    reviewed_at TIMESTAMPTZ,
    is_published BOOLEAN NOT NULL DEFAULT FALSE,
    published_at TIMESTAMPTZ,
    is_hidden BOOLEAN NOT NULL DEFAULT FALSE,
    is_featured BOOLEAN NOT NULL DEFAULT FALSE,
    metadata_sanitized BOOLEAN NOT NULL DEFAULT FALSE,
    sanitized_at TIMESTAMPTZ,
    allow_public_attribution BOOLEAN NOT NULL DEFAULT FALSE,
    state_id VARCHAR(36) NOT NULL REFERENCES states(id),
    lga_id VARCHAR(36) NOT NULL REFERENCES lgas(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    version INT NOT NULL DEFAULT 1
);

CREATE INDEX IF NOT EXISTS idx_event_media_event_id ON event_media(event_id);
CREATE INDEX IF NOT EXISTS idx_event_media_uploader ON event_media(uploaded_by_user_id);
CREATE INDEX IF NOT EXISTS idx_event_media_moderation_status ON event_media(moderation_status);
CREATE INDEX IF NOT EXISTS idx_event_media_published ON event_media(is_published, is_hidden, moderation_status);
CREATE INDEX IF NOT EXISTS idx_event_media_state_lga ON event_media(state_id, lga_id);
CREATE INDEX IF NOT EXISTS idx_event_media_created_at ON event_media(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_event_media_is_featured ON event_media(is_featured);

CREATE TABLE IF NOT EXISTS event_media_consent (
    id VARCHAR(36) PRIMARY KEY,
    media_id VARCHAR(36) NOT NULL REFERENCES event_media(id) ON DELETE CASCADE,
    has_owner_permission BOOLEAN NOT NULL DEFAULT FALSE,
    understands_public_display BOOLEAN NOT NULL DEFAULT FALSE,
    has_identifiable_consent BOOLEAN NOT NULL DEFAULT FALSE,
    has_no_confidential_info BOOLEAN NOT NULL DEFAULT FALSE,
    has_minors BOOLEAN NOT NULL DEFAULT FALSE,
    has_guardian_consent BOOLEAN NOT NULL DEFAULT FALSE,
    consent_notes TEXT,
    declared_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_event_media_consent_media_id ON event_media_consent(media_id);

CREATE TABLE IF NOT EXISTS media_moderation_actions (
    id VARCHAR(36) PRIMARY KEY,
    media_id VARCHAR(36) NOT NULL REFERENCES event_media(id) ON DELETE CASCADE,
    reviewer_user_id VARCHAR(36) NOT NULL REFERENCES users(id),
    action VARCHAR(50) NOT NULL, -- 'approve', 'reject', 'request_revision', 'remove', 'hide', 'unhide'
    reason TEXT NOT NULL,
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_media_moderation_actions_media_id ON media_moderation_actions(media_id);
CREATE INDEX IF NOT EXISTS idx_media_moderation_actions_reviewer ON media_moderation_actions(reviewer_user_id);

CREATE TABLE IF NOT EXISTS media_reports (
    id VARCHAR(36) PRIMARY KEY,
    media_id VARCHAR(36) NOT NULL REFERENCES event_media(id) ON DELETE CASCADE,
    reporter_user_id VARCHAR(36) REFERENCES users(id),
    reporter_email VARCHAR(150),
    report_reason VARCHAR(50) NOT NULL, -- 'privacy_violation', 'missing_consent', 'incorrect_event', 'inappropriate_content', 'copyright_concern', 'personal_safety', 'other'
    details TEXT NOT NULL,
    status VARCHAR(30) NOT NULL DEFAULT 'pending', -- 'pending', 'investigating', 'resolved', 'dismissed'
    resolution_notes TEXT,
    resolved_by_user_id VARCHAR(36) REFERENCES users(id),
    resolved_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_media_reports_media_id ON media_reports(media_id);
CREATE INDEX IF NOT EXISTS idx_media_reports_status ON media_reports(status);

CREATE TABLE IF NOT EXISTS media_takedown_requests (
    id VARCHAR(36) PRIMARY KEY,
    media_id VARCHAR(36) NOT NULL REFERENCES event_media(id) ON DELETE CASCADE,
    requester_user_id VARCHAR(36) REFERENCES users(id),
    requester_name VARCHAR(150) NOT NULL,
    requester_email VARCHAR(150) NOT NULL,
    relationship_to_media VARCHAR(100) NOT NULL, -- 'uploader', 'subject_in_media', 'legal_representative', 'concerned_citizen'
    reason VARCHAR(50) NOT NULL, -- 'privacy_violation', 'consent_withdrawn', 'defamation', 'copyright_infringement', 'safety_concern', 'other'
    details TEXT NOT NULL,
    status VARCHAR(30) NOT NULL DEFAULT 'pending', -- 'pending', 'approved_removed', 'rejected_retained'
    reviewed_by_user_id VARCHAR(36) REFERENCES users(id),
    resolution_reason TEXT,
    resolved_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_media_takedown_requests_media_id ON media_takedown_requests(media_id);
CREATE INDEX IF NOT EXISTS idx_media_takedown_requests_status ON media_takedown_requests(status);

-- +goose Down
DROP TABLE IF EXISTS media_takedown_requests CASCADE;
DROP TABLE IF EXISTS media_reports CASCADE;
DROP TABLE IF EXISTS media_moderation_actions CASCADE;
DROP TABLE IF EXISTS event_media_consent CASCADE;
DROP TABLE IF EXISTS event_media CASCADE;
