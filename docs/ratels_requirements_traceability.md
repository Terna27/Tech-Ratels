# Nigerian Ratels Management System — Requirements Traceability Matrix (RTM)

> **Document Version:** 1.0  
> **Date:** 19 October 2026  
> **Reference Baseline:** Nigerian Ratels Complete Blueprint v2.0 (17 October 2026)  
> **Source Repository:** `~/Tech-Ratels`  
> **Production API:** `https://ratels.onrender.com`  
> **Production Web:** `https://ratels-web.onrender.com`  

---

## 1. Traceability Methodology & Status Legend

This matrix maps every requirement from the authoritative blueprint (`Nigerian_Ratels_Complete_Blueprint_v2.pdf`, Version 2.0) to its technical implementation in the codebase and live production environment.

### Status Definitions

- **Complete and verified**: Implemented in database, backend services, frontend UI, automated tests pass, and verified in production.
- **Implemented but not fully verified**: Implemented in codebase and passing local tests, but pending production configuration, live third-party credentials, or production data seeding.
- **Partially implemented**: Core functionality exists, but specific workflows or edge cases from the blueprint are incomplete.
- **Not implemented**: No database tables, Go handlers, or frontend pages exist for this requirement.
- **Blocked by configuration**: Implementation is complete, but cannot be verified due to environment, credential, or daemon permission blocks.
- **Not assessable**: Cannot be evaluated without external legal, regulatory, or partner data.

---

## 2. Requirements Traceability Matrix

### 2.1 Geography & Leadership Hierarchy

| Req ID | Requirement Description | Blueprint Ref | Existing Implementation | Frontend Route | Backend API | Database Tables | Test Coverage | Status | Missing Functionality | Severity | Recommended Action |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- | :---: | :--- | :---: | :--- |
| **GEO-01** | 6 Geopolitical Zones | Sec 02 | `internal/geodata/dataset.go`, `geography_service.go` | `/admin/leadership`, `/leadership` | `GET /api/v1/geography/zones` | `geopolitical_zones` | `geodata_test.go`, `geography_test.go` | **Implemented but not fully verified** | None in code. Production Neon DB unseeded. | Medium | Run `go run ./cmd/seed` on production Neon DB. |
| **GEO-02** | 36 States + 1 FCT modeled as a federal territory (`is_fct=true`), not a 37th state | Sec 02 [R3] | `internal/geodata/dataset.go` (FCT flagged `IsFCT: true`) | `/admin/leadership`, `/leadership` | `GET /api/v1/geography/states` | `states` (`is_fct BOOLEAN`) | `geodata_test.go` | **Implemented but not fully verified** | None in code. Production Neon DB unseeded. | Medium | Seed reference data to Neon DB. |
| **GEO-03** | 774 Local Government Areas & FCT Area Councils (768 LGAs + 6 Area Councils) | Sec 02 | Full Constitution 1999 reference dataset in `internal/geodata/dataset.go` | Location dropdowns across web app | `GET /api/v1/geography/states/{id}/lgas` | `lgas` | `TestGeodata_CompletenessAndInvariants` | **Implemented but not fully verified** | None in code. Production Neon DB unseeded. | Medium | Seed reference data to Neon DB. |
| **GEO-04** | State Capitals decoupled from local venue areas | Sec 02 | `capital_city` column in `states` table; distinct from `activity_events.landmark/venue` | `/activities/[id]`, `/admin/activities` | `GET /api/v1/activities/events` | `states`, `activity_events` | `geography_test.go`, `cleanup_test.go` | **Complete and verified** | None | Low | Operational maintenance. |
| **LEAD-01** | Leadership directory with named appointments, terms, and photos | Sec 02 | `leadership_service.go`, `leadership_handler.go` | `/leadership`, `/admin/leadership` | `GET /api/v1/leadership/directory`, `POST /api/v1/admin/leadership/appointments` | `appointments` | `harden_integration_test.go` | **Complete and verified** | None | Low | Appoint regional leaders. |
| **LEAD-02** | Scoped RBAC by Jurisdiction (National, Zone, State, LGA) | Sec 03 | `middleware.go`, `leadership_service.go` | Admin workspace routing | Scope checked on all `/api/v1/*` routes | `appointments`, `users` | `scoped_permissions_test.go`, `TestPermissionsAndCrossStateIsolationHTTP` | **Complete and verified** | None | Critical | Enforce in all new modules. |
| **LEAD-03** | Immediate permission revocation on appointment expiry or removal | Sec 02, 03 | Appointment active status check in `middleware.go` | Session rejection in admin workspace | Handled in middleware query | `appointments` (`status='revoked'/'expired'`) | `TestExpiredAppointmentRevokesAccessHTTP` | **Complete and verified** | None | High | Maintained in regression suite. |

