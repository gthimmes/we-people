ALTER TABLE workers
    DROP COLUMN IF EXISTS work_auth_type,
    DROP COLUMN IF EXISTS work_auth_expiry,
    DROP COLUMN IF EXISTS i9_verified,
    DROP COLUMN IF EXISTS i9_verified_on;

ALTER TABLE positions
    DROP COLUMN IF EXISTS job_profile_id,
    DROP COLUMN IF EXISTS legal_entity_id;

DROP TABLE IF EXISTS job_profiles;
DROP TABLE IF EXISTS legal_entities;
