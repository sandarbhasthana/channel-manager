-- Casbin policy persistence. We use Casbin's standard rule layout
-- (ptype, v0..v5) so the table is compatible with off-the-shelf
-- adapters and tooling. We additionally enforce per-tenant isolation
-- via RLS, which inspects the rule's domain column based on its ptype:
--
--   p = sub, dom, obj, act           -> domain at v1
--   g = user, role, domain           -> domain at v2
--
-- The RLS predicate switches on ptype so both rule kinds are properly
-- scoped by app.current_org_id.

CREATE TABLE tenancy.casbin_rule (
    id         BIGSERIAL   PRIMARY KEY,
    ptype      TEXT        NOT NULL,
    v0         TEXT,
    v1         TEXT,
    v2         TEXT,
    v3         TEXT,
    v4         TEXT,
    v5         TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- The (ptype, v0..v5) tuple is the natural key. PostgreSQL treats
-- NULLs as distinct by default, which is what we want here.
CREATE UNIQUE INDEX casbin_rule_unique_idx ON tenancy.casbin_rule (ptype, v0, v1, v2, v3, v4, v5);

-- Common lookup paths. Two partial domain indexes cover both kinds.
CREATE INDEX casbin_rule_ptype_idx    ON tenancy.casbin_rule (ptype);
CREATE INDEX casbin_rule_p_domain_idx ON tenancy.casbin_rule (v1) WHERE ptype = 'p';
CREATE INDEX casbin_rule_g_domain_idx ON tenancy.casbin_rule (v2) WHERE ptype = 'g';

CREATE TRIGGER casbin_rule_set_updated_at BEFORE UPDATE ON tenancy.casbin_rule FOR EACH ROW EXECUTE FUNCTION tenancy.set_updated_at();

-- RLS: when app.current_org_id is set, only rows whose domain column
-- matches it (or rows with NULL domain) are visible/writable. Domain
-- lives in v1 for p rules and v2 for g rules. An empty setting
-- (i.e. unset) bypasses the check so the enforcer can bulk-load
-- policy at startup before tenant context exists.
ALTER TABLE tenancy.casbin_rule ENABLE ROW LEVEL SECURITY;
ALTER TABLE tenancy.casbin_rule FORCE  ROW LEVEL SECURITY;

CREATE POLICY casbin_rule_tenant_iso ON tenancy.casbin_rule
    USING      (current_setting('app.current_org_id', true) = '' OR (ptype = 'p' AND (v1 IS NULL OR v1 = current_setting('app.current_org_id', true))) OR (ptype = 'g' AND (v2 IS NULL OR v2 = current_setting('app.current_org_id', true))) OR (ptype NOT IN ('p', 'g')))
    WITH CHECK (current_setting('app.current_org_id', true) = '' OR (ptype = 'p' AND (v1 IS NULL OR v1 = current_setting('app.current_org_id', true))) OR (ptype = 'g' AND (v2 IS NULL OR v2 = current_setting('app.current_org_id', true))) OR (ptype NOT IN ('p', 'g')));

GRANT SELECT, INSERT, UPDATE, DELETE ON tenancy.casbin_rule TO app;
GRANT USAGE, SELECT ON SEQUENCE tenancy.casbin_rule_id_seq TO app;
