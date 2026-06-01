SET LOCAL search_path = tenancy, public;

CREATE OR REPLACE FUNCTION tenancy.resolve_integration_api_key(p_prefix TEXT)
RETURNS TABLE (
    org_id UUID,
    key_hash TEXT,
    expires_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ
)
LANGUAGE sql
SECURITY DEFINER
AS $$
    SELECT org_id, key_hash, expires_at, revoked_at
    FROM tenancy.integration_api_keys
    WHERE key_prefix = p_prefix;
$$;

GRANT EXECUTE ON FUNCTION tenancy.resolve_integration_api_key(TEXT) TO app;
