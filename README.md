# Nigerian Ratels Management System

> **Implementation Baseline:** Blueprint Version 2.0 (17 October 2026)  
> **Backend Architecture:** Go (Golang), net/http, chi, pgx, sqlc, goose, PostgreSQL  
> **Frontend Architecture:** Next.js 15, React 19, TypeScript, Tailwind CSS  
> **Status:** Phase 1 (Operational Foundation) Completed & Tested

Professional, scalable operational management platform for the Nigerian Ratels movement associated with Chief VeryDarkMan (VDM).

---

## 1. Phase 1 Capabilities Implemented

### A. Geopolitical & Administrative Hierarchy
- **6 Geopolitical Zones:** North Central, North East, North West, South East, South South, South West.
- **36 States + 1 Federal Territory:** The Federal Capital Territory (FCT) is strictly modeled as a federal territory (`is_fct=true`), **not a 37th state**.
- **Local Government Areas & FCT Area Councils:** Models the 6 Area Councils of the FCT (Abuja Municipal, Abaji, Bwari, Gwagwalada, Kuje, Kwali) as area councils, plus representative state LGAs from authoritative administrative references.
- **Capital Cities & Venues Decoupled:** State capitals and event venues are kept distinct from local areas to support multiple simultaneous venues in one metropolitan center.

### B. Membership & Authentication
- **Registration & Login:** Secure password hashing with bcrypt, input validation, and unique human-readable membership references (e.g. `RATEL-2026-XXXXX`).
- **Anti-Privilege Guard:** New user registrations default strictly to the `member` role; administrative and coordinator privileges cannot be selected during registration.
- **Contact Verification Adapter:** Uses `DevSafeDeliveryAdapter` in development to capture verification codes in a safe inspection sandbox without sending external SMS/emails.
- **Session Revocation:** JWT sessions are tracked in the database (`user_sessions`) with explicit JTI tracking; logging out revokes the session immediately.

### C. Scoped Role-Based Access Control (RBAC)
- **Roles:** `national_admin`, `zonal_coordinator`, `state_coordinator`, `lga_representative`, `member`.
- **Enforcement:** Every list, detail, and mutation API route enforces scope on the server. A coordinator cannot inspect or alter another jurisdiction's restricted records by changing URL IDs (e.g. Lagos coordinator cannot publish or alter Kano venues).
- **Appointment Governance:** Tracks appointee, role title, jurisdiction level, start/end dates, approval references, and status (`active`, `acting`, `expired`, `revoked`). Revoked appointments forfeit permissions promptly.

### D. Monthly Cleanups Management
- **Lagos Timezone Scheduling:** Dynamically calculates the **last Saturday of each month** in `Africa/Lagos` (UTC+1) using calendar mathematics rather than static day offsets.
- **Publication Gate:** Local representatives propose venues in `draft` status; venues remain unpublished until inspected and approved by authorized coordinators.
- **Capacity Limits:** Atomically checks capacity during registration using database transactions with row-level locking (`SELECT ... FOR UPDATE`) to prevent overselling.
- **Rescheduling Audit:** Rescheduling an event preserves previous timestamps and requires a recorded justification.

### E. Participation Verification & Recognition Badges
- **Review Queue:** Coordinators verify attendance claims with mandatory reasons.
- **Anti-Self-Approval Enforcement:** Coordinators are strictly prohibited from approving their own participation submissions (`reviewer_id != user_id`).
- **Badge Engine:** Automatically awards the **First Participation** badge upon approval of a member's first completed activity.
- **Appeals Workflow:** Supports formal appeals against rejected participation claims with independent review.

### F. Foundation Preparation
- **Provisional Legal Entity:** Configurable as "Martins Vincent Otse Foundation" (provisional display name pending presentation of verified registration documents).
- **Bank Details Protection:** Bank details require a dual-officer verification protocol (`verified_bank_account_versions`). In Phase 1, bank accounts remain `pending_verification` and are never exposed publicly.
- **No Live Payments:** Live donation collection is disabled in Phase 1. Displays honest notice: *"Official donation details and verified bank accounts will be published after regulatory and trustee verification."*

---

## 2. Technology Stack

- **Backend:** Go (Golang) 1.22+
  - Standard `net/http` package with `github.com/go-chi/chi/v5` router.
  - Native PostgreSQL connection pooling and transactions with `github.com/jackc/pgx/v5`.
  - Type-safe query generation with `sqlc`.
  - Versioned database migrations with `goose`.
  - Password hashing with `golang.org/x/crypto/bcrypt`.
  - Session tokens with `github.com/golang-jwt/jwt/v5`.
- **Database:** PostgreSQL 16.
- **Frontend:** Next.js 15 (App Router), React 19, TypeScript, Tailwind CSS, Lucide icons.
- **Testing:** Go's standard `testing` package with table-driven tests and database fixtures.
- **Deployment:** Docker & Docker Compose (`infra/docker-compose.yml`).
- **API Documentation:** OpenAPI 3.0 specification (`docs/openapi.yaml`).

---

## 3. Quick Startup Instructions

### Prerequisites
- Go 1.22+
- PostgreSQL 16
- Node.js 18+ (Node 24 recommended)
- npm 10+
- `sqlc` and `goose` (installed via `go install`)

### Option A: Local Quickstart (Fastest)

1. **Run the all-in-one development launcher:**
   ```bash
   ./scripts/start_dev.sh
   ```
   *This script runs Goose migrations, seeds geography & demo accounts, starts the Go backend on `http://127.0.0.1:8000`, and starts Next.js on `http://localhost:3000`.*

