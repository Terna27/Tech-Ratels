# Phase 70 — Coordinator Operational Workflows

## Audit and outcome

Audited baseline: `332d75ffdbfd095973727bc3400621a0f090fabf` on `main`.

| Workflow | Audit finding | Completed work |
|---|---|---|
| Coordinator dashboard | Regional overview counted national users, activities and queues; LGA coordinator route omitted | Database-backed scope resolver, scoped existing overview and `/coordinator` workspace |
| Member management | Existing member listing was national-only; no safe regional correction workflow | Paginated scoped ordinary-member directory; reason-required geography/address corrections with transactional audit |
| Scheduling | Create/publish/reschedule/cancel already implemented | Reused these endpoints in coordinator workspace; validate state/LGA/zone hierarchy; protect unpublished activity reads |
| Attendance | Migration 24 and existing services already support check-in codes, verification, anti-self rules and time windows | Reused existing attendance page and endpoints; retain current appointment/account checks and regression coverage |
| Inventory | Requests, approval, reservations, dispatch, receipts, discrepancies and ledger already exist, but many HTTP reads were nationwide and mutations relied on broad role gates | Scoped warehouse/stock/request/transfer/discrepancy reads; source/destination authorization; audited mutations and concurrency locks |
| Warehouse visibility | Unspecified active filter selected inactive warehouses | Active warehouses now appear by default |
| Reports | Regional users could list unrelated unpublished snapshot metadata and global system views | Filter snapshots and totals before pagination, check persisted snapshot scope on mutations, restrict system-wide metadata to national administration; add scoped operational report/CSV |
| Appointments | Replacement was super-admin-only, but national admins could still create LGA appointments | All appointment mutations require current Super Admin authority, including database verification; preserve specific denial response |

Migration 24 was inspected rather than recreated. It already adds event description/zone, check-in credentials and attendance verification fields, and participation evidence/reversal tracking. No schema migration is needed for this work. Migration 24 and Phase 70.7 migration 25 remain unchanged.

**Phase 70.7 payment code, subscription configuration, environment examples, Render configuration, Paystack credentials and checkout flags were not changed. No checkout was enabled.** No production database or live provider was used.

## Authority model

| Actor | Operational access | Appointment authority |
|---|---|---|
| Super Admin | National operations and administrative oversight | Create, replace, modify, suspend, reactivate and revoke |
| National admin | National operational access under existing financial/report policies | None |
| Zonal coordinator | States/LGAs inside current appointed zones | None |
| State coordinator | Current appointed state and its LGAs | None |
| LGA coordinator / LGA representative | Exact appointed LGAs | None |
| Ordinary member | Existing self-service/public workflows and their own material requests | None |

Regional authority requires an **active account** and a current active appointment whose start date has arrived and end date has not passed. Primary-role labels and JWT role claims alone do not grant a region. Multiple appointments form a union. Legacy zone/state codes are resolved to canonical IDs. Suspended, expired, future and replaced appointments do not grant scope.

The existing identity/session/CSRF middleware remains in use. New coordinator routes require authentication; services resolve authority from the database. The inventory and report HTTP groups always attach the authenticated actor before invoking scoped reads. Actor-less internal/background reads remain trusted internal APIs; every exposed HTTP read uses the actor context.

### Member corrections

The coordinator directory lists ordinary, unappointed members within scope. It omits password hashes, session data, phone numbers, email addresses and private administrative records. Pagination/search/counts are applied to the scoped population.

Corrections accept only `home_state_id`, `home_lga_id`, `address` and a required `reason`. Both existing and destination geography must be authorized, the LGA must belong to the state, and self-correction/active-officer correction through this endpoint is forbidden. Account activation, contact verification, roles and appointments cannot be changed. Unknown JSON fields are rejected. The correction and audit event commit together.

### Activities and attendance

The workspace reuses existing activity endpoints to create published activities or drafts, publish drafts, reschedule with a reason, cancel with a reason, and open attendance management. Local browser times are sent as UTC timestamps. The backend validates date ordering and geographic hierarchy.

Public activity reads expose published events. Unpublished lists require authentication and scope; unpublished details outside scope return not found. Attendance continues to use existing database-backed jurisdiction checks, unpredictable check-in codes, check-in windows, duplicate-verification protection and anti-self verification. No second attendance implementation or evidence system was added.

### Inventory

