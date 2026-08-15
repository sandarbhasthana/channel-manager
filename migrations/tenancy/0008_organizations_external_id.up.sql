-- Map a bundled tenant back to the PMS organization that owns it.
--
-- Two ways an organization can come to exist here:
--
--   standalone  -- the customer bought the Channel Manager on its own, signed in
--                  through WorkOS, and connects their own PMS. `workos_id` is set.
--   bundled     -- the customer bought the PMS, which includes this service. They
--                  never sign in here, so there is no WorkOS identity to mirror.
--                  `external_id` holds the PMS organization id instead.
--
-- So `workos_id IS NULL AND external_id IS NOT NULL` identifies a bundled tenant,
-- and the two columns are mutually exclusive in practice without needing a
-- constraint that would block a future migration between the two models.
--
-- Mirrors pms.properties.external_id, which already stores the PMS id for the
-- same reason. TEXT rather than UUID because PMS organization ids are cuids.
--
-- Partial unique index, matching organizations_workos_id_idx: NULLs are distinct
-- in Postgres anyway, but the partial form documents that "no external id" is a
-- normal state rather than something to be deduplicated.

ALTER TABLE tenancy.organizations
    ADD COLUMN external_id TEXT;

CREATE UNIQUE INDEX organizations_external_id_idx
    ON tenancy.organizations (external_id)
    WHERE external_id IS NOT NULL;
