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

export type GeopoliticalZone = Zone;

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

export interface FinancialDashboardSummary {
  total_verified_income_kobo: number;
  total_verified_expenditure_kobo: number;
  available_cash_balance_kobo: number;
  outstanding_commitments_kobo: number;
  available_to_spend_kobo: number;
  pending_approvals_count: number;
  pending_reconciliations_count: number;
  active_budgets_total_kobo: number;
  project_allocations_total_kobo: number;
  total_donations_received_kobo: number;
  monthly_trends: MonthlyTrendItem[];
  fund_balances: FundBalanceItem[];
  generated_at: string;
}

export interface MonthlyTrendItem {
  month_label: string;
  income_kobo: number;
  expenditure_kobo: number;
  net_savings_kobo: number;
}

export interface FundBalanceItem {
  fund_id: string;
  fund_name: string;
  balance_kobo: number;
}

export interface FinancialExpense {
  id: string;
  reference: string;
  title: string;
  category: string;
  description: string;
  amount_kobo: number;
  currency: string;
  beneficiary_name: string;
  beneficiary_bank_name?: string;
  beneficiary_account_number?: string;
  zone_id?: string;
  state_id?: string;
  lga_id?: string;
  zone_name?: string;
  state_name?: string;
  lga_name?: string;
  related_project_id?: string;
  related_campaign_id?: string;
  supporting_document_url?: string;
  receipt_url?: string;
  requested_by_user_id: string;
  requested_by_name?: string;
  reviewer_id?: string;
  reviewer_name?: string;
  reviewed_at?: string;
  approved_by_user_id?: string;
  approved_by_name?: string;
  approved_at?: string;
  rejection_reason?: string;
  authorized_by_user_id?: string;
  authorized_by_name?: string;
  authorized_at?: string;
  paid_by_user_id?: string;
  paid_by_name?: string;
  paid_at?: string;
  payment_reference?: string;
  transaction_id?: string;
  budget_id?: string;
  fund_id?: string;
  status: "draft" | "submitted" | "under_review" | "approved" | "rejected" | "authorized_for_payment" | "paid" | "reconciled";
  created_at: string;
  updated_at: string;
}

export interface FinancialBudget {
  id: string;
  reference: string;
  title: string;
  scope_type: "national" | "zone" | "state" | "lga" | "community_project" | "cleanup_campaign" | "foundation";
  scope_id?: string;
  zone_id?: string;
  state_id?: string;
  lga_id?: string;
  zone_name?: string;
  state_name?: string;
  lga_name?: string;
  period_type: "quarterly" | "annual" | "project_based" | "event_based";
  period_label: string;
  period_start: string;
  period_end: string;
  proposed_amount_kobo: number;
  approved_amount_kobo: number;
  allocated_amount_kobo: number;
  committed_amount_kobo: number;
  spent_amount_kobo: number;
  currency: string;
  status: "draft" | "submitted" | "approved" | "closed" | "revised";
  created_by_user_id: string;
  created_by_name?: string;
  approved_by_user_id?: string;
  approved_by_name?: string;
  approved_at?: string;
  notes?: string;
  created_at: string;
  updated_at: string;
}

export interface FinancialBudgetRevision {
  id: string;
  budget_id: string;
  revision_number: number;
  previous_amount_kobo: number;
  revised_amount_kobo: number;
  reason: string;
  requested_by_user_id: string;
  requested_by_name?: string;
  approved_by_user_id?: string;
  approved_by_name?: string;
  approved_at?: string;
  status: "pending" | "approved" | "rejected";
  created_at: string;
}

export interface FinancialAllocation {
  id: string;
  reference: string;
  source_type: string;
  source_id?: string;
  destination_type: string;
  destination_id?: string;
  source_fund_id: string;
  source_fund_name?: string;
  destination_fund_id: string;
  destination_fund_name?: string;
  amount_kobo: number;
  currency: string;
  purpose: string;
  approval_reference: string;
  status: "pending" | "disbursed" | "received" | "cancelled";
  allocated_by_user_id: string;
  allocated_by_name?: string;
  allocated_at: string;
  transaction_id?: string;
  created_at: string;
}

export interface FinancialDonation {
  id: string;
  reference: string;
  entity_type: "ratels" | "foundation";
  campaign_id?: string;
  campaign_title?: string;
  donor_name: string;
  donor_email: string;
  donor_phone?: string;
  is_anonymous: boolean;
  amount_kobo: number;
  currency: string;
  payment_method: string;
  payment_reference?: string;
  status: "pending" | "verified" | "failed" | "refunded";
  zone_id?: string;
  state_id?: string;
  destination_fund_id: string;
  transaction_id?: string;
  receipt_number?: string;
  verified_at?: string;
  notes?: string;
  created_at: string;
}

export interface FinancialBankAccount {
  id: string;
  bank_name: string;
  account_name: string;
  account_number: string;
  currency: string;
  org_type: "ratels" | "foundation";
  status: "active" | "inactive" | "pending_verification";
  is_primary: boolean;
  notes?: string;
  created_at: string;
}

export interface FinancialReconciliationPeriod {
  id: string;
  bank_account_id: string;
  bank_name?: string;
  account_name?: string;
  period_start: string;
  period_end: string;
  opening_balance_kobo: number;
  closing_balance_kobo: number;
  statement_closing_balance_kobo: number;
  status: "draft" | "in_review" | "reconciled" | "certified";
  reconciled_by_user_id?: string;
  reconciled_by_name?: string;
  reconciled_at?: string;
  certified_by_user_id?: string;
  certified_by_name?: string;
  certified_at?: string;
  notes?: string;
  created_at: string;
  items_count?: number;
  matched_count?: number;
}

