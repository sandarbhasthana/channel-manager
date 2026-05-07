-- inventory schema: per-day room availability.
--
-- One row per (org_id, room_type_id, stay_date). `room_type_id` is a
-- logical reference to pms.room_types \u2014 NOT a foreign key (cross-schema
-- FKs are forbidden by the schema-per-service discipline).

CREATE SCHEMA IF NOT EXISTS inventory;
SET LOCAL search_path = inventory, public;

CREATE OR REPLACE FUNCTION inventory.set_updated_at()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    NEW.updated_at := now();
    RETURN NEW;
END;
$$;

-- inventory_days holds the canonical availability for a given room-type
-- on a given date. `version` is an optimistic concurrency token bumped on
-- every write to detect lost updates from concurrent OTA pushes.
CREATE TABLE inventory.inventory_days (
    org_id          UUID        NOT NULL,
    room_type_id    UUID        NOT NULL,
    stay_date       DATE        NOT NULL,
    available       INT         NOT NULL CHECK (available >= 0),
    sold            INT         NOT NULL DEFAULT 0 CHECK (sold >= 0),
    blocked         INT         NOT NULL DEFAULT 0 CHECK (blocked >= 0),
    stop_sell       BOOLEAN     NOT NULL DEFAULT FALSE,
    min_stay        INT,                                -- nullable: no restriction
    max_stay        INT,
    cta             BOOLEAN     NOT NULL DEFAULT FALSE, -- closed to arrival
    ctd             BOOLEAN     NOT NULL DEFAULT FALSE, -- closed to departure
    version         BIGINT      NOT NULL DEFAULT 1,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (org_id, room_type_id, stay_date)
);

CREATE INDEX inventory_days_org_date_idx
    ON inventory.inventory_days (org_id, stay_date);
CREATE INDEX inventory_days_room_date_idx
    ON inventory.inventory_days (org_id, room_type_id, stay_date);

CREATE TRIGGER inventory_days_set_updated_at
    BEFORE UPDATE ON inventory.inventory_days
    FOR EACH ROW EXECUTE FUNCTION inventory.set_updated_at();

ALTER TABLE inventory.inventory_days ENABLE ROW LEVEL SECURITY;
ALTER TABLE inventory.inventory_days FORCE  ROW LEVEL SECURITY;
CREATE POLICY inventory_days_tenant_iso ON inventory.inventory_days
    USING       (org_id::text = current_setting('app.current_org_id', true))
    WITH CHECK  (org_id::text = current_setting('app.current_org_id', true));

GRANT USAGE  ON SCHEMA inventory TO app;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES    IN SCHEMA inventory TO app;
GRANT USAGE, SELECT                  ON ALL SEQUENCES IN SCHEMA inventory TO app;
ALTER DEFAULT PRIVILEGES IN SCHEMA inventory
    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES    TO app;
ALTER DEFAULT PRIVILEGES IN SCHEMA inventory
    GRANT USAGE, SELECT                  ON SEQUENCES TO app;