- Existing requests/stock/transfer forms stay in the existing inventory page.
- Location, balance, low-stock, request, transfer, discrepancy, summary and ledger responses are scoped. Ledger filtering happens before pagination/counting.
- Request approval uses the **actual destination warehouse**, not client-supplied jurisdiction labels, and still rejects self-approval.
- Allocation and dispatch require authority over the source warehouse; confirmation requires authority over the destination. Cross-region shipping can therefore be dispatched by the source and received by the destination without granting either officer access to unrelated stock.
- The transfer-destination directory exposes only active warehouse IDs, names, codes and types. It does not expose nationwide balances, addresses, custodians or contact details.
- Existing catalogue/warehouse administration and outbound dispatch route policies remain national/zonal/state; LGA workflows include local requests, approved request review, receipts and scoped ledgers.
- Requests and transfers are row-locked for allocation/review/dispatch/receipt transitions. Concurrent repeated confirmations cannot credit destination stock twice. Discrepancy resolution also locks its row.
- Stock-movement ledgers remain in use. Review, allocation, transfer creation, dispatch, receipt and discrepancy actions write audit records inside the same transaction, with an authorized-scope snapshot.
- Frontend receive actions are disabled for destinations outside the actor's visible warehouses; administrative controls are disabled for LGA-only appointments.

### Reports

The new report returns scoped current member/queue totals and period-filtered activity/attendance totals, plus an LGA breakdown. Regional exports are authenticated and audited. Date ranges use UTC with an exclusive end and a maximum span of 366 days. Members with only a state assigned can appear in state totals without appearing in an LGA breakdown.

Existing report snapshots are reused. Regional snapshot lists/counts are filtered before database pagination. Administrative detail and mutation checks use persisted snapshot geography and current database appointments; caller filters cannot confer authority. System-wide report scheduler, settings, export-index and audit-index views remain national administration only. Existing sanitized published-report routes are preserved. Report publication remains Super Admin-only, and existing financial governance/self-approval policies are retained. HTTP callers cannot label themselves as system report actors.

## Routes and integration

New API prefix: `/api/v1`.

| Method | Path | Purpose |
|---|---|---|
| GET | `/coordinator/dashboard` | Scoped summary, geography and reporting period |
| GET | `/coordinator/members?search=&page=1&limit=25` | Ordinary-member directory; maximum page size 100 |
| PATCH | `/coordinator/members/{id}/profile` | Audited authorized correction |
| GET | `/coordinator/events?state_id=` | Scoped events, bounded to 500 newest by start time |
| GET | `/coordinator/reports?from=YYYY-MM-DD&until=YYYY-MM-DD` | Operational aggregation |
| GET | `/coordinator/reports/export?from=&until=` | Audited scoped CSV |
| GET | `/inventory/transfer-destinations` | Minimal dispatch destination directory |

Profile correction body:

```json
{
  "home_state_id": "authorized-state-id",
  "home_lga_id": "authorized-lga-id",
  "address": "Corrected address",
  "reason": "Member confirmed the correction"
}
```

Frontend route: **`/coordinator`**, with overview, members, activities, inventory and reports sections. Linked from the existing regional admin overview and inventory page. Activity attendance links open the existing `/admin/activities/{id}/attendance` page. Inventory opens the existing `/admin/inventory` page.

The existing reports page opens regional users on scoped snapshots and hides national system dashboards, while linking to the new operational reports.

No new runtime variables, payment setup, storage integration or background service is required.

## Validation

Final results and exact file inventory follow below. Test setup uses disposable PostgreSQL 16 and existing migration/seed commands. Existing positive fixtures that granted operational authority to inactive/unverified accounts or role labels alone were updated to provide legitimate active accounts/appointments; negative authorization assertions were retained. No production data was changed.

## Remaining operational steps and limits

- Deploy through the existing process; no deployment setting was changed or live deployment verified here.
- Super Admin should verify each real coordinator's active account, appointment dates and geographic IDs. A regional role label without an appointment now intentionally fails authorization.
- Verify existing warehouse zone/state/LGA IDs. Warehouses without a regional geographic assignment remain national-only.
- Populate missing member home geography through authorized existing/self-service or new national correction workflows before expecting those members in regional directories.
- Complete production user acceptance with real coordinator accounts. Automated browser tests intercept API responses; real database/JWT authorization, inventory concurrency and audit behavior are tested separately through HTTP handlers and services.
- The operational event list is bounded to 500 entries; period reports aggregate the full authorized period independently. Member and snapshot lists are paginated.
- No account-contact verification bypass, coordinator appointment delegation, unrelated gallery/community/communications permission expansion, or Phase 70.7 change is included.

