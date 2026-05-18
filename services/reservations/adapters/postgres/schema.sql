CREATE SCHEMA IF NOT EXISTS reservations;

CREATE TABLE reservations.guests (
    id              UUID PRIMARY KEY,
    org_id          UUID NOT NULL,
    first_name      TEXT,
    last_name       TEXT,
    email           TEXT,
    phone           TEXT,
    country         TEXT,
    metadata        JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE reservations.reservations (
    id                      UUID PRIMARY KEY,
    org_id                  UUID NOT NULL,
    channel_connection_id   UUID,
    property_id             UUID NOT NULL,
    external_id             TEXT,
    confirmation_code       TEXT,
    primary_guest_id        UUID,
    status                  TEXT NOT NULL DEFAULT 'pending',
    check_in                DATE NOT NULL,
    check_out               DATE NOT NULL,
    adults                  INT NOT NULL DEFAULT 1,
    children                INT NOT NULL DEFAULT 0,
    currency                TEXT NOT NULL,
    total_amount_minor      BIGINT NOT NULL DEFAULT 0,
    notes                   TEXT,
    metadata                JSONB NOT NULL DEFAULT '{}'::jsonb,
    booked_at               TIMESTAMPTZ NOT NULL DEFAULT now(),
    cancelled_at            TIMESTAMPTZ,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);