---

### 2.2 Membership & Participation

| Req ID | Requirement Description | Blueprint Ref | Existing Implementation | Frontend Route | Backend API | Database Tables | Test Coverage | Status | Missing Functionality | Severity | Recommended Action |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- | :---: | :--- | :---: | :--- |
| **MEM-01** | Member Registration with Contact Verification | Sec 04 | `auth_service.go`, `delivery/resend.go`, `delivery/brevo.go` | `/register`, `/verify-email`, `/check-email` | `POST /api/v1/auth/register`, `POST /api/v1/auth/verify-email` | `users`, `email_verification_tokens`, `email_verification_rate_limits` | `auth_test.go` (9 test suites) | **Complete and verified** | None | High | Provision production Brevo API keys. |
| **MEM-02** | Anti-Privilege Invariant (registrations strictly default to `member`) | Sec 04 | `auth_service.go:Register` rejects privileged roles | `/register` | `POST /api/v1/auth/register` | `users` (`primary_role='member'`) | `TestAuthAndAntiPrivilegeRegistration` | **Complete and verified** | None | Critical | Maintained in CI. |
| **MEM-03** | Human-Readable Membership ID (`RATEL-2026-XXXXX`) | Sec 04 | Generated via `generateMembershipID()` in `auth_service.go` | `/member`, `/member/profile` | `GET /api/v1/auth/me` | `users.membership_id` | `auth_test.go` | **Complete and verified** | None | Low | In active use. |
| **PART-01** | Attendance Claims Submission with Photos/Notes | Sec 04 | `participation_service.go:SubmitClaim` | `/member/participations` | `POST /api/v1/participations/claims` | `activity_participations` | `participation_test.go` | **Complete and verified** | None | High | In active use. |
| **PART-02** | Review Queue & Anti-Self-Approval (`reviewer_id != user_id`) | Sec 04 | `participation_service.go:ReviewClaim` | `/admin/reviews` | `POST /api/v1/participations/claims/{id}/review` | `activity_participations` | `TestParticipationAndAntiSelfApproval` | **Complete and verified** | None | Critical | Server invariant verified. |
| **PART-03** | First Participation Badge Award Ledger | Sec 04 | Automated badge issuance in `participation_service.go` | `/member`, `/member/participations` | `GET /api/v1/participations/my-badges` | `badges`, `user_badges` | `TestParticipationBadges` | **Complete and verified** | None | Medium | Add other proposed badge rules. |
| **PART-04** | Formal Appeals for Rejected Participation | Sec 04 | `participation_service.go:SubmitAppeal` | `/member/participations` | `POST /api/v1/participations/claims/{id}/appeal` | `participation_appeals` | `participation_test.go` | **Complete and verified** | Independent reviewer assignment | Medium | Add reviewer re-assignment UI. |

---

### 2.3 Monthly Cleanups & Humanitarian Events

