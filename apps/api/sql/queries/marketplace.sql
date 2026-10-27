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
    idempotency_key, notes, reservation_expires_at, created_at, updated_at, version
) VALUES (
    $1, $2, $3, $4, $5, $6,
    $7, $8, $9, $10,
    $11, $12, $13, $14,
    $15, $16, $17, $18,
    $19, $20, $21, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 1
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
    id, order_id, business_id, carrier_name, tracking_number, tracking_url,
    status, dispatched_at, estimated_delivery_at, delivered_at,
    delivery_notes, proof_of_delivery_url, created_at, updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6,
    $7, $8, $9, $10,
    $11, $12, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
) RETURNING *;

-- name: GetMarketplaceShipmentByID :one
SELECT * FROM marketplace_shipments
WHERE id = $1;

-- name: GetMarketplaceShipmentByIDForUpdate :one
SELECT * FROM marketplace_shipments
WHERE id = $1
FOR UPDATE;

-- name: GetMarketplaceShipmentByOrderAndBusiness :one
SELECT * FROM marketplace_shipments
WHERE order_id = $1 AND business_id = $2
LIMIT 1;

-- name: GetMarketplaceShipmentByOrder :one
SELECT * FROM marketplace_shipments
WHERE order_id = $1
ORDER BY created_at DESC
LIMIT 1;

-- name: ListMarketplaceShipmentsByOrder :many
SELECT * FROM marketplace_shipments
WHERE order_id = $1
ORDER BY created_at ASC;

-- name: ListMarketplaceShipmentsByBusiness :many
SELECT s.*,
       o.order_number,
       o.buyer_name,
       o.buyer_email,
       o.buyer_phone,
       o.shipping_address,
       o.shipping_state_id,
       o.shipping_lga_id,
       o.payment_status,
       o.order_status,
       b.name as business_name
FROM marketplace_shipments s
JOIN marketplace_orders o ON o.id = s.order_id
JOIN marketplace_businesses b ON b.id = s.business_id
WHERE s.business_id = $1
  AND ($2::text IS NULL OR $2 = '' OR s.status = $2)
ORDER BY s.created_at DESC;

-- name: ListAllMarketplaceShipmentsAdmin :many
SELECT s.*,
       o.order_number,
       o.buyer_name,
       o.buyer_email,
       o.shipping_address,
       o.shipping_state_id,
       o.shipping_lga_id,
       o.pickup_branch_id,
       o.payment_status,
       o.order_status,
       b.name as business_name,
       b.owner_user_id as seller_user_id
FROM marketplace_shipments s
JOIN marketplace_orders o ON o.id = s.order_id
JOIN marketplace_businesses b ON b.id = s.business_id
WHERE ($1::text IS NULL OR $1 = '' OR s.status = $1)
ORDER BY s.created_at DESC;

-- name: DispatchMarketplaceShipment :one
UPDATE marketplace_shipments
SET status = 'dispatched',
    carrier_name = $2,
    tracking_number = $3,
    tracking_url = $4,
    dispatched_at = $5,
    estimated_delivery_at = $6,
    delivery_notes = $7,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING *;

-- name: ConfirmMarketplaceShipmentDelivery :one
UPDATE marketplace_shipments
SET status = 'delivered',
    delivered_at = $2,
    proof_of_delivery_url = $3,
    delivery_notes = $4,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING *;

-- name: UpdateMarketplaceShipmentTracking :one
UPDATE marketplace_shipments
SET carrier_name = $2,
    tracking_number = $3,
    tracking_url = $4,
    delivery_notes = $5,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING *;

-- name: CancelMarketplaceShipmentsByOrder :exec
UPDATE marketplace_shipments
SET status = 'cancelled',
    updated_at = CURRENT_TIMESTAMP
WHERE order_id = $1
  AND status IN ('pending', 'processing');

-- name: CancelMarketplaceShipmentByID :one
UPDATE marketplace_shipments
SET status = 'cancelled',
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING *;

-- name: UpdateMarketplaceShipmentStatus :one
UPDATE marketplace_shipments
SET status = $2,
    delivered_at = $3,
    proof_of_delivery_url = $4,
    delivery_notes = $5,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING *;

-- name: CreateMarketplaceShipmentItem :one
INSERT INTO marketplace_shipment_items (
    id, shipment_id, order_item_id, quantity, created_at
) VALUES (
    $1, $2, $3, $4, CURRENT_TIMESTAMP
) ON CONFLICT (shipment_id, order_item_id) DO NOTHING
RETURNING *;

-- name: ListMarketplaceShipmentItemsByShipment :many
SELECT msi.*,
       oi.product_id,
       oi.title_snapshot,
       oi.sku_snapshot,
       oi.price_kobo_snapshot,
       oi.image_url_snapshot,
       oi.business_id
