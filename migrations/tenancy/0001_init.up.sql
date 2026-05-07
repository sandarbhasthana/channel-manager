-- tenancy schema: shared organizations / users / memberships.
--
-- This schema is the cross-cutting tenant directory. Other service schemas
-- never reference it via foreign keys (per the schema-per-service discipline)
-- and instead carry org_id as a plain UUID with RLS policies enforcing
-- tenant isolation.

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE SCHEMA IF NOT EXISTS tenancy;
SET LOCAL search_path = tenancy, public;

-- Helper: bump updated_at on every UPDATE.
CREATE OR REPLACE FUNCTION tenancy.set_updated_at()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    NEW.updated_at := now();
    RETURN NEW;
END;
$$;

-- Application role used by all runtime database connections.
-- Superuser/migration role (postgres) creates and migrates schemas; the
-- application binds as `app` so RLS policies are actually enforced.
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'app') THEN
        CREATE ROLE app NOLOGIN;
    END IF;
END$$;

-- ── organizations ────────────────────────────────────────────────────────────
-- The tenant root. Not RLS-scoped: a user may need to list the orgs they
-- belong to before any org context exists.
CREATE TABLE tenancy.organizations (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT        NOT NULL,
    slug        TEXT        NOT NULL UNIQUE,
    status      TEXT        NOT NULL DEFAULT 'active'
                            CHECK (status IN ('active', 'suspended', 'deleted')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TRIGGER organizations_set_updated_at
    BEFORE UPDATE ON tenancy.organizations
    FOR EACH ROW EXECUTE FUNCTION tenancy.set_updated_at();

-- ── users ────────────────────────────────────────────────────────────────────
-- Mirror of identity from the IdP (Clerk/WorkOS). `id` matches the IdP
-- subject — we never store credentials.
CREATE TABLE tenancy.users (
    id              TEXT        PRIMARY KEY,
    email           TEXT        NOT NULL,
    full_name       TEXT,
    default_org_id  UUID        REFERENCES tenancy.organizations(id) ON DELETE SET NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX users_email_idx ON tenancy.users (lower(email));
CREATE TRIGGER users_set_updated_at
    BEFORE UPDATE ON tenancy.users
    FOR EACH ROW EXECUTE FUNCTION tenancy.set_updated_at();

-- ── memberships ──────────────────────────────────────────────────────────────
-- (org_id, user_id) tuple with role. RLS-scoped: a member can only see
-- memberships within their current org.
CREATE TABLE tenancy.memberships (
    org_id      UUID        NOT NULL REFERENCES tenancy.organizations(id) ON DELETE CASCADE,
    user_id     TEXT        NOT NULL REFERENCES tenancy.users(id) ON DELETE CASCADE,
    role        TEXT        NOT NULL DEFAULT 'member'
                            CHECK (role IN ('owner', 'admin', 'member', 'viewer')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (org_id, user_id)
);
CREATE INDEX memberships_user_idx ON tenancy.memberships (user_id);
CREATE TRIGGER memberships_set_updated_at
    BEFORE UPDATE ON tenancy.memberships
    FOR EACH ROW EXECUTE FUNCTION tenancy.set_updated_at();

ALTER TABLE tenancy.memberships ENABLE ROW LEVEL SECURITY;
ALTER TABLE tenancy.memberships FORCE  ROW LEVEL SECURITY;
CREATE POLICY memberships_tenant_iso ON tenancy.memberships
    USING       (org_id::text = current_setting('app.current_org_id', true))
    WITH CHECK  (org_id::text = current_setting('app.current_org_id', true));

-- ── grants for the application role ─────────────────────────────────────────
GRANT USAGE  ON SCHEMA tenancy TO app;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES    IN SCHEMA tenancy TO app;
GRANT USAGE, SELECT                  ON ALL SEQUENCES IN SCHEMA tenancy TO app;
ALTER DEFAULT PRIVILEGES IN SCHEMA tenancy
    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES    TO app;
ALTER DEFAULT PRIVILEGES IN SCHEMA tenancy
    GRANT USAGE, SELECT                  ON SEQUENCES TO app;
