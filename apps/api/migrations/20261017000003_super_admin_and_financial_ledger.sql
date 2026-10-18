-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied

-- 1. Appointment history enhancements to preserve previous holder, new holder, and effective date
ALTER TABLE appointment_history
ADD COLUMN IF NOT EXISTS previous_user_id VARCHAR(36) REFERENCES users(id),
ADD COLUMN IF NOT EXISTS new_user_id VARCHAR(36) REFERENCES users(id),
ADD COLUMN IF NOT EXISTS effective_date TIMESTAMPTZ;

-- 2. Financial Funds table for segregated fund accounting
CREATE TABLE IF NOT EXISTS financial_funds (
    id VARCHAR(50) PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    fund_type VARCHAR(30) NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO financial_funds (id, name, description, fund_type) VALUES
    ('foundation_donation_fund', 'Foundation Humanitarian Donations', 'Restricted donations allocated exclusively for charitable and foundation charter operations', 'foundation_restricted'),
    ('project_fund', 'Community Sanitation & Development Projects', 'Restricted funds allocated for local community cleanup and development projects', 'community_project'),
    ('commercial_receipt_fund', 'Commercial Receipts & Merchandise', 'Revenue generated from movement merchandise and commercial retail sales', 'commercial_operations'),
    ('general_operations_fund', 'General Operations & Logistics', 'General operational, transport, and administrative budget', 'general_clearing'),
    ('settlement_clearing_account', 'Bank Settlement Clearing Account', 'Intermediate clearing account for bank reconciliation and gateway settlement deposits', 'general_clearing')
ON CONFLICT (id) DO NOTHING;

-- 3. Financial Authorizations (Super Admin grants explicit access to other staff)
CREATE TABLE IF NOT EXISTS financial_authorizations (
    id VARCHAR(36) PRIMARY KEY,
    user_id VARCHAR(36) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    granted_by_user_id VARCHAR(36) NOT NULL REFERENCES users(id),
    access_level VARCHAR(30) NOT NULL DEFAULT 'viewer',
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    revoked_at TIMESTAMPTZ,
    version INT NOT NULL DEFAULT 1
);
CREATE INDEX IF NOT EXISTS idx_financial_authorizations_user ON financial_authorizations(user_id, status);

-- 4. Complete Fund Movement Records (Financial Transactions)
CREATE TABLE IF NOT EXISTS financial_transactions (
    id VARCHAR(36) PRIMARY KEY,
    reference VARCHAR(100) UNIQUE NOT NULL,
    idempotency_key VARCHAR(150) UNIQUE,
    transaction_type VARCHAR(50) NOT NULL,
    status VARCHAR(30) NOT NULL DEFAULT 'pending',
    amount BIGINT NOT NULL,
    currency VARCHAR(10) NOT NULL DEFAULT 'NGN',
    source_fund_id VARCHAR(50) REFERENCES financial_funds(id),
    destination_fund_id VARCHAR(50) REFERENCES financial_funds(id),
    related_campaign_id VARCHAR(36),
    related_project_id VARCHAR(36),
    state_id VARCHAR(36) REFERENCES states(id),
    zone_id VARCHAR(36) REFERENCES zones(id),
    provider_reference VARCHAR(150),
    supporting_evidence_url TEXT,
    notes TEXT,
    initiating_actor_id VARCHAR(36) REFERENCES users(id),
    initiating_actor_type VARCHAR(50) NOT NULL DEFAULT 'system',
    approved_by_user_id VARCHAR(36) REFERENCES users(id),
    approved_at TIMESTAMPTZ,
    is_external_income BOOLEAN NOT NULL DEFAULT FALSE,
    occurred_at TIMESTAMPTZ NOT NULL,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    reversal_of_transaction_id VARCHAR(36) REFERENCES financial_transactions(id),
    is_reversal BOOLEAN NOT NULL DEFAULT FALSE,
    reversal_reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    version INT NOT NULL DEFAULT 1
);
CREATE INDEX IF NOT EXISTS idx_financial_tx_occurred ON financial_transactions(occurred_at);
CREATE INDEX IF NOT EXISTS idx_financial_tx_type_status ON financial_transactions(transaction_type, status);
CREATE INDEX IF NOT EXISTS idx_financial_tx_state ON financial_transactions(state_id);
CREATE INDEX IF NOT EXISTS idx_financial_tx_zone ON financial_transactions(zone_id);
CREATE INDEX IF NOT EXISTS idx_financial_tx_source_fund ON financial_transactions(source_fund_id);
CREATE INDEX IF NOT EXISTS idx_financial_tx_dest_fund ON financial_transactions(destination_fund_id);

-- 5. Balanced Double-Entry General Ledger Entries
CREATE TABLE IF NOT EXISTS financial_ledger_entries (
    id VARCHAR(36) PRIMARY KEY,
    transaction_id VARCHAR(36) NOT NULL REFERENCES financial_transactions(id) ON DELETE RESTRICT,
    fund_id VARCHAR(50) NOT NULL REFERENCES financial_funds(id),
    entry_type VARCHAR(10) NOT NULL,
    amount BIGINT NOT NULL,
    currency VARCHAR(10) NOT NULL DEFAULT 'NGN',
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_financial_ledger_tx ON financial_ledger_entries(transaction_id);
CREATE INDEX IF NOT EXISTS idx_financial_ledger_fund ON financial_ledger_entries(fund_id, created_at);

-- 6. Financial Report & Statement Generation Audit Log
CREATE TABLE IF NOT EXISTS financial_report_logs (
    id VARCHAR(36) PRIMARY KEY,
    report_reference VARCHAR(100) UNIQUE NOT NULL,
    generated_by_user_id VARCHAR(36) NOT NULL REFERENCES users(id),
    report_type VARCHAR(50) NOT NULL,
    filters JSONB NOT NULL DEFAULT '{}'::jsonb,
    opening_balance BIGINT NOT NULL DEFAULT 0,
    closing_balance BIGINT NOT NULL DEFAULT 0,
    total_inflows BIGINT NOT NULL DEFAULT 0,
    total_outflows BIGINT NOT NULL DEFAULT 0,
    generated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_financial_report_logs_gen ON financial_report_logs(generated_by_user_id, generated_at);

-- +goose Down
-- SQL in section 'Down' is executed when this migration is rolled back

DROP TABLE IF EXISTS financial_report_logs;
DROP TABLE IF EXISTS financial_ledger_entries;
DROP TABLE IF EXISTS financial_transactions;
DROP TABLE IF EXISTS financial_authorizations;
DROP TABLE IF EXISTS financial_funds;

ALTER TABLE appointment_history
DROP COLUMN IF EXISTS effective_date,
DROP COLUMN IF EXISTS new_user_id,
DROP COLUMN IF EXISTS previous_user_id;
