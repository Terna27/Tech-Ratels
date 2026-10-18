-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied

-- Allow 'partially_refunded' status for marketplace order payments and payment transaction records
ALTER TABLE marketplace_orders DROP CONSTRAINT IF EXISTS marketplace_orders_payment_status_check;
ALTER TABLE marketplace_orders ADD CONSTRAINT marketplace_orders_payment_status_check 
    CHECK (payment_status IN ('pending', 'paid', 'failed', 'partially_refunded', 'refunded'));

ALTER TABLE marketplace_payments DROP CONSTRAINT IF EXISTS marketplace_payments_status_check;
ALTER TABLE marketplace_payments ADD CONSTRAINT marketplace_payments_status_check 
    CHECK (status IN ('pending', 'successful', 'failed', 'partially_refunded', 'refunded'));

-- +goose Down
-- SQL in section 'Down' is executed when this migration is rolled back

ALTER TABLE marketplace_orders DROP CONSTRAINT IF EXISTS marketplace_orders_payment_status_check;
ALTER TABLE marketplace_orders ADD CONSTRAINT marketplace_orders_payment_status_check 
    CHECK (payment_status IN ('pending', 'paid', 'failed', 'refunded'));

ALTER TABLE marketplace_payments DROP CONSTRAINT IF EXISTS marketplace_payments_status_check;
ALTER TABLE marketplace_payments ADD CONSTRAINT marketplace_payments_status_check 
    CHECK (status IN ('pending', 'successful', 'failed', 'refunded'));
