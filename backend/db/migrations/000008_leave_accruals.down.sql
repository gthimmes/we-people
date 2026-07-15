DROP TABLE IF EXISTS leave_accruals;
ALTER TABLE leave_types
    DROP COLUMN IF EXISTS accrual_enabled,
    DROP COLUMN IF EXISTS accrual_annual_hours,
    DROP COLUMN IF EXISTS max_balance_hours,
    DROP COLUMN IF EXISTS carryover_max_hours;
