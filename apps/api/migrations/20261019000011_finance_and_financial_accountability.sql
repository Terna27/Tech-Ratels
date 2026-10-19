-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied

-- 1. Extend financial_transactions for granular Income & Governance Attribution
ALTER TABLE financial_transactions
ADD COLUMN IF NOT EXISTS income_category VARCHAR(50),
ADD COLUMN IF NOT EXISTS payment_method VARCHAR(50),
ADD COLUMN IF NOT EXISTS funding_source VARCHAR(150),
ADD COLUMN IF NOT EXISTS lga_id VARCHAR(36) REFERENCES lgas(id),
ADD COLUMN IF NOT EXISTS receiving_account_id VARCHAR(36),
ADD COLUMN IF NOT EXISTS verification_status VARCHAR(30) NOT NULL DEFAULT 'unverified',
ADD COLUMN IF NOT EXISTS verified_by_user_id VARCHAR(36) REFERENCES users(id),
ADD COLUMN IF NOT EXISTS verified_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_financial_tx_verification ON financial_transactions(verification_status);
CREATE INDEX IF NOT EXISTS idx_financial_tx_income_category ON financial_transactions(income_category);
CREATE INDEX IF NOT EXISTS idx_financial_tx_lga ON financial_transactions(lga_id);

-- 2. Official Financial Bank Accounts (Governed strictly by Super Admin)
CREATE TABLE IF NOT EXISTS financial_bank_accounts (
    id VARCHAR(36) PRIMARY KEY,
    bank_name VARCHAR(100) NOT NULL,
    account_name VARCHAR(150) NOT NULL,
    account_number VARCHAR(50) NOT NULL,
    currency VARCHAR(10) NOT NULL DEFAULT 'NGN',
    org_type VARCHAR(50) NOT NULL DEFAULT 'ratels' CHECK (org_type IN ('ratels', 'foundation')),
    status VARCHAR(30) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'pending_verification', 'inactive')),
    is_primary BOOLEAN NOT NULL DEFAULT FALSE,
    notes TEXT,
    created_by_user_id VARCHAR(36) REFERENCES users(id),
    activated_by_user_id VARCHAR(36) REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    version INT NOT NULL DEFAULT 1
);
CREATE INDEX IF NOT EXISTS idx_financial_bank_accounts_org ON financial_bank_accounts(org_type, status);

-- Seed initial official accounts for Ratels & Foundation (provisional/operational)
INSERT INTO financial_bank_accounts (id, bank_name, account_name, account_number, currency, org_type, status, is_primary, notes) VALUES
    ('bank-ratels-primary', 'Zenith Bank PLC', 'Nigerian Ratels Operational Account', '1012345678', 'NGN', 'ratels', 'active', TRUE, 'Primary operational clearing account for Ratels Movement'),
    ('bank-foundation-primary', 'Guaranty Trust Bank (GTBank)', 'Martins Vincent Otse Foundation Account', '0123456789', 'NGN', 'foundation', 'active', TRUE, 'Dedicated humanitarian and charitable foundation account')
ON CONFLICT (id) DO NOTHING;

