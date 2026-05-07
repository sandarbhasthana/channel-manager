-- reservations schema: bookings, guests, and per-night reservation items.
--
-- Reservations originate from a channel (or are made directly), reference
-- a property and one-or-more room types, and have one primary guest plus
-- any number of additional guests. Cross-schema references (channel,
-- property, room_type) are logical only.

CREATE SCHEMA IF NOT EXISTS reservations;
SET LOCAL search_path = reservations, public;

CREATE OR REPLACE FUNCTION reservations.set_updated_at()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    NEW.updated_at := now();
    RETURN NEW;
END;
$$;

CREATE TABLE reservations.guests (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID        NOT NULL,
    first_name      TEXT,
    last_name       TEXT,
    email           TEXT,
    phone           TEXT,
    country         TEXT,
    metadata        JSONB       NOT NULL DEFAULT '{}'::jsonb,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX guests_org_email_idx
    ON reservations.guests (org_id, lower(email))
    WHERE email IS NOT NULL;
CREATE TRIGGER guests_set_updated_at
    BEFORE UPDATE ON reservations.guests
    FOR EACH ROW EXECUTE FUNCTION reservations.set_updated_at();

ALTER TABLE reservations.guests ENABLE ROW LEVEL SECURITY;
ALTER TABLE reservations.guests FORCE  ROW LEVEL SECURITY;
CREATE POLICY guests_tenant_iso ON reservations.guests
    USING       (org_id::text = current_setting('app.current_org_id', true))
    WITH CHECK  (org_id::text = current_setting('app.current_org_id', true));

CREATE TABLE reservations.reservations (
    id                      UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id                  UUID        NOT NULL,
    channel_connection_id   UUID,                   -- logical fk to channel.connections
    property_id             UUID        NOT NULL,   -- logical fk to pms.properties
    external_id             TEXT,                   -- channel reservation id
    confirmation_code       TEXT,
    primary_guest_id        UUID        REFERENCES reservations.guests(id) ON DELETE SET NULL,
    status                  TEXT        NOT NULL DEFAULT 'pending'
                                        CHECK (status IN ('pending', 'confirmed',
                                                           'modified', 'cancelled',
                                                           'no_show', 'checked_in',
                                                           'checked_out')),
    check_in                DATE        NOT NULL,
    check_out               DATE        NOT NULL CHECK (check_out > check_in),
    adults                  INT         NOT NULL DEFAULT 1 CHECK (adults > 0),
    children                INT         NOT NULL DEFAULT 0 CHECK (children >= 0),
    currency                TEXT        NOT NULL CHECK (length(currency) = 3),
    total_amount_minor      BIGINT      NOT NULL DEFAULT 0,
    notes                   TEXT,
    metadata                JSONB       NOT NULL DEFAULT '{}'::jsonb,
    booked_at               TIMESTAMPTZ NOT NULL DEFAULT now(),
    cancelled_at            TIMESTAMPTZ,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX reservations_org_dates_idx
    ON reservations.reservations (org_id, check_in, check_out);
CREATE INDEX reservations_org_status_idx
    ON reservations.reservations (org_id, status);
CREATE UNIQUE INDEX reservations_external_uniq
    ON reservations.reservations (org_id, channel_connection_id, external_id)
    WHERE external_id IS NOT NULL AND channel_connection_id IS NOT NULL;
CREATE TRIGGER reservations_set_updated_at
    BEFORE UPDATE ON reservations.reservations
    FOR EACH ROW EXECUTE FUNCTION reservations.set_updated_at();

ALTER TABLE reservations.reservations ENABLE ROW LEVEL SECURITY;
ALTER TABLE reservations.reservations FORCE  ROW LEVEL SECURITY;
CREATE POLICY reservations_tenant_iso ON reservations.reservations
    USING       (org_id::text = current_setting('app.current_org_id', true))
    WITH CHECK  (org_id::text = current_setting('app.current_org_id', true));

-- One row per (reservation, stay_date, room_type). A multi-night stay
-- yields N rows; a multi-room stay yields N\u00d7M rows. This denormalisation
-- makes per-night reporting and inventory reconciliation trivial.
CREATE TABLE reservations.reservation_items (
    id                  UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id              UUID        NOT NULL,
    reservation_id      UUID        NOT NULL REFERENCES reservations.reservations(id) ON DELETE CASCADE,
    room_type_id        UUID        NOT NULL,           -- logical fk to pms.room_types
    rate_plan_id        UUID,                            -- logical fk to pricing.rate_plans
    stay_date           DATE        NOT NULL,
    amount_minor        BIGINT      NOT NULL DEFAULT 0,
    currency            TEXT        NOT NULL CHECK (length(currency) = 3),
    metadata            JSONB       NOT NULL DEFAULT '{}'::jsonb,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX reservation_items_reservation_idx
    ON reservations.reservation_items (reservation_id);
CREATE INDEX reservation_items_org_date_idx
    ON reservations.reservation_items (org_id, stay_date);
CREATE TRIGGER reservation_items_set_updated_at
    BEFORE UPDATE ON reservations.reservation_items
    FOR EACH ROW EXECUTE FUNCTION reservations.set_updated_at();

ALTER TABLE reservations.reservation_items ENABLE ROW LEVEL SECURITY;
ALTER TABLE reservations.reservation_items FORCE  ROW LEVEL SECURITY;
CREATE POLICY reservation_items_tenant_iso ON reservations.reservation_items
    USING       (org_id::text = current_setting('app.current_org_id', true))
    WITH CHECK  (org_id::text = current_setting('app.current_org_id', true));

GRANT USAGE  ON SCHEMA reservations TO app;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES    IN SCHEMA reservations TO app;
GRANT USAGE, SELECT                  ON ALL SEQUENCES IN SCHEMA reservations TO app;
ALTER DEFAULT PRIVILEGES IN SCHEMA reservations
    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES    TO app;
ALTER DEFAULT PRIVILEGES IN SCHEMA reservations
    GRANT USAGE, SELECT                  ON SEQUENCES TO app;