export interface FinancialStatementItem {
  id: string;
  reconciliation_period_id: string;
  statement_reference: string;
  transaction_date: string;
  value_date?: string;
  description: string;
  amount_kobo: number;
  entry_type: "debit" | "credit";
  matched_transaction_id?: string;
  matched_tx_reference?: string;
  status: "unmatched" | "matched" | "flagged" | "manually_reconciled";
  match_notes?: string;
  created_at: string;
}

export interface FinancialDocument {
  document_type: string;
  document_reference: string;
  title: string;
  organization: string;
  reporting_period: string;
  geographic_scope: string;
  issue_date: string;
  amount_formatted: string;
  amount_in_words: string;
  header_data: Record<string, any>;
  line_items: Array<{
    description: string;
    reference: string;
    date: string;
    amount: string;
  }>;
  prepared_by: string;
  certified_by: string;
  certification_status: string;
  html_printable_body: string;
}

export interface PublicTransparencySummary {
  total_verified_income_kobo: number;
  total_verified_expenditure_kobo: number;
  total_project_funding_kobo: number;
  ratels_donations_total_kobo: number;
  foundation_donations_total_kobo: number;
  total_donations_count: number;
  available_reserve_balance_kobo: number;
  reporting_currency: string;
  certified_by: string;
  last_updated: string;
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
  order_status: "placed" | "processing" | "partially_dispatched" | "ready_for_pickup" | "dispatched" | "partially_delivered" | "delivered" | "cancelled" | "refunded" | "requires_reconciliation";
  payment_status: "pending" | "paid" | "failed" | "refunded";
  fulfilment_status: "unfulfilled" | "preparing" | "partially_dispatched" | "dispatched" | "partially_delivered" | "delivered" | "returned";
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
  business_id?: string;
  carrier_name: string;
  tracking_number: string;
  tracking_url?: string;
  status: "pending" | "processing" | "ready_for_pickup" | "dispatched" | "in_transit" | "delivered" | "failed" | "returned" | "cancelled";
  dispatched_at?: string;
  estimated_delivery_at?: string;
  delivered_at?: string;
  delivery_notes?: string;
  proof_of_delivery_url?: string;
  created_at: string;
  updated_at: string;
}

export interface ShipmentItemDetail {
  id: string;
  shipment_id: string;
  order_item_id: string;
  quantity: number;
  product_id?: string;
  title_snapshot: string;
  sku_snapshot: string;
  price_kobo_snapshot: number;
  image_url_snapshot?: string;
  business_id: string;
}

