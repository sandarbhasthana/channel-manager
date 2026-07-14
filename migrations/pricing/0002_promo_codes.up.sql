-- pricing schema: promotional discount codes.
--
-- Channel Manager owns promo definitions and, critically, the redemption
-- counter. A booking engine may read a code and evaluate its stateless rules
-- locally, but only Channel Manager increments `uses` — otherwise `max_uses`
-- is unenforceable across replicas and channels.

SET LOCAL search_path = pricing, public;

CREATE TABLE pricing.promo_codes (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID        NOT NULL,

    -- NULL means the code is valid across every property in the org.
    -- Non-NULL restricts it to one property. Logical reference to
    -- pms.properties (no cross-schema FK), consistent with rate_plans.
    property_id     UUID,

    code            TEXT        NOT NULL,
    description     TEXT,

    discount_pct    NUMERIC(5,2) NOT NULL
                                 CHECK (discount_pct > 0 AND discount_pct <= 100),

    -- NULL max_uses means unlimited. `uses` may never exceed it; the partial
    -- CHECK is a backstop for the conditional UPDATE that performs redemption.
    max_uses        INTEGER     CHECK (max_uses IS NULL OR max_uses > 0),
    uses            INTEGER     NOT NULL DEFAULT 0 CHECK (uses >= 0),

    valid_from      TIMESTAMPTZ,
    valid_until     TIMESTAMPTZ,
    is_active       BOOLEAN     NOT NULL DEFAULT TRUE,

    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- One code per org. A code must resolve unambiguously from (org, code)
    -- alone, so an org-wide and a property-scoped code cannot share a string.
    UNIQUE (org_id, code),

    CONSTRAINT promo_codes_uses_within_max
        CHECK (max_uses IS NULL OR uses <= max_uses),

    CONSTRAINT promo_codes_valid_window
        CHECK (valid_from IS NULL OR valid_until IS NULL OR valid_until > valid_from)
);

CREATE INDEX promo_codes_org_active_idx
    ON pricing.promo_codes (org_id, is_active);

CREATE INDEX promo_codes_property_idx
    ON pricing.promo_codes (org_id, property_id)
    WHERE property_id IS NOT NULL;

CREATE TRIGGER promo_codes_set_updated_at
    BEFORE UPDATE ON pricing.promo_codes
    FOR EACH ROW EXECUTE FUNCTION pricing.set_updated_at();

ALTER TABLE pricing.promo_codes ENABLE ROW LEVEL SECURITY;
ALTER TABLE pricing.promo_codes FORCE  ROW LEVEL SECURITY;
CREATE POLICY promo_codes_tenant_iso ON pricing.promo_codes
    USING       (org_id::text = current_setting('app.current_org_id', true))
    WITH CHECK  (org_id::text = current_setting('app.current_org_id', true));