| Req ID | Requirement Description | Blueprint Ref | Existing Implementation | Frontend Route | Backend API | Database Tables | Test Coverage | Status | Missing Functionality | Severity | Recommended Action |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- | :---: | :--- | :---: | :--- |
| **CLN-01** | Last Saturday Schedule in `Africa/Lagos` Timezone | Sec 06 | `internal/calendar/calendar.go` algorithm | `/activities`, `/admin/activities` | `GET /api/v1/activities/campaigns/current` | `activity_campaigns` | `cleanup_test.go` | **Complete and verified** | None | High | Verified across calendar years. |
| **CLN-02** | Draft Venue Approval Gate | Sec 06 | State coordinator inspection & publish command | `/admin/activities` | `POST /api/v1/activities/events/{id}/publish` | `activity_events` (`status='draft'/'published'`) | `cleanup_test.go` | **Complete and verified** | None | High | Enforced in router. |
| **CLN-03** | Concurrency-Safe Capacity Reservation | Sec 06 | `SELECT ... FOR UPDATE` in pgx transaction | `/activities/[id]` | `POST /api/v1/activities/events/{id}/register` | `event_registrations` | `cleanup_test.go` (`ErrEventAtCapacity`) | **Complete and verified** | None | High | Stress tested. |
| **CLN-04** | Event Rescheduling with Audit Justification | Sec 06 | Preserves prior timestamp, requires documented reason | `/admin/activities` | `POST /api/v1/activities/events/{id}/reschedule` | `activity_events`, `audit_events` | `cleanup_test.go` | **Complete and verified** | None | Medium | Fully operational. |
| **CLN-05** | Extended Humanitarian Events (Hospitals, Relief) | Sec 06 | `program_type` parameter in event creation | `/admin/activities` | `POST /api/v1/activities/events` | `activity_events.program_type` | `cleanup_test.go` | **Partially implemented** | Confidential participant list & permissions attachment | Medium | Add confidential attachment flags. |

---

### 2.4 Material Distribution & Inventory

| Req ID | Requirement Description | Blueprint Ref | Existing Implementation | Frontend Route | Backend API | Database Tables | Test Coverage | Status | Missing Functionality | Severity | Recommended Action |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- | :---: | :--- | :---: | :--- |
| **INV-01** | Multi-Tier Storage Locations & Warehouses | Sec 05 | `inventory_service.go:CreateLocation` | `/admin/inventory` | `GET /api/v1/inventory/locations` | `storage_locations` | `inventory_test.go` | **Complete and verified** | None | High | Operational. |
| **INV-02** | Stock Balances & Immutable Ledger | Sec 05 | On-hand = receipts - issues; movements append-only | `/admin/inventory` | `GET /api/v1/inventory/balances`, `GET /api/v1/inventory/ledger` | `stock_balances`, `stock_movements` | `inventory_test.go` | **Complete and verified** | None | Critical | Verified across transfers. |
| **INV-03** | Material Requests with Anti-Self-Approval | Sec 05 | Requester cannot approve own request | `/admin/inventory` | `POST /api/v1/inventory/requests`, `POST /api/v1/inventory/requests/{id}/review` | `material_requests`, `material_request_items` | `inventory_test.go` | **Complete and verified** | None | High | Operational. |
| **INV-04** | Stock Reservation & Concurrency Locking | Sec 05 | Row locks prevent negative available stock | `/admin/inventory` | `POST /api/v1/inventory/requests/{id}/allocate` | `stock_balances`, `stock_movements` | `inventory_test.go` (`ErrInsufficientStock`) | **Complete and verified** | None | Critical | Verified concurrency safety. |
| **INV-05** | Material Dispatches & In-Transit Custody | Sec 05 | Dispatches move stock to in-transit state | `/admin/inventory` | `POST /api/v1/inventory/transfers/{id}/dispatch` | `material_transfers`, `stock_movements` | `inventory_test.go` | **Complete and verified** | None | High | Operational. |
| **INV-06** | In-Transit Discrepancies & Discrepancy Resolution | Sec 05 | Partial receipts record discrepancies in transit | `/admin/inventory` | `POST /api/v1/inventory/transfers/{id}/receive`, `POST /api/v1/inventory/discrepancies/{id}/resolve` | `material_discrepancies` | `inventory_test.go` (100 sent, 95 received scenario) | **Complete and verified** | None | High | Blueprint acceptance scenario passed. |

---

### 2.5 Ratels Marketplace & Commerce

