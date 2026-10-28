# Phase 69 — Production hardening

Audited main at `08cdc38` on 2026-10-28. Changes are confined to
`feat/phase69-production-hardening`; no production service, secret, data, or migration was modified.

## Changes and regression coverage

| Finding | Change | Verification |
| --- | --- | --- |
| JWT parser accepted other HMAC algorithms and omitted expiration/session claims | Require HS256, expiration, subject and JTI; validate issued-at when present | JWT rejection cases and valid generated-token test |
| Session lookup did not bind the session owner to the token subject | Reject mismatched session ownership | HTTP authentication regression |
| Unconditional RealIP middleware trusted attacker-supplied forwarding headers | Preserve socket peer and use existing trusted-proxy resolver for authentication IPs | HTTP login with spoofed forwarding headers checks persisted session IP |
| PATCH missing from credentialed CORS allowlist | Allow PATCH with existing origin/header restrictions | CORS preflight regression |
| Cancelled events could still award attendance | Reject cancelled events before verification writes | Cancelled-event test verifies registration remains unchanged |
| Health endpoint returned 200 after database failure | Return 503 and unavailable status | Closed-database readiness regression; real server startup smoke test |
| Cached CSRF token could outlive another tab's login | Read current CSRF cookie before cached fallback | Cookie-rotation test |
| Next.js bundled vulnerable PostCSS 8.4.31 | Pin and override PostCSS to 8.5.32 without a Next.js major upgrade | Source-map traversal/file-disclosure regression, legitimate adjacent map test, frontend build and npm audit |
| Production safeguarding evidence used an ephemeral development store | Use configured private R2 bucket; reject uploads when durable storage is unavailable; prohibit reuse of public media bucket | Production storage factory regression and Go build |
| Backend image declared Go 1.25 despite module requiring Go 1.26 | Align image with module; run API as an unprivileged user | Go build; Docker build covered by added CI |
| Web image installed dependencies without its lockfile and ran as root | Use npm ci and the node runtime user | npm ci/build; Docker build covered by added CI |
| Compose seeded before migrations and built its proxy against an external API | Add explicit migrate command; migrate, seed, then exec server; configure local backend at build and runtime; wait for database readiness | Fresh-database migrate/seed/server smoke test; container/proxy smoke checks in CI |

The existing marketplace, refund, inventory, coordinator, and migration implementations
were inspected and exercised through their regression suites. Existing row locks,
payment amount/currency/reference checks, refund reservation accounting, and attendance
replay protections were retained. No schema changes were needed.

## Validation

- Go 1.26.0: `go test -p 1 ./... -count=1` passed with PostgreSQL 16.15.
  The application test database was migrated and reference-seeded. Migration rollback
  tests used their separate `ratels_migration_audit_db`, avoiding interference with
  application fixtures. After the final safeguarding wiring, storage tests and the
  full Go build passed again.
- `go build ./...` passed, including the new migrate command.
- Fresh database: migrate twice, seed, start server, then `/health` returned HTTP 200
  with `database: healthy`. The second migration invocation applied nothing.
- Frontend: 14 tests passed; one browser suite skipped because local Chrome was
  unavailable. Browser tests require an explicit `TEST_WEB_URL` and a Chrome binary.
- `npm run build` passed: all 59 pages generated, with type checking successful.
- `npm audit --omit=dev`: zero reported vulnerabilities after the PostCSS override.
- `git diff --check` passed.

## CI and operation notes

`.github/workflows/production-checks.yml` runs Go tests/build against disposable
PostgreSQL, frontend tests/build, and a fresh Compose build/start followed by a
frontend-to-backend health request. Docker is unavailable in the editing environment;
these container checks have been prepared but have not run here or on GitHub.

Set `TRUSTED_PROXIES` to the actual reverse-proxy addresses/CIDRs if client forwarding
headers should be honored. Untrusted peers cannot supply their own client address.
The trusted edge must sanitize the forwarding headers it sends.

Production safeguarding evidence requires `R2_SAFEGUARDING_BUCKET_NAME` to identify a
private bucket separate from `R2_BUCKET_NAME`, with the configured R2 credentials
authorized to access it. Remote bucket policy and permissions were not inspected.
Unconfigured evidence uploads now fail rather than claiming success and losing files
at restart. No credentials were changed.

Local browser invocation after starting the frontend:

```sh
cd apps/web
TEST_WEB_URL=http://127.0.0.1:3000 CHROME_BIN=/path/to/chrome npm test
```

## Publication status

The environment can clone public GitHub repositories but has no authenticated GitHub
write access. A push dry-run failed with `could not read Username for https://github.com`.
The feature branch and commit remain local until authentication is connected. No
remote branch or pull request has been created, and no merge was attempted.
