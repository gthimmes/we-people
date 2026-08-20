import type { HelpContent } from "help-navigator";

// The in-app help corpus: categories + markdown articles, rendered by the
// help-navigator widget mounted in App.tsx. Every article describes features
// that exist in the app today — keep it that way when the product grows.
export const helpContent: HelpContent = {
  categories: [
    {
      id: "getting-started",
      title: "Getting started",
      icon: "🚀",
      description: "First steps, signing in, and how access works.",
    },
    {
      id: "people",
      title: "People & profiles",
      icon: "👥",
      description: "The directory, worker profiles, and employment records.",
    },
    {
      id: "organization",
      title: "Organization",
      icon: "🏢",
      description: "Departments, locations, positions, and the org chart.",
    },
    {
      id: "time-off",
      title: "Time off",
      icon: "🌴",
      description: "Requesting leave, approvals, balances, and accrual.",
    },
    {
      id: "onboarding",
      title: "Onboarding & offboarding",
      icon: "✅",
      description: "Checklists and templates for new hires and departures.",
    },
    {
      id: "insights-admin",
      title: "Insights & administration",
      icon: "📊",
      description: "Analytics, and managing logins and roles.",
    },
  ],
  articles: [
    // ---------- Getting started ----------
    {
      id: "welcome-tour",
      title: "A quick tour of We People",
      category: "getting-started",
      featured: true,
      tags: ["overview", "tour", "navigation", "dashboard", "basics"],
      body: `We People is your organization's **system of record** for people: who they are, how they're organized, and the day-to-day workflows around them.

## The sidebar

- **Home** — your dashboard: headcount, open positions, departments, and anything *awaiting you*
- **People** — the searchable employee directory
- **Organization** — departments, locations, legal entities, job profiles, and positions
- **Org chart** — the reporting hierarchy
- **Time off** — request leave and approve your team's requests
- **Onboarding** — checklists for new hires and departures
- **Analytics** — people metrics across the organization
- **Team & access** — logins and roles (visible to admins only)

## The dashboard

The stat cards are clickable shortcuts. **Awaiting you** lights up when requests need your approval, **My tasks** lists your open checklist tasks, and **Out today** and **Recent hires** keep you current at a glance.

The bell in the top bar collects notifications — approval requests, decisions, and task assignments.

> Press **F1** anytime to open this help panel.`,
      related: ["accounts-and-orgs", "directory-basics", "roles-permissions"],
    },
    {
      id: "accounts-and-orgs",
      title: "Signing in and organizations",
      category: "getting-started",
      tags: ["login", "sign in", "register", "organization", "slug", "password"],
      body: `Every We People account belongs to exactly one **organization**, identified by a short slug (like \`acme-corp\`).

## Signing in

Enter your **organization slug**, **email**, and **password**. You stay signed in until you choose **Sign out** in the sidebar footer.

## Creating a new organization

Choose **Create a new organization** on the sign-in screen and provide an organization name, your email, and a password. This registers a brand-new organization with you as its first admin — the slug is derived from the name. It does *not* add you to an existing organization; for that, ask an admin to invite you from **Team & access**.

## Logins and employee records

A login may be **linked to an employee record** or stand alone. Linked users see their own balances and can request time off; standalone logins (for example, an external bookkeeper) can still act on anything their role permits, such as approving requests.`,
      related: ["team-access", "roles-permissions"],
    },
    {
      id: "roles-permissions",
      title: "Roles and what you can see",
      category: "getting-started",
      featured: true,
      tags: ["roles", "permissions", "access", "admin", "employee", "read-only"],
      body: `What you can do in We People is governed by the **permissions** on your roles — buttons and pages you're not permitted to use are simply hidden.

## The built-in roles

- **Org Admin** — full administrative access; created for the person who registers the organization
- **Employee** — standard self-service: browse people and the org structure read-only, manage their own time off and tasks

## How gating shows up in the UI

- **+ Add person**, profile editing, documents, and onboarding actions need *worker write* access
- Organization panels (departments, locations, positions…) and **Change assignment** need *org-structure write* access
- The **Compensation** card and compensation analytics only appear with *compensation read* access
- **Team & access** only appears with *user read* access; the **Leave types (admin)** panel needs *org write* access

If something described in these articles isn't visible to you, you most likely don't have that permission — ask your admin.`,
      related: ["team-access", "welcome-tour"],
    },

    // ---------- People & profiles ----------
    {
      id: "directory-basics",
      title: "Finding and adding people",
      category: "people",
      featured: true,
      tags: ["directory", "people", "search", "add person", "employee number", "status"],
      body: `The **People** page lists everyone in your organization.

## Searching

Type into the search box and press **Enter** (or click **Search**) to match by **name, email, or employee #**.

## Reading the table

Columns show employee #, name, work email, hire date, and a **status** badge — *active*, *pending*, *on leave*, or *terminated*. Click any row to open that person's full profile.

## Adding someone

With worker-write access, **+ Add person** opens an inline form. **Employee #**, **first name**, and **last name** are required; work email and hire date are optional and can be filled in later from the profile.

> Adding a person creates the worker record only. To give them a login, invite them from **Team & access** and link the login to their record. To run their first weeks, start an onboarding checklist.`,
      related: ["worker-profiles", "team-access", "onboarding-checklists"],
    },
    {
      id: "worker-profiles",
      title: "The worker profile",
      category: "people",
      featured: true,
      tags: ["profile", "edit", "emergency contacts", "documents", "i-9", "address", "personal"],
      body: `Click a person in the directory to open their profile — the single place for everything about them.

## Profile card

Contact details, hire date, date of birth, address, demographics, work authorization, and **I-9 verification** status. With worker-write access, **Edit** turns the card into a form covering all of these fields, including the I-9 verified checkbox and date.

## Emergency contacts

**+ Add** records a contact's name (required), relationship, phone, and email. Each contact can be edited or removed later.

## Documents

Upload files with the file picker + **Upload**; each document shows its size and upload date, downloads on click, and can be deleted. Access to documents follows worker permissions.

## Also on the profile

- **Current role** — position, department, location, and manager
- **Checklists** — any onboarding/offboarding plans in progress, with a task-level progress bar
- **Employment history** — a timeline of every lifecycle event
- **Compensation** — pay history, for those permitted to see it`,
      related: ["assignment-changes", "compensation-records", "directory-basics"],
    },
    {
      id: "assignment-changes",
      title: "Transfers, promotions, and terminations",
      category: "people",
      tags: ["transfer", "promote", "terminate", "assignment", "manager", "lifecycle", "history"],
      body: `Employment changes in We People are **effective-dated events**, so the record always reflects who held what role, when.

## Changing an assignment

On a profile's **Current role** card, **Change assignment** (org-structure write access) lets you pick a **change type** — *Transfer* or *Promotion* — plus the new **position**, the new **manager**, and an optional reason. Saving records the event and updates the org chart.

## Terminating

**Terminate** in the profile header asks for an **effective date** and a reason, then confirms. The person's status changes to *terminated*; editing and further assignment changes are closed off, though the record and its history remain.

## Employment history

Every event — *Hired, Transferred, Promoted, Went on leave, Terminated, Rehired* — appears on the profile's **Employment history** timeline with its effective date and reason.

> **Delete** on a profile permanently removes the record and is meant for mistakes; for a real departure, terminate instead so history is preserved.`,
      related: ["worker-profiles", "org-chart", "org-structure"],
    },
    {
      id: "compensation-records",
      title: "Compensation records",
      category: "people",
      tags: ["compensation", "pay", "salary", "merit", "history", "frequency"],
      body: `Pay lives on the worker profile as **effective-dated compensation records** — a full history, not just a current number.

## Who can see it

The **Compensation** card appears only with compensation-read access, and **+ Add change** requires compensation-write access. Compensation figures in Analytics are gated the same way.

## Recording a change

A change needs an **effective date** and an **amount**, plus a pay **frequency** — annual, monthly, biweekly, weekly, or hourly — and an optional reason such as *merit increase*. The card shows current pay (amount, frequency, and since-when) with the history listed beneath.

Workers are notified when their compensation changes.`,
      related: ["worker-profiles", "analytics-overview", "roles-permissions"],
    },

    // ---------- Organization ----------
    {
      id: "org-structure",
      title: "Departments, locations, and positions",
      category: "organization",
      featured: true,
      tags: ["departments", "locations", "legal entities", "job profiles", "positions", "cost center"],
      body: `The **Organization** page holds the structure that worker assignments reference, in five panels. Editing requires org-structure write access; everyone else sees read-only lists.

## The building blocks

- **Departments** — name, code, and cost center
- **Locations** — name, city, region, country, and timezone
- **Legal entities** — the employing entities, with country and tax ID
- **Job profiles** — a title plus job family, level, and FLSA classification (*exempt* / *non-exempt*)

## Positions

Positions tie it together: each has a title and optionally a department, location, job profile, and legal entity, plus a **status** — *open*, *filled*, or *frozen*. The dashboard's **Open positions** count comes from here.

Every panel works the same way: **+ Add** opens an inline form, each row has **Edit** and **Delete**, and deletes always ask you to confirm.`,
      related: ["org-chart", "assignment-changes", "directory-basics"],
    },
    {
      id: "org-chart",
      title: "Reading the org chart",
      category: "organization",
      tags: ["org chart", "hierarchy", "reporting", "manager", "tree"],
      body: `The **Org chart** draws the reporting hierarchy from current assignments — it is always live, never hand-maintained.

## How it's built

Each card shows a person's initials, name, and title. Everyone appears under their current **manager**; anyone without a manager becomes a **root** of their own tree, so multiple top-level people (or disconnected groups) are perfectly normal.

## Changing the chart

The chart itself is read-only. To move someone, open their profile and use **Change assignment** to set a new manager or position — the chart updates immediately.

If the page says *"No assignments yet"*, no one has a current assignment; add people and give them positions and managers first.`,
      related: ["assignment-changes", "org-structure"],
    },

    // ---------- Time off ----------
    {
      id: "requesting-time-off",
      title: "Requesting time off",
      category: "time-off",
      featured: true,
      tags: ["time off", "leave", "request", "balance", "pto", "vacation", "cancel"],
      body: `The **Time off** page is where you check balances, submit requests, and track their status.

## Your balances

**My balances** lists each leave type with your remaining hours.

## Submitting a request

Pick a **leave type**, a **start date**, an optional **end date** (defaults to the start date for a single day), the number of **hours** (defaults to 8), and an optional reason. Your manager is notified, and the request appears in **My requests** with a *pending* badge.

## After you submit

- Approval or rejection notifies you and updates the badge
- Approved requests deduct the hours from your balance
- A pending request can be withdrawn with **Cancel**

> If the page says your login isn't linked to an employee record, you have no personal balances — you can still approve your team's requests. An admin can link your login from **Team & access**.`,
      related: ["approving-time-off", "leave-types-accruals", "accounts-and-orgs"],
    },
    {
      id: "approving-time-off",
      title: "Approving your team's requests",
      category: "time-off",
      tags: ["approve", "reject", "approval", "manager", "awaiting", "inbox"],
      body: `Requests from your reports route to you for a decision.

## Where they show up

- The dashboard's **Awaiting you** card highlights and counts your pending approvals
- The **Time off** page shows an **Awaiting your approval** card with each request's requester, leave type, hours, dates, and reason
- The notification bell announces new requests as they arrive

## Deciding

**Approve** deducts the hours from the requester's balance and marks the request approved, in one atomic step. **Reject** declines it without touching the balance. Either way, the requester is notified of your decision.

Requests you've decided leave the card; the requester keeps the full record in their own **My requests** list.`,
      related: ["requesting-time-off", "leave-types-accruals"],
    },
    {
      id: "leave-types-accruals",
      title: "Leave types, accrual, and carryover",
      category: "time-off",
      tags: ["leave types", "accrual", "carryover", "cap", "admin", "balance", "use it or lose it"],
      body: `Admins with org-write access see the **Leave types (admin)** panel at the bottom of the Time off page.

## Defining a leave type

Each type has a name and, optionally, **Accrues over time**. Accruing types set:

- **Hours per year** — accrued in monthly slices
- **Max balance** — a cap the balance never exceeds (0 = uncapped)
- **Carryover** — either *unlimited year-end carryover* or a **carryover cap** in hours; a cap of 0 means use-it-or-lose-it

## Running accrual and carryover

Two buttons run the engine on demand:

- **Run this month's accrual** grants each worker their monthly slice, respecting caps — it reports how many hours were accrued across how many workers
- **Run year-end carryover** applies each type's carryover rule to close out the year

Both runs are **idempotent** — running the same period twice never double-grants — and every grant is recorded in a ledger.`,
      related: ["requesting-time-off", "approving-time-off", "roles-permissions"],
    },

    // ---------- Onboarding & offboarding ----------
    {
      id: "onboarding-checklists",
      title: "Running onboarding and offboarding checklists",
      category: "onboarding",
      featured: true,
      tags: ["onboarding", "offboarding", "checklist", "tasks", "new hire", "progress"],
      body: `The **Onboarding** page tracks structured checklists for both new hires and departures.

## Starting a checklist

**Start a checklist** (worker-write access) asks for the **employee**, a **template**, and an optional start date. The plan is created with every task's assignee and due date resolved automatically from the template.

## Tracking progress

**Active checklists** shows each plan with a progress bar (*done/total*); click a row to expand its tasks. Each task shows its assignee — *New hire*, *Manager*, or *HR* — and due date. Assignees are notified when tasks are assigned to them.

## Completing tasks

Assignees tick their own tasks here, on the dashboard's **My tasks** card, or on the worker's profile, where active plans also appear. Admins can tick any task. A plan completes automatically when its last task is done.`,
      related: ["checklist-templates", "directory-basics", "worker-profiles"],
    },
    {
      id: "checklist-templates",
      title: "Building checklist templates",
      category: "onboarding",
      tags: ["template", "checklist", "tasks", "assignee", "due date", "offboarding"],
      body: `Templates make checklists repeatable — define the plan once, start it for each person.

## Creating a template

**+ New template** (worker-write access) opens the builder:

1. Name the template and choose its **type** — *onboarding* or *offboarding*
2. Add task rows: a title, an **assignee** (*New hire*, *Manager*, or *HR*), and a due-date offset in **days after start**
3. **+ Add task** appends rows; ✕ removes one. Rows left blank are dropped on save

## How the offsets work

When a checklist starts, each task's due date is computed from the plan's start date plus the task's offset, and the assignee roles resolve to real people — the worker themself, their current manager, or HR.

Templates can be deleted from the **Templates** card; plans already started from them keep running.`,
      related: ["onboarding-checklists", "worker-profiles"],
    },

    // ---------- Insights & administration ----------
    {
      id: "analytics-overview",
      title: "People analytics",
      category: "insights-admin",
      tags: ["analytics", "headcount", "turnover", "diversity", "leave liability", "trends", "reports"],
      body: `The **Analytics** page gives a read-only picture of the workforce, computed live from your records.

## The headline tiles

- **Active headcount** — everyone currently active
- **Turnover (12mo)** — terminations over the trailing twelve months as a percentage
- **Pending time off** — requests still awaiting a decision
- **Leave liability** — total accrued leave hours on the books

## Breakdowns

Bar charts split headcount by **department** and **location**, and show workforce composition by **gender** and **employment status**. **Hires vs terminations** lists the last twelve months of arrivals and departures side by side.

## Compensation analytics

With compensation-read access, a **Compensation** section adds the organization-wide average pay and average pay by department. Without that permission the section simply doesn't appear.`,
      related: ["compensation-records", "roles-permissions"],
    },
    {
      id: "team-access",
      title: "Managing logins and roles",
      category: "insights-admin",
      featured: true,
      tags: ["users", "invite", "roles", "disable", "enable", "link employee", "access"],
      body: `**Team & access** is where admins manage who can sign in and what they're allowed to do. Seeing the page requires user-read access; making changes requires user-write access.

## Inviting a user

**+ Invite user** creates a login from an **email** and an **initial password** (share it securely — email invites are coming). Optionally **link the login to an employee record**, which connects their sign-in to their profile, balances, and tasks — or leave it as a standalone login. Pick their initial **roles** from the checkbox list.

## Ongoing management

- **Roles** on any row opens an inline editor to change that user's roles
- **Disable** blocks a user from signing in; **Enable** restores them — you can't disable your own account

## Roles

The **Roles** card lists each role with its description and permission count. The built-in roles are **Org Admin** and **Employee**; roles are read-only in the UI today.`,
      related: ["roles-permissions", "accounts-and-orgs", "directory-basics"],
    },
  ],
};
