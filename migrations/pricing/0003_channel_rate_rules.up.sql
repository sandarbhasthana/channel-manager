-- Per-channel pricing rules.
--
-- Channel Manager owns the per-channel adjustment applied on top of the PMS
-- base rate (just as it owns promo codes). The final rate a channel receives is
--   PMS_base * (1 + adjust_pct / 100)
-- One row per (property, room_type, channel). channel_id is a logical reference
-- to channel.connections (no cross-schema FK, matching house style).

SET LOCAL search_path = pricing, public;

CREATE TABLE pricing.channel_rate_rules (
    org_id       UUID             NOT NULL,
    property_id  UUID             NOT NULL,
    room_type_id UUID             NOT NULL,
    channel_id   UUID             NOT NULL,
    adjust_pct   DOUBLE PRECISION NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ      NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ      NOT NULL DEFAULT now(),
    PRIMARY KEY (org_id, property_id, room_type_id, channel_id)
);

CREATE INDEX channel_rate_rules_property_idx
    ON pricing.channel_rate_rules (org_id, property_id);

CREATE TRIGGER channel_rate_rules_set_updated_at
    BEFORE UPDATE ON pricing.channel_rate_rules
    FOR EACH ROW EXECUTE FUNCTION pricing.set_updated_at();

ALTER TABLE pricing.channel_rate_rules ENABLE ROW LEVEL SECURITY;
ALTER TABLE pricing.channel_rate_rules FORCE  ROW LEVEL SECURITY;
CREATE POLICY channel_rate_rules_tenant_iso ON pricing.channel_rate_rules
    USING       (org_id::text = current_setting('app.current_org_id', true))
    WITH CHECK  (org_id::text = current_setting('app.current_org_id', true));

GRANT SELECT, INSERT, UPDATE, DELETE ON pricing.channel_rate_rules TO app;
