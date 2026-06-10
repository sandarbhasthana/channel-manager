ALTER TABLE tenancy.memberships ADD preferences JSONB NOT NULL DEFAULT '{}'::jsonb;
