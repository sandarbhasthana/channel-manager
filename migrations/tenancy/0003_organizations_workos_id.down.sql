DROP INDEX IF EXISTS tenancy.organizations_workos_id_idx;
ALTER TABLE tenancy.organizations DROP COLUMN IF EXISTS workos_id;
