DROP TABLE IF EXISTS documents;
DROP TABLE IF EXISTS emergency_contacts;
ALTER TABLE workers
    DROP COLUMN IF EXISTS address_line1,
    DROP COLUMN IF EXISTS address_line2,
    DROP COLUMN IF EXISTS city,
    DROP COLUMN IF EXISTS region,
    DROP COLUMN IF EXISTS postal_code,
    DROP COLUMN IF EXISTS country,
    DROP COLUMN IF EXISTS gender,
    DROP COLUMN IF EXISTS ethnicity,
    DROP COLUMN IF EXISTS marital_status;
