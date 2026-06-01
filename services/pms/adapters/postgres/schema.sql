CREATE SCHEMA IF NOT EXISTS pms;

CREATE TABLE pms.connections (
    id              UUID        PRIMARY KEY,
    org_id          UUID        NOT NULL,
    provider        TEXT        NOT NULL,
    name            TEXT        NOT NULL,
    status          TEXT        NOT NULL DEFAULT 'inactive',
    secret_ref      TEXT,
    config          JSONB       NOT NULL DEFAULT '{}'::jsonb,
    last_sync_at    TIMESTAMPTZ,
    last_error      TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE pms.properties (
    id              UUID        PRIMARY KEY,
    org_id          UUID        NOT NULL,
    connection_id   UUID,
    external_id     TEXT,
    name            TEXT        NOT NULL,
    timezone        TEXT        NOT NULL DEFAULT 'UTC',
    currency        TEXT        NOT NULL,
    address         JSONB       NOT NULL DEFAULT '{}'::jsonb,
    is_active       BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE pms.room_types (
    id              UUID        PRIMARY KEY,
    org_id          UUID        NOT NULL,
    property_id     UUID        NOT NULL,
    external_id     TEXT,
    code            TEXT        NOT NULL,
    name            TEXT        NOT NULL,
    description     TEXT,
    capacity        INT         NOT NULL DEFAULT 2,
    base_occupancy  INT         NOT NULL DEFAULT 2,
    is_active       BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE pms.rooms (
    id              UUID        PRIMARY KEY,
    org_id          UUID        NOT NULL,
    property_id     UUID        NOT NULL,
    room_type_id    UUID        NOT NULL,
    external_id     TEXT        NOT NULL,
    name            TEXT        NOT NULL,
    is_active       BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
