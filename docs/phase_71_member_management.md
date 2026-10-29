# Phase 71 — Member management and digital membership cards

## Audit and completed gaps

Baseline: `bf146ec4ead17ddb38ba3b49e72e0267fa700607` on `main`.

| Workflow | Baseline | Phase 71 outcome |
| --- | --- | --- |
| Registration / login / contact verification | Existing; unique reference, collision retry, sessions, role-selection protection | Reused. Stale email/contact verification and legacy login repair can no longer undo inactive/suspended/revoked status. |
| Membership reference | Unique non-null database column; assigned at registration | Existing values preserved and database trigger makes references immutable. Only blank legacy references are repaired. |
| Member profile | Existing table, GET summary and PUT profile API; frontend called a nonexistent PUT route | Frontend repaired; validated self-service name, geography, address, biography, photo URL and badge preference; transactional audit. |
| Participation badges | Existing awards and verification workflows | Reused; corrected member dashboard badge URL to `/participations/my-badges`. No duplicate award system. |
| Digital cards / QR | Missing | Responsive private card, server-generated QR, public verification, downloadable HTML and print/save-as-PDF. |
| Coordinator member directory | Phase 70 scoped directory and audited geography corrections | Reused without widening permissions. Super Admin status controls integrated into its member tab. |
| Membership status | Existing users.status and authentication rejection of non-active accounts | Audited Super Admin transitions for ordinary members; session revocation; live public status reflects restrictions immediately. |

## Endpoints

All API paths below use `/api/v1`.

| Method | Path | Access and behavior |
| --- | --- | --- |
| GET | `/members/me` | Existing authenticated own summary, profile, appointments and badges. |
| PUT | `/members/me/profile` | Existing authenticated profile API, now accepting `display_name`, `home_state_id`, `home_lga_id`, `address`, `bio`, `avatar_url`, `is_public_badges`. Unknown fields rejected; no role/status/reference/contact/password changes. |
| GET | `/members/me/card` | Authenticated own card only; name, reference, current status, photo URL, opaque verification path and PNG QR data URI. |
| GET | `/members/me/card/download` | Authenticated own printable HTML attachment, with embedded QR PNG and external HTTPS photo URL. No script; restrictive CSP. |
| GET | `/members/verify/{token}` | Public, exact 64-hex random link token; minimal allowlist described below; 404 for malformed/unknown links; 60 requests/minute per client IP using existing trusted-proxy-aware limiter. |
| PATCH | `/members/{id}/status` | Current database-backed Super Admin authority only. Body: `{ "status": "suspended", "reason": "Administrative review" }`. |
| GET | `/coordinator/members` | Existing Phase 70 paginated, scoped ordinary-member directory. |
| PATCH | `/coordinator/members/{id}/profile` | Existing Phase 70 geography/address correction with reason; current and destination geography must both be authorized. |

Profile updates preserve omitted fields; empty strings clear optional values. Empty state also requires an empty LGA or a state change clears an omitted old LGA. State/LGA relationships are checked against existing reference data. Name must be nonblank and at most 100 bytes, address 255, biography 2,000 and photo URL 255. Photo URLs must be HTTPS, without embedded credentials or fragments; no server-side fetching is performed. Existing R2/public avatar URLs can be used. This phase does not create a second upload service or publish member photographs publicly.

## Frontend routes and access

- `/member/profile`: repaired editor; includes photo URL and card link.
- `/member/card`: responsive card, live verification link, HTML download and browser print/save-as-PDF controls. Missing photos get an initial placeholder.
- `/verify-membership/{token}`: anonymous verification, loading/error/non-active handling, no indexing or referrer transmission.
- `/coordinator`: existing scoped directory; only Super Admin sees membership-status controls. A reason is mandatory; backend authorization is decisive.
- `/member`: discoverable card link and repaired badge loading.

## Privacy and authorization

QR pixels encode only the existing configured frontend origin plus `/verify-membership/{random_token}`. The token contains no user ID, email, phone, reference or credentials. It is an opaque public verification link, not an authentication credential. There is no public member search or reference-to-token lookup.

