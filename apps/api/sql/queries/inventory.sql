-- name: CreateInventoryItem :one
INSERT INTO inventory_items (
    id, sku, name, description, category, unit_of_measure, min_threshold_quantity, is_batch_tracked, is_active, created_at, updated_at, version
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 1
) RETURNING *;

-- name: GetInventoryItemByID :one
SELECT * FROM inventory_items WHERE id = $1;

-- name: GetInventoryItemBySKU :one
SELECT * FROM inventory_items WHERE sku = $1;

-- name: ListInventoryItems :many
SELECT * FROM inventory_items
WHERE ($1::text IS NULL OR $1 = '' OR category = $1)
  AND ($2::boolean IS NULL OR is_active = $2)
ORDER BY category, name;

-- name: UpdateInventoryItem :one
UPDATE inventory_items
SET name = $2,
    description = $3,
    category = $4,
    unit_of_measure = $5,
    min_threshold_quantity = $6,
    is_batch_tracked = $7,
    is_active = $8,
    updated_at = CURRENT_TIMESTAMP,
    version = version + 1
WHERE id = $1
RETURNING *;

-- name: CreateStorageLocation :one
INSERT INTO storage_locations (
    id, code, name, location_type, zone_id, state_id, lga_id, address, custodian_user_id, contact_phone, is_active, created_at, updated_at, version
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 1
) RETURNING *;

-- name: GetStorageLocationByID :one
SELECT sl.*,
       z.name as zone_name,
       s.name as state_name,
       l.name as lga_name,
       u.display_name as custodian_name,
       u.email as custodian_email
FROM storage_locations sl
LEFT JOIN zones z ON sl.zone_id = z.id
LEFT JOIN states s ON sl.state_id = s.id
LEFT JOIN lgas l ON sl.lga_id = l.id
LEFT JOIN users u ON sl.custodian_user_id = u.id
WHERE sl.id = $1;

-- name: GetStorageLocationByCode :one
SELECT * FROM storage_locations WHERE code = $1;

-- name: ListStorageLocations :many
SELECT sl.*,
       z.name as zone_name,
       s.name as state_name,
       l.name as lga_name,
       u.display_name as custodian_name
FROM storage_locations sl
LEFT JOIN zones z ON sl.zone_id = z.id
LEFT JOIN states s ON sl.state_id = s.id
LEFT JOIN lgas l ON sl.lga_id = l.id
LEFT JOIN users u ON sl.custodian_user_id = u.id
WHERE ($1::text IS NULL OR $1 = '' OR sl.location_type = $1)
  AND ($2::text IS NULL OR $2 = '' OR sl.zone_id = $2)
  AND ($3::text IS NULL OR $3 = '' OR sl.state_id = $3)
  AND ($4::text IS NULL OR $4 = '' OR sl.lga_id = $4)
  AND ($5::boolean IS NULL OR sl.is_active = $5)
ORDER BY sl.location_type, sl.name;

-- name: UpdateStorageLocation :one
UPDATE storage_locations
SET name = $2,
    location_type = $3,
    zone_id = $4,
    state_id = $5,
    lga_id = $6,
    address = $7,
    custodian_user_id = $8,
    contact_phone = $9,
    is_active = $10,
    updated_at = CURRENT_TIMESTAMP,
    version = version + 1
WHERE id = $1
RETURNING *;

-- name: GetStockBalance :one
SELECT * FROM stock_balances
WHERE location_id = $1 AND item_id = $2 AND batch_number = $3;

-- name: GetStockBalanceForUpdate :one
SELECT * FROM stock_balances
WHERE location_id = $1 AND item_id = $2 AND batch_number = $3
FOR UPDATE;

-- name: UpsertStockBalance :one
INSERT INTO stock_balances (
    id, location_id, item_id, on_hand_quantity, reserved_quantity, batch_number, expiry_date, updated_at, version
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, CURRENT_TIMESTAMP, 1
)
ON CONFLICT (location_id, item_id, batch_number)
DO UPDATE SET
    on_hand_quantity = stock_balances.on_hand_quantity + EXCLUDED.on_hand_quantity,
    reserved_quantity = stock_balances.reserved_quantity + EXCLUDED.reserved_quantity,
    expiry_date = COALESCE(EXCLUDED.expiry_date, stock_balances.expiry_date),
    updated_at = CURRENT_TIMESTAMP,
    version = stock_balances.version + 1
RETURNING *;

-- name: SetStockBalanceQuantities :one
UPDATE stock_balances
SET on_hand_quantity = $4,
    reserved_quantity = $5,
    updated_at = CURRENT_TIMESTAMP,
    version = version + 1
