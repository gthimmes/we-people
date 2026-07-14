ALTER TABLE users DROP CONSTRAINT IF EXISTS users_worker_id_fkey;
DROP TABLE IF EXISTS lifecycle_events;
DROP TABLE IF EXISTS worker_assignments;
DROP TABLE IF EXISTS positions;
DROP TABLE IF EXISTS workers;
DROP TABLE IF EXISTS departments;
DROP TABLE IF EXISTS locations;
