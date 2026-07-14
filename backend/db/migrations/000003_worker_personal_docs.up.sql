-- Extended worker personal data, emergency contacts, and documents.

ALTER TABLE workers
    ADD COLUMN address_line1  text NOT NULL DEFAULT '',
    ADD COLUMN address_line2  text NOT NULL DEFAULT '',
    ADD COLUMN city           text NOT NULL DEFAULT '',
    ADD COLUMN region         text NOT NULL DEFAULT '',
    ADD COLUMN postal_code    text NOT NULL DEFAULT '',
    ADD COLUMN country        text NOT NULL DEFAULT '',
    ADD COLUMN gender         text NOT NULL DEFAULT '',
    ADD COLUMN ethnicity      text NOT NULL DEFAULT '',
    ADD COLUMN marital_status text NOT NULL DEFAULT '';

CREATE TABLE emergency_contacts (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id        uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    worker_id     uuid NOT NULL REFERENCES workers(id) ON DELETE CASCADE,
    name          text NOT NULL,
    relationship  text NOT NULL DEFAULT '',
    phone         text NOT NULL DEFAULT '',
    email         text NOT NULL DEFAULT '',
    is_primary    boolean NOT NULL DEFAULT false,
    created_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_emergency_contacts_worker ON emergency_contacts(worker_id);

CREATE TABLE documents (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id        uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    worker_id     uuid REFERENCES workers(id) ON DELETE CASCADE,
    name          text NOT NULL,
    content_type  text NOT NULL DEFAULT 'application/octet-stream',
    size_bytes    bigint NOT NULL DEFAULT 0,
    content       bytea NOT NULL,
    uploaded_by   uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_documents_worker ON documents(worker_id);
CREATE INDEX idx_documents_org ON documents(org_id, created_at DESC);
