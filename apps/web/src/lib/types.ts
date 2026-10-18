export interface User {
  id: string;
  email?: string;
  phone?: string;
  display_name: string;
  membership_reference: string;
  status: "pending_verification" | "active" | "suspended" | "closed";
  primary_role: "member" | "lga_representative" | "state_coordinator" | "zonal_coordinator" | "national_admin" | "super_admin";
  is_verified: boolean;
  created_at: string;
  profile?: MemberProfile;
  appointments?: Appointment[];
}

export interface TokenResponse {
  access_token: string;
  token_type: string;
  user: User;
}

export interface MemberProfile {
  id: string;
  home_state_id?: string;
  home_lga_id?: string;
  address?: string;
  bio?: string;
  avatar_url?: string;
  is_public_badges: boolean;
}

export interface Zone {
  id: string;
  code: string;
  name: string;
  states?: State[];
}

export interface State {
  id: string;
  zone_id: string;
  code: string;
  name: string;
  is_fct: boolean;
  lgas?: LGA[];
  capital_cities?: CapitalCity[];
}

export interface LGA {
  id: string;
  state_id: string;
  name: string;
  is_area_council: boolean;
  source_reference: string;
  revision_date: string;
}

export interface CapitalCity {
  id: string;
  state_id: string;
  name: string;
}

export interface PublicLeader {
  id?: string;
  role_title: string;
  role_type: string;
  jurisdiction_type: string;
  jurisdiction_id?: string;
  jurisdiction_name?: string;
  display_name: string;
  public_contact?: string;
  status: string;
}

export interface Appointment {
  id: string;
  user_id: string;
  user_display_name?: string;
  user_membership_ref?: string;
  role_title: string;
  role_type: string;
  jurisdiction_type: string;
  jurisdiction_id?: string;
  jurisdiction_name?: string;
  start_date: string;
  end_date?: string;
  status: "active" | "acting" | "expired" | "revoked";
  approval_reference: string;
  public_contact?: string;
  created_at: string;
}

export interface CleanupCampaign {
  id: string;
  year: number;
  month: number;
  scheduled_date: string;
  title: string;
  description?: string;
  status: string;
}

export interface EventInstance {
  id: string;
  campaign_id?: string;
  programme_type: string;
  title: string;
  state_id: string;
  state_name?: string;
  lga_id: string;
  lga_name?: string;
  address: string;
  landmark: string;
  latitude?: number;
  longitude?: number;
  start_time: string;
  end_time: string;
  capacity: number;
  registered_count: number;
  is_user_registered?: boolean;
  coordinator_id: string;
  coordinator_name?: string;
  public_contact: string;
  accessibility_notes?: string;
  safety_notes?: string;
  status: "draft" | "submitted" | "published" | "cancelled" | "completed";
  rescheduled_from?: string;
  rescheduled_reason?: string;
  created_at: string;
}

export interface EventRegistration {
  id: string;
  event_id: string;
  user_id: string;
  status: "registered" | "cancelled" | "attended";
  registered_at: string;
}

export interface ParticipationClaim {
  id: string;
  event_id: string;
  event_title?: string;
  event_date?: string;
  user_id: string;
  user_display_name?: string;
  user_membership_ref?: string;
  description: string;
  evidence_notes?: string;
  status: "pending" | "verified" | "rejected" | "appealed";
  submitted_at: string;
  reviewed_by_user_id?: string;
  review_decision?: string;
  review_reason?: string;
  reviewed_at?: string;
  appeals: ParticipationAppeal[];
}

export interface ParticipationAppeal {
  id: string;
  participation_id: string;
  appeal_reason: string;
  status: "pending" | "accepted" | "rejected";
  appealed_at: string;
  decision_reason?: string;
  reviewed_at?: string;
}

export interface BadgeAward {
  id: string;
  user_id: string;
  badge_code: string;
  badge_name: string;
  badge_description: string;
  qualifying_participation_id?: string;
  awarded_at: string;
  is_public: boolean;
}

export interface PublicFoundation {
  name: string;
  is_verified: boolean;
  mission_statement: string;
  disclaimer: string;
  donation_notice: string;
  bank_details_available: boolean;
  bank_instructions?: string;
}

export interface AdminOverview {
  current_role: string;
  current_jurisdiction_type: string;
  current_jurisdiction_name: string;
  total_members: number;
  pending_member_verifications: number;
  active_coordinators: number;
  published_activities: number;
  draft_venues_review_queue: number;
  unverified_participations_queue: number;
  pending_appeals_queue: number;
}

