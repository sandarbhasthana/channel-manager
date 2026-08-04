DROP INDEX IF EXISTS pms.properties_default_uniq;

ALTER TABLE pms.properties
    DROP COLUMN IF EXISTS is_default;
