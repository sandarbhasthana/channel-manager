-- channel schema: OTA / channel connections and sync jobs.

CREATE SCHEMA IF NOT EXISTS channel;
SET LOCAL search_path = channel, public;

CREATE OR REPLACE FUNCTION channel.set_updated_at()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    NEW.updated_at := now();
    RETURN NEW;
END;
$$;

-- A channel connection (Booking.com, Expedia, Airbnb, ...). Credentials
-- live in the secrets manager; only secret_ref is stored here.
CREATE TABLE channel.connections (
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
CREATE INDEX connections_org_status_idx ON channel.connections (org_id, status);
CREATE TRIGGER connections_set_updated_at
    BEFORE UPDATE ON channel.connections
    FOR EACH ROW EXECUTE FUNCTION channel.set_updated_at();

ALTER TABLE channel.connections ENABLE ROW LEVEL SECURITY;
ALTER TABLE channel.connections FORCE  ROW LEVEL SECURITY;
CREATE POLICY connections_tenant_iso ON channel.connections
    USING       (org_id::text = current_setting('app.current_org_id', true))
    WITH CHECK  (org_id::text = current_setting('app.current_org_id', true));

-- Sync jobs: queued / running / completed pushes to a channel.
CREATE TABLE channel.sync_jobs (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID        NOT NULL,
    connection_id   UUID        NOT NULL REFERENCES channel.connections(id) ON DELETE CASCADE,
    job_type        TEXT        NOT NULL
                                CHECK (job_type IN ('inventory_push', 'pricing_push',
                                                     'reservation_pull', 'mapping_sync',
                                                     'full_sync')),
    status          TEXT        NOT NULL DEFAULT 'queued'
                                CHECK (status IN ('queued', 'running', 'succeeded',
                                                   'failed', 'cancelled')),
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

CREATE INDEX sync_jobs_org_status_idx
    ON channel.sync_jobs (org_id, status, scheduled_at);
CREATE INDEX sync_jobs_connection_idx
    ON channel.sync_jobs (connection_id, created_at DESC);

CREATE TRIGGER sync_jobs_set_updated_at
    BEFORE UPDATE ON channel.sync_jobs
    FOR EACH ROW EXECUTE FUNCTION channel.set_updated_at();

ALTER TABLE channel.sync_jobs ENABLE ROW LEVEL SECURITY;
ALTER TABLE channel.sync_jobs FORCE  ROW LEVEL SECURITY;
CREATE POLICY sync_jobs_tenant_iso ON channel.sync_jobs
    USING       (org_id::text = current_setting('app.current_org_id', true))
    WITH CHECK  (org_id::text = current_setting('app.current_org_id', true));

GRANT USAGE  ON SCHEMA channel TO app;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES    IN SCHEMA channel TO app;
GRANT USAGE, SELECT                  ON ALL SEQUENCES IN SCHEMA channel TO app;
ALTER DEFAULT PRIVILEGES IN SCHEMA channel
    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES    TO app;
ALTER DEFAULT PRIVILEGES IN SCHEMA channel
    GRANT USAGE, SELECT                  ON SEQUENCES TO app;
