-- Core HRIS: workers, org structure, positions, assignments, lifecycle events.

CREATE TABLE locations (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id      uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name        text NOT NULL,
    address     text NOT NULL DEFAULT '',
    city        text NOT NULL DEFAULT '',
    region      text NOT NULL DEFAULT '',
    country     text NOT NULL DEFAULT '',
    timezone    text NOT NULL DEFAULT 'UTC',
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    UNIQUE (org_id, name)
);

CREATE TABLE departments (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id       uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name         text NOT NULL,
    code         text NOT NULL DEFAULT '',
    parent_id    uuid REFERENCES departments(id) ON DELETE SET NULL,
    cost_center  text NOT NULL DEFAULT '',
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    UNIQUE (org_id, name)
);
CREATE INDEX idx_departments_parent ON departments(parent_id);

CREATE TABLE workers (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id           uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    employee_number  text NOT NULL,
    first_name       text NOT NULL,
    last_name        text NOT NULL,
    preferred_name   text NOT NULL DEFAULT '',
    work_email       text NOT NULL DEFAULT '',
    personal_email   text NOT NULL DEFAULT '',
    phone            text NOT NULL DEFAULT '',
    date_of_birth    date,
    hire_date        date,
    status           text NOT NULL DEFAULT 'pending', -- pending | active | on_leave | terminated
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    UNIQUE (org_id, employee_number)
);
CREATE INDEX idx_workers_org ON workers(org_id);
CREATE INDEX idx_workers_name ON workers(org_id, last_name, first_name);

-- Now that workers exists, wire the users.worker_id FK.
ALTER TABLE users
    ADD CONSTRAINT users_worker_id_fkey
    FOREIGN KEY (worker_id) REFERENCES workers(id) ON DELETE SET NULL;

CREATE TABLE positions (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id        uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    title         text NOT NULL,
    department_id uuid REFERENCES departments(id) ON DELETE SET NULL,
    location_id   uuid REFERENCES locations(id) ON DELETE SET NULL,
    status        text NOT NULL DEFAULT 'open',  -- open | filled | frozen
    fte           numeric(4,2) NOT NULL DEFAULT 1.00,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_positions_department ON positions(department_id);

CREATE TABLE worker_assignments (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id         uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    worker_id      uuid NOT NULL REFERENCES workers(id) ON DELETE CASCADE,
    position_id    uuid REFERENCES positions(id) ON DELETE SET NULL,
    manager_id     uuid REFERENCES workers(id) ON DELETE SET NULL,
    effective_date date NOT NULL,
    end_date       date,
    is_primary     boolean NOT NULL DEFAULT true,
    created_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_assignments_worker ON worker_assignments(worker_id);
CREATE INDEX idx_assignments_manager ON worker_assignments(manager_id);
-- A worker has at most one primary assignment open at a time.
CREATE UNIQUE INDEX idx_assignments_one_open_primary
    ON worker_assignments(worker_id)
    WHERE end_date IS NULL AND is_primary;

CREATE TABLE lifecycle_events (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id         uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    worker_id      uuid NOT NULL REFERENCES workers(id) ON DELETE CASCADE,
    type           text NOT NULL,  -- hire | transfer | promotion | leave | termination | rehire
    effective_date date NOT NULL,
    reason         text NOT NULL DEFAULT '',
    payload        jsonb,
    created_by     uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_lifecycle_worker ON lifecycle_events(worker_id, effective_date);
