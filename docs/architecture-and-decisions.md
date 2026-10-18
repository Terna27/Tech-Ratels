# Nigerian Ratels Management System — Architecture & Decisions Record (Phase 1)

## 1. Context and Executive Summary
The Nigerian Ratels platform is an operational management system designed for the movement associated in the project brief with Chief VeryDarkMan (VDM). It delivers a robust, accessible, and scalable digital foundation for membership, administration, monthly cleanups, leadership appointments, and preparation for future modules (humanitarian relief, inventory, commerce, careers, community projects, and the Martins Vincent Otse Foundation).

This document formalizes the architectural baseline and technical decisions established for **Phase 1** in direct alignment with `Nigerian_Ratels_Complete_Blueprint_v2.pdf` and the technology directive selecting **Go (Golang)** for the backend.

---

## 2. Key Architectural Decisions (ADRs)

### ADR-01: Modular Monolith in Go with Clean Domain Boundaries
- **Decision**: Adopt a modular monolith inside `apps/api` (Go, `net/http`, `chi`) and `apps/web` (Next.js 15, React 19).
- **Rationale**: Microservices introduce premature distributed systems complexity. A modular monolith enforces strict domain boundaries through distinct domain packages (`geography`, `identity`, `leadership`, `activities`, `participation`, `foundation`, `admin`, `audit`) that communicate via clean Go service interfaces while sharing common database transaction semantics.
- **Entry Points & Organization**:
  - `apps/api/cmd/server`: HTTP server daemon.
  - `apps/api/cmd/seed`: Idempotent reference data populator.
  - `apps/api/internal/`: Domain packages (HTTP handlers thin, business rules in services, queries in repositories).

### ADR-02: Relational Data Model & PostgreSQL Persistence
- **Decision**: Use PostgreSQL (16+) as the primary persistent datastore via `jackc/pgx/v5`, type-safe query generation via `sqlc`, and versioned migrations via `goose`.
- **Rationale**: The domain demands strict relational integrity, ACID transactions for registrations and capacity locking, append-only ledgers for audit events, and hierarchical geographic foreign keys (Zones -> States/Territories -> LGAs/Area Councils).
- **Type Safety**: `sqlc` generates compile-time validated Go types and query methods directly from native PostgreSQL DDL and queries, eliminating ORM overhead and runtime SQL parsing bugs.

### ADR-03: Geopolitical Structure & FCT Modeling
- **Decision**:
  - The 6 geopolitical zones are seeded as top-level geographic regions.
  - The 36 States plus the Federal Capital Territory (FCT) are modeled with a strict discriminator (`is_fct=true` for FCT). FCT is explicitly treated as a federal territory, **not a 37th state**.
  - Local Government Areas (LGAs) and FCT Area Councils (Abaji, Abuja Municipal, Bwari, Gwagwalada, Kuje, Kwali) reference their respective parent State or FCT.
  - Capital cities and event venues are kept distinct from administrative local areas to allow multiple venues within a capital or LGA.
- **Reference Data Integrity**: Administrative reference data includes metadata on data provenance and revision date. Unofficial or developmental local datasets are explicitly flagged as sample data.

### ADR-04: Scoped Role-Based Access Control (RBAC) & Multi-Tenancy by Jurisdiction
- **Decision**: Authorization is enforced server-side by combining:
  1. **Role** (`national_admin`, `zonal_coordinator`, `state_coordinator`, `lga_representative`, `member`).
  2. **Geographic Scope** (`national`, `zone_id`, `state_id`, `lga_id`).
  3. **Appointment Status** (`active`, `acting`, `expired`, `revoked`).
- **Invariants**:
  - A coordinator cannot read, export, or modify records belonging to another jurisdiction by tampering with parameters or route IDs.
  - Expired or revoked appointments lose access immediately.
  - New user registrations default strictly to the `member` role; administrative and coordinator privileges cannot be selected during registration and require an authorized appointment.
  - Members cannot approve their own event participation or expenses.

### ADR-05: Monthly Cleanup Scheduling in Africa/Lagos Timezone
- **Decision**: Monthly cleanups are scheduled for the **last Saturday of each calendar month**, calculated dynamically using the `Africa/Lagos` timezone (UTC+1).
- **Invariant**: The schedule is computed mathematically using calendar algorithms rather than hardcoded day offsets, ensuring correct handling of 28, 29, 30, and 31-day months, leap years, and year rollovers.
- **Publication Gate**: Venues are created in `draft` status by local representatives or state coordinators. Venues remain unpublished until verified and approved by the responsible coordinator. Members see "Location pending" until publication.
- **Concurrency & Capacity**: Event registrations check capacity atomically using database row locks (`SELECT ... FOR UPDATE`) in `jackc/pgx/v5` transactions to prevent overselling during high-concurrency registration spikes.

### ADR-06: Participation Verification & Badge Ledger
- **Decision**:
  - Attendance claims require submission of participation descriptions and notes.
  - Claims must be approved or rejected with a recorded reason by an authorized coordinator whose user ID differs from the participant (`user_id != reviewer_id`).
  - Attendance awards an immutable "First Participation" badge on the participant's first approved activity.
  - Duplicate claims for the same event instance are rejected.
  - A formal appeal and correction workflow is supported for rejected participation claims.

### ADR-07: Contact Verification & Development Delivery Adapter
- **Decision**: Contact verification uses an adapter pattern (`DeliveryAdapter`).
  - In development and test environments, `DevSafeDeliveryAdapter` logs verification codes to an isolated audit log/console and prevents external SMS/email dispatch.
  - In production, a hardened gateway adapter (e.g., Termii, Twilio, Sendgrid) will be plugged in via configuration.
  - Real contact details are never fabricated or simulated as verified without token confirmation.

### ADR-08: Foundation Transparency & Bank Details Protection
- **Decision**:
  - The Foundation entity is named "Martins Vincent Otse Foundation" (provisional display name pending official registration certificate review).
  - Financial donation collection is **not activated in Phase 1**.
  - Bank details require a dual-officer verification protocol (`verified_bank_account_versions`) before publication. In Phase 1, bank details remain `pending_verification` and are never exposed publicly.
  - The public interface clearly displays: *"Donation details will be published after official verification."*

### ADR-09: Append-Only Audit Logging
- **Decision**: All administrative actions, role assignments, appointments, venue publications, and participation reviews append an immutable event record to the `audit_events` table with actor, action, scope, target, timestamp, and metadata.

---

## 3. Module Boundaries & Data Flow (Phase 1)

```
[ Public Web / PWA / Member Portal ]      [ Coordinator / Admin Workspace ]
                    \                                /
                     \                              /
               [ API Gateway / Chi HTTP Layer (/api/v1) ]
                                    |
                    +---------------+---------------+
                    |                               |
          [ Authentication & RBAC ]        [ Audit Event Logger ]
                    |                               |
       +------------+------------+                  |
       |            |            |                  |
 [ Geography ] [ Activities ] [ Leadership ]        |
       |            |            |                  |
 [ Identity  ] [ Badges /   ] [ Foundation ]        |
       |       [ Participate]        |              |
       +------------+----------------+              |
                    |                               |
            [ Repository Layer (sqlc + pgx) ]-------+
                    |
          [ PostgreSQL 16 Relational Engine ]
```
