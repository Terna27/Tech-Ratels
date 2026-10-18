-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied

CREATE TABLE IF NOT EXISTS zones (
    id VARCHAR(36) PRIMARY KEY,
    code VARCHAR(10) UNIQUE NOT NULL,
    name VARCHAR(50) UNIQUE NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    version INT NOT NULL DEFAULT 1
);
CREATE INDEX IF NOT EXISTS idx_zones_code ON zones(code);

CREATE TABLE IF NOT EXISTS states (
    id VARCHAR(36) PRIMARY KEY,
    zone_id VARCHAR(36) NOT NULL REFERENCES zones(id) ON DELETE CASCADE,
    code VARCHAR(10) UNIQUE NOT NULL,
    name VARCHAR(100) UNIQUE NOT NULL,
    is_fct BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    version INT NOT NULL DEFAULT 1
);
CREATE INDEX IF NOT EXISTS idx_states_zone_id ON states(zone_id);
CREATE INDEX IF NOT EXISTS idx_states_code ON states(code);

CREATE TABLE IF NOT EXISTS lgas (
    id VARCHAR(36) PRIMARY KEY,
    state_id VARCHAR(36) NOT NULL REFERENCES states(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    is_area_council BOOLEAN NOT NULL DEFAULT FALSE,
    source_reference VARCHAR(200) NOT NULL DEFAULT 'National Reference / NBS 2018',
    revision_date VARCHAR(20) NOT NULL DEFAULT '2026-10-17',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    version INT NOT NULL DEFAULT 1
);
CREATE INDEX IF NOT EXISTS idx_lgas_state_id ON lgas(state_id);
CREATE INDEX IF NOT EXISTS idx_lgas_name ON lgas(name);

CREATE TABLE IF NOT EXISTS capital_cities (
    id VARCHAR(36) PRIMARY KEY,
    state_id VARCHAR(36) NOT NULL REFERENCES states(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    version INT NOT NULL DEFAULT 1
);
CREATE INDEX IF NOT EXISTS idx_capital_cities_state_id ON capital_cities(state_id);

CREATE TABLE IF NOT EXISTS users (
    id VARCHAR(36) PRIMARY KEY,
    email VARCHAR(255) UNIQUE,
    phone VARCHAR(50) UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    display_name VARCHAR(100) NOT NULL,
    membership_reference VARCHAR(50) UNIQUE NOT NULL,
    status VARCHAR(30) NOT NULL DEFAULT 'pending_verification',
    primary_role VARCHAR(50) NOT NULL DEFAULT 'member',
    is_verified BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    version INT NOT NULL DEFAULT 1
);
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_users_phone ON users(phone);
CREATE INDEX IF NOT EXISTS idx_users_membership_reference ON users(membership_reference);
CREATE INDEX IF NOT EXISTS idx_users_primary_role ON users(primary_role);

CREATE TABLE IF NOT EXISTS member_profiles (
    id VARCHAR(36) PRIMARY KEY,
    user_id VARCHAR(36) UNIQUE NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    home_state_id VARCHAR(36) REFERENCES states(id),
    home_lga_id VARCHAR(36) REFERENCES lgas(id),
    address VARCHAR(255),
    bio TEXT,
    avatar_url VARCHAR(255),
    is_public_badges BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    version INT NOT NULL DEFAULT 1
);
CREATE INDEX IF NOT EXISTS idx_member_profiles_user_id ON member_profiles(user_id);
CREATE INDEX IF NOT EXISTS idx_member_profiles_home_state_id ON member_profiles(home_state_id);

CREATE TABLE IF NOT EXISTS contact_verifications (
    id VARCHAR(36) PRIMARY KEY,
    user_id VARCHAR(36) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    identifier VARCHAR(255) NOT NULL,
    code VARCHAR(20) NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    is_used BOOLEAN NOT NULL DEFAULT FALSE,
    attempt_count INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    version INT NOT NULL DEFAULT 1
);
CREATE INDEX IF NOT EXISTS idx_contact_verifications_user_id ON contact_verifications(user_id);

CREATE TABLE IF NOT EXISTS user_sessions (
    id VARCHAR(36) PRIMARY KEY,
    user_id VARCHAR(36) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_jti VARCHAR(64) UNIQUE NOT NULL,
    is_revoked BOOLEAN NOT NULL DEFAULT FALSE,
    expires_at TIMESTAMPTZ NOT NULL,
    ip_address VARCHAR(45),
    user_agent VARCHAR(255),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    version INT NOT NULL DEFAULT 1
);
CREATE INDEX IF NOT EXISTS idx_user_sessions_user_id ON user_sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_user_sessions_token_jti ON user_sessions(token_jti);

CREATE TABLE IF NOT EXISTS appointments (
    id VARCHAR(36) PRIMARY KEY,
    user_id VARCHAR(36) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_title VARCHAR(100) NOT NULL,
    role_type VARCHAR(50) NOT NULL,
    jurisdiction_type VARCHAR(30) NOT NULL,
    jurisdiction_id VARCHAR(36),
    reporting_to_appointment_id VARCHAR(36) REFERENCES appointments(id),
    start_date TIMESTAMPTZ NOT NULL,
    end_date TIMESTAMPTZ,
    status VARCHAR(30) NOT NULL DEFAULT 'active',
    approval_reference VARCHAR(100) NOT NULL,
    public_contact VARCHAR(150),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    version INT NOT NULL DEFAULT 1
);
CREATE INDEX IF NOT EXISTS idx_appointments_user_id ON appointments(user_id);
CREATE INDEX IF NOT EXISTS idx_appointments_role_type ON appointments(role_type);
CREATE INDEX IF NOT EXISTS idx_appointments_jurisdiction_id ON appointments(jurisdiction_id);
CREATE INDEX IF NOT EXISTS idx_appointments_status ON appointments(status);

CREATE TABLE IF NOT EXISTS appointment_history (
    id VARCHAR(36) PRIMARY KEY,
    appointment_id VARCHAR(36) NOT NULL REFERENCES appointments(id) ON DELETE CASCADE,
    action VARCHAR(50) NOT NULL,
    changed_by_user_id VARCHAR(36) REFERENCES users(id),
    previous_status VARCHAR(30),
    new_status VARCHAR(30) NOT NULL,
    reason TEXT,
    timestamp TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    version INT NOT NULL DEFAULT 1
);
CREATE INDEX IF NOT EXISTS idx_appointment_history_appointment_id ON appointment_history(appointment_id);

CREATE TABLE IF NOT EXISTS cleanup_campaigns (
    id VARCHAR(36) PRIMARY KEY,
    year INT NOT NULL,
    month INT NOT NULL,
    scheduled_date TIMESTAMPTZ NOT NULL,
    title VARCHAR(150) NOT NULL,
    description TEXT,
    status VARCHAR(30) NOT NULL DEFAULT 'scheduled',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    version INT NOT NULL DEFAULT 1,
    CONSTRAINT uq_campaign_year_month UNIQUE (year, month)
);
CREATE INDEX IF NOT EXISTS idx_cleanup_campaigns_year_month ON cleanup_campaigns(year, month);

CREATE TABLE IF NOT EXISTS event_instances (
    id VARCHAR(36) PRIMARY KEY,
    campaign_id VARCHAR(36) REFERENCES cleanup_campaigns(id) ON DELETE CASCADE,
    programme_type VARCHAR(50) NOT NULL DEFAULT 'cleanup',
    title VARCHAR(150) NOT NULL,
    state_id VARCHAR(36) NOT NULL REFERENCES states(id),
    lga_id VARCHAR(36) NOT NULL REFERENCES lgas(id),
    address VARCHAR(255) NOT NULL,
    landmark VARCHAR(150) NOT NULL,
    latitude DOUBLE PRECISION,
    longitude DOUBLE PRECISION,
    start_time TIMESTAMPTZ NOT NULL,
    end_time TIMESTAMPTZ NOT NULL,
    capacity INT NOT NULL DEFAULT 100,
    registered_count INT NOT NULL DEFAULT 0,
    coordinator_id VARCHAR(36) NOT NULL REFERENCES users(id),
    public_contact VARCHAR(150) NOT NULL,
    accessibility_notes TEXT,
    safety_notes TEXT,
    status VARCHAR(30) NOT NULL DEFAULT 'draft',
    rejection_reason TEXT,
    rescheduled_from TIMESTAMPTZ,
    rescheduled_reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    version INT NOT NULL DEFAULT 1
);
CREATE INDEX IF NOT EXISTS idx_event_instances_campaign_id ON event_instances(campaign_id);
CREATE INDEX IF NOT EXISTS idx_event_instances_state_id ON event_instances(state_id);
CREATE INDEX IF NOT EXISTS idx_event_instances_lga_id ON event_instances(lga_id);
CREATE INDEX IF NOT EXISTS idx_event_instances_coordinator_id ON event_instances(coordinator_id);
CREATE INDEX IF NOT EXISTS idx_event_instances_status ON event_instances(status);

CREATE TABLE IF NOT EXISTS event_registrations (
    id VARCHAR(36) PRIMARY KEY,
    event_id VARCHAR(36) NOT NULL REFERENCES event_instances(id) ON DELETE CASCADE,
    user_id VARCHAR(36) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status VARCHAR(30) NOT NULL DEFAULT 'registered',
    registered_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    cancelled_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    version INT NOT NULL DEFAULT 1,
    CONSTRAINT uq_event_user_registration UNIQUE (event_id, user_id)
);
CREATE INDEX IF NOT EXISTS idx_event_registrations_event_id ON event_registrations(event_id);
CREATE INDEX IF NOT EXISTS idx_event_registrations_user_id ON event_registrations(user_id);

CREATE TABLE IF NOT EXISTS participation_claims (
    id VARCHAR(36) PRIMARY KEY,
    event_id VARCHAR(36) NOT NULL REFERENCES event_instances(id) ON DELETE CASCADE,
    user_id VARCHAR(36) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    description TEXT NOT NULL,
    evidence_notes TEXT,
    status VARCHAR(30) NOT NULL DEFAULT 'pending',
    submitted_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    reviewed_by_user_id VARCHAR(36) REFERENCES users(id),
    review_decision VARCHAR(30),
    review_reason TEXT,
    reviewed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    version INT NOT NULL DEFAULT 1,
    CONSTRAINT uq_event_user_participation UNIQUE (event_id, user_id)
);
CREATE INDEX IF NOT EXISTS idx_participation_claims_event_id ON participation_claims(event_id);
CREATE INDEX IF NOT EXISTS idx_participation_claims_user_id ON participation_claims(user_id);
CREATE INDEX IF NOT EXISTS idx_participation_claims_status ON participation_claims(status);

CREATE TABLE IF NOT EXISTS participation_appeals (
    id VARCHAR(36) PRIMARY KEY,
    participation_id VARCHAR(36) NOT NULL REFERENCES participation_claims(id) ON DELETE CASCADE,
    appeal_reason TEXT NOT NULL,
    status VARCHAR(30) NOT NULL DEFAULT 'pending',
    appealed_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    reviewed_by_user_id VARCHAR(36) REFERENCES users(id),
    decision_reason TEXT,
    reviewed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    version INT NOT NULL DEFAULT 1
);
CREATE INDEX IF NOT EXISTS idx_participation_appeals_participation_id ON participation_appeals(participation_id);

CREATE TABLE IF NOT EXISTS badges (
    id VARCHAR(36) PRIMARY KEY,
    code VARCHAR(50) UNIQUE NOT NULL,
    name VARCHAR(100) NOT NULL,
    description TEXT NOT NULL,
    rule_version VARCHAR(20) NOT NULL DEFAULT '1.0',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    version INT NOT NULL DEFAULT 1
);
CREATE INDEX IF NOT EXISTS idx_badges_code ON badges(code);

CREATE TABLE IF NOT EXISTS badge_awards (
    id VARCHAR(36) PRIMARY KEY,
    user_id VARCHAR(36) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    badge_id VARCHAR(36) NOT NULL REFERENCES badges(id) ON DELETE CASCADE,
    qualifying_participation_id VARCHAR(36) REFERENCES participation_claims(id),
    awarded_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    is_public BOOLEAN NOT NULL DEFAULT TRUE,
    revocation_reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    version INT NOT NULL DEFAULT 1,
    CONSTRAINT uq_user_badge_award UNIQUE (user_id, badge_id)
);
CREATE INDEX IF NOT EXISTS idx_badge_awards_user_id ON badge_awards(user_id);
CREATE INDEX IF NOT EXISTS idx_badge_awards_badge_id ON badge_awards(badge_id);

CREATE TABLE IF NOT EXISTS foundation_entities (
    id VARCHAR(36) PRIMARY KEY,
    provisional_name VARCHAR(150) NOT NULL DEFAULT 'Martins Vincent Otse Foundation',
    registered_legal_name VARCHAR(150),
    registration_number VARCHAR(100),
    is_verified BOOLEAN NOT NULL DEFAULT FALSE,
    mission_statement TEXT NOT NULL,
    disclaimer TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    version INT NOT NULL DEFAULT 1
);

CREATE TABLE IF NOT EXISTS verified_bank_account_versions (
    id VARCHAR(36) PRIMARY KEY,
    bank_name VARCHAR(100) NOT NULL,
    account_name VARCHAR(150) NOT NULL,
    account_number VARCHAR(50) NOT NULL,
    currency VARCHAR(10) NOT NULL DEFAULT 'NGN',
    status VARCHAR(30) NOT NULL DEFAULT 'pending_verification',
    verified_by_officer_1 VARCHAR(100),
    verified_by_officer_2 VARCHAR(100),
    effective_date TIMESTAMPTZ,
    verification_notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    version INT NOT NULL DEFAULT 1
);

CREATE TABLE IF NOT EXISTS foundation_campaigns (
    id VARCHAR(36) PRIMARY KEY,
    title VARCHAR(150) NOT NULL,
    purpose TEXT NOT NULL,
    target_amount_kobo BIGINT NOT NULL DEFAULT 0,
    status VARCHAR(30) NOT NULL DEFAULT 'draft',
    is_live_donations_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    version INT NOT NULL DEFAULT 1
);

CREATE TABLE IF NOT EXISTS audit_events (
    id VARCHAR(36) PRIMARY KEY,
    actor_id VARCHAR(36),
    actor_role VARCHAR(50),
    action VARCHAR(100) NOT NULL,
    resource_type VARCHAR(50) NOT NULL,
    resource_id VARCHAR(36) NOT NULL,
    scope_type VARCHAR(30),
    scope_id VARCHAR(36),
    details TEXT,
    ip_address VARCHAR(45),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    version INT NOT NULL DEFAULT 1
);
CREATE INDEX IF NOT EXISTS idx_audit_events_actor_id ON audit_events(actor_id);
CREATE INDEX IF NOT EXISTS idx_audit_events_action ON audit_events(action);
CREATE INDEX IF NOT EXISTS idx_audit_events_resource ON audit_events(resource_type, resource_id);

-- +goose Down
-- SQL in section 'Down' is executed when this migration is rolled back

DROP TABLE IF EXISTS audit_events;
DROP TABLE IF EXISTS foundation_campaigns;
DROP TABLE IF EXISTS verified_bank_account_versions;
DROP TABLE IF EXISTS foundation_entities;
DROP TABLE IF EXISTS badge_awards;
DROP TABLE IF EXISTS badges;
DROP TABLE IF EXISTS participation_appeals;
DROP TABLE IF EXISTS participation_claims;
DROP TABLE IF EXISTS event_registrations;
DROP TABLE IF EXISTS event_instances;
DROP TABLE IF EXISTS cleanup_campaigns;
DROP TABLE IF EXISTS appointment_history;
DROP TABLE IF EXISTS appointments;
DROP TABLE IF EXISTS user_sessions;
DROP TABLE IF EXISTS contact_verifications;
DROP TABLE IF EXISTS member_profiles;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS capital_cities;
DROP TABLE IF EXISTS lgas;
DROP TABLE IF EXISTS states;
DROP TABLE IF EXISTS zones;
