-- +goose Up
-- Phase 67.1: Super Admin Organization-Wide Settings
CREATE TABLE IF NOT EXISTS organization_settings (
    key VARCHAR(100) PRIMARY KEY,
    value TEXT NOT NULL,
    description TEXT,
    category VARCHAR(50) NOT NULL DEFAULT 'general',
    is_sensitive BOOLEAN NOT NULL DEFAULT false,
    updated_by VARCHAR(36) REFERENCES users(id),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Seed authoritative baseline organization-wide settings
INSERT INTO organization_settings (key, value, description, category, is_sensitive)
VALUES
  ('movement_name', 'Nigerian Ratels Movement', 'Authoritative legal and public title of the movement', 'general', false),
  ('movement_motto', 'Clean Communities, Transparent Leadership', 'Official slogan displayed across public platforms', 'general', false),
  ('contact_email', 'admin@ratels.org.ng', 'Official national secretariat contact email address', 'general', false),
  ('membership_registration_enabled', 'true', 'System-wide flag allowing new members to register across Nigeria', 'membership', false),
  ('public_transparency_portal_enabled', 'true', 'Nationwide public availability of financial reports and disclosure portal', 'governance', false),
  ('activity_creation_open', 'true', 'Allows regional coordinators to propose and publish environmental cleanup venues', 'activities', false),
  ('emergency_broadcast_active', 'false', 'Circuit breaker activating critical banner alert across member apps', 'operations', false),
  ('emergency_broadcast_message', '', 'Notice content displayed when emergency broadcast circuit breaker is active', 'operations', false)
ON CONFLICT (key) DO NOTHING;

-- +goose Down
DROP TABLE IF EXISTS organization_settings;
