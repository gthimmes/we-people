# We People — Product Specification

This is the complete feature scope required to reach parity with major HR platforms
(Workday, Justworks, Bamboo, Rippling, Gusto, ADP). It is intentionally exhaustive —
the [roadmap](ROADMAP.md) sequences it into deliverable phases. Nothing here is built
until a phase claims it.

Each module lists: **purpose**, **core capabilities**, and **key entities**.

---

## 0. Platform & Foundation

The substrate every module sits on.

- **Multi-tenancy** — one deployment serves many organizations; every row is scoped to an `org_id`. Hard isolation, no cross-tenant leakage.
- **Identity & Access** — users, authentication (email/password, SSO/SAML/OIDC, MFA), session/refresh tokens.
- **RBAC + ABAC** — roles (Employee, Manager, HR Admin, Recruiter, Payroll Admin, Org Admin, Super Admin) and attribute rules (e.g. "manager can see only their reports").
- **Audit log** — immutable record of every state change: who, what, when, before/after.
- **Workflow engine** — configurable multi-step approvals (requests → approvers → actions) reused by PTO, comp changes, offers, terminations, etc.
- **Notifications** — in-app, email, and eventually push/Slack; templated, per-event.
- **Documents** — file storage with access control, e-signature integration, retention policies.
- **Configuration** — per-org settings: fiscal year, locales, currencies, business rules, custom fields.
- **Integrations & API** — public REST API, webhooks, SCIM provisioning, background jobs, import/export.

**Key entities:** `Organization`, `User`, `Role`, `Permission`, `AuditEvent`, `WorkflowDefinition`, `WorkflowInstance`, `Notification`, `Document`, `CustomField`.

---

## 1. Core HRIS — System of Record  ⭐ *first build*

The single source of truth for people and how the organization is structured.
Every other module reads from here.

- **Worker record** — personal info, contact, demographics (EEO), emergency contacts, IDs, work eligibility.
- **Employment lifecycle** — hire, rehire, promotion, transfer, leave, terminate — each an auditable *event* with an effective date.
- **Position management** — positions/jobs distinct from the people who fill them; open/filled/frozen; FTE.
- **Organizational structure** — legal entities, business units, departments, cost centers, locations, teams.
- **Reporting hierarchy / org chart** — manager relationships, dotted lines, spans & layers.
- **Employee self-service (ESS)** — view/update own profile, see org chart, request changes (routed for approval).
- **Manager self-service (MSS)** — team roster, initiate actions on reports.
- **Effective-dated data** — every attribute is a timeline; you can ask "what was true on date X."

**Key entities:** `Worker`, `EmploymentRecord`, `Position`, `JobProfile`, `Department`, `Location`, `LegalEntity`, `CostCenter`, `WorkerAssignment`, `LifecycleEvent`.

---

## 2. Time & Absence

- **PTO / leave policies** — accrual rules (per pay period, tenure tiers, caps, carryover), multiple plan types (vacation, sick, personal).
- **Balances & accruals** — automated accrual runs, negative-balance rules, payout on termination.
- **Time-off requests** — request → manager approval → calendar; team absence calendar.
- **Holidays** — per-location holiday calendars.
- **Timesheets** — hourly time entry, clock in/out, project/task allocation, overtime rules.
- **Leaves of absence** — FMLA, parental, medical, sabbatical; job protection tracking.

**Key entities:** `LeavePolicy`, `LeaveBalance`, `AccrualTransaction`, `TimeOffRequest`, `HolidayCalendar`, `Timesheet`, `TimeEntry`, `LeaveOfAbsence`.

---

## 3. Recruiting (ATS) & Onboarding

