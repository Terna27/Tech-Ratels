-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied

-- 1. Marketplace Businesses (Registered Vendors / Community Enterprises)
CREATE TABLE IF NOT EXISTS marketplace_businesses (
    id VARCHAR(36) PRIMARY KEY,
    owner_user_id VARCHAR(36) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(255) UNIQUE NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    category VARCHAR(100) NOT NULL,
    ownership_type VARCHAR(50) NOT NULL CHECK (ownership_type IN ('movement_owned', 'member_owned', 'approved_partner')),
    status VARCHAR(50) NOT NULL DEFAULT 'pending_review' CHECK (status IN ('pending_review', 'approved', 'suspended', 'rejected')),
    contact_email VARCHAR(255) NOT NULL,
    contact_phone VARCHAR(50) NOT NULL,
    logo_url TEXT,
    banner_url TEXT,
    settlement_bank_name VARCHAR(100),
    settlement_account_number VARCHAR(50),
    settlement_account_name VARCHAR(255),
    is_verified BOOLEAN NOT NULL DEFAULT FALSE,
    verified_by_user_id VARCHAR(36) REFERENCES users(id) ON DELETE SET NULL,
    verified_at TIMESTAMPTZ,
    rejection_reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    version INT NOT NULL DEFAULT 1
);
CREATE INDEX IF NOT EXISTS idx_marketplace_businesses_owner ON marketplace_businesses(owner_user_id);
CREATE INDEX IF NOT EXISTS idx_marketplace_businesses_status ON marketplace_businesses(status);
CREATE INDEX IF NOT EXISTS idx_marketplace_businesses_slug ON marketplace_businesses(slug);
CREATE INDEX IF NOT EXISTS idx_marketplace_businesses_category ON marketplace_businesses(category);

