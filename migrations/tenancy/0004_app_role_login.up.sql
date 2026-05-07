-- Enable LOGIN on the application role so the runtime can bind as
-- `app` and have RLS policies actually enforced. The migration runner
-- continues to bind as the superuser (postgres) since RLS owners and
-- DDL operations would otherwise be blocked.
--
-- The default password here is a development-only value. Operators
-- MUST rotate it after the migration runs:
--
--     psql -U postgres -d channel \
--          -c "ALTER ROLE app WITH PASSWORD '$APP_DB_PASSWORD'"
--
-- The runtime reads APP_DB_USER / APP_DB_PASSWORD from the
-- environment (see platform/config). Local dev defaults match the
-- value below.

ALTER ROLE app WITH LOGIN;
ALTER ROLE app WITH PASSWORD 'app_dev';
