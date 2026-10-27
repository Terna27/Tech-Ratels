-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied

-- 1. Attribution columns on marketplace_refunds for multi-vendor item and consignment isolation
ALTER TABLE marketplace_refunds
    ADD COLUMN IF NOT EXISTS shipment_id VARCHAR(36) REFERENCES marketplace_shipments(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS business_id VARCHAR(36) REFERENCES marketplace_businesses(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS order_item_id VARCHAR(36) REFERENCES marketplace_order_items(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_marketplace_refunds_shipment ON marketplace_refunds(shipment_id);
CREATE INDEX IF NOT EXISTS idx_marketplace_refunds_business ON marketplace_refunds(business_id);
CREATE INDEX IF NOT EXISTS idx_marketplace_refunds_order_item ON marketplace_refunds(order_item_id);

-- 2. Add refund tracking columns to marketplace_shipment_items
ALTER TABLE marketplace_shipment_items
    ADD COLUMN IF NOT EXISTS refunded_quantity INT NOT NULL DEFAULT 0 CHECK (refunded_quantity >= 0 AND refunded_quantity <= quantity),
    ADD COLUMN IF NOT EXISTS refund_id VARCHAR(36) REFERENCES marketplace_refunds(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_mkt_shipment_items_refund ON marketplace_shipment_items(refund_id);

-- 3. Backfill any remaining unassigned business_id on marketplace_shipments from order items
UPDATE marketplace_shipments s
SET business_id = (
    SELECT oi.business_id FROM marketplace_order_items oi
    WHERE oi.order_id = s.order_id
    LIMIT 1
)
WHERE s.business_id IS NULL;

-- 4. Harden unique index on marketplace_shipments to treat NULLs as NOT DISTINCT (PG15+)
DROP INDEX IF EXISTS uq_idx_mkt_shipments_order_business;
CREATE UNIQUE INDEX IF NOT EXISTS uq_idx_mkt_shipments_order_business
    ON marketplace_shipments(order_id, business_id) NULLS NOT DISTINCT;

-- +goose Down
-- SQL in section 'Down' is executed when this migration is rolled back

DROP INDEX IF EXISTS uq_idx_mkt_shipments_order_business;
CREATE UNIQUE INDEX IF NOT EXISTS uq_idx_mkt_shipments_order_business
    ON marketplace_shipments(order_id, business_id);

DROP INDEX IF EXISTS idx_mkt_shipment_items_refund;
ALTER TABLE marketplace_shipment_items DROP COLUMN IF EXISTS refund_id;
ALTER TABLE marketplace_shipment_items DROP COLUMN IF EXISTS refunded_quantity;

DROP INDEX IF EXISTS idx_marketplace_refunds_order_item;
DROP INDEX IF EXISTS idx_marketplace_refunds_business;
DROP INDEX IF EXISTS idx_marketplace_refunds_shipment;

ALTER TABLE marketplace_refunds DROP COLUMN IF EXISTS order_item_id;
ALTER TABLE marketplace_refunds DROP COLUMN IF EXISTS business_id;
ALTER TABLE marketplace_refunds DROP COLUMN IF EXISTS shipment_id;