-- 3. Structured Financial Budgets and Revision Control
CREATE TABLE IF NOT EXISTS financial_budgets (
    id VARCHAR(36) PRIMARY KEY,
    reference VARCHAR(100) UNIQUE NOT NULL,
    title VARCHAR(255) NOT NULL,
    scope_type VARCHAR(30) NOT NULL CHECK (scope_type IN ('national', 'zone', 'state', 'lga', 'community_project', 'cleanup_campaign', 'foundation')),
    scope_id VARCHAR(36),
    zone_id VARCHAR(36) REFERENCES zones(id),
    state_id VARCHAR(36) REFERENCES states(id),
    lga_id VARCHAR(36) REFERENCES lgas(id),
    period_type VARCHAR(30) NOT NULL DEFAULT 'annual' CHECK (period_type IN ('quarterly', 'annual', 'project_based', 'event_based')),
    period_label VARCHAR(50) NOT NULL,
    period_start DATE NOT NULL,
    period_end DATE NOT NULL,
    proposed_amount_kobo BIGINT NOT NULL DEFAULT 0 CHECK (proposed_amount_kobo >= 0),
    approved_amount_kobo BIGINT NOT NULL DEFAULT 0 CHECK (approved_amount_kobo >= 0),
    allocated_amount_kobo BIGINT NOT NULL DEFAULT 0 CHECK (allocated_amount_kobo >= 0),
    committed_amount_kobo BIGINT NOT NULL DEFAULT 0 CHECK (committed_amount_kobo >= 0),
    spent_amount_kobo BIGINT NOT NULL DEFAULT 0 CHECK (spent_amount_kobo >= 0),
    currency VARCHAR(10) NOT NULL DEFAULT 'NGN',
    status VARCHAR(30) NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'submitted', 'under_review', 'approved', 'active', 'rejected', 'closed')),
    created_by_user_id VARCHAR(36) NOT NULL REFERENCES users(id),
    approved_by_user_id VARCHAR(36) REFERENCES users(id),
    approved_at TIMESTAMPTZ,
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    version INT NOT NULL DEFAULT 1
);
CREATE INDEX IF NOT EXISTS idx_financial_budgets_scope ON financial_budgets(scope_type, scope_id);
CREATE INDEX IF NOT EXISTS idx_financial_budgets_status ON financial_budgets(status);
CREATE INDEX IF NOT EXISTS idx_financial_budgets_period ON financial_budgets(period_label);

CREATE TABLE IF NOT EXISTS financial_budget_revisions (
    id VARCHAR(36) PRIMARY KEY,
    budget_id VARCHAR(36) NOT NULL REFERENCES financial_budgets(id) ON DELETE CASCADE,
    revision_number INT NOT NULL DEFAULT 1,
    previous_amount_kobo BIGINT NOT NULL,
    revised_amount_kobo BIGINT NOT NULL,
    reason TEXT NOT NULL,
    requested_by_user_id VARCHAR(36) NOT NULL REFERENCES users(id),
    approved_by_user_id VARCHAR(36) REFERENCES users(id),
    approved_at TIMESTAMPTZ,
    status VARCHAR(30) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'rejected')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_budget_revisions_budget ON financial_budget_revisions(budget_id);

-- 4. Structured Expense Requests and Governance Workflow
CREATE TABLE IF NOT EXISTS financial_expenses (
    id VARCHAR(36) PRIMARY KEY,
    reference VARCHAR(100) UNIQUE NOT NULL,
    title VARCHAR(255) NOT NULL,
    category VARCHAR(50) NOT NULL CHECK (category IN ('cleanup_materials', 'waste_collection_logistics', 'transportation', 'community_projects', 'administrative_operations', 'equipment', 'event_expenses', 'emergency', 'other')),
    description TEXT NOT NULL,
    amount_kobo BIGINT NOT NULL CHECK (amount_kobo > 0),
    currency VARCHAR(10) NOT NULL DEFAULT 'NGN',
    beneficiary_name VARCHAR(150) NOT NULL,
    beneficiary_bank_name VARCHAR(100),
    beneficiary_account_number VARCHAR(50),
    zone_id VARCHAR(36) REFERENCES zones(id),
    state_id VARCHAR(36) REFERENCES states(id),
    lga_id VARCHAR(36) REFERENCES lgas(id),
    related_project_id VARCHAR(36) REFERENCES community_projects(id),
    related_campaign_id VARCHAR(36) REFERENCES cleanup_campaigns(id),
    supporting_document_url TEXT,
    receipt_url TEXT,
    requested_by_user_id VARCHAR(36) NOT NULL REFERENCES users(id),
    reviewer_id VARCHAR(36) REFERENCES users(id),
    reviewed_at TIMESTAMPTZ,
    approved_by_user_id VARCHAR(36) REFERENCES users(id),
    approved_at TIMESTAMPTZ,
    rejection_reason TEXT,
    authorized_by_user_id VARCHAR(36) REFERENCES users(id),
    authorized_at TIMESTAMPTZ,
    paid_by_user_id VARCHAR(36) REFERENCES users(id),
    paid_at TIMESTAMPTZ,
    payment_reference VARCHAR(100),
    transaction_id VARCHAR(36) REFERENCES financial_transactions(id),
    budget_id VARCHAR(36) REFERENCES financial_budgets(id),
    fund_id VARCHAR(50) REFERENCES financial_funds(id),
    status VARCHAR(30) NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'submitted', 'under_review', 'approved', 'rejected', 'authorized_for_payment', 'paid', 'reconciled', 'cancelled')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    version INT NOT NULL DEFAULT 1
);
CREATE INDEX IF NOT EXISTS idx_financial_expenses_status ON financial_expenses(status);
CREATE INDEX IF NOT EXISTS idx_financial_expenses_requester ON financial_expenses(requested_by_user_id);
CREATE INDEX IF NOT EXISTS idx_financial_expenses_state ON financial_expenses(state_id);
CREATE INDEX IF NOT EXISTS idx_financial_expenses_zone ON financial_expenses(zone_id);
CREATE INDEX IF NOT EXISTS idx_financial_expenses_category ON financial_expenses(category);
CREATE INDEX IF NOT EXISTS idx_financial_expenses_created ON financial_expenses(created_at DESC);

