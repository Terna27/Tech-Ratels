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
