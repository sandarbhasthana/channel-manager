-- The organization's default property: the one the booking engine falls back to
-- when a guest request names none, and the one the dashboard's property picker
-- marks with a star.
--
-- Org-level rather than per-user. The star was previously a dashboard-only
-- convenience stored on tenancy.memberships.preferences (see
-- migrations/tenancy/0007_user_preferences), which the booking engine cannot
-- read: the storefront ingress authenticates with an org-scoped integration key
-- and has no membership behind it. Moving it here makes one default that the
-- dashboard, every user in the org, and the booking engine all agree on.
--
-- The partial unique index enforces at most one default per org, so promoting a
-- property must clear the incumbent in the same transaction.

ALTER TABLE pms.properties
    ADD COLUMN is_default BOOLEAN NOT NULL DEFAULT FALSE;

CREATE UNIQUE INDEX properties_default_uniq
    ON pms.properties (org_id)
    WHERE is_default;

-- Backfill so no org is left without a fallback the moment the booking engine
-- starts relying on one. The oldest active property is the closest thing to
-- "the original property" and is stable across re-runs.
--
-- Runs as the migration role (superuser), which bypasses the FORCE ROW LEVEL
-- SECURITY on this table; the runtime `app` role could not see across orgs here.
WITH ranked AS (
    SELECT id,
           row_number() OVER (PARTITION BY org_id ORDER BY created_at, id) AS rn
      FROM pms.properties
     WHERE is_active
)
UPDATE pms.properties p
   SET is_default = TRUE
  FROM ranked
 WHERE p.id = ranked.id
   AND ranked.rn = 1;
