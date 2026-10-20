# Nigerian Ratels Management System — Remaining Development Roadmap

> **Document Version:** 1.0  
> **Date:** 19 October 2026  
> **Reference Baseline:** Nigerian Ratels Complete Blueprint v2.0 (17 October 2026)  
> **Target Release Window:** Operational Foundation Pilot $\to$ National Rollout  

---

## 1. Roadmap Prioritization Framework

Remaining work is grouped into four objective priority tiers:

- **Priority 0 (P0) — Production Blockers, Security & Data Integrity:** Critical configuration and operational tasks required for production stability, reference data availability, and security compliance.
- **Priority 1 (P1) — Missing Essential Blueprint Requirements:** Core functional modules specified in Blueprint v2.0 that have no existing implementation in the codebase (Media Moderation & Galleries, Transactional Communications & Outbox, Public Impact Reports, Safeguarding).
- **Priority 2 (P2) — Incomplete Workflows & Integration Gaps:** Workflows where foundational models exist but specific administrative features, multi-appointment ergonomics, or cross-cutting dispute/support routing are missing.
- **Priority 3 (P3) — UI, Localization, Performance & Operational Readiness:** Non-blocking platform enhancements, installable PWA service workers, Nigerian national language translations, volunteer skill tracking, and preparation for future native mobile applications.

---

## 2. Priority 0 (P0) — Production Blockers & Data Integrity

### Task P0-1: Seed Authoritative Reference Geography to Production Neon Database
- **Issue:** `GET https://ratels.onrender.com/api/v1/geography/states` returns `[]`. The production database lacks the 6 geopolitical zones, 36 states, 1 FCT territory, and 774 LGAs/area councils.
- **Dependencies:** Production Neon `DATABASE_URL` connectivity.
- **Implementation Scope:**
  - Execute `go run ./cmd/seed` (safe reference data only, without `--demo-accounts`) against the production Neon PostgreSQL database.
  - Verify that `ExpectedZonesCount = 6`, `ExpectedStatesCount = 37`, and `ExpectedTotalLGAs = 774` are populated with zero errors.
- **Acceptance Criteria:**
  - `curl -s https://ratels.onrender.com/api/v1/geography/zones` returns all 6 zones with HTTP 200.
  - `curl -s https://ratels.onrender.com/api/v1/geography/states` returns 37 records with `is_fct=true` for FCT.
  - Location selection dropdowns across `/register`, `/activities`, `/marketplace`, and `/projects` display complete Nigerian geography in production.
- **Complexity / Effort:** Low (1 person-day).

### Task P0-2: Production Email & Payment Secret Provisioning
- **Issue:** Email verification delivery and Paystack checkout currently rely on development sandbox adapters in development/testing. Production requires verified secrets.
- **Dependencies:** Merchant approval from Paystack and verified sender domain in Brevo.
- **Implementation Scope:**
  - In Render Dashboard for `ratels-api`, configure:
    - `EMAIL_PROVIDER=brevo`
    - `BREVO_API_KEY=<production-key>`
    - `EMAIL_FROM_ADDRESS=info@ratels.org.ng`
    - `PAYSTACK_SECRET_KEY=<live-secret-key>`
    - `PAYSTACK_PUBLIC_KEY=<live-public-key>`
  - Verify webhook HMAC signing secret on Paystack dashboard pointing to `https://ratels.onrender.com/api/v1/marketplace/webhooks/paystack`.
- **Acceptance Criteria:**
  - Real registration on `https://ratels-web.onrender.com/register` dispatches transactional verification email via Brevo HTTPS API.
  - Marketplace checkout completes with real Paystack checkout and authenticates webhook delivery.
- **Complexity / Effort:** Low (1 person-day).

