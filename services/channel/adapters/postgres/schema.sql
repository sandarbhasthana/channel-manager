-- Schema used by sqlc for type generation. Must mirror the real migrations
-- (migrations/channel/0001_init.up.sql + 0002_channels.up.sql).
-- Table names are unqualified because sqlc doesn't support schema-qualified
-- table references when using the default search_path.

CREATE TABLE IF NOT EXISTS connections (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID        NOT NULL,
    provider        TEXT        NOT NULL,
    name            TEXT        NOT NULL,
    status          TEXT        NOT NULL DEFAULT 'inactive',
    secret_ref      TEXT,
    config          JSONB       NOT NULL DEFAULT '{}'::jsonb,
    last_sync_at    TIMESTAMPTZ,
    last_error      TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (org_id, provider, name)
);

CREATE TABLE IF NOT EXISTS channels (
    id                   UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id               UUID        NOT NULL,
    property_id          UUID        NOT NULL,
    connection_id        UUID        NOT NULL REFERENCES connections(id) ON DELETE CASCADE,
    provider             TEXT        NOT NULL,
    external_property_id TEXT        NOT NULL,
    status               TEXT        NOT NULL DEFAULT 'inactive',
    last_sync_at         TIMESTAMPTZ,
    last_error           TEXT,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (org_id, property_id, connection_id)
);

CREATE TABLE IF NOT EXISTS sync_jobs (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID        NOT NULL,
    connection_id   UUID        NOT NULL REFERENCES connections(id) ON DELETE CASCADE,
    job_type        TEXT        NOT NULL,
    status          TEXT        NOT NULL DEFAULT 'queued',
    payload         JSONB       NOT NULL DEFAULT '{}'::jsonb,
    result          JSONB,
    attempts        INT         NOT NULL DEFAULT 0,
    last_error      TEXT,
    scheduled_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    started_at      TIMESTAMPTZ,
    finished_at     TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
