# RATELS WORLDWIDE — PRODUCTION DEPLOYMENT RUNBOOK & OPERATIONAL PLAYBOOK

## 1. System Overview & Deployment Architecture

The Ratels Worldwide management platform comprises three distributed containerized services deployed on Render backed by a managed PostgreSQL 16 cluster, Cloudflare R2 object storage, and external communications/fintech providers (Paystack, Brevo, Termii):

1. **`ratels-api`** (Web Service): High-performance Go 1.25 REST API server exposing `/api/v1` routes with connection pooling, automatic startup migration execution, in-process outbox worker, report scheduler, and refund recovery engine.
2. **`ratels-web`** (Web Service): Next.js 15 client dashboard providing responsive web interfaces for Buyers, Sellers, Coordinators, and Administrators.
3. **`ratels-worker`** (Background Worker): Dedicated container running `/app/worker` executing durable outbox notifications, scheduled monthly transparency reports, and background refund reconciliation.
4. **Neon Serverless PostgreSQL 16**: Primary authoritative database cluster with automated PITR daily backups, SSL transport encryption (`sslmode=require`), zero-downtime branching, and connection pooling. **Do not provision a Render-managed database.**
5. **Cloudflare R2**:
   - `ratels-marketplace-assets-prod`: Public S3-compatible bucket storing validated product and catalogue imagery.
   - `ratels-private-reports-prod`: Dedicated private bucket storing signed transparency snapshot exports and safeguarding records.

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

### Phase A: Neon PostgreSQL Baseline Snapshot (Zero-Downtime Backup)
1. **Never provision a Render-managed database.** The authoritative production database is Neon Serverless PostgreSQL 16.
2. In the **Neon Console**, create an instant point-in-time branch snapshot before release:
   - Branch name: `ratels-prod-pre-step66`
   - Parent branch: `main` (or production branch)
3. Take a standalone compressed snapshot dump via Render shell or local terminal:
   ```bash
   pg_dump -Fc "$NEON_DATABASE_URL" -f "ratels_neon_backup_$(date +%Y%m%d_%H%M%S).dump"
   ```

### Phase B: Render Environment Group Configuration & Service Linking
1. In Render Dashboard → Environment Groups → Create or update group `ratels-production-env`.
2. Populate the required environment variables:
   - `DATABASE_URL`: Neon PostgreSQL connection string (ending with `?sslmode=require`).
   - `JWT_SECRET`: Preserve the existing production secret string.
   - `BREVO_API_KEY`: Existing Brevo API key.
   - `PAYSTACK_SECRET_KEY` & `PAYSTACK_PUBLIC_KEY`: Live keys (or set `PAYMENT_PROVIDER=unconfigured` until keys are provisioned).
   - `R2_ACCESS_KEY_ID`, `R2_SECRET_ACCESS_KEY`, `R2_ACCOUNT_ID`, `R2_BUCKET_NAME`: Media assets bucket.
   - `SAFEGUARDING_ENCRYPTION_KEY`: 32+ byte key (or leave `SAFEGUARDING_REPORTING_ENABLED=false`).
3. Link `ratels-production-env` to existing services: `ratels-api` (or `ratels`), `ratels-worker`, and `ratels-web`.
4. **Service Verification**: Verify that existing services in Render (`ratels-api` / `ratels` and `ratels-web`) are linked rather than creating duplicate web services.

### Phase C: Deploy Backend Services
1. Deploy `ratels-api`:
   - Inspect build logs: verify Docker multi-stage build compiles `server`, `worker`, `report-cron`, `seed`, and `admin` binaries.
   - Inspect startup logs:
     - Verify PostgreSQL connection established to Neon.
     - Verify migrations 19, 20, 21, and 22 execute and report `applied`.
     - Verify reference geographic seeder completes (6 zones, 37 states, 774 LGAs).
     - Verify server reports: `listening on 0.0.0.0:8000`.
2. Deploy `ratels-worker`:
   - Inspect worker startup logs:
     - `Transactional Notification Outbox & Cleanup Reminder worker started.`
     - `Standalone Notification Outbox, Reports Scheduler & Marketplace Refund Recovery Worker running.`

### Phase D: Deploy Frontend
1. Deploy `ratels-web`:
   - Pass build argument: `NEXT_PUBLIC_API_URL=https://api.ratelsworldwide.org/api/v1`.
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

In the event of an unrecoverable runtime regression:

1. **Fast Web Service Rollback**:
   - In Render Dashboard → Select `ratels-api` (or `ratels-web`) → Go to `Deploys` tab.
   - Select the previous stable commit build → Click **Rollback to this deploy**.
   - Rollback completes in < 60 seconds without rebuild.
2. **Database Rollback Precautions**:
   - **Do NOT roll back migrations if live customer orders have been placed during deployment window.**
   - If rollback is mandated before any live traffic:
     ```bash
     # To roll back Migration 22 only:
     psql "$DATABASE_URL" -f apps/api/migrations/down/20261027000022_down.sql
     ```
   - If orders reside in `requires_reconciliation`, Migration 20 will block schema rollback until orders are administratively cleared.
3. **Payment Kill-Switch**:
   - If Paystack returns upstream errors or duplicate billing is suspected:
     - Set `PAYMENT_PROVIDER=unconfigured` on `ratels-api`.
     - The checkout endpoint immediately disables new transactions safely with HTTP 503 (`CHECKOUT_DISABLED`), preventing rogue charges while preserving cart contents.
