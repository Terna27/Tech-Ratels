-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied

-- 1. Inventory Items (Operational Movement Materials)
CREATE TABLE IF NOT EXISTS inventory_items (
    id VARCHAR(36) PRIMARY KEY,
    sku VARCHAR(50) UNIQUE NOT NULL,
    name VARCHAR(150) NOT NULL,
    description TEXT,
    category VARCHAR(50) NOT NULL,
    unit_of_measure VARCHAR(30) NOT NULL DEFAULT 'pcs',
    min_threshold_quantity BIGINT NOT NULL DEFAULT 10,
    is_batch_tracked BOOLEAN NOT NULL DEFAULT FALSE,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    version INT NOT NULL DEFAULT 1
);
CREATE INDEX IF NOT EXISTS idx_inventory_items_sku ON inventory_items(sku);
CREATE INDEX IF NOT EXISTS idx_inventory_items_category ON inventory_items(category);
CREATE INDEX IF NOT EXISTS idx_inventory_items_active ON inventory_items(is_active);

-- 2. Storage Locations (Warehouses / Depots at National, Zonal, State, and LGA levels)
CREATE TABLE IF NOT EXISTS storage_locations (
    id VARCHAR(36) PRIMARY KEY,
    code VARCHAR(50) UNIQUE NOT NULL,
    name VARCHAR(150) NOT NULL,
    location_type VARCHAR(20) NOT NULL, -- 'national', 'zonal', 'state', 'lga'
    zone_id VARCHAR(36) REFERENCES zones(id) ON DELETE SET NULL,
    state_id VARCHAR(36) REFERENCES states(id) ON DELETE SET NULL,
    lga_id VARCHAR(36) REFERENCES lgas(id) ON DELETE SET NULL,
    address TEXT,
    custodian_user_id VARCHAR(36) REFERENCES users(id) ON DELETE SET NULL,
    contact_phone VARCHAR(50),
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    version INT NOT NULL DEFAULT 1
);
CREATE INDEX IF NOT EXISTS idx_storage_locations_code ON storage_locations(code);
CREATE INDEX IF NOT EXISTS idx_storage_locations_type ON storage_locations(location_type);
CREATE INDEX IF NOT EXISTS idx_storage_locations_state ON storage_locations(state_id);
CREATE INDEX IF NOT EXISTS idx_storage_locations_zone ON storage_locations(zone_id);
CREATE INDEX IF NOT EXISTS idx_storage_locations_lga ON storage_locations(lga_id);
CREATE INDEX IF NOT EXISTS idx_storage_locations_custodian ON storage_locations(custodian_user_id);

-- 3. Stock Balances (Real-time on-hand, reserved, and available stock per location)
CREATE TABLE IF NOT EXISTS stock_balances (
    id VARCHAR(36) PRIMARY KEY,
    location_id VARCHAR(36) NOT NULL REFERENCES storage_locations(id) ON DELETE CASCADE,
    item_id VARCHAR(36) NOT NULL REFERENCES inventory_items(id) ON DELETE CASCADE,
    on_hand_quantity BIGINT NOT NULL DEFAULT 0 CHECK (on_hand_quantity >= 0),
    reserved_quantity BIGINT NOT NULL DEFAULT 0 CHECK (reserved_quantity >= 0),
    batch_number VARCHAR(100) NOT NULL DEFAULT '',
    expiry_date TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    version INT NOT NULL DEFAULT 1,
    CONSTRAINT uq_stock_balances_loc_item_batch UNIQUE (location_id, item_id, batch_number),
    CONSTRAINT chk_stock_balances_reserve CHECK (on_hand_quantity >= reserved_quantity)
);
CREATE INDEX IF NOT EXISTS idx_stock_balances_lookup ON stock_balances(location_id, item_id);

