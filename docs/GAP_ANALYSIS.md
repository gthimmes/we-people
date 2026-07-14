# We People — Gap Analysis: from system-of-record to *usable* HR system

This grounds the roadmap in what real HRIS products (Workday, BambooHR, Rippling,
Gusto, Paylocity, HiBob) treat as table-stakes, and honestly assesses what we're
missing. Sources at the bottom.

## Where we are

Solid **system of record** + a **generic approval engine**:
- Multi-tenant orgs, auth, RBAC, audit, full CRUD everywhere
- Workers, org structure, positions, effective-dated assignments, org chart, documents, work eligibility
- Time-off requests → manager approval → balance deduction

## The honest gap

The research is unanimous on one point: an HRIS becomes *usable* not when it can
**store** records but when it **drives work** — it tells people what needs their
attention, lets an admin bring their whole company in, and automates the repetitive
lifecycle tasks. We have none of that operational layer yet.

### Tier 1 — makes it *usable* (blocking day-to-day use) ⭐ building now

| Gap | Why it blocks real use |
|-----|------------------------|
| **Notifications & reminders** | Our approvals are *silent*. A manager is never told they have a request to review; an employee never learns their PTO was approved. Every source lists automated notifications/reminders as core. |
| **User & team management** | You can only add users via a seed script. An admin cannot invite employees, give them logins, or assign roles through the app — so you literally cannot onboard your company. |
| **Home dashboard** | No landing page surfacing "what needs my attention": my approvals, who's out today, headcount, recent hires. |

### Tier 2 — expected of any HR system (next)

| Gap | Notes |
|-----|-------|
| **Onboarding / offboarding checklists** | Universally cited. Task templates per role, assigned owners, due dates, reminders, document collection, e-sign; offboarding = access revocation, asset return, exit interview. Converts a candidate/new-hire into a Worker. |
| **PTO accrual engine** | Balances are static today. Real leave accrues per pay period / hours worked, with **caps**, **carryover** limits, and **use-it-or-lose-it**. Plus holiday calendars and a team absence calendar. |
| **Compensation** | "Employee records with role, location, and **compensation history**" is core HRIS. Pay rate, pay frequency, comp changes over time. |
| **Reporting & analytics** | Headcount, turnover/attrition, diversity, time-to-fill, comp equity; saved/scheduled reports. |
| **Employee self-service depth** | Update own personal info (with approval routing), direct deposit, beneficiaries, profile photo; searchable document/policy library. |
| **Bulk import & guided setup** | Import employees from a spreadsheet; setup wizard. Small teams won't hand-enter 200 people. |

### Tier 3 — full parity (larger modules, later phases)

- **Benefits administration** — plans, eligibility, open enrollment, qualifying life events, dependents/beneficiaries, carrier feeds.
- **Payroll** — earnings/deductions, tax engine, pay runs, direct deposit, payslips, filings, GL export. Most operationally critical *and* highest compliance risk.
- **Recruiting (ATS)** — requisitions, postings, careers page, pipeline, interviews, scorecards, offers.
- **Performance & talent** — goals/OKRs, review cycles, 360 feedback, 1:1s, calibration, succession.
- **Compliance** — I-9/E-Verify, EEO-1, ACA, OSHA, document retention, data-subject requests.
- **Platform** — email/SSO/MFA, public API + webhooks, SCIM, mobile, integrations marketplace.

## Prioritized plan

**This session — Tier 1 (the "operational layer"):**
1. **Notifications** — in-app notification center + unread badge; wired into the approval engine (approver notified on new request, requester on decision) and user invitations. Email delivery stubbed behind an interface for later.
2. **User & team management** — invite/create users, link to workers, assign/relieve roles, enable/disable; roles & permissions viewer.
3. **Home dashboard** — headcount, open positions, my pending approvals, unread notifications, who's out today, recent hires.

**Next — Tier 2:** onboarding checklists (reuses notifications + tasks), then the accrual engine + holidays, then compensation, then reporting.

**Then — Tier 3:** benefits → recruiting → performance → payroll (last; highest risk), with compliance and platform hardening throughout.

## Sources
- [HiBob — HRIS requirements checklist](https://www.hibob.com/hr-tools/hris-requirements-template/)
- [AIHR — HRIS 101](https://www.aihr.com/blog/human-resources-information-system-hris/)
- [Paylocity — Employee Self-Service](https://www.paylocity.com/resources/learn/articles/employee-self-service-ess/)
- [HR Partner — Onboarding checklists](https://www.hrpartner.io/features/employee-onboarding-checklists.html)
- [SHRM — PTO accrual & carryover provisions](https://www.shrm.org/topics-tools/tools/policies/paid-time-pto-accrual-carryover-provisions)
- [Rippling — How PTO accrual works](https://www.rippling.com/blog/pto-accrual)
