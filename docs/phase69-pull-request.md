# Phase 69: harden authentication, attendance, storage and deployment

## Summary

Harden JWT/session validation, preserve trusted-proxy checks when recording client
addresses, permit PATCH CORS preflights, and prevent attendance verification for
cancelled events. Make database readiness failures return 503, handle cross-tab CSRF
rotation, and replace vulnerable bundled PostCSS with 8.5.32.

Use durable private evidence storage in production, align the backend Docker image
with Go 1.26, run containers without root privileges, and install frontend dependencies
from the lockfile. Add an explicit migration command and correct Compose's initial
migration/seed/start order and frontend proxy destination.

Add targeted regression tests and CI for Go, Next.js, and fresh-container startup.
See [the Phase 69 audit](phase69-production-hardening.md) for each finding, coverage,
operational requirements, and remaining verification limits.

## Validation

- Full Go suite passed against isolated PostgreSQL application and migration databases.
- Go build passed; final storage and security tests passed.
- Fresh database migrate twice → seed → start API → healthy HTTP 200 passed.
- Frontend: 14 passed, one browser suite skipped (requires a running frontend/Chrome).
- Production Next.js build passed, including type checking and 59 generated pages.
- Production npm audit: zero vulnerabilities reported.
- Docker/Compose smoke checks added to CI; not run locally because Docker is unavailable.

No production secrets/data were changed and no new schema migration is introduced.
Production evidence uploads require a separate private R2 bucket; missing durable
storage now causes uploads to fail safely. Forwarding headers require a trusted proxy.
