-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied

-- 1. Immutable Inventory Movement & Audit Ledger
CREATE TABLE IF NOT EXISTS marketplace_inventory_logs (
    id VARCHAR(36) PRIMARY KEY,
    product_id VARCHAR(36) NOT NULL REFERENCES marketplace_products(id) ON DELETE RESTRICT,
    business_id VARCHAR(36) NOT NULL REFERENCES marketplace_businesses(id) ON DELETE RESTRICT,
    actor_user_id VARCHAR(36) REFERENCES users(id) ON DELETE SET NULL,
    action_type VARCHAR(50) NOT NULL,
    quantity_delta INT NOT NULL,
    balance_after INT NOT NULL CHECK (balance_after >= 0),
    order_id VARCHAR(36) REFERENCES marketplace_orders(id) ON DELETE SET NULL,
    reference_id VARCHAR(100) NOT NULL DEFAULT '',
    reason TEXT NOT NULL DEFAULT '',
    idempotency_key VARCHAR(128) UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_mkt_inventory_logs_product ON marketplace_inventory_logs(product_id);
CREATE INDEX IF NOT EXISTS idx_mkt_inventory_logs_business ON marketplace_inventory_logs(business_id);
CREATE INDEX IF NOT EXISTS idx_mkt_inventory_logs_order ON marketplace_inventory_logs(order_id);
CREATE INDEX IF NOT EXISTS idx_mkt_inventory_logs_action ON marketplace_inventory_logs(action_type);
CREATE INDEX IF NOT EXISTS idx_mkt_inventory_logs_idempotency ON marketplace_inventory_logs(idempotency_key);
CREATE INDEX IF NOT EXISTS idx_mkt_inventory_logs_created ON marketplace_inventory_logs(created_at DESC);

-- 2. Marketplace Orders Expiry, Cancellation Reason & Reconciliation Fields
ALTER TABLE marketplace_orders 
    ADD COLUMN IF NOT EXISTS reservation_expires_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS cancellation_reason VARCHAR(255) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS stock_released BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS reconciliation_required BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS reconciliation_notes TEXT NOT NULL DEFAULT '';

-- Update order_status check constraint to support requires_reconciliation state
ALTER TABLE marketplace_orders DROP CONSTRAINT IF EXISTS marketplace_orders_order_status_check;
ALTER TABLE marketplace_orders ADD CONSTRAINT marketplace_orders_order_status_check 
    CHECK (order_status IN ('placed', 'processing', 'ready_for_pickup', 'dispatched', 'delivered', 'cancelled', 'refunded', 'requires_reconciliation'));

CREATE INDEX IF NOT EXISTS idx_mkt_orders_reservation_expiry 
    ON marketplace_orders(payment_status, reservation_expires_at) 
    WHERE payment_status = 'pending' AND stock_released = FALSE;

CREATE INDEX IF NOT EXISTS idx_mkt_orders_reconciliation 
    ON marketplace_orders(reconciliation_required) 
    WHERE reconciliation_required = TRUE;

-- +goose Down
-- SQL in section 'Down' is executed when this migration is rolled back

DROP TABLE IF EXISTS marketplace_inventory_logs;
ALTER TABLE marketplace_orders DROP COLUMN IF EXISTS reservation_expires_at;
ALTER TABLE marketplace_orders DROP COLUMN IF EXISTS cancellation_reason;
ALTER TABLE marketplace_orders DROP COLUMN IF EXISTS stock_released;
ALTER TABLE marketplace_orders DROP COLUMN IF EXISTS reconciliation_required;
ALTER TABLE marketplace_orders DROP COLUMN IF EXISTS reconciliation_notes;
ALTER TABLE marketplace_orders DROP CONSTRAINT IF EXISTS marketplace_orders_order_status_check;
ALTER TABLE marketplace_orders ADD CONSTRAINT marketplace_orders_order_status_check 
    CHECK (order_status IN ('placed', 'processing', 'ready_for_pickup', 'dispatched', 'delivered', 'cancelled', 'refunded'));