export interface AuditEvent {
  id: string;
  actor_id?: string;
  actor_role?: string;
  action: string;
  resource_type: string;
  resource_id: string;
  scope_type?: string;
  scope_id?: string;
  details?: string;
  created_at: string;
}

export interface FinancialFund {
  id: string;
  name: string;
  fund_type: string;
  is_active: boolean;
  created_at: string;
}

export interface FinancialTransaction {
  id: string;
  reference: string;
  idempotency_key?: string;
  transaction_type: string;
  status: "pending" | "posted" | "reversed" | "failed";
  amount: number; // in integer minor units (Kobo)
  currency: string;
  source_fund_id?: string;
  destination_fund_id?: string;
  source_fund_name?: string;
  destination_fund_name?: string;
  related_campaign_id?: string;
  related_project_id?: string;
  state_id?: string;
  zone_id?: string;
  state_name?: string;
  zone_name?: string;
  provider_reference?: string;
  supporting_evidence_url?: string;
  notes?: string;
  initiating_actor_id?: string;
  initiating_actor_type: string;
  initiating_actor_name?: string;
  approved_by_user_id?: string;
  approved_by_name?: string;
  approved_at?: string;
  is_external_income: boolean;
  occurred_at: string;
  recorded_at: string;
  reversal_of_transaction_id?: string;
  is_reversal: boolean;
  reversal_reason?: string;
  created_at: string;
}

export interface FinancialLedgerEntry {
  id: string;
  transaction_id: string;
  fund_id: string;
  fund_name: string;
  entry_type: "debit" | "credit";
  amount: number;
  currency: string;
  description?: string;
  created_at: string;
}

export interface FinancialStatement {
  report_reference: string;
  generated_at: string;
  generating_user: string;
  reporting_period_from?: string;
  reporting_period_to?: string;
  applied_filters: Record<string, any>;
  opening_balance: number;
  confirmed_inflows: number;
  confirmed_outflows: number;
  provider_fees: number;
  refunds_adjustments: number;
  closing_balance: number;
  cash_balance: number;
  commitments: number;
  available_to_spend: number;
  reconciliation_status: string;
  transactions: FinancialTransaction[];
}

export interface FinancialAuthorization {
  id: string;
  user_id: string;
  user_name?: string;
  user_email?: string;
  user_role?: string;
  granted_by_user_id: string;
  granted_by_name?: string;
  access_level: "viewer" | "submitter";
  status: "active" | "revoked";
  notes?: string;
  created_at: string;
  updated_at: string;
}

export interface AppointmentHistory {
  id: string;
  appointment_id: string;
  action: string;
  changed_by_user_id?: string;
  changed_by_name?: string;
  previous_status?: string;
  new_status: string;
  reason?: string;
  previous_user_id?: string;
  previous_user_name?: string;
  new_user_id?: string;
  new_user_name?: string;
  effective_date: string;
  timestamp: string;
}

// -------------------------------------------------------------
// Inventory & Material Distribution (Blueprint v2.0 Section 05)
// -------------------------------------------------------------

export interface InventoryItem {
  id: string;
  sku: string;
  name: string;
  description?: string;
  category: string;
  unit_of_measure: string;
  min_threshold_quantity: number;
  is_batch_tracked: boolean;
  is_active: boolean;
  created_at: string;
  updated_at: string;
}

export interface StorageLocation {
  id: string;
  code: string;
  name: string;
  location_type: "national" | "zonal" | "state" | "lga";
  zone_id?: string;
  zone_name?: string;
  state_id?: string;
  state_name?: string;
  lga_id?: string;
  lga_name?: string;
  address?: string;
  custodian_user_id?: string;
  custodian_name?: string;
  custodian_email?: string;
  contact_phone?: string;
  is_active: boolean;
  created_at: string;
  updated_at: string;
}

export interface StockBalance {
  id: string;
  location_id: string;
  item_id: string;
  sku: string;
  item_name: string;
  item_category: string;
  unit_of_measure: string;
  min_threshold_quantity: number;
  on_hand_quantity: number;
  reserved_quantity: number;
  available_quantity: number;
  batch_number: string;
  expiry_date?: string;
  location_name?: string;
  location_code?: string;
  location_type?: string;
  updated_at: string;
}

