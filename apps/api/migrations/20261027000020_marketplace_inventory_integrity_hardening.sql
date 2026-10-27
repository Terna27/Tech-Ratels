-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied

-- 1. Backfill stock_released = TRUE for historical cancelled and refunded orders
-- In pre-Step 63.4 code, cancelled and refunded orders already had their stock restored to product inventory.
UPDATE marketplace_orders
SET stock_released = TRUE
WHERE stock_released = FALSE
  AND order_status IN ('cancelled', 'refunded');

-- 2. Backfill reservation_expires_at for historical pending orders
-- Stale unfulfilled orders created before reservation expiry was introduced can now be evaluated by the expiry worker.
UPDATE marketplace_orders
SET reservation_expires_at = created_at + INTERVAL '30 minutes'
WHERE reservation_expires_at IS NULL
  AND order_status = 'placed'
  AND payment_status = 'pending';

-- 3. Seed opening inventory ledger entries for existing products
-- Ensures immutable audit trail continuity for all catalogue products created prior to ledger introduction.
INSERT INTO marketplace_inventory_logs (
    id, product_id, business_id, actor_user_id, action_type,
    quantity_delta, balance_after, order_id, reference_id, reason, idempotency_key, created_at
)
SELECT
    gen_random_uuid()::text,
    p.id,
    p.business_id,
    NULL,
    'opening_balance',
    p.stock_quantity,
    p.stock_quantity,
    NULL,
    'INIT',
    'Initial opening inventory balance snapshot',
    'opening_balance:' || p.id,
    CURRENT_TIMESTAMP
FROM marketplace_products p
WHERE NOT EXISTS (
    SELECT 1 FROM marketplace_inventory_logs l WHERE l.product_id = p.id
);

-- 4. Update reservation expiry index to include both pending and failed payments
DROP INDEX IF EXISTS idx_mkt_orders_reservation_expiry;
CREATE INDEX IF NOT EXISTS idx_mkt_orders_reservation_expiry
    ON marketplace_orders(payment_status, reservation_expires_at)
    WHERE payment_status IN ('pending', 'failed') AND stock_released = FALSE;

-- +goose Down
-- SQL in section 'Down' is executed when this migration is rolled back

-- Safe rollback guard:
-- Rollback must FAIL explicitly if any order exists with order_status = 'requires_reconciliation'
-- or reconciliation_required = TRUE.
-- DO NOT silently rewrite paid/reconciliation orders to cancelled!
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM marketplace_orders
        WHERE order_status = 'requires_reconciliation' OR reconciliation_required = TRUE
    ) THEN
        RAISE EXCEPTION 'Cannot roll back migration 20: Orders exist with requires_reconciliation status. Recovery procedure: All orders in requires_reconciliation must first be resolved administratively by manual verification and refund/fulfilment before downgrading schema.';
    END IF;
END $$;

-- Restore index to pending-only filter
DROP INDEX IF EXISTS idx_mkt_orders_reservation_expiry;
CREATE INDEX IF NOT EXISTS idx_mkt_orders_reservation_expiry
    ON marketplace_orders(payment_status, reservation_expires_at)
    WHERE payment_status = 'pending' AND stock_released = FALSE;

-- Delete opening balance ledger entries created by this migration
DELETE FROM marketplace_inventory_logs
WHERE action_type = 'opening_balance' AND reference_id = 'INIT';
