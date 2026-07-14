# We People — Roadmap

Prioritized so that each phase is **shippable** and **unblocks** the next. Sequencing
rule: build the system of record before the systems that read from it; build the
workflow/approval and notification primitives early because half the product reuses them.

Timeframes assume a small focused team; they're relative, not commitments.

---

### Phase 0 — Foundation *(weeks 0–2)* — **in progress**
The substrate. Nothing user-facing ships without it.
- [x] Repo, docs, architecture decisions
- [x] Go backend scaffold, Postgres via Docker, migration runner
- [ ] Multi-tenant org model + tenant scoping
- [ ] Auth: register org, login, JWT access/refresh
- [ ] RBAC: roles, permissions, route guards
- [ ] Audit log primitive
- [ ] Frontend scaffold + auth flow

### Phase 1 — Core HRIS / System of Record *(weeks 2–8)* — **first module** ⭐
The spine every other module references.
- [x] Worker records (personal, contact, demographics, emergency contacts)
- [x] Org structure: departments, locations, cost centers
- [x] Positions (job profiles pending)
- [x] Employment lifecycle events (hire, transfer, promote, terminate) — effective-dated
- [x] Reporting hierarchy + org chart
- [x] Documents (upload, download, access control)
- [x] Worker & manager self-service views (profile detail, org chart)
- [x] Legal entities, job profiles (positions reference both)
- [x] Effective-dated "as of date" assignment queries + assignment history
- [x] Work eligibility / I-9 data on the worker record
- [ ] Deferred to later phases: department-tree UI, custom fields, GDPR/CCPA data-subject tooling

> **Delivered:** register/login, full worker CRUD + profile detail, address & demographics, emergency contacts, documents (upload/download/delete), departments/locations/positions management, effective-dated assignments with transfer/promote/terminate lifecycle events, and the org chart — all end-to-end with tests.

### Phase 2 — Time & Absence *(weeks 8–14)* — **in progress**
First to exercise the **workflow/approval engine** for real.
- [x] **Generic approval engine** — requests routed through ordered approver
  steps, with a finalizer registry so any module can plug in its effect
  (reused later by offers, comp changes, terminations)
- [x] Time-off requests → manager approval → balance deduction (atomic)
- [x] Leave types + balances; employee self-service + manager approval inbox
- [ ] Accrual engine (scheduled accruals, carryover, caps)
- [ ] Holiday calendars, team absence calendar
- [ ] Timesheets (hourly), overtime rules, leaves of absence (FMLA)

### Phase 3 — Recruiting (ATS) & Onboarding *(weeks 12–20)*
Feeds new Workers into the HRIS.
- Requisitions + approval
- Careers page + job postings
- Candidate pipeline, applications, interviews, scorecards
- Offers + e-signature
- Onboarding checklists → convert candidate to Worker

### Phase 4 — Performance & Talent *(weeks 18–26)*
- Goals / OKRs + check-ins
- Review cycles (self / manager / peer / 360)
- Continuous feedback, 1:1s
- Calibration + 9-box, succession, skills

### Phase 5 — Compensation & Benefits *(weeks 24–32)*
- Pay grades / salary bands
- Comp records + merit cycles
- Benefit plans, deductions
- Open enrollment + life events, dependents/beneficiaries

### Phase 6 — Payroll *(weeks 30–44)*
Most complex + compliance-heavy — built last on a solid HRIS + comp/benefits.
- Pay setup, earnings/deductions
- Tax engine (fed/state/local, FICA), multi-state
- Pay runs: gross-to-net, preview, approve, commit
- Direct deposit (ACH), payslips
- Tax filings (W-2, 941/940), GL export

### Phase 7 — Compliance, Analytics & Platform *(ongoing)*
- EEO-1, ACA, I-9/E-Verify, OSHA
- Report builder, people analytics dashboards
- Public API, webhooks, SCIM
- Mobile, localization depth, SOC 2 hardening

---

## Why this order

| Decision | Rationale |
|----------|-----------|
| HRIS before everything | Recruiting, performance, comp, and payroll all reference the Worker record. Build the noun before the verbs. |
| Workflow engine in Phase 0/2 | PTO, offers, comp changes, and terminations are all "request → approve → apply." Build it once. |
| Time & Absence before Payroll | Payroll consumes hours and leave; also a lower-risk way to harden approvals/accruals. |
| Recruiting mid-stream | High business value and it's the top-of-funnel that creates Workers, but it depends on org/position data. |
| Payroll last | Highest correctness/compliance risk; benefits from a mature, well-tested platform beneath it. |
