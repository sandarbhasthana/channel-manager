-- Minimal DDL for sqlc code generation only.
-- The canonical schema lives in migrations/inventory/0001_init.up.sql.
CREATE SCHEMA IF NOT EXISTS inventory;

CREATE TABLE inventory.inventory_days (
    org_id          UUID        NOT NULL,
    room_type_id    UUID        NOT NULL,
    stay_date       DATE        NOT NULL,
    available       INT         NOT NULL,
    sold            INT         NOT NULL DEFAULT 0,
    blocked         INT         NOT NULL DEFAULT 0,
    stop_sell       BOOLEAN     NOT NULL DEFAULT FALSE,
    min_stay        INT,
    max_stay        INT,
    cta             BOOLEAN     NOT NULL DEFAULT FALSE,
    ctd             BOOLEAN     NOT NULL DEFAULT FALSE,
    version         BIGINT      NOT NULL DEFAULT 1,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (org_id, room_type_id, stay_date)
);
