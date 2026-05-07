-- Add a stable mapping from WorkOS organization IDs to local
-- organizations. WorkOS IDs are opaque prefixed strings (e.g.
-- "org_01H..."); the local primary key remains a UUID so existing
-- foreign keys and RLS comparisons (cast to text) keep working.
--
-- Identity sync writes workos_id on first sight of an org; subsequent
-- requests look up the local id by workos_id without round-tripping
-- to the WorkOS API.

ALTER TABLE tenancy.organizations
    ADD COLUMN workos_id TEXT;

CREATE UNIQUE INDEX organizations_workos_id_idx
    ON tenancy.organizations (workos_id)
    WHERE workos_id IS NOT NULL;