export interface ShipmentDetailResult {
  shipment: MarketplaceShipment;
  order?: MarketplaceOrder;
  business?: {
    id: string;
    name: string;
    slug?: string;
  };
  items: ShipmentItemDetail[];
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

// -------------------------------------------------------------
// Ratels Careers (Blueprint v2.0 Section 09)
// -------------------------------------------------------------

export interface CareersEmployer {
  id: string;
  owner_user_id: string;
  organization_name: string;
  slug: string;
  registration_number: string;
  industry: string;
  company_size: string;
  description: string;
  website_url?: string;
  contact_email: string;
  contact_phone: string;
  logo_url?: string;
  state_id: string;
  lga_id?: string;
  address: string;
  verification_status: "pending" | "verified" | "rejected" | "suspended";
  verified_by_user_id?: string;
  verified_at?: string;
  rejection_reason?: string;
  owner_name?: string;
  owner_email?: string;
  state_name?: string;
  lga_name?: string;
  total_jobs?: number;
  created_at: string;
  updated_at: string;
}

export interface CareersJob {
  id: string;
  employer_id: string;
  title: string;
  slug: string;
  category: string;
  employment_type: "full_time" | "part_time" | "contract" | "internship" | "volunteer";
  workplace_type: "on_site" | "remote" | "hybrid";
  state_id?: string;
  lga_id?: string;
  location_summary: string;
  salary_min_kobo?: number;
  salary_max_kobo?: number;
  currency: string;
  is_salary_negotiable: boolean;
  description: string;
  responsibilities: string;
  requirements: string;
  skills_required: string[] | any;
  experience_level: "entry" | "mid" | "senior" | "executive" | "no_experience";
  deadline_at?: string;
  status: "draft" | "pending_approval" | "published" | "paused" | "closed" | "expired" | "rejected";
  moderated_by_user_id?: string;
  moderated_at?: string;
  moderation_notes?: string;
  views_count: number;
  applications_count: number;
  employer_name?: string;
  employer_slug?: string;
  employer_logo?: string;
  employer_verification_status?: string;
  employer_industry?: string;
  employer_website?: string;
  state_name?: string;
  lga_name?: string;
  created_at: string;
  updated_at: string;
}

export interface CareersCandidateProfile {
  id: string;
  user_id: string;
  professional_headline: string;
  summary: string;
  skills: string[] | any;
  education: Array<{
    institution: string;
    degree: string;
    field_of_study?: string;
    start_year?: number;
    end_year?: number;
  }> | any;
  experience: Array<{
    company: string;
    title: string;
    location?: string;
    start_date?: string;
    end_date?: string;
    current?: boolean;
    description?: string;
  }> | any;
  certifications: Array<{
    name: string;
    issuing_org: string;
    year?: number;
    credential_id?: string;
  }> | any;
  preferred_categories: string[] | any;
  preferred_workplace_type: "any" | "on_site" | "remote" | "hybrid";
  preferred_state_id?: string;
  preferred_state_name?: string;
  resume_url?: string;
  resume_filename?: string;
  resume_uploaded_at?: string;
  is_searchable_by_verified_employers: boolean;
  display_name?: string;
  email?: string;
  phone?: string;
  membership_reference?: string;
  created_at: string;
  updated_at: string;
}

export interface CareersApplication {
  id: string;
  job_id: string;
  candidate_user_id: string;
  employer_id: string;
  cover_letter: string;
  qualifications_summary: string;
  resume_url: string;
  resume_filename: string;
  status: "submitted" | "under_review" | "shortlisted" | "interview" | "offered" | "hired" | "rejected" | "withdrawn";
  rejection_reason?: string;
  withdrawal_reason?: string;
  status_updated_at: string;
  match_score: number;
  match_factors?: {
    skills_score?: number;
    skills_matched?: string[];
    location_score?: number;
    category_score?: number;
    experience_score?: number;
    criteria_explanation?: string;
  } | any;
  job_title?: string;
  job_category?: string;
  job_employment_type?: string;
  job_workplace_type?: string;
  job_status?: string;
  employer_name?: string;
  employer_slug?: string;
  employer_logo?: string;
  candidate_name?: string;
  candidate_email?: string;
  candidate_phone?: string;
  candidate_membership_reference?: string;
  created_at: string;
  updated_at: string;
}

export interface CareersApplicationHistory {
  id: string;
  application_id: string;
  from_status: string;
  to_status: string;
  changed_by_user_id?: string;
  changed_by_name?: string;
  notes: string;
  created_at: string;
}

export interface CareersInterview {
  id: string;
  application_id: string;
  job_id: string;
  employer_id: string;
  candidate_user_id: string;
  scheduled_time: string;
  format: "video" | "in_person" | "phone";
  location_or_link: string;
  instructions: string;
  status: "scheduled" | "completed" | "cancelled" | "rescheduled";
  job_title?: string;
  employer_name?: string;
  employer_slug?: string;
  candidate_name?: string;
  candidate_email?: string;
  candidate_phone?: string;
  created_at: string;
  updated_at: string;
}

export interface CareersAdminOverview {
  total_employers: number;
  verified_employers: number;
  pending_employers: number;
  total_jobs: number;
  active_jobs: number;
  pending_approval_jobs: number;
  total_applications: number;
  total_hires: number;
}

export interface CareersEmployerOverview {
  total_jobs: number;
  active_jobs: number;
  total_applications: number;
  pending_applications: number;
  shortlisted_applications: number;
  interview_applications: number;
  hired_applications: number;
}

export function safeJsonParse<T = any>(val: any, fallback: T): T {
  if (val === null || val === undefined) return fallback;
  if (typeof val !== "string") return val as T;
  const trimmed = val.trim();
  if (!trimmed) return fallback;

  try {
    return JSON.parse(trimmed);
  } catch (_) {}

  try {
    let decoded = "";
    if (typeof window !== "undefined" && window.atob) {
      decoded = window.atob(trimmed);
    } else if (typeof Buffer !== "undefined") {
      decoded = Buffer.from(trimmed, "base64").toString("utf-8");
    }
    if (decoded) {
      return JSON.parse(decoded);
    }
  } catch (_) {}

  return fallback;
}

export const R2_PUBLIC_DOMAIN = "https://pub-82990f82f02446b5a11817fdce96912d.r2.dev";

/**
 * Normalizes any image URL or object key to an absolute, deliverable URL.
 */
export function resolveImageUrl(urlOrKey: string): string {
  if (!urlOrKey) return "";
  let clean = urlOrKey.trim();
  if (!clean) return "";

  // If already full http(s) URL, return directly
  if (clean.startsWith("http://") || clean.startsWith("https://")) {
    return clean;
  }

  // If data URI or blob URL, return as-is
  if (clean.startsWith("data:image/") || clean.startsWith("blob:")) {
    return clean;
  }

  // If it's a storage key (e.g. marketplace/products/...), resolve with public R2 domain
  clean = clean.replace(/^\/+/, "");
  return `${R2_PUBLIC_DOMAIN}/${clean}`;
}

/**
 * Resiliently parses product images from:
 * 1. JavaScript string[] or object[]
 * 2. Raw JSON string array: '["https://..."]'
 * 3. Base64-encoded JSON string array: "WyJodHRwczovL..." (from PostgreSQL JSONB)
 * 4. Single raw image URL or comma-separated URLs
 * Resolves any storage object keys to the public R2 domain.
 */
export function parseProductImages(raw: any): string[] {
  if (raw === null || raw === undefined) return [];

  // 1. Direct array
  if (Array.isArray(raw)) {
    return raw
      .map((item) => {
        if (typeof item === "string") return resolveImageUrl(item);
        if (item && typeof item === "object" && item.url) return resolveImageUrl(item.url);
        if (item && typeof item === "object" && item.public_url) return resolveImageUrl(item.public_url);
        return "";
      })
      .filter(Boolean);
  }

  // 2. String representation
  if (typeof raw === "string") {
    const trimmed = raw.trim();
    if (!trimmed) return [];

    // 2a. Direct JSON parse
    try {
      const parsed = JSON.parse(trimmed);
      if (Array.isArray(parsed)) {
        return parsed
          .map((item) => (typeof item === "string" ? resolveImageUrl(item) : ""))
          .filter(Boolean);
      }
    } catch (_) {}

    // 2b. Base64 encoded JSON string
    try {
      let decoded = "";
      if (typeof window !== "undefined" && (window as any).atob) {
        decoded = (window as any).atob(trimmed);
      } else if (typeof Buffer !== "undefined") {
        decoded = Buffer.from(trimmed, "base64").toString("utf-8");
      }
      if (decoded) {
        const parsed = JSON.parse(decoded);
        if (Array.isArray(parsed)) {
          return parsed
            .map((item) => (typeof item === "string" ? resolveImageUrl(item) : ""))
            .filter(Boolean);
        }
      }
    } catch (_) {}

    // 2c. Comma-separated URLs
    if (trimmed.includes(",")) {
      return trimmed
        .split(",")
        .map((s) => resolveImageUrl(s.trim()))
        .filter(Boolean);
    }

    // 2d. Single URL or object key
    if (trimmed.startsWith("http://") || trimmed.startsWith("https://") || trimmed.startsWith("marketplace/")) {
      return [resolveImageUrl(trimmed)];
    }
  }

  return [];
}

// -------------------------------------------------------------
// Homepage Hero Media Types
// -------------------------------------------------------------

export interface HeroMedia {
  id: string;
  title: string;
  video_key: string;
  video_url: string;
  poster_key?: string;
  poster_url?: string;
  content_type: string;
  file_size_bytes: number;
  duration_seconds?: number;
  is_active: boolean;
  created_by_user_id: string;
  published_by_user_id?: string;
  published_at?: string;
  created_at: string;
  updated_at: string;
}

export interface ActiveHeroMediaResponse {
  active: boolean;
  media: HeroMedia | null;
}

export interface PresignedHeroUploadResponse {
  id: string;
  upload_url: string;
  object_key: string;
  public_url: string;
  content_type: string;
  file_size_bytes: number;
  expires_in_seconds: number;
}

export interface ToggleHeroMediaResponse {
  active: boolean;
  message: string;
  media?: HeroMedia;
}

// -------------------------------------------------------------
// Community Projects & Requests Types (Blueprint Section 10)
// -------------------------------------------------------------

export interface CommunityRequest {
  id: string;
  reference: string;
  title: string;
  description: string;
  category: string;
  state_id: string;
  lga_id: string;
  state_name?: string;
  lga_name?: string;
  community_ward: string;
  location_details: string;
  estimated_urgency: "low" | "medium" | "high" | "critical";
  evidence_image_urls: string[] | any;
  requester_id: string;
  requester_name?: string;
  requester_email?: string;
  requester_phone?: string;
  requester_membership_reference?: string;
  status: "submitted" | "under_review" | "info_requested" | "verified" | "approved" | "rejected";
  reviewed_by_user_id?: string;
  reviewed_at?: string;
  reviewer_name?: string;
  review_notes?: string;
  rejection_reason?: string;
  info_request_notes?: string;
  escalation_level?: string;
  converted_to_project_id?: string;
  created_at: string;
  updated_at: string;
}

export interface CommunityProject {
  id: string;
  reference: string;
  title: string;
  description: string;
  community_request_id?: string;
  request_reference?: string;
  request_title?: string;
  category: string;
  state_id: string;
  lga_id: string;
  state_name?: string;
  lga_name?: string;
  community_ward: string;
  location_details: string;
  responsible_officer_id?: string;
  responsible_officer_name?: string;
  responsible_officer_email?: string;
  responsible_department: string;
  start_date?: string;
  target_completion_date?: string;
  actual_completion_date?: string;
  proposed_budget: number;
  approved_budget: number;
  allocated_funding: number;
  recorded_expenditure: number;
  outstanding_commitments: number;
  funding_status: "unfunded" | "partially_funded" | "fully_funded";
  execution_status: "planned" | "in_progress" | "awaiting_verification" | "completed" | "paused" | "cancelled";
  progress_percentage: number;
  pause_or_cancel_reason?: string;
  supporting_documents?: any;
  completion_evidence_urls?: string[] | any;
  verified_by_user_id?: string;
  verified_by_name?: string;
  verified_at?: string;
  verification_notes?: string;
  is_public: boolean;
  created_by_user_id: string;
  created_by_name?: string;
  created_at: string;
  updated_at: string;
}

export interface ProjectMilestone {
  id: string;
  project_id: string;
  sequence_order: number;
  title: string;
  description: string;
  target_date: string;
  completion_date?: string;
  assigned_person_name?: string;
  assigned_person_id?: string;
  assigned_person_user_name?: string;
  status: "planned" | "in_progress" | "awaiting_verification" | "completed";
  progress_notes?: string;
  evidence_photo_urls?: string[] | any;
  verification_status: "unverified" | "verified" | "rejected";
  verified_by_user_id?: string;
  verified_by_name?: string;
  verified_at?: string;
  verification_notes?: string;
  created_at: string;
  updated_at: string;
}

export interface ProjectUpdate {
  id: string;
  project_id: string;
  milestone_id?: string;
  milestone_title?: string;
  author_id: string;
  author_name?: string;
  title: string;
  content: string;
  photo_urls?: string[] | any;
  progress_percentage?: number;
  is_public: boolean;
  created_at: string;
}

export interface PublicProjectFinancialSummary {
  approved_budget_kobo: number;
  allocated_funding_kobo: number;
  recorded_expenditure_kobo: number;
  available_balance_kobo: number;
  funding_status: string;
}

export interface AdminProjectFinancialSummary {
  proposed_budget_kobo: number;
  approved_budget_kobo: number;
  allocated_funding_kobo: number;
  recorded_expenditure_kobo: number;
  outstanding_commitments_kobo: number;
  available_balance_kobo: number;
  funding_status: string;
}

export interface PublicProjectResponse {
  project: CommunityProject;
  financial_summary: PublicProjectFinancialSummary;
  milestones: ProjectMilestone[];
  updates: ProjectUpdate[];
}

export interface AdminProjectDetailResponse {
  project: CommunityProject;
  financial_summary: AdminProjectFinancialSummary;
  milestones: ProjectMilestone[];
  updates: ProjectUpdate[];
}

export interface CommunityProjectsOverviewStats {
  total_requests: number;
  pending_requests: number;
  approved_requests: number;
  active_projects: number;
  completed_projects: number;
  total_approved_budget: number;
  total_allocated_funding: number;
  total_recorded_expenditure: number;
}

// ============================================================================
// Event Media Gallery & Moderation System (Phase A)
// ============================================================================

export interface GalleryMedia {
  id: string;
  event_id: string;
  media_type: "image" | "video";
  caption: string;
  description?: string;
  public_url?: string;
  thumbnail_url?: string;
  mime_type: string;
  file_size_bytes: number;
  width?: number;
  height?: number;
  duration_seconds?: number;
  moderation_status?: "draft" | "pending_review" | "approved" | "rejected" | "revision_requested" | "withdrawn" | "removed";
  rejection_reason?: string;
  reviewed_by_user_id?: string;
  reviewed_at?: string;
  is_published: boolean;
  published_at?: string;
  is_hidden: boolean;
  is_featured: boolean;
  metadata_sanitized: boolean;
  sanitized_at?: string;
  allow_public_attribution: boolean;
  uploader_display_name?: string;
  event_title?: string;
  event_date?: string;
  state_id?: string;
  state_name?: string;
  state_code?: string;
  lga_id?: string;
  lga_name?: string;
  zone_id?: string;
  zone_name?: string;
  created_at: string;
  updated_at?: string;
  version?: number;
  uploaded_by_user_id?: string;
  uploader_name?: string;
  uploader_email?: string;
  reviewer_name?: string;
}

export interface GalleryConsent {
  id?: string;
  media_id?: string;
  has_owner_permission: boolean;
  understands_public_display: boolean;
  has_identifiable_consent: boolean;
  has_no_confidential_info: boolean;
  has_minors: boolean;
  has_guardian_consent: boolean;
  consent_notes?: string;
  declared_at?: string;
}

export interface MediaModerationAction {
  id: string;
  media_id: string;
  reviewer_user_id: string;
  reviewer_name?: string;
  action: string;
  reason: string;
  notes?: string;
  created_at: string;
}

export interface MediaReport {
  id: string;
  media_id: string;
  reporter_user_id?: string;
  reporter_name?: string;
  reporter_email?: string;
  report_reason: string;
  details: string;
  status: "pending" | "investigating" | "resolved" | "dismissed";
  resolution_notes?: string;
  resolver_name?: string;
  media_caption?: string;
  media_url?: string;
  media_thumbnail?: string;
  created_at: string;
  resolved_at?: string;
}

export interface MediaTakedownRequest {
  id: string;
  media_id: string;
  requester_user_id?: string;
  requester_name: string;
  requester_email: string;
  relationship_to_media: string;
  reason: string;
  details: string;
  status: "pending" | "approved_removed" | "rejected_retained";
  resolution_reason?: string;
  reviewer_name?: string;
  media_caption?: string;
  media_url?: string;
  media_thumbnail?: string;
  created_at: string;
  resolved_at?: string;
}

export interface GalleryOverviewStats {
  total_media: number;
  pending_review_count: number;
  approved_count: number;
  rejected_count: number;
  revision_requested_count: number;
  removed_count: number;
  hidden_count: number;
  pending_reports_count: number;
  pending_takedowns_count: number;
}

export interface PaginatedGalleryResponse<T = GalleryMedia> {
  items: T[];
  total: number;
  page: number;
  limit: number;
}

// -------------------------------------------------------------
// Phase B: Notifications & Communications Engine Types
// -------------------------------------------------------------

export interface InAppNotification {
  id: string;
  recipient_id: string;
  notification_type: string;
  title: string;
  body: string;
  related_resource_type?: string;
  related_resource_id?: string;
  action_url?: string;
  priority: "low" | "normal" | "high" | "urgent";
  is_read: boolean;
  read_at?: string;
  created_at: string;
  expires_at?: string;
}

export interface PaginatedNotificationsResponse {
  items: InAppNotification[];
  total: number;
  unread_count: number;
  page: number;
  page_size: number;
  total_pages: number;
}

export interface NotificationPreferences {
  user_id: string;
  in_app_enabled: boolean;
  email_enabled: boolean;
  sms_enabled: boolean;
  cleanup_reminders_enabled: boolean;
  leadership_alerts_enabled: boolean;
  participation_updates_enabled: boolean;
  marketplace_updates_enabled: boolean;
  careers_updates_enabled: boolean;
  community_project_updates_enabled: boolean;
  finance_alerts_enabled: boolean;
  announcements_enabled: boolean;
  sms_consent_recorded_at?: string;
  sms_consent_withdrawn_at?: string;
  updated_at: string;
}

export interface UpdateNotificationPreferencesRequest {
  in_app_enabled?: boolean;
  email_enabled?: boolean;
  sms_enabled?: boolean;
  cleanup_reminders_enabled?: boolean;
  leadership_alerts_enabled?: boolean;
  participation_updates_enabled?: boolean;
  marketplace_updates_enabled?: boolean;
  careers_updates_enabled?: boolean;
  community_project_updates_enabled?: boolean;
  finance_alerts_enabled?: boolean;
  announcements_enabled?: boolean;
}

export interface CommunicationsOverviewStats {
  total_count: number;
  pending_count: number;
  processing_count: number;
  delivered_count: number;
  failed_count: number;
  skipped_count: number;
}

export interface NotificationOutboxItem {
  id: string;
  event_type: string;
  recipient_id?: string;
  recipient_email?: string;
  recipient_phone?: string;
  recipient_name?: string;
  channel: "in_app" | "email" | "sms";
  payload: Record<string, any>;
  delivery_state: "pending" | "processing" | "delivered" | "failed" | "dead_letter" | "skipped";
  attempt_count: number;
  max_attempts: number;
  next_attempt_at: string;
  lease_token?: string;
  leased_until?: string;
  last_error?: string;
  last_error_category?: string;
  idempotency_key?: string;
  created_at: string;
  delivered_at?: string;
}

export interface NotificationDeliveryRecord {
  id: string;
  outbox_id: string;
  provider: string;
  channel: "in_app" | "email" | "sms";
  provider_message_ref?: string;
  status: "accepted" | "delivered" | "rejected" | "failed" | "skipped";
  failure_category?: string;
  error_message?: string;
  attempt_number: number;
  response_payload?: string;
  created_at: string;
}

export interface NotificationTemplate {
  id: string;
  template_code: string;
  channel: "in_app" | "email" | "sms";
  title_template: string;
  body_template: string;
  version: number;
  is_active: boolean;
  created_at: string;
  updated_at: string;
}

export interface NotificationSchedule {
  id: string;
  schedule_type: string;
  event_id: string;
  event_schedule_version: number;
  target_time: string;
  status: "pending" | "dispatched" | "cancelled" | "superseded";
  dispatched_at?: string;
  created_at: string;
  updated_at: string;
  event_title?: string;
  event_start_time?: string;
}

export interface ProviderHealthResponse {
  brevo_provider: {
    configured: boolean;
    provider: string;
    status: "ready" | "sandbox" | "unconfigured";
  };
  termii_provider: {
    configured: boolean;
    provider: string;
    status: "ready" | "sandbox" | "unconfigured";
  };
  outbox_worker: {
    status: string;
  };
  lagos_time: string;
}

// -------------------------------------------------------------
// Ratels Safeguarding & Confidential Reporting (Phase C)
// -------------------------------------------------------------

export interface SafeguardingPublicConfig {
  reporting_enabled?: boolean;
  is_enabled?: boolean;
  emergency_banner?: string;
  emergency_guidance_text?: string;
  max_evidence_bytes?: number;
  allowed_mime_types?: string[];
}

export interface SafeguardingSubmissionResponse {
  case_id?: string;
  public_reference: string;
  reference_number?: string;
  follow_up_secret: string;
  status: string;
  category?: string;
  risk_level?: string;
  created_at: string;
  advisory_notice?: string;
  instructions?: string;
}

export interface SafeguardingMessageItem {
  id: string;
  author_type?: "reporter" | "officer" | "system";
  sender_role?: "reporter" | "safeguarding_officer";
  content?: string;
  message?: string;
  author_name?: string;
  created_at: string;
}

export interface SafeguardingStatusResponse {
  public_reference: string;
  reference_number?: string;
  status: string;
  category: string;
  risk_level?: string;
  urgency?: string;
  is_minor_involved?: boolean;
  is_immediate_danger?: boolean;
  created_at?: string;
  submitted_at?: string;
  last_status_update_at?: string;
  resolution_summary?: string;
  messages: SafeguardingMessageItem[];
}

export interface SafeguardingCaseListItem {
  id: string;
  public_reference: string;
  reference_number?: string;
  category: string;
  risk_level?: string;
  urgency?: string;
  status: string;
  is_anonymous: boolean;
  is_minor_involved: boolean;
  is_escalated: boolean;
  incident_date?: string;
  state_name?: string;
  lga_name?: string;
  created_at: string;
  triage_notes?: string;
}

export interface SafeguardingEvidenceItem {
  id: string;
  file_name?: string;
  original_filename?: string;
  mime_type: string;
  file_size_bytes: number;
  sha256_hash: string;
  uploaded_at?: string;
  created_at?: string;
}

export interface SafeguardingAssignmentItem {
  id: string;
  officer_id?: string;
  officer_user_id?: string;
  officer_name: string;
  assigned_role?: string;
  status?: string;
  assigned_at?: string;
  created_at?: string;
}

export interface SafeguardingActionItem {
  id: string;
  action_type: string;
  actor_user_id?: string;
  actor_name?: string;
  summary?: string;
  reason?: string;
  created_at: string;
}

export interface SafeguardingReporterInfo {
  is_anonymous?: boolean;
  name?: string;
  email?: string;
  phone?: string;
  relationship?: string;
  contact_preference?: string;
}

export interface SafeguardingCaseDetails {
  id: string;
  public_reference: string;
  reference_number?: string;
  category: string;
  risk_level?: string;
  urgency?: string;
  status: string;
  is_anonymous: boolean;
  is_minor_involved: boolean;
  is_escalated: boolean;
  escalation_reason?: string;
  escalated_at?: string;
  incident_date?: string;
  state_id?: string;
  lga_id?: string;
  state_name?: string;
  lga_name?: string;
  decrypted_description?: string;
  narrative_decrypted?: string;
  reporter?: SafeguardingReporterInfo;
  reporter_info?: SafeguardingReporterInfo;
  triage_notes?: string;
  resolution_summary?: string;
  resolved_at?: string;
  closed_at?: string;
  legal_hold?: boolean;
  retention_scheduled_for?: string;
  created_at: string;
  updated_at: string;
  evidence: SafeguardingEvidenceItem[];
  assignments: SafeguardingAssignmentItem[];
  messages: SafeguardingMessageItem[];
  actions: SafeguardingActionItem[];
}

export interface SafeguardingAccessLogItem {
  id: string;
  case_id?: string;
  actor_user_id: string;
  actor_name: string;
  access_type: string;
  ip_address?: string;
  access_reason: string;
  created_at: string;
}

export interface SafeguardingOfficer {
  id: string;
  user_id: string;
  officer_role: string;
  display_name: string;
  email: string;
  is_active: boolean;
  appointed_at: string;
}

// -------------------------------------------------------------
// Phase D: Public Transparency, Reports & Accountability Types
// -------------------------------------------------------------

export interface PublicReportDefinition {
  id: string;
  report_type: string;
  name: string;
  description: string;
  reporting_scope: string;
  is_active: boolean;
  created_at: string;
}

export interface PublicOverviewData {
  completed_cleanups_count: number;
  total_registered_volunteers: number;
  verified_attendees_count: number;
  unique_participants_count: number;
  active_states_count: number;
  active_lgas_count: number;
  approved_projects_count: number;
  completed_projects_count: number;
  approved_income_kobo: number;
  approved_income_formatted: string;
  approved_expenditure_kobo: number;
  approved_expenditure_formatted: string;
  foundation_donations_kobo: number;
  foundation_donations_formatted: string;
  total_waste_collected_kg: number;
  reporting_period: string;
  scope: string;
}

export interface CleanupStateRow {
  state_id: string;
  state_name: string;
  cleanups_count: number;
  waste_collected_kg: number;
  verified_attendees: number;
}

export interface PublicCleanupsData {
  completed_cleanups_count: number;
  active_states_count: number;
  active_lgas_count: number;
  total_waste_collected_kg: number;
  total_waste_bags: number;
  total_duration_hours: number;
  verified_attendees_count: number;
  unique_participants_count: number;
  state_breakdown: CleanupStateRow[];
  reporting_period: string;
}

export interface PublicParticipationData {
  total_claims_submitted: number;
  approved_claims_count: number;
  rejected_claims_count: number;
  approval_rate_percent: number;
  unique_verified_members: number;
  unique_members_suppressed?: string;
  reporting_period: string;
}

export interface PublicCategorySpend {
  category: string;
  category_label: string;
  expense_count: number;
  total_kobo: number;
  amount_formatted: string;
}

export interface PublicFinanceData {
  approved_income_kobo: number;
  approved_income_formatted: string;
  approved_expenditure_kobo: number;
  approved_expenditure_formatted: string;
  net_surplus_deficit_kobo: number;
  net_surplus_deficit_formatted: string;
  total_approved_budget_kobo: number;
  total_approved_budget_formatted: string;
  budget_variance_kobo: number;
  budget_variance_formatted: string;
  spending_categories: PublicCategorySpend[];
  reporting_period: string;
}

export interface PublicFoundationData {
  verified_donations_count: number;
  total_donations_kobo: number;
  total_donations_formatted: string;
  provisional_entity_notice: string;
  strict_segregation_notice: string;
  payment_safety_switch_enabled: boolean;
  reporting_period: string;
}

export interface PublicProjectSummaryItem {
  id: string;
  reference: string;
  title: string;
  description: string;
  category: string;
  state_name: string;
  lga_name: string;
  community_ward: string;
  location_details: string;
  funding_status: string;
  execution_status: string;
  progress_percentage: number;
  approved_budget_kobo: number;
  approved_budget_formatted: string;
  recorded_expenditure_kobo: number;
  recorded_expenditure_formatted: string;
}

export interface PublicCommunityProjectsData {
  total_projects_count: number;
  completed_projects_count: number;
  in_progress_projects_count: number;
  planned_projects_count: number;
  total_approved_budget_kobo: number;
  total_approved_budget_formatted: string;
  total_recorded_expenditure_kobo: number;
  total_recorded_expenditure_formatted: string;
  projects: PublicProjectSummaryItem[];
  reporting_period: string;
}

export interface PublicPublicationDTO {
  publication_id: string;
  snapshot_id: string;
  public_reference: string;
  report_type: string;
  title: string;
  reporting_period: string;
  period_start: string;
  period_end: string;
  scope_type: string;
  zone_name?: string;
  state_name?: string;
  lga_name?: string;
  publication_version: number;
  data_hash: string;
  published_at: string;
  publisher_name: string;
  aggregated_data: any;
  correction_notice?: string;
}

export interface PaginatedPublicationsResponse {
  publications: PublicPublicationDTO[];
  total_count: number;
  page: number;
  page_size: number;
}

export interface ReportApprovalItem {
  id: string;
  reviewer_id: string;
  reviewer_name: string;
  reviewer_email: string;
  decision: string;
  decision_reason: string;
  governance_scope: string;
  created_at: string;
}

export interface ReportAuditItem {
  id: string;
  snapshot_id?: string;
  action: string;
  performed_by_id?: string;
  actor_name?: string;
  actor_email?: string;
  details: any;
  ip_address?: string;
  created_at: string;
}

export interface AdminReportExportDTO {
  id: string;
  snapshot_id: string;
  export_format: "pdf" | "csv";
  file_name: string;
  file_size_bytes: number;
  mime_type: string;
  checksum_algorithm: string;
  content_hash: string;
  storage_reference?: string;
  export_status: string;
  created_at: string;
  updated_at: string;
}

export interface AdminSnapshotDetailDTO {
  snapshot_id: string;
  public_reference: string;
  definition_id?: string;
  report_type: string;
  title: string;
  reporting_period: string;
  period_start: string;
  period_end: string;
  scope_type: string;
  zone_id?: string;
  zone_name?: string;
  state_id?: string;
  state_name?: string;
  lga_id?: string;
  lga_name?: string;
  status: string;
  publication_version: number;
  data_hash: string;
  created_by_user_id?: string;
  created_by_type?: string;
  creator_name: string;
  correction_notes?: string;
  aggregated_data: any;
  created_at: string;
  updated_at: string;
  approvals: ReportApprovalItem[];
  audit_logs: ReportAuditItem[];
  exports?: AdminReportExportDTO[];
}

export interface PaginatedSnapshotsResponse {
  snapshots: AdminSnapshotDetailDTO[];
  total_count: number;
  page: number;
  page_size: number;
}

export interface NextExpectedPeriod {
  period_label: string;
  period_type: "monthly" | "quarterly";
  expected_date: string;
  description: string;
}

export interface LastSchedulerRun {
  run_identifier: string;
  status: "running" | "completed" | "partial_failure" | "failed";
  trigger_source: "scheduled" | "admin_manual" | "external_cron";
  reporting_period: string;
  started_at: string;
  completed_at?: string;
  reports_evaluated: number;
  reports_generated: number;
  reports_skipped: number;
  reports_failed: number;
  actor_type: string;
  actor_name: string;
}

export interface AdminDashboardStats {
  total_snapshots: number;
  draft_reports: number;
  pending_review_reports: number;
  approved_reports: number;
  published_reports: number;
  withdrawn_reports: number;
  failed_generation_attempts: number;
  available_pdf_exports: number;
  available_csv_exports: number;
  last_scheduler_run?: LastSchedulerRun;
  next_expected_period: NextExpectedPeriod;
}

export interface SchedulerRunItem {
  id: string;
  run_identifier: string;
  trigger_source: "scheduled" | "admin_manual" | "external_cron";
  reporting_period: string;
  period_start: string;
  period_end: string;
  started_at: string;
  completed_at?: string;
  status: "running" | "completed" | "partial_failure" | "failed";
  reports_evaluated: number;
  reports_generated: number;
  reports_skipped: number;
  reports_failed: number;
  generated_references: string[];
  skipped_reasons: Record<string, string>;
  error_details: string[];
  actor_type: string;
  actor_user_id?: string;
  actor_name: string;
  created_at: string;
}

export interface PaginatedSchedulerRunsResponse {
  runs: SchedulerRunItem[];
  total_count: number;
  page: number;
  page_size: number;
}

export interface AdminExportItem {
  id: string;
  snapshot_id: string;
  public_reference: string;
  report_title: string;
  report_type: string;
  reporting_period: string;
  snapshot_status: string;
  export_format: "pdf" | "csv";
  file_name: string;
  file_size_bytes: number;
  mime_type: string;
  checksum_algorithm: string;
  content_hash: string;
  export_status: "generated" | "withdrawn" | "revoked";
  storage_verified: boolean;
  created_at: string;
  updated_at: string;
}

export interface PaginatedAdminExportsResponse {
  exports: AdminExportItem[];
  total_count: number;
  page: number;
  page_size: number;
}

export interface PaginatedAuditsResponse {
  audits: ReportAuditItem[];
  total_count: number;
  page: number;
  page_size: number;
}

export interface RunSchedulerPayload {
  target_period?: string;
  period_type?: "monthly" | "quarterly" | "both";
  report_type?: string;
  scope_type?: string;
  zone_id?: string;
  state_id?: string;
  lga_id?: string;
}

export interface SchedulerRunResult {
  run_id?: string;
  run_identifier?: string;
  status?: string;
  started_at?: string;
  completed_at?: string;
  period_label: string;
  period_start: string;
  period_end: string;
  periods_evaluated?: string[];
  reports_evaluated: number;
  reports_generated: number;
  reports_skipped: number;
  reports_failed: number;
  generated_references: string[];
  skipped_reasons: Record<string, string>;
  errors: string[];
  lock_acquired: boolean;
}





