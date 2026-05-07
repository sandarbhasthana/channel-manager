-- audit schema: append-only audit trail.
--
-- Every domain mutation MUST insert an audit_events row in the same
-- transaction as the write (enforced by the Phase 4 audit hook). Updates
-- and deletes are forbidden by policy; the table is effectively WORM.

CREATE SCHEMA IF NOT EXISTS audit;
SET LOCAL search_path = audit, public;

CREATE TABLE audit.audit_events (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID        NOT NULL,
    actor_id        TEXT,                      -- IdP subject (user) or NULL for system
    actor_type      TEXT        NOT NULL DEFAULT 'user'
                                CHECK (actor_type IN ('user', 'system', 'integration')),
    action          TEXT        NOT NULL,      -- e.g. inventory.set, reservation.cancel
    resource_type   TEXT        NOT NULL,
    resource_id     TEXT        NOT NULL,
    request_id      TEXT,                      -- correlation id from platform/http
    trace_id        TEXT,                      -- otel trace id
    before          JSONB,
    after           JSONB,
    metadata        JSONB       NOT NULL DEFAULT '{}'::jsonb,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX audit_events_org_created_idx
    ON audit.audit_events (org_id, created_at DESC);
CREATE INDEX audit_events_resource_idx
    ON audit.audit_events (org_id, resource_type, resource_id, created_at DESC);
CREATE INDEX audit_events_actor_idx
    ON audit.audit_events (org_id, actor_id, created_at DESC)
    WHERE actor_id IS NOT NULL;

ALTER TABLE audit.audit_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE audit.audit_events FORCE  ROW LEVEL SECURITY;

-- Read policy: only rows belonging to the current org.
CREATE POLICY audit_events_select ON audit.audit_events
    FOR SELECT
    USING (org_id::text = current_setting('app.current_org_id', true));

-- Write policy: insert only, and only into the current org.
CREATE POLICY audit_events_insert ON audit.audit_events
    FOR INSERT
    WITH CHECK (org_id::text = current_setting('app.current_org_id', true));

-- No UPDATE / DELETE policy => those operations are denied for the app role.

GRANT USAGE  ON SCHEMA audit TO app;
GRANT SELECT, INSERT ON ALL TABLES IN SCHEMA audit TO app;
ALTER DEFAULT PRIVILEGES IN SCHEMA audit
    GRANT SELECT, INSERT ON TABLES TO app;
