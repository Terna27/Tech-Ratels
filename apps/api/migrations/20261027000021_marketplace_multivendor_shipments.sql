-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied

-- 1. Update marketplace_orders order_status and fulfilment_status check constraints
-- Supports partial fulfillment status aggregation for multi-vendor orders.
ALTER TABLE marketplace_orders DROP CONSTRAINT IF EXISTS marketplace_orders_order_status_check;
ALTER TABLE marketplace_orders ADD CONSTRAINT marketplace_orders_order_status_check
    CHECK (order_status IN ('placed', 'processing', 'partially_dispatched', 'ready_for_pickup', 'dispatched', 'partially_delivered', 'delivered', 'cancelled', 'refunded', 'requires_reconciliation'));

ALTER TABLE marketplace_orders DROP CONSTRAINT IF EXISTS marketplace_orders_fulfilment_status_check;
ALTER TABLE marketplace_orders ADD CONSTRAINT marketplace_orders_fulfilment_status_check
    CHECK (fulfilment_status IN ('unfulfilled', 'preparing', 'partially_dispatched', 'dispatched', 'partially_delivered', 'delivered', 'returned'));

-- 2. Enhance marketplace_shipments table for multi-vendor support
ALTER TABLE marketplace_shipments
    ADD COLUMN IF NOT EXISTS business_id VARCHAR(36) REFERENCES marketplace_businesses(id) ON DELETE CASCADE;

ALTER TABLE marketplace_shipments ALTER COLUMN carrier_name SET DEFAULT '';
ALTER TABLE marketplace_shipments ALTER COLUMN carrier_name SET NOT NULL;
ALTER TABLE marketplace_shipments ALTER COLUMN tracking_number SET DEFAULT '';
ALTER TABLE marketplace_shipments ALTER COLUMN tracking_number SET NOT NULL;
ALTER TABLE marketplace_shipments ALTER COLUMN dispatched_at DROP NOT NULL;
ALTER TABLE marketplace_shipments ALTER COLUMN dispatched_at DROP DEFAULT;
ALTER TABLE marketplace_shipments ALTER COLUMN status SET DEFAULT 'pending';

-- Update shipment status check constraint to support pending, processing, and cancelled states
ALTER TABLE marketplace_shipments DROP CONSTRAINT IF EXISTS marketplace_shipments_status_check;
ALTER TABLE marketplace_shipments ADD CONSTRAINT marketplace_shipments_status_check
    CHECK (status IN ('pending', 'processing', 'ready_for_pickup', 'dispatched', 'in_transit', 'delivered', 'failed', 'returned', 'cancelled'));