WHERE location_id = $1 AND item_id = $2 AND batch_number = $3
RETURNING *;

-- name: ListStockBalancesByLocation :many
SELECT sb.*,
       i.sku,
       i.name as item_name,
       i.category as item_category,
       i.unit_of_measure,
       i.min_threshold_quantity,
       (sb.on_hand_quantity - sb.reserved_quantity) as available_quantity
FROM stock_balances sb
JOIN inventory_items i ON sb.item_id = i.id
WHERE sb.location_id = $1
ORDER BY i.category, i.name;

-- name: ListAllStockBalances :many
SELECT sb.*,
       i.sku,
       i.name as item_name,
       i.category as item_category,
       i.unit_of_measure,
       i.min_threshold_quantity,
       (sb.on_hand_quantity - sb.reserved_quantity) as available_quantity,
       sl.name as location_name,
       sl.code as location_code,
       sl.location_type
FROM stock_balances sb
JOIN inventory_items i ON sb.item_id = i.id
JOIN storage_locations sl ON sb.location_id = sl.id
WHERE ($1::text IS NULL OR $1 = '' OR sb.location_id = $1)
  AND ($2::text IS NULL OR $2 = '' OR sb.item_id = $2)
  AND ($3::text IS NULL OR $3 = '' OR i.category = $3)
ORDER BY sl.name, i.name;

-- name: ListLowStockAlerts :many
SELECT sb.*,
       i.sku,
       i.name as item_name,
       i.category as item_category,
       i.unit_of_measure,
       i.min_threshold_quantity,
       (sb.on_hand_quantity - sb.reserved_quantity) as available_quantity,
       sl.name as location_name,
       sl.code as location_code
FROM stock_balances sb
JOIN inventory_items i ON sb.item_id = i.id
JOIN storage_locations sl ON sb.location_id = sl.id
WHERE (sb.on_hand_quantity - sb.reserved_quantity) <= i.min_threshold_quantity
  AND sl.is_active = true
ORDER BY available_quantity ASC;

-- name: CreateMaterialRequest :one
INSERT INTO material_requests (
    id, reference, requesting_user_id, jurisdiction_type, jurisdiction_id,
    destination_location_id, status, intended_use, beneficiary_category,
    required_date, notes, created_at, updated_at, version
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 1
) RETURNING *;

-- name: GetMaterialRequestByID :one
SELECT r.*,
       u.display_name as requesting_user_name,
       u.email as requesting_user_email,
       sl.name as destination_location_name,
       sl.code as destination_location_code,
       rev.display_name as reviewer_name
FROM material_requests r
JOIN users u ON r.requesting_user_id = u.id
JOIN storage_locations sl ON r.destination_location_id = sl.id
LEFT JOIN users rev ON r.reviewed_by_user_id = rev.id
WHERE r.id = $1;

-- name: GetMaterialRequestByReference :one
SELECT * FROM material_requests WHERE reference = $1;

-- name: ListMaterialRequests :many
SELECT r.*,
       u.display_name as requesting_user_name,
       sl.name as destination_location_name,
       sl.code as destination_location_code,
       rev.display_name as reviewer_name
FROM material_requests r
JOIN users u ON r.requesting_user_id = u.id
JOIN storage_locations sl ON r.destination_location_id = sl.id
LEFT JOIN users rev ON r.reviewed_by_user_id = rev.id
WHERE ($1::text IS NULL OR $1 = '' OR r.status = $1)
  AND ($2::text IS NULL OR $2 = '' OR r.destination_location_id = $2)
  AND ($3::text IS NULL OR $3 = '' OR r.requesting_user_id = $3)
  AND ($4::text IS NULL OR $4 = '' OR r.jurisdiction_type = $4)
ORDER BY r.created_at DESC;

-- name: UpdateMaterialRequestReview :one
UPDATE material_requests
SET status = $2,
    rejection_reason = $3,
    reduction_reason = $4,
    reviewed_by_user_id = $5,
    reviewed_at = CURRENT_TIMESTAMP,
    updated_at = CURRENT_TIMESTAMP,
    version = version + 1
WHERE id = $1
RETURNING *;

-- name: CreateMaterialRequestItem :one
INSERT INTO material_request_items (
    id, request_id, item_id, requested_quantity, approved_quantity, unit_of_measure, created_at
) VALUES (
    $1, $2, $3, $4, $5, $6, CURRENT_TIMESTAMP
) RETURNING *;

