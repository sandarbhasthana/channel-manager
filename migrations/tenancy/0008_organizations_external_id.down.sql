DROP INDEX IF EXISTS tenancy.organizations_external_id_idx;
ALTER TABLE tenancy.organizations DROP COLUMN IF EXISTS external_id;
