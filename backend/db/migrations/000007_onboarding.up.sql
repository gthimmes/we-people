-- Onboarding / offboarding checklists: reusable templates instantiated into
-- per-worker plans with assignable, due-dated tasks.

CREATE TABLE checklist_templates (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id      uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name        text NOT NULL,
    type        text NOT NULL DEFAULT 'onboarding', -- onboarding | offboarding
    description text NOT NULL DEFAULT '',
    created_at  timestamptz NOT NULL DEFAULT now(),
    UNIQUE (org_id, name)
);

CREATE TABLE checklist_template_tasks (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    template_id uuid NOT NULL REFERENCES checklist_templates(id) ON DELETE CASCADE,
    title       text NOT NULL,
    description text NOT NULL DEFAULT '',
    assignee    text NOT NULL DEFAULT 'new_hire', -- new_hire | manager | hr
    offset_days int  NOT NULL DEFAULT 0,          -- due = plan start_date + offset_days
    sort_order  int  NOT NULL DEFAULT 0
);
CREATE INDEX idx_template_tasks_template ON checklist_template_tasks(template_id, sort_order);

CREATE TABLE checklist_plans (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id      uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    worker_id   uuid NOT NULL REFERENCES workers(id) ON DELETE CASCADE,
    template_id uuid REFERENCES checklist_templates(id) ON DELETE SET NULL,
    name        text NOT NULL,
    type        text NOT NULL DEFAULT 'onboarding',
    status      text NOT NULL DEFAULT 'active',   -- active | completed | cancelled
    start_date  date NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_checklist_plans_worker ON checklist_plans(org_id, worker_id);

CREATE TABLE checklist_tasks (
    id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id             uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    plan_id            uuid NOT NULL REFERENCES checklist_plans(id) ON DELETE CASCADE,
    title              text NOT NULL,
    description        text NOT NULL DEFAULT '',
    assignee_worker_id uuid REFERENCES workers(id) ON DELETE SET NULL,
    due_date           date,
    status             text NOT NULL DEFAULT 'pending', -- pending | done
    completed_at       timestamptz,
    sort_order         int  NOT NULL DEFAULT 0
);
CREATE INDEX idx_checklist_tasks_plan ON checklist_tasks(plan_id, sort_order);
CREATE INDEX idx_checklist_tasks_assignee ON checklist_tasks(org_id, assignee_worker_id, status);
