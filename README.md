# We People

An open, modular **HR management platform** — built to reach feature parity with systems like Workday and Justworks, one well-designed vertical slice at a time.

We People is the **system of record** for an organization's people: who they are, how they're organized, how they're hired, paid, developed, and managed — with the workflows, approvals, compliance, and analytics that a modern HR/People team needs.

## Status

🚧 **Early foundation.** Building the Core HRIS (system of record) first — everything else depends on it. See [`docs/ROADMAP.md`](docs/ROADMAP.md).

## Documentation

| Doc | What's in it |
|-----|--------------|
| [`docs/PRODUCT_SPEC.md`](docs/PRODUCT_SPEC.md) | Full feature-parity map across every HR domain |
| [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) | Tech stack, multi-tenancy, security, service design |
| [`docs/DOMAIN_MODEL.md`](docs/DOMAIN_MODEL.md) | Core entities and their relationships |
| [`docs/ROADMAP.md`](docs/ROADMAP.md) | Prioritized, phased delivery plan |

## Tech stack

- **Backend:** Go (chi router, pgx, sqlc, golang-migrate)
- **Database:** PostgreSQL (multi-tenant, row-level org scoping)
- **Frontend:** React + TypeScript (Vite)
- **Auth:** JWT access/refresh, role-based access control (RBAC)

## Quick start

```bash
# 1. Start Postgres
docker compose up -d db

# 2. Run migrations + seed a demo org
cd backend
go run ./cmd/migrate up
go run ./cmd/seed

# 3. Run the API
go run ./cmd/api        # http://localhost:8080

# 4. Run the web app
cd ../web && npm install && npm run dev   # http://localhost:5173
```

See [`backend/README.md`](backend/README.md) for details.