export interface MaterialRequest {
  id: string;
  reference: string;
  requesting_user_id: string;
  requesting_user_name?: string;
  requesting_user_email?: string;
  jurisdiction_type: "national" | "zonal" | "state" | "lga";
  jurisdiction_id?: string;
  destination_location_id: string;
  destination_location_name?: string;
  destination_location_code?: string;
  status: "submitted" | "approved" | "reduced" | "rejected" | "allocated" | "fulfilled" | "cancelled";
  intended_use: string;
  beneficiary_category?: string;
  required_date: string;
  notes?: string;
  rejection_reason?: string;
  reduction_reason?: string;
  reviewed_by_user_id?: string;
  reviewer_name?: string;
  reviewed_at?: string;
  created_at: string;
  updated_at: string;
}

export interface MaterialRequestItem {
  id: string;
  request_id: string;
  item_id: string;
  sku?: string;
  item_name?: string;
  item_category?: string;
  requested_quantity: number;
  approved_quantity: number;
  unit_of_measure: string;
  created_at: string;
}

export interface MaterialTransfer {
  id: string;
  reference: string;
  idempotency_key?: string;
  request_id?: string;
  source_location_id: string;
  source_location_name?: string;
  source_location_code?: string;
  destination_location_id: string;
  destination_location_name?: string;
  destination_location_code?: string;
  status: "draft" | "dispatched" | "partially_received" | "received" | "disputed" | "cancelled";
  carrier_name?: string;
  carrier_phone?: string;
  tracking_number?: string;
  dispatch_notes?: string;
  dispatched_by_user_id?: string;
  dispatched_by_name?: string;
  dispatched_at?: string;
  received_by_user_id?: string;
  received_by_name?: string;
  received_at?: string;
  created_at: string;
  updated_at: string;
}

export interface MaterialTransferItem {
  id: string;
  transfer_id: string;
  item_id: string;
  sku?: string;
  item_name?: string;
  item_category?: string;
  batch_number: string;
  dispatched_quantity: number;
  received_quantity: number;
  discrepancy_quantity: number;
  unit_of_measure: string;
  created_at: string;
}

export interface TransferDiscrepancy {
  id: string;
  transfer_id: string;
  transfer_reference?: string;
  transfer_item_id: string;
  source_location_name?: string;
  destination_location_name?: string;
  item_name?: string;
  item_sku?: string;
  expected_quantity: number;
  received_quantity: number;
  difference_quantity: number;
  reason: "missing_in_transit" | "damaged" | "expired" | "wrong_item" | "other";
  supporting_evidence_url?: string;
  status: "open" | "investigating" | "resolved_writeoff" | "resolved_returned";
  assigned_reviewer_id?: string;
  reviewer_name?: string;
  resolution_notes?: string;
  resolved_at?: string;
  created_at: string;
  updated_at: string;
}

export interface StockLedgerEntry {
  id: string;
  reference: string;
  location_id: string;
  location_name?: string;
  location_code?: string;
  item_id: string;
  item_name?: string;
  item_sku?: string;
  unit_of_measure?: string;
  movement_type: "initial_receipt" | "donation_in_kind" | "transfer_dispatch" | "transfer_receipt" | "discrepancy_adjustment" | "write_off_loss" | "write_off_damage" | "return";
  quantity: number;
  balance_after: number;
  batch_number: string;
  related_transfer_id?: string;
  related_request_id?: string;
  related_discrepancy_id?: string;
  notes?: string;
  actor_user_id?: string;
  actor_name?: string;
  occurred_at: string;
  created_at: string;
}

export interface InventorySummary {
  total_items: number;
  total_locations: number;
  in_transit_transfers: number;
  open_discrepancies: number;
  low_stock_alerts_count: number;
}

// -------------------------------------------------------------
// Ratels Marketplace Types (Blueprint v2.0 Section 07)
// -------------------------------------------------------------

export interface MarketplaceBusiness {
  id: string;
  owner_user_id: string;
  owner_name?: string;
  owner_email?: string;
  owner_phone?: string;
  name: string;
  slug: string;
  description: string;
  category: string;
  ownership_type: "movement_owned" | "member_owned" | "approved_partner";
  status: "pending_review" | "approved" | "suspended" | "rejected";
  contact_email: string;
  contact_phone: string;
  logo_url?: string;
  banner_url?: string;
  settlement_bank_name?: string;
  settlement_account_number?: string;
  settlement_account_name?: string;
  is_verified: boolean;
  verified_by_user_id?: string;
  verified_by_name?: string;
  verified_at?: string;
  rejection_reason?: string;
  total_products?: number;
  total_branches?: number;
  created_at: string;
  updated_at: string;
}

