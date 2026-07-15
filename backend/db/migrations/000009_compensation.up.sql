-- Compensation: effective-dated pay records per worker.

INSERT INTO permissions (key, description) VALUES
    ('compensation:read',  'View worker compensation'),
    ('compensation:write', 'Manage worker compensation')
ON CONFLICT (key) DO NOTHING;

-- Grant the new permissions to existing Org Admin roles (fresh orgs get all
-- permissions at registration; this covers orgs created before this migration).
INSERT INTO role_permissions (role_id, permission_key)
SELECT r.id, k.key
FROM roles r
CROSS JOIN (VALUES ('compensation:read'), ('compensation:write')) AS k(key)
WHERE r.name = 'Org Admin'
ON CONFLICT DO NOTHING;

CREATE TABLE compensation_records (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id         uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    worker_id      uuid NOT NULL REFERENCES workers(id) ON DELETE CASCADE,
    effective_date date NOT NULL,
    pay_type       text NOT NULL DEFAULT 'salary',  -- salary | hourly
    amount         numeric(14,2) NOT NULL,
    currency       text NOT NULL DEFAULT 'USD',
    pay_frequency  text NOT NULL DEFAULT 'annual',  -- annual | monthly | biweekly | weekly | hourly
    reason         text NOT NULL DEFAULT '',
    created_by     uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_compensation_worker ON compensation_records(org_id, worker_id, effective_date DESC);