-- 4. Material Requests
CREATE TABLE IF NOT EXISTS material_requests (
    id VARCHAR(36) PRIMARY KEY,
    reference VARCHAR(50) UNIQUE NOT NULL,
    requesting_user_id VARCHAR(36) NOT NULL REFERENCES users(id),
    jurisdiction_type VARCHAR(20) NOT NULL, -- 'national', 'zonal', 'state', 'lga'
    jurisdiction_id VARCHAR(36),
    destination_location_id VARCHAR(36) NOT NULL REFERENCES storage_locations(id),
    status VARCHAR(30) NOT NULL DEFAULT 'submitted', -- 'submitted', 'approved', 'reduced', 'rejected', 'allocated', 'fulfilled', 'cancelled'
    intended_use TEXT NOT NULL,
    beneficiary_category VARCHAR(100),
    required_date TIMESTAMPTZ NOT NULL,
    notes TEXT,
    rejection_reason TEXT,
    reduction_reason TEXT,
    reviewed_by_user_id VARCHAR(36) REFERENCES users(id),
    reviewed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    version INT NOT NULL DEFAULT 1
);
CREATE INDEX IF NOT EXISTS idx_material_requests_ref ON material_requests(reference);
CREATE INDEX IF NOT EXISTS idx_material_requests_user ON material_requests(requesting_user_id);
CREATE INDEX IF NOT EXISTS idx_material_requests_status ON material_requests(status);
CREATE INDEX IF NOT EXISTS idx_material_requests_dest ON material_requests(destination_location_id);
CREATE INDEX IF NOT EXISTS idx_material_requests_jurisdiction ON material_requests(jurisdiction_type, jurisdiction_id);