export interface BusinessBranch {
  id: string;
  business_id: string;
  branch_name: string;
  state_id: string;
  state_name?: string;
  lga_id?: string;
  lga_name?: string;
  address: string;
  contact_phone: string;
  manager_name?: string;
  is_headquarters: boolean;
  is_active: boolean;
  created_at: string;
  updated_at: string;
}

export interface MarketplaceProduct {
  id: string;
  business_id: string;
  business_name?: string;
  business_slug?: string;
  business_ownership_type?: string;
  business_owner_id?: string;
  branch_id?: string;
  branch_name?: string;
  branch_state_name?: string;
  title: string;
  slug: string;
  description: string;
  category: string;
  sku: string;
  price_kobo: number;
  compare_at_price_kobo?: number;
  stock_quantity: number;
  low_stock_threshold: number;
  images: string[] | any;
  weight_grams?: number;
  delivery_restrictions?: string;
  return_policy?: string;
  status: "draft" | "published" | "archived";
  is_featured: boolean;
  created_at: string;
  updated_at: string;
}

export interface MarketplaceOrderItem {
  id: string;
  order_id: string;
  product_id?: string;
  business_id: string;
  business_name?: string;
  business_slug?: string;
  title_snapshot: string;
  sku_snapshot: string;
  price_kobo_snapshot: number;
  quantity: number;
  subtotal_kobo: number;
  image_url_snapshot?: string;
  created_at: string;
}

export interface MarketplaceOrder {
  id: string;
  order_number: string;
  buyer_user_id?: string;
  buyer_name: string;
  buyer_email: string;
  buyer_phone: string;
  delivery_type: "delivery" | "pickup";
  shipping_address: string;
  shipping_state_id?: string;
  shipping_state_name?: string;
  shipping_lga_id?: string;
  shipping_lga_name?: string;
  pickup_branch_id?: string;
  pickup_branch_name?: string;
  pickup_branch_address?: string;
  subtotal_kobo: number;
  delivery_fee_kobo: number;
  total_kobo: number;
  currency: string;
  order_status: "placed" | "processing" | "ready_for_pickup" | "dispatched" | "delivered" | "cancelled" | "refunded";
  payment_status: "pending" | "paid" | "failed" | "refunded";
  fulfilment_status: "unfulfilled" | "preparing" | "dispatched" | "delivered" | "returned";
  idempotency_key?: string;
  notes?: string;
  item_count?: number;
  created_at: string;
  updated_at: string;
}

export interface MarketplacePayment {
  id: string;
  order_id: string;
  order_number?: string;
  buyer_name?: string;
  buyer_email?: string;
  reference: string;
  provider: string;
  provider_reference?: string;
  channel?: string;
  amount_kobo: number;
  currency: string;
  status: "pending" | "successful" | "failed" | "refunded";
  paid_at?: string;
  error_message?: string;
  created_at: string;
  updated_at: string;
}

export interface MarketplaceShipment {
  id: string;
  order_id: string;
  carrier_name: string;
  tracking_number: string;
  tracking_url?: string;
  status: "dispatched" | "in_transit" | "delivered" | "failed" | "returned";
  dispatched_at: string;
  estimated_delivery_at?: string;
  delivered_at?: string;
  delivery_notes?: string;
  proof_of_delivery_url?: string;
  created_at: string;
  updated_at: string;
}

export interface MarketplaceDispute {
  id: string;
  order_id: string;
  order_number?: string;
  total_kobo?: number;
  raised_by_user_id: string;
  raised_by_name?: string;
  raised_by_email?: string;
  reason: "damaged_goods" | "missing_delivery" | "item_mismatch" | "cancelled_before_dispatch" | "other";
  customer_notes: string;
  status: "open" | "under_review" | "resolved_refunded" | "rejected";
  resolution_notes?: string;
  resolved_by_user_id?: string;
  resolved_by_name?: string;
  resolved_at?: string;
  created_at: string;
  updated_at: string;
}

export interface CartItem {
  productId: string;
  title: string;
  priceKobo: number;
  quantity: number;
  imageUrl?: string;
  sku: string;
  businessName?: string;
}

export interface MarketplaceAdminOverviewMetrics {
  total_gmv_kobo: number;
  total_orders_count: number;
  active_orders_count: number;
  pending_businesses_count: number;
  approved_businesses_count: number;
  active_products_count: number;
  open_disputes_count: number;
}

export interface MarketplaceSellerOverviewMetrics {
  seller_gross_sales_kobo: number;
  seller_orders_count: number;
  seller_pending_orders_count: number;
  seller_products_count: number;
}