## Final verification results

| Check | Result |
|---|---|
| Full backend suite, `go test -p 1 ./... -count=1` | Passed, with separate disposable migration-audit database |
| Coordinator HTTP authorization/concurrency suite, `go test -race ./internal/handler -run '^TestPhase70' -count=1` | Passed; no skipped Phase 70 cases |
| Existing coordinator privilege suite | Passed in the full backend regression suite |
| `go vet ./...` | Passed |
| `go build ./...` | Passed |
| Frontend `npm test` | 18 passed, 0 failed, 3 browser entry points skipped without `TEST_WEB_URL` |
| Dedicated coordinator browser acceptance | 5 test entries passed, 0 failed/skipped: mobile scope, audited correction, UTC scheduling, inventory navigation/CSV visibility and regional snapshot UI |
| Next.js production build | Passed, including `/coordinator` |
| `git diff --check` and changed-file secret-pattern scan | Passed |
| Protected Phase 70.7/configuration path comparison against baseline | Unchanged |

Go integration tests used real disposable PostgreSQL 16, seeded geography, real JWT/session middleware and actual service/query code. Coverage includes zonal/state/LGA/representative scope, cross-region denial, stale or missing appointments, contact/role field injection denial, CSRF, audit persistence, unpublished activity protection, super-admin-only appointment creation, forged service-role denial, actual warehouse authorization overriding forged jurisdiction labels, mismatched warehouse-parent rejection, approved local allocation, concurrent receipt idempotency, and scoped ledger/report metadata.

The browser suite intercepts API responses and does not replace the HTTP/database authorization tests. Existing external-provider and Phase 70.7 opt-in tests keep their existing behavior; no payment configuration was enabled. No live deployment was verified. Earlier failures were fixed by correcting SQL/active-filter bugs, preserving the legacy appointment-denial code, and updating positive fixtures to represent properly active/appointed officers rather than weakening authorization.

## Exact changed-file inventory

| Change | Path |
|---|---|
| Modified | `.github/workflows/production-checks.yml` |
| Modified | `apps/api/internal/handler/activity_handler.go` |
| Modified | `apps/api/internal/handler/admin_handler.go` |
| Modified | `apps/api/internal/handler/inventory_handler.go` |
| Modified | `apps/api/internal/handler/leadership_handler.go` |
| Modified | `apps/api/internal/handler/report_handler.go` |
| Modified | `apps/api/internal/handler/router.go` |
| Modified | `apps/api/internal/handler/super_admin_dashboard_test.go` |
| Modified | `apps/api/internal/service/admin_service.go` |
| Modified | `apps/api/internal/service/cleanup_service.go` |
| Modified | `apps/api/internal/service/cleanup_test.go` |
| Modified | `apps/api/internal/service/coordinator_auth.go` |
| Modified | `apps/api/internal/service/gallery_test.go` |
| Modified | `apps/api/internal/service/inventory_service.go` |
| Modified | `apps/api/internal/service/inventory_test.go` |
| Modified | `apps/api/internal/service/leadership_service.go` |
| Modified | `apps/api/internal/service/participation_test.go` |
| Modified | `apps/api/internal/service/report_service.go` |
| Modified | `apps/api/internal/service/scoped_permissions_test.go` |
| Modified | `apps/web/src/app/admin/inventory/page.tsx` |
| Modified | `apps/web/src/app/admin/page.tsx` |
| Modified | `apps/web/src/app/admin/reports/page.tsx` |
| Created | `apps/api/internal/db/coordinator_queries.go` |
| Created | `apps/api/internal/handler/coordinator_handler.go` |
| Created | `apps/api/internal/handler/coordinator_operations_test.go` |
| Created | `apps/api/internal/service/coordinator_service.go` |
| Created | `apps/api/internal/service/inventory_scope.go` |
| Created | `apps/api/internal/service/operational_scope.go` |
| Created | `apps/web/src/app/coordinator/page.tsx` |
| Created | `apps/web/src/lib/coordinator.ts` |
| Created | `apps/web/test/coordinator-browser.test.mjs` |
| Created | `apps/web/test/coordinator.test.mjs` |
| Created | `docs/phase_70_coordinator_workflows.md` |
