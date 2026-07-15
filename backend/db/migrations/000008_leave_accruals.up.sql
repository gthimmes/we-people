-- Accrual configuration on leave types + an idempotent accrual ledger.

ALTER TABLE leave_types
    ADD COLUMN accrual_enabled      boolean       NOT NULL DEFAULT false,
    ADD COLUMN accrual_annual_hours numeric(8,2)  NOT NULL DEFAULT 0,   -- hours earned per year
    ADD COLUMN max_balance_hours    numeric(8,2)  NOT NULL DEFAULT 0,   -- 0 = uncapped
    ADD COLUMN carryover_max_hours  numeric(8,2);                        -- NULL = unlimited carryover

-- Ledger of accrual/carryover events. The unique key makes runs idempotent:
-- running the same period twice credits nothing extra.
CREATE TABLE leave_accruals (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id        uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    worker_id     uuid NOT NULL REFERENCES workers(id) ON DELETE CASCADE,
    leave_type_id uuid NOT NULL REFERENCES leave_types(id) ON DELETE CASCADE,
    period        text NOT NULL,                 -- e.g. 2026-07 (accrual) or 2026-CO (carryover)
    kind          text NOT NULL DEFAULT 'accrual', -- accrual | carryover
    hours         numeric(8,2) NOT NULL,
    created_at    timestamptz NOT NULL DEFAULT now(),
    UNIQUE (org_id, worker_id, leave_type_id, period, kind)
);
CREATE INDEX idx_leave_accruals_worker ON leave_accruals(org_id, worker_id, leave_type_id);
