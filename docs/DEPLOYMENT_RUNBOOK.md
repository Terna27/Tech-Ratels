# RATELS WORLDWIDE — PRODUCTION DEPLOYMENT RUNBOOK & OPERATIONAL PLAYBOOK

## 1. System Overview & Deployment Architecture

The Ratels Worldwide management platform comprises distributed native services deployed on Render backed by a managed Neon PostgreSQL 16 database, Cloudflare R2 object storage, and external communications/fintech providers (Paystack, Brevo, Termii):

1. **`ratels`** (Native Go Web Service): High-performance Go 1.25 REST API server exposing `/api/v1` routes with connection pooling, automatic startup migration execution via distributed advisory lock, in-process outbox worker, report scheduler, and refund recovery engine.
2. **`ratels-web`** (Native Node.js Web Service): Next.js 15 client dashboard providing responsive web interfaces for Buyers, Sellers, Coordinators, and Administrators.
3. **Neon Serverless PostgreSQL 16**: Primary authoritative database cluster with automated PITR daily backups, SSL transport encryption (`sslmode=require`), zero-downtime branching, and connection pooling. **Do not provision a Render-managed database.**
4. **Cloudflare R2**:
   - `ratels` (Public media bucket): Public S3-compatible bucket storing validated product and catalogue imagery via public domain `pub-82990f82f02446b5a11817fdce96912d.r2.dev`.
   - Dedicated private bucket (optional/staged): Private bucket for signed transparency snapshot exports and safeguarding records.

---

## 2. Pre-Deployment Environment Variable & Secret Checklist

All values must be configured via Render Dashboard Environment Groups or Secrets Manager. **Never commit real values to Git.**

| Variable Name | Required Service | Classification | Description & Validation Rule |
| :--- | :--- | :--- | :--- |
| `APP_ENV` | api, worker, cron | System | Must be set to `production`. Activates strict CORS, SSL enforcement, and disables fallback dev sandboxes. |
| `DATABASE_URL` | api, worker, cron | Secret | Connection string from Neon PostgreSQL (must end with `?sslmode=require`). Never hardcode or provision Render DB. |
| `JWT_SECRET` | api, worker | Secret | High-entropy random string (minimum 32 characters). Preserve existing production secret; never auto-regenerate. |
| `JWT_EXPIRY_HOURS` | api | Config | Default `24`. Controls session cookie and token lifespan. |
| `ALLOWED_ORIGINS` | api | Config | Comma-separated frontend domains (e.g. `https://app.ratelsworldwide.org,https://ratelsworldwide.org,https://ratels-web.onrender.com`). Wildcards prohibited. |
| `FRONTEND_BASE_URL` | api, worker | Config | Primary frontend URL (e.g. `https://app.ratelsworldwide.org`). Used in verification and receipt links. |
| `DEV_DELIVERY_SAFE_MODE`| api, worker | Safety Flag | Must be set to `false` in production. |
| `PAYMENT_PROVIDER` | api, worker | Config | Set to `paystack` once live keys are configured, or `unconfigured` to safely disable checkout. `sandbox` is blocked in production. |
| `PAYSTACK_SECRET_KEY` | api, worker | Secret | Paystack Live Secret Key (`sk_live_...`). Required for checkout and webhook verification when `PAYMENT_PROVIDER=paystack`. |
| `PAYSTACK_PUBLIC_KEY` | api, worker | Config | Paystack Live Public Key (`pk_live_...`). |
| `EMAIL_PROVIDER` | api, worker | Config | Set to `brevo` or `resend`. |
| `BREVO_API_KEY` | api, worker | Secret | Live Brevo HTTPS API key (`xkeysib-...`). |
| `EMAIL_FROM_ADDRESS` | api, worker | Config | Verified sender email (e.g. `notifications@ratelsworldwide.org`). |
| `TERMII_API_KEY` | api, worker | Secret | Termii SMS Live API Key (`TL_...`). |
| `TERMII_SENDER_ID` | api, worker | Config | Approved alphanumeric sender ID (e.g. `RATELS`). |
| `R2_ACCOUNT_ID` | api, worker | Secret | Cloudflare Account ID. Required if Cloudflare R2 is configured. |
| `R2_ACCESS_KEY_ID` | api, worker | Secret | Cloudflare R2 S3-compatible Access Key ID. |
| `R2_SECRET_ACCESS_KEY` | api, worker | Secret | Cloudflare R2 Secret Access Key. |
| `R2_BUCKET_NAME` | api, worker | Config | Public marketplace catalogue bucket. |
| `R2_PUBLIC_DOMAIN` | api, worker | Config | Custom CDN domain or R2 public URL (`https://media.ratelsworldwide.org`). |
| `SAFEGUARDING_REPORTING_ENABLED` | api | Config | Default `false`. Set to `true` only after providing `SAFEGUARDING_ENCRYPTION_KEY`. |
| `SAFEGUARDING_ENCRYPTION_KEY` | api | Secret | 32-byte AES-GCM encryption key for confidential incident reports (required if safeguarding enabled). |
| `REPORT_CRON_SECRET` | api | Secret | Shared bearer secret for triggering monthly report generation. |
| `MARKETPLACE_CRON_SECRET` | api | Secret | Shared bearer secret for triggering reservation expiry and refund reconciliation. |
| `NEXT_PUBLIC_API_URL` | web | Build & Runtime | Public API base URL accessed by browser (`https://api.ratelsworldwide.org/api/v1`). Passed as build argument in Docker. |
| `API_URL` | web | Build & Runtime | Server-side SSR backend URL proxy destination (`https://api.ratelsworldwide.org/api/v1`). |

