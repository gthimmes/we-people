-- Phase 1 tails: legal entities, job profiles, work eligibility.

CREATE TABLE legal_entities (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id      uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name        text NOT NULL,
    country     text NOT NULL DEFAULT '',
    tax_id      text NOT NULL DEFAULT '',
    created_at  timestamptz NOT NULL DEFAULT now(),
    UNIQUE (org_id, name)
);

CREATE TABLE job_profiles (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id       uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    title        text NOT NULL,
    job_family   text NOT NULL DEFAULT '',
    level        text NOT NULL DEFAULT '',
    flsa_status  text NOT NULL DEFAULT 'exempt',  -- exempt | non_exempt
    created_at   timestamptz NOT NULL DEFAULT now(),
    UNIQUE (org_id, title, level)
);

ALTER TABLE positions
    ADD COLUMN job_profile_id  uuid REFERENCES job_profiles(id) ON DELETE SET NULL,
    ADD COLUMN legal_entity_id uuid REFERENCES legal_entities(id) ON DELETE SET NULL;

-- Work eligibility / I-9 on the worker record.
ALTER TABLE workers
    ADD COLUMN work_auth_type   text NOT NULL DEFAULT '',  -- citizen | permanent_resident | visa | ...
    ADD COLUMN work_auth_expiry date,
    ADD COLUMN i9_verified      boolean NOT NULL DEFAULT false,
    ADD COLUMN i9_verified_on   date;
