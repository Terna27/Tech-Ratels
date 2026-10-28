-- +goose Up
-- Phase 67.3: Coordinator Operational Privileges & Participation Verification

-- 1. Extend event_instances with description and zone_id
ALTER TABLE event_instances ADD COLUMN IF NOT EXISTS description TEXT;
ALTER TABLE event_instances ADD COLUMN IF NOT EXISTS zone_id VARCHAR(36) REFERENCES zones(id);
CREATE INDEX IF NOT EXISTS idx_event_instances_zone_id ON event_instances(zone_id);

-- 2. Extend event_registrations with unpredictable check-in credentials and attendance verification
ALTER TABLE event_registrations ADD COLUMN IF NOT EXISTS checkin_code VARCHAR(64) UNIQUE;
ALTER TABLE event_registrations ADD COLUMN IF NOT EXISTS attended_at TIMESTAMPTZ;
ALTER TABLE event_registrations ADD COLUMN IF NOT EXISTS verified_at TIMESTAMPTZ;
ALTER TABLE event_registrations ADD COLUMN IF NOT EXISTS verified_by_user_id VARCHAR(36) REFERENCES users(id);
ALTER TABLE event_registrations ADD COLUMN IF NOT EXISTS verification_method VARCHAR(30);
CREATE INDEX IF NOT EXISTS idx_event_registrations_checkin_code ON event_registrations(checkin_code);
CREATE INDEX IF NOT EXISTS idx_event_registrations_verified_at ON event_registrations(verified_at);

-- Backfill existing event_registrations with unique check-in codes if empty
UPDATE event_registrations
SET checkin_code = 'RT-' || UPPER(SUBSTRING(REPLACE(gen_random_uuid()::text, '-', '') FROM 1 FOR 16))
WHERE checkin_code IS NULL;

-- 3. Extend participation_claims with secure evidence references and Super Admin reversal trail
ALTER TABLE participation_claims ADD COLUMN IF NOT EXISTS evidence_url TEXT;
ALTER TABLE participation_claims ADD COLUMN IF NOT EXISTS evidence_type VARCHAR(50);
ALTER TABLE participation_claims ADD COLUMN IF NOT EXISTS verification_method VARCHAR(30) DEFAULT 'evidence_correction';
ALTER TABLE participation_claims ADD COLUMN IF NOT EXISTS reversed_at TIMESTAMPTZ;
ALTER TABLE participation_claims ADD COLUMN IF NOT EXISTS reversed_by_user_id VARCHAR(36) REFERENCES users(id);
ALTER TABLE participation_claims ADD COLUMN IF NOT EXISTS reversal_reason TEXT;
CREATE INDEX IF NOT EXISTS idx_participation_claims_reversed_at ON participation_claims(reversed_at);

-- +goose Down
DROP INDEX IF EXISTS idx_participation_claims_reversed_at;
ALTER TABLE participation_claims DROP COLUMN IF EXISTS reversal_reason;
ALTER TABLE participation_claims DROP COLUMN IF EXISTS reversed_by_user_id;
ALTER TABLE participation_claims DROP COLUMN IF EXISTS reversed_at;
ALTER TABLE participation_claims DROP COLUMN IF EXISTS verification_method;
ALTER TABLE participation_claims DROP COLUMN IF EXISTS evidence_type;
ALTER TABLE participation_claims DROP COLUMN IF EXISTS evidence_url;

DROP INDEX IF EXISTS idx_event_registrations_verified_at;
DROP INDEX IF EXISTS idx_event_registrations_checkin_code;
ALTER TABLE event_registrations DROP COLUMN IF EXISTS verification_method;
ALTER TABLE event_registrations DROP COLUMN IF EXISTS verified_by_user_id;
ALTER TABLE event_registrations DROP COLUMN IF EXISTS verified_at;
ALTER TABLE event_registrations DROP COLUMN IF EXISTS attended_at;
ALTER TABLE event_registrations DROP COLUMN IF EXISTS checkin_code;

DROP INDEX IF EXISTS idx_event_instances_zone_id;
ALTER TABLE event_instances DROP COLUMN IF EXISTS zone_id;
ALTER TABLE event_instances DROP COLUMN IF EXISTS description;
