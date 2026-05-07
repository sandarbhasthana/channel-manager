-- pms schema: PMS connections, properties, room types.
--
-- This schema owns the canonical property/room-type catalog for the
-- platform. Other schemas reference room_type_id as a logical id (no FK).

CREATE SCHEMA IF NOT EXISTS pms;
SET LOCAL search_path = pms, public;

CREATE OR REPLACE FUNCTION pms.set_updated_at()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    NEW.updated_at := now();
    RETURN NEW;
END;
$$;

-- A PMS connection (Cloudbeds, Mews, Apaleo, etc.). Credentials are NOT
-- stored here \u2014 only an opaque secret_ref pointing at the secrets manager.
CREATE TABLE pms.connections (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID        NOT NULL,
    provider        TEXT        NOT NULL,
    name            TEXT        NOT NULL,
    status          TEXT        NOT NULL DEFAULT 'inactive'
                                CHECK (status IN ('inactive', 'active', 'error', 'disabled')),
    secret_ref      TEXT,
    config          JSONB       NOT NULL DEFAULT '{}'::jsonb,
    last_sync_at    TIMESTAMPTZ,
    last_error      TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (org_id, provider, name)
);
CREATE TRIGGER connections_set_updated_at
    BEFORE UPDATE ON pms.connections
    FOR EACH ROW EXECUTE FUNCTION pms.set_updated_at();

ALTER TABLE pms.connections ENABLE ROW LEVEL SECURITY;
ALTER TABLE pms.connections FORCE  ROW LEVEL SECURITY;
CREATE POLICY connections_tenant_iso ON pms.connections
    USING       (org_id::text = current_setting('app.current_org_id', true))
    WITH CHECK  (org_id::text = current_setting('app.current_org_id', true));

CREATE TABLE pms.properties (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID        NOT NULL,
    connection_id   UUID        REFERENCES pms.connections(id) ON DELETE SET NULL,
    external_id     TEXT,
    name            TEXT        NOT NULL,
    timezone        TEXT        NOT NULL DEFAULT 'UTC',
    currency        TEXT        NOT NULL CHECK (length(currency) = 3),
    address         JSONB       NOT NULL DEFAULT '{}'::jsonb,
    is_active       BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX properties_org_idx ON pms.properties (org_id) WHERE is_active;
CREATE UNIQUE INDEX properties_external_uniq
    ON pms.properties (org_id, connection_id, external_id)
    WHERE external_id IS NOT NULL AND connection_id IS NOT NULL;
CREATE TRIGGER properties_set_updated_at
    BEFORE UPDATE ON pms.properties
    FOR EACH ROW EXECUTE FUNCTION pms.set_updated_at();

ALTER TABLE pms.properties ENABLE ROW LEVEL SECURITY;
ALTER TABLE pms.properties FORCE  ROW LEVEL SECURITY;
CREATE POLICY properties_tenant_iso ON pms.properties
    USING       (org_id::text = current_setting('app.current_org_id', true))
    WITH CHECK  (org_id::text = current_setting('app.current_org_id', true));

CREATE TABLE pms.room_types (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID        NOT NULL,
    property_id     UUID        NOT NULL REFERENCES pms.properties(id) ON DELETE CASCADE,
    external_id     TEXT,
    code            TEXT        NOT NULL,
    name            TEXT        NOT NULL,
    description     TEXT,
    capacity        INT         NOT NULL DEFAULT 2 CHECK (capacity > 0),
    base_occupancy  INT         NOT NULL DEFAULT 2 CHECK (base_occupancy > 0),
    is_active       BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (org_id, property_id, code)
);
CREATE INDEX room_types_property_idx ON pms.room_types (property_id) WHERE is_active;
CREATE TRIGGER room_types_set_updated_at
    BEFORE UPDATE ON pms.room_types
    FOR EACH ROW EXECUTE FUNCTION pms.set_updated_at();

ALTER TABLE pms.room_types ENABLE ROW LEVEL SECURITY;
ALTER TABLE pms.room_types FORCE  ROW LEVEL SECURITY;
CREATE POLICY room_types_tenant_iso ON pms.room_types
    USING       (org_id::text = current_setting('app.current_org_id', true))
    WITH CHECK  (org_id::text = current_setting('app.current_org_id', true));

GRANT USAGE  ON SCHEMA pms TO app;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES    IN SCHEMA pms TO app;
GRANT USAGE, SELECT                  ON ALL SEQUENCES IN SCHEMA pms TO app;
ALTER DEFAULT PRIVILEGES IN SCHEMA pms
    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES    TO app;
ALTER DEFAULT PRIVILEGES IN SCHEMA pms
    GRANT USAGE, SELECT                  ON SEQUENCES TO app;