---

## 3. Database Migration Safety & Historical Compatibility

### Forward Migration Assurance
1. Automatic forward execution is embedded in `apps/api/cmd/server/main.go` on application boot using `migrator.RunMigrations(ctx, pool)`.
2. All 22 migrations are backwards-compatible:
   - Migration 19 adds order reservation columns and immutable inventory logs with default values.
   - Migration 20 backfills historical stock release flags and seeds opening ledger balances.
   - Migration 21 adds multi-vendor shipment columns, backs up historical single-vendor shipments into `marketplace_shipment_items`.
   - Migration 22 hardens composite uniqueness (`order_id, business_id`) with `NULLS NOT DISTINCT` and adds refund item attribution.
3. **Rollback Guardrails**: Migration 20 contains an explicit PL/pgSQL guard that raises an exception preventing accidental rollback if unfulfilled orders reside in the `requires_reconciliation` state.

### Manual Standalone Migration Command (Render Shell / CI)
```bash
# Verify connection
psql "$DATABASE_URL" -c "SELECT version();"

# Apply all pending migrations forward
/app/server --migrate-only # or trigger via server boot
```

---

## 4. Payment Gateway (Paystack) Verification & Webhook Setup

1. **Webhook Configuration**:
   - URL: `https://api.ratelsworldwide.org/api/v1/marketplace/webhooks/paystack`
   - Events Subscribed:
     - `charge.success`
     - `refund.processed`
     - `refund.successful`
     - `refund.failed`