-- 5. Material Request Items
CREATE TABLE IF NOT EXISTS material_request_items (
    id VARCHAR(36) PRIMARY KEY,
    request_id VARCHAR(36) NOT NULL REFERENCES material_requests(id) ON DELETE CASCADE,
    item_id VARCHAR(36) NOT NULL REFERENCES inventory_items(id),
    requested_quantity BIGINT NOT NULL CHECK (requested_quantity > 0),
    approved_quantity BIGINT NOT NULL DEFAULT 0 CHECK (approved_quantity >= 0),
    unit_of_measure VARCHAR(30) NOT NULL DEFAULT 'pcs',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_material_request_items_req ON material_request_items(request_id);
CREATE INDEX IF NOT EXISTS idx_material_request_items_item ON material_request_items(item_id);

-- 6. Stock Reservations
CREATE TABLE IF NOT EXISTS stock_reservations (
    id VARCHAR(36) PRIMARY KEY,
    request_id VARCHAR(36) NOT NULL REFERENCES material_requests(id) ON DELETE CASCADE,
    request_item_id VARCHAR(36) REFERENCES material_request_items(id) ON DELETE CASCADE,
    location_id VARCHAR(36) NOT NULL REFERENCES storage_locations(id),
    item_id VARCHAR(36) NOT NULL REFERENCES inventory_items(id),
    batch_number VARCHAR(100) NOT NULL DEFAULT '',
    quantity BIGINT NOT NULL CHECK (quantity > 0),
    status VARCHAR(20) NOT NULL DEFAULT 'active', -- 'active', 'dispatched', 'released'
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_stock_reservations_req ON stock_reservations(request_id);
CREATE INDEX IF NOT EXISTS idx_stock_reservations_loc_item ON stock_reservations(location_id, item_id, status);

-- 7. Material Transfers (Dispatches & In-Transit Tracking)
CREATE TABLE IF NOT EXISTS material_transfers (
    id VARCHAR(36) PRIMARY KEY,
    reference VARCHAR(50) UNIQUE NOT NULL,
    idempotency_key VARCHAR(150) UNIQUE,
    request_id VARCHAR(36) REFERENCES material_requests(id) ON DELETE SET NULL,
    source_location_id VARCHAR(36) NOT NULL REFERENCES storage_locations(id),
    destination_location_id VARCHAR(36) NOT NULL REFERENCES storage_locations(id),
    status VARCHAR(30) NOT NULL DEFAULT 'draft', -- 'draft', 'dispatched', 'partially_received', 'received', 'disputed', 'cancelled'
    carrier_name VARCHAR(150),
    carrier_phone VARCHAR(50),
    tracking_number VARCHAR(100),
    dispatch_notes TEXT,
    dispatched_by_user_id VARCHAR(36) REFERENCES users(id),
    dispatched_at TIMESTAMPTZ,
    received_by_user_id VARCHAR(36) REFERENCES users(id),
    received_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    version INT NOT NULL DEFAULT 1,
    CONSTRAINT chk_transfers_distinct_locations CHECK (source_location_id <> destination_location_id)
);
CREATE INDEX IF NOT EXISTS idx_material_transfers_ref ON material_transfers(reference);
CREATE INDEX IF NOT EXISTS idx_material_transfers_idempotency ON material_transfers(idempotency_key);
CREATE INDEX IF NOT EXISTS idx_material_transfers_src ON material_transfers(source_location_id);
CREATE INDEX IF NOT EXISTS idx_material_transfers_dest ON material_transfers(destination_location_id);
CREATE INDEX IF NOT EXISTS idx_material_transfers_status ON material_transfers(status);

-- 8. Material Transfer Items
CREATE TABLE IF NOT EXISTS material_transfer_items (
    id VARCHAR(36) PRIMARY KEY,
    transfer_id VARCHAR(36) NOT NULL REFERENCES material_transfers(id) ON DELETE CASCADE,
    item_id VARCHAR(36) NOT NULL REFERENCES inventory_items(id),
    batch_number VARCHAR(100) NOT NULL DEFAULT '',
    dispatched_quantity BIGINT NOT NULL CHECK (dispatched_quantity > 0),
    received_quantity BIGINT NOT NULL DEFAULT 0 CHECK (received_quantity >= 0),
    discrepancy_quantity BIGINT NOT NULL DEFAULT 0 CHECK (discrepancy_quantity >= 0),
    unit_of_measure VARCHAR(30) NOT NULL DEFAULT 'pcs',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_material_transfer_items_tx ON material_transfer_items(transfer_id);
CREATE INDEX IF NOT EXISTS idx_material_transfer_items_item ON material_transfer_items(item_id);

-- 9. Transfer Discrepancies
CREATE TABLE IF NOT EXISTS transfer_discrepancies (
    id VARCHAR(36) PRIMARY KEY,
    transfer_id VARCHAR(36) NOT NULL REFERENCES material_transfers(id) ON DELETE CASCADE,
    transfer_item_id VARCHAR(36) NOT NULL REFERENCES material_transfer_items(id) ON DELETE CASCADE,
    expected_quantity BIGINT NOT NULL,
    received_quantity BIGINT NOT NULL,
    difference_quantity BIGINT NOT NULL CHECK (difference_quantity > 0),
    reason VARCHAR(50) NOT NULL DEFAULT 'missing_in_transit', -- 'missing_in_transit', 'damaged', 'expired', 'wrong_item', 'other'
    supporting_evidence_url TEXT,
    status VARCHAR(30) NOT NULL DEFAULT 'open', -- 'open', 'investigating', 'resolved_writeoff', 'resolved_returned'
    assigned_reviewer_id VARCHAR(36) REFERENCES users(id),
    resolution_notes TEXT,
    resolved_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_transfer_discrepancies_tx ON transfer_discrepancies(transfer_id);
CREATE INDEX IF NOT EXISTS idx_transfer_discrepancies_status ON transfer_discrepancies(status);

-- 10. Append-Only Immutable Stock Ledger
CREATE TABLE IF NOT EXISTS stock_ledger_entries (
    id VARCHAR(36) PRIMARY KEY,
    reference VARCHAR(100) UNIQUE NOT NULL,
    location_id VARCHAR(36) NOT NULL REFERENCES storage_locations(id),
    item_id VARCHAR(36) NOT NULL REFERENCES inventory_items(id),
    movement_type VARCHAR(50) NOT NULL, -- 'initial_receipt', 'donation_in_kind', 'transfer_dispatch', 'transfer_receipt', 'discrepancy_adjustment', 'write_off_loss', 'write_off_damage', 'return'
    quantity BIGINT NOT NULL, -- Signed: positive for inflow, negative for outflow
    balance_after BIGINT NOT NULL,
    batch_number VARCHAR(100) NOT NULL DEFAULT '',
    related_transfer_id VARCHAR(36) REFERENCES material_transfers(id),
    related_request_id VARCHAR(36) REFERENCES material_requests(id),
    related_discrepancy_id VARCHAR(36) REFERENCES transfer_discrepancies(id),
    notes TEXT,
    actor_user_id VARCHAR(36) REFERENCES users(id),
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_stock_ledger_loc_item ON stock_ledger_entries(location_id, item_id, occurred_at);
CREATE INDEX IF NOT EXISTS idx_stock_ledger_ref ON stock_ledger_entries(reference);
CREATE INDEX IF NOT EXISTS idx_stock_ledger_type ON stock_ledger_entries(movement_type);

-- Seed standard initial operational inventory items
INSERT INTO inventory_items (id, sku, name, description, category, unit_of_measure, min_threshold_quantity) VALUES
    ('itm_vest_reflective', 'RAT-SAN-001', 'High-Visibility Reflective Sanitation Vest', 'High-visibility fluorescent neon orange vest with reflective safety tape and Ratels crest', 'sanitation_gear', 'pcs', 25),
    ('itm_gloves_rubber', 'RAT-SAN-002', 'Heavy-Duty Industrial Rubber Gloves', 'Thick puncture-resistant elbow-length nitrile/rubber protective work gloves', 'sanitation_gear', 'pairs', 50),
    ('itm_boots_safety', 'RAT-PPE-001', 'Reinforced Protective Rubber Work Boots', 'Heavy-duty steel-toe waterproof rubber work boots for community cleanup operations', 'protective_equipment', 'pairs', 20),
    ('itm_masks_n95', 'RAT-PPE-002', 'Particulate N95 Respirator Masks (Box of 20)', 'Protective particulate filter dust masks for waste disposal and sanitation', 'protective_equipment', 'boxes', 30),
    ('itm_trash_bags_heavy', 'RAT-TLS-001', 'Heavy-Duty Bio-Degradable Trash Bags (Pack of 50)', 'Extra-thick 240-litre tear-resistant municipal waste disposal bags', 'tools', 'bags', 50),
    ('itm_cleanup_rakes', 'RAT-TLS-002', 'All-Steel Waste Collection Rakes', 'Reinforced steel 14-tine rakes with wooden handles for outdoor debris cleanup', 'tools', 'pcs', 15),
    ('itm_cleanup_shovels', 'RAT-TLS-003', 'Square Point Metal Cleanup Shovels', 'Industrial carbon steel trench shovels for drainage dredging and refuse removal', 'tools', 'pcs', 15),
    ('itm_wheelbarrows', 'RAT-TLS-004', 'Heavy-Duty Single-Wheel Steel Wheelbarrows', 'Seamless deep steel tray wheelbarrows with pneumatic puncture-proof tire', 'tools', 'pcs', 10),
    ('itm_first_aid_kit', 'RAT-AID-001', 'Emergency Comprehensive Field First Aid Kit', 'Emergency medical responder kit with antiseptics, bandages, shears, and burn dressings', 'first_aid', 'kits', 10),
    ('itm_canopy_tent', 'RAT-PRO-001', 'Ratels Mobile Outreach Canopy (3m x 3m)', 'Pop-up waterproof branded shade canopy tent for community mobilization centers', 'promotional_materials', 'sets', 5),
    ('itm_banner_rollup', 'RAT-PRO-002', 'Ratels Roll-Up Awareness Banner', 'Retractable aluminum base banner with durable weather-resistant print', 'promotional_materials', 'pcs', 10),
    ('itm_volunteer_badges', 'RAT-IDG-001', 'Official Laminated Volunteer Identification Badges (Pack of 100)', 'Security-encoded member credentials with branded Ratels lanyards', 'identity_gear', 'packs', 20)
ON CONFLICT (sku) DO NOTHING;

-- +goose Down
-- SQL in section 'Down' is executed when this migration is rolled back

DROP TABLE IF EXISTS stock_ledger_entries;
DROP TABLE IF EXISTS transfer_discrepancies;
DROP TABLE IF EXISTS material_transfer_items;
DROP TABLE IF EXISTS material_transfers;
DROP TABLE IF EXISTS stock_reservations;
DROP TABLE IF EXISTS material_request_items;
DROP TABLE IF EXISTS material_requests;
DROP TABLE IF EXISTS stock_balances;
DROP TABLE IF EXISTS storage_locations;
DROP TABLE IF EXISTS inventory_items;
