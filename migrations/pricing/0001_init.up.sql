-- pricing schema: rate plans and per-day rate points.

CREATE SCHEMA IF NOT EXISTS pricing;
SET LOCAL search_path = pricing, public;

CREATE OR REPLACE FUNCTION pricing.set_updated_at()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    NEW.updated_at := now();
    RETURN NEW;
END;
$$;

-- A rate plan describes the commercial structure (BAR, AAA, NRR, packages,
-- etc.) for a given room type. `room_type_id` is a logical reference to
-- pms.room_types (no cross-schema FK).
CREATE TABLE pricing.rate_plans (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID        NOT NULL,
    room_type_id    UUID        NOT NULL,
    code            TEXT        NOT NULL,
    name            TEXT        NOT NULL,
    currency        TEXT        NOT NULL CHECK (length(currency) = 3),
    is_active       BOOLEAN     NOT NULL DEFAULT TRUE,
    refundable      BOOLEAN     NOT NULL DEFAULT TRUE,
    meal_plan       TEXT,
    description     TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (org_id, code)
);

CREATE INDEX rate_plans_org_room_idx
    ON pricing.rate_plans (org_id, room_type_id)
    WHERE is_active;

CREATE TRIGGER rate_plans_set_updated_at
    BEFORE UPDATE ON pricing.rate_plans
    FOR EACH ROW EXECUTE FUNCTION pricing.set_updated_at();

ALTER TABLE pricing.rate_plans ENABLE ROW LEVEL SECURITY;
ALTER TABLE pricing.rate_plans FORCE  ROW LEVEL SECURITY;
CREATE POLICY rate_plans_tenant_iso ON pricing.rate_plans
    USING       (org_id::text = current_setting('app.current_org_id', true))
    WITH CHECK  (org_id::text = current_setting('app.current_org_id', true));

-- One row per (rate_plan, stay_date). amount_minor is in the currency's
-- minor units (e.g. cents) to avoid floating-point arithmetic.
CREATE TABLE pricing.rate_points (
    org_id          UUID        NOT NULL,
    rate_plan_id    UUID        NOT NULL REFERENCES pricing.rate_plans(id) ON DELETE CASCADE,
    stay_date       DATE        NOT NULL,
    amount_minor    BIGINT      NOT NULL CHECK (amount_minor >= 0),
    currency        TEXT        NOT NULL CHECK (length(currency) = 3),
    closed          BOOLEAN     NOT NULL DEFAULT FALSE,
    version         BIGINT      NOT NULL DEFAULT 1,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (org_id, rate_plan_id, stay_date)
);

CREATE INDEX rate_points_org_date_idx
    ON pricing.rate_points (org_id, stay_date);

CREATE TRIGGER rate_points_set_updated_at
    BEFORE UPDATE ON pricing.rate_points
    FOR EACH ROW EXECUTE FUNCTION pricing.set_updated_at();

ALTER TABLE pricing.rate_points ENABLE ROW LEVEL SECURITY;
ALTER TABLE pricing.rate_points FORCE  ROW LEVEL SECURITY;
CREATE POLICY rate_points_tenant_iso ON pricing.rate_points
    USING       (org_id::text = current_setting('app.current_org_id', true))
    WITH CHECK  (org_id::text = current_setting('app.current_org_id', true));

GRANT USAGE  ON SCHEMA pricing TO app;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES    IN SCHEMA pricing TO app;
GRANT USAGE, SELECT                  ON ALL SEQUENCES IN SCHEMA pricing TO app;
ALTER DEFAULT PRIVILEGES IN SCHEMA pricing
    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES    TO app;
ALTER DEFAULT PRIVILEGES IN SCHEMA pricing
    GRANT USAGE, SELECT                  ON SEQUENCES TO app;
