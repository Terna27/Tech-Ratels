-- -------------------------------------------------------------
-- Marketplace Businesses
-- -------------------------------------------------------------

-- name: CreateMarketplaceBusiness :one
INSERT INTO marketplace_businesses (
    id, owner_user_id, name, slug, description, category,
    ownership_type, status, contact_email, contact_phone,
    logo_url, banner_url, settlement_bank_name, settlement_account_number,
    settlement_account_name, is_verified, verified_by_user_id, verified_at,
    created_at, updated_at, version
) VALUES (
    $1, $2, $3, $4, $5, $6,
    $7, $8, $9, $10,
    $11, $12, $13, $14,
    $15, $16, $17, $18,
    CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 1
) RETURNING *;

-- name: GetMarketplaceBusinessByID :one
SELECT b.*,
       u.display_name as owner_name,
       u.email as owner_email,
       u.phone as owner_phone,
       u_ver.display_name as verified_by_name
FROM marketplace_businesses b
JOIN users u ON b.owner_user_id = u.id
LEFT JOIN users u_ver ON b.verified_by_user_id = u_ver.id
WHERE b.id = $1;

-- name: GetMarketplaceBusinessBySlug :one
SELECT b.*,
       u.display_name as owner_name,
       u.email as owner_email,
       u.phone as owner_phone,
       u_ver.display_name as verified_by_name
FROM marketplace_businesses b
JOIN users u ON b.owner_user_id = u.id
LEFT JOIN users u_ver ON b.verified_by_user_id = u_ver.id
WHERE b.slug = $1;

-- name: GetMarketplaceBusinessByOwner :one
SELECT * FROM marketplace_businesses
WHERE owner_user_id = $1
LIMIT 1;

-- name: ListMarketplaceBusinesses :many
SELECT b.*,
       u.display_name as owner_name,
       u.email as owner_email,
       COUNT(DISTINCT p.id) as total_products,
       COUNT(DISTINCT br.id) as total_branches
FROM marketplace_businesses b
JOIN users u ON b.owner_user_id = u.id
LEFT JOIN marketplace_products p ON p.business_id = b.id
LEFT JOIN marketplace_business_branches br ON br.business_id = b.id
WHERE ($1::text IS NULL OR $1 = '' OR b.status = $1)
  AND ($2::text IS NULL OR $2 = '' OR b.category = $2)
  AND ($3::text IS NULL OR $3 = '' OR b.ownership_type = $3)
  AND ($4::text IS NULL OR $4 = '' OR b.owner_user_id = $4)
GROUP BY b.id, u.display_name, u.email
ORDER BY b.created_at DESC;

-- name: UpdateMarketplaceBusiness :one
UPDATE marketplace_businesses
SET name = $2,
    description = $3,
    category = $4,
    contact_email = $5,
    contact_phone = $6,
    logo_url = $7,
    banner_url = $8,
    settlement_bank_name = $9,
    settlement_account_number = $10,
    settlement_account_name = $11,
    updated_at = CURRENT_TIMESTAMP,
    version = version + 1
WHERE id = $1
RETURNING *;

-- name: UpdateMarketplaceBusinessStatus :one
UPDATE marketplace_businesses
SET status = $2,
    is_verified = $3,
    verified_by_user_id = $4,
    verified_at = $5,
    rejection_reason = $6,
    updated_at = CURRENT_TIMESTAMP,
    version = version + 1
WHERE id = $1
RETURNING *;

-- -------------------------------------------------------------
-- Marketplace Business Branches
-- -------------------------------------------------------------

-- name: CreateBusinessBranch :one
INSERT INTO marketplace_business_branches (
    id, business_id, branch_name, state_id, lga_id, address,
    contact_phone, manager_name, is_headquarters, is_active,
    created_at, updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6,
    $7, $8, $9, $10,
    CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
) RETURNING *;

-- name: GetBusinessBranchByID :one
SELECT br.*,
       s.name as state_name,
       l.name as lga_name
FROM marketplace_business_branches br
JOIN states s ON br.state_id = s.id
LEFT JOIN lgas l ON br.lga_id = l.id
WHERE br.id = $1;

-- name: ListBusinessBranchesByBusiness :many
SELECT br.*,
       s.name as state_name,
       l.name as lga_name
FROM marketplace_business_branches br
JOIN states s ON br.state_id = s.id
LEFT JOIN lgas l ON br.lga_id = l.id
WHERE br.business_id = $1
ORDER BY br.is_headquarters DESC, br.branch_name ASC;