FROM marketplace_shipment_items msi
JOIN marketplace_order_items oi ON oi.id = msi.order_item_id
WHERE msi.shipment_id = $1
ORDER BY msi.created_at ASC;

-- name: UpdateOrderItemShipmentID :exec
UPDATE marketplace_order_items
SET shipment_id = $2
WHERE id = $1;

-- name: GetMarketplaceOrderItemByID :one
SELECT * FROM marketplace_order_items
WHERE id = $1;

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
    shipment_id, business_id, order_item_id,
    created_at, updated_at
) VALUES (
    $1, $2, $3, $4, $5,
    $6, $7, $8,
    $9, $10, $11, $12,
    $13, $14, $15,
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

-- name: GetPendingRefundByOrder :one
SELECT * FROM marketplace_refunds
WHERE order_id = $1
  AND status = 'pending'
ORDER BY created_at DESC
LIMIT 1;

-- name: ListMarketplaceRefundsByShipment :many
SELECT * FROM marketplace_refunds
WHERE shipment_id = $1
ORDER BY created_at DESC;

-- name: HasActiveRefundForShipment :one
SELECT EXISTS (
    SELECT 1 FROM marketplace_refunds
    WHERE shipment_id = $1
      AND status IN ('pending', 'successful')
) AS has_active_refund;

-- name: HasUnattributedSuccessfulOrPendingRefunds :one
SELECT EXISTS (
    SELECT 1 FROM marketplace_refunds
    WHERE order_id = $1
      AND shipment_id IS NULL
      AND status IN ('pending', 'successful')
) AS has_unattributed;

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

-- -------------------------------------------------------------
-- Inventory Ledger & Reservation Expiry Queries
-- -------------------------------------------------------------

-- name: GetMarketplaceProductByIDForUpdate :one
SELECT p.*,
       b.name as business_name,
       b.slug as business_slug,
       b.status as business_status,
       b.owner_user_id as business_owner_id
FROM marketplace_products p
JOIN marketplace_businesses b ON p.business_id = b.id
WHERE p.id = $1
FOR UPDATE;

-- name: SetProductStockQuantity :one
UPDATE marketplace_products
SET stock_quantity = $2,
    updated_at = CURRENT_TIMESTAMP,
    version = version + 1
WHERE id = $1 AND $2 >= 0
RETURNING *;

-- name: CreateMarketplaceInventoryLog :one
INSERT INTO marketplace_inventory_logs (
    id, product_id, business_id, actor_user_id, action_type,
    quantity_delta, balance_after, order_id, reference_id, reason, idempotency_key
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
)
RETURNING *;

-- name: ListMarketplaceInventoryLogsByProduct :many
SELECT * FROM marketplace_inventory_logs
WHERE product_id = $1
ORDER BY created_at DESC;

-- name: ListMarketplaceInventoryLogsByBusiness :many
SELECT * FROM marketplace_inventory_logs
WHERE business_id = $1
ORDER BY created_at DESC;

-- name: GetMarketplaceInventoryLogByIdempotencyKey :one
SELECT * FROM marketplace_inventory_logs
WHERE idempotency_key = $1;

-- name: CountOrderInventoryReleaseLogs :one
SELECT COUNT(*) FROM marketplace_inventory_logs
WHERE order_id = $1 AND product_id = $2
  AND action_type IN ('reservation_release', 'cancellation_restock', 'refund_restock');

-- name: ListExpiredPendingMarketplaceOrders :many
SELECT * FROM marketplace_orders
WHERE payment_status IN ('pending', 'failed')
  AND order_status = 'placed'
  AND stock_released = FALSE
  AND reservation_expires_at IS NOT NULL
  AND reservation_expires_at <= $1
ORDER BY reservation_expires_at ASC
LIMIT $2;

-- name: UpdateMarketplaceOrderExpiryFields :one
UPDATE marketplace_orders
SET reservation_expires_at = $2,
    cancellation_reason = $3,
    stock_released = $4,
    reconciliation_required = $5,
    reconciliation_notes = $6,
    updated_at = CURRENT_TIMESTAMP,
    version = version + 1
WHERE id = $1
RETURNING *;

-- name: MarkOrderCancelledWithReason :one
UPDATE marketplace_orders
SET order_status = 'cancelled',
    cancellation_reason = $2,
    stock_released = $3,
    updated_at = CURRENT_TIMESTAMP,
    version = version + 1
WHERE id = $1
RETURNING *;

-- name: MarkOrderRequiresReconciliation :one
UPDATE marketplace_orders
SET order_status = 'requires_reconciliation',
    reconciliation_required = TRUE,
    reconciliation_notes = $2,
    updated_at = CURRENT_TIMESTAMP,
    version = version + 1
WHERE id = $1
RETURNING *;