-- 5. Fund Allocations and Internal Transfers
CREATE TABLE IF NOT EXISTS financial_allocations (
    id VARCHAR(36) PRIMARY KEY,
    reference VARCHAR(100) UNIQUE NOT NULL,
    source_type VARCHAR(30) NOT NULL CHECK (source_type IN ('national', 'zone', 'state')),
    source_id VARCHAR(36),
    destination_type VARCHAR(30) NOT NULL CHECK (destination_type IN ('zone', 'state', 'lga', 'community_project')),
    destination_id VARCHAR(36),
    source_fund_id VARCHAR(50) NOT NULL REFERENCES financial_funds(id),
    destination_fund_id VARCHAR(50) NOT NULL REFERENCES financial_funds(id),
    amount_kobo BIGINT NOT NULL CHECK (amount_kobo > 0),
    currency VARCHAR(10) NOT NULL DEFAULT 'NGN',
    purpose TEXT NOT NULL,
    approval_reference VARCHAR(100),
    transaction_id VARCHAR(36) REFERENCES financial_transactions(id),
    allocated_by_user_id VARCHAR(36) NOT NULL REFERENCES users(id),
    status VARCHAR(30) NOT NULL DEFAULT 'completed' CHECK (status IN ('pending', 'completed', 'reversed', 'cancelled')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_financial_allocations_dest ON financial_allocations(destination_type, destination_id);
CREATE INDEX IF NOT EXISTS idx_financial_allocations_created ON financial_allocations(created_at DESC);

-- 6. Donations Management (Nigerian Ratels & Foundation Segregation)
CREATE TABLE IF NOT EXISTS financial_donations (
    id VARCHAR(36) PRIMARY KEY,
    reference VARCHAR(100) UNIQUE NOT NULL,
    entity_type VARCHAR(50) NOT NULL DEFAULT 'ratels' CHECK (entity_type IN ('ratels', 'foundation')),
    campaign_id VARCHAR(36) REFERENCES foundation_campaigns(id),
    donor_name VARCHAR(150) NOT NULL,
    donor_email VARCHAR(255) NOT NULL,
    donor_phone VARCHAR(50),
    is_anonymous BOOLEAN NOT NULL DEFAULT FALSE,
    amount_kobo BIGINT NOT NULL CHECK (amount_kobo > 0),
    currency VARCHAR(10) NOT NULL DEFAULT 'NGN',
    payment_method VARCHAR(50) NOT NULL DEFAULT 'paystack',
    payment_reference VARCHAR(150) UNIQUE,
    provider_channel VARCHAR(50),
    status VARCHAR(30) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'verified', 'failed', 'refunded')),
    zone_id VARCHAR(36) REFERENCES zones(id),
    state_id VARCHAR(36) REFERENCES states(id),
    destination_fund_id VARCHAR(50) NOT NULL REFERENCES financial_funds(id),
    transaction_id VARCHAR(36) REFERENCES financial_transactions(id),
    verified_at TIMESTAMPTZ,
    receipt_number VARCHAR(100) UNIQUE,
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_financial_donations_entity ON financial_donations(entity_type, status);
CREATE INDEX IF NOT EXISTS idx_financial_donations_campaign ON financial_donations(campaign_id);
CREATE INDEX IF NOT EXISTS idx_financial_donations_status ON financial_donations(status);
CREATE INDEX IF NOT EXISTS idx_financial_donations_created ON financial_donations(created_at DESC);

-- 7. Bank Account Reconciliation Periods and Imported Statement Items
CREATE TABLE IF NOT EXISTS financial_reconciliation_periods (
    id VARCHAR(36) PRIMARY KEY,
    bank_account_id VARCHAR(36) NOT NULL REFERENCES financial_bank_accounts(id),
    period_start DATE NOT NULL,
    period_end DATE NOT NULL,
    opening_balance_kobo BIGINT NOT NULL DEFAULT 0,
    closing_balance_kobo BIGINT NOT NULL DEFAULT 0,
    statement_closing_balance_kobo BIGINT NOT NULL DEFAULT 0,
    status VARCHAR(30) NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'in_progress', 'reconciled', 'certified')),
    reconciled_by_user_id VARCHAR(36) REFERENCES users(id),
    reconciled_at TIMESTAMPTZ,
    certified_by_user_id VARCHAR(36) REFERENCES users(id),
    certified_at TIMESTAMPTZ,
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_reconciliation_periods_account ON financial_reconciliation_periods(bank_account_id, period_start);
CREATE INDEX IF NOT EXISTS idx_reconciliation_periods_status ON financial_reconciliation_periods(status);