- **Requisitions** — headcount requests, approval, budget, hiring team.
- **Job postings** — careers site, job boards, referrals, internal mobility.
- **Candidate pipeline** — sourcing → screen → interview → offer → hire; configurable stages.
- **Applications & résumés** — parsing, tagging, duplicate detection.
- **Interview management** — scheduling, scorecards, structured feedback, panels.
- **Offers** — offer letters, approvals, e-signature, accept/decline.
- **Onboarding** — pre-boarding checklists, provisioning tasks, new-hire paperwork (I-9, W-4), day-1 flows; converts a candidate into a Worker.

**Key entities:** `Requisition`, `JobPosting`, `Candidate`, `Application`, `Interview`, `Scorecard`, `Offer`, `OnboardingPlan`, `OnboardingTask`.

---

## 4. Performance & Talent Management

- **Goals / OKRs** — individual, team, cascading; check-ins and progress.
- **Review cycles** — annual/quarterly; self, manager, upward, peer/360; templated questions & rating scales.
- **Continuous feedback** — praise, requested feedback, 1:1 agendas & notes.
- **Calibration** — cross-manager rating normalization, 9-box talent grid.
- **Succession planning** — key roles, successors, readiness, flight risk.
- **Development** — competencies, skills inventory, learning/training, career paths.

**Key entities:** `Goal`, `ReviewCycle`, `ReviewTemplate`, `ReviewAssignment`, `Feedback`, `OneOnOne`, `CalibrationSession`, `SuccessionPlan`, `Skill`, `Competency`.

---

## 5. Compensation & Benefits

- **Compensation structure** — pay grades, salary bands, geographic differentials, currencies.
- **Comp management** — base pay, bonuses, equity, allowances; total rewards statement.
- **Merit / comp cycles** — budget pools, manager recommendations, approvals, letters.
- **Benefits administration** — plans (medical, dental, vision, 401k, HSA/FSA, life), carriers, deductions.
- **Open enrollment** — annual windows, life events (QLE), dependents, beneficiaries.
- **Carrier connections** — EDI feeds to insurance carriers.

**Key entities:** `PayGrade`, `SalaryBand`, `CompensationRecord`, `CompCycle`, `BenefitPlan`, `Enrollment`, `Dependent`, `Deduction`.

---

## 6. Payroll

- **Worker pay setup** — pay rate, pay frequency, FLSA status, direct deposit, tax withholding (W-4).
- **Earnings & deductions** — regular, OT, bonus, commission; pre/post-tax deductions, garnishments.
- **Tax engine** — federal/state/local income tax, FICA, unemployment; reciprocity, multi-state.
- **Pay runs** — off-cycle & scheduled; gross-to-net calculation, preview, approve, commit.
- **Payments** — ACH direct deposit, checks, pay cards.
- **Payslips** — itemized, self-service access, YTD.
- **Tax filings & compliance** — W-2, 1099, 941/940, state filings, new-hire reporting.
- **GL integration** — journal entries to accounting systems.

**Key entities:** `PayrollProfile`, `EarningCode`, `DeductionCode`, `TaxProfile`, `PayRun`, `Paycheck`, `PayslipLine`, `GLEntry`.

---

## 7. Compliance & Analytics

- **Compliance** — EEO-1, ACA, OSHA, I-9/E-Verify, minimum wage, labor law by jurisdiction.
- **Reporting** — standard + custom report builder, scheduled delivery.
- **People analytics** — headcount, turnover/attrition, diversity, comp equity, time-to-hire, span/layers.
- **Dashboards** — role-based (exec, HR, manager).
- **Data export / warehouse** — feeds to BI tools.

**Key entities:** `Report`, `Dashboard`, `Metric`, `ComplianceFiling`.

---

## Cross-cutting requirements

- **Security & privacy** — encryption at rest/in transit, PII minimization, GDPR/CCPA (data subject requests, right to delete), SOC 2 controls.
- **Localization** — multi-currency, multi-language, locale-aware dates/addresses, country-specific rules.
- **Accessibility** — WCAG 2.1 AA.
- **Mobile** — responsive web first; native later.
- **Extensibility** — custom fields & objects, per-org workflows, marketplace/API.
