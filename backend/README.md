# We People — Backend (Go)

Modular-monolith HTTP API. See [`../docs/ARCHITECTURE.md`](../docs/ARCHITECTURE.md).

## Prerequisites

- Go 1.26+
- Docker (for Postgres) — `docker compose up -d db` from the repo root

## Run

```bash
# from repo root: start Postgres
docker compose up -d db

# from backend/
go run ./cmd/migrate up   # apply migrations
go run ./cmd/seed         # seed demo org (admin@acme.test / password123)
go run ./cmd/api          # serve on :8080
```

## Test

```bash
go test ./...
```

Unit tests (auth, slug) run anywhere. The integration test in `internal/app`
needs Postgres reachable at `DATABASE_URL` (it self-skips if not).

## Configuration (env)

| Var | Default | Notes |
|-----|---------|-------|
| `DATABASE_URL` | `postgres://wepeople:wepeople@localhost:5432/wepeople?sslmode=disable` | |
| `HTTP_ADDR` | `:8080` | |
| `JWT_SECRET` | dev default | **must** be set when `ENV=production` |
| `ENV` | `development` | |

## API surface (current slice)

Public:
- `POST /api/v1/auth/register` — create org + admin, returns tokens
- `POST /api/v1/auth/login` — `{slug, email, password}`
- `POST /api/v1/auth/refresh` — rotate refresh token
- `POST /api/v1/auth/logout`

Authenticated (`Authorization: Bearer <access>`):
- `GET /api/v1/me`
- `GET|POST /api/v1/workers`, `GET|PUT /api/v1/workers/{id}` — perms `worker:read` / `worker:write`
- `GET|POST /api/v1/departments`, `/locations`, `/positions` — perms `orgstructure:read` / `orgstructure:write`
- `POST /api/v1/assignments` — assign worker → position + manager
- `GET /api/v1/org-chart` — reporting hierarchy tree

## Layout

```
cmd/{api,migrate,seed}   entrypoints
internal/
  config database httpx auth audit   infrastructure
  org iam worker orgstructure         domain modules (store → service → handler)
db/migrations                         versioned SQL (embedded)
```