CREATE TABLE IF NOT EXISTS financial_statement_items (
    id VARCHAR(36) PRIMARY KEY,
    reconciliation_period_id VARCHAR(36) NOT NULL REFERENCES financial_reconciliation_periods(id) ON DELETE CASCADE,
    statement_reference VARCHAR(100) NOT NULL,
    transaction_date DATE NOT NULL,
    value_date DATE,
    description TEXT NOT NULL,
    amount_kobo BIGINT NOT NULL CHECK (amount_kobo > 0),
    entry_type VARCHAR(10) NOT NULL CHECK (entry_type IN ('credit', 'debit')),
    matched_transaction_id VARCHAR(36) REFERENCES financial_transactions(id),
    status VARCHAR(30) NOT NULL DEFAULT 'unmatched' CHECK (status IN ('unmatched', 'matched', 'flagged', 'manually_reconciled')),
    match_notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_statement_item_ref UNIQUE (reconciliation_period_id, statement_reference)
);
CREATE INDEX IF NOT EXISTS idx_statement_items_period ON financial_statement_items(reconciliation_period_id, status);
CREATE INDEX IF NOT EXISTS idx_statement_items_matched_tx ON financial_statement_items(matched_transaction_id);

-- +goose Down
-- SQL in section 'Down' is executed when this migration is rolled back

DROP TABLE IF EXISTS financial_statement_items CASCADE;
DROP TABLE IF EXISTS financial_reconciliation_periods CASCADE;
DROP TABLE IF EXISTS financial_donations CASCADE;
DROP TABLE IF EXISTS financial_allocations CASCADE;
DROP TABLE IF EXISTS financial_expenses CASCADE;
DROP TABLE IF EXISTS financial_budget_revisions CASCADE;
DROP TABLE IF EXISTS financial_budgets CASCADE;
DROP TABLE IF EXISTS financial_bank_accounts CASCADE;

ALTER TABLE financial_transactions
DROP COLUMN IF EXISTS verified_at,
DROP COLUMN IF EXISTS verified_by_user_id,
DROP COLUMN IF EXISTS verification_status,
DROP COLUMN IF EXISTS receiving_account_id,
DROP COLUMN IF EXISTS lga_id,
DROP COLUMN IF EXISTS funding_source,
DROP COLUMN IF EXISTS payment_method,
DROP COLUMN IF EXISTS income_category;
