-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied

-- 1. Dedicated table for single-use email verification tokens (storing SHA-256 token hashes only)
CREATE TABLE IF NOT EXISTS email_verification_tokens (
    id VARCHAR(36) PRIMARY KEY,
    user_id VARCHAR(36) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    email VARCHAR(255) NOT NULL,
    token_hash VARCHAR(64) UNIQUE NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    is_used BOOLEAN NOT NULL DEFAULT FALSE,
    used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_email_verif_tokens_user_id ON email_verification_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_email_verif_tokens_token_hash ON email_verification_tokens(token_hash);
CREATE INDEX IF NOT EXISTS idx_email_verif_tokens_email ON email_verification_tokens(email);

-- 2. Persistent rate limits and resend cooldown table (survives restarts, shared across instances)
CREATE TABLE IF NOT EXISTS email_verification_rate_limits (
    id VARCHAR(36) PRIMARY KEY,
    identifier VARCHAR(255) NOT NULL,
    action VARCHAR(50) NOT NULL,
    request_count INT NOT NULL DEFAULT 1,
    first_requested_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_requested_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_email_verif_rate_limits_id_action ON email_verification_rate_limits(identifier, action);

-- +goose Down
-- SQL in section 'Down' is executed when this migration is rolled back

DROP TABLE IF EXISTS email_verification_rate_limits;
DROP TABLE IF EXISTS email_verification_tokens;