-- name: ListMaterialRequestItemsByRequest :many
SELECT ri.*,
       i.sku,
       i.name as item_name,
       i.category as item_category
FROM material_request_items ri
JOIN inventory_items i ON ri.item_id = i.id
WHERE ri.request_id = $1
ORDER BY i.name;

-- name: UpdateMaterialRequestItemApprovedQty :one
UPDATE material_request_items
SET approved_quantity = $2
WHERE id = $1
RETURNING *;

-- name: CreateStockReservation :one
INSERT INTO stock_reservations (
    id, request_id, request_item_id, location_id, item_id, batch_number, quantity, status, expires_at, created_at, updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
) RETURNING *;

-- name: ListStockReservationsByRequest :many
SELECT sr.*,
       i.sku,
       i.name as item_name,
       sl.name as location_name
FROM stock_reservations sr
JOIN inventory_items i ON sr.item_id = i.id
JOIN storage_locations sl ON sr.location_id = sl.id
WHERE sr.request_id = $1;

-- name: UpdateStockReservationStatus :one
UPDATE stock_reservations
SET status = $2,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING *;

-- name: CreateMaterialTransfer :one
INSERT INTO material_transfers (
    id, reference, idempotency_key, request_id, source_location_id, destination_location_id,
    status, carrier_name, carrier_phone, tracking_number, dispatch_notes,
    created_at, updated_at, version
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 1
) RETURNING *;

-- name: GetMaterialTransferByID :one
SELECT t.*,
       src.name as source_location_name,
       src.code as source_location_code,
       dst.name as destination_location_name,
       dst.code as destination_location_code,
       u_disp.display_name as dispatched_by_name,
       u_recv.display_name as received_by_name
FROM material_transfers t
JOIN storage_locations src ON t.source_location_id = src.id
JOIN storage_locations dst ON t.destination_location_id = dst.id
LEFT JOIN users u_disp ON t.dispatched_by_user_id = u_disp.id
LEFT JOIN users u_recv ON t.received_by_user_id = u_recv.id
WHERE t.id = $1;

-- name: GetMaterialTransferByReference :one
SELECT * FROM material_transfers WHERE reference = $1;

-- name: GetMaterialTransferByIdempotencyKey :one
SELECT * FROM material_transfers WHERE idempotency_key = $1;

-- name: ListMaterialTransfers :many
SELECT t.*,
       src.name as source_location_name,
       dst.name as destination_location_name,
       u_disp.display_name as dispatched_by_name,
       u_recv.display_name as received_by_name
FROM material_transfers t
JOIN storage_locations src ON t.source_location_id = src.id
JOIN storage_locations dst ON t.destination_location_id = dst.id
LEFT JOIN users u_disp ON t.dispatched_by_user_id = u_disp.id
LEFT JOIN users u_recv ON t.received_by_user_id = u_recv.id
WHERE ($1::text IS NULL OR $1 = '' OR t.status = $1)
  AND ($2::text IS NULL OR $2 = '' OR t.source_location_id = $2)
  AND ($3::text IS NULL OR $3 = '' OR t.destination_location_id = $3)
ORDER BY t.created_at DESC;

-- name: MarkMaterialTransferDispatched :one
UPDATE material_transfers
SET status = 'dispatched',
    carrier_name = $2,
    carrier_phone = $3,
    tracking_number = $4,
    dispatch_notes = $5,
    dispatched_by_user_id = $6,
    dispatched_at = CURRENT_TIMESTAMP,
    updated_at = CURRENT_TIMESTAMP,
    version = version + 1
WHERE id = $1
RETURNING *;

-- name: MarkMaterialTransferReceived :one
UPDATE material_transfers
SET status = $2,
    received_by_user_id = $3,
    received_at = CURRENT_TIMESTAMP,
    updated_at = CURRENT_TIMESTAMP,
    version = version + 1
WHERE id = $1
RETURNING *;

-- name: CreateMaterialTransferItem :one
INSERT INTO material_transfer_items (
    id, transfer_id, item_id, batch_number, dispatched_quantity, received_quantity, discrepancy_quantity, unit_of_measure, created_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, CURRENT_TIMESTAMP
) RETURNING *;

-- name: ListMaterialTransferItemsByTransfer :many
SELECT ti.*,
       i.sku,
       i.name as item_name,
       i.category as item_category
FROM material_transfer_items ti
JOIN inventory_items i ON ti.item_id = i.id
WHERE ti.transfer_id = $1
ORDER BY i.name;

-- name: UpdateMaterialTransferItemReceipt :one
UPDATE material_transfer_items
SET received_quantity = $2,
    discrepancy_quantity = $3
