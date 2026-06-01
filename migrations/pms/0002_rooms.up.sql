CREATE TABLE pms.rooms (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID        NOT NULL,
    property_id     UUID        NOT NULL REFERENCES pms.properties(id) ON DELETE CASCADE,
    room_type_id    UUID        NOT NULL REFERENCES pms.room_types(id) ON DELETE CASCADE,
    external_id     TEXT        NOT NULL,
    name            TEXT        NOT NULL,
    is_active       BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (org_id, property_id, external_id)
);
CREATE INDEX rooms_room_type_idx ON pms.rooms (room_type_id) WHERE is_active;
CREATE TRIGGER rooms_set_updated_at
    BEFORE UPDATE ON pms.rooms
    FOR EACH ROW EXECUTE FUNCTION pms.set_updated_at();

ALTER TABLE pms.rooms ENABLE ROW LEVEL SECURITY;
ALTER TABLE pms.rooms FORCE  ROW LEVEL SECURITY;
CREATE POLICY rooms_tenant_iso ON pms.rooms
    USING       (org_id::text = current_setting('app.current_org_id', true))
    WITH CHECK  (org_id::text = current_setting('app.current_org_id', true));

GRANT SELECT, INSERT, UPDATE, DELETE ON pms.rooms TO app;