-- name: UpdateBusinessBranch :one
UPDATE marketplace_business_branches
SET branch_name = $2,
    state_id = $3,
    lga_id = $4,
    address = $5,
    contact_phone = $6,
    manager_name = $7,
    is_headquarters = $8,
    is_active = $9,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING *;

-- name: DeleteBusinessBranch :exec
DELETE FROM marketplace_business_branches
WHERE id = $1;

-- -------------------------------------------------------------
-- Marketplace Products
-- -------------------------------------------------------------

-- name: CreateMarketplaceProduct :one
INSERT INTO marketplace_products (
    id, business_id, branch_id, title, slug, description, category,
    sku, price_kobo, compare_at_price_kobo, stock_quantity, low_stock_threshold,
    images, weight_grams, delivery_restrictions, return_policy, status,
    is_featured, created_at, updated_at, version
) VALUES (
    $1, $2, $3, $4, $5, $6, $7,
    $8, $9, $10, $11, $12,
    $13, $14, $15, $16, $17,
    $18, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 1
) RETURNING *;

-- name: GetMarketplaceProductByID :one
SELECT p.*,
       b.name as business_name,
       b.slug as business_slug,
       b.category as business_category,
       b.ownership_type as business_ownership_type,
       b.status as business_status,
       b.owner_user_id as business_owner_id,
       br.branch_name,
       s.name as branch_state_name
FROM marketplace_products p
JOIN marketplace_businesses b ON p.business_id = b.id
LEFT JOIN marketplace_business_branches br ON p.branch_id = br.id
LEFT JOIN states s ON br.state_id = s.id
WHERE p.id = $1;

-- name: GetMarketplaceProductBySlug :one
SELECT p.*,
       b.name as business_name,
       b.slug as business_slug,
       b.category as business_category,
       b.ownership_type as business_ownership_type,
       b.status as business_status,
       b.owner_user_id as business_owner_id,
       br.branch_name,
       s.name as branch_state_name
FROM marketplace_products p
JOIN marketplace_businesses b ON p.business_id = b.id
LEFT JOIN marketplace_business_branches br ON p.branch_id = br.id
LEFT JOIN states s ON br.state_id = s.id
WHERE p.slug = $1;

-- name: ListMarketplaceProducts :many
SELECT p.*,
       b.name as business_name,
       b.slug as business_slug,
       b.ownership_type as business_ownership_type,
       br.branch_name,
       s.name as branch_state_name
FROM marketplace_products p
JOIN marketplace_businesses b ON p.business_id = b.id
LEFT JOIN marketplace_business_branches br ON p.branch_id = br.id
LEFT JOIN states s ON br.state_id = s.id
WHERE ($1::text IS NULL OR $1 = '' OR p.business_id = $1)
  AND ($2::text IS NULL OR $2 = '' OR p.category = $2)
  AND ($3::text IS NULL OR $3 = '' OR p.status = $3)
  AND ($4::text IS NULL OR $4 = '' OR ($4 = 'true' AND p.is_featured = true) OR ($4 = 'false' AND p.is_featured = false))
  AND ($5::text IS NULL OR $5 = '' OR b.status = $5)
  AND (
      $6::text IS NULL OR $6 = '' OR
      p.title ILIKE '%' || $6 || '%' OR
      p.description ILIKE '%' || $6 || '%' OR
      p.sku ILIKE '%' || $6 || '%'
  )
ORDER BY p.is_featured DESC, p.created_at DESC;

-- name: UpdateMarketplaceProduct :one
UPDATE marketplace_products
SET branch_id = $2,
    title = $3,
    description = $4,
    category = $5,
    sku = $6,
    price_kobo = $7,
    compare_at_price_kobo = $8,
    stock_quantity = $9,
    low_stock_threshold = $10,
    images = $11,
    weight_grams = $12,
    delivery_restrictions = $13,
    return_policy = $14,
    status = $15,
    is_featured = $16,
    updated_at = CURRENT_TIMESTAMP,
    version = version + 1
WHERE id = $1
RETURNING *;

-- name: DecrementProductStock :one
UPDATE marketplace_products
SET stock_quantity = stock_quantity - $2,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1 AND stock_quantity >= $2
RETURNING *;

-- name: IncrementProductStock :one
UPDATE marketplace_products
SET stock_quantity = stock_quantity + $2,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING *;

-- name: DeleteMarketplaceProduct :exec
DELETE FROM marketplace_products
WHERE id = $1;

-- -------------------------------------------------------------
-- Marketplace Orders
-- -------------------------------------------------------------