WHERE id = $1
RETURNING *;

-- name: CreateTransferDiscrepancy :one
INSERT INTO transfer_discrepancies (
    id, transfer_id, transfer_item_id, expected_quantity, received_quantity, difference_quantity,
    reason, supporting_evidence_url, status, created_at, updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
) RETURNING *;

-- name: GetTransferDiscrepancyByID :one
SELECT d.*,
       t.reference as transfer_reference,
       t.source_location_id,
       t.destination_location_id,
       ti.item_id,
       src.name as source_location_name,
       dst.name as destination_location_name,
       i.name as item_name,
       i.sku as item_sku,
       u_rev.display_name as reviewer_name
FROM transfer_discrepancies d
JOIN material_transfers t ON d.transfer_id = t.id
JOIN storage_locations src ON t.source_location_id = src.id
JOIN storage_locations dst ON t.destination_location_id = dst.id
JOIN material_transfer_items ti ON d.transfer_item_id = ti.id
JOIN inventory_items i ON ti.item_id = i.id
LEFT JOIN users u_rev ON d.assigned_reviewer_id = u_rev.id
WHERE d.id = $1;

-- name: ListTransferDiscrepancies :many
SELECT d.*,
       t.reference as transfer_reference,
       t.source_location_id,
       t.destination_location_id,
       ti.item_id,
       src.name as source_location_name,
       dst.name as destination_location_name,
       i.name as item_name,
       i.sku as item_sku,
       u_rev.display_name as reviewer_name
FROM transfer_discrepancies d
JOIN material_transfers t ON d.transfer_id = t.id
JOIN storage_locations src ON t.source_location_id = src.id
JOIN storage_locations dst ON t.destination_location_id = dst.id
JOIN material_transfer_items ti ON d.transfer_item_id = ti.id
JOIN inventory_items i ON ti.item_id = i.id
LEFT JOIN users u_rev ON d.assigned_reviewer_id = u_rev.id
WHERE ($1::text IS NULL OR $1 = '' OR d.status = $1)
  AND ($2::text IS NULL OR $2 = '' OR d.transfer_id = $2)
ORDER BY d.created_at DESC;

-- name: ResolveTransferDiscrepancy :one
UPDATE transfer_discrepancies
SET status = $2,
    assigned_reviewer_id = $3,
    resolution_notes = $4,
    resolved_at = CURRENT_TIMESTAMP,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING *;

-- name: CreateStockLedgerEntry :one
INSERT INTO stock_ledger_entries (
    id, reference, location_id, item_id, movement_type, quantity, balance_after,
    batch_number, related_transfer_id, related_request_id, related_discrepancy_id,
    notes, actor_user_id, occurred_at, created_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, CURRENT_TIMESTAMP
) RETURNING *;

-- name: ListStockLedgerEntries :many
SELECT le.*,
       sl.name as location_name,
       sl.code as location_code,
       i.name as item_name,
       i.sku as item_sku,
       i.unit_of_measure,
       u.display_name as actor_name
FROM stock_ledger_entries le
JOIN storage_locations sl ON le.location_id = sl.id
JOIN inventory_items i ON le.item_id = i.id
LEFT JOIN users u ON le.actor_user_id = u.id
WHERE ($1::text IS NULL OR $1 = '' OR le.location_id = $1)
  AND ($2::text IS NULL OR $2 = '' OR le.item_id = $2)
  AND ($3::text IS NULL OR $3 = '' OR le.movement_type = $3)
  AND ($4::timestamptz IS NULL OR le.occurred_at >= $4)
  AND ($5::timestamptz IS NULL OR le.occurred_at <= $5)
ORDER BY le.occurred_at DESC, le.created_at DESC
LIMIT $6 OFFSET $7;

-- name: CountStockLedgerEntries :one
SELECT COUNT(*) FROM stock_ledger_entries le
WHERE ($1::text IS NULL OR $1 = '' OR le.location_id = $1)
  AND ($2::text IS NULL OR $2 = '' OR le.item_id = $2)
  AND ($3::text IS NULL OR $3 = '' OR le.movement_type = $3)
  AND ($4::timestamptz IS NULL OR le.occurred_at >= $4)
  AND ($5::timestamptz IS NULL OR le.occurred_at <= $5);

-- name: GetLatestStockLedgerEntry :one
SELECT * FROM stock_ledger_entries
WHERE location_id = $1 AND item_id = $2 AND batch_number = $3
ORDER BY occurred_at DESC, created_at DESC
LIMIT 1;