2. **Or run separately:**
   - **Backend:**
     ```bash
     cd apps/api
     goose -dir migrations postgres "postgres://postgres@127.0.0.1:5432/ratels_dev?sslmode=disable" up
     go run ./cmd/seed
     go run ./cmd/server
     ```
   - **Frontend:**
     ```bash
     cd apps/web
     npm run dev
     ```

3. **Access the application:**
   - **Web Interface:** [http://localhost:3000](http://localhost:3000)
   - **API Health Check:** [http://127.0.0.1:8000/api/v1/health](http://127.0.0.1:8000/api/v1/health)
   - **OpenAPI Specification:** [docs/openapi.yaml](docs/openapi.yaml)

### Option B: Docker Compose
```bash
cd infra
docker compose up --build
```

---

## 4. Administrator Bootstrap & Development Accounts

### Bootstrapping First Real Administrator
To create the first production administrator without demonstration data:
```bash
cd apps/api
go run ./cmd/admin --email admin@yourdomain.org --name "Chief Executive Admin"
```
*If `--password` is omitted, a 24-character cryptographically secure password is generated and displayed once.*

### Demonstration Accounts (Development Only)
In development, demonstration accounts can be seeded using `--demo-accounts` or `SEED_DEMO_ACCOUNTS=true` (strictly rejected if `APP_ENV=production`):
```bash
cd apps/api
go run ./cmd/seed --demo-accounts
```

| Role | Email / Identifier | Password | Jurisdiction |
| :--- | :--- | :--- | :--- |
| **National Administrator** | `admin@ratels.example.org` | `Admin1234!` | Nigeria (National) |
| **State Coordinator** | `lagos_coord@ratels.example.org` | `Coord1234!` | Lagos State |
| **Volunteer Member** | `member1@ratels.example.org` | `Member1234!` | Member (Ikeja, Lagos) |

*Seeded accounts use RFC 2606 reserved domains (`.example.org`). The web login page provides one-click quickfill buttons for these evaluation accounts in development mode.*

---

## 5. Running Automated Tests & Security Verification

Run the comprehensive Phase 1 Go test suite:
```bash
./scripts/run_tests.sh
```
Or directly with `go test`:
```bash
cd apps/api
export GOTOOLCHAIN=local
go test -v ./...
```

**Test Coverage Highlights:**
- `harden_integration_test.go`:
  - `TestCookieSessionAndCSRFProtection`: Verifies `HttpOnly: true` cookies, `SameSite=Lax`, Double Submit Cookie CSRF protection (HTTP 403 on missing or invalid `X-CSRF-Token`), and database session revocation on logout (HTTP 401).
  - `TestExpiredAppointmentRevokesAccessHTTP`: Proves expired coordinator appointments promptly forfeit privileged role access over HTTP.
  - `TestPermissionsAndCrossStateIsolationHTTP`: Proves ordinary members cannot create appointments or review claims, and proves cross-state isolation (Lagos coordinator denied in Kano state).
  - `TestCompleteUserJourneyHTTP`: End-to-end user journey (admin login -> draft cleanup -> publish -> member registration -> duplicate and capacity limits -> claim submission -> anti-self-approval check -> independent admin review -> automatic badge award -> database persistence verification).
- `delivery_test.go`: Proves development sandbox capture works and production mode strictly blocks console logging and memory retention of OTP codes.
- `geography_test.go`: 6 zones, 36 states + FCT modeled as a territory, Area Councils, distinct capital cities.
- `auth_test.go`: Registration, anti-privilege guard (forbidden role selection), contact verification, JWT sessions, logout invalidation, duplicate prevention.
- `cleanup_test.go`: Last-Saturday calculation in Lagos time across month lengths, draft venue gates, atomic capacity locking (`ErrEventAtCapacity`).
- `participation_test.go`: Evidence submission, **strict anti-self-approval enforcement**, First Participation badge automatic awarding, appeals.
- `foundation_test.go`: Submitter anti-self-approval (account editor cannot approve their own changes), dual-officer sign-off requirement, provisional name configuration.

---

## 6. Architecture & Future Roadmap

Full architectural decisions and boundaries for later phases are documented in:
- [docs/architecture-and-decisions.md](docs/architecture-and-decisions.md)
- [docs/migration-to-go.md](docs/migration-to-go.md)

| Phase | Module | Scope & Status |
| :--- | :--- | :--- |
| **Phase 1** | **Foundation, Geography, Membership, Cleanups, Badges, Admin** | **Implemented & Tested in Go** |
| Phase 2 | Inventory & Material Distribution | Warehouses, allocations, dispatches, receipts, in-transit discrepancy handling. |
| Phase 3 | Media Moderation & Private Storage | S3-compatible private object storage, metadata stripping, consent review. |
| Phase 4 | Community Requests & Projects | Intake triage, milestone inspections, double-entry balanced project ledgers. |
| Phase 5 | Business Directory & Delivery Fulfilment | Member businesses, branches, products, stock reservations, single-seller cart. |
| Phase 6 | Careers & Interview Coordination | Private CV vault, job postings, explainable rule-based candidate matching. |
| Phase 7 | Foundation Fundraising & Reconciliation | Bank statement matching, Paystack adapter, donation receipts, fund accounting. |
| Phase 8 | Native Mobile Applications | React Native / Expo apps consuming the `/api/v1` REST contract. |