### Task P0-3: Container Daemon Access & Alpine Staging Pipeline
- **Issue:** Docker daemon socket access is restricted for non-root user `recoding` on the local workspace host (`permission denied while trying to connect to the docker API at unix:///var/run/docker.sock`).
- **Dependencies:** Host system administrative permissions (`sudo usermod -aG docker recoding`).
- **Implementation Scope:**
  - Grant local Docker permissions to user `recoding` or configure dedicated CI/CD container build runner.
  - Verify multi-stage build in [`infra/Dockerfile.api`](file:///home/recoding/Tech-Ratels/infra/Dockerfile.api) (`golang:1.25-alpine` builder $\to$ `alpine:3.21` runtime).
  - Execute automated container smoke test.
- **Acceptance Criteria:**
  - `docker compose -f infra/docker-compose.yml up --build` compiles and starts backend and web containers cleanly without permission errors.
- **Complexity / Effort:** Low (1–2 person-days).

---

## 3. Priority 1 (P1) — Missing Essential Blueprint Requirements

### Task P1-1: Event Photo & Video Moderation Gallery (Blueprint Sec 06, 13)
- **Problem Statement:** Monthly cleanups, humanitarian outreaches, and community projects generate extensive photo/video evidence. Currently, there is no public gallery, no moderation queue, no consent review, no automated metadata (EXIF/GPS) stripping, and no takedown mechanism.
- **Dependencies:** Cloudflare R2 bucket integration, Image inspection service.
- **Implementation Scope:**
  - **Database Migration:** Table `event_media` (`id`, `event_id`, `uploaded_by_user_id`, `media_type`, `object_key`, `public_url`, `caption`, `year`, `zone_id`, `state_id`, `status: pending_review | approved | rejected | quarantined`, `reviewed_by_user_id`, `moderation_notes`, `consent_confirmed`, `created_at`).
  - Table `media_takedown_requests` (`id`, `media_id`, `requester_email`, `reason`, `status: pending | resolved | dismissed`, `resolved_by_id`, `resolved_at`).
  - **Backend Service:** `media_gallery_service.go` implementing:
    - Direct R2 presigned upload with image/video validation.
    - Automated stripping of EXIF/GPS metadata prior to public availability to protect volunteer and beneficiary privacy.
    - Moderation review queue (`ApproveMedia`, `RejectMedia`, `QuarantineMedia`).
    - Public gallery query with multi-dimensional filtering (`year`, `zone`, `state`, `event`).
    - Public takedown request submission (`SubmitTakedownRequest`).
  - **Frontend UI:**
    - Public `/gallery` page with filters for Year, Zone, State, and Event.
    - Responsive image lightbox with accessible descriptions (`alt` text and captions).
    - Administrative moderation dashboard at `/admin/gallery` with review checklist and takedown resolution.
- **Acceptance Criteria:**
  - Uploaded cleanup photos are held in `pending_review` and are invisible to the public until approved.
  - Location/GPS metadata is stripped from all published images.
  - Public `/gallery` renders approved media grouped by year, zone, state, and activity.
  - Takedown request submitted via public modal notifies moderators and can immediately quarantine media.
- **Complexity / Effort:** Medium (5–7 person-days).

### Task P1-2: Transactional Communications, Outbox & In-App Notification Center (Blueprint Sec 13, 14, 17, 23)
- **Problem Statement:** The platform currently has an email verification adapter, but lacks a persistent notification system, an asynchronous outbox worker, an in-app notification center, and coordinator announcement broadcasting.
- **Dependencies:** PostgreSQL transaction manager, background daemon runner.
- **Implementation Scope:**
  - **Database Migration:** Table `notifications` (`id`, `user_id`, `channel: in_app | email | sms`, `category: cleanup_reminder | participation_status | order_update | interview_invite | announcement`, `title`, `body`, `action_url`, `is_read`, `created_at`).
  - Table `outbox_events` (`id`, `event_type`, `payload JSONB`, `status: pending | delivered | failed`, `retry_count`, `last_attempt_at`, `created_at`).
  - Table `user_notification_preferences` (`user_id`, `email_opt_in`, `sms_opt_in`, `reminder_lead_days`).
  - **Backend Service:** `notification_service.go` and `internal/worker/outbox_worker.go`:
    - Atomically insert outbox events in the same database transaction as domain state mutations (e.g., when an order is dispatched or a claim is reviewed).
    - Asynchronous worker polling outbox events with exponential backoff retries.
    - SMS Gateway adapter (Termii HTTPS API integration for Nigerian phone numbers).
    - Automated cleanup reminder generator (schedules reminders 7 days and 1 day prior to the last Saturday).
  - **Frontend UI:**
    - Notification bell icon in navigation bar with unread count badge and popover dropdown.
    - Member notification preferences tab in `/member/profile`.
    - Coordinator announcement composer in `/admin/announcements` with scoped audience selection.
- **Acceptance Criteria:**
  - Mutating events reliably produce outbox rows that deliver without blocking HTTP requests.
  - Unread notifications render in real-time in the web header bell dropdown.
  - Members can toggle notification channels in their profile settings.
- **Complexity / Effort:** Medium-High (7–9 person-days).

### Task P1-3: Dedicated Public Transparency & Annual Impact Reports Portal (Blueprint Sec 11, 27)
- **Problem Statement:** Blueprint Section 27 requires certified annual impact reports combining reconciled finances, verified materials handed over, cleanups completed, and confirmed jobs.
- **Dependencies:** Finance service, Community projects service, Inventory service.
- **Implementation Scope:**
  - **Frontend Portal:** Dedicated public `/reports` page.
  - Summary metric cards: Total Cleanups Completed, Total Materials Handed Over, Verified Project Beneficiaries, Jobs Confirmed, Reconciled Funds Disbursed.
  - Interactive annual filter (e.g. 2026, 2027).
  - Downloadable certified Annual Financial & Impact Summary PDF with Nigerian Ratels seal and Super Admin digital certification watermark.
  - Strict privacy filter: raw account numbers, personal payee names, and beneficiary PII are permanently excluded from public exports.
- **Acceptance Criteria:**
  - Public visitors can navigate to `/reports` and view aggregate impact metrics without authentication.
  - Downloadable PDF generates cleanly and matches certified ledger records.
- **Complexity / Effort:** Medium (4–5 person-days).

### Task P1-4: Safeguarding, Confidential Incident Reports & Minors Policy (Blueprint Sec 16, 27) — [COMPLETED — PHASE C]
- **Status:** **COMPLETED** (Verified via automated unit tests, integration tests, and headless Puppeteer browser acceptance suite). See detailed completion report: [`docs/ratels_phase_c_safeguarding_report.md`](file:///home/recoding/Tech-Ratels/docs/ratels_phase_c_safeguarding_report.md).
- **Implementation Scope:**
  - **Database Migration:** Table `safeguarding_cases`, `safeguarding_evidence`, `safeguarding_assignments`, `safeguarding_case_messages`, `safeguarding_case_notes`, `safeguarding_access_logs`, `safeguarding_retention_policies`.
  - **Cryptographic Security:** AES-256-GCM authenticated encryption for all narratives, reporter personal information, and internal notes. Salted SHA-256 passkey hashing.
  - **Backend Service:** Strict access control: only appointed safeguarding officers and Super Admin have access. Automatic conflict-of-interest recusal and trustee escalation.
  - **Frontend UI:**
    - Public confidential & anonymous report portal (`/report-incident`) with emergency warning banners and quick-exit safety escape.
    - Public status & two-way confidential messaging portal (`/report-incident/status`) via non-sequential reference (`CASE-YYYY-XXXXXXXX`) and passkey (`sec_...`).
    - Restricted administrative case management workspace at `/admin/safeguarding` with triage, assignment, notes, and immutable audit logs.
- **Acceptance Criteria:**
  - Confidential incident submissions return a private tracking code and secret passkey.
  - Ordinary national admins, state coordinators, and LGA coordinators are denied access to safeguarding records.
  - 100% verified via end-to-end browser tests.


---

## 4. Priority 2 (P2) — Incomplete Workflows & Integration Gaps

### Task P2-1: Admin Multi-Appointment Role Switcher (Blueprint Sec 22)
- **Problem Statement:** Coordinators who hold multiple roles (e.g., State Coordinator in Benue and National Logistics Lead) currently have their primary role evaluated; there is no UI switcher in the admin header.
- **Implementation Scope:**
  - Backend endpoint `GET /api/v1/auth/appointments/active` returning all active appointments held by the user.
  - Header dropdown in `/admin/*` allowing the user to switch their active session context.
  - Store active appointment ID in context header `X-Active-Appointment-ID`.
- **Complexity / Effort:** Low-Medium (3 person-days).

### Task P2-2: Unified Support & Complaints Case Management (Blueprint Sec 22, 27)
- **Problem Statement:** Currently, only marketplace orders have a dispute workflow (`marketplace_disputes`). Support cases arising from membership verifications, missing cleanup attendance, donation matching inquiries, and project complaints must be handled manually.
- **Implementation Scope:**
  - Create generic `support_tickets` table linked to polymorphic domain entities (`entity_type: order | donation | participation | project | account`).
  - Support ticket creation modal across member pages.
  - Centralized support workspace at `/admin/support` with ticket assignment, status, and SLA tracking.
- **Complexity / Effort:** Medium (4–5 person-days).

### Task P2-3: Content Management Engine (FAQs, Announcements, Policy Docs) (Blueprint Sec 23)
- **Problem Statement:** Blueprint Section 23 requires authorized administrators to manage FAQs, announcement banners, reminder templates, and approved public contacts without developer intervention.
- **Implementation Scope:**
  - Database table `platform_content` (`key`, `category`, `title`, `content_markdown`, `status: draft | published | archived`, `version`, `updated_by_id`).
  - Admin management interface at `/admin/content`.
  - Dynamic consumption on public `/about`, `/faq`, and announcement banners.
- **Complexity / Effort:** Medium (3–4 person-days).

### Task P2-4: Multi-Seller Cart & Sub-Order Splitting (Blueprint Sec 07)
- **Problem Statement:** The current marketplace implements a single-seller cart for the pilot. Full nationwide commercial launch requires allowing buyers to place orders containing items from multiple sellers.
- **Implementation Scope:**
  - Split checkout order into a parent order with child `marketplace_seller_orders`.
  - Calculate separate shipping quotes and delivery tracking per seller.
- **Complexity / Effort:** High (7–10 person-days).

### Task P2-5: Reusable Asset & Equipment Checkout Management (Blueprint Sec 27)
- **Problem Statement:** The inventory module handles consumable materials (shovels, gloves, trash bags, relief packs). Durable equipment (generators, water pumps, cleanup vehicles) needs asset tracking.
- **Implementation Scope:**
  - Table `inventory_assets` (`id`, `serial_number`, `asset_tag`, `name`, `status: available | checked_out | under_maintenance | retired`).
  - Table `asset_checkouts` (`asset_id`, `borrower_id`, `event_id`, `checked_out_at`, `returned_at`, `condition_notes`).
  - Asset tracking tab in `/admin/inventory`.
- **Complexity / Effort:** Medium (4–5 person-days).

---

## 5. Priority 3 (P3) — UI, Localization & Operational Readiness

### Task P3-1: Progressive Web App (PWA) Offline Caching & Service Worker (Blueprint Sec 17)
- **Implementation Scope:**
  - Add `manifest.json` with Ratels branding, app icons, and theme colors.
  - Configure `next-pwa` or custom Workbox service worker for offline caching of static assets and previously viewed cleanup locations.
  - Offline banner with safe retry for non-mutating screens.
- **Complexity / Effort:** Low-Medium (3–4 person-days).

### Task P3-2: Multilingual Localization (Hausa, Igbo, Yoruba) (Blueprint Sec 01, 27)
- **Implementation Scope:**
  - Implement `next-intl` localization framework in `apps/web`.
  - Extract user-facing strings into JSON resource files: `en.json`, `ha.json`, `ig.json`, `yo.json`.
  - Add language selector dropdown in navigation header.
- **Complexity / Effort:** Medium-High (6–8 person-days).

### Task P3-3: Volunteer Skills, Availability & Safety Records (Blueprint Sec 27)
- **Implementation Scope:**
  - Extend member profiles with volunteer skill tags (first aid, driving, heavy equipment, crowd control), availability times, and signed safety acknowledgments.
  - Coordinator volunteer filtering when organizing major cleanup campaigns.
- **Complexity / Effort:** Medium (3–4 person-days).

### Task P3-4: Preparation for Native Mobile Applications (Blueprint Sec 17)
- **Implementation Scope:**
  - Verify that all `/api/v1/*` endpoints accept Bearer tokens in `Authorization` header in addition to HttpOnly cookies.
  - Ensure OpenAPI 3.0 specification (`docs/openapi.yaml`) is 100% synchronized for automated TypeScript client generation in React Native / Expo.
- **Complexity / Effort:** Low-Medium (3 person-days).

---

## 6. Recommended Next Development Milestone: Milestone 2

Based on verified platform gaps rather than assumptions, the **recommended immediate development milestone** is:

### Milestone 2: Media Moderation, Communications Engine & Safeguarding
- **Estimated Duration:** 3 weeks (15 working days).
- **Deliverables:**
  1. **Event Media Gallery & Moderation Queue:**
     - Migration for `event_media` and `media_takedown_requests`.
     - Direct presigned Cloudflare R2 upload with metadata (EXIF/GPS) sanitization.
     - Public `/gallery` page and `/admin/gallery` moderation queue.
  2. **Transactional Communications & Notification Center:**
     - Migration for `notifications`, `outbox_events`, and `user_notification_preferences`.
     - Background outbox worker daemon with exponential backoff.
     - In-app notification bell dropdown in navigation bar.
     - Termii SMS gateway adapter for Nigerian phone numbers.
     - Automated last-Saturday cleanup reminder scheduler.
  3. **Safeguarding & Confidential Incident Reporting:**
     - Migration for `safeguarding_cases`.
     - Public `/report-incident` form and restricted `/admin/safeguarding` investigation queue.
  4. **Multi-Appointment Role Switcher:**
     - Admin header appointment switcher dropdown.
