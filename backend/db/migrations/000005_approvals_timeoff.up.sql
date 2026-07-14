-- Phase 2: generic approval engine + time off.

-- Generic approval requests, reusable by any module (time off, offers, comp, ...).
CREATE TABLE approval_requests (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id              uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    request_type        text NOT NULL,          -- e.g. time_off
    subject_type        text NOT NULL,          -- e.g. time_off_request
    subject_id          uuid NOT NULL,          -- the row being approved
    requester_worker_id uuid REFERENCES workers(id) ON DELETE SET NULL,
    status              text NOT NULL DEFAULT 'pending', -- pending|approved|rejected|cancelled
    current_step        int  NOT NULL DEFAULT 1,
    created_at          timestamptz NOT NULL DEFAULT now(),
    decided_at          timestamptz
);
CREATE INDEX idx_approval_requests_org ON approval_requests(org_id, status);
CREATE INDEX idx_approval_requests_subject ON approval_requests(subject_type, subject_id);

CREATE TABLE approval_steps (
    id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    request_id         uuid NOT NULL REFERENCES approval_requests(id) ON DELETE CASCADE,
    sequence           int  NOT NULL,
    approver_worker_id uuid REFERENCES workers(id) ON DELETE SET NULL,
    status             text NOT NULL DEFAULT 'pending', -- pending|approved|rejected
    note               text NOT NULL DEFAULT '',
    decided_at         timestamptz
);
CREATE INDEX idx_approval_steps_request ON approval_steps(request_id, sequence);
CREATE INDEX idx_approval_steps_approver ON approval_steps(approver_worker_id, status);

-- Time off.
CREATE TABLE leave_types (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id     uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name       text NOT NULL,
    is_paid    boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (org_id, name)
);

CREATE TABLE leave_balances (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id        uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    worker_id     uuid NOT NULL REFERENCES workers(id) ON DELETE CASCADE,
    leave_type_id uuid NOT NULL REFERENCES leave_types(id) ON DELETE CASCADE,
    balance_hours numeric(8,2) NOT NULL DEFAULT 0,
    UNIQUE (org_id, worker_id, leave_type_id)
);

CREATE TABLE time_off_requests (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id              uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    worker_id           uuid NOT NULL REFERENCES workers(id) ON DELETE CASCADE,
    leave_type_id       uuid NOT NULL REFERENCES leave_types(id),
    start_date          date NOT NULL,
    end_date            date NOT NULL,
    hours               numeric(6,2) NOT NULL,
    reason              text NOT NULL DEFAULT '',
    status              text NOT NULL DEFAULT 'pending', -- pending|approved|rejected|cancelled
    approval_request_id uuid REFERENCES approval_requests(id) ON DELETE SET NULL,
    created_at          timestamptz NOT NULL DEFAULT now(),
    decided_at          timestamptz
);
CREATE INDEX idx_time_off_worker ON time_off_requests(org_id, worker_id, created_at DESC);
