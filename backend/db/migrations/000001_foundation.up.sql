-- Foundation: tenancy, identity, RBAC, refresh tokens, audit.

CREATE TABLE organizations (
    id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name               text NOT NULL,
    slug               text NOT NULL UNIQUE,
    fiscal_year_start  smallint NOT NULL DEFAULT 1,      -- month 1-12
    default_currency   text NOT NULL DEFAULT 'USD',
    status             text NOT NULL DEFAULT 'active',   -- active | suspended
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE users (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id         uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    email          text NOT NULL,
    password_hash  text NOT NULL,
    status         text NOT NULL DEFAULT 'active',       -- active | disabled
    worker_id      uuid,                                 -- FK added in core HRIS migration
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now(),
    UNIQUE (org_id, email)
);

CREATE TABLE roles (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id       uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name         text NOT NULL,
    description  text NOT NULL DEFAULT '',
    is_system    boolean NOT NULL DEFAULT false,
    created_at   timestamptz NOT NULL DEFAULT now(),
    UNIQUE (org_id, name)
);

-- Permissions are a fixed global catalog (not per-tenant).
CREATE TABLE permissions (
    key          text PRIMARY KEY,
    description  text NOT NULL DEFAULT ''
);

CREATE TABLE role_permissions (
    role_id         uuid NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    permission_key  text NOT NULL REFERENCES permissions(key) ON DELETE CASCADE,
    PRIMARY KEY (role_id, permission_key)
);

CREATE TABLE user_roles (
    user_id  uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id  uuid NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, role_id)
);

CREATE TABLE refresh_tokens (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash  text NOT NULL UNIQUE,
    expires_at  timestamptz NOT NULL,
    revoked_at  timestamptz,
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_refresh_tokens_user ON refresh_tokens(user_id);

CREATE TABLE audit_events (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id         uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    actor_user_id  uuid REFERENCES users(id) ON DELETE SET NULL,
    action         text NOT NULL,          -- e.g. worker.create
    entity_type    text NOT NULL,          -- e.g. worker
    entity_id      uuid,
    before         jsonb,
    after          jsonb,
    created_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_audit_events_org ON audit_events(org_id, created_at DESC);
CREATE INDEX idx_audit_events_entity ON audit_events(entity_type, entity_id);

-- Seed the global permission catalog.
INSERT INTO permissions (key, description) VALUES
    ('org:read',        'View organization settings'),
    ('org:write',       'Manage organization settings'),
    ('user:read',       'View users'),
    ('user:write',      'Manage users and roles'),
    ('worker:read',     'View worker records'),
    ('worker:write',    'Create and edit worker records'),
    ('orgstructure:read',  'View departments, locations, positions'),
    ('orgstructure:write', 'Manage departments, locations, positions'),
    ('audit:read',      'View the audit log');