-- 3. Dedicated shipment-items relationship table
CREATE TABLE IF NOT EXISTS marketplace_shipment_items (
    id VARCHAR(36) PRIMARY KEY,
    shipment_id VARCHAR(36) NOT NULL REFERENCES marketplace_shipments(id) ON DELETE CASCADE,
    order_item_id VARCHAR(36) NOT NULL REFERENCES marketplace_order_items(id) ON DELETE CASCADE,
    quantity INT NOT NULL CHECK (quantity > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_mkt_shipment_order_item UNIQUE (shipment_id, order_item_id)
);

CREATE INDEX IF NOT EXISTS idx_mkt_shipment_items_shipment ON marketplace_shipment_items(shipment_id);
CREATE INDEX IF NOT EXISTS idx_mkt_shipment_items_order_item ON marketplace_shipment_items(order_item_id);

-- Add optional direct shipment_id link on marketplace_order_items for query convenience
ALTER TABLE marketplace_order_items
    ADD COLUMN IF NOT EXISTS shipment_id VARCHAR(36) REFERENCES marketplace_shipments(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_mkt_order_items_shipment ON marketplace_order_items(shipment_id);

-- 4. Backfill business_id for historical single-vendor shipments
UPDATE marketplace_shipments s
SET business_id = (
    SELECT oi.business_id FROM marketplace_order_items oi
    WHERE oi.order_id = s.order_id
    LIMIT 1
)
WHERE s.business_id IS NULL;

-- Create unique index to enforce one shipment per seller per order
CREATE UNIQUE INDEX IF NOT EXISTS uq_idx_mkt_shipments_order_business
    ON marketplace_shipments(order_id, business_id);

CREATE INDEX IF NOT EXISTS idx_mkt_shipments_business ON marketplace_shipments(business_id);
CREATE INDEX IF NOT EXISTS idx_mkt_shipments_status ON marketplace_shipments(status);

-- 5. Backfill historical shipments into marketplace_shipment_items and update marketplace_order_items.shipment_id
UPDATE marketplace_order_items oi
SET shipment_id = s.id
FROM marketplace_shipments s
WHERE oi.order_id = s.order_id
  AND oi.shipment_id IS NULL
  AND (s.business_id IS NULL OR oi.business_id = s.business_id);

INSERT INTO marketplace_shipment_items (id, shipment_id, order_item_id, quantity, created_at)
SELECT
    gen_random_uuid()::text,
    s.id,
    oi.id,
    oi.quantity,
    s.created_at
FROM marketplace_shipments s
JOIN marketplace_order_items oi ON oi.order_id = s.order_id AND (s.business_id IS NULL OR oi.business_id = s.business_id)
WHERE NOT EXISTS (
    SELECT 1 FROM marketplace_shipment_items msi WHERE msi.shipment_id = s.id AND msi.order_item_id = oi.id
);

-- 6. Backfill missing shipments for historical paid orders (idempotent creation)
INSERT INTO marketplace_shipments (
    id, order_id, business_id, carrier_name, tracking_number,
    tracking_url, status, dispatched_at, estimated_delivery_at,
    delivered_at, delivery_notes, proof_of_delivery_url, created_at, updated_at
)
SELECT
    gen_random_uuid()::text,
    oi_grouped.order_id,
    oi_grouped.business_id,
    '',
    '',
    '',
    CASE
        WHEN o.order_status = 'delivered' OR o.fulfilment_status = 'delivered' THEN 'delivered'
        WHEN o.order_status = 'dispatched' OR o.fulfilment_status = 'dispatched' THEN 'dispatched'
        ELSE 'pending'
    END,
    CASE
        WHEN o.order_status IN ('dispatched', 'delivered') OR o.fulfilment_status IN ('dispatched', 'delivered') THEN o.updated_at
        ELSE NULL
    END,
    NULL,
    CASE
        WHEN o.order_status = 'delivered' OR o.fulfilment_status = 'delivered' THEN o.updated_at
        ELSE NULL
    END,
    '',
    '',
    o.created_at,
    o.updated_at
FROM (
    SELECT DISTINCT order_id, business_id
    FROM marketplace_order_items
) oi_grouped
JOIN marketplace_orders o ON o.id = oi_grouped.order_id
WHERE o.payment_status = 'paid'
  AND o.order_status NOT IN ('cancelled', 'refunded', 'requires_reconciliation')
  AND o.stock_released = FALSE
  AND NOT EXISTS (
      SELECT 1 FROM marketplace_shipments s
      WHERE s.order_id = oi_grouped.order_id AND s.business_id = oi_grouped.business_id
  );

-- Link backfilled shipments to items
INSERT INTO marketplace_shipment_items (id, shipment_id, order_item_id, quantity, created_at)
SELECT
    gen_random_uuid()::text,
    s.id,
    oi.id,
    oi.quantity,
    s.created_at
FROM marketplace_shipments s
JOIN marketplace_order_items oi ON oi.order_id = s.order_id AND oi.business_id = s.business_id
WHERE NOT EXISTS (
    SELECT 1 FROM marketplace_shipment_items msi WHERE msi.shipment_id = s.id AND msi.order_item_id = oi.id
);

UPDATE marketplace_order_items oi
SET shipment_id = s.id
FROM marketplace_shipments s
WHERE oi.order_id = s.order_id
  AND oi.business_id = s.business_id
  AND oi.shipment_id IS NULL;


-- +goose Down
-- SQL in section 'Down' is executed when this migration is rolled back

-- Rollback guard: fail explicitly if multiple shipments exist per order
DO $$
BEGIN
    IF EXISTS (
        SELECT order_id FROM marketplace_shipments
        GROUP BY order_id HAVING count(*) > 1
    ) THEN
        RAISE EXCEPTION 'Cannot roll back migration 21: Orders exist with multiple vendor shipments. Downgrading schema would cause data loss.';
    END IF;
END $$;

ALTER TABLE marketplace_order_items DROP COLUMN IF EXISTS shipment_id;
DROP TABLE IF EXISTS marketplace_shipment_items CASCADE;
DROP INDEX IF EXISTS uq_idx_mkt_shipments_order_business;
DROP INDEX IF EXISTS idx_mkt_shipments_business;
DROP INDEX IF EXISTS idx_mkt_shipments_status;
ALTER TABLE marketplace_shipments DROP COLUMN IF EXISTS business_id;

ALTER TABLE marketplace_orders DROP CONSTRAINT IF EXISTS marketplace_orders_order_status_check;
ALTER TABLE marketplace_orders ADD CONSTRAINT marketplace_orders_order_status_check
    CHECK (order_status IN ('placed', 'processing', 'ready_for_pickup', 'dispatched', 'delivered', 'cancelled', 'refunded', 'requires_reconciliation'));

ALTER TABLE marketplace_orders DROP CONSTRAINT IF EXISTS marketplace_orders_fulfilment_status_check;
ALTER TABLE marketplace_orders ADD CONSTRAINT marketplace_orders_fulfilment_status_check
    CHECK (fulfilment_status IN ('unfulfilled', 'preparing', 'dispatched', 'delivered', 'returned'));
