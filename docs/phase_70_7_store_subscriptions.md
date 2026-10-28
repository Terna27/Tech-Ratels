# Phase 70.7 — Marketplace Paid Store Subscriptions

## Delivered behavior

This extends the Go marketplace, existing Next.js UI, Paystack gateway and Brevo email sender. It does not replace product orders, inventory, shipments, refunds, business approval, authentication or R2 storage.

| Plan | Monthly kobo | Yearly kobo | Draft + published products |
|---|---:|---:|---:|
| Basic Store | 200000 | 2000000 | 30 |
| Business Store | 500000 | 5000000 | Unlimited |

Amounts use integer kobo. Yearly pricing is ten monthly payments, saving two months. Basic provides storefront, product/order management and standard support. Business adds sales analytics, featured eligibility and priority support. Existing support fulfillment remains operational, rather than introducing another support-ticket system.

A member creates a business using the existing registration flow, selects a plan, initializes checkout server-side and completes Paystack checkout. Only server verification of the reference, amount and NGN currency grants a paid term. A browser redirect alone grants nothing. Business approval and verification remain separate and unchanged by payment. Store owners and super-admins manage subscriptions; other members and other administrative roles cannot manage subscription finances.

### Billing and transition policy

- Manual renewal only: customer payment authorizations are never saved or automatically charged.
- Calendar months/years use UTC, with end-of-month clamping (January 31 → February 28/29; February 29 annual renewal → February 28).
- Renewal, upgrade and downgrade purchase a full-price next term beginning at the end of the latest valid paid term. There is no immediate upgrade and no proration. After expiry a new term begins at verification time.
- Only one prepaid future term is accepted. Additional purchases wait until it becomes current.
- Downgrading to Basic requires archiving products until at most 30 draft/published products remain. Pending Basic checkout and prepaid Basic terms reserve that ceiling, preventing a concurrent publication from bypassing the downgrade limit.
- Archived products do not count. Restoring an archived product counts as adding a product. Existing products/history are preserved; nothing is automatically deleted.
- Cancellation retains access until the current paid term expires. Pending checkout or a prepaid future term must be reconciled/resolved before cancellation; paid future terms are never silently discarded.
- Suspension blocks commercial operations regardless of payment. Only a super-admin can suspend/resume, with a required audited reason. Payment never silently removes suspension.
- Expiry/suspension blocks new listings and new orders. Existing paid orders remain fulfillable. Stock-only corrections, order/shipment updates and archive operations remain possible.
- Business analytics are checked by the Go service. Featured product eligibility is enforced in the database for Business; entitlement does not itself imply editorial selection.

### Existing-store rollout

Enforcement and checkout default to disabled. Migration 25 records pre-existing stores as legacy. At **first enablement**, all stores then present (including stores created while rollout was disabled) are also enrolled in a single 30-day grace period. Disabling/re-enabling does not restart that clock. Baseline commercial operations and analytics remain usable during grace. The grace deadline is returned to sellers. New stores created after enforcement begins require payment. Explicit Basic selection applies the product ceiling. Keep rollout settings consistent on every API/worker instance.

### Concurrency and payment integrity

The business row is the common PostgreSQL transaction lock for product writes, checkout, term activation, cancellation and administrative changes. Product triggers enforce the cap even outside the Go handler. One subscription state row per business and a locking non-overlap trigger prevent simultaneous paid terms. Each payment reference and each `(business_id, idempotency_key)` is unique. Duplicate or concurrent webhooks can create only one billing term and one paid notification event. Checkout retries reuse the durable reference; interrupted initialization is resumable from payment history.

Subscription references use `sub_` and are isolated from product-order payments. Both the subscription webhook and existing marketplace Paystack webhook route support those references. HMAC is checked before processing. Charge and refund notifications trigger server reconciliation; refund webhook bodies are never accepted as financial truth. The separate Paystack Refund API is paginated by verified transaction ID, validates transaction/currency and deduplicates refund IDs. Full verified refunds void the corresponding entitlement. Partial refunds adjust reporting without automatically prorating access. Provider/reconciliation failures retain the durable payment for retry.