| Req ID | Requirement Description | Blueprint Ref | Existing Implementation | Frontend Route | Backend API | Database Tables | Test Coverage | Status | Missing Functionality | Severity | Recommended Action |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- | :---: | :--- | :---: | :--- |
| **MKT-01** | Business Entities & Branches Separation | Sec 07 | Businesses separate from owners; multi-state branches | `/marketplace/seller`, `/admin/marketplace` | `POST /api/v1/marketplace/seller/business`, `POST /api/v1/marketplace/seller/branches` | `marketplace_businesses`, `marketplace_business_branches` | `marketplace_test.go` | **Complete and verified** | None | High | Verified in production. |
| **MKT-02** | Product Catalog with Pricing in Kobo | Sec 07 | Variants, SKUs, integer kobo pricing, R2 images | `/marketplace`, `/marketplace/products/[id]` | `GET /api/v1/marketplace/products`, `POST /api/v1/marketplace/seller/products` | `marketplace_products` | `marketplace_test.go` | **Complete and verified** | None | High | Verified in production (live products). |
| **MKT-03** | Single-Seller Cart & Server-Side Price Snapshots | Sec 07 | Cart validation, price locked at order creation | `/marketplace/cart`, `/marketplace/checkout` | `POST /api/v1/marketplace/cart/validate`, `POST /api/v1/marketplace/checkout` | `marketplace_orders`, `marketplace_order_items` | `marketplace_test.go` | **Complete and verified** | Multi-seller cart splitting (planned for later phase) | Medium | Maintain single-seller for pilot. |
| **MKT-04** | Paystack Checkout & Webhook Deduplication | Sec 08 [R1, R2] | Signed HMAC webhook verification, unique references | `/marketplace/checkout`, `/marketplace/checkout/success` | `POST /api/v1/marketplace/webhooks/paystack`, `POST /api/v1/marketplace/payments/verify` | `marketplace_payments`, `marketplace_orders` | `marketplace_test.go` | **Implemented but not fully verified** | Live Paystack production secret keys pending | High | Configure live Paystack credentials. |
| **MKT-05** | Dispatch Tracking, Delivery & Dispute Resolution | Sec 07 | Carrier name, tracking number, dispute cases | `/marketplace/orders/[id]`, `/admin/marketplace` | `POST /api/v1/marketplace/seller/orders/{id}/dispatch`, `POST /api/v1/marketplace/admin/disputes/{id}/resolve` | `marketplace_shipments`, `marketplace_disputes` | `marketplace_test.go` | **Complete and verified** | None | Medium | Operational. |
| **MKT-06** | Full and Partial Refunds | Sec 07, 08 | Stock restoration, ledger adjustments | `/admin/marketplace` | `POST /api/v1/marketplace/admin/orders/{id}/refund` | `marketplace_payments`, `marketplace_orders` | `marketplace_test.go` | **Complete and verified** | None | High | Migration 7 applied. |

---

### 2.6 Careers & Candidate Matching

| Req ID | Requirement Description | Blueprint Ref | Existing Implementation | Frontend Route | Backend API | Database Tables | Test Coverage | Status | Missing Functionality | Severity | Recommended Action |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- | :---: | :--- | :---: | :--- |
| **CAR-01** | Private Candidate Profiles & CV Vault | Sec 09 | Skills, experience, private R2 CV links | `/careers/profile` | `GET /api/v1/careers/candidate/profile`, `PUT /api/v1/careers/candidate/profile` | `careers_candidate_profiles`, `careers_candidate_skills`, `careers_candidate_qualifications` | `careers_test.go` | **Complete and verified** | None | High | Operational. |
| **CAR-02** | Verified Employers & Recruiter Management | Sec 09 | Admin employer verification, multi-recruiter teams | `/careers/employer`, `/admin/careers` | `POST /api/v1/careers/employer/register`, `POST /api/v1/careers/admin/employers/{id}/verify` | `careers_employers`, `careers_recruiters` | `careers_test.go` | **Complete and verified** | None | Medium | Operational. |
| **CAR-03** | Vacancy Postings & Moderation | Sec 09 | Job creation, admin moderation queue | `/careers`, `/careers/jobs/[id]`, `/admin/careers` | `GET /api/v1/careers/jobs`, `POST /api/v1/careers/admin/jobs/{id}/moderate` | `careers_jobs` | `careers_test.go` | **Complete and verified** | None | High | Operational. |
| **CAR-04** | Explainable Matching Algorithm (50/20/15/15) | Sec 09 | Weighted transparent scoring with explanation breakdown | `/careers/employer` (Applicant list) | `GET /api/v1/careers/employer/jobs/{id}/applications` | Calculated in service layer | `careers_test.go` | **Complete and verified** | None | Medium | Operational. |
| **CAR-05** | Interview Coordination & Scheduling | Sec 09 | Scheduling with format, timezone, notes, status | `/careers/employer`, `/careers/applications` | `POST /api/v1/careers/employer/applications/{id}/interviews` | `careers_interviews` | `careers_test.go` | **Complete and verified** | None | Medium | Operational. |
| **CAR-06** | CV View Access Auditing | Sec 09 | View access logged to private audit table | Triggered on recruiter profile view | `GET /api/v1/careers/employer/applications/{id}/candidate-profile` | `careers_cv_view_audits` | `careers_test.go` | **Complete and verified** | None | Critical | Enforces privacy. |