-- name: CreateMarketplaceOrder :one
INSERT INTO marketplace_orders (
    id, order_number, buyer_user_id, buyer_name, buyer_email, buyer_phone,
    delivery_type, shipping_address, shipping_state_id, shipping_lga_id,
    pickup_branch_id, subtotal_kobo, delivery_fee_kobo, total_kobo,
    currency, order_status, payment_status, fulfilment_status,
    idempotency_key, notes, created_at, updated_at, version
) VALUES (
    $1, $2, $3, $4, $5, $6,
    $7, $8, $9, $10,
    $11, $12, $13, $14,
    $15, $16, $17, $18,
    $19, $20, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 1
) RETURNING *;

-- name: GetMarketplaceOrderByID :one
SELECT o.*,
       s.name as shipping_state_name,
       l.name as shipping_lga_name,
       br.branch_name as pickup_branch_name,
       br.address as pickup_branch_address
FROM marketplace_orders o
LEFT JOIN states s ON o.shipping_state_id = s.id
LEFT JOIN lgas l ON o.shipping_lga_id = l.id
LEFT JOIN marketplace_business_branches br ON o.pickup_branch_id = br.id
WHERE o.id = $1;

-- name: GetMarketplaceOrderByOrderNumber :one
SELECT o.*,
       s.name as shipping_state_name,
       l.name as shipping_lga_name,
       br.branch_name as pickup_branch_name,
       br.address as pickup_branch_address
FROM marketplace_orders o
LEFT JOIN states s ON o.shipping_state_id = s.id
LEFT JOIN lgas l ON o.shipping_lga_id = l.id
LEFT JOIN marketplace_business_branches br ON o.pickup_branch_id = br.id
WHERE o.order_number = $1;

-- name: GetMarketplaceOrderByIdempotencyKey :one
SELECT * FROM marketplace_orders
WHERE idempotency_key = $1;

-- name: GetMarketplaceOrderForUpdate :one
SELECT * FROM marketplace_orders
WHERE id = $1
FOR UPDATE;

-- name: ListMarketplaceOrdersByBuyer :many
SELECT o.*,
       COUNT(oi.id) as item_count
FROM marketplace_orders o
LEFT JOIN marketplace_order_items oi ON oi.order_id = o.id
WHERE o.buyer_user_id = $1
GROUP BY o.id
ORDER BY o.created_at DESC;

-- name: ListMarketplaceOrdersByBusiness :many
SELECT DISTINCT o.*,
       SUM(oi.subtotal_kobo) as business_subtotal_kobo,
       COUNT(oi.id) as item_count
FROM marketplace_orders o
JOIN marketplace_order_items oi ON oi.order_id = o.id
WHERE oi.business_id = $1
  AND ($2::text IS NULL OR $2 = '' OR o.order_status = $2)
GROUP BY o.id
ORDER BY o.created_at DESC;

-- name: ListAllMarketplaceOrders :many
SELECT o.*,
       COUNT(oi.id) as item_count
FROM marketplace_orders o
LEFT JOIN marketplace_order_items oi ON oi.order_id = o.id
WHERE ($1::text IS NULL OR $1 = '' OR o.order_status = $1)
  AND ($2::text IS NULL OR $2 = '' OR o.payment_status = $2)
  AND ($3::text IS NULL OR $3 = '' OR o.fulfilment_status = $3)
GROUP BY o.id
ORDER BY o.created_at DESC;

-- name: UpdateMarketplaceOrderStatus :one
UPDATE marketplace_orders
SET order_status = $2,
    updated_at = CURRENT_TIMESTAMP,
    version = version + 1
WHERE id = $1
RETURNING *;

-- name: UpdateMarketplaceOrderPaymentStatus :one
UPDATE marketplace_orders
SET payment_status = $2,
    updated_at = CURRENT_TIMESTAMP,
    version = version + 1
WHERE id = $1
RETURNING *;

-- name: UpdateMarketplaceFulfilmentStatus :one
UPDATE marketplace_orders
SET fulfilment_status = $2,
    updated_at = CURRENT_TIMESTAMP,
    version = version + 1
WHERE id = $1
RETURNING *;

-- -------------------------------------------------------------
-- Marketplace Order Items
-- -------------------------------------------------------------

-- name: CreateMarketplaceOrderItem :one
INSERT INTO marketplace_order_items (
    id, order_id, product_id, business_id, title_snapshot,
    sku_snapshot, price_kobo_snapshot, quantity, subtotal_kobo,
    image_url_snapshot, created_at
) VALUES (
    $1, $2, $3, $4, $5,
    $6, $7, $8, $9,
    $10, CURRENT_TIMESTAMP
) RETURNING *;

