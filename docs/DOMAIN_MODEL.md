# We People — Domain Model

Core entities for Phase 0 (Foundation) and Phase 1 (Core HRIS). Later modules extend
this. `⭑` = implemented in the current slice.

## Foundation

```
Organization ⭑           the tenant. Everything is scoped to one.
  id, name, slug, subdomain, fiscal_year_start, default_currency, status, created_at

User ⭑                   a login identity within an org.
  id, org_id, email, password_hash, status, worker_id?, created_at
  (email unique per org)

Role ⭑                   named bundle of permissions (per org, plus system defaults).
  id, org_id, name, description, is_system

Permission ⭑            fine-grained capability, e.g. "worker:write".
  key, description

RolePermission ⭑         role ↔ permission
UserRole ⭑               user ↔ role

RefreshToken ⭑           rotating refresh tokens, hashed + revocable.
  id, user_id, token_hash, expires_at, revoked_at

AuditEvent ⭑             immutable change log.
  id, org_id, actor_user_id, action, entity_type, entity_id, before, after, created_at
```

## Core HRIS

```
Worker ⭑                 a person the org employs (or did / will).
  id, org_id, employee_number, first_name, last_name, preferred_name,
  work_email, personal_email, phone, date_of_birth, hire_date, status
  (active | on_leave | terminated | pending)

  Extended (Phase 1): home address, demographics (EEO), emergency contacts,
  work eligibility, national IDs — stored in related tables.

Location ⭑               a physical/legal work site.
  id, org_id, name, address, city, region, country, timezone

Department ⭑             an org unit; self-referential tree.
  id, org_id, name, code, parent_id?, cost_center

JobProfile               reusable job definition (title, family, level, FLSA).
  id, org_id, title, job_family, level, flsa_status

Position ⭑               a specific seat (may be filled or open); references a JobProfile.
  id, org_id, title, department_id, location_id, status (open|filled|frozen), fte

WorkerAssignment ⭑       links a Worker to a Position + manager, effective-dated.
  id, org_id, worker_id, position_id, manager_id?, effective_date, end_date?, is_primary
  → the reporting hierarchy comes from manager_id here.

LifecycleEvent ⭑         an auditable employment event.
  id, org_id, worker_id, type (hire|transfer|promotion|leave|termination|rehire),
  effective_date, reason, payload(jsonb), created_by, created_at
```

## Relationships

- `Organization 1—* User`, `Worker`, `Department`, `Location`, `Position` (tenancy).
- `User *—1 Worker` (a login may be tied to a worker; some users are system/admin only).
- `Worker 1—* WorkerAssignment` (history); the current assignment gives current position + manager.
- `Department` is a tree via `parent_id`; `WorkerAssignment.manager_id` gives the people-reporting tree (org chart).
- `Position *—1 Department`, `*—1 Location`.
- Every mutating action → an `AuditEvent`; every employment change → a `LifecycleEvent`.

## Effective dating

`WorkerAssignment` carries `effective_date` / `end_date`. The **current** assignment is
the one whose `[effective_date, end_date)` range contains today (open-ended `end_date`
means still active). Transfers/promotions close the old row and open a new one — so the
full history is preserved and any past state is reconstructable.

## Design notes

- **Positions ≠ Workers.** A position can be open (unfilled) or backfilled. This is what
  lets recruiting and headcount planning reference a seat that has no person yet.
- **Manager via assignment, not a `Worker.manager_id` column.** Keeps the org chart
  effective-dated and lets a reorg be a clean set of new assignments.
- **Custom fields** (later) attach as JSONB columns + a `custom_field_definitions` table
  per org, so tenants extend entities without schema changes.
