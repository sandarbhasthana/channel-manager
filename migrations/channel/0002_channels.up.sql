-- Add channels table: property-level OTA listings attached to an org-level Connection.

CREATE TABLE channel.channels (
    id                   UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id               UUID        NOT NULL,
    property_id          UUID        NOT NULL,
    connection_id        UUID        NOT NULL REFERENCES channel.connections(id) ON DELETE CASCADE,
    provider             TEXT        NOT NULL,
    external_property_id TEXT        NOT NULL,
    status               TEXT        NOT NULL DEFAULT 'inactive'
                                     CHECK (status IN ('inactive', 'active', 'paused', 'error', 'disconnected')),
    last_sync_at         TIMESTAMPTZ,
    last_error           TEXT,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (org_id, property_id, connection_id)
);

CREATE INDEX channels_org_property_idx ON channel.channels (org_id, property_id);
CREATE INDEX channels_connection_idx   ON channel.channels (connection_id);

CREATE TRIGGER channels_set_updated_at
    BEFORE UPDATE ON channel.channels
    FOR EACH ROW EXECUTE FUNCTION channel.set_updated_at();

ALTER TABLE channel.channels ENABLE ROW LEVEL SECURITY;
ALTER TABLE channel.channels FORCE  ROW LEVEL SECURITY;
CREATE POLICY channels_tenant_iso ON channel.channels
    USING       (org_id::text = current_setting('app.current_org_id', true))
    WITH CHECK  (org_id::text = current_setting('app.current_org_id', true));