-- name: ListMarketplaceOrderItemsByOrder :many
SELECT oi.*,
       b.name as business_name,
       b.slug as business_slug
FROM marketplace_order_items oi
JOIN marketplace_businesses b ON oi.business_id = b.id
WHERE oi.order_id = $1
ORDER BY oi.created_at ASC;

-- -------------------------------------------------------------
-- Marketplace Payments
-- -------------------------------------------------------------

-- name: CreateMarketplacePayment :one
INSERT INTO marketplace_payments (
    id, order_id, reference, provider, provider_reference, channel,
    amount_kobo, currency, status, paid_at, raw_webhook_payload,
    error_message, created_at, updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6,
    $7, $8, $9, $10, $11,
    $12, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
) RETURNING *;

-- name: GetMarketplacePaymentByReference :one
SELECT * FROM marketplace_payments
WHERE reference = $1;

-- name: GetMarketplacePaymentByReferenceForUpdate :one
SELECT * FROM marketplace_payments
WHERE reference = $1
FOR UPDATE;

-- name: GetMarketplacePaymentByOrder :one
SELECT * FROM marketplace_payments
WHERE order_id = $1
ORDER BY created_at DESC
LIMIT 1;

-- name: GetMarketplacePaymentByOrderForUpdate :one
SELECT * FROM marketplace_payments
WHERE order_id = $1
ORDER BY created_at DESC
LIMIT 1
FOR UPDATE;

-- name: UpdateMarketplacePaymentTransactionStatus :one
UPDATE marketplace_payments
SET status = $2,
    provider_reference = $3,
    channel = $4,
    paid_at = $5,
    raw_webhook_payload = $6,
    error_message = $7,
    updated_at = CURRENT_TIMESTAMP
WHERE reference = $1
RETURNING *;

-- name: ListMarketplacePayments :many
SELECT p.*,
       o.order_number,
       o.buyer_name,
       o.buyer_email
FROM marketplace_payments p
JOIN marketplace_orders o ON p.order_id = o.id
WHERE ($1::text IS NULL OR $1 = '' OR p.status = $1)
  AND ($2::text IS NULL OR $2 = '' OR p.provider = $2)
ORDER BY p.created_at DESC;

-- -------------------------------------------------------------
-- Marketplace Shipments
-- -------------------------------------------------------------

-- name: CreateMarketplaceShipment :one
INSERT INTO marketplace_shipments (
    id, order_id, carrier_name, tracking_number, tracking_url,
    status, dispatched_at, estimated_delivery_at, delivered_at,
    delivery_notes, proof_of_delivery_url, created_at, updated_at
) VALUES (
    $1, $2, $3, $4, $5,
    $6, $7, $8, $9,
    $10, $11, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
) RETURNING *;

-- name: GetMarketplaceShipmentByOrder :one
SELECT * FROM marketplace_shipments
WHERE order_id = $1
ORDER BY created_at DESC
LIMIT 1;

-- name: UpdateMarketplaceShipmentStatus :one
UPDATE marketplace_shipments
SET status = $2,
    delivered_at = $3,
    proof_of_delivery_url = $4,
    delivery_notes = $5,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING *;

-- -------------------------------------------------------------
-- Marketplace Disputes
-- -------------------------------------------------------------

-- name: CreateMarketplaceDispute :one
INSERT INTO marketplace_disputes (
    id, order_id, raised_by_user_id, reason, customer_notes,
    status, resolution_notes, resolved_by_user_id, resolved_at,
    created_at, updated_at
) VALUES (
    $1, $2, $3, $4, $5,
    'open', '', NULL, NULL,
    CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
) RETURNING *;

-- name: GetMarketplaceDisputeByID :one
SELECT d.*,
       o.order_number,
       o.total_kobo,
       u_raised.display_name as raised_by_name,
       u_raised.email as raised_by_email,
       u_res.display_name as resolved_by_name
FROM marketplace_disputes d
JOIN marketplace_orders o ON d.order_id = o.id
JOIN users u_raised ON d.raised_by_user_id = u_raised.id
LEFT JOIN users u_res ON d.resolved_by_user_id = u_res.id
WHERE d.id = $1;

-- name: ListMarketplaceDisputesByOrder :many
SELECT * FROM marketplace_disputes
WHERE order_id = $1
ORDER BY created_at DESC;

-- name: ListAllMarketplaceDisputes :many
SELECT d.*,
       o.order_number,
       o.total_kobo,
       u_raised.display_name as raised_by_name,
       u_raised.email as raised_by_email