---

### 2.7 Community Projects & Public Requests

| Req ID | Requirement Description | Blueprint Ref | Existing Implementation | Frontend Route | Backend API | Database Tables | Test Coverage | Status | Missing Functionality | Severity | Recommended Action |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- | :---: | :--- | :---: | :--- |
| **PRJ-01** | Public Need Intake Form & Tracking Ref | Sec 10 | Public form with reference ID generation | `/projects/submit`, `/projects/requests` | `POST /api/v1/community/requests` | `community_requests` | `community_project_test.go` | **Complete and verified** | None | High | Operational. |
| **PRJ-02** | Request Triage & Anti-Self-Approval | Sec 10 | Independent triage officer review | `/admin/projects` | `POST /api/v1/admin/community/requests/{id}/review` | `community_requests` | `community_project_test.go` | **Complete and verified** | None | High | Operational. |
| **PRJ-03** | Conversion to Formal Project | Sec 10 | Creates project with code, budget, named officers | `/admin/projects` | `POST /api/v1/admin/community/requests/{id}/convert` | `community_projects`, `project_roles` | `community_project_test.go` | **Complete and verified** | None | High | Operational. |
| **PRJ-04** | Weighted Milestones & Independent Verification | Sec 10 | Weights total 100%; verifier accepts work | `/admin/projects`, `/projects/[id]` | `POST /api/v1/admin/community/projects/{id}/milestones/{mid}/verify` | `project_milestones`, `project_inspections` | `community_project_test.go` | **Complete and verified** | None | Critical | Operational. |
| **PRJ-05** | Physical vs Financial Progress Decoupling | Sec 10, 11 | Physical progress = accepted milestone weights only | `/projects/[id]`, `/admin/projects` | `GET /api/v1/community/projects/{id}` | `community_projects`, `financial_funds` | `community_project_test.go` | **Complete and verified** | None | Critical | Invariant verified. |

---

### 2.8 Finance, Ledgers & Bank Reconciliation

