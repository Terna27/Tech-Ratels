-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied

ALTER TABLE verified_bank_account_versions
ADD COLUMN IF NOT EXISTS submitted_by_user_id VARCHAR(36) REFERENCES users(id),
ADD COLUMN IF NOT EXISTS submitted_by_officer VARCHAR(100);

CREATE INDEX IF NOT EXISTS idx_appointments_active_lookup
ON appointments(user_id, status, end_date);

-- +goose Down
-- SQL in section 'Down' is executed when this migration is rolled back

DROP INDEX IF EXISTS idx_appointments_active_lookup;

ALTER TABLE verified_bank_account_versions
DROP COLUMN IF EXISTS submitted_by_officer,
DROP COLUMN IF EXISTS submitted_by_user_id;
