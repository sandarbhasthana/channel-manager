-- Drop in reverse dependency order. The `app` role is left in place because
-- it may be referenced by other schemas; it is dropped only by a global
-- teardown (make db-reset).
DROP TABLE IF EXISTS tenancy.memberships;
DROP TABLE IF EXISTS tenancy.users;
DROP TABLE IF EXISTS tenancy.organizations;
DROP FUNCTION IF EXISTS tenancy.set_updated_at();
DROP SCHEMA IF EXISTS tenancy;