Public JSON contains exactly `membership_reference`, `status`, and `display_name` **only for active, verified memberships**. Photos, badges, internal identifiers, geographic IDs, addresses, contacts, role claims and appointments are excluded. Inactive/suspended/revoked memberships return the reference and status without the name; legacy `closed` is treated as inactive. Unknown statuses fail closed to verification pending. Card and verification responses use no-store/private, noindex/nofollow/noarchive, no-referrer and nosniff headers. The public page also uses robots/referrer metadata. People holding a card/link can verify it: members are told to keep their card private.

Self-service profile writes lock the current active user and validate geography; only the authenticated user's profile can be edited. The DTO rejects injected privilege fields. Existing cookie CSRF, session checks and coordinator appointment/geography restrictions remain in effect. Moving a home profile does not grant coordinator authority, which still derives from appointments.

Status transitions allow `active`, `inactive`, `suspended`, `revoked`, for another ordinary member without a current active appointment. A verified contact is required for reactivation. Officers must use existing leadership procedures; this API never changes roles or appointments and cannot target the acting administrator. National admins and coordinators cannot use it. Reactivation requires a fresh login: sessions revoked by restriction stay revoked. Repeating the same status is an idempotent no-op. Audit rows and state/session changes commit or roll back together. Audit details record old/new status and the administrative reason; self-profile audit records the action without copying private address/bio/photo fields.

Unused email/contact verification tokens cannot reactivate restricted accounts. Their activation UPDATE rechecks status under PostgreSQL row locking. Existing active pre-verification accounts and pending Super Admin bootstrap compatibility remain supported, but restricted Super Admin login no longer auto-reactivates an account.

## Migration 26

`apps/api/migrations/20261029000026_member_cards.sql`:

1. Repairs only blank legacy references using `RATEL-` plus the existing UUID without hyphens; existing nonblank references remain unchanged.
2. Adds a nonempty-reference check; reuses the existing unique reference constraint.
3. Adds `member_card_links` with a user primary/foreign key and a unique 64-hex token. Cards are created lazily, including for existing members missing an optional profile (the editor repairs that profile on first save). Concurrent first requests return the same persisted link through a PostgreSQL upsert.
4. Adds an immutable-reference trigger. Direct reference edits now fail rather than breaking issued cards.

No existing member, profile, marketplace or appointment tables are dropped. Existing statuses are not reset. Migration 24 and Phase 70.7 migration 25 are unchanged. The application migrator embeds/applies migration 26 transactionally under its existing advisory lock.

Down removes the new trigger/function/check and the card-link table, preserving all members, profiles and repaired references. Rolling down intentionally invalidates issued QR links; reapplying creates new links lazily, so issued cards must be downloaded again after rollback. Do not run rollback against production merely for testing.

## Validation

Final results are recorded below. Tests use disposable PostgreSQL 16 databases, seeded reference geography and local provider adapters. Browser acceptance uses intercepted API fixtures for deterministic UI checks; actual JWT/CSRF/database authorization, profile/status transitions, PNG generation and concurrency are covered by the Go HTTP integration tests.

- Backend integration: reference immutability/uniqueness; existing-member card generation; PNG dimensions; response privacy allowlist and headers; invalid/anonymous access; profile validation and privilege injection; state/LGA mismatch; preserved partial updates; download content; 12 concurrent first-card requests; denied national/coordinator/self/officer status changes; status transitions/audits/session revocation; stale email/contact tokens; restricted Super Admin login; public throttling.
- Migration: transactionally exercises the actual embedded migration's down/up/down SQL, preserving a normal legacy reference, repairing a blank reference and retaining member records.
- Frontend unit: validated profile allowlist, optional clearing, status labels and fail-closed unknown status.
- Browser: mobile layout, QR/identity display, download link, photo rendering, print styles and a verified one-page PDF without JavaScript, correct profile API payload, anonymous verification privacy/noindex/referrer metadata, all non-active statuses and bad links; existing coordinator and subscription browser journeys.

## Deployment and remaining production checks

