-- Integration API keys for machine-to-machine access (PMS → Channel Manager).

SET LOCAL search_path = tenancy, public;

CREATE TABLE tenancy.integration_api_keys (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID        NOT NULL REFERENCES tenancy.organizations(id) ON DELETE CASCADE,
    name            TEXT        NOT NULL,
    key_prefix      TEXT        NOT NULL,
    key_hash        TEXT        NOT NULL,
    scopes          TEXT[]      NOT NULL DEFAULT '{integration:pms}',
    expires_at      TIMESTAMPTZ,
    revoked_at      TIMESTAMPTZ,
    created_by      TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (key_prefix)
);
CREATE INDEX integration_api_keys_org_idx ON tenancy.integration_api_keys (org_id)
    WHERE revoked_at IS NULL;

CREATE TRIGGER integration_api_keys_set_updated_at
    BEFORE UPDATE ON tenancy.integration_api_keys
    FOR EACH ROW EXECUTE FUNCTION tenancy.set_updated_at();

-- Keys are org-scoped; RLS uses org_id on the row.
ALTER TABLE tenancy.integration_api_keys ENABLE ROW LEVEL SECURITY;
ALTER TABLE tenancy.integration_api_keys FORCE ROW LEVEL SECURITY;
CREATE POLICY integration_api_keys_tenant_iso ON tenancy.integration_api_keys
    USING       (org_id::text = current_setting('app.current_org_id', true))
    WITH CHECK  (org_id::text = current_setting('app.current_org_id', true));

GRANT SELECT, INSERT, UPDATE ON tenancy.integration_api_keys TO app;