Financial totals count only verified successful payments. Reports distinguish gross, processed refunds, known provider fees and net before unknown fees. Unknown fees are explicitly counted, not presented as known zero. Monthly/yearly billing totals are gross less refunds; calendar-month revenue is also reported in UTC. Summary totals are global. Search/filter applies to detailed tables/exports. Dashboard tables cap at 1,000 rows, seller history at 250 and audit at 250; CSV includes all matching payments.

## Database

`apps/api/migrations/20261028000025_marketplace_store_subscriptions.sql` follows migration 24; no migration 25 existed on the inspected main commit.

New tables:

1. `marketplace_subscription_settings` — disabled rollout flag and first enablement timestamp.
2. `marketplace_subscription_plans` — seeded exact prices and limits.
3. `marketplace_store_subscriptions` — business-bound state and legacy/cancellation tracking.
4. `marketplace_subscription_payments` — durable checkout, idempotency, verification, fees/refunds.
5. `marketplace_subscription_terms` — paid billing/renewal history with dates and void state.
6. `marketplace_subscription_audit` — reconciliation and administrative records.
7. `marketplace_subscription_emails` — deduplicated durable notification events and retries.

Triggers protect product limits/entitlements, new-order entitlements and non-overlapping paid terms. It uses standard PostgreSQL types, transactions and PL/pgSQL, without requiring Neon extensions. Up/Down/Up has been exercised on disposable PostgreSQL 16. Down removes this phase's tables/triggers only; existing marketplace data remains. **After accepting payments, disable the feature for rollback instead of dropping financial history.** Export/retain billing records before any schema rollback. No production database was contacted or modified during implementation.

## API

Prefix all paths with `/api/v1/marketplace`.

| Method | Path | Access / behavior |
|---|---|---|
| GET | `/subscriptions/plans` | Public plans and configuration availability |
| POST | `/subscriptions/webhooks/paystack` | Paystack HMAC, independent of sessions/CSRF |
| GET | `/seller/businesses/{businessID}/subscription/` | Owner/super-admin dashboard |
| GET | `/seller/businesses/{businessID}/subscription/payments` | Owner/super-admin payment history |
| POST | `/seller/businesses/{businessID}/subscription/checkout` | Create/retry checkout |
| POST | `/seller/businesses/{businessID}/subscription/renew` | Buy next full-price term |
| POST | `/seller/businesses/{businessID}/subscription/change-plan` | Buy a different next-term plan |
| POST | `/seller/businesses/{businessID}/subscription/cancel` | Cancel while retaining paid access |
| POST | `/subscriptions/payments/{reference}/verify` | Owner/super-admin provider verification |
| POST | `/subscriptions/payments/{reference}/retry-checkout` | Owner/super-admin interrupted checkout setup |
| GET | `/admin/subscriptions/` | Super-admin financial/history/audit dashboard |
| GET | `/admin/subscriptions/export` | Super-admin CSV download |
| POST | `/admin/subscriptions/payments/{reference}/reconcile` | Super-admin provider reconciliation |
| POST | `/admin/subscriptions/{businessID}/suspend` | Super-admin; JSON `reason` required |
| POST | `/admin/subscriptions/{businessID}/resume` | Super-admin; JSON `reason` required |

Checkout/renew/change-plan body:

```json
{"plan":"basic","interval":"monthly","idempotency_key":"a-client-generated-unique-key"}
```

Admin filter query parameters: `search` (store/owner name, email or owner ID), `plan` and `status`. Existing JWT/cookie middleware, role gates and CSRF apply to user mutations. Seller subscription requests and payment verification reuse the existing rate limiter (60 requests/minute per user per API instance). Webhooks use bounded bodies and HMAC rather than user authentication. The existing `/webhooks/paystack` dispatches subscription references as well, so one Paystack account can retain its configured marketplace webhook URL.