No new environment variables. Reuse `FRONTEND_BASE_URL`, which must be the canonical public HTTPS frontend origin before issuing cards. A wrong existing origin produces a QR pointing to the wrong site. No payment keys or Paystack checkout configuration are changed or enabled.

One necessary runtime dependency is added: `github.com/skip2/go-qrcode`, pinned to `v0.0.0-20200617195104-da1b6568686e`, for local QR generation without a third-party QR service. No browser dependency is added.

Before production use:

1. Deploy through the existing Render process and confirm migration 26 is applied successfully; preserve normal backups.
2. Verify a real member's existing reference, avatar and geography, profile save, HTML download and print/PDF on desktop/mobile. The downloadable HTML embeds the QR but loads the photo from its HTTPS URL; photo availability remains dependent on that URL/network. Print/save-PDF captures the loaded photo.
3. Scan a real production card on a second device and check the canonical URL/current status. A downloaded/printed status is a snapshot; online verification is authoritative.
4. Check legitimate Super Admin status changes and fresh-login requirements in staging; confirm regional coordinators remain scoped. Configure missing member geography through existing workflows.
5. Confirm Render health/deploy logs and trusted proxy configuration. Deployment and production account checks are not claimed by local tests.

## Final verification results

| Check | Result |
| --- | --- |
| `go test -p 1 ./... -count=1` | Passed with a separate disposable migration-audit database. |
| `go test -race ./internal/handler -run '^TestPhase71\|^TestPhase70' -count=1` | Passed; actual database/JWT authorization, membership and coordinator concurrency. |
| Final membership race rerun | Passed with legacy missing-profile coverage. |
| `go test ./internal/handler -run '^TestPhase71' -count=1` | Passed against final card/download implementation. |
| `TEST_DATABASE_URL=... go test -race ./internal/subscription -count=1` | Passed using local test adapters; no real checkout or payment configuration changes. |
| `go vet ./...`, `go build ./...` | Passed. |
| `npm test` | 25 entries: 21 passed, 4 browser entrypoints skipped without TEST_WEB_URL. Dedicated browser results below cover these entrypoints. |
| `npm run build` | Passed, including private card and public verification routes. |
| Browser acceptance | 25 entries passed, zero failures/skips, across membership, coordinator, subscription and existing marketplace browser suites. |
| `git diff --check`, formatting and secret-pattern scan | Passed. |
| Protected Phase 70.7 source/config comparison | Unchanged. |

The first disposable-database run failed because the portable PostgreSQL package lacked LLVM JIT and migration audit tests were pointed at shared product fixtures. Disabling JIT only in the local test server and using the suite's separate migration-audit database resolved these environment failures; no production PostgreSQL or payment configuration was changed. All new Phase 71 cases ran without skips. The default Go run retains existing optional integration skips; the subscription integration/race suite was run separately with its disposable database enabled. No live external-provider calls are claimed.

## Exact changed files

- `.github/workflows/production-checks.yml`
- `apps/api/go.mod`
- `apps/api/go.sum`
- `apps/api/internal/db/membership_queries.go`
- `apps/api/internal/handler/member_handler.go`
- `apps/api/internal/handler/membership_handler.go`
- `apps/api/internal/handler/membership_test.go`
- `apps/api/internal/handler/router.go`
- `apps/api/internal/migrator/membership_test.go`
- `apps/api/internal/service/auth_service.go`
- `apps/api/internal/service/membership_service.go`
- `apps/api/migrations/20261029000026_member_cards.sql`
- `apps/web/src/app/coordinator/page.tsx`
- `apps/web/src/app/member/card/page.tsx`
- `apps/web/src/app/member/page.tsx`
- `apps/web/src/app/member/profile/page.tsx`
- `apps/web/src/app/verify-membership/[token]/page.tsx`
- `apps/web/src/app/verify-membership/layout.tsx`
- `apps/web/src/lib/membership.ts`
- `apps/web/src/lib/types.ts`
- `apps/web/test/membership-browser.test.mjs`
- `apps/web/test/membership.test.mjs`
- `docs/phase_71_member_management.md`