| Req ID | Requirement Description | Blueprint Ref | Existing Implementation | Frontend Route | Backend API | Database Tables | Test Coverage | Status | Missing Functionality | Severity | Recommended Action |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- | :---: | :--- | :---: | :--- |
| **FIN-01** | Authoritative Double-Entry General Ledger | Sec 11, 14 | Balanced debits == credits invariant enforced | `/admin/finance` | `GET /api/v1/finance/transactions`, `POST /api/v1/finance/transactions` | `financial_funds`, `financial_transactions`, `financial_ledger_entries` | `finance_accountability_test.go` | **Complete and verified** | None | Critical | Core accounting invariant verified. |
| **FIN-02** | Integer Minor Units (Kobo) Storage | Sec 11 | 64-bit integer kobo for all amounts | Entire finance module | All `/api/v1/finance/*` endpoints | All financial tables (`*_kobo BIGINT`) | `finance_accountability_test.go` | **Complete and verified** | None | Critical | Zero floating-point money. |
| **FIN-03** | Structured Expense Requests & Anti-Self-Approval | Sec 03, 11 | Draft $\to$ Submitted $\to$ Under Review $\to$ Approved $\to$ Authorized $\to$ Paid $\to$ Reconciled | `/admin/finance` | `POST /api/v1/finance/expenses`, `POST /api/v1/finance/expenses/{id}/review`, `POST /api/v1/finance/expenses/{id}/pay` | `financial_expenses` | `TestFinanceAccountability_ExpenseLifecycleAndAntiSelfApproval` | **Complete and verified** | None | Critical | Separate approval & disbursement. |
| **FIN-04** | Multi-Tier Budgets & Revision Audit Trail | Sec 11 | Budget periods, spending tracking, immutable revisions | `/admin/finance` | `POST /api/v1/finance/budgets`, `POST /api/v1/finance/budgets/{id}/revisions` | `financial_budgets`, `financial_budget_revisions` | `TestFinanceAccountability_BudgetLifecycleAndRevisions` | **Complete and verified** | None | High | Revisions cannot silently overwrite. |
| **FIN-05** | Controlled Fund Allocations (Zero Inflation) | Sec 11 | Balance-neutral inter-fund transfers (`is_external_income=false`) | `/admin/finance` | `POST /api/v1/finance/allocations` | `financial_allocations`, `financial_transactions` | `TestFinanceAccountability_AllocationsAndLedgerIntegrity` | **Complete and verified** | None | High | Zero artificial revenue inflation. |
| **FIN-06** | Bank Account Registry & Super Admin Activation | Sec 11, 24 | Super Admin activation required; masked numbers | `/admin/finance` | `GET /api/v1/finance/bank-accounts`, `POST /api/v1/finance/bank-accounts/{id}/activate` | `financial_bank_accounts` | `TestFinanceAccountability_BankAccountsAndReconciliation` | **Complete and verified** | None | High | Operational. |
| **FIN-07** | CSV Statement Import & Duplicate Row Detection | Sec 11, 26 | Statement parsing, SHA-256 row fingerprints | `/admin/finance` | `POST /api/v1/finance/reconciliation/periods/{id}/import-csv` | `financial_bank_statements`, `financial_statement_items` | `TestFinanceAccountability_BankAccountsAndReconciliation` | **Complete and verified** | None | High | Duplicate imports prevented. |
| **FIN-08** | Transaction Matching & Period Certification | Sec 11, 26 | Auto and manual matching; Super Admin locks period | `/admin/finance` | `POST /api/v1/finance/reconciliation/periods/{id}/auto-match`, `POST /api/v1/finance/reconciliation/periods/{id}/certify` | `financial_reconciliation_periods`, `financial_statement_items` | `TestFinanceAccountability_BankAccountsAndReconciliation` | **Complete and verified** | None | High | Certified periods immutable. |
| **FIN-09** | Official Printable Documents (Vouchers, Receipts) | Sec 11, 12 | Printable HTML with Ratels branding, control numbers, kobo-in-words | `/admin/finance` | `GET /api/v1/finance/documents/{type}/{id}/print` | Generated dynamically from posted records | `TestFinanceAccountability_DocumentGeneration` | **Complete and verified** | None | High | Multi-page print layout verified. |

---

### 2.9 Foundation & Donor Experience

| Req ID | Requirement Description | Blueprint Ref | Existing Implementation | Frontend Route | Backend API | Database Tables | Test Coverage | Status | Missing Functionality | Severity | Recommended Action |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- | :---: | :--- | :---: | :--- |
| **FDN-01** | Provisional Foundation Name & Disclaimers | Sec 24 | "Martins Vincent Otse Foundation" with honest disclaimer | `/foundation` | `GET /api/v1/foundation/` | Hardened config in `foundation_service.go` | `foundation_test.go` | **Complete and verified** | Official CAC verification docs | Medium | Review legal documents before launch. |
| **FDN-02** | Dual-Officer Bank Account Verification | Sec 24 | Submitter cannot approve; requires two officers | `/admin/foundation` | `POST /api/v1/foundation/bank-accounts/{id}/verify` | `verified_bank_account_versions` | `foundation_test.go` | **Complete and verified** | None | Critical | Account substitution prevented. |
| **FDN-03** | Segregated Accounting (Foundation vs Ratels) | Sec 24, 26 | Distinct fund identifiers in ledger | `/admin/finance`, `/foundation` | `GET /api/v1/finance/transparency/summary` | `financial_funds`, `financial_donations` | `finance_test.go` | **Complete and verified** | None | Critical | Invariant verified. |
| **FDN-04** | Donor Anonymity Masking ("Anonymous Supporter") | Sec 25 | Masks donor names in public/coordinator queries | `/foundation`, `/admin/finance` | `GET /api/v1/finance/donations` | `financial_donations.is_anonymous` | `TestFinanceAccountability_DonationsAndDonorPrivacy` | **Complete and verified** | None | High | Verified in automated tests. |
| **FDN-05** | Public Transparency Metrics Summary | Sec 25, 26 | Confirmed donations, project allocations, cash reserves | `/foundation` | `GET /api/v1/finance/transparency/summary` | Real-time aggregate query | `finance_donations.go` | **Complete and verified** | None | Medium | Operational in production. |