2. **Replay & Forgery Protection**:
   - Every incoming webhook verifies the `x-paystack-signature` HMAC-SHA512 header against `PAYSTACK_SECRET_KEY`. Requests with forged or missing signatures return HTTP 401 [`ErrInvalidSignature`](file:///home/recoding/Tech-Ratels/apps/api/internal/payment/gateway.go#L18).
   - Duplicate webhooks are idempotent: already `successful` or `failed` records are acknowledged with HTTP 200 without duplicate state modification or multiple inventory ledger adjustments.
3. **Ambiguous Timeout Safety**:
   - If a refund dispatch times out, the system **never** fires a duplicate request automatically based only on negative transaction lookups.
   - The worker marks `requires_privileged_manual_review = true` and holds the refund in `pending`.
   - An authorized administrator with anti-self-approval clearance must review Paystack logs and execute `AuthorizeAmbiguousRefundRedispatch` before the worker will re-attempt provider initiation.

---

## 5. Cloudflare R2 Media Storage Configuration

1. **Bucket Creation**:
   - Create bucket `ratels-marketplace-assets-prod` in Cloudflare Dashboard.
   - Enable Public Access via Custom Domain (`media.ratelsworldwide.org`) with TLS 1.3.
   - Configure CORS on the bucket:
     ```json
     [
       {
         "AllowedOrigins": ["https://app.ratelsworldwide.org", "https://ratelsworldwide.org"],
         "AllowedMethods": ["GET", "PUT", "HEAD"],
         "AllowedHeaders": ["Content-Type", "Content-Length"],
         "MaxAgeSeconds": 3600
       }
     ]
     ```
2. **Object Key Scoping**:
   - Product images are generated with pattern `marketplace/products/<business_id>/<uuid>.<ext>`.
   - Overwriting existing keys is structurally prohibited by cryptographic UUID pathing.

---

## 6. Render Deployment Execution Steps

### Phase A: Neon PostgreSQL Baseline Snapshot & Disaster Recovery Runbook
1. **Never provision a Render-managed database.** The authoritative production database is Neon Serverless PostgreSQL 16.
2. **Neon Instant Snapshot Branching (Zero-Downtime)**:
   - In the **Neon Console** (`console.neon.tech`), select the Ratels project.
   - Go to **Branches** → Click **Create Branch**.
   - Branch Name: `ratels-prod-pre-marketplace-release`
   - Parent Branch: `main` (the active production branch)
   - *Time required: < 5 seconds. Captures an exact byte-level copy-on-write snapshot without connection termination.*
3. **Standalone Compressed Dump Backup (`pg_dump`)**:
   - Run from an authorized administrative workstation or secure environment:
     ```bash
     pg_dump -Fc --no-owner --no-privileges "$DATABASE_URL" -f "ratels_neon_backup_$(date +%Y%m%d_%H%M%S).dump"
     ```
   - Verify backup file size and integrity:
     ```bash
     pg_restore --list "ratels_neon_backup_"*.dump | head -n 30
     ```
4. **CRITICAL WARNING — Distinction Between Routine Rollback and Disaster Recovery**:
   > [!CAUTION]
   > **RESTORING A DATABASE SNAPSHOT IS DESTRUCTIVE AND CAUSES PERMANENT DATA LOSS!**
   > Restoring a historical Neon branch snapshot or applying `pg_restore` will **permanently destroy all transactions, user registrations, orders, and state changes created after the snapshot was taken**.
   > - If an application bug or UI issue arises post-release, **DO NOT restore the database**. Perform a **Safe Application Rollback** (revert Render deployment to previous commit). Migrations 19–22 are purely additive and backward-compatible with older backend binaries.
   > - Database restoration must be reserved **strictly as a last-resort disaster recovery mechanism** for catastrophic migration failure occurring *before* any live user traffic has resumed.
5. **Standalone Neon Branch Disaster Restoration (Pre-Traffic Catastrophic Recovery Only)**:
   - **Method 1 (Instant Branch Promotion - Neon Console)**:
     - In Neon Console → Select branch `ratels-prod-pre-marketplace-release`.
     - Click **Set as Default Branch** (or update `DATABASE_URL` in Render to point to the snapshot branch endpoint).
   - **Method 2 (`pg_restore` Disaster Recovery)**:
     ```bash
     pg_restore -v --clean --if-exists --no-owner --no-privileges -d "$DATABASE_URL" ratels_neon_backup_*.dump
     ```

### Phase B: Render Deployment Control & Environment Setup
1. **Temporarily Pause Auto-Deploy in Render Dashboard**:
   - For `ratels-api` (or `ratels`): Go to **Settings** → Set **Auto-Deploy** to `No`.
   - For `ratels-web`: Go to **Settings** → Set **Auto-Deploy** to `No`.
   - *Rationale: Prevents partial, concurrent, or premature deployment when the branch is merged.*
2. **Verify Release Commit**:
   - Confirm git HEAD on release branch matches audited commit (`c9b5896` or release PR merge commit).
3. **Environment Group Configuration (`ratels-production-env`)**:
   - In Render Dashboard → Environment Groups → Create or update group `ratels-production-env`.
   - Populate verified environment variables:
     - `DATABASE_URL`: Neon PostgreSQL connection string (ending with `?sslmode=require`).
     - `JWT_SECRET`: Preserve the existing production secret string (do not regenerate).
     - `BREVO_API_KEY`: Existing Brevo API key.
     - `ALLOWED_ORIGINS`: `https://app.ratelsworldwide.org,https://ratelsworldwide.org,https://ratels-web.onrender.com`.
     - `PAYMENT_PROVIDER`: Set to `unconfigured` until live Paystack keys are ready, or `paystack` once keys are provided.
     - `PAYSTACK_SECRET_KEY` & `PAYSTACK_PUBLIC_KEY`: Live credentials (if activating Paystack).
     - `R2_ACCESS_KEY_ID`, `R2_SECRET_ACCESS_KEY`, `R2_ACCOUNT_ID`, `R2_BUCKET_NAME`: Media assets bucket.
     - `SAFEGUARDING_REPORTING_ENABLED`: Keep `false` unless 32-byte `SAFEGUARDING_ENCRYPTION_KEY` is supplied.
4. **Verify Existing Services (No Blueprint Provisioning)**:
   - Ensure Render updates the existing services (`ratels` native Go web service, and `ratels-web` native Node.js web service) rather than provisioning duplicate services or blueprints.

### Phase C: Deploy Backend Service (`ratels`)
1. **Deploy Native Go Backend (`ratels`)**:
   - Runtime: Native Go (Build Command: `cd apps/api && go build -o app ./cmd/server`, Start Command: `./apps/api/app` or `./app`).
   - Inspect build logs: Verify Go compilation succeeds with all embedded migration scripts.
   - Inspect startup logs:
     - Verify PostgreSQL connection established to Neon.
     - Verify distributed migration advisory lock acquired.
     - Verify migrations 19, 20, 21, and 22 execute and report `applied`.
     - Verify reference geographic seeder completes (6 zones, 37 states, 774 LGAs).
     - Verify in-process outbox worker, report scheduler, and refund recovery engine initialize.
     - Verify server reports: `listening on :8000` (or `PORT`).

### Phase D: Deploy Frontend Service (`ratels-web`)
1. **Deploy Native Node.js Frontend (`ratels-web`)**:
   - Runtime: Native Node.js (Build Command: `cd apps/web && npm install && npm run build`, Start Command: `cd apps/web && npm run start`).
   - Pass environment variable: `NEXT_PUBLIC_API_URL=https://api.ratelsworldwide.org/api/v1` (or relative `/api/v1` via Next.js proxy rewrite).
   - Verify static generation (58/58 pages compiled clean).
   - Verify health check on root `/`.

---

## 7. Post-Deployment Smoke Testing & Verification Checklist

Immediately following deployment, run the following verification sequence:

- [ ] **1. API Health Check**:
  `curl -sI https://api.ratelsworldwide.org/health` → Expect HTTP 200 with `status: ok` and `database: healthy`.
- [ ] **2. Frontend Storefront**:
  Visit `https://app.ratelsworldwide.org/marketplace` → Verify catalogue loads, category filters render, and search bar is interactive.
- [ ] **3. Authentication & Session Cookies**:
  Log in with a test member account → Verify `ratels_session` HttpOnly cookie is set with `Secure`, `SameSite=Lax`.
- [ ] **4. Cart Isolation & Naira Formatting**:
  Add product to cart → Verify cart total is formatted in Nigerian Naira (`₦`), and delivery fee calculation reflects correctly.
- [ ] **5. Seller Portal Protection**:
  Visit `/marketplace/seller` while logged out → Verify immediate redirect to `/login?redirect=%2Fmarketplace%2Fseller`.
- [ ] **6. Admin Marketplace Workspace**:
  Visit `/admin/marketplace` with Coordinator credentials → Verify oversight tabs (Businesses, Products, Orders, Disputes) reflect live DB data.
- [ ] **7. Geographic Jurisdictions**:
  Verify an LGA coordinator can only view and manage orders within their assigned LGA.
- [ ] **8. Paystack Webhook Ping**:
  Send a test webhook ping from Paystack Dashboard → Verify HTTP 200 response in Render log.

---

## 8. Rollback & Emergency Incident Procedures

In the event of an unrecoverable runtime regression, use the appropriate procedure based on failure domain:

### A. Safe Application Rollback (Fast, Lossless, Routine)
> [!TIP]
> **Zero Data Loss**: Migrations 19–22 are purely additive and backward-compatible. Older application code continues to run safely against the new database schema without data corruption.
1. In Render Dashboard → Select `ratels` (backend) or `ratels-web` (frontend) → Go to `Deploys` tab.
2. Select the previous stable commit build → Click **Rollback to this deploy**.
3. Render immediately reactivates the previous deployment artifact in under 60 seconds without rebuild.
4. **DO NOT revert or restore the database**. All active user sessions, unfulfilled orders, and audit logs remain intact.

### B. Payment Circuit-Breaker (Immediate Operational Kill-Switch)
If Paystack returns upstream errors, duplicate billing is suspected, or keys need immediate rotation:
1. In Render Dashboard → Set `PAYMENT_PROVIDER=unconfigured` on the `ratels` backend service.
2. The checkout endpoint immediately disables new payment transactions safely with HTTP 503 (`PAYMENT_PROCESSING_UNAVAILABLE`), protecting buyers while preserving existing cart items and product catalogues.

### C. Database Disaster Recovery (Severe Data Loss / Catastrophic Recovery Only)
> [!CAUTION]
> **DANGER: Restoring a historical database snapshot permanently wipes out all live transactions, orders, payments, and member registrations created after the snapshot was taken.**
1. Database restoration is **strictly reserved** for catastrophic migration failure or irrecoverable data corruption detected *prior* to opening the platform to live user traffic.
2. If catastrophic failure occurs during the deployment window before live traffic:
   - **Option 1 (Instant Neon Branch Switch)**: In Neon Console, set the pre-release branch (`ratels-prod-pre-marketplace-release`) as default, or update Render `DATABASE_URL` to point to the snapshot branch.
   - **Option 2 (Manual Schema Downgrade)**:
     ```bash
     # Roll back Migration 22 using goose:
     goose -dir apps/api/migrations postgres "$DATABASE_URL" down
     # Or apply the down block from apps/api/migrations/20261027000022_marketplace_shipment_and_refund_hardening.sql via psql:
     psql "$DATABASE_URL" -c "
       DROP INDEX IF EXISTS uq_idx_mkt_shipments_order_business;
       CREATE UNIQUE INDEX IF NOT EXISTS uq_idx_mkt_shipments_order_business ON marketplace_shipments(order_id, business_id);
       DROP INDEX IF EXISTS idx_mkt_shipment_items_refund;
       ALTER TABLE marketplace_shipment_items DROP COLUMN IF EXISTS refund_id;
       ALTER TABLE marketplace_shipment_items DROP COLUMN IF EXISTS refunded_quantity;
       DROP INDEX IF EXISTS idx_marketplace_refunds_order_item;
       DROP INDEX IF EXISTS idx_marketplace_refunds_business;
       DROP INDEX IF EXISTS idx_marketplace_refunds_shipment;
       ALTER TABLE marketplace_refunds DROP COLUMN IF EXISTS order_item_id;
       ALTER TABLE marketplace_refunds DROP COLUMN IF EXISTS business_id;
       ALTER TABLE marketplace_refunds DROP COLUMN IF EXISTS shipment_id;
       DELETE FROM goose_db_version WHERE version_id = 20261027000022;
     "
     ```
     *(Note: If orders reside in `requires_reconciliation`, Migration 20 will block schema rollback until records are administratively reconciled).*
