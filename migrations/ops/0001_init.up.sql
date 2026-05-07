-- ops schema: cross-cutting infrastructure tables.
--
--   ops.outbox           — transactional outbox; the event dispatcher polls
--                          this table and publishes to NATS / in-proc bus.
--   ops.idempotency_keys — dedupe table for OTA / PMS write requests.

CREATE SCHEMA IF NOT EXISTS ops;
SET LOCAL search_path = ops, public;

CREATE OR REPLACE FUNCTION ops.set_updated_at()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    NEW.updated_at := now();
    RETURN NEW;
END;
$$;

-- ── outbox ───────────────────────────────────────────────────────────────────
-- Every domain mutation writes a row here in the same transaction. A
-- separate dispatcher (Phase 4) atomically claims and publishes them.
CREATE TABLE ops.outbox (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID        NOT NULL,
    aggregate_type  TEXT        NOT NULL,
    aggregate_id    TEXT        NOT NULL,
    event_type      TEXT        NOT NULL,
    event_version   INT         NOT NULL DEFAULT 1,
    payload         JSONB       NOT NULL,
    idempotency_key TEXT,
    status          TEXT        NOT NULL DEFAULT 'pending'
                                CHECK (status IN ('pending', 'published', 'dead')),
    retry_count     INT         NOT NULL DEFAULT 0,
    available_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_error      TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    processed_at    TIMESTAMPTZ
);

CREATE INDEX outbox_pending_idx
    ON ops.outbox (available_at)
    WHERE status = 'pending';
CREATE INDEX outbox_org_aggregate_idx
    ON ops.outbox (org_id, aggregate_type, aggregate_id, created_at DESC);

ALTER TABLE ops.outbox ENABLE ROW LEVEL SECURITY;
ALTER TABLE ops.outbox FORCE  ROW LEVEL SECURITY;
CREATE POLICY outbox_tenant_iso ON ops.outbox
    USING       (org_id::text = current_setting('app.current_org_id', true))
    WITH CHECK  (org_id::text = current_setting('app.current_org_id', true));

-- ── idempotency_keys ────────────────────────────────────────────────────────
-- Required by the guide: every OTA write carries an idempotency key. The
-- key is unique per (org_id, scope) — `scope` distinguishes inventory vs
-- pricing vs reservations writes against the same OTA.
CREATE TABLE ops.idempotency_keys (
    org_id        UUID        NOT NULL,
    scope         TEXT        NOT NULL,
    key           TEXT        NOT NULL,
    request_hash  TEXT        NOT NULL,
    response      JSONB,
    status        TEXT        NOT NULL DEFAULT 'in_flight'
                              CHECK (status IN ('in_flight', 'completed', 'failed')),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at    TIMESTAMPTZ NOT NULL DEFAULT (now() + interval '7 days'),
    PRIMARY KEY (org_id, scope, key)
);
CREATE INDEX idempotency_keys_expires_idx ON ops.idempotency_keys (expires_at);
CREATE TRIGGER idempotency_keys_set_updated_at
    BEFORE UPDATE ON ops.idempotency_keys
    FOR EACH ROW EXECUTE FUNCTION ops.set_updated_at();

ALTER TABLE ops.idempotency_keys ENABLE ROW LEVEL SECURITY;
ALTER TABLE ops.idempotency_keys FORCE  ROW LEVEL SECURITY;
CREATE POLICY idempotency_keys_tenant_iso ON ops.idempotency_keys
    USING       (org_id::text = current_setting('app.current_org_id', true))
    WITH CHECK  (org_id::text = current_setting('app.current_org_id', true));

-- ── grants ──────────────────────────────────────────────────────────────────
GRANT USAGE  ON SCHEMA ops TO app;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES    IN SCHEMA ops TO app;
GRANT USAGE, SELECT                  ON ALL SEQUENCES IN SCHEMA ops TO app;
ALTER DEFAULT PRIVILEGES IN SCHEMA ops
    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES    TO app;
ALTER DEFAULT PRIVILEGES IN SCHEMA ops
    GRANT USAGE, SELECT                  ON SEQUENCES TO app;