## Frontend routes

- `/marketplace/subscriptions` — responsive plan cards, monthly/yearly prices and secure checkout handoff.
- `/marketplace/seller/subscription` — entitlement dates/status, remaining days, grace, usage, renewal/change/cancel and history/retry.
- `/marketplace/subscriptions/confirmation` — backend-verified pending/failed/successful states and cancelled-return handling.
- `/admin/marketplace/subscriptions` — super-admin revenue, stores/history, search/filter, CSV, reconciliation and suspend/resume.

Navigation is linked from marketplace, pending-business review and seller/admin marketplace pages. Pages use existing Ratels emerald styling, authentication context and same-origin API/CSRF client.

## Configuration and deployment

| Environment variable | Default / use |
|---|---|
| `STORE_SUBSCRIPTIONS_ENABLED` | `false`; explicit entitlement enforcement and checkout rollout |
| `STORE_SUBSCRIPTIONS_PAYSTACK_SECRET_KEY` | Empty; dedicated Paystack secret, start with `sk_test_` |
| `STORE_SUBSCRIPTIONS_PAYSTACK_TEST_SECRET_KEY` | Optional backward-compatible test-key alias, used only when the main subscription key is absent |
| `STORE_SUBSCRIPTIONS_ALLOW_LIVE_PAYMENTS` | `false`; live keys are rejected without this separate explicit opt-in |
| `FRONTEND_BASE_URL` | Existing HTTPS frontend origin; generates the confirmation callback URL |
| `BREVO_API_KEY`, `EMAIL_FROM_ADDRESS`, `EMAIL_FROM_NAME` | Existing Brevo transactional sender configuration |
| `DATABASE_URL` | Existing Neon PostgreSQL URL; use production TLS configuration |
| `TEST_DATABASE_URL` | Disposable database containing `test` in its name, only for integration tests |

The new subscription key is server-only. Existing product-order `PAYSTACK_SECRET_KEY` is not repurposed. API and standalone worker both run bounded lifecycle/reconciliation/email ticks. Distributed transaction/advisory locks protect multiple instances. Mail retries use the existing Brevo sender and a phase-specific durable queue; failure never reverses payment. Event keys and row locks suppress ordinary duplicate delivery. As with other external email systems, a provider accepting a message followed by process/database failure can still cause an at-least-once resend.

Notifications cover activation, renewal/plan change, upcoming expiry (seven-day window), expiry, payment failure and cancellation. API/worker availability is required for timely reminders; time-based entitlement enforcement itself does not depend on a scheduler tick.

Render's checked-in blueprint keeps its existing `autoDeploy: false` settings. This implementation adds disabled subscription flags and a private secret configuration slot to API and worker; it does not change hosting/deploy policy. Actual Render dashboard settings/deploy state are not available through this session.

Before accepting live payments:

1. Deploy with enforcement disabled, inspect migration/application health and set consistent API/worker configuration.
2. Configure a Paystack **test** key and webhook in the correct Paystack test integration. Configure a verified Brevo sender and correct HTTPS frontend URL.
3. Enable the feature, complete a merchant-account Paystack test checkout/webhook and a Brevo delivery check, and inspect the seller/admin records. Communicate the 30-day legacy-store deadline.
4. Only after those checks, deliberately configure a live key **and** set `STORE_SUBSCRIPTIONS_ALLOW_LIVE_PAYMENTS=true`. No automatic recurring charge authorization is implemented.

## Validation

Validation details and exact changed-file inventory follow below. Real merchant-account credentials, production Neon access, live Brevo delivery and Render deployment were not used or verified. HTTP provider fixtures exercise the actual Paystack adapter and refund API; browser acceptance intercepts API/payment responses and cannot charge a customer.

### Verified results

