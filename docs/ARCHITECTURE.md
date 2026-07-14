# We People — Architecture

## Principles

1. **System of record first.** Correctness and auditability of people data outrank feature breadth.
2. **Multi-tenant by construction.** Every query is org-scoped; isolation is enforced in the data layer, not left to callers.
3. **Effective-dated truth.** Core facts are timelines, not single values — we can reconstruct state as of any date.
4. **Events are first-class.** Lifecycle changes (hire, transfer, terminate) are recorded events, not just field edits. This drives audit, analytics, and downstream automation.
5. **Modular monolith → services.** Start as one deployable Go binary with clear internal module boundaries. Extract services (payroll, analytics) only when scale demands.

## Stack

| Layer | Choice | Why |
|-------|--------|-----|
| Language (backend) | **Go 1.26** | Fast, statically typed, excellent for services & batch (payroll), simple deploys. |
| HTTP | **chi** | Lightweight, idiomatic router + middleware. |
| DB | **PostgreSQL 16** | Relational integrity, transactions, JSONB for custom fields, mature. |
| DB access | **pgx** + **sqlc** | Type-safe SQL generated from real queries — no ORM magic, full SQL power. |
| Migrations | **golang-migrate** | Versioned, reversible schema. |
| Auth | **JWT** (access+refresh), bcrypt | Stateless access tokens, rotating refresh. |
| Frontend | **React + TypeScript + Vite** | Standard, fast, large ecosystem. |
| Jobs | in-process worker → later a queue | Accruals, payroll runs, notifications. |

## Backend layout (clean architecture)

```
backend/
  cmd/
    api/        # HTTP server entrypoint
    migrate/    # migration runner
    seed/       # demo data
  internal/
    config/     # env config
    database/   # pgx pool, tx helpers
    auth/       # jwt, password hashing, middleware
    httpx/      # server, router, middleware, error responses
    audit/      # audit logging
    org/        # organizations + tenancy
    iam/        # users, roles, permissions (RBAC)
    worker/     # workers, employment records, assignments
    orgstructure/ # departments, locations, positions, org chart
    ...         # (future modules: timeoff, ats, performance, payroll)
  db/
    migrations/ # .sql up/down files
    queries/    # .sql for sqlc
  sqlc.yaml
```

Each **module** (`worker`, `org`, ...) owns three layers:
- **store** — generated sqlc queries + repository methods (data access)
- **service** — business logic, validation, orchestration, emits audit + events
- **handler** — HTTP: decode request → call service → encode response

Handlers never touch the DB directly; services never write HTTP. Cross-module calls go service→service.

## Multi-tenancy

- Every tenant-owned table has `org_id uuid NOT NULL REFERENCES organizations(id)`.
- Auth middleware resolves the caller's `org_id` from their JWT and injects it into a request-scoped context.
- The store layer **requires** an `org_id` argument on every tenant query — it is part of the WHERE clause and (where relevant) unique constraints. No query can span tenants by accident.
- Postgres Row-Level Security (RLS) is enabled as defense-in-depth in a later phase.

## Security

- Passwords: bcrypt (cost 12).
- Access tokens: short-lived JWT (15 min); refresh tokens: rotating, stored hashed, revocable.
- RBAC: role → permissions; middleware guards routes by permission (e.g. `worker:write`). ABAC scoping (own-reports-only) enforced in services.
- All PII access and mutations are audit-logged.
- Secrets via environment/secret manager, never committed.

## Effective dating & events

- `employment_records` and `worker_assignments` carry `effective_date` (and `end_date`) — the "current" row is the one whose range contains today.
- Every mutation writes a `lifecycle_events` row (type, effective_date, payload) → the audit + analytics backbone.

## Error handling & API conventions

- REST, JSON. Resource-oriented URLs under `/api/v1/`.
- Consistent error envelope: `{ "error": { "code", "message", "details" } }`.
- Validation errors → 422; auth → 401; permission → 403; not found → 404.
- Pagination: cursor or `?limit=&offset=`; list responses `{ "data": [...], "meta": {...} }`.

## Testing

- Unit tests on services (business rules).
- Integration tests hit a real Postgres (Docker) with migrations applied, per-test transaction rollback.
- `go test ./...` is the gate.