---

### 2.10 Media, Communications & Remaining Blueprint Services

| Req ID | Requirement Description | Blueprint Ref | Existing Implementation | Frontend Route | Backend API | Database Tables | Test Coverage | Status | Missing Functionality | Severity | Recommended Action |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- | :---: | :--- | :---: | :--- |
| **MED-01** | Homepage Hero Video Streaming & R2 Direct Upload | Sec 13 | Presigned R2 uploads, playback toggle switch | `/`, `/admin/hero` | `GET /api/v1/homepage/hero-media`, `POST /api/v1/admin/hero-media/presign` | `homepage_hero_media` | `hero_media_test.go`, `browser_hero_media_verification.js` | **Complete and verified** | None | Medium | Live in production. |
| **MED-02** | Event Photo/Video Moderation Gallery | Sec 06, 13 | None | None | None | None | None | **Not implemented** | Year/zone/state/event gallery, consent review, moderation queue, EXIF metadata stripping, takedowns | High | Implement in Milestone 2. |
| **COM-01** | Transactional Outbox & Notification Worker | Sec 13, 14, 17 | Email delivery adapter only | None | None | None | `brevo_test.go` | **Partially implemented** | Outbox table, background retry worker, in-app notification center, SMS adapter (Termii) | High | Implement in Milestone 2. |
| **REP-01** | Dedicated Public Transparency & Impact Reports Portal | Sec 27 | Basic summary on `/foundation` | None (`/reports` not built) | `GET /api/v1/finance/transparency/summary` | None | None | **Partially implemented** | Dedicated `/reports` portal with downloadable annual certified impact reports | Medium | Implement in Milestone 2. |
| **SAFE-01**| Safeguarding, Confidential Reports & Minors Policy | Sec 16, 27 | None | None | None | None | None | **Not implemented** | Confidential reporting queue, safeguarding reviewer role, minors consent | High | Implement in Milestone 2. |
| **AST-01** | Asset & Equipment Checkout Management | Sec 27 | Controlled consumables inventory only | None | None | None | None | **Not implemented** | Reusable tool registry, checkout/return tracking, maintenance records | Medium | Implement in Milestone 3. |
| **ADM-01** | Multi-Appointment Role Switcher | Sec 22 | Role evaluated from primary appointment | Admin header has no dropdown | None | `appointments` | None | **Partially implemented** | Role switcher UI dropdown in admin header | Medium | Implement in Milestone 2. |
| **SUP-01** | Integrated Support & Complaints Case Management | Sec 22, 27 | Marketplace disputes only | None | `POST /api/v1/marketplace/orders/{id}/dispute` | `marketplace_disputes` | `marketplace_test.go` | **Partially implemented** | Unified support ticketing across activities, donations, projects, and careers | Medium | Implement in Milestone 3. |
| **PWA-01** | Progressive Web App (PWA) Offline Caching | Sec 17 | Responsive mobile CSS only | None | None | None | None | **Not implemented** | Web App Manifest, Service Worker, offline draft caching | Low | Implement in Milestone 4. |
| **LOC-01** | Multilingual Localization (Hausa, Igbo, Yoruba) | Sec 01, 27 | English only | None | None | None | None | **Not implemented** | i18n translation framework and language switcher | Low | Implement in Milestone 4. |