| Check | Result |
|---|---|
| Full Go suite, `go test -p 1 ./... -count=1` | Passed, using the existing separate disposable migration-audit database |
| Go backend build, `go build ./...` | Passed |
| Dedicated PostgreSQL subscription + Paystack suite, `go test -race ./internal/subscription ./internal/payment -count=1` with `TEST_DATABASE_URL` | Passed; no skipped subscription integration cases |
| Subscription HTTP authorization, CSRF and webhook tests | Passed |
| PostgreSQL migration 25 Up/Down/Up | Passed; original store records retained |
| Next.js tests, `npm test` | 16 passed, 0 failed, 2 browser entry points skipped without a test URL |
| Dedicated new subscription browser suite | 5 passed, 0 failed, 0 skipped; mobile pricing/handoff, all confirmation states, seller cancellation, admin reporting/reconciliation |
| Next.js production build | Passed, including four new routes |
| `go vet ./...` | Passed |
| `govulncheck ./...`, Go 1.26.10 | 0 reachable vulnerabilities, 0 imported-package findings; 17 uncalled module advisories remain in inherited dependency graph |
| `npm audit` | 0 vulnerabilities after patching transitive `postcss-selector-parser` 6.1.4 → 6.1.5 |
| Secret-pattern scan and `git diff --check` | Passed |

The inherited Go minimum was updated to 1.26.10 after the original 1.26.0 runtime produced standard-library findings. CI now explicitly runs subscription integration/race tests and the subscription browser suite. The original marketplace browser suite was not run against a seeded live frontend/API here; existing marketplace Go integration/regression tests passed. An initial full-suite attempt with one shared test database caused an inherited inventory-ledger migration assertion to fail; using the repository's separate migration-audit database made the full suite pass. No production DB operations were performed.

The browser tests use intercepted API/payment responses, and provider tests use local HTTP fixtures against the actual Paystack adapter. These do not establish a real merchant-account Paystack test transaction, a real Brevo delivery or a successful Render deployment. Those configuration-dependent acceptance checks remain required before live use.

### Exact file inventory

| Change | Path |
|---|---|
| Modified | `.env.example` |
| Modified | `.github/workflows/production-checks.yml` |
| Modified | `apps/api/cmd/server/main.go` |
| Modified | `apps/api/cmd/worker/main.go` |
| Modified | `apps/api/go.mod` |
| Modified | `apps/api/internal/handler/marketplace_handler.go` |
| Modified | `apps/api/internal/handler/middleware.go` |
| Modified | `apps/api/internal/handler/router.go` |
| Modified | `apps/api/internal/service/marketplace_service.go` |
| Modified | `apps/web/package-lock.json` |
| Modified | `apps/web/src/app/admin/marketplace/page.tsx` |
| Modified | `apps/web/src/app/marketplace/page.tsx` |
| Modified | `apps/web/src/app/marketplace/seller/page.tsx` |
| Modified | `render.yaml` |
| Created | `apps/api/internal/handler/subscription_handler.go` |
| Created | `apps/api/internal/handler/subscription_handler_test.go` |
| Created | `apps/api/internal/payment/paystack_subscription.go` |
| Created | `apps/api/internal/payment/paystack_subscription_test.go` |
| Created | `apps/api/internal/subscription/config.go` |
| Created | `apps/api/internal/subscription/integration_test.go` |
| Created | `apps/api/internal/subscription/service.go` |
| Created | `apps/api/internal/subscription/service_test.go` |
| Created | `apps/api/migrations/20261028000025_marketplace_store_subscriptions.sql` |
| Created | `apps/web/src/app/admin/marketplace/subscriptions/page.tsx` |
| Created | `apps/web/src/app/marketplace/seller/subscription/page.tsx` |
| Created | `apps/web/src/app/marketplace/subscriptions/confirmation/page.tsx` |
| Created | `apps/web/src/app/marketplace/subscriptions/page.tsx` |
| Created | `apps/web/src/lib/store-subscriptions.ts` |
| Created | `apps/web/test/store-subscriptions-browser.test.mjs` |
| Created | `apps/web/test/store-subscriptions.test.mjs` |
| Created | `docs/phase_70_7_store_subscriptions.md` |