-- 2. Marketplace Business Branches (Physical Outlets & Warehouses in Nigerian States)
CREATE TABLE IF NOT EXISTS marketplace_business_branches (
    id VARCHAR(36) PRIMARY KEY,
    business_id VARCHAR(36) NOT NULL REFERENCES marketplace_businesses(id) ON DELETE CASCADE,
    branch_name VARCHAR(255) NOT NULL,
    state_id VARCHAR(36) NOT NULL REFERENCES states(id),
    lga_id VARCHAR(36) REFERENCES lgas(id) ON DELETE SET NULL,
    address TEXT NOT NULL,
    contact_phone VARCHAR(50) NOT NULL,
    manager_name VARCHAR(255),
    is_headquarters BOOLEAN NOT NULL DEFAULT FALSE,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_marketplace_branches_business ON marketplace_business_branches(business_id);
CREATE INDEX IF NOT EXISTS idx_marketplace_branches_state ON marketplace_business_branches(state_id);
CREATE INDEX IF NOT EXISTS idx_marketplace_branches_lga ON marketplace_business_branches(lga_id);

-- 3. Marketplace Products (Separate from Movement Inventory)
CREATE TABLE IF NOT EXISTS marketplace_products (
    id VARCHAR(36) PRIMARY KEY,
    business_id VARCHAR(36) NOT NULL REFERENCES marketplace_businesses(id) ON DELETE CASCADE,
    branch_id VARCHAR(36) REFERENCES marketplace_business_branches(id) ON DELETE SET NULL,
    title VARCHAR(255) NOT NULL,
    slug VARCHAR(255) UNIQUE NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    category VARCHAR(100) NOT NULL,
    sku VARCHAR(100) NOT NULL,
    price_kobo BIGINT NOT NULL CHECK (price_kobo >= 0),
    compare_at_price_kobo BIGINT CHECK (compare_at_price_kobo IS NULL OR compare_at_price_kobo >= price_kobo),
    stock_quantity INT NOT NULL DEFAULT 0 CHECK (stock_quantity >= 0),
    low_stock_threshold INT NOT NULL DEFAULT 5 CHECK (low_stock_threshold >= 0),
    images JSONB NOT NULL DEFAULT '[]'::jsonb,
    weight_grams INT DEFAULT 0,
    delivery_restrictions TEXT DEFAULT '',
    return_policy TEXT DEFAULT '',
    status VARCHAR(50) NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published', 'archived')),
    is_featured BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    version INT NOT NULL DEFAULT 1
);
CREATE INDEX IF NOT EXISTS idx_marketplace_products_business ON marketplace_products(business_id);
CREATE INDEX IF NOT EXISTS idx_marketplace_products_branch ON marketplace_products(branch_id);
CREATE INDEX IF NOT EXISTS idx_marketplace_products_status ON marketplace_products(status);
CREATE INDEX IF NOT EXISTS idx_marketplace_products_category ON marketplace_products(category);
CREATE INDEX IF NOT EXISTS idx_marketplace_products_slug ON marketplace_products(slug);
CREATE INDEX IF NOT EXISTS idx_marketplace_products_featured ON marketplace_products(is_featured);

-- 4. Marketplace Orders
CREATE TABLE IF NOT EXISTS marketplace_orders (
    id VARCHAR(36) PRIMARY KEY,
    order_number VARCHAR(64) UNIQUE NOT NULL,
    buyer_user_id VARCHAR(36) REFERENCES users(id) ON DELETE SET NULL,
    buyer_name VARCHAR(255) NOT NULL,
    buyer_email VARCHAR(255) NOT NULL,
    buyer_phone VARCHAR(50) NOT NULL,
    delivery_type VARCHAR(50) NOT NULL DEFAULT 'delivery' CHECK (delivery_type IN ('delivery', 'pickup')),
    shipping_address TEXT NOT NULL DEFAULT '',
    shipping_state_id VARCHAR(36) REFERENCES states(id),
    shipping_lga_id VARCHAR(36) REFERENCES lgas(id),
    pickup_branch_id VARCHAR(36) REFERENCES marketplace_business_branches(id),
    subtotal_kobo BIGINT NOT NULL CHECK (subtotal_kobo >= 0),
    delivery_fee_kobo BIGINT NOT NULL DEFAULT 0 CHECK (delivery_fee_kobo >= 0),
    total_kobo BIGINT NOT NULL CHECK (total_kobo >= 0),
    currency VARCHAR(10) NOT NULL DEFAULT 'NGN',
    order_status VARCHAR(50) NOT NULL DEFAULT 'placed' CHECK (order_status IN ('placed', 'processing', 'ready_for_pickup', 'dispatched', 'delivered', 'cancelled', 'refunded')),
    payment_status VARCHAR(50) NOT NULL DEFAULT 'pending' CHECK (payment_status IN ('pending', 'paid', 'failed', 'partially_refunded', 'refunded')),
    fulfilment_status VARCHAR(50) NOT NULL DEFAULT 'unfulfilled' CHECK (fulfilment_status IN ('unfulfilled', 'preparing', 'dispatched', 'delivered', 'returned')),
    idempotency_key VARCHAR(128) UNIQUE,
    notes TEXT DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    version INT NOT NULL DEFAULT 1
);
CREATE INDEX IF NOT EXISTS idx_marketplace_orders_buyer ON marketplace_orders(buyer_user_id);
CREATE INDEX IF NOT EXISTS idx_marketplace_orders_number ON marketplace_orders(order_number);
CREATE INDEX IF NOT EXISTS idx_marketplace_orders_status ON marketplace_orders(order_status);
CREATE INDEX IF NOT EXISTS idx_marketplace_orders_payment ON marketplace_orders(payment_status);
CREATE INDEX IF NOT EXISTS idx_marketplace_orders_fulfilment ON marketplace_orders(fulfilment_status);
CREATE INDEX IF NOT EXISTS idx_marketplace_orders_created ON marketplace_orders(created_at DESC);

-- 5. Marketplace Order Items (Historical Price & Product Snapshots)
CREATE TABLE IF NOT EXISTS marketplace_order_items (
    id VARCHAR(36) PRIMARY KEY,
    order_id VARCHAR(36) NOT NULL REFERENCES marketplace_orders(id) ON DELETE CASCADE,
    product_id VARCHAR(36) REFERENCES marketplace_products(id) ON DELETE SET NULL,
    business_id VARCHAR(36) NOT NULL REFERENCES marketplace_businesses(id) ON DELETE CASCADE,
    title_snapshot VARCHAR(255) NOT NULL,
    sku_snapshot VARCHAR(100) NOT NULL,
    price_kobo_snapshot BIGINT NOT NULL CHECK (price_kobo_snapshot >= 0),
    quantity INT NOT NULL CHECK (quantity > 0),
    subtotal_kobo BIGINT NOT NULL CHECK (subtotal_kobo >= 0),
    image_url_snapshot TEXT DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_marketplace_order_items_order ON marketplace_order_items(order_id);
CREATE INDEX IF NOT EXISTS idx_marketplace_order_items_business ON marketplace_order_items(business_id);
CREATE INDEX IF NOT EXISTS idx_marketplace_order_items_product ON marketplace_order_items(product_id);

-- 6. Marketplace Payments (Provider Transactions & Reconciliation)
CREATE TABLE IF NOT EXISTS marketplace_payments (
    id VARCHAR(36) PRIMARY KEY,
    order_id VARCHAR(36) NOT NULL REFERENCES marketplace_orders(id) ON DELETE CASCADE,
    reference VARCHAR(128) UNIQUE NOT NULL,
    provider VARCHAR(50) NOT NULL DEFAULT 'paystack',
    provider_reference VARCHAR(255),
    channel VARCHAR(50) DEFAULT 'card',
    amount_kobo BIGINT NOT NULL CHECK (amount_kobo >= 0),
    currency VARCHAR(10) NOT NULL DEFAULT 'NGN',
    status VARCHAR(50) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'successful', 'failed', 'partially_refunded', 'refunded')),
    paid_at TIMESTAMPTZ,
    raw_webhook_payload JSONB DEFAULT '{}'::jsonb,
    error_message TEXT DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_marketplace_payments_order ON marketplace_payments(order_id);
CREATE INDEX IF NOT EXISTS idx_marketplace_payments_reference ON marketplace_payments(reference);
CREATE INDEX IF NOT EXISTS idx_marketplace_payments_status ON marketplace_payments(status);

-- 7. Marketplace Shipments & Delivery Tracking
CREATE TABLE IF NOT EXISTS marketplace_shipments (
    id VARCHAR(36) PRIMARY KEY,
    order_id VARCHAR(36) NOT NULL REFERENCES marketplace_orders(id) ON DELETE CASCADE,
    carrier_name VARCHAR(100) NOT NULL,
    tracking_number VARCHAR(100) NOT NULL,
    tracking_url TEXT DEFAULT '',
    status VARCHAR(50) NOT NULL DEFAULT 'dispatched' CHECK (status IN ('dispatched', 'in_transit', 'delivered', 'failed', 'returned')),
    dispatched_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    estimated_delivery_at TIMESTAMPTZ,
    delivered_at TIMESTAMPTZ,
    delivery_notes TEXT DEFAULT '',
    proof_of_delivery_url TEXT DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_marketplace_shipments_order ON marketplace_shipments(order_id);
CREATE INDEX IF NOT EXISTS idx_marketplace_shipments_tracking ON marketplace_shipments(tracking_number);

-- 8. Marketplace Disputes & Returns
CREATE TABLE IF NOT EXISTS marketplace_disputes (
    id VARCHAR(36) PRIMARY KEY,
    order_id VARCHAR(36) NOT NULL REFERENCES marketplace_orders(id) ON DELETE CASCADE,
    raised_by_user_id VARCHAR(36) NOT NULL REFERENCES users(id),
    reason VARCHAR(100) NOT NULL CHECK (reason IN ('damaged_goods', 'missing_delivery', 'item_mismatch', 'cancelled_before_dispatch', 'other')),
    customer_notes TEXT NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'under_review', 'resolved_refunded', 'rejected')),
    resolution_notes TEXT DEFAULT '',
    resolved_by_user_id VARCHAR(36) REFERENCES users(id) ON DELETE SET NULL,
    resolved_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_marketplace_disputes_order ON marketplace_disputes(order_id);
CREATE INDEX IF NOT EXISTS idx_marketplace_disputes_status ON marketplace_disputes(status);
CREATE INDEX IF NOT EXISTS idx_marketplace_disputes_raised ON marketplace_disputes(raised_by_user_id);

-- +goose Down
-- SQL in section 'Down' is executed when this migration is rolled back
DROP TABLE IF EXISTS marketplace_disputes CASCADE;
DROP TABLE IF EXISTS marketplace_shipments CASCADE;
DROP TABLE IF EXISTS marketplace_payments CASCADE;
DROP TABLE IF EXISTS marketplace_order_items CASCADE;
DROP TABLE IF EXISTS marketplace_orders CASCADE;
DROP TABLE IF EXISTS marketplace_products CASCADE;
DROP TABLE IF EXISTS marketplace_business_branches CASCADE;
DROP TABLE IF EXISTS marketplace_businesses CASCADE;
