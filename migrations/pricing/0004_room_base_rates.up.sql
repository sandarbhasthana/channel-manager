-- Channel Manager-stored base rates.
--
-- The PMS owns the authoritative rate card, but when the live PMS quote is
-- unavailable the owner can set a base rate here in CM. The per-channel
-- adjustment (channel_rate_rules) is applied on top of this base. One row per
-- (property, room_type). amount_minor is in the currency's minor units.

SET LOCAL search_path = pricing, public;

CREATE TABLE pricing.room_base_rates (
    org_id       UUID        NOT NULL,
    property_id  UUID        NOT NULL,
    room_type_id UUID        NOT NULL,
    amount_minor BIGINT      NOT NULL DEFAULT 0 CHECK (amount_minor >= 0),
    currency     TEXT        NOT NULL DEFAULT 'USD' CHECK (length(currency) = 3),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (org_id, property_id, room_type_id)
);

CREATE TRIGGER room_base_rates_set_updated_at
    BEFORE UPDATE ON pricing.room_base_rates
    FOR EACH ROW EXECUTE FUNCTION pricing.set_updated_at();

ALTER TABLE pricing.room_base_rates ENABLE ROW LEVEL SECURITY;
ALTER TABLE pricing.room_base_rates FORCE  ROW LEVEL SECURITY;
CREATE POLICY room_base_rates_tenant_iso ON pricing.room_base_rates
    USING       (org_id::text = current_setting('app.current_org_id', true))
    WITH CHECK  (org_id::text = current_setting('app.current_org_id', true));

GRANT SELECT, INSERT, UPDATE, DELETE ON pricing.room_base_rates TO app;
