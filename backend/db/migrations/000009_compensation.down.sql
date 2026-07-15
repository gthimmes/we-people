DROP TABLE IF EXISTS compensation_records;
DELETE FROM permissions WHERE key IN ('compensation:read', 'compensation:write');
