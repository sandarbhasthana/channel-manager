-- mapping schema: PMS \u2194 Channel mappings (room types, rate plans).
--
-- A mapping links a local PMS entity (room_type or rate_plan) to its
-- counterpart in an external channel. AI suggestions land here in the
-- 'suggested' status and require explicit approval before becoming
-- 'active' \u2014 implements the "shadow before auto-apply" guideline.

CREATE SCHEMA IF NOT EXISTS mapping;
SET LOCAL search_path = mapping, public;

CREATE OR REPLACE FUNCTION mapping.set_updated_at()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    NEW.updated_at := now();
    RETURN NEW;
END;
$$;

CREATE TABLE mapping.mappings (
    id                  UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id              UUID        NOT NULL,
    entity_type         TEXT        NOT NULL
                                    CHECK (entity_type IN ('room_type', 'rate_plan')),
    local_entity_id     UUID        NOT NULL,    -- pms.room_types.id or pricing.rate_plans.id
    channel_connection_id UUID      NOT NULL,    -- logical fk to channel.connections.id
    external_entity_id  TEXT        NOT NULL,
    external_metadata   JSONB       NOT NULL DEFAULT '{}'::jsonb,
    status              TEXT        NOT NULL DEFAULT 'suggested'
                                    CHECK (status IN ('suggested', 'active',
                                                      'disabled', 'rejected')),
    confidence          NUMERIC(4,3) CHECK (confidence IS NULL OR
                                            (confidence >= 0 AND confidence <= 1)),
    suggested_by        TEXT,                    -- 'ai', 'rule', or user id
    approved_by         TEXT,
    approved_at         TIMESTAMPTZ,
    notes               TEXT,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (org_id, entity_type, local_entity_id, channel_connection_id, external_entity_id)
);

CREATE INDEX mappings_org_local_idx
    ON mapping.mappings (org_id, entity_type, local_entity_id);
CREATE INDEX mappings_org_channel_idx
    ON mapping.mappings (org_id, channel_connection_id, status);
CREATE INDEX mappings_suggested_idx
    ON mapping.mappings (org_id, status, confidence DESC)
    WHERE status = 'suggested';

CREATE TRIGGER mappings_set_updated_at
    BEFORE UPDATE ON mapping.mappings
    FOR EACH ROW EXECUTE FUNCTION mapping.set_updated_at();

ALTER TABLE mapping.mappings ENABLE ROW LEVEL SECURITY;
ALTER TABLE mapping.mappings FORCE  ROW LEVEL SECURITY;
CREATE POLICY mappings_tenant_iso ON mapping.mappings
    USING       (org_id::text = current_setting('app.current_org_id', true))
    WITH CHECK  (org_id::text = current_setting('app.current_org_id', true));

GRANT USAGE  ON SCHEMA mapping TO app;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES    IN SCHEMA mapping TO app;
GRANT USAGE, SELECT                  ON ALL SEQUENCES IN SCHEMA mapping TO app;
ALTER DEFAULT PRIVILEGES IN SCHEMA mapping
    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES    TO app;
ALTER DEFAULT PRIVILEGES IN SCHEMA mapping
    GRANT USAGE, SELECT                  ON SEQUENCES TO app;
