-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied

CREATE TABLE IF NOT EXISTS marketplace_refunds (
    id VARCHAR(36) PRIMARY KEY,
    order_id VARCHAR(36) NOT NULL REFERENCES marketplace_orders(id) ON DELETE RESTRICT,
    payment_id VARCHAR(36) NOT NULL REFERENCES marketplace_payments(id) ON DELETE RESTRICT,
    amount_kobo BIGINT NOT NULL CHECK (amount_kobo > 0),
    status VARCHAR(50) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'successful', 'failed')),
    provider VARCHAR(50) NOT NULL DEFAULT 'paystack',
    provider_refund_reference VARCHAR(255) NOT NULL DEFAULT '',
    initiated_by_user_id VARCHAR(36) NOT NULL REFERENCES users(id),
    confirmed_by_user_id VARCHAR(36) REFERENCES users(id),
    reason TEXT NOT NULL DEFAULT '',
    settlement_notes TEXT NOT NULL DEFAULT '',
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_marketplace_refunds_order ON marketplace_refunds(order_id);
CREATE INDEX IF NOT EXISTS idx_marketplace_refunds_payment ON marketplace_refunds(payment_id);
CREATE INDEX IF NOT EXISTS idx_marketplace_refunds_status ON marketplace_refunds(status);
CREATE INDEX IF NOT EXISTS idx_marketplace_refunds_provider_ref ON marketplace_refunds(provider_refund_reference);
CREATE INDEX IF NOT EXISTS idx_marketplace_refunds_created ON marketplace_refunds(created_at DESC);

-- +goose Down
-- SQL in section 'Down' is executed when this migration is rolled back

DROP TABLE IF EXISTS marketplace_refunds;