FROM marketplace_disputes d
JOIN marketplace_orders o ON d.order_id = o.id
JOIN users u_raised ON d.raised_by_user_id = u_raised.id
WHERE ($1::text IS NULL OR $1 = '' OR d.status = $1)
ORDER BY d.created_at DESC;

-- name: ResolveMarketplaceDispute :one
UPDATE marketplace_disputes
SET status = $2,
    resolution_notes = $3,
    resolved_by_user_id = $4,
    resolved_at = $5,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING *;

-- -------------------------------------------------------------
-- Marketplace Overview Analytics
-- -------------------------------------------------------------

-- name: GetMarketplaceAdminOverviewMetrics :one
SELECT
    COALESCE(SUM(CASE WHEN payment_status = 'paid' THEN total_kobo ELSE 0 END), 0)::bigint as total_gmv_kobo,
    COUNT(id)::bigint as total_orders_count,
    COUNT(CASE WHEN order_status = 'placed' OR order_status = 'processing' THEN 1 END)::bigint as active_orders_count,
    (SELECT COUNT(id) FROM marketplace_businesses WHERE status = 'pending_review')::bigint as pending_businesses_count,
    (SELECT COUNT(id) FROM marketplace_businesses WHERE status = 'approved')::bigint as approved_businesses_count,
    (SELECT COUNT(id) FROM marketplace_products WHERE status = 'published')::bigint as active_products_count,
    (SELECT COUNT(id) FROM marketplace_disputes WHERE status = 'open' OR status = 'under_review')::bigint as open_disputes_count
FROM marketplace_orders;

-- name: GetMarketplaceSellerOverviewMetrics :one
SELECT
    COALESCE(SUM(CASE WHEN o.payment_status = 'paid' THEN oi.subtotal_kobo ELSE 0 END), 0)::bigint as seller_gross_sales_kobo,
    COUNT(DISTINCT o.id)::bigint as seller_orders_count,
    COUNT(DISTINCT CASE WHEN o.order_status = 'processing' OR o.order_status = 'ready_for_pickup' THEN o.id END)::bigint as seller_pending_orders_count,
    (SELECT COUNT(mp.id) FROM marketplace_products mp WHERE mp.business_id = $1)::bigint as seller_products_count
FROM marketplace_order_items oi
JOIN marketplace_orders o ON oi.order_id = o.id
WHERE oi.business_id = $1;

-- -------------------------------------------------------------
-- Marketplace Refund Accounting Ledger
-- -------------------------------------------------------------

-- name: CreateMarketplaceRefund :one
INSERT INTO marketplace_refunds (
    id, order_id, payment_id, amount_kobo, status,
    provider, provider_refund_reference, initiated_by_user_id,
    confirmed_by_user_id, reason, settlement_notes, metadata,
    created_at, updated_at
) VALUES (
    $1, $2, $3, $4, $5,
    $6, $7, $8,
    $9, $10, $11, $12,
    CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
) RETURNING *;

-- name: GetMarketplaceRefundByID :one
SELECT * FROM marketplace_refunds
WHERE id = $1;

-- name: GetMarketplaceRefundByIDForUpdate :one
SELECT * FROM marketplace_refunds
WHERE id = $1
FOR UPDATE;

-- name: GetMarketplaceRefundByProviderReference :one
SELECT * FROM marketplace_refunds
WHERE provider_refund_reference = $1
ORDER BY created_at DESC
LIMIT 1;

-- name: GetMarketplaceRefundByProviderReferenceForUpdate :one
SELECT * FROM marketplace_refunds
WHERE provider_refund_reference = $1
ORDER BY created_at DESC
LIMIT 1
FOR UPDATE;

-- name: ListMarketplaceRefundsByOrder :many
SELECT * FROM marketplace_refunds
WHERE order_id = $1
ORDER BY created_at DESC;

-- name: GetMarketplaceRefundSumsByOrder :one
SELECT 
    COALESCE(SUM(CASE WHEN status = 'successful' THEN amount_kobo ELSE 0 END), 0)::bigint AS successful_amount_kobo,
    COALESCE(SUM(CASE WHEN status = 'pending' THEN amount_kobo ELSE 0 END), 0)::bigint AS pending_amount_kobo
FROM marketplace_refunds
WHERE order_id = $1;

-- name: UpdateMarketplaceRefundStatus :one
UPDATE marketplace_refunds
SET status = $2,
    provider_refund_reference = $3,
    confirmed_by_user_id = $4,
    settlement_notes = $5,
    metadata = $6,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING *;
